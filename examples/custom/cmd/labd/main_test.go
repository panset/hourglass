// SPDX-License-Identifier: MIT

package main

import (
	"bufio"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

// syncBuf lets the test read the log while Run is still writing it.
type syncBuf struct {
	mu sync.Mutex
	b  strings.Builder
}

func (s *syncBuf) Write(p []byte) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.b.Write(p)
}

func (s *syncBuf) String() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.b.String()
}

// listenAddr waits for the "http listening" log record and returns its addr.
func (s *syncBuf) listenAddr(t *testing.T) string {
	t.Helper()
	deadline := time.Now().Add(15 * time.Second)
	for time.Now().Before(deadline) {
		sc := bufio.NewScanner(strings.NewReader(s.String()))
		for sc.Scan() {
			var m map[string]any
			if json.Unmarshal(sc.Bytes(), &m) == nil && m["msg"] == "http listening" {
				return m["addr"].(string)
			}
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("server never logged \"http listening\"; log:\n%s", s.String())
	return ""
}

// TestEXT5_LabdStartsThroughRunWithTheExampleConfig starts labd's own wiring
// in-process, through hourglass.Run, with the example config and seed
// copied into a temp directory, and checks /healthz. It proves the whole
// third-party path: public packages only, four custom extensions selected
// by name from YAML, seed imported, server up.
func TestEXT5_LabdStartsThroughRunWithTheExampleConfig(t *testing.T) {
	work := t.TempDir()
	for _, f := range []string{"config.yaml", "seed.yaml"} {
		data, err := os.ReadFile(filepath.Join("..", "..", f))
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(work, f), data, 0o600); err != nil {
			t.Fatal(err)
		}
	}
	// The config's relative paths (data/rigs, data/labd.db) resolve here.
	if err := os.MkdirAll(filepath.Join(work, "data", "rigs"), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(work, "data", "rigs", "rig-a1"), nil, 0o600); err != nil {
		t.Fatal(err)
	}
	t.Chdir(work)

	// Secrets come from the environment, as the config file says. Port 0
	// picks a free port; the log reports the one chosen.
	t.Setenv("HOURGLASS_LISTEN", "127.0.0.1:0")
	t.Setenv("HOURGLASS_SESSION_KEY", strings.Repeat("k", 40))
	t.Setenv("HOURGLASS_ADMIN_PASSWORD", "a-long-test-only-password")

	out := &syncBuf{}
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- run(ctx, []string{"-config", "config.yaml"}, out, io.Discard) }()
	t.Cleanup(func() { cancel(); <-done })

	base := "http://" + out.listenAddr(t)
	resp, err := http.Get(base + "/healthz")
	if err != nil {
		t.Fatal(err)
	}
	body, _ := io.ReadAll(resp.Body)
	resp.Body.Close()
	if resp.StatusCode != 200 || !strings.Contains(string(body), `"database":"ok"`) {
		t.Fatalf("healthz: %d %s", resp.StatusCode, body)
	}

	// Once a poll succeeds, the filedir provider and the other extensions
	// built from the config ran without error.
	deadline := time.Now().Add(15 * time.Second)
	for {
		resp, err := http.Get(base + "/healthz")
		if err != nil {
			t.Fatal(err)
		}
		b, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		if !strings.Contains(string(b), `"last_poll_age_seconds":null`) {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("no successful poll within 15s: %s\nlog:\n%s", b, out.String())
		}
		time.Sleep(20 * time.Millisecond)
	}

	cancel()
	select {
	case err := <-done:
		done <- err
		if err != nil {
			t.Fatalf("Run returned %v after cancel", err)
		}
	case <-time.After(20 * time.Second):
		t.Fatal("Run did not return after cancel")
	}
}

// TestEXT5_LabdFailsOnAnUnknownExtensionName shows that selecting an
// extension that was not linked in fails startup with the names that were.
func TestEXT5_LabdFailsOnAnUnknownExtensionName(t *testing.T) {
	work := t.TempDir()
	cfg := "listen: 127.0.0.1:0\ndatabase: " + filepath.Join(work, "h.db") + "\npressure:\n  type: no-such-source\n"
	path := filepath.Join(work, "c.yaml")
	if err := os.WriteFile(path, []byte(cfg), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("HOURGLASS_SESSION_KEY", strings.Repeat("k", 40))
	t.Setenv("HOURGLASS_ADMIN_PASSWORD", "a-long-test-only-password")
	err := run(context.Background(), []string{"-config", path}, io.Discard, io.Discard)
	if err == nil || !strings.Contains(err.Error(), "busiest-site") {
		t.Fatalf("want an error listing registered names including busiest-site, got %v", err)
	}
}
