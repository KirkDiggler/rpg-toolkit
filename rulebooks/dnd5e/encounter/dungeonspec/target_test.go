// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package dungeonspec_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter/dungeonspec"
)

// target_test.go is the dialect's half of smarter monsters S2
// (rpg-project#669): a row's `target: attacker | closest`, `closest` for an
// `enemy` that names none, refused where it cannot mean anything.

// captainsRow is the first `time` row the captain compiles to.
func captainsRow(t *testing.T, row string) encounter.Answer {
	t.Helper()
	source := searchingCaptain(t, row)
	require.Empty(t, defectsIn(t, source))
	compiled, err := dungeonspec.Load([]byte(source))
	require.NoError(t, err)
	for _, m := range compiled.Monsters {
		if m.ID == "captain" {
			return m.Table[encounter.AnswerTime][0]
		}
	}
	require.Fail(t, "the fixture still places the captain")

	return encounter.Answer{}
}

// TestAnEnemyCompilesWithItsTargetOrClosest: a written policy arrives as
// written on `attack` and `toward`, an omitted one as
// [dungeonspec.DefaultTargetPolicy], and a row that selects no enemy carries
// none.
func TestAnEnemyCompilesWithItsTargetOrClosest(t *testing.T) {
	require.Equal(t, encounter.TargetClosest, dungeonspec.DefaultTargetPolicy)

	require.Equal(t, encounter.TargetAttacker,
		captainsRow(t, `{ when: { enemy: reach }, attack: enemy, target: attacker }`).Attack.Target)
	require.Equal(t, encounter.TargetAttacker,
		captainsRow(t, `{ when: { enemy: seen }, toward: enemy, target: attacker }`).Toward.Target)
	require.Equal(t, encounter.TargetClosest,
		captainsRow(t, `{ when: { enemy: reach }, attack: enemy, target: closest }`).Attack.Target)
	require.Equal(t, encounter.TargetClosest,
		captainsRow(t, `{ when: { enemy: reach }, attack: enemy }`).Attack.Target, "omitted is closest")
	require.Equal(t, encounter.TargetClosest,
		captainsRow(t, `{ when: { enemy: seen }, away: enemy }`).Away.Target, "on every enemy")
	require.Empty(t, captainsRow(t, `{ when: { enemy: none }, toward: { at: [20, 5] } }`).Toward.Target,
		"a cell is no enemy")
}

// TestATargetIsRefusedWhereItCannotMeanAnything: a policy that is not one of
// the two, `attacker` on a run, a policy on a cell, and one on a row that
// selects nobody.
func TestATargetIsRefusedWhereItCannotMeanAnything(t *testing.T) {
	for _, tc := range []struct {
		name string
		row  string
		path string
		says []string
	}{
		{
			name: "a policy that is not one of the two",
			row:  `{ when: { enemy: reach }, attack: enemy, target: weakest }`,
			path: "place[18].on.time[0]",
			says: []string{"`attack: enemy` picks by `target` \"weakest\"", "`closest` or by `attacker`"},
		},
		{
			name: "attacker on a run",
			row:  `{ when: { enemy: seen }, away: enemy, target: attacker }`,
			path: "place[18].on.time[0]",
			says: []string{"`away: enemy` picks by `target` attacker"},
		},
		{
			name: "a policy on a cell",
			row:  `{ when: { enemy: none }, toward: { at: [20, 5] }, target: attacker }`,
			path: "place[18].on.time[0]",
			says: []string{"only `enemy` takes one"},
		},
		{
			name: "a policy on a row that selects nobody",
			row:  `{ hold: {}, target: attacker }`,
			path: "place[18].on.time[0].target",
			says: []string{"this row selects nobody"},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			requireDefect(t, defectsIn(t, searchingCaptain(t, tc.row)), append([]string{tc.path}, tc.says...)...)
		})
	}
}
