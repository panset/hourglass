// SPDX-License-Identifier: MIT

//go:build ui

package uiserve

import (
	"embed"
	"io/fs"
)

// dist is the built UI. Plain `go build -tags ui` needs ./dist to hold it
// (copy ui's build output here); Bazel fills it from //ui:dist.
//
//go:embed all:dist
var dist embed.FS

// Embedded returns the built UI, rooted at its index.html.
func Embedded() (fs.FS, bool) {
	sub, err := fs.Sub(dist, "dist")
	if err != nil {
		return nil, false
	}
	if _, err := fs.Stat(sub, indexName); err != nil {
		return nil, false
	}
	return sub, true
}
