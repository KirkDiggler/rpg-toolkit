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
  - { when: { fled: { within: 3 } },      away: actor,     weight: 3 }
  - { when: { attacked: { within: 3 } },  attack: attacker, weight: 3 }
  - { when: { enemy: seen },              attack: enemy }
  - { when: { enemy: remembered },        toward: enemy }
  - { hold: {} }
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
	require.Len(t, lines, 5)

	require.Contains(t, lines[0], "fled:",
		"a creature that answered flee runs first, on its own time — which is why `flee` lands a deed and steps nowhere")
	require.Contains(t, lines[0], "away: actor")

	require.Contains(t, lines[1], "attacked:",
		"being struck is answered when the creature next has time, not when the blow lands")
	require.Contains(t, lines[1], "attack: attacker")

	require.Contains(t, lines[2], "attack: enemy", "failing both, it fights what it can see")
	require.Contains(t, lines[3], "toward: enemy", "and walks toward what it only remembers")

	require.Equal(t, "- { hold: {} }", lines[4],
		"hold is last and unconditional, so a table always has an answer and an idle creature is idle on purpose")
	for i, line := range lines[:4] {
		require.Contains(t, line, "when:", "entry %d is conditional — only the fallback is not", i)
	}
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
