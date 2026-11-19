// SPDX-License-Identifier: MIT

package store

import (
	"context"
	"errors"
	"path/filepath"
	"testing"
	"time"
)

func openCollectorTest(t *testing.T) *Store {
	t.Helper()
	s, err := Open(filepath.Join(t.TempDir(), "c.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { s.Close() })
	return s
}

func TestMET4_LastSuccessfulRunBefore(t *testing.T) {
	ctx := context.Background()
	s := openCollectorTest(t)
	t0 := time.Date(2030, 1, 1, 12, 0, 0, 0, time.UTC)
	if _, err := s.LastSuccessfulRunBefore(ctx, t0); !errors.Is(err, ErrNotFound) {
		t.Fatalf("empty: %v", err)
	}
	run := func(at time.Time, ok bool) int64 {
		id, err := s.StartCollectorRun(ctx, at)
		if err != nil {
			t.Fatal(err)
		}
		if err := s.FinishCollectorRun(ctx, id, at, ok, ""); err != nil {
			t.Fatal(err)
		}
		return id
	}
	a := run(t0, true)
	run(t0.Add(5*time.Minute), false)
	c := run(t0.Add(10*time.Minute), true)
	got, err := s.LastSuccessfulRunBefore(ctx, t0.Add(10*time.Minute))
	if err != nil || got.ID != a {
		t.Fatalf("before T10 = %+v %v, want run %d", got, err, a)
	}
	got, err = s.LastSuccessfulRunBefore(ctx, t0.Add(11*time.Minute))
	if err != nil || got.ID != c {
		t.Fatalf("before T11 = %+v %v, want run %d", got, err, c)
	}
	if _, err := s.LastSuccessfulRunBefore(ctx, t0); !errors.Is(err, ErrNotFound) {
		t.Fatalf("before T0: %v", err)
	}
}

func TestMET5_InsertPressureSampleIfAbsent(t *testing.T) {
	ctx := context.Background()
	s := openCollectorTest(t)
	at := time.Date(2030, 1, 1, 12, 0, 0, 0, time.UTC)
	ok, err := s.InsertPressureSampleIfAbsent(ctx, PressureSample{PolledAt: at, Source: "a", Raw: 0.5, Smoothed: 0.5, Multiplier: 1})
	if err != nil || !ok {
		t.Fatalf("first insert = %v %v", ok, err)
	}
	ok, err = s.InsertPressureSampleIfAbsent(ctx, PressureSample{PolledAt: at, Source: "b", Raw: 0.9, Smoothed: 0.9, Multiplier: 3})
	if err != nil || ok {
		t.Fatalf("second insert = %v %v", ok, err)
	}
	got, err := s.LatestPressure(ctx, 5)
	if err != nil || len(got) != 1 || got[0].Source != "a" || got[0].Raw != 0.5 {
		t.Fatalf("rows = %+v %v", got, err)
	}
}
