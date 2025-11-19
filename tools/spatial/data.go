package spatial

import (
	"context"
	"fmt"

	"github.com/KirkDiggler/rpg-toolkit/core"
	"github.com/KirkDiggler/rpg-toolkit/game"
)

// Grid type constants for room data
const (
	// GridTypeSquare represents square grid type in room data
	GridTypeSquare = "square"

	// GridTypeHex represents hexagonal grid type in room data (generic)
	// Use when orientation is specified separately via HexOrientation field
	GridTypeHex = "hex"

	// GridTypeHexPointy represents pointy-top hexagonal grid type
	// Use when orientation needs to be explicit in the grid type string (e.g., proto enums)
	GridTypeHexPointy = "hex_pointy"

	// GridTypeHexFlat represents flat-top hexagonal grid type
	// Use when orientation needs to be explicit in the grid type string (e.g., proto enums)
	GridTypeHexFlat = "hex_flat"

	// GridTypeGridless represents gridless type in room data
	GridTypeGridless = "gridless"
)

// GridTypeFromHexOrientation returns the orientation-specific grid type string
// for hex grids. Returns GridTypeHexPointy for pointyTop=true, GridTypeHexFlat for false.
// Use this when you need to represent hex orientation as part of the grid type (e.g., proto enums).
func GridTypeFromHexOrientation(pointyTop bool) string {
	if pointyTop {
		return GridTypeHexPointy
	}
	return GridTypeHexFlat
}

// ParseHexGridType parses an orientation-specific hex grid type string
// and returns the orientation boolean. Returns error if not a hex grid type.
// Supports both generic "hex" and orientation-specific "hex_pointy"/"hex_flat" formats.
func ParseHexGridType(gridType string) (pointyTop bool, err error) {
	switch gridType {
	case GridTypeHexPointy:
		return true, nil
	case GridTypeHexFlat:
		return false, nil
	case GridTypeHex:
		// Generic hex defaults to pointy-top for D&D 5e compatibility
		return true, nil
	default:
		return false, fmt.Errorf("not a hex grid type: %s", gridType)
	}
}

// RoomData contains all information needed to persist and reconstruct a room.
// This follows the established data pattern for serialization and loading.
type RoomData struct {
	// ID is the unique identifier for the room
	ID string `json:"id"`

	// Type categorizes the room (e.g., "dungeon", "tavern", "outdoor")
	Type string `json:"type"`

	// Width defines the horizontal size of the room
	Width int `json:"width"`

	// Height defines the vertical size of the room
	Height int `json:"height"`

	// GridType specifies the grid system: "square", "hex", or "gridless"
	GridType string `json:"grid_type"`

	// HexOrientation specifies hex orientation: true for pointy-top, false for flat-top
	// Only used when GridType is "hex", defaults to true (pointy-top) for D&D 5e compatibility
	// Uses pointer to distinguish between explicit false and unset (which defaults to true)
	HexOrientation *bool `json:"hex_orientation,omitempty"`

	// Entities contains positioned entities within the room
	// Map of entity ID to their position and data
	Entities map[string]EntityPlacement `json:"entities,omitempty"`
}

// EntityPlacement represents an entity's position and spatial properties in a room
type EntityPlacement struct {
	// EntityID is the unique identifier of the entity
	EntityID string `json:"entity_id"`

	// EntityType is the type of the entity (e.g., "character", "monster", "object")
	EntityType string `json:"entity_type"`

	// Position is where the entity is placed in the room
	Position Position `json:"position"`

	// Size is how many grid spaces the entity occupies (default 1)
	Size int `json:"size,omitempty"`

	// BlocksMovement indicates if this entity blocks movement through its space
	BlocksMovement bool `json:"blocks_movement"`

	// BlocksLineOfSight indicates if this entity blocks line of sight
	BlocksLineOfSight bool `json:"blocks_line_of_sight"`
}

// PlaceableData is a minimal implementation of Placeable for spatial queries.
// It contains just enough data to support movement and line of sight calculations.
type PlaceableData struct {
	id                string
	entityType        string
	size              int
	blocksMovement    bool
	blocksLineOfSight bool
}

// GetID returns the entity's unique identifier
func (p *PlaceableData) GetID() string {
	return p.id
}

// GetType returns the entity's type
func (p *PlaceableData) GetType() core.EntityType {
	return core.EntityType(p.entityType)
}

// GetSize returns the size of the entity
func (p *PlaceableData) GetSize() int {
	if p.size < 1 {
		return 1 // Default size
	}
	return p.size
}

// BlocksMovement returns true if the entity blocks movement
func (p *PlaceableData) BlocksMovement() bool {
	return p.blocksMovement
}

// BlocksLineOfSight returns true if the entity blocks line of sight
func (p *PlaceableData) BlocksLineOfSight() bool {
	return p.blocksLineOfSight
}

// ToData converts a BasicRoom to RoomData for persistence.
// This captures the room's state including all placed entities.
func (r *BasicRoom) ToData() RoomData {
	r.mutex.RLock()
	defer r.mutex.RUnlock()

	// Determine grid type string and capture hex orientation
	var gridType string
	var hexOrientation *bool
	switch r.grid.GetShape() {
	case GridShapeSquare:
		gridType = GridTypeSquare
	case GridShapeHex:
		gridType = GridTypeHex
		// Hex grid shape will always be *HexGrid
		hexGrid := r.grid.(*HexGrid)
		orientation := hexGrid.GetOrientation()
		hexOrientation = &orientation
	case GridShapeGridless:
		gridType = GridTypeGridless
	default:
		gridType = GridTypeSquare // Default fallback
	}

	// Get dimensions from grid
	dims := r.grid.GetDimensions()

	// Build entity placements
	entities := make(map[string]EntityPlacement)
	for id, entity := range r.entities {
		if pos, exists := r.positions[id]; exists {
			placement := EntityPlacement{
				EntityID:   entity.GetID(),
				EntityType: string(entity.GetType()),
				Position:   pos,
			}

			// Check if entity implements Placeable to get spatial properties
			if placeable, ok := entity.(Placeable); ok {
				placement.Size = placeable.GetSize()
				placement.BlocksMovement = placeable.BlocksMovement()
				placement.BlocksLineOfSight = placeable.BlocksLineOfSight()
			}

			entities[id] = placement
		}
	}

	return RoomData{
		ID:             r.id,
		Type:           r.roomType,
		Width:          int(dims.Width),
		Height:         int(dims.Height),
		GridType:       gridType,
		HexOrientation: hexOrientation,
		Entities:       entities,
	}
}

// LoadRoomFromContext creates a BasicRoom from data using the GameContext pattern.
// This allows the room to integrate with the event system and other game infrastructure.
func LoadRoomFromContext(_ context.Context, gameCtx game.Context[RoomData]) (*BasicRoom, error) {
	data := gameCtx.Data()
	eventBus := gameCtx.EventBus()

	// Create the appropriate grid based on type
	var grid Grid
	switch data.GridType {
	case GridTypeSquare:
		grid = NewSquareGrid(SquareGridConfig{
			Width:  float64(data.Width),
			Height: float64(data.Height),
		})
	case GridTypeHex, GridTypeHexPointy, GridTypeHexFlat:
		// Determine orientation from grid type or HexOrientation field
		var hexOrientation bool
		if data.GridType == GridTypeHexPointy || data.GridType == GridTypeHexFlat {
			// Orientation is explicit in grid type
			hexOrientation = data.GridType == GridTypeHexPointy
		} else {
			// Generic "hex" type, check HexOrientation field
			hexOrientation = true // Default for D&D 5e
			if data.HexOrientation != nil {
				hexOrientation = *data.HexOrientation
			}
		}
		grid = NewHexGrid(HexGridConfig{
			Width:     float64(data.Width),
			Height:    float64(data.Height),
			PointyTop: hexOrientation,
		})
	case GridTypeGridless:
		grid = NewGridlessRoom(GridlessConfig{
			Width:  float64(data.Width),
			Height: float64(data.Height),
		})
	default:
		return nil, fmt.Errorf("unknown grid type: %s", data.GridType)
	}

	// Create the room
	room := NewBasicRoom(BasicRoomConfig{
		ID:   data.ID,
		Type: data.Type,
		Grid: grid,
	})

	// Connect to event bus for typed events
	room.ConnectToEventBus(eventBus)

	// Place entities using minimal spatial data
	for _, placement := range data.Entities {
		// Create a minimal placeable entity with just spatial properties
		entity := &PlaceableData{
			id:                placement.EntityID,
			entityType:        placement.EntityType,
			size:              placement.Size,
			blocksMovement:    placement.BlocksMovement,
			blocksLineOfSight: placement.BlocksLineOfSight,
		}

		// Place the entity in the room
		if err := room.PlaceEntity(entity, placement.Position); err != nil {
			// Log error but continue loading other entities
			// In production, might want to handle this differently
			continue
		}
	}

	return room, nil
}
