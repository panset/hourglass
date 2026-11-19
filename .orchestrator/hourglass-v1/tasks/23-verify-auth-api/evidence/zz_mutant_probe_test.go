// SPDX-License-Identifier: MIT

package api

import "testing"

func TestZZ_MutantProbe(t *testing.T) {
	f := newPermFix(t)
	rec := f.serve("GET", "/api/v1/setup-names", nil, false, "")
	t.Logf("anonymous: %d %s", rec.Code, rec.Body.String())
}
