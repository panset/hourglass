// SPDX-License-Identifier: MIT

package api

// Adopted from verification task 42: GET /admin/users (all, pending) and GET
// /setups compared field by field with figures computed independently from
// the ledger rows and the holds, not from the balance cache.

import (
	"context"
	"encoding/json"
	"math"
	"testing"
	"time"

	"github.com/panset/hourglass/internal/store"
)

func TestADM1_LED2_ResponsesMatchLedgerAndHolds(t *testing.T) {
	ctx := context.Background()
	e := newAdEnv(t)
	e.pressure.SetValue(0.8)
	// Users created at distinct times so the pending order is meaningful.
	p2 := e.user("pend-b", store.StatusPending)
	e.clk.Advance(time.Minute)
	over := e.dev("zz-over")
	rich := e.dev("aa-rich")
	idle := e.dev("mm-idle")
	norole := e.dev("nn-norole")
	e.clk.Advance(time.Minute)
	p1 := e.user("pend-a", store.StatusPending)
	e.clk.Advance(time.Minute)
	p3 := e.user("pend-c", store.StatusPending)
	_ = e.user("rej", store.StatusRejected)
	zero := e.dev("zero")
	if _, err := e.led.Grant(ctx, e.admin.ID, over.user.ID, -400000, "drive negative"); err != nil {
		t.Fatal(err)
	}
	if _, err := e.led.Grant(ctx, e.admin.ID, zero.user.ID, -300000, "to zero"); err != nil {
		t.Fatal(err)
	}
	s1 := e.setup("lab-1", true, map[string]string{"tier": "a"})
	s2 := e.setup("lab-2", true, map[string]string{"tier": "b"})
	s3 := e.setup("lab-3", true, map[string]string{"tier": "b"})
	e.setup("lab-4", true, nil)  // free
	e.setup("lab-5", false, nil) // disabled
	s6 := e.setup("lab-6", true, nil)
	e.hold(s1, over.user.ID)
	e.hold(s2, rich.user.ID)
	e.hold(s3, rich.user.ID)
	for i := 0; i < 6; i++ {
		if err := e.col.RunOnce(ctx); err != nil {
			t.Fatal(err)
		}
		e.clk.Advance(5 * time.Minute)
	}
	e.hold(s6, idle.user.ID) // held, not yet metering
	// norole had an allowance and a month without debits, then loses its role.
	if err := e.st.SetUserRoles(ctx, norole.user.ID, nil); err != nil {
		t.Fatal(err)
	}
	period := e.led.CurrentPeriod()

	// Independent figures: sums over the ledger rows of the period (every entry
	// of this test is in it) and the active holds.
	ledgerOf := func(id int64) (n int, bal, mtd int64) {
		rows, err := e.st.ListLedger(ctx, id, period)
		if err != nil {
			t.Fatal(err)
		}
		for _, r := range rows {
			bal += r.AmountMT
			if r.Type == "debit" {
				mtd -= r.AmountMT
			}
		}
		return len(rows), bal, mtd
	}
	active, err := e.st.ActiveHolds(ctx)
	if err != nil {
		t.Fatal(err)
	}
	holdsOf := map[int64]int{}
	holdOnSetup := map[int64]store.Hold{}
	for _, h := range active {
		holdsOf[h.UserID]++
		holdOnSetup[h.SetupID] = h
	}

	type U struct {
		ID        int64    `json:"id"`
		Username  string   `json:"username"`
		Status    string   `json:"status"`
		Roles     []string `json:"roles"`
		Local     bool     `json:"is_local_admin"`
		Balance   *float64 `json:"balance_tokens"`
		Overdrawn bool     `json:"overdrawn"`
		MTD       *float64 `json:"month_to_date_burn_tokens"`
		Holds     []struct {
			HoldID int64 `json:"hold_id"`
		} `json:"active_holds"`
		First string `json:"first_sign_in_at"`
	}
	type L struct {
		Users   []U            `json:"users"`
		Summary map[string]any `json:"summary"`
		Pending int            `json:"pending_count"`
	}
	get := func(path string) L {
		rec := e.get(path)
		if rec.Code != 200 {
			t.Fatalf("%s: %d %s", path, rec.Code, rec.Body)
		}
		var l L
		if err := json.Unmarshal(rec.Body.Bytes(), &l); err != nil {
			t.Fatal(err)
		}
		return l
	}
	tok := func(mt int64) float64 { return float64(mt) / 1000 }
	near := func(a, b float64) bool { return math.Abs(a-b) < 1e-9 }

	all := get("/api/v1/admin/users")
	everyone, err := e.st.ListUsers(ctx, "")
	if err != nil {
		t.Fatal(err)
	}
	if len(all.Users) != len(everyone) {
		t.Errorf("list has %d users, table %d", len(all.Users), len(everyone))
	}
	overdrawnN := 0
	for _, u := range all.Users {
		n, bal, mtd := ledgerOf(u.ID)
		roles, err := e.st.GetUserRoles(ctx, u.ID)
		if err != nil {
			t.Fatal(err)
		}
		if holdsOf[u.ID] != len(u.Holds) {
			t.Errorf("%s: %d holds, table %d", u.Username, len(u.Holds), holdsOf[u.ID])
		}
		if u.Local {
			if u.Balance != nil || u.MTD == nil || *u.MTD != 0 {
				t.Errorf("local admin: balance %v, burn %v", u.Balance, u.MTD)
			}
			continue
		}
		switch {
		case len(roles) == 0: // LED-3: no allowance, no figures
			if u.Balance != nil || u.MTD != nil {
				t.Errorf("user %d has no role: balance %v, burn %v, want null", u.ID, deref(u.Balance), deref(u.MTD))
			}
		case n == 0:
			if u.Balance != nil {
				t.Errorf("%s: balance %v without a ledger", u.Username, *u.Balance)
			}
		default:
			if u.Balance == nil || !near(*u.Balance, tok(bal)) {
				t.Errorf("%s: balance %v, ledger %v", u.Username, u.Balance, tok(bal))
			}
			if u.MTD == nil || !near(*u.MTD, tok(mtd)) {
				t.Errorf("%s: burn %v, ledger %v", u.Username, u.MTD, tok(mtd))
			}
		}
		wantOver := len(roles) > 0 && n > 0 && bal <= 0
		if u.Overdrawn != wantOver {
			t.Errorf("%s: overdrawn %v, want %v", u.Username, u.Overdrawn, wantOver)
		}
		if wantOver {
			overdrawnN++
		}
	}
	if overdrawnN != 2 {
		t.Errorf("%d overdrawn users, want 2 (negative and exactly zero)", overdrawnN)
	}
	for i := 1; i < len(all.Users); i++ {
		if a, b := all.Users[i-1], all.Users[i]; !a.Overdrawn && b.Overdrawn && a.Username != "admin" {
			t.Errorf("order: %s before overdrawn %s", a.Username, b.Username)
		}
	}
	if got, ok := all.Summary["overdrawn_count"].(float64); !ok || int(got) != overdrawnN {
		t.Errorf("summary overdrawn_count %v, want %d", all.Summary["overdrawn_count"], overdrawnN)
	}

	// Pending: only pending users, oldest first, and the count.
	pend := get("/api/v1/admin/users?status=pending")
	want := []int64{p2.user.ID, p1.user.ID, p3.user.ID}
	if len(pend.Users) != len(want) || pend.Pending != len(want) {
		t.Fatalf("pending list %d users, count %d, want %d", len(pend.Users), pend.Pending, len(want))
	}
	for i, u := range pend.Users {
		if u.ID != want[i] || u.Status != "pending" || u.Balance != nil || u.MTD != nil {
			t.Errorf("pending[%d] = %+v, want user %d with null figures", i, u, want[i])
		}
	}

	// GET /setups as a developer and as the admin: holder, balance from the
	// ledger, and a cost on every row.
	type S struct {
		ID     int64  `json:"id"`
		Name   string `json:"name"`
		State  string `json:"state"`
		Holder *struct {
			UserID  int64   `json:"user_id"`
			HoldID  int64   `json:"hold_id"`
			Balance float64 `json:"balance_tokens"`
			Over    bool    `json:"overdrawn"`
		} `json:"holder"`
		Cost *costView `json:"cost_per_hour_tokens"`
	}
	enabled, err := e.st.ListSetups(ctx, true)
	if err != nil {
		t.Fatal(err)
	}
	every, err := e.st.ListSetups(ctx, false)
	if err != nil {
		t.Fatal(err)
	}
	for _, who := range []*adSess{&rich, &e.adminS} {
		rec := e.do("GET", "/api/v1/setups", who, false, "")
		if rec.Code != 200 {
			t.Fatal(rec.Code)
		}
		var body struct {
			Setups []S `json:"setups"`
		}
		if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
			t.Fatal(err)
		}
		wantN := len(enabled)
		if who == &e.adminS {
			wantN = len(every)
		}
		if len(body.Setups) != wantN {
			t.Errorf("%s sees %d setups, want %d", who.user.Name, len(body.Setups), wantN)
		}
		for _, s := range body.Setups {
			h, held := holdOnSetup[s.ID]
			if !held {
				if s.Holder != nil {
					t.Errorf("%s: holder shown on a free setup", s.Name)
				}
			} else {
				_, bal, _ := ledgerOf(h.UserID)
				if s.Holder == nil || s.Holder.UserID != h.UserID || s.Holder.HoldID != h.ID || !near(s.Holder.Balance, tok(bal)) || s.Holder.Over != (bal <= 0) {
					t.Errorf("%s: holder %+v, want user %d hold %d balance %v", s.Name, s.Holder, h.UserID, h.ID, tok(bal))
				}
			}
			if s.Cost == nil || s.Cost.Now <= 0 {
				t.Errorf("%s (%s): cost per hour %+v", s.Name, s.State, s.Cost)
			}
		}
	}
}

func deref(p *float64) any {
	if p == nil {
		return nil
	}
	return *p
}
