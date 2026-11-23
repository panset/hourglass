-- SPDX-License-Identifier: MIT
-- Pre-approved SSO users (docs/design/ledger-labels-onboarding.md, D2): an
-- email and its roles. A pending SSO user whose email claim matches is
-- approved at sign-in. An entry is never consumed; used_at and used_by record
-- its first use. Entries of the optional file overlay are never stored here.
CREATE TABLE preapprovals (
    id          INTEGER PRIMARY KEY AUTOINCREMENT,
    email       TEXT NOT NULL UNIQUE,      -- trimmed, lower case
    roles_json  TEXT NOT NULL,             -- non-empty, from developer and admin
    created_by  INTEGER REFERENCES users (id),
    created_at  TEXT NOT NULL,
    used_at     TEXT,                      -- first use
    used_by     INTEGER REFERENCES users (id)
);
