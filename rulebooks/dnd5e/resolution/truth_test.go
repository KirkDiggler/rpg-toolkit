// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package resolution

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

// gamectxPath is the package the door is the only caller of. Matched by import
// path rather than by the name "gamectx", so aliasing the import does not walk
// an install past this pin.
const gamectxPath = "github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/gamectx"

// doorFile is the file [installTruth] lives in. Every gamectx.With* in this
// package belongs to it.
const doorFile = "truth.go"

// TestOnlyTheDoorInstallsGameContext is the R5 pin: exactly one function
// installs game context, and it is [installTruth].
//
// The three pins it sits with — TestNoCodePathProducesARoomlessInteraction,
// TestNoCodePathProducesACastlessInteraction and
// TestNoCodePathProducesAReadinesslessInteraction — each say that ONE fact is
// installed once and unconditionally. None of them can say what this one says:
// that no SECOND installer exists, and that the resolve path reaches the door
// at all. Drop the call to installTruth and all three still pass, because each
// of them is reading the door's own body.
//
// That gap is the defect they were written for, seen from the other side.
// gamectx reached five registries and a sixth in combat by exactly this route —
// each call site that needed a fact installed it where it stood, every one of
// them individually reasonable, and the set of them was what nobody checked
// (rpg-toolkit#1251). The count is the assertion, so the source is the
// assertion: this reads every file in the package, test files included, because
// "the tests supply what production does not" is the shape of the original bug.
func TestOnlyTheDoorInstallsGameContext(t *testing.T) {
	fset := token.NewFileSet()

	entries, err := os.ReadDir(".")
	require.NoError(t, err)

	var offenders []string
	for _, entry := range entries {
		name := entry.Name()
		if entry.IsDir() || !strings.HasSuffix(name, ".go") {
			continue
		}

		file, err := parser.ParseFile(fset, name, nil, 0)
		require.NoError(t, err)

		local, imported := importedAs(t, name, file, gamectxPath)
		if !imported {
			continue
		}

		ast.Inspect(file, func(n ast.Node) bool {
			call, ok := n.(*ast.CallExpr)
			if !ok {
				return true
			}
			sel, ok := call.Fun.(*ast.SelectorExpr)
			if !ok {
				return true
			}
			pkg, ok := sel.X.(*ast.Ident)
			if !ok || pkg.Name != local || !strings.HasPrefix(sel.Sel.Name, "With") {
				return true
			}
			if name == doorFile {
				return true
			}

			offenders = append(offenders, fmt.Sprintf("%s: %s.%s",
				fset.Position(call.Pos()), local, sel.Sel.Name))

			return true
		})
	}
	require.Empty(t, offenders,
		"%s is the only file allowed to install game context — a second installer is "+
			"how gamectx got five registries and one install (rpg-toolkit#1251). If the "+
			"new fact belongs in context, it goes through installTruth; if it needs a "+
			"repository it is a record, not a tenant, and admission is a ruling either way",
		doorFile)

	// And the door is ON the path. Everything above is about who may install;
	// this is the half that says the installing happens.
	file, err := parser.ParseFile(fset, "resolve.go", nil, 0)
	require.NoError(t, err)

	fn := funcDecl(t, "resolve.go", file, "resolveOn")

	var calls []token.Pos
	ast.Inspect(fn, func(n ast.Node) bool {
		call, ok := n.(*ast.CallExpr)
		if !ok {
			return true
		}
		if ident, ok := call.Fun.(*ast.Ident); ok && ident.Name == "installTruth" {
			calls = append(calls, call.Pos())
		}

		return true
	})
	require.Len(t, calls, 1, "resolveOn goes through the door exactly once")

	requireUnconditional(t, fset, fn, calls[0],
		"calls the door inside a condition — an interaction that skips it reads a "+
			"world, a cast and a readiness map that nobody installed, which is every "+
			"failure the three pins beside this one were written for at once")
}

// importedAs reports the local name a file imports path under, and whether it
// imports it at all.
//
// A dot import is refused rather than resolved: it would let `WithRoom(ctx, r)`
// install without naming a package, and a pin that cannot see an install is
// worse than no pin.
func importedAs(t *testing.T, filename string, file *ast.File, path string) (string, bool) {
	t.Helper()

	quoted := `"` + path + `"`
	for _, spec := range file.Imports {
		if spec.Path.Value != quoted {
			continue
		}
		if spec.Name == nil {
			return path[strings.LastIndex(path, "/")+1:], true
		}
		require.NotEqual(t, ".", spec.Name.Name,
			"%s dot-imports gamectx, which hides installs from this pin", filename)

		if spec.Name.Name == "_" {
			return "", false
		}

		return spec.Name.Name, true
	}

	return "", false
}

// requireUnconditional fails when the install at pos sits inside an if
// statement in fn.
//
// The shared half of all four pins in this family, and the reason each of them
// is structural rather than behavioural: whatever the condition said, there
// would be inputs it answered no for, and the interactions that hit those
// inputs are exactly the ones no test happens to write. msg carries the caller's
// own reason, because "sometimes absent" costs something different for each
// fact — a silent base AC, a reaction that never fires, a positional predicate
// switched off the moment somebody wanders into the next room.
func requireUnconditional(t *testing.T, fset *token.FileSet, fn *ast.FuncDecl, pos token.Pos, msg string) {
	t.Helper()

	where := fset.Position(pos)
	ast.Inspect(fn, func(n ast.Node) bool {
		branch, ok := n.(*ast.IfStmt)
		if !ok {
			return true
		}
		require.False(t, branch.Pos() <= pos && pos < branch.End(),
			"%s:%d %s", where.Filename, where.Line, msg)

		return true
	})
}
