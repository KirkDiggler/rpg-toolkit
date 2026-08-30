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

// combatPath is the package whose Combatant this pin is about.
const combatPath = "github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/combat"

// keeperFile is the one file allowed to name it.
const keeperFile = "strike.go"

// TestOnlyStrikeNamesTheKeeperSurface is the half of the write law that
// carries the meaning, and it lives here because resolution IS the keeper.
//
// cast.go says so outright: this package applies the damage and builds the
// dirty set, so it hands out [combat.Combatant] internally and [combat.Member]
// at the seam where rules read. That makes "resolution may name the keeper
// surface" true but far too generous — it would let any fold in this package
// reach a sheet's writer. What is actually true is narrower and worth pinning:
// exactly one file names it, and within that file only where damage is applied
// (the applyDamage seam and its stored func) and where a combatant is looked
// up for it (combatantFor).
//
// dnd5e's TestOnlyTheKeeperNamesCombatant states the outer half — nothing
// outside the combat package names the type at all. That one is cheap and
// locks in the Phase 6 deletions. This one says the keeper does not spread
// inside its own module, which no compiler check gives us.
//
// PRODUCTION ONLY. Tests name the type where they substitute the applyDamage
// seam — cost_test.go and damage_custody_test.go both do, legitimately, since
// standing in for that seam means having its signature. Policing them would
// forbid testing the very thing this pin protects.
//
// It reads source, not calls, so it sees a type named in a signature or a
// field and ignores the prose in cast.go that discusses it.
func TestOnlyStrikeNamesTheKeeperSurface(t *testing.T) {
	fset := token.NewFileSet()

	entries, err := os.ReadDir(".")
	require.NoError(t, err)

	var offenders []string
	for _, entry := range entries {
		name := entry.Name()
		if entry.IsDir() || !strings.HasSuffix(name, ".go") ||
			strings.HasSuffix(name, "_test.go") || name == keeperFile {
			continue
		}

		file, parseErr := parser.ParseFile(fset, name, nil, 0)
		require.NoError(t, parseErr)

		local, imported := combatImportedAs(t, name, file)
		if !imported {
			continue
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
	}

	require.Empty(t, offenders,
		"resolution names combat.Combatant in %s alone — the applyDamage seam and combatantFor.\n"+
			"A fold that needs a sheet's writer surface is a fold doing the keeper's job:\n%s",
		keeperFile, strings.Join(offenders, "\n"))
}

// combatImportedAs returns the local name the combat package is bound to,
// refusing a dot-import that would hide a naming from this pin.
func combatImportedAs(t *testing.T, filename string, file *ast.File) (string, bool) {
	t.Helper()

	quoted := `"` + combatPath + `"`
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
