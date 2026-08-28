// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package resolution

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"sort"
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
// that no SECOND installer exists, and that the paths which fold reach the door
// at all. Drop the call to installTruth and all three still pass, because each
// of them is reading the door's own body.
//
// "The paths which fold" is a list now, not a single function — see
// [foldEntries]. Resolution grew a second entry when the projection landed, and
// the law it serves is that FOLDS LIVE IN RESOLUTION: a caller that needs a
// derived number brings the computation here rather than carrying truth out to
// itself. This pin holds the near half of that law, the half resolution can
// see — every fold that starts here starts by opening the door, and nothing
// else opens it.
//
// That gap is the defect they were written for, seen from the other side.
// gamectx reached five registries and a sixth in combat by exactly this route —
// each call site that needed a fact installed it where it stood, every one of
// them individually reasonable, and the set of them was what nobody checked
// (rpg-toolkit#1251). The count is the assertion, so the source is the
// assertion.
//
// REFERENCES, NOT CALLS. It matches any mention of a gamectx.With* name, not
// just a call of one — Copilot's finding on this PR, and correct: a pin that
// only walks *ast.CallExpr lets `with := gamectx.WithRoom` past, along with
// gamectx.WithRoom handed to something else as a value, either of which installs
// a fact from outside the door while reading as clean. Nothing in resolution may
// so much as NAME those functions except truth.go, which is both easier to check
// and easier to obey than a rule about how they are invoked.
//
// TEST FILES ARE INCLUDED, and that is the half worth defending. "Every test
// installed a registry by hand that production never installed" is not a
// footnote to rpg-toolkit#1251, it is the whole reason the bug survived two
// releases with a green suite — so a pin that exempts _test.go cannot see the
// thing it was written for. A test in THIS package that needs an installed
// context calls installTruth, the same door production calls. There is no
// second way in, for tests either, and that is the point.
//
// ITS SCOPE IS THIS PACKAGE. It reads the .go files in resolution and no
// others. conditions/ and monstertraits/ hand-install a context 43 times
// between their suites, and none of that is in violation or even visible here:
// they are separate modules whose sources are not on disk at test time, and
// standing one fold up without running Resolve is what those tests are for.
// Phase 3 takes those readers off the hand-installed path. Until then they are
// out of this pin's reach rather than exempt from it — a distinction worth
// keeping, because the day resolution grows a test that installs by hand is the
// day this pin has something real to say.
func TestOnlyTheDoorInstallsGameContext(t *testing.T) {
	fset := token.NewFileSet()

	entries, err := os.ReadDir(".")
	require.NoError(t, err)

	var offenders []string
	for _, entry := range entries {
		name := entry.Name()
		if entry.IsDir() || !strings.HasSuffix(name, ".go") || name == doorFile {
			continue
		}

		file, err := parser.ParseFile(fset, name, nil, 0)
		require.NoError(t, err)

		local, imported := importedAs(t, name, file, gamectxPath)
		if !imported {
			continue
		}

		ast.Inspect(file, func(n ast.Node) bool {
			sel, ok := n.(*ast.SelectorExpr)
			if !ok {
				return true
			}
			pkg, ok := sel.X.(*ast.Ident)
			if !ok || pkg.Name != local || !strings.HasPrefix(sel.Sel.Name, "With") {
				return true
			}

			offenders = append(offenders, fmt.Sprintf("%s: %s.%s",
				fset.Position(sel.Pos()), local, sel.Sel.Name))

			return true
		})
	}
	require.Empty(t, offenders,
		"%s is the only file allowed to name a game-context installer — a second "+
			"installer is how gamectx got five registries and one install "+
			"(rpg-toolkit#1251), and taking one as a value is still being one. If the "+
			"new fact belongs in context, it goes through installTruth; if it needs a "+
			"repository it is a record, not a tenant, and admission is a ruling either way",
		doorFile)

	// And the door is ON every path that reaches a fold. Everything above is
	// about who may install; this is the half that says the installing happens.
	for _, entry := range foldEntries {
		file, err := parser.ParseFile(fset, entry.file, nil, 0)
		require.NoError(t, err)

		fn := funcDecl(t, entry.file, file, entry.fn)

		calls := doorCalls(fn)
		require.Len(t, calls, 1, "%s goes through the door exactly once", entry.fn)

		requireUnconditional(t, fset, fn, calls[0],
			"calls the door inside a condition — a fold that skips it reads a world, "+
				"a cast and a readiness map that nobody installed, which is every "+
				"failure the three pins beside this one were written for at once")
	}

	// And nothing ELSE reaches the door, which is what makes the list above a
	// list rather than a sample. A new entry that folds is a design moment: it
	// either goes through installTruth and gets named here, or it folds without
	// truth, which is the bug R6 is about.
	require.Equal(t, entryNames(foldEntries), doorCallers(t, fset, entries),
		"every non-test caller of installTruth is a declared fold entry, and every "+
			"declared fold entry calls it — a fold reached any other way is a fold "+
			"outside the door")
}

// foldEntries are the functions in this package that stand a fold up from
// nothing, and therefore the functions that must go through the door first.
//
// TWO of them, and the second is why this is a table. Resolve runs an
// interaction; ProjectCharacter folds one derived number for a caller that has
// no interaction to run — a character joining a session, who is not standing
// anywhere yet. Both need the truth installed for the same reason and install
// it the same way; neither is a mode of the other.
//
// The functions named are the *On forms rather than their exported wrappers,
// because those are where the bus is held and the door is called. A wrapper
// that stopped delegating would fail funcDecl or the count, loudly.
var foldEntries = []struct{ file, fn string }{
	{"resolve.go", "resolveOn"},
	{"projection.go", "projectCharacterOn"},
}

func entryNames(entries []struct{ file, fn string }) []string {
	names := make([]string, 0, len(entries))
	for _, entry := range entries {
		names = append(names, entry.fn)
	}
	sort.Strings(names)

	return names
}

// doorCalls returns every position in fn where installTruth is called.
func doorCalls(fn *ast.FuncDecl) []token.Pos {
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

	return calls
}

// doorCallers returns the sorted names of every non-test top-level function in
// the package that calls installTruth.
//
// TEST FILES ARE EXCLUDED here, and the asymmetry with the scan above is
// deliberate rather than an oversight. That scan asks who may INSTALL, where a
// test installing by hand is the original defect wearing a disguise. This one
// asks which production paths fold, and a test calling the door is a test using
// the sanctioned entrance — the one this file tells authors to use. Counting
// those would turn a correct test into a failure and teach the wrong lesson.
func doorCallers(t *testing.T, fset *token.FileSet, entries []os.DirEntry) []string {
	t.Helper()

	var callers []string
	for _, entry := range entries {
		name := entry.Name()
		if entry.IsDir() || !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
			continue
		}

		file, err := parser.ParseFile(fset, name, nil, 0)
		require.NoError(t, err)

		for _, decl := range file.Decls {
			fn, ok := decl.(*ast.FuncDecl)
			if !ok || fn.Recv != nil || len(doorCalls(fn)) == 0 {
				continue
			}
			callers = append(callers, fn.Name.Name)
		}
	}
	sort.Strings(callers)

	return callers
}

// installsOf returns every position in fn where the installer called name is
// NAMED — called, assigned to a variable, or handed over as a value.
//
// References rather than calls, for the reason
// TestOnlyTheDoorInstallsGameContext gives at length and one this side has to
// state for itself. For a pin asserting an install is PRESENT, aliasing the only
// install is loud: the count falls to zero and the test fails. The case that
// would pass in silence is a SECOND install added by alias beside the direct one
// — the call count stays at one, the fact is installed twice, and "installed in
// exactly one place" is false with nothing to say so. Inside the door, which
// TestOnlyTheDoorInstallsGameContext exempts by design, that is the only pin
// left to catch it. Counting names costs nothing: an ordinary call names its
// function exactly once.
func installsOf(fn *ast.FuncDecl, name string) []token.Pos {
	var installs []token.Pos
	ast.Inspect(fn, func(n ast.Node) bool {
		if sel, ok := n.(*ast.SelectorExpr); ok && sel.Sel.Name == name {
			installs = append(installs, sel.Pos())
		}

		return true
	})

	return installs
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
