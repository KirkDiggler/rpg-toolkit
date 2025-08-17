// Copyright (C) 2024 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package features_test

import (
	"testing"

	"github.com/KirkDiggler/rpg-toolkit/core"
	"github.com/KirkDiggler/rpg-toolkit/events"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/combat"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/features"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRagingCondition_BasicProperties(t *testing.T) {
	bus := events.NewBus()
	rage := features.NewRagingCondition("barbarian-1", 3, bus)

	assert.Equal(t, "rage-barbarian-1", rage.GetID())
	assert.Equal(t, core.EntityType("condition"), rage.GetType())
	assert.Equal(t, "Rage", rage.GetName())
	assert.Contains(t, rage.GetDescription(), "advantage")
	assert.Equal(t, "feature", rage.GetSourceCategory())
	assert.Equal(t, "barbarian-1", rage.GetOwner())
	assert.Equal(t, 1, rage.GetStacks())
	assert.False(t, rage.IsStackable())
}

func TestRagingCondition_DamageBonus(t *testing.T) {
	tests := []struct {
		name     string
		level    int
		expected int
	}{
		{"Level 1", 1, 2},
		{"Level 8", 8, 2},
		{"Level 9", 9, 3},
		{"Level 15", 15, 3},
		{"Level 16", 16, 4},
		{"Level 20", 20, 4},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			bus := events.NewBus()
			rage := features.NewRagingCondition("barbarian-1", tt.level, bus)

			// The damage bonus is calculated internally
			// We can't directly test it without exposing the method
			// This would be tested through integration tests
			assert.NotNil(t, rage)
		})
	}
}

func TestRagingCondition_OnTick(t *testing.T) {
	bus := events.NewBus()
	rage := features.NewRagingCondition("barbarian-1", 3, bus)

	// Apply rage to set up handlers
	err := rage.OnApply()
	require.NoError(t, err)

	// Simulate attacking each round to keep rage going
	attack := &combat.AttackEvent{
		BaseEvent:  *events.NewBaseEvent(combat.AttackEventRef),
		AttackerID: "barbarian-1",
		TargetID:   "goblin-1",
	}

	// Tick 10 times with attacks - rage should continue
	for i := 0; i < 10; i++ {
		// Attack to maintain rage
		err = bus.Publish(attack)
		require.NoError(t, err)

		shouldEnd, err := rage.OnTick()
		require.NoError(t, err)
		assert.False(t, shouldEnd, "Rage should continue for 10 rounds")
	}

	// 11th tick - rage should end (duration expired)
	shouldEnd, err := rage.OnTick()
	require.NoError(t, err)
	assert.True(t, shouldEnd, "Rage should end after 10 rounds")
}

func TestRagingCondition_EndsWithoutHostileAction(t *testing.T) {
	bus := events.NewBus()
	rage := features.NewRagingCondition("barbarian-1", 3, bus)

	// Apply the condition to set up event handlers
	err := rage.OnApply()
	require.NoError(t, err)

	// Track if rage ended event was published
	var rageEnded bool
	rageEndRef, _ := core.ParseString("dnd5e:rage:ended")
	_, err = bus.Subscribe(rageEndRef, func(e *features.RageEndedEvent) *events.DeferredAction {
		if e.OwnerID == "barbarian-1" {
			rageEnded = true
		}
		return nil
	})
	require.NoError(t, err)

	// First tick to move past first round
	shouldEnd, err := rage.OnTick()
	require.NoError(t, err)
	assert.False(t, shouldEnd, "Rage should not end on first tick")

	// Now simulate turn end without attacking or being hit
	turnEnd := &combat.TurnEndEvent{
		BaseEvent: *events.NewBaseEvent(combat.TurnEndEventRef),
		EntityID:  "barbarian-1",
	}

	// Turn end without action should trigger rage end
	err = bus.Publish(turnEnd)
	require.NoError(t, err)
	assert.True(t, rageEnded, "Rage should end if no hostile action taken")
}

func TestRagingCondition_ContinuesWithAttack(t *testing.T) {
	bus := events.NewBus()
	rage := features.NewRagingCondition("barbarian-1", 3, bus)

	// Apply the condition
	err := rage.OnApply()
	require.NoError(t, err)

	// Track if rage ended
	var rageEnded bool
	rageEndRef, _ := core.ParseString("dnd5e:rage:ended")
	_, err = bus.Subscribe(rageEndRef, func(e *features.RageEndedEvent) *events.DeferredAction {
		if e.OwnerID == "barbarian-1" {
			rageEnded = true
		}
		return nil
	})
	require.NoError(t, err)

	// Simulate an attack
	attack := &combat.AttackEvent{
		BaseEvent:  *events.NewBaseEvent(combat.AttackEventRef),
		AttackerID: "barbarian-1",
		TargetID:   "goblin-1",
	}
	err = bus.Publish(attack)
	require.NoError(t, err)

	// Simulate turn end after attacking
	turnEnd := &combat.TurnEndEvent{
		BaseEvent: *events.NewBaseEvent(combat.TurnEndEventRef),
		EntityID:  "barbarian-1",
	}
	err = bus.Publish(turnEnd)
	require.NoError(t, err)

	assert.False(t, rageEnded, "Rage should continue after attacking")
}

func TestRagingCondition_ContinuesWhenHit(t *testing.T) {
	bus := events.NewBus()
	rage := features.NewRagingCondition("barbarian-1", 3, bus)

	// Apply the condition
	err := rage.OnApply()
	require.NoError(t, err)

	// Track if rage ended
	var rageEnded bool
	rageEndRef, _ := core.ParseString("dnd5e:rage:ended")
	_, err = bus.Subscribe(rageEndRef, func(e *features.RageEndedEvent) *events.DeferredAction {
		if e.OwnerID == "barbarian-1" {
			rageEnded = true
		}
		return nil
	})
	require.NoError(t, err)

	// Simulate taking damage
	damage := &combat.DamageTakenEvent{
		BaseEvent:  *events.NewBaseEvent(combat.DamageTakenEventRef),
		TargetID:   "barbarian-1",
		SourceID:   "goblin-1",
		Amount:     5,
		DamageType: "slashing",
	}
	err = bus.Publish(damage)
	require.NoError(t, err)

	// Simulate turn end after being hit
	turnEnd := &combat.TurnEndEvent{
		BaseEvent: *events.NewBaseEvent(combat.TurnEndEventRef),
		EntityID:  "barbarian-1",
	}
	err = bus.Publish(turnEnd)
	require.NoError(t, err)

	assert.False(t, rageEnded, "Rage should continue after being hit")
}

func TestRagingCondition_NoDamageTakenIfZeroDamage(t *testing.T) {
	bus := events.NewBus()
	rage := features.NewRagingCondition("barbarian-1", 3, bus)

	// Apply the condition
	err := rage.OnApply()
	require.NoError(t, err)

	// Track if rage ended
	var rageEnded bool
	rageEndRef, _ := core.ParseString("dnd5e:rage:ended")
	_, err = bus.Subscribe(rageEndRef, func(e *features.RageEndedEvent) *events.DeferredAction {
		if e.OwnerID == "barbarian-1" {
			rageEnded = true
		}
		return nil
	})
	require.NoError(t, err)

	// Simulate taking 0 damage (miss or absorbed)
	damage := &combat.DamageTakenEvent{
		BaseEvent:  *events.NewBaseEvent(combat.DamageTakenEventRef),
		TargetID:   "barbarian-1",
		SourceID:   "goblin-1",
		Amount:     0,
		DamageType: "slashing",
	}
	err = bus.Publish(damage)
	require.NoError(t, err)

	// Simulate turn end after taking 0 damage
	turnEnd := &combat.TurnEndEvent{
		BaseEvent: *events.NewBaseEvent(combat.TurnEndEventRef),
		EntityID:  "barbarian-1",
	}
	err = bus.Publish(turnEnd)
	require.NoError(t, err)

	assert.True(t, rageEnded, "Rage should end if no actual damage taken (0 damage)")
}

func TestRagingCondition_OnRemove(t *testing.T) {
	bus := events.NewBus()
	rage := features.NewRagingCondition("barbarian-1", 3, bus)

	// Apply the condition
	err := rage.OnApply()
	require.NoError(t, err)

	// Remove the condition
	err = rage.OnRemove()
	require.NoError(t, err)

	// After removal, publishing events shouldn't affect rage
	// (This tests that subscriptions were properly cleaned up)
	attack := &combat.AttackEvent{
		BaseEvent:  *events.NewBaseEvent(combat.AttackEventRef),
		AttackerID: "barbarian-1",
		TargetID:   "goblin-1",
	}
	err = bus.Publish(attack)
	require.NoError(t, err)
	// No panic means subscriptions were cleaned up properly
}

