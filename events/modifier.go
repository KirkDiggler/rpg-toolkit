// Copyright (C) 2024 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package events

import "github.com/KirkDiggler/rpg-toolkit/core/events"

// Modifier represents a modification to be applied to an event's outcome.
// Modifiers are added by event handlers and interpreted by the event resolver.
type Modifier interface {
	// Source identifies what added this modifier (e.g., "rage", "bless")
	Source() events.ModifierSource

	// Type describes how this modifier should be applied (e.g., "additive", "multiplicative")
	Type() events.ModifierType

	// Target identifies what is being modified (e.g., "damage", "ac", "attack_roll")
	Target() events.ModifierTarget

	// Priority determines the order of application (lower = earlier)
	Priority() int

	// Value returns the modification data (int, string, bool, etc.)
	Value() any
}

// SimpleModifier is a basic implementation of the Modifier interface.
// This is all you need for most modifiers.
type SimpleModifier struct {
	source   events.ModifierSource
	modType  events.ModifierType
	target   events.ModifierTarget
	priority int
	value    any
}

// NewSimpleModifier creates a new modifier
func NewSimpleModifier(source events.ModifierSource, modType events.ModifierType, target events.ModifierTarget, priority int, value any) *SimpleModifier {
	return &SimpleModifier{
		source:   source,
		modType:  modType,
		target:   target,
		priority: priority,
		value:    value,
	}
}

// Source implements Modifier
func (m *SimpleModifier) Source() events.ModifierSource {
	return m.source
}

// Type implements Modifier
func (m *SimpleModifier) Type() events.ModifierType {
	return m.modType
}

// Target implements Modifier
func (m *SimpleModifier) Target() events.ModifierTarget {
	return m.target
}

// Priority implements Modifier
func (m *SimpleModifier) Priority() int {
	return m.priority
}

// Value implements Modifier
func (m *SimpleModifier) Value() any {
	return m.value
}
