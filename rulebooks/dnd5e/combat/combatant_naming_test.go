// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package combat_test

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

// combatPkgPath is the package whose Combatant this pin is about.
const combatPkgPath = "github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/combat"

// keeperProofs are the two files allowed to name [combat.Combatant] from
// outside this package, and they are allowed for the opposite of the usual
// reason: each is a one-line `var _ combat.Combatant = ...` whose entire job is
// to state that a sheet satisfies the keeper surface, checked by the compiler
// rather than inferred by a reader. A pin that forbade them would forbid
// writing the fact down.
var keeperProofs = map[string]bool{
	filepath.Join("character", "shield_surface_test.go"): true,
	filepath.Join("monster", "shield_test.go"):           true,
}

// TestOnlyTheKeeperNamesCombatant is the write law with the compiler behind it.
//
// [combat.Member] is what a rule reads and [combat.Combatant] is what the
// keeper writes through, and rpg-toolkit#1300 made the difference a type rather
// than a discipline. TestOnlyTheDoorWidensACastMember already says a rule
// cannot TURN a Member into the keeper's surface. This says the other half:
// outside this package, nobody NAMES that surface at all.
//
// The two together are why the split holds. A widening pin alone leaves open
// the case of code that never widens anything because it was handed a
// Combatant directly, which is exactly how the surface leaked before — and
// exactly what an integrationLookup holding a map[string]combat.Combatant was
// doing in integration/ until this pin was written. It turned out to be
// write-only, so making this statement true meant deleting dead code rather
// than widening the allowed set, which is the outcome to prefer.
//
// IT READS SOURCE, NOT CALLS, so it sees a type named in a signature or a
// field — where leaks actually live — and not the doc comments that mention
// Combatant in prose, which the parser discards. Same shape as
// resolution/truth_test.go's door pin.
//
// ITS SCOPE IS THIS MODULE. Directories carrying their own go.mod are other
// modules whose sources are not reliably on disk here; resolution states the
// same law for itself in TestOnlyStrikeNamesTheKeeperSurface, and that one
// carries the interesting half, because resolution IS the keeper.
func TestOnlyTheKeeperNamesCombatant(t *testing.T) {
	root := ".."
	fset := token.NewFileSet()

	var offenders []string

	err := filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}

		rel, relErr := filepath.Rel(root, path)
		require.NoError(t, relErr)

		if d.IsDir() {
			if rel == "." {
				return nil
			}
			// Another module: not ours to police, and it says so itself.
			if _, statErr := os.Stat(filepath.Join(path, "go.mod")); statErr == nil {
				return filepath.SkipDir
			}
			// This package owns the type.
			if rel == "combat" {
				return filepath.SkipDir
			}
			return nil
		}

		if !strings.HasSuffix(rel, ".go") || keeperProofs[rel] {
			return nil
		}

		file, parseErr := parser.ParseFile(fset, path, nil, 0)
		require.NoError(t, parseErr)

		local, imported := combatImportedAs(t, rel, file)
		if !imported {
			return nil
		}

		ast.Inspect(file, func(n ast.Node) bool {
			sel, ok := n.(*ast.SelectorExpr)
			if !ok {
				return true
			}
			pkg, ok := sel.X.(*ast.Ident)
			if !ok || pkg.Name != local || sel.Sel.Name != "Combatant" {
				return true
			}

			offenders = append(offenders, fmt.Sprintf("%s: %s.Combatant",
				fset.Position(sel.Pos()), local))

			return true
		})

		return nil
	})
	require.NoError(t, err)

	require.Empty(t, offenders,
		"only the keeper's own package names combat.Combatant; a rule reads a combat.Member.\n"+
			"If one of these is a deliberate keeper proof, add it to keeperProofs and say why:\n%s",
		strings.Join(offenders, "\n"))
}

// combatImportedAs returns the local name the combat package is bound to in
// this file, refusing a dot-import that would hide a naming from the walk.
func combatImportedAs(t *testing.T, filename string, file *ast.File) (string, bool) {
	t.Helper()

	quoted := `"` + combatPkgPath + `"`
	for _, spec := range file.Imports {
		if spec.Path.Value != quoted {
			continue
		}
		if spec.Name == nil {
			return "combat", true
		}
		require.NotEqual(t, ".", spec.Name.Name,
			"%s dot-imports combat, which hides namings from this pin", filename)
		if spec.Name.Name == "_" {
			return "", false
		}

		return spec.Name.Name, true
	}

	return "", false
}
