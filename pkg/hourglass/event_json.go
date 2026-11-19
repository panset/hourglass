// SPDX-License-Identifier: MIT

package hourglass

import (
	"bytes"
	"encoding/json"
	"time"
)

// The JSON form of an Event is a stable, snake_case object. It is what the
// webhook hook sends and what the core stores as a delivery payload.
//
//	{
//	  "type": "hold.acquired",
//	  "time": "2026-01-02T03:04:05Z",            // RFC 3339, UTC
//	  "user": {"id": 1, "username": "...", "email": "...", "name": "...",
//	           "roles": ["developer"]} | null,
//	  "setup": {"name": "...", "labels": {"k": "v"}, "details": <any JSON>} | null,
//	  "hold": {"id": 1, "setup": "<setup name>", "user_id": 1,
//	           "acquired_at": "...", "metering_started_at": "..." | null,
//	           "deployed_at": "..." | null, "released_at": "..." | null} | null,
//	  "data": {}                                  // always an object
//	}
//
// Setup.Details is passed through unmodified. Nil labels, roles and details
// encode as null; an empty Data map (or nil) encodes as {}.

type eventJSON struct {
	Type  string         `json:"type"`
	Time  time.Time      `json:"time"`
	User  *userJSON      `json:"user"`
	Setup *setupJSON     `json:"setup"`
	Hold  *holdJSON      `json:"hold"`
	Data  map[string]any `json:"data"`
}

type userJSON struct {
	ID       int64    `json:"id"`
	Username string   `json:"username"`
	Email    string   `json:"email"`
	Name     string   `json:"name"`
	Roles    []string `json:"roles"`
}

type setupJSON struct {
	Name    string            `json:"name"`
	Labels  map[string]string `json:"labels"`
	Details json.RawMessage   `json:"details"`
}

type holdJSON struct {
	ID                int64      `json:"id"`
	Setup             string     `json:"setup"`
	UserID            int64      `json:"user_id"`
	AcquiredAt        time.Time  `json:"acquired_at"`
	MeteringStartedAt *time.Time `json:"metering_started_at"`
	DeployedAt        *time.Time `json:"deployed_at"`
	ReleasedAt        *time.Time `json:"released_at"`
}

func utcPtr(t *time.Time) *time.Time {
	if t == nil {
		return nil
	}
	u := t.UTC()
	return &u
}

// MarshalJSON encodes the event in the documented form.
func (e Event) MarshalJSON() ([]byte, error) {
	j := eventJSON{Type: e.Type, Time: e.Time.UTC(), Data: e.Data}
	if j.Data == nil {
		j.Data = map[string]any{}
	}
	if u := e.User; u != nil {
		j.User = &userJSON{ID: u.ID, Username: u.Username, Email: u.Email, Name: u.Name, Roles: u.Roles}
	}
	if s := e.Setup; s != nil {
		j.Setup = &setupJSON{Name: s.Name, Labels: s.Labels, Details: s.Details}
		if len(s.Details) == 0 {
			j.Setup.Details = nil // encodes as null
		}
	}
	if h := e.Hold; h != nil {
		j.Hold = &holdJSON{
			ID: h.ID, Setup: h.SetupName, UserID: h.UserID, AcquiredAt: h.AcquiredAt.UTC(),
			MeteringStartedAt: utcPtr(h.MeteringStartedAt),
			DeployedAt:        utcPtr(h.DeployedAt),
			ReleasedAt:        utcPtr(h.ReleasedAt),
		}
	}
	return json.Marshal(j)
}

// UnmarshalJSON decodes the documented form. Absent or null user, setup and
// hold become nil pointers; an empty data object becomes a nil map.
func (e *Event) UnmarshalJSON(b []byte) error {
	var j eventJSON
	if err := json.Unmarshal(b, &j); err != nil {
		return err
	}
	out := Event{Type: j.Type, Time: j.Time}
	if len(j.Data) > 0 {
		out.Data = j.Data
	}
	if u := j.User; u != nil {
		out.User = &User{ID: u.ID, Username: u.Username, Email: u.Email, Name: u.Name, Roles: u.Roles}
	}
	if s := j.Setup; s != nil {
		out.Setup = &Setup{Name: s.Name, Labels: s.Labels}
		if len(s.Details) > 0 && !bytes.Equal(s.Details, []byte("null")) {
			out.Setup.Details = s.Details
		}
	}
	if h := j.Hold; h != nil {
		out.Hold = &Hold{
			ID: h.ID, SetupName: h.Setup, UserID: h.UserID, AcquiredAt: h.AcquiredAt,
			MeteringStartedAt: h.MeteringStartedAt, DeployedAt: h.DeployedAt, ReleasedAt: h.ReleasedAt,
		}
	}
	*e = out
	return nil
}
