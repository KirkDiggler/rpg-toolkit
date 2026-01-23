// Copyright (C) 2024 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package conditions_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/KirkDiggler/rpg-toolkit/events"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/combat"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/conditions"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/damage"
	dnd5eEvents "github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/events"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/refs"
)

type FightingStyleTwoWeaponFightingTestSuite struct {
	suite.Suite
	ctx context.Context
	bus events.EventBus
}

func (s *FightingStyleTwoWeaponFightingTestSuite) SetupTest() {
	s.ctx = context.Background()
	s.bus = events.NewEventBus()
}

func TestFightingStyleTwoWeaponFightingSuite(t *testing.T) {
	suite.Run(t, new(FightingStyleTwoWeaponFightingTestSuite))
}

func (s *FightingStyleTwoWeaponFightingTestSuite) TestNewFightingStyleTwoWeaponFightingCondition() {
	twf := conditions.NewFightingStyleTwoWeaponFightingCondition("fighter-1")

	s.NotNil(twf)
	s.False(twf.IsApplied())
}

func (s *FightingStyleTwoWeaponFightingTestSuite) TestApplyAndRemove() {
	twf := conditions.NewFightingStyleTwoWeaponFightingCondition("fighter-1")

	err := twf.Apply(s.ctx, s.bus)
	s.Require().NoError(err)
	s.True(twf.IsApplied())

	err = twf.Apply(s.ctx, s.bus)
	s.Error(err)

	err = twf.Remove(s.ctx, s.bus)
	s.Require().NoError(err)
	s.False(twf.IsApplied())
}

func (s *FightingStyleTwoWeaponFightingTestSuite) TestAddsDamageToOffHandAttack() {
	twf := conditions.NewFightingStyleTwoWeaponFightingCondition("fighter-1")

	err := twf.Apply(s.ctx, s.bus)
	s.Require().NoError(err)
	defer func() { _ = twf.Remove(s.ctx, s.bus) }()

	// Create damage chain event for off-hand attack
	// Normally off-hand attacks don't add ability modifier to damage
	// But with Two-Weapon Fighting, they do
	damageEvent := &dnd5eEvents.DamageChainEvent{
		AttackerID:      "fighter-1",
		TargetID:        "goblin-1",
		IsOffHandAttack: true, // This is an off-hand attack
		AbilityModifier: 3,    // STR or DEX modifier to add
		Components: []dnd5eEvents.DamageComponent{
			{
				Source:            dnd5eEvents.DamageSourceWeapon,
				OriginalDiceRolls: []int{4},
				FinalDiceRolls:    []int{4},
				FlatBonus:         0, // No ability modifier added yet
				DamageType:        damage.Slashing,
			},
		},
		DamageType: damage.Slashing,
	}

	// Execute through damage chain
	damageChain := events.NewStagedChain[*dnd5eEvents.DamageChainEvent](combat.ModifierStages)
	damages := dnd5eEvents.DamageChain.On(s.bus)
	modifiedChain, err := damages.PublishWithChain(s.ctx, damageEvent, damageChain)
	s.Require().NoError(err)

	finalEvent, err := modifiedChain.Execute(s.ctx, damageEvent)
	s.Require().NoError(err)

	// Should have 2 components: weapon + TWF ability modifier
	s.Len(finalEvent.Components, 2)
	s.Equal(3, finalEvent.Components[1].FlatBonus) // Ability modifier added
}

func (s *FightingStyleTwoWeaponFightingTestSuite) TestDoesNotAddToMainHandAttack() {
	twf := conditions.NewFightingStyleTwoWeaponFightingCondition("fighter-1")

	err := twf.Apply(s.ctx, s.bus)
	s.Require().NoError(err)
	defer func() { _ = twf.Remove(s.ctx, s.bus) }()

	// Create damage chain event for main-hand attack (not off-hand)
	damageEvent := &dnd5eEvents.DamageChainEvent{
		AttackerID:      "fighter-1",
		TargetID:        "goblin-1",
		IsOffHandAttack: false, // Main-hand attack
		AbilityModifier: 3,
		Components: []dnd5eEvents.DamageComponent{
			{
				Source:            dnd5eEvents.DamageSourceWeapon,
				OriginalDiceRolls: []int{8},
				FinalDiceRolls:    []int{8},
				FlatBonus:         3, // Already has ability modifier
				DamageType:        damage.Slashing,
			},
		},
		DamageType: damage.Slashing,
	}

	// Execute through damage chain
	damageChain := events.NewStagedChain[*dnd5eEvents.DamageChainEvent](combat.ModifierStages)
	damages := dnd5eEvents.DamageChain.On(s.bus)
	modifiedChain, err := damages.PublishWithChain(s.ctx, damageEvent, damageChain)
	s.Require().NoError(err)

	finalEvent, err := modifiedChain.Execute(s.ctx, damageEvent)
	s.Require().NoError(err)

	// Should only have 1 component (no TWF bonus for main-hand)
	s.Len(finalEvent.Components, 1)
}

func (s *FightingStyleTwoWeaponFightingTestSuite) TestDoesNotAddToOtherCharacter() {
	twf := conditions.NewFightingStyleTwoWeaponFightingCondition("fighter-1")

	err := twf.Apply(s.ctx, s.bus)
	s.Require().NoError(err)
	defer func() { _ = twf.Remove(s.ctx, s.bus) }()

	// Create damage chain event for different character
	damageEvent := &dnd5eEvents.DamageChainEvent{
		AttackerID:      "rogue-1", // Different character
		TargetID:        "goblin-1",
		IsOffHandAttack: true,
		AbilityModifier: 4,
		Components: []dnd5eEvents.DamageComponent{
			{
				Source:            dnd5eEvents.DamageSourceWeapon,
				OriginalDiceRolls: []int{4},
				FinalDiceRolls:    []int{4},
				FlatBonus:         0,
				DamageType:        damage.Piercing,
			},
		},
		DamageType: damage.Piercing,
	}

	// Execute through damage chain
	damageChain := events.NewStagedChain[*dnd5eEvents.DamageChainEvent](combat.ModifierStages)
	damages := dnd5eEvents.DamageChain.On(s.bus)
	modifiedChain, err := damages.PublishWithChain(s.ctx, damageEvent, damageChain)
	s.Require().NoError(err)

	finalEvent, err := modifiedChain.Execute(s.ctx, damageEvent)
	s.Require().NoError(err)

	// Should only have 1 component (TWF only applies to fighter-1)
	s.Len(finalEvent.Components, 1)
}

func (s *FightingStyleTwoWeaponFightingTestSuite) TestToJSON() {
	twf := conditions.NewFightingStyleTwoWeaponFightingCondition("fighter-1")

	jsonData, err := twf.ToJSON()
	s.Require().NoError(err)
	s.Contains(string(jsonData), refs.Conditions.FightingStyleTwoWeaponFighting().ID)
	s.Contains(string(jsonData), "fighter-1")
}
