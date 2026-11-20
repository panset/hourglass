# Result: 60 store migration 0005, queries, internal/reported

Status: done. `go test -race -count=1 ./internal/...` passes; `./ci.sh` exits 0 (ci.log).

## What was built
- `internal/store/migrations/0005_integration_api.sql`: the tables and columns of the design's Data model, nothing else. `service_tokens.account_id` is `ON DELETE CASCADE`; holds and audit rows hold the account name as text. The three report tables have no id column (D46).
- Store queries: `service_accounts.go`, `reported.go`; `holds.go` (new columns in the model and scans); `misc.go` (audit `ActorService`).
- `internal/reported`: ordering, skew, defaulting, atomic replace, reads with age. Imports only `internal/store`.
- Tests: `TestINT1_`, `TestINT2_` (concurrent readers, failed replace), `TestINT3_` (older refused for both reports, equal idempotent), `TestINT6_` (reopen), clock skew, empty set, and in the store: every new query, cascade delete keeps hold/audit names, migration 0005 on a v4 database, migration count and data-model checks.

## Choices
- Touch rule (at most once a minute) lives in the store (D44). INT-1 value range and duplicate-name validation live in `internal/reported`; size limits stay for the HTTP handler (D45).
- Decision rows appended: D44, D45, D46.

## Existing tests changed (only because of the new columns or migration count)
- `internal/store/store_test.go`: migration count 4 to 5, data-model spec map, and `TestNFR_Migration0003UpgradesVersion2Database` now inserts its hold by raw SQL (CreateHold reads columns that do not exist at schema v2) and expects version 5.
- `internal/store/migrate0004_test.go`: expects version 5 and 5 rows.

## Files changed
docs/decisions.md; internal/store/{BUILD.bazel,holds.go,misc.go,store_test.go,migrate0004_test.go,service_accounts.go,reported.go,integration_queries_test.go,migrations/0005_integration_api.sql}; internal/reported/{reported.go,reported_test.go,BUILD.bazel}; this task folder.

## API: store (methods on Store and Tx)
```go
type ServiceAccount struct{ ID int64; Name, Description string; Scopes []string; Enabled bool; CreatedBy *int64; CreatedAt time.Time }
type ServiceToken struct{ ID, AccountID int64; TokenHash string; CreatedAt time.Time; ExpiresAt, LastUsedAt *time.Time }
type ServiceAccountWithTokens struct{ ServiceAccount; Tokens []ServiceToken }
const LastUsedMinInterval = time.Minute
CreateServiceAccount(ctx, a ServiceAccount, now time.Time) (ServiceAccount, error)   // ErrConflict on duplicate name
GetServiceAccount(ctx, id int64) (ServiceAccount, error)
GetServiceAccountByName(ctx, name string) (ServiceAccount, error)
ListServiceAccounts(ctx) ([]ServiceAccountWithTokens, error)
UpdateServiceAccount(ctx, id int64, description string, scopes []string, enabled bool) (ServiceAccount, error)
DeleteServiceAccount(ctx, id int64) error
InsertServiceToken(ctx, accountID int64, tokenHash string, expiresAt *time.Time, now time.Time) (ServiceToken, error)
GetServiceTokenWithAccount(ctx, tokenID int64) (ServiceToken, ServiceAccount, error)  // does not judge expiry/enabled
RevokeServiceToken(ctx, accountID, tokenID int64) error
TouchServiceToken(ctx, tokenID int64, now time.Time) (written bool, err error)
type ReportedPressure struct{ Value float64; ObservedAt, ReceivedAt time.Time; ServiceAccount string }
type ReportedHeader struct{ ObservedAt, ReceivedAt time.Time; ServiceAccount string }
type ReportedDeployment struct{ SetupName string; Labels map[string]string }
GetReportedPressure(ctx) (ReportedPressure, error)               // ErrNotFound before first
ReplaceReportedPressure(ctx, p ReportedPressure) error
GetReportedDeployments(ctx) (ReportedHeader, []ReportedDeployment, error)  // one snapshot; ErrNotFound before first
ReplaceReportedDeployments(ctx, h ReportedHeader, set []ReportedDeployment) error
// Hold gains ReleaseRequestedAt *time.Time, ReleasedByService string
SetReleaseRequested(ctx, id int64, t time.Time) (set bool, err error)
ReleaseHoldByService(ctx, id int64, service, reason string, now time.Time) error  // ErrConflict if released
// AuditEntry gains ActorService string (AppendAudit, ListAudit)
```
Store replace methods do not compare timestamps; use `internal/reported` for that.

## API: internal/reported
```go
const MaxClockSkew = 2 * time.Minute
var ErrNoReport error
type StaleError struct{ Stored, Reported time.Time }      // map to 409 stale_report
type ValidationError struct{ Message string }              // map to 400 validation
type Clock interface{ Now() time.Time }
type Pressure struct{ Value float64; ObservedAt, ReceivedAt time.Time; ServiceAccount string; Age time.Duration }
type Deployment struct{ Setup string; Labels map[string]string }
type Deployments struct{ ObservedAt, ReceivedAt time.Time; ServiceAccount string; Deployments []Deployment; Age time.Duration }
func New(st *store.Store, clock Clock) *Service
func (*Service) SubmitPressure(ctx, account string, value float64, observedAt *time.Time) (Pressure, error)
func (*Service) SubmitDeployments(ctx, account string, observedAt *time.Time, d []Deployment) (Deployments, error)
func (*Service) Pressure(ctx) (Pressure, error)            // ErrNoReport before first
func (*Service) Deployments(ctx) (Deployments, error)
func Fresh(age, maxAge time.Duration) bool
func ExpiresAt(observedAt time.Time, maxAge time.Duration) time.Time
```

## Confidence and gaps
- Verified by tests and ci.sh. The INT-2 concurrency test depends on SQLite write-transaction serialisation; it appears robust but is a race test, not a proof.
- Not done (out of scope): matching names to setups (`unknown` list), size limits, HTTP, auth, extensions, release behaviour (task 65).
