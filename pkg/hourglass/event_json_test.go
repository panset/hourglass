// SPDX-License-Identifier: MIT

package hourglass

import (
	"encoding/json"
	"reflect"
	"testing"
	"time"
)

func TestEXT6_EventJSONRoundTrip(t *testing.T) {
	ts := time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC)
	later := ts.Add(time.Hour)
	full := Event{
		Type: EventHoldReleased, Time: ts,
		User:  &User{ID: 7, Username: "ada", Email: "a@example.test", Name: "Ada", Roles: []string{"developer", "admin"}},
		Setup: &Setup{Name: "rig-1", Labels: map[string]string{"k": "v"}, Details: json.RawMessage(`{"a":[1,2],"b":"x"}`)},
		Hold: &Hold{ID: 3, SetupName: "rig-1", UserID: 7, AcquiredAt: ts,
			MeteringStartedAt: &later, ReleasedAt: &later},
		Data: map[string]any{"reason": "manual", "n": 2.0},
	}
	cases := map[string]Event{
		"full":    full,
		"minimal": {Type: EventPollFailed, Time: ts},
		"no-details": {Type: EventUserPending, Time: ts,
			User: &User{ID: 1, Username: "u"}, Setup: &Setup{Name: "s"}},
	}
	for name, ev := range cases {
		t.Run(name, func(t *testing.T) {
			b, err := json.Marshal(ev)
			if err != nil {
				t.Fatal(err)
			}
			var got Event
			if err := json.Unmarshal(b, &got); err != nil {
				t.Fatal(err)
			}
			if name == "full" {
				// RawMessage bytes are compared as JSON, not as text.
				if !reflect.DeepEqual(got.Setup.Labels, ev.Setup.Labels) {
					t.Fatalf("labels %v", got.Setup.Labels)
				}
				got.Setup.Details, ev.Setup.Details = nil, nil
			}
			if !reflect.DeepEqual(got, ev) {
				t.Fatalf("round trip mismatch\n got %+v\nwant %+v\njson %s", got, ev, b)
			}
		})
	}
}

func TestEXT6_EventJSONShape(t *testing.T) {
	loc := time.FixedZone("x", 3600)
	ts := time.Date(2026, 1, 2, 4, 4, 5, 0, loc)
	b, err := json.Marshal(Event{Type: EventHoldAcquired, Time: ts,
		Setup: &Setup{Name: "s", Details: json.RawMessage(`{"z": 1}`)},
		Hold:  &Hold{ID: 1, SetupName: "s", UserID: 2, AcquiredAt: ts}})
	if err != nil {
		t.Fatal(err)
	}
	var m map[string]any
	if err := json.Unmarshal(b, &m); err != nil {
		t.Fatal(err)
	}
	if m["time"] != "2026-01-02T03:04:05Z" {
		t.Errorf("time = %v, want UTC RFC 3339", m["time"])
	}
	if v, ok := m["user"]; !ok || v != nil {
		t.Errorf("user = %v, want explicit null", v)
	}
	if d, ok := m["data"].(map[string]any); !ok || len(d) != 0 {
		t.Errorf("data = %v, want {}", m["data"])
	}
	h := m["hold"].(map[string]any)
	for _, k := range []string{"id", "setup", "user_id", "acquired_at", "metering_started_at", "deployed_at", "release_requested_at", "released_at"} {
		if _, ok := h[k]; !ok {
			t.Errorf("hold missing key %q", k)
		}
	}
	if h["released_at"] != nil || h["setup"] != "s" {
		t.Errorf("hold = %v", h)
	}
	s := m["setup"].(map[string]any)
	if !reflect.DeepEqual(s["details"], map[string]any{"z": 1.0}) {
		t.Errorf("details = %v", s["details"])
	}
}
