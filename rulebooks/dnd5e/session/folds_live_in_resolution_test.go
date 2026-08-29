// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package session_test

import (
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

// gamectxPath is the package this seam may not reach into. Matched as a PATH
// rather than by the name "gamectx", so an alias or a dot-import does not walk
// an install past this scan.
const gamectxPath = "github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/gamectx"

// TestNoFoldLivesInThisModule is a TRIPWIRE, and calling it that rather than a
// proof is the point of this comment.
//
// # Why a structural test at all
//
// TestABarbarianJoiningReportsItsFoldedAC proves the armour class is FOLDED
// rather than echoed off the sheet — 15 rather than the stored 11 — which is
// the production bug this slice was named for. It does not prove where the fold
// ran, and that was measured rather than assumed: put Join back on
// ch.EffectiveAC and it still answers 15, because Join's own sheet is attached.
// The number is silent about exactly the thing the reroute changed.
//
// What the reroute changed is WHO folds. A fold needs game context, one door
// installs it, and that door is in resolution — so a seam folding where it
// stands is folding without the truth, whatever number comes out. This module
// obeys that by not folding at all: it hands over a record and takes back
// numbers.
//
// # What this catches, and what it cannot
//
// It matches two literal things across every non-test file in the module: an
// import of the gamectx path, and the name EffectiveAC. That covers the
// regressions it was built for — reinstating the fold this slice moved, or
// reaching for game context from the seam — including from a subpackage, since
// the walk is recursive, and however the import is spelled, since the match is
// on the path.
//
// IT CANNOT SEE A FOLD UNDER ANOTHER NAME. Wrap the same effect in a FoldedAC
// method and this is blind to it by construction: it matches a spelling, not an
// effect. It is a fence around the paths that were actually walked, not a proof
// that no other path exists, and a pin that oversold itself would be worse than
// no pin — the confidence would be real and the coverage would not.
//
// The exhaustive version is not more cleverness here, it is a different shape.
// When this module holds no bus and no attach machinery at all, a fold cannot
// run in it whatever anybody calls the method, and the guarantee stops
// depending on a name. That is where the design is heading; until it arrives,
// this is the honest half.
func TestNoFoldLivesInThisModule(t *testing.T) {
	fset := token.NewFileSet()

	var folds, reaches []string
	err := filepath.WalkDir(".", func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() {
			// Walked recursively on purpose: a helper subpackage that grew its
			// own gamectx import is exactly the innocent regression a
			// single-directory scan waves through.
			if path != "." && (strings.HasPrefix(entry.Name(), ".") || entry.Name() == "testdata") {
				return fs.SkipDir
			}

			return nil
		}
		if !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return nil
		}

		file, parseErr := parser.ParseFile(fset, path, nil, 0)
		if parseErr != nil {
			return parseErr
		}

		for _, spec := range file.Imports {
			if spec.Path.Value == `"`+gamectxPath+`"` {
				reaches = append(reaches, fset.Position(spec.Pos()).String())
			}
		}

		ast.Inspect(file, func(n ast.Node) bool {
			if sel, ok := n.(*ast.SelectorExpr); ok && sel.Sel.Name == "EffectiveAC" {
				folds = append(folds, fset.Position(sel.Pos()).String())
			}

			return true
		})

		return nil
	})
	require.NoError(t, err)

	require.Empty(t, folds,
		"a non-test file in this module names EffectiveAC. A fold needs game context, "+
			"one door installs it, and that door is in resolution — so a fold here runs "+
			"without the truth however plausible its number looks. Send the record to "+
			"resolution.ProjectCharacter and take back the breakdown")

	require.Empty(t, reaches,
		"a non-test file in this module imports gamectx. This seam holds RECORDS, not "+
			"truth: a live room and a live cast never exist above resolution, and a seam "+
			"that installs one has already broken its own law that no runtime object "+
			"crosses the boundary")
}
