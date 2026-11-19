// SPDX-License-Identifier: MIT

// Package hourglasstest provides fakes, test-data builders and conformance
// suites for the four extension interfaces in pkg/hourglass (EXT-5).
//
// Fakes: FakeProvider, FakePressureSource, FakePolicy, FakeHook and
// FakeClock. All are safe for concurrent use.
//
// Builders: NewSnapshot builds a hourglass.Snapshot with setups, holds and
// allocations so tests read clearly.
//
// Conformance suites: TestProvider, TestPressureSource, TestMeteringPolicy
// and TestHook each take a *testing.T and a constructor. An extension author
// runs them from their own _test.go file:
//
//	func TestConformance(t *testing.T) {
//		hourglasstest.TestProvider(t, func() hourglass.ResourceProvider {
//			return newMyProvider()
//		})
//	}
//
// Every check is a subtest whose name states the contract it verifies.
package hourglasstest
