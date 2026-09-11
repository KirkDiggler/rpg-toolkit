// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package encounter_test

import (
	"math"
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

// --- MembersCovered: a footprint aimed at a cell -----------------------------

// CoveredSuite is about [Encounter.MembersCovered]: a shape drawn on the plane
// rather than a reach measured in cells, and who is standing under it.
//
// THE SCENE IS ONE AXIAL ROW, for the reason the file's header gives. Under
// pointy-top, cells sharing an authored row convert to cells sharing an axial R,
// so the bearing from the caster to anyone here is due east and the box is drawn
// along an axis. That is the case worth pinning first: it is the one a reader
// can check by hand, and the off-axis case is the walk's job (the design says
// the blob is two to three wide depending on the bearing, and that is the rule
// working rather than a bug).
//
//	authored col:  4       5       6       7       9
//	row 5:      behind  caster    .     ahead    far
//
// A 15-foot box on the caster's EDGE starts half a cell out and runs 15 feet
// from there, so it covers the three cells ahead and stops: the fourth cell's
// near boundary is exactly the box's far edge.
type CoveredSuite struct {
	suite.Suite
	enc *encounter.Encounter

	casterAt, aheadAt, behindAt, farAheadAt spatial.Position
}

func TestCoveredSuite(t *testing.T) {
	suite.Run(t, new(CoveredSuite))
}

func (s *CoveredSuite) SetupTest() {
	const casterCol, aheadCol, behindCol, farAheadCol, row = 5, 7, 4, 9, 5
	offset := func(col int) spatial.Position {
		return spatial.Position{X: float64(col), Y: float64(row)}
	}
	s.casterAt = cellAt(casterCol, row)
	s.aheadAt = cellAt(aheadCol, row)
	s.behindAt = cellAt(behindCol, row)
	s.farAheadAt = cellAt(farAheadCol, row)

	enc, err := encounter.NewEncounter(&encounter.SetupInput{
		Sight: everyoneSeesTheWholeMap{}, Equipment: noHandsAreObserved{},
		Standing: everyoneStanding{}, Initiative: orderAsGiven{}, TurnDriver: passDriver{},
		Striker: passStriker{}, Mover: quietMover{}, Announcer: quietAnnouncer{},
		Field: encounter.FieldInput{
			Canvas:  pointyCanvas(),
			Regions: []encounter.RegionInput{rectRegion(string(shapeRegion), 0, 0, 14, 14)},
		},
		Members: []encounter.MemberInput{
			{ID: "caster", Kind: encounter.KindPlayer, Position: offset(casterCol)},
			{ID: "ahead", Kind: encounter.KindMonster, Position: offset(aheadCol)},
			{ID: "behind", Kind: encounter.KindPlayer, Position: offset(behindCol)},
			{ID: "far-ahead", Kind: encounter.KindMonster, Position: offset(farAheadCol)},
		},
		Endings: []encounter.EndingInput{{Key: "withdrawn", Trigger: encounter.TriggerExternal{}}},
	})
	s.Require().NoError(err)
	s.enc = enc

	// The scene's own assumptions, asserted before anything depends on them.
	placed := map[encounter.MemberID]spatial.Position{}
	roster, err := enc.Members()
	s.Require().NoError(err)
	for _, m := range roster {
		placed[m.ID] = m.Position
	}
	s.Require().Equal(s.casterAt, placed["caster"])
	s.Require().Equal(s.aheadAt, placed["ahead"])
	s.Require().Equal(s.behindAt, placed["behind"])
	s.Require().Equal(s.farAheadAt, placed["far-ahead"])

	s.Require().Equal(2.0, enc.Distance(s.casterAt, s.aheadAt), "ahead is two cells out")
	s.Require().Equal(1.0, enc.Distance(s.casterAt, s.behindAt), "behind is one cell the other way")
	s.Require().Equal(4.0, enc.Distance(s.casterAt, s.farAheadAt), "far-ahead is four cells out")
	s.Require().Equal(
		enc.Distance(s.behindAt, s.farAheadAt), enc.Distance(s.behindAt, s.casterAt)+
			enc.Distance(s.casterAt, s.farAheadAt),
		"all four stand on one straight line, so the box is drawn along an axis")
}

// memberIDs is the roster ids of a covered set, in the order it came back.
func memberIDs(members []encounter.Member) []encounter.MemberID {
	out := make([]encounter.MemberID, 0, len(members))
	for _, m := range members {
		out = append(out, m.ID)
	}

	return out
}

// TestMembersCoveredByAnEdgeAnchoredBox.
//
// The 15-foot cube, aimed. Everyone the shape lies on at half or better is
// caught, and the caster — whose own cell the box starts in FRONT of — is not.
func (s *CoveredSuite) TestMembersCoveredByAnEdgeAnchoredBox() {
	out, err := s.enc.MembersCovered(&encounter.MembersCoveredInput{
		Footprint: spatial.Footprint{Box: &spatial.Box{W: 15, D: 15}},
		Anchor:    s.casterAt,
		Toward:    s.aheadAt,
		AtEdge:    true,
	})
	s.Require().NoError(err)

	ids := memberIDs(out.Members)
	s.Contains(ids, encounter.MemberID("ahead"))
	s.NotContains(ids, encounter.MemberID("caster"),
		"the caster is never under a box anchored on their own edge")
	s.NotContains(ids, encounter.MemberID("behind"), "the box is aimed the other way")
	s.NotContains(ids, encounter.MemberID("far-ahead"), "four cells out is past a 15-foot box")

	s.Equal(1.0, out.Cells[s.aheadAt], "the cell two straight ahead is wholly under the box")
	for cell, f := range out.Cells {
		s.GreaterOrEqual(f+1e-9, encounter.CoverageThreshold,
			"only cells at or above the threshold are reported: %v=%v", cell, f)
		s.LessOrEqual(f, 1.0, "a fraction of a cell is never more than the cell: %v=%v", cell, f)
	}
	s.Less(out.Cells[s.casterAt], encounter.CoverageThreshold,
		"whatever the box clips off the caster's own cell, it is not half of it")

	s.bisectedCellsAreAllCaught(out)
}

// bisectedCellsAreAllCaught is the reason the threshold comparison carries a
// tolerance, asserted rather than asserted-about.
//
// The box's long edges run through cells and cut them in half. Four of them,
// drawn on this axis — and the four do not agree with each other to the last
// bit: measured here, two come back a couple of ulp ABOVE 0.5, one lands on it
// exactly, and one sits at 0.49999999999999933. They are the same cut through
// four mirror-image cells, and which side of 0.5 each one lands on is which
// cosine it went through, nothing else.
//
// A bare `>= 0.5` would therefore catch three of those four and drop the
// fourth, which is a spell that is asymmetric for no reason a player could ever
// be told. So: every cell the raster puts within a whisker of half must be in
// the answer, and this asks the raster directly to find them.
func (s *CoveredSuite) bisectedCellsAreAllCaught(out encounter.MembersCoveredOutput) {
	canvas, err := s.enc.Canvas()
	s.Require().NoError(err)
	emb := spatial.NewHexEmbedding(spatial.HexEmbeddingConfig{
		Orientation: spatial.HexOrientationPointyTop, CellWidth: encounter.FeetPerCell,
	})
	facing, ok := emb.Bearing(s.casterAt, s.aheadAt)
	s.Require().True(ok)

	raw, err := spatial.Coverage(emb, canvas.GetGrid(), spatial.CoverageInput{
		Footprint: spatial.Footprint{Box: &spatial.Box{W: 15, D: 15}},
		At:        s.casterAt, Facing: facing, Anchor: spatial.AnchorAtEdge,
	})
	s.Require().NoError(err)

	var bisected int
	for cell, f := range raw.Cells {
		if math.Abs(f-encounter.CoverageThreshold) > 1e-9 {
			continue
		}
		bisected++
		s.Contains(out.Cells, cell,
			"cell %v is cut in half (%.17g) and must be caught like every other half", cell, f)
	}
	s.Positive(bisected, "this scene is about cells the box bisects, and it drew none")
}

// TestTheCoveredSetIsTheRostersOwnOrder.
//
// A producer that returned its members in map order would give two identical
// casts two different stories, and a save resolved in a different order is a
// different fight (C8).
func (s *CoveredSuite) TestTheCoveredSetIsTheRostersOwnOrder() {
	out, err := s.enc.MembersCovered(&encounter.MembersCoveredInput{
		Footprint: spatial.Footprint{Box: &spatial.Box{W: 25, D: 25}},
		Anchor:    s.casterAt,
		Toward:    s.aheadAt,
		AtEdge:    true,
	})
	s.Require().NoError(err)
	s.Require().NotEmpty(out.Members)

	roster, err := s.enc.Members()
	s.Require().NoError(err)
	caught := map[encounter.MemberID]bool{}
	for _, m := range out.Members {
		caught[m.ID] = true
	}
	var expected []encounter.MemberID
	for _, m := range roster {
		if caught[m.ID] {
			expected = append(expected, m.ID)
		}
	}
	s.Equal(expected, memberIDs(out.Members), "the covered set is the roster, filtered")
}

// TestACentredBoxCatchesTheCaster.
//
// The other anchor rule, and the reason AtEdge is a field rather than an
// assumption: a box centred on a cell covers that cell. No spell asks for it
// yet; the rule it proves is that WHERE the shape sits is content's to declare.
func (s *CoveredSuite) TestACentredBoxCatchesTheCaster() {
	out, err := s.enc.MembersCovered(&encounter.MembersCoveredInput{
		Footprint: spatial.Footprint{Box: &spatial.Box{W: 15, D: 15}},
		Anchor:    s.casterAt,
		Toward:    s.aheadAt,
		AtEdge:    false,
	})
	s.Require().NoError(err)
	s.Contains(memberIDs(out.Members), encounter.MemberID("caster"))
	s.Equal(1.0, out.Cells[s.casterAt], "a box centred on a cell covers it whole")
}

// TestTowardEqualToTheAnchorIsRefused.
//
// There is no bearing from a cell to itself, so there is no box to draw. An
// empty answer would read as a spell that caught nobody, which is a thing that
// happens — and this is not that.
func (s *CoveredSuite) TestTowardEqualToTheAnchorIsRefused() {
	_, err := s.enc.MembersCovered(&encounter.MembersCoveredInput{
		Footprint: spatial.Footprint{Box: &spatial.Box{W: 15, D: 15}},
		Anchor:    s.casterAt,
		Toward:    s.casterAt,
		AtEdge:    true,
	})
	s.ErrorIs(err, encounter.ErrBadReach)
}

// TestAMalformedFootprintIsRefusedByName.
//
// Content that authored a shape with no sides has a defect, and the refusal
// that says so is spatial's own — carried through rather than reworded, so a
// caller greps for one sentinel and finds it wherever the shape was wrong.
func (s *CoveredSuite) TestAMalformedFootprintIsRefusedByName() {
	_, err := s.enc.MembersCovered(&encounter.MembersCoveredInput{
		Anchor: s.casterAt, Toward: s.aheadAt, AtEdge: true,
	})
	s.ErrorIs(err, spatial.ErrNoFootprint)

	_, err = s.enc.MembersCovered(&encounter.MembersCoveredInput{
		Footprint: spatial.Footprint{Box: &spatial.Box{W: 0, D: 15}},
		Anchor:    s.casterAt, Toward: s.aheadAt, AtEdge: true,
	})
	s.ErrorIs(err, spatial.ErrBadFootprint)

	_, err = s.enc.MembersCovered(nil)
	s.ErrorIs(err, encounter.ErrNilInput)
}
