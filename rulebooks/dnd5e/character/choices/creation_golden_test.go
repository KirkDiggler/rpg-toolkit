// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package choices_test

import (
	"encoding/json"
	"flag"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/character/choices"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/classes"
)

// updateGolden rewrites the golden file instead of comparing against it.
// Run with `go test ./character/choices -run TestCreationRequirementsGolden -update`.
var updateGolden = flag.Bool("update", false, "rewrite the creation requirements golden file")

// goldenPath is the recorded creation surface: every class's level-1
// requirements, whole, as they were before advancement made requirements
// level-indexed data.
const goldenPath = "testdata/creation_requirements.json"

// TestCreationRequirementsGolden is the regression test for the level-indexed
// requirement table (design R4.4: "Every existing level-1 requirement MUST come
// through the new table unchanged, choice IDs byte-identical. Creation is the
// regression test: if a level-1 character can still be built choice for choice,
// the table is faithful.").
//
// It pins the WHOLE requirement struct, not just the identifiers, because the
// count, the option list and the spell level are each a fact a table row could
// get wrong in a way no choice ID would show. The dump is what a creation
// client is handed, so a diff here is a diff a player would see.
func TestCreationRequirementsGolden(t *testing.T) {
	dump := make(map[string]*choices.Requirements, len(classes.ClassData))
	for classID := range classes.ClassData {
		dump[string(classID)] = choices.GetClassRequirements(classID)
	}

	encoded, err := json.MarshalIndent(dump, "", "  ")
	require.NoError(t, err)
	encoded = append(encoded, '\n')

	if *updateGolden {
		require.NoError(t, os.MkdirAll(filepath.Dir(goldenPath), 0o750))
		require.NoError(t, os.WriteFile(goldenPath, encoded, 0o600))
		t.Logf("wrote %s", goldenPath)
		return
	}

	want, err := os.ReadFile(goldenPath) //nolint:gosec // fixed test path
	require.NoError(t, err, "golden file missing; regenerate with -update")
	require.JSONEq(t, string(want), string(encoded),
		"level-1 requirements moved: the per-level table is not faithful to creation (R4.4)")
}
