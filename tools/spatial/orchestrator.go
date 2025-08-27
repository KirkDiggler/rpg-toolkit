package spatial

import (
	"github.com/KirkDiggler/rpg-toolkit/core"
)

// ConnectionType represents different types of connections between rooms
type ConnectionType string

// Connection type constants define the various ways rooms can be linked
const (
	ConnectionTypeDoor    ConnectionType = "door"    // Standard doorway connection
	ConnectionTypeStairs  ConnectionType = "stairs"  // Stairway between different levels
	ConnectionTypePassage ConnectionType = "passage" // Corridor or hallway connection
	ConnectionTypePortal  ConnectionType = "portal"  // Magical or teleportation connection
	ConnectionTypeBridge  ConnectionType = "bridge"  // Bridge spanning a gap or obstacle
	ConnectionTypeTunnel  ConnectionType = "tunnel"  // Underground or enclosed tunnel
)

// LayoutType represents different arrangement patterns for multiple rooms
type LayoutType string

// Layout type constants define how multiple rooms can be spatially arranged
const (
	LayoutTypeTower     LayoutType = "tower"     // Vertical stacking arrangement
	LayoutTypeBranching LayoutType = "branching" // Hub and spoke arrangement
	LayoutTypeGrid      LayoutType = "grid"      // 2D grid arrangement
	LayoutTypeOrganic   LayoutType = "organic"   // Irregular organic connections
)

// Connection represents a logical link between two rooms (ADR-0015: Abstract Connections)
type Connection interface {
	core.Entity

	// GetConnectionType returns the connection type
	GetConnectionType() ConnectionType

	// GetFromRoom returns the source room ID
	GetFromRoom() string

	// GetToRoom returns the destination room ID
	GetToRoom() string

	// IsPassable checks if entities can currently traverse this connection
	IsPassable(entity core.Entity) bool

	// GetTraversalCost returns the cost to traverse this connection
	GetTraversalCost(entity core.Entity) float64

	// IsReversible returns true if the connection works both ways
	IsReversible() bool

	// GetRequirements returns any requirements for using this connection
	GetRequirements() []string
}

// RoomOrchestrator manages multiple rooms and their connections
type RoomOrchestrator interface {
	core.Entity
	EventBusIntegration

	// AddRoom adds a room to the orchestrator
	AddRoom(room Room) error

	// RemoveRoom removes a room from the orchestrator
	RemoveRoom(roomID string) error

	// GetRoom retrieves a room by ID
	GetRoom(roomID string) (Room, bool)

	// GetAllRooms returns all managed rooms
	GetAllRooms() map[string]Room

	// AddConnection creates a connection between two rooms
	AddConnection(connection Connection) error

	// RemoveConnection removes a connection
	RemoveConnection(connectionID string) error

	// GetConnection retrieves a connection by ID
	GetConnection(connectionID string) (Connection, bool)

	// GetRoomConnections returns all connections for a specific room
	GetRoomConnections(roomID string) []Connection

	// GetAllConnections returns all connections
	GetAllConnections() map[string]Connection

	// MoveEntityBetweenRooms moves an entity from one room to another
	MoveEntityBetweenRooms(entityID, fromRoom, toRoom, connectionID string) error

	// CanMoveEntityBetweenRooms checks if entity movement is possible
	CanMoveEntityBetweenRooms(entityID, fromRoom, toRoom, connectionID string) bool

	// GetEntityRoom returns which room contains the entity
	GetEntityRoom(entityID string) (string, bool)

	// FindPath finds a path between rooms using connections
	FindPath(fromRoom, toRoom string, entity core.Entity) ([]string, error)

	// GetLayout returns the current layout pattern
	GetLayout() LayoutType

	// SetLayout configures the arrangement pattern
	SetLayout(layout LayoutType) error
}

// LayoutOrchestrator handles spatial arrangement of multiple rooms
type LayoutOrchestrator interface {
	// ArrangeRooms arranges rooms according to the layout pattern
	ArrangeRooms(rooms map[string]Room, layout LayoutType) error

	// CalculateRoomPositions calculates optimal positions for rooms
	CalculateRoomPositions(rooms map[string]Room, connections []Connection) map[string]Position

	// ValidateLayout checks if a layout is structurally sound
	ValidateLayout(rooms map[string]Room, connections []Connection) error

	// GetLayoutMetrics returns metrics about the current layout
	GetLayoutMetrics() LayoutMetrics
}

// LayoutMetrics contains information about the spatial arrangement
type LayoutMetrics struct {
	TotalRooms       int                 `json:"total_rooms"`
	TotalConnections int                 `json:"total_connections"`
	AverageDistance  float64             `json:"average_distance"`
	MaxDistance      float64             `json:"max_distance"`
	Connectivity     float64             `json:"connectivity"` // connections per room
	RoomPositions    map[string]Position `json:"room_positions"`
	LayoutType       LayoutType          `json:"layout_type"`
}

// TransitionSystem handles entity movement between rooms
type TransitionSystem interface {
	// BeginTransition initiates movement between rooms
	BeginTransition(entityID, fromRoom, toRoom, connectionID string) (TransitionID, error)

	// CompleteTransition finalizes the movement
	CompleteTransition(transitionID TransitionID) error

	// CancelTransition cancels an in-progress transition
	CancelTransition(transitionID TransitionID) error

	// GetActiveTransitions returns all in-progress transitions
	GetActiveTransitions() []Transition

	// GetTransition retrieves a specific transition
	GetTransition(transitionID TransitionID) (Transition, bool)
}

// TransitionID uniquely identifies a transition
type TransitionID string

// Transition represents an entity moving between rooms
type Transition interface {
	// GetID returns the transition ID
	GetID() TransitionID

	// GetEntity returns the entity being moved
	GetEntity() core.Entity

	// GetFromRoom returns the source room ID
	GetFromRoom() string

	// GetToRoom returns the destination room ID
	GetToRoom() string

	// GetConnection returns the connection being used
	GetConnection() Connection

	// GetProgress returns completion progress (0.0 to 1.0)
	GetProgress() float64

	// IsComplete returns true if transition is finished
	IsComplete() bool

	// GetStartTime returns when the transition began
	GetStartTime() int64

	// GetEstimatedDuration returns expected transition time
	GetEstimatedDuration() int64
}
