// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package session

import (
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"

	combatActions "github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/combat/actions"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter"
	"github.com/KirkDiggler/rpg-toolkit/tools/spatial"
)

// area_internal_test.go is about deriveAreaMembers: turning a shape the CONTENT
// declared into the members this build can resolve against, plus the ones it
// caught and cannot.
//
// Members are placed by AUTHORED OFFSET and the scene asserts where they landed
// before asserting anything else. MemberInput.Position takes offset and the
// composition reports absolute axial, so a scene that skipped that check would
// pass or fail for reasons unrelated to the code under test.

type AreaDeriveSuite struct {
	suite.Suite
	enc *encounter.Encounter

	bardAt, closeAt, farAt spatial.Position
}

func TestAreaDeriveSuite(t *testing.T) {
	suite.Run(t, new(AreaDeriveSuite))
}

const (
	areaBard   = "bard"
	areaClose  = "skeleton"
	areaFar    = "ally"
	areaVendor = "vendor"
)

func (s *AreaDeriveSuite) SetupTest() {
	const row = 5
	offset := func(col int) spatial.Position {
		return spatial.Position{X: float64(col), Y: float64(row)}
	}

	enc, err := encounter.NewEncounter(&encounter.SetupInput{
		Striker: encounter.RefusingStriker{}, Mover: encounter.RefusingMover{},
		Announcer: encQuietAnnouncer{}, Sight: &sightSeam{}, Equipment: encNoHandsObserved{},
		Initiative: walkOrderAsGiven{}, TurnDriver: passDriver{}, Standing: walkEveryoneStanding{},
		Field: encounter.FieldInput{
			Canvas:  pointyCanvas(),
			Regions: []encounter.RegionInput{rectRegion("yard", 0, 0, 14, 14)},
		},
		Members: []encounter.MemberInput{
			{ID: areaBard, Kind: encounter.KindPlayer, Position: offset(5)},
			{ID: areaClose, Kind: encounter.KindMonster, Position: offset(6)},
			{ID: areaFar, Kind: encounter.KindPlayer, Position: offset(8)},
			{ID: areaVendor, Kind: encounter.MemberKind(KindWorld), Position: offset(6)},
		},
		Endings: []encounter.EndingInput{{Key: "done", Trigger: encounter.TriggerExternal{}}},
	})
	s.Require().NoError(err)
	s.enc = enc

	placed := map[encounter.MemberID]spatial.Position{}
	for _, m := range s.roster() {
		placed[m.ID] = m.Position
	}
	s.bardAt, s.closeAt, s.farAt = placed[areaBard], placed[areaClose], placed[areaFar]

	// The scene's own assumptions, before anything depends on them.
	s.Require().Equal(1.0, enc.Distance(s.bardAt, s.closeAt), "the skeleton is one cell from the bard")
	s.Require().Equal(3.0, enc.Distance(s.bardAt, s.farAt), "the ally is three cells away")
	s.Require().Equal(s.closeAt, placed[areaVendor], "the vendor shares the skeleton's cell")
}

func (s *AreaDeriveSuite) roster() []encounter.Member {
	roster, err := s.enc.Members()
	s.Require().NoError(err)
	return roster
}

func areaProfile(sizeFeet int, catches combatActions.AreaCatches) *combatActions.CastProfile {
	return &combatActions.CastProfile{
		RangeFeet: sizeFeet,
		Target:    combatActions.CastTargetArea,
		Area: &combatActions.CastArea{
			Footprint: combatActions.Footprint{
				Shape: combatActions.AreaRadius, SizeFeet: sizeFeet, Origin: combatActions.AreaOriginCaster,
			},
			Catches: catches,
		},
	}
}

func (s *AreaDeriveSuite) derive(profile *combatActions.CastProfile) *areaCaught {
	caught, err := deriveAreaMembers(s.enc, profile, areaBard, s.roster(), nil)
	s.Require().NoError(err)
	return caught
}

// TestTheSpellsOwnProjectionDecidesWhetherTheCasterIsCaught.
//
// The composition answers who is STANDING in the shape and has no idea why it
// was asked — the caster is in their own burst and comes back. "Each creature
// other than you" is the content's sentence, and it is applied here.
func (s *AreaDeriveSuite) TestTheSpellsOwnProjectionDecidesWhetherTheCasterIsCaught() {
	s.Run("others spares the caster", func() {
		caught := s.derive(areaProfile(5, combatActions.AreaCatchesOthers))
		s.ElementsMatch([]string{areaClose}, caught.resolvable)
		s.NotContains(caught.resolvable, areaBard)
	})
	s.Run("everyone does not", func() {
		caught := s.derive(areaProfile(5, combatActions.AreaCatchesEveryone))
		s.ElementsMatch([]string{areaBard, areaClose}, caught.resolvable)
	})
}

// TestReachIsTheSpellsOwnSizeInCells — five feet is one cell, so the ally three
// cells out is not caught. Fifteen feet reaches them.
func (s *AreaDeriveSuite) TestReachIsTheSpellsOwnSizeInCells() {
	s.ElementsMatch([]string{areaClose},
		s.derive(areaProfile(5, combatActions.AreaCatchesOthers)).resolvable)
	s.ElementsMatch([]string{areaClose, areaFar},
		s.derive(areaProfile(15, combatActions.AreaCatchesOthers)).resolvable)
}

// TestTheShopkeeperIsCaughtAndReported is the claim this slice exists to make
// honest.
//
// A placed world member has no sheet, so compileResolutionCast skips it and
// resolution could never resolve against it. Handing it over as a recipient
// would be refused mid-run, after the door had already charged. Dropping it
// would make "nobody was standing there" and "somebody was standing there and
// we have nothing to do about it" the same answer.
func (s *AreaDeriveSuite) TestTheShopkeeperIsCaughtAndReported() {
	caught := s.derive(areaProfile(5, combatActions.AreaCatchesOthers))

	s.NotContains(caught.resolvable, areaVendor, "a world member cannot be a recipient")
	s.Require().Len(caught.unresolved, 1)
	s.Equal(areaVendor, caught.unresolved[0].Member)
	s.Equal(KindWorld, caught.unresolved[0].Kind)
	s.Equal(UnresolvedNoSheet, caught.unresolved[0].Reason)
}

// TestAFootprintThatCannotReachIsRefusedHere.
//
// A one-to-four-foot radius floors to zero cells and could only ever catch
// something on the caster's own square. CastProfile.Validate cannot catch it —
// the rulebook module cannot import the feet-to-cells conversion — so this is
// the first layer that can measure, and it refuses rather than resolving a
// spell that does nothing forever.
func (s *AreaDeriveSuite) TestAFootprintThatCannotReachIsRefusedHere() {
	_, err := deriveAreaMembers(s.enc, areaProfile(4, combatActions.AreaCatchesOthers), areaBard, s.roster(), nil)
	s.Require().Error(err)
	s.ErrorIs(err, ErrBadCast)
	s.Contains(err.Error(), "4 feet")
	s.Contains(err.Error(), "5-foot cell", "the refusal names the scale it measured against")
}

// TestAnEmptyFootprintIsAnOrdinaryAnswer — a cast that catches nobody still
// happened, and encounter's RecordCastInput accepts empty targets for exactly
// this case.
func (s *AreaDeriveSuite) TestAnEmptyFootprintIsAnOrdinaryAnswer() {
	// The ally, alone at three cells out, catching nothing within one.
	caught, err := deriveAreaMembers(s.enc, areaProfile(5, combatActions.AreaCatchesOthers), areaFar, s.roster(), nil)
	s.Require().NoError(err)
	s.Empty(caught.resolvable)
	s.Empty(caught.unresolved)
}

// TestACasterNobodyPlacedIsRefused — the origin comes off the roster, so a
// caster the composition never placed has no footprint to project. Fails closed
// rather than defaulting to the origin cell.
func (s *AreaDeriveSuite) TestACasterNobodyPlacedIsRefused() {
	_, err := deriveAreaMembers(s.enc, areaProfile(5, combatActions.AreaCatchesOthers), "ghost", s.roster(), nil)
	s.Require().Error(err)
	s.ErrorIs(err, ErrBadCast)
}

// TestAProfileWithNoShapeIsRefused. CastProfile.Validate binds the target rule
// and the shape together, so reaching here without one is content that never
// went through it.
func (s *AreaDeriveSuite) TestAProfileWithNoShapeIsRefused() {
	profile := areaProfile(5, combatActions.AreaCatchesOthers)
	profile.Area = nil
	_, err := deriveAreaMembers(s.enc, profile, areaBard, s.roster(), nil)
	require.Error(s.T(), err)
	s.ErrorIs(err, ErrBadCast)
}

// AreaCoveredSuite is the caster-edge box: a shape with a DIRECTION, which the
// radius arm above has never needed.
//
// Its own scene rather than the one above, because the members this needs —
// somebody behind the caster, somebody past the far edge — would change what
// every radius test catches. A fixture shared past the point of honesty is how
// a test starts passing for a reason nobody wrote down.
type AreaCoveredSuite struct {
	suite.Suite
	enc *encounter.Encounter

	casterAt, aheadAt, behindAt, farAt spatial.Position
}

func TestAreaCoveredSuite(t *testing.T) {
	suite.Run(t, new(AreaCoveredSuite))
}

const (
	coveredCaster = "bard"
	coveredAhead  = "skeleton"
	coveredBehind = "ally"
	coveredFar    = "wolf"
)

func (s *AreaCoveredSuite) SetupTest() {
	const row = 5
	offset := func(col int) spatial.Position {
		return spatial.Position{X: float64(col), Y: float64(row)}
	}

	enc, err := encounter.NewEncounter(&encounter.SetupInput{
		Striker: encounter.RefusingStriker{}, Mover: encounter.RefusingMover{},
		Announcer: encQuietAnnouncer{}, Sight: &sightSeam{}, Equipment: encNoHandsObserved{},
		Initiative: walkOrderAsGiven{}, TurnDriver: passDriver{}, Standing: walkEveryoneStanding{},
		Field: encounter.FieldInput{
			Canvas:  pointyCanvas(),
			Regions: []encounter.RegionInput{rectRegion("yard", 0, 0, 14, 14)},
		},
		Members: []encounter.MemberInput{
			{ID: coveredCaster, Kind: encounter.KindPlayer, Position: offset(5)},
			{ID: coveredAhead, Kind: encounter.KindMonster, Position: offset(6)},
			{ID: coveredBehind, Kind: encounter.KindPlayer, Position: offset(4)},
			{ID: coveredFar, Kind: encounter.KindMonster, Position: offset(9)},
		},
		Endings: []encounter.EndingInput{{Key: "done", Trigger: encounter.TriggerExternal{}}},
	})
	s.Require().NoError(err)
	s.enc = enc

	placed := map[encounter.MemberID]spatial.Position{}
	for _, m := range s.roster() {
		placed[m.ID] = m.Position
	}
	s.casterAt = placed[coveredCaster]
	s.aheadAt = placed[coveredAhead]
	s.behindAt = placed[coveredBehind]
	s.farAt = placed[coveredFar]

	// The scene's own assumptions, before anything depends on them.
	s.Require().Equal(1.0, enc.Distance(s.casterAt, s.aheadAt), "the skeleton is one cell ahead")
	s.Require().Equal(1.0, enc.Distance(s.casterAt, s.behindAt), "the ally is one cell behind")
	s.Require().Equal(4.0, enc.Distance(s.casterAt, s.farAt), "the wolf is four cells out")
}

func (s *AreaCoveredSuite) roster() []encounter.Member {
	roster, err := s.enc.Members()
	s.Require().NoError(err)
	return roster
}

// coveredProfile is Thunderwave's shape: a box of sizeFeet on a side, anchored
// on the caster's own edge, catching everyone but them.
func coveredProfile(sizeFeet int) *combatActions.CastProfile {
	return &combatActions.CastProfile{
		RangeFeet: sizeFeet,
		Target:    combatActions.CastTargetArea,
		Area: &combatActions.CastArea{
			Footprint: combatActions.Footprint{
				Shape: combatActions.AreaBox, SizeFeet: sizeFeet,
				Origin: combatActions.AreaOriginCasterEdge,
			},
			Catches: combatActions.AreaCatchesOthers,
		},
	}
}

// TestTheCubeCatchesWhatItIsPointedAt is the whole of what the cell buys.
//
// The same spell, cast by the same bard standing in the same cell, catches a
// different creature depending only on which way it was aimed — and the caster,
// whose own cell an edge-anchored box never covers, is caught by neither.
func (s *AreaCoveredSuite) TestTheCubeCatchesWhatItIsPointedAt() {
	s.Run("aimed at the skeleton", func() {
		caught, err := deriveAreaMembers(
			s.enc, coveredProfile(15), coveredCaster, s.roster(), &s.aheadAt)
		s.Require().NoError(err)
		s.Contains(caught.resolvable, coveredAhead)
		s.NotContains(caught.resolvable, coveredBehind, "the cube hangs off one edge, not both")
		s.NotContains(caught.resolvable, coveredCaster)
		s.NotContains(caught.resolvable, coveredFar, "four cells out is past a fifteen-foot box")
	})

	s.Run("aimed at the ally", func() {
		caught, err := deriveAreaMembers(
			s.enc, coveredProfile(15), coveredCaster, s.roster(), &s.behindAt)
		s.Require().NoError(err)
		s.Contains(caught.resolvable, coveredBehind)
		s.NotContains(caught.resolvable, coveredAhead)
	})
}

// TestACubeWithNoCellIsRefused. The door validates the cell against the offer,
// so reaching here without one is a caller that went around it — and a cube
// given a direction here would be a rule invented at the layer that measures.
func (s *AreaCoveredSuite) TestACubeWithNoCellIsRefused() {
	_, err := deriveAreaMembers(s.enc, coveredProfile(15), coveredCaster, s.roster(), nil)
	s.Require().Error(err)
	s.ErrorIs(err, ErrBadCast)
	s.Contains(err.Error(), "cell")
}
