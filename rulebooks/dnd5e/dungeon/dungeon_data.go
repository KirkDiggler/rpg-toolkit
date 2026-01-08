package dungeon

import (
	"time"

	"github.com/KirkDiggler/rpg-toolkit/tools/environments"
)

// DungeonData is the persistence format for a complete dungeon.
// It composes EnvironmentData for spatial structure and adds game-specific state.
type DungeonData struct {
	// Environment contains zones, passages, entities, walls
	Environment environments.EnvironmentData `json:"environment"`

	// D&D 5e specific
	StartRoomID string `json:"start_room_id"`
	BossRoomID  string `json:"boss_room_id"`
	Seed        int64  `json:"seed"` // For reproducible generation

	// Room details indexed by zone ID
	Rooms map[string]RoomData `json:"rooms"`

	// Exploration state
	State         DungeonState    `json:"state"`
	CurrentRoomID string          `json:"current_room_id"`
	RevealedRooms map[string]bool `json:"revealed_rooms"`
	OpenDoors     map[string]bool `json:"open_doors"`

	// Metrics
	RoomsCleared   int `json:"rooms_cleared"`
	MonstersKilled int `json:"monsters_killed"`

	// Timestamps
	CreatedAt   time.Time  `json:"created_at"`
	CompletedAt *time.Time `json:"completed_at,omitempty"`
}

// RoomData contains D&D 5e-specific room content.
// Links to a zone in EnvironmentData via matching ID.
type RoomData struct {
	// Type categorizes the room (entrance, chamber, boss, etc.)
	Type RoomType `json:"type"`

	// Encounter defines monsters in this room (nil if cleared/empty)
	Encounter *EncounterData `json:"encounter,omitempty"`

	// Features contains obstacles and terrain
	Features FeatureData `json:"features"`

	// SpawnZones defines where players/monsters can spawn
	SpawnZones []SpawnZoneData `json:"spawn_zones,omitempty"`
}

// EncounterData defines the monster composition for a room.
type EncounterData struct {
	// Monsters contains all monster placements
	Monsters []MonsterPlacementData `json:"monsters"`
	// TotalCR is the sum of all monster CRs
	TotalCR float64 `json:"total_cr"`
}

// MonsterPlacementData represents a monster spawn point.
// The actual position is stored in EnvironmentData.Entities, linked by ID.
type MonsterPlacementData struct {
	// ID is the unique identifier for this placement
	ID string `json:"id"`
	// MonsterID references the rulebook monster type
	MonsterID string `json:"monster_id"`
	// Role defines tactical purpose (melee, ranged, support, boss)
	Role MonsterRole `json:"role"`
	// CR is the challenge rating
	CR float64 `json:"cr"`
}

// FeatureData contains room obstacles and terrain.
type FeatureData struct {
	// Obstacles are blocking features like pillars
	Obstacles []ObstacleData `json:"obstacles,omitempty"`
	// Terrain contains special terrain patches
	Terrain []TerrainPatchData `json:"terrain,omitempty"`
}

// ObstacleData represents a blocking feature in a room.
// The actual position is stored in EnvironmentData.Entities, linked by ID.
type ObstacleData struct {
	// ID is the unique identifier
	ID string `json:"id"`
	// Type specifies the kind of obstacle
	Type ObstacleType `json:"type"`
	// BlocksMovement indicates if entities can pass through
	BlocksMovement bool `json:"blocks_movement"`
	// BlocksLineOfSight indicates if vision is blocked
	BlocksLineOfSight bool `json:"blocks_los"`
}

// TerrainPatchData represents a special terrain area.
type TerrainPatchData struct {
	// ID is the unique identifier
	ID string `json:"id"`
	// Type specifies the terrain kind
	Type TerrainType `json:"type"`
	// MovementCost is the multiplier for movement (1.0 = normal, 2.0 = difficult)
	MovementCost float64 `json:"movement_cost"`
}

// SpawnZoneData defines a designated area for spawning.
type SpawnZoneData struct {
	// ID is the unique identifier
	ID string `json:"id"`
	// Type specifies the zone purpose
	Type ZoneType `json:"type"`
	// Capacity is the maximum entities that can spawn
	Capacity int `json:"capacity"`
}

// ConnectionData adds D&D-specific metadata to a passage.
// The actual passage is in EnvironmentData.Passages, linked by ID.
type ConnectionData struct {
	// ID matches the passage ID in EnvironmentData
	ID string `json:"id"`
	// Type specifies the connection kind (door, stairs, passage)
	Type ConnectionType `json:"type"`
	// Direction is the cardinal direction from the source room
	Direction Direction `json:"direction"`
	// IsMainPath indicates if this is on the critical path
	IsMainPath bool `json:"is_main_path"`
	// PhysicalHint describes the connection for players (e.g., "heavy stone door")
	PhysicalHint string `json:"physical_hint,omitempty"`
}
