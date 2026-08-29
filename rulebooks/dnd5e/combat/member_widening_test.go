// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package combat_test

import (
	"fmt"
	"go/ast"
	"go/importer"
	"go/parser"
	"go/token"
	"go/types"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"testing"

	"github.com/stretchr/testify/require"
)

// combatPath is the package that defines Member. Matched by import PATH, and
// then not matched by name at all: what follows resolves types rather than
// spellings, so an alias, a dot-import or a local type also called Member are
// simply different objects and never confusable with this one.
const (
	modulePath = "github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e"
	combatPath = modulePath + "/combat"
)

// theDoor is the one function allowed to widen a Member: GetEffectiveAC, which
// asks whether the sheet folds an AC chain. See [combat.GetEffectiveAC].
var theDoor = doorSite{path: "combat/combatant.go", fn: "GetEffectiveAC"}

type doorSite struct{ path, fn string }

func (d doorSite) covers(pos token.Position, enclosing string) bool {
	return strings.HasSuffix(filepath.ToSlash(pos.Filename), d.path) && enclosing == d.fn
}

// TestOnlyTheDoorWidensACastMember is the narrowing pin: the read surface the
// cast hands out cannot be turned back into a writer.
//
// # Why a test at all, when the compiler already did the work
//
// Cast members are the LIVE sheets — gamectx/cast.go says "a view, not a copy"
// and means it. Narrowing Cast.Member to combat.Member removes ApplyDamage,
// IsDirty and MarkClean from what a rule can reach, and the compiler holds that
// for free. It holds it against everything except one line:
//
//	sheet := member.(*character.Character)  // and the writes are back
//
// One assertion undoes the whole phase, reads as ordinary Go, and would sit in
// a rule nobody re-reads. That is the entire attack surface of D5, so it gets
// its own pin.
//
// # Two questions, because there are two ways to write that line
//
// It resolves types rather than matching source text, which is the difference
// between this and a grep. An offense is either:
//
//  1. The OPERAND is a combat.Member. Whatever it is asserted to — a concrete
//     sheet, an interface declared next door, an inline interface literal — a
//     Member is being turned into something else, and the only thing anyone
//     wants from that is a method Member does not have.
//
//  2. The TARGET is member-shaped and writes: it answers every question
//     combat.Member asks and offers at least one method that is not one of
//     them. This half catches the same escape laundered through an interface{}
//     — assign the member to an any, assert the any to *character.Character,
//     and question 1 sees an any while question 2 sees the destination for
//     what it is.
//
// Neither question can be defeated by naming: types are compared as objects, so
// `c "…/combat"` then `m.(c.Combatant)` is the same *types.Named as any other
// spelling of it. TestTheAliasEscapeIsClosed runs both shapes past the pin and
// requires them caught, rather than leaving that as a claim.
//
// # What it still cannot say
//
// A member handed to a function in ANOTHER module and widened there is out of
// reach — this scans the packages of this module, because a scan of a sibling
// module's directory pins a different artifact than the one that ships (the
// same limit session's no-bus pin states about resolution). resolution holds
// combatants legitimately and by construction: it IS the keeper.
//
// TEST FILES ARE INCLUDED. rpg-toolkit#1251 is the case where every test built
// what production never built, so a structural pin that exempts tests is blind
// to the shape it exists for.
//
// FAILING TO TYPE-CHECK IS A FAILURE, not a pass. A pin that quietly answers
// "no offenses" when the type-checker tripped is the fail-silent defect this
// whole slice is about, worn by the pin itself.
func TestOnlyTheDoorWidensACastMember(t *testing.T) {
	require.Equal(t, []string{}, widenings(t, ".."),
		"a cast member was widened outside %s. Cast members ARE the live sheets the "+
			"keepers mutate, so an assertion back to a writer surface hands a rule the "+
			"writes combat.Member exists to withhold, and the write law goes back to "+
			"being a convention with nothing behind it. A rule that needs a sheet "+
			"changed publishes a request; the keeper applies it",
		theDoor.fn)
}

// TestTheDoorOpensOntoAReader pins the far side of the one sanctioned widening.
//
// A door is only as safe as the room behind it. GetEffectiveAC may widen a
// Member because EffectiveACCalculator asks one question and answers with a
// number; a method added to that interface would be reachable from every cast
// member in the rulebook, through a widening this file's other test deliberately
// permits.
func TestTheDoorOpensOntoAReader(t *testing.T) {
	_, imp := sources()
	combatPkg, err := imp.Import(combatPath)
	require.NoError(t, err)

	calc, ok := combatPkg.Scope().Lookup("EffectiveACCalculator").Type().Underlying().(*types.Interface)
	require.True(t, ok, "EffectiveACCalculator is an interface")

	methods := make([]string, 0, calc.NumMethods())
	for i := range calc.NumMethods() {
		methods = append(methods, calc.Method(i).Name())
	}
	require.Equal(t, []string{"EffectiveAC"}, methods,
		"the one sanctioned widening of a Member is GetEffectiveAC's, and it is only "+
			"sanctioned because what is on the other side of it reads. A method added "+
			"here is a method every cast member in the rulebook can be widened into")
}

// TestTheAliasEscapeIsClosed runs the pin over the escapes it exists to catch,
// so the guarantee is a passing test rather than a claim about one.
//
// testdata/escape holds the three shapes: the concrete sheet, the keeper
// interface reached through an ALIASED import, and the laundering pass through
// an any. The go tool ignores testdata, so those files never build and never
// ship; the pin type-checks them on purpose, against the same combat package
// production compiles against.
func TestTheAliasEscapeIsClosed(t *testing.T) {
	found := widenings(t, filepath.Join("testdata", "escape"))

	require.Len(t, found, 3,
		"every planted escape must be caught: the concrete sheet, the aliased keeper "+
			"interface, and the one laundered through an any")
	require.Contains(t, strings.Join(found, "\n"), "aliased.go",
		"an aliased import must not walk a widening past this pin — c.Combatant is "+
			"combat.Combatant wearing a different name")
}

// sources hands out the one file set and the one source importer these tests
// share.
//
// Shared because type-checking the rulebook from source is the whole cost of
// this file, and every test here needs the same dependency graph: built once,
// the second and third readers are nearly free. A cache and nothing else — no
// test learns anything from another, and both values are append-only.
var sources = sync.OnceValues(func() (*token.FileSet, types.Importer) {
	fset := token.NewFileSet()

	return fset, importer.ForCompiler(fset, "source", nil)
})

// widenings reports every place a combat.Member is widened outside the door,
// across the packages of this module rooted at root.
func widenings(t *testing.T, root string) []string {
	t.Helper()

	fset, imp := sources()

	offenders := []string{}
	for _, dir := range packageDirs(t, root) {
		pkgPath := importPathOf(t, root, dir)
		for _, files := range parsePackages(t, fset, dir) {
			if !worthChecking(files, pkgPath) {
				continue
			}

			info := &types.Info{Types: map[ast.Expr]types.TypeAndValue{}}
			var failures []string
			conf := types.Config{
				Importer: imp,
				Error:    func(err error) { failures = append(failures, err.Error()) },
			}
			checked, _ := conf.Check(pkgPath, fset, files, info)
			require.Empty(t, failures,
				"%s must type-check for this pin to mean anything — an unchecked package "+
					"reports no widenings because it reports no types", dir)

			member, ok := memberSeenBy(checked)
			require.True(t, ok,
				"%s reaches the combat package but combat.Member could not be resolved in "+
					"it. Renaming Member would silence this pin package by package with "+
					"nothing failing, which is the shape of defect it exists to catch", dir)

			offenders = append(offenders, scan(fset, files, info, member)...)
		}
	}
	sort.Strings(offenders)

	return offenders
}

// memberSeenBy returns combat.Member AS THIS TYPE-CHECK SEES IT.
//
// Per check, and never once up front, because two type-checks of the same
// source produce two unrelated *types.Named for the same declaration.
// Resolving Member once and comparing it against every package would make
// types.Identical false everywhere — the pin would still catch what the second
// question catches and would answer "no" to the first one forever. It did, for
// exactly one run, until a planted escape came back reported by the wrong half.
//
// A package that imports combat sees the importer's copy. The combat package
// itself sees the one it just declared; both are correct for the files being
// scanned, which is the whole requirement.
//
// Not finding one is a FAILURE at the call site, never a skipped package. Every
// package that gets here reaches combat, so "no Member in scope" means the type
// was renamed or removed — and a pin that answered that by scanning nothing
// would go green across the whole rulebook in one commit.
func memberSeenBy(checked *types.Package) (*types.Named, bool) {
	if checked == nil {
		return nil, false
	}

	scope := (*types.Scope)(nil)
	for _, imported := range checked.Imports() {
		if imported.Path() == combatPath {
			scope = imported.Scope()
		}
	}
	if scope == nil && checked.Path() == combatPath {
		scope = checked.Scope()
	}
	if scope == nil {
		return nil, false
	}

	obj := scope.Lookup("Member")
	if obj == nil {
		return nil, false
	}
	named, ok := obj.Type().(*types.Named)

	return named, ok
}

// importPathOf gives a directory the import path it will really have, so that
// checked.Path() can be trusted to identify the combat package.
func importPathOf(t *testing.T, root, dir string) string {
	t.Helper()

	rel, err := filepath.Rel(root, dir)
	require.NoError(t, err)
	if rel == "." {
		return modulePath
	}

	return modulePath + "/" + filepath.ToSlash(rel)
}

// scan walks the type-checked files and reports both offense shapes.
func scan(fset *token.FileSet, files []*ast.File, info *types.Info, member *types.Named) []string {
	memberIface, ok := member.Underlying().(*types.Interface)
	if !ok {
		return nil
	}
	reads := make(map[string]bool, memberIface.NumMethods())
	for i := range memberIface.NumMethods() {
		reads[memberIface.Method(i).Name()] = true
	}

	var offenders []string
	for _, file := range files {
		for _, decl := range file.Decls {
			fn, isFunc := decl.(*ast.FuncDecl)
			enclosing := ""
			if isFunc {
				enclosing = fn.Name.Name
			}

			ast.Inspect(decl, func(n ast.Node) bool {
				assert, ok := n.(*ast.TypeAssertExpr)
				if !ok {
					return true
				}
				pos := fset.Position(assert.Pos())
				if theDoor.covers(pos, enclosing) {
					return true
				}

				if operand := info.TypeOf(assert.X); operand != nil && types.Identical(operand, member) {
					offenders = append(offenders,
						fmt.Sprintf("%s: a combat.Member is widened here", pos))

					return true
				}
				// A nil Type is the guard of a type switch; its cases are
				// asserted types and are reached by the walk in their own right.
				if assert.Type == nil {
					return true
				}
				if target := info.TypeOf(assert.Type); target != nil && writes(target, memberIface, reads) {
					offenders = append(offenders,
						fmt.Sprintf("%s: asserted to %s, which is a member that writes", pos, target))
				}

				return true
			})
		}
	}

	return offenders
}

// writes reports whether t is something a cast member could be, carrying a
// method a cast member must not reach.
func writes(t types.Type, memberIface *types.Interface, reads map[string]bool) bool {
	if !types.Implements(t, memberIface) {
		return false
	}
	set := types.NewMethodSet(t)
	for i := range set.Len() {
		if !reads[set.At(i).Obj().Name()] {
			return true
		}
	}

	return false
}

// worthChecking reports whether a package can hold an offense at all: it must
// reach the combat package — or BE it, which is how the door itself gets
// scanned — and it must assert a type somewhere. Type-checking is the expensive
// part, and a package that asserts nothing cannot widen anything.
func worthChecking(files []*ast.File, pkgPath string) bool {
	reachesCombat, asserts := pkgPath == combatPath, false
	for _, file := range files {
		for _, spec := range file.Imports {
			if spec.Path.Value == `"`+combatPath+`"` {
				reachesCombat = true
			}
		}
		ast.Inspect(file, func(n ast.Node) bool {
			if _, ok := n.(*ast.TypeAssertExpr); ok {
				asserts = true
			}

			return !asserts
		})
	}

	return reachesCombat && asserts
}

// parsePackages parses one directory into its packages, in filename order.
//
// Hand-rolled rather than parser.ParseDir because that returns the deprecated
// ast.Package. Test files are included: rpg-toolkit#1251 is the case where
// every test built what production never built.
func parsePackages(t *testing.T, fset *token.FileSet, dir string) map[string][]*ast.File {
	t.Helper()

	entries, err := os.ReadDir(dir)
	require.NoError(t, err)

	pkgs := map[string][]*ast.File{}
	for _, entry := range entries {
		name := entry.Name()
		if entry.IsDir() || !strings.HasSuffix(name, ".go") {
			continue
		}

		file, parseErr := parser.ParseFile(fset, filepath.Join(dir, name), nil, 0)
		require.NoError(t, parseErr)
		pkgs[file.Name.Name] = append(pkgs[file.Name.Name], file)
	}

	return pkgs
}

// packageDirs lists the directories of THIS module under root.
//
// A directory holding a go.mod is skipped whole: it is a different module,
// published on its own tag, and scanning its working copy would pin source that
// this module's build never sees.
func packageDirs(t *testing.T, root string) []string {
	t.Helper()

	var dirs []string
	err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !entry.IsDir() {
			return nil
		}
		if path != root {
			if _, statErr := os.Stat(filepath.Join(path, "go.mod")); statErr == nil {
				return fs.SkipDir
			}
			if strings.HasPrefix(entry.Name(), ".") || entry.Name() == "testdata" {
				return fs.SkipDir
			}
		}
		dirs = append(dirs, path)

		return nil
	})
	require.NoError(t, err)

	return dirs
}
