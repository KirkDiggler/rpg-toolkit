// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package session_test

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter"
)

// beatKinds are the composition's exported beat kinds this package reads.
// Listed by constant, so the set follows the composition's own words.
var beatKinds = []string{
	encounter.BeatSceneOpened, encounter.BeatJoined, encounter.BeatExited, encounter.BeatMoved,
	encounter.BeatTick, encounter.BeatTurnEnded, encounter.BeatFightStarted, encounter.BeatFightEnded,
	encounter.BeatTransferred, encounter.BeatEnded, encounter.BeatActivated, encounter.BeatActivationResult,
	encounter.BeatDoor, encounter.BeatInteracted, encounter.BeatLooted, encounter.BeatHeld,
	encounter.BeatDropped, encounter.BeatStance, encounter.BeatArrived, encounter.BeatCast,
	encounter.BeatCastMissed, encounter.BeatCastWarded, encounter.BeatSaved, encounter.BeatConcentrationEnded,
	encounter.BeatRoomRevealed, encounter.BeatConcealmentRevealed, encounter.BeatDiscoveryChecked,
	encounter.BeatSighted, encounter.BeatIntimidated, encounter.BeatPersuaded, encounter.BeatAnswered,
	encounter.BeatTempered, encounter.BeatStayed, encounter.BeatWindowOpened, encounter.BeatRollWindowOpened,
	string(encounter.OutcomeStruck), string(encounter.OutcomeMissed), string(encounter.OutcomeDeathSave),
	string(encounter.OutcomeDown), string(encounter.OutcomeWarded), string(encounter.OutcomeExperienceGained),
	string(encounter.OutcomeBought), string(encounter.OutcomeSold),
}

// TestNoBeatKindIsMatchedByALiteral holds rpg-project#539's line: the beat
// kinds a client reads are constants the composition exports, and this
// package's projection maps constants, never string literals. A literal match
// keeps compiling after the composition renames the beat and silently answers
// "unknown"; a constant fails to compile.
//
// It flags a string literal naming a beat kind wherever it is MATCHED against
// something called a beat: a case in a switch on a beat, or either side of an
// == or != whose other side is a beat. A literal used for anything else (a
// JSON key, a verb name that happens to share a word) is not a match on a beat
// and is left alone.
func TestNoBeatKindIsMatchedByALiteral(t *testing.T) {
	kinds := make(map[string]bool, len(beatKinds))
	for _, kind := range beatKinds {
		kinds[kind] = true
	}
	literalKind := func(expr ast.Expr) (string, bool) {
		lit, ok := expr.(*ast.BasicLit)
		if !ok || lit.Kind != token.STRING {
			return "", false
		}
		value, err := strconv.Unquote(lit.Value)
		return value, err == nil && kinds[value]
	}
	namesABeat := func(expr ast.Expr) bool {
		var b strings.Builder
		ast.Inspect(expr, func(n ast.Node) bool {
			switch v := n.(type) {
			case *ast.Ident:
				b.WriteString(v.Name)
			case *ast.BasicLit:
				b.WriteString(v.Value)
			}
			return true
		})
		return strings.Contains(strings.ToLower(b.String()), "beat")
	}

	files, err := filepath.Glob("*.go")
	require.NoError(t, err)
	fset := token.NewFileSet()
	var found []string
	for _, name := range files {
		if strings.HasSuffix(name, "_test.go") {
			continue
		}
		src, err := os.ReadFile(name)
		require.NoError(t, err)
		file, err := parser.ParseFile(fset, name, src, 0)
		require.NoError(t, err)
		ast.Inspect(file, func(n ast.Node) bool {
			switch v := n.(type) {
			case *ast.SwitchStmt:
				if v.Tag == nil || !namesABeat(v.Tag) {
					return true
				}
				for _, stmt := range v.Body.List {
					for _, expr := range stmt.(*ast.CaseClause).List {
						if kind, ok := literalKind(expr); ok {
							found = append(found, fset.Position(expr.Pos()).String()+" case "+strconv.Quote(kind))
						}
					}
				}
			case *ast.BinaryExpr:
				if v.Op != token.EQL && v.Op != token.NEQ {
					return true
				}
				if kind, ok := literalKind(v.Y); ok && namesABeat(v.X) {
					found = append(found, fset.Position(v.Pos()).String()+" compares "+strconv.Quote(kind))
				}
				if kind, ok := literalKind(v.X); ok && namesABeat(v.Y) {
					found = append(found, fset.Position(v.Pos()).String()+" compares "+strconv.Quote(kind))
				}
			}
			return true
		})
	}
	require.Empty(t, found, "a beat kind matched by a string literal; use the composition's exported constant")
}
