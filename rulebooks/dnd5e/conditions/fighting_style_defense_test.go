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
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/refs"
)

type FightingStyleDefenseTestSuite struct {
	suite.Suite
	ctx context.Context
	bus events.EventBus
}

func (s *FightingStyleDefenseTestSuite) SetupTest() {
	s.ctx = context.Background()
	s.bus = events.NewEventBus()
}

func TestFightingStyleDefenseSuite(t *testing.T) {
	suite.Run(t, new(FightingStyleDefenseTestSuite))
}

func (s *FightingStyleDefenseTestSuite) TestNewFightingStyleDefenseCondition() {
	defense := conditions.NewFightingStyleDefenseCondition("fighter-1")

	s.NotNil(defense)
	s.False(defense.IsApplied())
}

func (s *FightingStyleDefenseTestSuite) TestApplyAndRemove() {
	defense := conditions.NewFightingStyleDefenseCondition("fighter-1")

	err := defense.Apply(s.ctx, s.bus)
	s.Require().NoError(err)
	s.True(defense.IsApplied())

	err = defense.Apply(s.ctx, s.bus)
	s.Error(err)

	err = defense.Remove(s.ctx, s.bus)
	s.Require().NoError(err)
	s.False(defense.IsApplied())
}

func (s *FightingStyleDefenseTestSuite) TestAddsACWhenWearingArmor() {
	defense := conditions.NewFightingStyleDefenseCondition("fighter-1")

	err := defense.Apply(s.ctx, s.bus)
	s.Require().NoError(err)
	defer func() { _ = defense.Remove(s.ctx, s.bus) }()

	// Create AC chain event with armor
	acEvent := &combat.ACChainEvent{
		CharacterID: "fighter-1",
		HasArmor:    true, // Wearing armor
		Breakdown:   &combat.ACBreakdown{Total: 10},
	}

	// Execute through AC chain
	acChain := events.NewStagedChain[*combat.ACChainEvent](combat.ModifierStages)
	acChainTopic := combat.ACChain.On(s.bus)
	modifiedChain, err := acChainTopic.PublishWithChain(s.ctx, acEvent, acChain)
	s.Require().NoError(err)

	finalEvent, err := modifiedChain.Execute(s.ctx, acEvent)
	s.Require().NoError(err)

	// Defense adds +1 AC when wearing armor
	s.Equal(11, finalEvent.Breakdown.Total)
}

func (s *FightingStyleDefenseTestSuite) TestDoesNotAddACWithoutArmor() {
	defense := conditions.NewFightingStyleDefenseCondition("fighter-1")

	err := defense.Apply(s.ctx, s.bus)
	s.Require().NoError(err)
	defer func() { _ = defense.Remove(s.ctx, s.bus) }()

	// Create AC chain event without armor
	acEvent := &combat.ACChainEvent{
		CharacterID: "fighter-1",
		HasArmor:    false, // Not wearing armor
		Breakdown:   &combat.ACBreakdown{Total: 10},
	}

	// Execute through AC chain
	acChain := events.NewStagedChain[*combat.ACChainEvent](combat.ModifierStages)
	acChainTopic := combat.ACChain.On(s.bus)
	modifiedChain, err := acChainTopic.PublishWithChain(s.ctx, acEvent, acChain)
	s.Require().NoError(err)

	finalEvent, err := modifiedChain.Execute(s.ctx, acEvent)
	s.Require().NoError(err)

	// Defense does NOT add AC without armor
	s.Equal(10, finalEvent.Breakdown.Total)
}

func (s *FightingStyleDefenseTestSuite) TestDoesNotAddToOtherCharacters() {
	defense := conditions.NewFightingStyleDefenseCondition("fighter-1")

	err := defense.Apply(s.ctx, s.bus)
	s.Require().NoError(err)
	defer func() { _ = defense.Remove(s.ctx, s.bus) }()

	// Create AC chain event for different character
	acEvent := &combat.ACChainEvent{
		CharacterID: "paladin-1", // Different character
		HasArmor:    true,
		Breakdown:   &combat.ACBreakdown{Total: 10},
	}

	// Execute through AC chain
	acChain := events.NewStagedChain[*combat.ACChainEvent](combat.ModifierStages)
	acChainTopic := combat.ACChain.On(s.bus)
	modifiedChain, err := acChainTopic.PublishWithChain(s.ctx, acEvent, acChain)
	s.Require().NoError(err)

	finalEvent, err := modifiedChain.Execute(s.ctx, acEvent)
	s.Require().NoError(err)

	// Should not modify other character's AC
	s.Equal(10, finalEvent.Breakdown.Total)
}

func (s *FightingStyleDefenseTestSuite) TestToJSON() {
	defense := conditions.NewFightingStyleDefenseCondition("fighter-1")

	jsonData, err := defense.ToJSON()
	s.Require().NoError(err)
	s.Contains(string(jsonData), refs.Conditions.FightingStyleDefense().ID)
	s.Contains(string(jsonData), "fighter-1")
}
