// SPDX-License-Identifier: MIT

package api

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/panset/hourglass/internal/store"
	"github.com/panset/hourglass/pkg/hourglass"
)

// adEnv is the environment of the admin-route tests: the backend with the
// event worker running, the extension names set, and an admin session.
type adEnv struct {
	*backend
	adminS adSess
}

func newAdEnv(t *testing.T) *adEnv {
	t.Helper()
	b := newBackend(t, backendOpts{start: time.Date(2026, 11, 19, 10, 0, 0, 0, time.UTC), startEvents: true,
		runtime: Runtime{Provider: "fake-provider", PressureSource: "fake-pressure", MeteringPolicy: "fake-policy", Hooks: []string{"fake-hook"}}})
	return &adEnv{backend: b, adminS: b.session(b.admin)}
}

func (e *adEnv) user(name, status string, roles ...string) adSess {
	return e.newUser(name, status, roles...)
}

func (e *adEnv) dev(name string) adSess { return e.user(name, store.StatusActive, store.RoleDeveloper) }

func (e *adEnv) setup(name string, enabled bool, labels map[string]string) int64 {
	e.t.Helper()
	id, err := e.st.CreateSetup(uBg, store.Setup{Name: name, Enabled: enabled, Labels: labels, DetailsJSON: `{"k":1}`})
	if err != nil {
		e.t.Fatal(err)
	}
	return id
}

func (e *adEnv) hold(setupID, userID int64) store.Hold {
	e.t.Helper()
	h, err := e.st.CreateHold(uBg, setupID, userID, e.clk.Now())
	if err != nil {
		e.t.Fatal(err)
	}
	return h
}

func (e *adEnv) do(method, path string, s *adSess, csrf bool, body string) *httptest.ResponseRecorder {
	e.t.Helper()
	return e.serve(method, path, s, csrf, body)
}

// get, send: requests as the admin.
func (e *adEnv) get(path string) *httptest.ResponseRecorder {
	return e.do(http.MethodGet, path, &e.adminS, false, "")
}

func (e *adEnv) send(method, path, body string) *httptest.ResponseRecorder {
	return e.do(method, path, &e.adminS, true, body)
}

// adErr asserts an error response and returns its field.
func adErr(t *testing.T, rec *httptest.ResponseRecorder, status int, code string) (field, message string) {
	t.Helper()
	if rec.Code != status {
		t.Fatalf("status = %d, want %d: %s", rec.Code, status, rec.Body)
	}
	b := uObj(t, rec)
	e, _ := b["error"].(ujobj)
	if len(b) != 1 || e == nil || e["code"] != code {
		t.Fatalf("error body = %s, want code %q", rec.Body, code)
	}
	f, _ := e["field"].(string)
	m, _ := e["message"].(string)
	return f, m
}

func adArr(t *testing.T, m ujobj, k string) []any {
	t.Helper()
	a, ok := m[k].([]any)
	if !ok {
		t.Fatalf("%q is not an array: %v", k, m[k])
	}
	return a
}

func adObjOf(t *testing.T, v any) ujobj {
	t.Helper()
	m, ok := v.(ujobj)
	if !ok {
		t.Fatalf("not an object: %v", v)
	}
	return m
}

// audits returns the audit rows (newest first) of an action.
func (e *adEnv) audits(action string) []store.AuditEntry {
	e.t.Helper()
	list, err := e.st.ListAudit(uBg, store.AuditFilter{Action: action, Limit: 100})
	if err != nil {
		e.t.Fatal(err)
	}
	return list
}

func (e *adEnv) flushEvents() {
	e.t.Helper()
	ctx, cancel := context.WithTimeout(uBg, 5*time.Second)
	defer cancel()
	if err := e.ev.Flush(ctx); err != nil {
		e.t.Fatal(err)
	}
}

func hourglassAlloc(name string, labelPairs ...string) hourglass.Allocation {
	l := map[string]string{}
	for i := 0; i+1 < len(labelPairs); i += 2 {
		l[labelPairs[i]] = labelPairs[i+1]
	}
	return hourglass.Allocation{ResourceName: name, Labels: l}
}

func adBytes(v any) []byte { b, _ := json.Marshal(v); return b }
