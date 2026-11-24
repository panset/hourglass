-- SPDX-License-Identifier: MIT
-- Reported weights (docs/design/reported-weights.md): the last stored weights
-- report, the source of each usage sample's weight, and the per-setup weight
-- override. Additive only: no existing row is rewritten. Rows written before
-- this migration keep a null weight_source and a null weight_override.

-- Single-row table, like reported_deployments_meta: the store replaces the
-- row, it never accumulates.
CREATE TABLE reported_weights_meta (
    observed_at     TEXT NOT NULL,
    received_at     TEXT NOT NULL,
    service_account TEXT NOT NULL
);

-- The stored set. setup_name is a name, not a foreign key: a name that
-- matches no setup is stored too, and the weight follows the name.
CREATE TABLE reported_weights (
    setup_name      TEXT PRIMARY KEY,
    weight          REAL NOT NULL,
    attributes_json TEXT NOT NULL DEFAULT '{}'
);

-- override, reported, rule, default or policy; null before this migration.
ALTER TABLE usage_samples ADD COLUMN weight_source TEXT;
-- The admin's pinned weight for one setup; null when none.
ALTER TABLE setups ADD COLUMN weight_override REAL;
