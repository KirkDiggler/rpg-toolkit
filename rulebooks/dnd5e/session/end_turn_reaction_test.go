// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package session_test

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

// theClocksOwnEndTurn is the ONE receiver allowed to be sent EndTurn from this
// module's production source: the composition's own clock, which is what
// Manager.EndTurn exists to drive.
//
// Anything else is the hazard — see TestNothingHereEndsACharactersTurn.
const theClocksOwnEndTurn = "scope.enc"

// TestNothingHereEndsACharactersTurn is the session half of the guard the
// character package states for itself in TestNothingCallsEndTurn.
//
// [character.Character.EndTurn] zeroes ReactionsRemaining along with the rest
// of the economy. The reaction is the one slot that must SURVIVE its owner's
// turn, because a reaction is spent on somebody else's turn by definition —
// 5e refunds it at the start of your turn precisely so it can be spent before
// your next one.
//
// SESSION IS WHERE THAT WOULD ACTUALLY GET WIRED. The character package's own
// pin cannot see this module (separate go.mod, sources not reliably on disk
// when dnd5e is consumed as a dependency), and this is the module holding the
// turn boundary — Manager.EndTurn is right here. A well-meant "end the
// member's turn on their sheet too" is a two-line change away, and it would
// disarm every reactor for the whole window the reaction governs while every
// suite stayed green: the OA condition's canReact would find an empty purse
// and decline, which is indistinguishable from choosing not to react.
//
// Now that formation lights every combatant (rpg-project#316, ruling R2), the
// damage would be wider than it was: every member of a fight carries a real
// economy from the moment the bubble forms, so zeroing one is no longer a
// no-op on a cold sheet.
//
// # It reads source because there is nothing to observe
//
// The hazard is a call that does not exist yet. A behavioural test would have
// to assert that something nobody wrote does not happen.
//
// # Why an allowlist here and not in the character package
//
// There, Character.EndTurn was the ONLY EndTurn in the module, so the bare name
// was unambiguous. Here it is not: encounter.Encounter has its own EndTurn and
// driving it is exactly what this package's turn verb is for. So the pin allows
// that ONE receiver by name and flags every other, which fails in the loud
// direction — a new receiver has to be justified rather than silently accepted.
//
// If this fires: do not widen the allowlist to make it pass. Ending a
// character's turn is a legitimate thing a boundary may want; it must simply
// leave ReactionsRemaining alone.
func TestNothingHereEndsACharactersTurn(t *testing.T) {
	root := "."
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
			if _, statErr := os.Stat(filepath.Join(path, "go.mod")); statErr == nil {
				return filepath.SkipDir
			}
			return nil
		}

		// A test calling EndTurn is exercising a verb, not wiring one.
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
			receiver := renderReceiver(sel.X)
			if receiver == theClocksOwnEndTurn {
				return true
			}

			offenders = append(offenders, fmt.Sprintf("%s: %s.EndTurn",
				fset.Position(sel.Pos()), receiver))

			return true
		})

		return nil
	})
	require.NoError(t, err)

	require.Empty(t, offenders,
		"Character.EndTurn zeroes ReactionsRemaining, and a reaction must survive its\n"+
			"owner's turn — it is spent on somebody else's. Only %q may be sent EndTurn\n"+
			"here; that is the composition's clock. If one of these is a character sheet,\n"+
			"fix EndTurn to spare the reaction rather than widening the allowlist.\n%s",
		theClocksOwnEndTurn, strings.Join(offenders, "\n"))
}

// renderReceiver prints the receiver of a selector as source text, for the
// simple ident and selector chains a call site actually uses. Anything more
// elaborate renders as its type so the pin still names something a reader can
// find, and still fails.
func renderReceiver(x ast.Expr) string {
	switch e := x.(type) {
	case *ast.Ident:
		return e.Name
	case *ast.SelectorExpr:
		return renderReceiver(e.X) + "." + e.Sel.Name
	case *ast.CallExpr:
		return renderReceiver(e.Fun) + "()"
	case *ast.IndexExpr:
		return renderReceiver(e.X) + "[...]"
	case *ast.StarExpr:
		return "*" + renderReceiver(e.X)
	default:
		return fmt.Sprintf("%T", x)
	}
}
