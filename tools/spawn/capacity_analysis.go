package spawn

import (
	"context"
	"fmt"

	"github.com/KirkDiggler/rpg-toolkit/events"
	"github.com/KirkDiggler/rpg-toolkit/tools/environments"
	"github.com/KirkDiggler/rpg-toolkit/tools/spatial"
)

// handleCapacityAnalysis performs capacity analysis and scaling per ADR-0013
func (e *BasicSpawnEngine) handleCapacityAnalysis(
	ctx context.Context, roomID string, config SpawnConfig, result *SpawnResult,
) error {
	if e.environmentHandler == nil {
		// No environment handler - skip capacity analysis
		return nil
	}

	// Calculate total entity count from groups
	totalEntityCount := e.calculateTotalEntityCount(config.EntityGroups)

	// Get current room dimensions (simplified for Phase 4)
	currentDimensions := spatial.Dimensions{Width: 10, Height: 10} // Default room size

	// Perform capacity query as specified in ADR-0013
	capacityQuery := environments.CapacityQuery{
		RoomID:              roomID,
		EntityCount:         totalEntityCount,
		RoomSize:            currentDimensions,
		IncludeSplitOptions: true,
		Constraints: environments.CapacityConstraints{
			TargetSpatialFeeling:      environments.SpatialFeelingNormal,
			MinEntitySpacing:          2.0,
			MinMovementSpace:          0.6,
			WallDensityModifier:       0.5,
			RequiredPathwayMultiplier: 1.2,
		},
	}

	// Query environment handler for capacity analysis
	response, err := e.environmentHandler.HandleCapacityQuery(ctx, capacityQuery)
	if err != nil {
		return fmt.Errorf("capacity query failed: %w", err)
	}

	// Handle capacity issues using typed response
	if !response.Satisfied {
		// Try room scaling first
		if config.AdaptiveScaling != nil && config.AdaptiveScaling.Enabled {
			err := e.handleRoomScaling(ctx, roomID, totalEntityCount, currentDimensions, result)
			if err != nil {
				return fmt.Errorf("room scaling failed: %w", err)
			}
		}

		// Extract split recommendations and pass through to client
		if len(response.SplitOptions) > 0 {
			result.SplitRecommendations = e.convertEnvironmentSplitOptions(response.SplitOptions)

			// Publish split recommendation event (passthrough role per ADR)
			e.publishSplitRecommendationEvent(ctx, roomID, result.SplitRecommendations, totalEntityCount)
		}
	}

	return nil
}

// handleRoomScaling performs room scaling when entities don't fit
func (e *BasicSpawnEngine) handleRoomScaling(
	ctx context.Context, roomID string, entityCount int,
	currentDimensions spatial.Dimensions, result *SpawnResult,
) error {
	if e.environmentHandler == nil {
		return fmt.Errorf("environment handler required for room scaling")
	}

	// Use sizing query to calculate optimal dimensions
	sizingQuery := environments.SizingQuery{
		Intent:          environments.GetDefaultSpatialIntentProfile(environments.SpatialFeelingNormal),
		EntityCount:     entityCount,
		MinDimensions:   currentDimensions,
		AdditionalSpace: 1.2, // 20% buffer space
	}

	newDimensions, err := e.environmentHandler.HandleSizingQuery(ctx, sizingQuery)
	if err != nil {
		return fmt.Errorf("sizing query failed: %w", err)
	}

	// Calculate scaling factor
	scaleFactor := newDimensions.Width / currentDimensions.Width

	// Record room modification
	modification := RoomModification{
		Type:     "scaled",
		RoomID:   roomID,
		OldValue: currentDimensions,
		NewValue: newDimensions,
		Reason:   fmt.Sprintf("Scaled to accommodate %d entities (factor: %.2f)", entityCount, scaleFactor),
	}
	result.RoomModifications = append(result.RoomModifications, modification)

	// Emit scaling event with detailed reasoning
	e.publishRoomScalingEvent(ctx, roomID, currentDimensions, newDimensions, entityCount, scaleFactor)

	return nil
}

// calculateTotalEntityCount calculates total entities from groups
func (e *BasicSpawnEngine) calculateTotalEntityCount(groups []EntityGroup) int {
	total := 0
	for _, group := range groups {
		if group.Quantity.Fixed != nil {
			total += *group.Quantity.Fixed
		} else {
			// For Phase 4, assume 1 entity if no fixed quantity
			total++
		}
	}
	return total
}

// convertEnvironmentSplitOptions converts environment split options to spawn types
func (e *BasicSpawnEngine) convertEnvironmentSplitOptions(envSplits []environments.RoomSplit) []RoomSplit {
	splits := make([]RoomSplit, 0, len(envSplits))

	for _, envSplit := range envSplits {
		split := RoomSplit{
			SuggestedSize:      envSplit.SuggestedSize,
			ConnectionPoints:   envSplit.ConnectionPoints,
			SplitReason:        envSplit.SplitReason,
			EntityDistribution: envSplit.RecommendedEntityDistribution,
		}

		splits = append(splits, split)
	}

	return splits
}

// publishSplitRecommendationEvent publishes split recommendation event
func (e *BasicSpawnEngine) publishSplitRecommendationEvent(
	ctx context.Context, roomID string, splits []RoomSplit, entityCount int,
) {
	if !e.enableEvents || e.eventBus == nil {
		return
	}

	roomEntity := &SimpleRoomEntity{id: roomID, roomType: "spawn_target"}
	event := events.NewGameEvent("spawn.split.recommended", roomEntity, nil)
	event.Context().Set("room_id", roomID)
	event.Context().Set("entity_count", entityCount)
	event.Context().Set("split_count", len(splits))
	event.Context().Set("reason", "Capacity analysis suggests room splitting")

	_ = e.eventBus.Publish(ctx, event)
}

// publishRoomScalingEvent publishes room scaling event
func (e *BasicSpawnEngine) publishRoomScalingEvent(
	ctx context.Context, roomID string, oldDimensions, newDimensions spatial.Dimensions,
	entityCount int, scaleFactor float64,
) {
	if !e.enableEvents || e.eventBus == nil {
		return
	}

	roomEntity := &SimpleRoomEntity{id: roomID, roomType: "spawn_target"}
	event := events.NewGameEvent("spawn.room.scaled", roomEntity, nil)
	event.Context().Set("room_id", roomID)
	event.Context().Set("entity_count", entityCount)
	event.Context().Set("scale_factor", scaleFactor)
	event.Context().Set("old_width", oldDimensions.Width)
	event.Context().Set("old_height", oldDimensions.Height)
	event.Context().Set("new_width", newDimensions.Width)
	event.Context().Set("new_height", newDimensions.Height)

	_ = e.eventBus.Publish(ctx, event)
}
