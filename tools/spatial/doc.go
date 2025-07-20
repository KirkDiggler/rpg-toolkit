// Package spatial provides 2D positioning and movement infrastructure for
// entity placement and spatial queries without imposing game-specific rules.
//
// Purpose:
// This package handles all spatial mathematics, collision detection, and
// movement validation without imposing any game-specific movement rules
// or combat mechanics. It provides the mathematical foundation for position-based
// game systems.
//
// Scope:
//   - 2D coordinate system with configurable units
//   - Grid support (square, hex, gridless)
//   - Room-based spatial organization
//   - Collision detection and spatial queries
//   - Path validation (not pathfinding algorithms)
//   - Multi-room orchestration and connections
//   - Distance calculations and area queries
//   - Entity position tracking
//
// Non-Goals:
//   - Movement rules: Speed, difficult terrain are game-specific
//   - Line of sight rules: Cover/concealment mechanics belong in games
//   - Pathfinding algorithms: AI navigation belongs in behavior package
//   - Combat ranges: Weapon/spell ranges are game-specific
//   - 3D positioning: This is explicitly 2D only
//   - Movement costs: Action economy is game-specific
//   - Elevation: Height/flying is game-specific
//
// Integration:
// This package integrates with:
//   - behavior: Provides position queries for AI decisions
//   - spawn: Validates entity placement
//   - environments: Provides room infrastructure
//   - events: Publishes movement and room transition events
//
// The spatial package is the foundation for any position-based mechanics
// but deliberately avoids encoding any game rules about how space is used.
//
// Example:
//
//	// Create a room with square grid
//	room := spatial.NewBasicRoom(spatial.RoomConfig{
//	    ID:     "throne-room",
//	    Width:  40,
//	    Height: 30,
//	    Grid:   spatial.GridTypeSquare,
//	})
//
//	// Place entities
//	err := room.PlaceEntity("guard-1", spatial.Position{X: 10, Y: 5})
//	err = room.PlaceEntity("king", spatial.Position{X: 20, Y: 25})
//
//	// Query nearby entities
//	nearby := room.GetEntitiesWithinDistance(
//	    spatial.Position{X: 15, Y: 15},
//	    10.0, // 10 units radius
//	)
//
//	// Multi-room orchestration
//	orchestrator := spatial.NewBasicOrchestrator(spatial.OrchestratorConfig{})
//	orchestrator.AddRoom(room)
//	orchestrator.AddRoom(hallway)
//
//	// Connect rooms
//	door := spatial.NewDoorConnection("door-1", "throne-room", "hallway",
//	    spatial.Position{X: 40, Y: 15}, // Exit position
//	    spatial.Position{X: 0, Y: 5},   // Entry position
//	)
//	orchestrator.AddConnection(door)
package spatial
