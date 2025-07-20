package spawn

import (
	"context"
	"fmt"

	"github.com/KirkDiggler/rpg-toolkit/core"
	"github.com/KirkDiggler/rpg-toolkit/tools/spatial"
)

// applyScatteredSpawning implements scattered spawning pattern
func (e *BasicSpawnEngine) applyScatteredSpawning(ctx context.Context, roomID string, config SpawnConfig, result SpawnResult) (SpawnResult, error) {
	// Get room from spatial handler
	room, err := e.getRoomFromSpatial(roomID)
	if err != nil {
		return result, fmt.Errorf("failed to get room: %w", err)
	}

	// Process each entity group
	for _, group := range config.EntityGroups {
		entities, err := e.selectEntitiesForGroup(group)
		if err != nil {
			result.Failures = append(result.Failures, SpawnFailure{
				EntityType: group.Type,
				Reason:     fmt.Sprintf("selection failed: %v", err),
			})
			continue
		}

		// Place entities using scattered pattern
		for _, entity := range entities {
			position, err := e.findValidPosition(room, entity)
			if err != nil {
				result.Failures = append(result.Failures, SpawnFailure{
					EntityType: entity.GetType(),
					Reason:     err.Error(),
				})
				continue
			}

			// Place entity in room
			if err := e.placeEntityInRoom(room, entity, position); err != nil {
				result.Failures = append(result.Failures, SpawnFailure{
					EntityType: entity.GetType(),
					Reason:     fmt.Sprintf("placement failed: %v", err),
				})
				continue
			}

			result.SpawnedEntities = append(result.SpawnedEntities, SpawnedEntity{
				Entity:   entity,
				Position: position,
				RoomID:   roomID,
			})

			e.publishEntitySpawnedEvent(ctx, roomID, entity, position)
		}
	}

	result.Success = len(result.SpawnedEntities) > 0
	return result, nil
}

// applyFormationSpawning implements formation-based spawning pattern
func (e *BasicSpawnEngine) applyFormationSpawning(ctx context.Context, roomID string, config SpawnConfig, result SpawnResult) (SpawnResult, error) {
	// Phase 2: Simple implementation - delegate to scattered for now
	// TODO: Implement actual formation logic
	return e.applyScatteredSpawning(ctx, roomID, config, result)
}

// applyTeamBasedSpawning implements team-based spawning pattern
func (e *BasicSpawnEngine) applyTeamBasedSpawning(ctx context.Context, roomID string, config SpawnConfig, result SpawnResult) (SpawnResult, error) {
	// Phase 2: Basic implementation
	if config.TeamConfiguration == nil {
		return result, fmt.Errorf("team configuration required for team-based spawning")
	}

	// For now, delegate to scattered spawning
	// TODO: Implement actual team separation logic
	return e.applyScatteredSpawning(ctx, roomID, config, result)
}

// applyPlayerChoiceSpawning implements player choice spawning pattern
func (e *BasicSpawnEngine) applyPlayerChoiceSpawning(ctx context.Context, roomID string, config SpawnConfig, result SpawnResult) (SpawnResult, error) {
	// Phase 2: Basic implementation
	if len(config.PlayerSpawnZones) == 0 {
		return result, fmt.Errorf("player spawn zones required for player choice spawning")
	}

	// Get room from spatial handler
	room, err := e.getRoomFromSpatial(roomID)
	if err != nil {
		return result, fmt.Errorf("failed to get room: %w", err)
	}

	// Process each entity group
	for _, group := range config.EntityGroups {
		entities, err := e.selectEntitiesForGroup(group)
		if err != nil {
			result.Failures = append(result.Failures, SpawnFailure{
				EntityType: group.Type,
				Reason:     fmt.Sprintf("selection failed: %v", err),
			})
			continue
		}

		// For player entities, use zones; for others, use scattered
		for _, entity := range entities {
			if e.isPlayerEntity(entity) {
				position, err := e.findPlayerSpawnPosition(entity, config.PlayerSpawnZones, config.PlayerChoices)
				if err != nil {
					result.Failures = append(result.Failures, SpawnFailure{
						EntityType: entity.GetType(),
						Reason:     fmt.Sprintf("player spawn failed: %v", err),
					})
					continue
				}

				result.SpawnedEntities = append(result.SpawnedEntities, SpawnedEntity{
					Entity:   entity,
					Position: position,
					RoomID:   roomID,
				})
			} else {
				// Non-player entities use scattered placement
				position, err := e.findValidPosition(room, entity)
				if err != nil {
					result.Failures = append(result.Failures, SpawnFailure{
						EntityType: entity.GetType(),
						Reason:     err.Error(),
					})
					continue
				}

				result.SpawnedEntities = append(result.SpawnedEntities, SpawnedEntity{
					Entity:   entity,
					Position: position,
					RoomID:   roomID,
				})
			}

			e.publishEntitySpawnedEvent(ctx, roomID, entity, result.SpawnedEntities[len(result.SpawnedEntities)-1].Position)
		}
	}

	result.Success = len(result.SpawnedEntities) > 0
	return result, nil
}

// applyClusteredSpawning implements clustered spawning pattern
func (e *BasicSpawnEngine) applyClusteredSpawning(ctx context.Context, roomID string, config SpawnConfig, result SpawnResult) (SpawnResult, error) {
	// Phase 2: Simple implementation - delegate to scattered for now
	// TODO: Implement actual clustering logic
	return e.applyScatteredSpawning(ctx, roomID, config, result)
}

// isPlayerEntity determines if an entity is a player
func (e *BasicSpawnEngine) isPlayerEntity(entity core.Entity) bool {
	entityType := entity.GetType()
	return entityType == "player" || entityType == "character" || entityType == "pc"
}

// findPlayerSpawnPosition finds a position for a player within spawn zones
func (e *BasicSpawnEngine) findPlayerSpawnPosition(entity core.Entity, zones []SpawnZone, choices []PlayerSpawnChoice) (spatial.Position, error) {
	entityID := entity.GetID()

	// Check if player made a specific choice
	for _, choice := range choices {
		if choice.PlayerID == entityID {
			// Validate the choice is within a valid zone
			for _, zone := range zones {
				if zone.ID == choice.ZoneID && e.isEntityAllowedInZone(entity, zone) {
					if e.isPositionInZone(choice.Position, zone) {
						return choice.Position, nil
					}
				}
			}
		}
	}

	// Auto-assign to first available zone
	for _, zone := range zones {
		if e.isEntityAllowedInZone(entity, zone) {
			// Simple assignment: center of zone
			return spatial.Position{
				X: zone.Area.Position.X + zone.Area.Dimensions.Width/2,
				Y: zone.Area.Position.Y + zone.Area.Dimensions.Height/2,
			}, nil
		}
	}

	return spatial.Position{}, fmt.Errorf("no valid spawn zone found for player %s", entityID)
}

// isEntityAllowedInZone checks if an entity can spawn in a zone
func (e *BasicSpawnEngine) isEntityAllowedInZone(entity core.Entity, zone SpawnZone) bool {
	if len(zone.EntityTypes) == 0 {
		return true // Zone allows all types
	}

	entityType := entity.GetType()
	for _, allowedType := range zone.EntityTypes {
		if entityType == allowedType {
			return true
		}
	}
	return false
}

// isPositionInZone checks if a position is within zone boundaries
func (e *BasicSpawnEngine) isPositionInZone(position spatial.Position, zone SpawnZone) bool {
	return zone.Area.Contains(position)
}