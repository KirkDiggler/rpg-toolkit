// Copyright (C) 2024 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package features_test

import (
	"context"
	"testing"
	"time"

	"github.com/KirkDiggler/rpg-toolkit/events"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/features"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRage_PublishesConditionEvent(t *testing.T) {
	// Create a bus
	bus := events.NewBus()

	// Load rage feature
	featureJSON := `{
		"ref": "dnd5e:features:rage",
		"id": "barbarian-rage",
		"data": {
			"uses": 3,
			"level": 5
		}
	}`

	rage, err := features.LoadJSON([]byte(featureJSON), bus)
	require.NoError(t, err)

	// Create barbarian
	barbarian := &mockEntity{id: "conan", entityType: dnd5e.EntityTypeCharacter}

	// Subscribe to condition events
	var appliedEvent *dnd5e.ConditionAppliedEvent
	bus.Subscribe(dnd5e.EventRefConditionApplied, func(e any) error {
		if evt, ok := e.(*dnd5e.ConditionAppliedEvent); ok {
			appliedEvent = evt
		}
		return nil
	})

	// Activate rage
	err = rage.Activate(context.Background(), barbarian, features.FeatureInput{})
	require.NoError(t, err)

	// Verify condition event was published
	require.NotNil(t, appliedEvent)
	assert.Equal(t, "conan", appliedEvent.Target)
	assert.Equal(t, "dnd5e:conditions:raging", appliedEvent.Condition)
	assert.Equal(t, "barbarian-rage", appliedEvent.Source)
	
	// Check event data
	assert.Equal(t, 5, appliedEvent.Data["level"])
	assert.Equal(t, 10, appliedEvent.Data["duration"])
	assert.Equal(t, false, appliedEvent.Data["attacked_this_round"])
	assert.Equal(t, false, appliedEvent.Data["was_hit_this_round"])
}

func TestRagingCondition_Combat(t *testing.T) {
	// Create a bus
	bus := events.NewBus()

	// Create barbarian
	barbarian := &mockEntity{id: "conan", entityType: dnd5e.EntityTypeCharacter}

	// Create rage condition
	condition, err := features.NewRagingCondition(map[string]any{
		"level":    5,
		"duration": 10,
	}, bus)
	require.NoError(t, err)

	// Apply condition
	err = condition.Apply(bus, barbarian)
	require.NoError(t, err)

	t.Run("adds damage bonus to STR melee attacks", func(t *testing.T) {
		// Create an attack event from the barbarian
		attackEvent := dnd5e.NewAttackEvent(
			barbarian, // attacker
			&mockEntity{id: "goblin", entityType: dnd5e.EntityTypeMonster}, // target
			true,                  // is melee
			dnd5e.AbilityStrength, // using STR
			8,                     // base damage
		)

		// Publish the attack
		err := bus.Publish(attackEvent)
		require.NoError(t, err)

		// Check that rage added a damage modifier
		modifiers := attackEvent.Context().GetModifiers()
		require.Len(t, modifiers, 1, "rage should add one modifier")

		mod := modifiers[0]
		assert.Equal(t, dnd5e.ModifierSourceRage, mod.Source())
		assert.Equal(t, dnd5e.ModifierTypeAdditive, mod.Type())
		assert.Equal(t, dnd5e.ModifierTargetDamage, mod.Target())
		assert.Equal(t, 2.0, mod.Value()) // Level 1-8 = +2 damage
	})

	t.Run("doesn't add bonus to DEX attacks", func(t *testing.T) {
		// Create a DEX-based attack
		attackEvent := dnd5e.NewAttackEvent(
			barbarian,
			&mockEntity{id: "goblin", entityType: dnd5e.EntityTypeMonster},
			true,                   // is melee (finesse weapon)
			dnd5e.AbilityDexterity, // using DEX
			6,
		)

		err := bus.Publish(attackEvent)
		require.NoError(t, err)

		// Should have no modifiers
		modifiers := attackEvent.Context().GetModifiers()
		assert.Empty(t, modifiers, "rage shouldn't modify DEX attacks")
	})

	t.Run("doesn't add bonus to other attacker's attacks", func(t *testing.T) {
		// Create an attack from someone else
		wizard := &mockEntity{id: "gandalf", entityType: dnd5e.EntityTypeCharacter}
		attackEvent := dnd5e.NewAttackEvent(
			wizard, // different attacker
			&mockEntity{id: "goblin", entityType: dnd5e.EntityTypeMonster},
			true,
			dnd5e.AbilityStrength,
			10,
		)

		err := bus.Publish(attackEvent)
		require.NoError(t, err)

		// Should have no modifiers
		modifiers := attackEvent.Context().GetModifiers()
		assert.Empty(t, modifiers, "rage shouldn't modify other's attacks")
	})

	t.Run("applies resistance to physical damage", func(t *testing.T) {
		// Barbarian takes slashing damage
		damageEvent := dnd5e.NewDamageReceivedEvent(
			barbarian, // target
			&mockEntity{id: "orc", entityType: dnd5e.EntityTypeMonster}, // source
			10,                       // amount
			dnd5e.DamageTypeSlashing, // damage type
		)

		err := bus.Publish(damageEvent)
		require.NoError(t, err)

		// Check resistance modifier
		modifiers := damageEvent.Context().GetModifiers()
		require.Len(t, modifiers, 1, "rage should add resistance")

		mod := modifiers[0]
		assert.Equal(t, dnd5e.ModifierSourceRage, mod.Source())
		assert.Equal(t, dnd5e.ModifierTypeResistance, mod.Type())
		assert.Equal(t, 0.5, mod.Value()) // Half damage
	})

	t.Run("doesn't resist magical damage", func(t *testing.T) {
		// Barbarian takes fire damage
		damageEvent := dnd5e.NewDamageReceivedEvent(
			barbarian,
			&mockEntity{id: "dragon", entityType: dnd5e.EntityTypeMonster},
			20,
			dnd5e.DamageTypeFire, // Not physical
		)

		err := bus.Publish(damageEvent)
		require.NoError(t, err)

		// Should have no modifiers
		modifiers := damageEvent.Context().GetModifiers()
		assert.Empty(t, modifiers, "rage shouldn't resist fire damage")
	})
}

func TestRagingCondition_Duration(t *testing.T) {
	// Create a bus
	bus := events.NewBus()

	// Create barbarian
	barbarian := &mockEntity{id: "conan", entityType: dnd5e.EntityTypeCharacter}

	t.Run("ends if no combat activity", func(t *testing.T) {
		// Create rage condition
		condition, err := features.NewRagingCondition(map[string]any{
			"level":    5,
			"duration": 10,
		}, bus)
		require.NoError(t, err)

		// Apply condition
		err = condition.Apply(bus, barbarian)
		require.NoError(t, err)

		// Subscribe to removal events
		var removedEvent *dnd5e.ConditionRemovedEvent
		bus.Subscribe(dnd5e.EventRefConditionRemoved, func(e any) error {
			if evt, ok := e.(*dnd5e.ConditionRemovedEvent); ok {
				removedEvent = evt
			}
			return nil
		})

		// Publish round end without any attacks
		roundEnd := dnd5e.NewRoundEndEvent(1)
		err = bus.Publish(roundEnd)
		require.NoError(t, err)

		// Wait a bit for async removal
		time.Sleep(10 * time.Millisecond)
		
		// Verify rage ended
		require.NotNil(t, removedEvent)
		assert.Equal(t, "conan", removedEvent.Target)
		assert.Equal(t, "dnd5e:conditions:raging", removedEvent.Condition)
		assert.Equal(t, "You didn't attack or take damage", removedEvent.Reason)
	})

	t.Run("continues if attacked", func(t *testing.T) {
		// Create rage condition
		condition, err := features.NewRagingCondition(map[string]any{
			"level":    5,
			"duration": 3,
		}, bus)
		require.NoError(t, err)

		// Apply condition
		err = condition.Apply(bus, barbarian)
		require.NoError(t, err)

		// Subscribe to removal events
		var removedEvent *dnd5e.ConditionRemovedEvent
		bus.Subscribe(dnd5e.EventRefConditionRemoved, func(e any) error {
			if evt, ok := e.(*dnd5e.ConditionRemovedEvent); ok {
				removedEvent = evt
			}
			return nil
		})

		// Round 1: Attack
		attackEvent := dnd5e.NewAttackEvent(
			barbarian,
			&mockEntity{id: "goblin", entityType: dnd5e.EntityTypeMonster},
			true,
			dnd5e.AbilityStrength,
			8,
		)
		err = bus.Publish(attackEvent)
		require.NoError(t, err)

		roundEnd := dnd5e.NewRoundEndEvent(1)
		err = bus.Publish(roundEnd)
		require.NoError(t, err)

		// Should still be active
		assert.Nil(t, removedEvent)
		assert.Equal(t, 2, condition.GetTicksRemaining())

		// Round 2: Take damage
		damageEvent := dnd5e.NewDamageReceivedEvent(
			barbarian,
			&mockEntity{id: "orc", entityType: dnd5e.EntityTypeMonster},
			10,
			dnd5e.DamageTypeSlashing,
		)
		err = bus.Publish(damageEvent)
		require.NoError(t, err)

		err = bus.Publish(roundEnd)
		require.NoError(t, err)

		// Should still be active
		assert.Nil(t, removedEvent)
		assert.Equal(t, 1, condition.GetTicksRemaining())

		// Round 3: Attack again
		err = bus.Publish(attackEvent)
		require.NoError(t, err)

		err = bus.Publish(roundEnd)
		require.NoError(t, err)

		// Wait a bit for async removal
		time.Sleep(10 * time.Millisecond)
		
		// Should end due to duration
		require.NotNil(t, removedEvent)
		assert.Equal(t, "Rage duration expired", removedEvent.Reason)
	})
}
