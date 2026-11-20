# 66b progress

- 2026-11-20 Read brief, intent, contract, 66 RESULT.md.
- 2026-11-20 Ruling 4: analysed; hold.releasing was queued after commit, so a racing release could queue hold.released first. Fixed by queuing inside the transaction (D60), with a test.
- 2026-11-20 Rulings 1, 6, 7: tests added in providers/reported, internal/collector. Ruling 2: internal/app/storedreports_test.go. Ruling 3: internal/holds/pending_test.go. Ruling 5: stub code and not_implemented removed; D49 marked superseded. D60, D61 appended.
- 2026-11-20 Ruling 8: mutations run with scratchpad/mut66b.py; all red, git diff shasum identical before and after.
- 2026-11-20 go test -race -count=1 ./... exit 0; ./ci.sh exit 0. Done.
