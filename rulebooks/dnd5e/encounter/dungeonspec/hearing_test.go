// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package dungeonspec_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/KirkDiggler/rpg-toolkit/mind/behavior"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter/dungeonspec"
	"github.com/KirkDiggler/rpg-toolkit/tools/spatial"
)

// hearing_test.go is the castle's table words on the author's form
// (rpg-project#569): `heard`, `fact` and `not` as conditions, `when` as a
// list, `fact` on a `time` row, `toward: { path }`, and the `sound` word —
// each one parsed into what the engine rolls, and each mistake refused in a
// sentence the form can show.

// captainWith is the heirloom tomb with the captain's line carrying one more
// thing the scene is about.
func captainWith(t *testing.T, rest string) string {
	t.Helper()

	return withPlacement(t, `  - { id: captain, ref: "dnd5e:monsters:skeleton-captain", at: [23,5], targeting: closest,
      holds: [vault-map], `+rest+` }`)
}

// TestTheCastleWordsCompile is the gate patrol and the keep patrol from the
// use case's own file, on one placement: every new word, compiled.
func TestTheCastleWordsCompile(t *testing.T) {
	source := captainWith(t, `on: { time: [
        { when: { enemy: none }, toward: { path: [[19, 3], [25, 3], [25, 7], [19, 7]] } },
        { when: { heard: { kind: clang, within: 2 } }, toward: sound },
        { when: { heard: { kind: boom, within: 3 } }, away: sound },
        { when: { fact: alarm-raised }, toward: { at: [23, 3] } },
        { when: { enemy: seen }, say: "To arms! Intruders in the yard!", fact: alarm-raised },
        { when: [{ fact: guards-turned }, { not: { enemy: seen } }], toward: { at: [23, 3] } },
        { hold: {} },
      ] }`)

	require.Empty(t, defectsIn(t, source), "every word the castle needs, on one placement")

	compiled, err := dungeonspec.Load([]byte(source))
	require.NoError(t, err)
	var rows []encounter.Answer
	for _, m := range compiled.Monsters {
		if m.ID == "captain" {
			rows = m.Table[encounter.AnswerTime]
		}
	}
	require.Len(t, rows, 7)

	require.Len(t, rows[0].When, 1, "a single mapping reads as a list of one")
	// ODD ROWS, so the conversion is pinned: on a pointy-top map an odd row's
	// authored column is not its axial q, and a raw pair would walk the
	// creature to a different cell than the one the author drew.
	o := encounter.HexesArePointyTop()
	require.Equal(t, []spatial.Position{
		encounter.HexCellAt(o, 19, 3), encounter.HexCellAt(o, 25, 3),
		encounter.HexCellAt(o, 25, 7), encounter.HexCellAt(o, 19, 7),
	}, rows[0].Toward.Path, "the route reached the host as the cells the author drew")
	require.NotEqual(t, spatial.Position{X: 19, Y: 3}, rows[0].Toward.Path[0],
		"precondition: the raw pair and the cell differ on this row")
	at := encounter.HexCellAt(o, 23, 3)
	require.Equal(t, &at, rows[3].Toward.At, "and an `at` is converted the same way")

	require.Equal(t, &encounter.HeardWhen{Kind: behavior.Name{Literal: encounter.SoundClang}, Within: 2}, rows[1].When[0].Heard)
	require.Equal(t, encounter.SelectorSound, rows[1].Toward.Word)
	require.Equal(t, encounter.SelectorSound, rows[2].Away.Word)

	require.Equal(t, behavior.Name{Literal: "alarm-raised"}, rows[3].When[0].Fact)

	require.Equal(t, encounter.FactID("alarm-raised"), rows[4].Fact, "`fact` is a time word now")
	require.Equal(t, "To arms! Intruders in the yard!", rows[4].Say)

	require.Len(t, rows[5].When, 2, "the list form: both must hold")
	require.Equal(t, behavior.Name{Literal: "guards-turned"}, rows[5].When[0].Fact)
	require.NotNil(t, rows[5].When[1].Not)
	require.Equal(t, encounter.EnemySeen, rows[5].When[1].Not.Enemy)
}

// TestTheHearingWordsAreRefusedOnTheForm is every refusal this slice adds to
// the grammar, each at its own path, in its own sentence. The selector shape
// refusals are mind/behavior's own sentences, asked of the compiled row and
// reported at the entry.
func TestTheHearingWordsAreRefusedOnTheForm(t *testing.T) {
	for _, tc := range []struct {
		name string
		on   string
		path string
		says []string
	}{
		{
			name: "a route on `attack`",
			on:   `on: { time: [ { attack: { path: [[23, 3], [23, 5]] } } ] }`,
			path: "place[18].on.time[0]: ",
			says: []string{"`attack` cannot take a `path`: a route is walked, and only `toward` and `dash` walk one"},
		},
		{
			name: "a route on `away`",
			on:   `on: { time: [ { away: { path: [[23, 3], [23, 5]] } } ] }`,
			path: "place[18].on.time[0]: ",
			says: []string{"`away` cannot take a `path`"},
		},
		{
			name: "a waypoint that is not floor",
			on:   `on: { time: [ { toward: { path: [[23, 3], [99, 99]] } } ] }`,
			path: "place[18].on.time[0].toward.path[1]",
			says: []string{"this walks to [99,99], which is not floor"},
		},
		{
			name: "`sound` on `attack`",
			on:   `on: { time: [ { attack: sound } ] }`,
			path: "place[18].on.time[0]: ",
			says: []string{"`attack` cannot take `sound`: a sound cannot be struck, only walked toward or away from"},
		},
		{
			name: "`heard` without a kind",
			on:   `on: { time: [ { when: { heard: { within: 2 } }, hold: {} } ] }`,
			path: "place[18].on.time[0]: ",
			says: []string{"`heard` needs a `kind`, the sound it listens for"},
		},
		{
			name: "`heard` with no span",
			on:   `on: { time: [ { when: { heard: { kind: clang } }, hold: {} } ] }`,
			path: "place[18].on.time[0]: ",
			says: []string{"`heard` within 0: a span is counted from 1"},
		},
		{
			name: "a cell and a route at once",
			on:   `on: { time: [ { toward: { at: [23, 3], path: [[19, 1], [25, 1], [25, 7]] } } ] }`,
			path: "place[18].on.time[0]: ",
			says: []string{"`toward` names a cell (`at`) and a route (`path`); a target is exactly one of somebody, a cell or a route"},
		},
		{
			name: "an empty route",
			on:   `on: { time: [ { toward: { path: [] } } ] }`,
			path: "place[18].on.time[0]: ",
			says: []string{"`toward` names no target"},
		},
		{
			// R7 as ruled on rpg-toolkit#2044's review: A to B and back is the
			// same cells twice. Named in the author's own pairs.
			name: "a route that retraces itself",
			on:   `on: { time: [ { toward: { path: [[19, 1], [25, 1]] } } ] }`,
			path: "place[18].on.time[0].toward.path",
			says: []string{"this route retraces itself at [24,1]: a patrol walks a loop — add a waypoint so no leg crosses another"},
		},
		{
			// A leg is the line the creature walks, so a line across void is a
			// patrol that stops dead at the edge — refused while it is still a
			// drawing.
			name: "a leg that leaves the floor",
			on:   `on: { time: [ { toward: { path: [[27, 0], [29, 2], [29, 4], [27, 7]] } } ] }`,
			path: "place[18].on.time[0].toward.path",
			says: []string{"leg 1 of this route walks through [28,1], which is not floor"},
		},
		{
			// R10 as the behavior review refined it: the sound a row walks to
			// is the one it listened for, and a `heard` inside a `not` is not
			// listening.
			name: "`sound` on a row that listens for nothing",
			on:   `on: { time: [ { when: { not: { heard: { kind: clang, within: 2 } } }, toward: sound } ] }`,
			path: "place[18].on.time[0]: ",
			says: []string{"`sound` needs a `heard` condition on the row: the sound it walks to is the one the row listened for"},
		},
		{
			name: "`fact` on an `attack` row",
			on:   `on: { time: [ { attack: enemy, fact: alarm-raised } ] }`,
			path: "place[18].on.time[0]",
			says: []string{"an entry does one thing: `fact` and `attack` in the same entry is 2"},
		},
		{
			name: "`actor` whose only deed is negated",
			on:   `on: { time: [ { when: { not: { fled: { within: 2 } } }, away: actor } ] }`,
			path: "place[18].on.time[0].away",
			says: []string{"this entry names no deed"},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			requireDefect(t, defectsIn(t, captainWith(t, tc.on)), append([]string{tc.path}, tc.says...)...)
		})
	}
}

// TestTheHearingWordsAreRefusedAtDecode is every mistake the decoder catches
// before validation runs: a shape the grammar cannot read at all.
func TestTheHearingWordsAreRefusedAtDecode(t *testing.T) {
	for _, tc := range []struct {
		name string
		on   string
		says string
	}{
		{
			name: "`heard` for a sound nothing makes",
			on:   `on: { time: [ { when: { heard: { kind: growl, within: 2 } }, hold: {} } ] }`,
			says: "`heard: { kind: growl }` is not a sound this build makes: they are boom, crash, clang, voice",
		},
		{
			name: "`fact` in a `when` naming nothing",
			on:   `on: { time: [ { when: { fact: "" }, hold: {} } ] }`,
			says: "`fact` in a `when` names the fact the creature must hold, and this names none",
		},
		{
			name: "two conditions in one entry",
			on:   `on: { time: [ { when: { enemy: seen, fact: alarm-raised }, hold: {} } ] }`,
			says: "a `when` entry is one condition, and this names 2: give each its own entry in a list, " +
				"`when: [{ … }, { … }]`, and every entry must hold",
		},
		{
			name: "an empty list",
			on:   `on: { time: [ { when: [], hold: {} } ] }`,
			says: "a `when` with no condition in it is on the table always — omit it",
		},
		{
			name: "`patrol`",
			on:   `on: { time: [ { patrol: {} } ] }`,
			says: "`patrol` is not a word: a patrol is a route, write toward: { path: [[col, row], …] }",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, err := dungeonspec.Decode([]byte(captainWith(t, tc.on)))
			require.Error(t, err)
			require.Contains(t, err.Error(), tc.says)
		})
	}
}
