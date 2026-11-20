// SPDX-License-Identifier: MIT

package settings

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/panset/hourglass/internal/metering"
	"github.com/panset/hourglass/internal/store"
)

type fake struct {
	rows []store.Setting
	puts map[string]string
	by   map[string]int64
}

func (f *fake) AllSettings(context.Context) ([]store.Setting, error) { return f.rows, nil }
func (f *fake) PutSetting(_ context.Context, k, v string, by *int64, _ time.Time) error {
	if f.puts == nil {
		f.puts, f.by = map[string]string{}, map[string]int64{}
	}
	f.puts[k], f.by[k] = v, *by
	return nil
}

var (
	_ Reader = (*store.Store)(nil)
	_ Writer = (*store.Store)(nil)
)

func TestADM2_Defaults(t *testing.T) {
	d := Defaults()
	if d.DefaultWeight != 1 || len(d.DisplayLabelKeys) != 0 ||
		d.DefaultAllowanceMT != 300000 || d.MaxHoldsPerUser != 2 || !d.ReleaseRequiresTeardown ||
		d.PollInterval != 5*time.Minute || d.Enforcement {
		t.Fatalf("unexpected defaults %+v", d)
	}
	if err := d.Validate(); err != nil {
		t.Fatal(err)
	}
	if d.Bands.Multiplier(0.8) != 2 {
		t.Fatal("default bands")
	}
}

func TestADM2_Validate(t *testing.T) {
	long := make([]string, 33)
	for i := range long {
		long[i] = "k" + strings.Repeat("a", i)
	}
	cases := []struct {
		name  string
		mod   func(*Settings)
		field string
	}{
		{"weight neg", func(s *Settings) { s.DefaultWeight = -1 }, KeyDefaultWeight},
		{"weight nan", func(s *Settings) { s.DefaultWeight = nan() }, KeyDefaultWeight},
		{"label key bad", func(s *Settings) { s.DisplayLabelKeys = []string{"Bad Key"} }, KeyDisplayLabelKeys},
		{"label dup", func(s *Settings) { s.DisplayLabelKeys = []string{"a", "a"} }, KeyDisplayLabelKeys},
		{"label many", func(s *Settings) { s.DisplayLabelKeys = long }, KeyDisplayLabelKeys},
		{"bands", func(s *Settings) {
			s.Bands = metering.Bands{Steps: []metering.Step{{From: 0.5, Multiplier: 1}, {From: 0.4, Multiplier: 2}}}
		}, KeyBands},
		{"allowance", func(s *Settings) { s.DefaultAllowanceMT = -1 }, KeyDefaultAllowance},
		{"holds", func(s *Settings) { s.MaxHoldsPerUser = -1 }, KeyMaxHoldsPerUser},
		{"poll", func(s *Settings) { s.PollInterval = 59 * time.Second }, KeyPollInterval},
	}
	for _, c := range cases {
		s := Defaults()
		c.mod(&s)
		var fe *FieldError
		if err := s.Validate(); !errors.As(err, &fe) || fe.Field != c.field {
			t.Errorf("%s: got %v, want field %s", c.name, err, c.field)
		}
	}
	s := Defaults()
	s.PollInterval = time.Minute
	s.MaxHoldsPerUser = 0
	s.DefaultWeight = 0
	if err := s.Validate(); err != nil {
		t.Fatalf("boundary values: %v", err)
	}
}

func nan() float64 { var z float64; return z / z }

func TestADM2_LoadMissingAndUnknownKeys(t *testing.T) {
	f := &fake{rows: []store.Setting{
		{Key: KeyMaxHoldsPerUser, ValueJSON: "5"},
		{Key: KeyPollInterval, ValueJSON: `"10m"`},
		{Key: KeyDefaultAllowance, ValueJSON: "12.5"},
		{Key: "something_else", ValueJSON: "{"},
	}}
	s, err := Load(context.Background(), f)
	if err != nil {
		t.Fatal(err)
	}
	want := Defaults()
	want.MaxHoldsPerUser, want.PollInterval, want.DefaultAllowanceMT = 5, 10*time.Minute, 12500
	a, _ := json.Marshal(s)
	b, _ := json.Marshal(want)
	if string(a) != string(b) {
		t.Fatalf("got %s want %s", a, b)
	}
	f.rows = []store.Setting{{Key: KeyEnforcement, ValueJSON: `"x"`}}
	if _, err := Load(context.Background(), f); err == nil {
		t.Fatal("expected error for bad stored value")
	}
}

func TestADM3_SaveWritesOnlyChanged(t *testing.T) {
	old := Defaults()
	next := old
	next.Enforcement = true
	next.PollInterval = 2 * time.Minute
	f := &fake{}
	ch, err := Save(context.Background(), f, old, next, 7)
	if err != nil {
		t.Fatal(err)
	}
	if len(f.puts) != 2 || f.puts[KeyEnforcement] != "true" || f.puts[KeyPollInterval] != `"2m"` || f.by[KeyEnforcement] != 7 {
		t.Fatalf("puts %v", f.puts)
	}
	if len(ch) != 2 || ch[0].Key != KeyPollInterval || ch[0].Before != `"5m"` || ch[0].After != `"2m"` ||
		ch[1].Key != KeyEnforcement || ch[1].Before != "false" || ch[1].After != "true" {
		t.Fatalf("changes %+v", ch)
	}
	f = &fake{}
	if ch, err := Save(context.Background(), f, next, next, 7); err != nil || len(ch) != 0 || len(f.puts) != 0 {
		t.Fatal("no-op save wrote")
	}
	bad := old
	bad.PollInterval = time.Second
	if _, err := Save(context.Background(), f, old, bad, 7); err == nil || len(f.puts) != 0 {
		t.Fatal("invalid settings must not be written")
	}
}

func TestADM2_JSONRoundTrip(t *testing.T) {
	s := Defaults()
	s.DisplayLabelKeys = []string{"team", "env"}
	s.DefaultAllowanceMT = 1500
	b, err := json.Marshal(s)
	if err != nil {
		t.Fatal(err)
	}
	for _, frag := range []string{`"poll_interval":"5m"`, `"default_allowance":1.5`, `"bands":{"base":0,"steps":[{"from":0.5,"multiplier":1}`, `"enforcement":false`} {
		if !strings.Contains(string(b), frag) {
			t.Errorf("missing %s in %s", frag, b)
		}
	}
	var back Settings
	if err := json.Unmarshal(b, &back); err != nil {
		t.Fatal(err)
	}
	b2, _ := json.Marshal(back)
	if string(b) != string(b2) {
		t.Fatalf("round trip\n%s\n%s", b, b2)
	}
	var d Settings
	if err := json.Unmarshal([]byte(`{"poll_interval":"5m0s","default_allowance":300}`), &d); err != nil {
		t.Fatal(err)
	}
	if d.PollInterval != 5*time.Minute || d.DefaultAllowanceMT != 300000 {
		t.Fatalf("%+v", d)
	}
}

func TestADM2_JSONRejectsUnknownFields(t *testing.T) {
	for _, in := range []string{
		`{"nope":1}`,
		`{"bands":{"base":0,"steps":[],"extra":1}}`,
		`{"bands":{"base":0,"steps":[{"from":0.5,"multiplier":1,"x":2}]}}`,
		`{"poll_interval":300}`,
	} {
		var s Settings
		if err := json.Unmarshal([]byte(in), &s); err == nil {
			t.Errorf("%s accepted", in)
		}
	}
}

func TestSET6_DisplayLabelKeysValid(t *testing.T) {
	s := Defaults()
	s.DisplayLabelKeys = []string{"team", "env/tier", "a.b-c_d"}
	if err := s.Validate(); err != nil {
		t.Fatal(err)
	}
}

// stampRec records the updated_at passed to PutSetting.
type stampRec struct{ at []time.Time }

func (r *stampRec) PutSetting(_ context.Context, _, _ string, _ *int64, at time.Time) error {
	r.at = append(r.at, at)
	return nil
}

// Clock rule (finding 7): SaveAt stamps with the supplied time, and Save
// delegates to it.
func TestSaveAtUsesSuppliedTime(t *testing.T) {
	old := Defaults()
	next := old
	next.Enforcement = !old.Enforcement
	at := time.Date(2030, 1, 2, 3, 4, 5, 0, time.UTC)
	r := &stampRec{}
	ch, err := SaveAt(context.Background(), r, old, next, 7, at)
	if err != nil || len(ch) != 1 || len(r.at) != 1 || !r.at[0].Equal(at) {
		t.Fatalf("changes %v err %v stamps %v", ch, err, r.at)
	}
	saved := now
	t.Cleanup(func() { now = saved })
	fixed := time.Date(2031, 5, 6, 7, 8, 9, 0, time.UTC)
	now = func() time.Time { return fixed }
	r = &stampRec{}
	if _, err := Save(context.Background(), r, old, next, 7); err != nil || len(r.at) != 1 || !r.at[0].Equal(fixed) {
		t.Fatalf("Save stamps %v err %v", r.at, err)
	}
}
