// SPDX-License-Identifier: MIT

//go:build !ui

package uiserve

import "testing"

func TestUI3_NoEmbeddedUIWithoutTag(t *testing.T) {
	if fsys, ok := Embedded(); ok || fsys != nil {
		t.Fatalf("Embedded() = %v, %v", fsys, ok)
	}
}
