// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package dungeonspec_test

// intimidate_test.go is the first shenanigan's authoring surface
// (rpg-project#454, ideas/shenanigans/intimidate.md): what the author writes
// on a monster placement to price a threat and to say what the camp learns
// when one lands, and what the compiler hands the host.

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter/dungeonspec"
)

// The camp's sergeant, in the design's own words: a DC the author priced and
// a fact the camp learns when somebody beats it.
const cowedChief = `  - { id: chief,  ref: "dnd5e:monsters:skeleton-captain", at: [12,4], faction: raiders, ` +
	`intimidate: [{ ability: intimidation, dc: 12 }, { ability: str, dc: 15 }], ` +
	`on: { intimidated: { fact: sergeant-cowed } } }`

// TestAnAuthoredThreatReachesTheHost: both keys compile onto the placement
// the host spawns from, approaches verbatim and in the authored order, the
// `on:` map still keyed by the verb that landed.
func TestAnAuthoredThreatReachesTheHost(t *testing.T) {
	compiled, err := dungeonspec.Load([]byte(edited(t, chiefLine, cowedChief)))
	require.NoError(t, err)

	chief := compiled.Monsters[0]
	require.Equal(t, "chief", chief.ID, "the fixture's first monster is the one edited")
	require.Equal(t, []encounter.CheckApproach{
		{Ability: "intimidation", DC: 12},
		{Ability: "str", DC: 15},
	}, chief.Intimidate, "every route the author priced, in the order they wrote them")
	require.Equal(t, map[string]string{dungeonspec.OnIntimidated: "sergeant-cowed"}, chief.On)
}

// TestAMonsterNobodyPricedCarriesNothing: absent is the common case and it
// compiles to nil, not to an empty list — the rulebook derives the DC from
// the stat block's own passive Insight, and nil is how it knows to.
func TestAMonsterNobodyPricedCarriesNothing(t *testing.T) {
	compiled, err := dungeonspec.Load([]byte(campSource(t)))
	require.NoError(t, err)

	for _, m := range compiled.Monsters {
		require.Nil(t, m.Intimidate, "%s prices no threat", m.ID)
		require.Nil(t, m.On, "%s teaches the world nothing", m.ID)
	}
}

// TestAFactNothingElseMentionsIsAllowed: the same R8 rule `arrives: { fact }`
// keeps — the dungeon shows the cost rather than refusing a fact no record
// reveals and no disposition waits for.
func TestAFactNothingElseMentionsIsAllowed(t *testing.T) {
	compiled, err := dungeonspec.Load([]byte(edited(t, chiefLine,
		strings.Replace(cowedChief, "sergeant-cowed", "nobody-waits-for-this", 1))))
	require.NoError(t, err)
	require.Equal(t,
		map[string]string{dungeonspec.OnIntimidated: "nobody-waits-for-this"},
		compiled.Monsters[0].On)
}

// TestTheNewKeysAreKnownToTheDecoder: a yaml tag alone is not enough —
// PlaceSpec's hand-written key list is what decides, and a typo'd sibling
// must still be refused by name.
func TestTheNewKeysAreKnownToTheDecoder(t *testing.T) {
	t.Run("both keys decode", func(t *testing.T) {
		_, err := dungeonspec.Load([]byte(edited(t, chiefLine, cowedChief)))
		require.NoError(t, err)
	})
	t.Run("a typo is still refused by name", func(t *testing.T) {
		_, err := dungeonspec.Load([]byte(edited(t, chiefLine,
			strings.Replace(cowedChief, "intimidate:", "intimidation:", 1))))
		require.Error(t, err)
		require.Contains(t, err.Error(), "field intimidation not found")
	})
}
