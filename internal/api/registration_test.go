// SPDX-License-Identifier: MIT

package api

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"testing"
)

// TestAUTH4_EveryRegisteredPatternIsInTheRouteTable: the patterns that reach
// the public mux are exactly the route table plus the three catch-alls.
func TestAUTH4_EveryRegisteredPatternIsInTheRouteTable(t *testing.T) {
	b := newBackend(t, backendOpts{})
	_, reg := build(b.deps)
	want := []string{"/api/", "/auth/", "/"}
	for _, r := range Routes() {
		want = append(want, r.Method+" "+r.Pattern)
	}
	got := append([]string(nil), reg.patterns...)
	sort.Strings(got)
	sort.Strings(want)
	if strings.Join(got, "\n") != strings.Join(want, "\n") {
		t.Errorf("registered patterns differ from the route table\n got: %q\nwant: %q", got, want)
	}
}

// handlerRegistration matches what adds a handler to a mux: a call of a
// method named Handle or HandleFunc (mux.Handle(, x.mux.HandleFunc(), the
// package-level http.Handle/http.HandleFunc, and the creation of a new
// ServeMux (http.NewServeMux or &http.ServeMux{}), which would be a second
// router. Calls of a function value named handle or register do not match.
var handlerRegistration = regexp.MustCompile(`\.Handle(Func)?\(|http\.Handle(Func)?\(|NewServeMux\(|http\.ServeMux\b`)

// registrationAllowed lists the only non-test lines that may match, by file
// and a substring of the line.
var registrationAllowed = []struct{ file, contains, why string }{
	{"internal/api/api.go", "r.mux.Handle(pattern, h)", "the one registration point of the public listener"},
	{"internal/api/api.go", "http.NewServeMux()", "the public listener's mux, wrapped by registrar"},
	{"internal/api/api.go", "mux      *http.ServeMux", "the registrar's field"},
	{"internal/app/metrics.go", "", "the separate metrics listener, which never serves the API or the UI"},
}

// TestAUTH4_NoHandlerIsRegisteredOutsideTheRouteTable is a source scan: any
// non-test .go file in internal/api, internal/app, internal/auth or cmd/ that
// calls .Handle(/.HandleFunc(, http.Handle(/http.HandleFunc(, or creates a
// ServeMux, other than the lines in registrationAllowed, fails the suite. A
// route added anywhere else would escape the permission and CSRF tests, which
// walk the route table.
func TestAUTH4_NoHandlerIsRegisteredOutsideTheRouteTable(t *testing.T) {
	root := filepath.Join("..", "..")
	scanned := 0
	for _, dir := range []string{"internal/api", "internal/app", "internal/auth", "cmd"} {
		err := filepath.WalkDir(filepath.Join(root, dir), func(p string, d os.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if d.IsDir() || !strings.HasSuffix(p, ".go") || strings.HasSuffix(p, "_test.go") {
				return nil
			}
			rel, _ := filepath.Rel(root, p)
			rel = filepath.ToSlash(rel)
			src, err := os.ReadFile(p)
			if err != nil {
				return err
			}
			scanned++
			for i, line := range strings.Split(string(src), "\n") {
				if c := strings.Index(line, "//"); c >= 0 && !strings.Contains(line[:c], `"`) {
					line = line[:c]
				}
				if !handlerRegistration.MatchString(line) {
					continue
				}
				ok := false
				for _, a := range registrationAllowed {
					if a.file == rel && strings.Contains(line, a.contains) {
						ok = true
					}
				}
				if !ok {
					t.Errorf("%s:%d registers an HTTP handler outside the route table: %s", rel, i+1, strings.TrimSpace(line))
				}
			}
			return nil
		})
		if err != nil {
			t.Fatal(err)
		}
	}
	if scanned < 20 {
		t.Errorf("scanned only %d files; the scan is not finding the sources", scanned)
	}
}

// uiRouteLiterals are the UI's client-side routes. UI-7: nothing in the Go
// code may depend on them (the OIDC callback redirects only to "/").
var uiRouteLiterals = []string{"/login", "/unauthorized", "/setups", "/usage", "/admin/users", "/admin/setups", "/admin/settings", "/admin/audit"}

// TestUI7_NoGoCodeNamesAUIRoute is a source scan: a string literal in non-test
// Go code under internal/, cmd/ or pkg/ that is, or starts with, a UI route
// (outside comments) fails the suite. API paths such as /api/v1/setups do not
// match because they do not start with the route.
func TestUI7_NoGoCodeNamesAUIRoute(t *testing.T) {
	root := filepath.Join("..", "..")
	fset := token.NewFileSet()
	scanned := 0
	for _, dir := range []string{"internal", "cmd", "pkg"} {
		err := filepath.WalkDir(filepath.Join(root, dir), func(p string, d os.DirEntry, err error) error {
			if err != nil {
				if os.IsNotExist(err) {
					return nil
				}
				return err
			}
			if d.IsDir() || !strings.HasSuffix(p, ".go") || strings.HasSuffix(p, "_test.go") {
				return nil
			}
			f, err := parser.ParseFile(fset, p, nil, 0) // comments are dropped
			if err != nil {
				return err
			}
			scanned++
			ast.Inspect(f, func(n ast.Node) bool {
				lit, ok := n.(*ast.BasicLit)
				if !ok || lit.Kind != token.STRING {
					return true
				}
				v, err := strconv.Unquote(lit.Value)
				if err != nil {
					return true
				}
				for _, r := range uiRouteLiterals {
					if v == r || strings.HasPrefix(v, r+"/") || strings.HasPrefix(v, r+"?") {
						t.Errorf("UI-7: %s names the UI route %q in %q; the backend must not depend on UI routes", fset.Position(lit.Pos()), r, v)
					}
				}
				return true
			})
			return nil
		})
		if err != nil {
			t.Fatal(err)
		}
	}
	if scanned == 0 {
		t.Fatal("scanned no files")
	}
}
