// Copyright (C) 2024 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package effects

import (
	"context"
	"time"

	"github.com/KirkDiggler/rpg-toolkit/core"
	"github.com/KirkDiggler/rpg-toolkit/dice"
	"github.com/KirkDiggler/rpg-toolkit/events"
)

// ConditionalEffect represents an effect that only applies under certain conditions.
// Examples: Weapon proficiency (only with that weapon), Sneak Attack (only with advantage)
type ConditionalEffect interface {
	// CheckCondition returns true if the effect should apply given the context
	CheckCondition(ctx context.Context, event events.Event) bool
}

// ResourceConsumer represents an effect that consumes limited resources.
// Examples: Rage (uses rage charges), Divine Smite (uses spell slots)
type ResourceConsumer interface {
	// GetResourceRequirements returns the resources needed to activate
	GetResourceRequirements() []ResourceRequirement

	// ConsumeResources attempts to consume the required resources
	// Returns error if resources are not available
	ConsumeResources(ctx context.Context, bus events.EventBus) error
}

// ResourceRequirement defines a resource needed by an effect
type ResourceRequirement struct {
	Key      string // Resource key (e.g., "rage_uses", "spell_slots_1")
	Amount   int    // Amount needed
	Optional bool   // If true, effect can still apply without this resource
}

// TemporaryEffect represents an effect with a limited duration.
// Examples: Bless (10 rounds), Rage (1 minute), Poison (until cured)
type TemporaryEffect interface {
	// GetDuration returns when the effect expires
	GetDuration() Duration

	// CheckExpiration returns true if the effect has expired
	CheckExpiration(ctx context.Context, currentTime time.Time) bool

	// OnExpire is called when the effect expires
	OnExpire(bus events.EventBus) error
}

// Duration represents how long an effect lasts
type Duration struct {
	Type  DurationType
	Value int    // Rounds, minutes, etc. based on Type
	Until string // For "until" conditions like "until_rest" or "until_dispelled"
}

// DurationType defines how duration is measured
type DurationType string

// Duration type constants
const (
	DurationInstant    DurationType = "instant"    // Happens immediately
	DurationRounds     DurationType = "rounds"     // Combat rounds
	DurationMinutes    DurationType = "minutes"    // Real-time minutes
	DurationHours      DurationType = "hours"      // Real-time hours
	DurationDays       DurationType = "days"       // Game days
	DurationUntil      DurationType = "until"      // Until a condition
	DurationMaintained DurationType = "maintained" // While actively maintained
	DurationPermanent  DurationType = "permanent"  // Never expires
)

// StackableEffect represents an effect that can stack with itself.
// Examples: Ability score damage, temporary hit points (special stacking)
type StackableEffect interface {
	// GetStackingRule returns how this effect stacks
	GetStackingRule() StackingRule

	// CanStackWith returns true if this can stack with another effect
	CanStackWith(other core.Entity) bool

	// Stack combines this effect with another, returning the result
	Stack(other core.Entity) error
}

// StackingRule defines how effects combine
type StackingRule string

// Stacking rule constants
const (
	StackingNone     StackingRule = "none"     // Cannot stack (most buffs)
	StackingAdd      StackingRule = "add"      // Values add together
	StackingMax      StackingRule = "max"      // Take highest value
	StackingDuration StackingRule = "duration" // Extend duration
	StackingCustom   StackingRule = "custom"   // Custom stacking logic
)

// DiceModifier represents an effect that adds dice to rolls.
// IMPORTANT: The dice package caches roll results. To ensure fresh rolls,
// create new dice.Roll instances each time a modifier is needed.
// Examples: Bless (+1d4 to attacks), Bane (-1d4 to saves)
type DiceModifier interface {
	// GetDiceExpression returns the dice to add (e.g., "1d4", "2d6")
	GetDiceExpression(ctx context.Context, event events.Event) string

	// GetModifierType returns when this modifier applies
	GetModifierType() ModifierType

	// ShouldApply returns true if the modifier should apply to this event
	ShouldApply(ctx context.Context, event events.Event) bool
}

// ModifierType indicates what kind of roll a modifier affects
type ModifierType string

// Modifier type constants
const (
	ModifierAttack       ModifierType = "attack"        // Attack rolls
	ModifierDamage       ModifierType = "damage"        // Damage rolls
	ModifierSave         ModifierType = "save"          // Saving throws
	ModifierSkill        ModifierType = "skill"         // Skill checks
	ModifierInitiative   ModifierType = "initiative"    // Initiative rolls
	ModifierAbilityCheck ModifierType = "ability_check" // Ability checks
	ModifierAll          ModifierType = "all"           // All rolls
)

// TargetedEffect represents an effect that affects specific entities.
// Examples: Auras, charm effects, curses
type TargetedEffect interface {
	// GetTargets returns the entities affected by this effect
	GetTargets() []core.Entity

	// AddTarget adds a new target to the effect
	AddTarget(target core.Entity) error

	// RemoveTarget removes a target from the effect
	RemoveTarget(target core.Entity) error

	// IsValidTarget checks if an entity can be targeted
	IsValidTarget(target core.Entity) bool
}

// TriggeredEffect represents an effect that responds to specific triggers.
// Examples: Reactions, contingent spells, trap effects
type TriggeredEffect interface {
	// GetTriggers returns the events that trigger this effect
	GetTriggers() []TriggerCondition

	// OnTrigger is called when a trigger condition is met
	OnTrigger(ctx context.Context, event events.Event, bus events.EventBus) error
}

// TriggerCondition defines when a triggered effect activates
type TriggerCondition struct {
	EventType string                                             // Event to listen for
	Condition func(ctx context.Context, event events.Event) bool // Additional conditions
	Priority  int                                                // Handler priority
}

// SavingThrowEffect represents an effect that allows a saving throw.
// Examples: Poison, charm, fear effects
type SavingThrowEffect interface {
	// GetSaveDetails returns information about the required save
	GetSaveDetails() SaveDetails

	// OnSaveSuccess is called when the save succeeds
	OnSaveSuccess(ctx context.Context, bus events.EventBus) error

	// OnSaveFailure is called when the save fails
	OnSaveFailure(ctx context.Context, bus events.EventBus) error
}

// SaveDetails contains information about a required saving throw
type SaveDetails struct {
	Ability     string // Ability used for save (e.g., "wisdom", "constitution")
	DC          int    // Difficulty class
	RepeatType  SaveRepeatType
	RepeatValue string // When repeats occur (e.g., "turn_end", "long_rest")
}

// SaveRepeatType defines when saves can be repeated
type SaveRepeatType string

// Save repeat type constants
const (
	SaveOnce     SaveRepeatType = "once"      // Single save when applied
	SaveRepeat   SaveRepeatType = "repeat"    // Can repeat on intervals
	SaveOnDamage SaveRepeatType = "on_damage" // New save when damaged
)

// ModifierValue represents a value that modifies a roll or stat.
// This integrates with the existing modifier system from ADR-0001.
type ModifierValue interface {
	Calculate(ctx context.Context, roller dice.Roller) int
	Description() string
}

// EffectContext provides context for behavioral checks
type EffectContext struct {
	Source core.Entity
	Target core.Entity
	Event  events.Event
	Time   time.Time
	Custom map[string]interface{}
}
