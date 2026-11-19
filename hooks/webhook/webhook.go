// SPDX-License-Identifier: MIT

// Package webhook is a Hook that POSTs each event as JSON to a URL, so
// automation can be added without writing Go. Register it by importing:
//
//	import _ "github.com/panset/hourglass/hooks/webhook"
//
// Config:
//
//	{"url": "https://example.test/hook", "secret": "shared-secret",
//	 "types": ["hold.acquired", "hold.released"]}
//
// url is required (http or https). secret is optional. types is optional and
// defaults to every event; unknown types are rejected at construction.
//
// Each request is a POST of the Event JSON (see hourglass.Event.MarshalJSON)
// with Content-Type application/json and an X-Hourglass-Event header holding
// the event type. When a secret is set, X-Hourglass-Signature holds
// "sha256=" plus the hex HMAC-SHA256 of the body keyed with the secret. Any
// non-2xx status or transport error is returned as an error carrying the
// status code and at most the first 200 bytes of the response body, so a
// vetoing endpoint's message can be shown for hold.acquiring. Redirects are
// not followed. The secret never appears in errors.
package webhook

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/panset/hourglass/pkg/hourglass"
)

// Name is the registered name of the hook.
const Name = "webhook"

const (
	maxBody         = 200
	safetyTimeout   = 60 * time.Second // the core applies 30 s through the context
	eventHeader     = "X-Hourglass-Event"
	signatureHeader = "X-Hourglass-Signature"
)

func init() {
	hourglass.RegisterHook(Name, New)
}

type config struct {
	URL    string   `json:"url"`
	Secret string   `json:"secret"`
	Types  []string `json:"types"`
}

type hook struct {
	url    string
	secret []byte
	types  map[string]bool // nil means all
	client *http.Client
}

// New is the factory registered under "webhook".
func New(raw json.RawMessage) (hourglass.Hook, error) {
	var c config
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&c); err != nil {
		return nil, fmt.Errorf("webhook: invalid config: %w", err)
	}
	if dec.More() {
		return nil, errors.New("webhook: invalid config: trailing data")
	}
	if c.URL == "" {
		return nil, errors.New(`webhook: config "url" is required`)
	}
	u, err := url.Parse(c.URL)
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
		// The URL is not echoed: it may carry credentials.
		return nil, errors.New(`webhook: config "url" must be an absolute http or https URL`)
	}
	h := &hook{
		url:    c.URL,
		secret: []byte(c.Secret),
		client: &http.Client{
			Timeout: safetyTimeout,
			CheckRedirect: func(*http.Request, []*http.Request) error {
				return http.ErrUseLastResponse
			},
		},
	}
	if c.Types != nil {
		known := map[string]bool{}
		for _, t := range hourglass.EventTypes() {
			known[t] = true
		}
		h.types = map[string]bool{}
		for _, t := range c.Types {
			if !known[t] {
				return nil, fmt.Errorf("webhook: unknown event type %q in types (known: %s)", t,
					strings.Join(hourglass.EventTypes(), ", "))
			}
			h.types[t] = true
		}
	}
	return h, nil
}

func (h *hook) Name() string { return Name }

// Handle posts the event unless its type is filtered out.
func (h *hook) Handle(ctx context.Context, e hourglass.Event) error {
	if h.types != nil && !h.types[e.Type] {
		return nil
	}
	body, err := json.Marshal(e)
	if err != nil {
		return fmt.Errorf("webhook: encode event: %w", err)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, h.url, bytes.NewReader(body))
	if err != nil {
		return errors.New("webhook: build request failed")
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set(eventHeader, e.Type)
	if len(h.secret) > 0 {
		mac := hmac.New(sha256.New, h.secret)
		mac.Write(body)
		req.Header.Set(signatureHeader, "sha256="+hex.EncodeToString(mac.Sum(nil)))
	}
	resp, err := h.client.Do(req)
	if err != nil {
		// Drop the *url.Error wrapper: it repeats the URL, which may hold credentials.
		var ue *url.Error
		if errors.As(err, &ue) {
			err = ue.Err
		}
		return fmt.Errorf("webhook: POST failed: %w", err)
	}
	defer resp.Body.Close()
	snippet, _ := io.ReadAll(io.LimitReader(resp.Body, maxBody))
	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		return nil
	}
	msg := string(snippet)
	if len(h.secret) > 0 {
		msg = strings.ReplaceAll(msg, string(h.secret), "[redacted]")
	}
	msg = strings.TrimSpace(msg)
	if msg == "" {
		return fmt.Errorf("webhook: endpoint returned status %d", resp.StatusCode)
	}
	return fmt.Errorf("webhook: endpoint returned status %d: %s", resp.StatusCode, msg)
}
