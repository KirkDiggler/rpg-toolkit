package spatial

import (
	"context"
	"fmt"

	"github.com/KirkDiggler/rpg-toolkit/game"
)

// Grid type constants for room data
const (
	// GridTypeSquare represents square grid type in room data
	GridTypeSquare = "square"
	// GridTypeHex represents hexagonal grid type in room data
	GridTypeHex = "hex"
	// GridTypeGridless represents gridless type in room data
	GridTypeGridless = "gridless"
)

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

	// Entities contains positioned entities within the room
	// Map of entity ID to their position and data
	Entities map[string]EntityPlacement `json:"entities,omitempty"`
}

// EntityPlacement represents an entity's position and basic data in a room
type EntityPlacement struct {
	// EntityID is the unique identifier of the entity
	EntityID string `json:"entity_id"`

	// EntityType is the type of the entity (e.g., "character", "monster", "object")
	EntityType string `json:"entity_type"`

	// Position is where the entity is placed in the room
	Position Position `json:"position"`
}

// ToData converts a BasicRoom to RoomData for persistence.
// This captures the room's state including all placed entities.
func (r *BasicRoom) ToData() RoomData {
	r.mutex.RLock()
	defer r.mutex.RUnlock()

	// Determine grid type string
	var gridType string
	switch r.grid.GetShape() {
	case GridShapeSquare:
		gridType = GridTypeSquare
	case GridShapeHex:
		gridType = GridTypeHex
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
			entities[id] = EntityPlacement{
				EntityID:   entity.GetID(),
				EntityType: entity.GetType(),
				Position:   pos,
			}
		}
	}

	return RoomData{
		ID:       r.id,
		Type:     r.roomType,
		Width:    int(dims.Width),
		Height:   int(dims.Height),
		GridType: gridType,
		Entities: entities,
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
	case GridTypeHex:
		grid = NewHexGrid(HexGridConfig{
			Width:  float64(data.Width),
			Height: float64(data.Height),
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
		ID:       data.ID,
		Type:     data.Type,
		Grid:     grid,
		EventBus: eventBus,
	})

	// Note: Entity placement would be handled by the caller
	// They would need to resolve entity IDs to actual entities
	// This keeps the room loader focused on room construction

	return room, nil
}
