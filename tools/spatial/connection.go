package spatial

import (
	"github.com/KirkDiggler/rpg-toolkit/core"
)

// BasicConnection implements the Connection interface
type BasicConnection struct {
	id           string
	entityType   string
	connType     ConnectionType
	fromRoom     string
	toRoom       string
	fromPosition Position
	toPosition   Position
	reversible   bool
	passable     bool
	cost         float64
	requirements []string
}

// BasicConnectionConfig holds configuration for creating a basic connection
type BasicConnectionConfig struct {
	ID           string
	Type         string
	ConnType     ConnectionType
	FromRoom     string
	ToRoom       string
	FromPosition Position
	ToPosition   Position
	Reversible   bool
	Passable     bool
	Cost         float64
	Requirements []string
}

// NewBasicConnection creates a new basic connection
func NewBasicConnection(config BasicConnectionConfig) *BasicConnection {
	requirements := config.Requirements
	if requirements == nil {
		requirements = make([]string, 0)
	}

	return &BasicConnection{
		id:           config.ID,
		entityType:   config.Type,
		connType:     config.ConnType,
		fromRoom:     config.FromRoom,
		toRoom:       config.ToRoom,
		fromPosition: config.FromPosition,
		toPosition:   config.ToPosition,
		reversible:   config.Reversible,
		passable:     config.Passable,
		cost:         config.Cost,
		requirements: requirements,
	}
}

// GetID returns the connection ID
func (bc *BasicConnection) GetID() string {
	return bc.id
}

// GetType returns the entity type (implementing core.Entity)
func (bc *BasicConnection) GetType() string {
	return bc.entityType
}

// GetConnectionType returns the connection type
func (bc *BasicConnection) GetConnectionType() ConnectionType {
	return bc.connType
}

// GetFromRoom returns the source room ID
func (bc *BasicConnection) GetFromRoom() string {
	return bc.fromRoom
}

// GetToRoom returns the destination room ID
func (bc *BasicConnection) GetToRoom() string {
	return bc.toRoom
}

// GetFromPosition returns the position in the source room
func (bc *BasicConnection) GetFromPosition() Position {
	return bc.fromPosition
}

// GetToPosition returns the position in the destination room
func (bc *BasicConnection) GetToPosition() Position {
	return bc.toPosition
}

// IsPassable checks if entities can currently traverse this connection
func (bc *BasicConnection) IsPassable(_ core.Entity) bool {
	return bc.passable
}

// GetTraversalCost returns the cost to traverse this connection
func (bc *BasicConnection) GetTraversalCost(_ core.Entity) float64 {
	return bc.cost
}

// IsReversible returns true if the connection works both ways
func (bc *BasicConnection) IsReversible() bool {
	return bc.reversible
}

// GetRequirements returns any requirements for using this connection
func (bc *BasicConnection) GetRequirements() []string {
	return bc.requirements
}

// SetPassable changes the passable state of the connection
func (bc *BasicConnection) SetPassable(passable bool) {
	bc.passable = passable
}

// AddRequirement adds a new requirement
func (bc *BasicConnection) AddRequirement(requirement string) {
	bc.requirements = append(bc.requirements, requirement)
}

// RemoveRequirement removes a requirement
func (bc *BasicConnection) RemoveRequirement(requirement string) {
	for i, req := range bc.requirements {
		if req == requirement {
			bc.requirements = append(bc.requirements[:i], bc.requirements[i+1:]...)
			break
		}
	}
}

// HasRequirement checks if a specific requirement exists
func (bc *BasicConnection) HasRequirement(requirement string) bool {
	for _, req := range bc.requirements {
		if req == requirement {
			return true
		}
	}
	return false
}
