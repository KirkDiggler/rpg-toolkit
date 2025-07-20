package spawn

import (
	"context"
	"strings"

	"github.com/KirkDiggler/rpg-toolkit/core"
	"github.com/KirkDiggler/rpg-toolkit/events"
	"github.com/KirkDiggler/rpg-toolkit/tools/spatial"
)

// Event publishing implementation following ADR patterns with split-awareness

// publishOperationStartedEvent publishes spawn operation start event
func (e *BasicSpawnEngine) publishOperationStartedEvent(ctx context.Context, roomOrGroup interface{}, config SpawnConfig) {
	if !e.shouldPublishEvents() {
		return
	}

	roomEntity := e.createRoomEntityUnsafe(roomOrGroup, "spawn_target")
	
	operationData := SpawnOperationEventData{
		RoomOrGroup:   roomOrGroup,
		Configuration: config,
	}

	event := events.NewGameEvent(EventSpawnOperationStarted, roomEntity, nil)
	event.Context().Set("operation_data", operationData)
	_ = e.eventBus.Publish(ctx, event)
}

// publishSpawnEventsUnsafe publishes comprehensive spawn completion events with split-awareness
func (e *BasicSpawnEngine) publishSpawnEventsUnsafe(ctx context.Context, roomOrGroup interface{}, config SpawnConfig, result SpawnResult) {
	if !e.shouldPublishEvents() {
		return
	}

	// Create appropriate room entity based on single vs split configuration
	roomEntity := e.createRoomEntityUnsafe(roomOrGroup, "spawn_target")

	// Publish operation completed event (different event type for split vs single)
	eventType := EventSpawnOperationCompleted
	if result.RoomStructure.IsSplit {
		eventType = EventMultiRoomSpawn
	}

	operationData := SpawnOperationEventData{
		RoomOrGroup:       roomOrGroup,
		Configuration:     config,
		Result:           result,
		TotalEntities:    len(result.SpawnedEntities),
		FailedEntities:   len(result.Failures),
		ExecutionTime:    result.Metadata.ExecutionTime,
		RoomModifications: result.RoomModifications,
		UsedSplitRooms:   result.RoomStructure.IsSplit,
	}

	event := events.NewGameEvent(eventType, roomEntity, nil)
	event.Context().Set("operation_data", operationData)
	_ = e.eventBus.Publish(ctx, event)

	// Publish individual entity spawn events (now with room info for splits)
	for _, spawnedEntity := range result.SpawnedEntities {
		e.publishEntitySpawnedEventUnsafe(ctx, roomEntity, spawnedEntity)
	}

	// Publish failure events for debugging
	for _, failure := range result.Failures {
		e.publishEntitySpawnFailureEventUnsafe(ctx, roomEntity, failure)
	}

	// Publish room modification events
	for _, modification := range result.RoomModifications {
		e.publishRoomModificationEventUnsafe(ctx, modification)
	}
}

// publishEntitySpawnedEventUnsafe publishes individual entity spawn event
func (e *BasicSpawnEngine) publishEntitySpawnedEventUnsafe(ctx context.Context, roomEntity core.Entity, spawnedEntity SpawnedEntity) {
	entityData := EntitySpawnEventData{
		Entity:      spawnedEntity.Entity,
		Position:    spawnedEntity.Position,
		RoomID:      spawnedEntity.RoomID, // Specific room in split config
		GroupID:     spawnedEntity.GroupID,
		TeamID:      spawnedEntity.TeamID,
		SpawnReason: spawnedEntity.SpawnReason,
	}

	event := events.NewGameEvent(EventEntitySpawned, roomEntity, spawnedEntity.Entity)
	event.Context().Set("entity_data", entityData)
	_ = e.eventBus.Publish(ctx, event)
}

// publishEntitySpawnFailureEventUnsafe publishes entity spawn failure event
func (e *BasicSpawnEngine) publishEntitySpawnFailureEventUnsafe(ctx context.Context, roomEntity core.Entity, failure SpawnFailure) {
	failureData := EntitySpawnFailureEventData{
		EntityType:        failure.EntityType,
		GroupID:          failure.GroupID,
		Reason:           failure.Reason,
		AttemptedPosition: failure.AttemptedPosition,
		ConstraintsFailed: failure.ConstraintsFailed,
		RoomID:           failure.GroupID, // Simplified for now
	}

	event := events.NewGameEvent(EventEntitySpawnFailed, roomEntity, nil)
	event.Context().Set("failure_data", failureData)
	_ = e.eventBus.Publish(ctx, event)
}

// publishSplitRecommendationEventUnsafe publishes split recommendation event (passthrough to client)
func (e *BasicSpawnEngine) publishSplitRecommendationEventUnsafe(ctx context.Context, roomID string, splitOptions []RoomSplit, entityCount int) {
	if !e.shouldPublishEvents() {
		return
	}

	roomEntity := &SpawnRoomEntity{id: roomID, roomType: "single_spawn_target"}

	splitData := SplitRecommendationEventData{
		OriginalRoomID: roomID,
		SplitOptions:   splitOptions,
		EntityCount:    entityCount,
		Reason:         "Capacity analysis suggests room splitting",
	}

	event := events.NewGameEvent(EventSplitRecommended, roomEntity, nil)
	event.Context().Set("split_data", splitData)
	_ = e.eventBus.Publish(ctx, event)
}

// publishRoomScalingEventUnsafe publishes room scaling event with detailed reasoning
func (e *BasicSpawnEngine) publishRoomScalingEventUnsafe(ctx context.Context, roomID string, oldDimensions, newDimensions spatial.Dimensions, entityCount int, scaleFactor float64) {
	if !e.shouldPublishEvents() {
		return
	}

	roomEntity := &SpawnRoomEntity{id: roomID, roomType: "single_spawn_target"}

	scalingData := RoomModificationEventData{
		RoomID:            roomID,
		ModificationType:  "scaled",
		OldDimensions:     oldDimensions,
		NewDimensions:     newDimensions,
		ScaleFactor:       scaleFactor,
		Reason:           "Room scaled to accommodate entities",
		EntityCount:       entityCount,
		Alternatives:     []string{"Consider splitting the room", "Reduce entity count"},
	}

	event := events.NewGameEvent(EventRoomScaled, roomEntity, nil)
	event.Context().Set("scaling_data", scalingData)
	_ = e.eventBus.Publish(ctx, event)
}

// publishRoomModificationEventUnsafe publishes room modification event
func (e *BasicSpawnEngine) publishRoomModificationEventUnsafe(ctx context.Context, modification RoomModification) {
	if !e.shouldPublishEvents() {
		return
	}

	roomEntity := &SpawnRoomEntity{id: modification.RoomID, roomType: "modified_room"}

	modificationData := RoomModificationEventData{
		RoomID:           modification.RoomID,
		ModificationType: modification.Type,
		OldDimensions:    modification.OldValue,
		NewDimensions:    modification.NewValue,
		Reason:          modification.Reason,
	}

	event := events.NewGameEvent(EventRoomModified, roomEntity, nil)
	event.Context().Set("modification_data", modificationData)
	_ = e.eventBus.Publish(ctx, event)
}

// publishTeamSpawnEventUnsafe publishes team-based spawn event
func (e *BasicSpawnEngine) publishTeamSpawnEventUnsafe(ctx context.Context, roomEntity core.Entity, teamID string, entities []SpawnedEntity, spawnArea spatial.Rectangle) {
	if !e.shouldPublishEvents() {
		return
	}

	teamData := TeamSpawnEventData{
		TeamID:             teamID,
		EntityCount:        len(entities),
		SpawnArea:          spawnArea,
		CohesionAchieved:   1.0, // Placeholder - would calculate actual cohesion
		SeparationDistance: 5.0, // Placeholder - would calculate actual separation
	}

	event := events.NewGameEvent(EventTeamSpawned, roomEntity, nil)
	event.Context().Set("team_data", teamData)
	_ = e.eventBus.Publish(ctx, event)
}

// publishFormationEventUnsafe publishes formation application event
func (e *BasicSpawnEngine) publishFormationEventUnsafe(ctx context.Context, roomEntity core.Entity, formation FormationPattern, centerPosition spatial.Position, entities []SpawnedEntity) {
	if !e.shouldPublishEvents() {
		return
	}

	formationData := FormationEventData{
		FormationName:  formation.Name,
		EntityCount:    len(entities),
		CenterPosition: centerPosition,
		Scaling:        formation.Scaling,
		QualityScore:   0.8, // Placeholder - would calculate actual quality
		Adjustments:    []string{}, // Would list any adjustments made
	}

	event := events.NewGameEvent(EventFormationApplied, roomEntity, nil)
	event.Context().Set("formation_data", formationData)
	_ = e.eventBus.Publish(ctx, event)
}

// publishPlayerSpawnEventUnsafe publishes player spawn choice event
func (e *BasicSpawnEngine) publishPlayerSpawnEventUnsafe(ctx context.Context, roomEntity core.Entity, choice PlayerSpawnChoice, granted bool, denialReason string) {
	if !e.shouldPublishEvents() {
		return
	}

	var eventType string
	if granted {
		eventType = EventPlayerSpawnRequested
	} else {
		eventType = EventPlayerSpawnDenied
	}

	playerData := PlayerSpawnEventData{
		PlayerID:          choice.PlayerID,
		RequestedZone:     choice.ZoneID,
		RequestedPosition: choice.Position,
		Granted:          granted,
		DenialReason:     denialReason,
	}

	event := events.NewGameEvent(eventType, roomEntity, nil)
	event.Context().Set("player_data", playerData)
	_ = e.eventBus.Publish(ctx, event)
}

// publishConstraintViolationEventUnsafe publishes constraint violation event
func (e *BasicSpawnEngine) publishConstraintViolationEventUnsafe(ctx context.Context, roomEntity core.Entity, constraintType, violatedRule string, severity string) {
	if !e.shouldPublishEvents() {
		return
	}

	violationData := ConstraintViolationEventData{
		ConstraintType: constraintType,
		ViolatedRule:   violatedRule,
		Severity:      severity,
		Resolution:    "Constraint relaxed for spawn completion",
	}

	event := events.NewGameEvent(EventConstraintViolation, roomEntity, nil)
	event.Context().Set("violation_data", violationData)
	_ = e.eventBus.Publish(ctx, event)
}

// Helper methods for event publishing

// shouldPublishEvents checks if events should be published
func (e *BasicSpawnEngine) shouldPublishEvents() bool {
	return e.eventBus != nil && e.config.EnableEvents
}

// createRoomEntityUnsafe creates appropriate entity for event context based on single vs split rooms
func (e *BasicSpawnEngine) createRoomEntityUnsafe(roomOrGroup interface{}, entityType string) core.Entity {
	switch v := roomOrGroup.(type) {
	case string:
		// Single room
		return &SpawnRoomEntity{
			id:       v,
			roomType: "single_" + entityType,
		}
	case []string:
		// Split rooms
		roomID := strings.Join(v, "+")
		return &SpawnRoomGroupEntity{
			id:             roomID,
			roomType:       "split_" + entityType,
			connectedRooms: v,
		}
	default:
		// Fallback
		return &SpawnRoomEntity{
			id:       "unknown",
			roomType: "unknown_" + entityType,
		}
	}
}

// Entity implementations for event contexts

// SpawnRoomEntity represents a single room for event context
type SpawnRoomEntity struct {
	id       string
	roomType string
}

func (e *SpawnRoomEntity) GetID() string   { return e.id }
func (e *SpawnRoomEntity) GetType() string { return e.roomType }

// SpawnRoomGroupEntity represents multiple connected rooms for event context
type SpawnRoomGroupEntity struct {
	id             string
	roomType       string
	connectedRooms []string
}

func (e *SpawnRoomGroupEntity) GetID() string   { return e.id }
func (e *SpawnRoomGroupEntity) GetType() string { return e.roomType }
func (e *SpawnRoomGroupEntity) GetConnectedRooms() []string { return e.connectedRooms }