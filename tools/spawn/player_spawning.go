package spawn

import (
	"context"
	"fmt"

	"github.com/KirkDiggler/rpg-toolkit/core"
	"github.com/KirkDiggler/rpg-toolkit/tools/spatial"
)

// Player spawn zone implementation following user requirements
// Purpose: Allow players to choose spawn positions within designated zones

// applyPlayerChoiceSpawningUnsafe handles player-chosen spawn positions
func (e *BasicSpawnEngine) applyPlayerChoiceSpawningUnsafe(ctx context.Context, roomID string, entities []core.Entity, config SpawnConfig, result *SpawnResult) error {
	if len(config.PlayerSpawnZones) == 0 {
		return fmt.Errorf("player choice spawning requested but no spawn zones defined")
	}

	// Group entities by type
	playerEntities := make([]core.Entity, 0)
	nonPlayerEntities := make([]core.Entity, 0)

	for _, entity := range entities {
		if e.isPlayerEntityUnsafe(entity) {
			playerEntities = append(playerEntities, entity)
		} else {
			nonPlayerEntities = append(nonPlayerEntities, entity)
		}
	}

	// Handle player spawn choices
	if err := e.processPlayerSpawnChoicesUnsafe(ctx, roomID, playerEntities, config, result); err != nil {
		return fmt.Errorf("failed to process player spawn choices: %w", err)
	}

	// Handle non-player entities with regular spawning
	if len(nonPlayerEntities) > 0 {
		if err := e.populateSingleRoomUnsafe(ctx, roomID, nonPlayerEntities, config, result); err != nil {
			return fmt.Errorf("failed to spawn non-player entities: %w", err)
		}
	}

	return nil
}

// processPlayerSpawnChoicesUnsafe handles player spawn position choices
func (e *BasicSpawnEngine) processPlayerSpawnChoicesUnsafe(ctx context.Context, roomID string, playerEntities []core.Entity, config SpawnConfig, result *SpawnResult) error {
	// Create mapping of player choices
	playerChoices := make(map[string]PlayerSpawnChoice)
	for _, choice := range config.PlayerChoices {
		playerChoices[choice.PlayerID] = choice
	}

	// Track zone occupancy
	zoneOccupancy := make(map[string][]spatial.Position)
	for _, zone := range config.PlayerSpawnZones {
		zoneOccupancy[zone.ID] = make([]spatial.Position, 0)
	}

	// Process each player entity
	for _, entity := range playerEntities {
		playerID := entity.GetID()
		
		if choice, hasChoice := playerChoices[playerID]; hasChoice {
			// Player made a specific choice
			err := e.processSpecificPlayerChoiceUnsafe(ctx, roomID, entity, choice, config.PlayerSpawnZones, zoneOccupancy, result)
			if err != nil {
				// Choice failed - try automatic assignment
				err = e.autoAssignPlayerSpawnUnsafe(ctx, roomID, entity, config.PlayerSpawnZones, zoneOccupancy, result)
				if err != nil {
					return fmt.Errorf("failed to spawn player %s: %w", playerID, err)
				}
			}
		} else {
			// No specific choice - auto assign
			err := e.autoAssignPlayerSpawnUnsafe(ctx, roomID, entity, config.PlayerSpawnZones, zoneOccupancy, result)
			if err != nil {
				return fmt.Errorf("failed to auto-assign player %s: %w", playerID, err)
			}
		}
	}

	return nil
}

// processSpecificPlayerChoiceUnsafe handles a specific player spawn choice
func (e *BasicSpawnEngine) processSpecificPlayerChoiceUnsafe(ctx context.Context, roomID string, entity core.Entity, choice PlayerSpawnChoice, zones []SpawnZone, zoneOccupancy map[string][]spatial.Position, result *SpawnResult) error {
	// Find the requested zone
	var selectedZone *SpawnZone
	for _, zone := range zones {
		if zone.ID == choice.ZoneID {
			selectedZone = &zone
			break
		}
	}

	if selectedZone == nil {
		return fmt.Errorf("requested zone %s not found", choice.ZoneID)
	}

	// Validate player can spawn in this zone
	if !e.canEntitySpawnInZoneUnsafe(entity, *selectedZone) {
		denialReason := fmt.Sprintf("entity type %s not allowed in zone %s", entity.GetType(), selectedZone.ID)
		roomEntity := &SpawnRoomEntity{id: roomID, roomType: "player_spawn_target"}
		e.publishPlayerSpawnEventUnsafe(ctx, roomEntity, choice, false, denialReason)
		return fmt.Errorf("entity type %s not allowed in zone %s", entity.GetType(), selectedZone.ID)
	}

	// Check if zone has capacity
	currentOccupancy := len(zoneOccupancy[selectedZone.ID])
	if currentOccupancy >= selectedZone.MaxEntities {
		denialReason := fmt.Sprintf("zone %s is full (%d/%d)", selectedZone.ID, currentOccupancy, selectedZone.MaxEntities)
		roomEntity := &SpawnRoomEntity{id: roomID, roomType: "player_spawn_target"}
		e.publishPlayerSpawnEventUnsafe(ctx, roomEntity, choice, false, denialReason)
		return fmt.Errorf("zone %s is full (%d/%d)", selectedZone.ID, currentOccupancy, selectedZone.MaxEntities)
	}

	// Validate position is within zone
	if !e.isPositionInZoneUnsafe(choice.Position, *selectedZone) {
		denialReason := fmt.Sprintf("position (%.1f,%.1f) is outside zone %s", choice.Position.X, choice.Position.Y, selectedZone.ID)
		roomEntity := &SpawnRoomEntity{id: roomID, roomType: "player_spawn_target"}
		e.publishPlayerSpawnEventUnsafe(ctx, roomEntity, choice, false, denialReason)
		return fmt.Errorf("position (%.1f,%.1f) is outside zone %s", choice.Position.X, choice.Position.Y, selectedZone.ID)
	}

	// Check if position is already occupied (spatial module would normally do this)
	if e.isPositionOccupiedUnsafe(choice.Position, zoneOccupancy[selectedZone.ID]) {
		denialReason := fmt.Sprintf("position (%.1f,%.1f) is already occupied", choice.Position.X, choice.Position.Y)
		roomEntity := &SpawnRoomEntity{id: roomID, roomType: "player_spawn_target"}
		e.publishPlayerSpawnEventUnsafe(ctx, roomEntity, choice, false, denialReason)
		return fmt.Errorf("position (%.1f,%.1f) is already occupied", choice.Position.X, choice.Position.Y)
	}

	// Position is valid - spawn the player
	spawnedEntity := SpawnedEntity{
		Entity:      entity,
		Position:    choice.Position,
		RoomID:      roomID,
		GroupID:     fmt.Sprintf("player_choice_%s", selectedZone.ID),
		SpawnReason: "player_choice",
	}

	result.SpawnedEntities = append(result.SpawnedEntities, spawnedEntity)
	zoneOccupancy[selectedZone.ID] = append(zoneOccupancy[selectedZone.ID], choice.Position)

	// Publish success event
	roomEntity := &SpawnRoomEntity{id: roomID, roomType: "player_spawn_target"}
	e.publishPlayerSpawnEventUnsafe(ctx, roomEntity, choice, true, "")

	return nil
}

// autoAssignPlayerSpawnUnsafe automatically assigns a player to an available zone position
func (e *BasicSpawnEngine) autoAssignPlayerSpawnUnsafe(ctx context.Context, roomID string, entity core.Entity, zones []SpawnZone, zoneOccupancy map[string][]spatial.Position, result *SpawnResult) error {
	// Find first available zone that allows this entity type
	for _, zone := range zones {
		if !e.canEntitySpawnInZoneUnsafe(entity, zone) {
			continue
		}

		// Check if zone has capacity
		currentOccupancy := len(zoneOccupancy[zone.ID])
		if currentOccupancy >= zone.MaxEntities {
			continue
		}

		// Find available position in zone
		position, found := e.findAvailablePositionInZoneUnsafe(zone, zoneOccupancy[zone.ID])
		if !found {
			continue
		}

		// Spawn the player
		spawnedEntity := SpawnedEntity{
			Entity:      entity,
			Position:    position,
			RoomID:      roomID,
			GroupID:     fmt.Sprintf("auto_assigned_%s", zone.ID),
			SpawnReason: "auto_assigned_to_zone",
		}

		result.SpawnedEntities = append(result.SpawnedEntities, spawnedEntity)
		zoneOccupancy[zone.ID] = append(zoneOccupancy[zone.ID], position)

		return nil
	}

	return fmt.Errorf("no available spawn zones for entity %s", entity.GetID())
}

// validatePlayerSpawnZonesUnsafe validates spawn zone configuration
func (e *BasicSpawnEngine) validatePlayerSpawnZonesUnsafe(zones []SpawnZone) error {
	zoneIDs := make(map[string]bool)

	for i, zone := range zones {
		if zone.ID == "" {
			return fmt.Errorf("spawn zone %d missing ID", i)
		}

		if zoneIDs[zone.ID] {
			return fmt.Errorf("duplicate spawn zone ID: %s", zone.ID)
		}
		zoneIDs[zone.ID] = true

		if zone.MaxEntities <= 0 {
			return fmt.Errorf("spawn zone %s must allow at least 1 entity", zone.ID)
		}

		if zone.Area.Dimensions.Width <= 0 || zone.Area.Dimensions.Height <= 0 {
			return fmt.Errorf("spawn zone %s must have positive dimensions", zone.ID)
		}
	}

	return nil
}

// Helper methods for player spawning

// isPlayerEntityUnsafe determines if an entity is a player
func (e *BasicSpawnEngine) isPlayerEntityUnsafe(entity core.Entity) bool {
	entityType := entity.GetType()
	return entityType == "player" || entityType == "character" || entityType == "pc"
}

// canEntitySpawnInZoneUnsafe checks if an entity can spawn in a zone
func (e *BasicSpawnEngine) canEntitySpawnInZoneUnsafe(entity core.Entity, zone SpawnZone) bool {
	if len(zone.EntityTypes) == 0 {
		return true // Zone allows all entity types
	}

	entityType := entity.GetType()
	for _, allowedType := range zone.EntityTypes {
		if entityType == allowedType {
			return true
		}
	}

	return false
}

// isPositionInZoneUnsafe checks if a position is within a zone's boundaries
func (e *BasicSpawnEngine) isPositionInZoneUnsafe(position spatial.Position, zone SpawnZone) bool {
	return zone.Area.Contains(position)
}

// isPositionOccupiedUnsafe checks if a position is already occupied
func (e *BasicSpawnEngine) isPositionOccupiedUnsafe(position spatial.Position, occupiedPositions []spatial.Position) bool {
	minDistance := 1.0 // Minimum distance between entities

	for _, occupied := range occupiedPositions {
		distance := e.calculateDistanceUnsafe(position, occupied)
		if distance < minDistance {
			return true
		}
	}

	return false
}

// findAvailablePositionInZoneUnsafe finds an available position within a zone
func (e *BasicSpawnEngine) findAvailablePositionInZoneUnsafe(zone SpawnZone, occupiedPositions []spatial.Position) (spatial.Position, bool) {
	// Grid-based search for available position
	gridSize := 1.0
	
	for y := zone.Area.Position.Y; y < zone.Area.Position.Y+zone.Area.Dimensions.Height; y += gridSize {
		for x := zone.Area.Position.X; x < zone.Area.Position.X+zone.Area.Dimensions.Width; x += gridSize {
			position := spatial.Position{X: x, Y: y}
			
			if !e.isPositionOccupiedUnsafe(position, occupiedPositions) {
				return position, true
			}
		}
	}

	return spatial.Position{}, false
}

// calculateDistanceUnsafe calculates distance between two positions
func (e *BasicSpawnEngine) calculateDistanceUnsafe(a, b spatial.Position) float64 {
	dx := a.X - b.X
	dy := a.Y - b.Y
	return dx*dx + dy*dy // Squared distance is sufficient for comparison
}

// getAlternativePositionsUnsafe gets alternative positions when requested position is denied
func (e *BasicSpawnEngine) getAlternativePositionsUnsafe(zone SpawnZone, occupiedPositions []spatial.Position, count int) []spatial.Position {
	alternatives := make([]spatial.Position, 0, count)
	gridSize := 1.0

	for y := zone.Area.Position.Y; y < zone.Area.Position.Y+zone.Area.Dimensions.Height && len(alternatives) < count; y += gridSize {
		for x := zone.Area.Position.X; x < zone.Area.Position.X+zone.Area.Dimensions.Width && len(alternatives) < count; x += gridSize {
			position := spatial.Position{X: x, Y: y}
			
			if !e.isPositionOccupiedUnsafe(position, occupiedPositions) {
				alternatives = append(alternatives, position)
			}
		}
	}

	return alternatives
}