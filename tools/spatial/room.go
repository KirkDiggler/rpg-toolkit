package spatial

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/KirkDiggler/rpg-toolkit/core"
	"github.com/KirkDiggler/rpg-toolkit/events"
)

// BasicRoom implements the Room interface with event integration
type BasicRoom struct {
	id       string
	roomType string
	grid     Grid

	// Type-safe event publishers (replaces eventBus events.EventBus)
	entityPlacements events.TypedTopic[EntityPlacedEvent]
	entityMovements  events.TypedTopic[EntityMovedEvent]
	entityRemovals   events.TypedTopic[EntityRemovedEvent]
	roomCreated      events.TypedTopic[RoomCreatedEvent]

	// Triple entity tracking for efficient lookups
	entities   map[string]core.Entity // ID -> Entity
	positions  map[string]Position    // ID -> Position
	occupancy  map[Position][]string  // Position -> []EntityID
	boundaries map[boundaryKey]Boundary

	// Mutex for thread-safe access
	mutex sync.RWMutex
}

// BasicRoom provides optional boundary-aware behavior without changing the
// legacy Room interface.
var _ BoundaryAwareRoom = (*BasicRoom)(nil)

// BasicRoomConfig holds configuration for creating a basic room
type BasicRoomConfig struct {
	ID   string
	Type string
	Grid Grid
	// EventBus removed - use ConnectToEventBus() method after creation
}

// NewBasicRoom creates a new basic room (call ConnectToEventBus after creation)
func NewBasicRoom(config BasicRoomConfig) *BasicRoom {
	room := &BasicRoom{
		id:       config.ID,
		roomType: config.Type,
		grid:     config.Grid,
		// Event topics will be connected via ConnectToEventBus()
		entities:   make(map[string]core.Entity),
		positions:  make(map[string]Position),
		occupancy:  make(map[Position][]string),
		boundaries: make(map[boundaryKey]Boundary),
	}

	return room
}

// ConnectToEventBus connects the room to an event bus for typed event publishing
func (r *BasicRoom) ConnectToEventBus(bus events.EventBus) {
	r.entityPlacements = EntityPlacedTopic.On(bus)
	r.entityMovements = EntityMovedTopic.On(bus)
	r.entityRemovals = EntityRemovedTopic.On(bus)
	r.roomCreated = RoomCreatedTopic.On(bus)

	// Now emit room creation event since we're connected
	if r.roomCreated != nil {
		dimensions := r.grid.GetDimensions()
		_ = r.roomCreated.Publish(context.Background(), RoomCreatedEvent{
			RoomID:       r.id,
			RoomType:     r.roomType,
			GridType:     gridShapeToString(r.grid.GetShape()),
			Width:        int(dimensions.Width),
			Height:       int(dimensions.Height),
			CreationTime: time.Now(),
		})
	}
}

// GetID returns the room's unique identifier (implements core.Entity)
func (r *BasicRoom) GetID() string {
	return r.id
}

// GetType returns the room's type (implements core.Entity)
func (r *BasicRoom) GetType() core.EntityType {
	return core.EntityType(r.roomType)
}

// GetGrid returns the grid system used by this room
func (r *BasicRoom) GetGrid() Grid {
	return r.grid
}

// Note: SetEventBus/GetEventBus methods removed - use ConnectToEventBus() instead

// PlaceEntity places an entity at a specific position
func (r *BasicRoom) PlaceEntity(entity core.Entity, pos Position) error {
	if entity == nil {
		return fmt.Errorf("entity cannot be nil")
	}

	r.mutex.Lock()
	defer r.mutex.Unlock()

	// Check if position is valid
	if !r.grid.IsValidPosition(pos) {
		return fmt.Errorf("position %v is not valid for this room", pos)
	}

	// Check if entity can be placed at this position
	if !r.canPlaceEntityUnsafe(entity, pos) {
		return fmt.Errorf("entity %s cannot be placed at position %v", entity.GetID(), pos)
	}

	// Remove entity from old position if it exists
	if oldPos, exists := r.positions[entity.GetID()]; exists {
		r.removeFromOccupancyUnsafe(entity.GetID(), oldPos)
	}

	// Add entity to new position
	r.entities[entity.GetID()] = entity
	r.positions[entity.GetID()] = pos
	r.addToOccupancyUnsafe(entity.GetID(), pos)

	// Emit placement event
	if r.entityPlacements != nil {
		_ = r.entityPlacements.Publish(context.Background(), EntityPlacedEvent{
			EntityID:     entity.GetID(),
			Position:     pos,
			CubePosition: r.getCubePosition(pos),
			RoomID:       r.id,
			GridType:     gridShapeToString(r.grid.GetShape()),
		})
	}

	return nil
}

// MoveEntity moves an entity to a new position
func (r *BasicRoom) MoveEntity(entityID string, newPos Position) error {
	r.mutex.Lock()
	defer r.mutex.Unlock()

	// Check if entity exists
	entity, exists := r.entities[entityID]
	if !exists {
		return fmt.Errorf("entity %s not found in room", entityID)
	}

	// Get current position
	oldPos, exists := r.positions[entityID]
	if !exists {
		return fmt.Errorf("entity %s has no position in room", entityID)
	}

	// Check if new position is valid
	if !r.grid.IsValidPosition(newPos) {
		return fmt.Errorf("position %v is not valid for this room", newPos)
	}

	// Check if entity can be placed at new position
	if !r.canPlaceEntityUnsafe(entity, newPos) {
		return fmt.Errorf("entity %s cannot be moved to position %v", entityID, newPos)
	}

	if r.isDirectMovementBoundaryBlockedUnsafe(oldPos, newPos) {
		return fmt.Errorf("entity %s cannot cross movement-blocking boundary from %v to %v", entityID, oldPos, newPos)
	}

	// Update positions
	r.removeFromOccupancyUnsafe(entityID, oldPos)
	r.positions[entityID] = newPos
	r.addToOccupancyUnsafe(entityID, newPos)

	// Emit movement event
	if r.entityMovements != nil {
		_ = r.entityMovements.Publish(context.Background(), EntityMovedEvent{
			EntityID:         entity.GetID(),
			FromPosition:     oldPos,
			ToPosition:       newPos,
			FromCubePosition: r.getCubePosition(oldPos),
			ToCubePosition:   r.getCubePosition(newPos),
			RoomID:           r.id,
			MovementType:     "normal", // Could be "teleport", "forced" based on context
		})
	}

	return nil
}

// RemoveEntity removes an entity from the room
func (r *BasicRoom) RemoveEntity(entityID string) error {
	r.mutex.Lock()
	defer r.mutex.Unlock()

	// Check if entity exists
	entity, exists := r.entities[entityID]
	if !exists {
		return fmt.Errorf("entity %s not found in room", entityID)
	}

	// Get current position
	pos, exists := r.positions[entityID]
	if !exists {
		return fmt.Errorf("entity %s has no position in room", entityID)
	}

	// Remove entity
	delete(r.entities, entityID)
	delete(r.positions, entityID)
	r.removeFromOccupancyUnsafe(entityID, pos)

	// Emit removal event
	if r.entityRemovals != nil {
		_ = r.entityRemovals.Publish(context.Background(), EntityRemovedEvent{
			EntityID:    entity.GetID(),
			Position:    pos,
			RoomID:      r.id,
			RemovalType: "normal", // Could be "destroyed", "teleported" based on context
		})
	}

	return nil
}

// RegisterBoundary validates and registers an undirected boundary between two
// adjacent in-grid positions. Endpoint order is normalized and registering the
// same pair replaces its blocking flags.
func (r *BasicRoom) RegisterBoundary(boundary Boundary) error {
	r.mutex.Lock()
	defer r.mutex.Unlock()

	normalized, err := r.validateAndNormalizeBoundaryUnsafe(boundary)
	if err != nil {
		return err
	}
	r.boundaries[newBoundaryKey(normalized.From, normalized.To)] = normalized
	return nil
}

// RemoveBoundary validates an undirected boundary pair and removes it.
// Removing an otherwise-valid pair with no registered boundary is a no-op.
func (r *BasicRoom) RemoveBoundary(from, to Position) error {
	r.mutex.Lock()
	defer r.mutex.Unlock()

	normalized, err := r.validateAndNormalizeBoundaryUnsafe(Boundary{From: from, To: to})
	if err != nil {
		return err
	}
	delete(r.boundaries, newBoundaryKey(normalized.From, normalized.To))
	return nil
}

// GetBoundary returns the normalized boundary for an endpoint pair regardless
// of the direction passed by the caller.
func (r *BasicRoom) GetBoundary(from, to Position) (Boundary, bool) {
	r.mutex.RLock()
	defer r.mutex.RUnlock()

	boundary, exists := r.boundaries[newBoundaryKey(from, to)]
	return boundary, exists
}

// IsBoundaryMovementBlocked reports whether a registered boundary blocks the
// crossing between two positions.
func (r *BasicRoom) IsBoundaryMovementBlocked(from, to Position) bool {
	r.mutex.RLock()
	defer r.mutex.RUnlock()

	return r.isBoundaryMovementBlockedUnsafe(from, to)
}

// IsBoundaryLineOfSightBlocked reports whether a registered boundary blocks
// line of sight across the crossing between two positions.
func (r *BasicRoom) IsBoundaryLineOfSightBlocked(from, to Position) bool {
	r.mutex.RLock()
	defer r.mutex.RUnlock()

	return r.isBoundaryLineOfSightBlockedUnsafe(from, to)
}

func (r *BasicRoom) validateAndNormalizeBoundaryUnsafe(boundary Boundary) (Boundary, error) {
	if r.grid.GetShape() == GridShapeGridless {
		return Boundary{}, fmt.Errorf("boundaries require a discrete grid")
	}
	if !isDiscretePosition(boundary.From) || !isDiscretePosition(boundary.To) {
		return Boundary{}, fmt.Errorf("boundary endpoints must be finite discrete cell positions")
	}
	if !r.grid.IsValidPosition(boundary.From) || !r.grid.IsValidPosition(boundary.To) {
		return Boundary{}, fmt.Errorf(
			"boundary endpoints %v and %v must be valid positions in this room", boundary.From, boundary.To,
		)
	}
	if boundary.From.Equals(boundary.To) {
		return Boundary{}, fmt.Errorf("boundary endpoints must be distinct")
	}
	if !r.grid.IsAdjacent(boundary.From, boundary.To) {
		return Boundary{}, fmt.Errorf("boundary endpoints %v and %v must be adjacent", boundary.From, boundary.To)
	}
	return normalizedBoundary(boundary), nil
}

func (r *BasicRoom) isBoundaryMovementBlockedUnsafe(from, to Position) bool {
	boundary, exists := r.boundaries[newBoundaryKey(from, to)]
	return exists && boundary.BlocksMovement
}

// isDirectMovementBoundaryBlockedUnsafe checks every crossing in the direct
// ray supplied by the grid. It deliberately does not find an alternate route:
// MoveEntity's established multi-cell behavior is a direct move.
func (r *BasicRoom) isDirectMovementBoundaryBlockedUnsafe(from, to Position) bool {
	path := r.grid.GetLineOfSight(from, to)
	for i := 1; i < len(path); i++ {
		if r.isBoundaryMovementBlockedUnsafe(path[i-1], path[i]) {
			return true
		}
	}
	return false
}

func (r *BasicRoom) isBoundaryLineOfSightBlockedUnsafe(from, to Position) bool {
	boundary, exists := r.boundaries[newBoundaryKey(from, to)]
	return exists && boundary.BlocksLineOfSight
}

// isDirectLineOfSightBoundaryBlockedUnsafe checks boundary crossings on a
// canonical grid ray. Square Bresenham can choose different cells by direction,
// so canonical endpoint ordering makes boundary LoS reciprocal. Entity blockers
// intentionally continue to use the caller's requested ray.
func (r *BasicRoom) isDirectLineOfSightBoundaryBlockedUnsafe(from, to Position) bool {
	if positionLess(to, from) {
		from, to = to, from
	}
	path := r.grid.GetLineOfSight(from, to)
	for i := 1; i < len(path); i++ {
		if r.isBoundaryLineOfSightBlockedUnsafe(path[i-1], path[i]) {
			return true
		}
	}
	return false
}

// GetEntitiesAt returns all entities at a specific position
func (r *BasicRoom) GetEntitiesAt(pos Position) []core.Entity {
	r.mutex.RLock()
	defer r.mutex.RUnlock()

	entityIDs, exists := r.occupancy[pos]
	if !exists {
		return []core.Entity{}
	}

	entities := make([]core.Entity, 0, len(entityIDs))
	for _, entityID := range entityIDs {
		if entity, exists := r.entities[entityID]; exists {
			entities = append(entities, entity)
		}
	}

	return entities
}

// GetEntityPosition returns the position of an entity
func (r *BasicRoom) GetEntityPosition(entityID string) (Position, bool) {
	r.mutex.RLock()
	defer r.mutex.RUnlock()

	pos, exists := r.positions[entityID]
	return pos, exists
}

// GetEntityCubePosition returns the cube coordinate position of an entity
// Returns nil if the entity doesn't exist or the grid is not a hex grid
func (r *BasicRoom) GetEntityCubePosition(entityID string) *CubeCoordinate {
	r.mutex.RLock()
	defer r.mutex.RUnlock()

	pos, exists := r.positions[entityID]
	if !exists {
		return nil
	}
	return r.getCubePosition(pos)
}

// GetAllEntities returns all entities in the room
func (r *BasicRoom) GetAllEntities() map[string]core.Entity {
	r.mutex.RLock()
	defer r.mutex.RUnlock()

	// Create a copy to avoid concurrent access issues
	entities := make(map[string]core.Entity, len(r.entities))
	for id, entity := range r.entities {
		entities[id] = entity
	}

	return entities
}

// GetEntitiesInRange returns entities within a given range
func (r *BasicRoom) GetEntitiesInRange(center Position, radius float64) []core.Entity {
	r.mutex.RLock()
	defer r.mutex.RUnlock()

	entities := make([]core.Entity, 0)

	for entityID, pos := range r.positions {
		if r.grid.Distance(center, pos) <= radius {
			if entity, exists := r.entities[entityID]; exists {
				entities = append(entities, entity)
			}
		}
	}

	return entities
}

// IsPositionOccupied checks if a position is occupied
func (r *BasicRoom) IsPositionOccupied(pos Position) bool {
	r.mutex.RLock()
	defer r.mutex.RUnlock()

	entityIDs, exists := r.occupancy[pos]
	return exists && len(entityIDs) > 0
}

// CanPlaceEntity checks if an entity can be placed at a position
func (r *BasicRoom) CanPlaceEntity(entity core.Entity, pos Position) bool {
	r.mutex.RLock()
	defer r.mutex.RUnlock()

	return r.canPlaceEntityUnsafe(entity, pos)
}

// canPlaceEntityUnsafe checks if an entity can be placed (without locking)
func (r *BasicRoom) canPlaceEntityUnsafe(entity core.Entity, pos Position) bool {
	// Check if position is valid
	if !r.grid.IsValidPosition(pos) {
		return false
	}

	// Check if position is occupied by other entities
	if entityIDs, exists := r.occupancy[pos]; exists {
		for _, entityID := range entityIDs {
			// Allow placement if it's the same entity (for movement)
			if entityID != entity.GetID() {
				// Check if the existing entity blocks placement
				if existingEntity, exists := r.entities[entityID]; exists {
					if placeable, ok := existingEntity.(Placeable); ok {
						if placeable.BlocksMovement() {
							return false
						}
					}
				}
			}
		}
	}

	return true
}

// addToOccupancyUnsafe adds an entity to the occupancy map (without locking)
func (r *BasicRoom) addToOccupancyUnsafe(entityID string, pos Position) {
	if _, exists := r.occupancy[pos]; !exists {
		r.occupancy[pos] = make([]string, 0)
	}
	r.occupancy[pos] = append(r.occupancy[pos], entityID)
}

// removeFromOccupancyUnsafe removes an entity from the occupancy map (without locking)
func (r *BasicRoom) removeFromOccupancyUnsafe(entityID string, pos Position) {
	if entityIDs, exists := r.occupancy[pos]; exists {
		for i, id := range entityIDs {
			if id == entityID {
				// Remove from slice
				r.occupancy[pos] = append(entityIDs[:i], entityIDs[i+1:]...)
				break
			}
		}

		// Remove position from map if no entities remain
		if len(r.occupancy[pos]) == 0 {
			delete(r.occupancy, pos)
		}
	}
}

// GetPositionsInRange returns all positions within a given range
func (r *BasicRoom) GetPositionsInRange(center Position, radius float64) []Position {
	return r.grid.GetPositionsInRange(center, radius)
}

// GetLineOfSight returns positions along the line of sight
func (r *BasicRoom) GetLineOfSight(from, to Position) []Position {
	return r.grid.GetLineOfSight(from, to)
}

// IsLineOfSightBlocked checks if line of sight is blocked by entities
func (r *BasicRoom) IsLineOfSightBlocked(from, to Position) bool {
	r.mutex.RLock()
	defer r.mutex.RUnlock()

	losPositions := r.grid.GetLineOfSight(from, to)

	if r.isDirectLineOfSightBoundaryBlockedUnsafe(from, to) {
		return true
	}

	// Check each position along the caller's requested line of sight (except
	// start and end). Keeping this ray preserves established entity-blocker
	// semantics even when the canonical boundary ray differs on square grids.
	for i := 1; i < len(losPositions)-1; i++ {
		pos := losPositions[i]
		if entityIDs, exists := r.occupancy[pos]; exists {
			for _, entityID := range entityIDs {
				if entity, exists := r.entities[entityID]; exists {
					if placeable, ok := entity.(Placeable); ok {
						if placeable.BlocksLineOfSight() {
							return true
						}
					}
				}
			}
		}
	}

	return false
}

// GetEntityCount returns the number of entities in the room
func (r *BasicRoom) GetEntityCount() int {
	r.mutex.RLock()
	defer r.mutex.RUnlock()

	return len(r.entities)
}

// GetOccupiedPositions returns all positions that have entities
func (r *BasicRoom) GetOccupiedPositions() []Position {
	r.mutex.RLock()
	defer r.mutex.RUnlock()

	positions := make([]Position, 0, len(r.occupancy))
	for pos := range r.occupancy {
		positions = append(positions, pos)
	}

	return positions
}

// gridShapeToString converts GridShape to string for events
func gridShapeToString(shape GridShape) string {
	switch shape {
	case GridShapeSquare:
		return "square"
	case GridShapeHex:
		return "hex"
	case GridShapeGridless:
		return "gridless"
	default:
		return "unknown"
	}
}

// getCubePosition returns the cube coordinate for a position if the grid is a hex grid
// Returns nil for non-hex grids
func (r *BasicRoom) getCubePosition(pos Position) *CubeCoordinate {
	if hexGrid, ok := r.grid.(*HexGrid); ok {
		cube := hexGrid.OffsetToCube(pos)
		return &cube
	}
	return nil
}
