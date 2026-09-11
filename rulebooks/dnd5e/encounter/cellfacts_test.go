// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package encounter_test

import (
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter"
	"github.com/KirkDiggler/rpg-toolkit/tools/spatial"
)

// CellFactsTestSuite is about the fold: one scene holding every kind of
// contributor a cell can have, asked one cell at a time.
//
//	x:  0  1  2  3  4
//	y=0: .  .  .  .  X   <- sealed by a wall
//	y=1: .  .  .  .  .
//	y=2: G  g  #  .  H   <- goblin, second goblin, pillar, hero
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
			Regions: []encounter.RegionInput{rectRegion("hall", 0, 0, 5, 3)},
			Sealed:  []spatial.Position{{X: 4, Y: 0}},
			Props: []encounter.PropInput{{
				Ref:               "dnd5e:props:pillar",
				At:                spatial.Position{X: 2, Y: 2},
				BlocksMovement:    propTrue(),
				BlocksLineOfSight: propFalse(),
			}},
			Dispositions: []encounter.DispositionInput{{
				Between: [2]encounter.FactionID{encounter.FactionMonsters, encounter.FactionParty},
				Stance:  encounter.StanceHostile,
			}},
		},
		Members: []encounter.MemberInput{
			{ID: s.hero, Kind: encounter.KindPlayer, Position: spatial.Position{X: 4, Y: 2}},
			{ID: s.goblin, Kind: encounter.KindMonster, Position: spatial.Position{X: 0, Y: 2}},
			{ID: s.secondGoblin, Kind: encounter.KindMonster, Position: spatial.Position{X: 1, Y: 2}},
		},
		Endings: []encounter.EndingInput{{Key: "called", Trigger: encounter.TriggerExternal{}}},
	})
	s.Require().NoError(err)
	s.enc = enc
}

func (s *CellFactsTestSuite) TestAnOpenFloorCellIsStandable() {
	got := s.enc.CellAt(encounter.CellAtInput{Cell: cellAt(1, 1), Mover: s.goblin})
	s.Equal(encounter.PassageStandable, got.Passage)
	s.Equal(1, got.Cost)
	s.Empty(got.Contribs, "nothing stands on open floor, so nobody contributed a fact")
}

func (s *CellFactsTestSuite) TestAPillarBlocks() {
	got := s.enc.CellAt(encounter.CellAtInput{Cell: cellAt(2, 2), Mover: s.goblin})
	s.Equal(encounter.PassageBlocked, got.Passage)
	s.Contains(got.Contribs, encounter.ContribRef{
		Kind: encounter.ContribProp, ID: "prop-0", Ref: "dnd5e:props:pillar",
	}, "the fold names the prop that refused, by placement AND by what it is")
}

func (s *CellFactsTestSuite) TestAHostileCreatureBlocksAndAnAllyIsPassedThrough() {
	hostile := s.enc.CellAt(encounter.CellAtInput{Cell: cellAt(4, 2), Mover: s.goblin})
	s.Equal(encounter.PassageBlocked, hostile.Passage,
		"2014: a hostile creature's space is not yours to enter")
	s.Contains(hostile.Contribs, encounter.ContribRef{Kind: encounter.ContribMember, ID: string(s.hero)})

	ally := s.enc.CellAt(encounter.CellAtInput{Cell: cellAt(1, 2), Mover: s.goblin})
	s.Equal(encounter.PassagePassThrough, ally.Passage,
		"2014: you may move through a nonhostile creature's space but not stop there")
	s.Contains(ally.Contribs, encounter.ContribRef{Kind: encounter.ContribMember, ID: string(s.secondGoblin)})
}

func (s *CellFactsTestSuite) TestAMoverMayReEnterItsOwnCell() {
	got := s.enc.CellAt(encounter.CellAtInput{Cell: cellAt(0, 2), Mover: s.goblin})
	s.Equal(encounter.PassageStandable, got.Passage,
		"a mover standing here is not a reason it may not be here")
	s.Empty(got.Contribs)
}

func (s *CellFactsTestSuite) TestASealedCellIsBlockedByTheField() {
	got := s.enc.CellAt(encounter.CellAtInput{Cell: cellAt(4, 0), Mover: s.goblin})
	s.Equal(encounter.PassageBlocked, got.Passage)
	s.Contains(got.Contribs, encounter.ContribRef{Kind: encounter.ContribField})
}

func (s *CellFactsTestSuite) TestACellNoRegionOwnsIsBlockedByTheField() {
	got := s.enc.CellAt(encounter.CellAtInput{Cell: cellAt(9, 9), Mover: s.goblin})
	s.Equal(encounter.PassageBlocked, got.Passage)
	s.Contains(got.Contribs, encounter.ContribRef{Kind: encounter.ContribField})
}

func (s *CellFactsTestSuite) TestTheSameCellAnswersDifferentlyForDifferentMovers() {
	// The hero's own reading of the second goblin: hostile, so blocked —
	// the same cell answers differently for a different mover, which is
	// what makes this a fold over a MOVER rather than a property of a cell.
	got := s.enc.CellAt(encounter.CellAtInput{Cell: cellAt(1, 2), Mover: s.hero})
	s.Equal(encounter.PassageBlocked, got.Passage)
}

func TestCellFactsSuite(t *testing.T) { suite.Run(t, new(CellFactsTestSuite)) }
