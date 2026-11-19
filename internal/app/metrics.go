// SPDX-License-Identifier: MIT

package app

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"sync/atomic"
	"time"

	"github.com/panset/hourglass/internal/collector"
	"github.com/panset/hourglass/internal/store"
)

// metrics serves the Prometheus text exposition format (version 0.0.4),
// written by hand: poll duration, poll failures, lab pressure and tokens
// debited (non-functional requirements, "Observability"). It is mounted only
// on the separate metrics listener, never on the public port.
type metrics struct {
	started      time.Time
	pollFailures atomic.Int64 // poll.failed events seen since start
}

func newMetrics(started time.Time) *metrics { return &metrics{started: started} }

// source is what a scrape reads.
type metricSource interface {
	LatestRun(ctx context.Context) (store.CollectorRun, error)
	SumAllDebitsSince(ctx context.Context, since time.Time) (int64, error)
}

type pollSource interface{ LastPoll() collector.PollView }

func (m *metrics) handler(st metricSource, c pollSource, log *slog.Logger) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /metrics", func(w http.ResponseWriter, r *http.Request) {
		var sb strings.Builder
		m.write(r.Context(), &sb, st, c, log)
		w.Header().Set("Content-Type", "text/plain; version=0.0.4; charset=utf-8")
		w.Header().Set("Cache-Control", "no-store")
		_, _ = io.WriteString(w, sb.String())
	})
	return mux
}

func (m *metrics) write(ctx context.Context, w *strings.Builder, st metricSource, c pollSource, log *slog.Logger) {
	metric := func(name, typ, help string) {
		fmt.Fprintf(w, "# HELP %s %s\n# TYPE %s %s\n", name, help, name, typ)
	}

	metric("hourglass_poll_duration_seconds", "gauge", "Duration of the most recent finished poll.")
	if run, err := st.LatestRun(ctx); err == nil && run.FinishedAt != nil {
		fmt.Fprintf(w, "hourglass_poll_duration_seconds %s\n", num(run.FinishedAt.Sub(run.StartedAt).Seconds()))
	} else if err != nil && err != store.ErrNotFound {
		log.Error("metrics: reading the latest poll failed", "err", err)
	}

	metric("hourglass_poll_failures_total", "counter", "Polls that failed since the process started.")
	fmt.Fprintf(w, "hourglass_poll_failures_total %d\n", m.pollFailures.Load())

	metric("hourglass_lab_pressure", "gauge", "Lab pressure of the last successful poll, from 0 to 1.")
	if v := c.LastPoll(); v.OK {
		fmt.Fprintf(w, "hourglass_lab_pressure{kind=\"raw\"} %s\n", num(v.RawPressure))
		fmt.Fprintf(w, "hourglass_lab_pressure{kind=\"smoothed\"} %s\n", num(v.SmoothedPressure))
	}

	metric("hourglass_tokens_debited_total", "counter", "Tokens debited by metering since the process started.")
	if mt, err := st.SumAllDebitsSince(ctx, m.started); err == nil {
		fmt.Fprintf(w, "hourglass_tokens_debited_total %s\n", num(float64(mt)/1000))
	} else {
		log.Error("metrics: summing debits failed", "err", err)
	}
}

// num formats a float the way the exposition format wants it.
func num(f float64) string { return fmt.Sprintf("%g", f) }
