// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package castletest

// castle_king_bump_test.go is THE BUMP SPEAKS on the castle (rpg-project#842,
// S2 of the guard holds the gate, R2 and R7): walking into gate-1, or across
// the opening he holds, or putting a hand on the gate door he holds, writes
// `barred`, and he answers at once in his own words. A second bump while he
// is still challenging gets the shorter line.

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter"
	"github.com/KirkDiggler/rpg-toolkit/tools/spatial"
)

// The gate guard's two lines, as the castle file writes them.
const (
	gateQuestion      = "Whoa there, buddy. Can I help you?"
	gateQuestionAgain = "I said, can I help you?"
)

// gateLines is everything gate-1 has said, in record order.
func (s *kingScene) gateLines() []string {
	s.t.Helper()
	return s.saidBy(hero, kingGate1)
}

// bumps steps the hero into a body and requires the stop, naming gate-1,
// and the bump on the record.
func (s *kingScene) bumps(to spatial.Position) {
	s.t.Helper()
	from := s.at(hero)
	before := len(s.beatsOf(hero, encounter.BeatBarred))
	out, err := s.enc.Step(&encounter.StepInput{Member: hero, To: to})
	require.NoError(s.t, err, "a bump is answered, not refused")
	require.NotNil(s.t, out.Stopped, "the step is stopped by a body")
	require.Equal(s.t, kingGate1, out.Stopped.Barrier, "gate-1 is the body in the way")
	require.Equal(s.t, from, s.at(hero), "and nobody moved")
	require.Len(s.t, s.beatsOf(hero, encounter.BeatBarred), before+1, "the bump is on the record")
}

func TestMadKingTheGuardAnswersTheBump(t *testing.T) {
	t.Parallel()

	t.Run("walked into at the shut gate, he asks, and asks again shorter", func(t *testing.T) {
		t.Parallel()
		s := openTheScript(t, gatehouseSeat, nil)
		s.stepTo(besideTheCrossing)
		require.Empty(t, s.gateLines(), "precondition: he has said nothing")

		s.bumps(beneathTheGate)
		require.Equal(t, []string{gateQuestion}, s.gateLines(), "his question, at once")
		require.Equal(t, "challenged", machineState(t, s.enc, kingGate1), "and he is challenging")

		s.bumps(beneathTheGate)
		lines := s.gateLines()
		require.Equal(t, gateQuestionAgain, lines[len(lines)-1], "a second bump gets the shorter line")
		require.Equal(t, beneathTheGate, s.at(kingGate1), "and he still holds the crossing")
		require.Equal(t, encounter.ClockWorld, clockKind(t, s.enc, hero), "a bump is not a fight")

		// THE CASTLE'S OWN RETURN: three rounds on, challenged stands down
		// to posted, still on his post, and a bump asks the question afresh.
		for range 3 {
			s.aRoundPasses()
		}
		require.Contains(t, []string{"", "posted"}, machineState(t, s.enc, kingGate1), "three rounds on, he is posted again")
		require.Equal(t, beneathTheGate, s.at(kingGate1), "on his post")
		if s.at(hero) != besideTheCrossing {
			s.stepTo(besideTheCrossing)
		}
		s.bumps(beneathTheGate)
		lines = s.gateLines()
		require.Equal(t, gateQuestion, lines[len(lines)-1], "posted again, he asks afresh")
	})

	t.Run("across the open gate's other cell, the holder asks", func(t *testing.T) {
		t.Parallel()
		s := openTheHeldGate(t, castleHost{at: besideTheCrossingSeat, soleGateHolder: true})
		require.Equal(t, beneathTheGate, s.at(kingGate1), "precondition: gate-1 is on his post")

		s.bumps(pastTheGate)
		require.Equal(t, []string{gateQuestion}, s.gateLines(), "the holder hears the bump beside him")
	})

	t.Run("a hand on the gate door he holds is a bump", func(t *testing.T) {
		t.Parallel()
		s := openTheCastleAs(t, castleHost{at: besideTheCrossingSeat, soleGateHolder: true})
		out, err := s.enc.OpenDoor(&encounter.OpenDoorInput{Door: kingGateDoor, Actor: hero})
		require.NoError(t, err)
		require.NotNil(t, out.Refused, "the held gate door refuses the hero")
		require.Equal(t, kingGate1, out.Refused.Holder)
		require.Equal(t, []string{gateQuestion}, s.gateLines(), "and he asks what the hero wants")
	})
}
