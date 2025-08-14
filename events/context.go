// Copyright (C) 2024 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package events

import "sync"

// TypedKey provides type-safe context keys for event data.
// Rulebooks and features can create their own typed keys.
type TypedKey[T any] struct {
	name string
}

// NewTypedKey creates a new typed key for context values.
// The name should be unique to avoid collisions.
func NewTypedKey[T any](name string) *TypedKey[T] {
	return &TypedKey[T]{name: name}
}

// String returns the key name for debugging
func (k *TypedKey[T]) String() string {
	return k.name
}

// EventContext holds modifiers and typed data for events.
// It provides thread-safe access to both modifiers and context data.
type EventContext struct {
	modifiers []Modifier
	data      map[string]any
	mu        sync.RWMutex
}

// NewEventContext creates a new event context
func NewEventContext() *EventContext {
	return &EventContext{
		data:      make(map[string]any),
		modifiers: []Modifier{},
	}
}

// Set stores a typed value in the context
func Set[T any](ctx *EventContext, key *TypedKey[T], value T) {
	ctx.mu.Lock()
	defer ctx.mu.Unlock()
	if ctx.data == nil {
		ctx.data = make(map[string]any)
	}
	ctx.data[key.name] = value
}

// Get retrieves a typed value from the context
func Get[T any](ctx *EventContext, key *TypedKey[T]) (T, bool) {
	ctx.mu.RLock()
	defer ctx.mu.RUnlock()

	var zero T
	if val, ok := ctx.data[key.name]; ok {
		if typed, ok := val.(T); ok {
			return typed, true
		}
	}
	return zero, false
}

// AddModifier adds a modifier to the event
func (c *EventContext) AddModifier(m Modifier) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.modifiers = append(c.modifiers, m)
}

// GetModifiers returns a copy of all modifiers
func (c *EventContext) GetModifiers() []Modifier {
	c.mu.RLock()
	defer c.mu.RUnlock()

	// Return a copy to prevent external mutations
	result := make([]Modifier, len(c.modifiers))
	copy(result, c.modifiers)
	return result
}

// ClearModifiers removes all modifiers (useful for tests)
func (c *EventContext) ClearModifiers() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.modifiers = []Modifier{}
}

// HasKey checks if a key exists in the context
func HasKey[T any](ctx *EventContext, key *TypedKey[T]) bool {
	ctx.mu.RLock()
	defer ctx.mu.RUnlock()
	_, exists := ctx.data[key.name]
	return exists
}

// Delete removes a key from the context
func Delete[T any](ctx *EventContext, key *TypedKey[T]) {
	ctx.mu.Lock()
	defer ctx.mu.Unlock()
	delete(ctx.data, key.name)
}
