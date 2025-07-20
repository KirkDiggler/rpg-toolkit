// Package features provides infrastructure for managing entity capabilities
// and traits without implementing specific features or their effects.
//
// Purpose:
// This package establishes a framework for entities to have features
// (abilities, traits, or capabilities) that can be activated, passive,
// or triggered, while remaining agnostic to what these features do.
//
// Scope:
//   - Feature interface and lifecycle
//   - Feature registry for available features
//   - Feature holder for entities with features
//   - Event-driven feature triggers
//   - Feature activation and cooldowns
//   - Feature requirements and prerequisites
//   - Feature categories and tags
//
// Non-Goals:
//   - Specific features: Class abilities, racial traits are game-specific
//   - Feature effects: What features do is game logic
//   - Feature acquisition: How features are gained is game rules
//   - Balance: Feature power and limitations are game design
//   - Feature trees: Progression systems are game-specific
//   - UI: How features are displayed is presentation
//
// Integration:
// This package integrates with:
//   - events: Features can listen for and emit events
//   - resources: Features may consume resources when used
//   - conditions: Features may require or apply conditions
//   - effects: Features often create effects
//
// Games implement concrete features while this package provides
// the infrastructure for managing and triggering them.
//
// Example:
//
//	// Game defines a feature
//	type SecondWindFeature struct {
//	    usesRemaining int
//	    cooldown      time.Duration
//	}
//
//	func (f *SecondWindFeature) ID() string { return "second_wind" }
//	func (f *SecondWindFeature) Name() string { return "Second Wind" }
//	func (f *SecondWindFeature) CanActivate(holder FeatureHolder) bool {
//	    return f.usesRemaining > 0 && !f.onCooldown()
//	}
//
//	func (f *SecondWindFeature) Activate(holder FeatureHolder) error {
//	    // Game-specific healing logic
//	    holder.RestoreHealth(rollHitDie() + level)
//	    f.usesRemaining--
//	    f.startCooldown()
//	    return nil
//	}
//
//	// Use feature system
//	registry := features.NewRegistry()
//	registry.Register(&SecondWindFeature{})
//
//	holder := features.NewHolder()
//	holder.AddFeature("second_wind")
//
//	// Activate when needed
//	if holder.CanActivate("second_wind") {
//	    err := holder.Activate("second_wind")
//	}
//
//	// Listen for feature events
//	holder.OnEvent("turn.start", func(e events.Event) {
//	    // Reset per-turn features
//	})
package features
