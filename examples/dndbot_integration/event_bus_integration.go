// Package dndbot shows how to replace the DND bot's event bus
package dndbot

import (
	"context"
	"fmt"

	"github.com/KirkDiggler/rpg-toolkit/events"
)

// EventBusAdapter replaces the DND bot's event system with toolkit's event bus
type EventBusAdapter struct {
	bus *events.Bus

	// Map old event types to new event names
	eventTypeMap map[string]string
}

// NewEventBusAdapter creates a new event bus adapter
func NewEventBusAdapter() *EventBusAdapter {
	return &EventBusAdapter{
		bus: events.NewBus(),
		eventTypeMap: map[string]string{
			// Combat events
			"OnAttackRoll":   events.EventOnAttackRoll,
			"OnDamageRoll":   events.EventOnDamageRoll,
			"OnBeforeAttack": events.EventBeforeAttackRoll,
			"OnAfterAttack":  events.EventAfterAttackRoll,
			"OnHit":          events.EventOnHit,
			"OnMiss":         events.EventAfterAttackRoll, // Map to after attack
			"OnCriticalHit":  events.EventOnHit,
			"OnTurnStart":    events.EventOnTurnStart,
			"OnTurnEnd":      events.EventOnTurnEnd,

			// Ability/Save events
			"OnAbilityCheck": events.EventOnAbilityCheck,
			"OnSavingThrow":  events.EventOnSavingThrow,
			"OnSkillCheck":   events.EventOnAbilityCheck,

			// Status events
			"OnConditionApply":  events.EventOnConditionApplied,
			"OnConditionRemove": events.EventOnConditionRemoved,
			"OnStatusApply":     events.EventOnStatusApplied,
			"OnStatusRemove":    events.EventOnStatusRemoved,

			// Rest events
			"OnShortRest": events.EventOnShortRest,
			"OnLongRest":  events.EventOnLongRest,

			// Resource events
			"OnResourceUse":     "resource.consumed",
			"OnResourceRestore": "resource.restored",
			"OnSpellCast":       events.EventOnSpellCast,

			// Character events
			"OnLevelUp": "character.levelup",
			"OnDeath":   "character.death",
			"OnRevive":  "character.revive",
			"OnHeal":    "character.heal",
			"OnDamage":  events.EventOnTakeDamage,
		},
	}
}

// Subscribe replaces the old bot's Subscribe method
// This maintains compatibility with existing bot code
func (a *EventBusAdapter) Subscribe(eventType string, priority int, handler func(interface{}) error) string {
	// Map old event type to new
	toolkitEvent, ok := a.eventTypeMap[eventType]
	if !ok {
		// If no mapping, use as-is
		toolkitEvent = eventType
	}

	// Create a handler function that adapts the interface
	handlerFunc := func(_ context.Context, e events.Event) error {
		// Convert toolkit event to bot's expected format
		// In real implementation, this would convert Event to bot's event type
		return handler(e)
	}

	// Subscribe with priority
	return a.bus.SubscribeFunc(toolkitEvent, priority, handlerFunc)
}

// Publish replaces the old bot's Publish method
func (a *EventBusAdapter) Publish(eventType string, data interface{}) error {
	ctx := context.Background()

	// Map old event type to new
	toolkitEvent, ok := a.eventTypeMap[eventType]
	if !ok {
		toolkitEvent = eventType
	}

	// Create event based on type and data
	// In real implementation, this would properly convert data
	var event events.Event

	switch v := data.(type) {
	case map[string]interface{}:
		// Most common case - data is already a map
		source, _ := v["source"].(CharacterWrapper)
		target, _ := v["target"].(CharacterWrapper)

		event = events.NewGameEvent(toolkitEvent, &source, &target)

		// Copy other data to context
		for k, val := range v {
			if k != "source" && k != "target" {
				event.Context().Set(k, val)
			}
		}

	default:
		// Simple data - wrap in event
		event = events.NewGameEvent(toolkitEvent, nil, nil)
		event.Context().Set("data", data)
	}

	return a.bus.Publish(ctx, event)
}

// GetToolkitBus returns the underlying toolkit event bus
// Use this for new code that works directly with toolkit
func (a *EventBusAdapter) GetToolkitBus() *events.Bus {
	return a.bus
}

// MigrateEventHandlers shows how to migrate existing event handlers
func MigrateEventHandlers(_ *EventBusAdapter) {
	// Example: Migrate attack handler
	/*
		// Old bot code:
		eventBus.Subscribe("OnAttackRoll", 100, func(data interface{}) error {
			attackData := data.(*AttackEventData)
			// Add proficiency bonus
			if character.IsProficient(attackData.Weapon) {
				attackData.Bonus += character.GetProficiencyBonus()
			}
			return nil
		})

		// New code using adapter (maintains compatibility):
		adapter.Subscribe("OnAttackRoll", 100, func(data interface{}) error {
			// Same handler code works!
			attackData := data.(*AttackEventData)
			if character.IsProficient(attackData.Weapon) {
				attackData.Bonus += character.GetProficiencyBonus()
			}
			return nil
		})

		// Or use toolkit directly for new code:
		adapter.GetToolkitBus().SubscribeFunc(events.EventOnAttackRoll, 100,
			func(ctx context.Context, e events.Event) error {
				weapon, _ := e.Context().GetString("weapon")
				if proficiencyService.CheckProficiency(e.Source().GetID(), weapon) {
					bonus := GetProficiencyBonus(characterLevel)
					e.Context().AddModifier(events.NewModifier("proficiency",
						events.ModifierAttackBonus,
						events.NewRawValue(bonus, "proficiency"), 100))
				}
				return nil
			})
	*/
}

// ExampleEventBusMigration shows gradual migration from old to new event system
func ExampleEventBusMigration() {
	// Create the adapter
	adapter := NewEventBusAdapter()

	// Old bot code continues to work
	adapter.Subscribe("OnAttackRoll", 100, func(_ interface{}) error {
		fmt.Println("Old style handler called")
		return nil
	})

	// New code can use toolkit features
	bus := adapter.GetToolkitBus()
	bus.SubscribeFunc(events.EventOnDamageRoll, 100, func(_ context.Context, e events.Event) error {
		fmt.Println("New style handler with full event features")

		// Can check for conditions
		if hasRage, _ := e.Context().GetBool("has_rage"); hasRage {
			e.Context().AddModifier(events.NewModifier("rage", "damage_bonus",
				events.NewRawValue(2, "rage"), 50))
		}

		return nil
	})

	// Publish works for both old and new style
	_ = adapter.Publish("OnAttackRoll", map[string]interface{}{
		"attacker": "fighter-123",
		"target":   "goblin-456",
		"weapon":   "longsword",
	})
}
