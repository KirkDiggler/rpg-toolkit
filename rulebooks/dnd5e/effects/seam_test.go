// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package effects_test

import (
	"os/exec"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

// The vocabulary must not pull its condition registry or item consumer below
// the rulebook plug. Inspect the actual production dependency graph, not aliases.
func TestVocabularyStaysBelowConditionsAndItems(t *testing.T) {
	command := exec.Command("go", "list", "-deps", ".")
	output, err := command.CombinedOutput()
	require.NoError(t, err, string(output))
	for _, path := range strings.Fields(string(output)) {
		require.NotEqual(t, "github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/conditions", path)
		require.False(t, strings.HasPrefix(path, "github.com/KirkDiggler/rpg-toolkit/examples/"), path)
		require.False(t, strings.HasPrefix(path, "github.com/KirkDiggler/rpg-api"), path)
	}
}
