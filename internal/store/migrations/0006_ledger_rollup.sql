-- SPDX-License-Identifier: MIT
-- LED-7..9 (design A1, A2): the storage half of the ledger roll-up.

-- Number of raw debit rows a rolled-up row replaced; null for every other row.
ALTER TABLE ledger ADD COLUMN rollup_count INTEGER;

-- Compaction marker. Only the compaction step writes it: it inserts the one
-- row at its start and deletes it before it ends, inside its transaction, so
-- the row is never committed and no other connection ever sees it.
CREATE TABLE ledger_compaction (
    one INTEGER PRIMARY KEY CHECK (one = 1)
);

-- LED-1 as amended by D69: every delete still aborts, except a delete of a
-- debit row while the marker exists (compaction in progress). The update and
-- replace triggers are unchanged.
DROP TRIGGER ledger_no_delete;
CREATE TRIGGER ledger_no_delete BEFORE DELETE ON ledger
WHEN OLD.type <> 'debit' OR NOT EXISTS (SELECT 1 FROM ledger_compaction)
BEGIN
    SELECT RAISE(ABORT, 'ledger is append-only');
END;

-- Raw debits (the only rows compaction groups). Partial, so they hold about a
-- week of per-poll rows once the backlog is worked off. The first finds the
-- oldest due day and groups one day by a range seek; the second lets a step
-- reach one hold's rows of one day without walking the other holds' rows.
CREATE INDEX ledger_raw_debit_created ON ledger (created_at)
    WHERE type = 'debit' AND rollup_count IS NULL AND hold_id IS NOT NULL;
CREATE INDEX ledger_raw_debit_hold ON ledger (hold_id, created_at)
    WHERE type = 'debit' AND rollup_count IS NULL AND hold_id IS NOT NULL;
