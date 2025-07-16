// Package spatial provides 2D spatial positioning and movement capabilities for RPG games.
package spatial

import (
	"github.com/KirkDiggler/rpg-toolkit/core"
	"github.com/KirkDiggler/rpg-toolkit/events"
)

// Spatial event constants following the toolkit's dot notation pattern
const (
	// Entity placement events
	EventEntityPlaced  = "spatial.entity.placed"
	EventEntityMoved   = "spatial.entity.moved"
	EventEntityRemoved = "spatial.entity.removed"

	// Room events
	EventRoomCreated = "spatial.room.created"

	// Query events
	EventQueryPositionsInRange = "spatial.query.positions_in_range"
	EventQueryEntitiesInRange  = "spatial.query.entities_in_range"
	EventQueryLineOfSight      = "spatial.query.line_of_sight"
	EventQueryMovement         = "spatial.query.movement"
	EventQueryPlacement        = "spatial.query.placement"
)

// EntityPlacedData contains data for entity placement events
type EntityPlacedData struct {
	Entity   core.Entity `json:"entity"`
	Position Position    `json:"position"`
	RoomID   string      `json:"room_id"`
}

// EntityMovedData contains data for entity movement events
type EntityMovedData struct {
	Entity      core.Entity `json:"entity"`
	OldPosition Position    `json:"old_position"`
	NewPosition Position    `json:"new_position"`
	RoomID      string      `json:"room_id"`
}

// EntityRemovedData contains data for entity removal events
type EntityRemovedData struct {
	Entity   core.Entity `json:"entity"`
	Position Position    `json:"position"`
	RoomID   string      `json:"room_id"`
}

// RoomCreatedData contains data for room creation events
type RoomCreatedData struct {
	RoomID   string          `json:"room_id"`
	Grid     Grid            `json:"grid"`
	EventBus events.EventBus `json:"-"` // Don't serialize event bus
}

// QueryPositionsInRangeData contains data for position range queries
type QueryPositionsInRangeData struct {
	Center  Position   `json:"center"`
	Radius  float64    `json:"radius"`
	RoomID  string     `json:"room_id"`
	Results []Position `json:"results,omitempty"`
	Error   error      `json:"error,omitempty"`
}

// QueryEntitiesInRangeData contains data for entity range queries
type QueryEntitiesInRangeData struct {
	Center  Position      `json:"center"`
	Radius  float64       `json:"radius"`
	RoomID  string        `json:"room_id"`
	Filter  EntityFilter  `json:"filter,omitempty"`
	Results []core.Entity `json:"results,omitempty"`
	Error   error         `json:"error,omitempty"`
}

// QueryLineOfSightData contains data for line of sight queries
type QueryLineOfSightData struct {
	From    Position   `json:"from"`
	To      Position   `json:"to"`
	RoomID  string     `json:"room_id"`
	Results []Position `json:"results,omitempty"`
	Blocked bool       `json:"blocked,omitempty"`
	Error   error      `json:"error,omitempty"`
}

// QueryMovementData contains data for movement queries
type QueryMovementData struct {
	Entity   core.Entity `json:"entity"`
	From     Position    `json:"from"`
	To       Position    `json:"to"`
	RoomID   string      `json:"room_id"`
	Valid    bool        `json:"valid,omitempty"`
	Path     []Position  `json:"path,omitempty"`
	Distance float64     `json:"distance,omitempty"`
	Error    error       `json:"error,omitempty"`
}

// QueryPlacementData contains data for placement queries
type QueryPlacementData struct {
	Entity   core.Entity `json:"entity"`
	Position Position    `json:"position"`
	RoomID   string      `json:"room_id"`
	Valid    bool        `json:"valid,omitempty"`
	Error    error       `json:"error,omitempty"`
}

// SimpleEntityFilter implements basic entity filtering
type SimpleEntityFilter struct {
	EntityTypes []string `json:"entity_types,omitempty"`
	EntityIDs   []string `json:"entity_ids,omitempty"`
	ExcludeIDs  []string `json:"exclude_ids,omitempty"`
}

// Matches checks if an entity matches the filter criteria
func (f *SimpleEntityFilter) Matches(entity core.Entity) bool {
	if entity == nil {
		return false
	}

	// Check exclusions first
	for _, excludeID := range f.ExcludeIDs {
		if entity.GetID() == excludeID {
			return false
		}
	}

	// Check specific entity IDs
	if len(f.EntityIDs) > 0 {
		for _, id := range f.EntityIDs {
			if entity.GetID() == id {
				return true
			}
		}
		return false
	}

	// Check entity types
	if len(f.EntityTypes) > 0 {
		for _, entityType := range f.EntityTypes {
			if entity.GetType() == entityType {
				return true
			}
		}
		return false
	}

	// No filters specified, match all
	return true
}

// NewSimpleEntityFilter creates a new simple entity filter
func NewSimpleEntityFilter() *SimpleEntityFilter {
	return &SimpleEntityFilter{
		EntityTypes: make([]string, 0),
		EntityIDs:   make([]string, 0),
		ExcludeIDs:  make([]string, 0),
	}
}

// WithEntityTypes adds entity type filtering
func (f *SimpleEntityFilter) WithEntityTypes(types ...string) *SimpleEntityFilter {
	f.EntityTypes = append(f.EntityTypes, types...)
	return f
}

// WithEntityIDs adds entity ID filtering
func (f *SimpleEntityFilter) WithEntityIDs(ids ...string) *SimpleEntityFilter {
	f.EntityIDs = append(f.EntityIDs, ids...)
	return f
}

// WithExcludeIDs adds entity ID exclusion
func (f *SimpleEntityFilter) WithExcludeIDs(ids ...string) *SimpleEntityFilter {
	f.ExcludeIDs = append(f.ExcludeIDs, ids...)
	return f
}
