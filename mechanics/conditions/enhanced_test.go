// Copyright (C) 2024 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package conditions_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/KirkDiggler/rpg-toolkit/events"
	"github.com/KirkDiggler/rpg-toolkit/mechanics/conditions"
)

// Mock entity for testing
type mockEntity struct {
	id         string
	entityType string
}

func (m *mockEntity) GetID() string   { return m.id }
func (m *mockEntity) GetType() string { return m.entityType }

func TestEnhancedCondition(t *testing.T) {
	target := &mockEntity{id: "test-target", entityType: "character"}

	config := conditions.EnhancedConditionConfig{
		ID:            "test-poisoned",
		ConditionType: conditions.ConditionPoisoned,
		Target:        target,
		Source:        "poison_dart",
		SaveDC:        15,
	}

	cond, err := conditions.NewEnhancedCondition(config)
	require.NoError(t, err)

	assert.Equal(t, "test-poisoned", cond.GetID())
	assert.Equal(t, "poisoned", cond.GetType())
	assert.Equal(t, target, cond.Target())
	assert.Equal(t, "poison_dart", cond.Source())
	assert.Equal(t, conditions.ConditionPoisoned, cond.GetConditionType())
	assert.Equal(t, 15, cond.GetSaveDC())
}

func TestConditionBuilder(t *testing.T) {
	target := &mockEntity{id: "test-target", entityType: "character"}

	t.Run("Basic condition", func(t *testing.T) {
		cond, err := conditions.Poisoned().
			WithTarget(target).
			WithSource("spider_bite").
			WithSaveDC(12).
			WithRoundsDuration(10).
			Build()

		require.NoError(t, err)
		assert.Equal(t, conditions.ConditionPoisoned, cond.GetConditionType())
		assert.Equal(t, 12, cond.GetSaveDC())
	})

	t.Run("Exhaustion with level", func(t *testing.T) {
		cond, err := conditions.Exhaustion(3).
			WithTarget(target).
			WithSource("forced_march").
			Build()

		require.NoError(t, err)
		assert.Equal(t, conditions.ConditionExhaustion, cond.GetConditionType())
		assert.Equal(t, 3, cond.GetLevel())
	})

	t.Run("Charmed with charmer", func(t *testing.T) {
		charmer := &mockEntity{id: "charmer", entityType: "creature"}

		cond, err := conditions.Charmed().
			WithTarget(target).
			WithSource("charm_person").
			WithCharmer(charmer).
			WithMinutesDuration(1).
			Build()

		require.NoError(t, err)
		charmerMeta, exists := cond.GetMetadata("charmer")
		assert.True(t, exists)
		assert.Equal(t, charmer, charmerMeta)
	})

	t.Run("Missing target", func(t *testing.T) {
		_, err := conditions.Blinded().
			WithSource("darkness").
			Build()

		assert.Error(t, err)
		assert.Contains(t, err.Error(), "target is required")
	})
}

func TestPoisonedConditionEffects(t *testing.T) {
	target := &mockEntity{id: "test-target", entityType: "character"}
	bus := events.NewBus()

	// Create poisoned condition
	poisoned, err := conditions.Poisoned().
		WithTarget(target).
		WithSource("poison_cloud").
		Build()
	require.NoError(t, err)

	// Apply the condition
	err = poisoned.Apply(bus)
	require.NoError(t, err)

	// Test attack roll disadvantage
	t.Run("Attack roll disadvantage", func(t *testing.T) {
		attackEvent := events.NewGameEvent(
			events.EventOnAttackRoll,
			target, // Poisoned creature attacking
			&mockEntity{id: "enemy", entityType: "creature"},
		)

		err = bus.Publish(context.Background(), attackEvent)
		require.NoError(t, err)

		// Check that disadvantage was applied
		modifiers := attackEvent.Context().Modifiers()
		found := false
		for _, mod := range modifiers {
			if mod.Type() == events.ModifierDisadvantage {
				found = true
				assert.Equal(t, "poisoned_disadvantage", mod.Source())
			}
		}
		assert.True(t, found, "Poisoned should apply disadvantage to attack rolls")
	})

	// Test ability check disadvantage
	t.Run("Ability check disadvantage", func(t *testing.T) {
		checkEvent := events.NewGameEvent(
			events.EventOnAbilityCheck,
			target, // Poisoned creature making check
			nil,
		)

		err = bus.Publish(context.Background(), checkEvent)
		require.NoError(t, err)

		modifiers := checkEvent.Context().Modifiers()
		found := false
		for _, mod := range modifiers {
			if mod.Type() == events.ModifierDisadvantage {
				found = true
			}
		}
		assert.True(t, found, "Poisoned should apply disadvantage to ability checks")
	})
}

func TestBlindedConditionEffects(t *testing.T) {
	target := &mockEntity{id: "test-target", entityType: "character"}
	attacker := &mockEntity{id: "attacker", entityType: "creature"}
	bus := events.NewBus()

	// Create blinded condition
	blinded, err := conditions.Blinded().
		WithTarget(target).
		WithSource("darkness_spell").
		Build()
	require.NoError(t, err)

	// Apply the condition
	err = blinded.Apply(bus)
	require.NoError(t, err)

	// Test attack roll disadvantage
	t.Run("Attack roll disadvantage", func(t *testing.T) {
		attackEvent := events.NewGameEvent(
			events.EventOnAttackRoll,
			target, // Blinded creature attacking
			attacker,
		)

		err = bus.Publish(context.Background(), attackEvent)
		require.NoError(t, err)

		modifiers := attackEvent.Context().Modifiers()
		found := false
		for _, mod := range modifiers {
			if mod.Type() == events.ModifierDisadvantage {
				found = true
			}
		}
		assert.True(t, found, "Blinded should apply disadvantage to attack rolls")
	})

	// Test attacks against have advantage
	t.Run("Attacks against have advantage", func(t *testing.T) {
		attackEvent := events.NewGameEvent(
			events.EventOnAttackRoll,
			attacker, // Someone attacking the blinded creature
			target,
		)

		err = bus.Publish(context.Background(), attackEvent)
		require.NoError(t, err)

		modifiers := attackEvent.Context().Modifiers()
		found := false
		for _, mod := range modifiers {
			if mod.Type() == events.ModifierAdvantage {
				found = true
			}
		}
		assert.True(t, found, "Attacks against blinded should have advantage")
	})

	// Test auto-fail sight checks
	t.Run("Auto-fail sight checks", func(t *testing.T) {
		checkEvent := events.NewGameEvent(
			events.EventOnAbilityCheck,
			target,
			nil,
		)
		checkEvent.Context().Set("check_type", "perception_sight")

		err = bus.Publish(context.Background(), checkEvent)
		require.NoError(t, err)

		autoFail, exists := checkEvent.Context().Get("auto_fail")
		assert.True(t, exists)
		assert.True(t, autoFail.(bool))
	})
}

func TestGrappledConditionEffects(t *testing.T) {
	target := &mockEntity{id: "test-target", entityType: "character"}
	grappler := &mockEntity{id: "grappler", entityType: "creature"}
	bus := events.NewBus()

	// Create grappled condition
	grappled, err := conditions.Grappled().
		WithTarget(target).
		WithSource("grapple_attack").
		WithGrappler(grappler).
		Build()
	require.NoError(t, err)

	// Apply the condition
	err = grappled.Apply(bus)
	require.NoError(t, err)

	// Test speed zero
	t.Run("Speed reduced to zero", func(t *testing.T) {
		moveEvent := events.NewGameEvent(
			conditions.EventOnMovement,
			target,
			nil,
		)

		err = bus.Publish(context.Background(), moveEvent)
		require.NoError(t, err)

		speedMult, exists := moveEvent.Context().GetFloat64("speed_multiplier")
		assert.True(t, exists)
		assert.Equal(t, 0.0, speedMult)
	})
}

func TestExhaustionCondition(t *testing.T) {
	target := &mockEntity{id: "test-target", entityType: "character"}
	bus := events.NewBus()

	t.Run("Level 1 effects", func(t *testing.T) {
		exhaustion, err := conditions.NewExhaustionCondition(target, 1, "forced_march")
		require.NoError(t, err)

		err = exhaustion.Apply(bus)
		require.NoError(t, err)

		// Test ability check disadvantage
		checkEvent := events.NewGameEvent(
			events.EventOnAbilityCheck,
			target,
			nil,
		)

		err = bus.Publish(context.Background(), checkEvent)
		require.NoError(t, err)

		modifiers := checkEvent.Context().Modifiers()
		hasDisadvantage := false
		for _, mod := range modifiers {
			if mod.Type() == events.ModifierDisadvantage {
				hasDisadvantage = true
			}
		}
		assert.True(t, hasDisadvantage, "Level 1 exhaustion should give disadvantage on ability checks")
	})

	t.Run("Level 3 effects", func(t *testing.T) {
		exhaustion, err := conditions.NewExhaustionCondition(target, 3, "extreme_conditions")
		require.NoError(t, err)

		err = exhaustion.Apply(bus)
		require.NoError(t, err)

		// Test attack roll disadvantage
		attackEvent := events.NewGameEvent(
			events.EventOnAttackRoll,
			target,
			nil,
		)

		err = bus.Publish(context.Background(), attackEvent)
		require.NoError(t, err)

		modifiers := attackEvent.Context().Modifiers()
		hasDisadvantage := false
		for _, mod := range modifiers {
			if mod.Type() == events.ModifierDisadvantage {
				hasDisadvantage = true
			}
		}
		assert.True(t, hasDisadvantage, "Level 3 exhaustion should give disadvantage on attack rolls")

		// Test saving throw disadvantage
		saveEvent := events.NewGameEvent(
			events.EventOnSavingThrow,
			target,
			nil,
		)

		err = bus.Publish(context.Background(), saveEvent)
		require.NoError(t, err)

		modifiers = saveEvent.Context().Modifiers()
		hasDisadvantage = false
		for _, mod := range modifiers {
			if mod.Type() == events.ModifierDisadvantage {
				hasDisadvantage = true
			}
		}
		assert.True(t, hasDisadvantage, "Level 3 exhaustion should give disadvantage on saving throws")
	})

	t.Run("Invalid level", func(t *testing.T) {
		_, err := conditions.NewExhaustionCondition(target, 7, "test")
		assert.Error(t, err)

		_, err = conditions.NewExhaustionCondition(target, 0, "test")
		assert.Error(t, err)
	})
}

func TestConditionRemoval(t *testing.T) {
	target := &mockEntity{id: "test-target", entityType: "character"}
	bus := events.NewBus()

	// Subscribe to attack events
	bus.SubscribeFunc(events.EventOnAttackRoll, 200, func(ctx context.Context, event events.Event) error {
		return nil
	})

	// Create and apply poisoned condition
	poisoned, err := conditions.Poisoned().
		WithTarget(target).
		WithSource("test").
		Build()
	require.NoError(t, err)

	err = poisoned.Apply(bus)
	require.NoError(t, err)

	// Test that handler is called while poisoned
	attackEvent := events.NewGameEvent(
		events.EventOnAttackRoll,
		target,
		nil,
	)

	err = bus.Publish(context.Background(), attackEvent)
	require.NoError(t, err)

	// Should have disadvantage
	modifiers := attackEvent.Context().Modifiers()
	hasDisadvantage := false
	for _, mod := range modifiers {
		if mod.Type() == events.ModifierDisadvantage {
			hasDisadvantage = true
		}
	}
	assert.True(t, hasDisadvantage)

	// Remove the condition
	err = poisoned.Remove(bus)
	require.NoError(t, err)

	// Test that effects are gone
	attackEvent2 := events.NewGameEvent(
		events.EventOnAttackRoll,
		target,
		nil,
	)

	err = bus.Publish(context.Background(), attackEvent2)
	require.NoError(t, err)

	// Should NOT have disadvantage anymore
	modifiers = attackEvent2.Context().Modifiers()
	hasDisadvantage = false
	for _, mod := range modifiers {
		if mod.Type() == events.ModifierDisadvantage {
			hasDisadvantage = true
		}
	}
	assert.False(t, hasDisadvantage, "Removed condition should not apply effects")
}
