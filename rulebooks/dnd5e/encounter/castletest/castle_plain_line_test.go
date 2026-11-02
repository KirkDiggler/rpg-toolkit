// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package castletest

// castle_plain_line_test.go is rpg-project#785 R5 on the castle: "the check
// is the action… speaking with no check would be a free action". The hero
// greets the captain — a plain line, no `check` — and the world does not
// move; then threatens him — Intimidation, a check — and the world is paid
// its round.

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter"
)

// TestGreetingTheCaptainIsFreeAndAThreatIsTheAction: greet, no tick; stand
// aside, one tick.
func TestGreetingTheCaptainIsFreeAndAThreatIsTheAction(t *testing.T) {
	t.Parallel()
	s := openTheCastleAs(t, castleHost{at: eastOfTheCaptain})
	require.Equal(t, encounter.ClockWorld, clockKind(t, s.enc, hero), "precondition: the hero is in free roam")

	offered, err := s.enc.OptionsFor(hero, kingCaptain)
	require.NoError(t, err)
	checked := map[string]bool{}
	for _, o := range offered {
		checked[o.ID] = o.Checked()
	}
	greetChecked, greetOffered := checked["greet"]
	require.True(t, greetOffered, "the captain offers a stranger the greeting")
	require.False(t, greetChecked, "and it is a plain line")
	threatChecked, threatOffered := checked["stand-aside"]
	require.True(t, threatOffered, "and the threat")
	require.True(t, threatChecked, "which rolls a check")

	// The hero hails him from where they stand, the way the gate is talked
	// at: reach is not what this test is about.
	reach := int(s.enc.Distance(s.at(hero), s.at(kingCaptain)))
	before := s.enc.ToData().Clock.HighWater
	n := len(s.beats(hero))
	out, err := s.enc.Speak(context.Background(), &encounter.SpeakInput{
		Range: reach, Actor: hero, Target: kingCaptain, OptionID: "greet", Roller: rollsOne{},
	})
	require.NoError(t, err)
	require.NotNil(t, out.Answered, "the captain answers a greeting")
	require.Equal(t, "Evening. Keep to the road and mind the guards.", out.Answered.Answer.Say)
	kinds := s.beatKindsSince(n)
	require.Zero(t, countOf(kinds, encounter.BeatTick), "a plain line pays the world nothing: %v", kinds)
	require.Equal(t, before, s.enc.ToData().Clock.HighWater, "the world clock has not moved")

	n = len(s.beats(hero))
	_, err = s.speakWithin(reach, kingCaptain, "stand-aside", "intimidation", true)
	require.NoError(t, err)
	kinds = s.beatKindsSince(n)
	require.Equal(t, 1, countOf(kinds, encounter.BeatTick), "the threat rolled a check, and a check is the action: %v", kinds)
	require.Equal(t, before+1, s.enc.ToData().Clock.HighWater, "the world clock reads one round on")
}
