// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package resolution

import (
	"context"
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/abilities"
	combatActions "github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/combat/actions"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/damage"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/refs"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/saves"
)

// ContestMoveTestSuite drives the third thing a failed save can cost: the cast
// MOVES whoever failed it.
//
// Resolution describes the move and nothing else — a policy, an anchor and a
// budget. Which cells those are is encounter's under its own fold
// (rpg-project#431 §2), and nothing in this file computes one.
type ContestMoveTestSuite struct {
	suite.Suite

	ctx context.Context
}

func TestContestMoveSuite(t *testing.T) {
	suite.Run(t, new(ContestMoveTestSuite))
}

func (s *ContestMoveTestSuite) SetupTest() {
	s.ctx = context.Background()
}

// fixtures reuses the damage suite's world, sheets and roller, for the reason
// the cast suite does: this is a third consequence of the same contest, not a
// second interaction.
func (s *ContestMoveTestSuite) fixtures() *ContestDamageTestSuite {
	fixtures := &ContestDamageTestSuite{}
	fixtures.SetT(s.T())
	fixtures.ctx = s.ctx

	return fixtures
}

// thunderFace is the face every d8 shows here, so 2d8 is a number the
// assertions can name: eight, which a saver on five hit points does not
// survive.
const thunderFace = 4

// shoveProfile is Thunderwave's cast profile minus its shape: a Constitution
// save, one shared 2d8, and whatever move the scene declares.
//
// Built here rather than read from the content table because this package must
// not know WHICH spell it is holding (ADR-0045). What it reads is a profile
// with a move on it, and the profile is the whole of what it may know.
func shoveProfile(move *combatActions.CastMove) *combatActions.CastProfile {
	return &combatActions.CastProfile{
		RangeFeet:  30,
		Target:     combatActions.CastTargetOneCreature,
		MinTargets: 1,
		MaxTargets: 1,
		Save: &saves.SaveGate{
			Abilities:  []abilities.Ability{abilities.CON},
			DC:         saves.DCStatic(spellSaveDC),
			OnSuccess:  saves.Negated,
			Recurrence: saves.RecurrenceNone,
		},
		Damage: []damage.Damage{{Dice: "2d8", Type: damage.Thunder}},
		Move:   move,
	}
}

func shoveDefinition(move *combatActions.CastMove) combatActions.Definition {
	return combatActions.Definition{
		Ref:  *refs.Spells.Thunderwave(),
		Name: "Thunderwave",
		Cost: oneAction(),
		Cast: shoveProfile(move),
	}
}

// shove builds the machine the cast door builds for a profile that moves.
func (s *ContestMoveTestSuite) shove(move *combatActions.CastMove, roll int) (Machine, error) {
	return NewAction(&ActionInput{
		Definition: shoveDefinition(move),
		AttackerID: bardID,
		TargetIDs:  []string{heroID},
		Roller:     facedRoller{d20: roll, other: thunderFace},
	})
}

func (s *ContestMoveTestSuite) castOutcome(out *Output) CastOutcome {
	outcome, ok := out.Outcome.(CastOutcome)
	s.Require().True(ok, "a cast produces a CastOutcome")

	return outcome
}

// kindsOf is the imposed list as the story reads it: which consequences, in
// which order, and nothing about their contents.
func kindsOf(applied []ImposedEffect) []ImposedEffectKind {
	kinds := make([]ImposedEffectKind, 0, len(applied))
	for _, effect := range applied {
		kinds = append(kinds, effect.Kind)
	}

	return kinds
}

// THE HEADLINE. A failed save costs the damage and then the push, in that
// order, and the push is a DESCRIPTION: a policy, the caster it is measured
// from, and a budget in cells.
func (s *ContestMoveTestSuite) TestAFailedSaveIsPushedAfterItIsDamaged() {
	fixtures := s.fixtures()
	machine, err := s.shove(&combatActions.CastMove{Policy: combatActions.MoveLine, Cells: 2}, straightRoll)
	s.Require().NoError(err)

	out, err := fixtures.resolve(fixtures.saver(14), machine, castCost(), fixtures.bard(1))
	s.Require().NoError(err)

	target := s.castOutcome(out).Targets[0]
	s.Require().NotNil(target.Save)
	s.Require().False(target.Save.Succeeded, "CON +2 on a 3 is 5 against DC 13")
	s.Equal([]ImposedEffectKind{ImposedDamage, ImposedMove}, kindsOf(target.Applied),
		"damage first, then the push: a body sliding across the floor is not a story we tell")

	moved := target.Applied[1]
	s.Require().NotNil(moved.Move, "an imposed move carries the directive it imposes")
	s.Equal(heroID, moved.RecipientID, "whoever failed the save is who moves")
	s.Equal(bardID, moved.Move.AnchorID, "a line is measured from the caster")
	s.Equal(combatActions.MoveLine, moved.Move.Policy)
	s.Equal(2, moved.Move.Cells)
	s.False(moved.Move.Speed, "a fixed budget is not the mover's own speed")
	s.Equal(combatActions.PaysNothing, moved.Move.Pays, "the push costs its victim nothing")
	s.False(moved.Move.Provokes, "being thrown across a room is not walking out of a reach")
	s.Require().NotNil(moved.Ref)
	s.Equal(refs.Spells.Thunderwave().String(), moved.Ref.String(), "named by whatever moved them")
	s.NotEmpty(moved.Description, "and it reads as something in a step log")
	s.Zero(moved.Amount, "a move has no amount to report")
}

// The control: the save is made, and nothing at all is delivered.
func (s *ContestMoveTestSuite) TestAMadeSaveIsNotPushed() {
	fixtures := s.fixtures()
	machine, err := s.shove(&combatActions.CastMove{Policy: combatActions.MoveLine, Cells: 2}, advantageRoll)
	s.Require().NoError(err)

	out, err := fixtures.resolve(fixtures.saver(14), machine, castCost(), fixtures.bard(1))
	s.Require().NoError(err)

	target := s.castOutcome(out).Targets[0]
	s.Require().NotNil(target.Save)
	s.Require().True(target.Save.Succeeded, "CON +2 on an 18 is 20 against DC 13")
	s.Empty(target.Applied, "a made save negates the damage and the push together")
}

// THE DROPPED ARE NOT PUSHED. The damage takes the saver to zero, and what
// falls there stays there — read at the moment the push would happen, off the
// same sheet the damage was just applied to.
func (s *ContestMoveTestSuite) TestTheDroppedAreNotPushed() {
	fixtures := s.fixtures()
	machine, err := s.shove(&combatActions.CastMove{Policy: combatActions.MoveLine, Cells: 2}, straightRoll)
	s.Require().NoError(err)

	out, err := fixtures.resolve(fixtures.saver(5), machine, castCost(), fixtures.bard(1))
	s.Require().NoError(err)

	target := s.castOutcome(out).Targets[0]
	s.Equal([]ImposedEffectKind{ImposedDamage}, kindsOf(target.Applied),
		"eight thunder on five hit points, and nothing to push afterwards")
	s.Zero(fixtures.sheet(out, heroID).HitPoints, "they are down where they fell")
}

// A cast that moves somebody needs the moment at which it does. Content's own
// validator refuses it, and this pins that the refusal reaches the door here —
// a profile this package cannot resolve never gets as far as charging anybody.
func (s *ContestMoveTestSuite) TestACastThatMovesWithNoSaveIsRefusedAtTheDoor() {
	definition := shoveDefinition(&combatActions.CastMove{Policy: combatActions.MoveLine, Cells: 2})
	definition.Cast.Save = nil

	_, err := NewAction(&ActionInput{
		Definition: definition,
		AttackerID: bardID,
		TargetIDs:  []string{heroID},
		Roller:     facedRoller{d20: straightRoll, other: thunderFace},
	})
	s.Require().ErrorIs(err, ErrBadAction)
	s.Contains(err.Error(), "save", "and the refusal says what is missing")
}

// The anchor is this layer's own field — content declares no anchor, the cast
// door names the caster — so it is refused here rather than by the profile.
func (s *ContestMoveTestSuite) TestADirectiveWithNoAnchorIsRefused() {
	fixtures := s.fixtures()

	_, err := fixtures.resolve(fixtures.saver(14), NewContest(&ContestInput{
		Gate:       mockeryGate(),
		SaverID:    heroID,
		Damage:     psychic(),
		SourceName: mockeryName,
		Cause:      mockedCause(),
		Move:       &MoveDirective{Policy: combatActions.MoveLine, Cells: 2},
		Roller:     facedRoller{d20: straightRoll, other: psychicFace},
	}), nil, nil)

	s.Require().ErrorIs(err, ErrBadAction)
	s.Contains(err.Error(), "anchor")
}

// A price nobody charges and a budget nobody resolves are refused rather than
// described, because a description this stack cannot execute is an affordance
// with nothing behind it. Both arrive with Dissonant Whispers, which brings
// the reaction spend and the speed lookup with it (rpg-project#431 §0).
func (s *ContestMoveTestSuite) TestADirectiveNothingCanExecuteIsRefused() {
	fixtures := s.fixtures()

	s.Run("a reaction nobody spends", func() {
		_, err := fixtures.resolve(fixtures.saver(14), s.contestMoving(&MoveDirective{
			Policy: combatActions.MoveLine, AnchorID: bardID, Cells: 2,
			Pays: combatActions.PaysReaction,
		}), nil, nil)
		s.Require().ErrorIs(err, ErrBadAction)
		s.Contains(err.Error(), "reaction")
	})

	s.Run("a speed nobody reads", func() {
		_, err := fixtures.resolve(fixtures.saver(14), s.contestMoving(&MoveDirective{
			Policy: combatActions.MoveLine, AnchorID: bardID, Speed: true,
		}), nil, nil)
		s.Require().ErrorIs(err, ErrBadAction)
		s.Contains(err.Error(), "speed")
	})
}

// contestMoving is the damage suite's contest with a directive on it.
func (s *ContestMoveTestSuite) contestMoving(move *MoveDirective) Machine {
	return NewContest(&ContestInput{
		Gate:       mockeryGate(),
		SaverID:    heroID,
		Damage:     psychic(),
		SourceName: mockeryName,
		Cause:      mockedCause(),
		Move:       move,
		Roller:     facedRoller{d20: straightRoll, other: psychicFace},
	})
}
