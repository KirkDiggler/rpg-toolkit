// Copyright (C) 2024 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package game

import (
	"errors"
	"reflect"

	"github.com/KirkDiggler/rpg-toolkit/events"
)

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
//
// Context is immutable after creation to guarantee validity.
type Context[T any] struct {
	// eventBus provides event-driven communication between game systems.
	// This allows entities to participate in the game's event ecosystem.
	eventBus events.EventBus

	// data contains all information needed to reconstruct the entity.
	// This should be self-contained with no external dependencies.
	data T

	// Future infrastructure can be added here as needed:
	// registry EntityRegistry  // For complex entity lookups
	// logger   Logger          // For debugging
	// metrics  MetricsCollector // For performance tracking
}

// NewContext creates a new Context with the provided infrastructure and data.
// Both eventBus and data are required - returns error if either is nil/zero.
func NewContext[T any](eventBus events.EventBus, data T) (Context[T], error) {
	if eventBus == nil {
		return Context[T]{}, errors.New("eventBus is required")
	}

	// Check if data is zero value using reflection
	// This ensures we're not creating contexts with empty data
	if reflect.ValueOf(data).IsZero() {
		return Context[T]{}, errors.New("data cannot be zero value")
	}

	return Context[T]{
		eventBus: eventBus,
		data:     data,
	}, nil
}

// EventBus returns the event bus for this context.
func (c Context[T]) EventBus() events.EventBus {
	return c.eventBus
}

// Data returns the entity data for this context.
func (c Context[T]) Data() T {
	return c.data
}
