// SPDX-License-Identifier: MIT

package api

import (
	"context"
	"strings"
	"sync"
	"testing"

	"github.com/panset/hourglass/internal/store"
)

// PRE-3: ApplyPreapproval checks the user's source and status again inside
// its own transaction, so a user rejected, approved or changed between the
// sign-in callback's read and the step is never overridden. These tests call
// the step directly with a matching database entry and a matching file entry,
// which is the state the callback's own filter cannot protect against.
func TestPRE3_ApplyPreapprovalRechecksTheUserInsideItsTransaction(t *testing.T) {
	e := newPreEnv(t, PreapprovalEntry{Email: "gina@example.test", Roles: []string{store.RoleAdmin}})
	e.add(`["developer"]`, "gina@example.test")
	ctx := context.Background()
	auditsBefore := len(e.preAudits())

	mk := func(sub, status string, roles ...string) store.User {
		t.Helper()
		s := e.newUser(sub, status, roles...)
		return s.user
	}
	cases := []struct {
		name string
		u    store.User
	}{
		{"rejected", mk("rc-rejected", store.StatusRejected)},
		{"active with no roles", mk("rc-active", store.StatusActive)},
		{"active with a role", mk("rc-active-dev", store.StatusActive, store.RoleDeveloper)},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			before, _ := e.st.GetUserRoles(ctx, c.u.ID)
			e.ev.reset()
			ok, err := e.pre.ApplyPreapproval(ctx, c.u.ID, "gina@example.test")
			if err != nil || ok {
				t.Fatalf("approved %v err %v, want false and no error", ok, err)
			}
			u, _ := e.st.GetUser(ctx, c.u.ID)
			after, _ := e.st.GetUserRoles(ctx, c.u.ID)
			if u.Status != c.u.Status || strings.Join(after, ",") != strings.Join(before, ",") {
				t.Fatalf("user changed: %s %v -> %s %v", c.u.Status, before, u.Status, after)
			}
			if len(e.ev.types()) != 0 {
				t.Fatalf("events emitted: %v", e.ev.types())
			}
		})
	}
	// A pending user whose source is not SSO (the local admin, made pending
	// here only to isolate the source check) is never approved either.
	if err := e.st.SetUserStatus(ctx, e.admin.ID, store.StatusPending); err != nil {
		t.Fatal(err)
	}
	if ok, err := e.pre.ApplyPreapproval(ctx, e.admin.ID, "gina@example.test"); err != nil || ok {
		t.Fatalf("pending local admin: approved %v err %v", ok, err)
	}
	if roles, _ := e.st.GetUserRoles(ctx, e.admin.ID); strings.Join(roles, ",") != store.RoleAdmin {
		t.Fatalf("local admin roles %v", roles)
	}
	if n := len(e.preAudits()); n != auditsBefore {
		t.Fatalf("%d user.preapprove entries written by refused calls", n-auditsBefore)
	}
}

// PRE-3: several sign-ins at once for one pending user approve once.
func TestPRE3_ConcurrentApplyPreapprovalApprovesOnce(t *testing.T) {
	e := newPreEnv(t)
	e.add(`["developer"]`, "hank@example.test")
	ctx := context.Background()
	u := e.newUser("rc-hank", store.StatusPending).user
	e.ev.reset()
	const n = 8
	var wg sync.WaitGroup
	results := make([]bool, n)
	errs := make([]error, n)
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			results[i], errs[i] = e.pre.ApplyPreapproval(ctx, u.ID, "hank@example.test")
		}()
	}
	wg.Wait()
	won := 0
	for i := range results {
		if errs[i] != nil {
			t.Fatalf("call %d: %v", i, errs[i])
		}
		if results[i] {
			won++
		}
	}
	if won != 1 {
		t.Fatalf("%d calls approved, want 1", won)
	}
	if got := e.ev.types(); strings.Join(got, ",") != "user.approved" {
		t.Fatalf("events %v", got)
	}
	if n := len(e.preAudits()); n != 1 {
		t.Fatalf("%d user.preapprove entries", n)
	}
	got, _ := e.st.GetUser(ctx, u.ID)
	cnt, _ := e.st.CountLedger(ctx, u.ID, e.led.CurrentPeriod(), "allowance")
	if got.Status != store.StatusActive || cnt != 1 {
		t.Fatalf("status %s allowances %d", got.Status, cnt)
	}
}
