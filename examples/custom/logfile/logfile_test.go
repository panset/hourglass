// SPDX-License-Identifier: MIT

package logfile

import (
	"bufio"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/panset/hourglass/pkg/hourglass"
	"github.com/panset/hourglass/pkg/hourglasstest"
)

func newHook(t *testing.T, path string, events ...string) hourglass.Hook {
	t.Helper()
	raw, _ := json.Marshal(map[string]any{"path": path, "events": events})
	h, err := New(raw)
	if err != nil {
		t.Fatal(err)
	}
	return h
}

func readLines(t *testing.T, path string) []map[string]any {
	t.Helper()
	f, err := os.Open(path)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	var out []map[string]any
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		var m map[string]any
		if err := json.Unmarshal(sc.Bytes(), &m); err != nil {
			t.Fatalf("not a JSON line: %q", sc.Text())
		}
		out = append(out, m)
	}
	return out
}

func TestEXT5_LogfileConformance(t *testing.T) {
	path := filepath.Join(t.TempDir(), "events.jsonl")
	hourglasstest.TestHook(t, func() hourglass.Hook { return newHook(t, path) })
}

func TestEXT5_LogfileAppendsOneJSONLinePerEvent(t *testing.T) {
	path := filepath.Join(t.TempDir(), "events.jsonl")
	h := newHook(t, path)
	at := time.Date(2030, 1, 1, 12, 0, 0, 0, time.UTC)
	ev := hourglass.Event{
		Type: hourglass.EventHoldAcquired, Time: at,
		User:  &hourglass.User{Username: "ada"},
		Setup: &hourglass.Setup{Name: "rig-1", Labels: map[string]string{"site": "east"}},
		Hold:  &hourglass.Hold{ID: 7},
	}
	for i := 0; i < 2; i++ {
		if err := h.Handle(context.Background(), ev); err != nil {
			t.Fatal(err)
		}
	}
	lines := readLines(t, path)
	if len(lines) != 2 {
		t.Fatalf("%d lines, want 2", len(lines))
	}
	l := lines[0]
	if l["type"] != "hold.acquired" || l["user"] != "ada" || l["setup"] != "rig-1" || l["hold"] != float64(7) {
		t.Errorf("line: %v", l)
	}
}

func TestEXT5_LogfileEventFilter(t *testing.T) {
	path := filepath.Join(t.TempDir(), "events.jsonl")
	h := newHook(t, path, hourglass.EventHoldReleased)
	for _, typ := range []string{hourglass.EventHoldAcquired, hourglass.EventHoldReleased, hourglass.EventPollFailed} {
		if err := h.Handle(context.Background(), hourglass.Event{Type: typ, Time: time.Now()}); err != nil {
			t.Fatal(err)
		}
	}
	lines := readLines(t, path)
	if len(lines) != 1 || lines[0]["type"] != hourglass.EventHoldReleased {
		t.Fatalf("lines: %v", lines)
	}
}

func TestEXT5_LogfileVetoesMaintenanceAcquire(t *testing.T) {
	path := filepath.Join(t.TempDir(), "events.jsonl")
	h := newHook(t, path)
	acquiring := func(labels map[string]string) error {
		return h.Handle(context.Background(), hourglass.Event{
			Type: hourglass.EventHoldAcquiring, Time: time.Now(),
			Setup: &hourglass.Setup{Name: "rig-1", Labels: labels},
		})
	}
	err := acquiring(map[string]string{"maintenance": "true"})
	if err == nil || !strings.Contains(err.Error(), "rig-1 is under maintenance") {
		t.Fatalf("want a veto naming the rig, got %v", err)
	}
	if got := readLines(t, path); len(got) != 0 {
		t.Errorf("a vetoed acquire must not be logged as if it went ahead: %v", got)
	}
	if err := acquiring(map[string]string{"maintenance": "false"}); err != nil {
		t.Errorf("maintenance=false: %v", err)
	}
	// Only hold.acquiring is vetoable: other events about the same setup pass.
	err = h.Handle(context.Background(), hourglass.Event{
		Type: hourglass.EventHoldReleased, Time: time.Now(),
		Setup: &hourglass.Setup{Name: "rig-1", Labels: map[string]string{"maintenance": "true"}},
	})
	if err != nil {
		t.Errorf("hold.released must not be vetoed: %v", err)
	}
}

func TestEXT5_LogfileVetoIgnoresTheEventFilter(t *testing.T) {
	h := newHook(t, filepath.Join(t.TempDir(), "e.jsonl"), hourglass.EventHoldReleased)
	err := h.Handle(context.Background(), hourglass.Event{
		Type:  hourglass.EventHoldAcquiring,
		Setup: &hourglass.Setup{Name: "rig-1", Labels: map[string]string{"maintenance": "true"}},
	})
	if err == nil {
		t.Fatal("the veto must apply even when hold.acquiring is not in the log filter")
	}
}

func TestEXT5_LogfileRejectsBadConfig(t *testing.T) {
	for _, raw := range []string{``, `{}`, `{"path":"x","events":["hold.nope"]}`, `{"path":"x","extra":1}`} {
		if _, err := New(json.RawMessage(raw)); err == nil {
			t.Errorf("config %q: want an error", raw)
		}
	}
}
