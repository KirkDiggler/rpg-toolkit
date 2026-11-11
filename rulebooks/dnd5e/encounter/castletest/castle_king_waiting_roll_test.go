// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package castletest

// castle_king_waiting_roll_test.go is RESERVE BEFORE DICE (rpg-project#924)
// on the castle's own gate: while the hero's guest claim waits on the hero's
// roll, the bard meets both of the gate's claims pending on the hero, while
// the captain's lines that read no interaction stay open to be spoken; the settle lands the hero's verdict once; and a menu read
// before it is refused.

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter"
)

// captainOffers is the captain's offer to a member, by line id.
func (s *kingScene) captainOffers(member encounter.MemberID) map[string]encounter.OfferedOption {
	s.t.Helper()
	offered, err := s.enc.OptionsFor(member, kingCaptain)
	require.NoError(s.t, err)
	out := map[string]encounter.OfferedOption{}
	for _, o := range offered {
		out[o.ID] = o
	}

	return out
}

func TestMadKingTheGateWaitsOnTheHerosRoll(t *testing.T) {
	t.Parallel()
	s := toTheCaptainWithTheBard(t)
	bardsMenu := s.captainOffers(bard)
	at := s.revisionOf(hero, kingCaptain, "guest")

	reserved, err := s.enc.Reserve(&encounter.ReserveInput{Actor: hero, Target: kingCaptain, OptionID: "guest", Revision: at})
	require.NoError(t, err)
	require.Equal(t, frontGate, reserved.Interaction)

	offer := s.captainOffers(bard)
	require.Equal(t, hero, offer["guest"].PendingOn, "the bard sees the guest claim waiting on the hero")
	require.Equal(t, hero, offer["troupe"].PendingOn, "and the troupe claim too: both read the gate")
	require.False(t, offer["greet"].Pending(), "greeting the captain reads no interaction")
	require.False(t, offer["stand-aside"].Pending(), "nor does threatening him")

	_, err = s.enc.Speak(t.Context(), &encounter.SpeakInput{
		Range: speakReach, Actor: bard, Target: kingCaptain, OptionID: "guest", Skill: "persuasion",
		Beaten: true, DC: 11, Total: 20, Roller: rollsOne{}, Revision: offer["guest"].Revision,
	})
	require.ErrorIs(t, err, encounter.ErrReserved, "the bard cannot speak over the hero's roll")
	_, err = s.enc.Speak(t.Context(), &encounter.SpeakInput{
		Range: speakReach, Actor: hero, Target: kingCaptain, OptionID: "greet", Roller: rollsOne{},
	})
	require.NoError(t, err, "a line that reads no interaction is spoken while the claim waits")

	_, err = s.enc.Speak(t.Context(), &encounter.SpeakInput{
		Range: speakReach, Actor: hero, Target: kingCaptain, OptionID: "guest", Skill: "persuasion",
		DC: 11, Total: 3, Roller: rollsOne{}, Revision: at,
	})
	require.NoError(t, err, "the hero's verdict settles the claim")
	require.Equal(t, "warned-guest", s.entrance(), "and lands once")

	_, err = s.enc.Speak(t.Context(), &encounter.SpeakInput{
		Range: speakReach, Actor: hero, Target: kingCaptain, OptionID: "stand-aside", Skill: "intimidation",
		Beaten: false, DC: 14, Total: 3, Roller: rollsOne{}, Revision: bardsMenu["stand-aside"].Revision,
	})
	require.NoError(t, err, "a line that reads no interaction is never stale")
	_, err = s.enc.Speak(t.Context(), &encounter.SpeakInput{
		Range: speakReach, Actor: bard, Target: kingCaptain, OptionID: "guest", Skill: "persuasion",
		Beaten: true, DC: 11, Total: 20, Roller: rollsOne{}, Revision: bardsMenu["guest"].Revision,
	})
	require.Error(t, err, "the bard's menu from before the claim is refused")
	require.Equal(t, "warned-guest", s.entrance(), "and moves nothing")
}
