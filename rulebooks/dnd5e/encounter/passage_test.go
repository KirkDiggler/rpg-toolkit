// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package encounter_test

import (
	"context"
	"errors"
	"fmt"
	"github.com/KirkDiggler/rpg-toolkit/core"
	"testing"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter"
	"github.com/KirkDiggler/rpg-toolkit/tools/spatial"
	"github.com/stretchr/testify/suite"
)

// passageAssessment makes recovery and provider outages observable without
// mutating persisted encounter data or deriving life state from initiative.
type passageAssessment struct {
	everyoneStanding
	down   map[encounter.MemberID]bool
	err    error
	calls  int
	failAt int
}

func (p *passageAssessment) Assess(ids []encounter.MemberID) (*encounter.ParticipationAssessment, error) {
	p.calls++
	if p.err != nil && (p.failAt == 0 || p.calls >= p.failAt) {
		return nil, p.err
	}
	out, err := p.everyoneStanding.Assess(ids)
	if err != nil {
		return nil, err
	}
	for i := range out.Members {
		if p.down[out.Members[i].Member] {
			out.Members[i].Down = true
			out.Members[i].Contact = false
			out.Members[i].Conscious = false
			out.Members[i].Turn = encounter.TurnParticipationRemove
		}
	}
	return out, nil
}

type PassageSuite struct {
	suite.Suite
	enc  *encounter.Encounter
	life *passageAssessment
}

func TestPassageSuite(t *testing.T) { suite.Run(t, new(PassageSuite)) }

func (s *PassageSuite) TestPreviewDoesNotReadUnobservedParticipationChanges() {
	before, err := s.enc.ObservedPassages(&encounter.ViewInput{Member: alice})
	s.Require().NoError(err)
	s.Equal(encounter.PassageBlocked, before[goblin])
	s.life.down[goblin] = true
	s.life.err = fmt.Errorf("live participation must not be queried by preview")
	after, err := s.enc.ObservedPassages(&encounter.ViewInput{Member: alice})
	s.Require().NoError(err)
	s.Equal(before, after)
}

func (s *PassageSuite) SetupTest() {
	s.life = &passageAssessment{down: map[encounter.MemberID]bool{}}
	enc, err := encounter.NewEncounter(&encounter.SetupInput{
		Sight: everyoneSeesTheWholeMap{}, Equipment: encounter.UnobservedEquipment{}, Standing: s.life,
		Initiative: orderAsGiven{}, TurnDriver: passDriver{}, Striker: passStriker{},
		Mover: quietMover{}, Announcer: quietAnnouncer{},
		Field: encounter.FieldInput{Canvas: pointyCanvas(), Regions: []encounter.RegionInput{rectRegion("corridor", 0, 0, 4, 1)}},
		Members: []encounter.MemberInput{
			{ID: alice, Kind: encounter.KindPlayer, Position: cellAt(0, 0)},
			{ID: goblin, Kind: encounter.KindMonster, Position: cellAt(1, 0)},
		},
		Endings: []encounter.EndingInput{{Key: "withdrawn", Trigger: encounter.TriggerExternal{}}},
	})
	s.Require().NoError(err)
	s.enc = enc
}

func (s *PassageSuite) TestDownedMonsterCanBeCrossedAndStoodOnThenRecoveryBlocksAgain() {
	in := encounter.CellAtInput{Cell: cellAt(1, 0), Mover: alice}
	s.Equal(encounter.PassageBlocked, cellFactFor(s.T(), s.enc, in).Passage)
	s.life.down[goblin] = true
	s.Equal(encounter.PassageStandable, cellFactFor(s.T(), s.enc, in).Passage)
	route, err := s.enc.Route(encounter.RouteInput{Mover: alice, Policy: encounter.MoveToward, Anchor: cellAt(3, 0), Budget: 3})
	s.Require().NoError(err)
	s.Equal([]spatial.Position{cellAt(1, 0), cellAt(2, 0)}, route.Path)
	delete(s.life.down, goblin)
	s.Equal(encounter.PassageBlocked, cellFactFor(s.T(), s.enc, in).Passage)
	route, err = s.enc.Route(encounter.RouteInput{Mover: alice, Policy: encounter.MoveToward, Anchor: cellAt(3, 0), Budget: 3})
	s.Require().NoError(err)
	s.Empty(route.Path, "a one-cell corridor cannot route around the recovered hostile")
}

func (s *PassageSuite) TestStepCanLandOnDownedMonster() {
	s.life.down[goblin] = true
	out, err := s.enc.Step(&encounter.StepInput{Member: alice, To: cellAt(1, 0)})
	s.Require().NoError(err)
	s.Equal(cellAt(1, 0), out.Stepped.To)
}

func (s *PassageSuite) TestDownedMonsterCannotOpenAnotherBlockingOccupant() {
	s.life.down[goblin] = true
	_, err := s.enc.Join(&encounter.JoinInput{Member: "vendor", Kind: encounter.KindWorld, Cell: cellAt(1, 0), BlocksMovement: true})
	s.Require().NoError(err)
	fact := cellFactFor(s.T(), s.enc, encounter.CellAtInput{Cell: cellAt(1, 0), Mover: alice})
	s.Equal(encounter.PassageBlocked, fact.Passage)
	_, err = s.enc.Step(&encounter.StepInput{Member: alice, To: cellAt(1, 0)})
	s.Require().ErrorIs(err, encounter.ErrBadPlacement)
}

func (s *PassageSuite) TestReadStepAndRoutePreserveAssessmentFailure() {
	failure := errors.New("participation repository unavailable")
	s.life.err = failure
	_, err := s.enc.CellAt(encounter.CellAtInput{Cell: cellAt(1, 0), Mover: alice})
	s.Require().ErrorIs(err, failure)
	_, err = s.enc.Step(&encounter.StepInput{Member: alice, To: cellAt(1, 0)})
	s.Require().ErrorIs(err, failure)
	for _, policy := range []encounter.MovePolicy{encounter.MoveToward, encounter.MoveAway, encounter.MoveLine, encounter.MovePull} {
		_, err = s.enc.Route(encounter.RouteInput{Mover: alice, Policy: policy, Anchor: cellAt(3, 0), Budget: 3})
		s.Require().ErrorIs(err, failure)
	}
	members, err := s.enc.Members()
	s.Require().NoError(err)
	for _, m := range members {
		if m.ID == alice {
			s.Equal(cellAt(0, 0), m.Position)
		}
	}
}

func (s *PassageSuite) TestRouteAssessesOnceAndNextRouteAsksAgain() {
	s.life.down[goblin] = true
	s.life.calls = 0
	in := encounter.RouteInput{Mover: alice, Policy: encounter.MoveToward, Anchor: cellAt(3, 0), Budget: 3}
	out, err := s.enc.Route(in)
	s.Require().NoError(err)
	s.NotEmpty(out.Path)
	s.Equal(1, s.life.calls, "one assessment for the whole search, not one per visited cell")
	delete(s.life.down, goblin)
	out, err = s.enc.Route(in)
	s.Require().NoError(err)
	s.Empty(out.Path)
	s.Equal(2, s.life.calls, "the query must not cache life state across calls")
}

func (s *PassageSuite) TestReloadUsesCurrentParticipationRatherThanStoredDownFlag() {
	s.life.down[goblin] = true
	loaded, err := encounter.LoadEncounter(&encounter.LoadEncounterInput{
		Data: s.enc.ToData(), Sight: everyoneSeesTheWholeMap{}, Equipment: encounter.UnobservedEquipment{}, Standing: s.life,
		Initiative: orderAsGiven{}, TurnDriver: passDriver{}, Striker: passStriker{}, Mover: quietMover{}, Announcer: quietAnnouncer{},
	})
	s.Require().NoError(err)
	in := encounter.CellAtInput{Cell: cellAt(1, 0), Mover: alice}
	s.Equal(encounter.PassageStandable, cellFactFor(s.T(), loaded, in).Passage)
	delete(s.life.down, goblin)
	s.Equal(encounter.PassageBlocked, cellFactFor(s.T(), loaded, in).Passage)
}

func (s *PassageSuite) TestDownedPlayerKeepsExistingOccupancyPolicy() {
	s.life.down[alice] = true
	fact := cellFactFor(s.T(), s.enc, encounter.CellAtInput{Cell: cellAt(0, 0), Mover: goblin})
	s.Equal(encounter.PassageBlocked, fact.Passage)
}

func (s *PassageSuite) TestDrivenStepDoesNotSwallowProviderErrorWithPlacementSentinel() {
	s.life.down[goblin] = true
	// Direct consults the mover first; fail inside the step's occupancy query.
	s.life.calls = 0
	s.life.failAt = 2
	failure := fmt.Errorf("provider placement lookup failed: %w", encounter.ErrBadPlacement)
	s.life.err = failure
	_, err := s.enc.Direct(context.Background(), encounter.DirectInput{
		Mover: alice, Route: []spatial.Position{cellAt(1, 0)},
		Cause: core.Ref{Module: "test", Type: "effect", ID: "push"},
	})
	s.Require().ErrorIs(err, failure)
	s.Equal(2, s.life.calls, "the probe must fail at the step, not its earlier down check")
}

func (s *PassageSuite) TestRecordedFallUpdatesWitnessPassageWithoutMovement() {
	s.life.down[goblin] = true
	_, err := s.enc.Record(&encounter.RecordInput{
		Kind: encounter.OutcomeStruck, Actor: alice, Targets: []encounter.MemberID{goblin},
		Values:      map[encounter.OutcomeValue]int{encounter.ValueRoll: 17, encounter.ValueTotal: 22, encounter.ValueAgainst: 15, encounter.ValueAmount: 9},
		Calculation: attackCalculation(17, 5, 0),
	})
	s.Require().NoError(err)
	passages, err := s.enc.ObservedPassages(&encounter.ViewInput{Member: alice})
	s.Require().NoError(err)
	s.Equal(encounter.PassageStandable, passages[goblin])
}

func (s *PassageSuite) TestObstructionPublicReasonNeverNamesItsOccupant() {
	_, err := s.enc.Step(&encounter.StepInput{Member: alice, To: cellAt(1, 0)})
	var obstruction *encounter.StepObstructedError
	s.Require().ErrorAs(err, &obstruction)
	s.NotContains(obstruction.PublicReason(), string(goblin))
	s.ErrorIs(err, encounter.ErrBadPlacement)
}
