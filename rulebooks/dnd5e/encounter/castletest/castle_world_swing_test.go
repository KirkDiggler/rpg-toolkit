// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package castletest

// castle_world_swing_test.go is rpg-project#785 R1 on the castle: a swing
// made from the world clock is an action, and an action on the world clock
// costs one round of the world, as a cast, a loot or a door verb does. R3
// orders it: the blow lands, the aggression law turns the pair, the fight
// forms with the one it struck in it, and the world outside that fight lives
// the round the swing took, raised by the world itself (worldtime.go's
// deedMovesTheWorld). Inside a fight the turn clock prices the swing,
// and the world charges nothing for it.

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter"
	"github.com/KirkDiggler/rpg-toolkit/tools/spatial"
)

// swingAt records one swing of the hero's at the target, as the session
// tells it: struck or missed, nobody falls.
func (s *kingScene) swingAt(kind encounter.OutcomeKind, target encounter.MemberID) {
	s.t.Helper()
	in := &encounter.RecordInput{
		Kind: kind, Actor: hero, Targets: []encounter.MemberID{target},
		Attack: &encounter.AttackIdentity{Ref: kingSpear.String(), Name: "Spear", DamageType: "piercing"},
	}
	if kind == encounter.OutcomeStruck {
		in.Values = map[encounter.OutcomeValue]int{encounter.ValueAmount: 6}
	}
	_, err := s.enc.RecordTrain(&encounter.RecordTrainInput{Units: []encounter.TrainUnit{{Outcome: in}}})
	require.NoError(s.t, err)
}

// beatKindsSince is every beat the hero holds from story index n on.
func (s *kingScene) beatKindsSince(n int) []string {
	s.t.Helper()
	all := s.beats(hero)
	out := make([]string, 0, len(all)-n)
	for _, b := range all[n:] {
		out = append(out, b.Beat)
	}

	return out
}

// countOf counts one kind in a list of beat kinds.
func countOf(kinds []string, kind string) int {
	n := 0
	for _, k := range kinds {
		if k == kind {
			n++
		}
	}

	return n
}

// indexOf is where a kind first appears in a list of beat kinds, or -1.
func indexOf(kinds []string, kind string) int {
	for i, k := range kinds {
		if k == kind {
			return i
		}
	}

	return -1
}

// TestWorldSwingPaysARound: a swing at the captain from free roam pays one
// round of the world after its blow, turns the garrison hostile, and forms
// the fight with the gate's posts in it; a miss pays the same; a swing inside
// the fight it formed pays the world nothing.
func TestWorldSwingPaysARound(t *testing.T) {
	t.Parallel()

	for _, kind := range []encounter.OutcomeKind{encounter.OutcomeStruck, encounter.OutcomeMissed} {
		t.Run(string(kind)+" from free roam pays one round", func(t *testing.T) {
			t.Parallel()
			s := openTheGateFight(t)
			require.Equal(t, encounter.ClockWorld, clockKind(t, s.enc, hero), "precondition: the hero is in free roam")
			require.Equal(t, encounter.StanceNeutral, s.stance(garrison, encounter.FactionParty),
				"precondition: the garrison is civil")
			before := s.enc.ToData().Clock.HighWater
			n := len(s.beats(hero))

			s.swingAt(kind, kingCaptain)

			kinds := s.beatKindsSince(n)
			require.Equal(t, 1, countOf(kinds, encounter.BeatTick), "exactly one round of the world: %v", kinds)
			require.Equal(t, before+1, s.enc.ToData().Clock.HighWater, "the world clock reads one round on")
			blow, stance, formed, tick := indexOf(kinds, string(kind)), indexOf(kinds, "stance"),
				indexOf(kinds, encounter.BeatFightStarted), indexOf(kinds, encounter.BeatTick)
			require.Equal(t, 0, blow, "the blow lands first: %v", kinds)
			require.Less(t, blow, stance, "then the aggression law turns the pair: %v", kinds)
			require.Less(t, stance, formed, "then the fight forms: %v", kinds)
			require.Less(t, formed, tick, "and the round is charged after the outcome: %v", kinds)

			require.Equal(t, encounter.StanceHostile, s.stance(garrison, encounter.FactionParty),
				"the garrison turned hostile to the party")
			for _, post := range []encounter.MemberID{hero, kingFighter, kingCaptain, kingGate1, kingGate2} {
				require.Equal(t, encounter.ClockTurn, clockKind(t, s.enc, post), "%s is in the fight", post)
			}
		})
	}

	t.Run("a swing inside the fight pays the world nothing", func(t *testing.T) {
		t.Parallel()
		s := openTheGateFight(t)
		s.swingAt(encounter.OutcomeStruck, kingCaptain)
		require.Equal(t, encounter.ClockTurn, clockKind(t, s.enc, hero), "precondition: the first swing formed the fight")
		before := s.enc.ToData().Clock.HighWater
		n := len(s.beats(hero))

		s.swingAt(encounter.OutcomeMissed, kingCaptain)

		kinds := s.beatKindsSince(n)
		require.Zero(t, countOf(kinds, encounter.BeatTick), "the turn clock prices it: %v", kinds)
		require.Equal(t, before, s.enc.ToData().Clock.HighWater, "the world clock has not moved")
	})
}

// TestAStruckKnightStandsInTheFightAndTheWorldMovesOn is the ruling's own
// scene (rpg-project#785 R3 as the lead amended it): with the king fallen and
// the court civil, the hero strikes keep-2 from across the yard. The fight
// forms with keep-2 in it — and keep-1, who can see the hero — so neither
// wanders off on its patrol; the round the swing took is the world's, and
// hall-1, still on the world clock across the hall, walks its patrol in it.
// A reload keeps the world's own progress.
func TestAStruckKnightStandsInTheFightAndTheWorldMovesOn(t *testing.T) {
	t.Parallel()
	s := openTheCastleAs(t, castleHost{at: besideTheKing})
	s.struck(theKing, true)
	s.reload()
	_, err := s.speakWithin(10, kingKeep2, "refuse", "intimidation", false)
	require.NoError(t, err)
	require.Equal(t, encounter.ClockWorld, clockKind(t, s.enc, hero), "precondition: the hero is in free roam")
	keep1, keep2, hall1 := s.at(kingKeep1), s.at(kingKeep2), s.at(kingHall1)
	before := s.enc.ToData().Clock.HighWater
	n := len(s.beats(hero))

	s.struck(kingKeep2, false)

	kinds := s.beatKindsSince(n)
	require.Equal(t, 1, countOf(kinds, encounter.BeatTick), "exactly one round of the world: %v", kinds)
	require.Less(t, indexOf(kinds, encounter.BeatFightStarted), indexOf(kinds, encounter.BeatTick),
		"the fight forms first, then the world moves: %v", kinds)
	require.Equal(t, before+1, s.enc.ToData().Clock.HighWater)

	for id, was := range map[encounter.MemberID]spatial.Position{kingKeep1: keep1, kingKeep2: keep2} {
		require.Equal(t, encounter.ClockTurn, clockKind(t, s.enc, id), "%s is in the fight", id)
		require.Equal(t, was, s.at(id), "%s does not wander", id)
	}
	require.Equal(t, encounter.ClockWorld, clockKind(t, s.enc, kingHall1), "hall-1 is still on the world clock")
	require.NotEqual(t, hall1, s.at(kingHall1), "hall-1 walks its patrol in the round the swing took")

	s.reload()
	require.Equal(t, before+1, s.enc.ToData().Clock.HighWater, "the world's round survives a reload")
}
