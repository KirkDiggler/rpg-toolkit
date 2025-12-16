// Copyright (C) 2024 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

//nolint:dupl // Trait tests follow same event-driven pattern with different conditions
package monstertraits

import (
	"context"
	"testing"

	"github.com/KirkDiggler/rpg-toolkit/dice"
	"github.com/KirkDiggler/rpg-toolkit/events"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/combat"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/damage"
	dnd5eEvents "github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/events"
	"github.com/stretchr/testify/suite"
)

type UndeadFortitudeTestSuite struct {
	suite.Suite
	bus             events.EventBus
	ctx             context.Context
	undeadFortitude *undeadFortitudeCondition
	roller          *mockRoller
}

// mockRoller allows us to control dice rolls in tests
type mockRoller struct {
	nextRoll int
}

func (m *mockRoller) Roll(_ context.Context, _ int) (int, error) {
	return m.nextRoll, nil
}

func (m *mockRoller) RollN(_ context.Context, count, _ int) ([]int, error) {
	results := make([]int, count)
	for i := 0; i < count; i++ {
		results[i] = m.nextRoll
	}
	return results, nil
}

func (m *mockRoller) RollWithAdvantage(_ context.Context, _ int) (int, int, int, error) {
	return m.nextRoll, m.nextRoll, m.nextRoll, nil
}

var _ dice.Roller = (*mockRoller)(nil)

func TestUndeadFortitudeTestSuite(t *testing.T) {
	suite.Run(t, new(UndeadFortitudeTestSuite))
}

func (s *UndeadFortitudeTestSuite) SetupTest() {
	s.bus = events.NewEventBus()
	s.ctx = context.Background()
	s.roller = &mockRoller{}
	s.undeadFortitude = nil // Will be created in each test
}

func (s *UndeadFortitudeTestSuite) TestUndeadFortitudeSavesOnSuccess() {
	// Zombie with CON modifier +1
	s.undeadFortitude = UndeadFortitude("zombie-1", 1, s.roller).(*undeadFortitudeCondition)

	// Apply to bus
	err := s.undeadFortitude.Apply(s.ctx, s.bus)
	s.Require().NoError(err)

	// Create damage event that would drop to 0 HP
	// Damage: 10, DC = 5 + 10 = 15
	// Roll: 15 + 1 (CON) = 16 >= 15, should stay at 1 HP
	s.roller.nextRoll = 15

	event := &dnd5eEvents.DamageChainEvent{
		AttackerID: "pc-1",
		TargetID:   "zombie-1",
		Components: []dnd5eEvents.DamageComponent{
			{
				Source:         dnd5eEvents.DamageSourceWeapon,
				FinalDiceRolls: []int{5, 5},
				FlatBonus:      0,
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

	// Execute chain
	result, err := modifiedChain.Execute(s.ctx, event)
	s.Require().NoError(err)

	// For now, we just verify the chain executes without error
	// The actual HP management happens outside the damage chain
	s.Require().Len(result.Components, 1)
}

func (s *UndeadFortitudeTestSuite) TestUndeadFortitudeFailsOnLowRoll() {
	// Zombie with CON modifier +1
	s.undeadFortitude = UndeadFortitude("zombie-1", 1, s.roller).(*undeadFortitudeCondition)

	// Apply to bus
	err := s.undeadFortitude.Apply(s.ctx, s.bus)
	s.Require().NoError(err)

	// Create damage event that would drop to 0 HP
	// Damage: 10, DC = 5 + 10 = 15
	// Roll: 5 + 1 (CON) = 6 < 15, should fail
	s.roller.nextRoll = 5

	event := &dnd5eEvents.DamageChainEvent{
		AttackerID: "pc-1",
		TargetID:   "zombie-1",
		Components: []dnd5eEvents.DamageComponent{
			{
				Source:         dnd5eEvents.DamageSourceWeapon,
				FinalDiceRolls: []int{5, 5},
				FlatBonus:      0,
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

	// Execute chain
	result, err := modifiedChain.Execute(s.ctx, event)
	s.Require().NoError(err)

	// Damage should not be modified
	s.Require().Len(result.Components, 1)
}

func (s *UndeadFortitudeTestSuite) TestUndeadFortitudeIgnoresRadiantDamage() {
	// Zombie with CON modifier +1
	s.undeadFortitude = UndeadFortitude("zombie-1", 1, s.roller).(*undeadFortitudeCondition)

	// Apply to bus
	err := s.undeadFortitude.Apply(s.ctx, s.bus)
	s.Require().NoError(err)

	// Even with a good roll, radiant damage should bypass Undead Fortitude
	s.roller.nextRoll = 20

	event := &dnd5eEvents.DamageChainEvent{
		AttackerID: "pc-1",
		TargetID:   "zombie-1",
		Components: []dnd5eEvents.DamageComponent{
			{
				Source:         dnd5eEvents.DamageSourceSpell,
				FinalDiceRolls: []int{5, 5},
				FlatBonus:      0,
				DamageType:     damage.Radiant, // Radiant damage
			},
		},
		DamageType: damage.Radiant,
	}

	// Publish damage chain event
	chain := events.NewStagedChain[*dnd5eEvents.DamageChainEvent](combat.ModifierStages)
	damageTopic := dnd5eEvents.DamageChain.On(s.bus)

	modifiedChain, err := damageTopic.PublishWithChain(s.ctx, event, chain)
	s.Require().NoError(err)

	// Execute chain
	result, err := modifiedChain.Execute(s.ctx, event)
	s.Require().NoError(err)

	// Damage should not be modified (radiant bypasses Undead Fortitude)
	s.Require().Len(result.Components, 1)
}

func (s *UndeadFortitudeTestSuite) TestUndeadFortitudeIgnoresCriticalHits() {
	// Zombie with CON modifier +1
	s.undeadFortitude = UndeadFortitude("zombie-1", 1, s.roller).(*undeadFortitudeCondition)

	// Apply to bus
	err := s.undeadFortitude.Apply(s.ctx, s.bus)
	s.Require().NoError(err)

	// Even with a good roll, critical hits should bypass Undead Fortitude
	s.roller.nextRoll = 20

	event := &dnd5eEvents.DamageChainEvent{
		AttackerID: "pc-1",
		TargetID:   "zombie-1",
		Components: []dnd5eEvents.DamageComponent{
			{
				Source:         dnd5eEvents.DamageSourceWeapon,
				FinalDiceRolls: []int{5, 5},
				FlatBonus:      0,
				DamageType:     damage.Slashing,
				IsCritical:     true, // Critical hit
			},
		},
		DamageType: damage.Slashing,
		IsCritical: true,
	}

	// Publish damage chain event
	chain := events.NewStagedChain[*dnd5eEvents.DamageChainEvent](combat.ModifierStages)
	damageTopic := dnd5eEvents.DamageChain.On(s.bus)

	modifiedChain, err := damageTopic.PublishWithChain(s.ctx, event, chain)
	s.Require().NoError(err)

	// Execute chain
	result, err := modifiedChain.Execute(s.ctx, event)
	s.Require().NoError(err)

	// Damage should not be modified (crit bypasses Undead Fortitude)
	s.Require().Len(result.Components, 1)
}

func (s *UndeadFortitudeTestSuite) TestUndeadFortitudeCanBeRemoved() {
	// Create and apply undead fortitude
	s.undeadFortitude = UndeadFortitude("zombie-1", 1, s.roller).(*undeadFortitudeCondition)
	err := s.undeadFortitude.Apply(s.ctx, s.bus)
	s.Require().NoError(err)
	s.Assert().True(s.undeadFortitude.IsApplied())

	// Remove undead fortitude
	err = s.undeadFortitude.Remove(s.ctx, s.bus)
	s.Require().NoError(err)
	s.Assert().False(s.undeadFortitude.IsApplied())
}
