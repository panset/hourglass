// SPDX-License-Identifier: MIT

package app

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/panset/hourglass/internal/settings"
	"github.com/panset/hourglass/internal/store"
)

func openStore(t *testing.T) *store.Store {
	t.Helper()
	st, err := store.Open(filepath.Join(t.TempDir(), "t.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { st.Close() })
	return st
}

func writeSeed(t *testing.T, name, body string) string {
	t.Helper()
	p := filepath.Join(t.TempDir(), name)
	if err := os.WriteFile(p, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
	return p
}

const goodSeedYAML = `
default_weight: 1.5
setups:
  - name: lab-01
    labels: {tier: large, rack: r1}
    details: {cpu: 8, tags: [a, b]}
  - name: lab-02
    labels: {tier: small}
    enabled: false
    resource_name_override: custom_rp
rate_rules:
  - {selector: "tier=large", weight: 2}
  - {selector: "tier in (small)", weight: 0.5}
`

func TestSET7_SeedAppliedOnEmptyDatabase(t *testing.T) {
	ctx := context.Background()
	st := openStore(t)
	ok, err := ImportSeed(ctx, st, writeSeed(t, "seed.yaml", goodSeedYAML), time.Now())
	if err != nil || !ok {
		t.Fatalf("ok=%v err=%v", ok, err)
	}
	setups, _ := st.ListSetups(ctx, false)
	if len(setups) != 2 {
		t.Fatalf("setups %+v", setups)
	}
	by := map[string]store.Setup{}
	for _, s := range setups {
		by[s.Name] = s
	}
	a, b := by["lab-01"], by["lab-02"]
	if !a.Enabled || a.Labels["tier"] != "large" || a.Labels["rack"] != "r1" || a.DetailsJSON != `{"cpu":8,"tags":["a","b"]}` {
		t.Errorf("lab-01 %+v", a)
	}
	if b.Enabled || b.ResourceNameOverride != "custom_rp" || b.DetailsJSON != "{}" {
		t.Errorf("lab-02 %+v", b)
	}
	rs, _ := st.ListRules(ctx, "rate")
	if len(rs) != 2 || rs[0].Selector != "tier=large" || rs[0].Value != 2 || rs[1].Value != 0.5 || rs[0].Position != 0 || rs[1].Position != 1 {
		t.Errorf("rules %+v", rs)
	}
	cfg, err := settings.Load(ctx, st)
	if err != nil || cfg.DefaultWeight != 1.5 {
		t.Errorf("default weight %v %v", cfg.DefaultWeight, err)
	}
}

func TestSET7_SeedJSONByExtension(t *testing.T) {
	ctx := context.Background()
	st := openStore(t)
	p := writeSeed(t, "seed.json", `{"setups":[{"name":"j1","labels":{"a":"b"},"details":{"x":1}}],"rate_rules":[{"selector":"a=b","weight":3}]}`)
	if ok, err := ImportSeed(ctx, st, p, time.Now()); err != nil || !ok {
		t.Fatalf("%v %v", ok, err)
	}
	if s, _ := st.ListSetups(ctx, false); len(s) != 1 || s[0].Name != "j1" {
		t.Fatalf("%+v", s)
	}
}

func TestSET7_NeverAppliedAgainOnceASetupExists(t *testing.T) {
	ctx := context.Background()
	st := openStore(t)
	p := writeSeed(t, "seed.yaml", goodSeedYAML)
	if ok, err := ImportSeed(ctx, st, p, time.Now()); err != nil || !ok {
		t.Fatal(ok, err)
	}
	// An admin deletes one setup and edits nothing else; a second start must not re-seed.
	setups, _ := st.ListSetups(ctx, false)
	if err := st.DeleteSetup(ctx, setups[0].ID); err != nil {
		t.Fatal(err)
	}
	if ok, err := ImportSeed(ctx, st, p, time.Now()); err != nil || ok {
		t.Fatalf("second import ok=%v err=%v", ok, err)
	}
	if s, _ := st.ListSetups(ctx, false); len(s) != 1 {
		t.Fatalf("setups after second import: %d", len(s))
	}
}

func TestSET7_SkippedWhenSetupsExistEvenIfFileIsInvalidOrMissing(t *testing.T) {
	ctx := context.Background()
	st := openStore(t)
	if _, err := st.CreateSetup(ctx, store.Setup{Name: "mine", Enabled: true}); err != nil {
		t.Fatal(err)
	}
	for _, p := range []string{filepath.Join(t.TempDir(), "missing.yaml"), writeSeed(t, "bad.yaml", "setups: [")} {
		if ok, err := ImportSeed(ctx, st, p, time.Now()); err != nil || ok {
			t.Fatalf("%s: ok=%v err=%v", p, ok, err)
		}
	}
}

func TestSET7_NoSeedPathIsNoop(t *testing.T) {
	if ok, err := ImportSeed(context.Background(), openStore(t), "", time.Now()); ok || err != nil {
		t.Fatal(ok, err)
	}
}

func TestSET7_InvalidSeedRejectedAtomically(t *testing.T) {
	cases := map[string]string{
		"bad selector":     "setups: [{name: a}]\nrate_rules: [{selector: 'a in (', weight: 1}]\n",
		"negative":         "setups: [{name: a}]\nrate_rules: [{selector: 'a=b', weight: -1}]\n",
		"missing weight":   "setups: [{name: a}]\nrate_rules: [{selector: 'a=b'}]\n",
		"bad label key":    "setups: [{name: a, labels: {'Bad Key': x}}]\n",
		"long value":       "setups: [{name: a, labels: {k: '" + strings.Repeat("x", 64) + "'}}]\n",
		"no name":          "setups: [{labels: {a: b}}]\n",
		"duplicate name":   "setups: [{name: a}, {name: a}]\n",
		"unknown key":      "setups: [{name: a, kind: big}]\n",
		"unknown top":      "setups: [{name: a}]\nextra: 1\n",
		"bad default":      "setups: [{name: a}]\ndefault_weight: -2\n",
		"rules not a list": "setups: [{name: a}]\nrate_rules: {selector: 'a=b', weight: 1}\n",
		"oversize details": `{"setups":[{"name":"a","details":"` + strings.Repeat("x", maxDetailsBytes) + `"}]}`,
	}
	for name, body := range cases {
		t.Run(name, func(t *testing.T) {
			ctx := context.Background()
			st := openStore(t)
			ok, err := ImportSeed(ctx, st, writeSeed(t, "s.yaml", body), time.Now())
			if err == nil || ok {
				t.Fatalf("ok=%v err=%v, want an error", ok, err)
			}
			var se *SeedError
			if !errors.As(err, &se) {
				t.Fatalf("error is %T, want *SeedError: %v", err, err)
			}
			if s, _ := st.ListSetups(ctx, false); len(s) != 0 {
				t.Fatalf("partial import: %+v", s)
			}
			if r, _ := st.ListRules(ctx, "rate"); len(r) != 0 {
				t.Fatalf("partial rules: %+v", r)
			}
		})
	}
}

func marker(t *testing.T, st *store.Store) string {
	t.Helper()
	s, err := st.GetSetting(context.Background(), SeedMarkerKey)
	if errors.Is(err, store.ErrNotFound) {
		return ""
	}
	if err != nil {
		t.Fatal(err)
	}
	return s.ValueJSON
}

// SET-7: "never applied again", even when every setup was deleted afterwards
// (task 23 finding 5), and the admin's rate rules are not overwritten.
func TestSET7_NeverAppliedAgainEvenAfterEverySetupIsDeleted(t *testing.T) {
	ctx := context.Background()
	st := openStore(t)
	p := writeSeed(t, "seed.yaml", goodSeedYAML)
	if ok, err := ImportSeed(ctx, st, p, time.Now()); err != nil || !ok {
		t.Fatal(ok, err)
	}
	if m := marker(t, st); !strings.Contains(m, SeedStateApplied) {
		t.Fatalf("marker = %q", m)
	}
	setups, _ := st.ListSetups(ctx, false)
	for _, s := range setups {
		if err := st.DeleteSetup(ctx, s.ID); err != nil {
			t.Fatal(err)
		}
	}
	if err := st.ReplaceRules(ctx, "rate", []store.Rule{{Selector: "mine=1", Value: 9}}, nil, time.Now()); err != nil {
		t.Fatal(err)
	}
	if ok, err := ImportSeed(ctx, st, p, time.Now()); err != nil || ok {
		t.Fatalf("second import ok=%v err=%v", ok, err)
	}
	if s, _ := st.ListSetups(ctx, false); len(s) != 0 {
		t.Fatalf("setups came back: %+v", s)
	}
	if r, _ := st.ListRules(ctx, "rate"); len(r) != 1 || r[0].Selector != "mine=1" {
		t.Fatalf("admin's rules were replaced: %+v", r)
	}
}

// A first start without a seed records that too: a seed configured later is not applied.
func TestSET7_StartWithoutSeedRecordsMarker(t *testing.T) {
	ctx := context.Background()
	st := openStore(t)
	if ok, err := ImportSeed(ctx, st, "", time.Now()); ok || err != nil {
		t.Fatal(ok, err)
	}
	if m := marker(t, st); !strings.Contains(m, SeedStateNoSeed) {
		t.Fatalf("marker = %q", m)
	}
	if ok, err := ImportSeed(ctx, st, writeSeed(t, "seed.yaml", goodSeedYAML), time.Now()); ok || err != nil {
		t.Fatalf("late seed ok=%v err=%v", ok, err)
	}
	if s, _ := st.ListSetups(ctx, false); len(s) != 0 {
		t.Fatalf("late seed applied: %+v", s)
	}
}

// An existing database without a marker (made before the marker existed) is
// initialised when it holds setups, rules, or a user beyond the local admin.
func TestSET7_ExistingDatabaseWithoutMarkerIsTreatedAsInitialised(t *testing.T) {
	ctx := context.Background()
	now := time.Now()
	for name, prep := range map[string]func(*store.Store){
		"setups": func(st *store.Store) {
			if _, err := st.CreateSetup(ctx, store.Setup{Name: "mine", Enabled: true}); err != nil {
				t.Fatal(err)
			}
		},
		"rules": func(st *store.Store) {
			if err := st.ReplaceRules(ctx, "rate", []store.Rule{{Selector: "a=b", Value: 1}}, nil, now); err != nil {
				t.Fatal(err)
			}
		},
		"oidc user": func(st *store.Store) {
			if _, err := st.CreateOIDCUser(ctx, "sub-1", "x@example.com", "x", now); err != nil {
				t.Fatal(err)
			}
		},
	} {
		t.Run(name, func(t *testing.T) {
			st := openStore(t)
			prep(st)
			p := writeSeed(t, "seed.yaml", goodSeedYAML)
			if ok, err := ImportSeed(ctx, st, p, now); ok || err != nil {
				t.Fatalf("ok=%v err=%v", ok, err)
			}
			if m := marker(t, st); !strings.Contains(m, SeedStateExisting) {
				t.Fatalf("marker = %q", m)
			}
			if s, _ := st.ListSetups(ctx, false); len(s) > 1 {
				t.Fatalf("seed applied over existing data: %+v", s)
			}
		})
	}
}

// The local admin alone does not make a database initialised, and an invalid seed leaves no marker.
func TestSET7_LocalAdminAloneIsFreshAndInvalidSeedLeavesNoMarker(t *testing.T) {
	ctx := context.Background()
	st := openStore(t)
	if _, err := st.CreateLocalAdmin(ctx, "admin", "hash", true, time.Now()); err != nil {
		t.Fatal(err)
	}
	if _, err := ImportSeed(ctx, st, writeSeed(t, "bad.yaml", "setups: ["), time.Now()); err == nil {
		t.Fatal("invalid seed accepted")
	}
	if m := marker(t, st); m != "" {
		t.Fatalf("marker after a failed seed: %q", m)
	}
	if ok, err := ImportSeed(ctx, st, writeSeed(t, "seed.yaml", goodSeedYAML), time.Now()); !ok || err != nil {
		t.Fatalf("ok=%v err=%v", ok, err)
	}
}
