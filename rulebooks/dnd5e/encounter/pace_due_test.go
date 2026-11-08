// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package encounter_test

import (
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter"
	"github.com/KirkDiggler/rpg-toolkit/tools/spatial"
)

// pace_due_test.go pins [encounter.Encounter.PaceDue] against the payment it
// predicts (rpg-project#879 S1): the session splits a walk at the step it
// names, so a prediction that drifts from the step that really raises the
// world splits the walk in the wrong place.

// TestPaceDueNamesTheStepThatRaisesTheWorld walks a thirty-foot member across
// more than two paces and, before every step, asks whether it is due; the
// answer must be exactly whether that step then raised the world clock.
func (s *WorldTimeSuite) TestPaceDueNamesTheStepThatRaisesTheWorld() {
	enc := s.hall(encounter.MemberInput{
		ID: alice, Kind: encounter.KindPlayer, Position: spatial.Position{X: 1, Y: 1},
	})

	raises := 0
	for x := 2; x <= 15; x++ {
		due, err := enc.PaceDue(&encounter.PaceDueInput{Member: alice})
		s.Require().NoError(err)

		before := enc.WorldRound()
		_, err = enc.Step(&encounter.StepInput{Member: alice, To: cellAt(x, 1)})
		s.Require().NoError(err, "step to %d", x)
		raised := enc.WorldRound() > before

		s.Equal(raised, due.Due, "step to x=%d: PaceDue said %v, the step raised %v", x, due.Due, raised)
		if raised {
			raises++
		}
	}
	s.Equal(2, raises, "fourteen cells of a thirty-foot walker is two paces and two over")
}

// TestPaceDueIsARead: asking twice changes nothing, so the answer is the same
// and the walk that follows raises where it would have anyway.
func (s *WorldTimeSuite) TestPaceDueIsARead() {
	enc := s.hall(encounter.MemberInput{
		ID: alice, Kind: encounter.KindPlayer, Position: spatial.Position{X: 1, Y: 1},
	})
	s.walk(enc, alice, 1, 5, 1)

	for range 3 {
		due, err := enc.PaceDue(&encounter.PaceDueInput{Member: alice})
		s.Require().NoError(err)
		s.True(due.Due, "the sixth cell is the pace, however often it is asked")
	}
	s.Zero(s.reading(enc), "asking raised nothing")
	s.walk(enc, alice, 6, 1, 1)
	s.Equal(1, s.reading(enc), "and the sixth cell still pays it")
}

// TestPaceDueForAMemberWhoPacesNothing: a zero-speed sheet and a member in a
// fight both walk without paying the world, so neither is ever due.
func (s *WorldTimeSuite) TestPaceDueForAMemberWhoPacesNothing() {
	s.Run("zero speed", func() {
		enc := s.hallSheets(sheetFacts{alice: {}}, nil, encounter.MemberInput{
			ID: alice, Kind: encounter.KindPlayer, Position: spatial.Position{X: 1, Y: 1},
		})
		for x := 2; x <= 9; x++ {
			due, err := enc.PaceDue(&encounter.PaceDueInput{Member: alice})
			s.Require().NoError(err)
			s.False(due.Due, "a creature that cannot walk paces nothing (x=%d)", x)
			_, err = enc.Step(&encounter.StepInput{Member: alice, To: cellAt(x, 1)})
			s.Require().NoError(err)
		}
	})

	s.Run("in a fight", func() {
		enc := s.hall(
			encounter.MemberInput{ID: alice, Kind: encounter.KindPlayer,
				Position: spatial.Position{X: 1, Y: 1}},
			encounter.MemberInput{ID: goblin, Kind: encounter.KindMonster,
				Position: spatial.Position{X: 3, Y: 1}},
		)
		on, err := enc.ClockOf(&encounter.ClockOfInput{Member: alice})
		s.Require().NoError(err)
		s.Require().Equal(encounter.ClockTurn, on.Kind, "precondition: seeing each other started a fight")

		due, err := enc.PaceDue(&encounter.PaceDueInput{Member: alice})
		s.Require().NoError(err)
		s.False(due.Due, "the turn clock prices a fighter's time, never pace")
	})
}

// TestPaceDueRefusals: the input errors, in validation order.
func (s *WorldTimeSuite) TestPaceDueRefusals() {
	enc := s.hall(encounter.MemberInput{
		ID: alice, Kind: encounter.KindPlayer, Position: spatial.Position{X: 1, Y: 1},
	})

	_, err := enc.PaceDue(nil)
	s.ErrorIs(err, encounter.ErrNilInput)

	_, err = enc.PaceDue(&encounter.PaceDueInput{})
	s.ErrorIs(err, encounter.ErrNoMember)

	_, err = enc.PaceDue(&encounter.PaceDueInput{Member: "nobody"})
	s.ErrorIs(err, encounter.ErrNotMember)
}

// TestAStepNamesItsBeatsRecordID: StepOutput.ID is the record id of the moved
// beat at StepOutput.Seq, the id a stream consumer resumes after.
func (s *WorldTimeSuite) TestAStepNamesItsBeatsRecordID() {
	enc := s.hall(encounter.MemberInput{
		ID: alice, Kind: encounter.KindPlayer, Position: spatial.Position{X: 1, Y: 1},
	})

	out, err := enc.Step(&encounter.StepInput{Member: alice, To: cellAt(2, 1)})
	s.Require().NoError(err)
	s.Require().NotEmpty(out.ID)

	story, err := enc.Story(&encounter.StoryInput{Audience: alice})
	s.Require().NoError(err)
	found := false
	for _, entry := range story {
		if entry.Seq == out.Seq {
			found = true
			s.Equal(entry.ID, out.ID, "the id is the entry's at that seq")
		}
	}
	s.True(found, "the step's seq is in the story")
}
