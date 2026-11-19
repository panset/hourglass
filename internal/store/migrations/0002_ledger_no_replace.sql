-- SPDX-License-Identifier: MIT
-- LED-1: INSERT OR REPLACE deletes the old row without firing the DELETE
-- trigger (recursive_triggers is off), so also refuse any insert whose id
-- already exists, whatever the conflict-resolution clause.
CREATE TRIGGER ledger_no_replace BEFORE INSERT ON ledger
WHEN NEW.id IS NOT NULL AND EXISTS (SELECT 1 FROM ledger WHERE id = NEW.id)
BEGIN
    SELECT RAISE(ABORT, 'ledger is append-only');
END;
