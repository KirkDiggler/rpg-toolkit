package actions_test

import (
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestProductionPackageDoesNotImportProducersResolutionOrBus(t *testing.T) {
	forbidden := []string{
		"github.com/KirkDiggler/rpg-toolkit/events",
		"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/character",
		"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/monster",
		"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/resolution",
	}

	entries, err := os.ReadDir(".")
	require.NoError(t, err)

	files := token.NewFileSet()
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".go") || strings.HasSuffix(entry.Name(), "_test.go") {
			continue
		}

		path := filepath.Join(".", entry.Name())
		parsed, parseErr := parser.ParseFile(files, path, nil, parser.ImportsOnly)
		require.NoError(t, parseErr)

		for _, spec := range parsed.Imports {
			importPath, unquoteErr := strconv.Unquote(spec.Path.Value)
			require.NoError(t, unquoteErr)
			for _, prefix := range forbidden {
				if importPath == prefix || strings.HasPrefix(importPath, prefix+"/") {
					position := files.Position(spec.Pos())
					t.Errorf("%s imports forbidden dependency %q", position, importPath)
				}
			}
		}
	}
}
