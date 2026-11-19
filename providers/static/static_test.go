// SPDX-License-Identifier: MIT

package static

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/panset/hourglass/pkg/hourglass"
	"github.com/panset/hourglass/pkg/hourglasstest"
)

func writeFile(t *testing.T, content string) string {
	t.Helper()
	p := filepath.Join(t.TempDir(), "alloc.json")
	if err := os.WriteFile(p, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
	return p
}

func build(t *testing.T, path string) hourglass.ResourceProvider {
	t.Helper()
	cfg, _ := json.Marshal(map[string]string{"path": path})
	p, err := hourglass.NewProvider(Name, cfg)
	if err != nil {
		t.Fatal(err)
	}
	return p
}

func TestEXT5_Conformance(t *testing.T) {
	path := writeFile(t, `[{"resource_name":"a","labels":{"k":"v"}},{"resource_name":"b"}]`)
	hourglasstest.TestProvider(t, func() hourglass.ResourceProvider { return build(t, path) })
}

func TestEXT3_FactoryConfig(t *testing.T) {
	cases := []struct {
		name, cfg string
		ok        bool
	}{
		{"valid", `{"path":"/x"}`, true},
		{"empty", ``, false},
		{"empty object", `{}`, false},
		{"unknown field", `{"path":"/x","extra":1}`, false},
		{"malformed", `{"path":`, false},
		{"empty path", `{"path":""}`, false},
		{"null", `null`, false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			_, err := New(json.RawMessage(c.cfg))
			if (err == nil) != c.ok {
				t.Fatalf("err = %v, want ok=%v", err, c.ok)
			}
		})
	}
}

func TestEXT3_ListBehaviour(t *testing.T) {
	cases := []struct {
		name, content string
		want          []hourglass.Allocation
		wantErr       bool
	}{
		{"entries", `[{"resource_name":"a","labels":{"k":"v"}},{"resource_name":"b"}]`,
			[]hourglass.Allocation{{ResourceName: "a", Labels: map[string]string{"k": "v"}}, {ResourceName: "b"}}, false},
		{"empty array", `[]`, []hourglass.Allocation{}, false},
		{"empty name", `[{"resource_name":""}]`, nil, true},
		{"missing name", `[{"labels":{}}]`, nil, true},
		{"malformed", `[{`, nil, true},
		{"not an array", `{"resource_name":"a"}`, nil, true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got, err := build(t, writeFile(t, c.content)).List(context.Background())
			if (err != nil) != c.wantErr {
				t.Fatalf("err = %v, wantErr %v", err, c.wantErr)
			}
			if !c.wantErr && !reflect.DeepEqual(got, c.want) {
				t.Fatalf("got %#v, want %#v", got, c.want)
			}
		})
	}
	t.Run("missing file", func(t *testing.T) {
		if _, err := build(t, filepath.Join(t.TempDir(), "nope.json")).List(context.Background()); err == nil {
			t.Fatal("want error")
		}
	})
}

func TestEXT3_ReReadOnEveryList(t *testing.T) {
	path := writeFile(t, `[{"resource_name":"a"}]`)
	p := build(t, path)
	a, err := p.List(context.Background())
	if err != nil || len(a) != 1 {
		t.Fatalf("first: %v %v", a, err)
	}
	if err := os.WriteFile(path, []byte(`[{"resource_name":"a"},{"resource_name":"b"}]`), 0o600); err != nil {
		t.Fatal(err)
	}
	b, err := p.List(context.Background())
	if err != nil || len(b) != 2 {
		t.Fatalf("second: %v %v", b, err)
	}
}

func TestEXT3_CancelledContext(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := build(t, writeFile(t, `[]`)).List(ctx); err == nil {
		t.Fatal("want context error")
	}
}

func TestEXT3_NullAndEmptyConfigAndFile(t *testing.T) {
	for _, raw := range []string{"", "null", " null "} {
		_, err := New(json.RawMessage(raw))
		if err == nil || !strings.Contains(err.Error(), `"path" is required`) {
			t.Errorf("config %q: %v", raw, err)
		}
	}
	if _, err := build(t, writeFile(t, "null")).List(context.Background()); err == nil {
		t.Error("file containing null: want error")
	}
}
