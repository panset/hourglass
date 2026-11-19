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

var adBg = context.Background()

// adEnv is a full backend for the admin handler tests: real store, ledger,
// holds, collector, events and router; fakes for the extensions and the clock.
type adEnv struct {
	t        *testing.T
	st       *store.Store
	clk      *hourglasstest.FakeClock
	auth     *auth.Service
	led      *ledger.Service
	col      *collector.Collector
	ev       *events.Dispatcher
	prov     *hourglasstest.FakeProvider
	pressure *hourglasstest.FakePressureSource
	hook     *hourglasstest.FakeHook
	h        http.Handler
	admin    store.User
	adminS   adSess
}

func newAdEnv(t *testing.T) *adEnv {
	t.Helper()
	st, err := store.Open(filepath.Join(t.TempDir(), "admin.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { st.Close() })
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	svc, err := auth.New(auth.Config{SessionKey: strings.Repeat("k", 40)}, st, nil, log)
	if err != nil {
		t.Fatal(err)
	}
	admin, err := auth.EnsureLocalAdmin(adBg, st, "correct horse battery staple", log)
	if err != nil {
		t.Fatal(err)
	}
	if err := st.SetPassword(adBg, admin.ID, admin.PasswordHash, false); err != nil {
		t.Fatal(err)
	}
	e := &adEnv{t: t, st: st, auth: svc, admin: admin, clk: hourglasstest.NewFakeClock(time.Date(2026, 11, 19, 10, 0, 0, 0, time.UTC)),
		prov: hourglasstest.NewFakeProvider(), pressure: hourglasstest.NewFakePressureSource(), hook: hourglasstest.NewFakeHook()}
	e.pressure.SetValue(0.8) // 2x with the default bands
	e.ev = events.New([]hourglass.Hook{e.hook}, st, events.WithTimeout(time.Second))
	e.ev.Start(adBg)
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(adBg, 5*time.Second)
		defer cancel()
		_ = e.ev.Close(ctx)
	})
	e.led = ledger.NewService(st, e.ev, e.clk, time.UTC)
	e.col = collector.New(collector.Deps{Store: st, Provider: e.prov, Pressure: e.pressure,
		Policy: hourglasstest.NewFakePolicy(), Events: e.ev, Ledger: e.led, Clock: e.clk, Logger: log})
	hs := holds.NewService(holds.Deps{Store: st, Events: e.ev, ProviderConfigured: true, Clock: e.clk, Logger: log,
		Poll: func() holds.PollView { return e.col.LastPoll().ForHolds() }})
	e.h = New(Deps{Store: st, Auth: svc, Holds: hs, Ledger: e.led, Events: e.ev, Collector: e.col, Clock: e.clk,
		Location: time.UTC, Logger: log,
		Runtime: Runtime{Provider: "fake-provider", PressureSource: "fake-pressure", MeteringPolicy: "fake-policy", Hooks: []string{"fake-hook"}}})
	e.adminS = e.session(admin)
	return e
}

type adSess struct {
	user         store.User
	cookie, csrf string
}

func (e *adEnv) session(u store.User) adSess {
	e.t.Helper()
	raw := make([]byte, 32)
	if _, err := rand.Read(raw); err != nil {
		e.t.Fatal(err)
	}
	sum := sha256.Sum256(raw)
	id := hex.EncodeToString(sum[:])
	now := time.Now()
	if err := e.st.CreateSession(adBg, id, u.ID, now, now.Add(time.Hour)); err != nil {
		e.t.Fatal(err)
	}
	return adSess{user: u, cookie: base64.RawURLEncoding.EncodeToString(raw), csrf: e.auth.CSRFToken(id)}
}

// user creates an OIDC user. Status "pending" leaves the new-user default;
// any other status is set. Roles are stored with the first allowance, as an
// approval would.
func (e *adEnv) user(name, status string, roles ...string) adSess {
	e.t.Helper()
	u, err := e.st.CreateOIDCUser(adBg, name, name+"@example.test", "User "+name, e.clk.Now())
	if err != nil {
		e.t.Fatal(err)
	}
	if status != store.StatusPending {
		if err := e.st.SetUserStatus(adBg, u.ID, status); err != nil {
			e.t.Fatal(err)
		}
	}
	if len(roles) > 0 {
		if err := e.st.SetUserRoles(adBg, u.ID, roles); err != nil {
			e.t.Fatal(err)
		}
		if err := e.st.Tx(adBg, func(tx *store.Tx) error { return e.led.EnsureFirstAllowanceTx(tx, u.ID, e.clk.Now()) }); err != nil {
			e.t.Fatal(err)
		}
	}
	u, _ = e.st.GetUser(adBg, u.ID)
	return e.session(u)
}

func (e *adEnv) dev(name string) adSess { return e.user(name, store.StatusActive, store.RoleDeveloper) }

func (e *adEnv) setup(name string, enabled bool, labels map[string]string) int64 {
	e.t.Helper()
	id, err := e.st.CreateSetup(adBg, store.Setup{Name: name, Enabled: enabled, Labels: labels, DetailsJSON: `{"k":1}`})
	if err != nil {
		e.t.Fatal(err)
	}
	return id
}

func (e *adEnv) hold(setupID, userID int64) store.Hold {
	e.t.Helper()
	h, err := e.st.CreateHold(adBg, setupID, userID, e.clk.Now())
	if err != nil {
		e.t.Fatal(err)
	}
	return h
}

func (e *adEnv) do(method, path string, s *adSess, csrf bool, body string) *httptest.ResponseRecorder {
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

// get, send: requests as the admin.
func (e *adEnv) get(path string) *httptest.ResponseRecorder {
	return e.do(http.MethodGet, path, &e.adminS, false, "")
}

func (e *adEnv) send(method, path, body string) *httptest.ResponseRecorder {
	return e.do(method, path, &e.adminS, true, body)
}

type adObj = map[string]any

func adItoa(n int64) string { b, _ := json.Marshal(n); return string(b) }

func adDecode(t *testing.T, rec *httptest.ResponseRecorder) adObj {
	t.Helper()
	var m adObj
	if err := json.Unmarshal(rec.Body.Bytes(), &m); err != nil {
		t.Fatalf("body is not a JSON object: %v: %s", err, rec.Body)
	}
	return m
}

// adOK asserts the status and returns the decoded object.
func adOK(t *testing.T, rec *httptest.ResponseRecorder, status int) adObj {
	t.Helper()
	if rec.Code != status {
		t.Fatalf("status = %d, want %d: %s", rec.Code, status, rec.Body)
	}
	return adDecode(t, rec)
}

// adErr asserts an error response and returns its field.
func adErr(t *testing.T, rec *httptest.ResponseRecorder, status int, code string) (field, message string) {
	t.Helper()
	if rec.Code != status {
		t.Fatalf("status = %d, want %d: %s", rec.Code, status, rec.Body)
	}
	b := adDecode(t, rec)
	e, _ := b["error"].(adObj)
	if len(b) != 1 || e == nil || e["code"] != code {
		t.Fatalf("error body = %s, want code %q", rec.Body, code)
	}
	f, _ := e["field"].(string)
	m, _ := e["message"].(string)
	return f, m
}

func adKeys(t *testing.T, what string, m adObj, keys ...string) {
	t.Helper()
	if len(m) != len(keys) {
		t.Errorf("%s has %d fields, want %d: %v", what, len(m), len(keys), m)
	}
	for _, k := range keys {
		if _, ok := m[k]; !ok {
			t.Errorf("%s lacks field %q", what, k)
		}
	}
}

func adArr(t *testing.T, m adObj, k string) []any {
	t.Helper()
	a, ok := m[k].([]any)
	if !ok {
		t.Fatalf("%q is not an array: %v", k, m[k])
	}
	return a
}

func adObjOf(t *testing.T, v any) adObj {
	t.Helper()
	m, ok := v.(adObj)
	if !ok {
		t.Fatalf("not an object: %v", v)
	}
	return m
}

func adNum(t *testing.T, m adObj, k string) float64 {
	t.Helper()
	f, ok := m[k].(float64)
	if !ok {
		t.Fatalf("%q is not a number: %v", k, m[k])
	}
	return f
}

// audits returns the audit rows (newest first) of an action.
func (e *adEnv) audits(action string) []store.AuditEntry {
	e.t.Helper()
	list, err := e.st.ListAudit(adBg, store.AuditFilter{Action: action, Limit: 100})
	if err != nil {
		e.t.Fatal(err)
	}
	return list
}

func (e *adEnv) flushEvents() {
	e.t.Helper()
	ctx, cancel := context.WithTimeout(adBg, 5*time.Second)
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
