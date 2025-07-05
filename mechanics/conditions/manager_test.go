// Copyright (C) 2024 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package conditions_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/KirkDiggler/rpg-toolkit/events"
	"github.com/KirkDiggler/rpg-toolkit/mechanics/conditions"
)

func TestConditionManager(t *testing.T) {
	bus := events.NewBus()
	manager := conditions.NewConditionManager(bus)
	target := &mockEntity{id: "test-target", entityType: "character"}

	t.Run("Apply and get conditions", func(t *testing.T) {
		// Create a poisoned condition
		poisoned, err := conditions.Poisoned().
			WithTarget(target).
			WithSource("spider_bite").
			Build()
		require.NoError(t, err)

		// Apply it
		err = manager.ApplyCondition(poisoned)
		require.NoError(t, err)

		// Check it exists
		assert.True(t, manager.HasCondition(target, conditions.ConditionPoisoned))

		// Get all conditions
		conds := manager.GetConditions(target)
		assert.Len(t, conds, 1)

		// Get by type
		poisonConds := manager.GetConditionsByType(target, conditions.ConditionPoisoned)
		assert.Len(t, poisonConds, 1)
	})

	t.Run("Remove conditions", func(t *testing.T) {
		// Apply a condition
		blinded, err := conditions.Blinded().
			WithTarget(target).
			WithSource("darkness").
			Build()
		require.NoError(t, err)

		err = manager.ApplyCondition(blinded)
		require.NoError(t, err)
		assert.True(t, manager.HasCondition(target, conditions.ConditionBlinded))

		// Remove it
		err = manager.RemoveCondition(blinded)
		require.NoError(t, err)
		assert.False(t, manager.HasCondition(target, conditions.ConditionBlinded))
	})
}

func TestConditionImmunity(t *testing.T) {
	bus := events.NewBus()
	manager := conditions.NewConditionManager(bus)
	target := &mockEntity{id: "test-target", entityType: "character"}

	// Add immunity to poison
	manager.AddImmunity(target, conditions.ConditionPoisoned)

	// Try to apply poisoned condition
	poisoned, err := conditions.Poisoned().
		WithTarget(target).
		WithSource("poison_dart").
		Build()
	require.NoError(t, err)

	err = manager.ApplyCondition(poisoned)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "immune")
	assert.False(t, manager.HasCondition(target, conditions.ConditionPoisoned))

	// Remove immunity
	manager.RemoveImmunity(target, conditions.ConditionPoisoned)

	// Now it should work
	err = manager.ApplyCondition(poisoned)
	require.NoError(t, err)
	assert.True(t, manager.HasCondition(target, conditions.ConditionPoisoned))
}

func TestConditionSuppression(t *testing.T) {
	bus := events.NewBus()
	manager := conditions.NewConditionManager(bus)
	target := &mockEntity{id: "test-target", entityType: "character"}

	// Apply incapacitated
	incap, err := conditions.Incapacitated().
		WithTarget(target).
		WithSource("test").
		Build()
	require.NoError(t, err)

	err = manager.ApplyCondition(incap)
	require.NoError(t, err)
	assert.True(t, manager.HasCondition(target, conditions.ConditionIncapacitated))

	// Apply paralyzed (which includes incapacitated)
	paralyzed, err := conditions.Paralyzed().
		WithTarget(target).
		WithSource("hold_person").
		Build()
	require.NoError(t, err)

	err = manager.ApplyCondition(paralyzed)
	require.NoError(t, err)

	// Should have paralyzed
	assert.True(t, manager.HasCondition(target, conditions.ConditionParalyzed))

	// Should still have incapacitated (from paralyzed)
	assert.True(t, manager.HasCondition(target, conditions.ConditionIncapacitated))

	// But should only be one incapacitated
	incapConds := manager.GetConditionsByType(target, conditions.ConditionIncapacitated)
	assert.Len(t, incapConds, 1)
}

func TestConditionIncludes(t *testing.T) {
	bus := events.NewBus()
	manager := conditions.NewConditionManager(bus)
	target := &mockEntity{id: "test-target", entityType: "character"}

	// Apply unconscious (includes incapacitated and prone)
	unconscious, err := conditions.Unconscious().
		WithTarget(target).
		WithSource("sleep_spell").
		Build()
	require.NoError(t, err)

	err = manager.ApplyCondition(unconscious)
	require.NoError(t, err)

	// Should have all three conditions
	assert.True(t, manager.HasCondition(target, conditions.ConditionUnconscious))
	assert.True(t, manager.HasCondition(target, conditions.ConditionIncapacitated))
	assert.True(t, manager.HasCondition(target, conditions.ConditionProne))

	// Total should be 3
	conds := manager.GetConditions(target)
	assert.Len(t, conds, 3)
}

func TestExhaustionStacking(t *testing.T) {
	bus := events.NewBus()
	manager := conditions.NewConditionManager(bus)
	exhaustionMgr := conditions.NewExhaustionManager(manager)
	target := &mockEntity{id: "test-target", entityType: "character"}

	// Add 1 level of exhaustion
	err := exhaustionMgr.AddExhaustion(target, 1, "forced_march")
	require.NoError(t, err)
	assert.Equal(t, 1, exhaustionMgr.GetExhaustionLevel(target))

	// Add 2 more levels
	err = exhaustionMgr.AddExhaustion(target, 2, "extreme_heat")
	require.NoError(t, err)
	assert.Equal(t, 3, exhaustionMgr.GetExhaustionLevel(target))

	// Remove 1 level
	err = exhaustionMgr.RemoveExhaustion(target, 1)
	require.NoError(t, err)
	assert.Equal(t, 2, exhaustionMgr.GetExhaustionLevel(target))

	// Long rest removes 1 more
	err = exhaustionMgr.ApplyExhaustionOnRest(target, "long")
	require.NoError(t, err)
	assert.Equal(t, 1, exhaustionMgr.GetExhaustionLevel(target))

	// Clear all
	err = exhaustionMgr.ClearExhaustion(target)
	require.NoError(t, err)
	assert.Equal(t, 0, exhaustionMgr.GetExhaustionLevel(target))
}

func TestExhaustionDeath(t *testing.T) {
	bus := events.NewBus()
	manager := conditions.NewConditionManager(bus)
	exhaustionMgr := conditions.NewExhaustionManager(manager)
	target := &mockEntity{id: "test-target", entityType: "character"}

	// Add 5 levels - not dead yet
	err := exhaustionMgr.AddExhaustion(target, 5, "torture")
	require.NoError(t, err)
	assert.False(t, exhaustionMgr.CheckExhaustionDeath(target))

	// Add 1 more - death
	err = exhaustionMgr.AddExhaustion(target, 1, "final_straw")
	require.NoError(t, err)
	assert.True(t, exhaustionMgr.CheckExhaustionDeath(target))

	// Can't go above 6
	err = exhaustionMgr.AddExhaustion(target, 5, "overkill")
	require.NoError(t, err)
	assert.Equal(t, 6, exhaustionMgr.GetExhaustionLevel(target))
}

func TestConditionDuplication(t *testing.T) {
	bus := events.NewBus()
	manager := conditions.NewConditionManager(bus)
	target := &mockEntity{id: "test-target", entityType: "character"}

	// Apply poisoned
	poisoned1, err := conditions.Poisoned().
		WithTarget(target).
		WithSource("spider1").
		Build()
	require.NoError(t, err)

	err = manager.ApplyCondition(poisoned1)
	require.NoError(t, err)

	// Apply poisoned again from different source
	poisoned2, err := conditions.Poisoned().
		WithTarget(target).
		WithSource("spider2").
		Build()
	require.NoError(t, err)

	err = manager.ApplyCondition(poisoned2)
	require.NoError(t, err)

	// Should only have one poisoned condition (the newer one)
	poisonConds := manager.GetConditionsByType(target, conditions.ConditionPoisoned)
	assert.Len(t, poisonConds, 1)

	// Should be from spider2
	assert.Equal(t, "spider2", poisonConds[0].Source())
}
