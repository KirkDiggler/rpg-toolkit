// Package environments provides procedural environment and room generation capabilities.
package environments

import (
	"time"

	"github.com/KirkDiggler/rpg-toolkit/events"
)

// Typed topic definitions for environment module events
// These are defined at compile-time and connected to event bus at runtime via .On(bus)

var (
	// GenerationStartedTopic publishes events when environment generation starts
	GenerationStartedTopic = events.DefineTypedTopic[GenerationStartedEvent]("environment.generation.started")
	// GenerationProgressTopic publishes events during environment generation progress
	GenerationProgressTopic = events.DefineTypedTopic[GenerationProgressEvent]("environment.generation.progress")
	// GenerationCompletedTopic publishes events when environment generation completes
	GenerationCompletedTopic = events.DefineTypedTopic[GenerationCompletedEvent]("environment.generation.completed")
	// GenerationFailedTopic publishes events when environment generation fails
	GenerationFailedTopic = events.DefineTypedTopic[GenerationFailedEvent]("environment.generation.failed")
	// EmergencyFallbackTopic publishes events when emergency fallback procedures are triggered
	EmergencyFallbackTopic = events.DefineTypedTopic[EmergencyFallbackEvent]("environment.emergency_fallback.triggered")

	// EnvironmentGeneratedTopic publishes events when environments are generated
	EnvironmentGeneratedTopic = events.DefineTypedTopic[EnvironmentGeneratedEvent]("environment.generated")
	// EnvironmentDestroyedTopic publishes events when environments are destroyed
	EnvironmentDestroyedTopic = events.DefineTypedTopic[EnvironmentDestroyedEvent]("environment.destroyed")

	// EnvironmentEntityAddedTopic publishes events when entities are added to environments
	EnvironmentEntityAddedTopic = events.DefineTypedTopic[EnvironmentEntityAddedEvent]("environment.entity.added")
	// EnvironmentEntityRemovedTopic publishes events when entities are removed from environments
	EnvironmentEntityRemovedTopic = events.DefineTypedTopic[EnvironmentEntityRemovedEvent]("environment.entity.removed")

	// EnvironmentRoomAddedTopic publishes events when rooms are added to environments
	EnvironmentRoomAddedTopic   = events.DefineTypedTopic[EnvironmentRoomAddedEvent]("environment.room.added")
	EnvironmentRoomRemovedTopic = events.DefineTypedTopic[EnvironmentRoomRemovedEvent]("environment.room.removed")

	// FeatureAddedTopic publishes events when features are added to environments
	FeatureAddedTopic   = events.DefineTypedTopic[FeatureAddedEvent]("environment.feature.added")
	FeatureRemovedTopic = events.DefineTypedTopic[FeatureRemovedEvent]("environment.feature.removed")

	// HazardTriggeredTopic publishes events when environmental hazards are triggered
	HazardTriggeredTopic = events.DefineTypedTopic[HazardTriggeredEvent]("environment.hazard.triggered")

	// ThemeChangedTopic publishes events when environment themes change
	ThemeChangedTopic               = events.DefineTypedTopic[ThemeChangedEvent]("environment.theme.changed")
	EnvironmentMetadataChangedTopic = events.DefineTypedTopic[EnvironmentMetadataChangedEvent]("environment.metadata")

	// EnvironmentEntityMovedTopic publishes events when entities move within environments
	EnvironmentEntityMovedTopic = events.DefineTypedTopic[EnvironmentEntityMovedEvent]("environment.entity.moved")

	// QueryExecutedTopic publishes events when environment queries are executed
	QueryExecutedTopic = events.DefineTypedTopic[QueryExecutedEvent]("environment.query.executed")
	QueryFailedTopic   = events.DefineTypedTopic[QueryFailedEvent]("environment.query.failed")

	// RoomBuiltTopic publishes events when rooms are built within environments
	RoomBuiltTopic = events.DefineTypedTopic[RoomBuiltEvent]("environment.room.built")
)

// GenerationStartedEvent contains data for environment generation start events
type GenerationStartedEvent struct {
	GenerationID string                 `json:"generation_id"`
	RequestID    string                 `json:"request_id,omitempty"`
	Config       map[string]interface{} `json:"config"`
	StartTime    time.Time              `json:"start_time"`
}

// GenerationProgressEvent contains data for environment generation progress events
type GenerationProgressEvent struct {
	GenerationID string    `json:"generation_id"`
	Stage        string    `json:"stage"`    // "layout", "walls", "features", "validation"
	Progress     float64   `json:"progress"` // 0.0 to 1.0
	Message      string    `json:"message,omitempty"`
	Timestamp    time.Time `json:"timestamp"`
}

// GenerationCompletedEvent contains data for environment generation completion events
type GenerationCompletedEvent struct {
	GenerationID    string                 `json:"generation_id"`
	RequestID       string                 `json:"request_id,omitempty"`
	Config          map[string]interface{} `json:"config"`
	RoomCount       int                    `json:"room_count"`
	ConnectionCount int                    `json:"connection_count"`
	Duration        time.Duration          `json:"duration"`
	CompletedAt     time.Time              `json:"completed_at"`
}

// GenerationFailedEvent contains data for environment generation failure events
type GenerationFailedEvent struct {
	GenerationID string                 `json:"generation_id"`
	RequestID    string                 `json:"request_id,omitempty"`
	Config       map[string]interface{} `json:"config,omitempty"`
	Error        string                 `json:"error"`
	Stage        string                 `json:"stage"`
	FailedAt     time.Time              `json:"failed_at"`
}

// EmergencyFallbackEvent contains data for emergency fallback activation events
type EmergencyFallbackEvent struct {
	GenerationID string                 `json:"generation_id"`
	Trigger      string                 `json:"trigger"`
	FallbackType string                 `json:"fallback_type"`
	Parameters   map[string]interface{} `json:"parameters,omitempty"`
	Timestamp    time.Time              `json:"timestamp"`
}

// EnvironmentGeneratedEvent contains data for environment generation completion events
type EnvironmentGeneratedEvent struct {
	EnvironmentID string                 `json:"environment_id"`
	Type          string                 `json:"type"`
	Theme         string                 `json:"theme,omitempty"`
	RoomCount     int                    `json:"room_count"`
	Features      []string               `json:"features,omitempty"`
	Metadata      map[string]interface{} `json:"metadata,omitempty"`
	GeneratedAt   time.Time              `json:"generated_at"`
}

// EnvironmentDestroyedEvent contains data for environment destruction events
type EnvironmentDestroyedEvent struct {
	EnvironmentID string    `json:"environment_id"`
	Reason        string    `json:"reason,omitempty"`
	DestroyedAt   time.Time `json:"destroyed_at"`
}

// EnvironmentEntityAddedEvent contains data for entity addition to environment events
type EnvironmentEntityAddedEvent struct {
	EnvironmentID string                 `json:"environment_id"`
	EntityID      string                 `json:"entity_id"`
	EntityType    string                 `json:"entity_type,omitempty"`
	RoomID        string                 `json:"room_id,omitempty"`
	Properties    map[string]interface{} `json:"properties,omitempty"`
	AddedAt       time.Time              `json:"added_at"`
}

// EnvironmentEntityRemovedEvent contains data for entity removal from environment events
type EnvironmentEntityRemovedEvent struct {
	EnvironmentID string    `json:"environment_id"`
	EntityID      string    `json:"entity_id"`
	RoomID        string    `json:"room_id,omitempty"`
	Reason        string    `json:"reason,omitempty"`
	RemovedAt     time.Time `json:"removed_at"`
}

// EnvironmentRoomAddedEvent contains data for room addition to environment events
type EnvironmentRoomAddedEvent struct {
	EnvironmentID string                 `json:"environment_id"`
	RoomID        string                 `json:"room_id"`
	RoomType      string                 `json:"room_type,omitempty"`
	Properties    map[string]interface{} `json:"properties,omitempty"`
	AddedAt       time.Time              `json:"added_at"`
}

// EnvironmentRoomRemovedEvent contains data for room removal from environment events
type EnvironmentRoomRemovedEvent struct {
	EnvironmentID string    `json:"environment_id"`
	RoomID        string    `json:"room_id"`
	Reason        string    `json:"reason,omitempty"`
	RemovedAt     time.Time `json:"removed_at"`
}

// FeatureAddedEvent contains data for feature addition to environment events
type FeatureAddedEvent struct {
	EnvironmentID string                 `json:"environment_id"`
	FeatureID     string                 `json:"feature_id"`
	FeatureType   string                 `json:"feature_type"`
	RoomID        string                 `json:"room_id,omitempty"`
	Properties    map[string]interface{} `json:"properties,omitempty"`
	AddedAt       time.Time              `json:"added_at"`
}

// FeatureRemovedEvent contains data for feature removal from environment events
type FeatureRemovedEvent struct {
	EnvironmentID string    `json:"environment_id"`
	FeatureID     string    `json:"feature_id"`
	RoomID        string    `json:"room_id,omitempty"`
	Reason        string    `json:"reason,omitempty"`
	RemovedAt     time.Time `json:"removed_at"`
}

// HazardTriggeredEvent contains data for environmental hazard trigger events
type HazardTriggeredEvent struct {
	EnvironmentID string                 `json:"environment_id"`
	HazardID      string                 `json:"hazard_id"`
	HazardType    string                 `json:"hazard_type"`
	TriggerEntity string                 `json:"trigger_entity,omitempty"`
	RoomID        string                 `json:"room_id,omitempty"`
	Effect        string                 `json:"effect,omitempty"`
	Properties    map[string]interface{} `json:"properties,omitempty"`
	TriggeredAt   time.Time              `json:"triggered_at"`
}

// ThemeChangedEvent contains data for environment theme change events
type ThemeChangedEvent struct {
	EnvironmentID string    `json:"environment_id"`
	OldTheme      string    `json:"old_theme"`
	NewTheme      string    `json:"new_theme"`
	AffectedRooms []string  `json:"affected_rooms,omitempty"`
	ChangedAt     time.Time `json:"changed_at"`
}

// EnvironmentMetadataChangedEvent contains data for environment metadata change events
type EnvironmentMetadataChangedEvent struct {
	EnvironmentID string              `json:"environment_id"`
	Metadata      EnvironmentMetadata `json:"metadata"`
	ChangedAt     time.Time           `json:"changed_at"`
}

// EnvironmentEntityMovedEvent contains data for entity movement within environment events
type EnvironmentEntityMovedEvent struct {
	EnvironmentID string                 `json:"environment_id"`
	EntityID      string                 `json:"entity_id"`
	EntityType    string                 `json:"entity_type,omitempty"`
	RoomID        string                 `json:"room_id,omitempty"`
	FromPosition  map[string]interface{} `json:"from_position,omitempty"`
	ToPosition    map[string]interface{} `json:"to_position,omitempty"`
	Properties    map[string]interface{} `json:"properties,omitempty"`
	MovedAt       time.Time              `json:"moved_at"`
}

// QueryExecutedEvent contains data for environment query execution events
type QueryExecutedEvent struct {
	QueryType  string      `json:"query_type"`
	Query      interface{} `json:"query"`
	ExecutedAt time.Time   `json:"executed_at"`
}

// QueryFailedEvent contains data for environment query failure events
type QueryFailedEvent struct {
	QueryType string      `json:"query_type"`
	Query     interface{} `json:"query"`
	Error     string      `json:"error"`
	FailedAt  time.Time   `json:"failed_at"`
}

// RoomBuiltEvent contains data for room construction completion events
type RoomBuiltEvent struct {
	RoomID     string                 `json:"room_id"`
	Size       map[string]interface{} `json:"size"`
	Theme      string                 `json:"theme"`
	Pattern    string                 `json:"pattern"`
	Features   []string               `json:"features,omitempty"`
	Properties map[string]interface{} `json:"properties,omitempty"`
	BuiltAt    time.Time              `json:"built_at"`
}
