// Copyright (C) 2024 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package conditions_test

import (
	"context"
	"testing"

	"github.com/KirkDiggler/rpg-toolkit/events"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/conditions"
	"github.com/stretchr/testify/suite"
)

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
	raging := conditions.NewRagingCondition(conditions.RagingConditionInput{
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
	attackTopic := events.DefineTypedTopic[conditions.AttackEvent]("dnd5e.combat.attack").On(s.bus)
	err = attackTopic.Publish(s.ctx, conditions.AttackEvent{
		AttackerID: "barbarian-1",
		TargetID:   "goblin-1",
		WeaponRef:  "greatsword",
		IsMelee:    true,
		Damage:     10,
	})
	s.Require().NoError(err)

	// Check that the condition tracked the attack
	s.True(raging.DidAttackThisTurn)
}

func (s *RagingConditionTestSuite) TestRagingConditionTracksDamage() {
	// Create a raging condition
	raging := conditions.NewRagingCondition(conditions.RagingConditionInput{
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
	damageTopic := events.DefineTypedTopic[conditions.DamageReceivedEvent]("dnd5e.combat.damage.received").On(s.bus)
	err = damageTopic.Publish(s.ctx, conditions.DamageReceivedEvent{
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
	raging := conditions.NewRagingCondition(conditions.RagingConditionInput{
		CharacterID: "barbarian-1",
		DamageBonus: 2,
		Level:       5,
		Source:      "rage-feature",
	})

	// Apply it to subscribe to events
	err := raging.Apply(s.ctx, s.bus)
	s.Require().NoError(err)

	// Track if condition removed event is published
	var removedEvent *conditions.ConditionRemovedEvent
	removalTopic := events.DefineTypedTopic[conditions.ConditionRemovedEvent]("dnd5e.condition.removed").On(s.bus)
	_, err = removalTopic.Subscribe(s.ctx, func(ctx context.Context, event conditions.ConditionRemovedEvent) error {
		removedEvent = &event
		return nil
	})
	s.Require().NoError(err)

	// Publish turn end event without any combat activity
	turnEndTopic := events.DefineTypedTopic[conditions.TurnEndEvent]("dnd5e.turn.end").On(s.bus)
	err = turnEndTopic.Publish(s.ctx, conditions.TurnEndEvent{
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
	raging := conditions.NewRagingCondition(conditions.RagingConditionInput{
		CharacterID: "barbarian-1",
		DamageBonus: 2,
		Level:       5,
		Source:      "rage-feature",
	})

	// Apply it to subscribe to events
	err := raging.Apply(s.ctx, s.bus)
	s.Require().NoError(err)

	// Track if condition removed event is published
	var removedEvent *conditions.ConditionRemovedEvent
	removalTopic := events.DefineTypedTopic[conditions.ConditionRemovedEvent]("dnd5e.condition.removed").On(s.bus)
	_, err = removalTopic.Subscribe(s.ctx, func(ctx context.Context, event conditions.ConditionRemovedEvent) error {
		removedEvent = &event
		return nil
	})
	s.Require().NoError(err)

	// Publish an attack event (combat activity)
	attackTopic := events.DefineTypedTopic[conditions.AttackEvent]("dnd5e.combat.attack").On(s.bus)
	err = attackTopic.Publish(s.ctx, conditions.AttackEvent{
		AttackerID: "barbarian-1",
		TargetID:   "goblin-1",
		WeaponRef:  "greatsword",
		IsMelee:    true,
		Damage:     10,
	})
	s.Require().NoError(err)

	// Publish turn end event
	turnEndTopic := events.DefineTypedTopic[conditions.TurnEndEvent]("dnd5e.turn.end").On(s.bus)
	err = turnEndTopic.Publish(s.ctx, conditions.TurnEndEvent{
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
	raging := conditions.NewRagingCondition(conditions.RagingConditionInput{
		CharacterID: "barbarian-1",
		DamageBonus: 2,
		Level:       5,
		Source:      "rage-feature",
	})

	// Apply it to subscribe to events
	err := raging.Apply(s.ctx, s.bus)
	s.Require().NoError(err)

	// Track if condition removed event is published
	var removedEvent *conditions.ConditionRemovedEvent
	removalTopic := events.DefineTypedTopic[conditions.ConditionRemovedEvent]("dnd5e.condition.removed").On(s.bus)
	_, err = removalTopic.Subscribe(s.ctx, func(ctx context.Context, event conditions.ConditionRemovedEvent) error {
		removedEvent = &event
		return nil
	})
	s.Require().NoError(err)

	attackTopic := events.DefineTypedTopic[conditions.AttackEvent]("dnd5e.combat.attack").On(s.bus)
	turnEndTopic := events.DefineTypedTopic[conditions.TurnEndEvent]("dnd5e.turn.end").On(s.bus)

	// Simulate 10 rounds of combat with attacks
	for round := 1; round <= 10; round++ {
		// Attack each round to keep rage active
		err = attackTopic.Publish(s.ctx, conditions.AttackEvent{
			AttackerID: "barbarian-1",
			TargetID:   "goblin-1",
			WeaponRef:  "greatsword",
			IsMelee:    true,
			Damage:     10,
		})
		s.Require().NoError(err)

		// End turn
		err = turnEndTopic.Publish(s.ctx, conditions.TurnEndEvent{
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
