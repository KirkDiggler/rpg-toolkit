// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package encounter_test

import (
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter"
	"github.com/KirkDiggler/rpg-toolkit/tools/spatial"
)

// shape_test.go is about [Encounter.MembersWithin]: naming a footprint in the
// moment rather than pointing at an authored region, and getting back who is
// standing in it.
//
// Every scene here places members by AUTHORED OFFSET and then asserts the
// distances it is about before asserting anything else. That is not ceremony:
// cellAt takes offset and the composition measures in axial, and a scene that
// assumed "one column apart means adjacent" without checking would pass or fail
// for reasons that have nothing to do with the code under test
// (rpg-toolkit#1141, #1150).

const shapeRegion encounter.RegionID = "yard"

type ShapeSuite struct {
	suite.Suite
	enc *encounter.Encounter

	// Where each member ACTUALLY stands, in the dungeon-absolute axial cells
	// Member.Position reports and MembersWithin.Origin takes. Authored offset
	// goes in (see SetupTest); axial comes back.
	bardAt, closeAt, farAt spatial.Position
}

func TestShapeSuite(t *testing.T) {
	suite.Run(t, new(ShapeSuite))
}

func (s *ShapeSuite) SetupTest() {
	// MemberInput.Position is AUTHORED OFFSET — the composition converts it to
	// the absolute axial cell every read reports back. Handing it a cellAt()
	// value would convert twice and place everybody somewhere nobody meant,
	// which is what the first draft of this file did.
	const bardCol, closeCol, farCol, row = 5, 6, 8, 5
	offset := func(col int) spatial.Position {
		return spatial.Position{X: float64(col), Y: float64(row)}
	}
	s.bardAt = cellAt(bardCol, row)
	s.closeAt = cellAt(closeCol, row)
	s.farAt = cellAt(farCol, row)

	enc, err := encounter.NewEncounter(&encounter.SetupInput{
		Sight: everyoneSeesTheWholeMap{}, Equipment: noHandsAreObserved{},
		Standing: everyoneStanding{}, Initiative: orderAsGiven{}, TurnDriver: passDriver{},
		Striker: passStriker{}, Mover: quietMover{}, Announcer: quietAnnouncer{},
		Field: encounter.FieldInput{
			Canvas:  pointyCanvas(),
			Regions: []encounter.RegionInput{rectRegion(string(shapeRegion), 0, 0, 14, 14)},
		},
		Members: []encounter.MemberInput{
			{ID: "bard", Kind: encounter.KindPlayer, Position: offset(bardCol)},
			{ID: "close", Kind: encounter.KindPlayer, Position: offset(closeCol)},
			{ID: "far", Kind: encounter.KindPlayer, Position: offset(farCol)},
			{ID: "vendor", Kind: encounter.KindWorld, Position: offset(bardCol)},
		},
		Endings: []encounter.EndingInput{{Key: "withdrawn", Trigger: encounter.TriggerExternal{}}},
	})
	s.Require().NoError(err)
	s.enc = enc

	// The scene's own assumptions, asserted before anything depends on them —
	// including that every member landed where this file thinks it did. The
	// first draft got that wrong and every distance assertion still passed,
	// because axial arithmetic on the wrong cells is still valid arithmetic.
	placed := map[encounter.MemberID]spatial.Position{}
	roster, err := enc.Members()
	s.Require().NoError(err)
	for _, m := range roster {
		placed[m.ID] = m.Position
	}
	s.Require().Equal(s.bardAt, placed["bard"], "bard must stand where this scene says")
	s.Require().Equal(s.closeAt, placed["close"])
	s.Require().Equal(s.farAt, placed["far"])
	s.Require().Equal(s.bardAt, placed["vendor"], "the vendor shares the bard's cell")

	s.Require().Equal(0.0, enc.Distance(s.bardAt, s.bardAt))
	s.Require().Equal(1.0, enc.Distance(s.bardAt, s.closeAt), "close must be one cell east of the bard")
	s.Require().Equal(3.0, enc.Distance(s.bardAt, s.farAt), "far must be three cells east of the bard")
}

func (s *ShapeSuite) caught(origin spatial.Position, cells float64) []encounter.MemberID {
	in, err := s.enc.MembersWithin(&encounter.MembersWithinInput{Origin: origin, RadiusCells: cells})
	s.Require().NoError(err)
	ids := make([]encounter.MemberID, 0, len(in))
	for _, m := range in {
		ids = append(ids, m.ID)
	}
	return ids
}

// TestReachDecidesWhoIsCaught is the headline, and it is a boundary test on
// purpose: "far" sits at exactly three cells, so a reach of three catches it and
// a reach of two does not. That pins the comparison as <= rather than <, and
// pins it against the composition's OWN Distance rather than arithmetic a test
// did for itself.
func (s *ShapeSuite) TestReachDecidesWhoIsCaught() {
	s.Run("a reach of zero is the origin cell alone", func() {
		s.ElementsMatch([]encounter.MemberID{"bard", "vendor"}, s.caught(s.bardAt, 0))
	})
	s.Run("a reach of one takes in the neighbour", func() {
		s.ElementsMatch([]encounter.MemberID{"bard", "vendor", "close"}, s.caught(s.bardAt, 1))
	})
	s.Run("a reach of two still does not reach three cells away", func() {
		s.ElementsMatch([]encounter.MemberID{"bard", "vendor", "close"}, s.caught(s.bardAt, 2))
	})
	s.Run("a reach of exactly three does", func() {
		s.ElementsMatch([]encounter.MemberID{"bard", "vendor", "close", "far"}, s.caught(s.bardAt, 3))
	})
}

// TestTheCasterIsReturnedAndSoIsTheShopkeeper is the seam, stated as a test.
//
// This module is not told why it was asked, so it cannot leave anybody out.
// "Every creature other than you" is a rule a spell states, and the projection
// belongs to whoever holds the rule. A KindWorld member standing in the blast is
// likewise REPORTED and kind-tagged, so a caller that must treat it differently
// can see it rather than discovering later that something in the blast was
// silently never mentioned.
func (s *ShapeSuite) TestTheCasterIsReturnedAndSoIsTheShopkeeper() {
	in, err := s.enc.MembersWithin(&encounter.MembersWithinInput{Origin: s.bardAt, RadiusCells: 1})
	s.Require().NoError(err)

	kinds := map[encounter.MemberID]encounter.MemberKind{}
	for _, m := range in {
		kinds[m.ID] = m.Kind
	}
	s.Contains(kinds, encounter.MemberID("bard"), "the member at the centre is not filtered out here")
	s.Equal(encounter.KindWorld, kinds["vendor"], "a world member is reported, and reported AS one")
}

// TestItAgreesWithMembersAboutWhereAnybodyStands — MembersWithin is a roster
// read, so it must report what Members reports, filtered. A parallel projection
// would be a second answer to "where is the bard", which is the dual-state
// defect placementOf exists to prevent.
func (s *ShapeSuite) TestItAgreesWithMembersAboutWhereAnybodyStands() {
	all, err := s.enc.Members()
	s.Require().NoError(err)
	byID := map[encounter.MemberID]encounter.Member{}
	for _, m := range all {
		byID[m.ID] = m
	}

	in, err := s.enc.MembersWithin(&encounter.MembersWithinInput{Origin: s.bardAt, RadiusCells: 3})
	s.Require().NoError(err)
	s.Require().NotEmpty(in)
	for _, m := range in {
		s.Equal(byID[m.ID], m, "MembersWithin and Members must agree about %q", m.ID)
	}
}

// TestAnEmptyFootprintIsAnAnswerButABackwardsOneIsNot.
//
// Nobody standing in the blast is a fact worth reporting — a spell that catches
// nobody still happened. A NEGATIVE reach catches nobody too, arithmetically,
// and that is exactly why it must not be allowed to say so: content that
// converted feet to cells and came out below zero would be indistinguishable
// from a spell that simply missed, and would quietly do nothing forever.
func (s *ShapeSuite) TestAnEmptyFootprintIsAnAnswerButABackwardsOneIsNot() {
	s.Run("an empty footprint is an ordinary answer", func() {
		s.Empty(s.caught(cellAt(13, 13), 1))
	})
	s.Run("a backwards footprint is refused", func() {
		_, err := s.enc.MembersWithin(&encounter.MembersWithinInput{Origin: s.bardAt, RadiusCells: -1})
		s.Require().Error(err)
		s.ErrorIs(err, encounter.ErrBadReach)
	})
	s.Run("nil input is refused", func() {
		_, err := s.enc.MembersWithin(nil)
		s.Require().Error(err)
		s.ErrorIs(err, encounter.ErrNilInput)
	})
}
