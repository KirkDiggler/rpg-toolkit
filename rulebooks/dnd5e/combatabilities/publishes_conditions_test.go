// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package combatabilities_test

import (
	"go/ast"
	"go/parser"
	"go/token"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

// TestNothingAppliesAConditionToTheBusItself is the structural guard
// rpg-toolkit#1272 exists to install.
//
// # The bug it prevents
//
// A condition reaches a CHARACTER only through ConditionAppliedTopic: the
// owner's SheetKeeper subscribes to it, applies what arrives, and records it so
// ToData writes it down. An ability that calls condition.Apply(ctx, input.Bus)
// itself skips all of that. The condition works — on the interaction's bus,
// for the length of one call — and then ADR-0038 tears the bus down and it is
// gone. Nothing errors. Nothing logs. The sheet is written without it.
//
// Disengage shipped that way and did nothing for a player: you spent your
// action and still provoked, because the next move is a different interaction
// with a different bus. Step of the Wind's disengage branch had the identical
// line. Two out of two occurrences in the module were bugs, which is the ratio
// that makes this worth pinning structurally rather than by example.
//
// # Why AST rather than a test per ability
//
// A behavioural test proves one ability publishes. This proves NO ability
// applies — including the one somebody adds next year by copying the file that
// still looked reasonable. The tests that missed the original bug were not
// missing, they were asking the wrong question: they drove the condition on a
// bare bus, where a direct Apply and a publish-with-listener are
// indistinguishable.
func TestNothingAppliesAConditionToTheBusItself(t *testing.T) {
	// Both packages that author activations, scanned together: the defect
	// appeared once in each, and a guard installed in only one of them would
	// have caught only half of it.
	dirs := []string{".", "../features"}

	var offenders []string
	for _, dir := range dirs {
		paths, err := filepath.Glob(filepath.Join(dir, "*.go"))
		require.NoError(t, err)

		for _, path := range paths {
			if strings.HasSuffix(path, "_test.go") {
				continue
			}
			file, err := parser.ParseFile(token.NewFileSet(), path, nil, 0)
			require.NoError(t, err, "parse %s", path)

			for _, decl := range file.Decls {
				fn, ok := decl.(*ast.FuncDecl)
				if !ok || fn.Body == nil {
					continue
				}
				offenders = append(offenders, appliedConditionsIn(path, fn)...)
			}
		}
	}

	require.Empty(t, offenders,
		"an ability must PUBLISH its condition on ConditionAppliedTopic, never apply it "+
			"to the bus itself — a directly applied condition never reaches the owner's "+
			"sheet and dies with the interaction (rpg-toolkit#1272)")
}

// appliedConditionsIn finds, within one function, every value built by a
// conditions.NewX constructor that then has Apply called on it.
//
// Narrow ON PURPOSE. A feature's own Apply method — SecondWind.Apply,
// ActionSurge.Apply — delegates to a recoverable resource so it recovers on a
// rest, which is attachment rather than a condition being applied and is
// entirely correct. An earlier draft of this guard flagged both, and a guard
// that cries wolf gets deleted by the third person who trips over it.
func appliedConditionsIn(path string, fn *ast.FuncDecl) []string {
	built := map[string]bool{}
	ast.Inspect(fn.Body, func(n ast.Node) bool {
		assign, ok := n.(*ast.AssignStmt)
		if !ok {
			return true
		}
		for i, rhs := range assign.Rhs {
			call, ok := rhs.(*ast.CallExpr)
			if !ok || i >= len(assign.Lhs) {
				continue
			}
			selector, ok := call.Fun.(*ast.SelectorExpr)
			if !ok {
				continue
			}
			pkg, ok := selector.X.(*ast.Ident)
			if !ok || pkg.Name != "conditions" || !strings.HasPrefix(selector.Sel.Name, "New") {
				continue
			}
			if name, ok := assign.Lhs[i].(*ast.Ident); ok {
				built[name.Name] = true
			}
		}
		return true
	})
	if len(built) == 0 {
		return nil
	}

	var found []string
	ast.Inspect(fn.Body, func(n ast.Node) bool {
		call, ok := n.(*ast.CallExpr)
		if !ok {
			return true
		}
		selector, ok := call.Fun.(*ast.SelectorExpr)
		if !ok || selector.Sel.Name != "Apply" {
			return true
		}
		receiver, ok := selector.X.(*ast.Ident)
		if !ok || !built[receiver.Name] {
			return true
		}
		found = append(found, path+": "+fn.Name.Name+" applies "+receiver.Name)
		return true
	})
	return found
}
