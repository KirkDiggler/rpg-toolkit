// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package resolution

import (
	"encoding/json"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/status"
)

// lapseHeld is a status an author gave for three rounds, as the status
// compiler writes it: the ending is on the giving, not in the status.
func (s *GiveStatusTestSuite) lapseHeld() json.RawMessage {
	out, err := status.Compile(&status.CompileInput{
		ID:     "held",
		Status: json.RawMessage(`{"name":"Held","changes":[{"cannot":["move"]}],"maintenance":[{"ends_on":"run"}],"again":"ignored"}`),
		Until:  &status.Until{Rounds: 3},
	})
	s.Require().NoError(err)
	return out.Library
}

// heldBoth gives hero and bard the timed hold and returns both sheets' interaction.
//
// Rounds of world time pass only when someone acts or waits (a Wait act is its
// own story, rpg-project#1079); these tests hand resolution the lapse the
// session's timer would, none counts rounds.
func (s *GiveStatusTestSuite) heldBoth() (*ContestDamageTestSuite, *Input) {
	fixtures := s.fixtures()
	library := s.lapseHeld()
	give := func(interaction *Input, member, application string) *Output {
		out, err := GiveStatus(s.ctx, &GiveStatusInput{
			Interaction: interaction, Member: member,
			Library: library, StatusID: "held",
			SourceID: "hall-1", Source: cookRef(), ApplicationID: application,
		})
		s.Require().NoError(err)
		return out.Resolved
	}
	hero := fixtures.sheet(give(s.interaction(fixtures,
		Participant{Character: fixtures.saver(14)}, Participant{Character: fixtures.bard(1)}), heroID, "held:hero"), heroID)
	bard := fixtures.sheet(give(s.interaction(fixtures,
		Participant{Character: hero}, Participant{Character: fixtures.bard(1)}), bardID, "held:bard"), bardID)
	s.Require().NotEmpty(hero.Conditions)
	s.Require().NotEmpty(bard.Conditions)
	return fixtures, s.interaction(fixtures, Participant{Character: hero}, Participant{Character: bard})
}

func (s *GiveStatusTestSuite) boundary(interaction *Input, in *BoundaryInput) *Output {
	machine, err := NewBoundary(in)
	s.Require().NoError(err)
	interaction.Machine = machine
	out, err := Resolve(s.ctx, interaction)
	s.Require().NoError(err)
	return out
}

// A LAPSE ENDS THAT APPLICATION ONLY: the hero's rounds are up, the bard's are not.
func (s *GiveStatusTestSuite) TestARoundsLapseEndsThatApplicationOnly() {
	fixtures, interaction := s.heldBoth()
	out := s.boundary(interaction, &BoundaryInput{Lapses: []Lapse{{Member: heroID, ApplicationID: "held:hero"}}})

	s.Empty(fixtures.sheet(out, heroID).Conditions, "the hero's rounds were up")
	for _, dirty := range out.DirtyCharacters {
		if dirty.ID == bardID {
			s.NotEmpty(dirty.Conditions, "the bard's rounds are not up")
		}
	}
}

// A timer whose status already ended fires harmlessly.
func (s *GiveStatusTestSuite) TestALapseForAnApplicationNobodyHoldsChangesNothing() {
	_, interaction := s.heldBoth()
	out := s.boundary(interaction, &BoundaryInput{Lapses: []Lapse{{Member: heroID, ApplicationID: "held:nobody"}}})

	s.Empty(out.DirtyCharacters, "nothing listens for that application, so no sheet changed")
}

func (s *GiveStatusTestSuite) TestNewBoundaryRefusesNothingToAnnounceAndABlankLapse() {
	_, err := NewBoundary(&BoundaryInput{})
	s.ErrorIs(err, ErrBadBoundary)
	_, err = NewBoundary(&BoundaryInput{Lapses: []Lapse{}})
	s.ErrorIs(err, ErrBadBoundary)
	_, err = NewBoundary(&BoundaryInput{Lapses: []Lapse{{Member: "", ApplicationID: "held:hero"}}})
	s.ErrorIs(err, ErrBadBoundary, "a lapse needs its holder")
	_, err = NewBoundary(&BoundaryInput{Lapses: []Lapse{{Member: heroID, ApplicationID: " "}}})
	s.ErrorIs(err, ErrBadBoundary, "a lapse needs its application")
}

// A fight starting for the hero (which ends neither hold) and the bard's rounds running out, in one boundary.
func (s *GiveStatusTestSuite) TestCrossingsAndLapsesInOneBoundary() {
	_, interaction := s.heldBoth()
	out := s.boundary(interaction, &BoundaryInput{
		Crossed: []encounter.Boundary{{Kind: combatStarted, Subject: heroID, Round: 1}},
		Lapses:  []Lapse{{Member: bardID, ApplicationID: "held:bard"}},
	})
	bardEnded := false
	for _, dirty := range out.DirtyCharacters {
		if dirty.ID == bardID {
			bardEnded = len(dirty.Conditions) == 0
		}
	}
	s.True(bardEnded, "the bard's rounds were up, beside the crossing")
}
