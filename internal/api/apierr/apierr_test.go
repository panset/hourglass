// SPDX-License-Identifier: MIT

package apierr

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestWriteShape(t *testing.T) {
	rec := httptest.NewRecorder()
	Write(rec, http.StatusForbidden, NoRole, "you have no role")
	if rec.Code != http.StatusForbidden {
		t.Fatalf("status %d", rec.Code)
	}
	if ct := rec.Header().Get("Content-Type"); ct != "application/json" {
		t.Fatalf("content type %q", ct)
	}
	var raw map[string]map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &raw); err != nil {
		t.Fatal(err)
	}
	if len(raw) != 1 || len(raw["error"]) != 2 || raw["error"]["code"] != NoRole || raw["error"]["message"] != "you have no role" {
		t.Fatalf("body %s", rec.Body.String())
	}
}

func TestWriteFieldAndStatus(t *testing.T) {
	rec := httptest.NewRecorder()
	WriteField(rec, Status(Validation), Validation, "too short", "new_password")
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status %d", rec.Code)
	}
	var b Body
	if err := json.Unmarshal(rec.Body.Bytes(), &b); err != nil {
		t.Fatal(err)
	}
	if b.Error.Field != "new_password" {
		t.Fatalf("field %q", b.Error.Field)
	}
	for code, want := range map[string]int{Unauthenticated: 401, CSRF: 403, RateLimited: 429, Conflict: 409, "nope": 500} {
		if got := Status(code); got != want {
			t.Errorf("Status(%s) = %d, want %d", code, got, want)
		}
	}
}
