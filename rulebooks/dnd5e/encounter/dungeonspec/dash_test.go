// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package dungeonspec_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter/dungeonspec"
)

// dash_test.go is the dialect's half of the dash (rpg-project#671, R17, R18):
// `dash:` takes `toward`'s selectors, `enemy:` takes the three move words, and
// `threat:` is a condition of its own, refused in this dialect's voice when it
// names a word this build does not read.

// dashingCaptain is the heirloom tomb's captain with this `time` table.
func dashingCaptain(t *testing.T, time string) string {
	t.Helper()

	return withPlacement(t, `  - { id: captain, ref: "dnd5e:monsters:skeleton-captain", at: [23,5], targeting: closest,
      holds: [vault-map], on: { time: [ `+time+` ] } }`)
}

// captainRows compiles a source and returns the captain's `time` rows.
func captainRows(t *testing.T, source string) []encounter.Answer {
	t.Helper()
	require.Empty(t, defectsIn(t, source))
	compiled, err := dungeonspec.Load([]byte(source))
	require.NoError(t, err)
	for _, m := range compiled.Monsters {
		if m.ID == "captain" {
			return m.Table[encounter.AnswerTime]
		}
	}
	require.Fail(t, "the fixture still places the captain")

	return nil
}

// TestTheCarefulDasherCompiles: the design's careful dasher, a list of two
// conditions and a run, arrives as the composition's own row.
func TestTheCarefulDasherCompiles(t *testing.T) {
	rows := captainRows(t, dashingCaptain(t, `{ when: [{ enemy: dash }, { threat: none }], dash: enemy }`))
	require.Len(t, rows, 1)
	require.NotNil(t, rows[0].Dash)
	require.Equal(t, encounter.SelectorEnemy, rows[0].Dash.Word)
	require.Nil(t, rows[0].Toward, "a dash is its own word, not a walk")
	require.Equal(t, []encounter.When{{Enemy: encounter.EnemyDash}, {Threat: encounter.ThreatNone}}, rows[0].When)
}

// TestADashTakesTowardsSelectors: a cell and a route are places a run goes
// to, as they are for a walk; the move words and every threat word read.
func TestADashTakesTowardsSelectors(t *testing.T) {
	for _, row := range []string{
		`{ dash: attacker, when: { attacked: { within: 2 } } }`,
		`{ dash: { at: [20, 5] } }`,
		`{ dash: { path: [[19, 3], [25, 3], [25, 7], [19, 7]] } }`,
		`{ when: { heard: { kind: clang, within: 2 } }, dash: sound }`,
		`{ when: { enemy: walk }, toward: enemy }`,
		`{ when: { enemy: far }, hold: {} }`,
		`{ when: { threat: walk }, hold: {} }`,
		`{ when: { not: { threat: dash } }, dash: enemy }`,
	} {
		t.Run(row, func(t *testing.T) {
			require.NotEmpty(t, captainRows(t, dashingCaptain(t, row)))
		})
	}
}

// TestADashIsRefusedWhereItCannotMeanAnything: a threat word this build does
// not read, a dash beside a walk, a run under a verdict, and a run with no
// heard sound to run at.
func TestADashIsRefusedWhereItCannotMeanAnything(t *testing.T) {
	t.Run("a threat that is not a word", func(t *testing.T) {
		requireDefect(t, decodeDefects(t, dashingCaptain(t, `{ when: { threat: maybe }, dash: enemy }`)),
			"`threat: maybe` is not a threat this build reads: they are none, walk, dash")
	})
	for _, tc := range []struct {
		name string
		row  string
		says []string
	}{
		{
			name: "a dash beside a walk",
			row:  `{ dash: enemy, toward: enemy }`,
			says: []string{"an entry does one thing", "`toward` and `dash`"},
		},
		{
			name: "a run with no sound held",
			row:  `{ dash: sound }`,
			says: []string{"sound"},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			requireDefect(t, defectsIn(t, dashingCaptain(t, tc.row)), tc.says...)
		})
	}
}
