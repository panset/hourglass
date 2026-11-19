# Extending Hourglass

Hourglass meters how long people hold shared setups. It does not know what a setup is, how it gets deployed, how busy the lab is, or when charging should start. You supply those four things as Go packages, build your own binary, and leave the core untouched.

This page covers the four extension interfaces, the event list, how to build a binary with your own extensions, and the bundled extensions. A complete third-party build lives in [`examples/custom`](../examples/custom/README.md).

## What the core knows

| The core does | The core does not |
| --- | --- |
| Store setups as a name, free-form labels and an opaque `details` JSON blob | Interpret any label key or any part of `details` |
| Turn labels into a weight with the admin's rate rules (first match wins, default weight 1) | Know what `size=large` means |
| Poll on an interval, build a `Snapshot`, and ask your extensions for allocations, pressure and metering decisions | Talk to vCenter or any other backend |
| Smooth pressure over 15 minutes, pick a multiplier from the admin's bands, debit `weight x multiplier x elapsed hours` | Decide how pressure is measured or when metering starts |
| Emit events and hand them to hooks | Deploy, power off or delete anything |

The core never writes to your infrastructure. Extensions receive a read-only view and never touch the database.

## The four interfaces

These are the declarations in [`pkg/hourglass/types.go`](../pkg/hourglass/types.go), copied as they are.

```go
// Setup is what every extension sees of a setup. The core never
// interprets Labels or Details.
type Setup struct {
	Name    string
	Labels  map[string]string
	Details json.RawMessage
}

// ResourceProvider reports what is currently deployed. Optional:
// with none configured, every other extension receives no allocations.
type ResourceProvider interface {
	Name() string
	List(ctx context.Context) ([]Allocation, error)
}

// Allocation is one deployed resource as reported by a ResourceProvider.
type Allocation struct {
	ResourceName string            // matched to a setup by the name template
	Labels       map[string]string // provider-specific, shown to admins
}

// PressureSource reports how contended the lab is, from 0.0 to 1.0.
// An error or a value outside 0 to 1 makes the poll fail.
type PressureSource interface {
	Name() string
	Pressure(ctx context.Context, s Snapshot) (float64, error)
}

// MeteringPolicy decides, per hold and per poll, whether to charge.
type MeteringPolicy interface {
	Name() string
	Decide(ctx context.Context, h HoldState, s Snapshot) (Decision, error)
}

// Decision is a MeteringPolicy's verdict for one hold in one poll.
type Decision struct {
	Meter  bool
	Weight float64 // 0 means use the weight from the rate rules
	Reason string  // shown to the user, e.g. "2 h since acquire"
}

// Hook receives every event. An error returned for a vetoable event
// (hold.acquiring) aborts the action; other errors are logged.
// Each call has a 30-second timeout.
type Hook interface {
	Name() string
	Handle(ctx context.Context, e Event) error
}

// Event is delivered to every configured hook. Type is one of the Event*
// constants.
type Event struct {
	Type  string // e.g. "hold.acquired"
	Time  time.Time
	User  *User  // nil when not about a user
	Setup *Setup // nil when not about a setup
	Hold  *Hold  // nil when not about a hold
	Data  map[string]any
}

// User mirrors a row of the users table together with its roles.
type User struct {
	ID       int64
	Username string
	Email    string
	Name     string
	Roles    []string // "developer", "admin"; empty means unassigned
}

// Hold mirrors a row of the holds table. Nil times have not happened yet.
type Hold struct {
	ID                int64
	SetupName         string
	UserID            int64
	AcquiredAt        time.Time
	MeteringStartedAt *time.Time
	DeployedAt        *time.Time
	ReleasedAt        *time.Time
}
```

### Snapshot, HoldState and Decision

```go
// SetupState is an enabled setup as the core resolves it for one poll.
type SetupState struct {
	Setup
	// Weight is the weight resolved from the rate rules, or the default
	// weight when no rule matches.
	Weight float64
	// ResourceName is the name a provider is expected to report for this
	// setup, from the resource name template or the setup's override.
	ResourceName string
}

// Snapshot is the read-only view of the lab for one poll. Extensions need
// no database access.
type Snapshot struct {
	// Time is when the poll started.
	Time time.Time
	// Setups lists the enabled setups.
	Setups []SetupState
	// Holds lists the active (unreleased) holds.
	Holds []Hold
	// Allocations is what the provider reported for this poll; empty when
	// ProviderConfigured is false.
	Allocations []Allocation
	// ProviderConfigured reports whether a ResourceProvider is configured.
	ProviderConfigured bool
}

// HoldState is one active hold with everything a MeteringPolicy needs.
type HoldState struct {
	Hold  Hold
	Setup SetupState
	User  User
	// Allocation is the provider's allocation for the setup, or nil when
	// the provider does not report it or none is configured.
	Allocation *Allocation
}

func (s Snapshot) SetupByName(name string) (SetupState, bool)
func (s Snapshot) AllocationFor(setupName string) (Allocation, bool)
```

Rules for these types:

- **Use `Snapshot.Time`, never the wall clock.** `Time` is when the poll started. The core can run on an injected clock, and the conformance suite calls `Decide` twice with the same input and expects the same answer. A policy that reads `time.Now()` fails that check.
- **Treat everything as read-only.** The same `Snapshot` goes to the pressure source and to every `Decide` call. The conformance suites check that you do not modify it.
- **`Decision.Weight` of 0 means "use the rate rules".** A policy cannot override a hold to a weight of exactly 0. To make a setup free, return `Meter: false`, or give it a rate rule of weight 0. A negative or non-finite weight fails the poll.
- **Set `Decision.Reason` whenever `Meter` is true.** Users see it on the dashboard. The conformance suite checks it.
- **`Snapshot.Setups` lists enabled setups only.** `Snapshot.Holds` lists active holds only. Allocations with the same resource name are merged into one before you see them.
- **`SetupState.Weight` is already resolved from the rate rules**, for the pressure source and the policy alike. Only a snapshot built by `hourglassd check-provider` has weight 0 (see [Check a provider](#check-a-provider)).

## What the core guarantees when it calls you

| | `ResourceProvider.List` | `PressureSource.Pressure` | `MeteringPolicy.Decide` | `Hook.Handle` |
| --- | --- | --- | --- | --- |
| Called | Once per poll, first | Once per poll, after `List` | Once per active hold per poll, after `Pressure` | Once per event per hook, in config order |
| Timeout | 30 s | 30 s | 30 s | 30 s |
| Concurrency | Never two polls at once, so never two calls at once from the collector | Same | Same, holds are decided one after another | Background events: one at a time, in order. `hold.acquiring`: inside the request, so several can run at the same time for different users |
| On error or timeout | The poll fails | The poll fails | The poll fails | `hold.acquiring`: the acquire is refused. Others: logged and recorded, nothing else happens |
| On panic | The poll fails | The poll fails | The poll fails | Treated as an error |

What "the poll fails" means: the collector records a failed run, emits `poll.failed`, writes no debits and no usage samples, and nobody is charged for that poll. The next poll runs on schedule. A value outside 0 to 1 from a pressure source (including NaN) fails the poll the same way.

Other details you can rely on:

- Each call gets a context with a 30-second deadline. A call that ignores its context is abandoned at the deadline, not killed. Honour the context.
- A timeout is an error. For `hold.acquiring` the user sees `hook <name> timed out`.
- Metering starts at the first poll where `Decide` returns `Meter: true`. That poll sets `metering_started_at` and debits nothing. Each later poll in which `Decide` returns `Meter: true` debits the time since the previous successful poll (at most twice the poll interval). A poll where it returns `Meter: false` debits nothing. The core does not remember "metering is on": your policy decides every poll. The bundled `default` policy keeps returning true once `HoldState.Hold.MeteringStartedAt` is set.
- When `Decision.Weight` is greater than 0 it replaces the weight from the rate rules for that poll.
- `Provider.List` is the only source of `hold.deployed`: the event fires the first poll where the provider reports the hold's resource. With no provider, holds are never marked deployed.

## Register an extension

An extension package defines a type, a factory, and an `init` function that registers the factory under a name. The config file selects extensions by that name.

```go
func RegisterProvider(name string, f ProviderFactory)
func RegisterPressureSource(name string, f PressureFactory)
func RegisterMeteringPolicy(name string, f PolicyFactory)
func RegisterHook(name string, f HookFactory)

type ProviderFactory func(config json.RawMessage) (ResourceProvider, error)
type PressureFactory func(config json.RawMessage) (PressureSource, error)
type PolicyFactory func(config json.RawMessage) (MeteringPolicy, error)
type HookFactory func(config json.RawMessage) (Hook, error)
```

Registering an empty name, a nil factory, or a name twice panics.

### The config block

The config file names each extension with `type` and an optional `config` block:

```yaml
provider:
  type: fixed
  config:
    names: [lab-01_RP]
```

The factory receives the `config` mapping as JSON, whatever the file's syntax. Keys other than `type` and `config` in that entry are rejected. Some rules:

- The block is `nil` or empty when the file gives none. A factory with no required settings must accept that. `null` counts as none.
- Every YAML mapping key must be a string. YAML timestamps arrive as RFC 3339 strings.
- Hourglass logs the config with every extension block replaced by `[redacted]`, because blocks can hold secrets. Do the same in your own logs and error messages. Never put a secret in an error.
- Reject unknown keys with `json.Decoder.DisallowUnknownFields`, so a typo fails at startup. Build everything the extension needs in the factory and do not touch the network there: a factory error stops startup.
- An unknown `type` fails startup with the registered names, for example `unknown pressure source "nope" (registered: held-count, inventory)`.

The config selects at most one provider, exactly one pressure source, exactly one metering policy, and any number of hooks, in order. A pressure source or policy that the config does not name falls back to the default your `main` passes to `hourglass.Run`.

## Minimal examples

These four files compile together as one package and pass the conformance suites. They are the shortest useful version of each interface. For richer examples that read config, group by label and veto acquires, see [`examples/custom`](../examples/custom/README.md).

### Provider

```go
package mylab

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"

	"github.com/panset/hourglass/pkg/hourglass"
)

// decode reads an extension's config block. The block is nil or empty when
// the config file gives none.
func decode(raw json.RawMessage, dst any) error {
	if t := bytes.TrimSpace(raw); len(t) == 0 || string(t) == "null" {
		return nil
	}
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.DisallowUnknownFields()
	if err := dec.Decode(dst); err != nil {
		return fmt.Errorf("invalid config: %w", err)
	}
	return nil
}

// fixedProvider reports a fixed list of resources as deployed.
type fixedProvider struct{ names []string }

func (fixedProvider) Name() string { return "fixed" }

func (p fixedProvider) List(ctx context.Context) ([]hourglass.Allocation, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	out := make([]hourglass.Allocation, 0, len(p.names))
	for _, n := range p.names {
		out = append(out, hourglass.Allocation{ResourceName: n})
	}
	return out, nil
}

func init() {
	hourglass.RegisterProvider("fixed", func(raw json.RawMessage) (hourglass.ResourceProvider, error) {
		var c struct {
			Names []string `json:"names"`
		}
		if err := decode(raw, &c); err != nil {
			return nil, err
		}
		return fixedProvider{names: c.Names}, nil
	})
}
```

A provider reports what exists. The core matches each `ResourceName` to a setup through the resource name template (default `{{.SetupName}}`) or the setup's own override. Allocations that match no setup are ignored. An allocation that matches a setup with no active hold is recorded as an orphan and shown to admins. Return an error when you cannot see the backend: the poll fails and nobody is charged for the gap. Do not return an empty list in that case, because an empty list means "nothing is deployed".

### Pressure source

```go
package mylab

import (
	"context"
	"encoding/json"

	"github.com/panset/hourglass/pkg/hourglass"
)

// heldCount is the share of enabled setups that are held, ignoring weights.
type heldCount struct{}

func (heldCount) Name() string { return "held-count" }

func (heldCount) Pressure(ctx context.Context, s hourglass.Snapshot) (float64, error) {
	if len(s.Setups) == 0 {
		return 0, nil
	}
	return float64(len(s.Holds)) / float64(len(s.Setups)), nil
}

func init() {
	hourglass.RegisterPressureSource("held-count", func(json.RawMessage) (hourglass.PressureSource, error) {
		return heldCount{}, nil
	})
}
```

Return a number from 0 to 1. The core averages the raw values over the last 15 minutes and looks the result up in the admin's bands (by default 0x below 50%, 1x from 50%, 2x from 75%, 3x from 90%). The multiplier of the admin's bands applies to the smoothed value, so a pressure source does not need to smooth.

### Metering policy

```go
package mylab

import (
	"context"
	"encoding/json"

	"github.com/panset/hourglass/pkg/hourglass"
)

// weekdays meters only from Monday to Friday, in UTC. It leaves Weight at 0
// so the rate rules decide the weight.
type weekdays struct{}

func (weekdays) Name() string { return "weekdays" }

func (weekdays) Decide(ctx context.Context, h hourglass.HoldState, s hourglass.Snapshot) (hourglass.Decision, error) {
	// Use the snapshot's time, not time.Now: the core may run on another clock.
	switch s.Time.UTC().Weekday() {
	case 0, 6:
		return hourglass.Decision{Meter: false, Reason: "weekend: not metering"}, nil
	}
	return hourglass.Decision{Meter: true, Reason: "weekday"}, nil
}

func init() {
	hourglass.RegisterMeteringPolicy("weekdays", func(json.RawMessage) (hourglass.MeteringPolicy, error) {
		return weekdays{}, nil
	})
}
```

### Hook

```go
package mylab

import (
	"context"
	"encoding/json"
	"fmt"
	"os"

	"github.com/panset/hourglass/pkg/hourglass"
)

// printer writes one JSON line per event to standard error and refuses to
// let anyone acquire a setup labelled frozen=true.
type printer struct{}

func (printer) Name() string { return "printer" }

func (printer) Handle(ctx context.Context, e hourglass.Event) error {
	if e.Type == hourglass.EventHoldAcquiring && e.Setup != nil && e.Setup.Labels["frozen"] == "true" {
		return fmt.Errorf("%s is frozen for maintenance", e.Setup.Name) // shown to the user
	}
	b, err := json.Marshal(e)
	if err != nil {
		return err
	}
	fmt.Fprintln(os.Stderr, string(b))
	return nil
}

func init() {
	hourglass.RegisterHook("printer", func(json.RawMessage) (hourglass.Hook, error) {
		return printer{}, nil
	})
}
```

`Handle` is called for every event type, so check `e.Type` first and guard `e.User`, `e.Setup` and `e.Hold` against nil: the conformance suite calls it with all three nil. The error returned for `hold.acquiring` is shown to the user verbatim, so write it for a person and keep internals out of it.

## Build your own binary

A custom binary needs three kinds of import. Nothing in it imports `internal/`, which Go would refuse from another module anyway.

```go
package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/panset/hourglass/pkg/hourglass"
	_ "github.com/panset/hourglass/pkg/hourglass/server" // links hourglass.Run

	_ "example.com/lab/mylab"
	_ "github.com/panset/hourglass/policies/default"
	_ "github.com/panset/hourglass/pressure/inventory"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	err := hourglass.Run(ctx, hourglass.Options{
		DefaultPressureSource: "inventory",
		DefaultMeteringPolicy: "default",
		Args:                  os.Args[1:],
	})
	if err != nil {
		log.Fatal(err)
	}
}
```

1. **`pkg/hourglass`** supplies `Run` and `Options`.
2. **`pkg/hourglass/server`**, imported for its side effect, links the implementation behind `Run`. Without it, `Run` returns an error that names this import.
3. **Blank imports of every extension the config may name**, yours and the bundled ones. Importing does not enable an extension; the config does.

The defaults matter. The core names no concrete extension, so a config that omits `pressure:` or `metering:` falls back to the names in `Options`. If you pass a name that no imported package registers, the binary starts and then fails with `unknown metering policy "default" (registered: none)`. The bundled defaults live in `pressure/inventory` and `policies/default`, which are public packages. Import them as above, or register your own and name them in `Options`.

In a module that requires `github.com/panset/hourglass`, build and run it like this:

```sh
CGO_ENABLED=0 go build -o mylabd ./cmd/mylabd
HOURGLASS_SESSION_KEY="$(head -c 32 /dev/urandom | base64)" \
HOURGLASS_ADMIN_PASSWORD="choose-a-long-local-password" \
  ./mylabd -config hourglass.yaml
```

### `hourglass.Options`

| Field | Meaning |
| --- | --- |
| `ConfigPath` | YAML config file. The `-config` flag in `Args` wins, then `HOURGLASS_CONFIG`. With no path at all you get the defaults plus the environment. |
| `DefaultPressureSource`, `DefaultMeteringPolicy` | Names used when the config names none. |
| `Args` | Command-line arguments after the program name. `-config` is the only flag. Handle your own subcommands before calling `Run`. |
| `Stdout`, `Stderr` | The JSON log goes to `Stdout`; flag and fatal errors go to `Stderr`. Nil means the process streams. |
| `UI` | Static files served at `/` in bundled mode (an `fs.FS` with `index.html` at its root). Nil means API-only. |
| `Clock` | Anything with `Now() time.Time`. Nil means the wall clock. Used by tests. |

`Run` blocks until its context is cancelled, then shuts down in order (HTTP, collector, event queue, database) within 10 seconds and returns nil. A third-party binary cannot embed the bundled UI, because the embedding sits behind a build tag inside this repository. Run it API-only and serve the `hourglass-ui` image beside it ([split mode](deploying.md#bundled-and-split-mode)), or pass your own `fs.FS` in `Options.UI`.

## Events

Hooks receive every event. `hourglass.EventTypes()` returns the list in this order; `hourglass.Vetoable(type)` tells you whether an error aborts the action.

| Type | Fires when | Vetoable | `user` | `setup` | `hold` | `data` |
| --- | --- | --- | --- | --- | --- | --- |
| `hold.acquiring` | A user asks to acquire, after the balance, limit and free-setup checks pass | Yes | the user | yes | yes, `id` is 0 (it does not exist yet) | `{}` |
| `hold.acquired` | The hold is created | No | the user | yes | yes | `{}` |
| `hold.metering_started` | The policy first says to charge | No | the holder | yes | yes, `metering_started_at` set | `{}` |
| `hold.deployed` | The provider first reports the setup | No | the holder | yes | yes, `deployed_at` set | `{}` |
| `hold.released` | The hold ends, by the user or an admin | No | the holder, not the admin who released it | yes | yes, `released_at` set | `{"forced": bool, "reason": string}` |
| `balance.low` | A balance first drops below 10% of the allowance | No | the user | null | null | `{"balance_tokens": number, "allowance_tokens": number}` |
| `balance.overdrawn` | A balance first reaches zero or below | No | the user | null | null | `{"balance_tokens": number, "allowance_tokens": number}` |
| `user.pending` | A new user signs in for the first time | No | the new user, `roles` empty | null | null | `{}` |
| `user.approved` | An admin assigns a first role | No | the user, with the new roles | null | null | `{}` |
| `period.closed` | The monthly rollover finishes | No | null | null | null | `{"period": "2026-10", "next": "2026-11", "users": 12}` |
| `poll.failed` | A collector run fails | No | null | null | null | `{"error": string}` |

Notes on the table:

- `balance.low` and `balance.overdrawn` can both fire on the same debit. `balance.low` never fires when the allowance is zero.
- `hold.released` carries `reason` as an empty string when a user releases their own hold.
- `hold.metering_started`, `hold.deployed` and the balance events come from the poll, and their `time` is the poll time. If the same poll produces several, they are queued in that order: metering started, deployed, then balance events.
- A hook may filter by event type. The core delivers every event to every hook and records each delivery; filtering is the hook's job (the bundled `webhook` hook has a `types` list).

### JSON form

`Event` marshals to a stable snake_case object. This is what the `webhook` hook sends and what the core stores as the delivery payload. `json.Marshal(e)` and `json.Unmarshal` give you the same form in your own code.

```json
{
  "type": "hold.acquired",
  "time": "2026-11-19T16:30:00Z",
  "user": {"id": 2, "username": "alice", "email": "alice@example.com", "name": "Alice Example", "roles": ["developer"]},
  "setup": {"name": "lab-03", "labels": {"size": "large"}, "details": {"note": "sample setup"}},
  "hold": {
    "id": 7, "setup": "lab-03", "user_id": 2,
    "acquired_at": "2026-11-19T16:30:00Z",
    "metering_started_at": null, "deployed_at": null, "released_at": null
  },
  "data": {}
}
```

That payload was produced by `json.Marshal` on an `hourglass.Event` (the values are made up and the output is reformatted). Times are RFC 3339 in UTC. `user`, `setup` and `hold` are `null` when the event is not about one. `data` is always an object. `setup.details` is passed through as JSON; it may be re-compacted but is otherwise unchanged. Nil labels, roles and details encode as `null`.

A payload the core really sent, captured from the `webhook` hook while a poll failed because the `static` provider's file was missing:

```json
{"type":"poll.failed","time":"2026-11-19T16:35:42.519Z","user":null,"setup":null,"hold":null,"data":{"error":"provider static: static: read allocations: open missing.json: no such file or directory"}}
```

## Delivery semantics

| | |
| --- | --- |
| Order | One in-memory queue, delivered in the order events were emitted. For each event the hooks are called one after another, in config order. |
| Where | `hold.acquiring` runs inside the user's HTTP request. Every other event goes through the background queue, so a slow hook never delays a poll or a request. It does delay later events and later hooks in the queue. |
| Timeout | 30 seconds per call. A hook that starts a long job should enqueue the job and return. |
| Veto | For `hold.acquiring`, hooks run in config order. The first error or timeout stops delivery, refuses the acquire with HTTP 409 and code `hook_veto`, and shows the hook's message to the user. The hold is not created. The core checks the hold limit again after your hook returns, because your hook ran outside the database transaction. |
| Retries | None. Delivery is at most once. |
| Failure | Logged, recorded in `event_deliveries`, and listed for admins as failed deliveries on the collector page. It never blocks anything. |
| Records | One `event_deliveries` row per (event, hook) with the payload, outcome and times. Rows are pruned after 90 days. A hook that filters an event out still gets a row. |
| Shutdown | The queue is lost. Events still waiting are dropped (the log says how many). The call in flight is allowed to finish within the 10-second shutdown bound, then cancelled. |
| Queue size | Unbounded in v1. A hook that never returns within 30 seconds can let the queue grow. |

## The webhook hook

`webhook` POSTs each event as JSON, so automation needs no Go.

```yaml
hooks:
  - type: webhook
    config:
      url: https://automation.example.com/hourglass
      secret: a-long-shared-secret
      types: [hold.acquired, hold.released, balance.low]
```

| Key | Meaning |
| --- | --- |
| `url` | Required. An absolute `http` or `https` URL. |
| `secret` | Optional. When set, each request is signed. |
| `types` | Optional list of event types. Default: all. An empty list sends nothing. An unknown type fails startup. |

Each request is a `POST` with these headers:

| Header | Value |
| --- | --- |
| `Content-Type` | `application/json` |
| `X-Hourglass-Event` | The event type, for example `hold.acquired` |
| `X-Hourglass-Signature` | Only with a secret: `sha256=` and the hex HMAC-SHA256 of the raw request body, keyed with the secret |

Any status outside 200 to 299, or a transport error, is a failed delivery. Redirects are not followed. For `hold.acquiring`, the first 200 bytes of the response body (with the secret redacted) become the veto message shown to the user, so a vetoing endpoint should return a short human-readable body. Transport errors are reduced to a class such as `timeout` or `connection failed` so the URL never leaks.

To verify a signature, compute the HMAC over the raw bytes you received, before parsing:

```go
func verify(secret, body []byte, header string) bool {
	mac := hmac.New(sha256.New, secret)
	mac.Write(body)
	want := "sha256=" + hex.EncodeToString(mac.Sum(nil))
	return hmac.Equal([]byte(want), []byte(header))
}
```

Imports are `crypto/hmac`, `crypto/sha256` and `encoding/hex`. The same check in Python:

```python
import hashlib, hmac
want = "sha256=" + hmac.new(secret, body, hashlib.sha256).hexdigest()
ok = hmac.compare_digest(want, header)
```

The Python check was run against the captured `poll.failed` delivery above and matched.

## Test an extension

`pkg/hourglasstest` has fakes, a snapshot builder and one conformance suite per interface. Each check is a subtest named after the contract it verifies. Run each suite from its own test function:

```go
func TestProviderConformance(t *testing.T) {
	hourglasstest.TestProvider(t, func() hourglass.ResourceProvider { return newMine(t) })
}

func TestPressureConformance(t *testing.T) {
	hourglasstest.TestPressureSource(t, func() hourglass.PressureSource { return newMine() },
		hourglasstest.NewSnapshot().Setup("a").Setup("b").Hold("a", 1).Build())
}

func TestPolicyConformance(t *testing.T) {
	hourglasstest.TestMeteringPolicy(t, func() hourglass.MeteringPolicy { return newMine() })
}

func TestHookConformance(t *testing.T) {
	hourglasstest.TestHook(t, func() hourglass.Hook { return newMine() })
}
```

| Suite | What it checks |
| --- | --- |
| `TestProvider` | Name is non-empty and stable; `List` honours cancellation promptly, is repeatable when the backend is unchanged and safe for concurrent use; no allocation has an empty `ResourceName` |
| `TestPressureSource` | Name; the value is finite and within 0 to 1, or an error; cancellation; the snapshot is not mutated |
| `TestMeteringPolicy` | Name; weight is finite and not negative; `Reason` is set when `Meter` is true; the same input gives the same decision; cancellation; inputs are not mutated |
| `TestHook` | Name; every event type is accepted with nil `User`, `Setup` and `Hold`; `Handle` returns within the context deadline; an unknown event type is tolerated |

Point `TestHook` at a test double, not a live service: it sends every event type.

The fakes (`FakeProvider`, `FakePressureSource`, `FakePolicy`, `FakeHook`, `FakeClock`) record calls and let you set values and errors. `hourglasstest.NewSnapshot()` builds a `Snapshot` with `.Setup(...)`, `.Hold(...)`, `.Allocation(...)`, `.At(...)` and `.ProviderConfigured(...)`. Run `go doc github.com/panset/hourglass/pkg/hourglasstest` for the full list.

To run the suites that ship with the repository:

```sh
go test -race -count=1 ./pkg/... ./providers/... ./pressure/... ./policies/... ./hooks/... ./examples/...
```

## Check a provider

`hourglassd check-provider [-config file]` builds only the provider and the pressure source from your config, calls each once, and prints what Hourglass would see. It writes nothing to the database.

```
setups: 2 (database hourglass.db)
provider: static
matched setups (1):
  lab-01 -> lab-01_RP
expected but not reported (1):
  lab-02 -> lab-02_RP
orphans (1):
  lab-01_RP
ignored allocations: 1
pressure source: inventory
raw pressure: 0.0000  multiplier: 0x (current bands, unsmoothed)
```

| Section | Meaning |
| --- | --- |
| `matched setups` | Setups whose expected resource name the provider reported. Disabled and held setups are flagged. |
| `expected but not reported` | Enabled setups the provider did not report. |
| `orphans` | Reported resources that match a setup with no active hold. |
| `ignored allocations` | Distinct reported resources that match no setup. |
| `raw pressure`, `multiplier` | The pressure source's value now, and the multiplier the current bands give it. No smoothing history is read. |

Setups come from the database if it exists (opened read-only), else from the seed file, else none. The pressure source gets a snapshot whose setups carry resource names but weight 0 and whose holds carry only the setup name. Each call has a 30-second bound. Any error exits 1 with a message. The command is part of `hourglassd`; a custom binary does not get it unless you build the same wiring yourself, which needs `internal/` and so is not possible outside this module.

Opening a WAL database read-only can create empty `-wal` and `-shm` files next to it. The database file itself is not modified.

## Bundled extensions

All are linked into `hourglassd`. Setting a name in the config is what enables one.

| Name | Kind | Config keys |
| --- | --- | --- |
| `vsphere` | provider | `url`, `username`, `password`, `insecure_skip_verify`, `datacenter`, `clusters`, `request_timeout` |
| `static` | provider | `path` |
| `inventory` | pressure source | none |
| `vsphere` | pressure source | the same block as the `vsphere` provider |
| `default` | metering policy | `start_after`, `start_on_allocation` |
| `webhook` | hook | `url`, `secret`, `types` |

**`static`.** `path` is a JSON file: an array of `{"resource_name": "...", "labels": {"k": "v"}}`. It is re-read on every poll, so you can edit it while Hourglass runs. A missing or malformed file, or an entry with an empty `resource_name`, is an error and fails the poll. Use it for demos and tests.

**`inventory`.** Pressure is the weight of held setups divided by the weight of all enabled setups. Setups with weight 0 are ignored. It takes no config and rejects any key. It is the default pressure source.

**`default`.** Meters a hold once the provider reports its resource (`start_on_allocation`, default `true`) or once `start_after` has passed since acquire (default `2h`), whichever comes first, and keeps metering until release.

| `start_after` | Effect |
| --- | --- |
| `"2h"`, `"90m"`, any Go duration | Time trigger after that long |
| `"0"` | Meter from the moment of acquire |
| `"off"` | No time trigger |

With both triggers off, nothing is ever metered. It always returns `Weight: 0`, so the rate rules decide the weight. It is the default metering policy.

**`vsphere`** (provider and pressure source share one block and one vCenter session):

| Key | Default | Meaning |
| --- | --- | --- |
| `url` | required | `http` or `https` URL of the SOAP endpoint, for example `https://vcenter.example.com/sdk`. No credentials in the URL. An empty path becomes `/sdk`. |
| `username` | required | A read-only vCenter account |
| `password` | `HOURGLASS_VSPHERE_PASSWORD` | Falls back to the variable when empty. Never printed. |
| `insecure_skip_verify` | `false` | Skip TLS certificate verification. Use only for a simulator. |
| `datacenter` | all datacenters | Limit the scope to one datacenter |
| `clusters` | every cluster in scope | Names of the clusters to read. A name that does not exist fails the pressure source. |
| `request_timeout` | `20s` | Go duration, per call |

The provider lists every resource pool except each cluster's hidden root pool, nested pools included, by the pool's own name. The pressure source returns, for each cluster, the highest of: CPU reservation used over capacity of the root pool, memory reservation used over capacity of the root pool, and one minus free over total space of the accessible datastores. It then takes the highest across clusters. The provider and pressure source only read. See [Deploying](deploying.md#the-vsphere-provider) for privileges and the first-run checklist.

For the `webhook` hook, see [The webhook hook](#the-webhook-hook).
