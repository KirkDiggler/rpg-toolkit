// Copyright (C) 2024 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

// Package proficiency provides infrastructure for entities to be proficient
// with various skills, tools, weapons, or other game elements.
// Proficiencies modify game mechanics through the event system.
package proficiency

import (
	"github.com/KirkDiggler/rpg-toolkit/core"
	"github.com/KirkDiggler/rpg-toolkit/events"
)

// Proficiency represents something an entity is proficient with.
// Proficiencies are entities themselves, allowing them to be persisted,
// queried, and managed like other game objects.
//
//go:generate mockgen -destination=mock/mock_proficiency.go -package=mock github.com/KirkDiggler/rpg-toolkit/mechanics/proficiency Proficiency
type Proficiency interface {
	core.Entity // Proficiencies are entities with ID and Type

	// Owner returns the entity that has this proficiency.
	Owner() core.Entity

	// Subject returns what the entity is proficient with (e.g., "longsword", "athletics").
	Subject() string

	// Source returns what granted this proficiency (e.g., "fighter-class", "elf-race").
	Source() string

	// IsActive returns whether the proficiency is currently active.
	IsActive() bool

	// Apply registers the proficiency's effects with the event system.
	// Called when the proficiency is first added to an entity.
	Apply(bus events.EventBus) error

	// Remove unregisters the proficiency's effects from the event system.
	// Called when the proficiency is removed from an entity.
	Remove(bus events.EventBus) error
}
