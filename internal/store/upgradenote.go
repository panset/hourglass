// SPDX-License-Identifier: MIT

package store

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
)

// maxNoteValue caps how much of a stored template a note may carry.
const maxNoteValue = 100

// UpgradeNotes lists what this Open changed that an operator should know
// about, one line each. It is empty unless a migration dropped data.
func (s *Store) UpgradeNotes() []string { return s.notes }

// noteDroppedNaming runs just before migrations, on a database still below
// schema version 4 (D40). Migration 0004 deletes a stored resource-name
// template and per-setup overrides without a trace, so this records a note
// when either held a non-default value. Errors are ignored: the note is
// advisory and must never stop the upgrade.
func (s *Store) noteDroppedNaming(ctx context.Context, current int) {
	if current < 1 || current >= 4 {
		return
	}
	var overrides int
	if err := s.sqldb.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM setups WHERE resource_name_override IS NOT NULL AND resource_name_override <> ''`).Scan(&overrides); err != nil {
		overrides = 0
	}
	var tmpl string
	err := s.sqldb.QueryRowContext(ctx, `SELECT value_json FROM settings WHERE key = 'resource_name_template'`).Scan(&tmpl)
	if err != nil && err != sql.ErrNoRows {
		tmpl = ""
	}
	if tmpl == `"{{.SetupName}}"` || tmpl == `""` {
		tmpl = ""
	}
	if tmpl == "" && overrides == 0 {
		return
	}
	var parts []string
	if tmpl != "" {
		if r := []rune(tmpl); len(r) > maxNoteValue {
			tmpl = string(r[:maxNoteValue]) + "..."
		}
		parts = append(parts, fmt.Sprintf("the stored resource name template %s was removed", tmpl))
	}
	if overrides > 0 {
		parts = append(parts, fmt.Sprintf("%d setup(s) had a resource name override that was removed", overrides))
	}
	s.notes = append(s.notes, "migration 0004: "+strings.Join(parts, " and ")+
		"; setups now match the provider's reported name exactly, so set the provider's own name mapping (for example pool_name_prefix / pool_name_suffix) or setups will stop matching")
}
