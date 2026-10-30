// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package castletest

// castle_king_theft_test.go is A FAILED THEFT CLOSES THAT ROOM'S CHANCES, NOT
// THE KEEP'S (rpg-project#719, Frank's list S5): the hero is caught lifting a
// pie, the cook's theft and gossip are gone for the hero, and the pedlar
// outside, who saw nothing, still talks. The locality is the fact's audience
// and the withholding is `requires: { not: { fact: caught-in-kitchen } }`.

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter"
)

// TestMadKingACaughtThiefLosesTheCooksLinesAndNotThePedlars is the slice's
// rule in one run: caught in the kitchen, told so, the cook's lines withheld,
// and the pedlar outside still offering his line. The walk
// to the pedlar is the theft-walk site's (rpg-project#719's Try-it).
func TestMadKingACaughtThiefLosesTheCooksLinesAndNotThePedlars(t *testing.T) {
	t.Parallel()
	s := openTheCastle(t, besideTheCook)
	require.True(t, s.offers(kingCook, "pilfer"), "precondition: nobody has been caught")
	require.True(t, s.offers(kingCook, "gossip"), "precondition: the cook talks to strangers")

	s.said(kingCook, "pilfer", "sleight-of-hand", false)

	var caught bool
	for _, b := range s.beatsOf(hero, encounter.BeatAnswered) {
		caught = caught || (b.Creature == string(kingCook) && b.Fact == "caught-in-kitchen" && b.Say == "Thief!")
	}
	require.True(t, caught, "the hero is told the cook caught them, and learns it")
	require.False(t, s.offers(kingCook, "pilfer"), "no second try at the sill")
	require.False(t, s.offers(kingCook, "gossip"), "the cook's gossip is closed to the thief")

	require.True(t, s.offers(kingPedlar, "tonight"), "the pedlar outside saw nothing, and still talks to the thief")
}
