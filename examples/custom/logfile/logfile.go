// SPDX-License-Identifier: MIT

// Package logfile is a Hook that appends each event to a file as one JSON
// line, and that vetoes acquiring a setup labelled maintenance=true.
//
// It shows the two roles a hook plays:
//
//   - Observer. Every event type reaches Handle, in order, on a background
//     queue; the optional "events" list says which ones to write. An error
//     from an observer is logged and recorded but never blocks anything.
//   - Gatekeeper. hold.acquiring is delivered inside the user's request,
//     and an error returned for it aborts the acquire; the error text is
//     shown to the user. That is the only vetoable event
//     (hourglass.Vetoable).
//
// Config:
//
//	hooks:
//	  - type: logfile
//	    config:
//	      path: /var/log/lab/events.jsonl
//	      events: [hold.acquired, hold.released]   # optional; default all
//
// The events filter only controls what is written. The maintenance veto
// applies regardless, so narrowing the log never re-opens a closed rig.
package logfile

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"sync"
	"time"

	"github.com/panset/hourglass/pkg/hourglass"
)

// Name is the name the config file uses to select this hook.
const Name = "logfile"

// MaintenanceLabel is the label that closes a setup to new holds.
const MaintenanceLabel = "maintenance"

func init() {
	hourglass.RegisterHook(Name, New)
}

type config struct {
	Path   string   `json:"path"`
	Events []string `json:"events"`
}

type hook struct {
	path   string
	events map[string]bool // nil means every event
	mu     sync.Mutex      // one writer at a time keeps lines whole
}

// New is the factory. "path" is required; "events" is checked against the
// known event types so a misspelling fails at startup.
func New(raw json.RawMessage) (hourglass.Hook, error) {
	var c config
	if t := bytes.TrimSpace(raw); len(t) == 0 || string(t) == "null" {
		return nil, errors.New(`logfile: config "path" is required`)
	}
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&c); err != nil {
		return nil, fmt.Errorf("logfile: invalid config: %w", err)
	}
	if c.Path == "" {
		return nil, errors.New(`logfile: config "path" is required`)
	}
	h := &hook{path: c.Path}
	if len(c.Events) > 0 {
		known := map[string]bool{}
		for _, e := range hourglass.EventTypes() {
			known[e] = true
		}
		h.events = map[string]bool{}
		for _, e := range c.Events {
			if !known[e] {
				return nil, fmt.Errorf("logfile: unknown event type %q (known: %v)", e, hourglass.EventTypes())
			}
			h.events[e] = true
		}
	}
	return h, nil
}

func (*hook) Name() string { return Name }

// line is what one log line holds. Only names and labels of the setup, not
// its details blob, are written.
type line struct {
	Type   string            `json:"type"`
	Time   time.Time         `json:"time"`
	User   string            `json:"user,omitempty"`
	Setup  string            `json:"setup,omitempty"`
	Labels map[string]string `json:"labels,omitempty"`
	Hold   int64             `json:"hold,omitempty"`
	Data   map[string]any    `json:"data,omitempty"`
}

// Handle vetoes first, then logs. User, Setup and Hold are nil for events
// that are not about them, so every access is guarded.
func (h *hook) Handle(ctx context.Context, e hourglass.Event) error {
	if hourglass.Vetoable(e.Type) && e.Setup != nil && e.Setup.Labels[MaintenanceLabel] == "true" {
		return fmt.Errorf("%s is under maintenance and cannot be acquired right now", e.Setup.Name)
	}
	if h.events != nil && !h.events[e.Type] {
		return nil
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	l := line{Type: e.Type, Time: e.Time.UTC(), Data: e.Data}
	if e.User != nil {
		l.User = e.User.Username
	}
	if e.Setup != nil {
		l.Setup, l.Labels = e.Setup.Name, e.Setup.Labels
	}
	if e.Hold != nil {
		l.Hold = e.Hold.ID
	}
	b, err := json.Marshal(l)
	if err != nil {
		return fmt.Errorf("logfile: encode %s: %w", e.Type, err)
	}
	b = append(b, '\n')

	h.mu.Lock()
	defer h.mu.Unlock()
	// Open per event: slow enough not to matter at lab scale, and it keeps
	// working when logrotate moves the file away.
	f, err := os.OpenFile(h.path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o600)
	if err != nil {
		return fmt.Errorf("logfile: %w", err)
	}
	_, werr := f.Write(b)
	if cerr := f.Close(); werr == nil {
		werr = cerr
	}
	if werr != nil {
		return fmt.Errorf("logfile: write %s: %w", h.path, werr)
	}
	return nil
}
