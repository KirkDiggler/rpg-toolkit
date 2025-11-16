// Copyright (C) 2024 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package conditions

import (
	"context"
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/KirkDiggler/rpg-toolkit/events"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/combat"
)

// ragingConditionInput provides configuration for creating a raging condition
type ragingConditionInput struct {
	CharacterID string // ID of the raging character
	DamageBonus int    // Bonus damage for rage
	Level       int    // Barbarian level
	Source      string // What triggered this (feature ID)
}

// newRagingCondition creates a raging condition from input
func newRagingCondition(input ragingConditionInput) *RagingCondition {
	return &RagingCondition{
		CharacterID: input.CharacterID,
		DamageBonus: input.DamageBonus,
		Level:       input.Level,
		Source:      input.Source,
	}
}

// RagingConditionTestSuite tests the RagingCondition behavior
type RagingConditionTestSuite struct {
	suite.Suite
	ctx context.Context
	bus events.EventBus
}

func (s *RagingConditionTestSuite) SetupTest() {
	s.ctx = context.Background()
	s.bus = events.NewEventBus()
}

func TestRagingConditionTestSuite(t *testing.T) {
	suite.Run(t, new(RagingConditionTestSuite))
}

func (s *RagingConditionTestSuite) TestRagingConditionTracksAttacks() {
	// Create a raging condition
	raging := newRagingCondition(ragingConditionInput{
		CharacterID: "barbarian-1",
		DamageBonus: 2,
		Level:       5,
		Source:      "rage-feature",
	})

	// Apply it to subscribe to events
	err := raging.Apply(s.ctx, s.bus)
	s.Require().NoError(err)

	// Verify initial state
	s.False(raging.DidAttackThisTurn)

	// Publish an attack event for this character
	attackTopic := dnd5e.AttackTopic.On(s.bus)
	err = attackTopic.Publish(s.ctx, dnd5e.AttackEvent{
		AttackerID: "barbarian-1",
		TargetID:   "goblin-1",
		WeaponRef:  "greatsword",
		IsMelee:    true,
	})
	s.Require().NoError(err)

	// Check that the condition tracked the attack
	s.True(raging.DidAttackThisTurn)
}

func (s *RagingConditionTestSuite) TestRagingConditionTracksDamage() {
	// Create a raging condition
	raging := newRagingCondition(ragingConditionInput{
		CharacterID: "barbarian-1",
		DamageBonus: 2,
		Level:       5,
		Source:      "rage-feature",
	})

	// Apply it to subscribe to events
	err := raging.Apply(s.ctx, s.bus)
	s.Require().NoError(err)

	// Verify initial state
	s.False(raging.WasHitThisTurn)

	// Publish a damage event for this character
	damageTopic := dnd5e.DamageReceivedTopic.On(s.bus)
	err = damageTopic.Publish(s.ctx, dnd5e.DamageReceivedEvent{
		TargetID:   "barbarian-1",
		SourceID:   "goblin-1",
		Amount:     5,
		DamageType: "slashing",
	})
	s.Require().NoError(err)

	// Check that the condition tracked being hit
	s.True(raging.WasHitThisTurn)
}

func (s *RagingConditionTestSuite) TestRagingConditionEndsWithoutCombatActivity() {
	// Create a raging condition
	raging := newRagingCondition(ragingConditionInput{
		CharacterID: "barbarian-1",
		DamageBonus: 2,
		Level:       5,
		Source:      "rage-feature",
	})

	// Apply it to subscribe to events
	err := raging.Apply(s.ctx, s.bus)
	s.Require().NoError(err)

	// Track if condition removed event is published
	var removedEvent *dnd5e.ConditionRemovedEvent
	removalTopic := dnd5e.ConditionRemovedTopic.On(s.bus)
	_, err = removalTopic.Subscribe(s.ctx, func(_ context.Context, event dnd5e.ConditionRemovedEvent) error {
		removedEvent = &event
		return nil
	})
	s.Require().NoError(err)

	// Publish turn end event without any combat activity
	turnEndTopic := dnd5e.TurnEndTopic.On(s.bus)
	err = turnEndTopic.Publish(s.ctx, dnd5e.TurnEndEvent{
		CharacterID: "barbarian-1",
		Round:       1,
	})
	s.Require().NoError(err)

	// Verify condition published removal event
	s.Require().NotNil(removedEvent)
	s.Equal("barbarian-1", removedEvent.CharacterID)
	s.Equal("dnd5e:conditions:raging", removedEvent.ConditionRef)
	s.Equal("no_combat_activity", removedEvent.Reason)
}

func (s *RagingConditionTestSuite) TestRagingConditionContinuesWithCombatActivity() {
	// Create a raging condition
	raging := newRagingCondition(ragingConditionInput{
		CharacterID: "barbarian-1",
		DamageBonus: 2,
		Level:       5,
		Source:      "rage-feature",
	})

	// Apply it to subscribe to events
	err := raging.Apply(s.ctx, s.bus)
	s.Require().NoError(err)

	// Track if condition removed event is published
	var removedEvent *dnd5e.ConditionRemovedEvent
	removalTopic := dnd5e.ConditionRemovedTopic.On(s.bus)
	_, err = removalTopic.Subscribe(s.ctx, func(_ context.Context, event dnd5e.ConditionRemovedEvent) error {
		removedEvent = &event
		return nil
	})
	s.Require().NoError(err)

	// Publish an attack event (combat activity)
	attackTopic := dnd5e.AttackTopic.On(s.bus)
	err = attackTopic.Publish(s.ctx, dnd5e.AttackEvent{
		AttackerID: "barbarian-1",
		TargetID:   "goblin-1",
		WeaponRef:  "greatsword",
		IsMelee:    true,
	})
	s.Require().NoError(err)

	// Publish turn end event
	turnEndTopic := dnd5e.TurnEndTopic.On(s.bus)
	err = turnEndTopic.Publish(s.ctx, dnd5e.TurnEndEvent{
		CharacterID: "barbarian-1",
		Round:       1,
	})
	s.Require().NoError(err)

	// Verify condition did NOT publish removal event
	s.Nil(removedEvent, "Rage should continue when there's combat activity")

	// Verify flags were reset for next turn
	s.False(raging.DidAttackThisTurn)
	s.False(raging.WasHitThisTurn)
	s.Equal(1, raging.TurnsActive)
}

func (s *RagingConditionTestSuite) TestRagingConditionEndsAfter10Rounds() {
	// Create a raging condition
	raging := newRagingCondition(ragingConditionInput{
		CharacterID: "barbarian-1",
		DamageBonus: 2,
		Level:       5,
		Source:      "rage-feature",
	})

	// Apply it to subscribe to events
	err := raging.Apply(s.ctx, s.bus)
	s.Require().NoError(err)

	// Track if condition removed event is published
	var removedEvent *dnd5e.ConditionRemovedEvent
	removalTopic := dnd5e.ConditionRemovedTopic.On(s.bus)
	_, err = removalTopic.Subscribe(s.ctx, func(_ context.Context, event dnd5e.ConditionRemovedEvent) error {
		removedEvent = &event
		return nil
	})
	s.Require().NoError(err)

	attackTopic := dnd5e.AttackTopic.On(s.bus)
	turnEndTopic := dnd5e.TurnEndTopic.On(s.bus)

	// Simulate 10 rounds of combat with attacks
	for round := 1; round <= 10; round++ {
		// Attack each round to keep rage active
		err = attackTopic.Publish(s.ctx, dnd5e.AttackEvent{
			AttackerID: "barbarian-1",
			TargetID:   "goblin-1",
			WeaponRef:  "greatsword",
			IsMelee:    true,
		})
		s.Require().NoError(err)

		// End turn
		err = turnEndTopic.Publish(s.ctx, dnd5e.TurnEndEvent{
			CharacterID: "barbarian-1",
			Round:       round,
		})
		s.Require().NoError(err)

		if round < 10 {
			// Before round 10, rage should continue
			s.Nil(removedEvent, "Rage should continue until round 10")
		}
	}

	// After 10 rounds, rage should end
	s.Require().NotNil(removedEvent)
	s.Equal("barbarian-1", removedEvent.CharacterID)
	s.Equal("dnd5e:conditions:raging", removedEvent.ConditionRef)
	s.Equal("duration_expired", removedEvent.Reason)
}

// executeDamageChain creates a damage chain event and executes it through the damage chain topic.
// Returns the final event after all chain modifications have been applied.
// This helper reduces duplication in tests that verify damage bonus modifications.
func (s *RagingConditionTestSuite) executeDamageChain(
	attackerID string,
	baseDamage, damageBonus int,
) (*combat.DamageChainEvent, error) {
	damageEvent := &combat.DamageChainEvent{
		AttackerID:   attackerID,
		TargetID:     "goblin-1",
		BaseDamage:   baseDamage,
		DamageBonus:  damageBonus,
		DamageType:   "slashing",
		IsCritical:   false,
		WeaponDamage: "1d8",
	}

	chain := events.NewStagedChain[*combat.DamageChainEvent](dnd5e.ModifierStages)
	damageTopic := combat.DamageChain.On(s.bus)

	modifiedChain, err := damageTopic.PublishWithChain(s.ctx, damageEvent, chain)
	if err != nil {
		return nil, err
	}

	return modifiedChain.Execute(s.ctx, damageEvent)
}

func (s *RagingConditionTestSuite) TestRagingConditionAddsDamageBonus() {
	// Create a level 3 raging condition (+2 damage)
	raging := newRagingCondition(ragingConditionInput{
		CharacterID: "barbarian-1",
		DamageBonus: 2,
		Level:       3,
		Source:      "rage-feature",
	})

	// Apply it to subscribe to damage chain
	err := raging.Apply(s.ctx, s.bus)
	s.Require().NoError(err)

	// Execute damage chain for the raging barbarian
	finalEvent, err := s.executeDamageChain("barbarian-1", 5, 3)
	s.Require().NoError(err)

	// Verify rage damage bonus was added (STR 3 + Rage 2 = 5)
	s.Equal(5, finalEvent.DamageBonus, "Should include STR(+3) and rage(+2)")
}

func (s *RagingConditionTestSuite) TestRagingConditionOnlyAffectsOwnAttacks() {
	// Create a raging condition for barbarian-1
	raging := newRagingCondition(ragingConditionInput{
		CharacterID: "barbarian-1",
		DamageBonus: 2,
		Level:       3,
		Source:      "rage-feature",
	})

	err := raging.Apply(s.ctx, s.bus)
	s.Require().NoError(err)

	// Execute damage chain for a DIFFERENT attacker
	finalEvent, err := s.executeDamageChain("barbarian-2", 5, 3)
	s.Require().NoError(err)

	// Verify NO rage bonus was added (only STR modifier)
	s.Equal(3, finalEvent.DamageBonus, "Should only include STR(+3), no rage for other character")
}
