// SPDX-License-Identifier: MIT

package webhook

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/panset/hourglass/pkg/hourglass"
	"github.com/panset/hourglass/pkg/hourglasstest"
)

type captured struct {
	header http.Header
	body   []byte
}

type recorder struct {
	mu   sync.Mutex
	reqs []captured
}

func (r *recorder) handler(status int, reply string) http.HandlerFunc {
	return func(w http.ResponseWriter, req *http.Request) {
		b, _ := io.ReadAll(req.Body)
		r.mu.Lock()
		r.reqs = append(r.reqs, captured{req.Header.Clone(), b})
		r.mu.Unlock()
		w.WriteHeader(status)
		_, _ = io.WriteString(w, reply)
	}
}

func (r *recorder) count() int { r.mu.Lock(); defer r.mu.Unlock(); return len(r.reqs) }

func newHook(t *testing.T, cfg map[string]any) hourglass.Hook {
	t.Helper()
	raw, _ := json.Marshal(cfg)
	h, err := hourglass.NewHook(Name, raw)
	if err != nil {
		t.Fatal(err)
	}
	return h
}

var ts = time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC)

func TestEXT5_Conformance(t *testing.T) {
	srv := httptest.NewServer((&recorder{}).handler(200, ""))
	defer srv.Close()
	hourglasstest.TestHook(t, func() hourglass.Hook { return newHook(t, map[string]any{"url": srv.URL}) })
}

func TestEXT3_FactoryConfig(t *testing.T) {
	cases := []struct {
		name, cfg string
		ok        bool
	}{
		{"valid", `{"url":"http://x.test/h"}`, true},
		{"valid full", `{"url":"https://x.test/h","secret":"s","types":["hold.acquired"]}`, true},
		{"empty types list", `{"url":"http://x.test","types":[]}`, true},
		{"empty", ``, false},
		{"empty object", `{}`, false},
		{"unknown field", `{"url":"http://x.test","extra":1}`, false},
		{"malformed", `{"url":`, false},
		{"ftp", `{"url":"ftp://x.test"}`, false},
		{"no host", `{"url":"http://"}`, false},
		{"relative", `{"url":"/hook"}`, false},
		{"hold.releasing type", `{"url":"http://x.test","types":["hold.releasing"]}`, true},
		{"unknown type", `{"url":"http://x.test","types":["hold.nope"]}`, false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			_, err := New(json.RawMessage(c.cfg))
			if (err == nil) != c.ok {
				t.Fatalf("err = %v, want ok=%v", err, c.ok)
			}
		})
	}
}

func TestEXT6_PayloadAndHeaders(t *testing.T) {
	rec := &recorder{}
	srv := httptest.NewServer(rec.handler(204, ""))
	defer srv.Close()
	h := newHook(t, map[string]any{"url": srv.URL, "secret": "s3cret"})
	ev := hourglass.Event{Type: hourglass.EventHoldAcquired, Time: ts,
		User:  &hourglass.User{ID: 1, Username: "ada", Roles: []string{"developer"}},
		Setup: &hourglass.Setup{Name: "rig", Labels: map[string]string{"k": "v"}},
		Hold:  &hourglass.Hold{ID: 2, SetupName: "rig", UserID: 1, AcquiredAt: ts}}
	if err := h.Handle(context.Background(), ev); err != nil {
		t.Fatal(err)
	}
	if rec.count() != 1 {
		t.Fatalf("requests = %d", rec.count())
	}
	got := rec.reqs[0]
	if ct := got.header.Get("Content-Type"); ct != "application/json" {
		t.Errorf("content-type %q", ct)
	}
	if et := got.header.Get("X-Hourglass-Event"); et != "hold.acquired" {
		t.Errorf("event header %q", et)
	}
	var back hourglass.Event
	if err := json.Unmarshal(got.body, &back); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(back, ev) {
		t.Errorf("payload mismatch: %+v", back)
	}
	mac := hmac.New(sha256.New, []byte("s3cret"))
	mac.Write(got.body)
	want := "sha256=" + hex.EncodeToString(mac.Sum(nil))
	if sig := got.header.Get("X-Hourglass-Signature"); sig != want {
		t.Errorf("signature %q, want %q", sig, want)
	}
}

func TestEXT6_NoSecretNoSignature(t *testing.T) {
	rec := &recorder{}
	srv := httptest.NewServer(rec.handler(200, ""))
	defer srv.Close()
	if err := newHook(t, map[string]any{"url": srv.URL}).Handle(context.Background(), hourglass.Event{Type: hourglass.EventPollFailed, Time: ts}); err != nil {
		t.Fatal(err)
	}
	if _, ok := rec.reqs[0].header["X-Hourglass-Signature"]; ok {
		t.Error("signature header present without a secret")
	}
}

func TestEXT6_TypeFilter(t *testing.T) {
	rec := &recorder{}
	srv := httptest.NewServer(rec.handler(200, ""))
	defer srv.Close()
	filtered := newHook(t, map[string]any{"url": srv.URL, "types": []string{"hold.released"}})
	all := newHook(t, map[string]any{"url": srv.URL})
	for _, typ := range hourglass.EventTypes() {
		_ = filtered.Handle(context.Background(), hourglass.Event{Type: typ, Time: ts})
	}
	if rec.count() != 1 || rec.reqs[0].header.Get("X-Hourglass-Event") != "hold.released" {
		t.Fatalf("filtered hook sent %d requests", rec.count())
	}
	before := rec.count()
	for _, typ := range hourglass.EventTypes() {
		_ = all.Handle(context.Background(), hourglass.Event{Type: typ, Time: ts})
	}
	if got := rec.count() - before; got != len(hourglass.EventTypes()) {
		t.Fatalf("default hook sent %d, want %d", got, len(hourglass.EventTypes()))
	}
}

func TestEXT6_NonSuccessStatus(t *testing.T) {
	long := strings.Repeat("x", 500)
	cases := []struct {
		name   string
		status int
		reply  string
		want   []string
		maxLen int
	}{
		{"veto message", 403, "quota exceeded", []string{"403", "quota exceeded"}, 0},
		{"server error", 500, "boom", []string{"500", "boom"}, 0},
		{"empty body", 502, "", []string{"502"}, 0},
		{"long body truncated", 400, long, []string{"400", strings.Repeat("x", 200)}, 0},
		{"1xx-3xx not success", 301, "moved", []string{"301"}, 0},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			srv := httptest.NewServer((&recorder{}).handler(c.status, c.reply))
			defer srv.Close()
			err := newHook(t, map[string]any{"url": srv.URL}).Handle(context.Background(), hourglass.Event{Type: hourglass.EventHoldAcquiring, Time: ts})
			if err == nil {
				t.Fatal("want error")
			}
			for _, w := range c.want {
				if !strings.Contains(err.Error(), w) {
					t.Errorf("error %q lacks %q", err, w)
				}
			}
			if strings.Contains(err.Error(), strings.Repeat("x", 201)) {
				t.Error("body not truncated to 200 bytes")
			}
		})
	}
}

func TestEXT6_RedirectNotFollowed(t *testing.T) {
	target := &recorder{}
	tsrv := httptest.NewServer(target.handler(200, ""))
	defer tsrv.Close()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, tsrv.URL, http.StatusTemporaryRedirect)
	}))
	defer srv.Close()
	err := newHook(t, map[string]any{"url": srv.URL}).Handle(context.Background(), hourglass.Event{Type: hourglass.EventHoldAcquired, Time: ts})
	if err == nil || !strings.Contains(err.Error(), "307") {
		t.Fatalf("err = %v", err)
	}
	if target.count() != 0 {
		t.Error("redirect was followed")
	}
}

func TestEXT6_ContextDeadline(t *testing.T) {
	release := make(chan struct{})
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		select {
		case <-release:
		case <-r.Context().Done():
		}
	}))
	defer srv.Close()
	defer close(release)
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()
	start := time.Now()
	err := newHook(t, map[string]any{"url": srv.URL}).Handle(ctx, hourglass.Event{Type: hourglass.EventHoldAcquired, Time: ts})
	if err == nil {
		t.Fatal("want error")
	}
	if d := time.Since(start); d > 2*time.Second {
		t.Errorf("took %v", d)
	}
}

func TestEXT6_SecretNotInErrors(t *testing.T) {
	const secret = "TOPSECRET-value-123"
	// Non-2xx whose body echoes the secret.
	srv := httptest.NewServer((&recorder{}).handler(500, "bad "+secret))
	err := newHook(t, map[string]any{"url": srv.URL, "secret": secret}).Handle(context.Background(), hourglass.Event{Type: hourglass.EventHoldAcquired, Time: ts})
	srv.Close()
	if err == nil || strings.Contains(err.Error(), secret) {
		t.Errorf("status error leaks secret: %v", err)
	}
	// Transport error (closed server) and an unreachable URL with userinfo.
	err = newHook(t, map[string]any{"url": srv.URL, "secret": secret}).Handle(context.Background(), hourglass.Event{Type: hourglass.EventHoldAcquired, Time: ts})
	if err == nil || strings.Contains(err.Error(), secret) {
		t.Errorf("transport error leaks secret: %v", err)
	}
	h := newHook(t, map[string]any{"url": "http://user:" + secret + "@127.0.0.1:1/x", "secret": secret})
	if err := h.Handle(context.Background(), hourglass.Event{Type: hourglass.EventHoldAcquired, Time: ts}); err == nil || strings.Contains(err.Error(), secret) {
		t.Errorf("url userinfo leaks: %v", err)
	}
	// Construction errors.
	if _, err := New(json.RawMessage(`{"url":"ftp://u:` + secret + `@x","secret":"` + secret + `"}`)); err == nil || strings.Contains(err.Error(), secret) {
		t.Errorf("config error leaks: %v", err)
	}
}

func TestEXT6_SecretStraddlingCutIsRedacted(t *testing.T) {
	const secret = "s3cr3t-straddle-value"
	ev := hourglass.Event{Type: hourglass.EventHoldAcquired, Time: ts}
	// Place the secret so it starts before byte 200 and ends after it, at every offset.
	for off := 200 - len(secret) + 1; off < 200; off++ {
		body := strings.Repeat("x", off) + secret + strings.Repeat("y", 50)
		srv := httptest.NewServer((&recorder{}).handler(500, body))
		err := newHook(t, map[string]any{"url": srv.URL, "secret": secret}).Handle(context.Background(), ev)
		srv.Close()
		if err == nil {
			t.Fatal("want error")
		}
		msg := err.Error()
		for n := 3; n <= len(secret); n++ { // no prefix of 3+ bytes of the secret may appear
			if strings.Contains(msg, secret[:n]) {
				t.Fatalf("offset %d: leak of %q in %q", off, secret[:n], msg)
			}
		}
	}
}

func TestEXT6_TransportErrorsAreGeneric(t *testing.T) {
	ev := hourglass.Event{Type: hourglass.EventHoldAcquired, Time: ts}
	// Connection refused: start and close a server to get a dead port.
	srv := httptest.NewServer(http.NotFoundHandler())
	hostport := strings.TrimPrefix(srv.URL, "http://")
	srv.Close()
	err := newHook(t, map[string]any{"url": srv.URL + "/private/path?token=abc"}).Handle(context.Background(), ev)
	if err == nil {
		t.Fatal("want error")
	}
	if strings.Contains(err.Error(), hostport) || strings.Contains(err.Error(), "127.0.0.1") ||
		strings.Contains(err.Error(), "private") || !strings.Contains(err.Error(), "connection failed") {
		t.Errorf("refused: %v", err)
	}
	// Timeout via context deadline.
	release := make(chan struct{})
	slow := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { <-release }))
	defer slow.Close()
	defer close(release)
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()
	err = newHook(t, map[string]any{"url": slow.URL}).Handle(ctx, ev)
	if err == nil || !strings.Contains(err.Error(), "timeout") || strings.Contains(err.Error(), "127.0.0.1") {
		t.Errorf("timeout: %v", err)
	}
	// Status path unchanged.
	bad := httptest.NewServer((&recorder{}).handler(422, "nope"))
	defer bad.Close()
	err = newHook(t, map[string]any{"url": bad.URL}).Handle(context.Background(), ev)
	if err == nil || !strings.Contains(err.Error(), "status 422: nope") {
		t.Errorf("status: %v", err)
	}
}
