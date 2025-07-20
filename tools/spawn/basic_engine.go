package spawn

import (
	"context"
	"fmt"
	"math/rand"
	"time"

	"github.com/KirkDiggler/rpg-toolkit/core"
	"github.com/KirkDiggler/rpg-toolkit/events"
	"github.com/KirkDiggler/rpg-toolkit/tools/environments"
	"github.com/KirkDiggler/rpg-toolkit/tools/spatial"
)

// BasicSpawnEngine implements SpawnEngine interface.
// Purpose: Complete implementation per ADR-0013 with environment integration and split-aware spawning.
type BasicSpawnEngine struct {
	id                 string
	spatialHandler     spatial.QueryHandler
	environmentHandler *environments.BasicQueryHandler
	selectablesReg     SelectablesRegistry
	eventBus           events.EventBus
	enableEvents       bool
	maxAttempts        int
	random             *rand.Rand
}

// BasicSpawnEngineConfig configures a BasicSpawnEngine.
// Purpose: Configuration struct following toolkit patterns for dependency injection.
type BasicSpawnEngineConfig struct {
	ID                 string
	SpatialHandler     spatial.QueryHandler
	EnvironmentHandler *environments.BasicQueryHandler
	SelectablesReg     SelectablesRegistry
	EventBus           events.EventBus
	EnableEvents       bool
	MaxAttempts        int
}

// NewBasicSpawnEngine creates a new spawn engine with the specified configuration.
// Purpose: Standard constructor following toolkit config pattern with proper dependency injection.
func NewBasicSpawnEngine(config BasicSpawnEngineConfig) *BasicSpawnEngine {
	if config.ID == "" {
		config.ID = "spawn-engine"
	}
	if config.MaxAttempts == 0 {
		config.MaxAttempts = 100
	}

	return &BasicSpawnEngine{
		id:                 config.ID,
		spatialHandler:     config.SpatialHandler,
		environmentHandler: config.EnvironmentHandler,
		selectablesReg:     config.SelectablesReg,
		eventBus:           config.EventBus,
		enableEvents:       config.EnableEvents,
		maxAttempts:        config.MaxAttempts,
		random:             rand.New(rand.NewSource(time.Now().UnixNano())), // #nosec G404
	}
}

// PopulateSpace implements SpawnEngine.PopulateSpace
func (e *BasicSpawnEngine) PopulateSpace(
	ctx context.Context, roomOrGroup interface{}, config SpawnConfig,
) (SpawnResult, error) {
	result := SpawnResult{
		SpawnedEntities:      make([]SpawnedEntity, 0),
		Failures:             make([]SpawnFailure, 0),
		RoomModifications:    make([]RoomModification, 0),
		SplitRecommendations: make([]RoomSplit, 0),
	}

	// Analyze room structure
	roomStructure, err := e.analyzeRoomStructure(roomOrGroup)
	if err != nil {
		return result, fmt.Errorf("failed to analyze room structure: %w", err)
	}
	result.RoomStructure = roomStructure

	if roomStructure.IsSplit {
		return e.PopulateSplitRooms(ctx, roomStructure.ConnectedRooms, config)
	}
	return e.PopulateRoom(ctx, roomStructure.PrimaryRoomID, config)
}

// PopulateRoom implements SpawnEngine.PopulateRoom
func (e *BasicSpawnEngine) PopulateRoom(
	ctx context.Context, roomID string, config SpawnConfig,
) (SpawnResult, error) {
	result := SpawnResult{
		SpawnedEntities:      make([]SpawnedEntity, 0),
		Failures:             make([]SpawnFailure, 0),
		RoomModifications:    make([]RoomModification, 0),
		SplitRecommendations: make([]RoomSplit, 0),
		RoomStructure: RoomStructureInfo{
			IsSplit:        false,
			ConnectedRooms: []string{roomID},
			PrimaryRoomID:  roomID,
		},
	}

	if err := e.ValidateSpawnConfig(config); err != nil {
		return result, fmt.Errorf("invalid config: %w", err)
	}

	// Phase 4: Perform capacity analysis and scaling if needed
	err := e.handleCapacityAnalysis(ctx, roomID, config, &result)
	if err != nil {
		return result, fmt.Errorf("capacity analysis failed: %w", err)
	}

	// Route to appropriate spawning method based on pattern
	switch config.Pattern {
	case PatternScattered:
		return e.applyScatteredSpawning(ctx, roomID, config, result)
	case PatternFormation:
		return e.applyFormationSpawning(ctx, roomID, config, result)
	case PatternTeamBased:
		return e.applyTeamBasedSpawning(ctx, roomID, config, result)
	case PatternPlayerChoice:
		return e.applyPlayerChoiceSpawning(ctx, roomID, config, result)
	case PatternClustered:
		return e.applyClusteredSpawning(ctx, roomID, config, result)
	default:
		return result, fmt.Errorf("unsupported spawn pattern: %s", config.Pattern)
	}
}

// ValidateSpawnConfig implements SpawnEngine.ValidateSpawnConfig
func (e *BasicSpawnEngine) ValidateSpawnConfig(config SpawnConfig) error {
	if len(config.EntityGroups) == 0 {
		return fmt.Errorf("no entity groups specified")
	}

	// Phase 2: All patterns supported
	validPatterns := []SpawnPattern{
		PatternScattered, PatternFormation, PatternTeamBased,
		PatternPlayerChoice, PatternClustered,
	}
	validPattern := false
	for _, pattern := range validPatterns {
		if config.Pattern == pattern {
			validPattern = true
			break
		}
	}
	if !validPattern {
		return fmt.Errorf("unsupported pattern: %s", config.Pattern)
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

// getRoomFromSpatial gets room from spatial module (Phase 1: not implemented)
func (e *BasicSpawnEngine) getRoomFromSpatial(_ string) (spatial.Room, error) {
	// Phase 1: Spatial integration not yet implemented
	// Real implementation would query e.spatialHandler.GetRoom(roomID)
	return nil, fmt.Errorf("spatial integration not implemented in Phase 1")
}

// findValidPosition finds a valid position for an entity (simplified for Phase 1)
func (e *BasicSpawnEngine) findValidPosition(_ spatial.Room, _ core.Entity) (spatial.Position, error) {
	// Phase 1: Simple random position within reasonable bounds
	// Real implementation would query the spatial room for valid positions
	x := e.random.Float64() * 10.0
	y := e.random.Float64() * 10.0

	return spatial.Position{X: x, Y: y}, nil
}

// placeEntityInRoom places entity in the spatial room
func (e *BasicSpawnEngine) placeEntityInRoom(_ spatial.Room, _ core.Entity, _ spatial.Position) error {
	// Phase 1: This would call room.PlaceEntity(entity, position)
	// For now, just validate the basic operation
	return nil
}

// publishEntitySpawnedEvent publishes entity spawned event
func (e *BasicSpawnEngine) publishEntitySpawnedEvent(
	ctx context.Context, roomID string, entity core.Entity, position spatial.Position,
) {
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

// SimpleRoomEntity implements core.Entity for event publishing.
// Purpose: Minimal entity implementation for spawn event sources.
type SimpleRoomEntity struct {
	id       string
	roomType string
}

// GetID returns the room entity ID
func (r *SimpleRoomEntity) GetID() string { return r.id }

// GetType returns the room entity type
func (r *SimpleRoomEntity) GetType() string { return r.roomType }

// PopulateSplitRooms implements SpawnEngine.PopulateSplitRooms
func (e *BasicSpawnEngine) PopulateSplitRooms(
	ctx context.Context, connectedRooms []string, config SpawnConfig,
) (SpawnResult, error) {
	result := SpawnResult{
		SpawnedEntities:      make([]SpawnedEntity, 0),
		Failures:             make([]SpawnFailure, 0),
		RoomModifications:    make([]RoomModification, 0),
		SplitRecommendations: make([]RoomSplit, 0),
		RoomStructure: RoomStructureInfo{
			IsSplit:        true,
			ConnectedRooms: connectedRooms,
			PrimaryRoomID:  connectedRooms[0],
		},
	}

	if err := e.ValidateSpawnConfig(config); err != nil {
		return result, fmt.Errorf("invalid config: %w", err)
	}

	// For Phase 2: Distribute entities across connected rooms
	// Simple approach: spawn in first room for now
	singleRoomResult, err := e.PopulateRoom(ctx, connectedRooms[0], config)
	if err != nil {
		return result, err
	}

	// Copy results and update room structure info
	result.Success = singleRoomResult.Success
	result.SpawnedEntities = singleRoomResult.SpawnedEntities
	result.Failures = singleRoomResult.Failures

	return result, nil
}

// AnalyzeRoomStructure implements SpawnEngine.AnalyzeRoomStructure
func (e *BasicSpawnEngine) AnalyzeRoomStructure(roomID string) RoomStructureInfo {
	// Phase 2: Simple implementation - assume single room
	return RoomStructureInfo{
		IsSplit:        false,
		ConnectedRooms: []string{roomID},
		PrimaryRoomID:  roomID,
	}
}

// analyzeRoomStructure analyzes the room structure from roomOrGroup parameter
func (e *BasicSpawnEngine) analyzeRoomStructure(roomOrGroup interface{}) (RoomStructureInfo, error) {
	switch v := roomOrGroup.(type) {
	case string:
		// Single room
		return RoomStructureInfo{
			IsSplit:        false,
			ConnectedRooms: []string{v},
			PrimaryRoomID:  v,
		}, nil
	case []string:
		// Multiple connected rooms
		if len(v) == 0 {
			return RoomStructureInfo{}, fmt.Errorf("empty room list")
		}
		return RoomStructureInfo{
			IsSplit:        true,
			ConnectedRooms: v,
			PrimaryRoomID:  v[0],
		}, nil
	default:
		return RoomStructureInfo{}, fmt.Errorf("unsupported room structure type: %T", roomOrGroup)
	}
}
