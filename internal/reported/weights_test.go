// SPDX-License-Identifier: MIT

package reported

import (
	"errors"
	"fmt"
	"math"
	"path/filepath"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"
)

var lim20 = WeightLimits{Min: 0, Max: 20}

func ws(pairs ...any) []Weight {
	var out []Weight
	for i := 0; i < len(pairs); i += 2 {
		out = append(out, Weight{Setup: pairs[i].(string), Weight: pairs[i+1].(float64)})
	}
	return out
}

func TestWGT7_WeightsNoReportYet(t *testing.T) {
	svc, _ := newSvc(t)
	if _, err := svc.Weights(ctx); !errors.Is(err, ErrNoReport) {
		t.Errorf("weights: %v", err)
	}
}

// observed_at defaults to the receipt time; the stored report carries the
// account, both times and its age; the whole set is replaced (WGT-7).
func TestWGT7_WeightsReplaceWholeSetAndDefaultObservedAt(t *testing.T) {
	svc, clk := newSvc(t)
	clk.Set(t0.Add(1500 * time.Microsecond)) // truncated to the millisecond
	got, err := svc.SubmitWeights(ctx, "svc", nil, []Weight{
		{Setup: "lab-07", Weight: 0.8},
		{Setup: "lab-03", Weight: 1.15, Attributes: map[string]string{"memory-gb": "512", "tier": "gold"}},
	}, lim20)
	if err != nil {
		t.Fatal(err)
	}
	want := t0.Add(time.Millisecond)
	if !got.ObservedAt.Equal(want) || !got.ReceivedAt.Equal(want) || got.ServiceAccount != "svc" || got.Age != 0 {
		t.Errorf("submit: %+v", got)
	}
	clk.Set(t0.Add(time.Minute))
	read, err := svc.Weights(ctx)
	if err != nil {
		t.Fatal(err)
	}
	wantSet := []Weight{
		{Setup: "lab-03", Weight: 1.15, Attributes: map[string]string{"memory-gb": "512", "tier": "gold"}},
		{Setup: "lab-07", Weight: 0.8, Attributes: map[string]string{}},
	}
	if !reflect.DeepEqual(read.Weights, wantSet) || !reflect.DeepEqual(got.Weights, wantSet) {
		t.Errorf("set: read %+v, submit %+v", read.Weights, got.Weights)
	}
	if read.Age != time.Minute-time.Millisecond {
		t.Errorf("age = %v", read.Age)
	}
	// The next report replaces everything: lab-03 and lab-07 are gone.
	if _, err := svc.SubmitWeights(ctx, "svc2", nil, ws("lab-09", 3.0), lim20); err != nil {
		t.Fatal(err)
	}
	read, _ = svc.Weights(ctx)
	if !reflect.DeepEqual(read.Weights, []Weight{{Setup: "lab-09", Weight: 3, Attributes: map[string]string{}}}) || read.ServiceAccount != "svc2" {
		t.Errorf("replaced: %+v", read)
	}
	// An empty list is valid: no setup has a weight.
	if _, err := svc.SubmitWeights(ctx, "svc", nil, nil, lim20); err != nil {
		t.Fatal(err)
	}
	if read, _ = svc.Weights(ctx); read.Weights == nil || len(read.Weights) != 0 {
		t.Errorf("empty: %#v", read.Weights)
	}
}

// Ordering and skew: older is stale and changes nothing, equal replaces, up
// to 2 minutes ahead is accepted and more is refused (WGT-7).
func TestWGT7_WeightsOrderingAndClockSkew(t *testing.T) {
	svc, _ := newSvc(t)
	if _, err := svc.SubmitWeights(ctx, "svc", at(0), ws("lab-01", 1.0), lim20); err != nil {
		t.Fatal(err)
	}
	_, err := svc.SubmitWeights(ctx, "svc", at(-time.Second), ws("lab-01", 2.0), lim20)
	var se *StaleError
	if !errors.As(err, &se) || !se.Stored.Equal(t0) {
		t.Fatalf("older: %v", err)
	}
	if got, _ := svc.Weights(ctx); got.Weights[0].Weight != 1 {
		t.Errorf("stale report changed the stored one: %+v", got)
	}
	if _, err := svc.SubmitWeights(ctx, "svc", at(0), ws("lab-02", 2.0), lim20); err != nil {
		t.Fatalf("equal observed_at refused: %v", err)
	}
	if got, _ := svc.Weights(ctx); len(got.Weights) != 1 || got.Weights[0].Setup != "lab-02" {
		t.Errorf("equal did not replace: %+v", got)
	}
	if _, err := svc.SubmitWeights(ctx, "svc", at(2*time.Minute), ws("lab-03", 3.0), lim20); err != nil {
		t.Errorf("exactly 2 minutes ahead refused: %v", err)
	}
	_, err = svc.SubmitWeights(ctx, "svc", at(2*time.Minute+time.Millisecond), ws("lab-04", 4.0), lim20)
	var ve *ValidationError
	if !errors.As(err, &ve) || !strings.Contains(ve.Message, "2 minutes in the future") {
		t.Errorf("too far ahead: %v", err)
	}
	if got, _ := svc.Weights(ctx); got.Weights[0].Setup != "lab-03" {
		t.Errorf("refused report stored: %+v", got)
	}
}

// The weights report's ordering is independent of the deployments report's:
// either may be ahead of the other, and they may share an observed_at.
func TestWGT7_WeightsOrderingIndependentOfDeployments(t *testing.T) {
	svc, _ := newSvc(t)
	if _, err := svc.SubmitDeployments(ctx, "svc", at(time.Minute), deps("lab-01")); err != nil {
		t.Fatal(err)
	}
	// Older than the stored deployments report, but the first weights report.
	if _, err := svc.SubmitWeights(ctx, "svc", at(0), ws("lab-01", 1.0), lim20); err != nil {
		t.Fatalf("weights compared with deployments: %v", err)
	}
	// Deployments older than the stored weights report are refused only by
	// their own ordering.
	if _, err := svc.SubmitWeights(ctx, "svc", at(90*time.Second), ws("lab-01", 1.0), lim20); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.SubmitDeployments(ctx, "svc", at(time.Minute), deps("lab-02")); err != nil {
		t.Fatalf("deployments compared with weights: %v", err)
	}
	// The same observed_at for both.
	if _, err := svc.SubmitWeights(ctx, "svc", at(100*time.Second), ws("lab-01", 2.0), lim20); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.SubmitDeployments(ctx, "svc", at(100*time.Second), deps("lab-03")); err != nil {
		t.Fatal(err)
	}
	w, _ := svc.Weights(ctx)
	d, _ := svc.Deployments(ctx)
	if !w.ObservedAt.Equal(t0.Add(100*time.Second)) || !d.ObservedAt.Equal(w.ObservedAt) || d.Deployments[0].Setup != "lab-03" {
		t.Errorf("weights %+v deployments %+v", w, d)
	}
}

// Every entry rule refuses the whole report with the entry's position and
// name, and nothing is stored: the previous report stays (WGT-8).
func TestWGT8_EachEntryRuleRefusesWholeReport(t *testing.T) {
	long := strings.Repeat("x", 64)
	many := map[string]string{}
	for i := 0; i < 17; i++ {
		many[fmt.Sprintf("k%02d", i)] = "v"
	}
	sixteen := map[string]string{}
	for i := 0; i < 16; i++ {
		sixteen[fmt.Sprintf("k%02d", i)] = strings.Repeat("y", 63)
	}
	good := Weight{Setup: "lab-01", Weight: 1}
	cases := []struct {
		name  string
		entry Weight
		want  string
	}{
		{"empty name", Weight{Setup: "", Weight: 1}, `weights[1]: setup must be a non-empty string`},
		{"repeated name", Weight{Setup: "lab-01", Weight: 2}, `weights[1] (setup "lab-01"): setup is repeated (first at weights[0])`},
		{"above max", Weight{Setup: "lab-03", Weight: 25}, `weights[1] (setup "lab-03"): weight 25 is above the maximum 20`},
		{"just above max", Weight{Setup: "lab-03", Weight: math.Nextafter(20, 21)}, `weights[1] (setup "lab-03"): weight 20.000000000000004 is above the maximum 20`},
		{"below min", Weight{Setup: "lab-03", Weight: -0.5}, `weights[1] (setup "lab-03"): weight -0.5 is below the minimum 0`},
		{"NaN", Weight{Setup: "lab-03", Weight: math.NaN()}, `weights[1] (setup "lab-03"): weight must be a finite number`},
		{"+Inf", Weight{Setup: "lab-03", Weight: math.Inf(1)}, `weights[1] (setup "lab-03"): weight must be a finite number`},
		{"-Inf", Weight{Setup: "lab-03", Weight: math.Inf(-1)}, `weights[1] (setup "lab-03"): weight must be a finite number`},
		{"17 attributes", Weight{Setup: "lab-03", Weight: 1, Attributes: many}, `weights[1] (setup "lab-03"): attributes has 17 entries (at most 16)`},
		{"bad attribute key", Weight{Setup: "lab-03", Weight: 1, Attributes: map[string]string{"Memory GB": "1"}}, `weights[1] (setup "lab-03"): attributes: invalid label key "Memory GB"`},
		{"long attribute value", Weight{Setup: "lab-03", Weight: 1, Attributes: map[string]string{"tier": long}}, `weights[1] (setup "lab-03"): attributes: label "tier": value is 64 characters (at most 63)`},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			svc, _ := newSvc(t)
			if _, err := svc.SubmitWeights(ctx, "svc", at(0), ws("lab-09", 9.0), lim20); err != nil {
				t.Fatal(err)
			}
			_, err := svc.SubmitWeights(ctx, "svc", at(time.Second), []Weight{good, c.entry, {Setup: "lab-02", Weight: 1}}, lim20)
			var ee *EntryError
			var ve *ValidationError
			if !errors.As(err, &ee) || !errors.As(err, &ve) || ee.Index != 1 {
				t.Fatalf("err = %#v", err)
			}
			if !strings.HasPrefix(err.Error(), c.want) || ve.Message != err.Error() {
				t.Errorf("message %q, want prefix %q (validation %q)", err.Error(), c.want, ve.Message)
			}
			got, _ := svc.Weights(ctx)
			if !got.ObservedAt.Equal(t0) || len(got.Weights) != 1 || got.Weights[0].Setup != "lab-09" {
				t.Errorf("previous report not kept: %+v", got)
			}
		})
	}
	// The limits are inclusive at both ends; 16 attributes of 63 characters
	// are allowed; a weight of 0 is valid when the minimum allows it.
	svc, _ := newSvc(t)
	ok := []Weight{
		{Setup: "lab-min", Weight: 2}, {Setup: "lab-max", Weight: 20},
		{Setup: "lab-attrs", Weight: 5, Attributes: sixteen},
	}
	if _, err := svc.SubmitWeights(ctx, "svc", nil, ok, WeightLimits{Min: 2, Max: 20}); err != nil {
		t.Errorf("limits not inclusive: %v", err)
	}
	if _, err := svc.SubmitWeights(ctx, "svc", nil, ws("lab-free", 0.0), lim20); err != nil {
		t.Errorf("zero refused: %v", err)
	}
}

// The first invalid entry is the one reported.
func TestWGT8_FirstInvalidEntryReported(t *testing.T) {
	svc, _ := newSvc(t)
	_, err := svc.SubmitWeights(ctx, "svc", nil, []Weight{
		{Setup: "lab-01", Weight: 1}, {Setup: "lab-02", Weight: 30}, {Setup: "", Weight: 1},
	}, lim20)
	if err == nil || err.Error() != `weights[1] (setup "lab-02"): weight 30 is above the maximum 20` {
		t.Errorf("err = %v", err)
	}
	if _, err := svc.Weights(ctx); !errors.Is(err, ErrNoReport) {
		t.Errorf("something was stored: %v", err)
	}
}

// The entry message is one function the HTTP handler reuses.
func TestWGT8_EntryMessageForm(t *testing.T) {
	if got := EntryMessage(3, "lab-03", "weight 25 is above the maximum 20"); got != `weights[3] (setup "lab-03"): weight 25 is above the maximum 20` {
		t.Errorf("with name: %q", got)
	}
	if got := EntryMessage(0, "", "weight must be a JSON number"); got != `weights[0]: weight must be a JSON number` {
		t.Errorf("without name: %q", got)
	}
}

func TestWGT8_TooManyEntriesRefused(t *testing.T) {
	svc, _ := newSvc(t)
	list := make([]Weight, MaxWeightEntries+1)
	for i := range list {
		list[i] = Weight{Setup: fmt.Sprintf("lab-%05d", i), Weight: 1}
	}
	var ve *ValidationError
	if _, err := svc.SubmitWeights(ctx, "svc", nil, list, lim20); !errors.As(err, &ve) {
		t.Fatalf("10001 entries: %v", err)
	}
	if _, err := svc.SubmitWeights(ctx, "svc", nil, list[:MaxWeightEntries], lim20); err != nil {
		t.Fatalf("10000 entries: %v", err)
	}
	if got, _ := svc.Weights(ctx); len(got.Weights) != MaxWeightEntries {
		t.Errorf("stored %d", len(got.Weights))
	}
}

// Negative zero is stored as 0.
func TestWGT8_NegativeZeroStoredAsZero(t *testing.T) {
	svc, _ := newSvc(t)
	if _, err := svc.SubmitWeights(ctx, "svc", nil, ws("lab-01", math.Copysign(0, -1)), lim20); err != nil {
		t.Fatal(err)
	}
	got, _ := svc.Weights(ctx)
	if w := got.Weights[0].Weight; w != 0 || math.Signbit(w) {
		t.Errorf("weight %v signbit %v", w, math.Signbit(w))
	}
}

// Names that match no setup are validated and stored like any other: the
// package knows nothing about setups (WGT-9 is the caller's).
func TestWGT8_UnknownNamesStored(t *testing.T) {
	svc, _ := newSvc(t)
	if _, err := svc.SubmitWeights(ctx, "svc", nil, ws("no-such-setup", 1.0, "lab-01", 2.0), lim20); err != nil {
		t.Fatal(err)
	}
	got, _ := svc.Weights(ctx)
	if len(got.Weights) != 2 || got.Weights[1].Setup != "no-such-setup" {
		t.Errorf("stored: %+v", got.Weights)
	}
}

// Attributes are stored and returned unchanged, and the caller's map is
// copied, not kept.
func TestWGT7_AttributesStoredUnchanged(t *testing.T) {
	svc, _ := newSvc(t)
	attrs := map[string]string{"memory-gb": "512", "storage-gb": "4000", "tier": "gold", "note": ""}
	if _, err := svc.SubmitWeights(ctx, "svc", nil, []Weight{{Setup: "lab-03", Weight: 1.15, Attributes: attrs}}, lim20); err != nil {
		t.Fatal(err)
	}
	attrs["tier"] = "changed"
	got, _ := svc.Weights(ctx)
	want := map[string]string{"memory-gb": "512", "storage-gb": "4000", "tier": "gold", "note": ""}
	if !reflect.DeepEqual(got.Weights[0].Attributes, want) {
		t.Errorf("attributes: %+v", got.Weights[0].Attributes)
	}
}

// The stored weights report survives reopening the database (WGT-7, B3).
func TestWGT7_WeightsSurviveReopen(t *testing.T) {
	path := filepath.Join(t.TempDir(), "h.db")
	clk := &fakeClock{t: t0}
	st := open(t, path)
	svc := New(st, clk)
	if _, err := svc.SubmitWeights(ctx, "svc", at(-time.Minute), []Weight{{Setup: "lab-03", Weight: 1.15, Attributes: map[string]string{"tier": "gold"}}}, lim20); err != nil {
		t.Fatal(err)
	}
	st.Close()
	svc2 := New(open(t, path), clk)
	got, err := svc2.Weights(ctx)
	if err != nil || !got.ObservedAt.Equal(t0.Add(-time.Minute)) || got.Age != time.Minute || got.ServiceAccount != "svc" ||
		!reflect.DeepEqual(got.Weights, []Weight{{Setup: "lab-03", Weight: 1.15, Attributes: map[string]string{"tier": "gold"}}}) {
		t.Fatalf("after reopen: %+v %v", got, err)
	}
	// Ordering survives too.
	var se *StaleError
	if _, err := svc2.SubmitWeights(ctx, "svc", at(-2*time.Minute), nil, lim20); !errors.As(err, &se) {
		t.Errorf("older after reopen: %v", err)
	}
}

// The last refusal is kept in memory, replaced by the next one, and safe for
// concurrent use (B2).
func TestWeightsLastRefusal(t *testing.T) {
	svc, _ := newSvc(t)
	if _, ok := svc.LastWeightsRefusal(); ok {
		t.Fatal("refusal before any")
	}
	r1 := Refusal{At: t0, ServiceAccount: "svc", Message: "weights[0]: setup must be a non-empty string"}
	svc.RecordWeightsRefusal(r1)
	if got, ok := svc.LastWeightsRefusal(); !ok || got != r1 {
		t.Errorf("got %+v %v", got, ok)
	}
	var wg sync.WaitGroup
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			svc.RecordWeightsRefusal(Refusal{At: t0.Add(time.Duration(i) * time.Second), ServiceAccount: "svc", Message: "m"})
			svc.LastWeightsRefusal()
		}(i)
	}
	wg.Wait()
	r2 := Refusal{At: t0.Add(time.Hour), ServiceAccount: "other", Message: "x"}
	svc.RecordWeightsRefusal(r2)
	if got, _ := svc.LastWeightsRefusal(); got != r2 {
		t.Errorf("not replaced: %+v", got)
	}
}
