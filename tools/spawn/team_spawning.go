package spawn

import (
	"context"
	"fmt"
	"math"

	"github.com/KirkDiggler/rpg-toolkit/core"
	"github.com/KirkDiggler/rpg-toolkit/tools/spatial"
)

// Team-based spawning implementation following user requirements
// Purpose: Keep friendlies together, keep enemies together, maintain team separation

// Helper functions for spatial.Rectangle field access (for backward compatibility)
func getRectCenter(rect spatial.Rectangle) (float64, float64) {
	return rect.Position.X + rect.Dimensions.Width/2, rect.Position.Y + rect.Dimensions.Height/2
}

func getRectTopLeft(rect spatial.Rectangle) spatial.Position {
	return rect.Position
}

func getRectWidth(rect spatial.Rectangle) float64 {
	return rect.Dimensions.Width
}

func getRectHeight(rect spatial.Rectangle) float64 {
	return rect.Dimensions.Height
}

func rectIntersects(a, b spatial.Rectangle, minSeparation float64) bool {
	return a.Position.X < b.Position.X+b.Dimensions.Width+minSeparation &&
		a.Position.X+a.Dimensions.Width+minSeparation > b.Position.X &&
		a.Position.Y < b.Position.Y+b.Dimensions.Height+minSeparation &&
		a.Position.Y+a.Dimensions.Height+minSeparation > b.Position.Y
}

func clampPosition(x, y float64, area spatial.Rectangle) spatial.Position {
	if x < area.Position.X {
		x = area.Position.X
	}
	if x > area.Position.X+area.Dimensions.Width {
		x = area.Position.X + area.Dimensions.Width
	}
	if y < area.Position.Y {
		y = area.Position.Y
	}
	if y > area.Position.Y+area.Dimensions.Height {
		y = area.Position.Y + area.Dimensions.Height
	}
	return spatial.Position{X: x, Y: y}
}

// applyTeamBasedSpawningUnsafe handles team cohesion and separation
func (e *BasicSpawnEngine) applyTeamBasedSpawningUnsafe(ctx context.Context, roomID string, entities []core.Entity, config SpawnConfig, result *SpawnResult) error {
	if config.TeamConfiguration == nil {
		// No team configuration - fall back to regular spawning
		return e.populateSingleRoomUnsafe(ctx, roomID, entities, config, result)
	}

	teamConfig := config.TeamConfiguration

	// Group entities by team
	entityTeams := e.groupEntitiesByTeamUnsafe(entities, config.EntityGroups, teamConfig.Teams)

	// Determine spawn areas for each team
	teamSpawnAreas, err := e.calculateTeamSpawnAreasUnsafe(roomID, entityTeams, teamConfig)
	if err != nil {
		return fmt.Errorf("failed to calculate team spawn areas: %w", err)
	}

	// Spawn each team in their designated area
	for teamID, teamEntities := range entityTeams {
		spawnArea, exists := teamSpawnAreas[teamID]
		if !exists {
			return fmt.Errorf("no spawn area calculated for team %s", teamID)
		}

		err := e.spawnTeamInAreaUnsafe(ctx, roomID, teamID, teamEntities, spawnArea, teamConfig, result)
		if err != nil {
			return fmt.Errorf("failed to spawn team %s: %w", teamID, err)
		}

		// Publish team spawn event
		roomEntity := &SpawnRoomEntity{id: roomID, roomType: "team_spawn_target"}
		e.publishTeamSpawnEventUnsafe(ctx, roomEntity, teamID, result.SpawnedEntities, spawnArea)
	}

	return nil
}

// groupEntitiesByTeamUnsafe organizes entities into teams based on configuration
func (e *BasicSpawnEngine) groupEntitiesByTeamUnsafe(entities []core.Entity, entityGroups []EntityGroup, teams []Team) map[string][]core.Entity {
	entityTeams := make(map[string][]core.Entity)

	// Create mapping from entity group to team
	groupToTeam := make(map[string]string)
	for _, group := range entityGroups {
		if group.TeamID != "" {
			groupToTeam[group.ID] = group.TeamID
		}
	}

	// Create mapping from entity type to team
	typeToTeam := make(map[string]string)
	for _, team := range teams {
		for _, entityType := range team.EntityTypes {
			typeToTeam[entityType] = team.ID
		}
	}

	// Group entities by team
	for i, entity := range entities {
		teamID := "neutral" // Default team

		// Try to find team by entity type
		if assignedTeam, exists := typeToTeam[entity.GetType()]; exists {
			teamID = assignedTeam
		}

		// For entities from groups, use group's team assignment if available
		if i < len(entityGroups) {
			if groupTeam, exists := groupToTeam[entityGroups[i].ID]; exists {
				teamID = groupTeam
			}
		}

		entityTeams[teamID] = append(entityTeams[teamID], entity)
	}

	return entityTeams
}

// calculateTeamSpawnAreasUnsafe determines where each team should spawn
func (e *BasicSpawnEngine) calculateTeamSpawnAreasUnsafe(roomID string, entityTeams map[string][]core.Entity, teamConfig *TeamConfig) (map[string]spatial.Rectangle, error) {
	// Get room dimensions (placeholder - would query spatial system)
	roomDimensions := spatial.Dimensions{Width: 20, Height: 20}

	teamIDs := make([]string, 0, len(entityTeams))
	for teamID := range entityTeams {
		teamIDs = append(teamIDs, teamID)
	}

	switch teamConfig.SeparationRules.TeamPlacement {
	case TeamPlacementCorners:
		return e.calculateCornerSpawnAreasUnsafe(roomDimensions, teamIDs, teamConfig)
	case TeamPlacementOppositeSides:
		return e.calculateOppositeSideSpawnAreasUnsafe(roomDimensions, teamIDs, teamConfig)
	case TeamPlacementRandom:
		return e.calculateRandomSpawnAreasUnsafe(roomDimensions, teamIDs, teamConfig)
	default:
		return e.calculateOppositeSideSpawnAreasUnsafe(roomDimensions, teamIDs, teamConfig)
	}
}

// calculateCornerSpawnAreasUnsafe places teams in room corners
func (e *BasicSpawnEngine) calculateCornerSpawnAreasUnsafe(roomDimensions spatial.Dimensions, teamIDs []string, teamConfig *TeamConfig) (map[string]spatial.Rectangle, error) {
	teamAreas := make(map[string]spatial.Rectangle)
	
	// Define corner positions
	corners := []spatial.Position{
		{X: 0, Y: 0},                                    // Top-left
		{X: roomDimensions.Width - 5, Y: 0},             // Top-right
		{X: 0, Y: roomDimensions.Height - 5},            // Bottom-left
		{X: roomDimensions.Width - 5, Y: roomDimensions.Height - 5}, // Bottom-right
	}

	// Calculate area size based on minimum separation
	minSeparation := teamConfig.SeparationRules.MinTeamDistance
	if minSeparation == 0 {
		minSeparation = 5.0 // Default
	}

	areaSize := minSeparation / 2

	for i, teamID := range teamIDs {
		if i >= len(corners) {
			break // More teams than corners
		}

		corner := corners[i]
		teamAreas[teamID] = spatial.Rectangle{
			Position: corner,
			Dimensions: spatial.Dimensions{
				Width:  areaSize,
				Height: areaSize,
			},
		}
	}

	return teamAreas, nil
}

// calculateOppositeSideSpawnAreasUnsafe places teams on opposite sides of the room
func (e *BasicSpawnEngine) calculateOppositeSideSpawnAreasUnsafe(roomDimensions spatial.Dimensions, teamIDs []string, teamConfig *TeamConfig) (map[string]spatial.Rectangle, error) {
	teamAreas := make(map[string]spatial.Rectangle)

	if len(teamIDs) == 0 {
		return teamAreas, nil
	}

	// For simplicity, handle up to 4 teams (north, south, east, west)
	sides := []struct {
		position spatial.Position
		width    float64
		height   float64
	}{
		{spatial.Position{X: 0, Y: 0}, roomDimensions.Width, 5},                           // North
		{spatial.Position{X: 0, Y: roomDimensions.Height - 5}, roomDimensions.Width, 5},  // South
		{spatial.Position{X: 0, Y: 5}, 5, roomDimensions.Height - 10},                    // West
		{spatial.Position{X: roomDimensions.Width - 5, Y: 5}, 5, roomDimensions.Height - 10}, // East
	}

	for i, teamID := range teamIDs {
		if i >= len(sides) {
			break
		}

		side := sides[i]
		teamAreas[teamID] = spatial.Rectangle{
			Position: side.position,
			Dimensions: spatial.Dimensions{
				Width:  side.width,
				Height: side.height,
			},
		}
	}

	return teamAreas, nil
}

// calculateRandomSpawnAreasUnsafe places teams randomly with separation
func (e *BasicSpawnEngine) calculateRandomSpawnAreasUnsafe(roomDimensions spatial.Dimensions, teamIDs []string, teamConfig *TeamConfig) (map[string]spatial.Rectangle, error) {
	teamAreas := make(map[string]spatial.Rectangle)

	minSeparation := teamConfig.SeparationRules.MinTeamDistance
	if minSeparation == 0 {
		minSeparation = 5.0
	}

	areaSize := minSeparation / 2
	usedAreas := make([]spatial.Rectangle, 0)

	for _, teamID := range teamIDs {
		// Try to find a random position that doesn't overlap with existing areas
		for attempts := 0; attempts < 100; attempts++ {
			// Simplified random positioning
			x := float64(attempts%10) * 2 // Placeholder random logic
			y := float64(attempts/10) * 2

			if x+areaSize > roomDimensions.Width || y+areaSize > roomDimensions.Height {
				continue
			}

			proposedArea := spatial.Rectangle{
				Position: spatial.Position{X: x, Y: y},
				Dimensions: spatial.Dimensions{
					Width:  areaSize,
					Height: areaSize,
				},
			}

			// Check if it overlaps with existing areas
			overlaps := false
			for _, usedArea := range usedAreas {
				if e.rectanglesOverlapUnsafe(proposedArea, usedArea, minSeparation) {
					overlaps = true
					break
				}
			}

			if !overlaps {
				teamAreas[teamID] = proposedArea
				usedAreas = append(usedAreas, proposedArea)
				break
			}
		}
	}

	return teamAreas, nil
}

// spawnTeamInAreaUnsafe spawns a team's entities within their designated area
func (e *BasicSpawnEngine) spawnTeamInAreaUnsafe(ctx context.Context, roomID, teamID string, entities []core.Entity, spawnArea spatial.Rectangle, teamConfig *TeamConfig, result *SpawnResult) error {
	// Find team configuration
	var team *Team
	for _, t := range teamConfig.Teams {
		if t.ID == teamID {
			team = &t
			break
		}
	}

	// Apply team cohesion rules
	cohesionRules := teamConfig.CohesionRules
	shouldKeepTogether := e.shouldKeepTeamTogetherUnsafe(teamID, cohesionRules)

	if shouldKeepTogether && team != nil && team.Formation != nil {
		// Use formation spawning
		return e.spawnTeamInFormationUnsafe(ctx, roomID, teamID, entities, spawnArea, *team.Formation, result)
	} else if shouldKeepTogether {
		// Use clustered spawning
		return e.spawnTeamClusteredUnsafe(ctx, roomID, teamID, entities, spawnArea, result)
	} else {
		// Use scattered spawning within area
		return e.spawnTeamScatteredUnsafe(ctx, roomID, teamID, entities, spawnArea, result)
	}
}

// shouldKeepTeamTogetherUnsafe determines if a team should be kept together
func (e *BasicSpawnEngine) shouldKeepTeamTogetherUnsafe(teamID string, cohesionRules TeamCohesionRules) bool {
	switch teamID {
	case "friendlies", "players", "allies":
		return cohesionRules.KeepFriendliesTogether
	case "enemies", "hostiles", "monsters":
		return cohesionRules.KeepEnemiesTogether
	default:
		// For other teams, default to keeping together
		return true
	}
}

// spawnTeamInFormationUnsafe spawns team entities in a formation pattern
func (e *BasicSpawnEngine) spawnTeamInFormationUnsafe(ctx context.Context, roomID, teamID string, entities []core.Entity, spawnArea spatial.Rectangle, formation FormationPattern, result *SpawnResult) error {
	// Calculate formation center within spawn area
	centerX, centerY := getRectCenter(spawnArea)
	centerPosition := spatial.Position{X: centerX, Y: centerY}

	// Apply formation positions
	for i, entity := range entities {
		var position spatial.Position
		
		if i < len(formation.Positions) {
			// Use defined formation position
			relPos := formation.Positions[i]
			position = spatial.Position{
				X: centerX + relPos.X,
				Y: centerY + relPos.Y,
			}
		} else {
			// Fallback to center for extra entities
			position = centerPosition
		}

		// Ensure position is within spawn area
		position = e.constrainPositionToAreaUnsafe(position, spawnArea)

		spawnedEntity := SpawnedEntity{
			Entity:      entity,
			Position:    position,
			RoomID:      roomID,
			GroupID:     fmt.Sprintf("team_%s", teamID),
			TeamID:      teamID,
			SpawnReason: fmt.Sprintf("formation_%s", formation.Name),
		}

		result.SpawnedEntities = append(result.SpawnedEntities, spawnedEntity)
	}

	// Publish formation event
	roomEntity := &SpawnRoomEntity{id: roomID, roomType: "team_spawn_target"}
	e.publishFormationEventUnsafe(ctx, roomEntity, formation, centerPosition, result.SpawnedEntities)

	return nil
}

// spawnTeamClusteredUnsafe spawns team entities in a tight cluster
func (e *BasicSpawnEngine) spawnTeamClusteredUnsafe(ctx context.Context, roomID, teamID string, entities []core.Entity, spawnArea spatial.Rectangle, result *SpawnResult) error {
	// Calculate cluster center
	centerX, centerY := getRectCenter(spawnArea)

	// Arrange entities in a tight cluster around center
	clusterRadius := 2.0 // Tight clustering
	angleStep := 2 * math.Pi / float64(len(entities))

	for i, entity := range entities {
		angle := float64(i) * angleStep
		position := spatial.Position{
			X: centerX + clusterRadius*math.Cos(angle),
			Y: centerY + clusterRadius*math.Sin(angle),
		}

		// Ensure position is within spawn area
		position = e.constrainPositionToAreaUnsafe(position, spawnArea)

		spawnedEntity := SpawnedEntity{
			Entity:      entity,
			Position:    position,
			RoomID:      roomID,
			GroupID:     fmt.Sprintf("team_%s", teamID),
			TeamID:      teamID,
			SpawnReason: "clustered_team_spawn",
		}

		result.SpawnedEntities = append(result.SpawnedEntities, spawnedEntity)
	}

	return nil
}

// spawnTeamScatteredUnsafe spawns team entities scattered within their area
func (e *BasicSpawnEngine) spawnTeamScatteredUnsafe(ctx context.Context, roomID, teamID string, entities []core.Entity, spawnArea spatial.Rectangle, result *SpawnResult) error {
	for i, entity := range entities {
		// Simple scattered positioning within area
		x := spawnArea.Position.X + float64(i%3)*spawnArea.Dimensions.Width/3
		y := spawnArea.Position.Y + float64(i/3)*spawnArea.Dimensions.Height/3

		position := spatial.Position{X: x, Y: y}
		position = e.constrainPositionToAreaUnsafe(position, spawnArea)

		spawnedEntity := SpawnedEntity{
			Entity:      entity,
			Position:    position,
			RoomID:      roomID,
			GroupID:     fmt.Sprintf("team_%s", teamID),
			TeamID:      teamID,
			SpawnReason: "scattered_team_spawn",
		}

		result.SpawnedEntities = append(result.SpawnedEntities, spawnedEntity)
	}

	return nil
}

// Helper methods

// rectanglesOverlapUnsafe checks if two rectangles overlap within a separation distance
func (e *BasicSpawnEngine) rectanglesOverlapUnsafe(a, b spatial.Rectangle, minSeparation float64) bool {
	return rectIntersects(a, b, minSeparation)
}

// constrainPositionToAreaUnsafe ensures a position stays within the given area
func (e *BasicSpawnEngine) constrainPositionToAreaUnsafe(position spatial.Position, area spatial.Rectangle) spatial.Position {
	return clampPosition(position.X, position.Y, area)
}