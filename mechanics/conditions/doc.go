// Package conditions provides infrastructure for managing entity states
// and status effects without defining specific condition implementations.
//
// Purpose:
// This package establishes the framework for tracking, applying, and
// managing conditions (status effects) on entities while remaining
// agnostic to what those conditions do or how they affect gameplay.
//
// Scope:
//   - Condition interface and lifecycle management
//   - Condition relationships (suppresses, prevents, removes)
//   - Duration tracking and expiration
//   - Condition stacking and intensity levels
//   - Event integration for condition changes
//   - Manager for applying/removing conditions
//   - Builder pattern for complex conditions
//
// Non-Goals:
//   - Specific condition effects: Paralyzed, poisoned, etc. are game-specific
//   - Mechanical impacts: How conditions affect stats/actions is game logic
//   - Save mechanics: Resisting conditions is game-specific
//   - Condition sources: Who can apply what is game rules
//   - Visual effects: How conditions look is presentation layer
//   - Condition prerequisites: Requirements to apply are game-specific
//
// Integration:
// This package integrates with:
//   - events: Publishes condition applied/removed/expired events
//   - effects: Conditions may have associated effects
//   - behavior: AI may check for conditions affecting decisions
//
// Games define their own conditions and rules for how they work,
// while this package provides the management infrastructure.
//
// Example:
//
//	// Game defines a condition
//	type PoisonedCondition struct {
//	    intensity int
//	    duration  time.Duration
//	}
//
//	func (p *PoisonedCondition) ID() string { return "poisoned" }
//	func (p *PoisonedCondition) Suppresses() []string { return nil }
//	func (p *PoisonedCondition) Prevents() []string { return []string{"regeneration"} }
//
//	// Use the condition manager
//	manager := conditions.NewManager()
//	manager.Apply(entity, &PoisonedCondition{
//	    intensity: 2,
//	    duration:  time.Minute,
//	})
//
//	// Check conditions
//	if manager.HasCondition(entity, "poisoned") {
//	    // Game-specific logic for poisoned entities
//	}
//
//	// Listen for condition events
//	bus.Subscribe("condition.applied", func(e events.Event) {
//	    // React to new conditions
//	})
package conditions
