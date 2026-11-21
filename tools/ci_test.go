// SPDX-License-Identifier: MIT

package tools

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// findUp walks up from the working directory (under Bazel, the runfiles
// directory) until rel exists.
func findUp(t *testing.T, rel string) string {
	t.Helper()
	dir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	for {
		p := filepath.Join(dir, rel)
		if _, err := os.Stat(p); err == nil {
			return p
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatalf("%s not found above the working directory (Bazel: is it in the test's data?)", rel)
		}
		dir = parent
	}
}

// TestCMP2_CIRunsEveryRequiredStep checks that ci.sh still runs each step
// CMP-2 names: SPDX headers, genericity, licenses, Gazelle, bazel build and
// bazel test. A step is a line that starts with the command.
func TestCMP2_CIRunsEveryRequiredStep(t *testing.T) {
	raw, err := os.ReadFile(findUp(t, "ci.sh"))
	if err != nil {
		t.Fatal(err)
	}
	var lines []string
	for _, l := range strings.Split(string(raw), "\n") {
		l = strings.TrimSpace(l)
		if l != "" && !strings.HasPrefix(l, "#") {
			lines = append(lines, l)
		}
	}
	steps := []struct{ name, prefix string }{
		{"SPDX check", "bash tools/check_spdx.sh"},
		{"genericity check", "bash tools/check_core_generic.sh"},
		{"license check", "bash tools/check_licenses.sh"},
		{"Gazelle check", "bazel run //:gazelle -- update -mode=diff"},
		{"bazel build //...", "bazel build //..."},
		{"bazel test //...", "bazel test //..."},
	}
	for _, s := range steps {
		found := false
		for _, l := range lines {
			if strings.HasPrefix(l, s.prefix) {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("ci.sh has no step for the %s (expected a line starting %q)", s.name, s.prefix)
		}
	}
}
