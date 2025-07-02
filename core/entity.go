// Package core provides the fundamental interfaces and types for the RPG toolkit.
package core

// Entity represents a fundamental game object in the RPG system.
// All game entities (characters, items, locations, etc.) must implement this interface.
type Entity interface {
	// GetID returns the unique identifier for this entity.
	// The ID should be unique within the entity's type scope.
	GetID() string

	// GetType returns the type of this entity.
	// This helps categorize entities (e.g., "character", "item", "location").
	GetType() string
}