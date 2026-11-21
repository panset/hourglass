// SPDX-License-Identifier: MIT

package main

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"testing/fstest"

	"github.com/panset/hourglass/internal/servetls/servetlstest"
)

var testUI = fstest.MapFS{
	"index.html":    {Data: []byte("<!doctype html><title>hg</title>")},
	"assets/app.js": {Data: []byte("console.log(1)")},
}

// fakeAPI records the last request and answers by path.
type fakeAPI struct {
	srv  *httptest.Server
	last *http.Request
}

func newFakeAPI(t *testing.T) *fakeAPI {
	t.Helper()
	f := &fakeAPI{}
	f.srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		f.last = r
		switch r.URL.Path {
		case "/auth/oidc/callback":
			http.SetCookie(w, &http.Cookie{Name: "hg_session", Value: "abc", Path: "/", HttpOnly: true, Secure: true, SameSite: http.SameSiteLaxMode})
			http.Redirect(w, r, "http://idp.example.test/next?x=1", http.StatusFound)
		default:
			w.Header().Set("Content-Type", "application/json")
			_, _ = io.WriteString(w, `{"path":"`+r.URL.Path+`"}`)
		}
	}))
	t.Cleanup(f.srv.Close)
	return f
}

func serve(t *testing.T, apiURL string) *httptest.Server {
	t.Helper()
	h, err := newHandler(testUI, apiURL)
	if err != nil {
		t.Fatal(err)
	}
	s := httptest.NewServer(h)
	t.Cleanup(s.Close)
	return s
}

// noRedirect keeps a 302 visible to the test, as a browser sees it before following.
var noRedirect = &http.Client{CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}

func body(t *testing.T, resp *http.Response) string {
	t.Helper()
	defer resp.Body.Close()
	b, _ := io.ReadAll(resp.Body)
	return string(b)
}

func TestUI3_StaticFileAndSPAFallback(t *testing.T) {
	api := newFakeAPI(t)
	s := serve(t, api.srv.URL)
	resp, err := http.Get(s.URL + "/assets/app.js")
	if err != nil {
		t.Fatal(err)
	}
	if b := body(t, resp); resp.StatusCode != 200 || b != "console.log(1)" {
		t.Errorf("static: %d %q", resp.StatusCode, b)
	}
	resp, _ = http.Get(s.URL + "/setups/12")
	if b := body(t, resp); resp.StatusCode != 200 || !strings.Contains(b, "<title>hg</title>") {
		t.Errorf("fallback: %d %q", resp.StatusCode, b)
	}
	if api.last != nil {
		t.Error("a UI path reached the API")
	}
}

func TestUI3_ProxiesApiAuthAndHealthz(t *testing.T) {
	api := newFakeAPI(t)
	s := serve(t, api.srv.URL)
	for _, p := range []string{"/api/v1/me", "/auth/local", "/healthz", "/api/v1/setups?x=1"} {
		api.last = nil
		resp, err := http.Get(s.URL + p)
		if err != nil {
			t.Fatal(err)
		}
		b := body(t, resp)
		if resp.StatusCode != 200 || !strings.HasPrefix(b, `{"path":"/`) || api.last == nil {
			t.Errorf("%s: %d %q", p, resp.StatusCode, b)
		}
	}
	// The query string and method are forwarded as sent.
	api.last = nil
	req, _ := http.NewRequest("POST", s.URL+"/api/v1/holds?a=b", strings.NewReader("{}"))
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	body(t, resp)
	if api.last.Method != "POST" || api.last.URL.RawQuery != "a=b" {
		t.Errorf("forwarded %s %s", api.last.Method, api.last.URL)
	}
}

func TestUI4_OneOrigin_HostForwardedAndRedirectWithCookiePassedThrough(t *testing.T) {
	api := newFakeAPI(t)
	s := serve(t, api.srv.URL)
	req, _ := http.NewRequest("GET", s.URL+"/auth/oidc/callback?code=1", nil)
	req.Header.Set("Cookie", "hg_csrf=tok")
	resp, err := noRedirect.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	body(t, resp)
	if resp.StatusCode != http.StatusFound {
		t.Fatalf("status %d, want 302", resp.StatusCode)
	}
	if got := resp.Header.Get("Location"); got != "http://idp.example.test/next?x=1" {
		t.Errorf("Location rewritten: %q", got)
	}
	if got := resp.Header.Get("Set-Cookie"); got != "hg_session=abc; Path=/; HttpOnly; Secure; SameSite=Lax" {
		t.Errorf("Set-Cookie changed: %q", got)
	}
	if api.last.Host != req.URL.Host {
		t.Errorf("API saw Host %q, want the browser's %q", api.last.Host, req.URL.Host)
	}
	if got := api.last.Header.Get("X-Forwarded-Host"); got != req.URL.Host {
		t.Errorf("X-Forwarded-Host = %q", got)
	}
	if got := api.last.Header.Get("X-Forwarded-Proto"); got != "http" {
		t.Errorf("X-Forwarded-Proto = %q", got)
	}
	if got := api.last.Header.Get("Cookie"); got != "hg_csrf=tok" {
		t.Errorf("Cookie = %q", got)
	}
	// A front proxy's statement of the scheme is kept.
	req, _ = http.NewRequest("GET", s.URL+"/healthz", nil)
	req.Header.Set("X-Forwarded-Proto", "https")
	resp, _ = http.DefaultClient.Do(req)
	body(t, resp)
	if got := api.last.Header.Get("X-Forwarded-Proto"); got != "https" {
		t.Errorf("X-Forwarded-Proto behind TLS front = %q", got)
	}
}

func TestUI3_NothingElseIsProxied(t *testing.T) {
	api := newFakeAPI(t)
	s := serve(t, api.srv.URL)
	for _, p := range []string{"/", "/apix", "/authority", "/healthzz", "/metrics", "/setups", "/admin/api/x", "/ui-healthz"} {
		api.last = nil
		resp, err := http.Get(s.URL + p)
		if err != nil {
			t.Fatal(err)
		}
		body(t, resp)
		if api.last != nil {
			t.Errorf("%s reached the API", p)
		}
	}
	resp, _ := http.Get(s.URL + "/ui-healthz")
	if b := body(t, resp); resp.StatusCode != 200 || !strings.Contains(b, `"ok"`) {
		t.Errorf("/ui-healthz: %d %q", resp.StatusCode, b)
	}
}

func TestUI3_UpstreamDownIs502InTheStandardErrorShape(t *testing.T) {
	// A port nothing listens on.
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	addr := ln.Addr().String()
	ln.Close()
	s := serve(t, "http://"+addr)
	for _, p := range []string{"/api/v1/me", "/auth/local", "/healthz"} {
		resp, err := http.Get(s.URL + p)
		if err != nil {
			t.Fatal(err)
		}
		var out struct {
			Error struct{ Code, Message string }
		}
		ct := resp.Header.Get("Content-Type")
		if err := json.Unmarshal([]byte(body(t, resp)), &out); err != nil {
			t.Fatalf("%s: not JSON: %v", p, err)
		}
		if resp.StatusCode != http.StatusBadGateway || ct != "application/json" || out.Error.Code == "" || out.Error.Message == "" {
			t.Errorf("%s: %d %s %+v", p, resp.StatusCode, ct, out)
		}
	}
	// The UI itself still works with the API down.
	resp, _ := http.Get(s.URL + "/")
	if body(t, resp); resp.StatusCode != 200 {
		t.Errorf("/ with API down: %d", resp.StatusCode)
	}
}

func TestParseAPI(t *testing.T) {
	for _, ok := range []string{"http://api:8080", "https://h.example.test", "http://127.0.0.1:1/"} {
		if _, err := parseAPI(ok); err != nil {
			t.Errorf("%q: %v", ok, err)
		}
	}
	for _, bad := range []string{"", "api:8080", "ftp://x", "http://", "http://x/base", "http://x?q=1"} {
		if _, err := parseAPI(bad); err == nil {
			t.Errorf("%q accepted", bad)
		}
	}
}

func TestRunRequiresAPIAndServesUntilCancelled(t *testing.T) {
	var out, errb strings.Builder
	none := func(string) string { return "" }
	if code := run(context.Background(), nil, none, testUI, &out, &errb); code != 2 || !strings.Contains(errb.String(), "HOURGLASS_UI_API") {
		t.Errorf("no api: %d %q", code, errb.String())
	}
	errb.Reset()
	if code := run(context.Background(), []string{"-api", "http://x:1"}, none, nil, &out, &errb); code != 1 {
		t.Errorf("no UI built in: %d", code)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	env := func(k string) string {
		return map[string]string{apiEnv: "http://127.0.0.1:1", listenEnv: "127.0.0.1:0"}[k]
	}
	if code := run(ctx, nil, env, testUI, &out, &errb); code != 0 {
		t.Errorf("run: %d %s", code, errb.String())
	}
}

// UI-3: a path the prefix check and the upstream could read differently (dot
// segments, encoded slashes) is refused, not forwarded (task 23 finding 7).
func TestUI3_UnnormalisedProxiedPathsAreRefused(t *testing.T) {
	api := newFakeAPI(t)
	h, err := newHandler(testUI, api.srv.URL)
	if err != nil {
		t.Fatal(err)
	}
	for _, p := range []string{"/api/../etc/passwd", "/api%2f..%2fsecret", "/healthz/../admin", "/api/v1/./me", "/auth/%2e%2e/x", "/api/v1%5cme", "/api/v1/..%2Fx"} {
		api.last = nil
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, httptest.NewRequest("GET", p, nil))
		if rec.Code != http.StatusBadRequest || api.last != nil {
			t.Errorf("%s: status %d, forwarded=%v", p, rec.Code, api.last != nil)
		}
	}
	for _, p := range []string{"/api/v1/me", "/api/v1/setups/3", "/auth/local"} {
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, httptest.NewRequest("GET", p, nil))
		if rec.Code != 200 {
			t.Errorf("%s: status %d", p, rec.Code)
		}
	}
}

// D64: with a certificate and key the proxy serves HTTPS, and tells the API so.
func TestTLS_ServesHTTPSAndForwardsTheScheme(t *testing.T) {
	var proto atomic.Value
	api := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		proto.Store(r.Header.Get("X-Forwarded-Proto"))
	}))
	defer api.Close()
	cert, key, pool := servetlstest.WriteSelfSigned(t)
	env := func(k string) string {
		return map[string]string{apiEnv: api.URL, listenEnv: "127.0.0.1:0", tlsCertEnv: cert, tlsKeyEnv: key}[k]
	}
	ctx, cancel := context.WithCancel(context.Background())
	pr, pw := io.Pipe()
	done := make(chan int, 1)
	go func() { done <- run(ctx, nil, env, testUI, pw, io.Discard); pw.Close() }()
	var line struct {
		Addr string `json:"addr"`
		TLS  bool   `json:"tls"`
	}
	dec := json.NewDecoder(pr)
	if err := dec.Decode(&line); err != nil || !line.TLS {
		t.Fatalf("first log line: %v %+v", err, line)
	}
	go func() { _, _ = io.Copy(io.Discard, pr) }()
	client := &http.Client{Transport: &http.Transport{TLSClientConfig: &tls.Config{RootCAs: pool}}}
	defer client.CloseIdleConnections()
	for _, path := range []string{uiHealthPath, "/api/v1/session"} {
		resp, err := client.Get("https://" + line.Addr + path)
		if err != nil {
			t.Fatal(err)
		}
		resp.Body.Close()
		if resp.StatusCode != 200 {
			t.Errorf("%s over https: %d", path, resp.StatusCode)
		}
	}
	if proto.Load() != "https" {
		t.Errorf("X-Forwarded-Proto = %v, want https", proto.Load())
	}
	client.CloseIdleConnections()
	cancel()
	if code := <-done; code != 0 {
		t.Errorf("run: %d", code)
	}
}

func TestTLS_CertAndKeyAreAPair(t *testing.T) {
	cert, _, _ := servetlstest.WriteSelfSigned(t)
	var out, errb strings.Builder
	none := func(string) string { return "" }
	args := []string{"-api", "http://127.0.0.1:1", "-listen", "127.0.0.1:0", "-tls-cert", cert}
	if code := run(context.Background(), args, none, testUI, &out, &errb); code != 2 || !strings.Contains(errb.String(), tlsKeyEnv) {
		t.Errorf("cert only: %d %q", code, errb.String())
	}
	errb.Reset()
	args = append(args, "-tls-key", filepath.Join(t.TempDir(), "none.pem"))
	if code := run(context.Background(), args, none, testUI, &out, &errb); code != 1 {
		t.Errorf("missing key file: %d %q", code, errb.String())
	}
}
