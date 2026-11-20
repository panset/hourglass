// SPDX-License-Identifier: MIT

package api

import (
	"context"
	"net/http/httptest"
	"time"

	"github.com/panset/hourglass/internal/auth"
	"github.com/panset/hourglass/internal/store"
)

// Test helpers for service accounts. They write through the store and the
// auth package's token functions, exactly as the issue route does, so a test
// can get a working token without the admin API.

// svcAccount is a service account created for a test, with one live token.
type svcAccount struct {
	acct  store.ServiceAccount
	token string // hgsa_<id>_<secret>
	tokID int64
}

// mintServiceAccount creates an enabled account with the scopes and issues
// one token without expiry.
func (b *backend) mintServiceAccount(name string, scopes ...string) svcAccount {
	b.t.Helper()
	if scopes == nil {
		scopes = []string{}
	}
	a, err := b.st.CreateServiceAccount(context.Background(), store.ServiceAccount{Name: name, Scopes: scopes, Enabled: true}, b.clk.Now())
	if err != nil {
		b.t.Fatal(err)
	}
	tok, id := b.mintServiceToken(a.ID, nil)
	return svcAccount{acct: a, token: tok, tokID: id}
}

// mintServiceToken issues another token for an account and returns it and
// its id.
func (b *backend) mintServiceToken(accountID int64, expiresAt *time.Time) (string, int64) {
	b.t.Helper()
	secret, hash, err := auth.NewServiceSecret()
	if err != nil {
		b.t.Fatal(err)
	}
	t, err := b.st.InsertServiceToken(context.Background(), accountID, hash, expiresAt, b.clk.Now())
	if err != nil {
		b.t.Fatal(err)
	}
	return auth.FormatServiceToken(t.ID, secret), t.ID
}

// bearer is the header pair for serve's hdr argument.
func bearer(token string) []string { return []string{"Authorization", "Bearer " + token} }

// serveService sends one request with a bearer token and no session.
func (b *backend) serveService(method, path, token, body string, hdr ...string) *httptest.ResponseRecorder {
	b.t.Helper()
	return b.serve(method, path, nil, false, body, append(bearer(token), hdr...)...)
}

// allScopes is every scope.
func allScopes() []string { return auth.Scopes() }

// scopesWithout is every scope except s.
func scopesWithout(s string) []string {
	var out []string
	for _, x := range auth.Scopes() {
		if x != s {
			out = append(out, x)
		}
	}
	return out
}
