// SPDX-License-Identifier: MIT

package app

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/panset/hourglass/internal/api"
	"github.com/panset/hourglass/internal/config"
	"github.com/panset/hourglass/internal/store"
)

// MaxPreapprovedFileEntries is the limit on entries in preapproved_users_file (D4).
const MaxPreapprovedFileEntries = 10000

// preapprovedFile is the document named by preapproved_users_file
// (docs/design/ledger-labels-onboarding.md, D4). YAML and JSON share one
// decoder, as for the seed file: YAML is converted to JSON, then decoded
// strictly.
type preapprovedFile struct {
	Users []preapprovedUser `json:"users"`
}

type preapprovedUser struct {
	Email *string   `json:"email"`
	Roles *[]string `json:"roles"` // absent: [developer]
}

// PreapprovedFileError reports a preapproved_users_file that cannot be used;
// the server does not start. It names the entry by its index and never
// repeats a valid address.
type PreapprovedFileError struct{ Msg string }

func (e *PreapprovedFileError) Error() string { return "preapproved_users_file: " + e.Msg }

func preErrf(format string, a ...any) error {
	return &PreapprovedFileError{Msg: fmt.Sprintf(format, a...)}
}

// checkJSONKeys makes the JSON decoding strict where encoding/json is not: a
// key repeated in one object is an error (the decoder would keep the last
// value), and a known key must be spelled exactly (the decoder matches keys
// without regard to case).
func checkJSONKeys(raw []byte) error {
	dec := json.NewDecoder(bytes.NewReader(raw))
	var walk func(level int, inUsers bool) error
	walk = func(level int, inUsers bool) error {
		tok, err := dec.Token()
		if err != nil {
			return err
		}
		d, ok := tok.(json.Delim)
		if !ok {
			return nil
		}
		if d == '[' {
			for dec.More() {
				if err := walk(level, inUsers); err != nil {
					return err
				}
			}
			_, err := dec.Token()
			return err
		}
		seen := map[string]bool{}
		for dec.More() {
			kt, err := dec.Token()
			if err != nil {
				return err
			}
			k, _ := kt.(string)
			if seen[k] {
				return fmt.Errorf("duplicate key %q", shortenKey(k))
			}
			seen[k] = true
			isUsers := false
			switch {
			case level == 0:
				if !strings.EqualFold(k, "users") {
					break
				}
				if k != "users" {
					return fmt.Errorf("unknown field %q", shortenKey(k))
				}
				isUsers = true
			case inUsers && level == 1:
				for _, known := range []string{"email", "roles"} {
					if k != known && strings.EqualFold(k, known) {
						return fmt.Errorf("unknown field %q", shortenKey(k))
					}
				}
			}
			if err := walk(level+1, isUsers); err != nil {
				return err
			}
		}
		_, err = dec.Token()
		return err
	}
	return walk(0, false)
}

func shortenKey(k string) string {
	if r := []rune(k); len(r) > 40 {
		return string(r[:40]) + "..."
	}
	return k
}

// LoadPreapprovedUsers reads and validates the file overlay: a .json file is
// JSON, anything else YAML; unknown keys are errors; at most 10,000 entries;
// each email and role set follows the admin route's rules; an email may
// appear once. The file is only read, never written. An empty path is no file.
func LoadPreapprovedUsers(path string) ([]api.PreapprovalEntry, error) {
	if path == "" {
		return nil, nil
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, preErrf("%v", err)
	}
	raw := json.RawMessage(data)
	if strings.ToLower(filepath.Ext(path)) != ".json" {
		if raw, err = config.YAMLToJSONSingle(data); err != nil {
			return nil, preErrf("%s: %v", path, err)
		}
	}
	if err := checkJSONKeys(raw); err != nil {
		return nil, preErrf("%s: %v", path, err)
	}
	var f preapprovedFile
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&f); err != nil {
		return nil, preErrf("%s: %v", path, err)
	}
	if dec.More() {
		return nil, preErrf("%s: more than one document", path)
	}
	if len(f.Users) > MaxPreapprovedFileEntries {
		return nil, preErrf("%s: %d entries, at most %d are allowed", path, len(f.Users), MaxPreapprovedFileEntries)
	}
	out := make([]api.PreapprovalEntry, 0, len(f.Users))
	first := make(map[string]int, len(f.Users))
	for i, u := range f.Users {
		where := fmt.Sprintf("%s: users[%d]", path, i)
		if u.Email == nil {
			return nil, preErrf("%s: email is required", where)
		}
		email, err := api.ParsePreapprovalEmail(*u.Email)
		if err != nil {
			return nil, preErrf("%s: %v", where, err)
		}
		if j, dup := first[email]; dup {
			return nil, preErrf("%s: repeats the email of users[%d]", where, j)
		}
		first[email] = i
		roles := []string{store.RoleDeveloper}
		if u.Roles != nil {
			if roles, err = api.ParsePreapprovalRoles(*u.Roles); err != nil {
				return nil, preErrf("%s: %v", where, err)
			}
		}
		out = append(out, api.PreapprovalEntry{Email: email, Roles: roles})
	}
	return out, nil
}
