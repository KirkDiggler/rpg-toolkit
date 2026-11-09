// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package items_test

import (
	"go/parser"
	"go/token"
	"os"
	"strconv"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

// Rulebook code and dice belong to consumers. This guards the actual package,
// not the consumer tests that intentionally use real Second Wind healing.
func TestItemPackageStaysInsideItsGenericSeam(t *testing.T) {
	allowed := map[string]bool{
		"github.com/KirkDiggler/rpg-toolkit/core":                      true,
		"github.com/KirkDiggler/rpg-toolkit/events":                    true,
		"github.com/KirkDiggler/rpg-toolkit/mechanics/effects":         true,
		"github.com/KirkDiggler/rpg-toolkit/mechanics/effects/machine": true,
		"github.com/KirkDiggler/rpg-toolkit/mechanics/resources":       true,
	}
	entries, err := os.ReadDir(".")
	require.NoError(t, err)
	for _, entry := range entries {
		name := entry.Name()
		if entry.IsDir() || !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
			continue
		}
		file, err := parser.ParseFile(token.NewFileSet(), name, nil, parser.ImportsOnly)
		require.NoError(t, err)
		for _, imp := range file.Imports {
			path, err := strconv.Unquote(imp.Path.Value)
			require.NoError(t, err)
			if strings.Contains(strings.Split(path, "/")[0], ".") {
				require.True(t, allowed[path], "%s imports %s outside the generic item seam", name, path)
			}
		}
	}
}
