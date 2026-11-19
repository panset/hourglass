// SPDX-License-Identifier: MIT

package holds

import "testing"

func TestHOLD1_AcquireBlockRule(t *testing.T) {
	cases := []struct {
		name        string
		local, enf  bool
		balance     int64
		held, limit int
		want        string
	}{
		{"free", false, true, 1, 0, 2, ""},
		{"local admin first", true, true, 0, 5, 2, BlockLocalAdmin},
		{"zero balance with enforcement", false, true, 0, 0, 2, CodeInsufficientTokens},
		{"negative balance with enforcement", false, true, -5, 0, 2, CodeInsufficientTokens},
		{"shadow mode ignores the balance (MET-6)", false, false, -5, 0, 2, ""},
		{"at the limit", false, true, 10, 2, 2, CodeHoldLimit},
		{"tokens before limit", false, true, 0, 2, 2, CodeInsufficientTokens},
		{"limit lowered below count (HOLD-5)", false, false, 10, 3, 2, CodeHoldLimit},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := AcquireBlock(c.local, c.enf, c.balance, c.held, c.limit); got != c.want {
				t.Fatalf("AcquireBlock = %q, want %q", got, c.want)
			}
		})
	}
}
