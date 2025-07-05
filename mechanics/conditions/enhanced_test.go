// Copyright (C) 2024 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package conditions_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/KirkDiggler/rpg-toolkit/mechanics/conditions"
)

// Mock entity for testing
type mockEntity struct {
	id         string
	entityType string
}

func (m *mockEntity) GetID() string   { return m.id }
func (m *mockEntity) GetType() string { return m.entityType }

func TestConditionRegistration(t *testing.T) {
	// Test registering a custom condition definition
	poisonedDef := &conditions.ConditionDefinition{
		Type:        conditions.ConditionType("poisoned"),
		Name:        "Poisoned",
		Description: "A poisoned creature has disadvantage on attack rolls and ability checks.",
		Effects: []conditions.ConditionEffect{
			{Type: conditions.EffectDisadvantage, Target: conditions.TargetAttackRolls},
			{Type: conditions.EffectDisadvantage, Target: conditions.TargetAbilityChecks},
		},
	}

	conditions.RegisterConditionDefinition(poisonedDef)

	// Verify it was registered
	retrieved, exists := conditions.GetConditionDefinition(conditions.ConditionType("poisoned"))
	assert.True(t, exists)
	assert.Equal(t, "Poisoned", retrieved.Name)
	assert.Len(t, retrieved.Effects, 2)
}

func TestConditionBuilder(t *testing.T) {
	target := &mockEntity{id: "test-target", entityType: "character"}

	// Register a test condition
	conditions.RegisterConditionDefinition(&conditions.ConditionDefinition{
		Type:        conditions.ConditionType("stunned"),
		Name:        "Stunned",
		Description: "A stunned creature cannot act.",
		Effects: []conditions.ConditionEffect{
			{Type: conditions.EffectIncapacitated, Target: conditions.TargetActions},
		},
	})

	t.Run("Basic condition creation", func(t *testing.T) {
		cond, err := conditions.NewConditionBuilder(conditions.ConditionType("stunned")).
			WithTarget(target).
			WithSource("spell").
			WithSaveDC(15).
			Build()

		require.NoError(t, err)
		assert.Equal(t, "stunned", string(cond.GetConditionType()))
		assert.Equal(t, target, cond.Target())
		assert.Equal(t, "spell", cond.Source())
		assert.Equal(t, 15, cond.GetSaveDC())
	})

	t.Run("Missing target", func(t *testing.T) {
		_, err := conditions.NewConditionBuilder(conditions.ConditionType("stunned")).
			WithSource("spell").
			Build()

		assert.Error(t, err)
		assert.Contains(t, err.Error(), "target is required")
	})

	t.Run("Missing source", func(t *testing.T) {
		_, err := conditions.NewConditionBuilder(conditions.ConditionType("stunned")).
			WithTarget(target).
			Build()

		assert.Error(t, err)
		assert.Contains(t, err.Error(), "source is required")
	})
}

func TestSimpleCondition(t *testing.T) {
	target := &mockEntity{id: "test-target", entityType: "character"}

	config := conditions.SimpleConditionConfig{
		ID:     "test-condition",
		Type:   "burning",
		Target: target,
		Source: "fire_spell",
	}

	cond := conditions.NewSimpleCondition(config)
	assert.Equal(t, "test-condition", cond.GetID())
	assert.Equal(t, "burning", cond.GetType())
	assert.Equal(t, target, cond.Target())
	assert.Equal(t, "fire_spell", cond.Source())
}

func TestConditionMetadata(t *testing.T) {
	target := &mockEntity{id: "test-target", entityType: "character"}
	caster := &mockEntity{id: "caster", entityType: "character"}

	// Register a test condition
	conditions.RegisterConditionDefinition(&conditions.ConditionDefinition{
		Type:        conditions.ConditionType("charmed"),
		Name:        "Charmed",
		Description: "A charmed creature cannot attack the charmer.",
		Effects:     []conditions.ConditionEffect{},
	})

	cond, err := conditions.NewConditionBuilder(conditions.ConditionType("charmed")).
		WithTarget(target).
		WithSource("charm_person").
		WithRelatedEntity("charmer", caster).
		WithMetadata("duration", "1 hour").
		Build()

	require.NoError(t, err)

	charmer, exists := cond.GetMetadata("charmer")
	assert.True(t, exists)
	assert.Equal(t, caster, charmer)

	duration, exists := cond.GetMetadata("duration")
	assert.True(t, exists)
	assert.Equal(t, "1 hour", duration)
}