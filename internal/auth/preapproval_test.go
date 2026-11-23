// SPDX-License-Identifier: MIT

package auth

import (
	"context"
	"errors"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/panset/hourglass/internal/store"
	"github.com/panset/hourglass/pkg/hourglass"
)

// The sign-in half of the pre-approval step (docs/design/ledger-labels-onboarding.md,
// D1): which sign-ins reach the injected step, with which email, and what the
// callback does with its answer. The step itself (the transaction, the
// sources, the audit entry) is tested in internal/api, which implements it.

type preCall struct {
	userID int64
	email  string
}

// fakePreapprover records its calls and answers approve / err.
type fakePreapprover struct {
	mu      sync.Mutex
	calls   []preCall
	approve bool
	err     error
}

func (p *fakePreapprover) ApplyPreapproval(_ context.Context, userID int64, email string) (bool, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.calls = append(p.calls, preCall{userID, email})
	return p.approve && p.err == nil, p.err
}

func (p *fakePreapprover) list() []preCall {
	p.mu.Lock()
	defer p.mu.Unlock()
	return append([]preCall(nil), p.calls...)
}

func newPreEnv(t *testing.T, pre Preapprover) (*env, *fakeIssuer) {
	t.Helper()
	var e *env
	iss := newFakeIssuer(t, func() time.Time { return e.clock.Now() })
	e = newEnv(t, Config{OIDC: OIDCConfig{IssuerURL: iss.srv.URL, ClientID: testClientID,
		ClientSecret: testClientSecret, RedirectURL: testRedirect}}, WithPreapprover(pre))
	return e, iss
}

func eventTypes(evs []hourglass.Event) []string {
	out := []string{}
	for _, e := range evs {
		out = append(out, e.Type)
	}
	return out
}

// PRE-2: email_verified false (boolean or string) never reaches the step;
// true, "true", null and an absent claim do. Any other value is treated as
// not verified.
func TestPRE2_EmailVerifiedClaimGatesTheStep(t *testing.T) {
	cases := []struct {
		name  string
		value any // nil: the claim is absent
		null  bool
		want  bool
	}{
		{name: "absent", want: true},
		{name: "null", null: true, want: true},
		{name: "true", value: true, want: true},
		{name: "string true", value: "true", want: true},
		{name: "string TRUE", value: " TRUE ", want: true},
		{name: "false", value: false, want: false},
		{name: "string false", value: "false", want: false},
		{name: "string False", value: "False", want: false},
		{name: "other string", value: "yes", want: false},
		{name: "number", value: 1, want: false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			pre := &fakePreapprover{}
			e, iss := newPreEnv(t, pre)
			iss.set(func(f *fakeIssuer) {
				f.claims = map[string]any{}
				switch {
				case c.null:
					f.claims["email_verified"] = jsonNull{}
				case c.value != nil:
					f.claims["email_verified"] = c.value
				}
			})
			r := signIn(t, e)
			if r.location != "/" || r.session == nil {
				t.Fatalf("sign-in failed: %+v", r)
			}
			if got := len(pre.list()) == 1; got != c.want {
				t.Fatalf("step called = %v, want %v (calls %v)", got, c.want, pre.list())
			}
		})
	}
}

// jsonNull marshals as JSON null, so a claim can be sent as an explicit null.
type jsonNull struct{}

func (jsonNull) MarshalJSON() ([]byte, error) { return []byte("null"), nil }

// PRE-2: matching needs a non-empty email claim, and only the email claim is
// handed to the step, as sent; an address in another claim is never used.
func TestPRE2_OnlyANonEmptyEmailClaimIsUsed(t *testing.T) {
	t.Run("empty email", func(t *testing.T) {
		pre := &fakePreapprover{approve: true}
		e, iss := newPreEnv(t, pre)
		iss.set(func(f *fakeIssuer) { f.email, f.preferred = "", "alice@example.test" })
		signIn(t, e)
		if len(pre.list()) != 0 {
			t.Fatalf("step called for an empty email claim: %v", pre.list())
		}
	})
	t.Run("whitespace email", func(t *testing.T) {
		pre := &fakePreapprover{approve: true}
		e, iss := newPreEnv(t, pre)
		iss.set(func(f *fakeIssuer) { f.email = "   " })
		signIn(t, e)
		if len(pre.list()) != 0 {
			t.Fatalf("step called for a blank email claim: %v", pre.list())
		}
	})
	t.Run("absent email, address in other claims", func(t *testing.T) {
		pre := &fakePreapprover{approve: true}
		e, iss := newPreEnv(t, pre)
		iss.set(func(f *fakeIssuer) {
			f.preferred = "alice@example.test"
			f.claims = map[string]any{"email": nil, "upn": "alice@example.test", "mail": "alice@example.test"}
		})
		signIn(t, e)
		if len(pre.list()) != 0 {
			t.Fatalf("step called without an email claim: %v", pre.list())
		}
	})
	t.Run("email claim passed as sent", func(t *testing.T) {
		pre := &fakePreapprover{}
		e, iss := newPreEnv(t, pre)
		iss.set(func(f *fakeIssuer) { f.email, f.preferred = " Alice@Example.TEST ", "other@example.test" })
		signIn(t, e)
		u, _ := e.st.GetUserBySub(e.ctx(), "sub-alice")
		calls := pre.list()
		if len(calls) != 1 || calls[0].email != " Alice@Example.TEST " || calls[0].userID != u.ID {
			t.Fatalf("calls %v, want one with the email claim as sent for user %d", calls, u.ID)
		}
	})
}

// PRE-3: only a pending SSO user reaches the step: an active user (even with
// no roles) and a rejected user do not, and local sign-in never does.
func TestPRE3_OnlyAPendingSSOUserReachesTheStep(t *testing.T) {
	for _, status := range []string{store.StatusActive, store.StatusRejected} {
		t.Run(status, func(t *testing.T) {
			pre := &fakePreapprover{approve: true}
			e, _ := newPreEnv(t, pre)
			e.user("sub-alice", status) // no roles
			signIn(t, e)
			if len(pre.list()) != 0 {
				t.Fatalf("step called for a %s user: %v", status, pre.list())
			}
			u, _ := e.st.GetUserBySub(e.ctx(), "sub-alice")
			if roles, _ := e.st.GetUserRoles(e.ctx(), u.ID); u.Status != status || len(roles) != 0 {
				t.Fatalf("user changed: %s %v", u.Status, roles)
			}
		})
	}
	t.Run("pending from an earlier sign-in", func(t *testing.T) {
		pre := &fakePreapprover{}
		e, _ := newPreEnv(t, pre)
		signIn(t, e)
		signIn(t, e)
		if n := len(pre.list()); n != 2 {
			t.Fatalf("step called %d times for two sign-ins of a pending user, want 2", n)
		}
	})
	t.Run("local admin", func(t *testing.T) {
		pre := &fakePreapprover{approve: true}
		e, _ := newPreEnv(t, pre)
		if _, err := EnsureLocalAdmin(e.ctx(), e.st, "a long enough admin password", e.svc.log); err != nil {
			t.Fatal(err)
		}
		admin, _ := e.st.GetUserByUsername(e.ctx(), LocalAdminUsername)
		if err := e.st.SetPassword(e.ctx(), admin.ID, admin.PasswordHash, false); err != nil {
			t.Fatal(err)
		}
		rec := e.do(req{method: "POST", path: "/auth/local", body: `{"username":"admin","password":"a long enough admin password"}`})
		if rec.Code != 200 {
			t.Fatalf("local sign-in: %d %s", rec.Code, rec.Body)
		}
		if len(pre.list()) != 0 {
			t.Fatalf("step called for the local admin: %v", pre.list())
		}
		// Even a stored local user that somehow reached the gate is refused.
		if e.svc.applyPreapproval(e.ctx(), admin, "admin@example.test", nil) || len(pre.list()) != 0 {
			t.Fatal("the gate let the local admin through")
		}
	})
}

// PRE-4: a new user the step approves gets no user.pending; one it does not
// approve gets user.pending exactly as before.
func TestPRE4_ApprovedNewUserGetsNoPendingEvent(t *testing.T) {
	pre := &fakePreapprover{approve: true}
	e, _ := newPreEnv(t, pre)
	r := signIn(t, e)
	if r.location != "/" || r.session == nil {
		t.Fatalf("sign-in %+v", r)
	}
	if got := eventTypes(e.em.list()); len(got) != 0 {
		t.Fatalf("events from auth = %v, want none (the step emits user.approved)", got)
	}

	pre2 := &fakePreapprover{approve: false}
	e2, _ := newPreEnv(t, pre2)
	signIn(t, e2)
	if got := eventTypes(e2.em.list()); len(got) != 1 || got[0] != hourglass.EventUserPending {
		t.Fatalf("events = %v, want one user.pending", got)
	}
}

// PRE-8: when the step fails, the sign-in still succeeds, the user stays
// pending, the error is logged without the address, and a new user gets
// user.pending as before.
func TestPRE8_FailedStepStillSignsInAndLeavesUserPending(t *testing.T) {
	pre := &fakePreapprover{approve: true, err: errors.New("forced approval failure")}
	e, _ := newPreEnv(t, pre)
	r := signIn(t, e)
	if r.location != "/" || r.session == nil {
		t.Fatalf("sign-in did not succeed: %+v", r)
	}
	v := decodeView(t, e.do(req{method: "GET", path: "/api/v1/session", cookie: r.session.Value}))
	if v.Status != store.StatusPending || len(v.Roles) != 0 {
		t.Fatalf("session %+v, want pending with no roles", v)
	}
	logs := e.logs.String()
	if !strings.Contains(logs, "applying a pre-approval failed") || !strings.Contains(logs, "forced approval failure") {
		t.Fatalf("error not logged: %s", logs)
	}
	if strings.Contains(logs, "alice@example.test") {
		t.Fatalf("the address was logged: %s", logs)
	}
	if got := eventTypes(e.em.list()); len(got) != 1 || got[0] != hourglass.EventUserPending {
		t.Fatalf("events = %v, want one user.pending", got)
	}
}

// Without a step, sign-in is as before (Compatibility, "Sign-in").
func TestPRE6_NoStepSignInAsBefore(t *testing.T) {
	e, _ := newOIDCEnv(t)
	r := signIn(t, e)
	v := decodeView(t, e.do(req{method: "GET", path: "/api/v1/session", cookie: r.session.Value}))
	if v.Status != store.StatusPending || len(v.Roles) != 0 {
		t.Fatalf("session %+v", v)
	}
	if got := eventTypes(e.em.list()); len(got) != 1 || got[0] != hourglass.EventUserPending {
		t.Fatalf("events = %v", got)
	}
}

func TestPRE2_EmailVerifiedParsing(t *testing.T) {
	for raw, want := range map[string]bool{
		"": true, "null": true, "true": true, `"true"`: true, `"True"`: true, `" true "`: true,
		"false": false, `"false"`: false, `"FALSE"`: false, `"no"`: false, `""`: false, "0": false, "1": false, "{}": false, "[]": false,
	} {
		if got := emailVerified([]byte(raw)); got != want {
			t.Errorf("emailVerified(%s) = %v, want %v", raw, got, want)
		}
	}
}
