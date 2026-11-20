-- SPDX-License-Identifier: MIT
-- Integration API (push model): service accounts and their tokens, the last
-- stored pressure and deployment reports, and the columns that record a
-- pending release and a release or audit entry made by a service account.
-- Service account names on holds and audit rows are plain text, not foreign
-- keys, so deleting an account never rewrites history.
CREATE TABLE service_accounts (
    id          INTEGER PRIMARY KEY AUTOINCREMENT,
    name        TEXT NOT NULL UNIQUE,
    description TEXT NOT NULL DEFAULT '',
    scopes_json TEXT NOT NULL DEFAULT '[]',
    enabled     INTEGER NOT NULL DEFAULT 1,
    created_by  INTEGER REFERENCES users (id),
    created_at  TEXT NOT NULL
);

CREATE TABLE service_tokens (
    id           INTEGER PRIMARY KEY AUTOINCREMENT,
    account_id   INTEGER NOT NULL REFERENCES service_accounts (id) ON DELETE CASCADE,
    token_hash   TEXT NOT NULL,
    created_at   TEXT NOT NULL,
    expires_at   TEXT,
    last_used_at TEXT
);
CREATE INDEX service_tokens_account ON service_tokens (account_id);

-- Single-row tables: the store replaces the row, it never accumulates.
CREATE TABLE reported_pressure (
    value           REAL NOT NULL,
    observed_at     TEXT NOT NULL,
    received_at     TEXT NOT NULL,
    service_account TEXT NOT NULL
);

CREATE TABLE reported_deployments_meta (
    observed_at     TEXT NOT NULL,
    received_at     TEXT NOT NULL,
    service_account TEXT NOT NULL
);

CREATE TABLE reported_deployments (
    setup_name  TEXT PRIMARY KEY,
    labels_json TEXT NOT NULL DEFAULT '{}'
);

ALTER TABLE holds ADD COLUMN release_requested_at TEXT;
ALTER TABLE holds ADD COLUMN released_by_service TEXT;
ALTER TABLE audit_log ADD COLUMN actor_service TEXT;
