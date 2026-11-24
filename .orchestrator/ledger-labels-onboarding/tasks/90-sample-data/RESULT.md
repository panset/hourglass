# 90 sample data for alice (stack hourglass-llo)

Local admin password now: `dev-only-manual-test`. Alice signs in with Keycloak `alice` / `dev-only-alice`.

## What was inserted (UTC, "now" = 2026-11-24 00:57)
The volume `hourglass-llo_data` was reset once (hourglass.db removed, fresh start) so alice's rows could be written in id order
(ledger order and balance_after follow id; an allowance written at sign-in would have sat before history dated earlier).
Alice was NOT pre-approved: she signed in pending, history was inserted, then she was given `developer` through the admin API.
dave, bob, carol are untouched; no pre-approvals exist.

- 3 holds: lab-01 (medium, weight 1) 2026-10-30 12:02 to 2026-11-11 18:57, released; lab-03 (large, weight 2) 11-08 to 11-21 21:57, released;
  lab-05 (extralarge, weight 4) from 11-16, still active. One usage_sample plus one debit per hold per 5-minute poll (first poll of a hold has debit 0
  and no ledger row, as the collector writes), multiplier 1 at night, 2 in working hours, 3 at 14:00 UTC.
- 9,842 samples; ledger 9,844 rows: Oct allowance 300, Oct expiry (closing balance), Nov allowance 300, grants 3,000 (Nov 1 "Dev stack starter credit")
  and 500 (Nov 15 15:22 "Extra tokens for load testing", mid-day), 431 Oct and 9,408 Nov debits.
- users.balance_mt = ledger sum of 2026-11 = 1,539.212 tokens (same rule as RebuildBalances); never below 0 running.
- After start the collector compacted by itself on its first cycle: 4 log lines `compacted ledger` removing 1864+1710+1949+849 = 6,372 raw rows (23 groups).

## Redo
Fresh volume (`docker compose -p hourglass-llo ... down -v`, up), then from the repo root:
1. `01-admin-and-signin.sh` (admin password change), `02-alice-signin.sh` (alice pending),
2. `docker stop hourglass-llo-hourglass-1`; copy the db out with an alpine container (`cp -a /data/. /out/`), run `python3 03-insert-history.py <copy>/hourglass.db`,
   copy it back and `chown 65532:65532`; `docker start hourglass-llo-hourglass-1`,
3. `04-approve-alice.sh`.
Keycloak cookies are Secure, so curl cannot keep them for http://keycloak.localhost; script 02 copies them into a Cookie header by hand.
