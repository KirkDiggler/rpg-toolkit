// Copyright (C) 2024 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package conditions

import (
	"context"
	"testing"

	"github.com/stretchr/testify/suite"

	coreResources "github.com/KirkDiggler/rpg-toolkit/core/resources"
	"github.com/KirkDiggler/rpg-toolkit/events"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/abilities"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/combat"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/damage"
	dnd5eEvents "github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/events"
)

// ragingConditionInput provides configuration for creating a raging condition
type ragingConditionInput struct {
	CharacterID string // ID of the raging character
	DamageBonus int    // Bonus damage for rage
	Level       int    // Barbarian level
	Source      string // Ref string in "module:type:value" format (e.g., "dnd5e:features:rage")
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

func (s *RagingConditionTestSuite) TestRagingConditionTracksHits() {
	// Create a raging condition
	raging := newRagingCondition(ragingConditionInput{
		CharacterID: "barbarian-1",
		DamageBonus: 2,
		Level:       5,
		Source:      "dnd5e:features:rage",
	})

	// Apply it to subscribe to events
	err := raging.Apply(s.ctx, s.bus)
	s.Require().NoError(err)

	// Verify initial state
	s.False(raging.DidAttackThisTurn)

	// Execute damage chain (simulates a successful hit)
	// Note: DamageChain only fires when an attack hits
	_, err = s.executeDamageChain("barbarian-1", 5, 3)
	s.Require().NoError(err)

	// Check that the condition tracked the successful hit
	s.True(raging.DidAttackThisTurn)
}

func (s *RagingConditionTestSuite) TestRagingConditionTracksDamage() {
	// Create a raging condition
	raging := newRagingCondition(ragingConditionInput{
		CharacterID: "barbarian-1",
		DamageBonus: 2,
		Level:       5,
		Source:      "dnd5e:features:rage",
	})

	// Apply it to subscribe to events
	err := raging.Apply(s.ctx, s.bus)
	s.Require().NoError(err)

	// Verify initial state
	s.False(raging.WasHitThisTurn)

	// Publish a damage event for this character
	damageTopic := dnd5eEvents.DamageReceivedTopic.On(s.bus)
	err = damageTopic.Publish(s.ctx, dnd5eEvents.DamageReceivedEvent{
		TargetID:   "barbarian-1",
		SourceID:   "goblin-1",
		Amount:     5,
		DamageType: damage.Slashing,
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
		Source:      "dnd5e:features:rage",
	})

	// Apply it to subscribe to events
	err := raging.Apply(s.ctx, s.bus)
	s.Require().NoError(err)

	// Track if condition removed event is published
	var removedEvent *dnd5eEvents.ConditionRemovedEvent
	removalTopic := dnd5eEvents.ConditionRemovedTopic.On(s.bus)
	_, err = removalTopic.Subscribe(s.ctx, func(_ context.Context, event dnd5eEvents.ConditionRemovedEvent) error {
		removedEvent = &event
		return nil
	})
	s.Require().NoError(err)

	// Publish turn end event without any combat activity
	turnEndTopic := dnd5eEvents.TurnEndTopic.On(s.bus)
	err = turnEndTopic.Publish(s.ctx, dnd5eEvents.TurnEndEvent{
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
		Source:      "dnd5e:features:rage",
	})

	// Apply it to subscribe to events
	err := raging.Apply(s.ctx, s.bus)
	s.Require().NoError(err)

	// Track if condition removed event is published
	var removedEvent *dnd5eEvents.ConditionRemovedEvent
	removalTopic := dnd5eEvents.ConditionRemovedTopic.On(s.bus)
	_, err = removalTopic.Subscribe(s.ctx, func(_ context.Context, event dnd5eEvents.ConditionRemovedEvent) error {
		removedEvent = &event
		return nil
	})
	s.Require().NoError(err)

	// Execute damage chain (simulates a successful hit - combat activity)
	// Note: DamageChain only fires when an attack hits
	_, err = s.executeDamageChain("barbarian-1", 5, 3)
	s.Require().NoError(err)

	// Publish turn end event
	turnEndTopic := dnd5eEvents.TurnEndTopic.On(s.bus)
	err = turnEndTopic.Publish(s.ctx, dnd5eEvents.TurnEndEvent{
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
		Source:      "dnd5e:features:rage",
	})

	// Apply it to subscribe to events
	err := raging.Apply(s.ctx, s.bus)
	s.Require().NoError(err)

	// Track if condition removed event is published
	var removedEvent *dnd5eEvents.ConditionRemovedEvent
	removalTopic := dnd5eEvents.ConditionRemovedTopic.On(s.bus)
	_, err = removalTopic.Subscribe(s.ctx, func(_ context.Context, event dnd5eEvents.ConditionRemovedEvent) error {
		removedEvent = &event
		return nil
	})
	s.Require().NoError(err)

	turnEndTopic := dnd5eEvents.TurnEndTopic.On(s.bus)

	// Simulate 10 rounds of combat with successful hits
	for round := 1; round <= 10; round++ {
		// Execute damage chain each round to keep rage active (simulates successful hit)
		_, err = s.executeDamageChain("barbarian-1", 5, 3)
		s.Require().NoError(err)

		// End turn
		err = turnEndTopic.Publish(s.ctx, dnd5eEvents.TurnEndEvent{
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
//
//nolint:unparam // Parameters kept for consistency with other test helpers in this package
func (s *RagingConditionTestSuite) executeDamageChain(
	attackerID string,
	baseDamage, damageBonus int,
) (*dnd5eEvents.DamageChainEvent, error) {
	// Create weapon component with base damage
	weaponComp := dnd5eEvents.DamageComponent{
		Source:            dnd5eEvents.DamageSourceWeapon,
		OriginalDiceRolls: []int{baseDamage},
		FinalDiceRolls:    []int{baseDamage},
		Rerolls:           nil,
		FlatBonus:         0,
		DamageType:        damage.Slashing,
		IsCritical:        false,
	}

	// Create ability component with damage bonus (STR modifier)
	abilityComp := dnd5eEvents.DamageComponent{
		Source:            dnd5eEvents.DamageSourceAbility,
		OriginalDiceRolls: nil,
		FinalDiceRolls:    nil,
		Rerolls:           nil,
		FlatBonus:         damageBonus,
		DamageType:        damage.Slashing,
		IsCritical:        false,
	}

	damageEvent := &dnd5eEvents.DamageChainEvent{
		AttackerID:   attackerID,
		TargetID:     "goblin-1",
		Components:   []dnd5eEvents.DamageComponent{weaponComp, abilityComp},
		DamageType:   damage.Slashing,
		IsCritical:   false,
		WeaponDamage: "1d8",
		AbilityUsed:  abilities.STR,
	}

	chain := events.NewStagedChain[*dnd5eEvents.DamageChainEvent](combat.ModifierStages)
	damageTopic := dnd5eEvents.DamageChain.On(s.bus)

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
		Source:      "dnd5e:features:rage",
	})

	// Apply it to subscribe to damage chain
	err := raging.Apply(s.ctx, s.bus)
	s.Require().NoError(err)

	// Execute damage chain for the raging barbarian
	finalEvent, err := s.executeDamageChain("barbarian-1", 5, 3)
	s.Require().NoError(err)

	// Verify rage damage component was added
	s.Require().Len(finalEvent.Components, 3, "Should have weapon, ability, and rage components")

	// Verify weapon component
	s.Equal(dnd5eEvents.DamageSourceWeapon, finalEvent.Components[0].Source)
	s.Equal(5, finalEvent.Components[0].Total())

	// Verify ability component
	s.Equal(dnd5eEvents.DamageSourceAbility, finalEvent.Components[1].Source)
	s.Equal(3, finalEvent.Components[1].Total())

	// Verify rage component was added
	s.Equal(dnd5eEvents.DamageSourceCondition, finalEvent.Components[2].Source)
	s.Equal(2, finalEvent.Components[2].FlatBonus, "Rage should add +2 damage")
	s.Equal(2, finalEvent.Components[2].Total())

	// Verify total damage
	totalDamage := 0
	for _, comp := range finalEvent.Components {
		totalDamage += comp.Total()
	}
	s.Equal(10, totalDamage, "Total should be 5 (weapon) + 3 (ability) + 2 (rage)")
}

func (s *RagingConditionTestSuite) TestRagingConditionOnlyAffectsOwnAttacks() {
	// Create a raging condition for barbarian-1
	raging := newRagingCondition(ragingConditionInput{
		CharacterID: "barbarian-1",
		DamageBonus: 2,
		Level:       3,
		Source:      "dnd5e:features:rage",
	})

	err := raging.Apply(s.ctx, s.bus)
	s.Require().NoError(err)

	// Execute damage chain for a DIFFERENT attacker
	finalEvent, err := s.executeDamageChain("barbarian-2", 5, 3)
	s.Require().NoError(err)

	// Verify NO rage component was added (only weapon and ability)
	s.Require().Len(finalEvent.Components, 2, "Should only have weapon and ability components, no rage")

	// Verify weapon component
	s.Equal(dnd5eEvents.DamageSourceWeapon, finalEvent.Components[0].Source)
	s.Equal(5, finalEvent.Components[0].Total())

	// Verify ability component
	s.Equal(dnd5eEvents.DamageSourceAbility, finalEvent.Components[1].Source)
	s.Equal(3, finalEvent.Components[1].Total())

	// Verify total damage (no rage bonus)
	totalDamage := 0
	for _, comp := range finalEvent.Components {
		totalDamage += comp.Total()
	}
	s.Equal(8, totalDamage, "Total should be 5 (weapon) + 3 (ability), no rage for other character")
}

func (s *RagingConditionTestSuite) TestRagingConditionRejectsDoubleApply() {
	// Create a raging condition
	raging := newRagingCondition(ragingConditionInput{
		CharacterID: "barbarian-1",
		DamageBonus: 2,
		Level:       5,
		Source:      "dnd5e:features:rage",
	})

	// Apply it once - should succeed
	err := raging.Apply(s.ctx, s.bus)
	s.Require().NoError(err)

	// Apply it again - should fail
	err = raging.Apply(s.ctx, s.bus)
	s.Require().Error(err)
	s.Contains(err.Error(), "already applied")
}

func (s *RagingConditionTestSuite) TestRagingConditionEndsOnRest() {
	// Create a raging condition
	raging := newRagingCondition(ragingConditionInput{
		CharacterID: "barbarian-1",
		DamageBonus: 2,
		Level:       5,
		Source:      "dnd5e:features:rage",
	})

	// Apply it to subscribe to events
	err := raging.Apply(s.ctx, s.bus)
	s.Require().NoError(err)
	s.True(raging.IsApplied(), "rage should be applied")

	// Track if condition removed event is published
	var removedEvent *dnd5eEvents.ConditionRemovedEvent
	removalTopic := dnd5eEvents.ConditionRemovedTopic.On(s.bus)
	_, err = removalTopic.Subscribe(s.ctx, func(_ context.Context, event dnd5eEvents.ConditionRemovedEvent) error {
		removedEvent = &event
		return nil
	})
	s.Require().NoError(err)

	// Publish a rest event for this character
	restTopic := dnd5eEvents.RestTopic.On(s.bus)
	err = restTopic.Publish(s.ctx, dnd5eEvents.RestEvent{
		RestType:    coreResources.ResetLongRest,
		CharacterID: "barbarian-1",
	})
	s.Require().NoError(err)

	// Verify condition published removal event
	s.Require().NotNil(removedEvent, "rage should be removed on rest")
	s.Equal("barbarian-1", removedEvent.CharacterID)
	s.Equal("dnd5e:conditions:raging", removedEvent.ConditionRef)
	s.Equal("rest", removedEvent.Reason)

	// Verify condition is no longer applied
	s.False(raging.IsApplied(), "rage should no longer be applied after rest")
}

func (s *RagingConditionTestSuite) TestRagingConditionIgnoresOtherCharacterRest() {
	// Create a raging condition for barbarian-1
	raging := newRagingCondition(ragingConditionInput{
		CharacterID: "barbarian-1",
		DamageBonus: 2,
		Level:       5,
		Source:      "dnd5e:features:rage",
	})

	// Apply it to subscribe to events
	err := raging.Apply(s.ctx, s.bus)
	s.Require().NoError(err)
	s.True(raging.IsApplied(), "rage should be applied")

	// Track if condition removed event is published
	var removedEvent *dnd5eEvents.ConditionRemovedEvent
	removalTopic := dnd5eEvents.ConditionRemovedTopic.On(s.bus)
	_, err = removalTopic.Subscribe(s.ctx, func(_ context.Context, event dnd5eEvents.ConditionRemovedEvent) error {
		removedEvent = &event
		return nil
	})
	s.Require().NoError(err)

	// Publish a rest event for a DIFFERENT character
	restTopic := dnd5eEvents.RestTopic.On(s.bus)
	err = restTopic.Publish(s.ctx, dnd5eEvents.RestEvent{
		RestType:    coreResources.ResetLongRest,
		CharacterID: "barbarian-2", // Different character
	})
	s.Require().NoError(err)

	// Verify condition did NOT publish removal event
	s.Nil(removedEvent, "rage should NOT be removed when another character rests")

	// Verify condition is still applied
	s.True(raging.IsApplied(), "rage should still be applied")
}

// executeDamageChainAgainstTarget creates a damage chain event where a specific target is hit
// and executes it through the damage chain topic. Used to test resistance.
func (s *RagingConditionTestSuite) executeDamageChainAgainstTarget(
	attackerID, targetID string,
	baseDamage int,
	damageType damage.Type,
) (*dnd5eEvents.DamageChainEvent, error) {
	// Create weapon component with base damage
	weaponComp := dnd5eEvents.DamageComponent{
		Source:            dnd5eEvents.DamageSourceWeapon,
		OriginalDiceRolls: []int{baseDamage},
		FinalDiceRolls:    []int{baseDamage},
		DamageType:        damageType,
	}

	damageEvent := &dnd5eEvents.DamageChainEvent{
		AttackerID:   attackerID,
		TargetID:     targetID,
		Components:   []dnd5eEvents.DamageComponent{weaponComp},
		DamageType:   damageType,
		IsCritical:   false,
		WeaponDamage: "1d8",
		AbilityUsed:  abilities.STR,
	}

	chain := events.NewStagedChain[*dnd5eEvents.DamageChainEvent](combat.ModifierStages)
	damageTopic := dnd5eEvents.DamageChain.On(s.bus)

	modifiedChain, err := damageTopic.PublishWithChain(s.ctx, damageEvent, chain)
	if err != nil {
		return nil, err
	}

	return modifiedChain.Execute(s.ctx, damageEvent)
}

func (s *RagingConditionTestSuite) TestRagingConditionAppliesResistanceToPhysicalDamage() {
	// Create a raging condition for barbarian-1
	raging := newRagingCondition(ragingConditionInput{
		CharacterID: "barbarian-1",
		DamageBonus: 2,
		Level:       5,
		Source:      "dnd5e:features:rage",
	})

	// Apply it to subscribe to damage chain
	err := raging.Apply(s.ctx, s.bus)
	s.Require().NoError(err)

	// Test each physical damage type
	physicalTypes := []damage.Type{damage.Bludgeoning, damage.Piercing, damage.Slashing}

	for _, dmgType := range physicalTypes {
		s.Run(string(dmgType), func() {
			// Execute damage chain with the barbarian as the target
			finalEvent, err := s.executeDamageChainAgainstTarget("goblin-1", "barbarian-1", 10, dmgType)
			s.Require().NoError(err)

			// Should have 2 components: weapon damage and resistance multiplier
			s.Require().Len(finalEvent.Components, 2, "Should have weapon and resistance components")

			// Verify weapon component
			s.Equal(dnd5eEvents.DamageSourceWeapon, finalEvent.Components[0].Source)
			s.Equal(10, finalEvent.Components[0].Total())

			// Verify resistance component was added with 0.5 multiplier
			s.Equal(dnd5eEvents.DamageSourceCondition, finalEvent.Components[1].Source)
			s.Equal(0.5, finalEvent.Components[1].Multiplier, "Resistance should halve damage")
		})
	}
}

func (s *RagingConditionTestSuite) TestRagingConditionDoesNotResistNonPhysicalDamage() {
	// Create a raging condition for barbarian-1
	raging := newRagingCondition(ragingConditionInput{
		CharacterID: "barbarian-1",
		DamageBonus: 2,
		Level:       5,
		Source:      "dnd5e:features:rage",
	})

	// Apply it to subscribe to damage chain
	err := raging.Apply(s.ctx, s.bus)
	s.Require().NoError(err)

	// Test non-physical damage types
	nonPhysicalTypes := []damage.Type{
		damage.Fire, damage.Cold, damage.Lightning, damage.Thunder,
		damage.Acid, damage.Poison, damage.Necrotic, damage.Radiant,
		damage.Force, damage.Psychic,
	}

	for _, dmgType := range nonPhysicalTypes {
		s.Run(string(dmgType), func() {
			// Execute damage chain with the barbarian as the target
			finalEvent, err := s.executeDamageChainAgainstTarget("goblin-1", "barbarian-1", 10, dmgType)
			s.Require().NoError(err)

			// Should only have 1 component: weapon damage (no resistance)
			s.Require().Len(finalEvent.Components, 1, "Should only have weapon component, no resistance")

			// Verify weapon component
			s.Equal(dnd5eEvents.DamageSourceWeapon, finalEvent.Components[0].Source)
			s.Equal(10, finalEvent.Components[0].Total())
		})
	}
}

func (s *RagingConditionTestSuite) TestRagingConditionResistanceOnlyAffectsOwnCharacter() {
	// Create a raging condition for barbarian-1
	raging := newRagingCondition(ragingConditionInput{
		CharacterID: "barbarian-1",
		DamageBonus: 2,
		Level:       5,
		Source:      "dnd5e:features:rage",
	})

	// Apply it to subscribe to damage chain
	err := raging.Apply(s.ctx, s.bus)
	s.Require().NoError(err)

	// Execute damage chain with a DIFFERENT character as the target
	finalEvent, err := s.executeDamageChainAgainstTarget("goblin-1", "barbarian-2", 10, damage.Slashing)
	s.Require().NoError(err)

	// Should only have 1 component: weapon damage (no resistance for other characters)
	s.Require().Len(finalEvent.Components, 1, "Should only have weapon component, no resistance for other characters")

	// Verify weapon component
	s.Equal(dnd5eEvents.DamageSourceWeapon, finalEvent.Components[0].Source)
	s.Equal(10, finalEvent.Components[0].Total())
}
