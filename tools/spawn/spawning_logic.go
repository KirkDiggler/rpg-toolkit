package spawn

import (
	"context"
	"fmt"

	"github.com/KirkDiggler/rpg-toolkit/core"
	"github.com/KirkDiggler/rpg-toolkit/tools/selectables"
	"github.com/KirkDiggler/rpg-toolkit/tools/spatial"
)

// Core spawning logic implementation following ADR patterns

// analyzeRoomStructureUnsafe determines if we're dealing with single or split rooms
// Purpose: Split-awareness - detect room configuration for appropriate spawning strategy
func (e *BasicSpawnEngine) analyzeRoomStructureUnsafe(roomOrGroup interface{}) (RoomStructureInfo, error) {
	switch v := roomOrGroup.(type) {
	case string:
		// Single room ID
		return RoomStructureInfo{
			IsSplit:        false,
			ConnectedRooms: []string{v},
			PrimaryRoomID:  v,
			TotalCapacity:  0, // Will be calculated during capacity analysis
		}, nil

	case []string:
		// Multiple connected rooms (split configuration)
		if len(v) == 0 {
			return RoomStructureInfo{}, fmt.Errorf("empty room list provided")
		}
		
		return RoomStructureInfo{
			IsSplit:        len(v) > 1,
			ConnectedRooms: v,
			PrimaryRoomID:  v[0], // Use first room as primary
			TotalCapacity:  0,    // Will be calculated during capacity analysis
		}, nil

	default:
		return RoomStructureInfo{}, fmt.Errorf("unsupported room specification type: %T", roomOrGroup)
	}
}

// handleCapacityAnalysisUnsafe manages room scaling and split recommendations
// Purpose: Implement capacity-first approach from ADR with environment package integration
func (e *BasicSpawnEngine) handleCapacityAnalysisUnsafe(ctx context.Context, roomOrGroup interface{}, config SpawnConfig, result *SpawnResult) error {
	if e.environmentQueryHandler == nil {
		return nil // Skip capacity analysis if no environment handler
	}

	// Calculate total entity count needed
	totalEntityCount := 0
	for _, group := range config.EntityGroups {
		count, err := e.calculateGroupQuantityUnsafe(group.Quantity)
		if err != nil {
			return fmt.Errorf("failed to calculate quantity for group %s: %w", group.ID, err)
		}
		totalEntityCount += count
	}

	// Handle capacity analysis differently for single vs split rooms
	if result.RoomStructure.IsSplit {
		return e.handleSplitRoomCapacityUnsafe(ctx, result.RoomStructure.ConnectedRooms, totalEntityCount, config, result)
	} else {
		return e.handleSingleRoomCapacityUnsafe(ctx, result.RoomStructure.PrimaryRoomID, totalEntityCount, config, result)
	}
}

// handleSingleRoomCapacityUnsafe manages single room capacity analysis and scaling
func (e *BasicSpawnEngine) handleSingleRoomCapacityUnsafe(ctx context.Context, roomID string, entityCount int, config SpawnConfig, result *SpawnResult) error {
	// Get current room dimensions from spatial system
	// This would typically query the spatial system, for now we'll use a placeholder
	currentDimensions := spatial.Dimensions{Width: 10, Height: 10} // Placeholder
	
	// Check if current room can handle the entities (always request split options per ADR)
	capacityQuery := map[string]interface{}{
		"RoomID":              roomID,
		"RoomSize":            currentDimensions,
		"EntityCount":         entityCount,
		"IncludeSplitOptions": true, // Always get split recommendations per ADR
		"Constraints": map[string]interface{}{
			"TargetSpatialFeeling":      "normal",
			"MinEntitySpacing":          2.0,
			"MinMovementSpace":          0.6,
			"WallDensityModifier":       0.5,
			"RequiredPathwayMultiplier": 1.2,
		},
	}

	response, err := e.environmentQueryHandler.HandleCapacityQuery(ctx, capacityQuery)
	if err != nil {
		return fmt.Errorf("capacity query failed: %w", err)
	}

	// Parse response from interface{}
	responseMap, ok := response.(map[string]interface{})
	if !ok {
		return fmt.Errorf("invalid capacity query response format")
	}

	// Store split recommendations for passthrough to client (convert from environment type)
	if splitOptions, exists := responseMap["SplitOptions"]; exists {
		if optionSlice, ok := splitOptions.([]interface{}); ok {
			result.SplitRecommendations = e.convertEnvironmentSplitOptionsUnsafe(optionSlice)
		}
	}

	// If room is too small, calculate optimal size and scale
	satisfied, _ := responseMap["Satisfied"].(bool)
	if !satisfied {
		if err := e.handleRoomScalingUnsafe(ctx, roomID, entityCount, currentDimensions, result); err != nil {
			return fmt.Errorf("room scaling failed: %w", err)
		}
	}

	// Communicate split recommendations to client (passthrough role per ADR)
	if len(result.SplitRecommendations) > 0 {
		e.publishSplitRecommendationEventUnsafe(ctx, roomID, result.SplitRecommendations, entityCount)
	}

	return nil
}

// handleRoomScalingUnsafe performs room scaling when entities don't fit
func (e *BasicSpawnEngine) handleRoomScalingUnsafe(ctx context.Context, roomID string, entityCount int, currentDimensions spatial.Dimensions, result *SpawnResult) error {
	// Calculate optimal room size using environment package
	sizingQuery := map[string]interface{}{
		"Intent":          "normal_spatial_feeling",
		"EntityCount":     entityCount,
		"MinDimensions":   currentDimensions, // Don't go smaller than original
		"AdditionalSpace": 1.2,               // 20% buffer space
	}

	newDimensions, err := e.environmentQueryHandler.HandleSizingQuery(ctx, sizingQuery)
	if err != nil {
		return fmt.Errorf("sizing query failed: %w", err)
	}

	// Apply scaling (this would typically call spatial module room modification)
	scaleFactor := newDimensions.Width / currentDimensions.Width

	// Record room modification
	modification := RoomModification{
		Type:      "scaled",
		RoomID:    roomID,
		OldValue:  currentDimensions,
		NewValue:  newDimensions,
		Reason:    fmt.Sprintf("Scaled to accommodate %d entities", entityCount),
	}
	result.RoomModifications = append(result.RoomModifications, modification)

	// Emit scaling event with detailed reasoning
	e.publishRoomScalingEventUnsafe(ctx, roomID, currentDimensions, newDimensions, entityCount, scaleFactor)

	return nil
}

// handleSplitRoomCapacityUnsafe manages capacity analysis for multiple connected rooms
func (e *BasicSpawnEngine) handleSplitRoomCapacityUnsafe(ctx context.Context, connectedRooms []string, entityCount int, config SpawnConfig, result *SpawnResult) error {
	// For split rooms, analyze total capacity across all connected rooms
	// This is a simplified implementation - full version would query each room individually
	
	// Calculate total capacity across all rooms (placeholder implementation)
	totalCapacity := len(connectedRooms) * 10 // Simplified calculation
	result.RoomStructure.TotalCapacity = totalCapacity

	// Split rooms typically have adequate capacity by design
	// But we still provide split recommendations if rooms could be further optimized
	if entityCount > totalCapacity {
		// Even split rooms are overcrowded - provide recommendations
		avgDimensions := spatial.Dimensions{Width: 10, Height: 10} // Placeholder
		
		capacityQuery := map[string]interface{}{
			"RoomSize":            avgDimensions,
			"EntityCount":         entityCount,
			"IncludeSplitOptions": true,
		}
		
		response, err := e.environmentQueryHandler.HandleCapacityQuery(ctx, capacityQuery)
		if err == nil {
			if responseMap, ok := response.(map[string]interface{}); ok {
				if splitOptions, exists := responseMap["SplitOptions"]; exists {
					if optionSlice, ok := splitOptions.([]interface{}); ok {
						result.SplitRecommendations = e.convertEnvironmentSplitOptionsUnsafe(optionSlice)
					}
				}
			}
		}
	}

	return nil
}

// selectEntitiesFromGroupsUnsafe handles entity selection using selectables integration
func (e *BasicSpawnEngine) selectEntitiesFromGroupsUnsafe(ctx context.Context, entityGroups []EntityGroup) ([]core.Entity, error) {
	var allEntities []core.Entity

	for _, group := range entityGroups {
		var groupEntities []core.Entity
		var err error

		if len(group.Entities) > 0 {
			// Use pre-provided entities
			groupEntities, err = e.selectFromProvidedEntitiesUnsafe(group)
		} else if group.SelectionTable != "" {
			// Use selectables table
			groupEntities, err = e.selectFromTableUnsafe(ctx, group)
		} else {
			return nil, fmt.Errorf("entity group %s has no entities or selection table", group.ID)
		}

		if err != nil {
			return nil, fmt.Errorf("failed to select entities for group %s: %w", group.ID, err)
		}

		allEntities = append(allEntities, groupEntities...)
	}

	return allEntities, nil
}

// selectFromProvidedEntitiesUnsafe selects entities from pre-provided list
func (e *BasicSpawnEngine) selectFromProvidedEntitiesUnsafe(group EntityGroup) ([]core.Entity, error) {
	quantity, err := e.calculateGroupQuantityUnsafe(group.Quantity)
	if err != nil {
		return nil, err
	}

	if quantity > len(group.Entities) {
		return nil, fmt.Errorf("requested %d entities but group %s only has %d", quantity, group.ID, len(group.Entities))
	}

	// Simple selection - take first N entities
	// In full implementation, could add random selection, weighting, etc.
	return group.Entities[:quantity], nil
}

// selectFromTableUnsafe selects entities using selectables table
func (e *BasicSpawnEngine) selectFromTableUnsafe(ctx context.Context, group EntityGroup) ([]core.Entity, error) {
	if e.selectablesRegistry == nil {
		return nil, fmt.Errorf("no selectables registry configured")
	}

	table, err := e.selectablesRegistry.GetTable(group.SelectionTable)
	if err != nil {
		return nil, fmt.Errorf("selection table %s not found: %w", group.SelectionTable, err)
	}

	quantity, err := e.calculateGroupQuantityUnsafe(group.Quantity)
	if err != nil {
		return nil, err
	}

	// Create selection context (would typically include dice roller and game state)
	selectionCtx := selectables.NewBasicSelectionContext() // Placeholder implementation

	// Perform selection based on group type
	switch group.Type {
	case "treasure":
		// Use unique selection for treasure (no duplicates)
		return table.SelectUnique(selectionCtx, quantity)
	case "enemy", "player":
		// Allow duplicates for enemies and players
		return table.SelectMany(selectionCtx, quantity)
	default:
		// Default to allowing duplicates
		return table.SelectMany(selectionCtx, quantity)
	}
}

// calculateGroupQuantityUnsafe determines how many entities to select for a group
func (e *BasicSpawnEngine) calculateGroupQuantityUnsafe(spec QuantitySpec) (int, error) {
	if spec.Fixed != nil {
		return *spec.Fixed, nil
	}

	if spec.MinMax != nil {
		// For now, return midpoint - in full implementation would use random selection
		return (spec.MinMax.Min + spec.MinMax.Max) / 2, nil
	}

	if spec.DiceRoll != nil {
		// For now, return fixed value - in full implementation would parse and roll dice
		// This would integrate with the dice package from the toolkit
		return 3, nil // Placeholder
	}

	return 1, nil // Default
}

// populateSingleRoomUnsafe handles entity placement in a single room
func (e *BasicSpawnEngine) populateSingleRoomUnsafe(ctx context.Context, roomID string, entities []core.Entity, config SpawnConfig, result *SpawnResult) error {
	// This is where the actual entity placement logic would go
	// For now, we'll create a basic implementation that simulates placement
	
	for i, entity := range entities {
		position := spatial.Position{X: float64(i), Y: float64(i)} // Simplified positioning
		
		spawnedEntity := SpawnedEntity{
			Entity:      entity,
			Position:    position,
			RoomID:      roomID,
			GroupID:     fmt.Sprintf("group_%d", i),
			SpawnReason: "basic_placement",
		}
		
		result.SpawnedEntities = append(result.SpawnedEntities, spawnedEntity)
	}

	return nil
}

// populateSplitRoomsUnsafe handles entity placement across multiple connected rooms
func (e *BasicSpawnEngine) populateSplitRoomsUnsafe(ctx context.Context, connectedRooms []string, entities []core.Entity, config SpawnConfig, result *SpawnResult) error {
	// Distribute entities across connected rooms
	// For now, use simple round-robin distribution
	
	for i, entity := range entities {
		roomIndex := i % len(connectedRooms)
		roomID := connectedRooms[roomIndex]
		position := spatial.Position{X: float64(i % 10), Y: float64(i / 10)} // Simplified positioning
		
		spawnedEntity := SpawnedEntity{
			Entity:      entity,
			Position:    position,
			RoomID:      roomID,
			GroupID:     fmt.Sprintf("group_%d", i),
			SpawnReason: "split_room_placement",
		}
		
		result.SpawnedEntities = append(result.SpawnedEntities, spawnedEntity)
	}

	return nil
}

// convertHelperConfigUnsafe converts helper config to full spawn config
func (e *BasicSpawnEngine) convertHelperConfigUnsafe(entities []core.Entity, helperConfig HelperConfig) SpawnConfig {
	config := SpawnConfig{
		EntityGroups: []EntityGroup{
			{
				ID:       "helper_group",
				Type:     "mixed",
				Entities: entities,
				Quantity: QuantitySpec{Fixed: &[]int{len(entities)}[0]},
			},
		},
		Pattern:  PatternScattered,
		Strategy: StrategyBalanced,
	}

	// Apply helper config settings
	if helperConfig.TeamSeparation {
		config.Pattern = PatternTeamBased
		config.TeamConfiguration = &TeamConfig{
			CohesionRules: TeamCohesionRules{
				KeepFriendliesTogether: true,
				KeepEnemiesTogether:    true,
				MinTeamSeparation:      5.0,
			},
		}
	}

	if helperConfig.PlayerChoice {
		config.Pattern = PatternPlayerChoice
	}

	if helperConfig.AutoScale {
		config.AdaptiveScaling = &ScalingConfig{
			Enabled:       true,
			ScalingFactor: 1.2,
			EmitEvents:    true,
		}
	}

	return config
}

// convertEnvironmentSplitOptionsUnsafe converts environment package RoomSplit to local type
func (e *BasicSpawnEngine) convertEnvironmentSplitOptionsUnsafe(envSplits []interface{}) []RoomSplit {
	localSplits := make([]RoomSplit, len(envSplits))
	for i := range envSplits {
		// For now, create placeholder RoomSplit - in full implementation would parse interface{} data
		localSplits[i] = RoomSplit{
			Reason:    "capacity_analysis",
			SplitType: "horizontal",
			Dimensions: []spatial.Dimensions{
				{Width: 10, Height: 5},
				{Width: 10, Height: 5},
			},
			Benefits:   []string{"Improved capacity", "Better spatial feeling"},
			Complexity: 0.5,
			SuggestedSize: spatial.Dimensions{Width: 10, Height: 5},
		}
	}
	return localSplits
}