// Package resources provides infrastructure for managing consumable and
// regenerating resources without defining what those resources represent.
//
// Purpose:
// This package handles resource pools that can be consumed, restored, and
// regenerated over time, supporting game mechanics like health, mana,
// stamina, or any other depleting/restoring values.
//
// Scope:
//   - Resource pool management with current/maximum values
//   - Consumption and restoration operations
//   - Regeneration over time with configurable rates
//   - Resource modification triggers and events
//   - Overflow and underflow handling
//   - Resource dependencies and relationships
//   - Temporary maximum adjustments
//
// Non-Goals:
//   - Specific resource types: HP, MP, stamina are game-specific
//   - Resource costs: What actions cost resources is game logic
//   - Recovery rules: When/how resources regenerate is game-specific
//   - Resource UI: Bars and displays are presentation layer
//   - Balance values: Resource amounts and rates are game design
//   - Death/exhaustion: What happens at zero is game-specific
//
// Integration:
// This package integrates with:
//   - events: Publishes resource change events
//   - effects: Effects may consume or restore resources
//   - conditions: Conditions may affect resource regeneration
//
// Games define what resources mean and how they're used, while this
// package provides the numerical tracking infrastructure.
//
// Example:
//
//	// Create a resource pool
//	health := resources.NewPool(resources.PoolConfig{
//	    Name:     "health",
//	    Current:  50,
//	    Maximum:  50,
//	    Minimum:  0,
//	    RegenRate: 1, // per interval
//	})
//
//	// Consume resources
//	err := health.Consume(10) // Take 10 damage
//	if err == resources.ErrInsufficientResources {
//	    // Not enough health
//	}
//
//	// Restore resources
//	restored := health.Restore(20) // Heal 20 HP
//	// restored = actual amount restored (capped by maximum)
//
//	// Check if depleted
//	if health.IsDepleted() {
//	    // Game-specific death/unconscious logic
//	}
//
//	// Regeneration (called by game loop)
//	health.Regenerate(deltaTime)
//
//	// Listen for changes
//	bus.Subscribe("resource.consumed", func(e events.Event) {
//	    data := e.Data.(ResourceEventData)
//	    fmt.Printf("%s consumed %d %s\n",
//	        data.EntityID, data.Amount, data.ResourceName)
//	})
package resources
