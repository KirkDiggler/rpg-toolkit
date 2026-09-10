// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package resolution

import (
	"context"
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/KirkDiggler/rpg-toolkit/dice"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/character"
	combatActions "github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/combat/actions"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/spells"
)

// AreaActionTestSuite covers the cast arm whose recipients the ENGINE derived
// from a declared footprint, rather than a caller naming them.
type AreaActionTestSuite struct {
	suite.Suite
	ctx context.Context
}

func TestAreaActionTestSuite(t *testing.T) {
	suite.Run(t, new(AreaActionTestSuite))
}

func (s *AreaActionTestSuite) SetupTest() { s.ctx = context.Background() }

func thunderclapDefinition() *combatActions.Definition {
	return spells.CastDefinition(spells.CastDefinitionInput{
		Spell: spells.Thunderclap, SpellSaveDC: spellSaveDC,
	})
}

// resolveArea drives one area cast against a world the caller chooses, so a
// scene can put a derived member anywhere it likes.
func (s *AreaActionTestSuite) resolveArea(
	world encounter.EncounterData, machine Machine, saver, payer *character.Data,
) (*Output, error) {
	fixtures := &ContestDamageTestSuite{}
	fixtures.SetT(s.T())
	participants := []Participant{{Character: saver}, {Monster: fixtures.wolfData()}}
	if payer != nil {
		participants = append(participants, Participant{Character: payer})
	}
	return Resolve(s.ctx, &Input{
		Initiative: orderAsGiven{}, TurnDriver: passDriver{}, Standing: everyoneStanding{},
		Sight: everyoneSeesTheWholeMap{}, Equipment: noHandsAreObserved{}, Roller: dice.NewRoller(),
		World: world, Participants: participants, Machine: machine, Cost: castCost(),
	})
}

// TestAnAreaCastResolvesTheMembersItWasHanded is the arm itself: the caller
// names nobody, hands over what the engine derived, and those are the
// recipients.
func (s *AreaActionTestSuite) TestAnAreaCastResolvesTheMembersItWasHanded() {
	fixtures := &ContestDamageTestSuite{}
	fixtures.SetT(s.T())

	machine, err := NewAction(&ActionInput{
		Definition: *thunderclapDefinition(), AttackerID: bardID,
		AreaMembers: []string{heroID}, Roller: &facedRoller{d20: 1, other: 4},
	})
	s.Require().NoError(err)

	out, err := s.resolveArea(baneWorld(s.T(), 2), machine, fixtures.saver(14), baneCaster(1, 2))
	s.Require().NoError(err)

	outcome, ok := out.Outcome.(CastOutcome)
	s.Require().True(ok, "a cast produces a CastOutcome")
	s.Require().Len(outcome.Targets, 1)
	s.Equal(heroID, outcome.Targets[0].TargetID)
}

// TestACallerMayNotNameTargetsForAnAreaCast — MinTargets/MaxTargets bound what
// the CALLER may name and an area profile declares zero of each, so a populated
// TargetIDs is a caller claiming to decide something it does not decide. The
// derived list travels in its own field precisely so this gate keeps meaning
// what it always meant.
func (s *AreaActionTestSuite) TestACallerMayNotNameTargetsForAnAreaCast() {
	_, err := NewAction(&ActionInput{
		Definition: *thunderclapDefinition(), AttackerID: bardID,
		TargetIDs: []string{heroID}, Roller: &facedRoller{d20: 1, other: 4},
	})
	s.Require().Error(err)
	s.ErrorIs(err, ErrBadAction)
}

// TestAFootprintThatCatchesNobodyStillHappens.
//
// A thunderclap in an empty room is not an error and not a no-op: the caster
// spent their action, and the interaction has to say so. Encounter's
// RecordCastInput already accepts empty Targets for exactly this case.
func (s *AreaActionTestSuite) TestAFootprintThatCatchesNobodyStillHappens() {
	fixtures := &ContestDamageTestSuite{}
	fixtures.SetT(s.T())

	machine, err := NewAction(&ActionInput{
		Definition: *thunderclapDefinition(), AttackerID: bardID,
		AreaMembers: nil, Roller: &facedRoller{d20: 1, other: 4},
	})
	s.Require().NoError(err)

	out, err := s.resolveArea(baneWorld(s.T(), 2), machine, fixtures.saver(14), baneCaster(1, 2))
	s.Require().NoError(err)

	outcome, ok := out.Outcome.(CastOutcome)
	s.Require().True(ok)
	s.Empty(outcome.Targets, "nobody was caught, and that is the answer")

	payer := fixtures.sheet(out, bardID)
	s.Zero(payer.ActionEconomy.ActionsRemaining, "the action is spent either way")
}

// TestADerivedMemberIsNotMeasuredFromTheCaster is the behaviour that keeps
// slice 2 possible, and it is the reason derived members skip validateCastTarget.
//
// That check measures caster-to-target against RangeFeet, which answers "could
// you have aimed there" — right for a target a client picked, wrong for a member
// the engine found inside a shape. The two coincide only while a footprint is
// centred on the caster and reaches exactly as far as the spell's range. Here
// the member sits far outside Thunderclap's five feet and still resolves,
// because containment was already decided by whoever derived it.
//
// The mirror is already pinned: TestBaneRefusesStaleAndOutOfRangeTargets shows a
// NAMED target at distance being refused by the same machinery.
func (s *AreaActionTestSuite) TestADerivedMemberIsNotMeasuredFromTheCaster() {
	fixtures := &ContestDamageTestSuite{}
	fixtures.SetT(s.T())

	machine, err := NewAction(&ActionInput{
		Definition: *thunderclapDefinition(), AttackerID: bardID,
		AreaMembers: []string{heroID}, Roller: &facedRoller{d20: 1, other: 4},
	})
	s.Require().NoError(err)

	// Far outside a five-foot burst by any reading of the grid.
	out, err := s.resolveArea(baneWorld(s.T(), 12), machine, fixtures.saver(14), baneCaster(1, 2))
	s.Require().NoError(err, "a derived member is not re-measured against the caster's range")

	outcome, ok := out.Outcome.(CastOutcome)
	s.Require().True(ok)
	s.Require().Len(outcome.Targets, 1)
	s.Equal(heroID, outcome.Targets[0].TargetID)
}
