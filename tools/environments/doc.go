// Package environments provides procedural generation of rooms and areas
// using spatial primitives and environmental features without game logic.
//
// Purpose:
// This package generates physical spaces with rooms, corridors, and features
// while remaining agnostic to game-specific concepts like encounter design
// or narrative purpose. It creates the stage, not the play.
//
// Scope:
//   - Room shape generation (rectangle, L-shape, cross, etc.)
//   - Corridor and connection generation between rooms
//   - Environmental feature placement (walls, pillars, etc.)
//   - Graph-based and BSP layout algorithms
//   - Room sizing based on capacity calculations
//   - Wall patterns (empty, random destructible)
//   - Integration with selectables for random generation
//
// Non-Goals:
//   - Encounter design: What spawns in rooms is game-specific
//   - Trap mechanics: Trap effects and saves are game rules
//   - Secret doors: Detection DCs are game-specific
//   - Narrative generation: Story/quest hooks belong in games
//   - Dungeon ecology: Logical creature placement is game logic
//   - Treasure placement: Value and rarity are game-specific
//   - Environmental hazards: Damage and effects are game rules
//
// Integration:
// This package integrates with:
//   - spatial: Creates rooms compatible with spatial system
//   - selectables: For random room shapes and features
//   - events: Publishes generation events
//
// The environments package generates interesting spaces that games
// can populate with their own content and meaning.
//
// Example:
//
//	gen := environments.NewGraphGenerator(environments.GeneratorConfig{
//	    MinRooms: 5,
//	    MaxRooms: 10,
//	})
//
//	// Generate a dungeon level
//	env := gen.Generate(environments.Parameters{
//	    TargetCapacity: 50,  // Space for ~50 entities total
//	    Connectivity:   0.7, // 70% connected graph
//	    Shapes: []environments.ShapeWeight{
//	        {Shape: "rectangle", Weight: 60},
//	        {Shape: "L", Weight: 30},
//	        {Shape: "cross", Weight: 10},
//	    },
//	})
//
//	// Iterate generated rooms
//	for _, room := range env.GetRooms() {
//	    // Room is ready for spawn system to populate
//	    fmt.Printf("Room %s: %dx%d at %v\n",
//	        room.ID(), room.Width(), room.Height(), room.Position())
//	}
//
//	// Access the spatial orchestrator
//	orchestrator := env.GetOrchestrator()
//	// Use for movement, queries, etc.
package environments
