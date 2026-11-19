// SPDX-License-Identifier: MIT

//go:build !ui

package uiserve

import "io/fs"

// Embedded reports the UI built into this binary. This build has none (no
// "ui" tag): the binary is API-only.
func Embedded() (fs.FS, bool) { return nil, false }
