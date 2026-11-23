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
		if raw, err = config.YAMLToJSON(data); err != nil {
			return nil, preErrf("%s: %v", path, err)
		}
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
