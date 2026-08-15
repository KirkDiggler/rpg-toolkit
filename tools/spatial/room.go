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

// hasRegisteredBoundaries reports whether this room currently has boundary
// records. It is intentionally internal because BoundaryAwareRoom remains
// source-compatible with existing external implementations.
func (r *BasicRoom) hasRegisteredBoundaries() bool {
	r.mutex.RLock()
	defer r.mutex.RUnlock()

	return len(r.boundaries) > 0
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
	if len(r.boundaries) == 0 {
		return false
	}

	path := CanonicalBoundaryRay(r.grid, from, to)
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

// IsLineOfSightBlocked reports whether sight between two cells is blocked.
//
// SIGHT IS NOT ONE LINE. A single centre-to-centre ray was the rule here until
// rpg-toolkit#1022, and it was wrong in two ways that turned out to be one:
// squares disagreed with themselves by direction (Bresenham steps X first one
// way and Y first the other, so A→B and B→A are different cells), and every
// grid family blocked far more than the game's own rule allows. Measured
// against 5e's stated test — you can see a target if a line from ANY corner of
// your space to ANY corner of theirs is unobstructed — the old rule blocked
// 3.6x too many pairs on squares and 4.5x too many on hexes, and hid about one
// in seven things a player should have been able to see. It never blocked too
// little; it was uniformly stricter than the game.
//
// So sight asks for a LANE rather than a line: blocked only when the direct
// lane is obstructed AND so is every lane from a neighbouring cell that does
// not give ground. Corner-clipping stops costing you the whole sightline,
// which is what a player at the table already assumes.
//
// This reaches the corner rule exactly on squares and within 0.01% of it on
// hexes. The remaining gap is the price of staying grid-native: the corner rule
// itself needs cell-polygon geometry and a plane embedding, which this module
// does not have — see [BasicRoom.lineOfSightLaneBlockedUnsafe] for why that is
// the endpoint rather than the answer today.
//
// SYMMETRY IS STRUCTURAL, not incidental: every lane is rasterized on the
// canonical ray, and the neighbour lanes are explored from both ends, so the
// rule has no direction left to disagree about. It is pinned as a law over
// fuzzed rooms in every grid family.
//
// The common case costs exactly what it used to. A pair whose direct lane is
// clear returns on that first test, and most pairs are clear — the extra work
// lands only where the answer used to be wrong. That matters because callers
// run this O(range²) per viewer.
func (r *BasicRoom) IsLineOfSightBlocked(from, to Position) bool {
	r.mutex.RLock()
	defer r.mutex.RUnlock()

	// A BOUNDARY IS AN EDGE AND STAYS A HARD BLOCK. Neighbour lanes model
	// leaning around something that has extent — an occluding cell is a pillar
	// or a wall block, and a viewer really can look past its corner. A boundary
	// is a wall drawn ON the edge between two cells, with no extent in this
	// model and no stated length, so "around it" is not a thing the data
	// describes. Softening it would rewrite a primitive nobody reported, and
	// rpg-toolkit#1022 is about occluders: the wall cells a player watches
	// swallow their sightline.
	if r.boundaryBlocksSightUnsafe(from, to) {
		return true
	}

	if !r.lineOfSightLaneBlockedUnsafe(from, to) {
		return false
	}

	// GRIDLESS KEEPS THE SINGLE LANE, for the same reason boundaries do.
	// Neighbour lanes model a CELL'S EXTENT — a square or a hex tiles the
	// plane, so a viewer really can look past its corner. A gridless position
	// is a point in continuous space with no cell around it and no neighbours
	// of its own; what GetNeighbors offers there is eight samples on a unit
	// circle, an arbitrary distance that means one thing in a ten-foot room
	// and nothing at all in a mile-wide one. Leaning by an arbitrary amount is
	// not the rule this fixes.
	if r.grid.GetShape() == GridShapeGridless {
		return true
	}

	// The direct lane is obstructed. Sight survives if any neighbour of either
	// end has a clear lane — that is the corner a player would lean around.
	//
	// A neighbour must MAKE PROGRESS: strictly closer to the other end than the
	// cell itself. Merely "no further" was tried and measured worse on every
	// grid family — it turns leaning into wandering, letting sight recover from
	// a vantage a full cell sideways, and on squares it doubled the pairs seen
	// that the game's corner rule denies. Progress keeps the alternative on the
	// way to the target rather than beside it.
	distance := r.grid.Distance(from, to)
	for _, alt := range r.grid.GetNeighbors(from) {
		if r.blocksLineOfSightUnsafe(alt) || r.grid.Distance(alt, to) >= distance {
			continue
		}
		if !r.lineOfSightLaneBlockedUnsafe(alt, to) {
			return false
		}
	}
	for _, alt := range r.grid.GetNeighbors(to) {
		if r.blocksLineOfSightUnsafe(alt) || r.grid.Distance(from, alt) >= distance {
			continue
		}
		if !r.lineOfSightLaneBlockedUnsafe(from, alt) {
			return false
		}
	}

	return true
}

// boundaryBlocksSightUnsafe reports whether the canonical ray between two cells
// crosses a sight-blocking boundary.
//
// Split out from the lane test because the two are not the same kind of
// obstacle: this one is absolute, and the lane test is what neighbour lanes are
// allowed to route around. Both rasterize the same canonical ray, so a boundary
// answer never depends on which end asked — that was already true before
// rpg-toolkit#1022 and is unchanged by it.
func (r *BasicRoom) boundaryBlocksSightUnsafe(from, to Position) bool {
	if len(r.boundaries) == 0 {
		return false
	}
	path := CanonicalBoundaryRay(r.grid, from, to)
	for i := 1; i < len(path); i++ {
		if r.isBoundaryLineOfSightBlockedUnsafe(path[i-1], path[i]) {
			return true
		}
	}
	return false
}

// lineOfSightLaneBlockedUnsafe reports whether ONE lane between two cells is
// obstructed, by a boundary it crosses or by something standing in it.
//
// It rasterizes the canonical ray for BOTH checks. Until rpg-toolkit#1022 a
// single call consulted two different rays — [CanonicalBoundaryRay] for
// boundaries, the caller's own ray for entities — and the second of those was
// the direction-dependence this issue is named for. One lane, one ray.
//
// The endpoints are never opaque: you are not blocked by the cell you stand in
// or the one you are looking at.
//
// THE ENDPOINT THIS APPROXIMATES is 5e's corner rule, evaluated as real
// geometry: cell polygons in a plane, and a lane for every corner pair. That
// is exact where this is within 0.01%, and it is what belongs here the day
// this module grows a plane embedding — it has none today, and measured 5x the
// cost per query on hexes, on a path callers already run O(range²) per viewer.
// Recorded so the comparison does not have to be re-derived: rpg-toolkit#1022.
func (r *BasicRoom) lineOfSightLaneBlockedUnsafe(from, to Position) bool {
	if r.boundaryBlocksSightUnsafe(from, to) {
		return true
	}

	path := CanonicalBoundaryRay(r.grid, from, to)
	for i := 1; i < len(path)-1; i++ {
		if r.blocksLineOfSightUnsafe(path[i]) {
			return true
		}
	}

	return false
}

// blocksLineOfSightUnsafe reports whether anything standing in a cell is opaque.
func (r *BasicRoom) blocksLineOfSightUnsafe(pos Position) bool {
	entityIDs, exists := r.occupancy[pos]
	if !exists {
		return false
	}
	for _, entityID := range entityIDs {
		entity, exists := r.entities[entityID]
		if !exists {
			continue
		}
		if placeable, ok := entity.(Placeable); ok && placeable.BlocksLineOfSight() {
			return true
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
