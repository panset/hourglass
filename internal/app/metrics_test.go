// SPDX-License-Identifier: MIT

package app

import (
	"context"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/panset/hourglass/internal/collector"
	"github.com/panset/hourglass/internal/store"
)

type fakeMetricSource struct {
	run   store.CollectorRun
	runEr error
	debit int64
	got   time.Time
}

func (f *fakeMetricSource) LatestRun(context.Context) (store.CollectorRun, error) {
	return f.run, f.runEr
}
func (f *fakeMetricSource) SumAllDebitsSince(_ context.Context, since time.Time) (int64, error) {
	f.got = since
	return f.debit, nil
}

type fakePoll struct{ v collector.PollView }

func (f fakePoll) LastPoll() collector.PollView { return f.v }

func TestNFR_MetricsTextFormat(t *testing.T) {
	t0 := time.Date(2030, 1, 1, 0, 0, 0, 0, time.UTC)
	fin := t0.Add(1500 * time.Millisecond)
	start := t0.Add(-time.Hour)
	src := &fakeMetricSource{run: store.CollectorRun{StartedAt: t0, FinishedAt: &fin}, debit: 12345}
	m := newMetrics(start)
	m.pollFailures.Add(3)
	h := m.handler(src, fakePoll{collector.PollView{OK: true, RawPressure: 0.75, SmoothedPressure: 0.5}}, slog.New(slog.NewJSONHandler(io.Discard, nil)))
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest("GET", "/metrics", nil))
	if rec.Code != 200 || !strings.HasPrefix(rec.Header().Get("Content-Type"), "text/plain; version=0.0.4") {
		t.Fatalf("%d %v", rec.Code, rec.Header())
	}
	want := `# HELP hourglass_poll_duration_seconds Duration of the most recent finished poll.
# TYPE hourglass_poll_duration_seconds gauge
hourglass_poll_duration_seconds 1.5
# HELP hourglass_poll_failures_total Polls that failed since the process started.
# TYPE hourglass_poll_failures_total counter
hourglass_poll_failures_total 3
# HELP hourglass_lab_pressure Lab pressure of the last successful poll, from 0 to 1.
# TYPE hourglass_lab_pressure gauge
hourglass_lab_pressure{kind="raw"} 0.75
hourglass_lab_pressure{kind="smoothed"} 0.5
# HELP hourglass_tokens_debited_total Tokens debited by metering since the process started.
# TYPE hourglass_tokens_debited_total counter
hourglass_tokens_debited_total 12.345
`
	if rec.Body.String() != want {
		t.Fatalf("got:\n%s\nwant:\n%s", rec.Body, want)
	}
	if !src.got.Equal(start) {
		t.Errorf("debits counted since %v, want process start %v", src.got, start)
	}
	// Before the first poll: no pressure or duration samples, counters still present.
	src = &fakeMetricSource{runEr: store.ErrNotFound}
	h = m.handler(src, fakePoll{}, slog.New(slog.NewJSONHandler(io.Discard, nil)))
	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest("GET", "/metrics", nil))
	body := rec.Body.String()
	if strings.Contains(body, "hourglass_lab_pressure{") || strings.Contains(body, "\nhourglass_poll_duration_seconds ") ||
		!strings.Contains(body, "hourglass_tokens_debited_total 0\n") {
		t.Errorf("empty state:\n%s", body)
	}
	// Only GET /metrics exists.
	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/metrics", nil))
	if rec.Code == 200 {
		t.Error("POST /metrics answered 200")
	}
}
