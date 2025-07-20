package spawn

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/KirkDiggler/rpg-toolkit/core"
	"github.com/KirkDiggler/rpg-toolkit/events"
	"github.com/KirkDiggler/rpg-toolkit/tools/spatial"
)

// BasicSpawnEngine implements the SpawnEngine interface
// Purpose: Provides comprehensive entity spawning with split-awareness,
// environment integration, and clean separation of concerns
type BasicSpawnEngine struct {
	// Core identification
	id   string
	entityType string

	// Dependencies - we are a CLIENT of these services
	spatialQueryHandler   spatial.QueryHandler
	environmentQueryHandler SpawnEnvironmentQueryHandler
	selectablesRegistry   SelectablesRegistry
	eventBus             events.EventBus

	// Configuration and state
	config SpawnEngineConfiguration
	
	// Thread safety
	mutex sync.RWMutex
}

// BasicSpawnEngineConfig follows toolkit config pattern
type BasicSpawnEngineConfig struct {
	ID                      string                        `json:"id"`
	SpatialQueryHandler     spatial.QueryHandler          `json:"-"`
	EnvironmentQueryHandler SpawnEnvironmentQueryHandler  `json:"-"`
	SelectablesRegistry     SelectablesRegistry           `json:"-"`
	EventBus               events.EventBus               `json:"-"`
	Configuration          SpawnEngineConfiguration      `json:"configuration"`
}

// SpawnEngineConfiguration defines engine behavior settings
type SpawnEngineConfiguration struct {
	EnableEvents         bool   `json:"enable_events"`
	EnableDebugging      bool   `json:"enable_debugging"`
	MaxPlacementAttempts int    `json:"max_placement_attempts"`
	DefaultTimeoutSeconds int   `json:"default_timeout_seconds"`
	PerformanceMode      string `json:"performance_mode"` // "fast", "thorough", "balanced"
	QualityThreshold     float64 `json:"quality_threshold"` // 0.0-1.0, accept 80% constraint satisfaction
}

// NewBasicSpawnEngine creates a new spawn engine
// Purpose: Standard constructor following toolkit patterns
func NewBasicSpawnEngine(config BasicSpawnEngineConfig) *BasicSpawnEngine {
	if config.ID == "" {
		config.ID = generateEngineID()
	}

	// Set defaults following performance decisions from ADR
	if config.Configuration.MaxPlacementAttempts == 0 {
		config.Configuration.MaxPlacementAttempts = 1000 // Per ADR performance decisions
	}
	if config.Configuration.DefaultTimeoutSeconds == 0 {
		config.Configuration.DefaultTimeoutSeconds = 30 // Per ADR performance decisions
	}
	if config.Configuration.QualityThreshold == 0 {
		config.Configuration.QualityThreshold = 0.8 // Accept 80% constraint satisfaction
	}
	if config.Configuration.PerformanceMode == "" {
		config.Configuration.PerformanceMode = "balanced"
	}

	return &BasicSpawnEngine{
		id:                      config.ID,
		entityType:              "spawn_engine",
		spatialQueryHandler:     config.SpatialQueryHandler,
		environmentQueryHandler: config.EnvironmentQueryHandler,
		selectablesRegistry:     config.SelectablesRegistry,
		eventBus:               config.EventBus,
		config:                 config.Configuration,
	}
}

// Core Entity interface implementation
func (e *BasicSpawnEngine) GetID() string {
	return e.id
}

func (e *BasicSpawnEngine) GetType() string {
	return e.entityType
}

// PopulateSpace is the universal interface for any room configuration
// Purpose: Split-aware spawning that works with single rooms or multi-room configurations
func (e *BasicSpawnEngine) PopulateSpace(ctx context.Context, roomOrGroup interface{}, config SpawnConfig) (SpawnResult, error) {
	e.mutex.Lock()
	defer e.mutex.Unlock()

	startTime := time.Now()
	result := SpawnResult{
		Success:              false,
		SpawnedEntities:      make([]SpawnedEntity, 0),
		Failures:             make([]SpawnFailure, 0),
		RoomModifications:    make([]RoomModification, 0),
		SplitRecommendations: make([]RoomSplit, 0),
		Metadata: SpawnMetadata{
			AttemptCounts:      make(map[string]int),
			PerformanceMetrics: make(map[string]float64),
		},
	}

	// Publish operation started event
	e.publishOperationStartedEvent(ctx, roomOrGroup, config)

	// Validate configuration
	if err := e.ValidateSpawnConfig(config); err != nil {
		return result, fmt.Errorf("invalid spawn config: %w", err)
	}

	// Analyze room structure to determine spawning approach
	roomStructure, err := e.analyzeRoomStructureUnsafe(roomOrGroup)
	if err != nil {
		return result, fmt.Errorf("failed to analyze room structure: %w", err)
	}
	result.RoomStructure = roomStructure

	// Handle capacity analysis and potential scaling/splitting
	if err := e.handleCapacityAnalysisUnsafe(ctx, roomOrGroup, config, &result); err != nil {
		return result, fmt.Errorf("capacity analysis failed: %w", err)
	}

	// Select entities from groups using selectables
	selectedEntities, err := e.selectEntitiesFromGroupsUnsafe(ctx, config.EntityGroups)
	if err != nil {
		return result, fmt.Errorf("entity selection failed: %w", err)
	}

	// Perform spawning based on room structure
	if roomStructure.IsSplit {
		err = e.populateSplitRoomsUnsafe(ctx, roomStructure.ConnectedRooms, selectedEntities, config, &result)
	} else {
		roomID := roomOrGroup.(string)
		err = e.populateSingleRoomUnsafe(ctx, roomID, selectedEntities, config, &result)
	}

	if err != nil {
		return result, fmt.Errorf("entity placement failed: %w", err)
	}

	// Calculate execution metrics
	result.Metadata.ExecutionTime = time.Since(startTime)
	result.Success = len(result.SpawnedEntities) > 0

	// Publish completion events
	e.publishSpawnEventsUnsafe(ctx, roomOrGroup, config, result)

	return result, nil
}

// PopulateRoom provides backwards compatibility for single-room spawning
func (e *BasicSpawnEngine) PopulateRoom(ctx context.Context, roomID string, config SpawnConfig) (SpawnResult, error) {
	return e.PopulateSpace(ctx, roomID, config)
}

// PopulateSplitRooms handles explicit multi-room spawning
func (e *BasicSpawnEngine) PopulateSplitRooms(ctx context.Context, connectedRooms []string, config SpawnConfig) (SpawnResult, error) {
	return e.PopulateSpace(ctx, connectedRooms, config)
}

// HandleRoomTransition manages entity movement between rooms
func (e *BasicSpawnEngine) HandleRoomTransition(ctx context.Context, entityID, fromRoom, toRoom, connectionID string) (spatial.Position, error) {
	e.mutex.RLock()
	defer e.mutex.RUnlock()

	// This would typically coordinate with spatial orchestrator for room transitions
	// For now, return a basic implementation
	return spatial.Position{X: 0, Y: 0}, fmt.Errorf("room transitions not yet implemented")
}

// ValidateSpawnConfig validates spawn configuration
func (e *BasicSpawnEngine) ValidateSpawnConfig(config SpawnConfig) error {
	var errors []string

	// Validate entity groups
	for i, group := range config.EntityGroups {
		if group.ID == "" {
			errors = append(errors, fmt.Sprintf("entity group %d missing ID", i))
		}

		// Must have either selection table or entities, not both
		hasTable := group.SelectionTable != ""
		hasEntities := len(group.Entities) > 0

		if !hasTable && !hasEntities {
			errors = append(errors, fmt.Sprintf("entity group %s must have either selection_table or entities", group.ID))
		}

		if hasTable && hasEntities {
			errors = append(errors, fmt.Sprintf("entity group %s cannot have both selection_table and entities", group.ID))
		}

		// Validate quantity specification
		if err := e.validateQuantitySpec(group.Quantity); err != nil {
			errors = append(errors, fmt.Sprintf("entity group %s: %v", group.ID, err))
		}
	}

	// Validate player spawn zones
	for i, zone := range config.PlayerSpawnZones {
		if zone.ID == "" {
			errors = append(errors, fmt.Sprintf("player spawn zone %d missing ID", i))
		}
		if zone.MaxEntities <= 0 {
			errors = append(errors, fmt.Sprintf("player spawn zone %s must allow at least 1 entity", zone.ID))
		}
	}

	// Validate player choices reference valid zones
	zoneIDs := make(map[string]bool)
	for _, zone := range config.PlayerSpawnZones {
		zoneIDs[zone.ID] = true
	}

	for _, choice := range config.PlayerChoices {
		if choice.PlayerID == "" {
			errors = append(errors, "player choice missing player_id")
		}
		if !zoneIDs[choice.ZoneID] {
			errors = append(errors, fmt.Sprintf("player choice references unknown zone: %s", choice.ZoneID))
		}
	}

	if len(errors) > 0 {
		return fmt.Errorf("spawn config validation failed: %s", strings.Join(errors, "; "))
	}

	return nil
}

// AnalyzeRoomStructure provides room structure information for split-awareness
func (e *BasicSpawnEngine) AnalyzeRoomStructure(roomID string) RoomStructureInfo {
	e.mutex.RLock()
	defer e.mutex.RUnlock()

	// For now, return basic single room structure
	// In full implementation, would query spatial orchestrator for connections
	return RoomStructureInfo{
		IsSplit:        false,
		ConnectedRooms: []string{roomID},
		PrimaryRoomID:  roomID,
		TotalCapacity:  0, // Would be calculated from spatial queries
	}
}

// PopulateSpaceWithHelper provides convenience interface using helper configs
func (e *BasicSpawnEngine) PopulateSpaceWithHelper(ctx context.Context, roomOrGroup interface{}, entities []core.Entity, helperConfig HelperConfig) (SpawnResult, error) {
	// Convert helper config to full spawn config
	config := e.convertHelperConfigUnsafe(entities, helperConfig)
	return e.PopulateSpace(ctx, roomOrGroup, config)
}

// Helper methods

func (e *BasicSpawnEngine) validateQuantitySpec(spec QuantitySpec) error {
	count := 0
	if spec.Fixed != nil {
		count++
	}
	if spec.DiceRoll != nil {
		count++
	}
	if spec.MinMax != nil {
		count++
	}

	if count != 1 {
		return fmt.Errorf("quantity spec must have exactly one of: fixed, dice_roll, or min_max")
	}

	if spec.Fixed != nil && *spec.Fixed < 0 {
		return fmt.Errorf("fixed quantity cannot be negative")
	}

	if spec.MinMax != nil {
		if spec.MinMax.Min < 0 {
			return fmt.Errorf("min_max min cannot be negative")
		}
		if spec.MinMax.Max < spec.MinMax.Min {
			return fmt.Errorf("min_max max must be >= min")
		}
	}

	return nil
}

func generateEngineID() string {
	return fmt.Sprintf("spawn_engine_%d", time.Now().UnixNano())
}