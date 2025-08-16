// Copyright (C) 2024 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

// Package events provides type definitions for event-related constants.
// These types are used by rulebooks to define event types, modifier types,
// modifier sources, and priorities for the event-driven architecture.
package events

// EventType identifies a specific type of event.
// Rulebooks should define constants of this type rather than using strings.
// Example: const AttackRoll EventType = "combat.attack.roll"
type EventType string

// ModifierType identifies what kind of modifier is being applied.
// Example: const ModifierDamageBonus ModifierType = "damage_bonus"
type ModifierType string

// ModifierSource identifies where a modifier comes from.
// This helps with stacking rules and debugging.
// Example: const SourceRage ModifierSource = "rage"
type ModifierSource string

// Priority represents the order in which handlers or modifiers are applied.
// Lower values are processed first.
type Priority int

// Common priority levels that can be used as baselines
const (
	PriorityVeryHigh Priority = 10
	PriorityHigh     Priority = 25
	PriorityNormal   Priority = 50
	PriorityLow      Priority = 75
	PriorityVeryLow  Priority = 90
)
