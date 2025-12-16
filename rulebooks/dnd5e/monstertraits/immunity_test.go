// Copyright (C) 2024 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

//nolint:dupl // Trait tests follow same event-driven pattern with different conditions
package monstertraits

import (
	"context"
	"testing"

	"github.com/KirkDiggler/rpg-toolkit/events"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/combat"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/damage"
	dnd5eEvents "github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/events"
	"github.com/stretchr/testify/suite"
)

type ImmunityTestSuite struct {
	suite.Suite
	bus      events.EventBus
	ctx      context.Context
	immunity *immunityCondition
}

func TestImmunityTestSuite(t *testing.T) {
	suite.Run(t, new(ImmunityTestSuite))
}

func (s *ImmunityTestSuite) SetupTest() {
	s.bus = events.NewEventBus()
	s.ctx = context.Background()
	s.immunity = nil // Will be created in each test
}

func (s *ImmunityTestSuite) TestImmunityReducesDamageToZero() {
	// Create immunity to poison
	s.immunity = Immunity("monster-1", damage.Poison).(*immunityCondition)

	// Apply to bus
	err := s.immunity.Apply(s.ctx, s.bus)
	s.Require().NoError(err)

	// Create damage event with poison damage
	event := &dnd5eEvents.DamageChainEvent{
		AttackerID: "pc-1",
		TargetID:   "monster-1",
		Components: []dnd5eEvents.DamageComponent{
			{
				Source:         dnd5eEvents.DamageSourceWeapon,
				FinalDiceRolls: []int{3, 4}, // 7 damage
				FlatBonus:      2,           // Total: 9 poison damage
				DamageType:     damage.Poison,
			},
		},
		DamageType: damage.Poison,
	}

	// Publish damage chain event
	chain := events.NewStagedChain[*dnd5eEvents.DamageChainEvent](combat.ModifierStages)
	damageTopic := dnd5eEvents.DamageChain.On(s.bus)

	modifiedChain, err := damageTopic.PublishWithChain(s.ctx, event, chain)
	s.Require().NoError(err)

	// Execute chain to get modified event
	result, err := modifiedChain.Execute(s.ctx, event)
	s.Require().NoError(err)

	// Verify damage was reduced to 0
	s.Require().Len(result.Components, 1)
	s.Assert().Equal(0, result.Components[0].Total())
}

func (s *ImmunityTestSuite) TestImmunityDoesNotAffectOtherDamageTypes() {
	// Create immunity to poison
	s.immunity = Immunity("monster-1", damage.Poison).(*immunityCondition)

	// Apply to bus
	err := s.immunity.Apply(s.ctx, s.bus)
	s.Require().NoError(err)

	// Create damage event with slashing damage (not immune)
	event := &dnd5eEvents.DamageChainEvent{
		AttackerID: "pc-1",
		TargetID:   "monster-1",
		Components: []dnd5eEvents.DamageComponent{
			{
				Source:         dnd5eEvents.DamageSourceWeapon,
				FinalDiceRolls: []int{3, 4}, // 7 damage
				FlatBonus:      2,           // Total: 9 slashing damage
				DamageType:     damage.Slashing,
			},
		},
		DamageType: damage.Slashing,
	}

	// Publish damage chain event
	chain := events.NewStagedChain[*dnd5eEvents.DamageChainEvent](combat.ModifierStages)
	damageTopic := dnd5eEvents.DamageChain.On(s.bus)

	modifiedChain, err := damageTopic.PublishWithChain(s.ctx, event, chain)
	s.Require().NoError(err)

	// Execute chain to get modified event
	result, err := modifiedChain.Execute(s.ctx, event)
	s.Require().NoError(err)

	// Verify damage was NOT modified
	s.Require().Len(result.Components, 1)
	s.Assert().Equal(9, result.Components[0].Total())
}

func (s *ImmunityTestSuite) TestImmunityIgnoresOtherTargets() {
	// Create immunity to poison for monster-1
	s.immunity = Immunity("monster-1", damage.Poison).(*immunityCondition)

	// Apply to bus
	err := s.immunity.Apply(s.ctx, s.bus)
	s.Require().NoError(err)

	// Create damage event targeting different monster
	event := &dnd5eEvents.DamageChainEvent{
		AttackerID: "pc-1",
		TargetID:   "monster-2", // Different target
		Components: []dnd5eEvents.DamageComponent{
			{
				Source:         dnd5eEvents.DamageSourceWeapon,
				FinalDiceRolls: []int{3, 4}, // 7 damage
				FlatBonus:      2,           // Total: 9 poison damage
				DamageType:     damage.Poison,
			},
		},
		DamageType: damage.Poison,
	}

	// Publish damage chain event
	chain := events.NewStagedChain[*dnd5eEvents.DamageChainEvent](combat.ModifierStages)
	damageTopic := dnd5eEvents.DamageChain.On(s.bus)

	modifiedChain, err := damageTopic.PublishWithChain(s.ctx, event, chain)
	s.Require().NoError(err)

	// Execute chain to get modified event
	result, err := modifiedChain.Execute(s.ctx, event)
	s.Require().NoError(err)

	// Verify damage was NOT modified (wrong target)
	s.Require().Len(result.Components, 1)
	s.Assert().Equal(9, result.Components[0].Total())
}

func (s *ImmunityTestSuite) TestImmunityCanBeRemoved() {
	// Create and apply immunity
	s.immunity = Immunity("monster-1", damage.Poison).(*immunityCondition)
	err := s.immunity.Apply(s.ctx, s.bus)
	s.Require().NoError(err)
	s.Assert().True(s.immunity.IsApplied())

	// Remove immunity
	err = s.immunity.Remove(s.ctx, s.bus)
	s.Require().NoError(err)
	s.Assert().False(s.immunity.IsApplied())
}
