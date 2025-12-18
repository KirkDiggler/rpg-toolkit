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
	dnd5eEvents "github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/events"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/refs"
)

type FightingStyleArcheryTestSuite struct {
	suite.Suite
	ctx context.Context
	bus events.EventBus
}

func (s *FightingStyleArcheryTestSuite) SetupTest() {
	s.ctx = context.Background()
	s.bus = events.NewEventBus()
}

func TestFightingStyleArcherySuite(t *testing.T) {
	suite.Run(t, new(FightingStyleArcheryTestSuite))
}

func (s *FightingStyleArcheryTestSuite) TestNewFightingStyleArcheryCondition() {
	// Create Archery fighting style condition
	archery := conditions.NewFightingStyleArcheryCondition("fighter-1")

	s.NotNil(archery)
	s.False(archery.IsApplied())
}

func (s *FightingStyleArcheryTestSuite) TestApplyAndRemove() {
	archery := conditions.NewFightingStyleArcheryCondition("fighter-1")

	// Apply should work
	err := archery.Apply(s.ctx, s.bus)
	s.Require().NoError(err)
	s.True(archery.IsApplied())

	// Double apply should error
	err = archery.Apply(s.ctx, s.bus)
	s.Error(err)

	// Remove should work
	err = archery.Remove(s.ctx, s.bus)
	s.Require().NoError(err)
	s.False(archery.IsApplied())
}

func (s *FightingStyleArcheryTestSuite) TestAddsToRangedAttacks() {
	archery := conditions.NewFightingStyleArcheryCondition("fighter-1")

	err := archery.Apply(s.ctx, s.bus)
	s.Require().NoError(err)
	defer func() { _ = archery.Remove(s.ctx, s.bus) }()

	// Create attack chain event for ranged attack
	attackEvent := dnd5eEvents.AttackChainEvent{
		AttackerID:        "fighter-1",
		TargetID:          "goblin-1",
		IsMelee:           false, // Ranged attack
		AttackBonus:       5,     // DEX(3) + Prof(2)
		TargetAC:          13,
		CriticalThreshold: 20,
	}

	// Execute through attack chain
	attackChain := events.NewStagedChain[dnd5eEvents.AttackChainEvent](combat.ModifierStages)
	attacks := dnd5eEvents.AttackChain.On(s.bus)
	modifiedChain, err := attacks.PublishWithChain(s.ctx, attackEvent, attackChain)
	s.Require().NoError(err)

	finalEvent, err := modifiedChain.Execute(s.ctx, attackEvent)
	s.Require().NoError(err)

	// Archery adds +2 to ranged attacks
	s.Equal(7, finalEvent.AttackBonus)
}

func (s *FightingStyleArcheryTestSuite) TestDoesNotAddToMeleeAttacks() {
	archery := conditions.NewFightingStyleArcheryCondition("fighter-1")

	err := archery.Apply(s.ctx, s.bus)
	s.Require().NoError(err)
	defer func() { _ = archery.Remove(s.ctx, s.bus) }()

	// Create attack chain event for melee attack
	attackEvent := dnd5eEvents.AttackChainEvent{
		AttackerID:        "fighter-1",
		TargetID:          "goblin-1",
		IsMelee:           true, // Melee attack
		AttackBonus:       5,    // STR(3) + Prof(2)
		TargetAC:          13,
		CriticalThreshold: 20,
	}

	// Execute through attack chain
	attackChain := events.NewStagedChain[dnd5eEvents.AttackChainEvent](combat.ModifierStages)
	attacks := dnd5eEvents.AttackChain.On(s.bus)
	modifiedChain, err := attacks.PublishWithChain(s.ctx, attackEvent, attackChain)
	s.Require().NoError(err)

	finalEvent, err := modifiedChain.Execute(s.ctx, attackEvent)
	s.Require().NoError(err)

	// Archery does NOT add to melee attacks
	s.Equal(5, finalEvent.AttackBonus)
}

func (s *FightingStyleArcheryTestSuite) TestDoesNotAddToOtherCharacterAttacks() {
	archery := conditions.NewFightingStyleArcheryCondition("fighter-1")

	err := archery.Apply(s.ctx, s.bus)
	s.Require().NoError(err)
	defer func() { _ = archery.Remove(s.ctx, s.bus) }()

	// Create attack chain event for different character
	attackEvent := dnd5eEvents.AttackChainEvent{
		AttackerID:        "ranger-1", // Different character
		TargetID:          "goblin-1",
		IsMelee:           false, // Ranged
		AttackBonus:       5,
		TargetAC:          13,
		CriticalThreshold: 20,
	}

	// Execute through attack chain
	attackChain := events.NewStagedChain[dnd5eEvents.AttackChainEvent](combat.ModifierStages)
	attacks := dnd5eEvents.AttackChain.On(s.bus)
	modifiedChain, err := attacks.PublishWithChain(s.ctx, attackEvent, attackChain)
	s.Require().NoError(err)

	finalEvent, err := modifiedChain.Execute(s.ctx, attackEvent)
	s.Require().NoError(err)

	// Should not modify other character's attacks
	s.Equal(5, finalEvent.AttackBonus)
}

func (s *FightingStyleArcheryTestSuite) TestToJSON() {
	archery := conditions.NewFightingStyleArcheryCondition("fighter-1")

	jsonData, err := archery.ToJSON()
	s.Require().NoError(err)
	s.Contains(string(jsonData), refs.Conditions.FightingStyleArchery().ID)
	s.Contains(string(jsonData), "fighter-1")
}
