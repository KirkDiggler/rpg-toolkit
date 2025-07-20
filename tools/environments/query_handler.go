package environments

import (
	"context"
	"fmt"
	"sync"

	"github.com/KirkDiggler/rpg-toolkit/core"
	"github.com/KirkDiggler/rpg-toolkit/events"
	"github.com/KirkDiggler/rpg-toolkit/tools/spatial"
)

// BasicQueryHandler implements the QueryHandler interface
// Purpose: Provides environment-level queries that aggregate across multiple rooms
// while delegating the actual spatial math to the spatial infrastructure.
// This is the "query aggregator" pattern from ADR-0011.
type BasicQueryHandler struct {
	// Dependencies - we are a CLIENT of spatial, not replacement
	orchestrator spatial.RoomOrchestrator
	spatialQuery *spatial.QueryUtils
	eventBus     events.EventBus

	// Thread safety
	mutex sync.RWMutex
}

// BasicQueryHandlerConfig follows toolkit config pattern
type BasicQueryHandlerConfig struct {
	Orchestrator spatial.RoomOrchestrator `json:"-"`
	SpatialQuery *spatial.QueryUtils      `json:"-"`
	EventBus     events.EventBus          `json:"-"`
}

// NewBasicQueryHandler creates a new query handler
func NewBasicQueryHandler(config BasicQueryHandlerConfig) *BasicQueryHandler {
	return &BasicQueryHandler{
		orchestrator: config.Orchestrator,
		spatialQuery: config.SpatialQuery,
		eventBus:     config.EventBus,
	}
}

// QueryHandler interface implementation

// HandleEntityQuery processes an entity query across the environment
// It handles different query types: specific rooms, range-based, or environment-wide.
func (h *BasicQueryHandler) HandleEntityQuery(ctx context.Context, query EntityQuery) ([]core.Entity, error) {
	h.mutex.RLock()
	defer h.mutex.RUnlock()

	// Publish query event for monitoring/debugging
	h.publishQueryEventUnsafe(ctx, "entity_query", query)

	// Handle different query types
	switch {
	case len(query.RoomIDs) > 0:
		// Specific rooms query
		return h.handleSpecificRoomsEntityQueryUnsafe(ctx, query)
	case query.Center != nil:
		// Range-based query across all rooms
		return h.handleRangeEntityQueryUnsafe(ctx, query)
	default:
		// Environment-wide query with filters
		return h.handleEnvironmentWideEntityQueryUnsafe(ctx, query)
	}
}

// HandleRoomQuery processes a room query across the environment
// It applies filters for room types, themes, features, spatial proximity, and connections.
func (h *BasicQueryHandler) HandleRoomQuery(ctx context.Context, query RoomQuery) ([]spatial.Room, error) {
	h.mutex.RLock()
	defer h.mutex.RUnlock()

	// Publish query event for monitoring/debugging
	h.publishQueryEventUnsafe(ctx, "room_query", query)

	// Start with all rooms from orchestrator
	allRooms := h.orchestrator.GetAllRooms()
	var matchingRooms []spatial.Room

	// Apply filters
	for _, room := range allRooms {
		if h.roomMatchesQueryUnsafe(room, query) {
			matchingRooms = append(matchingRooms, room)
		}
	}

	// Apply spatial proximity filters
	if query.NearPosition != nil {
		matchingRooms = h.filterRoomsByProximityUnsafe(matchingRooms, *query.NearPosition, query.MaxDistance)
	}

	// Apply connection filters
	if query.ConnectedTo != "" {
		matchingRooms = h.filterRoomsByConnectionUnsafe(
			matchingRooms, query.ConnectedTo, query.MinConnections, query.MaxConnections,
		)
	}

	// Apply limit
	if query.Limit > 0 && len(matchingRooms) > query.Limit {
		matchingRooms = matchingRooms[:query.Limit]
	}

	return matchingRooms, nil
}

// HandlePathQuery processes a pathfinding query between two positions
// It finds the rooms containing the positions and delegates pathfinding to spatial infrastructure.
func (h *BasicQueryHandler) HandlePathQuery(ctx context.Context, query PathQuery) ([]spatial.Position, error) {
	h.mutex.RLock()
	defer h.mutex.RUnlock()

	// Publish query event for monitoring/debugging
	h.publishQueryEventUnsafe(ctx, "path_query", query)

	// For now, use basic orchestrator pathfinding
	// This is a simplified implementation - in a full system we'd implement
	// A* pathfinding with constraint handling

	// Find rooms containing the from and to positions
	fromRoomID := h.findRoomContainingPositionUnsafe(query.From)
	toRoomID := h.findRoomContainingPositionUnsafe(query.To)

	if fromRoomID == "" || toRoomID == "" {
		return nil, fmt.Errorf("could not find rooms containing path endpoints")
	}

	// If same room, delegate to spatial room pathfinding
	if fromRoomID == toRoomID {
		return h.handleSameRoomPathUnsafe(query, fromRoomID)
	}

	// Cross-room pathfinding
	return h.handleCrossRoomPathUnsafe(query, fromRoomID, toRoomID)
}

// HandleCapacityQuery processes a capacity analysis query
// Purpose: Provides advisory information about room capacity for intelligent room design
func (h *BasicQueryHandler) HandleCapacityQuery(
	ctx context.Context, query CapacityQuery,
) (CapacityQueryResponse, error) {
	h.mutex.RLock()
	defer h.mutex.RUnlock()

	// Publish query event for monitoring/debugging
	h.publishQueryEventUnsafe(ctx, "capacity_query", query)

	// Use room size from query or try to get from existing room
	roomSize := query.RoomSize
	if roomSize.Width == 0 || roomSize.Height == 0 {
		if query.RoomID != "" {
			room, exists := h.orchestrator.GetRoom(query.RoomID)
			if exists {
				dimensions := room.GetGrid().GetDimensions()
				roomSize = spatial.Dimensions{Width: dimensions.Width, Height: dimensions.Height}
			}
		}
	}

	// Use default constraints if none provided
	constraints := query.Constraints
	if constraints.TargetSpatialFeeling == "" {
		constraints.TargetSpatialFeeling = SpatialFeelingNormal
	}
	if constraints.MinMovementSpace == 0 {
		constraints.MinMovementSpace = 0.6
	}
	if constraints.MinEntitySpacing == 0 {
		constraints.MinEntitySpacing = 1.0
	}
	if constraints.WallDensityModifier == 0 {
		constraints.WallDensityModifier = 0.5
	}
	if constraints.RequiredPathwayMultiplier == 0 {
		constraints.RequiredPathwayMultiplier = 1.2
	}

	// Estimate capacity
	estimate := EstimateRoomCapacity(roomSize, constraints)

	// Get split options if requested
	var splitOptions []RoomSplit
	if query.IncludeSplitOptions {
		entityCount := query.EntityCount
		if entityCount == 0 {
			entityCount = query.ExistingEntityCount
		}
		if entityCount == 0 {
			entityCount = estimate.RecommendedEntityCount
		}
		splitOptions = GetSplitOptions(roomSize, entityCount, constraints)
	}

	// Analyze capacity for specific entity count if provided
	var analysis CapacityAnalysis
	entityCount := query.EntityCount
	if entityCount == 0 {
		entityCount = query.ExistingEntityCount
	}
	if entityCount > 0 {
		analysis = AnalyzeRoomCapacityForEntityCount(roomSize, entityCount, constraints)
	}

	// Determine if requirements are satisfied
	satisfied := true
	var alternatives []string
	if entityCount > 0 && entityCount > estimate.RecommendedEntityCount {
		satisfied = false
		alternatives = append(alternatives, "Consider splitting the room")
		alternatives = append(alternatives, "Reduce entity count")
		alternatives = append(alternatives, "Increase room size")
	}

	return CapacityQueryResponse{
		Estimate:     estimate,
		SplitOptions: splitOptions,
		Analysis:     analysis,
		Satisfied:    satisfied,
		Alternatives: alternatives,
	}, nil
}

// HandleSizingQuery processes a room sizing query
// Purpose: Calculates optimal room dimensions for specific requirements
func (h *BasicQueryHandler) HandleSizingQuery(ctx context.Context, query SizingQuery) (spatial.Dimensions, error) {
	h.mutex.RLock()
	defer h.mutex.RUnlock()

	// Publish query event for monitoring/debugging
	h.publishQueryEventUnsafe(ctx, "sizing_query", query)

	// Use IntentProfile if Intent is empty
	intent := query.Intent
	if intent.Feeling == "" && query.IntentProfile.Feeling != "" {
		intent = query.IntentProfile
	}

	// Use default intent if none provided
	if intent.Feeling == "" {
		intent = GetDefaultSpatialIntentProfile(SpatialFeelingNormal)
	}

	// Calculate optimal room size
	dimensions := CalculateOptimalRoomSize(intent, query.EntityCount)

	// Apply additional space multiplier
	if query.AdditionalSpace > 0 {
		dimensions.Width *= query.AdditionalSpace
		dimensions.Height *= query.AdditionalSpace
	}

	// Enforce constraints
	if query.MinDimensions.Width > 0 && dimensions.Width < query.MinDimensions.Width {
		dimensions.Width = query.MinDimensions.Width
	}
	if query.MinDimensions.Height > 0 && dimensions.Height < query.MinDimensions.Height {
		dimensions.Height = query.MinDimensions.Height
	}
	if query.MaxDimensions.Width > 0 && dimensions.Width > query.MaxDimensions.Width {
		dimensions.Width = query.MaxDimensions.Width
	}
	if query.MaxDimensions.Height > 0 && dimensions.Height > query.MaxDimensions.Height {
		dimensions.Height = query.MaxDimensions.Height
	}

	return dimensions, nil
}

// Entity query implementations

func (h *BasicQueryHandler) handleSpecificRoomsEntityQueryUnsafe(
	ctx context.Context, query EntityQuery,
) ([]core.Entity, error) {
	var allEntities []core.Entity

	// Query each specified room and aggregate results
	for _, roomID := range query.RoomIDs {
		room, exists := h.orchestrator.GetRoom(roomID)
		if !exists {
			continue // Skip non-existent rooms
		}

		// Delegate to spatial query for this room
		roomEntities, err := h.querySingleRoomEntitiesUnsafe(ctx, room, query)
		if err != nil {
			return nil, fmt.Errorf("failed to query room %s: %w", roomID, err)
		}

		allEntities = append(allEntities, roomEntities...)
	}

	// Apply environment-specific filters
	allEntities = h.applyEnvironmentFiltersUnsafe(allEntities, query)

	// Apply limit
	if query.Limit > 0 && len(allEntities) > query.Limit {
		allEntities = allEntities[:query.Limit]
	}

	return allEntities, nil
}

func (h *BasicQueryHandler) handleRangeEntityQueryUnsafe(
	ctx context.Context, query EntityQuery,
) ([]core.Entity, error) {
	var allEntities []core.Entity

	// Query all rooms that intersect with the range
	allRooms := h.orchestrator.GetAllRooms()

	for _, room := range allRooms {
		// Check if room intersects with query range
		if h.roomIntersectsRangeUnsafe(room, *query.Center, query.Radius) {
			// Delegate to spatial query for this room
			roomEntities, err := h.querySingleRoomEntitiesUnsafe(ctx, room, query)
			if err != nil {
				return nil, fmt.Errorf("failed to query room %s: %w", room.GetID(), err)
			}

			allEntities = append(allEntities, roomEntities...)
		}
	}

	// Apply environment-specific filters
	allEntities = h.applyEnvironmentFiltersUnsafe(allEntities, query)

	// Apply limit
	if query.Limit > 0 && len(allEntities) > query.Limit {
		allEntities = allEntities[:query.Limit]
	}

	return allEntities, nil
}

func (h *BasicQueryHandler) handleEnvironmentWideEntityQueryUnsafe(
	ctx context.Context, query EntityQuery,
) ([]core.Entity, error) {
	var allEntities []core.Entity

	// Query all rooms in the environment
	allRooms := h.orchestrator.GetAllRooms()

	for _, room := range allRooms {
		// Apply room-level filters first
		if !h.roomMatchesEntityQueryUnsafe(room, query) {
			continue
		}

		// Delegate to spatial query for this room
		roomEntities, err := h.querySingleRoomEntitiesUnsafe(ctx, room, query)
		if err != nil {
			return nil, fmt.Errorf("failed to query room %s: %w", room.GetID(), err)
		}

		allEntities = append(allEntities, roomEntities...)
	}

	// Apply environment-specific filters
	allEntities = h.applyEnvironmentFiltersUnsafe(allEntities, query)

	// Apply limit
	if query.Limit > 0 && len(allEntities) > query.Limit {
		allEntities = allEntities[:query.Limit]
	}

	return allEntities, nil
}

func (h *BasicQueryHandler) querySingleRoomEntitiesUnsafe(
	ctx context.Context, room spatial.Room, query EntityQuery,
) ([]core.Entity, error) {
	// This is where we DELEGATE to spatial infrastructure
	// We don't duplicate the spatial query logic

	if h.spatialQuery == nil {
		// Fallback to room's direct methods
		if query.Center != nil {
			return room.GetEntitiesInRange(*query.Center, query.Radius), nil
		}

		// Get all entities in room
		entitiesMap := room.GetAllEntities()
		entities := make([]core.Entity, 0, len(entitiesMap))
		for _, entity := range entitiesMap {
			entities = append(entities, entity)
		}
		return entities, nil
	}

	// Use spatial query utils for more sophisticated queries
	filter := h.createSpatialFilterUnsafe(query)

	if query.Center != nil {
		return h.spatialQuery.QueryEntitiesInRange(ctx, *query.Center, query.Radius, room.GetID(), filter)
	}
	// Query entire room
	center := spatial.Position{X: 0, Y: 0} // Room center would be better
	dimensions := room.GetGrid().GetDimensions()
	maxRadius := float64(dimensions.Width+dimensions.Height) / 2.0
	return h.spatialQuery.QueryEntitiesInRange(ctx, center, maxRadius, room.GetID(), filter)
}

// Room query helper methods

func (h *BasicQueryHandler) roomMatchesQueryUnsafe(room spatial.Room, query RoomQuery) bool {
	// Check room type filter
	if len(query.RoomTypes) > 0 {
		roomType := room.GetType()
		found := false
		for _, allowedType := range query.RoomTypes {
			if roomType == allowedType {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}

	// Check themes filter
	if len(query.Themes) > 0 {
		// Note: spatial rooms don't have themes - this would need to be tracked
		// in environment metadata or room properties
		// For now, we'll assume rooms can be queried for theme
		// This is where environment adds value beyond spatial
		return true // TODO: Implement theme checking
	}

	// Check features filter
	if len(query.Features) > 0 {
		// Note: spatial rooms don't have features - this would need to be tracked
		// in environment metadata or room properties
		// For now, we'll assume rooms can be queried for features
		return true // TODO: Implement feature checking
	}

	return true
}

func (h *BasicQueryHandler) filterRoomsByProximityUnsafe(
	rooms []spatial.Room, position spatial.Position, maxDistance int,
) []spatial.Room {
	if maxDistance <= 0 {
		return rooms
	}

	filteredRooms := make([]spatial.Room, 0, len(rooms))

	for _, room := range rooms {
		// Calculate distance from position to room
		// This is a simplified implementation - in reality we'd need to:
		// 1. Find the closest point in the room to the position
		// 2. Calculate actual distance through connections

		// For now, check if position is within room bounds
		if h.isPositionInRoomUnsafe(position, room) {
			filteredRooms = append(filteredRooms, room)
			continue
		}

		// TODO: Implement proper distance calculation through connections
		// For now, include all rooms
		filteredRooms = append(filteredRooms, room)
	}

	return filteredRooms
}

func (h *BasicQueryHandler) filterRoomsByConnectionUnsafe(
	rooms []spatial.Room, connectedTo string, minConnections, maxConnections int,
) []spatial.Room {
	filteredRooms := make([]spatial.Room, 0, len(rooms))

	for _, room := range rooms {
		// Count connections for this room
		connectionCount := 0
		allConnections := h.orchestrator.GetAllConnections()

		for _, conn := range allConnections {
			if conn.GetFromRoom() == room.GetID() || conn.GetToRoom() == room.GetID() {
				connectionCount++
			}
		}

		// Check connection count constraints
		if minConnections > 0 && connectionCount < minConnections {
			continue
		}
		if maxConnections > 0 && connectionCount > maxConnections {
			continue
		}

		// Check if connected to specific room
		if connectedTo != "" {
			connected := false
			for _, conn := range allConnections {
				if (conn.GetFromRoom() == room.GetID() && conn.GetToRoom() == connectedTo) ||
					(conn.GetToRoom() == room.GetID() && conn.GetFromRoom() == connectedTo) {
					connected = true
					break
				}
			}
			if !connected {
				continue
			}
		}

		filteredRooms = append(filteredRooms, room)
	}

	return filteredRooms
}

// Pathfinding helper methods

func (h *BasicQueryHandler) findRoomContainingPositionUnsafe(position spatial.Position) string {
	// Find which room contains this position
	allRooms := h.orchestrator.GetAllRooms()

	for _, room := range allRooms {
		if h.isPositionInRoomUnsafe(position, room) {
			return room.GetID()
		}
	}

	return ""
}

func (h *BasicQueryHandler) handleSameRoomPathUnsafe(
	query PathQuery, roomID string,
) ([]spatial.Position, error) {
	// For same-room pathfinding, delegate to spatial room
	_, exists := h.orchestrator.GetRoom(roomID)
	if !exists {
		return nil, fmt.Errorf("room %s not found", roomID)
	}

	// Use spatial query if available
	if h.spatialQuery != nil {
		// TODO: Implement spatial pathfinding query
		return nil, fmt.Errorf("spatial pathfinding not yet implemented")
	}

	// Fallback: simple direct path
	return []spatial.Position{query.From, query.To}, nil
}

func (h *BasicQueryHandler) handleCrossRoomPathUnsafe(
	query PathQuery, fromRoomID, toRoomID string,
) ([]spatial.Position, error) {
	// For cross-room pathfinding, we need to:
	// 1. Find path between rooms using orchestrator
	// 2. Find path within each room
	// 3. Combine the paths

	// Use orchestrator to find room-to-room path
	_, err := h.orchestrator.FindPath(fromRoomID, toRoomID, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to find room path: %w", err)
	}

	// TODO: Convert room path to position path
	// This would require:
	// 1. Finding connection points between rooms
	// 2. Pathfinding within each room from entry to exit
	// 3. Combining all path segments

	// For now, return simplified path
	return []spatial.Position{query.From, query.To}, nil
}

// Filter and utility methods

func (h *BasicQueryHandler) createSpatialFilterUnsafe(query EntityQuery) spatial.EntityFilter {
	// Convert environment query to spatial filter
	filter := spatial.NewSimpleEntityFilter()

	if len(query.EntityTypes) > 0 {
		filter = filter.WithEntityTypes(query.EntityTypes...)
	}

	if len(query.ExcludeIDs) > 0 {
		filter = filter.WithExcludeIDs(query.ExcludeIDs...)
	}

	return filter
}

func (h *BasicQueryHandler) applyEnvironmentFiltersUnsafe(entities []core.Entity, query EntityQuery) []core.Entity {
	// Apply environment-specific filters that go beyond spatial
	filteredEntities := make([]core.Entity, 0, len(entities))

	for _, entity := range entities {
		// Apply theme filter
		if query.InTheme != "" {
			// TODO: Check if entity is in a room with specified theme
			// This requires environment metadata
			// For now, skip entities when theme filter is specified
			continue
		}

		// Apply feature filter
		if query.HasFeature != "" {
			// TODO: Check if entity is in a room with specified feature
			// This requires environment metadata
			// For now, skip entities when feature filter is specified
			continue
		}

		// Include entities that pass all filters
		filteredEntities = append(filteredEntities, entity)
	}

	return filteredEntities
}

func (h *BasicQueryHandler) roomMatchesEntityQueryUnsafe(_ spatial.Room, query EntityQuery) bool {
	// Check if room matches entity query filters

	// Check theme filter
	if query.InTheme != "" {
		// TODO: Check room theme
		// This requires environment metadata
		// For now, reject rooms when theme filter is specified
		return false
	}

	// Check feature filter
	if query.HasFeature != "" {
		// TODO: Check room features
		// This requires environment metadata
		// For now, reject rooms when feature filter is specified
		return false
	}

	return true
}

func (h *BasicQueryHandler) roomIntersectsRangeUnsafe(room spatial.Room, center spatial.Position, radius float64) bool {
	// Check if room intersects with query range
	grid := room.GetGrid()
	dimensions := grid.GetDimensions()

	// Get room bounds (simplified - assumes room starts at origin)
	roomWidth := dimensions.Width
	roomHeight := dimensions.Height

	// Check if the query circle intersects with the room rectangle
	// Simple bounding box intersection check
	// TODO: Implement more sophisticated spatial intersection
	if center.X+radius < 0 || center.X-radius > roomWidth ||
		center.Y+radius < 0 || center.Y-radius > roomHeight {
		return false
	}
	return true
}

func (h *BasicQueryHandler) isPositionInRoomUnsafe(position spatial.Position, room spatial.Room) bool {
	// Check if position is within room bounds
	grid := room.GetGrid()

	// Use the grid's built-in position validation
	return grid.IsValidPosition(position)
}

// Event publishing

func (h *BasicQueryHandler) publishQueryEventUnsafe(ctx context.Context, queryType string, query interface{}) {
	if h.eventBus != nil {
		event := events.NewGameEvent(EventEnvironmentQueryExecuted, nil, nil)
		event.Context().Set("query_type", queryType)
		event.Context().Set("query", query)
		_ = h.eventBus.Publish(ctx, event)
	}
}
