// Copyright (C) 2024 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package events

import "context"

// BusEffect represents an effect that interacts with the event bus by
// subscribing to and unsubscribing from events. This is the standard interface
// for any game mechanic that modifies behavior through the event system.
//
// Purpose: Provides a standard lifecycle for event-driven game mechanics.
// Effects can subscribe to events when applied (e.g., when a condition starts,
// a feature activates, or equipment is equipped) and unsubscribe when removed.
//
// Examples:
//   - Conditions that modify attack rolls (subscribe to attack events)
//   - Features that trigger on damage (subscribe to damage events)
//   - Equipment effects that enhance abilities (subscribe to ability check events)
//   - Resources that recover on rest (subscribe to rest events)
//
// Lifecycle:
//  1. Effect is created (not yet applied)
//  2. Apply() is called - subscribes to relevant events, marks as applied
//  3. Events are published - effect's handlers modify game state
//  4. Remove() is called - unsubscribes from events, marks as not applied
type BusEffect interface {
	// Apply subscribes this effect to relevant events on the bus.
	// This is called when the effect becomes active (e.g., condition applied,
	// feature activated, equipment equipped).
	//
	// The effect should:
	//  - Subscribe to any events it needs to listen to
	//  - Store subscription IDs for later cleanup
	//  - Mark itself as applied/active
	//
	// Returns an error if subscription fails.
	Apply(ctx context.Context, bus EventBus) error

	// Remove unsubscribes this effect from all events on the bus.
	// This is called when the effect becomes inactive (e.g., condition removed,
	// feature deactivated, equipment unequipped).
	//
	// The effect should:
	//  - Unsubscribe from all events using stored subscription IDs
	//  - Clean up any internal state
	//  - Mark itself as not applied/inactive
	//
	// Returns an error if unsubscription fails.
	Remove(ctx context.Context, bus EventBus) error

	// IsApplied returns true if this effect is currently subscribed to events.
	// This allows callers to check whether the effect is active without
	// attempting to apply or remove it.
	//
	// An applied effect should be listening to events and modifying behavior.
	// A non-applied effect should not be interacting with the event system.
	IsApplied() bool
}
