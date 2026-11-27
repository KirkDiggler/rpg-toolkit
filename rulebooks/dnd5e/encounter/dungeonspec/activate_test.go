// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package dungeonspec_test

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

// activate_test.go is the dialect's half of `activate` (rpg-project#1131,
// ADR-0051): a table row can name an ability by its word. Modelled on
// dash_test.go. Whether the word is a real ability is the host's content
// check, never this module's: nothing here lists one.

// activatingCaptain is the heirloom tomb's captain with this `time` table.
func activatingCaptain(t *testing.T, rows string) string {
	t.Helper()

	return dashingCaptain(t, rows)
}

const activateShape = "`activate` names one ability by its word, as a stat block's `lets` writes it: `activate: second wind`"

// Test 1: it compiles, trimmed, and its other words are empty.
func TestAnActivateRowCompiles(t *testing.T) {
	rows := captainRows(t, activatingCaptain(t, `{ activate: second wind }`))
	require.Len(t, rows, 1)
	require.Equal(t, "second wind", rows[0].Activate)
	require.Nil(t, rows[0].Toward)
	require.Nil(t, rows[0].Dash)
	require.Nil(t, rows[0].Attack)
	require.Nil(t, rows[0].Away)
	require.Nil(t, rows[0].Search)
	require.False(t, rows[0].Hold)
	require.False(t, rows[0].Flee)
	require.Empty(t, rows[0].Fact)

	trimmed := captainRows(t, activatingCaptain(t, `{ activate: "  second wind  " }`))
	require.Equal(t, "second wind", trimmed[0].Activate)
}

// Test 2: the shape is refused at the row, with the path or the line.
func TestAnActivateThatIsNotOneWordIsRefused(t *testing.T) {
	t.Run("empty", func(t *testing.T) {
		requireDefect(t, defectsIn(t, activatingCaptain(t, `{ activate: "" }`)), ".activate", activateShape, "line")
	})
	t.Run("blank", func(t *testing.T) {
		requireDefect(t, defectsIn(t, activatingCaptain(t, `{ activate: "   " }`)), ".activate", activateShape, "line")
	})
	t.Run("a mapping", func(t *testing.T) {
		requireDefect(t, decodeDefects(t, activatingCaptain(t, `{ activate: { use: second wind } }`)),
			activateShape, "line")
	})
	t.Run("a list", func(t *testing.T) {
		requireDefect(t, decodeDefects(t, activatingCaptain(t, `{ activate: [second wind] }`)),
			activateShape, "line")
	})
}

// Test 3: one thing per row; a row may speak and act.
func TestAnActivateRowDoesOneThing(t *testing.T) {
	requireDefect(t, defectsIn(t, activatingCaptain(t, `{ activate: second wind, attack: enemy }`)),
		"an entry does one thing", "`attack` and `activate`")
	rows := captainRows(t, activatingCaptain(t, `{ say: "Hold the line", activate: second wind }`))
	require.Equal(t, "second wind", rows[0].Activate)
	require.Equal(t, "Hold the line", rows[0].Say)
}

// Test 4: not under an outcome.
func TestAnActivateIsRefusedUnderAnOutcome(t *testing.T) {
	const at = "room.room.monsterBindings.goblin-1.options[0]"
	requireRefusedAt(t, refusals(t, optionsOnGoblin(
		"          - { id: hi, say: \"Hi.\", check: [{ ability: persuasion, dc: 10 }], "+
			"success: [{ activate: second wind }] }\n")),
		at+".success[0].activate", "`activate` is what a creature does with time, and `success` is an outcome")
}

// Test 5: not on a group; a bundle's own row may say it.
func TestAnActivateIsRefusedOnAGroupAndAllowedInsideABundle(t *testing.T) {
	const bundle = `  rally:
    rows:
      - { activate: second wind }
`
	t.Run("on a use row", func(t *testing.T) {
		defects := roomDefects(t, bundledPatrol(t, bundle, "use: rally\nactivate: second wind"))
		require.NotEmpty(t, defects)
		var found bool
		for _, d := range defects {
			found = found || strings.Contains(d, "`activate`")
		}
		require.True(t, found, "%v", defects)
	})
	t.Run("inside a bundle", func(t *testing.T) {
		source := bundledPatrol(t, bundle, "use: rally")
		require.Empty(t, roomDefects(t, source))
		rows := gateTable(t, source)
		require.Len(t, rows, 1)
		require.Len(t, rows[0].Rows, 1)
		require.Equal(t, "second wind", rows[0].Rows[0].Activate)
	})
}

// Test 6: a row that is not a mapping is refused in a sentence that lists the word.
func TestARowThatIsNotAMappingListsActivate(t *testing.T) {
	requireDefect(t, decodeDefects(t, activatingCaptain(t, `[activate]`)),
		"plus exactly one of { fact, flee, hold, attack, toward, dash, activate, away, search }")
}

// Test 8 (the compiled picture of a site that never says the word): a whole
// marshalled Compiled of the castle carries no "Activate" key.
func TestACompiledSiteThatDoesNotActivateHasNoActivateKey(t *testing.T) {
	picture := pictureOf(t, "testdata/world-builder-v4-castle.yaml")
	require.Zero(t, strings.Count(picture, `"Activate"`), "a row that activates nothing leaves no key")
	require.Zero(t, strings.Count(picture, `"activate"`))

	compiled := captainRows(t, activatingCaptain(t, `{ activate: second wind }`))
	raw, err := json.Marshal(compiled)
	require.NoError(t, err)
	require.Contains(t, string(raw), `"Activate":"second wind"`)
}
