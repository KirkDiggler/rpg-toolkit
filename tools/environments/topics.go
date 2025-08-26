// Package environments provides procedural environment and room generation capabilities.
package environments

import (
	"time"

	"github.com/KirkDiggler/rpg-toolkit/events"
)

// Typed topic definitions for environment module events
// These are defined at compile-time and connected to event bus at runtime via .On(bus)

var (
	// Generation lifecycle topics - these are the actively published events
	GenerationStartedTopic   = events.DefineTypedTopic[GenerationStartedEvent]("environment.generation.started")
	GenerationProgressTopic  = events.DefineTypedTopic[GenerationProgressEvent]("environment.generation.progress")
	GenerationCompletedTopic = events.DefineTypedTopic[GenerationCompletedEvent]("environment.generation.completed")
	GenerationFailedTopic    = events.DefineTypedTopic[GenerationFailedEvent]("environment.generation.failed")
	EmergencyFallbackTopic   = events.DefineTypedTopic[EmergencyFallbackEvent]("environment.emergency_fallback.triggered")

	// Environment lifecycle topics
	EnvironmentGeneratedTopic = events.DefineTypedTopic[EnvironmentGeneratedEvent]("environment.generated")
	EnvironmentDestroyedTopic = events.DefineTypedTopic[EnvironmentDestroyedEvent]("environment.destroyed")

	// Entity management topics  
	EnvironmentEntityAddedTopic   = events.DefineTypedTopic[EnvironmentEntityAddedEvent]("environment.entity.added")
	EnvironmentEntityRemovedTopic = events.DefineTypedTopic[EnvironmentEntityRemovedEvent]("environment.entity.removed")
	
	// Room management topics
	EnvironmentRoomAddedTopic   = events.DefineTypedTopic[EnvironmentRoomAddedEvent]("environment.room.added")
	EnvironmentRoomRemovedTopic = events.DefineTypedTopic[EnvironmentRoomRemovedEvent]("environment.room.removed")

	// Feature management topics
	FeatureAddedTopic   = events.DefineTypedTopic[FeatureAddedEvent]("environment.feature.added")
	FeatureRemovedTopic = events.DefineTypedTopic[FeatureRemovedEvent]("environment.feature.removed")
	
	// Hazard management topics
	HazardTriggeredTopic = events.DefineTypedTopic[HazardTriggeredEvent]("environment.hazard.triggered")
)

// Generation lifecycle events
type GenerationStartedEvent struct {
	GenerationID string                 `json:"generation_id"`
	RequestID    string                 `json:"request_id,omitempty"`
	Config       map[string]interface{} `json:"config"`
	StartTime    time.Time              `json:"start_time"`
}

type GenerationProgressEvent struct {
	GenerationID string    `json:"generation_id"`
	Stage        string    `json:"stage"` // "layout", "walls", "features", "validation"
	Progress     float64   `json:"progress"` // 0.0 to 1.0
	Message      string    `json:"message,omitempty"`
	Timestamp    time.Time `json:"timestamp"`
}

type GenerationCompletedEvent struct {
	GenerationID  string                 `json:"generation_id"`
	RequestID     string                 `json:"request_id,omitempty"`
	Config        map[string]interface{} `json:"config"`
	RoomCount     int                    `json:"room_count"`
	ConnectionCount int                  `json:"connection_count"`
	Duration      time.Duration          `json:"duration"`
	CompletedAt   time.Time              `json:"completed_at"`
}

type GenerationFailedEvent struct {
	GenerationID string                 `json:"generation_id"`
	RequestID    string                 `json:"request_id,omitempty"`
	Config       map[string]interface{} `json:"config,omitempty"`
	Error        string                 `json:"error"`
	Stage        string                 `json:"stage"`
	FailedAt     time.Time              `json:"failed_at"`
}

type EmergencyFallbackEvent struct {
	GenerationID string                 `json:"generation_id"`
	Trigger      string                 `json:"trigger"`
	FallbackType string                 `json:"fallback_type"`
	Parameters   map[string]interface{} `json:"parameters,omitempty"`
	Timestamp    time.Time              `json:"timestamp"`
}

// Environment lifecycle events
type EnvironmentGeneratedEvent struct {
	EnvironmentID string                 `json:"environment_id"`
	Type          string                 `json:"type"`
	Theme         string                 `json:"theme,omitempty"`
	RoomCount     int                    `json:"room_count"`
	Features      []string               `json:"features,omitempty"`
	Metadata      map[string]interface{} `json:"metadata,omitempty"`
	GeneratedAt   time.Time              `json:"generated_at"`
}

type EnvironmentDestroyedEvent struct {
	EnvironmentID string    `json:"environment_id"`
	Reason        string    `json:"reason,omitempty"`
	DestroyedAt   time.Time `json:"destroyed_at"`
}

// Entity management events
type EnvironmentEntityAddedEvent struct {
	EnvironmentID string                 `json:"environment_id"`
	EntityID      string                 `json:"entity_id"`
	EntityType    string                 `json:"entity_type,omitempty"`
	RoomID        string                 `json:"room_id,omitempty"`
	Properties    map[string]interface{} `json:"properties,omitempty"`
	AddedAt       time.Time              `json:"added_at"`
}

type EnvironmentEntityRemovedEvent struct {
	EnvironmentID string    `json:"environment_id"`
	EntityID      string    `json:"entity_id"`
	RoomID        string    `json:"room_id,omitempty"`
	Reason        string    `json:"reason,omitempty"`
	RemovedAt     time.Time `json:"removed_at"`
}

// Room management events
type EnvironmentRoomAddedEvent struct {
	EnvironmentID string                 `json:"environment_id"`
	RoomID        string                 `json:"room_id"`
	RoomType      string                 `json:"room_type,omitempty"`
	Properties    map[string]interface{} `json:"properties,omitempty"`
	AddedAt       time.Time              `json:"added_at"`
}

type EnvironmentRoomRemovedEvent struct {
	EnvironmentID string    `json:"environment_id"`
	RoomID        string    `json:"room_id"`
	Reason        string    `json:"reason,omitempty"`
	RemovedAt     time.Time `json:"removed_at"`
}

// Feature management events
type FeatureAddedEvent struct {
	EnvironmentID string                 `json:"environment_id"`
	FeatureID     string                 `json:"feature_id"`
	FeatureType   string                 `json:"feature_type"`
	RoomID        string                 `json:"room_id,omitempty"`
	Properties    map[string]interface{} `json:"properties,omitempty"`
	AddedAt       time.Time              `json:"added_at"`
}

type FeatureRemovedEvent struct {
	EnvironmentID string    `json:"environment_id"`
	FeatureID     string    `json:"feature_id"`
	RoomID        string    `json:"room_id,omitempty"`
	Reason        string    `json:"reason,omitempty"`
	RemovedAt     time.Time `json:"removed_at"`
}

// Hazard management events
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