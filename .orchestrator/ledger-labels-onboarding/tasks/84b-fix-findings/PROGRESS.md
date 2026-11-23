# Progress 84b
- 2026-11-23 Read BRIEF, INTENT, CONTRACT, 84 RESULT. Checked d96a950 oidc.go: `Name string` claim, so a non-string name failed sign-in with "bad id token claims".
- Done: shared ASCII-only NormalizeEmail + Cc/Cf refusal (internal/api/preapproval.go); config.YAMLToJSONSingle + checkJSONKeys for the file loader (seed loader untouched); given/family/preferred decoded leniently (internal/auth/oidc.go).
- Done: tests added (api recheck + concurrency, PRE-2 look-alike/invisible, PRE-5 invisible, PRE-6 loader cases, AUTH-9 claims). Break probes run and restored.
- Done: go test ./internal/... ok.
- Next: race, core-generic, ./ci.sh, RESULT.md.
- Done: race, core-generic, ci.sh exit 0; RESULT.md written.
