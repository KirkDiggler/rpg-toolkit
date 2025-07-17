package spatial

import "github.com/google/uuid"

// RoomID is a unique identifier for a room
type RoomID string

// EntityID is a unique identifier for an entity
type EntityID string

// ConnectionID is a unique identifier for a connection
type ConnectionID string

// OrchestratorID is a unique identifier for an orchestrator
type OrchestratorID string

// NewRoomID generates a new unique room identifier
func NewRoomID() RoomID {
	return RoomID(uuid.New().String())
}

// NewEntityID generates a new unique entity identifier
func NewEntityID() EntityID {
	return EntityID(uuid.New().String())
}

// NewConnectionID generates a new unique connection identifier
func NewConnectionID() ConnectionID {
	return ConnectionID(uuid.New().String())
}

// NewOrchestratorID generates a new unique orchestrator identifier
func NewOrchestratorID() OrchestratorID {
	return OrchestratorID(uuid.New().String())
}

// String conversion methods
func (id RoomID) String() string {
	return string(id)
}

func (id EntityID) String() string {
	return string(id)
}

func (id ConnectionID) String() string {
	return string(id)
}

func (id OrchestratorID) String() string {
	return string(id)
}
