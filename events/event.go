// Copyright (C) 2024 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package events

import (
	"context"
	"time"

	"github.com/KirkDiggler/rpg-toolkit/core"
)

// EventType represents a type-safe event type.
type EventType int

// Event type constants for type-safe access.
const (
	// Attack sequence events
	EventTypeBeforeAttackRoll EventType = iota
	EventTypeOnAttackRoll
	EventTypeAfterAttackRoll
	EventTypeBeforeHit
	EventTypeOnHit
	EventTypeAfterHit

	// Damage events
	EventTypeBeforeDamageRoll
	EventTypeOnDamageRoll
	EventTypeAfterDamageRoll
	EventTypeBeforeTakeDamage
	EventTypeOnTakeDamage
	EventTypeAfterTakeDamage

	// Saving throws
	EventTypeBeforeSavingThrow
	EventTypeOnSavingThrow
	EventTypeAfterSavingThrow

	// Ability checks
	EventTypeBeforeAbilityCheck
	EventTypeOnAbilityCheck
	EventTypeAfterAbilityCheck

	// Turn management
	EventTypeOnTurnStart
	EventTypeOnTurnEnd

	// Status effects
	EventTypeOnStatusApplied
	EventTypeOnStatusRemoved

	// Rest events
	EventTypeOnShortRest
	EventTypeOnLongRest

	// Spell events
	EventTypeOnSpellCast
	EventTypeOnSpellDamage

	// Condition events
	EventTypeOnConditionApplied
	EventTypeOnConditionRemoved

	// Custom event type for extensions
	EventTypeCustom
)

// eventTypeToString maps EventType to string for backward compatibility.
var eventTypeToString = map[EventType]string{
	EventTypeBeforeAttackRoll:   EventBeforeAttackRoll,
	EventTypeOnAttackRoll:       EventOnAttackRoll,
	EventTypeAfterAttackRoll:    EventAfterAttackRoll,
	EventTypeBeforeHit:          EventBeforeHit,
	EventTypeOnHit:              EventOnHit,
	EventTypeAfterHit:           EventAfterHit,
	EventTypeBeforeDamageRoll:   EventBeforeDamageRoll,
	EventTypeOnDamageRoll:       EventOnDamageRoll,
	EventTypeAfterDamageRoll:    EventAfterDamageRoll,
	EventTypeBeforeTakeDamage:   EventBeforeTakeDamage,
	EventTypeOnTakeDamage:       EventOnTakeDamage,
	EventTypeAfterTakeDamage:    EventAfterTakeDamage,
	EventTypeBeforeSavingThrow:  EventBeforeSavingThrow,
	EventTypeOnSavingThrow:      EventOnSavingThrow,
	EventTypeAfterSavingThrow:   EventAfterSavingThrow,
	EventTypeBeforeAbilityCheck: EventBeforeAbilityCheck,
	EventTypeOnAbilityCheck:     EventOnAbilityCheck,
	EventTypeAfterAbilityCheck:  EventAfterAbilityCheck,
	EventTypeOnTurnStart:        EventOnTurnStart,
	EventTypeOnTurnEnd:          EventOnTurnEnd,
	EventTypeOnStatusApplied:    EventOnStatusApplied,
	EventTypeOnStatusRemoved:    EventOnStatusRemoved,
	EventTypeOnShortRest:        EventOnShortRest,
	EventTypeOnLongRest:         EventOnLongRest,
	EventTypeOnSpellCast:        EventOnSpellCast,
	EventTypeOnSpellDamage:      EventOnSpellDamage,
	EventTypeOnConditionApplied: EventOnConditionApplied,
	EventTypeOnConditionRemoved: EventOnConditionRemoved,
}

// stringToEventType maps string to EventType for parsing.
var stringToEventType = map[string]EventType{
	EventBeforeAttackRoll:   EventTypeBeforeAttackRoll,
	EventOnAttackRoll:       EventTypeOnAttackRoll,
	EventAfterAttackRoll:    EventTypeAfterAttackRoll,
	EventBeforeHit:          EventTypeBeforeHit,
	EventOnHit:              EventTypeOnHit,
	EventAfterHit:           EventTypeAfterHit,
	EventBeforeDamageRoll:   EventTypeBeforeDamageRoll,
	EventOnDamageRoll:       EventTypeOnDamageRoll,
	EventAfterDamageRoll:    EventTypeAfterDamageRoll,
	EventBeforeTakeDamage:   EventTypeBeforeTakeDamage,
	EventOnTakeDamage:       EventTypeOnTakeDamage,
	EventAfterTakeDamage:    EventTypeAfterTakeDamage,
	EventBeforeSavingThrow:  EventTypeBeforeSavingThrow,
	EventOnSavingThrow:      EventTypeOnSavingThrow,
	EventAfterSavingThrow:   EventTypeAfterSavingThrow,
	EventBeforeAbilityCheck: EventTypeBeforeAbilityCheck,
	EventOnAbilityCheck:     EventTypeOnAbilityCheck,
	EventAfterAbilityCheck:  EventTypeAfterAbilityCheck,
	EventOnTurnStart:        EventTypeOnTurnStart,
	EventOnTurnEnd:          EventTypeOnTurnEnd,
	EventOnStatusApplied:    EventTypeOnStatusApplied,
	EventOnStatusRemoved:    EventTypeOnStatusRemoved,
	EventOnShortRest:        EventTypeOnShortRest,
	EventOnLongRest:         EventTypeOnLongRest,
	EventOnSpellCast:        EventTypeOnSpellCast,
	EventOnSpellDamage:      EventTypeOnSpellDamage,
	EventOnConditionApplied: EventTypeOnConditionApplied,
	EventOnConditionRemoved: EventTypeOnConditionRemoved,
}

// String returns the string representation of the event type.
func (t EventType) String() string {
	if s, ok := eventTypeToString[t]; ok {
		return s
	}
	return "custom_event"
}

// ParseEventType converts a string to EventType.
func ParseEventType(s string) EventType {
	if t, ok := stringToEventType[s]; ok {
		return t
	}
	return EventTypeCustom
}

// Event represents something that happened in the game.
type Event interface {
	// Type returns the event type (e.g., "attack_roll", "damage_calculation").
	Type() string

	// TypedType returns the type-safe event type.
	TypedType() EventType

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

	// GetInt retrieves an int value by key.
	GetInt(key string) (int, bool)

	// GetString retrieves a string value by key.
	GetString(key string) (string, bool)

	// GetBool retrieves a bool value by key.
	GetBool(key string) (bool, bool)

	// GetFloat64 retrieves a float64 value by key.
	GetFloat64(key string) (float64, bool)

	// GetEntity retrieves an Entity value by key.
	GetEntity(key string) (core.Entity, bool)

	// GetDuration retrieves a Duration value by key.
	GetDuration(key string) (Duration, bool)

	// AddModifier adds a modifier that can affect calculations.
	AddModifier(modifier Modifier)

	// Modifiers returns all modifiers added to this context.
	Modifiers() []Modifier
}

// ModifierSource provides rich information about what created a modifier.
type ModifierSource struct {
	Type        string      // "spell", "condition", "feature", etc.
	Name        string      // "Bless", "Rage", etc.
	Description string      // Human-readable description
	Entity      core.Entity // Who/what created it
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

	// Condition checks if this modifier should apply to the given event.
	// If nil, the modifier always applies.
	Condition(event Event) bool

	// Duration returns how long this modifier lasts.
	// If nil, the modifier is permanent.
	Duration() Duration

	// SourceDetails returns rich source information.
	SourceDetails() *ModifierSource
}

// GameEvent is the standard implementation of Event.
type GameEvent struct {
	eventType      string
	eventTypeTyped EventType
	source         core.Entity
	target         core.Entity
	timestamp      time.Time
	context        Context
	cancelled      bool
}

// NewGameEvent creates a new game event with string type.
func NewGameEvent(eventType string, source, target core.Entity) *GameEvent {
	return &GameEvent{
		eventType:      eventType,
		eventTypeTyped: ParseEventType(eventType),
		source:         source,
		target:         target,
		timestamp:      time.Now(),
		context:        NewEventContext(),
	}
}

// NewTypedGameEvent creates a new game event with typed event type.
func NewTypedGameEvent(eventType EventType, source, target core.Entity) *GameEvent {
	return &GameEvent{
		eventType:      eventType.String(),
		eventTypeTyped: eventType,
		source:         source,
		target:         target,
		timestamp:      time.Now(),
		context:        NewEventContext(),
	}
}

// Type returns the event type.
func (e *GameEvent) Type() string { return e.eventType }

// TypedType returns the type-safe event type.
func (e *GameEvent) TypedType() EventType { return e.eventTypeTyped }

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

// WithSource sets the event source (builder pattern).
func (e *GameEvent) WithSource(source core.Entity) *GameEvent {
	e.source = source
	return e
}

// WithTarget sets the event target (builder pattern).
func (e *GameEvent) WithTarget(target core.Entity) *GameEvent {
	e.target = target
	return e
}

// WithContext sets a context value (builder pattern).
func (e *GameEvent) WithContext(key string, value interface{}) *GameEvent {
	e.context.Set(key, value)
	return e
}

// WithModifier adds a modifier to the event (builder pattern).
func (e *GameEvent) WithModifier(modifier Modifier) *GameEvent {
	e.context.AddModifier(modifier)
	return e
}

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

// GetInt retrieves an int value by key.
func (c *EventContext) GetInt(key string) (int, bool) {
	val, exists := c.data[key]
	if !exists {
		return 0, false
	}
	intVal, ok := val.(int)
	return intVal, ok
}

// GetString retrieves a string value by key.
func (c *EventContext) GetString(key string) (string, bool) {
	val, exists := c.data[key]
	if !exists {
		return "", false
	}
	strVal, ok := val.(string)
	return strVal, ok
}

// GetBool retrieves a bool value by key.
func (c *EventContext) GetBool(key string) (bool, bool) {
	val, exists := c.data[key]
	if !exists {
		return false, false
	}
	boolVal, ok := val.(bool)
	return boolVal, ok
}

// GetFloat64 retrieves a float64 value by key.
func (c *EventContext) GetFloat64(key string) (float64, bool) {
	val, exists := c.data[key]
	if !exists {
		return 0, false
	}
	floatVal, ok := val.(float64)
	return floatVal, ok
}

// GetEntity retrieves an Entity value by key.
func (c *EventContext) GetEntity(key string) (core.Entity, bool) {
	val, exists := c.data[key]
	if !exists {
		return nil, false
	}
	entityVal, ok := val.(core.Entity)
	return entityVal, ok
}

// GetDuration retrieves a Duration value by key.
func (c *EventContext) GetDuration(key string) (Duration, bool) {
	val, exists := c.data[key]
	if !exists {
		return nil, false
	}
	durVal, ok := val.(Duration)
	return durVal, ok
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
	source        string
	modType       string
	modValue      ModifierValue
	priority      int
	condition     func(Event) bool
	duration      Duration
	sourceDetails *ModifierSource
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

// ModifierConfig provides configuration for creating modifiers.
type ModifierConfig struct {
	Source        string
	Type          string
	Value         ModifierValue
	Priority      int
	Condition     func(Event) bool
	Duration      Duration
	SourceDetails *ModifierSource
}

// NewModifierWithConfig creates a new modifier with full configuration.
func NewModifierWithConfig(cfg ModifierConfig) *BasicModifier {
	return &BasicModifier{
		source:        cfg.Source,
		modType:       cfg.Type,
		modValue:      cfg.Value,
		priority:      cfg.Priority,
		condition:     cfg.Condition,
		duration:      cfg.Duration,
		sourceDetails: cfg.SourceDetails,
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

// Condition checks if this modifier should apply.
func (m *BasicModifier) Condition(event Event) bool {
	if m.condition == nil {
		return true // Always applies if no condition
	}
	return m.condition(event)
}

// Duration returns how long this modifier lasts.
func (m *BasicModifier) Duration() Duration {
	return m.duration
}

// SourceDetails returns rich source information.
func (m *BasicModifier) SourceDetails() *ModifierSource {
	return m.sourceDetails
}

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
