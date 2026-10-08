// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package session_test

import (
	"go/ast"
	"go/parser"
	"go/token"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

// storeFile is the one non-test source allowed to call the host's character
// repository (rpg-project#542, "One sheet store per verb").
const storeFile = "store.go"

// TestOnlyTheStoreCallsTheCharacterRepository holds the store law: every
// character record a verb reads or writes goes through its one sheet store,
// so the absent-sheet answer and the save report each live in one place. A
// seam, builder or machine calling GetCharacter or SaveCharacter on its own
// would be a second answer to "is this sheet missing" and a write no report
// names.
//
// Matched by method name on any receiver, so an alias or a field rename
// cannot walk a call past it. The repository interface's own declaration is
// not a call and is not flagged.
func TestOnlyTheStoreCallsTheCharacterRepository(t *testing.T) {
	files, err := filepath.Glob("*.go")
	require.NoError(t, err)
	fset := token.NewFileSet()
	var found []string
	calledInStore := map[string]bool{}
	for _, name := range files {
		if strings.HasSuffix(name, "_test.go") {
			continue
		}
		file, err := parser.ParseFile(fset, name, nil, 0)
		require.NoError(t, err)
		ast.Inspect(file, func(node ast.Node) bool {
			call, ok := node.(*ast.CallExpr)
			if !ok {
				return true
			}
			selector, ok := call.Fun.(*ast.SelectorExpr)
			if !ok {
				return true
			}
			method := selector.Sel.Name
			if method != "GetCharacter" && method != "SaveCharacter" {
				return true
			}
			if name == storeFile {
				calledInStore[method] = true
				return true
			}
			found = append(found, fset.Position(call.Pos()).String()+": "+method)
			return true
		})
	}
	require.Empty(t, found, "only %s calls the character repository", storeFile)
	require.True(t, calledInStore["GetCharacter"], "the store reads the repository")
	require.True(t, calledInStore["SaveCharacter"], "the store writes the repository")
}
