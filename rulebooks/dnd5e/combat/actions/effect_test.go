// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package actions_test

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/KirkDiggler/rpg-toolkit/core"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/combat/actions"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/damage"
	dnd5eEvents "github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/events"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/healing"
)

// EffectProfileSuite covers the fourth body: an action that does something to
// someone without a weapon and without a spell.
type EffectProfileSuite struct {
	suite.Suite
}

func TestEffectProfileSuite(t *testing.T) { suite.Run(t, new(EffectProfileSuite)) }

func (s *EffectProfileSuite) ref(id string) core.Ref {
	return core.Ref{Module: "dnd5e", Type: "features", ID: id}
}

func (s *EffectProfileSuite) healingDeclaration() *healing.Declaration {
	fighter := core.Ref{Module: "dnd5e", Type: "classes", ID: "fighter"}
	return &healing.Declaration{Dice: "1d10", Modifiers: []healing.Modifier{{
		Source: dnd5eEvents.RollSource{Ref: &fighter, Name: "Fighter", Label: "Fighter level", SourceID: "hero"},
		Amount: 3,
	}}}
}

func (s *EffectProfileSuite) effect() actions.Definition {
	return actions.Definition{
		Ref: s.ref("second_wind"), Name: "Second Wind", Description: "Catch your breath.",
		Effect: &actions.EffectProfile{To: actions.EffectSelf, Healing: s.healingDeclaration()},
	}
}

func (s *EffectProfileSuite) attackProfile() *actions.AttackProfile {
	return &actions.AttackProfile{
		Category: actions.AttackCategoryWeapon,
		Delivery: actions.AttackDelivery{Melee: &actions.MeleeDelivery{ReachFeet: 5}},
		Damage:   []damage.Damage{{Dice: "1d8", Type: damage.Bludgeoning}},
	}
}

func (s *EffectProfileSuite) TestAnEffectAloneIsADefinition() {
	s.Require().NoError(s.effect().Validate())
}

func (s *EffectProfileSuite) TestAnEffectAndAnAttackAreRefused() {
	definition := s.effect()
	definition.Attack = s.attackProfile()

	err := definition.Validate()

	s.Require().Error(err)
	s.Contains(err.Error(), "exactly one")
}

func (s *EffectProfileSuite) TestNoBodyAtAllIsRefused() {
	definition := s.effect()
	definition.Effect = nil

	err := definition.Validate()

	s.Require().Error(err)
	s.Contains(err.Error(), "exactly one")
}

func (s *EffectProfileSuite) TestAnEffectThatDoesNothingIsRefused() {
	definition := s.effect()
	definition.Effect.Healing = nil

	err := definition.Validate()

	s.Require().Error(err)
	s.Contains(err.Error(), "healing")
}

func (s *EffectProfileSuite) TestAnEffectOnSomeoneElseIsRefusedNamingSelf() {
	definition := s.effect()
	definition.Effect.To = "ally"

	err := definition.Validate()

	s.Require().Error(err)
	s.Contains(err.Error(), string(actions.EffectSelf))
}

func (s *EffectProfileSuite) TestMalformedHealingDiceCarryTheDeclarationsOwnMessage() {
	definition := s.effect()
	definition.Effect.Healing.Dice = "ten"
	own := definition.Effect.Healing.Validate()
	s.Require().Error(own)

	err := definition.Validate()

	s.Require().Error(err)
	s.Contains(err.Error(), own.Error())
}

func (s *EffectProfileSuite) TestCloneDetachesTheHealingModifiers() {
	original := s.effect()

	clone := original.Clone()
	clone.Effect.Healing.Modifiers[0].Amount = 99
	clone.Effect.Healing.Modifiers[0].Source.Ref.ID = "changed"
	clone.Effect.Healing.Dice = "9d9"
	clone.Effect.To = "ally"

	s.Equal(3, original.Effect.Healing.Modifiers[0].Amount)
	s.Equal("fighter", original.Effect.Healing.Modifiers[0].Source.Ref.ID)
	s.Equal("1d10", original.Effect.Healing.Dice)
	s.Equal(actions.EffectSelf, original.Effect.To)
	s.NotSame(original.Effect, clone.Effect)
}

func (s *EffectProfileSuite) TestTheDefinitionRoundTripsThroughJSON() {
	original := s.effect()

	data, err := json.Marshal(original)
	s.Require().NoError(err)
	s.Contains(string(data), `"effect"`)
	s.NotContains(string(data), `"attack"`)
	var loaded actions.Definition
	s.Require().NoError(json.Unmarshal(data, &loaded))

	s.Equal(original, loaded)
	s.Require().NoError(loaded.Validate())
}

// A healing spell with no attack answers Describe with its prose and no
// attack facts; an effect answers the same way: its prose, and no damage,
// grip or delivery, without being refused.
func (s *EffectProfileSuite) TestDescribeAnswersAsForAHealingSpellWithNoAttack() {
	out, err := actions.Describe(&actions.DescribeInput{Definition: s.effect()})

	s.Require().NoError(err)
	s.Equal("Catch your breath.", out.Description)
	s.Empty(out.Facts.Damage)
	s.Equal(actions.GripNone, out.Facts.Grip)
	s.Nil(out.Facts.Melee)
	s.Nil(out.Facts.Ranged)
	s.Nil(out.Facts.Cast, "an effect is not a cast and states no cast facts")
}

// A sequence step that names an effect definition is passed as a cast
// component is: resolved, and a clone of the repertoire's own definition.
func (s *EffectProfileSuite) TestASequenceStepMayNameAnEffect() {
	effect := s.effect()
	swing := actions.Definition{Ref: s.ref("swing"), Name: "Swing", Attack: s.attackProfile()}
	script := actions.Definition{
		Ref: s.ref("flurry"), Name: "Flurry",
		Sequence: &actions.SequenceProfile{Steps: []actions.SequenceStep{{Action: effect.Ref}, {Action: swing.Ref}}},
	}

	components, err := actions.ResolveSequence(script, []actions.Definition{script, effect, swing})

	s.Require().NoError(err)
	s.Require().Len(components, 2)
	s.Require().NotNil(components[0].Definition.Effect)
	s.NotSame(effect.Effect, components[0].Definition.Effect, "the component is a clone")
	s.Equal(effect.Effect, components[0].Definition.Effect)
}
