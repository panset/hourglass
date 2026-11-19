// SPDX-License-Identifier: MIT

// Package apierr writes the API's single error shape,
// {"error": {"code": "...", "message": "..."}}, with the codes listed in
// api/openapi.yaml (ErrorCode). Auth and the API handlers share it.
package apierr

import (
	"encoding/json"
	"net/http"
)

// Error codes (api/openapi.yaml components.schemas.ErrorCode).
const (
	Unauthenticated        = "unauthenticated"          // 401
	NoRole                 = "no_role"                  // 403
	Forbidden              = "forbidden"                // 403
	CSRF                   = "csrf"                     // 403
	PasswordChangeRequired = "password_change_required" // 403
	NotFound               = "not_found"                // 404
	Validation             = "validation"               // 400
	InvalidSelector        = "invalid_selector"         // 400
	InsufficientTokens     = "insufficient_tokens"      // 409
	HoldLimit              = "hold_limit"               // 409
	SetupHeld              = "setup_held"               // 409
	SetupDisabled          = "setup_disabled"           // 409
	StillDeployed          = "still_deployed"           // 409
	HookVeto               = "hook_veto"                // 409
	HasActiveHold          = "has_active_hold"          // 409
	LastAdmin              = "last_admin"               // 409
	SelfDemote             = "self_demote"              // 409
	LocalAdminProtected    = "local_admin_protected"    // 409
	Conflict               = "conflict"                 // 409
	RateLimited            = "rate_limited"             // 429
	Internal               = "internal"                 // 500
)

var statusOf = map[string]int{
	Unauthenticated:        http.StatusUnauthorized,
	NoRole:                 http.StatusForbidden,
	Forbidden:              http.StatusForbidden,
	CSRF:                   http.StatusForbidden,
	PasswordChangeRequired: http.StatusForbidden,
	NotFound:               http.StatusNotFound,
	Validation:             http.StatusBadRequest,
	InvalidSelector:        http.StatusBadRequest,
	InsufficientTokens:     http.StatusConflict,
	HoldLimit:              http.StatusConflict,
	SetupHeld:              http.StatusConflict,
	SetupDisabled:          http.StatusConflict,
	StillDeployed:          http.StatusConflict,
	HookVeto:               http.StatusConflict,
	HasActiveHold:          http.StatusConflict,
	LastAdmin:              http.StatusConflict,
	SelfDemote:             http.StatusConflict,
	LocalAdminProtected:    http.StatusConflict,
	Conflict:               http.StatusConflict,
	RateLimited:            http.StatusTooManyRequests,
	Internal:               http.StatusInternalServerError,
}

// Status returns the HTTP status for a code (500 for an unknown code).
func Status(code string) int {
	if s, ok := statusOf[code]; ok {
		return s
	}
	return http.StatusInternalServerError
}

// Body is the JSON error document.
type Body struct {
	Error Detail `json:"error"`
}

// Detail is the inner error object. Field is set only for validation errors.
type Detail struct {
	Code    string `json:"code"`
	Message string `json:"message"`
	Field   string `json:"field,omitempty"`
}

// Write sends {"error": {"code", "message"}} with the given status.
func Write(w http.ResponseWriter, status int, code, message string) {
	WriteField(w, status, code, message, "")
}

// WriteField is Write with the optional "field" member (validation errors).
func WriteField(w http.ResponseWriter, status int, code, message, field string) {
	h := w.Header()
	h.Set("Content-Type", "application/json")
	h.Set("Cache-Control", "no-store")
	h.Set("X-Content-Type-Options", "nosniff")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(Body{Error: Detail{Code: code, Message: message, Field: field}})
}

// WriteCode is Write with the status taken from Status(code).
func WriteCode(w http.ResponseWriter, code, message string) {
	Write(w, Status(code), code, message)
}
