package spatial

import (
	"github.com/KirkDiggler/rpg-toolkit/core"
	"github.com/KirkDiggler/rpg-toolkit/events"
)

// GridShape represents the type of grid system
type GridShape int

const (
	GridShapeSquare GridShape = iota
	GridShapeHex
	GridShapeGridless
)

// Grid defines the interface for all grid systems
type Grid interface {
	// GetShape returns the grid type
	GetShape() GridShape
	
	// IsValidPosition checks if a position is valid within the grid
	IsValidPosition(pos Position) bool
	
	// GetDimensions returns the grid dimensions
	GetDimensions() Dimensions
	
	// Distance calculates the distance between two positions
	Distance(from, to Position) float64
	
	// GetNeighbors returns all adjacent positions
	GetNeighbors(pos Position) []Position
	
	// IsAdjacent checks if two positions are adjacent
	IsAdjacent(pos1, pos2 Position) bool
	
	// GetLineOfSight returns positions along the line of sight
	GetLineOfSight(from, to Position) []Position
	
	// GetPositionsInRange returns all positions within a given range
	GetPositionsInRange(center Position, radius float64) []Position
}

// Room defines the interface for spatial containers
type Room interface {
	core.Entity
	
	// GetGrid returns the grid system used by this room
	GetGrid() Grid
	
	// PlaceEntity places an entity at a specific position
	PlaceEntity(entity core.Entity, pos Position) error
	
	// MoveEntity moves an entity to a new position
	MoveEntity(entityID string, newPos Position) error
	
	// RemoveEntity removes an entity from the room
	RemoveEntity(entityID string) error
	
	// GetEntitiesAt returns all entities at a specific position
	GetEntitiesAt(pos Position) []core.Entity
	
	// GetEntityPosition returns the position of an entity
	GetEntityPosition(entityID string) (Position, bool)
	
	// GetAllEntities returns all entities in the room
	GetAllEntities() map[string]core.Entity
	
	// GetEntitiesInRange returns entities within a given range
	GetEntitiesInRange(center Position, radius float64) []core.Entity
	
	// IsPositionOccupied checks if a position is occupied
	IsPositionOccupied(pos Position) bool
	
	// CanPlaceEntity checks if an entity can be placed at a position
	CanPlaceEntity(entity core.Entity, pos Position) bool
	
	// GetPositionsInRange returns all positions within a given range
	GetPositionsInRange(center Position, radius float64) []Position
	
	// GetLineOfSight returns positions along the line of sight
	GetLineOfSight(from, to Position) []Position
	
	// IsLineOfSightBlocked checks if line of sight is blocked by entities
	IsLineOfSightBlocked(from, to Position) bool
}

// Placeable defines the interface for entities that can be placed spatially
type Placeable interface {
	core.Entity
	
	// GetSize returns the size of the entity (for multi-space entities)
	GetSize() int
	
	// BlocksMovement returns true if the entity blocks movement
	BlocksMovement() bool
	
	// BlocksLineOfSight returns true if the entity blocks line of sight
	BlocksLineOfSight() bool
}

// QueryHandler defines the interface for spatial query processing
type QueryHandler interface {
	// ProcessQuery processes a spatial query and returns results
	ProcessQuery(query Query) (QueryResult, error)
}

// Query represents a spatial query
type Query interface {
	// GetType returns the query type
	GetType() string
	
	// GetRoom returns the room being queried
	GetRoom() Room
	
	// GetCenter returns the center position for the query
	GetCenter() Position
	
	// GetRadius returns the radius for range-based queries
	GetRadius() float64
	
	// GetFilter returns any entity filter for the query
	GetFilter() EntityFilter
}

// QueryResult represents the result of a spatial query
type QueryResult interface {
	// GetPositions returns positions that match the query
	GetPositions() []Position
	
	// GetEntities returns entities that match the query
	GetEntities() []core.Entity
	
	// GetDistances returns distances for each result
	GetDistances() map[string]float64
}

// EntityFilter defines filtering criteria for spatial queries
type EntityFilter interface {
	// Matches returns true if the entity matches the filter
	Matches(entity core.Entity) bool
}

// EventBusIntegration defines how the spatial module integrates with the event bus
type EventBusIntegration interface {
	// SetEventBus sets the event bus for the spatial module
	SetEventBus(bus events.EventBus)
	
	// GetEventBus returns the current event bus
	GetEventBus() events.EventBus
}