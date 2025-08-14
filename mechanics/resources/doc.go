// Package resources provides simple resource tracking for games.
// Resources track current/maximum values, counters track simple counts.
//
// Purpose:
// This package provides basic numerical tracking for game resources and
// counters without defining what those values represent.
//
// Scope:
//   - Resource tracking with current/maximum values
//   - Counter tracking with optional limits
//   - Simple consumption and restoration
//   - Pool management for organizing resources
//
// Non-Goals:
//   - Specific resource types: HP, MP, stamina are game-specific
//   - Recovery rules: When/how resources restore is game-specific
//   - Resource costs: What actions cost resources is game logic
//   - Events or triggers: Use the event bus separately if needed
//   - Complex restoration: Games define their own rest mechanics
//
// Games define what resources mean and how they're used, while this
// package provides the numerical tracking infrastructure.
//
// Example:
//
//	// Create a pool for a character
//	pool := resources.NewPool()
//
//	// Add resources
//	hp := resources.NewResource("hit_points", 45)
//	pool.AddResource(hp)
//	pool.AddResource(resources.NewResource("spell_slots_1", 4))
//	pool.AddResource(resources.NewResource("rage", 3))
//
//	// Add counters
//	pool.AddCounter(resources.NewCounter("death_saves", 3))
//	pool.AddCounter(resources.NewCounter("attacks", 0)) // No limit
//
//	// Use resources
//	err := hp.Use(10) // Take damage
//	if err != nil {
//	    // Not enough HP
//	}
//
//	// Track counts
//	saves, _ := pool.GetCounter("death_saves")
//	saves.Increment() // Death save success
//
//	// Rest operations
//	pool.RestoreAllResources() // Long rest
//	pool.ResetAllCounters()
package resources
