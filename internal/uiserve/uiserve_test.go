// SPDX-License-Identifier: MIT

package uiserve

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"testing/fstest"
)

func testFS() fstest.MapFS {
	return fstest.MapFS{
		"index.html":         {Data: []byte(`<!doctype html><script type="module" src="/assets/app-abc.js"></script>`)},
		"assets/app-abc.js":  {Data: []byte("console.log(1)")},
		"assets/app-abc.css": {Data: []byte("body{}")},
		"favicon.svg":        {Data: []byte("<svg/>")},
		"sub/index.html":     {Data: []byte("sub")},
	}
}

func get(h http.Handler, method, target string) *httptest.ResponseRecorder {
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(method, target, nil))
	return rec
}

func TestUI3_IndexAtRoot(t *testing.T) {
	rec := get(Handler(testFS()), "GET", "/")
	if rec.Code != 200 || !strings.Contains(rec.Body.String(), "<!doctype html>") {
		t.Fatalf("got %d %q", rec.Code, rec.Body.String())
	}
	if ct := rec.Header().Get("Content-Type"); !strings.HasPrefix(ct, "text/html") {
		t.Errorf("content type %q", ct)
	}
	if cc := rec.Header().Get("Cache-Control"); cc != "no-cache" {
		t.Errorf("index cache-control %q", cc)
	}
	csp := rec.Header().Get("Content-Security-Policy")
	if !strings.Contains(csp, "script-src 'self'") || strings.Contains(csp, "unsafe-") {
		t.Errorf("csp %q", csp)
	}
}

func TestUI3_AssetImmutableAndTypes(t *testing.T) {
	h := Handler(testFS())
	for target, want := range map[string]string{
		"/assets/app-abc.js":  "javascript",
		"/assets/app-abc.css": "text/css",
	} {
		rec := get(h, "GET", target)
		if rec.Code != 200 {
			t.Fatalf("%s: %d", target, rec.Code)
		}
		if !strings.Contains(rec.Header().Get("Content-Type"), want) {
			t.Errorf("%s content type %q", target, rec.Header().Get("Content-Type"))
		}
		if cc := rec.Header().Get("Cache-Control"); !strings.Contains(cc, "immutable") || !strings.Contains(cc, "max-age=31536000") {
			t.Errorf("%s cache-control %q", target, cc)
		}
	}
	if cc := get(h, "GET", "/favicon.svg").Header().Get("Cache-Control"); cc != "no-cache" {
		t.Errorf("non-asset cache-control %q", cc)
	}
}

func TestUI3_ClientRouteFallsBackToIndex(t *testing.T) {
	h := Handler(testFS())
	// Paths whose last segment contains a dot are client routes too.
	for _, target := range []string{"/setups/123", "/admin/users", "/login", "/users/jo.smith", "/setups/lab-01.example.org", "/missing.png", "/v1.2/notes.txt", "/index.htm"} {
		rec := get(h, "GET", target)
		if rec.Code != 200 || !strings.Contains(rec.Body.String(), "<!doctype html>") {
			t.Errorf("%s: %d %q", target, rec.Code, rec.Body.String())
		}
		if rec.Header().Get("Cache-Control") != "no-cache" {
			t.Errorf("%s cache-control %q", target, rec.Header().Get("Cache-Control"))
		}
	}
}

func TestUI3_ReservedPathsStayNotFound(t *testing.T) {
	h := Handler(testFS())
	for _, target := range []string{"/api/v1/nope", "/api/", "/auth/x", "/auth/", "/healthz", "/healthz/x"} {
		rec := get(h, "GET", target)
		if rec.Code != 404 || strings.Contains(rec.Body.String(), "<!doctype") {
			t.Errorf("%s: %d %q", target, rec.Code, rec.Body.String())
		}
	}
}

func TestUI3_MissingAssetIsNotHTML(t *testing.T) {
	h := Handler(testFS())
	for _, target := range []string{"/assets/gone.js", "/assets/sub/gone.png", "/assets/noextension", "/assets"} {
		if rec := get(h, "GET", target); rec.Code != 404 {
			t.Errorf("%s: %d", target, rec.Code)
		}
	}
}

func TestUI3_NoDirectoryListing(t *testing.T) {
	h := Handler(testFS())
	for _, target := range []string{"/assets", "/assets/", "/sub", "/sub/"} {
		rec := get(h, "GET", target)
		body := rec.Body.String()
		if strings.Contains(body, "app-abc.css") || strings.Contains(body, "sub") || strings.Contains(body, "<pre>") {
			t.Errorf("%s lists a directory: %q", target, body)
		}
	}
}

func TestUI3_TraversalImpossible(t *testing.T) {
	fsys := testFS()
	fsys["../etc/passwd"] = &fstest.MapFile{Data: []byte("root")}
	h := Handler(fsys)
	for _, target := range []string{
		"/../etc/passwd", "/assets/../../etc/passwd", "/%2e%2e/etc/passwd",
		"/..%2fetc/passwd", "/assets/..%2f..%2fetc/passwd", "/a\\..\\b", "/%00",
	} {
		req := httptest.NewRequest("GET", "/", nil)
		req.URL.Path = ""
		req.URL.RawPath = ""
		rec := httptest.NewRecorder()
		u := *req.URL
		u.Path = unescape(target)
		req.URL = &u
		h.ServeHTTP(rec, req)
		if strings.Contains(rec.Body.String(), "root") {
			t.Errorf("%s leaked: %q", target, rec.Body.String())
		}
	}
}

func unescape(s string) string {
	r := strings.NewReplacer("%2e", ".", "%2f", "/", "%00", "\x00")
	return r.Replace(s)
}

func TestUI3_HeadersAndMethods(t *testing.T) {
	h := Handler(testFS())
	for _, target := range []string{"/", "/assets/app-abc.js", "/setups/1", "/nope.png", "/assets/gone.js"} {
		if got := get(h, "GET", target).Header().Get("X-Content-Type-Options"); got != "nosniff" {
			t.Errorf("%s nosniff %q", target, got)
		}
	}
	if rec := get(h, "POST", "/"); rec.Code != 405 {
		t.Errorf("POST: %d", rec.Code)
	}
	if rec := get(h, "HEAD", "/"); rec.Code != 200 || rec.Body.Len() != 0 {
		t.Errorf("HEAD: %d %d", rec.Code, rec.Body.Len())
	}
}
