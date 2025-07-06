package dndbot

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/KirkDiggler/rpg-toolkit/events"
)

func TestEventBusAdapter(t *testing.T) {
	t.Run("old style subscription works", func(t *testing.T) {
		adapter := NewEventBusAdapter()

		// Track if handler was called
		called := false
		var receivedData interface{}

		// Subscribe old style
		adapter.Subscribe("OnAttackRoll", 100, func(data interface{}) error {
			called = true
			receivedData = data
			return nil
		})

		// Publish old style
		testData := map[string]interface{}{
			"attacker": "fighter-123",
			"target":   "goblin-456",
			"weapon":   "longsword",
		}

		err := adapter.Publish("OnAttackRoll", testData)
		require.NoError(t, err)

		assert.True(t, called, "Handler should have been called")
		assert.NotNil(t, receivedData, "Should have received data")
	})

	t.Run("event type mapping", func(t *testing.T) {
		adapter := NewEventBusAdapter()
		bus := adapter.GetToolkitBus()

		// Subscribe to toolkit event directly
		called := false
		bus.SubscribeFunc(events.EventOnAttackRoll, 100, func(_ context.Context, _ events.Event) error {
			called = true
			return nil
		})

		// Publish using old event type - should map to toolkit type
		err := adapter.Publish("OnAttackRoll", map[string]interface{}{})
		require.NoError(t, err)

		assert.True(t, called, "Toolkit handler should receive mapped event")
	})

	t.Run("mixed old and new handlers", func(t *testing.T) {
		adapter := NewEventBusAdapter()
		bus := adapter.GetToolkitBus()

		oldCalled := false
		newCalled := false

		// Old style handler
		adapter.Subscribe("OnDamageRoll", 100, func(_ interface{}) error {
			oldCalled = true
			return nil
		})

		// New style handler for same event
		bus.SubscribeFunc(events.EventOnDamageRoll, 100, func(_ context.Context, _ events.Event) error {
			newCalled = true
			return nil
		})

		// Publish should trigger both
		err := adapter.Publish("OnDamageRoll", map[string]interface{}{
			"damage": 10,
			"type":   "slashing",
		})
		require.NoError(t, err)

		assert.True(t, oldCalled, "Old style handler should be called")
		assert.True(t, newCalled, "New style handler should be called")
	})

	t.Run("proficiency bonus via events", func(t *testing.T) {
		adapter := NewEventBusAdapter()
		bus := adapter.GetToolkitBus()

		// Create proficiency integration
		profIntegration := NewProficiencyIntegration(bus)

		// Add fighter proficiencies
		err := profIntegration.AddCharacterProficiencies("fighter-123", 5)
		require.NoError(t, err)

		// Subscribe to attack rolls to add proficiency bonus
		bus.SubscribeFunc(events.EventOnAttackRoll, 100, func(_ context.Context, e events.Event) error {
			// Get weapon from context
			weapon, ok := e.Context().GetString("weapon")
			if !ok {
				return nil
			}

			// Get attacker ID
			if e.Source() == nil {
				return nil
			}
			attackerID := e.Source().GetID()

			// Check proficiency and add modifier
			if profIntegration.CheckProficiency(attackerID, weapon) {
				bonus := GetProficiencyBonus(5) // Level 5 = +3
				e.Context().AddModifier(events.NewModifier(
					"proficiency",
					events.ModifierAttackBonus,
					events.NewRawValue(bonus, "proficiency"),
					100,
				))
			}

			return nil
		})

		// Create attack event
		attacker := WrapCharacter("fighter-123", "Fighter", 5)
		target := WrapCharacter("goblin-1", "Goblin", 1)

		attackEvent := events.NewGameEvent(events.EventOnAttackRoll, attacker, target)
		attackEvent.Context().Set("weapon", "longsword")

		// Publish event
		ctx := context.Background()
		err = bus.Publish(ctx, attackEvent)
		require.NoError(t, err)

		// Check that proficiency modifier was added
		modifiers := attackEvent.Context().Modifiers()
		require.Len(t, modifiers, 1, "Should have proficiency modifier")

		mod := modifiers[0]
		assert.Equal(t, "proficiency", mod.Source())
		assert.Equal(t, events.ModifierAttackBonus, mod.Type())
		assert.Equal(t, 3, mod.ModifierValue().GetValue()) // Level 5 = +3
	})
}

// TestEventPriority shows how priority affects handler order
func TestEventPriority(t *testing.T) {
	adapter := NewEventBusAdapter()
	bus := adapter.GetToolkitBus()

	var callOrder []string

	// Subscribe with different priorities
	bus.SubscribeFunc(events.EventOnAttackRoll, 100, func(_ context.Context, _ events.Event) error {
		callOrder = append(callOrder, "high-priority")
		return nil
	})

	bus.SubscribeFunc(events.EventOnAttackRoll, 50, func(_ context.Context, _ events.Event) error {
		callOrder = append(callOrder, "medium-priority")
		return nil
	})

	bus.SubscribeFunc(events.EventOnAttackRoll, 10, func(_ context.Context, _ events.Event) error {
		callOrder = append(callOrder, "low-priority")
		return nil
	})

	// Publish event
	event := events.NewGameEvent(events.EventOnAttackRoll, nil, nil)
	err := bus.Publish(context.Background(), event)
	require.NoError(t, err)

	// Check order - higher priority numbers execute later
	assert.Equal(t, []string{"low-priority", "medium-priority", "high-priority"}, callOrder)
}
