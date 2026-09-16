// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package session

import (
	"testing"

	"github.com/stretchr/testify/require"

	coreResources "github.com/KirkDiggler/rpg-toolkit/core/resources"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/character"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/classes"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/resources"
)

// TestAPoolNobodyNamedShipsUnderItsOwnKey holds the fallback in levelGained to
// the argument its comment makes: *"a pool the rulebook's display table has
// never heard of still ships, named by its key. Dropping it would be the worse
// answer."*
//
// Nothing held it to that. A reviewer changed the fallback to return "" and the
// suite stayed green, because the only test that reads Resources uses two keys
// the display table knows. An empty Name is the failure the comment argues
// against and it was indistinguishable from the fallback working.
//
// An INTERNAL test on purpose. The pools a level moves are the rulebook's to
// decide, so no fixture reachable through the verbs can present a key the
// display table does not carry — which is exactly why the branch was untested.
// Calling the projection directly is the only place the claim can be observed
// at all, the same reason compelled_internal_test.go gives for its own.
func TestAPoolNobodyNamedShipsUnderItsOwnKey(t *testing.T) {
	const unnamed coreResources.ResourceKey = "eldritch_charges"
	require.Empty(t, displayNameOf(unnamed), "the fixture must name a key the table really does not carry")

	gained := levelGained(classes.Fighter, character.GainedAtLevel{
		CharacterLevel: 2, ClassLevel: 2,
		Resources: []character.ResourceChange{
			{Key: resources.HitDice, From: 1, To: 2},
			{Key: unnamed, From: 0, To: 3},
		},
	})

	require.Len(t, gained.Resources, 2, "an unnamed pool is reported, not dropped")
	require.Equal(t, "Hit Dice", gained.Resources[0].Name, "a key the table knows keeps its display name")

	require.Equal(t, string(unnamed), gained.Resources[1].Key)
	require.Equal(t, string(unnamed), gained.Resources[1].Name,
		"and one it does not is named by its key rather than by nothing")
	require.Equal(t, 3, gained.Resources[1].To, "with the change it actually made")
}

// displayNameOf is the rulebook's answer with the found flag collapsed, so the
// test above can state its own precondition instead of assuming it.
func displayNameOf(key coreResources.ResourceKey) string {
	if name, ok := resources.DisplayName(key); ok {
		return name
	}
	return ""
}
