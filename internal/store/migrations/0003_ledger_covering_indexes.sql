-- SPDX-License-Identifier: MIT
-- NFR scale (200 users, 100 setups, 5-minute polling): covering indexes so
-- the admin user list and the per-hold charged totals never scan the ledger.

-- Per-user period sums and month-to-date debits; its prefix (user_id, period)
-- replaces ledger_user_period.
DROP INDEX ledger_user_period;
CREATE INDEX ledger_user_period_type ON ledger (user_id, period, type, amount_mt);

-- Per-hold charged total (setups, holds, me).
CREATE INDEX ledger_hold_type ON ledger (hold_id, type, amount_mt);

-- Lab-wide period totals (debited and granted this month) of the admin list.
CREATE INDEX ledger_period_type ON ledger (period, type, amount_mt);
