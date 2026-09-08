// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package session_test

import (
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

// rulebookEventsPath is the package that owns the damage fact a concentration
// check hangs off, and the follow-up carrier it travels in. Matched by import
// PATH, so an alias cannot walk one in under another name.
const rulebookEventsPath = "github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/events"

// savesPath is the package that turns an ability and a DC into an answer.
const savesPath = "github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/saves"

// conditionsPath is the package that owns the concentrating condition itself.
const conditionsPath = "github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/conditions"

// checkMachinery are the entries in the rulebook's events package that stand a
// concentration check up: the fact it triggers on, the event carrying it, the
// follow-up a condition answers with, and the trigger word the save is asked
// under. Reaching ANY of them from this module is this seam deciding a rule.
var checkMachinery = map[string]bool{
	"DamageTakenTopic":         true,
	"DamageTakenEvent":         true,
	"FollowUp":                 true,
	"Consequence":              true,
	"SaveTriggerConcentration": true,
}

// TestSessionConstructsNoCheck is the whole point of concentration's session
// slice, held as a test rather than as a comment.
//
// # The ruling
//
// Kirk, on where the concentration check lives: *"keep it out of session for
// sure. resolution is the place that should resolve things. it shouldn't have
// to leak out."* So the check runs inside the interaction that caused it — the
// strike, or the cast — and this seam projects what the composition recorded
// and decides nothing. It appends no beat of its own, computes no DC, asks for
// no saving throw, and never learns that damage was taken except as a number
// already written into an outcome.
//
// # What would break it, and what this catches
//
// The shape that would leak the rule up here is small and reads as helpful:
// the session takes the damage back out of a strike outcome, computes
// max(10, amount/2), and asks resolution for a save. Every piece of that needs
// a name from one of three packages — the damage fact and its follow-up from
// the rulebook's events package, the save from saves, the condition itself
// from conditions — and this refuses all three.
//
// Imports are RESOLVED rather than matched as text, so an alias is no escape;
// TestTheAliasEscapeIsClosed already proves that machinery on its sibling pin
// and TestTheCheckEscapeIsCaught below proves it on this one. A dot-import of
// any of the three is refused outright rather than analysed, for the reason
// that pin gives: it puts the names in scope as bare identifiers, and telling
// those from any other identifier would be guessing.
//
// # Non-test files only, and why this one differs from its neighbour
//
// TestNoBusLivesInThisModule scans tests too, and says why: nothing in this
// module imports a bus, production or test, so the strong form costs nothing.
// Here it would cost the suite its fixtures. The tests in this package build
// worlds — a bard holding True Strike, a skeleton that swings — and building
// one legitimately names the condition and the save that a rule uses. What the
// ruling forbids is the SEAM doing it at runtime, which is what ships.
//
// # What it cannot say
//
// It does not prove the check is correct, or that resolution runs one at all.
// It proves this module cannot be the place that does, which is the specific
// claim slice three makes about it.
func TestSessionConstructsNoCheck(t *testing.T) {
	fset := token.NewFileSet()

	var machinery, saveImports, conditionImports []string
	err := filepath.WalkDir(".", func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() {
			// Walked recursively for the reason the bus pin walks: a helper
			// subpackage that grew a check is exactly the innocent regression
			// a single-directory scan waves through.
			if path != "." && (strings.HasPrefix(entry.Name(), ".") || entry.Name() == "testdata") {
				return fs.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return nil
		}

		machinery = append(machinery, checkMachineryReachedBy(t, path)...)

		file, parseErr := parser.ParseFile(fset, path, nil, parser.ImportsOnly)
		if parseErr != nil {
			return parseErr
		}
		for _, spec := range file.Imports {
			switch spec.Path.Value {
			case `"` + savesPath + `"`:
				saveImports = append(saveImports, fset.Position(spec.Pos()).String())
			case `"` + conditionsPath + `"`:
				conditionImports = append(conditionImports, fset.Position(spec.Pos()).String())
			}
		}
		return nil
	})
	require.NoError(t, err)

	require.Empty(t, machinery,
		"a non-test file in this module reaches the machinery a concentration check is "+
			"built from. The check runs inside the interaction that caused it, in resolution, "+
			"and this seam projects the beat the composition already recorded. A damage fact "+
			"or a follow-up read here is this seam deciding a rule it does not own")

	require.Empty(t, saveImports,
		"a non-test file in this module imports the saves package. Computing a DC or asking "+
			"for a saving throw is resolution's work; this seam records the answer somebody "+
			"else got")

	require.Empty(t, conditionImports,
		"a non-test file in this module imports the conditions package. The concentrating "+
			"condition ends itself and strips its own children; a seam holding it would be "+
			"holding a rule")
}

// TestTheCheckEscapeIsCaught runs this pin over the shape it exists to refuse,
// so the guarantee is a passing test rather than a claim — the same probe
// TestTheAliasEscapeIsClosed keeps for its sibling, and written the same way:
// through an alias, because an alias is what defeated the first version of
// that one.
func TestTheCheckEscapeIsCaught(t *testing.T) {
	dir := t.TempDir()
	leaked := filepath.Join(dir, "leaked.go")
	require.NoError(t, os.WriteFile(leaked, []byte(`package session

import ev "`+rulebookEventsPath+`"

var _ = ev.DamageTakenTopic
`), 0o600))

	require.NotEmpty(t, checkMachineryReachedBy(t, leaked),
		"a session file that reads the damage fact a concentration check hangs off must not "+
			"walk past this pin, whatever the import is called locally")
}

// checkMachineryReachedBy reports every check-building entry the file reaches
// through the rulebook's events package, however that package is named
// locally.
func checkMachineryReachedBy(t *testing.T, path string) []string {
	t.Helper()

	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, path, nil, 0)
	require.NoError(t, err)

	local := ""
	for _, spec := range file.Imports {
		if spec.Path.Value != `"`+rulebookEventsPath+`"` {
			continue
		}
		if spec.Name == nil {
			local = "events"
			continue
		}
		if spec.Name.Name == "." {
			// A dot-import puts every name in scope bare. Refused rather than
			// analysed, exactly as the standing pin refuses one.
			require.Failf(t, "dot-import of the rulebook events package",
				"%s dot-imports %s. This pin cannot tell a bare DamageTakenEvent from any "+
					"other identifier, so the import shape itself is refused",
				path, rulebookEventsPath)
		}
		local = spec.Name.Name
	}
	if local == "" {
		return nil
	}

	var reached []string
	ast.Inspect(file, func(node ast.Node) bool {
		selector, ok := node.(*ast.SelectorExpr)
		if !ok {
			return true
		}
		pkg, ok := selector.X.(*ast.Ident)
		if !ok || pkg.Name != local || !checkMachinery[selector.Sel.Name] {
			return true
		}
		reached = append(reached, fset.Position(selector.Pos()).String()+": "+selector.Sel.Name)
		return true
	})
	return reached
}
