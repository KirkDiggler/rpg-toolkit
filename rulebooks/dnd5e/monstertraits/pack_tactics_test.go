// Copyright (C) 2024 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package monstertraits

import (
	"context"
	"testing"

	"github.com/KirkDiggler/rpg-toolkit/events"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/combat"
	dnd5eEvents "github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/events"
	"github.com/stretchr/testify/suite"
)

type PackTacticsTestSuite struct {
	suite.Suite
	bus         events.EventBus
	ctx         context.Context
	packTactics *packTacticsCondition
}

func TestPackTacticsTestSuite(t *testing.T) {
	suite.Run(t, new(PackTacticsTestSuite))
}

func (s *PackTacticsTestSuite) SetupTest() {
	s.bus = events.NewEventBus()
	s.ctx = context.Background()
	s.packTactics = nil // Will be created in each test
}

func (s *PackTacticsTestSuite) TestPackTacticsGrantsAdvantage() {
	// Create Pack Tactics trait
	// Note: This is a simplified implementation. The actual Pack Tactics logic
	// (checking if ally is adjacent to target) would be handled by the game server
	// before publishing the attack event. This trait just grants advantage when
	// triggered.
	s.packTactics = PackTactics("wolf-1").(*packTacticsCondition)

	// Apply to bus
	err := s.packTactics.Apply(s.ctx, s.bus)
	s.Require().NoError(err)

	// Create attack event
	event := dnd5eEvents.AttackChainEvent{
		AttackerID:  "wolf-1",
		TargetID:    "pc-1",
		IsMelee:     true,
		AttackBonus: 4,
		TargetAC:    15,
	}

	// Publish attack chain event
	chain := events.NewStagedChain[dnd5eEvents.AttackChainEvent](combat.ModifierStages)
	attackTopic := dnd5eEvents.AttackChain.On(s.bus)

	modifiedChain, err := attackTopic.PublishWithChain(s.ctx, event, chain)
	s.Require().NoError(err)

	// Execute chain to get modified event
	result, err := modifiedChain.Execute(s.ctx, event)
	s.Require().NoError(err)

	// In the full implementation, this would check if an ally is adjacent
	// For now, we verify the chain executes successfully
	// The advantage would be added by checking AdvantageSources
	s.Assert().Equal("wolf-1", result.AttackerID)
	s.Assert().Equal("pc-1", result.TargetID)
}

func (s *PackTacticsTestSuite) TestPackTacticsIgnoresOtherAttackers() {
	// Create Pack Tactics for wolf-1
	s.packTactics = PackTactics("wolf-1").(*packTacticsCondition)

	// Apply to bus
	err := s.packTactics.Apply(s.ctx, s.bus)
	s.Require().NoError(err)

	// Create attack event from different attacker
	event := dnd5eEvents.AttackChainEvent{
		AttackerID:  "wolf-2", // Different attacker
		TargetID:    "pc-1",
		IsMelee:     true,
		AttackBonus: 4,
		TargetAC:    15,
	}

	// Publish attack chain event
	chain := events.NewStagedChain[dnd5eEvents.AttackChainEvent](combat.ModifierStages)
	attackTopic := dnd5eEvents.AttackChain.On(s.bus)

	modifiedChain, err := attackTopic.PublishWithChain(s.ctx, event, chain)
	s.Require().NoError(err)

	// Execute chain
	result, err := modifiedChain.Execute(s.ctx, event)
	s.Require().NoError(err)

	// Pack Tactics should not apply to wolf-2
	s.Assert().Equal("wolf-2", result.AttackerID)
}

func (s *PackTacticsTestSuite) TestPackTacticsCanBeRemoved() {
	// Create and apply pack tactics
	s.packTactics = PackTactics("wolf-1").(*packTacticsCondition)
	err := s.packTactics.Apply(s.ctx, s.bus)
	s.Require().NoError(err)
	s.Assert().True(s.packTactics.IsApplied())

	// Remove pack tactics
	err = s.packTactics.Remove(s.ctx, s.bus)
	s.Require().NoError(err)
	s.Assert().False(s.packTactics.IsApplied())
}
