package spatial

import (
	"context"
	"fmt"

	"github.com/KirkDiggler/rpg-toolkit/core"
)

// SpatialQueryHandler handles spatial query events
//
//nolint:revive // type name follows package naming conventions used throughout codebase
type SpatialQueryHandler struct {
	rooms map[string]Room
}

// NewSpatialQueryHandler creates a new spatial query handler
func NewSpatialQueryHandler() *SpatialQueryHandler {
	return &SpatialQueryHandler{
		rooms: make(map[string]Room),
	}
}

// RegisterRoom registers a room with the query handler
func (h *SpatialQueryHandler) RegisterRoom(room Room) {
	h.rooms[room.GetID()] = room
}

// UnregisterRoom removes a room from the query handler
func (h *SpatialQueryHandler) UnregisterRoom(roomID string) {
	delete(h.rooms, roomID)
}

// HandleQuery processes spatial queries
func (h *SpatialQueryHandler) HandleQuery(ctx context.Context, query interface{}) (interface{}, error) {
	switch q := query.(type) {
	case *QueryPositionsInRangeData:
		return h.handlePositionsInRange(ctx, q)
	case *QueryEntitiesInRangeData:
		return h.handleEntitiesInRange(ctx, q)
	case *QueryLineOfSightData:
		return h.handleLineOfSight(ctx, q)
	case *QueryMovementData:
		return h.handleMovement(ctx, q)
	case *QueryPlacementData:
		return h.handlePlacement(ctx, q)
	default:
		return nil, fmt.Errorf("unsupported query type: %T", query)
	}
}

// handlePositionsInRange handles position range queries
//
//nolint:unparam // error is always nil by design - errors are stored in data struct
func (h *SpatialQueryHandler) handlePositionsInRange(
	_ context.Context, data *QueryPositionsInRangeData,
) (*QueryPositionsInRangeData, error) {
	room, exists := h.rooms[data.RoomID]
	if !exists {
		data.Error = fmt.Errorf("room %s not found", data.RoomID)
		return data, nil
	}

	data.Results = room.GetPositionsInRange(data.Center, data.Radius)
	return data, nil
}

// handleEntitiesInRange handles entity range queries
//
//nolint:unparam // error is always nil by design - errors are stored in data struct
func (h *SpatialQueryHandler) handleEntitiesInRange(
	_ context.Context, data *QueryEntitiesInRangeData,
) (*QueryEntitiesInRangeData, error) {
	room, exists := h.rooms[data.RoomID]
	if !exists {
		data.Error = fmt.Errorf("room %s not found", data.RoomID)
		return data, nil
	}

	entities := room.GetEntitiesInRange(data.Center, data.Radius)

	// Apply filter if provided
	if data.Filter != nil {
		filtered := make([]core.Entity, 0)
		for _, entity := range entities {
			if data.Filter.Matches(entity) {
				filtered = append(filtered, entity)
			}
		}
		data.Results = filtered
	} else {
		data.Results = entities
	}

	return data, nil
}

// handleLineOfSight handles line of sight queries
//
//nolint:unparam // error is always nil by design - errors are stored in data struct
func (h *SpatialQueryHandler) handleLineOfSight(
	_ context.Context, data *QueryLineOfSightData,
) (*QueryLineOfSightData, error) {
	room, exists := h.rooms[data.RoomID]
	if !exists {
		data.Error = fmt.Errorf("room %s not found", data.RoomID)
		return data, nil
	}

	data.Results = room.GetLineOfSight(data.From, data.To)
	data.Blocked = room.IsLineOfSightBlocked(data.From, data.To)
	return data, nil
}

// handleMovement handles movement queries
//
//nolint:unparam // error is always nil by design - errors are stored in data struct
func (h *SpatialQueryHandler) handleMovement(_ context.Context, data *QueryMovementData) (*QueryMovementData, error) {
	room, exists := h.rooms[data.RoomID]
	if !exists {
		data.Error = fmt.Errorf("room %s not found", data.RoomID)
		return data, nil
	}

	// Check if the entity can move to the target position
	data.Valid = room.CanPlaceEntity(data.Entity, data.To)

	// Calculate distance using the room's grid
	data.Distance = room.GetGrid().Distance(data.From, data.To)

	// Get the path (line of sight for now, could be enhanced with pathfinding)
	data.Path = room.GetLineOfSight(data.From, data.To)

	return data, nil
}

// handlePlacement handles placement queries
//
//nolint:unparam // error is always nil by design - errors are stored in data struct
func (h *SpatialQueryHandler) handlePlacement(
	_ context.Context, data *QueryPlacementData,
) (*QueryPlacementData, error) {
	room, exists := h.rooms[data.RoomID]
	if !exists {
		data.Error = fmt.Errorf("room %s not found", data.RoomID)
		return data, nil
	}

	data.Valid = room.CanPlaceEntity(data.Entity, data.Position)
	return data, nil
}
