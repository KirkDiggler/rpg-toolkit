// Copyright (C) 2024 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package game

import "github.com/KirkDiggler/rpg-toolkit/events"

// Context provides a consistent pattern for loading game entities from data.
// It combines the entity's data with the game infrastructure needed for runtime operations.
//
// The generic type T represents the data structure for the specific entity being loaded.
// For example: Context[RoomData], Context[CharacterData], etc.
//
// This pattern ensures:
//   - Consistent loading signatures across all entity types
//   - Self-contained data (T has everything needed to reconstruct the entity)
//   - Access to game infrastructure (event bus, future systems)
//   - Clean separation between data and behavior
type Context[T any] struct {
	// EventBus provides event-driven communication between game systems.
	// This allows entities to participate in the game's event ecosystem.
	EventBus events.EventBus

	// Data contains all information needed to reconstruct the entity.
	// This should be self-contained with no external dependencies.
	Data T

	// Future infrastructure can be added here as needed:
	// Registry EntityRegistry  // For complex entity lookups
	// Logger   Logger          // For debugging
	// Metrics  MetricsCollector // For performance tracking
}

// NewContext creates a new Context with the provided infrastructure and data.
func NewContext[T any](eventBus events.EventBus, data T) Context[T] {
	return Context[T]{
		EventBus: eventBus,
		Data:     data,
	}
}

// WithEventBus returns a new Context with the specified event bus.
func (c Context[T]) WithEventBus(eventBus events.EventBus) Context[T] {
	c.EventBus = eventBus
	return c
}

// WithData returns a new Context with the specified data.
func (c Context[T]) WithData(data T) Context[T] {
	c.Data = data
	return c
}
