// SPDX-License-Identifier: MIT

package store

import (
	"testing"
	"time"
)

func TestHOLD_APIUserQueries(t *testing.T) {
	s := openTemp(t)
	u := newUser(t, s, "dev")
	a, b := newSetup(t, s, "a"), newSetup(t, s, "b")
	h1 := must(s.CreateHold(ctx, a, u.ID, t0))
	h2 := must(s.CreateHold(ctx, b, u.ID, t0))

	// No data yet: empty maps, no error (also for an empty id list).
	if m, err := s.HoldCharges(ctx, nil); err != nil || len(m) != 0 {
		t.Fatalf("empty ids: %v %v", m, err)
	}
	if m, err := s.HoldLastSamples(ctx, []int64{h1.ID, h2.ID}); err != nil || len(m) != 0 {
		t.Fatalf("no samples: %v %v", m, err)
	}

	for i, mt := range []int64{100, 250} {
		at := t0.Add(time.Duration(i+1) * 5 * time.Minute)
		if _, err := s.InsertUsageSample(ctx, UsageSample{HoldID: h1.ID, PolledAt: at, Weight: float64(i + 1), Multiplier: 2, DebitMT: mt, Reason: "r" + string(rune('0'+i))}); err != nil {
			t.Fatal(err)
		}
		h := h1.ID
		must(s.AppendLedger(ctx, LedgerEntry{UserID: u.ID, Period: "2026-11", Type: "debit", AmountMT: -mt, HoldID: &h}, "2026-11", at))
	}
	must(s.AppendLedger(ctx, LedgerEntry{UserID: u.ID, Period: "2026-10", Type: "allowance", AmountMT: 300000}, "2026-11", t0))
	must(s.AppendLedger(ctx, LedgerEntry{UserID: u.ID, Period: "2026-11", Type: "allowance", AmountMT: 300000}, "2026-11", t0))

	ch, err := s.HoldCharges(ctx, []int64{h1.ID, h2.ID})
	if err != nil || len(ch) != 1 || ch[h1.ID] != 350 {
		t.Fatalf("charges %v %v", ch, err)
	}
	ls, err := s.HoldLastSamples(ctx, []int64{h1.ID, h2.ID})
	if err != nil || len(ls) != 1 || ls[h1.ID].Weight != 2 || ls[h1.ID].Reason != "r1" || !ls[h1.ID].PolledAt.Equal(t0.Add(10*time.Minute)) {
		t.Fatalf("last samples %+v %v", ls, err)
	}
	ps, err := s.LedgerPeriods(ctx, u.ID)
	if err != nil || len(ps) != 2 || ps[0] != "2026-11" || ps[1] != "2026-10" {
		t.Fatalf("periods %v %v", ps, err)
	}
	es, err := s.DebitEntries(ctx, u.ID, t0, t0.Add(7*time.Minute))
	if err != nil || len(es) != 1 || es[0].MT != 100 {
		t.Fatalf("debit entries %+v %v", es, err)
	}
	es, _ = s.DebitEntries(ctx, u.ID, t0, t0.Add(time.Hour))
	if len(es) != 2 || es[1].MT != 250 {
		t.Fatalf("debit entries %+v", es)
	}

	mm, err := s.DebitMultipliers(ctx, u.ID, "2026-11")
	if err != nil || len(mm) != 2 {
		t.Fatalf("multipliers %v %v", mm, err)
	}
	for _, m := range mm {
		if m != 2 {
			t.Errorf("multiplier %v", m)
		}
	}

	// More ids than one chunk still works.
	ids := make([]int64, 0, idChunk+3)
	for i := 0; i < idChunk+2; i++ {
		ids = append(ids, 1000+int64(i))
	}
	ids = append(ids, h1.ID)
	if m, err := s.HoldCharges(ctx, ids); err != nil || m[h1.ID] != 350 {
		t.Fatalf("chunked %v %v", m, err)
	}
}
