// Copyright (C) 2024 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package features_test

import (
	"context"
	"testing"

	"github.com/KirkDiggler/rpg-toolkit/core"
	"github.com/KirkDiggler/rpg-toolkit/events"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/features"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRage_EventSubscriptions(t *testing.T) {
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
	barbarian := &mockEntity{id: "conan", entityType: core.EntityType("character")}
	
	// Activate rage
	err = rage.Activate(context.Background(), barbarian, features.FeatureInput{})
	require.NoError(t, err)
	
	t.Run("rage adds damage bonus to STR melee attacks", func(t *testing.T) {
		// Create an attack event from the barbarian
		attackEvent := dnd5e.NewAttackEvent(
			barbarian,                    // attacker
			&mockEntity{id: "goblin", entityType: core.EntityType("monster")}, // target
			true,                         // is melee
			dnd5e.AbilityStrength,       // using STR
			8,                           // base damage
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
		assert.Equal(t, 2, mod.Value()) // Level 1-8 = +2 damage
	})
	
	t.Run("rage doesn't add bonus to DEX attacks", func(t *testing.T) {
		// Create a DEX-based attack
		attackEvent := dnd5e.NewAttackEvent(
			barbarian,
			&mockEntity{id: "goblin", entityType: core.EntityType("monster")},
			true,                         // is melee (finesse weapon)
			dnd5e.AbilityDexterity,      // using DEX
			6,
		)
		
		err := bus.Publish(attackEvent)
		require.NoError(t, err)
		
		// Should have no modifiers
		modifiers := attackEvent.Context().GetModifiers()
		assert.Empty(t, modifiers, "rage shouldn't modify DEX attacks")
	})
	
	t.Run("rage doesn't add bonus to other attacker's attacks", func(t *testing.T) {
		// Create an attack from someone else
		wizard := &mockEntity{id: "gandalf", entityType: core.EntityType("character")}
		attackEvent := dnd5e.NewAttackEvent(
			wizard,                       // different attacker
			&mockEntity{id: "goblin", entityType: core.EntityType("monster")},
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
	
	t.Run("rage applies resistance to physical damage", func(t *testing.T) {
		// Barbarian takes slashing damage
		damageEvent := dnd5e.NewDamageReceivedEvent(
			barbarian,                    // target
			&mockEntity{id: "orc", entityType: core.EntityType("monster")}, // source
			10,                          // amount
			dnd5e.DamageTypeSlashing,    // damage type
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
	
	t.Run("rage doesn't resist magical damage", func(t *testing.T) {
		// Barbarian takes fire damage
		damageEvent := dnd5e.NewDamageReceivedEvent(
			barbarian,
			&mockEntity{id: "dragon", entityType: core.EntityType("monster")},
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