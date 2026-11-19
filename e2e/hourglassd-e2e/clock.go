// SPDX-License-Identifier: MIT

package main

import (
	"encoding/json"
	"net/http"
	"sync/atomic"
	"time"
)

// offsetClock is the wall clock plus an offset that can be changed while the
// process runs. It satisfies the Clock option of hourglass.Options.
type offsetClock struct {
	offset atomic.Int64 // nanoseconds added to the wall clock
	wall   func() time.Time
}

func newOffsetClock() *offsetClock { return &offsetClock{wall: time.Now} }

// Now is the wall clock plus the current offset.
func (c *offsetClock) Now() time.Time { return c.wall().Add(time.Duration(c.offset.Load())) }

// SetOffset replaces the offset (it does not add to the previous one).
func (c *offsetClock) SetOffset(d time.Duration) { c.offset.Store(int64(d)) }

// Offset is the current offset.
func (c *offsetClock) Offset() time.Duration { return time.Duration(c.offset.Load()) }

type clockView struct {
	Offset string    `json:"offset"`
	Now    time.Time `json:"now"`
}

// controlHandler serves the control endpoint:
//
//	GET /clock                    {"offset":"720h0m0s","now":"..."}
//	PUT /clock {"offset":"720h"}  sets the offset (a Go duration string)
func controlHandler(c *offsetClock) http.Handler {
	mux := http.NewServeMux()
	view := func() clockView { return clockView{Offset: c.Offset().String(), Now: c.Now().UTC()} }
	mux.HandleFunc("/clock", func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
		case http.MethodPut:
			var in struct {
				Offset string `json:"offset"`
			}
			dec := json.NewDecoder(http.MaxBytesReader(w, r.Body, 4096))
			dec.DisallowUnknownFields()
			if err := dec.Decode(&in); err != nil {
				writeJSON(w, http.StatusBadRequest, map[string]string{"error": "body must be {\"offset\": \"<duration>\"}"})
				return
			}
			d, err := time.ParseDuration(in.Offset)
			if err != nil {
				writeJSON(w, http.StatusBadRequest, map[string]string{"error": "offset is not a duration like 720h or -1h30m"})
				return
			}
			c.SetOffset(d)
		default:
			w.Header().Set("Allow", "GET, PUT")
			writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
			return
		}
		writeJSON(w, http.StatusOK, view())
	})
	return mux
}

func writeJSON(w http.ResponseWriter, code int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(v)
}
