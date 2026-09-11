// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package actions_test

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/KirkDiggler/rpg-toolkit/core"
	coreCombat "github.com/KirkDiggler/rpg-toolkit/core/combat"
	coreResources "github.com/KirkDiggler/rpg-toolkit/core/resources"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/abilities"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/combat"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/combat/actions"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/damage"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/resources"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/saves"
)

// CastProfileSuite covers the second profile arm: what a valid cast declares,
// what it refuses, and that a definition still populates exactly one profile.
type CastProfileSuite struct {
	suite.Suite
}

type castLedger struct {
	actions int
	pools   map[coreResources.ResourceKey]int
	writes  int
}

func (l *castLedger) InCombat() bool { return true }
func (l *castLedger) SlotsLeft(slot coreCombat.ActionType) int {
	if slot == coreCombat.ActionStandard {
		return l.actions
	}
	return 0
}
func (l *castLedger) CapacityLeft(combat.CapacityType) int           { return 0 }
func (l *castLedger) PoolLeft(key coreResources.ResourceKey) int     { return l.pools[key] }
func (l *castLedger) SpendSlots(_ coreCombat.ActionType, amount int) { l.writes++; l.actions -= amount }
func (l *castLedger) SpendCapacity(combat.CapacityType, int)         {}
func (l *castLedger) SpendPool(key coreResources.ResourceKey, amount int) {
	l.writes++
	l.pools[key] -= amount
}
func (l *castLedger) BankCapacity(combat.CapacityType, int) {}

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
		RangeFeet:  30,
		Target:     actions.CastTargetOneCreature,
		MinTargets: 1,
		MaxTargets: 1,
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
		RangeFeet:  60,
		Target:     actions.CastTargetOneCreature,
		MinTargets: 1,
		MaxTargets: 1,
		Save:       saves.NewSaveGate(abilities.WIS, 13),
		Damage:     []damage.Damage{{Dice: "1d4", Type: damage.Psychic}},
		Effects: []actions.CastEffect{{
			Recipient:      actions.CastRecipientTarget,
			Ref:            conditionRef("vicious_mockery"),
			CounterpartKey: "source_id",
		}},
	}
}

// gatedDamageProfile is Dissonant Whispers' shape without its move: a save, a
// damage pool, and nothing delivered. The shape half-on-a-save is for.
func gatedDamageProfile() actions.CastProfile {
	return actions.CastProfile{
		RangeFeet:  60,
		Target:     actions.CastTargetOneCreature,
		MinTargets: 1,
		MaxTargets: 1,
		Save:       saves.NewSaveGate(abilities.WIS, 13),
		Damage:     []damage.Damage{{Dice: "3d6", Type: damage.Psychic}},
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

// TestHalfIsPermittedForADamageOnlyGate — half arrived with Dissonant
// Whispers, and a cast that only deals damage is the whole of what it means:
// the pool is rolled and the successful save takes half of the number.
func (s *CastProfileSuite) TestHalfIsPermittedForADamageOnlyGate() {
	p := gatedDamageProfile()
	p.Save.OnSuccess = saves.Half
	s.NoError(p.Validate())
}

// TestHalfIsRefusedWhenAConditionIsDelivered — half a condition means nothing.
// Vicious Mockery's shape delivers a rider, so the refusal stays exactly where
// there is no arithmetic to halve.
func (s *CastProfileSuite) TestHalfIsRefusedWhenAConditionIsDelivered() {
	p := gatedProfile()
	p.Save.OnSuccess = saves.Half
	err := p.Validate()
	s.Require().Error(err)
	s.Contains(err.Error(), "half on a save is only for damage")
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

	s.Run("creature target minimum is zero", func() {
		profile := gatelessProfile()
		profile.MinTargets = 0
		s.Require().ErrorContains(profile.Validate(), "at least one target")
	})

	s.Run("maximum is below minimum", func() {
		profile := gatelessProfile()
		profile.MinTargets = 3
		profile.MaxTargets = 2
		s.Require().ErrorContains(profile.Validate(), "at least its minimum")
	})

	s.Run("no consequence at all", func() {
		profile := gatelessProfile()
		profile.Effects = nil
		s.Require().ErrorContains(profile.Validate(), "damage or a delivered condition")
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
		profile.MinTargets = 0
		profile.MaxTargets = 0
		profile.Effects[0].Recipient = actions.CastRecipientTarget
		s.Require().ErrorContains(profile.Validate(), "delivers only to the caster")
	})

	s.Run("a self cast binding a counterpart", func() {
		profile := gatelessProfile()
		profile.Target = actions.CastTargetSelf
		profile.MinTargets = 0
		profile.MaxTargets = 0
		s.Require().ErrorContains(profile.Validate(), "no counterpart to bind")
	})
}

func (s *CastProfileSuite) TestLevelOneCastPriceRefusalPreservesActionAndPoolAtomically() {
	price := &combat.SpendProfile{
		Slots: map[coreCombat.ActionType]int{coreCombat.ActionStandard: 1},
		Pools: map[coreResources.ResourceKey]int{resources.SpellSlotLevel1: 1},
	}

	for _, test := range []struct {
		name    string
		actions int
		pool    int
	}{
		{name: "action shortage", actions: 0, pool: 1},
		{name: "pool shortage", actions: 1, pool: 0},
	} {
		s.Run(test.name, func() {
			ledger := &castLedger{actions: test.actions, pools: map[coreResources.ResourceKey]int{
				resources.SpellSlotLevel1: test.pool,
			}}
			s.False(combat.CanPay(ledger, price))
			s.Require().Error(combat.Pay(ledger, price))
			s.Zero(ledger.writes)
			s.Equal(test.actions, ledger.actions)
			s.Equal(test.pool, ledger.pools[resources.SpellSlotLevel1])
		})
	}
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
