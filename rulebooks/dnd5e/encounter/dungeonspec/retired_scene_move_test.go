// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package dungeonspec_test

// retired_scene_move_test.go is `sceneMove` RETIRED (rpg-project#1133, road
// step 1 of rpg-project#989): a row moves its speaker with `relocate`, and a
// guard leaves by its own machine stepping on a fact a row teaches. A site
// that still writes it is refused at load, at its own line, in a sentence
// naming what to write instead.

import (
	"fmt"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter/dungeonspec"
)

const sceneMoveRetired = "`sceneMove` is retired: a row moves its speaker with relocate: { to, words, rounds }, " +
	"and a guard leaves by its own machine stepping on a fact a row teaches (fact: <id>, then on: fact)"

// lineOf is the 1-based line of the one place the source says the words.
func lineOf(t *testing.T, source, words string) int {
	t.Helper()
	at := strings.Index(source, words)
	require.GreaterOrEqual(t, at, 0, "the source says %q", words)
	return strings.Count(source[:at], "\n") + 1
}

func TestRetiredSceneMove_AConversationOutcomeIsRefusedAtItsLine(t *testing.T) {
	source := castleWith(t, theOutcome, "        - sceneMove: { leave: gone }\n")
	_, err := dungeonspec.Load([]byte(source))
	require.Error(t, err)
	require.Contains(t, err.Error(), fmt.Sprintf("line %d: %s", lineOf(t, source, "sceneMove: { leave: gone }"), sceneMoveRetired))
}

func TestRetiredSceneMove_AMachineStatesOutcomeIsRefusedAtItsLine(t *testing.T) {
	source := outcomesSiteWith(outcomesStates, "    outcomes:\n      back-to-gate:\n        - sceneMove: { leave: gone }\n", "")
	_, err := dungeonspec.Load([]byte(source))
	require.Error(t, err)
	require.Contains(t, err.Error(), fmt.Sprintf("line %d: %s", lineOf(t, source, "sceneMove: { leave: gone }"), sceneMoveRetired))
}

func TestRetiredSceneMove_ASiteThatWritesNeitherStillLoads(t *testing.T) {
	_, err := dungeonspec.Load([]byte(castleSource(t)))
	require.NoError(t, err)
	_, err = dungeonspec.Load([]byte(outcomesSite()))
	require.NoError(t, err)
}
