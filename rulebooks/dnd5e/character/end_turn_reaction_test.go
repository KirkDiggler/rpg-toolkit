// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package character_test

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

// TestNothingCallsEndTurn guards the one slot that must survive its owner's
// turn.
//
// [character.Character.EndTurn] zeroes ReactionsRemaining along with the rest
// of the economy. Every other field it clears is correct — you do not carry an
// action into someone else's turn. The reaction is the exception, because an
// opportunity attack happens on somebody else's turn BY DEFINITION: 5e refunds
// a reaction at the start of your turn precisely so you can spend it before
// your next one comes round.
//
// So the method is a loaded gun. Wiring it into the turn boundary would leave
// every combatant unable to react for the whole window the reaction governs,
// and it would do it QUIETLY — the OA condition's canReact would find an empty
// purse and decline, no error, no failed assertion, a green suite and a rule
// that never fires. This is the fail-silent shape, and the reason OA survives
// a turn boundary today is not that the code is careful. It is that nothing
// calls this method.
//
// That accident is what this pin makes into a statement. It reads SOURCE, not
// behavior, because there is no behavior to observe: the hazard is a call that
// does not exist yet, and the day it does exist is the day this must fail.
//
// # If this fires
//
// Do not add the caller to an allowlist. Ending a character's turn is a real
// thing a boundary may need to do — but it must leave ReactionsRemaining
// alone. Fix EndTurn to spare the reaction, then this pin has nothing to catch
// and can be replaced by a test of that behavior. The pin exists because the
// method is unreachable, and it should not outlive that fact.
//
// # Scope: this module
//
// Directories carrying their own go.mod are other modules — session, encounter,
// resolution — whose sources are not reliably on disk when this module is
// consumed as a dependency, and a walk that silently found nothing there would
// be worse than one that does not look. session is the likelier wiring site and
// states this law for itself; the same reasoning combat's
// TestOnlyTheKeeperNamesCombatant gives for its own boundary applies here.
//
// Within this module the breadth is safe rather than lucky: Character.EndTurn
// is the ONLY EndTurn defined here (the other two belong to the encounter and
// session modules), so every EndTurn selector in this module's production
// source is a call to this method. The pin therefore flags the bare name and
// does not try to infer receiver types — over-reporting is the direction to
// fail in.
//
// Test files are exempt: two of them call EndTurn to test EndTurn, which is
// the method being exercised rather than wired.
func TestNothingCallsEndTurn(t *testing.T) {
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
			return nil
		}

		// A test calling EndTurn is testing EndTurn, not wiring it.
		if !strings.HasSuffix(rel, ".go") || strings.HasSuffix(rel, "_test.go") {
			return nil
		}

		file, parseErr := parser.ParseFile(fset, path, nil, 0)
		require.NoError(t, parseErr)

		ast.Inspect(file, func(n ast.Node) bool {
			sel, ok := n.(*ast.SelectorExpr)
			if !ok || sel.Sel.Name != "EndTurn" {
				return true
			}

			offenders = append(offenders, fmt.Sprintf("%s: .EndTurn",
				fset.Position(sel.Pos())))

			return true
		})

		return nil
	})
	require.NoError(t, err)

	require.Empty(t, offenders,
		"Character.EndTurn zeroes ReactionsRemaining, and a reaction must survive its\n"+
			"owner's turn — an opportunity attack is spent on somebody else's turn. Calling\n"+
			"this method disarms every reactor for the window the reaction governs, and does\n"+
			"it silently. Fix EndTurn to spare the reaction rather than allowlisting the\n"+
			"caller; see this test's doc.\n%s",
		strings.Join(offenders, "\n"))
}
