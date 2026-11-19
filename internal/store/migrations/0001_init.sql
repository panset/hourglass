-- SPDX-License-Identifier: MIT
-- Initial schema. Timestamps are UTC RFC 3339 text with fixed width
-- (2006-01-02T15:04:05.000Z) so they sort as text; token amounts are integer
-- milli-tokens.

CREATE TABLE users (
    id                   INTEGER PRIMARY KEY AUTOINCREMENT,
    auth_source          TEXT NOT NULL CHECK (auth_source IN ('oidc', 'local')),
    oidc_sub             TEXT UNIQUE,
    username             TEXT,
    email                TEXT NOT NULL DEFAULT '',
    name                 TEXT NOT NULL DEFAULT '',
    status               TEXT NOT NULL CHECK (status IN ('pending', 'active', 'rejected')),
    labels_json          TEXT NOT NULL DEFAULT '{}',
    password_hash        TEXT,
    must_change_password INTEGER NOT NULL DEFAULT 0 CHECK (must_change_password IN (0, 1)),
    balance_mt           INTEGER NOT NULL DEFAULT 0,
    created_at           TEXT NOT NULL,
    last_login_at        TEXT
);

CREATE TABLE user_roles (
    user_id INTEGER NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    role    TEXT NOT NULL CHECK (role IN ('developer', 'admin')),
    PRIMARY KEY (user_id, role)
);

CREATE TABLE setups (
    id                     INTEGER PRIMARY KEY AUTOINCREMENT,
    name                   TEXT NOT NULL UNIQUE,
    resource_name_override TEXT,
    details_json           TEXT NOT NULL DEFAULT '{}',
    enabled                INTEGER NOT NULL DEFAULT 1 CHECK (enabled IN (0, 1))
);

CREATE TABLE setup_labels (
    setup_id INTEGER NOT NULL REFERENCES setups (id) ON DELETE CASCADE,
    key      TEXT NOT NULL,
    value    TEXT NOT NULL,
    PRIMARY KEY (setup_id, key)
);
CREATE INDEX setup_labels_key_value ON setup_labels (key, value);

CREATE TABLE rules (
    id         INTEGER PRIMARY KEY AUTOINCREMENT,
    kind       TEXT NOT NULL,
    position   INTEGER NOT NULL,
    selector   TEXT NOT NULL,
    value      REAL NOT NULL,
    updated_by INTEGER REFERENCES users (id),
    updated_at TEXT NOT NULL
);
CREATE INDEX rules_kind_position ON rules (kind, position);

CREATE TABLE holds (
    id                   INTEGER PRIMARY KEY AUTOINCREMENT,
    setup_id             INTEGER NOT NULL REFERENCES setups (id),
    user_id              INTEGER NOT NULL REFERENCES users (id),
    acquired_at          TEXT NOT NULL,
    metering_started_at  TEXT,
    deployed_at          TEXT,
    released_at          TEXT,
    released_by          INTEGER REFERENCES users (id),
    release_reason       TEXT
);
CREATE UNIQUE INDEX holds_one_active_per_setup ON holds (setup_id) WHERE released_at IS NULL;
CREATE INDEX holds_user ON holds (user_id);

CREATE TABLE usage_samples (
    id            INTEGER PRIMARY KEY AUTOINCREMENT,
    hold_id       INTEGER NOT NULL REFERENCES holds (id),
    polled_at     TEXT NOT NULL,
    weight        REAL NOT NULL,
    rule_selector TEXT,
    pressure      REAL NOT NULL,
    multiplier    REAL NOT NULL,
    debit_mt      INTEGER NOT NULL,
    reason        TEXT NOT NULL DEFAULT '',
    UNIQUE (hold_id, polled_at)
);
CREATE INDEX usage_samples_polled_at ON usage_samples (polled_at);

CREATE TABLE ledger (
    id         INTEGER PRIMARY KEY AUTOINCREMENT,
    user_id    INTEGER NOT NULL REFERENCES users (id),
    period     TEXT NOT NULL CHECK (period GLOB '[0-9][0-9][0-9][0-9]-[0-9][0-9]'),
    type       TEXT NOT NULL CHECK (type IN ('allowance', 'grant', 'debit', 'expiry', 'carryover', 'reset')),
    amount_mt  INTEGER NOT NULL,
    hold_id    INTEGER REFERENCES holds (id),
    actor_id   INTEGER REFERENCES users (id),
    reason     TEXT NOT NULL DEFAULT '',
    created_at TEXT NOT NULL
);
CREATE INDEX ledger_user_period ON ledger (user_id, period);

CREATE TRIGGER ledger_no_update BEFORE UPDATE ON ledger
BEGIN
    SELECT RAISE(ABORT, 'ledger is append-only');
END;

CREATE TRIGGER ledger_no_delete BEFORE DELETE ON ledger
BEGIN
    SELECT RAISE(ABORT, 'ledger is append-only');
END;

CREATE TABLE pressure_samples (
    polled_at  TEXT NOT NULL PRIMARY KEY,
    source     TEXT NOT NULL,
    raw        REAL NOT NULL,
    smoothed   REAL NOT NULL,
    multiplier REAL NOT NULL
);

CREATE TABLE orphans (
    resource_name TEXT NOT NULL PRIMARY KEY,
    first_seen_at TEXT NOT NULL,
    last_seen_at  TEXT NOT NULL,
    labels_json   TEXT NOT NULL DEFAULT '{}'
);

CREATE TABLE event_deliveries (
    id           INTEGER PRIMARY KEY AUTOINCREMENT,
    event_type   TEXT NOT NULL,
    hook         TEXT NOT NULL,
    payload_json TEXT NOT NULL DEFAULT '{}',
    ok           INTEGER CHECK (ok IN (0, 1)),
    error        TEXT NOT NULL DEFAULT '',
    started_at   TEXT NOT NULL,
    finished_at  TEXT
);
CREATE INDEX event_deliveries_started_at ON event_deliveries (started_at);

CREATE TABLE settings (
    key        TEXT NOT NULL PRIMARY KEY,
    value_json TEXT NOT NULL,
    updated_by INTEGER REFERENCES users (id),
    updated_at TEXT NOT NULL
);

CREATE TABLE audit_log (
    id          INTEGER PRIMARY KEY AUTOINCREMENT,
    actor_id    INTEGER REFERENCES users (id),
    action      TEXT NOT NULL,
    target      TEXT NOT NULL DEFAULT '',
    before_json TEXT,
    after_json  TEXT,
    created_at  TEXT NOT NULL
);
CREATE INDEX audit_log_created ON audit_log (created_at);

CREATE TABLE sessions (
    id           TEXT NOT NULL PRIMARY KEY,
    user_id      INTEGER NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    created_at   TEXT NOT NULL,
    last_seen_at TEXT NOT NULL,
    expires_at   TEXT NOT NULL
);
CREATE INDEX sessions_user ON sessions (user_id);

CREATE TABLE collector_runs (
    id          INTEGER PRIMARY KEY AUTOINCREMENT,
    started_at  TEXT NOT NULL,
    finished_at TEXT,
    ok          INTEGER CHECK (ok IN (0, 1)),
    error       TEXT NOT NULL DEFAULT ''
);

CREATE TABLE outages (
    id         INTEGER PRIMARY KEY AUTOINCREMENT,
    started_at TEXT NOT NULL,
    ended_at   TEXT NOT NULL,
    reason     TEXT NOT NULL DEFAULT ''
);
