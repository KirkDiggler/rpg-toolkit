// Package spawn provides entity placement and population infrastructure
// for rooms and areas without creating entities or defining what spawns.
//
// Purpose:
// This package handles the placement of pre-created entities within spatial
// constraints, integrating with the selectables system for random selection
// while remaining agnostic to what is being spawned.
//
// Scope:
//   - Entity placement with constraints
//   - Spawn point management
//   - Integration with selectables for random choice
//   - Density calculations and space management
//   - Placement validation and collision avoidance
//   - Progressive constraint relaxation
//
// Non-Goals:
//   - Entity creation: Entities must be pre-created by the game
//   - Spawn tables: Use selectables package directly
//   - Creature stats: This only handles placement, not attributes
//   - Loot generation: Create items before spawning
//   - Encounter balancing: CR/difficulty is game-specific
//   - Spawn triggers: When to spawn is game logic
//
// Integration:
// This package integrates with:
//   - spatial: For placement validation and collision detection
//   - selectables: For random entity selection from pools
//   - core: Uses Entity interface for type-agnostic placement
//
// The spawn package acts as a bridge between game-created entities and
// the spatial system, ensuring valid placement while respecting constraints.
//
// Example:
//
//	engine := spawn.NewEngine(spawn.Config{
//	    CollisionRadius: 2.5, // Units depend on spatial configuration
//	})
//
//	// Game provides pre-created entities
//	entities := map[EntityPoolType][]core.Entity{
//	    EntityPoolTypeMonsters: {goblin1, goblin2, orc1},
//	    EntityPoolTypeTreasure: {goldPile, healingPotion},
//	}
//
//	result, err := engine.PopulateRoom("room-1", spawn.Config{
//	    EntityPools: entities,
//	    Density:     spawn.DensityModerate,
//	    Constraints: []spawn.Constraint{
//	        spawn.AvoidCenter{Radius: 10},
//	        spawn.MinDistanceFromEntry{Distance: 15},
//	        spawn.NearFeature{Feature: "pillar", MaxDistance: 5},
//	    },
//	})
//
//	// Result contains placed entities with their positions
//	for _, placement := range result.Placements {
//	    fmt.Printf("Placed %s at %v\n", placement.Entity.ID(), placement.Position)
//	}
package spawn
