// SPDX-License-Identifier: MIT

package hourglass

// Event types, in the order of the spec's event table.
const (
	EventHoldAcquiring       = "hold.acquiring"        // a user asks to acquire, after the core checks pass; vetoable
	EventHoldAcquired        = "hold.acquired"         // the hold is created
	EventHoldMeteringStarted = "hold.metering_started" // the policy first says to charge
	EventHoldDeployed        = "hold.deployed"         // the provider first reports the setup
	EventHoldReleasing       = "hold.releasing"        // a user asked to release a setup that is still deployed
	EventHoldReleased        = "hold.released"         // the hold ends, by the user, an admin, a service or the collector
	EventBalanceLow          = "balance.low"           // a balance first drops below 10% of the allowance
	EventBalanceOverdrawn    = "balance.overdrawn"     // a balance first reaches zero or below
	EventUserPending         = "user.pending"          // a new user signs in for the first time
	EventUserApproved        = "user.approved"         // an admin assigns a first role, or a pre-approved SSO user signs in
	EventPeriodClosed        = "period.closed"         // the monthly rollover finishes
	EventPollFailed          = "poll.failed"           // a collector run fails
)

// EventTypes returns every event type in the order of the spec's event
// table. The returned slice is a fresh copy.
func EventTypes() []string {
	return []string{
		EventHoldAcquiring,
		EventHoldAcquired,
		EventHoldMeteringStarted,
		EventHoldDeployed,
		EventHoldReleasing,
		EventHoldReleased,
		EventBalanceLow,
		EventBalanceOverdrawn,
		EventUserPending,
		EventUserApproved,
		EventPeriodClosed,
		EventPollFailed,
	}
}

// Vetoable reports whether a hook error for this event type aborts the
// action. Only hold.acquiring is vetoable.
func Vetoable(eventType string) bool {
	return eventType == EventHoldAcquiring
}
