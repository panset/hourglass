// SPDX-License-Identifier: MIT

//go:build ui

package uiserve

import (
	"io/fs"
	"strings"
	"testing"
)

func TestUI3_EmbeddedContainsIndexAndServesIt(t *testing.T) {
	fsys, ok := Embedded()
	if !ok {
		t.Fatal("Embedded() = false in a ui build")
	}
	if _, err := fs.Stat(fsys, "index.html"); err != nil {
		t.Fatal(err)
	}
	rec := get(Handler(fsys), "GET", "/")
	if rec.Code != 200 || !strings.Contains(strings.ToLower(rec.Body.String()), "<!doctype html>") {
		t.Fatalf("got %d %q", rec.Code, rec.Body.String())
	}
	if rec := get(Handler(fsys), "GET", "/setups/9"); rec.Code != 200 {
		t.Fatalf("fallback: %d", rec.Code)
	}
}
