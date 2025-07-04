// Copyright (C) 2024 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package events

import (
	"context"
	"time"

	"github.com/KirkDiggler/rpg-toolkit/core"
)

// Event represents something that happened in the game.
type Event interface {
	// Type returns the event type (e.g., "attack_roll", "damage_calculation").
	Type() string

	// Source returns the entity that triggered the event.
	Source() core.Entity

	// Target returns the entity affected by the event, if any.
	Target() core.Entity

	// Timestamp returns when the event occurred.
	Timestamp() time.Time

	// Context returns the event-specific context with additional data.
	Context() Context

	// IsCancelled returns whether the event has been cancelled.
	IsCancelled() bool

	// Cancel marks the event as cancelled, preventing further processing.
	Cancel()
}

// Context holds event-specific data and allows modifications.
type Context interface {
	// Get retrieves a value by key.
	Get(key string) (interface{}, bool)

	// Set stores a value by key.
	Set(key string, value interface{})

	// AddModifier adds a modifier that can affect calculations.
	AddModifier(modifier Modifier)

	// Modifiers returns all modifiers added to this context.
	Modifiers() []Modifier
}

// Modifier represents a modification to be applied during event processing.
type Modifier interface {
	// Source identifies what created this modifier (e.g., "rage", "bless").
	Source() string

	// Type categorizes the modifier (e.g., "damage", "attack_roll").
	Type() string

	// Value returns the modifier value.
	// Deprecated: Use ModifierValue() for type-safe access.
	Value() interface{}

	// ModifierValue returns the typed modifier value.
	ModifierValue() ModifierValue

	// Priority determines application order (higher = later).
	Priority() int
}

// GameEvent is the standard implementation of Event.
type GameEvent struct {
	eventType string
	source    core.Entity
	target    core.Entity
	timestamp time.Time
	context   Context
	cancelled bool
}

// NewGameEvent creates a new game event.
func NewGameEvent(eventType string, source, target core.Entity) *GameEvent {
	return &GameEvent{
		eventType: eventType,
		source:    source,
		target:    target,
		timestamp: time.Now(),
		context:   NewEventContext(),
	}
}

// Type returns the event type.
func (e *GameEvent) Type() string { return e.eventType }

// Source returns the event source.
func (e *GameEvent) Source() core.Entity { return e.source }

// Target returns the event target.
func (e *GameEvent) Target() core.Entity { return e.target }

// Timestamp returns the event timestamp.
func (e *GameEvent) Timestamp() time.Time { return e.timestamp }

// Context returns the event context.
func (e *GameEvent) Context() Context { return e.context }

// IsCancelled returns whether the event has been cancelled.
func (e *GameEvent) IsCancelled() bool { return e.cancelled }

// Cancel marks the event as cancelled.
func (e *GameEvent) Cancel() { e.cancelled = true }

// EventContext is the standard implementation of Context.
type EventContext struct {
	data      map[string]interface{}
	modifiers []Modifier
}

// NewEventContext creates a new event context.
func NewEventContext() *EventContext {
	return &EventContext{
		data:      make(map[string]interface{}),
		modifiers: []Modifier{},
	}
}

// Get retrieves a value by key.
func (c *EventContext) Get(key string) (interface{}, bool) {
	val, ok := c.data[key]
	return val, ok
}

// Set stores a value by key.
func (c *EventContext) Set(key string, value interface{}) {
	c.data[key] = value
}

// AddModifier adds a modifier to this context.
func (c *EventContext) AddModifier(modifier Modifier) {
	c.modifiers = append(c.modifiers, modifier)
}

// Modifiers returns all modifiers added to this context.
func (c *EventContext) Modifiers() []Modifier {
	return c.modifiers
}

// BasicModifier is a simple implementation of Modifier.
type BasicModifier struct {
	source   string
	modType  string
	modValue ModifierValue
	priority int
}

// NewModifier creates a new basic modifier.
func NewModifier(source, modType string, value ModifierValue, priority int) *BasicModifier {
	return &BasicModifier{
		source:   source,
		modType:  modType,
		modValue: value,
		priority: priority,
	}
}

// Source returns the source of the modifier.
func (m *BasicModifier) Source() string { return m.source }

// Type returns the type of the modifier.
func (m *BasicModifier) Type() string { return m.modType }

// Value returns the value of the modifier.
func (m *BasicModifier) Value() interface{} { return m.modValue }

// ModifierValue returns the value of the modifier.
func (m *BasicModifier) ModifierValue() ModifierValue { return m.modValue }

// Priority returns the priority of the modifier.
func (m *BasicModifier) Priority() int { return m.priority }

// Common event types
const (
	// Attack sequence events
	EventBeforeAttackRoll = "before_attack_roll"
	EventOnAttackRoll     = "on_attack_roll"
	EventAfterAttackRoll  = "after_attack_roll"
	EventBeforeHit        = "before_hit"
	EventOnHit            = "on_hit"
	EventAfterHit         = "after_hit"

	// Damage events
	EventBeforeDamageRoll = "before_damage_roll"
	EventOnDamageRoll     = "on_damage_roll"
	EventAfterDamageRoll  = "after_damage_roll"
	EventBeforeTakeDamage = "before_take_damage"
	EventOnTakeDamage     = "on_take_damage"
	EventAfterTakeDamage  = "after_take_damage"

	// Saving throws
	EventBeforeSavingThrow = "before_saving_throw"
	EventOnSavingThrow     = "on_saving_throw"
	EventAfterSavingThrow  = "after_saving_throw"

	// Ability checks
	EventBeforeAbilityCheck = "before_ability_check"
	EventOnAbilityCheck     = "on_ability_check"
	EventAfterAbilityCheck  = "after_ability_check"

	// Turn management
	EventOnTurnStart = "on_turn_start"
	EventOnTurnEnd   = "on_turn_end"

	// Status effects
	EventOnStatusApplied = "on_status_applied"
	EventOnStatusRemoved = "on_status_removed"

	// Rest events
	EventOnShortRest = "on_short_rest"
	EventOnLongRest  = "on_long_rest"

	// Spell events
	EventOnSpellCast   = "on_spell_cast"
	EventOnSpellDamage = "on_spell_damage"

	// Condition events
	EventOnConditionApplied = "on_condition_applied"
	EventOnConditionRemoved = "on_condition_removed"
)

// Common modifier types
const (
	ModifierAttackBonus  = "attack_bonus"
	ModifierDamageBonus  = "damage_bonus"
	ModifierACBonus      = "ac_bonus"
	ModifierSaveBonus    = "save_bonus"
	ModifierAdvantage    = "advantage"
	ModifierDisadvantage = "disadvantage"
)

// HandlerFunc is a function that handles events.
type HandlerFunc func(ctx context.Context, event Event) error
