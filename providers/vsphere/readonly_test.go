// SPDX-License-Identifier: MIT

package vsphere

import (
	_ "embed"
	"go/ast"
	"go/parser"
	"go/token"
	"strings"
	"testing"
)

//go:embed govmomi.go
var adapterSrc string

//go:embed vsphere.go
var logicSrc string

// TestEXT3_NoMutatingCalls: the package contains no call that creates,
// changes or destroys anything on a vCenter. It inspects the product source
// (test files are exempt: the simulator tests create pools).
func TestEXT3_NoMutatingCalls(t *testing.T) {
	banned := []string{"Create", "Destroy", "Delete", "Remove", "Rename", "Reconfigure", "Reconfig",
		"Power", "Move", "Relocate", "Clone", "Update", "Set", "Add", "Mark", "Register", "Unregister",
		"Terminate", "Migrate", "Apply", "Import", "Upload", "Reset", "Suspend", "Shutdown", "Reboot"}
	for name, src := range map[string]string{"govmomi.go": adapterSrc, "vsphere.go": logicSrc} {
		f, err := parser.ParseFile(token.NewFileSet(), name, src, 0)
		if err != nil {
			t.Fatal(err)
		}
		ast.Inspect(f, func(n ast.Node) bool {
			call, ok := n.(*ast.CallExpr)
			if !ok {
				return true
			}
			sel, ok := call.Fun.(*ast.SelectorExpr)
			if !ok {
				return true
			}
			if id, ok := sel.X.(*ast.Ident); ok && id.Name == "hourglass" {
				return true // local extension registry, not vCenter
			}
			for _, b := range banned {
				if strings.HasPrefix(sel.Sel.Name, b) {
					// Finder.SetDatacenter only selects a lookup scope locally.
					if sel.Sel.Name == "SetDatacenter" {
						continue
					}
					t.Errorf("%s: call to %s looks mutating", name, sel.Sel.Name)
				}
			}
			return true
		})
	}
}
