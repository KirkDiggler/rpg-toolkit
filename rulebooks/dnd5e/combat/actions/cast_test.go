// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package actions_test

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/KirkDiggler/rpg-toolkit/core"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/abilities"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/combat/actions"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/damage"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/saves"
)

// CastProfileSuite covers the second profile arm: what a valid cast declares,
// what it refuses, and that a definition still populates exactly one profile.
type CastProfileSuite struct {
	suite.Suite
}

func TestCastProfileSuite(t *testing.T) {
	suite.Run(t, new(CastProfileSuite))
}

func conditionRef(id string) core.Ref {
	return core.Ref{Module: "dnd5e", Type: "conditions", ID: id}
}

// gatelessProfile is True Strike's shape: a named creature, one condition on
// the caster, no roll anywhere.
func gatelessProfile() actions.CastProfile {
	return actions.CastProfile{
		RangeFeet: 30,
		Target:    actions.CastTargetOneCreature,
		Effects: []actions.CastEffect{{
			Recipient:      actions.CastRecipientCaster,
			Ref:            conditionRef("true_strike"),
			CounterpartKey: "target_id",
		}},
	}
}

// gatedProfile is Vicious Mockery's shape: a save, damage, and a rider.
func gatedProfile() actions.CastProfile {
	return actions.CastProfile{
		RangeFeet: 60,
		Target:    actions.CastTargetOneCreature,
		Save:      saves.NewSaveGate(abilities.WIS, 13),
		Damage:    []damage.Damage{{Dice: "1d4", Type: damage.Psychic}},
		Effects: []actions.CastEffect{{
			Recipient:      actions.CastRecipientTarget,
			Ref:            conditionRef("vicious_mockery"),
			CounterpartKey: "source_id",
		}},
	}
}

func (s *CastProfileSuite) TestAGatelessCastValidates() {
	s.Require().NoError(gatelessProfile().Validate())
}

func (s *CastProfileSuite) TestAGatedCastValidates() {
	s.Require().NoError(gatedProfile().Validate())
}

// A pointer rather than a bool beside a duration: nil is "no concentration",
// and there is no state where a declared duration means nothing.
func (s *CastProfileSuite) TestAConcentrationCastValidates() {
	profile := gatelessProfile()
	profile.Concentration = &actions.CastConcentration{TurnEnds: 2}

	s.Require().NoError(profile.Validate())
	s.Nil(gatelessProfile().Concentration, "and a profile that declares none carries nothing")
}

func (s *CastProfileSuite) TestItRefusesAConcentrationThatEndsBeforeItBegins() {
	profile := gatelessProfile()
	profile.Concentration = &actions.CastConcentration{}

	s.Require().ErrorContains(profile.Validate(), "at least one turn end")
}

func (s *CastProfileSuite) TestCloningACastProfileCopiesItsConcentration() {
	profile := gatelessProfile()
	profile.Concentration = &actions.CastConcentration{TurnEnds: 10, SkipFirstTurnEnd: true}

	clone := profile.Clone()
	clone.Concentration.TurnEnds = 99
	clone.Concentration.SkipFirstTurnEnd = false

	s.Require().NotNil(profile.Concentration)
	s.Equal(10, profile.Concentration.TurnEnds, "a clone that aliased the duration would rewrite the original")
	s.True(profile.Concentration.SkipFirstTurnEnd)
}

func (s *CastProfileSuite) TestConcentrationSkipFirstTurnEndRoundTrips() {
	profile := gatelessProfile()
	profile.Concentration = &actions.CastConcentration{TurnEnds: 10, SkipFirstTurnEnd: true}

	raw, err := json.Marshal(profile)
	s.Require().NoError(err)
	var back actions.CastProfile
	s.Require().NoError(json.Unmarshal(raw, &back))
	s.Require().NotNil(back.Concentration)
	s.True(back.Concentration.SkipFirstTurnEnd)
}

func (s *CastProfileSuite) TestItRefusesWhatItCannotResolve() {
	s.Run("no range", func() {
		profile := gatelessProfile()
		profile.RangeFeet = 0
		s.Require().ErrorContains(profile.Validate(), "positive range")
	})

	s.Run("unknown target rule", func() {
		profile := gatelessProfile()
		profile.Target = "everybody"
		s.Require().ErrorContains(profile.Validate(), "unknown cast target rule")
	})

	s.Run("no consequence at all", func() {
		profile := gatelessProfile()
		profile.Effects = nil
		s.Require().ErrorContains(profile.Validate(), "damage or a delivered condition")
	})

	s.Run("a save that buys half", func() {
		profile := gatedProfile()
		profile.Save.OnSuccess = saves.Half
		s.Require().ErrorContains(profile.Validate(), "negate the cast")
	})

	s.Run("a save that recurs", func() {
		profile := gatedProfile()
		profile.Save.Recurrence = saves.RecurrenceEndOfTurn
		s.Require().ErrorContains(profile.Validate(), "must not recur")
	})

	s.Run("damage marked with the attack ability", func() {
		profile := gatedProfile()
		profile.Damage[0].Properties = []damage.Property{damage.AddsAttackAbilityModifier}
		s.Require().ErrorContains(profile.Validate(), "attack ability modifier")
	})

	s.Run("an effect that is not a condition", func() {
		profile := gatelessProfile()
		profile.Effects[0].Ref = core.Ref{Module: "dnd5e", Type: "spells", ID: "true-strike"}
		s.Require().ErrorContains(profile.Validate(), "dnd5e:conditions")
	})

	s.Run("an unknown recipient", func() {
		profile := gatelessProfile()
		profile.Effects[0].Recipient = "the room"
		s.Require().ErrorContains(profile.Validate(), "unknown cast effect recipient")
	})

	s.Run("a self cast delivering to a target", func() {
		profile := gatelessProfile()
		profile.Target = actions.CastTargetSelf
		profile.Effects[0].Recipient = actions.CastRecipientTarget
		s.Require().ErrorContains(profile.Validate(), "delivers only to the caster")
	})

	s.Run("a self cast binding a counterpart", func() {
		profile := gatelessProfile()
		profile.Target = actions.CastTargetSelf
		s.Require().ErrorContains(profile.Validate(), "no counterpart to bind")
	})
}

func (s *CastProfileSuite) TestADefinitionWithTwoProfilesIsRefused() {
	definition := actions.Definition{
		Ref:  core.Ref{Module: "dnd5e", Type: "spells", ID: "vicious-mockery"},
		Name: "Vicious Mockery",
		Cast: ptr(gatedProfile()),
		Attack: &actions.AttackProfile{
			Category: actions.AttackCategorySpell,
			Delivery: actions.AttackDelivery{Ranged: &actions.RangedDelivery{NormalFeet: 60}},
			Damage:   []damage.Damage{{Dice: "1d4", Type: damage.Psychic}},
		},
	}

	s.Require().ErrorContains(definition.Validate(), "exactly one profile")
}

func (s *CastProfileSuite) TestADefinitionWithNoProfileIsStillRefused() {
	definition := actions.Definition{
		Ref:  core.Ref{Module: "dnd5e", Type: "spells", ID: "mage-hand"},
		Name: "Mage Hand",
	}

	s.Require().ErrorContains(definition.Validate(), "exactly one profile")
}

func (s *CastProfileSuite) TestADefinitionWithACastProfileValidates() {
	definition := actions.Definition{
		Ref:  core.Ref{Module: "dnd5e", Type: "spells", ID: "vicious-mockery"},
		Name: "Vicious Mockery",
		Cast: ptr(gatedProfile()),
	}

	s.Require().NoError(definition.Validate())
}

func (s *CastProfileSuite) TestAnInvalidCastProfileFailsTheDefinition() {
	profile := gatedProfile()
	profile.RangeFeet = 0

	definition := actions.Definition{
		Ref:  core.Ref{Module: "dnd5e", Type: "spells", ID: "vicious-mockery"},
		Name: "Vicious Mockery",
		Cast: &profile,
	}

	s.Require().ErrorContains(definition.Validate(), "cast profile is invalid")
}

func (s *CastProfileSuite) TestCloningACastDefinitionAliasesNothing() {
	profile := gatedProfile()
	profile.Effects[0].Parameters = json.RawMessage(`{"note":"original"}`)
	original := actions.Definition{
		Ref:  core.Ref{Module: "dnd5e", Type: "spells", ID: "vicious-mockery"},
		Name: "Vicious Mockery",
		Cast: &profile,
	}

	clone := original.Clone()

	s.Require().NotSame(original.Cast, clone.Cast)
	clone.Cast.RangeFeet = 5
	clone.Cast.Damage[0].Dice = "9d9"
	clone.Cast.Effects[0].Ref.ID = "rewritten"
	clone.Cast.Effects[0].Parameters[2] = 'X'
	clone.Cast.Save.Abilities[0] = abilities.STR

	s.Equal(60, original.Cast.RangeFeet)
	s.Equal("1d4", original.Cast.Damage[0].Dice)
	s.Equal("vicious_mockery", original.Cast.Effects[0].Ref.ID)
	s.JSONEq(`{"note":"original"}`, string(original.Cast.Effects[0].Parameters))
	s.Equal(abilities.WIS, original.Cast.Save.Abilities[0])
}

func ptr(profile actions.CastProfile) *actions.CastProfile { return &profile }
