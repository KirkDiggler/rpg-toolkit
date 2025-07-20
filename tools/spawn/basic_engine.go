package spawn

import (
	"context"
	"fmt"
	"math/rand"
	"time"

	"github.com/KirkDiggler/rpg-toolkit/core"
	"github.com/KirkDiggler/rpg-toolkit/events"
	"github.com/KirkDiggler/rpg-toolkit/tools/spatial"
)

// BasicSpawnEngine implements SpawnEngine interface
// Phase 1: Core functionality only - scattered spawning with basic validation
type BasicSpawnEngine struct {
	id             string
	spatialHandler spatial.QueryHandler
	selectablesReg SelectablesRegistry
	eventBus       events.EventBus
	enableEvents   bool
	maxAttempts    int
	random         *rand.Rand
}

// BasicSpawnEngineConfig configures a BasicSpawnEngine
type BasicSpawnEngineConfig struct {
	ID             string
	SpatialHandler spatial.QueryHandler
	SelectablesReg SelectablesRegistry
	EventBus       events.EventBus
	EnableEvents   bool
	MaxAttempts    int
}

// NewBasicSpawnEngine creates a new spawn engine
func NewBasicSpawnEngine(config BasicSpawnEngineConfig) *BasicSpawnEngine {
	if config.ID == "" {
		config.ID = "spawn-engine"
	}
	if config.MaxAttempts == 0 {
		config.MaxAttempts = 100
	}

	return &BasicSpawnEngine{
		id:             config.ID,
		spatialHandler: config.SpatialHandler,
		selectablesReg: config.SelectablesReg,
		eventBus:       config.EventBus,
		enableEvents:   config.EnableEvents,
		maxAttempts:    config.MaxAttempts,
		random:         rand.New(rand.NewSource(time.Now().UnixNano())),
	}
}

// PopulateRoom implements SpawnEngine.PopulateRoom
func (e *BasicSpawnEngine) PopulateRoom(ctx context.Context, roomID string, config SpawnConfig) (SpawnResult, error) {
	result := SpawnResult{
		SpawnedEntities: make([]SpawnedEntity, 0),
		Failures:        make([]SpawnFailure, 0),
	}

	if err := e.ValidateSpawnConfig(config); err != nil {
		return result, fmt.Errorf("invalid config: %w", err)
	}

	// Get room from spatial handler
	room, err := e.getRoomFromSpatial(roomID)
	if err != nil {
		return result, fmt.Errorf("failed to get room: %w", err)
	}

	// Process each entity group
	for _, group := range config.EntityGroups {
		entities, err := e.selectEntitiesForGroup(group)
		if err != nil {
			result.Failures = append(result.Failures, SpawnFailure{
				EntityType: group.Type,
				Reason:     fmt.Sprintf("selection failed: %v", err),
			})
			continue
		}

		// Place entities using scattered pattern
		for _, entity := range entities {
			position, err := e.findValidPosition(room, entity)
			if err != nil {
				result.Failures = append(result.Failures, SpawnFailure{
					EntityType: entity.GetType(),
					Reason:     err.Error(),
				})
				continue
			}

			// Place entity in room
			if err := e.placeEntityInRoom(room, entity, position); err != nil {
				result.Failures = append(result.Failures, SpawnFailure{
					EntityType: entity.GetType(),
					Reason:     fmt.Sprintf("placement failed: %v", err),
				})
				continue
			}

			result.SpawnedEntities = append(result.SpawnedEntities, SpawnedEntity{
				Entity:   entity,
				Position: position,
				RoomID:   roomID,
			})

			e.publishEntitySpawnedEvent(ctx, roomID, entity, position)
		}
	}

	result.Success = len(result.SpawnedEntities) > 0
	return result, nil
}

// ValidateSpawnConfig implements SpawnEngine.ValidateSpawnConfig
func (e *BasicSpawnEngine) ValidateSpawnConfig(config SpawnConfig) error {
	if len(config.EntityGroups) == 0 {
		return fmt.Errorf("no entity groups specified")
	}

	if config.Pattern != PatternScattered {
		return fmt.Errorf("only scattered pattern supported in Phase 1")
	}

	for i, group := range config.EntityGroups {
		if group.ID == "" {
			return fmt.Errorf("entity group %d missing ID", i)
		}
		if group.Type == "" {
			return fmt.Errorf("entity group %s missing type", group.ID)
		}
		if group.SelectionTable == "" {
			return fmt.Errorf("entity group %s missing selection table", group.ID)
		}
		if group.Quantity.Fixed == nil {
			return fmt.Errorf("entity group %s missing quantity", group.ID)
		}
		if *group.Quantity.Fixed < 1 {
			return fmt.Errorf("entity group %s quantity must be >= 1", group.ID)
		}
	}

	return nil
}

// selectEntitiesForGroup selects entities from the selectables registry
func (e *BasicSpawnEngine) selectEntitiesForGroup(group EntityGroup) ([]core.Entity, error) {
	if group.Quantity.Fixed == nil {
		return nil, fmt.Errorf("fixed quantity required")
	}

	entities, err := e.selectablesReg.GetEntities(group.SelectionTable, *group.Quantity.Fixed)
	if err != nil {
		return nil, fmt.Errorf("failed to get entities from table %s: %w", group.SelectionTable, err)
	}

	return entities, nil
}

// getRoomFromSpatial gets room from spatial module (mocked for Phase 1)
func (e *BasicSpawnEngine) getRoomFromSpatial(roomID string) (spatial.Room, error) {
	// Phase 1: This would query the spatial handler
	// For now, return a simple interface that we can work with
	return nil, fmt.Errorf("spatial integration not implemented in Phase 1")
}

// findValidPosition finds a valid position for an entity (simplified for Phase 1)
func (e *BasicSpawnEngine) findValidPosition(room spatial.Room, entity core.Entity) (spatial.Position, error) {
	// Phase 1: Simple random position within reasonable bounds
	// Real implementation would query the spatial room for valid positions
	x := e.random.Float64() * 10.0
	y := e.random.Float64() * 10.0

	return spatial.Position{X: x, Y: y}, nil
}

// placeEntityInRoom places entity in the spatial room
func (e *BasicSpawnEngine) placeEntityInRoom(room spatial.Room, entity core.Entity, position spatial.Position) error {
	// Phase 1: This would call room.PlaceEntity(entity, position)
	// For now, just validate the basic operation
	return nil
}

// publishEntitySpawnedEvent publishes entity spawned event
func (e *BasicSpawnEngine) publishEntitySpawnedEvent(ctx context.Context, roomID string, entity core.Entity, position spatial.Position) {
	if !e.enableEvents || e.eventBus == nil {
		return
	}

	// Create a simple room entity for event source
	roomEntity := &SimpleRoomEntity{id: roomID, roomType: "spawn_target"}

	// Publish entity spawned event
	event := events.NewGameEvent("spawn.entity.spawned", roomEntity, nil)
	event.Context().Set("entity_id", entity.GetID())
	event.Context().Set("entity_type", entity.GetType())
	event.Context().Set("position_x", position.X)
	event.Context().Set("position_y", position.Y)
	event.Context().Set("room_id", roomID)

	_ = e.eventBus.Publish(ctx, event)
}

// SimpleRoomEntity implements core.Entity for event publishing
type SimpleRoomEntity struct {
	id       string
	roomType string
}

func (r *SimpleRoomEntity) GetID() string   { return r.id }
func (r *SimpleRoomEntity) GetType() string { return r.roomType }
