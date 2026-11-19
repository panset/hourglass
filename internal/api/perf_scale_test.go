// SPDX-License-Identifier: MIT

package api

// NFR (spec, "Scale and performance"): up to 200 users, 100 setups and
// 5-minute polling; any API call answers in under 500 ms on that data.
//
// TestNFR_ApiUnder500msAtSpecScale seeds a data set of that shape and times
// every GET route of the route table through the real router.
//
//   - Default run (CI): a reduced data set of the same shape, twice at two
//     sizes. Every route must run the same number of SQL statements at both
//     sizes (a per-row query creeping back makes the count grow with the data
//     set) and answer inside a generous time bound.
//   - Full size: HOURGLASS_SCALE_TEST=1 seeds 200 users, 100 setups, 100 active
//     holds and 18 days of 5-minute polls (518,400 usage samples and ledger
//     debits; seeding takes about half a minute), requires every route under
//     500 ms and logs each route's time and one collector poll's time. See
//     CONTRIBUTING.md.

import (
	"context"
	"fmt"
	"os"
	"sort"
	"testing"
	"time"

	"github.com/panset/hourglass/internal/store"
)

// scaleShape is the size of a seeded data set.
type scaleShape struct{ users, setups, holds, days int }

var (
	fullScale    = scaleShape{users: 200, setups: 100, holds: 100, days: 18}
	reducedSmall = scaleShape{users: 12, setups: 6, holds: 6, days: 1}
	reducedLarge = scaleShape{users: 48, setups: 24, holds: 24, days: 1}
)

type scaleEnv struct {
	b      *backend
	devs   []sess
	hold   sess // a developer who holds setups
	admin  sess
	free   sess // a developer with no hold
	polls  int
	seeded time.Duration
}

// seedScale fills the database as 5-minute polling would have: per poll one
// collector run and pressure sample, and per active hold a usage sample and a
// ledger debit. Holds are spread two per user over the first users.
func seedScale(t *testing.T, sh scaleShape) *scaleEnv {
	t.Helper()
	start := time.Date(2026, 11, 1, 0, 0, 0, 0, time.UTC)
	now := start.Add(time.Duration(sh.days) * 24 * time.Hour)
	b := newBackend(t, backendOpts{start: now})
	ctx := context.Background()
	t0 := time.Now()
	e := &scaleEnv{b: b}
	for i := 0; i < sh.users-1; i++ {
		role := store.RoleDeveloper
		if i%40 == 0 {
			role = store.RoleAdmin
		}
		e.devs = append(e.devs, b.newUser(fmt.Sprintf("user-%03d", i), store.StatusActive, role))
	}
	// One pending user so the pending filter has something to return.
	b.newUser("pending-user", store.StatusPending)
	tiers := []string{"t1", "t2", "t3"}
	var setupIDs []int64
	for i := 0; i < sh.setups; i++ {
		id, err := b.st.CreateSetup(ctx, store.Setup{Name: fmt.Sprintf("lab-%03d", i), Enabled: true,
			Labels: map[string]string{"tier": tiers[i%3], "site": fmt.Sprintf("s%d", i%4)}, DetailsJSON: `{"note":"x"}`})
		if err != nil {
			t.Fatal(err)
		}
		setupIDs = append(setupIDs, id)
	}
	type hh struct{ id, user int64 }
	var hs []hh
	for i := 0; i < sh.holds; i++ {
		u := e.devs[i/2].user.ID
		h, err := b.st.CreateHold(ctx, setupIDs[i], u, start)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := b.st.SetMeteringStarted(ctx, h.ID, start); err != nil {
			t.Fatal(err)
		}
		hs = append(hs, hh{h.ID, u})
	}
	for p := start.Add(5 * time.Minute); !p.After(now); p = p.Add(5 * time.Minute) {
		e.polls++
	}
	for p := start.Add(5 * time.Minute); !p.After(now); {
		end := p.Add(24 * time.Hour)
		err := b.st.Tx(ctx, func(tx *store.Tx) error {
			for ; p.Before(end) && !p.After(now); p = p.Add(5 * time.Minute) {
				run, err := tx.StartCollectorRun(ctx, p)
				if err != nil {
					return err
				}
				if err := tx.FinishCollectorRun(ctx, run, p.Add(time.Second), true, ""); err != nil {
					return err
				}
				if err := tx.InsertPressureSample(ctx, store.PressureSample{PolledAt: p, Source: "inventory", Raw: 0.8, Smoothed: 0.8, Multiplier: 2}); err != nil {
					return err
				}
				for _, h := range hs {
					hid := h.id
					if _, err := tx.InsertUsageSample(ctx, store.UsageSample{HoldID: hid, PolledAt: p, Weight: 2, Pressure: 0.8, Multiplier: 2, DebitMT: 333, Reason: "metering"}); err != nil {
						return err
					}
					if _, err := tx.AppendLedger(ctx, store.LedgerEntry{UserID: h.user, Period: "2026-11", Type: "debit", AmountMT: -333, HoldID: &hid}, "2026-11", p); err != nil {
						return err
					}
				}
			}
			return nil
		})
		if err != nil {
			t.Fatal(err)
		}
	}
	e.seeded = time.Since(t0)
	e.hold, e.admin, e.free = e.devs[1], e.devs[0], e.devs[len(e.devs)-1]
	return e
}

type scaleRoute struct {
	who  string
	s    *sess
	path string
}

// scaleRoutes is every GET route of the route table, each with the caller its
// access level needs, plus the filtered variants the dashboard uses.
func (e *scaleEnv) scaleRoutes() []scaleRoute {
	var out []scaleRoute
	for _, r := range routes {
		if r.Method != "GET" {
			continue
		}
		path := r.Pattern
		switch path {
		case "/api/v1/me/ledger", "/api/v1/me/usage":
			path += "?period=2026-11"
		}
		who, s := "dev", &e.hold
		switch r.Access {
		case AccessAdmin:
			who, s = "admin", &e.admin
		case AccessPublic:
			who, s = "none", nil
		}
		out = append(out, scaleRoute{who, s, path})
		switch path {
		case "/api/v1/setups":
			out = append(out, scaleRoute{who, s, path + "?selector=tier%3Dt2"}, scaleRoute{"admin", &e.admin, path})
		case "/api/v1/admin/users":
			out = append(out, scaleRoute{who, s, path + "?status=pending"})
		}
	}
	return out
}

// timeRoute serves path n times and returns the sorted times, the status of
// the last answer and the statements one request ran.
func (e *scaleEnv) timeRoute(r scaleRoute, n int) (ds []time.Duration, code, size int, stmts int64) {
	for i := 0; i < n; i++ {
		before := e.b.st.StatementCount()
		t1 := time.Now()
		rec := e.b.serve("GET", r.path, r.s, false, "")
		ds = append(ds, time.Since(t1))
		code, size, stmts = rec.Code, rec.Body.Len(), e.b.st.StatementCount()-before
	}
	sort.Slice(ds, func(i, j int) bool { return ds[i] < ds[j] })
	return ds, code, size, stmts
}

func isAPIDataRoute(p string) bool {
	return len(p) > 8 && p[:8] == "/api/v1/" && p != "/api/v1/auth/methods"
}

func TestNFR_ApiUnder500msAtSpecScale(t *testing.T) {
	full := os.Getenv("HOURGLASS_SCALE_TEST") != ""
	if full {
		runFullScale(t)
		return
	}
	small, large := seedScale(t, reducedSmall), seedScale(t, reducedLarge)
	for i, r := range small.scaleRoutes() {
		lr := large.scaleRoutes()[i]
		_, c1, _, n1 := small.timeRoute(r, 1)
		ds, c2, _, n2 := large.timeRoute(lr, 3)
		if isAPIDataRoute(r.path) && (c1 != 200 || c2 != 200) {
			t.Errorf("%s %s: status %d / %d", r.who, r.path, c1, c2)
		}
		if n1 != n2 {
			t.Errorf("%s %s runs %d statements on %d users / %d setups and %d on %d / %d: a per-row query has crept in",
				r.who, r.path, n1, reducedSmall.users, reducedSmall.setups, n2, reducedLarge.users, reducedLarge.setups)
		}
		if ds[2] > 400*time.Millisecond { // generous: the data set is tiny
			t.Errorf("%s %s took %s on the reduced data set", r.who, r.path, ds[2])
		}
	}
}

func runFullScale(t *testing.T) {
	e := seedScale(t, fullScale)
	t.Logf("seeded %d users, %d setups, %d holds, %d polls (%d usage samples) in %s",
		fullScale.users, fullScale.setups, fullScale.holds, e.polls, e.polls*fullScale.holds, e.seeded.Round(time.Second))
	for _, r := range e.scaleRoutes() {
		ds, code, size, stmts := e.timeRoute(r, 7)
		flag := ""
		if ds[len(ds)-1] > 500*time.Millisecond {
			flag = "  OVER 500ms"
			t.Errorf("%s %s: worst %s is over 500 ms", r.who, r.path, ds[len(ds)-1])
		}
		if isAPIDataRoute(r.path) && code != 200 {
			t.Errorf("%s %s: status %d", r.who, r.path, code)
		}
		t.Logf("%-5s %-44s %d %8d B %3d stmts  best %6.1f ms  median %6.1f ms  worst %6.1f ms%s", r.who, r.path, code, size, stmts,
			ms(ds[0]), ms(ds[len(ds)/2]), ms(ds[len(ds)-1]), flag)
	}
	// One acquire as a write, then one collector poll over the full data.
	ctx := context.Background()
	sid, err := e.b.st.CreateSetup(ctx, store.Setup{Name: "lab-free", Enabled: true, Labels: map[string]string{}, DetailsJSON: `{}`})
	if err != nil {
		t.Fatal(err)
	}
	t1 := time.Now()
	rec := e.b.serve("POST", fmt.Sprintf("/api/v1/setups/%d/acquire", sid), &e.free, true, "")
	t.Logf("dev   POST acquire %d in %.1f ms", rec.Code, ms(time.Since(t1)))
	t1 = time.Now()
	e.b.clk.Advance(5 * time.Minute)
	err = e.b.col.RunOnce(ctx)
	t.Logf("collector poll over %d holds: err=%v in %.1f ms", fullScale.holds, err, ms(time.Since(t1)))
	if err != nil {
		t.Error(err)
	}
}

func ms(d time.Duration) float64 { return float64(d.Microseconds()) / 1000 }
