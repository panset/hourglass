# 62b progress
- 2026-11-20 Read brief, 62 RESULT, 61 RESULT. Rewrote RequireService: verify first, limiter only on failed attempts (valid token never refused).
- 2026-11-20 Replaced addrLimiter: one lock for check+record, sweep at most once a minute, 10,000-address cap. Canonical decimal token ids.
- 2026-11-20 Tests added/changed; D47 rewritten, D50 added. go test -race ./internal/... and ./ci.sh pass.
