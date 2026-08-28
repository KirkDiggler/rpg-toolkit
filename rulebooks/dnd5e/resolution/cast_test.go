// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package resolution

import (
	"go/ast"
	"go/parser"
	"go/token"
	"testing"

	"github.com/stretchr/testify/require"
)

// TestNoCodePathProducesACastlessInteraction is the STRUCTURAL half, and it is
// deliberately not a behavioural one. It is the same pin as
// TestNoCodePathProducesARoomlessInteraction and for a sharper reason.
//
// A behavioural test can only show that the interactions it happened to run got
// a cast. The claim rpg-toolkit#1251 needs is that no input CAN produce one
// without — and an optional ambient dependency is EXACTLY the defect being
// removed. gamectx had five installers and one install; three conditions read
// one of the four empty ones and returned its error into a chain fold that
// swallows errors, so a barbarian fought at base AC in every real fight and
// nothing was logged. Every test of those rules passed, because every test
// installed a registry by hand.
//
// That is the failure mode a behavioural suite is structurally unable to catch:
// the tests supply what production does not. So the source is the assertion.
// The install is one statement at the top level of the door's body, and a
// condition wrapped around it is the defect by construction — whatever the
// condition said, there would be inputs it answered no for.
//
// It reads [installTruth] rather than resolveOn because that is where the
// install moved when the door was extracted. That the door is reached at all —
// the half this test cannot see from inside it — is
// TestOnlyTheDoorInstallsGameContext's.
func TestNoCodePathProducesACastlessInteraction(t *testing.T) {
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, doorFile, nil, 0)
	require.NoError(t, err)

	fn := funcDecl(t, doorFile, file, "installTruth")

	var installs []token.Pos
	ast.Inspect(fn, func(n ast.Node) bool {
		call, ok := n.(*ast.CallExpr)
		if !ok {
			return true
		}
		if sel, ok := call.Fun.(*ast.SelectorExpr); ok && sel.Sel.Name == "WithCast" {
			installs = append(installs, call.Pos())
		}

		return true
	})
	require.Len(t, installs, 1, "the cast is installed in exactly one place")

	requireUnconditional(t, fset, fn, installs[0],
		"installs the cast inside a condition — some input answers no, and an ambient "+
			"dependency that is sometimes absent is rpg-toolkit#1251")
}
