# 84b Fix the findings of verification 84

Job intent: `.orchestrator/ledger-labels-onboarding/INTENT.md` (read it first).
Agent contract: `.orchestrator/hourglass-v1/CONTRACT.md` ("the spec" means
`docs/design/ledger-labels-onboarding.md`). Your task folder is
`.orchestrator/ledger-labels-onboarding/tasks/84b-fix-findings/`.

## Goal

Close the six findings (1 medium, 5 low) in
`.orchestrator/ledger-labels-onboarding/tasks/84-verify-m6-m7/RESULT.md`.

## Context

Read that RESULT.md in full (file:line, failure scenario, suggested fix per
finding) and design sections C1, D1, D3, D4. D75 and D76 in
`docs/decisions.md` record the choices so far. This is sign-in code: take the
conservative option wherever access is granted.

## What to do (orchestrator's directions; follow them unless one contradicts
the design, in which case stop and report)

1. Medium, F1: add a committed `TestPRE3_` test that fails when the status
   re-check inside the approval transaction is removed (for example a user
   rejected or approved between the callback's read and the step's
   transaction). Prove it by removing the re-check, seeing red, restoring.
2. Look-alike characters: email comparison folds case for ASCII letters only,
   on both sides, in every place an email is normalised (admin route, file
   loader, claim). A non-ASCII character then matches only itself, so the
   Kelvin sign in a claim no longer matches an ASCII `k` entry. Use one shared
   normalising function. Tests under PRE-2.
3. Invisible characters: an email with a control or format character (Unicode
   categories Cc and Cf) is invalid in the admin route and the file, like
   whitespace. A claim containing one simply never matches. Tests under PRE-5
   and PRE-6.
4. File loader strictness: a second YAML document and a duplicate key (YAML or
   JSON) stop start-up with a message, like other invalid files. Check how the
   seed loader treats the same inputs and say so in RESULT.md; do not change
   the seed loader. Tests under PRE-6.
5. Name claims: a `name`, `given_name`, `family_name` or `preferred_username`
   claim that is not a string. First establish what commit d96a950 did with a
   non-string `name` claim (`git show d96a950:internal/auth/oidc.go`). The
   upgraded server must not refuse a sign-in that d96a950 accepted: the three
   new claims must never fail a sign-in (a non-string value counts as empty),
   and `name` must behave as it did at d96a950. Tests under AUTH-9.
6. The remaining low finding(s) in the RESULT.md: fix, or say why not.

## Scope

May write: `internal/auth/`, `internal/api/`, `internal/app/`,
`internal/config/`, `internal/store/` (only if normalisation lives there),
`BUILD.bazel` files Gazelle updates, your task folder.
Must not touch: `docs/`, `api/`, `ui/`, `e2e/`, git state. Do not weaken any
test.

## Done when

`go test ./internal/...` passes; `go test -race ./internal/auth/ ./internal/api/`
passes; `tools/check_core_generic.sh` passes; `./ci.sh` exits 0 with the
changed tests executed (record `ci.exit`). RESULT.md lists each finding and
what closed it, and "Decisions for docs/decisions.md".

## Long-running steps

`./ci.sh`: allow 10 minutes (timeout 600000), output to `ci.log` in your task
folder, read the exit code and tail only.

## Report

Follow CONTRACT.md. One final reply of at most ten lines.
