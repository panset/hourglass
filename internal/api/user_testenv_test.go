// SPDX-License-Identifier: MIT

package api

import (
	"context"
	"encoding/json"
	"math"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/panset/hourglass/internal/store"
	"github.com/panset/hourglass/pkg/hourglass"
)

var uBg = context.Background()

// uenv is the environment of the user-route tests: the backend with a clock,
// a metering policy that always meters, and the pressure and policy names set.
type uenv struct{ *backend }

func newUserEnv(t *testing.T, start time.Time, loc *time.Location) *uenv {
	t.Helper()
	b := newBackend(t, backendOpts{start: start, loc: loc,
		runtime: Runtime{PressureSource: "fake-pressure", MeteringPolicy: "fake-policy"}})
	b.policy.SetDefault(hourglass.Decision{Meter: true, Reason: "test reason"})
	return &uenv{b}
}

func defaultEnv(t *testing.T) *uenv {
	return newUserEnv(t, time.Date(2026, 11, 19, 10, 0, 0, 0, time.UTC), nil)
}

// poll runs one collector poll at the current fake time.
func (e *uenv) poll() {
	e.t.Helper()
	if err := e.col.RunOnce(uBg); err != nil {
		e.t.Fatalf("poll: %v", err)
	}
}

// step advances the clock by d and polls.
func (e *uenv) step(d time.Duration) {
	e.t.Helper()
	e.clk.Advance(d)
	e.poll()
}

// dev creates an active user with the roles, their first allowance and a session.
func (e *uenv) dev(name string, roles ...string) usess {
	return e.newUser(name, store.StatusActive, roles...)
}

// pending is an active user with no role (AUTH-4).
func (e *uenv) pending(name string) usess { return e.dev(name) }

func (e *uenv) adminSess() usess { return e.session(e.admin) }

func (e *uenv) setup(name string, enabled bool, labels map[string]string) int64 {
	e.t.Helper()
	id, err := e.st.CreateSetup(uBg, store.Setup{Name: name, Enabled: enabled, Labels: labels, DetailsJSON: `{"note":"opaque","n":[1,2]}`})
	if err != nil {
		e.t.Fatal(err)
	}
	return id
}

func (e *uenv) setting(key, valueJSON string) {
	e.t.Helper()
	if err := e.st.PutSetting(uBg, key, valueJSON, nil, e.clk.Now()); err != nil {
		e.t.Fatal(err)
	}
}

func (e *uenv) rateRule(selector string, weight float64) {
	e.t.Helper()
	if err := e.st.ReplaceRules(uBg, "rate", []store.Rule{{Selector: selector, Value: weight}}, nil, e.clk.Now()); err != nil {
		e.t.Fatal(err)
	}
}

func (e *uenv) do(method, path string, s *usess, csrf bool, body string) *httptest.ResponseRecorder {
	e.t.Helper()
	return e.serve(method, path, s, csrf, body)
}

func (e *uenv) get(path string, s *usess) *httptest.ResponseRecorder {
	return e.do(http.MethodGet, path, s, false, "")
}

func (e *uenv) post(path string, s *usess, body string) *httptest.ResponseRecorder {
	return e.do(http.MethodPost, path, s, true, body)
}

// acquire acquires the setup for the user and returns the new hold's id.
func (e *uenv) acquire(s *usess, setupID int64) int64 {
	e.t.Helper()
	rec := e.post("/api/v1/setups/"+uItoa(setupID)+"/acquire", s, "")
	if rec.Code != http.StatusCreated {
		e.t.Fatalf("acquire: %d %s", rec.Code, rec.Body)
	}
	return int64(uObj(e.t, rec)["id"].(float64))
}

type ujobj = map[string]any

func uItoa(n int64) string { b, _ := json.Marshal(n); return string(b) }

// obj decodes a JSON object response.
func uObj(t *testing.T, rec *httptest.ResponseRecorder) ujobj {
	t.Helper()
	var m ujobj
	if err := json.Unmarshal(rec.Body.Bytes(), &m); err != nil {
		t.Fatalf("response is not a JSON object: %v: %s", err, rec.Body)
	}
	return m
}

func uErrOf(t *testing.T, rec *httptest.ResponseRecorder) (code, field string) {
	t.Helper()
	var b struct {
		Error struct{ Code, Message, Field string }
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &b); err != nil {
		t.Fatalf("not an error body: %s", rec.Body)
	}
	return b.Error.Code, b.Error.Field
}

func uWantErr(t *testing.T, rec *httptest.ResponseRecorder, status int, code string) {
	t.Helper()
	if rec.Code != status {
		t.Fatalf("status %d, want %d: %s", rec.Code, status, rec.Body)
	}
	if c, _ := uErrOf(t, rec); c != code {
		t.Fatalf("code %q, want %q: %s", c, code, rec.Body)
	}
}

func uWantOK(t *testing.T, rec *httptest.ResponseRecorder, status int) ujobj {
	t.Helper()
	if rec.Code != status {
		t.Fatalf("status %d, want %d: %s", rec.Code, status, rec.Body)
	}
	return uObj(t, rec)
}

// keys returns the sorted-insensitive key set of an object.
func uKeysOf(m ujobj) map[string]bool {
	out := map[string]bool{}
	for k := range m {
		out[k] = true
	}
	return out
}

// wantKeys checks that the object has exactly the keys.
func uWantKeys(t *testing.T, what string, m ujobj, keys ...string) {
	t.Helper()
	got := uKeysOf(m)
	for _, k := range keys {
		if !got[k] {
			t.Errorf("%s: missing field %q", what, k)
		}
		delete(got, k)
	}
	for k := range got {
		t.Errorf("%s: unexpected field %q", what, k)
	}
}

func uNear(a, b float64) bool { return math.Abs(a-b) < 0.0005 }

func uNum(t *testing.T, m ujobj, k string) float64 {
	t.Helper()
	f, ok := m[k].(float64)
	if !ok {
		t.Fatalf("field %q = %v, want a number", k, m[k])
	}
	return f
}
