// SPDX-License-Identifier: MIT

package api

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"net/http"
	"sort"

	"github.com/panset/hourglass/internal/api/apierr"
	"github.com/panset/hourglass/internal/auth"
	"github.com/panset/hourglass/internal/reported"
	"github.com/panset/hourglass/internal/settings"
	"github.com/panset/hourglass/internal/weights"
)

// The weights report (docs/design/reported-weights.md, B1 to B3) and the
// weight fields of the integration state (B4).

type weightsReportRequest struct {
	ObservedAt *string `json:"observed_at"`
	// Weights is decoded entry by entry (B2), so a problem inside one entry
	// is reported with its position and name and never fails the whole body
	// in the shared decoder.
	Weights json.RawMessage `json:"weights"`
}

type weightsResultJSON struct {
	ObservedAt string   `json:"observed_at"`
	ReceivedAt string   `json:"received_at"`
	ExpiresAt  string   `json:"expires_at"`
	Accepted   int      `json:"accepted"`
	Unknown    []string `json:"unknown"`
}

// weightsOff answers 409 not_configured naming the admin setting (B2).
func weightsOff(w http.ResponseWriter) {
	apierr.WriteCode(w, apierr.NotConfigured, fmt.Sprintf(
		"reported weights are off: an admin turns them on with the %s setting; the report was not stored", settings.KeyReportedWeights))
}

// refusalCapture remembers the status and body a handler writes, so the
// weights route can record the message of every report it refuses with 400,
// whichever check refused it.
type refusalCapture struct {
	http.ResponseWriter
	status int
	body   bytes.Buffer
}

func (c *refusalCapture) WriteHeader(status int) {
	c.status = status
	c.ResponseWriter.WriteHeader(status)
}

func (c *refusalCapture) Write(p []byte) (int, error) {
	if c.status == 0 {
		c.status = http.StatusOK
	}
	if c.status == http.StatusBadRequest {
		c.body.Write(p)
	}
	return c.ResponseWriter.Write(p)
}

// reportWeights is PUT /api/v1/integration/weights (scope weights:write).
// Checks run in the order of D53: scope (the route guard), the switch, the
// body, then ordering. A report refused with 400 is kept in memory for the
// admins (B2). The report never triggers a poll, a debit or an event (B3).
func (a *API) reportWeights(w http.ResponseWriter, r *http.Request) {
	p, ok := auth.ServiceFrom(r.Context())
	if !ok {
		apierr.WriteCode(w, apierr.Unauthenticated, "authentication required")
		return
	}
	set, err := settings.Load(r.Context(), a.d.Store)
	if err != nil {
		a.writeError(w, r, err)
		return
	}
	cfg := set.ReportedWeights
	if !cfg.Enabled {
		weightsOff(w)
		return
	}
	c := &refusalCapture{ResponseWriter: w}
	a.submitWeights(c, r, p.AccountName, cfg)
	if c.status == http.StatusBadRequest {
		var e struct {
			Error struct {
				Message string `json:"message"`
			} `json:"error"`
		}
		_ = json.Unmarshal(c.body.Bytes(), &e)
		a.reported.RecordWeightsRefusal(reported.Refusal{At: a.d.Clock.Now().UTC(), ServiceAccount: p.AccountName, Message: e.Error.Message})
	}
}

func (a *API) submitWeights(w http.ResponseWriter, r *http.Request, account string, cfg settings.ReportedWeights) {
	var req weightsReportRequest
	if !decodeJSON(w, r, &req) {
		return
	}
	if len(req.Weights) == 0 || bytes.Equal(req.Weights, []byte("null")) {
		apierr.WriteField(w, http.StatusBadRequest, apierr.Validation, "weights is required (an empty list means no setup has a reported weight)", "weights")
		return
	}
	var raw []json.RawMessage
	if err := json.Unmarshal(req.Weights, &raw); err != nil {
		apierr.WriteField(w, http.StatusBadRequest, apierr.Validation, "weights must be a list", "weights")
		return
	}
	if len(raw) > reported.MaxWeightEntries {
		apierr.WriteField(w, http.StatusBadRequest, apierr.Validation,
			fmt.Sprintf("weights has %d entries (at most %d)", len(raw), reported.MaxWeightEntries), "weights")
		return
	}
	limits := reported.WeightLimits{Min: cfg.Min, Max: math.Inf(1)}
	if cfg.Max != nil {
		limits.Max = *cfg.Max
	}
	check := reported.NewWeightChecker(limits)
	set := make([]reported.Weight, len(raw))
	for i, e := range raw {
		wt, reason := decodeWeightEntry(e)
		if reason != "" {
			apierr.WriteField(w, http.StatusBadRequest, apierr.Validation, reported.EntryMessage(i, wt.Setup, reason), "weights")
			return
		}
		if err := check.Check(i, wt); err != nil {
			apierr.WriteField(w, http.StatusBadRequest, apierr.Validation, err.Error(), "weights")
			return
		}
		set[i] = wt
	}
	observed, ok := parseObservedAt(w, req.ObservedAt)
	if !ok {
		return
	}
	res, err := a.reported.SubmitWeights(r.Context(), account, observed, set, limits)
	if err != nil {
		var entry *reported.EntryError
		if errors.As(err, &entry) {
			apierr.WriteField(w, http.StatusBadRequest, apierr.Validation, err.Error(), "weights")
			return
		}
		a.writeError(w, r, err)
		return
	}
	setups, err := a.d.Store.ListSetups(r.Context(), false)
	if err != nil {
		a.writeError(w, r, err)
		return
	}
	known := make(map[string]bool, len(setups))
	for _, s := range setups {
		known[s.Name] = true
	}
	out := weightsResultJSON{
		ObservedAt: ts(res.ObservedAt), ReceivedAt: ts(res.ReceivedAt),
		ExpiresAt: ts(reported.ExpiresAt(res.ObservedAt, cfg.MaxAge)), Unknown: []string{},
	}
	for _, e := range set {
		if known[e.Setup] {
			out.Accepted++
		} else {
			out.Unknown = append(out.Unknown, e.Setup)
		}
	}
	writeJSON(w, http.StatusOK, out)
}

// decodeWeightEntry reads one entry of a weights report. It returns the
// entry and, when the entry's shape is wrong, the reason. Setup is filled
// whenever the entry has a string name, so the refusal can name it. The
// value rules (empty or repeated name, limits, attribute rules) are
// reported.WeightChecker's.
func decodeWeightEntry(raw json.RawMessage) (reported.Weight, string) {
	var out reported.Weight
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(raw, &fields); err != nil || fields == nil {
		return out, "must be an object with setup, weight and optional attributes"
	}
	name, hasName := fields["setup"]
	nameOK := hasName && json.Unmarshal(name, &out.Setup) == nil && !bytes.Equal(name, []byte("null"))
	if !nameOK {
		out.Setup = ""
	}
	var unknown []string
	for k := range fields {
		if k != "setup" && k != "weight" && k != "attributes" {
			unknown = append(unknown, k)
		}
	}
	if len(unknown) > 0 {
		sort.Strings(unknown)
		return out, fmt.Sprintf("unknown field %q (an entry holds only setup, weight and attributes)", unknown[0])
	}
	if !nameOK || out.Setup == "" {
		return out, "setup must be a non-empty string"
	}
	v, has := fields["weight"]
	if !has {
		return out, "weight is required"
	}
	if kind := jsonKind(v); kind != "number" {
		return out, "weight must be a JSON number, not " + kind
	}
	if err := json.Unmarshal(v, &out.Weight); err != nil {
		return out, fmt.Sprintf("weight %s is not a finite 64-bit number", bytes.TrimSpace(v))
	}
	if attrs, has := fields["attributes"]; has && !bytes.Equal(bytes.TrimSpace(attrs), []byte("null")) {
		var m map[string]json.RawMessage
		if err := json.Unmarshal(attrs, &m); err != nil {
			return out, "attributes must be an object of strings"
		}
		keys := make([]string, 0, len(m))
		for k := range m {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		out.Attributes = make(map[string]string, len(m))
		for _, k := range keys {
			var s string
			if jsonKind(m[k]) != "a string" || json.Unmarshal(m[k], &s) != nil {
				return out, fmt.Sprintf("attributes %q must be a string", k)
			}
			out.Attributes[k] = s
		}
	}
	return out, ""
}

// jsonKind names the kind of a JSON value for a refusal message: "number",
// "a string", "null", "a boolean", "an object" or "a list".
func jsonKind(v json.RawMessage) string {
	t := bytes.TrimSpace(v)
	if len(t) == 0 {
		return "empty"
	}
	switch c := t[0]; {
	case c == '-' || (c >= '0' && c <= '9'):
		return "number"
	case c == '"':
		return "a string"
	case c == 'n':
		return "null"
	case c == 't' || c == 'f':
		return "a boolean"
	case c == '{':
		return "an object"
	default:
		return "a list"
	}
}

// ---- the state fields (B4) ----

type stateWeightsJSON struct {
	Enabled    bool     `json:"enabled"`
	Min        float64  `json:"min"`
	Max        *float64 `json:"max"`
	ObservedAt *string  `json:"observed_at"`
	ExpiresAt  *string  `json:"expires_at"`
}

type stateWeightJSON struct {
	Applied    float64           `json:"applied"`
	Source     string            `json:"source"`
	Reported   *float64          `json:"reported"`
	NotApplied *string           `json:"not_applied"`
	Attributes map[string]string `json:"attributes"`
}

// stateWeightReport reads the stored weights report for the state route:
// nil when reported weights are off or before the first report.
func (a *API) stateWeightReport(r *http.Request, cfg settings.ReportedWeights) (*weights.Report, error) {
	if !cfg.Enabled {
		return nil, nil
	}
	rep, err := a.reported.Weights(r.Context())
	if err == reported.ErrNoReport {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	entries := make([]weights.Entry, len(rep.Weights))
	for i, e := range rep.Weights {
		entries[i] = weights.Entry{Setup: e.Setup, Weight: e.Weight, Attributes: e.Attributes}
	}
	return weights.NewReport(rep.ObservedAt, entries), nil
}

func stateWeightsTop(cfg settings.ReportedWeights, report *weights.Report) stateWeightsJSON {
	out := stateWeightsJSON{Enabled: cfg.Enabled, Min: cfg.Min, Max: cfg.Max}
	if report != nil {
		o, e := ts(report.ObservedAt), ts(reported.ExpiresAt(report.ObservedAt, cfg.MaxAge))
		out.ObservedAt, out.ExpiresAt = &o, &e
	}
	return out
}

func stateWeight(res weights.Result) *stateWeightJSON {
	out := &stateWeightJSON{Applied: res.Weight, Source: string(res.Source), Reported: res.Reported, Attributes: res.Attributes}
	if res.NotApplied != "" {
		s := string(res.NotApplied)
		out.NotApplied = &s
	}
	if out.Attributes == nil {
		out.Attributes = map[string]string{}
	}
	return out
}
