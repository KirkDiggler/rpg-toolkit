// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package resolution

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/suite"

	coreCombat "github.com/KirkDiggler/rpg-toolkit/core/combat"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/character"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/combat"
	combatActions "github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/combat/actions"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/damage"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/refs"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/spells"
)

// CastActionTestSuite drives the cast door end to end: content compiles to a
// definition, NewAction reads the profile's arms, and the machine that runs is
// one that already existed.
//
// The two cantrips are the two halves of every cantrip — roll to see whether
// you deliver it, and just deliver it — so between them they are the whole
// branch.
type CastActionTestSuite struct {
	suite.Suite

	ctx context.Context
}

func TestCastActionSuite(t *testing.T) {
	suite.Run(t, new(CastActionTestSuite))
}

func (s *CastActionTestSuite) SetupTest() {
	s.ctx = context.Background()
}

// castDamage reuses the damage suite's world, sheets and roller: the cast door
// is a second way into the same interaction, and a second set of fixtures would
// be a second thing to keep true.
func (s *CastActionTestSuite) fixtures() *ContestDamageTestSuite {
	fixtures := &ContestDamageTestSuite{}
	fixtures.SetT(s.T())
	fixtures.ctx = s.ctx

	return fixtures
}

// oneAction is a cantrip's whole price.
func oneAction() *combat.SpendProfile {
	return &combat.SpendProfile{Slots: map[coreCombat.ActionType]int{coreCombat.ActionStandard: 1}}
}

func castCost() *Cost {
	return &Cost{
		PayerID: bardID,
		Profile: oneAction(),
		Turn:    &Turn{Number: mockeryTurn, Speed: mockerySpeed},
	}
}

// cast builds the machine the door would build for one cantrip.
func (s *CastActionTestSuite) cast(id spells.Spell, casterID, targetID string, roll int) Machine {
	definition := spells.CastDefinition(id, spellSaveDC)
	s.Require().NotNil(definition, "this build has cast content for %s", id)
	definition.Cost = oneAction()

	machine, err := NewAction(&ActionInput{
		Definition: *definition,
		AttackerID: casterID,
		TargetID:   targetID,
		Roller:     facedRoller{d20: roll, other: psychicFace},
	})
	s.Require().NoError(err)

	return machine
}

// castParams reads one persisted condition's raw configuration off a sheet, so
// the counterpart binding is asserted against what was actually stored rather
// than against what was passed in.
func (s *CastActionTestSuite) castParams(data *character.Data, conditionRef string) map[string]any {
	for _, raw := range data.Conditions {
		var peek map[string]any
		s.Require().NoError(json.Unmarshal(raw, &peek))
		if ref, _ := peek["ref"].(string); ref == conditionRef {
			return peek
		}
	}
	s.Require().Failf("condition not persisted", "%s is not on %s", conditionRef, data.ID)

	return nil
}

// THE HEADLINE FOR THE GATED HALF. Vicious Mockery, compiled from content,
// entered through the door at a price: the save fails, 1d4 psychic lands, and
// the rider goes on the target carrying the bard who imposed it.
func (s *CastActionTestSuite) TestViciousMockeryLandsDamageAndItsRider() {
	fixtures := s.fixtures()

	out, err := fixtures.resolve(
		fixtures.saver(14), s.cast(spells.ViciousMockery, bardID, heroID, straightRoll),
		castCost(), fixtures.bard(1),
	)
	s.Require().NoError(err)

	outcome := fixtures.outcome(out)
	s.Require().False(outcome.Succeeded, "WIS +1 on a 3 is 4 against DC 13")
	s.Require().Equal(spellSaveDC, outcome.DC)
	s.Require().Len(outcome.Imposed, 2)
	s.Require().Equal(ImposedDamage, outcome.Imposed[0].Kind)
	s.Require().Equal(psychicFace, outcome.Imposed[0].Amount)
	s.Require().Equal(damage.Psychic, outcome.Imposed[0].Components[0].DamageType)
	s.Require().Equal(ImposedCondition, outcome.Imposed[1].Kind)
	s.Require().Equal(refs.Conditions.ViciousMockery().String(), outcome.Imposed[1].Ref.String())

	target := fixtures.sheet(out, heroID)
	s.Require().Equal(11, target.HitPoints, "14 - 3 psychic")
	params := s.castParams(target, refs.Conditions.ViciousMockery().String())
	s.Require().Equal(bardID, params[spells.ViciousMockeryCasterParameter],
		"the counterpart the content named is the CASTER, written by resolution")

	payer := fixtures.sheet(out, bardID)
	s.Require().Zero(payer.ActionEconomy.ActionsRemaining, "the action was charged at the door")
}

// The control: the same cantrip, a made save, and nothing at all is delivered.
func (s *CastActionTestSuite) TestAMadeSaveAgainstViciousMockeryDeliversNothing() {
	fixtures := s.fixtures()

	out, err := fixtures.resolve(
		fixtures.saver(14), s.cast(spells.ViciousMockery, bardID, heroID, advantageRoll),
		castCost(), fixtures.bard(1),
	)
	s.Require().NoError(err)

	outcome := fixtures.outcome(out)
	s.Require().True(outcome.Succeeded)
	s.Require().Empty(outcome.Imposed, "no damage, no rider")
	s.Require().Equal(spellSaveDC, outcome.DC, "and the save is still on the record")

	for _, sheet := range out.DirtyCharacters {
		if sheet.ID == heroID {
			s.Require().Equal(14, sheet.HitPoints)
			s.Require().Empty(sheet.Conditions)
		}
	}

	payer := fixtures.sheet(out, bardID)
	s.Require().Zero(payer.ActionEconomy.ActionsRemaining,
		"the action is spent whether or not the save was made")
}

// THE HEADLINE FOR THE GATELESS HALF. True Strike has no gate, so there is
// nothing to roll: the condition goes on the CASTER, keyed to the creature the
// advantage is good against, and the collector reports the one effect.
func (s *CastActionTestSuite) TestTrueStrikeDeliversToTheCasterWithNoRoll() {
	fixtures := s.fixtures()

	out, err := fixtures.resolve(
		fixtures.saver(14), s.cast(spells.TrueStrike, bardID, heroID, straightRoll),
		castCost(), fixtures.bard(1),
	)
	s.Require().NoError(err)

	outcome, ok := out.Outcome.(ActivationOutcome)
	s.Require().True(ok, "a gateless cast is a delivery, and delivery is the activation outcome")
	s.Require().Equal(refs.Spells.ByID(string(spells.TrueStrike)).String(), outcome.Ability,
		"the outcome echoes the spell that ran")
	s.Require().Len(outcome.Effects, 1, "one Gather, one condition, one effect")
	s.Require().Equal(EffectConditionApplied, outcome.Effects[0].Kind)
	s.Require().Equal(bardID, outcome.Effects[0].TargetID, "on the caster, not the creature named")
	s.Require().Equal(refs.Conditions.TrueStrike().String(), outcome.Effects[0].Ref)

	caster := fixtures.sheet(out, bardID)
	params := s.castParams(caster, refs.Conditions.TrueStrike().String())
	s.Require().Equal(heroID, params[spells.TrueStrikeTargetParameter],
		"the counterpart the content named is the TARGET, written by resolution")
	s.Require().Zero(caster.ActionEconomy.ActionsRemaining, "charged at the door like any cast")

	for _, sheet := range out.DirtyCharacters {
		s.Require().NotEqual(heroID, sheet.ID, "the creature named is untouched: nobody resists True Strike")
	}
}

// A cast with an empty action budget is refused at the door: nothing is
// published, nothing is rolled, and no sheet comes back.
func (s *CastActionTestSuite) TestACastWithNoActionLeftIsRefusedAtTheDoor() {
	fixtures := s.fixtures()

	out, err := fixtures.resolve(
		fixtures.saver(14), s.cast(spells.TrueStrike, bardID, heroID, straightRoll),
		castCost(), fixtures.bard(0),
	)

	s.Require().ErrorIs(err, ErrCannotPay)
	s.Require().Nil(out, "a refused cast hands back nothing to store")
	s.Require().Contains(err.Error(), bardID)
}

// The same refusal for the gated half, and it happens before the save is
// rolled — the door is the only place a cantrip's price moves.
func (s *CastActionTestSuite) TestAGatedCastWithNoActionLeftIsRefusedBeforeTheSave() {
	fixtures := s.fixtures()

	_, err := fixtures.resolve(
		fixtures.saver(14), s.cast(spells.ViciousMockery, bardID, heroID, straightRoll),
		castCost(), fixtures.bard(0),
	)

	s.Require().ErrorIs(err, ErrCannotPay)
}

// NewAction reads the arm, and nothing below it reads a spell id. Two profiles
// from the same content table pick two different machines.
func (s *CastActionTestSuite) TestTheProfileArmPicksTheMachine() {
	gated, err := NewAction(&ActionInput{
		Definition: *spells.CastDefinition(spells.ViciousMockery, spellSaveDC),
		AttackerID: bardID, TargetID: heroID, Roller: facedRoller{d20: straightRoll, other: psychicFace},
	})
	s.Require().NoError(err)
	s.Require().IsType(&contestMachine{}, gated, "a gate is a save, and a save is a contest")

	gateless, err := NewAction(&ActionInput{
		Definition: *spells.CastDefinition(spells.TrueStrike, spellSaveDC),
		AttackerID: bardID, TargetID: heroID,
	})
	s.Require().NoError(err)
	s.Require().IsType(&activationMachine{}, gateless, "no gate is a delivery")
}

// Every refusal the cast branch makes, and each says what content got wrong.
func (s *CastActionTestSuite) TestTheCastBranchRefusesWhatItCannotDeliver() {
	trueStrike := func() combatActions.Definition { return *spells.CastDefinition(spells.TrueStrike, spellSaveDC) }

	s.Run("no caster", func() {
		_, err := NewAction(&ActionInput{Definition: trueStrike(), TargetID: heroID})
		s.Require().ErrorIs(err, ErrBadAction)
		s.Require().Contains(err.Error(), "cast by nobody")
	})

	s.Run("a one-creature cast with no creature", func() {
		_, err := NewAction(&ActionInput{Definition: trueStrike(), AttackerID: bardID})
		s.Require().ErrorIs(err, ErrBadAction)
		s.Require().Contains(err.Error(), "names one creature")
	})

	s.Run("a self-targeted cast handed a target", func() {
		definition := trueStrike()
		definition.Cast.Target = combatActions.CastTargetSelf
		definition.Cast.Effects[0].CounterpartKey = ""

		_, err := NewAction(&ActionInput{Definition: definition, AttackerID: bardID, TargetID: heroID})
		s.Require().ErrorIs(err, ErrBadAction)
		s.Require().Contains(err.Error(), "reaches only its caster")
	})

	s.Run("a gated cast delivering to its caster", func() {
		definition := *spells.CastDefinition(spells.ViciousMockery, spellSaveDC)
		definition.Cast.Effects[0].Recipient = combatActions.CastRecipientCaster

		_, err := NewAction(&ActionInput{Definition: definition, AttackerID: bardID, TargetID: heroID})
		s.Require().ErrorIs(err, ErrBadAction)
		s.Require().Contains(err.Error(), "delivers to its caster")
	})

	s.Run("a gated cast with two conditions", func() {
		definition := *spells.CastDefinition(spells.ViciousMockery, spellSaveDC)
		definition.Cast.Effects = append(definition.Cast.Effects, definition.Cast.Effects[0])

		_, err := NewAction(&ActionInput{Definition: definition, AttackerID: bardID, TargetID: heroID})
		s.Require().ErrorIs(err, ErrBadAction)
		s.Require().Contains(err.Error(), "a contest delivers one")
	})

	s.Run("undefended damage", func() {
		definition := trueStrike()
		definition.Cast.Damage = []damage.Damage{{Dice: "1d4", Type: damage.Psychic}}

		_, err := NewAction(&ActionInput{Definition: definition, AttackerID: bardID, TargetID: heroID})
		s.Require().ErrorIs(err, ErrBadAction)
		s.Require().Contains(err.Error(), "deals damage with no save")
	})

	s.Run("a recipient who is not in the cast", func() {
		fixtures := s.fixtures()
		_, err := fixtures.resolve(
			fixtures.saver(14), s.cast(spells.TrueStrike, "nobody", heroID, straightRoll), nil, nil,
		)
		s.Require().ErrorIs(err, ErrBadActivation)
	})
}

// The counterpart binding is content's key, not a convention: the two cantrips
// spell it differently and the writer is told which by the profile.
func (s *CastActionTestSuite) TestTheCounterpartKeyIsWrittenWhereContentSaid() {
	forEach := []struct {
		name          string
		effect        combatActions.CastEffect
		counterpartID string
		wantKey       string
	}{
		{
			"true strike names the target", combatActions.CastEffect{
				Recipient: combatActions.CastRecipientCaster, Ref: *refs.Conditions.TrueStrike(),
				CounterpartKey: spells.TrueStrikeTargetParameter,
			}, heroID, spells.TrueStrikeTargetParameter,
		},
		{
			"vicious mockery names the caster", combatActions.CastEffect{
				Recipient: combatActions.CastRecipientTarget, Ref: *refs.Conditions.ViciousMockery(),
				CounterpartKey: spells.ViciousMockeryCasterParameter,
			}, bardID, spells.ViciousMockeryCasterParameter,
		},
	}

	for _, tc := range forEach {
		s.Run(tc.name, func() {
			bound, err := bindCounterpart(tc.effect, tc.counterpartID)
			s.Require().NoError(err)

			var fields map[string]string
			s.Require().NoError(json.Unmarshal(bound, &fields))
			s.Require().Equal(map[string]string{tc.wantKey: tc.counterpartID}, fields)
		})
	}

	s.Run("an effect that binds nothing keeps its parameters", func() {
		bound, err := bindCounterpart(combatActions.CastEffect{
			Ref: *refs.Conditions.TrueStrike(), Parameters: json.RawMessage(`{"kept":true}`),
		}, "")
		s.Require().NoError(err)
		s.Require().JSONEq(`{"kept":true}`, string(bound))
	})

	s.Run("a binding with no counterpart is refused", func() {
		_, err := bindCounterpart(combatActions.CastEffect{
			Ref: *refs.Conditions.TrueStrike(), CounterpartKey: spells.TrueStrikeTargetParameter,
		}, "")
		s.Require().Error(err)
		s.Require().Contains(err.Error(), "the cast's other party")
	})
}
