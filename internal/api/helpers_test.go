// SPDX-License-Identifier: MIT

package api

import (
	"context"
	"errors"
	"fmt"
	"math"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/panset/hourglass/internal/api/apierr"
	"github.com/panset/hourglass/internal/events"
	"github.com/panset/hourglass/internal/holds"
	"github.com/panset/hourglass/internal/ledger"
	"github.com/panset/hourglass/internal/settings"
	"github.com/panset/hourglass/internal/store"
)

type body struct {
	Name   string `json:"name"`
	Amount int    `json:"amount"`
}

func post(ct, payload string) (*httptest.ResponseRecorder, *http.Request) {
	req := httptest.NewRequest("POST", "/x", strings.NewReader(payload))
	if ct != "" {
		req.Header.Set("Content-Type", ct)
	}
	return httptest.NewRecorder(), req
}

func errBody(rec *httptest.ResponseRecorder) (code, field string) {
	r := reply{rec}
	var b struct {
		Error struct{ Code, Field string }
	}
	_ = jsonUnmarshal(r.Body.Bytes(), &b)
	return b.Error.Code, b.Error.Field
}

func TestDecodeJSON(t *testing.T) {
	cases := []struct {
		name, ct, payload string
		ok                bool
		field             string
	}{
		{"valid", "application/json", `{"name":"a","amount":2}`, true, ""},
		{"charset parameter", "application/json; charset=utf-8", `{"name":"a"}`, true, ""},
		{"wrong content type", "text/plain", `{"name":"a"}`, false, ""},
		{"no content type", "", `{"name":"a"}`, false, ""},
		{"unknown field", "application/json", `{"name":"a","extra":1}`, false, "extra"},
		{"wrong type", "application/json", `{"name":"a","amount":"x"}`, false, "amount"},
		{"trailing document", "application/json", `{"name":"a"}{"name":"b"}`, false, ""},
		{"trailing garbage", "application/json", `{"name":"a"} x`, false, ""},
		{"empty body", "application/json", ``, false, ""},
		{"truncated", "application/json", `{"name":`, false, ""},
		{"not an object", "application/json", `[1]`, false, ""},
		{"too large", "application/json", `{"name":"` + strings.Repeat("x", MaxBodyBytes) + `"}`, false, ""},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			rec, req := post(c.ct, c.payload)
			var b body
			ok := decodeJSON(rec, req, &b)
			if ok != c.ok {
				t.Fatalf("ok = %v, want %v (%d %s)", ok, c.ok, rec.Code, rec.Body)
			}
			if ok {
				if rec.Body.Len() != 0 {
					t.Fatalf("wrote on success: %s", rec.Body)
				}
				return
			}
			code, field := errBody(rec)
			if rec.Code != 400 || code != apierr.Validation || field != c.field {
				t.Fatalf("%d %s, want 400 validation field=%q", rec.Code, rec.Body, c.field)
			}
		})
	}
}

func TestPathID(t *testing.T) {
	for _, c := range []struct {
		in   string
		want int64
		ok   bool
	}{{"7", 7, true}, {"9223372036854775807", 9223372036854775807, true}, {"0", 0, false}, {"-3", 0, false},
		{"abc", 0, false}, {"1.5", 0, false}, {"", 0, false}, {"99999999999999999999", 0, false}} {
		req := httptest.NewRequest("GET", "/x", nil)
		req.SetPathValue("id", c.in)
		rec := httptest.NewRecorder()
		got, ok := pathID(rec, req)
		if ok != c.ok || got != c.want {
			t.Errorf("pathID(%q) = %d,%v want %d,%v", c.in, got, ok, c.want, c.ok)
		}
		if !ok {
			if code, field := errBody(rec); rec.Code != 400 || code != apierr.Validation || field != "id" {
				t.Errorf("pathID(%q) error: %d %s", c.in, rec.Code, rec.Body)
			}
		}
	}
}

func TestPeriodParam(t *testing.T) {
	for _, c := range []struct {
		q, want string
		ok      bool
	}{{"", "2030-02", true}, {"period=2029-12", "2029-12", true}, {"period=2030-13", "", false},
		{"period=2030-1", "", false}, {"period=abc", "", false}, {"period=2030-00", "", false}} {
		req := httptest.NewRequest("GET", "/x?"+c.q, nil)
		rec := httptest.NewRecorder()
		got, ok := periodParam(rec, req, "2030-02")
		if ok != c.ok || got != c.want {
			t.Errorf("periodParam(%q) = %q,%v want %q,%v", c.q, got, ok, c.want, c.ok)
		}
		if !ok {
			if code, field := errBody(rec); rec.Code != 400 || code != apierr.Validation || field != "period" {
				t.Errorf("periodParam(%q) error: %d %s", c.q, rec.Code, rec.Body)
			}
		}
	}
}

func TestTokensConversion(t *testing.T) {
	for _, c := range []struct {
		mt int64
		t  float64
	}{{0, 0}, {1, 0.001}, {1500, 1.5}, {142000, 142}, {-2500, -2.5}, {300000, 300}, {1234, 1.234}} {
		if got := tokens(c.mt); got != c.t {
			t.Errorf("tokens(%d) = %v, want %v", c.mt, got, c.t)
		}
		back, err := milliFromTokens(c.t)
		if err != nil || back != c.mt {
			t.Errorf("milliFromTokens(%v) = %d,%v want %d", c.t, back, err, c.mt)
		}
	}
	if got, _ := milliFromTokens(0.0004); got != 0 {
		t.Errorf("0.0004 rounds to %d", got)
	}
	if got, _ := milliFromTokens(0.0005); got != 1 {
		t.Errorf("0.0005 rounds to %d", got)
	}
	for _, bad := range []float64{math.NaN(), math.Inf(1), math.Inf(-1), 1e13, -1e13} {
		if _, err := milliFromTokens(bad); err == nil {
			t.Errorf("milliFromTokens(%v) accepted", bad)
		}
	}
}

func TestClassifyMapsServiceErrorsPerOpenAPITable(t *testing.T) {
	type want struct{ code, field string }
	cases := []struct {
		name string
		err  error
		want want
	}{
		{"holds forbidden", &holds.Error{Code: "forbidden", Message: "m"}, want{"forbidden", ""}},
		{"holds not_found", &holds.Error{Code: "not_found"}, want{"not_found", ""}},
		{"holds setup_disabled", &holds.Error{Code: "setup_disabled"}, want{"setup_disabled", ""}},
		{"holds setup_held", &holds.Error{Code: "setup_held"}, want{"setup_held", ""}},
		{"holds insufficient", &holds.Error{Code: "insufficient_tokens"}, want{"insufficient_tokens", ""}},
		{"holds hold_limit", &holds.Error{Code: "hold_limit"}, want{"hold_limit", ""}},
		{"holds hook_veto", &holds.Error{Code: "hook_veto", Message: "frozen"}, want{"hook_veto", ""}},
		{"holds validation", &holds.Error{Code: "validation"}, want{"validation", ""}},
		{"wrapped holds", fmt.Errorf("x: %w", &holds.Error{Code: "hold_limit"}), want{"hold_limit", ""}},
		{"veto", &events.VetoError{Hook: "h", Err: errors.New("no")}, want{"hook_veto", ""}},
		{"settings field", &settings.FieldError{Field: "poll_interval", Message: "too small"}, want{"validation", "poll_interval"}},
		{"ledger local admin", ledger.ErrLocalAdmin, want{"local_admin_protected", ""}},
		{"ledger no role", ledger.ErrNoRole, want{"conflict", ""}},
		{"ledger reason", ledger.ErrReasonRequired, want{"validation", "reason"}},
		{"ledger zero", ledger.ErrZeroAmount, want{"validation", "amount_tokens"}},
		{"store not found", store.ErrNotFound, want{"not_found", ""}},
		{"store setup held", store.ErrSetupHeld, want{"setup_held", ""}},
		{"store conflict", store.ErrConflict, want{"conflict", ""}},
		{"unknown", errors.New("sql: disk I/O error at /secret/path"), want{"internal", ""}},
		{"canceled", context.Canceled, want{"internal", ""}},
	}
	e := newEnv(t, envOpts{})
	a := &API{d: Deps{Logger: newLogger(e.logs)}}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			rec := httptest.NewRecorder()
			a.writeError(rec, httptest.NewRequest("GET", "/x", nil), c.err)
			code, field := errBody(rec)
			if code != c.want.code || field != c.want.field || rec.Code != apierr.Status(c.want.code) {
				t.Fatalf("%d %s, want %s field=%q", rec.Code, rec.Body, c.want.code, c.want.field)
			}
			if c.want.code == "internal" && strings.Contains(rec.Body.String(), "secret") {
				t.Fatalf("internal error leaks detail: %s", rec.Body)
			}
		})
	}
	if !strings.Contains(e.logs.String(), "/secret/path") {
		t.Error("internal error detail was not logged")
	}
}

func TestHoldsErrorCodesAreAllKnownToTheAPI(t *testing.T) {
	// holds' codes must be api error codes with the OpenAPI status.
	for code, status := range map[string]int{
		"forbidden": 403, "not_found": 404, "setup_disabled": 409, "setup_held": 409, "insufficient_tokens": 409,
		"hold_limit": 409, "hook_veto": 409, "validation": 400,
	} {
		if got := apierr.Status(code); got != status {
			t.Errorf("%s -> %d, want %d", code, got, status)
		}
	}
}
