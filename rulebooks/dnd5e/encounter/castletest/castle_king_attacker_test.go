// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package castletest

// castle_king_attacker_test.go is SMARTER MONSTERS S2 (rpg-project#669) in
// the castle: the captain's `attack` and `toward` rows carry `target:
// attacker`, so he goes for the one who hit him, past a nearer enemy, while
// he still sees them; and for the closest when nobody hit him.

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter"
	"github.com/KirkDiggler/rpg-toolkit/tools/spatial"
)

// The gatehouse cells of the route, as offset cells for
// [encounter.MemberInput.Position]. The captain is posted at (0, 8).
var (
	// eastOfTheCaptain is four cells east of the captain, on his row: where
	// the hero stands when he hits the captain.
	eastOfTheCaptain = spatial.Position{X: 4, Y: 8}

	// westOfTheCaptain is two cells west of the captain: where the fighter
	// stands, nearer the captain than the hero.
	westOfTheCaptain = spatial.Position{X: -2, Y: 8}
)

// targetBeat is the fields of an `answered` beat this route reads.
type targetBeat struct {
	Beat     string `json:"beat"`
	Creature string `json:"creature"`
	Word     string `json:"word"`
	Target   string `json:"target"`
	Targeted string `json:"targeted"`
}

// openTheGateFight opens the castle with the hero east of the captain and
// the fighter, nearer, west of him.
func openTheGateFight(t *testing.T) *kingScene {
	t.Helper()
	s := openTheCastleAs(t, castleHost{
		at:      eastOfTheCaptain,
		players: []encounter.MemberInput{{ID: kingFighter, Kind: encounter.KindPlayer, Position: westOfTheCaptain}},
	})
	require.Less(t, s.enc.Distance(s.at(kingFighter), s.at(kingCaptain)),
		s.enc.Distance(s.at(hero), s.at(kingCaptain)), "precondition: the fighter is nearer the captain than the hero")

	return s
}

// captainsFirstStrike ends each player's turn as it comes up, until the
// captain swings, and answers whom he swung at.
func (s *kingScene) captainsFirstStrike() encounter.MemberID {
	s.t.Helper()
	for range 6 {
		for _, strike := range *s.strikes {
			if strike.attacker == kingCaptain {
				return strike.target
			}
		}
		on, err := s.enc.ClockOf(&encounter.ClockOfInput{Member: hero})
		require.NoError(s.t, err)
		require.Equal(s.t, encounter.ClockTurn, on.Kind, "the fight is still on")
		s.reload()
		_, err = s.enc.EndTurn(&encounter.EndTurnInput{Member: on.Active})
		require.NoError(s.t, err, "%s ends its turn", on.Active)
	}
	s.t.Fatal("the captain never swung")

	return ""
}

// captainsPicks is every `answered` beat of the captain's that carried a
// selector word, in story order.
func (s *kingScene) captainsPicks() []targetBeat {
	s.t.Helper()
	story, err := s.enc.Story(&encounter.StoryInput{Audience: hero})
	require.NoError(s.t, err)
	var out []targetBeat
	for _, entry := range story {
		var b targetBeat
		require.NoError(s.t, json.Unmarshal(entry.Payload, &b))
		if b.Beat == encounter.BeatAnswered && b.Creature == string(kingCaptain) && b.Target != "" {
			out = append(out, b)
		}
	}

	return out
}

// TestMadKingCaptainGoesForWhoeverHitHim is the slice's checklist: hit the
// captain with an ally nearer him and he attacks you; never hit him and he
// attacks the closest. Each pick's record says which policy named its
// target, so the fallback reads as one.
func TestMadKingCaptainGoesForWhoeverHitHim(t *testing.T) {
	t.Parallel()

	t.Run("hit him, with an ally nearer: he comes for you", func(t *testing.T) {
		t.Parallel()
		s := openTheGateFight(t)
		s.struck(kingCaptain, false)
		require.Equal(t, encounter.ClockTurn, clockKind(t, s.enc, kingCaptain), "precondition: the captain is in the fight")

		require.Equal(t, hero, s.captainsFirstStrike(), "the captain strikes the one who hit him, not the nearer fighter")
		picks := s.captainsPicks()
		require.NotEmpty(t, picks)
		for _, p := range picks {
			require.Equal(t, string(encounter.TargetAttacker), p.Target, "the captain's rows ask for his attacker")
			require.Equal(t, string(encounter.TargetAttacker), p.Targeted, "and his attacker, in sight, is who they named (%s)", p.Word)
		}
	})

	t.Run("never hit him: he comes for the closest", func(t *testing.T) {
		t.Parallel()
		s := openTheGateFight(t)
		s.struck(kingGate1, false)
		require.Equal(t, encounter.ClockTurn, clockKind(t, s.enc, kingCaptain), "precondition: the captain is in the fight")

		require.Equal(t, kingFighter, s.captainsFirstStrike(), "nobody hit the captain: he strikes the nearest")
		picks := s.captainsPicks()
		require.NotEmpty(t, picks)
		for _, p := range picks {
			require.Equal(t, string(encounter.TargetAttacker), p.Target)
			require.Equal(t, string(encounter.TargetClosest), p.Targeted, "the record says it fell back to the closest (%s)", p.Word)
		}
	})
}
