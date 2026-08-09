// Copyright (C) 2024 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

// Package core provides the fundamental interfaces and types for the RPG toolkit.
package core

// EntityID is the shared identity value for game entities across toolkit
// modules. Leaf modules (play/*) key members, subjects, and audiences by
// EntityID so composition layers never pay a conversion tax between
// module-local ID newtypes. Entity.GetID() remains string for
// compatibility; EntityID(entity.GetID()) bridges.
type EntityID string

// EntityType identifies the category of an entity.
// Rulebooks and modules define constants of this type for their entities.
type EntityType string

// Entity represents a fundamental game object in the RPG system.
// All game entities (characters, items, locations, etc.) must implement this interface.
type Entity interface {
	// GetID returns the unique identifier for this entity.
	// The ID should be unique within the entity's type scope.
	GetID() string

	// GetType returns the type of this entity.
	// This helps categorize entities (e.g., EntityTypeCharacter, EntityTypeItem).
	// Returns an EntityType constant, not a raw string.
	GetType() EntityType
}
