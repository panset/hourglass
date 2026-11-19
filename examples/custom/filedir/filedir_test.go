// SPDX-License-Identifier: MIT

package filedir

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/panset/hourglass/pkg/hourglass"
	"github.com/panset/hourglass/pkg/hourglasstest"
)

func rigDir(t *testing.T, files ...string) string {
	t.Helper()
	dir := t.TempDir()
	for _, f := range files {
		if err := os.WriteFile(filepath.Join(dir, f), nil, 0o600); err != nil {
			t.Fatal(err)
		}
	}
	return dir
}

func newProvider(t *testing.T, dir string) hourglass.ResourceProvider {
	t.Helper()
	raw, _ := json.Marshal(map[string]string{"dir": dir})
	p, err := New(raw)
	if err != nil {
		t.Fatal(err)
	}
	return p
}

func TestEXT5_FiledirConformance(t *testing.T) {
	dir := rigDir(t, "rig-a", "rig-b")
	hourglasstest.TestProvider(t, func() hourglass.ResourceProvider { return newProvider(t, dir) })
}

func TestEXT5_FiledirListsOneAllocationPerFile(t *testing.T) {
	dir := rigDir(t, "rig-b", "rig-a", ".hidden")
	if err := os.Mkdir(filepath.Join(dir, "subdir"), 0o700); err != nil {
		t.Fatal(err)
	}
	got, err := newProvider(t, dir).List(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	var names []string
	for _, a := range got {
		names = append(names, a.ResourceName)
	}
	if want := []string{"rig-a", "rig-b"}; !reflect.DeepEqual(names, want) {
		t.Fatalf("names %v, want %v", names, want)
	}
}

func TestEXT5_FiledirSeesFilesComeAndGo(t *testing.T) {
	dir := rigDir(t, "rig-a")
	p := newProvider(t, dir)
	if err := os.Remove(filepath.Join(dir, "rig-a")); err != nil {
		t.Fatal(err)
	}
	got, err := p.List(context.Background())
	if err != nil || len(got) != 0 {
		t.Fatalf("got %v, %v; want no allocations", got, err)
	}
}

func TestEXT5_FiledirMissingDirIsAnError(t *testing.T) {
	if _, err := newProvider(t, filepath.Join(t.TempDir(), "nope")).List(context.Background()); err == nil {
		t.Fatal("want an error for a missing directory")
	}
}

func TestEXT5_FiledirRejectsBadConfig(t *testing.T) {
	for _, raw := range []string{``, `null`, `{}`, `{"dir":""}`, `{"dir":"x","extra":1}`} {
		if _, err := New(json.RawMessage(raw)); err == nil {
			t.Errorf("config %q: want an error", raw)
		}
	}
}
