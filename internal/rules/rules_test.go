// SPDX-License-Identifier: MIT

package rules

import (
	"math"
	"reflect"
	"strings"
	"testing"
)

type L = map[string]string

func TestRULE2_SelectorMatch(t *testing.T) {
	tests := []struct {
		name, sel string
		labels    L
		want      bool
	}{
		{"empty matches all", "", L{"tier": "a"}, true},
		{"empty matches nil labels", "", nil, true},
		{"whitespace only matches all", "  \t ", L{}, true},
		{"eq true", "tier=a", L{"tier": "a"}, true},
		{"eq false value", "tier=a", L{"tier": "b"}, false},
		{"eq absent", "tier=a", L{}, false},
		{"double eq true", "tier==a", L{"tier": "a"}, true},
		{"double eq false", "tier==a", L{"tier": "b"}, false},
		{"eq empty value matches empty", "tier=", L{"tier": ""}, true},
		{"eq empty value vs non-empty", "tier=", L{"tier": "a"}, false},
		{"neq differs", "tier!=a", L{"tier": "b"}, true},
		{"neq same", "tier!=a", L{"tier": "a"}, false},
		{"neq absent is true", "tier!=a", L{}, true},
		{"in hit", "tier in (a,b)", L{"tier": "b"}, true},
		{"in miss", "tier in (a,b)", L{"tier": "c"}, false},
		{"in absent", "tier in (a,b)", L{}, false},
		{"in single", "tier in (a)", L{"tier": "a"}, true},
		{"in no space before paren", "tier in(a,b)", L{"tier": "a"}, true},
		{"notin hit", "tier notin (a,b)", L{"tier": "a"}, false},
		{"notin miss", "tier notin (a,b)", L{"tier": "c"}, true},
		{"notin absent is true", "tier notin (a,b)", L{}, true},
		{"exists true", "tier", L{"tier": ""}, true},
		{"exists false", "tier", L{"site": "x"}, false},
		{"not exists true", "!tier", L{"site": "x"}, true},
		{"not exists false", "!tier", L{"tier": "a"}, false},
		{"not exists spaced", "! tier", L{}, true},
		{"and all true", "tier=a,site=x", L{"tier": "a", "site": "x"}, true},
		{"and one false", "tier=a,site=x", L{"tier": "a", "site": "y"}, false},
		{"mixed ops", "tier in (a,b), !retired, site!=y", L{"tier": "a", "site": "x"}, true},
		{"mixed ops fail", "tier in (a,b), !retired, site!=y", L{"tier": "a", "retired": "1"}, false},
		{"whitespace everywhere", "  tier  =  a ,  site   notin ( x , y )  ", L{"tier": "a", "site": "z"}, true},
		{"key named in", "in", L{"in": "1"}, true},
		{"key named in with in op", "in in (a)", L{"in": "a"}, true},
		{"key named notin exists", "notin", L{"notin": "1"}, true},
		{"slash dot key", "example.org/tier=a", L{"example.org/tier": "a"}, true},
		{"value with odd chars", "tier=a:b+c", L{"tier": "a:b+c"}, true},
		{"value with uppercase", "tier=A", L{"tier": "A"}, true},
		{"value of 63 chars", "tier=" + strings.Repeat("a", 63), L{"tier": strings.Repeat("a", 63)}, true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := Match(tc.sel, tc.labels)
			if err != nil {
				t.Fatalf("Match(%q): %v", tc.sel, err)
			}
			if got != tc.want {
				t.Fatalf("Match(%q, %v) = %v, want %v", tc.sel, tc.labels, got, tc.want)
			}
			s, _ := Parse(tc.sel)
			if s.Matches(tc.labels) != tc.want {
				t.Fatalf("Selector.Matches disagrees with Match")
			}
		})
	}
}

func TestRULE2_ZeroSelectorMatchesAll(t *testing.T) {
	var s Selector
	if !s.Matches(L{"a": "b"}) || !s.Matches(nil) {
		t.Fatal("zero Selector must match everything")
	}
}

func TestRULE2_ParseErrors(t *testing.T) {
	tests := []struct {
		name, sel, wantSub string
	}{
		{"leading comma", ",a=b", "position 1"},
		{"trailing comma", "a=b,", "found end of selector"},
		{"double comma", "a=b,,c=d", "position 5"},
		{"uppercase key", "Tier=a", "invalid label key"},
		{"key starts with dash", "-a=b", "invalid label key"},
		{"key ends with dot", "a.=b", "invalid label key"},
		{"key too long", strings.Repeat("a", 64) + "=x", "invalid label key"},
		{"bang key invalid", "!Tier", "invalid label key"},
		{"bang without key", "!", "expected a label key"},
		{"bare neq", "!=a", "needs a key"},
		{"value too long", "a=" + strings.Repeat("v", 64), "at most 63"},
		{"in value too long", "a in (" + strings.Repeat("v", 64) + ")", "at most 63"},
		{"unknown operator", "a like b", "unknown operator"},
		{"in without paren", "a in b", "expected '('"},
		{"in unclosed", "a in (x,y", "missing ')'"},
		{"in empty list", "a in ()", "expected a value"},
		{"in empty element", "a in (x,,y)", "expected a value"},
		{"in trailing comma", "a in (x,)", "expected a value"},
		{"in junk in list", "a in (x y)", "unexpected"},
		{"in missing operand", "a in", "expected '('"},
		{"single bang after key", "a ! b", "expected '!='"},
		{"paren after key", "a (b)", "unexpected \"(\""},
		{"stray close paren", "a=b)", "unexpected"},
		{"triple eq", "a===b", "unexpected \"=\""},
		{"two requirements no comma", "a=b c=d", "expected ',' or end"},
		{"missing key before eq", "=b", "expected a label key"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			_, err := Parse(tc.sel)
			if err == nil {
				t.Fatalf("Parse(%q) succeeded, want error", tc.sel)
			}
			if !strings.Contains(err.Error(), tc.wantSub) {
				t.Fatalf("Parse(%q) error %q, want substring %q", tc.sel, err, tc.wantSub)
			}
			if !strings.Contains(err.Error(), "position") {
				t.Fatalf("error %q does not say where", err)
			}
			if m, merr := Match(tc.sel, L{}); merr == nil || m {
				t.Fatalf("Match must return the parse error")
			}
		})
	}
}

func TestSET3_ValidateLabels(t *testing.T) {
	many := L{}
	for i := 0; i < 32; i++ {
		many["k"+strings.Repeat("a", i)] = "v"
	}
	tooMany := L{}
	for k, v := range many {
		tooMany[k] = v
	}
	tooMany["extra"] = "v"
	tests := []struct {
		name    string
		labels  L
		wantErr bool
	}{
		{"nil", nil, false},
		{"simple", L{"tier": "a", "site": "x"}, false},
		{"32 labels", many, false},
		{"33 labels", tooMany, true},
		{"single char key", L{"a": "b"}, false},
		{"key with punctuation", L{"a.b_c/d-e": "v"}, false},
		{"key 63 chars", L{strings.Repeat("a", 63): "v"}, false},
		{"key 64 chars", L{strings.Repeat("a", 64): "v"}, true},
		{"empty key", L{"": "v"}, true},
		{"uppercase key", L{"Tier": "v"}, true},
		{"key leading dot", L{".a": "v"}, true},
		{"key trailing slash", L{"a/": "v"}, true},
		{"key with space", L{"a b": "v"}, true},
		{"key with unicode", L{"é": "v"}, true},
		{"empty value", L{"a": ""}, false},
		{"value 63 chars", L{"a": strings.Repeat("v", 63)}, false},
		{"value 64 chars", L{"a": strings.Repeat("v", 64)}, true},
		{"value any characters", L{"a": "Hello, (world)! = é"}, false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := ValidateLabels(tc.labels)
			if (err != nil) != tc.wantErr {
				t.Fatalf("ValidateLabels err = %v, wantErr %v", err, tc.wantErr)
			}
		})
	}
}

func TestRULE1_Evaluate(t *testing.T) {
	rs := []Rule{
		{ID: 1, Kind: KindRate, Position: 30, Selector: "", Value: "1"},
		{ID: 2, Kind: KindRate, Position: 10, Selector: "tier=a", Value: "2"},
		{ID: 3, Kind: "other", Position: 1, Selector: "", Value: "x"},
		{ID: 4, Kind: KindRate, Position: 20, Selector: "tier in (a,b)", Value: "3"},
		{ID: 5, Kind: KindRate, Position: 20, Selector: "tier=b", Value: "4"},
	}
	tests := []struct {
		name   string
		kind   string
		labels L
		wantID int64 // 0 = nil
	}{
		{"lowest position wins", KindRate, L{"tier": "a"}, 2},
		{"falls to next position", KindRate, L{"tier": "b"}, 4},
		{"ties keep input order", KindRate, L{"tier": "b"}, 4},
		{"catch-all last", KindRate, L{"tier": "z"}, 1},
		{"other kind only sees own rules", "other", L{"tier": "a"}, 3},
		{"unknown kind", "none", L{"tier": "a"}, 0},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := Evaluate(rs, tc.kind, tc.labels)
			if err != nil {
				t.Fatal(err)
			}
			if tc.wantID == 0 {
				if got != nil {
					t.Fatalf("got rule %d, want nil", got.ID)
				}
				return
			}
			if got == nil || got.ID != tc.wantID {
				t.Fatalf("got %+v, want rule %d", got, tc.wantID)
			}
		})
	}
	t.Run("input not reordered", func(t *testing.T) {
		if rs[0].ID != 1 || rs[1].ID != 2 {
			t.Fatal("Evaluate mutated its input")
		}
	})
	t.Run("no rules", func(t *testing.T) {
		if got, err := Evaluate(nil, KindRate, L{}); got != nil || err != nil {
			t.Fatalf("got %v, %v", got, err)
		}
	})
	t.Run("no match is nil", func(t *testing.T) {
		got, err := Evaluate([]Rule{{ID: 1, Kind: KindRate, Selector: "tier=a"}}, KindRate, L{"tier": "b"})
		if got != nil || err != nil {
			t.Fatalf("got %v, %v", got, err)
		}
	})
	t.Run("ties stable for equal positions", func(t *testing.T) {
		tie := []Rule{
			{ID: 7, Kind: KindRate, Position: 1, Selector: ""},
			{ID: 8, Kind: KindRate, Position: 1, Selector: ""},
		}
		got, _ := Evaluate(tie, KindRate, nil)
		if got.ID != 7 {
			t.Fatalf("got %d, want 7", got.ID)
		}
	})
	t.Run("bad selector errors naming rule", func(t *testing.T) {
		_, err := Evaluate([]Rule{{ID: 9, Kind: KindRate, Selector: "A=b"}}, KindRate, L{})
		if err == nil || !strings.Contains(err.Error(), "rule 9") {
			t.Fatalf("err = %v", err)
		}
	})
	t.Run("bad selector in other kind ignored", func(t *testing.T) {
		got, err := Evaluate([]Rule{{ID: 9, Kind: "other", Selector: "A=b"}}, KindRate, L{})
		if got != nil || err != nil {
			t.Fatalf("got %v, %v", got, err)
		}
	})
	t.Run("returned rule is a copy", func(t *testing.T) {
		one := []Rule{{ID: 1, Kind: KindRate, Value: "5"}}
		got, _ := Evaluate(one, KindRate, nil)
		got.Value = "changed"
		if one[0].Value != "5" {
			t.Fatal("aliasing")
		}
	})
}

func TestRULE1_EqualPositionsKeepInputOrder(t *testing.T) {
	// One rule at Position 9 comes first, then 20 rules share Position 5 and
	// all match. Sorting the 20 ties with sort.Slice (not stable) moves ID 10
	// to the front; only a stable sort gives ID 1, the first of them in input.
	rs := []Rule{{ID: 100, Kind: KindRate, Position: 9, Selector: "", Value: "9"}}
	for i := int64(1); i <= 20; i++ {
		rs = append(rs, Rule{ID: i, Kind: KindRate, Position: 5, Selector: "", Value: "1"})
	}
	got, err := Evaluate(rs, KindRate, L{"tier": "a"})
	if err != nil || got == nil {
		t.Fatalf("Evaluate: got %v, %v", got, err)
	}
	if got.ID != 1 {
		t.Fatalf("first match among equal positions = rule %d, want 1 (input order)", got.ID)
	}
}

func TestRULE1_ParseWeight(t *testing.T) {
	ok := []struct {
		in   string
		want float64
	}{
		{"0", 0}, {"1", 1}, {"0.5", 0.5}, {"2.25", 2.25}, {".5", 0.5}, {"3.", 3}, {"0.0", 0}, {"007", 7}, {"1000000", 1e6},
	}
	for _, tc := range ok {
		t.Run("ok "+tc.in, func(t *testing.T) {
			got, err := ParseWeight(tc.in)
			if err != nil || got != tc.want {
				t.Fatalf("got %v, %v", got, err)
			}
		})
	}
	bad := []string{"", " ", "-1", "-0", "+1", "NaN", "nan", "Inf", "+Inf", "inf", "1e3", "0x10", "1_0", "abc", "1.2.3", ".", " 1", "1 ", "1,5",
		strings.Repeat("9", 400)}
	for _, in := range bad {
		name := in
		if len(name) > 20 {
			name = name[:20] + "..."
		}
		t.Run("bad "+name, func(t *testing.T) {
			got, err := ParseWeight(in)
			if err == nil {
				t.Fatalf("ParseWeight(%q) = %v, want error", in, got)
			}
		})
	}
	t.Run("never inf or nan", func(t *testing.T) {
		got, _ := ParseWeight("1")
		if math.IsInf(got, 0) || math.IsNaN(got) {
			t.Fatal("bad")
		}
	})
}

func TestRULE1_Weight(t *testing.T) {
	rs := []Rule{
		{ID: 1, Kind: KindRate, Position: 2, Selector: "tier=b", Value: "0"},
		{ID: 2, Kind: KindRate, Position: 1, Selector: "tier=a", Value: "2.5"},
		{ID: 3, Kind: "other", Position: 0, Selector: "", Value: "99"},
		{ID: 4, Kind: KindRate, Position: 3, Selector: "site=x", Value: "oops"},
	}
	tests := []struct {
		name        string
		labels      L
		wantW       float64
		wantSel     string
		wantMatched bool
		wantErr     bool
	}{
		{"first match", L{"tier": "a"}, 2.5, "tier=a", true, false},
		{"zero weight is a match", L{"tier": "b"}, 0, "tier=b", true, false},
		{"default when none", L{"tier": "c"}, 7, "", false, false},
		{"other kind ignored", L{}, 7, "", false, false},
		{"invalid value errors", L{"site": "x"}, 7, "", false, true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			w, sel, matched, err := Weight(rs, tc.labels, 7)
			if (err != nil) != tc.wantErr || w != tc.wantW || sel != tc.wantSel || matched != tc.wantMatched {
				t.Fatalf("got (%v, %q, %v, %v)", w, sel, matched, err)
			}
		})
	}
	t.Run("bad selector errors", func(t *testing.T) {
		_, _, m, err := Weight([]Rule{{ID: 1, Kind: KindRate, Selector: "A=b"}}, nil, 1)
		if err == nil || m {
			t.Fatal("want error")
		}
	})
	t.Run("no rules uses default", func(t *testing.T) {
		w, _, m, err := Weight(nil, nil, 1)
		if w != 1 || m || err != nil {
			t.Fatal("bad default")
		}
	})
}

func TestRULE3_MatchCounts(t *testing.T) {
	rs := []Rule{
		{ID: 1, Kind: KindRate, Position: 2, Selector: "tier in (a,b)"},
		{ID: 2, Kind: KindRate, Position: 1, Selector: "tier=a"},
		{ID: 3, Kind: KindRate, Position: 3, Selector: "site=x"},
		{ID: 4, Kind: KindRate, Position: 4, Selector: "never=1"},
		{ID: 5, Kind: "other", Position: 0, Selector: ""},
		{ID: 6, Kind: KindRate, Position: 5, Selector: "A=bad"},
	}
	sets := []L{
		{"tier": "a"},              // rule 2 (first), though 1 also matches
		{"tier": "b"},              // rule 1
		{"tier": "b", "site": "x"}, // rule 1
		{"site": "x"},              // rule 3
		{"tier": "z"},              // unmatched
		nil,                        // unmatched
	}
	per, un := MatchCounts(rs, KindRate, sets)
	wantPer := map[int64]int{1: 2, 2: 1, 3: 1, 4: 0, 6: 0}
	if !reflect.DeepEqual(per, wantPer) {
		t.Fatalf("perRule = %v, want %v", per, wantPer)
	}
	if !reflect.DeepEqual(un, []int{4, 5}) {
		t.Fatalf("unmatched = %v", un)
	}
	t.Run("no label sets", func(t *testing.T) {
		per, un := MatchCounts(rs, KindRate, nil)
		if len(un) != 0 || per[1] != 0 || len(per) != 5 {
			t.Fatalf("%v %v", per, un)
		}
	})
	t.Run("no rules everything falls through", func(t *testing.T) {
		per, un := MatchCounts(nil, KindRate, sets)
		if len(per) != 0 || len(un) != len(sets) {
			t.Fatalf("%v %v", per, un)
		}
	})
	t.Run("other kind", func(t *testing.T) {
		per, un := MatchCounts(rs, "other", sets)
		if per[5] != len(sets) || len(un) != 0 {
			t.Fatalf("%v %v", per, un)
		}
	})
}
