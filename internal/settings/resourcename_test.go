// SPDX-License-Identifier: MIT

package settings

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/panset/hourglass/internal/store"
)

func TestSET4_TemplateAllowedForms(t *testing.T) {
	labels := map[string]string{"size": "large", "env": "lab"}
	for _, c := range []struct{ tmpl, want string }{
		{`{{.SetupName}}`, "s1"},
		{`{{.SetupName}}_RP`, "s1_RP"},
		{`{{.Labels.size}}-{{.SetupName}}`, "large-s1"},
		{`{{index .Labels "env"}}/{{.SetupName}}`, "lab/s1"},
		{`{{printf "%s-%s" .SetupName .Labels.size}}`, "s1-large"},
		{`{{print .SetupName "x"}}`, "s1x"},
		{`{{.SetupName | printf "%s!"}}`, "s1!"},
		{`{{if .Labels.size}}{{.Labels.size}}{{else}}none{{end}}-{{.SetupName}}`, "large-s1"},
		{`{{with .Labels}}{{.size}}{{end}}{{.SetupName}}`, "larges1"},
		{`{{.Labels.missing}}{{.SetupName}}`, "s1"},
	} {
		got, err := ResourceName(c.tmpl, "s1", labels, "")
		if err != nil || got != c.want {
			t.Errorf("%s: got %q, %v; want %q", c.tmpl, got, err, c.want)
		}
	}
}

func TestSET4_TemplateRejected(t *testing.T) {
	// The first is the verifier's hanging template (task 23, finding 1).
	for _, tmpl := range []string{
		`{{if eq .SetupName "sample-setup"}}x{{else}}{{range 3000000000}}{{end}}{{.SetupName}}{{end}}`,
		`{{range .Labels}}{{.}}{{end}}x`,
		`{{define "a"}}{{template "a"}}{{end}}{{template "a"}}`,
		`{{block "a" .}}x{{end}}`,
		`{{template "a"}}`,
		`{{printf "%0999999d" 1}}`,
		`{{printf "%*d" 5 1}}`,
		`{{printf .SetupName}}`,
		`{{len .SetupName}}`,
		`{{lower .SetupName}}`,
		`{{call .SetupName}}`,
		`{{with .SetupName}}x{{end}}`,
		`{{$x := .SetupName}}{{$x}}`,
		`{{.Nope}}`,
		`{{.SetupName.Foo}}`,
		`{{.Labels.a.b}}`,
		`{{if eq .SetupName "a"}}x{{end}}`,
		`{{slice .SetupName 1}}`,
		`{{.}}`,
		strings.Repeat("a", MaxTemplateLen) + "{{.SetupName}}",
	} {
		start := time.Now()
		if _, err := ResourceName(tmpl, "s1", nil, ""); err == nil {
			t.Errorf("template %.60q accepted", tmpl)
		}
		if time.Since(start) > time.Second {
			t.Errorf("template %.60q took %s to reject", tmpl, time.Since(start))
		}
		s := Defaults()
		s.ResourceNameTemplate = tmpl
		if err := s.Validate(); err == nil {
			t.Errorf("Validate accepted %.60q", tmpl)
		}
	}
}

func TestSET4_RenderedNameCapped(t *testing.T) {
	long := strings.Repeat("x", 200)
	if _, err := ResourceName(`{{.SetupName}}`, long, nil, ""); err != nil {
		t.Fatalf("200 characters: %v", err)
	}
	if _, err := ResourceName(`{{.SetupName}}{{.SetupName}}`, long, nil, ""); err == nil ||
		!strings.Contains(err.Error(), "253") {
		t.Fatalf("400 characters: %v", err)
	}
	if got, err := ResourceName(`{{.SetupName}}`, strings.Repeat("y", 253), nil, ""); err != nil || len(got) != 253 {
		t.Fatalf("exactly 253: %d, %v", len(got), err)
	}
}

type rowsReader struct{ rows []store.Setting }

func (r rowsReader) AllSettings(context.Context) ([]store.Setting, error) { return r.rows, nil }

func TestSET4_BadStoredTemplateFailsLoad(t *testing.T) {
	r := rowsReader{rows: []store.Setting{{Key: KeyResourceNameTemplate, ValueJSON: `"{{range 3000000000}}{{end}}x"`}}}
	_, err := Load(context.Background(), r)
	if err == nil || !strings.Contains(err.Error(), "resource_name_template") || !strings.Contains(err.Error(), "range") {
		t.Fatalf("Load: %v", err)
	}
}
