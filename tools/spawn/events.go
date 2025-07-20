package spawn

// Event constants following toolkit patterns
// Purpose: Define spawn-related events for observability and game integration
const (
	// Core spawn operation events
	EventSpawnOperationStarted   = "spawn.operation.started"
	EventSpawnOperationCompleted = "spawn.operation.completed"
	EventEntitySpawned          = "spawn.entity.spawned"
	EventEntitySpawnFailed      = "spawn.entity.failed"

	// Room modification events
	EventRoomModified    = "spawn.room.modified"
	EventRoomScaled      = "spawn.room.scaled"

	// Split-awareness events (passthrough to client)
	EventSplitRecommended = "spawn.split.recommended"      // Environment suggests splitting
	EventMultiRoomSpawn   = "spawn.multiroom.completed"    // Split room spawning completed

	// Formation and team events
	EventFormationApplied = "spawn.formation.applied"
	EventTeamSpawned      = "spawn.team.spawned"

	// Constraint and quality events
	EventConstraintViolation = "spawn.constraint.violation"
	EventQualityDegraded     = "spawn.quality.degraded"

	// Player choice events
	EventPlayerSpawnRequested = "spawn.player.requested"   // Player chose spawn position
	EventPlayerSpawnDenied    = "spawn.player.denied"      // Player choice invalid
)

// Event data structures for spawn events
// Purpose: Structured event data following toolkit patterns

// SpawnOperationEventData contains data for spawn operation events
type SpawnOperationEventData struct {
	RoomOrGroup       interface{}       `json:"room_or_group"`
	Configuration     SpawnConfig       `json:"configuration"`
	Result           SpawnResult       `json:"result"`
	TotalEntities    int               `json:"total_entities"`
	FailedEntities   int               `json:"failed_entities"`
	ExecutionTime    interface{}       `json:"execution_time"` // time.Duration
	RoomModifications []RoomModification `json:"room_modifications"`
	UsedSplitRooms   bool              `json:"used_split_rooms"`
}

// EntitySpawnEventData contains data for individual entity spawn events
type EntitySpawnEventData struct {
	Entity      interface{}      `json:"entity"`      // core.Entity
	Position    interface{}      `json:"position"`    // spatial.Position
	RoomID      string           `json:"room_id"`     // Specific room in split config
	GroupID     string           `json:"group_id"`
	TeamID      string           `json:"team_id,omitempty"`
	SpawnReason string           `json:"spawn_reason"`
}

// EntitySpawnFailureEventData contains data for spawn failure events
type EntitySpawnFailureEventData struct {
	EntityType        string           `json:"entity_type"`
	GroupID          string           `json:"group_id"`
	Reason           string           `json:"reason"`
	AttemptedPosition interface{}      `json:"attempted_position,omitempty"` // *spatial.Position
	ConstraintsFailed []string         `json:"constraints_failed,omitempty"`
	RoomID           string           `json:"room_id"`
}

// SplitRecommendationEventData contains data for split recommendation events
type SplitRecommendationEventData struct {
	OriginalRoomID   string           `json:"original_room_id"`
	SplitOptions     interface{}      `json:"split_options"`     // []environments.RoomSplit
	EntityCount      int              `json:"entity_count"`
	Reason           string           `json:"reason"`
	CapacityAnalysis interface{}      `json:"capacity_analysis"` // environments.CapacityQueryResponse
}

// RoomModificationEventData contains data for room modification events
type RoomModificationEventData struct {
	RoomID           string      `json:"room_id"`
	ModificationType string      `json:"modification_type"` // "scaled", "split", "modified"
	OldDimensions    interface{} `json:"old_dimensions"`    // spatial.Dimensions
	NewDimensions    interface{} `json:"new_dimensions"`    // spatial.Dimensions
	ScaleFactor      float64     `json:"scale_factor"`
	Reason           string      `json:"reason"`
	EntityCount      int         `json:"entity_count"`
	Alternatives     []string    `json:"alternatives"`
}

// TeamSpawnEventData contains data for team-based spawn events
type TeamSpawnEventData struct {
	TeamID          string      `json:"team_id"`
	EntityCount     int         `json:"entity_count"`
	SpawnArea       interface{} `json:"spawn_area"`       // spatial.Rectangle
	CohesionAchieved float64    `json:"cohesion_achieved"` // 0.0-1.0
	SeparationDistance float64  `json:"separation_distance"`
	FormationUsed   string      `json:"formation_used,omitempty"`
}

// FormationEventData contains data for formation application events
type FormationEventData struct {
	FormationName   string      `json:"formation_name"`
	EntityCount     int         `json:"entity_count"`
	CenterPosition  interface{} `json:"center_position"`  // spatial.Position
	Scaling         interface{} `json:"scaling"`          // FormationScaling
	QualityScore    float64     `json:"quality_score"`    // How well formation was applied
	Adjustments     []string    `json:"adjustments"`      // What was modified
}

// ConstraintViolationEventData contains data for constraint violation events
type ConstraintViolationEventData struct {
	ConstraintType   string      `json:"constraint_type"`
	ViolatedRule     string      `json:"violated_rule"`
	AffectedEntities []string    `json:"affected_entities"`
	Severity         string      `json:"severity"`         // "minor", "major", "critical"
	Resolution       string      `json:"resolution"`       // How it was handled
	ImpactArea       interface{} `json:"impact_area"`      // spatial.Rectangle
}

// PlayerSpawnEventData contains data for player spawn choice events
type PlayerSpawnEventData struct {
	PlayerID        string      `json:"player_id"`
	RequestedZone   string      `json:"requested_zone"`
	RequestedPosition interface{} `json:"requested_position"` // spatial.Position
	Granted         bool        `json:"granted"`
	DenialReason    string      `json:"denial_reason,omitempty"`
	AlternativePositions []interface{} `json:"alternative_positions,omitempty"` // []spatial.Position
}