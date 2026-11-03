// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package castletest

// castle_king_post_test.go is THE GUARD HOLDS THE GATE (rpg-project#836 S1,
// ideas/the-guard-holds-the-gate): gate-1 is posted on the gate's one
// crossing, the cell beneath the gate in front of the shut door, and his body
// blocks while he is posted. A walk into him stops one cell short and names
// him. The captain's word ends the post in the beat a witness learns it:
// cowed (the stand-aside won) or turned (the garrison is the party's), and
// his table walks a cowed guard to his door post. A guard who did not see the
// captain cowed holds the gate. The other gate guard is not posted and is
// crossed as any neutral is.
//
// The dice are not walked here: the stand-aside and the turn are won by the
// route host, whose die picks every level's first row.

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter"
	"github.com/KirkDiggler/rpg-toolkit/tools/spatial"
)

// machineState is a member's machine state as the record holds it, the
// recipe's initial state written as empty.
func machineState(t *testing.T, enc *encounter.Encounter, id encounter.MemberID) string {
	t.Helper()
	for _, m := range enc.ToData().Members {
		if m.ID == id {
			if m.Machine == nil {
				return ""
			}
			return m.Machine.State
		}
	}
	require.FailNowf(t, "not in the run", "%s", id)

	return ""
}

// passageFor is the fold's answer for the hero at a cell.
func (s *kingScene) passageFor(cell encounter.CellAtInput) encounter.Passage {
	s.t.Helper()
	fact, err := s.enc.CellAt(cell)
	require.NoError(s.t, err)

	return fact.Passage
}

// aRoundPasses paces the hero between where it stands and a free cell
// beside it until the world clock rises once.
func (s *kingScene) aRoundPasses() {
	s.t.Helper()
	here := s.at(hero)
	var beside *spatial.Position
	for _, d := range [][2]float64{{1, 0}, {-1, 0}, {0, 1}, {0, -1}, {1, -1}, {-1, 1}} {
		cell := spatial.Position{X: here.X + d[0], Y: here.Y + d[1]}
		if s.passageFor(encounter.CellAtInput{Cell: cell, Mover: hero}) == encounter.PassageStandable {
			beside = &cell
			break
		}
	}
	require.NotNil(s.t, beside, "a free cell beside the hero")
	start := s.enc.ToData().Clock.HighWater
	require.True(s.t, s.pace(here, *beside, 12, func() bool { return s.enc.ToData().Clock.HighWater > start }),
		"a round passes")
}

func TestMadKingTheGuardHoldsTheGate(t *testing.T) {
	t.Parallel()
	crossing := encounter.CellAtInput{Cell: beneathTheGate, Mover: hero}

	t.Run("a walk into him stops one cell short, by his name", func(t *testing.T) {
		t.Parallel()
		s := openTheScript(t, gatehouseSeat, nil)
		require.Equal(t, beneathTheGate, s.at(kingGate1), "precondition: gate-1 is posted on the crossing")
		require.False(t, s.hostile(hero, kingGate1), "precondition: he is neutral")
		require.Equal(t, encounter.PassageBlocked, s.passageFor(crossing), "posted, his body holds the cell")

		s.stepTo(besideTheCrossing)
		_, err := s.enc.Step(&encounter.StepInput{Member: hero, To: beneathTheGate})
		var stop *encounter.StepObstructedError
		require.ErrorAs(t, err, &stop, "the walk stops")
		require.ErrorIs(t, err, encounter.ErrBodyInTheWay)
		require.Equal(t, string(kingGate1)+" stands in the way", stop.PublicReason(), "and names the guard the hero sees")
		require.Equal(t, besideTheCrossing, s.at(hero), "one cell short of him")
		require.Equal(t, encounter.ClockWorld, clockKind(t, s.enc, hero), "a bump is not a fight")

		require.Equal(t, encounter.PassagePassThrough, s.passageFor(encounter.CellAtInput{Cell: s.at(kingGate2), Mover: hero}),
			"the other gate guard is not posted: his cell is crossed as any neutral's")
	})

	t.Run("cowed with him watching, the crossing opens and he walks to his door post", func(t *testing.T) {
		t.Parallel()
		s := openTheScript(t, gatehouseSeat, nil)
		s.walkTo(s.at(kingCaptain))
		s.said(kingCaptain, "stand-aside", "intimidation", true)
		require.True(t, s.learners("gate-cowed")[kingGate1], "precondition: gate-1 saw the captain cowed")

		// THE LINE'S OWN ROUND: the hero's word is a world action, so the
		// round it takes is the one the post hears the fact in.
		require.Equal(t, "cowed", machineState(t, s.enc, kingGate1), "he stood aside in the round he learned it")
		for i := 0; i < 2 && s.at(kingGate1) != westDoorPost; i++ {
			s.aRoundPasses()
		}
		require.Equal(t, westDoorPost, s.at(kingGate1), "his table walked him to his door post")
		require.Equal(t, encounter.PassageStandable, s.passageFor(crossing), "the crossing is open floor")
		s.stepTo(beneathTheGate)
		require.Equal(t, encounter.ClockWorld, clockKind(t, s.enc, hero), "nobody fights")
	})

	t.Run("a guard who did not see the captain cowed holds the gate", func(t *testing.T) {
		t.Parallel()
		s := openTheCastleAs(t, castleHost{
			at: gatehouseSeat, sightOf: map[encounter.MemberID]int{kingGate1: 0},
		})
		s.walkTo(s.at(kingCaptain))
		s.said(kingCaptain, "stand-aside", "intimidation", true)
		require.True(t, s.learners("gate-cowed")[hero], "precondition: the captain was cowed")
		require.False(t, s.learners("gate-cowed")[kingGate1], "precondition: gate-1, who sees nothing, did not see it")

		for i := 0; i < 3; i++ {
			s.aRoundPasses()
		}
		require.Equal(t, "", machineState(t, s.enc, kingGate1), "still posted")
		require.Equal(t, beneathTheGate, s.at(kingGate1), "still on the crossing")
		require.Equal(t, encounter.PassageBlocked, s.passageFor(crossing), "still holding it")
	})

	t.Run("turned, the crossing opens", func(t *testing.T) {
		t.Parallel()
		s, _ := turnTheGarrison(t)
		require.True(t, s.learners("guards-turned")[kingGate1], "precondition: gate-1 heard the garrison turn")
		require.Equal(t, encounter.StanceAllied, s.stance(garrison, encounter.FactionParty), "precondition: the garrison is the party's")

		if machineState(t, s.enc, kingGate1) != "turned" {
			s.aRoundPasses()
		}
		require.Equal(t, "turned", machineState(t, s.enc, kingGate1), "he left the post in the round he heard it")
		require.NotEqual(t, encounter.PassageBlocked, s.passageFor(crossing), "the crossing is open")
	})
}
