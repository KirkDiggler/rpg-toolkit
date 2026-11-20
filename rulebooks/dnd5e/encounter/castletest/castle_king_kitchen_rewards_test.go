// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package castletest

// castle_king_kitchen_rewards_test.go is THE KEEP'S TWO KITCHEN REWARDS
// (rpg-project#1024): a line's answer gives its speaker a status an author
// wrote. The hero lends the cook a hand, and a win leaves them warmed up; the
// hero tastes the feast, a plain line, and is feasted, with its once-when-given
// heal carried in the author's words. Each grant is returned on the speak for
// the session to give, and each starts from one stored moment, so the grant is
// proved to survive a save and a load.

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter"
)

func TestMadKingHelpingTheCookLeavesTheSpeakerWarmedUp(t *testing.T) {
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
		require.Equal(t, "warmed-up", given.Grant.ID)
		require.JSONEq(t, `{"name":"Warmed up","changes":[{"initiative":1}],`+
			`"maintenance":[{"ends_on":"run"}],"again":"ignored"}`, string(given.Grant.Status))
	}
	require.Contains(t, won.saidBy(hero, kingCook), "Bless you. Here, eat.")

	lost := loadTheSceneOn(t, blob, s.compiled, s.host)
	out, err = lost.speak(kingCook, "help", "athletics", false)
	require.NoError(t, err)
	require.Empty(t, out.Statuses, "a miss gives nothing")
	require.Contains(t, lost.saidBy(hero, kingCook), "Mind the pastry! Out from under my feet.")
}

func TestMadKingTastingTheFeastLeavesTheSpeakerFeasted(t *testing.T) {
	t.Parallel()
	s := openTheCastle(t, besideTheCook)
	require.True(t, s.offers(kingCook, "taste"), "the cook lets the hero taste the feast")
	blob, err := json.Marshal(s.enc.ToData())
	require.NoError(t, err)

	tasted := loadTheSceneOn(t, blob, s.compiled, s.host)
	out, err := tasted.enc.Speak(context.Background(), &encounter.SpeakInput{
		Range: speakReach, Actor: hero, Target: kingCook, OptionID: "taste", Roller: rollsOne{},
		Revision: tasted.revisionOf(hero, kingCook, "taste"),
	})
	require.NoError(t, err, "a plain line: no check")
	require.NotEmpty(t, out.Statuses, "tasting gives a status")
	for _, given := range out.Statuses {
		require.Equal(t, hero, given.Member, "the speaker is given it")
		require.Equal(t, "feasted", given.Grant.ID)
		require.JSONEq(t, `{"name":"Feasted","changes":[{"max_hit_points":2}],"when_given":[{"heal":2}],`+
			`"maintenance":[{"ends_on":"run"}],"again":"ignored"}`, string(given.Grant.Status),
			"when_given is carried through, unread")
	}
	require.Contains(t, tasted.saidBy(hero, kingCook), "Go on, then. One plate, mind.")
}
