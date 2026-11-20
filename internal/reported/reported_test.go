// SPDX-License-Identifier: MIT

package reported

import (
	"context"
	"errors"
	"math"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/panset/hourglass/internal/store"
)

var (
	ctx = context.Background()
	t0  = time.Date(2026, 11, 20, 10, 0, 0, 0, time.UTC)
)

type fakeClock struct {
	mu sync.Mutex
	t  time.Time
}

func (c *fakeClock) Now() time.Time  { c.mu.Lock(); defer c.mu.Unlock(); return c.t }
func (c *fakeClock) Set(t time.Time) { c.mu.Lock(); c.t = t; c.mu.Unlock() }

func open(t *testing.T, path string) *store.Store {
	t.Helper()
	s, err := store.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { s.Close() })
	return s
}

func newSvc(t *testing.T) (*Service, *fakeClock) {
	t.Helper()
	clk := &fakeClock{t: t0}
	return New(open(t, filepath.Join(t.TempDir(), "h.db")), clk), clk
}

func at(d time.Duration) *time.Time { v := t0.Add(d); return &v }

func deps(names ...string) []Deployment {
	var out []Deployment
	for _, n := range names {
		out = append(out, Deployment{Setup: n, Labels: map[string]string{"k": n}})
	}
	return out
}

func TestNoReportYet(t *testing.T) {
	svc, _ := newSvc(t)
	if _, err := svc.Pressure(ctx); !errors.Is(err, ErrNoReport) {
		t.Errorf("pressure: %v", err)
	}
	if _, err := svc.Deployments(ctx); !errors.Is(err, ErrNoReport) {
		t.Errorf("deployments: %v", err)
	}
}

func TestINT1_PressureValueValidatedAndNothingStored(t *testing.T) {
	svc, _ := newSvc(t)
	for _, v := range []float64{-0.01, 1.01, math.NaN(), math.Inf(1), math.Inf(-1)} {
		_, err := svc.SubmitPressure(ctx, "svc", v, nil)
		var ve *ValidationError
		if !errors.As(err, &ve) {
			t.Errorf("value %v: err = %v", v, err)
		}
	}
	if _, err := svc.Pressure(ctx); !errors.Is(err, ErrNoReport) {
		t.Errorf("something was stored: %v", err)
	}
	for _, v := range []float64{0, 1, 0.5} {
		if _, err := svc.SubmitPressure(ctx, "svc", v, nil); err != nil {
			t.Errorf("value %v refused: %v", v, err)
		}
	}
}

func TestPressureDefaultsObservedAtAndRecordsReceipt(t *testing.T) {
	svc, clk := newSvc(t)
	got, err := svc.SubmitPressure(ctx, "svc-a", 0.72, nil)
	if err != nil || !got.ObservedAt.Equal(t0) || !got.ReceivedAt.Equal(t0) || got.ServiceAccount != "svc-a" || got.Age != 0 {
		t.Fatalf("submit: %+v %v", got, err)
	}
	clk.Set(t0.Add(3 * time.Minute))
	r, err := svc.Pressure(ctx)
	if err != nil || r.Value != 0.72 || r.Age != 3*time.Minute || r.ServiceAccount != "svc-a" {
		t.Fatalf("read: %+v %v", r, err)
	}
	// Explicit past observed_at: age counts from it, received_at is receipt time.
	got, err = svc.SubmitPressure(ctx, "svc-b", 0.1, at(2*time.Minute))
	if err != nil || !got.ObservedAt.Equal(t0.Add(2*time.Minute)) || !got.ReceivedAt.Equal(t0.Add(3*time.Minute)) || got.Age != time.Minute {
		t.Fatalf("explicit: %+v %v", got, err)
	}
	if Fresh(r.Age, 2*time.Minute) || !Fresh(r.Age, 3*time.Minute) {
		t.Error("Fresh boundary")
	}
	if !ExpiresAt(t0, 10*time.Minute).Equal(t0.Add(10 * time.Minute)) {
		t.Error("ExpiresAt")
	}
}

func TestClockSkew(t *testing.T) {
	svc, _ := newSvc(t)
	var ve *ValidationError
	if _, err := svc.SubmitPressure(ctx, "s", 0.5, at(2*time.Minute+time.Second)); !errors.As(err, &ve) {
		t.Errorf("pressure beyond skew: %v", err)
	}
	if _, err := svc.SubmitDeployments(ctx, "s", at(2*time.Minute+time.Second), nil); !errors.As(err, &ve) {
		t.Errorf("deployments beyond skew: %v", err)
	}
	if _, err := svc.Pressure(ctx); !errors.Is(err, ErrNoReport) {
		t.Error("skewed report stored")
	}
	if _, err := svc.SubmitPressure(ctx, "s", 0.5, at(2*time.Minute)); err != nil {
		t.Errorf("exactly 2 minutes: %v", err)
	}
	if _, err := svc.SubmitDeployments(ctx, "s", at(2*time.Minute), nil); err != nil {
		t.Errorf("exactly 2 minutes: %v", err)
	}
}

func TestINT3_OlderPressureRefusedNothingChanges(t *testing.T) {
	svc, _ := newSvc(t)
	if _, err := svc.SubmitPressure(ctx, "first", 0.4, at(-time.Minute)); err != nil {
		t.Fatal(err)
	}
	before, _ := svc.Pressure(ctx)
	_, err := svc.SubmitPressure(ctx, "second", 0.9, at(-2*time.Minute))
	var se *StaleError
	if !errors.As(err, &se) || !se.Stored.Equal(t0.Add(-time.Minute)) {
		t.Fatalf("err = %v", err)
	}
	after, _ := svc.Pressure(ctx)
	if after != before {
		t.Errorf("changed: %+v -> %+v", before, after)
	}
}

func TestINT3_EqualPressureReplacesAndIsIdempotent(t *testing.T) {
	svc, clk := newSvc(t)
	obs := at(-time.Minute)
	a, err := svc.SubmitPressure(ctx, "s", 0.4, obs)
	if err != nil {
		t.Fatal(err)
	}
	clk.Set(t0.Add(30 * time.Second))
	b, err := svc.SubmitPressure(ctx, "s", 0.4, obs) // a retry
	if err != nil || b.Value != a.Value || !b.ObservedAt.Equal(a.ObservedAt) {
		t.Fatalf("retry: %+v %v", b, err)
	}
	c, err := svc.SubmitPressure(ctx, "s", 0.6, obs) // equal observed_at replaces
	if err != nil || c.Value != 0.6 {
		t.Fatalf("equal replace: %+v %v", c, err)
	}
	if g, _ := svc.Pressure(ctx); g.Value != 0.6 || !g.ReceivedAt.Equal(t0.Add(30*time.Second)) {
		t.Errorf("stored: %+v", g)
	}
}

func TestINT3_OlderDeploymentsRefusedNothingChanges(t *testing.T) {
	svc, _ := newSvc(t)
	if _, err := svc.SubmitDeployments(ctx, "first", at(-time.Minute), deps("a", "b")); err != nil {
		t.Fatal(err)
	}
	before, _ := svc.Deployments(ctx)
	_, err := svc.SubmitDeployments(ctx, "second", at(-2*time.Minute), deps("z"))
	var se *StaleError
	if !errors.As(err, &se) {
		t.Fatalf("err = %v", err)
	}
	_, err = svc.SubmitDeployments(ctx, "second", at(-2*time.Minute), nil) // an older empty report too
	if !errors.As(err, &se) {
		t.Fatalf("empty: %v", err)
	}
	after, _ := svc.Deployments(ctx)
	if after.ServiceAccount != before.ServiceAccount || !after.ObservedAt.Equal(before.ObservedAt) ||
		len(after.Deployments) != 2 || after.Deployments[0].Setup != "a" || after.Deployments[1].Setup != "b" {
		t.Errorf("changed: %+v -> %+v", before, after)
	}
}

func TestINT3_EqualDeploymentsIdempotent(t *testing.T) {
	svc, _ := newSvc(t)
	obs := at(-time.Minute)
	if _, err := svc.SubmitDeployments(ctx, "s", obs, deps("a")); err != nil {
		t.Fatal(err)
	}
	first, _ := svc.Deployments(ctx)
	if _, err := svc.SubmitDeployments(ctx, "s", obs, deps("a")); err != nil {
		t.Fatalf("retry: %v", err)
	}
	again, _ := svc.Deployments(ctx)
	if len(again.Deployments) != 1 || again.Deployments[0].Setup != "a" || !again.ObservedAt.Equal(first.ObservedAt) {
		t.Errorf("after retry: %+v", again)
	}
	if _, err := svc.SubmitDeployments(ctx, "s", obs, deps("b", "c")); err != nil {
		t.Fatalf("equal replace: %v", err)
	}
	if g, _ := svc.Deployments(ctx); len(g.Deployments) != 2 {
		t.Errorf("equal observed_at did not replace: %+v", g)
	}
}

func TestDeploymentsValidationAndEmptySet(t *testing.T) {
	svc, _ := newSvc(t)
	var ve *ValidationError
	if _, err := svc.SubmitDeployments(ctx, "s", nil, deps("a", "b", "a")); !errors.As(err, &ve) {
		t.Errorf("duplicate: %v", err)
	}
	if _, err := svc.SubmitDeployments(ctx, "s", nil, deps("")); !errors.As(err, &ve) {
		t.Errorf("empty name: %v", err)
	}
	if _, err := svc.Deployments(ctx); !errors.Is(err, ErrNoReport) {
		t.Error("invalid report stored")
	}
	got, err := svc.SubmitDeployments(ctx, "s", nil, nil)
	if err != nil || got.Deployments == nil || len(got.Deployments) != 0 {
		t.Fatalf("empty: %+v %v", got, err)
	}
	r, err := svc.Deployments(ctx)
	if err != nil || r.Deployments == nil || len(r.Deployments) != 0 || r.ServiceAccount != "s" {
		t.Errorf("empty read: %+v %v (an empty report is not 'no report')", r, err)
	}
}

// A concurrent reader never sees a header of one report with the set of
// another, and a refused or failed replace changes nothing.
func TestINT2_ReplaceIsAtomicForConcurrentReaders(t *testing.T) {
	svc, clk := newSvc(t)
	clk.Set(t0.Add(time.Hour))
	names := []string{"a", "b", "c", "d", "e"}
	mk := func(i int) (*time.Time, []Deployment) {
		return at(time.Duration(i) * time.Second), deps(names[:i%5]...)
	}
	o, d := mk(0)
	if _, err := svc.SubmitDeployments(ctx, "s", o, d); err != nil {
		t.Fatal(err)
	}
	var wg sync.WaitGroup
	stop := make(chan struct{})
	bad := make(chan string, 1)
	report := func(m string) {
		select {
		case bad <- m:
		default:
		}
	}
	for r := 0; r < 3; r++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for {
				select {
				case <-stop:
					return
				default:
				}
				got, err := svc.Deployments(ctx)
				if err != nil {
					report(err.Error())
					return
				}
				i := int(got.ObservedAt.Sub(t0) / time.Second)
				if len(got.Deployments) != i%5 {
					report("mixed report")
					return
				}
				for j, x := range got.Deployments {
					if x.Setup != names[j] || x.Labels["k"] != names[j] {
						report("mixed set")
						return
					}
				}
			}
		}()
	}
	for i := 1; i <= 60; i++ {
		o, d := mk(i)
		if _, err := svc.SubmitDeployments(ctx, "s", o, d); err != nil {
			t.Fatal(err)
		}
	}
	close(stop)
	wg.Wait()
	select {
	case m := <-bad:
		t.Fatal(m)
	default:
	}
}

func TestINT2_FailedReplaceIsAllOrNothing(t *testing.T) {
	svc, _ := newSvc(t)
	if _, err := svc.SubmitDeployments(ctx, "s", at(-time.Minute), deps("a", "b")); err != nil {
		t.Fatal(err)
	}
	// A newer report that fails halfway in the store (duplicate row, bypassing
	// the service's own check) must leave the old report whole.
	err := svc.st.ReplaceReportedDeployments(ctx,
		store.ReportedHeader{ObservedAt: t0, ReceivedAt: t0, ServiceAccount: "other"},
		[]store.ReportedDeployment{{SetupName: "x"}, {SetupName: "x"}})
	if err == nil {
		t.Fatal("expected failure")
	}
	got, _ := svc.Deployments(ctx)
	if got.ServiceAccount != "s" || len(got.Deployments) != 2 || !got.ObservedAt.Equal(t0.Add(-time.Minute)) {
		t.Errorf("partial replace visible: %+v", got)
	}
}

func TestINT6_ReportsSurviveReopen(t *testing.T) {
	path := filepath.Join(t.TempDir(), "h.db")
	clk := &fakeClock{t: t0}
	st, err := store.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	svc := New(st, clk)
	if _, err := svc.SubmitPressure(ctx, "svc", 0.33, at(-time.Minute)); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.SubmitDeployments(ctx, "svc", at(-time.Minute), deps("lab-1", "lab-2")); err != nil {
		t.Fatal(err)
	}
	st.Close()

	clk.Set(t0.Add(4 * time.Minute))
	svc2 := New(open(t, path), clk)
	p, err := svc2.Pressure(ctx)
	if err != nil || p.Value != 0.33 || p.ServiceAccount != "svc" || p.Age != 5*time.Minute {
		t.Errorf("pressure after reopen: %+v %v", p, err)
	}
	d, err := svc2.Deployments(ctx)
	if err != nil || len(d.Deployments) != 2 || d.Deployments[0].Setup != "lab-1" || d.Deployments[1].Labels["k"] != "lab-2" || d.Age != 5*time.Minute {
		t.Errorf("deployments after reopen: %+v %v", d, err)
	}
}
