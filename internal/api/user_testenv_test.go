// SPDX-License-Identifier: MIT

package api

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"io"
	"log/slog"
	"math"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/panset/hourglass/internal/auth"
	"github.com/panset/hourglass/internal/collector"
	"github.com/panset/hourglass/internal/events"
	"github.com/panset/hourglass/internal/holds"
	"github.com/panset/hourglass/internal/ledger"
	"github.com/panset/hourglass/internal/store"
	"github.com/panset/hourglass/pkg/hourglass"
	"github.com/panset/hourglass/pkg/hourglasstest"
)

var uBg = context.Background()

// uenv is a full backend (real store, ledger, holds, collector and router)
// with fakes only for the extensions and the clock.
type uenv struct {
	t        *testing.T
	st       *store.Store
	clk      *hourglasstest.FakeClock
	auth     *auth.Service
	led      *ledger.Service
	col      *collector.Collector
	prov     *hourglasstest.FakeProvider
	pressure *hourglasstest.FakePressureSource
	policy   *hourglasstest.FakePolicy
	hook     *hourglasstest.FakeHook
	h        http.Handler
	admin    store.User
}

func newUserEnv(t *testing.T, start time.Time, loc *time.Location) *uenv {
	t.Helper()
	if loc == nil {
		loc = time.UTC
	}
	st, err := store.Open(filepath.Join(t.TempDir(), "user.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { st.Close() })
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	svc, err := auth.New(auth.Config{SessionKey: strings.Repeat("k", 40)}, st, nil, log)
	if err != nil {
		t.Fatal(err)
	}
	admin, err := auth.EnsureLocalAdmin(uBg, st, "correct horse battery staple", log)
	if err != nil {
		t.Fatal(err)
	}
	if err := st.SetPassword(uBg, admin.ID, admin.PasswordHash, false); err != nil {
		t.Fatal(err)
	}
	e := &uenv{t: t, st: st, clk: hourglasstest.NewFakeClock(start), auth: svc, admin: admin,
		prov: hourglasstest.NewFakeProvider(), pressure: hourglasstest.NewFakePressureSource(),
		policy: hourglasstest.NewFakePolicy(), hook: hourglasstest.NewFakeHook()}
	e.pressure.SetValue(0.8) // 2x with the default bands
	e.policy.SetDefault(hourglass.Decision{Meter: true, Reason: "test reason"})
	ev := events.New([]hourglass.Hook{e.hook}, st, events.WithTimeout(time.Second))
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(uBg, 5*time.Second)
		defer cancel()
		_ = ev.Close(ctx)
	})
	e.led = ledger.NewService(st, ev, e.clk, loc)
	e.col = collector.New(collector.Deps{Store: st, Provider: e.prov, Pressure: e.pressure, Policy: e.policy,
		Events: ev, Ledger: e.led, Clock: e.clk, Logger: log})
	hs := holds.NewService(holds.Deps{Store: st, Events: ev, ProviderConfigured: true, Clock: e.clk, Logger: log,
		Poll: func() holds.PollView { return e.col.LastPoll().ForHolds() }})
	e.h = New(Deps{Store: st, Auth: svc, Holds: hs, Ledger: e.led, Events: ev, Collector: e.col, Clock: e.clk,
		Location: loc, Logger: log, Runtime: Runtime{PressureSource: "fake-pressure", MeteringPolicy: "fake-policy"}})
	return e
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

type usess struct {
	user         store.User
	cookie, csrf string
}

// dev creates an active user with the roles, their first allowance and a session.
func (e *uenv) dev(name string, roles ...string) usess {
	e.t.Helper()
	u, err := e.st.CreateOIDCUser(uBg, name, name+"@example.test", "User "+name, e.clk.Now())
	if err != nil {
		e.t.Fatal(err)
	}
	if err := e.st.SetUserStatus(uBg, u.ID, store.StatusActive); err != nil {
		e.t.Fatal(err)
	}
	if len(roles) > 0 {
		if err := e.st.SetUserRoles(uBg, u.ID, roles); err != nil {
			e.t.Fatal(err)
		}
		err = e.st.Tx(uBg, func(tx *store.Tx) error { return e.led.EnsureFirstAllowanceTx(tx, u.ID, e.clk.Now()) })
		if err != nil {
			e.t.Fatal(err)
		}
	}
	return e.session(u)
}

// pending is an active user with no role (AUTH-4).
func (e *uenv) pending(name string) usess { return e.dev(name) }

func (e *uenv) session(u store.User) usess {
	e.t.Helper()
	raw := make([]byte, 32)
	if _, err := rand.Read(raw); err != nil {
		e.t.Fatal(err)
	}
	sum := sha256.Sum256(raw)
	id := hex.EncodeToString(sum[:])
	now := time.Now()
	if err := e.st.CreateSession(uBg, id, u.ID, now, now.Add(time.Hour)); err != nil {
		e.t.Fatal(err)
	}
	return usess{user: u, cookie: base64.RawURLEncoding.EncodeToString(raw), csrf: e.auth.CSRFToken(id)}
}

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
	var rd io.Reader
	if body != "" {
		rd = strings.NewReader(body)
	}
	req := httptest.NewRequest(method, path, rd)
	if body != "" {
		req.Header.Set("Content-Type", "application/json")
	}
	if s != nil {
		req.Header.Set("Cookie", auth.SessionCookie+"="+s.cookie)
		if csrf {
			req.Header.Set(auth.CSRFHeader, s.csrf)
		}
	}
	rec := httptest.NewRecorder()
	e.h.ServeHTTP(rec, req)
	return rec
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
