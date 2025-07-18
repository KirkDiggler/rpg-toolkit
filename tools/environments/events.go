package environments

// Event constants following the toolkit's dot notation pattern
// Format: {module}.{category}.{action}

const (
	// Environment generation events
	EventEnvironmentGenerated = "environment.generated"
	EventEnvironmentDestroyed = "environment.destroyed"
	EventEnvironmentExported  = "environment.exported"
	EventEnvironmentImported  = "environment.imported"

	// Environment state changes
	EventThemeChanged               = "environment.theme.changed"
	EventEnvironmentMetadataChanged = "environment.metadata.changed"
	EventLayoutChanged              = "environment.layout.changed"

	// Room template events
	EventRoomTemplateApplied = "environment.room_template.applied"
	EventRoomTemplateRemoved = "environment.room_template.removed"

	// Connection template events
	EventConnectionTemplateApplied = "environment.connection_template.applied"
	EventConnectionTemplateRemoved = "environment.connection_template.removed"

	// Feature events
	EventFeatureAdded       = "environment.feature.added"
	EventFeatureRemoved     = "environment.feature.removed"
	EventFeatureActivated   = "environment.feature.activated"
	EventFeatureDeactivated = "environment.feature.deactivated"

	// Environmental effects
	EventEnvironmentEffectApplied = "environment.effect.applied"
	EventEnvironmentEffectRemoved = "environment.effect.removed"
	EventEnvironmentEffectExpired = "environment.effect.expired"

	// Hazard events
	EventHazardActivated   = "environment.hazard.activated"
	EventHazardDeactivated = "environment.hazard.deactivated"
	EventHazardTriggered   = "environment.hazard.triggered"

	// Entity tracking events (environment-specific)
	EventEnvironmentEntityAdded   = "environment.entity.added"
	EventEnvironmentEntityRemoved = "environment.entity.removed"
	EventEnvironmentEntityMoved   = "environment.entity.moved"

	// Room tracking events (environment-specific)
	EventEnvironmentRoomAdded    = "environment.room.added"
	EventEnvironmentRoomRemoved  = "environment.room.removed"
	EventEnvironmentRoomModified = "environment.room.modified"

	// Query events (for extensibility and monitoring)
	EventEnvironmentQueryExecuted = "environment.query.executed"
	EventEnvironmentQueryFailed   = "environment.query.failed"

	// Generation process events (for monitoring and debugging)
	EventGenerationStarted   = "environment.generation.started"
	EventGenerationCompleted = "environment.generation.completed"
	EventGenerationFailed    = "environment.generation.failed"
	EventGenerationProgress  = "environment.generation.progress"

	// Emergency fallback events (for client notification and debugging)
	EventEmergencyFallbackTriggered = "environment.emergency_fallback.triggered"
)
