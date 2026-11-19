// SPDX-License-Identifier: MIT

// Package server links the Hourglass server implementation into a binary.
//
// hourglass.Run is declared in pkg/hourglass but implemented in a package
// that cannot be imported from outside this module. A custom main (EXT-5)
// therefore imports this package, for its side effect only, next to its own
// extensions:
//
//	import (
//		"github.com/panset/hourglass/pkg/hourglass"
//		_ "github.com/panset/hourglass/pkg/hourglass/server"
//
//		_ "example.com/lab/myprovider"
//	)
//
// Without this import hourglass.Run returns an error that says so.
package server

import _ "github.com/panset/hourglass/internal/app" // implements hourglass.Run
