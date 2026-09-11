// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package encounter_test

import (
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter"
	"github.com/KirkDiggler/rpg-toolkit/tools/spatial"
)

// CellFactsTestSuite is about the fold and the step that reads it: one scene
// holding every kind of contributor a cell can have, asked one cell at a time.
//
// AUTHORED AT [10,6], NOT THE ORIGIN, for step_test.go's reason: where a scene
// sits at (0,0) its authored coordinates and its absolute ones are the same
// numbers, and a fold answering in the wrong frame would pass every assertion
// anyway.
//
//	col: 10 11 12 13 14
//	row=6: .  .  .  .  X   <- sealed by a wall
//	row=7: G  #  H  g  .   <- goblin, pillar, hero, second goblin, open
//	row=8: .  .  .  .  .
type CellFactsTestSuite struct {
	suite.Suite
	enc          *encounter.Encounter
	goblin       encounter.MemberID
	secondGoblin encounter.MemberID
	hero         encounter.MemberID
}

func (s *CellFactsTestSuite) SetupTest() {
	s.goblin = encounter.MemberID("goblin")
	s.secondGoblin = encounter.MemberID("second-goblin")
	s.hero = encounter.MemberID("hero")

	enc, err := encounter.NewEncounter(&encounter.SetupInput{
		Sight:     everyoneSeesTheWholeMap{},
		Equipment: noHandsAreObserved{}, Standing: everyoneStanding{}, Initiative: orderAsGiven{},
		TurnDriver: &scriptedDriver{}, Striker: &scriptedStriker{kind: encounter.OutcomeMissed},
		Mover: quietMover{}, Announcer: quietAnnouncer{},
		Field: encounter.FieldInput{
			Canvas:  pointyCanvas(),
			Regions: []encounter.RegionInput{rectRegion("hall", 10, 6, 5, 3)},
			Sealed:  []spatial.Position{{X: 14, Y: 6}},
			Props: []encounter.PropInput{{
				Ref:               "dnd5e:props:pillar",
				At:                spatial.Position{X: 11, Y: 7},
				BlocksMovement:    propTrue(),
				BlocksLineOfSight: propFalse(),
			}},
			Dispositions: []encounter.DispositionInput{{
				Between: [2]encounter.FactionID{encounter.FactionMonsters, encounter.FactionParty},
				Stance:  encounter.StanceHostile,
			}},
		},
		Members: []encounter.MemberInput{
			{ID: s.hero, Kind: encounter.KindPlayer, Position: spatial.Position{X: 12, Y: 7}},
			{ID: s.goblin, Kind: encounter.KindMonster, Position: spatial.Position{X: 10, Y: 7}},
			{ID: s.secondGoblin, Kind: encounter.KindMonster, Position: spatial.Position{X: 13, Y: 7}},
		},
		Endings: []encounter.EndingInput{{Key: "called", Trigger: encounter.TriggerExternal{}}},
	})
	s.Require().NoError(err)
	s.enc = enc
}

// --- The fold ---

func (s *CellFactsTestSuite) TestAnOpenFloorCellIsStandable() {
	got := s.enc.CellAt(encounter.CellAtInput{Cell: cellAt(14, 7), Mover: s.goblin})
	s.Equal(encounter.PassageStandable, got.Passage)
	s.Equal(1, got.Cost)
	s.Empty(got.Contribs, "nothing stands on open floor, so nobody contributed a fact")
}

func (s *CellFactsTestSuite) TestAPillarBlocks() {
	got := s.enc.CellAt(encounter.CellAtInput{Cell: cellAt(11, 7), Mover: s.goblin})
	s.Equal(encounter.PassageBlocked, got.Passage)
	s.Contains(got.Contribs, encounter.ContribRef{
		Kind: encounter.ContribProp, ID: "prop-0", Ref: "dnd5e:props:pillar", Blocks: true,
	}, "the fold names the prop that refused, by placement AND by what it is")
}

func (s *CellFactsTestSuite) TestAHostileCreatureBlocksAndAnAllyIsPassedThrough() {
	hostile := s.enc.CellAt(encounter.CellAtInput{Cell: cellAt(12, 7), Mover: s.goblin})
	s.Equal(encounter.PassageBlocked, hostile.Passage,
		"2014: a hostile creature's space is not yours to enter")
	s.Contains(hostile.Contribs, encounter.ContribRef{
		Kind: encounter.ContribMember, ID: string(s.hero), Blocks: true,
	})

	ally := s.enc.CellAt(encounter.CellAtInput{Cell: cellAt(13, 7), Mover: s.goblin})
	s.Equal(encounter.PassagePassThrough, ally.Passage,
		"2014: you may move through a nonhostile creature's space but not stop there")
	s.Contains(ally.Contribs, encounter.ContribRef{
		Kind: encounter.ContribMember, ID: string(s.secondGoblin), Blocks: false,
	}, "an ally contributes a fact about the cell without being the reason it is closed")
}

func (s *CellFactsTestSuite) TestAMoverMayReEnterItsOwnCell() {
	got := s.enc.CellAt(encounter.CellAtInput{Cell: cellAt(10, 7), Mover: s.goblin})
	s.Equal(encounter.PassageStandable, got.Passage,
		"a mover standing here is not a reason it may not be here")
	s.Empty(got.Contribs)
}

func (s *CellFactsTestSuite) TestASealedCellIsBlockedByTheField() {
	got := s.enc.CellAt(encounter.CellAtInput{Cell: cellAt(14, 6), Mover: s.goblin})
	s.Equal(encounter.PassageBlocked, got.Passage)
	s.Contains(got.Contribs, encounter.ContribRef{Kind: encounter.ContribField, Blocks: true})
}

func (s *CellFactsTestSuite) TestACellNoRegionOwnsIsBlockedByTheField() {
	got := s.enc.CellAt(encounter.CellAtInput{Cell: cellAt(0, 0), Mover: s.goblin})
	s.Equal(encounter.PassageBlocked, got.Passage)
	s.Contains(got.Contribs, encounter.ContribRef{Kind: encounter.ContribField, Blocks: true})
}

func (s *CellFactsTestSuite) TestTheSameCellAnswersDifferentlyForDifferentMovers() {
	// The second goblin's cell: an ally to the first goblin, an enemy to the
	// hero. A cell is not passable or impassable on its own, which is why the
	// fold takes a mover rather than describing the map.
	s.Equal(encounter.PassagePassThrough,
		s.enc.CellAt(encounter.CellAtInput{Cell: cellAt(13, 7), Mover: s.goblin}).Passage)
	s.Equal(encounter.PassageBlocked,
		s.enc.CellAt(encounter.CellAtInput{Cell: cellAt(13, 7), Mover: s.hero}).Passage)
}

// --- The step reads the same fold, and says which contributor refused ---

func (s *CellFactsTestSuite) TestAStepOntoAPillarIsRefusedByTheProp() {
	_, err := s.enc.Step(&encounter.StepInput{Member: s.hero, To: cellAt(11, 7)})
	s.Require().Error(err)
	s.ErrorIs(err, encounter.ErrBadPlacement)
	s.ErrorContains(err, "dnd5e:props:pillar",
		"the refusal names WHAT stopped the step, not merely that something did")
}

func (s *CellFactsTestSuite) TestAStepOntoAHostileCreatureIsRefusedByName() {
	// THE ONE BEHAVIOUR CHANGE (#1652): before the fold, nothing on the step
	// path consulted stance, so a player walked onto an enemy's cell as long
	// as the enemy had not set BlocksMovement. 2014 says a hostile creature's
	// space is closed.
	_, err := s.enc.Step(&encounter.StepInput{Member: s.hero, To: cellAt(13, 7)})
	s.Require().Error(err)
	s.ErrorIs(err, encounter.ErrBadPlacement)
	s.ErrorContains(err, string(s.secondGoblin))
}

func (s *CellFactsTestSuite) TestAStepOntoOpenFloorStillSucceeds() {
	out, err := s.enc.Step(&encounter.StepInput{Member: s.hero, To: cellAt(12, 6)})
	s.Require().NoError(err)
	s.Equal(cellAt(12, 6), out.Stepped.To)
}

func TestCellFactsSuite(t *testing.T) { suite.Run(t, new(CellFactsTestSuite)) }
