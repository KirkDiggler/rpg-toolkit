package environments

import (
	"github.com/KirkDiggler/rpg-toolkit/tools/spatial"
)

// EnvironmentData contains all information needed to persist and reconstruct
// a multi-zone environment with absolute coordinates.
//
// This is generic infrastructure - game-specific concepts (dungeons, towns, etc.)
// should compose this structure and add their own metadata.
//
// All coordinates are absolute - consumers should never perform coordinate conversions.
type EnvironmentData struct {
	// ID is the unique identifier for this environment
	ID string `json:"id"`

	// Seed is the random seed used to generate this environment (for reproducibility)
	Seed int64 `json:"seed"`

	// Zones contains logical zone groupings with their absolute origins
	Zones []ZoneData `json:"zones"`

	// Passages defines how zones connect (for pathfinding)
	Passages []PassageData `json:"passages"`

	// Entities contains all placed entities with absolute coordinates
	Entities []PlacedEntityData `json:"entities"`

	// Walls contains all wall segments in absolute coordinates
	Walls []WallSegmentData `json:"walls"`
}

// ZoneData represents a logical zone within an environment.
// Zones are boundaries for queries and visibility, not separate coordinate spaces.
// All coordinates are already absolute.
//
// Purpose: Allows queries like "what zone contains this hex?" and enables
// zone-based visibility control.
type ZoneData struct {
	// ID is the unique identifier for this zone
	ID string `json:"id"`

	// Type categorizes the zone (game-specific, e.g., "room", "corridor", "outdoor")
	Type string `json:"type"`

	// Origin is the zone's position in absolute environment coordinates
	// This is where the zone's local (0,0,0) maps to in environment space
	Origin spatial.CubeCoordinate `json:"origin"`

	// Width is the number of hexes across (X range)
	Width int `json:"width"`

	// Height is the number of hexes down (Z range)
	Height int `json:"height"`

	// EntityIDs lists all entities that belong to this zone
	// Useful for zone-based queries and visibility filtering
	EntityIDs []string `json:"entity_ids,omitempty"`
}

// PassageData represents a link between two zones.
// Passages are abstract relationships - any physical representation (doors, etc.)
// is stored as an entity.
//
// Purpose: Enables pathfinding between zones and tracks which entities
// control passage (if any).
type PassageData struct {
	// ID is the unique identifier for this passage
	ID string `json:"id"`

	// FromZoneID is the source zone
	FromZoneID string `json:"from_zone_id"`

	// ToZoneID is the destination zone
	ToZoneID string `json:"to_zone_id"`

	// ControllingEntityID references an entity that controls this passage (optional)
	// If empty, the passage is always open
	// Games use this to reference doors, gates, portals, etc.
	ControllingEntityID string `json:"controlling_entity_id,omitempty"`

	// Bidirectional indicates if movement works both ways (default true)
	Bidirectional bool `json:"bidirectional"`
}

// PlacedEntityData represents an entity placed in the environment.
// All positions are in absolute environment coordinates.
//
// This is a generic placement record - the entity type and properties
// are game-specific strings/data.
type PlacedEntityData struct {
	// ID is the unique identifier for this entity
	ID string `json:"id"`

	// Type is a game-specific entity type (e.g., "monster", "door", "character")
	// The environments package does not interpret this value
	Type string `json:"type"`

	// Position is where the entity is located in absolute coordinates
	Position spatial.CubeCoordinate `json:"position"`

	// Size is how many hexes the entity occupies (1 = single hex)
	Size int `json:"size"`

	// BlocksMovement indicates if other entities cannot move through this space
	BlocksMovement bool `json:"blocks_movement"`

	// BlocksLoS indicates if this entity blocks line of sight
	BlocksLoS bool `json:"blocks_los"`

	// ZoneID indicates which zone this entity belongs to
	// Used for visibility filtering and zone-based queries
	ZoneID string `json:"zone_id"`

	// Subtype is a game-specific visual/behavioral subtype
	// Games map this to assets or behaviors
	Subtype string `json:"subtype,omitempty"`

	// Properties contains game-specific data
	// The environments package does not interpret this data
	Properties map[string]any `json:"properties,omitempty"`
}

// WallSegmentData represents a wall segment in the environment.
// Walls are defined by their endpoints in absolute coordinates.
//
// Purpose: Enables collision detection and line-of-sight calculations.
type WallSegmentData struct {
	// Start is one endpoint of the wall segment
	Start spatial.CubeCoordinate `json:"start"`

	// End is the other endpoint of the wall segment
	End spatial.CubeCoordinate `json:"end"`

	// BlocksMovement indicates if entities cannot pass through
	BlocksMovement bool `json:"blocks_movement"`

	// BlocksLoS indicates if the wall blocks line of sight
	BlocksLoS bool `json:"blocks_los"`
}
