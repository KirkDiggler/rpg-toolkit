// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package castletest

// castle_king_survivors_test.go is THE COURT AFTER THE KING (rpg-project#718,
// Frank's list S4): the loyalists' lines are the faction's, so every knight
// speaks them; the king's fall stands the court down by the file's
// `until: { down: king }`, the fight dissolves, and a surviving knight can be
// told to yield — or refuse, and the next blow brings the fight back (D4).
//
// THE ROUTE OPENS WITHOUT THE `king-slain` ENDING. On this file that ending
// fires on `{ down: king }` and closes the run at the fall, so no line could
// be said after it; S2's leave ending (rpg-toolkit#2155, lane D7) moves every
// ending to the front door. Until that lands this route takes the one ending
// out by id, which is the honest form: the content under test is the court's
// lines and the court's `until`, and neither reads an ending.

import (
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter"
)

// CastleSurvivorsSuite is the loyalists after the king falls.
type CastleSurvivorsSuite struct {
	suite.Suite
}

func TestCastleSurvivorsSuite(t *testing.T) { suite.Run(t, new(CastleSurvivorsSuite)) }

// felled is the castle opened beside the king without `king-slain`, and the
// king put down by the hero's spear, the blob saved and loaded back as a host
// does after a verb.
func (s *CastleSurvivorsSuite) felled() *kingScene {
	scene := openTheCastleAs(s.T(), castleHost{at: besideTheKing, withoutEndings: map[string]bool{"king-slain": true}})
	s.Require().Equal(encounter.StanceHostile, scene.stance(loyalists, encounter.FactionParty))
	s.Require().Equal(encounter.ClockTurn, clockKind(s.T(), scene.enc, hero), "precondition: the court saw the hero")

	scene.struck(theKing, true)
	scene.reload()

	return scene
}

// TestEveryLoyalistSpeaksTheCourtsLines: the knights and the king wrote no
// line of their own, and each carries the faction's two.
func (s *CastleSurvivorsSuite) TestEveryLoyalistSpeaksTheCourtsLines() {
	compiled := loadContent(s.T(), kingFixture)
	for _, m := range compiled.Monsters {
		if m.Faction != loyalists {
			continue
		}
		ids := make([]string, 0, len(m.Options))
		for _, o := range m.Options {
			ids = append(ids, o.ID)
		}
		s.Equal([]string{"yield", "refuse"}, ids, "%s speaks the court's lines", m.ID)
	}
}

// TestTheKingsFallStandsTheCourtDown: the pair turns neutral, and the
// fight it was dissolves.
func (s *CastleSurvivorsSuite) TestTheKingsFallStandsTheCourtDown() {
	scene := s.felled()

	s.Equal(encounter.StanceNeutral, scene.stance(loyalists, encounter.FactionParty))
	for _, id := range []encounter.MemberID{hero, kingKeep1, kingKeep2} {
		s.Equal(encounter.ClockWorld, clockKind(s.T(), scene.enc, id), "%s is out of the fight", id)
	}
	offered, err := scene.enc.OptionsFor(hero, theKing)
	s.Require().NoError(err)
	s.Empty(offered, "the king is down and speaks nothing")
}

// TestAKnightYieldsAndNobodySwings: the yield said to a surviving knight and
// beaten answers from the faction's row and teaches its fact; the court stays
// neutral and the fight stays gone.
func (s *CastleSurvivorsSuite) TestAKnightYieldsAndNobodySwings() {
	scene := s.felled()

	out, err := scene.speak(kingKeep1, "yield", "persuasion", true)
	s.Require().NoError(err)

	s.Require().NotNil(out.Answered)
	s.Equal("The king is dead. We yield.", out.Answered.Answer.Say)
	s.True(scene.learners("loyalists-yielded")[kingKeep1])
	s.Equal(encounter.StanceNeutral, scene.stance(loyalists, encounter.FactionParty))
	for _, id := range []encounter.MemberID{hero, kingKeep1, kingKeep2} {
		s.Equal(encounter.ClockWorld, clockKind(s.T(), scene.enc, id), "%s does not swing", id)
	}
}

// TestAKnightRefusesAndABlowBringsTheFightBack is D4 on the castle: the
// refusal changes nothing, and the hero's next blow across the neutral pair
// turns it hostile by provocation and the fight forms again.
func (s *CastleSurvivorsSuite) TestAKnightRefusesAndABlowBringsTheFightBack() {
	scene := s.felled()

	out, err := scene.speak(kingKeep2, "refuse", "intimidation", false)
	s.Require().NoError(err)
	s.Require().NotNil(out.Answered)
	s.Equal("Then we die fighting.", out.Answered.Answer.Say)
	s.Equal(encounter.StanceNeutral, scene.stance(loyalists, encounter.FactionParty), "a word turns nothing")

	scene.struck(kingKeep2, false)

	s.Equal(encounter.StanceHostile, scene.stance(loyalists, encounter.FactionParty))
	s.Equal(encounter.ClockTurn, clockKind(s.T(), scene.enc, hero), "the fight re-forms")
	s.Equal(encounter.ClockTurn, clockKind(s.T(), scene.enc, kingKeep2))
}
