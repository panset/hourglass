// SPDX-License-Identifier: MIT

package store

import (
	"path/filepath"
	"reflect"
	"testing"
)

// D40: migration 0004 on a database made by schema version 3 (built by
// openAtVersion from the embedded migrations 0001-0003) holding a setup with
// an override, labels, a hold and the old template setting. Open drops the
// column and the stored setting; every setup row, its labels, its hold and
// the other settings survive. This file and the migration it tests are the
// only core files allowed to name the removed column and setting
// (tools/check_core_generic.sh).
func TestD40_Migration0004DropsResourceNameOverride(t *testing.T) {
	path := filepath.Join(t.TempDir(), "h.db")
	s := openAtVersion(t, path, 3)
	for _, q := range []string{
		`INSERT INTO users (auth_source, username, email, name, status, labels_json, created_at) VALUES ('local', 'u', 'u@example.com', 'U', 'active', '{}', '2026-11-01T00:00:00.000Z')`,
		`INSERT INTO setups (name, resource_name_override, details_json, enabled) VALUES ('lab-01', 'other-name', '{"a":1}', 1), ('lab-02', NULL, '{}', 0)`,
		`INSERT INTO setup_labels (setup_id, key, value) VALUES (1, 'tier', 'gold'), (2, 'tier', 'silver')`,
		`INSERT INTO holds (setup_id, user_id, acquired_at) VALUES (1, 1, '2026-11-01T00:00:00.000Z')`,
		`INSERT INTO settings (key, value_json, updated_at) VALUES ('resource_name_template', '"{{.SetupName}}-x"', '2026-11-01T00:00:00.000Z'), ('default_weight', '2', '2026-11-01T00:00:00.000Z')`,
	} {
		if _, err := s.sqldb.Exec(q); err != nil {
			t.Fatalf("%.60s: %v", q, err)
		}
	}
	s.Close()

	s = must(Open(path))
	defer func() { s.Close() }()
	var v, cols, tmpl, other int
	var ic string
	s.sqldb.QueryRow(`SELECT MAX(version) FROM schema_migrations`).Scan(&v)
	s.sqldb.QueryRow(`SELECT COUNT(*) FROM pragma_table_info('setups') WHERE name = 'resource_name_override'`).Scan(&cols)
	s.sqldb.QueryRow(`SELECT COUNT(*) FROM settings WHERE key = 'resource_name_template'`).Scan(&tmpl)
	s.sqldb.QueryRow(`SELECT COUNT(*) FROM settings WHERE key = 'default_weight'`).Scan(&other)
	s.sqldb.QueryRow(`PRAGMA integrity_check`).Scan(&ic)
	if v != 4 || cols != 0 || tmpl != 0 || other != 1 || ic != "ok" {
		t.Fatalf("after upgrade: version %d, override column %d, template rows %d, other settings %d, integrity %q", v, cols, tmpl, other, ic)
	}
	want := []Setup{
		{ID: 1, Name: "lab-01", DetailsJSON: `{"a":1}`, Enabled: true, Labels: map[string]string{"tier": "gold"}},
		{ID: 2, Name: "lab-02", DetailsJSON: `{}`, Enabled: false, Labels: map[string]string{"tier": "silver"}},
	}
	if l := must(s.ListSetups(ctx, false)); !reflect.DeepEqual(l, want) {
		t.Fatalf("setups after upgrade %+v", l)
	}
	if hs := must(s.ActiveHolds(ctx)); len(hs) != 1 || hs[0].SetupID != 1 {
		t.Fatalf("holds after upgrade %+v", hs)
	}
	id := must(s.CreateSetup(ctx, Setup{Name: "lab-03", Enabled: true}))
	if got := must(s.GetSetup(ctx, id)); got.Name != "lab-03" || !got.Enabled {
		t.Fatalf("create after upgrade %+v", got)
	}
	s.Close()
	s = must(Open(path)) // reopen: nothing to run
	var n int
	s.sqldb.QueryRow(`SELECT COUNT(*) FROM schema_migrations`).Scan(&n)
	if n != 4 {
		t.Errorf("schema_migrations rows = %d after reopen", n)
	}
}
