// SPDX-License-Identifier: MIT

package store

import (
	"context"
	"database/sql"
)

// Setup is a row of setups with its labels.
type Setup struct {
	ID          int64
	Name        string
	DetailsJSON string
	Enabled     bool
	Labels      map[string]string
	// WeightOverride is the admin's pinned weight (design D2), nil when none.
	// CreateSetup writes it; UpdateSetup leaves it alone, and only
	// SetSetupWeightOverride changes it on an existing setup.
	WeightOverride *float64
}

func scanSetup(r rowScanner) (Setup, error) {
	var (
		s  Setup
		en int
		wo sql.NullFloat64
	)
	err := r.Scan(&s.ID, &s.Name, &s.DetailsJSON, &en, &wo)
	s.Enabled = en != 0
	if wo.Valid {
		v := wo.Float64
		s.WeightOverride = &v
	}
	s.Labels = map[string]string{}
	return s, err
}

const setupCols = `id, name, details_json, enabled, weight_override`

func (q *queries) loadLabels(ctx context.Context, setups []Setup) error {
	if len(setups) == 0 {
		return nil
	}
	idx := make(map[int64]*Setup, len(setups))
	for i := range setups {
		idx[setups[i].ID] = &setups[i]
	}
	rows, err := q.db.QueryContext(ctx, `SELECT setup_id, key, value FROM setup_labels`)
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		var id int64
		var k, v string
		if err := rows.Scan(&id, &k, &v); err != nil {
			return err
		}
		if s, ok := idx[id]; ok {
			s.Labels[k] = v
		}
	}
	return rows.Err()
}

func (q *queries) setLabels(ctx context.Context, id int64, labels map[string]string) error {
	if _, err := q.db.ExecContext(ctx, `DELETE FROM setup_labels WHERE setup_id = ?`, id); err != nil {
		return err
	}
	for k, v := range labels {
		if _, err := q.db.ExecContext(ctx, `INSERT INTO setup_labels (setup_id, key, value) VALUES (?, ?, ?)`, id, k, v); err != nil {
			return mapErr(err)
		}
	}
	return nil
}

// CreateSetup inserts a setup, its labels and its weight override (nil for
// none) and returns its id. A duplicate name returns ErrConflict.
func (q *queries) CreateSetup(ctx context.Context, s Setup) (int64, error) {
	var id int64
	err := q.atomic(ctx, func(q *queries) error {
		details := s.DetailsJSON
		if details == "" {
			details = "{}"
		}
		res, err := q.db.ExecContext(ctx, `INSERT INTO setups (name, details_json, enabled)
			VALUES (?, ?, ?)`, s.Name, details, boolInt(s.Enabled))
		if err != nil {
			return mapErr(err)
		}
		id, _ = res.LastInsertId()
		if s.WeightOverride != nil {
			// A separate statement, so a setup without an override is
			// inserted exactly as before migration 0008.
			if _, err := q.db.ExecContext(ctx, `UPDATE setups SET weight_override = ? WHERE id = ?`, *s.WeightOverride, id); err != nil {
				return mapErr(err)
			}
		}
		return q.setLabels(ctx, id, s.Labels)
	})
	return id, err
}

// UpdateSetup updates a setup's name, details and enabled flag and replaces
// its labels as a set. It does not change the weight override (see
// SetSetupWeightOverride).
func (q *queries) UpdateSetup(ctx context.Context, s Setup) error {
	return q.atomic(ctx, func(q *queries) error {
		details := s.DetailsJSON
		if details == "" {
			details = "{}"
		}
		res, err := q.db.ExecContext(ctx, `UPDATE setups SET name = ?,
			details_json = ?, enabled = ? WHERE id = ?`,
			s.Name, details, boolInt(s.Enabled), s.ID)
		if err != nil {
			return mapErr(err)
		}
		if n, _ := res.RowsAffected(); n == 0 {
			return ErrNotFound
		}
		return q.setLabels(ctx, s.ID, s.Labels)
	})
}

// SetSetupWeightOverride sets a setup's weight override, or clears it when w
// is nil (design D2). Validating the value is the caller's job. An unknown id
// returns ErrNotFound.
func (q *queries) SetSetupWeightOverride(ctx context.Context, id int64, w *float64) error {
	res, err := q.db.ExecContext(ctx, `UPDATE setups SET weight_override = ? WHERE id = ?`, nullFloat(w), id)
	if err != nil {
		return mapErr(err)
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return ErrNotFound
	}
	return nil
}

// DeleteSetup deletes a setup (its labels go with it). A setup that has
// holds cannot be deleted and returns ErrConflict.
func (q *queries) DeleteSetup(ctx context.Context, id int64) error {
	res, err := q.db.ExecContext(ctx, `DELETE FROM setups WHERE id = ?`, id)
	if err != nil {
		if isForeignKey(err) {
			return ErrConflict
		}
		return mapErr(err)
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return ErrNotFound
	}
	return nil
}

// GetSetup returns a setup with labels.
func (q *queries) GetSetup(ctx context.Context, id int64) (Setup, error) {
	s, err := scanSetup(q.db.QueryRowContext(ctx, `SELECT `+setupCols+` FROM setups WHERE id = ?`, id))
	if err != nil {
		return s, mapErr(err)
	}
	list := []Setup{s}
	if err := q.loadLabels(ctx, list); err != nil {
		return s, err
	}
	return list[0], nil
}

// ListSetups lists setups ordered by name, optionally only enabled ones.
func (q *queries) ListSetups(ctx context.Context, enabledOnly bool) ([]Setup, error) {
	query := `SELECT ` + setupCols + ` FROM setups`
	if enabledOnly {
		query += ` WHERE enabled = 1`
	}
	list, err := collect(ctx, q.db, scanSetup, query+` ORDER BY name`)
	if err != nil {
		return nil, err
	}
	return list, q.loadLabels(ctx, list)
}
