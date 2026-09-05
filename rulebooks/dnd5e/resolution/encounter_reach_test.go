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
// FOUR, and the shortness is still the property. LoadEncounter builds the
// world — a constructor rather than a method, so it is not in this list — and
// then Canvas hands out the room to install, ToData reads the world back, and
// IsHostile and IsAllied are the two side questions the cast asks the run on
// an effect's behalf (rpg-project#375, design §4). Load, look, ask who is
// whose enemy, serialise. Nothing here advances a clock, ends a turn, or
// pumps a pass: the two side reads fold the run's own graph and consult no
// capability — checked on the way in, as the failure message below asks.
var encounterReach = []string{"Canvas", "IsAllied", "IsHostile", "ToData"}

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
// interface type is outside what it sees. It does follow the handle where this
// package carries it — into a parameter or a struct field declared as
// *encounter.Encounter, which is how the cast came to hold the run — but a
// handle laundered through an interface or an `any` would slip past. That is
// worth stating plainly rather than leaving a reader to assume a completeness
// the walk does not have: this is a fence around the direct calls, which is
// where the reach has always been, not a proof that no indirection exists.
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
// package calls on a loaded Encounter.
//
// It finds the local name of the encounter import, then every name this file
// holds an Encounter under — the variables assigned from that package's
// LoadEncounter, and the parameters and struct fields declared as a pointer
// to its Encounter — then every method selected on one of them. Tracking the
// HOLDER rather than a name spelled "enc" is what makes it survive a rename.
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

// encounterHolders returns the names this file holds a loaded Encounter under:
// the identifiers assigned from LoadEncounter, and every parameter or struct
// field declared as *encounter.Encounter. The second kind is how the handle
// travels — resolveOn hands it to the door, the door parks it on the cast —
// and a fence that stopped at the assignment would have let the cast's calls
// through unseen.
func encounterHolders(file *ast.File, local string) []string {
	var holders []string
	ast.Inspect(file, func(n ast.Node) bool {
		switch node := n.(type) {
		case *ast.AssignStmt:
			for _, rhs := range node.Rhs {
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
				if ident, ok := node.Lhs[0].(*ast.Ident); ok {
					holders = append(holders, ident.Name)
				}
			}
		case *ast.Field:
			// One node type for both a parameter and a struct field.
			if isEncounterPointer(node.Type, local) {
				for _, name := range node.Names {
					holders = append(holders, name.Name)
				}
			}
		}

		return true
	})

	return holders
}

// isEncounterPointer reports whether a type expression spells
// *<local>.Encounter.
func isEncounterPointer(expr ast.Expr, local string) bool {
	star, ok := expr.(*ast.StarExpr)
	if !ok {
		return false
	}
	sel, ok := star.X.(*ast.SelectorExpr)
	if !ok || sel.Sel.Name != "Encounter" {
		return false
	}
	pkg, ok := sel.X.(*ast.Ident)

	return ok && pkg.Name == local
}

// methodsOn returns every method selected on the named holder, whether it is
// reached as a bare identifier (enc.Canvas) or as a field of something
// (v.run.IsHostile). A field name shared with an unrelated struct would count
// that struct's calls too, which can only ADD to the reach and fail loudly —
// the honest direction for a fence to err in.
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
		switch x := sel.X.(type) {
		case *ast.Ident:
			if x.Name == holder {
				methods = append(methods, sel.Sel.Name)
			}
		case *ast.SelectorExpr:
			if x.Sel.Name == holder {
				methods = append(methods, sel.Sel.Name)
			}
		}

		return true
	})

	return methods
}
