package spatial

import (
	"context"
	"fmt"
	"sync"

	"github.com/KirkDiggler/rpg-toolkit/core"
	"github.com/KirkDiggler/rpg-toolkit/events"
)

// BasicRoom implements the Room interface with event integration
type BasicRoom struct {
	id       string
	roomType string
	grid     Grid
	eventBus events.EventBus

	// Triple entity tracking for efficient lookups
	entities  map[string]core.Entity // ID -> Entity
	positions map[string]Position    // ID -> Position
	occupancy map[Position][]string  // Position -> []EntityID

	// Mutex for thread-safe access
	mutex sync.RWMutex
}

// BasicRoomConfig holds configuration for creating a basic room
type BasicRoomConfig struct {
	ID       string
	Type     string
	Grid     Grid
	EventBus events.EventBus
}

// NewBasicRoom creates a new basic room with event integration
func NewBasicRoom(config BasicRoomConfig) *BasicRoom {
	room := &BasicRoom{
		id:        config.ID,
		roomType:  config.Type,
		grid:      config.Grid,
		eventBus:  config.EventBus,
		entities:  make(map[string]core.Entity),
		positions: make(map[string]Position),
		occupancy: make(map[Position][]string),
	}

	// Emit room creation event
	if room.eventBus != nil {
		event := events.NewGameEvent(EventRoomCreated, nil, nil)
		event.Context().Set("room_id", room.id)
		event.Context().Set("grid", room.grid)
		_ = room.eventBus.Publish(context.Background(), event)
	}

	return room
}

// GetID returns the room's unique identifier (implements core.Entity)
func (r *BasicRoom) GetID() string {
	return r.id
}

// GetType returns the room's type (implements core.Entity)
func (r *BasicRoom) GetType() string {
	return r.roomType
}

// GetGrid returns the grid system used by this room
func (r *BasicRoom) GetGrid() Grid {
	return r.grid
}

// SetEventBus sets the event bus for the room
func (r *BasicRoom) SetEventBus(bus events.EventBus) {
	r.mutex.Lock()
	defer r.mutex.Unlock()
	r.eventBus = bus
}

// GetEventBus returns the current event bus
func (r *BasicRoom) GetEventBus() events.EventBus {
	r.mutex.RLock()
	defer r.mutex.RUnlock()
	return r.eventBus
}

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
	if r.eventBus != nil {
		event := events.NewGameEvent(EventEntityPlaced, entity, nil)
		event.Context().Set("position", pos)
		event.Context().Set("room_id", r.id)
		_ = r.eventBus.Publish(context.Background(), event)
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

	// Update positions
	r.removeFromOccupancyUnsafe(entityID, oldPos)
	r.positions[entityID] = newPos
	r.addToOccupancyUnsafe(entityID, newPos)

	// Emit movement event
	if r.eventBus != nil {
		event := events.NewGameEvent(EventEntityMoved, entity, nil)
		event.Context().Set("old_position", oldPos)
		event.Context().Set("new_position", newPos)
		event.Context().Set("room_id", r.id)
		_ = r.eventBus.Publish(context.Background(), event)
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
	if r.eventBus != nil {
		event := events.NewGameEvent(EventEntityRemoved, entity, nil)
		event.Context().Set("position", pos)
		event.Context().Set("room_id", r.id)
		_ = r.eventBus.Publish(context.Background(), event)
	}

	return nil
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

	// Check each position along the line of sight (except start and end)
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
