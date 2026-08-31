// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package resolution

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/character"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/monster"
	"github.com/KirkDiggler/rpg-toolkit/world/graph"
	"github.com/KirkDiggler/rpg-toolkit/world/journal"
)

// The cast this suite reads castView's answers against: two characters, two
// monsters, and one id ("stranger") deliberately never attached to anything.
const (
	heroA     = "hero-a"
	heroB     = "hero-b"
	monsterA  = "monster-a"
	monsterB  = "monster-b"
	strangeID = "stranger"
)

// relationsCast is the fixture every test in this file reads castView's
// answers against. Nil sheets are fine — side() and Character/Monster only
// ever ask whether an id is present in the map, never what is behind it.
func relationsCast() *Participants {
	return &Participants{
		characters: map[string]*character.Character{
			heroA: nil,
			heroB: nil,
		},
		monsters: map[string]*monster.Monster{
			monsterA: nil,
			monsterB: nil,
		},
		order: []string{heroA, heroB, monsterA, monsterB},
	}
}

// TestCastViewReadsTheRelationTableFromTheCast pins v1's truth table exactly
// as cast.go's own code computes it today — read from the source, not copied
// from an existing assertion, because no existing test in this package
// exercised IsHostile or IsAllied at all before this change.
//
// Two characters are allied and not hostile; two monsters are allied and not
// hostile; a character and a monster are hostile and not allied. Either id
// missing from the cast is the "cannot answer" case (side()'s own doc)
// regardless of the other id's kind — known is false and the boolean answer
// is unreadable alongside it, so both are asserted false rather than left
// unchecked.
func TestCastViewReadsTheRelationTableFromTheCast(t *testing.T) {
	v := &castView{cast: relationsCast()}

	cases := []struct {
		name         string
		a, b         string
		wantHostile  bool
		wantAllied   bool
		wantKnownYes bool
	}{
		{"two characters are allied, not hostile", heroA, heroB, false, true, true},
		{"a character is hostile to a monster, not allied", heroA, monsterA, true, false, true},
		{"a monster is hostile to a character, not allied", monsterA, heroA, true, false, true},
		{"two monsters are allied, not hostile", monsterA, monsterB, false, true, true},
		{"a participant is hostile to themself: no — allied, not hostile", heroA, heroA, false, true, true},
		{"a stranger as a: cannot answer", strangeID, heroA, false, false, false},
		{"a stranger as b: cannot answer", heroA, strangeID, false, false, false},
		{"two strangers: cannot answer", strangeID, strangeID, false, false, false},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			hostile, hostileKnown := v.IsHostile(tc.a, tc.b)
			allied, alliedKnown := v.IsAllied(tc.a, tc.b)

			require.Equal(t, tc.wantKnownYes, hostileKnown, "IsHostile known")
			require.Equal(t, tc.wantKnownYes, alliedKnown, "IsAllied known")

			if !tc.wantKnownYes {
				// Today's table happens to leave both false on "cannot
				// answer" — asserted, but the known flag above is the one a
				// caller is required to check first.
				require.False(t, hostile)
				require.False(t, allied)

				return
			}

			require.Equal(t, tc.wantHostile, hostile, "IsHostile answer")
			require.Equal(t, tc.wantAllied, allied, "IsAllied answer")
		})
	}
}

// TestCastRelationsBuilds pins that the fixed relation table cast.go declares
// is itself valid — the panic in castRelations documents this test as the
// reason a graph.New failure there would mean a bug in that literal, not bad
// caller input. If this test ever fails, the panic message it is pointing at
// is about to start firing in production.
func TestCastRelationsBuilds(t *testing.T) {
	table := castRelations()
	require.NotNil(t, table)

	require.True(t, table.HasEdge(characterSide, hostileTo, monsterSide))
	require.True(t, table.HasEdge(monsterSide, hostileTo, characterSide))
	require.True(t, table.HasEdge(characterSide, alliedWith, characterSide))
	require.True(t, table.HasEdge(monsterSide, alliedWith, monsterSide))

	require.False(t, table.HasEdge(characterSide, alliedWith, monsterSide))
	require.False(t, table.HasEdge(monsterSide, alliedWith, characterSide))
}

// TestARelationTableCanExpressANeutralSideWithoutTouchingAnyRule is the shape
// claim, proven on world/graph directly rather than on castView — this does
// NOT assert anything about production behaviour. castView's table still has
// exactly two sides today; rung 2 is what would add a third to it, and that
// is a design decision for its own round, not this test's.
//
// What this test pins is narrower and already true: the MECHANISM castView
// now reads does not forbid a third side the way v1's old two-valued guess
// did. sideA != sideB is a total function over a boolean — there is no third
// value for it to return. A graph has no such ceiling: two factions can be
// declared hostile to a shared third without saying anything about each
// other, and the fold answers "neither" for that pair honestly, because
// neither edge was ever declared. That is the room the old shape had nowhere
// to put.
func TestARelationTableCanExpressANeutralSideWithoutTouchingAnyRule(t *testing.T) {
	const (
		party    journal.EntityID = "party"
		bandits  journal.EntityID = "bandits"
		cultists journal.EntityID = "cultists"

		belongsTo  graph.Relation = "belongs-to"
		hostileRel graph.Relation = "hostile-to"
		alliedRel  graph.Relation = "allied-with"
	)

	w, err := graph.New(graph.Config{
		Membership: belongsTo,
		Entities: []graph.Entity{
			{ID: party, Kind: "side"},
			{ID: bandits, Kind: "side"},
			{ID: cultists, Kind: "side"},
		},
		Edges: []graph.Edge{
			// Both factions hate the party. Neither statement says anything
			// about the other faction — that is the whole point.
			{From: party, Rel: hostileRel, To: bandits},
			{From: bandits, Rel: hostileRel, To: party},
			{From: party, Rel: hostileRel, To: cultists},
			{From: cultists, Rel: hostileRel, To: party},
			{From: party, Rel: alliedRel, To: party},
			{From: bandits, Rel: alliedRel, To: bandits},
			{From: cultists, Rel: alliedRel, To: cultists},
		},
	})
	require.NoError(t, err)

	state := w.Truth(journal.New())

	require.True(t, state.HasEdge(party, hostileRel, bandits))
	require.True(t, state.HasEdge(party, hostileRel, cultists))

	require.False(t, state.HasEdge(bandits, hostileRel, cultists), "neither declared, so neither holds")
	require.False(t, state.HasEdge(cultists, hostileRel, bandits))
	require.False(t, state.HasEdge(bandits, alliedRel, cultists), "not hostile is not the same as allied")
	require.False(t, state.HasEdge(cultists, alliedRel, bandits))
}
