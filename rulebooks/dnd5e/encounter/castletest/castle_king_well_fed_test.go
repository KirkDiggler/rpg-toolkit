// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package castletest

// castle_king_well_fed_test.go is THE COOK'S PIES (rpg-project#1024): a line's
// answer gives its speaker a status an author wrote. The hero lends the cook a
// hand; a win leaves the hero well fed, returned on the speak for the session
// to give, and a miss gives nothing. Both start from one stored moment, so
// the grant is proved to survive a save and a load.

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestMadKingHelpingTheCookLeavesTheSpeakerWellFed(t *testing.T) {
	t.Parallel()
	s := openTheCastle(t, besideTheCook)
	require.True(t, s.offers(kingCook, "help"), "the cook takes a hand with the pies")
	blob, err := json.Marshal(s.enc.ToData())
	require.NoError(t, err)

	won := loadTheSceneOn(t, blob, s.compiled, s.host)
	out, err := won.speak(kingCook, "help", "athletics", true)
	require.NoError(t, err)
	require.NotEmpty(t, out.Statuses, "a win gives a status")
	for _, given := range out.Statuses {
		require.Equal(t, hero, given.Member, "the speaker is given it")
		require.Equal(t, "well-fed", given.Grant.ID)
		require.JSONEq(t, `{"name":"Well fed","changes":[{"advantage":"checks","when":{"ability":"charisma"}}],`+
			`"maintenance":[{"ends_on":"run"}],"again":"ignored"}`, string(given.Grant.Status))
	}
	require.Contains(t, won.saidBy(hero, kingCook), "Bless you. Here, eat.")

	lost := loadTheSceneOn(t, blob, s.compiled, s.host)
	out, err = lost.speak(kingCook, "help", "athletics", false)
	require.NoError(t, err)
	require.Empty(t, out.Statuses, "a miss gives nothing")
	require.Contains(t, lost.saidBy(hero, kingCook), "Mind the pastry! Out from under my feet.")
}
