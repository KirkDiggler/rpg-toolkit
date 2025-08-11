// Copyright (C) 2024 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

// Package conditions provides the infrastructure for status effects and conditions
// that can be applied to entities in the RPG toolkit.
package conditions

import (
	"github.com/KirkDiggler/rpg-toolkit/core"
	"github.com/KirkDiggler/rpg-toolkit/events"
)

// Condition represents a status effect that affects an entity.
// Conditions are entities themselves, allowing them to be persisted,
// queried, and managed like other game objects.
//
//go:generate mockgen -destination=mock/mock_condition.go -package=mock github.com/KirkDiggler/rpg-toolkit/mechanics/conditions Condition
type Condition interface {
	core.Entity // Conditions are entities with ID and Type

	// Target returns the entity this condition affects.
	Target() core.Entity

	// Source returns what created this condition (spell name, item, etc).
	Source() *core.Source

	// IsActive returns whether the condition is currently active.
	IsActive() bool

	// Apply registers the condition's effects with the event system.
	// Called when the condition is first added to an entity.
	Apply(bus events.EventBus) error

	// Remove unregisters the condition's effects from the event system.
	// Called when the condition is removed from an entity.
	Remove(bus events.EventBus) error
}
