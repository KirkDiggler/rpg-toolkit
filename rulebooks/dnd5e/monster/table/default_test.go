// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package table

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/KirkDiggler/rpg-toolkit/core"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/monster/monsters"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/refs"
)

// wantGeneric is the design's table, written out a second time on purpose
// (rpg-project `ideas/creature-table/design.md` §2). The shipped table is
// content an author reads and a walk tunes, so a change to it should be a
// deliberate change to two places, not a character that drifted in one.
const wantGeneric = `time:
  - { when: { fled: { within: 3 } },     away: actor,      weight: 3 }
  - { when: { attacked: { within: 3 } }, attack: attacker, weight: 3 }
  - { when: { enemy: reach },            attack: enemy }
  - { when: { enemy: seen },             toward: enemy }
  - { when: { enemy: remembered },       toward: enemy }
  - { when: { enemy: none },             hold: {} }
`

func TestDefaultIsTheDesignsTable(t *testing.T) {
	got, err := Default(*refs.Monsters.Goblin())

	require.NoError(t, err)
	require.NotNil(t, got)
	require.Equal(t, wantGeneric, got.Source)
}

// dungeonspec's CompileTable takes the CONTENTS of an `on:` mapping, so a
// Source carrying its own `on:` header would nest one level too deep and
// compile to a table with a single trigger nobody named.
func TestDefaultCarriesNoOnHeader(t *testing.T) {
	got, err := Default(*refs.Monsters.Goblin())
	require.NoError(t, err)

	require.NotContains(t, got.Source, "on:", "the header belongs to whoever holds the table, not to the table")

	lines := strings.Split(strings.TrimRight(got.Source, "\n"), "\n")
	require.Equal(t, "time:", lines[0], "the source starts at a trigger key, at column zero")
	for _, line := range lines[1:] {
		require.True(t, strings.HasPrefix(line, "  - "),
			"an entry sits one level under its trigger, not two: %q", line)
	}
}

func TestDefaultAnswersEveryShippedMonsterWithTheSameTable(t *testing.T) {
	known := monsters.Refs()
	require.NotEmpty(t, known, "the registry names the monsters this rulebook ships")

	for _, name := range known {
		ref, err := core.ParseString(name)
		require.NoError(t, err, "%s is a registry key, so it parses", name)

		got, err := Default(*ref)

		require.NoError(t, err, "%s", name)
		require.NotNil(t, got, "%s: Default is never (nil, nil)", name)
		require.Equal(t, wantGeneric, got.Source,
			"%s: one generic table stands behind every kind this slice — a kind-specific one "+
				"arrives when a use case brings it, not before", name)
	}
}

func TestDefaultRefusesARefThatNamesNothing(t *testing.T) {
	tests := []struct {
		name string
		ref  core.Ref
	}{
		{name: "the zero ref", ref: core.Ref{}},
		{name: "no type", ref: core.Ref{Module: "dnd5e", ID: "goblin"}},
		{name: "no id", ref: core.Ref{Module: "dnd5e", Type: "monsters"}},
		{name: "no module", ref: core.Ref{Type: "monsters", ID: "goblin"}},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := Default(tc.ref)

			require.Error(t, err, "asking for the table of nothing is a bug, and a table would hide it")
			require.Nil(t, got)
			require.Contains(t, err.Error(), "no default table")
		})
	}
}

// The order of the entries is the whole argument of the table, so it is what
// this pins: reordering them changes which creature the rulebook ships even
// though every line survives.
func TestDefaultsEntriesRunInTheOrderTheArgumentNeeds(t *testing.T) {
	got, err := Default(*refs.Monsters.Thug())
	require.NoError(t, err)

	lines := entriesOf(t, got.Source)
	require.Len(t, lines, 6)

	require.Contains(t, lines[0], "fled:",
		"a creature that answered flee runs first, on its own time — which is why `flee` lands a deed and steps nowhere")
	require.Contains(t, lines[0], "away: actor")

	require.Contains(t, lines[1], "attacked:",
		"being struck is answered when the creature next has time, not when the blow lands")
	require.Contains(t, lines[1], "attack: attacker")

	require.Contains(t, lines[2], "enemy: reach")
	require.Contains(t, lines[2], "attack: enemy", "what is already in reach gets struck")

	require.Contains(t, lines[3], "enemy: seen")
	require.Contains(t, lines[3], "toward: enemy",
		"an enemy it can only SEE gets closed on — a sighted band that attacked would leave the creature "+
			"swinging at nothing from across the room")

	require.Contains(t, lines[4], "enemy: remembered")
	require.Contains(t, lines[4], "toward: enemy", "and one it only remembers gets walked toward")

	require.Contains(t, lines[5], "enemy: none")
	require.Contains(t, lines[5], "hold: {}", "holding is what a creature does when there is nobody, and it is last")
}

// An unconditional entry is eligible on EVERY roll, so an unconditional `hold`
// competes with the attack and the walk and a monster in reach stands there
// half its turns. A table with no eligible entry is already a hold, so the
// condition costs the creature nothing.
func TestDefaultHasNoUnconditionalEntry(t *testing.T) {
	got, err := Default(*refs.Monsters.Thug())
	require.NoError(t, err)

	for i, line := range entriesOf(t, got.Source) {
		require.Contains(t, line, "when:",
			"entry %d is eligible on every roll and would compete with the words that do something: %q", i, line)
	}
}

// The `enemy:` bands are exclusive and run near-to-far, so a creature never
// reads a farther band while a nearer one holds.
func TestDefaultsEnemyBandsRunNearToFar(t *testing.T) {
	got, err := Default(*refs.Monsters.Goblin())
	require.NoError(t, err)

	var bands []string
	for _, line := range entriesOf(t, got.Source) {
		for _, band := range []string{"reach", "seen", "remembered", "none"} {
			if strings.Contains(line, "enemy: "+band) {
				bands = append(bands, band)
			}
		}
	}

	require.Equal(t, []string{"reach", "seen", "remembered", "none"}, bands,
		"nearest band first, and every band is named — including the one that holds")
}

// entriesOf pulls the entry lines out of a table's source without parsing the
// grammar, which belongs to dungeonspec and not here.
func entriesOf(t *testing.T, source string) []string {
	t.Helper()

	var entries []string
	for _, line := range strings.Split(source, "\n") {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "- ") {
			entries = append(entries, trimmed)
		}
	}

	return entries
}
