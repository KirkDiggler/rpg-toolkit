// Package core provides fundamental interfaces and types that define entities
// in the RPG toolkit ecosystem without imposing any game-specific attributes.
//
// Purpose:
// This package establishes the base contracts that all game entities must fulfill,
// providing identity and type information without imposing any game-specific
// attributes or behaviors. It is the foundation upon which all other packages build.
//
// Scope:
//   - Entity interface: Basic identity contract (ID, Type)
//   - Error types: Common errors used across packages
//   - No game logic, stats, or behaviors
//   - No persistence or storage concerns
//   - Pure interfaces and contracts
//
// Non-Goals:
//   - Game statistics: HP, AC, attributes belong in game implementations
//   - Entity behaviors: Use the behavior package for AI/actions
//   - Persistence: Storage/serialization belongs in repository implementations
//   - Game rules: All game-specific logic belongs in rulebooks
//   - Entity creation: Factories and builders belong in games
//   - Entity relationships: Parent/child, ownership are game-specific
//
// Integration:
// This package is imported by all other toolkit packages as it defines
// the fundamental Entity contract. It has no dependencies on other
// toolkit packages, maintaining its position at the base of the dependency
// hierarchy. This ensures the toolkit remains loosely coupled.
//
// Example:
//
//	// Game implements the Entity interface
//	type Monster struct {
//	    id   string
//	    kind string
//	    // Game-specific fields like HP, AC, etc.
//	}
//
//	func (m *Monster) ID() string   { return m.id }
//	func (m *Monster) Type() string { return m.kind }
//
//	// The toolkit can work with any Entity
//	var entity core.Entity = &Monster{id: "goblin-1", kind: "goblin"}
//
//	// Spatial system can position it
//	room.PlaceEntity(entity, position)
//
//	// Behavior system can make decisions for it
//	action := behavior.Execute(entity, context)
package core
