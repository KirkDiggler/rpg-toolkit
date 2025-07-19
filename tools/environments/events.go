package environments

// Event constants following the toolkit's dot notation pattern
// Format: {module}.{category}.{action}

const (
	// EventEnvironmentGenerated is published when a new environment is successfully generated.
	EventEnvironmentGenerated = "environment.generated"
	// EventEnvironmentDestroyed is published when an environment is destroyed or cleaned up.
	EventEnvironmentDestroyed = "environment.destroyed"
	// EventEnvironmentExported is published when an environment is exported to a file or byte array.
	EventEnvironmentExported = "environment.exported"
	// EventEnvironmentImported is published when an environment is imported from a file or byte array.
	EventEnvironmentImported = "environment.imported"

	// EventThemeChanged is published when the environment's theme is changed.
	EventThemeChanged = "environment.theme.changed"
	// EventEnvironmentMetadataChanged is published when the environment's metadata is changed.
	EventEnvironmentMetadataChanged = "environment.metadata.changed"
	// EventLayoutChanged is published when the layout of the environment is changed.
	EventLayoutChanged = "environment.layout.changed"

	// EventRoomTemplateApplied is published when a room template is applied to create a new room.
	EventRoomTemplateApplied = "environment.room_template.applied"
	// EventRoomTemplateRemoved is published when a room template is removed.
	EventRoomTemplateRemoved = "environment.room_template.removed"

	// EventConnectionTemplateApplied is published when a connection template is applied to create a new connection.
	EventConnectionTemplateApplied = "environment.connection_template.applied"
	// EventConnectionTemplateRemoved is published when a connection template is removed.
	EventConnectionTemplateRemoved = "environment.connection_template.removed"

	// EventFeatureAdded is published when a feature is added to the environment.
	EventFeatureAdded = "environment.feature.added"
	// EventFeatureRemoved is published when a feature is removed from the environment.
	EventFeatureRemoved = "environment.feature.removed"
	// EventFeatureActivated is published when a feature is activated in the environment.
	EventFeatureActivated = "environment.feature.activated"
	// EventFeatureDeactivated is published when a feature is deactivated in the environment.
	EventFeatureDeactivated = "environment.feature.deactivated"

	// EventEnvironmentEffectApplied is published when an environmental effect is applied.
	EventEnvironmentEffectApplied = "environment.effect.applied"
	// EventEnvironmentEffectRemoved is published when an environmental effect is removed.
	EventEnvironmentEffectRemoved = "environment.effect.removed"
	// EventEnvironmentEffectExpired is published when an environmental effect expires.
	EventEnvironmentEffectExpired = "environment.effect.expired"

	// EventHazardActivated is published when a hazard becomes active in the environment.
	EventHazardActivated = "environment.hazard.activated"
	// EventHazardDeactivated is published when a hazard is deactivated in the environment.
	EventHazardDeactivated = "environment.hazard.deactivated"
	// EventHazardTriggered is published when a hazard is triggered in the environment.
	EventHazardTriggered = "environment.hazard.triggered"

	// EventEnvironmentEntityAdded is published when an entity is added to the environment.
	EventEnvironmentEntityAdded = "environment.entity.added"
	// EventEnvironmentEntityRemoved is published when an entity is removed from the environment.
	EventEnvironmentEntityRemoved = "environment.entity.removed"
	// EventEnvironmentEntityMoved is published when an entity is moved in the environment.
	EventEnvironmentEntityMoved = "environment.entity.moved"

	// EventEnvironmentRoomAdded is published when a room is added to the environment.
	EventEnvironmentRoomAdded = "environment.room.added"
	// EventEnvironmentRoomRemoved is published when a room is removed from the environment.
	EventEnvironmentRoomRemoved = "environment.room.removed"
	// EventEnvironmentRoomModified is published when a room is modified in the environment.
	EventEnvironmentRoomModified = "environment.room.modified"

	// EventEnvironmentQueryExecuted is published when an environment query is executed.
	EventEnvironmentQueryExecuted = "environment.query.executed"
	// EventEnvironmentQueryFailed is published when an environment query fails to execute.
	EventEnvironmentQueryFailed = "environment.query.failed"

	// EventGenerationStarted is published when environment generation begins.
	EventGenerationStarted = "environment.generation.started"
	// EventGenerationCompleted is published when environment generation completes successfully.
	EventGenerationCompleted = "environment.generation.completed"
	// EventGenerationFailed is published when environment generation fails.
	EventGenerationFailed = "environment.generation.failed"
	// EventGenerationProgress is published to indicate the progress of environment generation.
	EventGenerationProgress = "environment.generation.progress"

	// EventEmergencyFallbackTriggered is published when emergency fallback is triggered during generation.
	EventEmergencyFallbackTriggered = "environment.emergency_fallback.triggered"
)
