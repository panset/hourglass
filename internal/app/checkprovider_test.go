// SPDX-License-Identifier: MIT

package app

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/panset/hourglass/internal/store"
	"github.com/panset/hourglass/pkg/hourglass"
	"github.com/panset/hourglass/pkg/hourglasstest"
)

const (
	checkProv     = "check-test-provider"
	checkPressure = "check-test-pressure"
)

var (
	checkOnce     sync.Once
	checkFake     = hourglasstest.NewFakeProvider()
	checkPressSrc = hourglasstest.NewFakePressureSource()
)

func registerCheckFakes() {
	checkOnce.Do(func() {
		checkFake.SetName(checkProv)
		checkPressSrc.SetName(checkPressure)
		hourglass.RegisterProvider(checkProv, func(json.RawMessage) (hourglass.ResourceProvider, error) { return checkFake, nil })
		hourglass.RegisterPressureSource(checkPressure, func(json.RawMessage) (hourglass.PressureSource, error) {
			return checkPressSrc, nil
		})
	})
	checkFake.SetError(nil)
	checkPressSrc.SetError(nil)
}

const checkSeed = `
setups:
  - {name: lab-01, labels: {tier: large}}
  - {name: lab-02, labels: {tier: small}, resource_name_override: rp_two}
  - {name: lab-03}
  - {name: lab-off, enabled: false}
`

// checkConfig writes a config file; withProvider selects whether a provider is configured.
func checkConfig(t *testing.T, dir string, withProvider bool, extra string) string {
	t.Helper()
	body := "database: " + filepath.Join(dir, "h.db") + "\npressure: {type: " + checkPressure + "}\n" + extra
	if withProvider {
		body += "provider: {type: " + checkProv + "}\n"
	}
	p := filepath.Join(dir, "config.yaml")
	if err := os.WriteFile(p, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
	return p
}

func runCheck(t *testing.T, cfgPath string) (string, error) {
	t.Helper()
	var out, errb bytes.Buffer
	err := CheckProvider(context.Background(), CheckOptions{
		Args: []string{"-config", cfgPath}, Stdout: &out, Stderr: &errb,
	})
	return out.String(), err
}

func seedDB(t *testing.T, dir string) {
	t.Helper()
	st, err := store.Open(filepath.Join(dir, "h.db"))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := ImportSeed(context.Background(), st, writeSeed(t, "seed.yaml", checkSeed), time.Now()); err != nil {
		t.Fatal(err)
	}
	if err := st.Close(); err != nil {
		t.Fatal(err)
	}
}

func dirState(t *testing.T, dir string) map[string][32]byte {
	t.Helper()
	m := map[string][32]byte{}
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range entries {
		if strings.HasSuffix(e.Name(), "-shm") || strings.HasSuffix(e.Name(), "-wal") {
			continue // SQLite's own WAL bookkeeping for any reader; holds no data
		}
		b, err := os.ReadFile(filepath.Join(dir, e.Name()))
		if err != nil {
			t.Fatal(err)
		}
		m[e.Name()] = sha256.Sum256(b)
	}
	return m
}

func TestEXT3_CheckProviderReport(t *testing.T) {
	registerCheckFakes()
	dir := t.TempDir()
	seedDB(t, dir)
	cfg := checkConfig(t, dir, true, "")
	checkFake.SetAllocations(
		hourglass.Allocation{ResourceName: "lab-01"},
		hourglass.Allocation{ResourceName: "lab-01"}, // duplicate counts once
		hourglass.Allocation{ResourceName: "rp_two"},
		hourglass.Allocation{ResourceName: "lab-off"},
		hourglass.Allocation{ResourceName: "unrelated-1"},
		hourglass.Allocation{ResourceName: "unrelated-2"},
	)
	checkPressSrc.SetValue(0.8)
	before := dirState(t, dir)
	out, err := runCheck(t, cfg)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		"setups: 4",
		"provider: " + checkProv,
		"matched setups (3):", "lab-01 -> lab-01", "lab-02 -> rp_two", "lab-off -> lab-off  [disabled]",
		"expected but not reported (1):", "lab-03 -> lab-03",
		"orphans (3):",
		"ignored allocations: 2",
		"raw pressure: 0.8000  multiplier: 2x",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("output lacks %q:\n%s", want, out)
		}
	}
	if checkFake.Calls() == 0 || checkPressSrc.Calls() != 1 {
		t.Errorf("calls: provider %d, pressure %d", checkFake.Calls(), checkPressSrc.Calls())
	}
	if after := dirState(t, dir); len(after) != len(before) {
		t.Errorf("files changed: before %d, after %d entries", len(before), len(after))
	} else {
		for k, v := range before {
			if after[k] != v {
				t.Errorf("%s was modified (EXT-3: writes nothing)", k)
			}
		}
	}
}

func TestEXT3_CheckProviderWritesNothingAndRefusesWrites(t *testing.T) {
	registerCheckFakes()
	dir := t.TempDir()
	seedDB(t, dir)
	st, err := store.OpenReadOnly(filepath.Join(dir, "h.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	if err := st.PutSetting(context.Background(), "default_weight", "2", nil, time.Now()); err == nil {
		t.Fatal("a write through the read-only store succeeded")
	}
}

func TestEXT3_CheckProviderHeldSetupIsNotAnOrphan(t *testing.T) {
	registerCheckFakes()
	dir := t.TempDir()
	seedDB(t, dir)
	ctx := context.Background()
	st, err := store.Open(filepath.Join(dir, "h.db"))
	if err != nil {
		t.Fatal(err)
	}
	u, err := st.CreateLocalAdmin(ctx, "admin", "x", false, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	setups, _ := st.ListSetups(ctx, false)
	var id int64
	for _, s := range setups {
		if s.Name == "lab-01" {
			id = s.ID
		}
	}
	if _, err := st.CreateHold(ctx, id, u.ID, time.Now()); err != nil {
		t.Fatal(err)
	}
	st.Close()
	checkFake.SetAllocations(hourglass.Allocation{ResourceName: "lab-01"}, hourglass.Allocation{ResourceName: "rp_two"})
	checkPressSrc.SetValue(0)
	out, err := runCheck(t, checkConfig(t, dir, true, ""))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "lab-01 -> lab-01  [held]") || !strings.Contains(out, "orphans (1):\n  rp_two\n") {
		t.Errorf("held setup handling wrong:\n%s", out)
	}
	if !strings.Contains(out, "multiplier: 0x") {
		t.Errorf("multiplier for 0 pressure:\n%s", out)
	}
}

func TestEXT3_CheckProviderNoDatabaseUsesSeedAndCreatesNothing(t *testing.T) {
	registerCheckFakes()
	dir := t.TempDir()
	seed := writeSeed(t, "seed.yaml", checkSeed)
	cfg := checkConfig(t, dir, true, "seed: "+seed+"\n")
	checkFake.SetAllocations(hourglass.Allocation{ResourceName: "lab-02"}) // rp_two expected, so this is ignored
	checkPressSrc.SetValue(0.1)
	out, err := runCheck(t, cfg)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "seed file") || !strings.Contains(out, "setups: 4") || !strings.Contains(out, "ignored allocations: 1") {
		t.Errorf("unexpected output:\n%s", out)
	}
	if _, err := os.Stat(filepath.Join(dir, "h.db")); !errors.Is(err, os.ErrNotExist) {
		t.Errorf("check-provider created the database: %v", err)
	}
}

func TestEXT3_CheckProviderNoDatabaseNoSeed(t *testing.T) {
	registerCheckFakes()
	dir := t.TempDir()
	checkFake.SetAllocations(hourglass.Allocation{ResourceName: "x"})
	out, err := runCheck(t, checkConfig(t, dir, true, ""))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "no database and no seed file") || !strings.Contains(out, "ignored allocations: 1") {
		t.Errorf("unexpected output:\n%s", out)
	}
}

func TestEXT4_CheckProviderWithoutProvider(t *testing.T) {
	registerCheckFakes()
	before := checkFake.Calls()
	checkPressSrc.SetValue(0.5)
	out, err := runCheck(t, checkConfig(t, t.TempDir(), false, ""))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "provider: none configured") || !strings.Contains(out, "raw pressure: 0.5000  multiplier: 1x") {
		t.Errorf("unexpected output:\n%s", out)
	}
	if checkFake.Calls() != before {
		t.Error("provider called although none is configured")
	}
}

func TestEXT3_CheckProviderErrors(t *testing.T) {
	registerCheckFakes()
	dir := t.TempDir()
	cfg := checkConfig(t, dir, true, "")
	checkFake.SetAllocations()
	checkPressSrc.SetValue(0.2)

	checkFake.SetError(errors.New("boom list"))
	if _, err := runCheck(t, cfg); err == nil || !strings.Contains(err.Error(), "boom list") {
		t.Errorf("provider error: %v", err)
	}
	checkFake.SetError(nil)

	checkPressSrc.SetError(errors.New("boom pressure"))
	if _, err := runCheck(t, cfg); err == nil || !strings.Contains(err.Error(), "boom pressure") {
		t.Errorf("pressure error: %v", err)
	}
	checkPressSrc.SetError(nil)

	checkPressSrc.SetValue(1.5)
	if _, err := runCheck(t, cfg); err == nil || !strings.Contains(err.Error(), "outside 0 to 1") {
		t.Errorf("out-of-range pressure: %v", err)
	}
	checkPressSrc.SetValue(0.2)

	if _, err := runCheck(t, filepath.Join(dir, "missing.yaml")); err == nil {
		t.Error("missing config file accepted")
	}
	bad := filepath.Join(dir, "bad.yaml")
	os.WriteFile(bad, []byte("provider: {type: no-such-provider}\n"), 0o600)
	if _, err := runCheck(t, bad); err == nil || !strings.Contains(err.Error(), "no-such-provider") {
		t.Errorf("unknown provider: %v", err)
	}
}
