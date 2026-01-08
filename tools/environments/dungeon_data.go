package environments

import (
	"github.com/KirkDiggler/rpg-toolkit/tools/spatial"
)

// DungeonData contains all information needed to persist and reconstruct a dungeon.
// All coordinates are in absolute space - the game server should never perform
// coordinate conversions. This is the output of dungeon generation.
//
// Purpose: Single source of truth for dungeon state, ready for direct use by
// the game server without transformation.
type DungeonData struct {
	// ID is the unique identifier for this dungeon
	ID string `json:"id"`

	// Seed is the random seed used to generate this dungeon (for reproducibility)
	Seed int64 `json:"seed"`

	// Theme is the visual/atmospheric theme (e.g., "crypt", "cave", "bandit_lair")
	Theme string `json:"theme"`

	// StartRoomID identifies where players begin
	StartRoomID string `json:"start_room_id"`

	// BossRoomID identifies the final encounter room (optional)
	BossRoomID string `json:"boss_room_id,omitempty"`

	// Rooms contains logical room groupings with their absolute origins
	Rooms []DungeonRoomData `json:"rooms"`

	// Connections defines how rooms link together (for pathfinding and door references)
	Connections []DungeonConnectionData `json:"connections"`

	// Entities contains all entities including monsters, obstacles, and doors
	// All positions are in absolute coordinates
	Entities []DungeonEntityData `json:"entities"`

	// Walls contains all wall segments in absolute coordinates
	Walls []DungeonWallData `json:"walls"`

	// RevealedRooms tracks which rooms the players have discovered
	// This is runtime state that gets persisted with the dungeon
	RevealedRooms []string `json:"revealed_rooms,omitempty"`
}

// DungeonRoomData represents a logical room within a dungeon.
// Rooms are boundaries for queries and visibility, not separate coordinate spaces.
// All coordinates within are already absolute.
//
// Purpose: Allows queries like "what room is this hex in?" and controls
// what the client sees (only revealed rooms are sent).
type DungeonRoomData struct {
	// ID is the unique identifier for this room
	ID string `json:"id"`

	// Type categorizes the room (e.g., "combat", "treasure", "boss", "entrance")
	Type string `json:"type"`

	// Origin is the room's position in absolute dungeon coordinates
	// This is where the room's local (0,0,0) maps to in dungeon space
	Origin spatial.CubeCoordinate `json:"origin"`

	// Width is the number of hexes across (X range)
	Width int `json:"width"`

	// Height is the number of hexes down (Z range)
	Height int `json:"height"`

	// EntityIDs lists all entities that belong to this room
	// Useful for room-based queries and visibility filtering
	EntityIDs []string `json:"entity_ids,omitempty"`
}

// DungeonConnectionData represents a link between two rooms.
// Connections are abstract relationships - the physical door entity
// is stored separately in the Entities list.
//
// Purpose: Enables pathfinding between rooms and associates doors with
// the rooms they connect.
type DungeonConnectionData struct {
	// ID is the unique identifier for this connection
	ID string `json:"id"`

	// FromRoomID is the source room
	FromRoomID string `json:"from_room_id"`

	// ToRoomID is the destination room
	ToRoomID string `json:"to_room_id"`

	// DoorEntityID references the door entity at this connection (optional)
	// If empty, the connection is an open passage
	DoorEntityID string `json:"door_entity_id,omitempty"`

	// Bidirectional indicates if movement works both ways (default true)
	Bidirectional bool `json:"bidirectional"`
}

// DungeonEntityType categorizes entities in a dungeon.
// Purpose: Allows type-based queries and determines how the entity is rendered/handled.
type DungeonEntityType string

const (
	// DungeonEntityTypeMonster represents hostile creatures
	DungeonEntityTypeMonster DungeonEntityType = "monster"
	// DungeonEntityTypeObstacle represents blocking terrain (pillars, rubble, etc.)
	DungeonEntityTypeObstacle DungeonEntityType = "obstacle"
	// DungeonEntityTypeDoor represents doors between rooms or areas
	DungeonEntityTypeDoor DungeonEntityType = "door"
	// DungeonEntityTypeCharacter represents player characters
	DungeonEntityTypeCharacter DungeonEntityType = "character"
)

// DungeonEntityData represents any entity placed in the dungeon.
// All positions are in absolute dungeon coordinates.
//
// Purpose: Unified entity representation supporting monsters, obstacles, doors,
// and characters with type-specific data in Properties.
type DungeonEntityData struct {
	// ID is the unique identifier for this entity
	ID string `json:"id"`

	// Type categorizes the entity (monster, obstacle, door, character)
	Type DungeonEntityType `json:"type"`

	// Position is where the entity is located in absolute dungeon coordinates
	Position spatial.CubeCoordinate `json:"position"`

	// Size is how many hexes the entity occupies (1 = single hex)
	Size int `json:"size"`

	// BlocksMovement indicates if other entities cannot move through this space
	BlocksMovement bool `json:"blocks_movement"`

	// BlocksLoS indicates if this entity blocks line of sight
	BlocksLoS bool `json:"blocks_los"`

	// RoomID indicates which room this entity belongs to
	// Used for visibility filtering and room-based queries
	RoomID string `json:"room_id"`

	// VisualType is a subtype for rendering (e.g., "skeleton", "pillar", "wooden_door")
	// The game layer maps this to actual visual assets
	VisualType string `json:"visual_type,omitempty"`

	// Properties contains type-specific data
	// For doors: "open", "locked", "trap_dc", "key_id"
	// For monsters: "cr", "hp", "initiative"
	// For obstacles: "destructible", "hp"
	Properties map[string]any `json:"properties,omitempty"`
}

// DungeonWallData represents a wall segment in the dungeon.
// Walls are defined by their endpoints in absolute coordinates.
//
// Purpose: Enables collision detection and line-of-sight calculations
// without needing to reference room geometry.
type DungeonWallData struct {
	// Start is one endpoint of the wall segment
	Start spatial.CubeCoordinate `json:"start"`

	// End is the other endpoint of the wall segment
	End spatial.CubeCoordinate `json:"end"`

	// BlocksMovement indicates if entities cannot pass through (usually true)
	BlocksMovement bool `json:"blocks_movement"`

	// BlocksLoS indicates if the wall blocks line of sight (usually true)
	BlocksLoS bool `json:"blocks_los"`

	// Destructible indicates if the wall can be destroyed
	Destructible bool `json:"destructible,omitempty"`
}
