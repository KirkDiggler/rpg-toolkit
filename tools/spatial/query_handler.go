package spatial

import (
	"context"
	"fmt"

	"github.com/KirkDiggler/rpg-toolkit/core"
	"github.com/KirkDiggler/rpg-toolkit/events"
)

// SpatialQueryHandler handles spatial query events
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
func (h *SpatialQueryHandler) handlePositionsInRange(ctx context.Context, data *QueryPositionsInRangeData) (*QueryPositionsInRangeData, error) {
	room, exists := h.rooms[data.RoomID]
	if !exists {
		data.Error = fmt.Errorf("room %s not found", data.RoomID)
		return data, nil
	}

	data.Results = room.GetPositionsInRange(data.Center, data.Radius)
	return data, nil
}

// handleEntitiesInRange handles entity range queries
func (h *SpatialQueryHandler) handleEntitiesInRange(ctx context.Context, data *QueryEntitiesInRangeData) (*QueryEntitiesInRangeData, error) {
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
func (h *SpatialQueryHandler) handleLineOfSight(ctx context.Context, data *QueryLineOfSightData) (*QueryLineOfSightData, error) {
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
func (h *SpatialQueryHandler) handleMovement(ctx context.Context, data *QueryMovementData) (*QueryMovementData, error) {
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
func (h *SpatialQueryHandler) handlePlacement(ctx context.Context, data *QueryPlacementData) (*QueryPlacementData, error) {
	room, exists := h.rooms[data.RoomID]
	if !exists {
		data.Error = fmt.Errorf("room %s not found", data.RoomID)
		return data, nil
	}

	data.Valid = room.CanPlaceEntity(data.Entity, data.Position)
	return data, nil
}

// RegisterWithEventBus registers the query handler with an event bus
func (h *SpatialQueryHandler) RegisterWithEventBus(eventBus events.EventBus) {
	// Register handlers for all spatial query events
	eventBus.SubscribeFunc(EventQueryPositionsInRange, 0, h.handlePositionsInRangeEvent)
	eventBus.SubscribeFunc(EventQueryEntitiesInRange, 0, h.handleEntitiesInRangeEvent)
	eventBus.SubscribeFunc(EventQueryLineOfSight, 0, h.handleLineOfSightEvent)
	eventBus.SubscribeFunc(EventQueryMovement, 0, h.handleMovementEvent)
	eventBus.SubscribeFunc(EventQueryPlacement, 0, h.handlePlacementEvent)
}

// Event handler functions that extract data from events and process queries

func (h *SpatialQueryHandler) handlePositionsInRangeEvent(ctx context.Context, event events.Event) error {
	center, _ := event.Context().Get("center")
	radius, _ := event.Context().Get("radius")
	roomID, _ := event.Context().Get("room_id")
	
	data := &QueryPositionsInRangeData{
		Center: center.(Position),
		Radius: radius.(float64),
		RoomID: roomID.(string),
	}
	
	result, err := h.handlePositionsInRange(ctx, data)
	if err != nil {
		return err
	}
	
	// Store result back in event context
	event.Context().Set("results", result.Results)
	if result.Error != nil {
		event.Context().Set("error", result.Error)
	}
	
	return nil
}

func (h *SpatialQueryHandler) handleEntitiesInRangeEvent(ctx context.Context, event events.Event) error {
	center, _ := event.Context().Get("center")
	radius, _ := event.Context().Get("radius")
	roomID, _ := event.Context().Get("room_id")
	filter, _ := event.Context().Get("filter")
	
	data := &QueryEntitiesInRangeData{
		Center: center.(Position),
		Radius: radius.(float64),
		RoomID: roomID.(string),
	}
	
	if filter != nil {
		data.Filter = filter.(EntityFilter)
	}
	
	result, err := h.handleEntitiesInRange(ctx, data)
	if err != nil {
		return err
	}
	
	// Store result back in event context
	event.Context().Set("results", result.Results)
	if result.Error != nil {
		event.Context().Set("error", result.Error)
	}
	
	return nil
}

func (h *SpatialQueryHandler) handleLineOfSightEvent(ctx context.Context, event events.Event) error {
	from, _ := event.Context().Get("from")
	to, _ := event.Context().Get("to")
	roomID, _ := event.Context().Get("room_id")
	
	data := &QueryLineOfSightData{
		From:   from.(Position),
		To:     to.(Position),
		RoomID: roomID.(string),
	}
	
	result, err := h.handleLineOfSight(ctx, data)
	if err != nil {
		return err
	}
	
	// Store result back in event context
	event.Context().Set("results", result.Results)
	event.Context().Set("blocked", result.Blocked)
	if result.Error != nil {
		event.Context().Set("error", result.Error)
	}
	
	return nil
}

func (h *SpatialQueryHandler) handleMovementEvent(ctx context.Context, event events.Event) error {
	entity, _ := event.Context().Get("entity")
	from, _ := event.Context().Get("from")
	to, _ := event.Context().Get("to")
	roomID, _ := event.Context().Get("room_id")
	
	data := &QueryMovementData{
		Entity: entity.(core.Entity),
		From:   from.(Position),
		To:     to.(Position),
		RoomID: roomID.(string),
	}
	
	result, err := h.handleMovement(ctx, data)
	if err != nil {
		return err
	}
	
	// Store result back in event context
	event.Context().Set("valid", result.Valid)
	event.Context().Set("path", result.Path)
	event.Context().Set("distance", result.Distance)
	if result.Error != nil {
		event.Context().Set("error", result.Error)
	}
	
	return nil
}

func (h *SpatialQueryHandler) handlePlacementEvent(ctx context.Context, event events.Event) error {
	entity, _ := event.Context().Get("entity")
	position, _ := event.Context().Get("position")
	roomID, _ := event.Context().Get("room_id")
	
	data := &QueryPlacementData{
		Entity:   entity.(core.Entity),
		Position: position.(Position),
		RoomID:   roomID.(string),
	}
	
	result, err := h.handlePlacement(ctx, data)
	if err != nil {
		return err
	}
	
	// Store result back in event context
	event.Context().Set("valid", result.Valid)
	if result.Error != nil {
		event.Context().Set("error", result.Error)
	}
	
	return nil
}