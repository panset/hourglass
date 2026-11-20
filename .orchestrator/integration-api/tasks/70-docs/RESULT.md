# Result: 70 documentation

Status: done (ci.sh result in "Commands"). No server-versus-design difference found.

## Part 1: docs/integrating.md run against the live stack
Stack: `e2e/integration-stack.sh up` (provider and pressure `reported`, `max_age: 10m`, poll set to 1m in Settings), a Python webhook receiver on host port 9100 (scripts in `work/`), alice (OIDC, approved as developer), a second account `readonly` with `state:read` only. Torn down at the end. Setups lab-01..06 from the sample seed (so `lab-07` in the guide's example is legitimately `unknown`).

| Example | Command run | Expected per guide | Actual | Verdict |
|---|---|---|---|---|
| PUT deployments | guide curl, as written | 200 with observed_at, received_at, expires_at, accepted, unknown | `accepted 1, unknown ["lab-07"]`, expires_at = observed_at + 10m | matches |
| PUT pressure | guide curl | (no response shown) | 200 `{value, observed_at, received_at, expires_at}` | guide corrected (response block added) |
| GET state | guide curl | setups, labels, details, enabled, hold, reports and expiry | also `server_time`, `poll`, `pressure.smoothed/multiplier`, `reported_deployed`, `hold.user`; reports null before first | guide corrected (shape and rules added) |
| Release hold | guide curl on hold 2 | hold ends at once, audited | 200 with the Hold JSON, `release_reason` set, `hold.released` with `data.service_account`, audit `hold.service_release` actor = account | guide corrected (response, 409, length) |
| 401 | no token; wrong token | 401 unauthenticated | 401; a user cookie on the route is also 401 | matches (cookie case added) |
| 403 | `readonly` token on PUT pressure and on release | 403 forbidden | 403 `not allowed` | matches |
| 404 | release hold 9999 | 404 not_found | 404 `Hold not found.` | matches |
| 409 stale_report | older observed_at, both PUTs | 409 | 409; an equal observed_at is accepted (200) | guide corrected (equal accepted) |
| 409 not_configured | second server with no `reported` config | 409, names the key | 409, `pressure.type is not set to reported ...` / `provider.type ...` | matches |
| 409 conflict | release hold 2 again | 409, treat as success | 409 `This hold is already released.` | matches |
| 400 validation | value 1.5; missing value; `"high"`; observed_at 2027; observed_at +90s (accepted); duplicate setup; `{}` and `null` deployments; empty name; 33 labels; 1.1 MB body; invalid JSON; missing reason | named causes | all 400; `{"deployments": []}` is 200 | guide corrected (full cause list) |
| 429 | 6 bad tokens from one address | 429 rate_limited | 5th failure still 401 (earlier failure counted), next 429 with `Retry-After: 900`; the valid token still 200 | matches (Retry-After added) |
| Omitted observed_at | PUT pressure `{"value":0.72}` | may be omitted | stored at receive time | guide corrected (says so) |
| Config snippet | `hourglassd -config` with the guide's YAML (URL and secret adjusted, plus listen/database/seed) | starts | healthz ok, no config error; `hold.releasing` accepted in `types` | matches |
| Webhook body and headers | captured `hold.acquired`, `hold.releasing`, `hold.released`, `poll.failed` | fields, `X-Hourglass-Event`, signature | all fields present incl. `release_requested_at`; `time` has nanoseconds, hold times ms; `Content-Type: application/json`; guide's Python snippet verifies every capture (needs bytes) | guide corrected (time precision, `data`, bytes comment) |
| Lifecycle 1 | alice acquires lab-03 | `hold.acquired` | delivered, signature ok | matches |
| Lifecycle 2 to 3 | report lab-03; poll | deployed, charging starts at next poll | `deployed_at` = `metering_started_at` = poll time (23:42:55), smoothed 0.72, multiplier 1 | matches |
| Lifecycle 4 | alice releases | `hold.releasing`, still charged, 202 | 202, `release_requested_at` set, one `hold.releasing`, `metering.active` true; second click 202 unchanged | matches |
| Lifecycle 5 to 6 | report without lab-03; poll | released at next poll, `hold.released` | released at 23:43:55 (poll), `released_at` = poll time, debit -0.033 (that poll's minute charged) | guide corrected (last interval charged) |
| If you stop reporting | test clock +11m | polls fail naming max_age, nobody charged, outage | poll.failed `... deployment report is 12m14s old, older than max_age 10m0s`; hold not released; after fresh reports an outage of 13 min, no debit in the gap | matches |
| Orphan | report lab-05 with labels, no hold | shown to admins with labels | collector `orphans` lists lab-05 with the labels | guide corrected (where) |
| Service release of a pending hold | user release, then service release (hold 4) | ends at once | 200, released | matches (sentence added) |

## Non-example facts checked
Default bands and 15-minute averaging (SPEC and settings: base 0, 0.5/0.75/0.9); 5-minute default and 1-minute minimum poll (`internal/settings/settings.go:42`); `max_age` default 10m, minimum 1m (D51, `internal/config/config.go`); 2-minute skew (server message); 30 seconds and 200 bytes and at-most-once (docs/extending.md, existing); webhook `types` must list `hold.acquiring` for a veto (extending.md); what admins see (collector orphans, outages).
Fixed: banner removed; setup note that polls fail until the first reports; token rotation; limits (10,000 / 32 / 1 MB); `username` rule; `hold.releasing` after a restart (D61); `hold.acquiring` needs `types`.

## username (task 68 follow-up)
Local admin: `username` "admin". OIDC user: `username` is "" in the webhook body and in the state `hold.user` (observed live); `email`, `name`, `id` are present. The guide now says to key on `user.id` (email if you need a human-readable key). The state response includes username, email and name, so no server-versus-design difference.

## Part 2
`docs/extending.md` (Hold.ReleaseRequestedAt, `hold.releasing` row and notes, JSON form, `reported` rows and section, `pkg/storedreports`, pointer to the guide, no `still_deployed`), `docs/deploying.md` (integration API section: config, `max_age` versus poll interval, service accounts and rotation, proxy note, Upgrade entry for 0005 and D43; the `release_requires_teardown` row), `README.md` (paragraph and a link; quick start's "Release is refused" fixed), `docs/progress.md` (section; REVIEW decisions D43 and D61 added to the table; the others on the "worth a look" line), `docs/ui-notes.md` (release note, settings note, the refusal item). `deploy/README.md`, `e2e/README.md`: no `still_deployed` text found. SPEC.md, design/ and wireframes untouched (`docs/wireframes/project/Main.dc.html` still carries the old note; it is an unchanged saved artboard and outside the brief's scope).

## Decisions
No row appended to decisions.md.

## Commands
`e2e/integration-stack.sh up|down`, curl, `go build ./cmd/hourglassd`, `./ci.sh` (ci.log, ci.exit). The stack is down (no hourglass containers or volumes remain).

## Confidence and gaps
- The config snippet was verified by starting a server with it; a delivery from that server (secret `a-long-shared-secret`) was not captured (no user to acquire on it); signature checks used the e2e stack's secret.
- `hold.acquiring` veto delivery was not re-run; the 200-byte statement rests on extending.md.
- The vsphere provider path was not exercised.
