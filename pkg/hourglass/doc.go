// SPDX-License-Identifier: MIT

// Package hourglass is the public extension API of Hourglass (EXT-1).
//
// The core knows nothing about how a setup gets deployed, how lab pressure
// is measured, or when metering should start. Each of those sits behind one
// of four interfaces defined here:
//
//   - ResourceProvider reports what is currently deployed, as a list of
//     Allocations. It is optional: with none configured, every other
//     extension receives no allocations.
//   - PressureSource reports how contended the lab is, from 0.0 to 1.0.
//   - MeteringPolicy decides, per hold and per poll, whether to charge.
//   - Hook receives every event in the event table (see EventTypes). An
//     error returned for the vetoable event (hold.acquiring) aborts the
//     action; other errors are logged.
//
// Extensions are registered by name, the way database/sql drivers are: a
// package calls RegisterProvider, RegisterPressureSource,
// RegisterMeteringPolicy or RegisterHook from its init function, and a main
// package imports it for that side effect. The config file then selects
// extensions by name and hands each one an opaque config block, which
// reaches the factory as JSON whatever the config file's own format is. A
// factory that needs no config must accept a nil or empty block. An unknown
// name fails startup with the list of registered names (EXT-2).
//
// Extensions never touch the database. On every poll the core builds a
// Snapshot with the enabled setups (labels, details, resolved weight and
// expected resource name), the active holds and the provider's allocations,
// and passes it to the pressure source and the metering policy. Setup,
// User and Hold are plain structs that mirror the data model. Extensions
// should read the time of a poll from Snapshot.Time, not the wall clock, so
// that they behave the same under the test clocks and in replays.
//
// A minimal hook package looks like this:
//
//	package audit
//
//	import (
//		"context"
//		"encoding/json"
//		"log"
//
//		"github.com/panset/hourglass/pkg/hourglass"
//	)
//
//	type hook struct{}
//
//	func (hook) Name() string { return "audit" }
//
//	func (hook) Handle(ctx context.Context, e hourglass.Event) error {
//		log.Printf("hourglass event %s", e.Type)
//		return nil
//	}
//
//	func init() {
//		hourglass.RegisterHook("audit", func(config json.RawMessage) (hourglass.Hook, error) {
//			return hook{}, nil
//		})
//	}
//
// and a custom binary enables it with a blank import in its own main. The
// binary also imports pkg/hourglass/server, which links the implementation of
// Run (without it Run returns an error), and then calls Run:
//
//	package main
//
//	import (
//		"context"
//		"log"
//		"os"
//
//		"github.com/panset/hourglass/pkg/hourglass"
//		_ "github.com/panset/hourglass/pkg/hourglass/server"
//
//		_ "example.com/lab/audit"
//	)
//
//	func main() {
//		err := hourglass.Run(context.Background(), hourglass.Options{
//			DefaultPressureSource: "inventory", // some registered pressure source
//			DefaultMeteringPolicy: "default",   // some registered metering policy
//			Args:                  os.Args[1:],
//		})
//		if err != nil {
//			log.Fatal(err)
//		}
//	}
package hourglass
