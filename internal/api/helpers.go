// SPDX-License-Identifier: MIT

package api

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"mime"
	"net/http"
	"strconv"
	"strings"

	"github.com/panset/hourglass/internal/api/apierr"
	"github.com/panset/hourglass/internal/events"
	"github.com/panset/hourglass/internal/holds"
	"github.com/panset/hourglass/internal/ledger"
	"github.com/panset/hourglass/internal/settings"
	"github.com/panset/hourglass/internal/store"
)

// MaxBodyBytes bounds a JSON request body. A setup's details may be 256 KB
// (SET-2), so the limit leaves room for the rest of the document.
const MaxBodyBytes = 1 << 20

// MilliPerToken is the number of milli-tokens in one token.
const MilliPerToken = 1000

// writeJSON writes v with the status. Cache-Control: no-store is also set by
// the router middleware; it is repeated so a handler used alone is safe.
func writeJSON(w http.ResponseWriter, status int, v any) {
	h := w.Header()
	h.Set("Content-Type", "application/json")
	h.Set("Cache-Control", "no-store")
	h.Set("X-Content-Type-Options", "nosniff")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

// decodeJSON reads one JSON document into v: Content-Type must be
// application/json, the body is limited to MaxBodyBytes, unknown fields are
// rejected, and trailing data is an error. On failure it writes a 400
// validation error (with `field` when the decoder names one) and returns
// false.
func decodeJSON(w http.ResponseWriter, r *http.Request, v any) bool {
	if mt, _, err := mime.ParseMediaType(r.Header.Get("Content-Type")); err != nil || mt != "application/json" {
		apierr.Write(w, http.StatusBadRequest, apierr.Validation, "Content-Type must be application/json")
		return false
	}
	dec := json.NewDecoder(http.MaxBytesReader(w, r.Body, MaxBodyBytes))
	dec.DisallowUnknownFields()
	if err := dec.Decode(v); err != nil {
		writeDecodeError(w, err)
		return false
	}
	if _, err := dec.Token(); !errors.Is(err, io.EOF) {
		apierr.Write(w, http.StatusBadRequest, apierr.Validation, "request body must be a single JSON document")
		return false
	}
	return true
}

func writeDecodeError(w http.ResponseWriter, err error) {
	var (
		tooBig *http.MaxBytesError
		typeEr *json.UnmarshalTypeError
		synEr  *json.SyntaxError
	)
	switch {
	case errors.As(err, &tooBig):
		apierr.Write(w, http.StatusBadRequest, apierr.Validation, fmt.Sprintf("request body exceeds %d bytes", MaxBodyBytes))
	case errors.Is(err, io.EOF):
		apierr.Write(w, http.StatusBadRequest, apierr.Validation, "request body is required")
	case errors.As(err, &typeEr):
		apierr.WriteField(w, http.StatusBadRequest, apierr.Validation,
			fmt.Sprintf("%s must be of type %s", typeEr.Field, typeEr.Type), typeEr.Field)
	case errors.As(err, &synEr), errors.Is(err, io.ErrUnexpectedEOF):
		apierr.Write(w, http.StatusBadRequest, apierr.Validation, "request body is not valid JSON")
	case strings.HasPrefix(err.Error(), "json: unknown field "):
		field := strings.Trim(strings.TrimPrefix(err.Error(), "json: unknown field "), `"`)
		apierr.WriteField(w, http.StatusBadRequest, apierr.Validation, "unknown field "+strconv.Quote(field), field)
	default:
		apierr.Write(w, http.StatusBadRequest, apierr.Validation, "invalid request body")
	}
}

// pathID parses the {id} path value as a positive integer. On failure it
// writes 400 validation (field "id") and returns false.
func pathID(w http.ResponseWriter, r *http.Request) (int64, bool) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil || id <= 0 {
		apierr.WriteField(w, http.StatusBadRequest, apierr.Validation, "id must be a positive integer", "id")
		return 0, false
	}
	return id, true
}

// periodParam reads the optional ?period=YYYY-MM query parameter. An absent
// parameter yields current. A malformed one writes 400 validation (field
// "period") and returns false.
func periodParam(w http.ResponseWriter, r *http.Request, current string) (string, bool) {
	p := r.URL.Query().Get("period")
	if p == "" {
		return current, true
	}
	if _, _, err := ledger.ParsePeriod(p); err != nil {
		apierr.WriteField(w, http.StatusBadRequest, apierr.Validation, "period must be YYYY-MM", "period")
		return "", false
	}
	return p, true
}

// tokens converts milli-tokens to the API's whole-token number (up to three
// decimals).
func tokens(mt int64) float64 { return float64(mt) / MilliPerToken }

// maxTokens bounds a token amount accepted from a client: far above any real
// allowance and far below int64 overflow in milli-tokens.
const maxTokens = 1e12

// milliFromTokens converts an API token amount to milli-tokens, rounding to
// the nearest milli-token. NaN, infinities and absurd magnitudes are errors.
func milliFromTokens(t float64) (int64, error) {
	if math.IsNaN(t) || math.IsInf(t, 0) || math.Abs(t) > maxTokens {
		return 0, errors.New("amount is out of range")
	}
	return int64(math.Round(t * MilliPerToken)), nil
}

// writeError maps a service error to the API's error shape and status per
// the OpenAPI error table; anything unrecognised is logged and answered as
// 500 internal with a fixed message.
func (a *API) writeError(w http.ResponseWriter, r *http.Request, err error) {
	code, msg, field := classify(err)
	if code == apierr.Internal {
		a.d.Logger.Error("request failed", "method", r.Method, "path", r.URL.Path, "err", err)
		msg = "internal error"
	}
	apierr.WriteField(w, apierr.Status(code), code, msg, field)
}

// classify returns the error code, message and optional field for err.
func classify(err error) (code, msg, field string) {
	var (
		he *holds.Error
		ve *events.VetoError
		fe *settings.FieldError
	)
	switch {
	case errors.As(err, &he):
		return he.Code, he.Message, ""
	case errors.As(err, &ve):
		return apierr.HookVeto, ve.Error(), ""
	case errors.As(err, &fe):
		return apierr.Validation, fe.Message, fe.Field
	case errors.Is(err, ledger.ErrLocalAdmin):
		return apierr.LocalAdminProtected, "the local admin account has no token balance", ""
	case errors.Is(err, ledger.ErrNoRole):
		return apierr.Conflict, "the user has no role", ""
	case errors.Is(err, ledger.ErrReasonRequired):
		return apierr.Validation, "reason is required", "reason"
	case errors.Is(err, ledger.ErrZeroAmount):
		return apierr.Validation, "amount_tokens must not be zero", "amount_tokens"
	case errors.Is(err, store.ErrNotFound):
		return apierr.NotFound, "not found", ""
	case errors.Is(err, store.ErrSetupHeld):
		return apierr.SetupHeld, "setup is already held", ""
	case errors.Is(err, store.ErrConflict):
		return apierr.Conflict, "conflicts with existing data", ""
	case errors.Is(err, context.Canceled), errors.Is(err, context.DeadlineExceeded):
		return apierr.Internal, "", ""
	default:
		return apierr.Internal, "", ""
	}
}

// nullIfEmpty is a pointer to s, or nil (JSON null) when s is empty.
func nullIfEmpty(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

// labelsNonNil makes a nil label map an empty one (JSON {} not null).
func labelsNonNil(m map[string]string) map[string]string {
	if m == nil {
		return map[string]string{}
	}
	return m
}

// userDisplayName is the name shown for a user: name, else username, else email.
func userDisplayName(u store.User) string {
	switch {
	case u.Name != "":
		return u.Name
	case u.Username != "":
		return u.Username
	default:
		return u.Email
	}
}

func isNotFound(err error) bool { return errors.Is(err, store.ErrNotFound) }
