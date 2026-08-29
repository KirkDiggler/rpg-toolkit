// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package session_test

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

// gamectxPath is the package this seam may not reach into.
const gamectxPath = "github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/gamectx"

// TestNoFoldLivesInThisPackage is the structural half of the reroute, and it
// exists because the behavioural half cannot make the claim.
//
// TestABarbarianJoiningReportsItsFoldedAC proves the armour class is FOLDED
// rather than echoed off the sheet — 15 rather than the stored 11 — and that is
// the production bug this slice was named for. It does not prove where the fold
// happened, and that was measured rather than assumed: put Join back on
// ch.EffectiveAC and it still answers 15, because Join's own sheet is attached.
// So the number is silent about the thing the reroute changed.
//
// What the reroute changed is WHO folds. A fold needs game context, game
// context is installed by exactly one door, and that door is in resolution — so
// a seam that folds where it stands is folding without the truth, whatever
// number comes out. The rule is that the computation goes to resolution rather
// than the truth coming out to the caller, and the way this package obeys it is
// by never folding at all: no chain, no game context, no runtime object. It
// hands over a record and takes back numbers.
//
// Hence the source is the assertion. Two claims, both about absence, which is
// exactly the shape no behavioural test can carry.
func TestNoFoldLivesInThisPackage(t *testing.T) {
	fset := token.NewFileSet()

	entries, err := os.ReadDir(".")
	require.NoError(t, err)

	var folds, reaches []string
	for _, entry := range entries {
		name := entry.Name()
		if entry.IsDir() || !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
			continue
		}

		file, err := parser.ParseFile(fset, name, nil, 0)
		require.NoError(t, err)

		for _, spec := range file.Imports {
			if spec.Path.Value == `"`+gamectxPath+`"` {
				reaches = append(reaches, fset.Position(spec.Pos()).String())
			}
		}

		ast.Inspect(file, func(n ast.Node) bool {
			sel, ok := n.(*ast.SelectorExpr)
			if !ok || sel.Sel.Name != "EffectiveAC" {
				return true
			}
			folds = append(folds, fset.Position(sel.Pos()).String())

			return true
		})
	}

	require.Empty(t, folds,
		"this package folded an AC chain itself. A fold needs game context, one door "+
			"installs it, and that door is in resolution — so a fold here runs without "+
			"the truth however plausible its number looks. Send the record to "+
			"resolution.ProjectCharacter and take back the breakdown")

	require.Empty(t, reaches,
		"this package imported gamectx. It holds RECORDS, not truth: a live room and a "+
			"live cast never exist above resolution, and a seam that installs one has "+
			"already broken its own law that no runtime object crosses the boundary")
}
