// SPDX-License-Identifier: MIT

// Package store is the single SQLite database of Hourglass: embedded numbered
// migrations and typed access functions. It imports only the standard library
// and the pure-Go SQLite driver.
package store

import (
	"context"
	"database/sql"
	"embed"
	"errors"
	"fmt"
	"net/url"
	"os"
	"sort"
	"strconv"
	"strings"
	"sync/atomic"
	"time"

	"modernc.org/sqlite"
	sqlite3 "modernc.org/sqlite/lib"
)

//go:embed migrations/*.sql
var migrationFiles embed.FS

// Sentinel errors.
var (
	// ErrNotFound is returned when a row does not exist.
	ErrNotFound = errors.New("store: not found")
	// ErrSetupHeld is returned when a setup already has an active hold.
	ErrSetupHeld = errors.New("store: setup already held")
	// ErrConflict is returned for unique-constraint violations.
	ErrConflict = errors.New("store: conflict")
)

// TimeLayout is the fixed-width UTC RFC 3339 layout used for every stored
// timestamp, so that text order equals time order.
const TimeLayout = "2006-01-02T15:04:05.000Z"

// FormatTime renders t in the stored timestamp format: UTC, millisecond
// precision, fixed width (24 characters). Sub-millisecond precision is
// truncated, not rounded, so a time with nanoseconds does not round-trip;
// callers must not rely on finer than millisecond resolution.
func FormatTime(t time.Time) string { return t.UTC().Format(TimeLayout) }

// ParseTime parses a stored timestamp.
func ParseTime(s string) (time.Time, error) {
	t, err := time.Parse(TimeLayout, s)
	if err != nil {
		return time.Time{}, fmt.Errorf("store: bad timestamp %q: %w", s, err)
	}
	return t, nil
}

// dbtx is satisfied by *sql.DB and *sql.Tx.
type dbtx interface {
	ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error)
	QueryContext(ctx context.Context, query string, args ...any) (*sql.Rows, error)
	QueryRowContext(ctx context.Context, query string, args ...any) *sql.Row
}

// queries holds every read and write method; Store and Tx both embed it.
type queries struct {
	db    dbtx
	root  *sql.DB // non-nil only when not already inside a transaction
	count *atomic.Int64
}

// countDB counts the statements sent to the database (Store.StatementCount).
type countDB struct {
	dbtx
	n *atomic.Int64
}

func (c countDB) ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error) {
	c.n.Add(1)
	return c.dbtx.ExecContext(ctx, query, args...)
}

func (c countDB) QueryContext(ctx context.Context, query string, args ...any) (*sql.Rows, error) {
	c.n.Add(1)
	return c.dbtx.QueryContext(ctx, query, args...)
}

func (c countDB) QueryRowContext(ctx context.Context, query string, args ...any) *sql.Row {
	c.n.Add(1)
	return c.dbtx.QueryRowContext(ctx, query, args...)
}

func newQueries(db *sql.DB) queries {
	n := new(atomic.Int64)
	return queries{db: countDB{db, n}, root: db, count: n}
}

// StatementCount is the number of statements sent to the database through
// this store since it was opened. Tests use it to assert that a request runs a
// constant number of statements however many rows exist.
func (s *Store) StatementCount() int64 { return s.count.Load() }

// Store is an open database.
type Store struct {
	queries
	sqldb *sql.DB
	notes []string // see UpgradeNotes
}

// Tx is a transaction; it offers the same methods as Store.
type Tx struct {
	queries
}

// Open opens (creating if needed) the database at path, applies pending
// migrations and returns the store. A database whose schema is newer than
// this binary is an error.
func Open(path string) (*Store, error) {
	u := url.URL{Scheme: "file", Opaque: (&url.URL{Path: path}).EscapedPath()}
	q := url.Values{}
	q.Add("_pragma", "foreign_keys(1)")
	q.Add("_pragma", "busy_timeout(10000)")
	q.Add("_pragma", "journal_mode(WAL)")
	q.Add("_pragma", "synchronous(NORMAL)")
	q.Set("_txlock", "immediate")
	u.RawQuery = q.Encode()
	db, err := sql.Open("sqlite", u.String())
	if err != nil {
		return nil, err
	}
	if err := db.Ping(); err != nil {
		db.Close()
		return nil, fmt.Errorf("store: open %s: %w", path, err)
	}
	s := &Store{queries: newQueries(db), sqldb: db}
	if err := s.migrate(context.Background()); err != nil {
		db.Close()
		return nil, err
	}
	return s, nil
}

// OpenReadOnly opens an existing database without writing to it: no file is
// created, no migration or pragma that changes the file is run, and every
// write fails. A missing file is an error, as is a schema newer or older than
// this binary (it cannot be migrated read-only).
func OpenReadOnly(path string) (*Store, error) {
	if _, err := os.Stat(path); err != nil {
		return nil, fmt.Errorf("store: open %s read-only: %w", path, err)
	}
	u := url.URL{Scheme: "file", Opaque: (&url.URL{Path: path}).EscapedPath()}
	q := url.Values{}
	q.Set("mode", "ro")
	q.Add("_pragma", "foreign_keys(1)")
	q.Add("_pragma", "busy_timeout(10000)")
	q.Add("_pragma", "query_only(1)")
	u.RawQuery = q.Encode()
	db, err := sql.Open("sqlite", u.String())
	if err != nil {
		return nil, err
	}
	s := &Store{queries: newQueries(db), sqldb: db}
	migs, err := loadMigrations()
	if err != nil {
		db.Close()
		return nil, err
	}
	var current sql.NullInt64
	if err := db.QueryRow(`SELECT MAX(version) FROM schema_migrations`).Scan(&current); err != nil {
		db.Close()
		return nil, fmt.Errorf("store: open %s read-only: %w", path, err)
	}
	if want := migs[len(migs)-1].version; int(current.Int64) != want {
		db.Close()
		return nil, fmt.Errorf("store: database schema version %d differs from this binary's %d; start the server once to migrate it",
			current.Int64, want)
	}
	return s, nil
}

// Close closes the database.
func (s *Store) Close() error { return s.sqldb.Close() }

// Tx runs fn in a write transaction, committing if it returns nil and rolling
// back otherwise. A panic in fn rolls back, releases the connection and is
// re-raised.
func (s *Store) Tx(ctx context.Context, fn func(*Tx) error) error {
	tx, err := s.sqldb.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	done := false
	defer func() {
		if !done {
			_ = tx.Rollback()
		}
	}()
	if err := fn(&Tx{queries{db: countDB{tx, s.count}, count: s.count}}); err != nil {
		done = true
		_ = tx.Rollback()
		return err
	}
	done = true
	return tx.Commit()
}

// atomic runs fn inside a transaction, joining the current one if any.
func (q *queries) atomic(ctx context.Context, fn func(*queries) error) error {
	if q.root == nil {
		return fn(q)
	}
	tx, err := q.root.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	if err := fn(&queries{db: countDB{tx, q.count}, count: q.count}); err != nil {
		_ = tx.Rollback()
		return err
	}
	return tx.Commit()
}

type migration struct {
	version int
	name    string
	sql     string
}

func loadMigrations() ([]migration, error) {
	entries, err := migrationFiles.ReadDir("migrations")
	if err != nil {
		return nil, err
	}
	var out []migration
	for _, e := range entries {
		base := strings.TrimSuffix(e.Name(), ".sql")
		num, name, ok := strings.Cut(base, "_")
		v, err := strconv.Atoi(num)
		if !ok || err != nil {
			return nil, fmt.Errorf("store: bad migration name %q", e.Name())
		}
		b, err := migrationFiles.ReadFile("migrations/" + e.Name())
		if err != nil {
			return nil, err
		}
		out = append(out, migration{v, name, string(b)})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].version < out[j].version })
	return out, nil
}

func (s *Store) migrate(ctx context.Context) error {
	migs, err := loadMigrations()
	if err != nil {
		return err
	}
	if _, err := s.sqldb.ExecContext(ctx, `CREATE TABLE IF NOT EXISTS schema_migrations (
		version INTEGER PRIMARY KEY, name TEXT NOT NULL, applied_at TEXT NOT NULL)`); err != nil {
		return err
	}
	var current sql.NullInt64
	if err := s.sqldb.QueryRowContext(ctx, `SELECT MAX(version) FROM schema_migrations`).Scan(&current); err != nil {
		return err
	}
	if len(migs) == 0 || int(current.Int64) > migs[len(migs)-1].version {
		return fmt.Errorf("store: database schema version %d is newer than this binary (%d)",
			current.Int64, len(migs))
	}
	s.noteDroppedNaming(ctx, int(current.Int64))
	for _, m := range migs {
		if m.version <= int(current.Int64) {
			continue
		}
		err := s.Tx(ctx, func(tx *Tx) error {
			if _, err := tx.db.ExecContext(ctx, m.sql); err != nil {
				return err
			}
			_, err := tx.db.ExecContext(ctx,
				`INSERT INTO schema_migrations (version, name, applied_at) VALUES (?, ?, ?)`,
				m.version, m.name, FormatTime(time.Now()))
			return err
		})
		if err != nil {
			return fmt.Errorf("store: migration %04d_%s: %w", m.version, m.name, err)
		}
	}
	return nil
}

// mapErr translates driver errors to the package's sentinels.
func mapErr(err error) error {
	if err == nil {
		return nil
	}
	if errors.Is(err, sql.ErrNoRows) {
		return ErrNotFound
	}
	var se *sqlite.Error
	if errors.As(err, &se) {
		switch se.Code() {
		case sqlite3.SQLITE_CONSTRAINT_UNIQUE, sqlite3.SQLITE_CONSTRAINT_PRIMARYKEY:
			return fmt.Errorf("%w: %v", ErrConflict, err)
		}
	}
	return err
}

// nullable helpers.

func nullStr(s string) any {
	if s == "" {
		return nil
	}
	return s
}

func nullTime(t *time.Time) any {
	if t == nil {
		return nil
	}
	return FormatTime(*t)
}

func nullFloat(p *float64) any {
	if p == nil {
		return nil
	}
	return *p
}

func nullInt(p *int64) any {
	if p == nil {
		return nil
	}
	return *p
}

func boolInt(b bool) int {
	if b {
		return 1
	}
	return 0
}

func parseNullTime(ns sql.NullString) (*time.Time, error) {
	if !ns.Valid {
		return nil, nil
	}
	t, err := ParseTime(ns.String)
	if err != nil {
		return nil, err
	}
	return &t, nil
}

func nullInt64Ptr(n sql.NullInt64) *int64 {
	if !n.Valid {
		return nil
	}
	v := n.Int64
	return &v
}

func nullBoolPtr(n sql.NullInt64) *bool {
	if !n.Valid {
		return nil
	}
	v := n.Int64 != 0
	return &v
}

// rowScanner is satisfied by *sql.Row and *sql.Rows.
type rowScanner interface{ Scan(dest ...any) error }

// collect runs a query and maps every row.
func collect[T any](ctx context.Context, db dbtx, scan func(rowScanner) (T, error), query string, args ...any) ([]T, error) {
	rows, err := db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []T
	for rows.Next() {
		v, err := scan(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, v)
	}
	return out, rows.Err()
}

// isForeignKey reports whether err is a foreign-key violation.
func isForeignKey(err error) bool {
	var se *sqlite.Error
	return errors.As(err, &se) && se.Code() == sqlite3.SQLITE_CONSTRAINT_FOREIGNKEY
}
