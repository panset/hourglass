// SPDX-License-Identifier: MIT

package api

import (
	"net/url"
	"reflect"
	"sort"
	"strings"
	"testing"

	"github.com/panset/hourglass/internal/rules"
	"github.com/panset/hourglass/internal/store"
)

type indexValue struct {
	Value    string
	Selector *string
}

// labelIndexOf reads label_index from a GET /api/v1/setups response.
func labelIndexOf(t *testing.T, l ujobj) (keys []string, values map[string][]indexValue) {
	t.Helper()
	raw, ok := l["label_index"].([]any)
	if !ok {
		t.Fatalf("label_index is %v, want an array", l["label_index"])
	}
	values = map[string][]indexValue{}
	for _, k := range raw {
		ko := k.(ujobj)
		uWantKeys(t, "label_index key", ko, "key", "values")
		key := ko["key"].(string)
		keys = append(keys, key)
		for _, v := range ko["values"].([]any) {
			vo := v.(ujobj)
			uWantKeys(t, "label_index value", vo, "value", "selector")
			iv := indexValue{Value: vo["value"].(string)}
			if s, ok := vo["selector"].(string); ok {
				iv.Selector = &s
			}
			values[key] = append(values[key], iv)
		}
	}
	return keys, values
}

func indexValueNames(vs []indexValue) []string {
	var out []string
	for _, v := range vs {
		out = append(out, v.Value)
	}
	return out
}

func TestSET8_LabelIndexIsSortedAndDistinct(t *testing.T) {
	e := defaultEnv(t)
	e.setup("s1", true, map[string]string{"site": "sjc", "tier": "b"})
	e.setup("s2", true, map[string]string{"site": "ams", "tier": "a"})
	e.setup("s3", true, map[string]string{"site": "sjc", "gpu": "true"})
	dev := e.dev("dev", store.RoleDeveloper)
	keys, values := labelIndexOf(t, uWantOK(t, e.get("/api/v1/setups", &dev), 200))
	if want := []string{"gpu", "site", "tier"}; !reflect.DeepEqual(keys, want) {
		t.Errorf("keys %v, want %v (sorted)", keys, want)
	}
	if got, want := indexValueNames(values["site"]), []string{"ams", "sjc"}; !reflect.DeepEqual(got, want) {
		t.Errorf("site values %v, want %v (sorted, distinct)", got, want)
	}
	if s := values["site"][1].Selector; s == nil || *s != "site=sjc" {
		t.Errorf("site=sjc term %v", s)
	}
}

func TestSET8_LabelIndexEmptyIsEmptyArray(t *testing.T) {
	e := defaultEnv(t)
	e.setup("s1", true, map[string]string{})
	dev := e.dev("dev", store.RoleDeveloper)
	rec := e.get("/api/v1/setups", &dev)
	uWantOK(t, rec, 200)
	if !strings.Contains(rec.Body.String(), `"label_index":[]`) {
		t.Errorf("label_index is not []: %s", rec.Body)
	}
}

func TestSET8_LabelIndexIgnoresSelector(t *testing.T) {
	e := defaultEnv(t)
	e.setup("s1", true, map[string]string{"site": "sjc"})
	e.setup("s2", true, map[string]string{"site": "ams", "tier": "a"})
	dev := e.dev("dev", store.RoleDeveloper)
	l := uWantOK(t, e.get("/api/v1/setups?selector="+url.QueryEscape("site=sjc"), &dev), 200)
	if n := len(l["setups"].([]any)); n != 1 {
		t.Fatalf("selector did not filter the list: %d setups", n)
	}
	keys, values := labelIndexOf(t, l)
	if want := []string{"site", "tier"}; !reflect.DeepEqual(keys, want) {
		t.Errorf("keys %v, want %v: the index must ignore selector", keys, want)
	}
	if got := indexValueNames(values["site"]); !reflect.DeepEqual(got, []string{"ams", "sjc"}) {
		t.Errorf("site values %v: the index must ignore selector", got)
	}
}

func TestSET8_DeveloperIndexHidesDisabledOnlyLabels(t *testing.T) {
	e := defaultEnv(t)
	e.setup("on", true, map[string]string{"site": "sjc"})
	e.setup("off", false, map[string]string{"site": "sjc", "secret": "x", "tier": "only-off"})
	dev, adm := e.dev("dev", store.RoleDeveloper), e.dev("boss", store.RoleAdmin)
	devKeys, devValues := labelIndexOf(t, uWantOK(t, e.get("/api/v1/setups", &dev), 200))
	if want := []string{"site"}; !reflect.DeepEqual(devKeys, want) {
		t.Errorf("developer keys %v, want %v: a label only on a disabled setup leaked", devKeys, want)
	}
	if len(devValues["site"]) != 1 {
		t.Errorf("developer site values %v", devValues["site"])
	}
	admKeys, _ := labelIndexOf(t, uWantOK(t, e.get("/api/v1/setups", &adm), 200))
	if want := []string{"secret", "site", "tier"}; !reflect.DeepEqual(admKeys, want) {
		t.Errorf("admin keys %v, want %v", admKeys, want)
	}
}

// Every non-null term parses with the real parser and selects exactly the
// setups that carry that value; values the grammar cannot express have a null term.
func TestSET8_LabelIndexTermsMatchExactlyTheirSetups(t *testing.T) {
	e := defaultEnv(t)
	odd := []string{"a.b", "x_y", "a-b/c", "v1:2", "UPPER", "ünï", "in", "notin", "ok'q", "[z]", "*", "{}", "a;b", "~", "100%", ""}
	unwritable := []string{"has space", "tab\there", "a,b", "(p)", "a=b", "x!y", "!", "new\nline", "=", ","}
	labelsOf := map[string]map[string]string{}
	add := func(name, key, value string) {
		l := map[string]string{key: value, "site": "common"}
		labelsOf[name] = l
		e.setup(name, true, l)
	}
	for i, v := range odd {
		add("odd"+uItoa(int64(i)), "tier", v)
	}
	for i, v := range unwritable {
		add("bad"+uItoa(int64(i)), "tier", v)
	}
	add("dotkey", "a.b-c/d_e", "v")
	dev := e.dev("dev", store.RoleDeveloper)
	_, values := labelIndexOf(t, uWantOK(t, e.get("/api/v1/setups", &dev), 200))

	check := func(key, value string, wantNull bool) {
		var got *indexValue
		for i := range values[key] {
			if values[key][i].Value == value {
				got = &values[key][i]
			}
		}
		if got == nil {
			t.Fatalf("%q=%q missing from the index", key, value)
		}
		if wantNull {
			if got.Selector != nil {
				t.Errorf("%q=%q has term %q, want null", key, value, *got.Selector)
			}
			return
		}
		if got.Selector == nil {
			t.Fatalf("%q=%q has a null term", key, value)
		}
		term := *got.Selector
		sel, err := rules.Parse(term)
		if err != nil {
			t.Fatalf("term %q does not parse: %v", term, err)
		}
		var want, have []string
		for name, l := range labelsOf {
			if v, ok := l[key]; ok && v == value {
				want = append(want, name)
			}
			if sel.Matches(l) {
				have = append(have, name)
			}
		}
		sort.Strings(want)
		sort.Strings(have)
		if !reflect.DeepEqual(want, have) {
			t.Errorf("term %q matches %v, want exactly %v", term, have, want)
		}
		// The API agrees with the parser end to end.
		l := uWantOK(t, e.get("/api/v1/setups?selector="+url.QueryEscape(term), &dev), 200)
		var viaAPI []string
		for _, s := range l["setups"].([]any) {
			viaAPI = append(viaAPI, s.(ujobj)["name"].(string))
		}
		sort.Strings(viaAPI)
		if !reflect.DeepEqual(want, viaAPI) {
			t.Errorf("GET /setups?selector=%q returned %v, want %v", term, viaAPI, want)
		}
	}
	for _, v := range odd {
		check("tier", v, false)
	}
	for _, v := range unwritable {
		check("tier", v, true)
	}
	check("a.b-c/d_e", "v", false)
	check("site", "common", false)
	// The empty value is written key=.
	for _, v := range values["tier"] {
		if v.Value == "" && (v.Selector == nil || *v.Selector != "tier=") {
			t.Errorf("empty value term %v, want tier=", v.Selector)
		}
	}
}
