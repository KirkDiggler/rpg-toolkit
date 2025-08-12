// Copyright (C) 2024 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package effects_test

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/KirkDiggler/rpg-toolkit/core"
	"github.com/KirkDiggler/rpg-toolkit/events"
	"github.com/KirkDiggler/rpg-toolkit/mechanics/effects"
)

func TestBehaviorComposition(t *testing.T) {
	// Create a character
	character := &MockEntity{id: "fighter-1", typ: "character"}
	bus := events.NewBus()

	// Create a Bless condition using composition
	bless := effects.CreateBlessCondition(character, &core.Source{Category: core.SourceManual, Name: "cleric-spell"})

	// Apply the condition
	err := bless.Apply(bus)
	require.NoError(t, err)
	assert.True(t, bless.IsActive())

	// Verify it has the expected behaviors
	assert.Equal(t, "condition.bless", bless.GetType())
	assert.Equal(t, character, bless.Owner())

	// Test that it adds dice to attack rolls
	var modifiersApplied []interface{}
	bus.SubscribeFunc("attack.before", 100, events.HandlerFunc(func(_ context.Context, e events.Event) error {
		if gameEvent, ok := e.(*events.GameEvent); ok {
			val, exists := gameEvent.Context().Get("modifiers")
			if mods, ok := val.([]interface{}); exists && ok {
				modifiersApplied = mods
			}
		}
		return nil
	}))

	// Simulate an attack
	attackEvent := events.NewGameEvent("attack.before", character, nil)
	attackEvent.Context().Set("attacker", character)
	attackEvent.Context().Set("modifiers", []interface{}{})

	err = bus.Publish(context.Background(), attackEvent)
	require.NoError(t, err)

	// Should have added a 1d4 modifier
	assert.Len(t, modifiersApplied, 1)
	if len(modifiersApplied) > 0 {
		mod := modifiersApplied[0].(map[string]interface{})
		// Check that we have a dice.Roll value
		rollValue, ok := mod["value"].(events.ModifierValue)
		assert.True(t, ok, "Expected ModifierValue in 'value' field")
		if ok {
			// The value should be between 1 and 4 for a d4
			val := rollValue.GetValue()
			assert.GreaterOrEqual(t, val, 1)
			assert.LessOrEqual(t, val, 4)
			// Description should contain d4
			assert.Contains(t, rollValue.GetDescription(), "d4")
		}
		assert.Equal(t, "dice", mod["type"])
	}
}

func TestConditionalBehavior(t *testing.T) {
	owner := &MockEntity{id: "rogue-1", typ: "character"}
	bus := events.NewBus()

	// Create a conditional effect that only works in dim light
	sneakAttack := &ConditionalDamageBonus{
		Core: effects.NewCore(effects.CoreConfig{
			ID:   "sneak-attack",
			Type: "feature",
		}),
		owner: owner,
		checkFunc: func(_ context.Context, e events.Event) bool {
			// Check for advantage or ally nearby
			if gameEvent, ok := e.(*events.GameEvent); ok {
				advVal, _ := gameEvent.Context().Get("advantage")
				hasAdvantage, _ := advVal.(bool)
				allyVal, _ := gameEvent.Context().Get("ally_nearby")
				hasAllyNearby, _ := allyVal.(bool)
				return hasAdvantage || hasAllyNearby
			}
			return false
		},
	}

	// Apply the effect
	err := sneakAttack.Apply(bus)
	require.NoError(t, err)

	// Subscribe to track damage additions
	var damageAdded bool
	bus.SubscribeFunc("damage.calculate", 100, events.HandlerFunc(func(_ context.Context, e events.Event) error {
		if gameEvent, ok := e.(*events.GameEvent); ok {
			val, _ := gameEvent.Context().Get("source")
			if source, _ := val.(string); source == "sneak-attack" {
				damageAdded = true
			}
		}
		return nil
	}))

	// Test without conditions met
	attackEvent := events.NewGameEvent("attack.before", owner, nil)
	attackEvent.Context().Set("attacker", owner)
	attackEvent.Context().Set("advantage", false)

	err = bus.Publish(context.Background(), attackEvent)
	require.NoError(t, err)
	assert.False(t, damageAdded)

	// Test with advantage
	attackEvent.Context().Set("advantage", true)
	err = bus.Publish(context.Background(), attackEvent)
	require.NoError(t, err)
	// In real implementation, this would trigger damage calculation
}

func TestTemporaryEffectExpiration(t *testing.T) {
	// owner := &MockEntity{id: "wizard-1", typ: "character"}
	bus := events.NewBus()

	// Create a temporary effect that lasts 3 rounds
	mageArmor := &TemporaryACBonus{
		Core: effects.NewCore(effects.CoreConfig{
			ID:   "mage-armor",
			Type: "spell_effect",
		}),
		duration: effects.Duration{
			Type:  effects.DurationRounds,
			Value: 3,
		},
		roundsElapsed: 0,
	}

	// Apply the effect
	err := mageArmor.Apply(bus)
	require.NoError(t, err)

	// Simulate round progression
	for i := 0; i < 3; i++ {
		roundEvent := events.NewGameEvent("time.round_end", nil, nil)
		err = bus.Publish(context.Background(), roundEvent)
		require.NoError(t, err)

		if i < 2 {
			assert.True(t, mageArmor.IsActive(), "Should still be active on round %d", i+1)
		}
	}

	// After 3 rounds, should expire
	assert.False(t, mageArmor.IsActive(), "Should expire after 3 rounds")
}

func TestStackingBehavior(t *testing.T) {
	owner := &MockEntity{id: "fighter-1", typ: "character"}

	// Create two instances of a stackable effect (ability damage)
	strDamage1 := &StackableAbilityDamage{
		Core: effects.NewCore(effects.CoreConfig{
			ID:   "str-damage-1",
			Type: "ability_damage",
		}),
		ability: "strength",
		amount:  2,
	}

	strDamage2 := &StackableAbilityDamage{
		Core: effects.NewCore(effects.CoreConfig{
			ID:   "str-damage-2",
			Type: "ability_damage",
		}),
		ability: "strength",
		amount:  3,
	}

	// They should stack
	assert.True(t, strDamage1.CanStackWith(strDamage2))
	assert.Equal(t, effects.StackingAdd, strDamage1.GetStackingRule())

	// Create a non-stackable effect
	bless := effects.CreateBlessCondition(owner, &core.Source{Category: core.SourceManual, Name: "spell"})
	noStack := &effects.NoStacking{}

	// Bless shouldn't stack
	assert.False(t, noStack.CanStackWith(bless))
}

func TestDiceModifierBehavior(t *testing.T) {
	ctx := context.Background()

	// Create a dice modifier that adds 2d6 fire damage
	flameTongue := &effects.SimpleDiceModifier{
		Expression: "2d6",
		ModType:    effects.ModifierDamage,
		AppliesTo: func(_ context.Context, e events.Event) bool {
			// Only on weapon attacks
			if gameEvent, ok := e.(*events.GameEvent); ok {
				val, _ := gameEvent.Context().Get("weapon_type")
				weaponType, _ := val.(string)
				return weaponType == "sword"
			}
			return false
		},
	}

	// Test the modifier
	assert.Equal(t, "2d6", flameTongue.GetDiceExpression(ctx, nil))
	assert.Equal(t, effects.ModifierDamage, flameTongue.GetModifierType())

	// Test conditional application
	swordAttack := events.NewGameEvent("damage.calculate", nil, nil)
	swordAttack.Context().Set("weapon_type", "sword")

	bowAttack := events.NewGameEvent("damage.calculate", nil, nil)
	bowAttack.Context().Set("weapon_type", "bow")

	assert.True(t, flameTongue.ShouldApply(ctx, swordAttack))
	assert.False(t, flameTongue.ShouldApply(ctx, bowAttack))
}

func TestBlessConditionStartTime(t *testing.T) {
	owner := &MockEntity{id: "cleric-1", typ: "character"}

	// Create bless condition
	before := time.Now()
	bless := effects.CreateBlessCondition(owner, &core.Source{Category: core.SourceManual, Name: "divine-favor"})
	after := time.Now()

	// Get the temporary behavior
	tempEffect := bless.GetTemporary()
	require.NotNil(t, tempEffect)

	// Check that StartTime is set correctly
	simpleDur, ok := tempEffect.(*effects.SimpleDuration)
	require.True(t, ok, "Expected SimpleDuration type")

	assert.True(t, simpleDur.StartTime.After(before) || simpleDur.StartTime.Equal(before),
		"StartTime should be after or equal to before time")
	assert.True(t, simpleDur.StartTime.Before(after) || simpleDur.StartTime.Equal(after),
		"StartTime should be before or equal to after time")

	// Verify it doesn't expire immediately
	assert.False(t, simpleDur.CheckExpiration(context.Background(), time.Now()),
		"Effect should not expire immediately after creation")
}

func TestFreshDiceRollsEachTime(t *testing.T) {
	// Test that dice modifiers create fresh rolls each time
	character := &MockEntity{id: "fighter-1", typ: "character"}
	bus := events.NewBus()

	// Create a Bless condition
	bless := effects.CreateBlessCondition(character, &core.Source{Category: core.SourceManual, Name: "cleric-spell"})
	err := bless.Apply(bus)
	require.NoError(t, err)

	// Collect multiple modifier values
	var rollValues []int
	var rollDescriptions []string

	// Subscribe to capture modifiers
	bus.SubscribeFunc("attack.before", 100, events.HandlerFunc(func(_ context.Context, e events.Event) error {
		if gameEvent, ok := e.(*events.GameEvent); ok {
			val, _ := gameEvent.Context().Get("modifiers")
			if mods, ok := val.([]interface{}); ok {
				for _, modInterface := range mods {
					if mod, ok := modInterface.(map[string]interface{}); ok {
						if rollValue, ok := mod["value"].(events.ModifierValue); ok {
							rollValues = append(rollValues, rollValue.GetValue())
							rollDescriptions = append(rollDescriptions, rollValue.GetDescription())
						}
					}
				}
			}
		}
		return nil
	}))

	// Simulate multiple attacks
	for i := 0; i < 10; i++ {
		attackEvent := events.NewGameEvent("attack.before", character, nil)
		attackEvent.Context().Set("attacker", character)
		attackEvent.Context().Set("modifiers", []interface{}{})

		err = bus.Publish(context.Background(), attackEvent)
		require.NoError(t, err)
	}

	// We should have 10 rolls
	assert.Len(t, rollValues, 10)

	// All values should be between 1 and 4
	for i, val := range rollValues {
		assert.GreaterOrEqual(t, val, 1, "Roll %d should be at least 1", i)
		assert.LessOrEqual(t, val, 4, "Roll %d should be at most 4", i)
	}

	// With 10 d4 rolls, we should see some variety (not all the same)
	// This could theoretically fail but is extremely unlikely
	uniqueValues := make(map[int]bool)
	for _, val := range rollValues {
		uniqueValues[val] = true
	}
	assert.Greater(t, len(uniqueValues), 1, "Should see different roll values with 10 d4 rolls")

	// All descriptions should contain d4
	for _, desc := range rollDescriptions {
		assert.Contains(t, desc, "d4")
	}
}

// Test implementations

type ConditionalDamageBonus struct {
	*effects.Core
	owner     *MockEntity
	checkFunc func(context.Context, events.Event) bool
}

func (c *ConditionalDamageBonus) CheckCondition(ctx context.Context, e events.Event) bool {
	return c.checkFunc(ctx, e)
}

func (c *ConditionalDamageBonus) Apply(bus events.EventBus) error {
	c.Subscribe(bus, "attack.before", 50, events.HandlerFunc(func(ctx context.Context, e events.Event) error {
		if c.CheckCondition(ctx, e) {
			// Add sneak attack damage
			damageEvent := events.NewGameEvent("damage.add", c, nil)
			damageEvent.Context().Set("source", "sneak-attack")
			damageEvent.Context().Set("dice", "3d6")
			_ = bus.Publish(ctx, damageEvent)
		}
		return nil
	}))
	return c.Core.Apply(bus)
}

type TemporaryACBonus struct {
	*effects.Core
	duration      effects.Duration
	roundsElapsed int
}

func (t *TemporaryACBonus) GetDuration() effects.Duration { return t.duration }

func (t *TemporaryACBonus) CheckExpiration(_ context.Context, _ time.Time) bool {
	return t.roundsElapsed >= t.duration.Value
}

func (t *TemporaryACBonus) OnExpire(bus events.EventBus) error {
	return t.Remove(bus)
}

func (t *TemporaryACBonus) Apply(bus events.EventBus) error {
	// Track rounds
	t.Subscribe(bus, "time.round_end", 10, events.HandlerFunc(func(ctx context.Context, _ events.Event) error {
		t.roundsElapsed++
		if t.CheckExpiration(ctx, time.Now()) {
			return t.OnExpire(bus)
		}
		return nil
	}))
	return t.Core.Apply(bus)
}

type StackableAbilityDamage struct {
	*effects.Core
	ability string
	amount  int
}

func (s *StackableAbilityDamage) GetStackingRule() effects.StackingRule {
	return effects.StackingAdd
}

func (s *StackableAbilityDamage) CanStackWith(other core.Entity) bool {
	if otherDamage, ok := other.(*StackableAbilityDamage); ok {
		return otherDamage.ability == s.ability
	}
	return false
}

func (s *StackableAbilityDamage) Stack(other core.Entity) error {
	if otherDamage, ok := other.(*StackableAbilityDamage); ok {
		s.amount += otherDamage.amount
	}
	return nil
}

// MockEntity for testing
type MockEntity struct {
	id  string
	typ string
}

func (m *MockEntity) GetID() string   { return m.id }
func (m *MockEntity) GetType() string { return m.typ }
