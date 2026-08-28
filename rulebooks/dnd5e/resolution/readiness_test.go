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

// TestNoCodePathProducesAReadinesslessInteraction is the STRUCTURAL half of the
// readiness install, and it is the SIXTH member of the family
// TestNoCodePathProducesACastlessInteraction was written for.
//
// gamectx had five registries and one install. This is the one that was missed:
// WithReactionReadiness had zero non-test callers in either repo, and
// IsReactionReady fails closed — so every reaction condition in the rulebook was
// gated behind a map nobody supplied, and the opportunity attack could not fire
// in any real interaction. Its own six-case suite is green because every one of
// those tests installs a readiness map by hand.
//
// That is the same failure a behavioural suite is structurally unable to catch,
// for the same reason: the tests supply what production does not. So the source
// is the assertion. The install is one statement at the top level of the door's
// body, and a condition wrapped around it is the defect by construction —
// whatever the condition said, there would be inputs it answered no for.
//
// It reads [installTruth] rather than resolveOn because that is where the
// install moved when the door was extracted. That the door is reached at all —
// the half this test cannot see from inside it — is
// TestOnlyTheDoorInstallsGameContext's.
func TestNoCodePathProducesAReadinesslessInteraction(t *testing.T) {
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
		if sel, ok := call.Fun.(*ast.SelectorExpr); ok && sel.Sel.Name == "WithReactionReadiness" {
			installs = append(installs, call.Pos())
		}

		return true
	})
	require.Len(t, installs, 1, "reaction readiness is installed in exactly one place")

	requireUnconditional(t, fset, fn, installs[0],
		"installs reaction readiness inside a condition — some input answers no, and a "+
			"reaction gate that is sometimes absent fails CLOSED, which is a reaction "+
			"that silently never fires")
}
