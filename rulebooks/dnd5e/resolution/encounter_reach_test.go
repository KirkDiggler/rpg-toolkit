// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package resolution

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"sort"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

// encounterPath is the composition this package loads a world through.
const encounterPath = "github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter"

// encounterReach is every method this package may call on a loaded Encounter.
//
// TWO, and the shortness is the property. LoadEncounter builds the world — a
// constructor rather than a method, so it is not in this list — and then Canvas
// hands out the room to install and ToData reads the world back. Load, look,
// serialise. Nothing here advances a clock, ends a turn, or pumps a pass.
var encounterReach = []string{"Canvas", "ToData"}

// TestResolveTouchesTheEncounterThroughTwoMethods is the half of the reentrancy
// invariant that only this package can hold.
//
// # The invariant, and why it is not session's to check
//
// The composition consults a standing capability, and that capability is
// implemented by the session seam — which, since the game-context slice, calls
// back into this package. So if a resolution reached a composition method that
// consults standing, a consult could open a second interaction inside the
// first: two casts, two buses, one of them mid-flight. R7 forbids exactly that.
//
// It does not happen because this package touches a loaded encounter only
// through Canvas and ToData, neither of which consults anything. The
// composition's own standing consults live in Pump, buildMonsterView and
// noticeDown, and no path from here reaches them.
//
// Session pinned the half it could: that its standing seam names no entry that
// runs an interaction. It could not pin THIS half and said so — it compiles
// against a PUBLISHED resolution from the module cache, so scanning the sibling
// directory would pin a different artifact than the one it runs. Here the scan
// and the code are the same thing, which is the whole reason this test lives
// on this side of the boundary.
//
// # What it cannot say
//
// It reads the reach at the call site, so a method invoked through a value of
// interface type, or through a helper that took the encounter as a parameter,
// is outside what it sees. That is worth stating plainly rather than leaving a
// reader to assume a completeness the walk does not have: this is a fence
// around the direct calls, which is where the reach has always been, not a
// proof that no indirection exists.
func TestResolveTouchesTheEncounterThroughTwoMethods(t *testing.T) {
	reached := encounterMethodsCalled(t)

	require.Equal(t, encounterReach, reached,
		"this package calls a method on a loaded Encounter that is not in its declared reach. "+
			"The composition consults the standing capability, session implements that capability, "+
			"and session now calls back into this package — so a resolution that reached a "+
			"composition method which consults standing would open a second interaction inside "+
			"the first (R7). If the new call is genuinely needed, it joins encounterReach "+
			"deliberately and somebody checks what it consults on the way")
}

// encounterMethodsCalled returns the sorted, deduplicated set of methods this
// package calls on values returned by encounter.LoadEncounter.
//
// It finds the local name of the encounter import, then the variables assigned
// from that package's LoadEncounter, then every method selected on one of them.
// Tracking the VARIABLE rather than a name spelled "enc" is what makes it
// survive a rename.
func encounterMethodsCalled(t *testing.T) []string {
	t.Helper()

	entries, err := os.ReadDir(".")
	require.NoError(t, err)

	seen := map[string]bool{}
	fset := token.NewFileSet()
	for _, entry := range entries {
		name := entry.Name()
		if entry.IsDir() || !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
			continue
		}

		file, parseErr := parser.ParseFile(fset, name, nil, 0)
		require.NoError(t, parseErr)

		local := localNameFor(file, encounterPath)
		if local == "" {
			continue
		}

		for _, holder := range encounterHolders(file, local) {
			for _, method := range methodsOn(file, holder) {
				seen[method] = true
			}
		}
	}

	out := make([]string, 0, len(seen))
	for method := range seen {
		out = append(out, method)
	}
	sort.Strings(out)

	return out
}

// localNameFor returns what this file calls the given import path, or "" if it
// does not import it. A dot-import returns "." and finds nothing, which is
// honest: this package does not use them and a walk that pretended to handle
// one would be guessing.
func localNameFor(file *ast.File, path string) string {
	for _, spec := range file.Imports {
		if spec.Path.Value != `"`+path+`"` {
			continue
		}
		if spec.Name != nil {
			return spec.Name.Name
		}

		return path[strings.LastIndex(path, "/")+1:]
	}

	return ""
}

// encounterHolders returns the identifiers assigned from LoadEncounter.
func encounterHolders(file *ast.File, local string) []string {
	var holders []string
	ast.Inspect(file, func(n ast.Node) bool {
		assign, ok := n.(*ast.AssignStmt)
		if !ok {
			return true
		}
		for _, rhs := range assign.Rhs {
			call, ok := rhs.(*ast.CallExpr)
			if !ok {
				continue
			}
			sel, ok := call.Fun.(*ast.SelectorExpr)
			if !ok || sel.Sel.Name != "LoadEncounter" {
				continue
			}
			if pkg, ok := sel.X.(*ast.Ident); !ok || pkg.Name != local {
				continue
			}
			if ident, ok := assign.Lhs[0].(*ast.Ident); ok {
				holders = append(holders, ident.Name)
			}
		}

		return true
	})

	return holders
}

// methodsOn returns every method selected on the named identifier.
func methodsOn(file *ast.File, holder string) []string {
	var methods []string
	ast.Inspect(file, func(n ast.Node) bool {
		call, ok := n.(*ast.CallExpr)
		if !ok {
			return true
		}
		sel, ok := call.Fun.(*ast.SelectorExpr)
		if !ok {
			return true
		}
		if ident, ok := sel.X.(*ast.Ident); ok && ident.Name == holder {
			methods = append(methods, sel.Sel.Name)
		}

		return true
	})

	return methods
}
