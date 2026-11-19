// SPDX-License-Identifier: MIT

// Package uiserve serves a built single-page UI from a file tree (UI-3,
// UI-7). It knows nothing about the UI's framework, layout or routes: the
// tree needs an index.html at its root, and everything under assets/ is
// taken to be fingerprinted build output.
//
// The embedded copy of the UI sits behind the "ui" build tag (embed_ui.go);
// without the tag Embedded reports false and the binary is API-only.
package uiserve

import (
	"bytes"
	"errors"
	"io"
	"io/fs"
	"mime"
	"net/http"
	"path"
	"strings"
	"time"
)

const (
	indexName = "index.html"
	// assetsPrefix is the directory of fingerprinted build output; its files
	// never change under the same name, so they cache for a year.
	assetsPrefix = "assets/"

	cacheImmutable  = "public, max-age=31536000, immutable"
	cacheRevalidate = "no-cache"

	// CSP for a same-origin SPA. The built index.html has no inline script
	// or style: one module script and one stylesheet, both from /assets/, so
	// 'self' is enough for script-src and style-src. Nothing here allows
	// 'unsafe-inline' or 'unsafe-eval'. img-src adds data: for small inlined
	// images that bundlers emit; connect-src 'self' covers fetch to the API
	// on the same origin (UI-4). If a future build needs more, relax only the
	// directive it needs.
	contentSecurityPolicy = "default-src 'self'; script-src 'self'; style-src 'self'; " +
		"img-src 'self' data:; font-src 'self'; connect-src 'self'; " +
		"object-src 'none'; base-uri 'self'; form-action 'self'; frame-ancestors 'none'"
)

// reserved are the path prefixes that belong to the API and never fall back
// to index.html: an unknown path under them stays the API's JSON 404.
var reserved = []string{"/api/", "/auth/", "/healthz"}

// Handler serves fsys (the root of the built UI). GET and HEAD only. A path
// that names an existing regular file is served as is; "/" and any other
// path that is not a file fall back to index.html (client-side routes),
// except reserved API paths, which get a plain 404 (in a full server the
// API's own handlers match those first). Directories are never listed.
func Handler(fsys fs.FS) http.Handler {
	return &handler{fsys: fsys}
}

type handler struct{ fsys fs.FS }

func (h *handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	hd := w.Header()
	hd.Set("X-Content-Type-Options", "nosniff")
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		hd.Set("Allow", "GET, HEAD")
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	p := r.URL.Path
	if isReserved(p) {
		http.NotFound(w, r)
		return
	}
	if strings.ContainsRune(p, 0) || strings.Contains(p, "\\") {
		http.NotFound(w, r)
		return
	}
	// path.Clean resolves "." and ".." against a rooted path, so the result
	// cannot climb out of the tree; fs.FS implementations also reject
	// unrooted or dotted names (fs.ValidPath).
	name := strings.TrimPrefix(path.Clean("/"+p), "/")
	if name == "" {
		name = indexName
	}
	if fi, err := fs.Stat(h.fsys, name); err != nil || !fi.Mode().IsRegular() {
		if err != nil && !errors.Is(err, fs.ErrNotExist) && !errors.Is(err, fs.ErrInvalid) {
			http.Error(w, "internal error", http.StatusInternalServerError)
			return
		}
		// A missing file under assets/ is a missing build asset: 404. Any
		// other path is a client route, even when its last segment has a
		// dot (a user name, a version, a host name).
		if name == strings.TrimSuffix(assetsPrefix, "/") || strings.HasPrefix(name, assetsPrefix) {
			http.NotFound(w, r)
			return
		}
		name = indexName
	}
	h.serveFile(w, r, name)
}

func (h *handler) serveFile(w http.ResponseWriter, r *http.Request, name string) {
	f, err := h.fsys.Open(name)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	defer f.Close()
	fi, err := f.Stat()
	if err != nil || !fi.Mode().IsRegular() {
		http.NotFound(w, r)
		return
	}
	hd := w.Header()
	if ct := mime.TypeByExtension(path.Ext(name)); ct != "" {
		hd.Set("Content-Type", ct)
	}
	if name == indexName {
		hd.Set("Cache-Control", cacheRevalidate)
		hd.Set("Content-Security-Policy", contentSecurityPolicy)
		hd.Set("X-Frame-Options", "DENY")
		hd.Set("Referrer-Policy", "same-origin")
	} else if strings.HasPrefix(name, assetsPrefix) {
		hd.Set("Cache-Control", cacheImmutable)
	} else {
		hd.Set("Cache-Control", cacheRevalidate)
	}
	if rs, ok := f.(io.ReadSeeker); ok {
		http.ServeContent(w, r, name, fi.ModTime(), rs)
		return
	}
	// Not seekable (e.g. some in-memory FS): read it whole.
	data, err := io.ReadAll(f)
	if err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	http.ServeContent(w, r, name, time.Time{}, bytes.NewReader(data))
}

func isReserved(p string) bool {
	for _, pre := range reserved {
		if p == strings.TrimSuffix(pre, "/") || strings.HasPrefix(p, pre) {
			return true
		}
	}
	return false
}
