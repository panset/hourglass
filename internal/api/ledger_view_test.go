// SPDX-License-Identifier: MIT

package api

import (
	"encoding/json"
	"net/url"
	"os"
	"reflect"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/panset/hourglass/internal/store"
	"github.com/panset/hourglass/pkg/hourglass"
)

// Tests of GET /api/v1/me/ledger's roll-up view, paging and balances (design
// A4; LED-11 to LED-15).

// ledgerWorld is a user's ledger across two local days that are due for
// compaction and today. The configured zone is Europe/Berlin (UTC+1 in
// November), so the polls of 21:00Z..23:05Z on 2026-11-02 fall on two local
// days, Nov 2 and Nov 3, although they share a UTC date. Two holds meter at
// 2x; an admin grant lands in the middle of Nov 2; the second hold is released
// on Nov 3; then the clock moves to 2026-11-12, where the first hold keeps
// metering ("today", the entry that moves to the front).
type ledgerWorld struct {
	*uenv
	dev    usess
	h1, h2 int64
}

func newLedgerWorld(t *testing.T) *ledgerWorld {
	t.Helper()
	berlin, err := time.LoadLocation("Europe/Berlin")
	if err != nil {
		t.Fatal(err)
	}
	e := newUserEnv(t, time.Date(2026, 11, 2, 21, 0, 0, 0, time.UTC), berlin)
	e.rateRule("tier=a", 600)
	s1 := e.setup("lab-01", true, map[string]string{"tier": "a"})
	s2 := e.setup("lab-02", true, map[string]string{"tier": "a"})
	dev := e.dev("dev", store.RoleDeveloper)
	e.prov.SetAllocations(hourglass.Allocation{ResourceName: "lab-01"}, hourglass.Allocation{ResourceName: "lab-02"})
	w := &ledgerWorld{uenv: e, dev: dev}
	w.h1 = e.acquire(&dev, s1)
	w.h2 = e.acquire(&dev, s2)
	e.poll()
	for i := 0; i < 6; i++ { // to 21:30Z, 22:30 local on Nov 2
		e.step(5 * time.Minute)
	}
	if _, err := e.led.Grant(uBg, e.admin.ID, dev.user.ID, 50000, "mid-day"); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 19; i++ { // to 23:05Z, 00:05 local on Nov 3
		e.step(5 * time.Minute)
	}
	if rec := e.post("/api/v1/holds/"+uItoa(w.h2)+"/release", &dev, ""); rec.Code != 202 {
		t.Fatalf("release: %d %s", rec.Code, rec.Body)
	}
	e.prov.SetAllocations(hourglass.Allocation{ResourceName: "lab-01"})
	e.step(5 * time.Minute) // 23:10Z: completes the release of h2
	e.step(5 * time.Minute) // 23:15Z
	// Nine days later: Nov 2 and Nov 3 are due (earlier than Nov 12 - 7).
	e.clk.Advance(9*24*time.Hour + 9*time.Hour + 45*time.Minute) // 2026-11-12T09:00Z
	e.poll()
	for i := 0; i < 3; i++ {
		e.step(5 * time.Minute)
	}
	return w
}

// compact runs compaction steps until nothing is due and returns the number
// of rows replaced.
func (w *ledgerWorld) compact() int {
	w.t.Helper()
	total := 0
	for i := 0; i < 100; i++ {
		s, err := w.led.CompactDue(uBg)
		if err != nil {
			w.t.Fatal(err)
		}
		total += s.Replaced
		if s.Replaced == 0 && !s.More {
			return total
		}
	}
	w.t.Fatal("compaction did not finish")
	return total
}

// ledger fetches GET /api/v1/me/ledger with the query (without "?").
func (w *ledgerWorld) ledger(query string) ujobj {
	w.t.Helper()
	path := "/api/v1/me/ledger"
	if query != "" {
		path += "?" + query
	}
	return uWantOK(w.t, w.get(path, &w.dev), 200)
}

func ledgerEntries(t *testing.T, m ujobj) []ujobj {
	t.Helper()
	raw, ok := m["entries"].([]any)
	if !ok {
		t.Fatalf("entries is not an array: %v", m["entries"])
	}
	out := make([]ujobj, len(raw))
	for i, e := range raw {
		out[i] = e.(ujobj)
	}
	return out
}

func entryID(t *testing.T, e ujobj) int64 { return int64(uNum(t, e, "id")) }

// walk follows next_cursor from the first page of the view to the end and
// returns the entries in the order received. between runs before every
// request after the first.
func (w *ledgerWorld) walk(view string, limit int, between func()) []ujobj {
	w.t.Helper()
	var out []ujobj
	q := view
	if q != "" {
		q += "&"
	}
	q += "limit=" + strconv.Itoa(limit)
	cursor := ""
	for pages := 0; ; pages++ {
		if pages > 1000 {
			w.t.Fatal("walk does not end")
		}
		if pages > 0 && between != nil {
			between()
		}
		qq := q
		if cursor != "" {
			qq += "&cursor=" + url.QueryEscape(cursor)
		}
		page := w.ledger(qq)
		es := ledgerEntries(w.t, page)
		if len(es) > limit {
			w.t.Fatalf("%s: %d entries, limit %d", qq, len(es), limit)
		}
		out = append(out, es...)
		next, isStr := page["next_cursor"].(string)
		if page["next_cursor"] == nil {
			return out
		}
		if !isStr || next == "" {
			w.t.Fatalf("%s: next_cursor %v", qq, page["next_cursor"])
		}
		if len(es) == 0 {
			w.t.Fatalf("%s: an empty page with a next_cursor", qq)
		}
		cursor = next
	}
}

func readGolden(t *testing.T) map[string]ujobj {
	t.Helper()
	b, err := os.ReadFile("testdata/led11_pre_change_responses.json")
	if err != nil {
		t.Fatal(err)
	}
	var g map[string]ujobj
	if err := json.Unmarshal(b, &g); err != nil {
		t.Fatal(err)
	}
	return g
}

// ledgerWeightFields are the LedgerEntry fields of reported weights (C1).
var ledgerWeightFields = []string{"weight_min", "weight_max", "weight_sources"}

func isLedgerWeightField(k string) bool {
	for _, f := range ledgerWeightFields {
		if k == f {
			return true
		}
	}
	return false
}

// withoutNewFields checks the two fields A4 adds and returns the response
// without them: next_cursor must be null, and entry_count is checked by want.
func withoutNewFields(t *testing.T, m ujobj, want func(i int, e ujobj, count float64)) ujobj {
	t.Helper()
	nc, has := m["next_cursor"]
	if !has || nc != nil {
		t.Errorf("next_cursor = %v (present %v), want null", nc, has)
	}
	out := ujobj{}
	for k, v := range m {
		if k != "next_cursor" {
			out[k] = v
		}
	}
	es := ledgerEntries(t, m)
	stripped := make([]any, len(es))
	for i, e := range es {
		c, ok := e["entry_count"].(float64)
		if !ok {
			t.Errorf("entry %v: entry_count missing", e["id"])
		}
		want(i, e, c)
		// The weight fields of design C1 (reported weights) are added too.
		for _, k := range ledgerWeightFields {
			if _, ok := e[k]; !ok {
				t.Errorf("entry %v: %s missing", e["id"], k)
			}
		}
		se := ujobj{}
		for k, v := range e {
			if k != "entry_count" && !isLedgerWeightField(k) {
				se[k] = v
			}
		}
		stripped[i] = se
	}
	out["entries"] = stripped
	return out
}

func diffJSON(t *testing.T, what string, got, want any) {
	t.Helper()
	if !reflect.DeepEqual(got, want) {
		g, _ := json.MarshalIndent(got, "", " ")
		wb, _ := json.MarshalIndent(want, "", " ")
		t.Errorf("%s differs from the pre-change response\n got: %s\nwant: %s", what, g, wb)
	}
}

// LED-11: without rollup, limit and cursor, the response is the one the
// handler gave before the parameters existed (recorded from the unchanged
// handler on the same data, before and after compaction), plus entry_count
// and next_cursor: null.
func TestLED11_NoNewParameterAnswersAsBefore(t *testing.T) {
	w := newLedgerWorld(t)
	golden := readGolden(t)

	before := w.ledger("")
	debits := 0
	got := withoutNewFields(t, before, func(i int, e ujobj, c float64) {
		if c != 1 {
			t.Errorf("before compaction: entry %v has entry_count %v, want 1", e["id"], c)
		}
		if e["type"] == "debit" {
			debits++
		}
	})
	diffJSON(t, "before compaction", got, golden["before_compaction"])
	// period given explicitly is the same response.
	diffJSON(t, "period=2026-11", withoutNewFields(t, w.ledger("period=2026-11"), func(int, ujobj, float64) {}), golden["before_compaction"])

	rows, err := w.st.ListLedger(uBg, w.dev.user.ID, "2026-11")
	if err != nil {
		t.Fatal(err)
	}
	if len(ledgerEntries(t, before)) != len(rows) {
		t.Errorf("%d entries for %d stored rows", len(ledgerEntries(t, before)), len(rows))
	}

	if w.compact() == 0 {
		t.Fatal("nothing was compacted")
	}
	rows, err = w.st.ListLedger(uBg, w.dev.user.ID, "2026-11")
	if err != nil {
		t.Fatal(err)
	}
	stored := map[int64]store.LedgerEntry{}
	for _, r := range rows {
		stored[r.ID] = r
	}
	after := w.ledger("")
	polls := 0
	got = withoutNewFields(t, after, func(i int, e ujobj, c float64) {
		r := stored[entryID(t, e)]
		want := 1
		if r.RollupCount > 0 {
			want = r.RollupCount
		}
		if int(c) != want {
			t.Errorf("after compaction: entry %v has entry_count %v, want %d", e["id"], c, want)
		}
		if e["type"] == "debit" {
			polls += int(c)
		}
	})
	diffJSON(t, "after compaction", got, golden["after_compaction"])
	if len(ledgerEntries(t, after)) != len(rows) {
		t.Errorf("%d entries for %d stored rows", len(ledgerEntries(t, after)), len(rows))
	}
	if polls != debits {
		t.Errorf("the debit entries stand for %d polls after compaction, %d debit rows before", polls, debits)
	}
}

// LED-15 at the API level: in the default view a compacted row shows no
// multiplier, though a usage sample exists at its created_at.
func TestLED15_CompactedRowHasNoMultiplierInDefaultView(t *testing.T) {
	w := newLedgerWorld(t)
	w.compact()
	seen := 0
	for _, e := range ledgerEntries(t, w.ledger("")) {
		if uNum(t, e, "entry_count") > 1 {
			seen++
			if e["multiplier"] != nil {
				t.Errorf("rolled-up entry %v reports multiplier %v", e["id"], e["multiplier"])
			}
		} else if e["type"] == "debit" && e["multiplier"] == nil {
			t.Errorf("raw debit %v lost its multiplier", e["id"])
		}
	}
	if seen == 0 {
		t.Fatal("no rolled-up entry in the default view")
	}
}

// dayGroups computes, from the default view of uncompacted data, what the
// rollup=day view must hold: per hold and local day the newest id, the
// summed amount and the poll count; other rows as they are.
type dayWant struct {
	id     int64
	amount float64
	count  int
	first  ujobj // the newest row
}

func dayView(t *testing.T, raw []ujobj, loc *time.Location) []dayWant {
	t.Helper()
	type key struct {
		hold float64
		day  string
	}
	idx := map[key]int{}
	var out []dayWant
	for i := len(raw) - 1; i >= 0; i-- { // oldest first
		e := raw[i]
		if e["type"] == "debit" && e["hold_id"] != nil {
			at, err := time.Parse(time.RFC3339, e["created_at"].(string))
			if err != nil {
				t.Fatal(err)
			}
			k := key{e["hold_id"].(float64), at.In(loc).Format("2006-01-02")}
			if j, ok := idx[k]; ok {
				out[j].id, out[j].amount, out[j].count, out[j].first = entryID(t, e), out[j].amount+uNum(t, e, "amount_tokens"), out[j].count+1, e
				continue
			}
			idx[k] = len(out)
		}
		out = append(out, dayWant{id: entryID(t, e), amount: uNum(t, e, "amount_tokens"), count: 1, first: e})
	}
	// newest first by id
	for i := 1; i < len(out); i++ {
		for j := i; j > 0 && out[j].id > out[j-1].id; j-- {
			out[j], out[j-1] = out[j-1], out[j]
		}
	}
	return out
}

// LED-12: with rollup=day the debit rows of one hold and local day are one
// entry, identical (ids included) before and after storage compaction, with
// entry_count and a null multiplier when it stands for more than one poll.
func TestLED12_DayViewIsTheSameBeforeAndAfterCompaction(t *testing.T) {
	w := newLedgerWorld(t)
	raw := ledgerEntries(t, w.ledger(""))
	want := dayView(t, raw, w.deps.Location)
	before := w.ledger("rollup=day")
	es := ledgerEntries(t, before)
	if len(es) != len(want) {
		t.Fatalf("rollup=day: %d entries, want %d", len(es), len(want))
	}
	merged := 0
	for i, e := range es {
		wt := want[i]
		if entryID(t, e) != wt.id || !uNear(uNum(t, e, "amount_tokens"), wt.amount) || int(uNum(t, e, "entry_count")) != wt.count ||
			e["created_at"] != wt.first["created_at"] || e["type"] != wt.first["type"] || e["hold_id"] != wt.first["hold_id"] ||
			e["setup_name"] != wt.first["setup_name"] {
			t.Errorf("entry %d: got %v, want id %d amount %v count %d like %v", i, e, wt.id, wt.amount, wt.count, wt.first)
		}
		if wt.count > 1 {
			merged++
			if e["multiplier"] != nil || e["actor_name"] != nil || e["reason"] != nil {
				t.Errorf("merged entry %v: multiplier %v actor %v reason %v, want null", e["id"], e["multiplier"], e["actor_name"], e["reason"])
			}
		} else if !reflect.DeepEqual(e["multiplier"], wt.first["multiplier"]) || e["reason"] != wt.first["reason"] || e["actor_name"] != wt.first["actor_name"] {
			t.Errorf("single entry %v differs from its row %v", e, wt.first)
		}
	}
	// Two local days (Nov 2 and Nov 3, Berlin) for each hold, and today's for h1.
	if merged != 5 {
		t.Errorf("%d merged entries, want 5", merged)
	}
	if w.compact() == 0 {
		t.Fatal("nothing was compacted")
	}
	after := w.ledger("rollup=day")
	if !reflect.DeepEqual(before, after) {
		g, _ := json.Marshal(after)
		b, _ := json.Marshal(before)
		t.Errorf("rollup=day changed with compaction\nbefore: %s\n after: %s", b, g)
	}

	// A single raw debit in the day view keeps its multiplier.
	e := defaultEnv(t)
	e.rateRule("tier=a", 600)
	sid := e.setup("lab-01", true, map[string]string{"tier": "a"})
	dev := e.dev("dev", store.RoleDeveloper)
	e.prov.SetAllocations(hourglass.Allocation{ResourceName: "lab-01"})
	e.acquire(&dev, sid)
	e.poll()
	e.step(5 * time.Minute)
	one := ledgerEntries(t, uWantOK(t, e.get("/api/v1/me/ledger?rollup=day", &dev), 200))
	if len(one) != 2 || one[0]["type"] != "debit" || uNum(t, one[0], "entry_count") != 1 || one[0]["multiplier"] != 2.0 {
		t.Errorf("single debit in the day view: %v", one)
	}
}

// LED-13: limit bounds the page; following cursors returns no entry twice
// and every unchanged entry exactly once while polls write between requests;
// bad values are 400 validation.
func TestLED13_PagingWalksEveryEntryOnce(t *testing.T) {
	w := newLedgerWorld(t)
	for _, compacted := range []bool{false, true} {
		if compacted {
			w.compact()
		}
		for _, view := range []string{"", "rollup=day"} {
			full := ledgerEntries(t, w.ledger(view))
			// Without new polls the pages are the unpaged response, cut.
			for _, limit := range []int{1, 2, 3, len(full), len(full) + 1, 200} {
				got := w.walk(view, limit, nil)
				if !reflect.DeepEqual(got, full) {
					t.Errorf("compacted=%v view=%q limit=%d: the pages are not the unpaged entries", compacted, view, limit)
				}
			}
			if p := w.ledger(strings.TrimPrefix(view+"&limit="+strconv.Itoa(len(full)), "&")); p["next_cursor"] != nil {
				t.Errorf("view=%q: limit = entry count still has a next_cursor", view)
			}
			if p := w.ledger(strings.TrimPrefix(view+"&limit=1", "&")); len(ledgerEntries(t, p)) != 1 || p["next_cursor"] == nil {
				t.Errorf("view=%q limit=1: %v", view, p)
			}

			// Polls between requests: today's entry of h1 gains polls (and, in
			// the day view, moves to the front). With limit 1 the first cursor
			// is that entry's own position.
			for _, limit := range []int{1, 2} {
				before := map[int64]ujobj{}
				for _, e := range ledgerEntries(t, w.ledger(view)) {
					before[entryID(t, e)] = e
				}
				got := w.walk(view, limit, func() { w.step(5 * time.Minute) })
				final := map[int64]ujobj{}
				for _, e := range ledgerEntries(t, w.ledger(view)) {
					final[entryID(t, e)] = e
				}
				seen := map[int64]int{}
				type dayKey struct {
					hold any
					day  string
				}
				seenDay := map[dayKey]int{}
				for _, e := range got {
					id := entryID(t, e)
					seen[id]++
					if view != "" && e["type"] == "debit" {
						at, err := time.Parse(time.RFC3339, e["created_at"].(string))
						if err != nil {
							t.Fatal(err)
						}
						seenDay[dayKey{e["hold_id"], at.In(w.deps.Location).Format("2006-01-02")}]++
					}
				}
				for id, n := range seen {
					if n > 1 {
						t.Errorf("compacted=%v view=%q limit=%d: entry %d returned %d times", compacted, view, limit, id, n)
					}
				}
				for k, n := range seenDay {
					if n > 1 { // the day view has one entry per hold and local day
						t.Errorf("compacted=%v view=%q limit=%d: the entry of hold %v on %s returned %d times", compacted, view, limit, k.hold, k.day, n)
					}
				}
				unchanged := 0
				for id, e := range before {
					if f, ok := final[id]; ok && reflect.DeepEqual(e, f) {
						unchanged++
						if seen[id] != 1 {
							t.Errorf("compacted=%v view=%q limit=%d: unchanged entry %d returned %d times", compacted, view, limit, id, seen[id])
						}
					}
				}
				if unchanged < len(before)-1 {
					t.Errorf("compacted=%v view=%q: only %d of %d entries unchanged", compacted, view, unchanged, len(before))
				}
			}
		}
	}

	bad := []struct{ query, field string }{
		{"rollup=week", "rollup"}, {"rollup=", "rollup"}, {"rollup=DAY", "rollup"},
		{"limit=0", "limit"}, {"limit=201", "limit"}, {"limit=abc", "limit"}, {"limit=-1", "limit"},
		{"limit=1.5", "limit"}, {"limit=", "limit"},
		{"limit=%2B5", "limit"}, {"limit=1.0", "limit"}, {"limit=%205", "limit"}, {"limit=5%20", "limit"},
		{"limit=00", "limit"}, {"limit=99999999999999999999", "limit"},
		{"cursor=5", "cursor"}, {"rollup=day&cursor=5", "cursor"},
		{"limit=5&cursor=abc", "cursor"}, {"limit=5&cursor=", "cursor"}, {"limit=5&cursor=0", "cursor"},
		{"limit=5&cursor=-3", "cursor"}, {"limit=5&cursor=007", "cursor"},
		{"limit=5&cursor=99999999999999999999", "cursor"},
	}
	for _, b := range bad {
		rec := w.get("/api/v1/me/ledger?"+b.query, &w.dev)
		uWantErr(t, rec, 400, "validation")
		if _, f := uErrOf(t, rec); f != b.field {
			t.Errorf("%s: field %q, want %q", b.query, f, b.field)
		}
	}
}

// LED-14: in both views, before and after compaction, across page
// boundaries, each entry's balance is the next older entry's balance plus its
// own amount (the oldest entry's, its amount), and the newest entry's balance
// is the period's balance. The world has a grant in the middle of a day.
func TestLED14_BalancesHoldAcrossPages(t *testing.T) {
	w := newLedgerWorld(t)
	for _, compacted := range []bool{false, true} {
		if compacted {
			w.compact()
		}
		balance := uNum(t, uWantOK(t, w.get("/api/v1/me", &w.dev), 200), "balance_tokens")
		for _, view := range []string{"", "rollup=day"} {
			for _, limit := range []int{1, 2, 5} {
				es := w.walk(view, limit, nil)
				if len(es) == 0 {
					t.Fatal("no entries")
				}
				if !uNear(uNum(t, es[0], "balance_after_tokens"), balance) {
					t.Errorf("compacted=%v view=%q limit=%d: newest balance %v, period balance %v",
						compacted, view, limit, es[0]["balance_after_tokens"], balance)
				}
				for i, e := range es {
					var older float64
					if i+1 < len(es) {
						older = uNum(t, es[i+1], "balance_after_tokens")
					}
					if !uNear(uNum(t, e, "balance_after_tokens"), older+uNum(t, e, "amount_tokens")) {
						t.Errorf("compacted=%v view=%q limit=%d: entry %v (page %d) balance %v, older %v + amount %v",
							compacted, view, limit, e["id"], i/limit, e["balance_after_tokens"], older, e["amount_tokens"])
					}
				}
			}
		}
	}
}

// LED-13 (A4): the page is smaller than the number of holds that are metering,
// and polls write between the requests. Every hold's entry of today moves to
// the front of the day view each time, so a cursor is only safe if it is the id
// of the page's own oldest entry. No entry may come twice, and an entry that
// did not change while the walk ran must come exactly once.
func TestLED13_PagingWithMoreActiveHoldsThanLimit(t *testing.T) {
	e := newUserEnv(t, time.Date(2026, 11, 2, 20, 0, 0, 0, time.UTC), time.UTC)
	e.rateRule("tier=a", 600)
	dev := e.dev("dev", store.RoleDeveloper)
	e.setting("max_holds_per_user", "5")
	var holds []int64
	var allocs []hourglass.Allocation
	for _, n := range []string{"lab-01", "lab-02", "lab-03"} {
		sid := e.setup(n, true, map[string]string{"tier": "a"})
		allocs = append(allocs, hourglass.Allocation{ResourceName: n})
		e.prov.SetAllocations(allocs...)
		holds = append(holds, e.acquire(&dev, sid))
	}
	e.poll()
	for i := 0; i < 60; i++ { // 5 h: across midnight, so each hold has two days
		e.step(5 * time.Minute)
	}
	fetch := func(view, qs string) ujobj {
		path := "/api/v1/me/ledger?" + view + qs
		return uWantOK(t, e.get(path, &dev), 200)
	}
	for _, view := range []string{"", "rollup=day&"} {
		for _, limit := range []int{1, 2} {
			if limit >= len(holds) {
				t.Fatal("the limit must be below the number of active holds")
			}
			before := map[int64]ujobj{}
			for _, en := range ledgerEntries(t, fetch(view, "")) {
				before[entryID(t, en)] = en
			}
			var got []ujobj
			cursor := ""
			for pages := 0; ; pages++ {
				if pages > 1000 {
					t.Fatal("walk does not end")
				}
				if pages > 0 {
					e.step(5 * time.Minute)
				}
				qs := "limit=" + strconv.Itoa(limit)
				if cursor != "" {
					qs += "&cursor=" + url.QueryEscape(cursor)
				}
				page := fetch(view, qs)
				es := ledgerEntries(t, page)
				if len(es) > limit {
					t.Fatalf("view=%q limit=%d: %d entries", view, limit, len(es))
				}
				got = append(got, es...)
				if page["next_cursor"] == nil {
					break
				}
				next, ok := page["next_cursor"].(string)
				if !ok || next == "" || len(es) == 0 {
					t.Fatalf("view=%q limit=%d: next_cursor %v on a page of %d", view, limit, page["next_cursor"], len(es))
				}
				cursor = next
			}
			final := map[int64]ujobj{}
			for _, en := range ledgerEntries(t, fetch(view, "")) {
				final[entryID(t, en)] = en
			}
			seen := map[int64]int{}
			for _, en := range got {
				seen[entryID(t, en)]++
			}
			for id, n := range seen {
				if n > 1 {
					t.Errorf("view=%q limit=%d: entry %d returned %d times", view, limit, id, n)
				}
			}
			unchanged := 0
			for id, en := range before {
				if f, ok := final[id]; ok && reflect.DeepEqual(en, f) {
					unchanged++
					if seen[id] != 1 {
						t.Errorf("view=%q limit=%d: unchanged entry %d returned %d times", view, limit, id, seen[id])
					}
				}
			}
			// Every active hold's newest entry may have changed; all others must be unchanged.
			if unchanged < len(before)-len(holds) {
				t.Errorf("view=%q limit=%d: only %d of %d entries unchanged", view, limit, unchanged, len(before))
			}
		}
	}
}

// LED-14: the newest entry's balance is the period's balance also for a past
// period, before and after compaction. The expected value is the sum of the
// stored rows of that period, read from the store and not from the handler.
func TestLED14_NewestBalanceInAPastPeriod(t *testing.T) {
	w := newLedgerWorld(t)
	w.clk.Advance(21 * 24 * time.Hour) // 2026-12-03: November is a past period
	w.poll()
	for _, compacted := range []bool{false, true} {
		if compacted {
			w.compact()
		}
		var want int64
		for _, typ := range []string{"allowance", "grant", "debit", "expiry", "carryover", "reset"} {
			n, err := w.st.SumLedger(uBg, w.dev.user.ID, "2026-11", typ)
			if err != nil {
				t.Fatal(err)
			}
			want += n
		}
		for _, view := range []string{"period=2026-11", "period=2026-11&rollup=day"} {
			for _, limit := range []int{1, 3, 200} {
				es := w.walk(view, limit, nil)
				if len(es) == 0 {
					t.Fatalf("compacted=%v view=%q: no entries", compacted, view)
				}
				if !uNear(uNum(t, es[0], "balance_after_tokens"), tokens(want)) {
					t.Errorf("compacted=%v view=%q limit=%d: newest balance %v, period balance %v",
						compacted, view, limit, es[0]["balance_after_tokens"], tokens(want))
				}
			}
		}
	}
}
