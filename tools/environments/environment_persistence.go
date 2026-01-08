package environments

import (
	"fmt"

	"github.com/KirkDiggler/rpg-toolkit/core"
	"github.com/KirkDiggler/rpg-toolkit/events"
	"github.com/KirkDiggler/rpg-toolkit/tools/spatial"
)

// ToData converts a BasicEnvironment to EnvironmentData for persistence.
// All coordinates are converted to absolute dungeon space.
//
// Purpose: Serialize the environment for storage. The game server receives
// this and passes it through without coordinate conversions.
func (e *BasicEnvironment) ToData() EnvironmentData {
	e.mutex.RLock()
	defer e.mutex.RUnlock()

	data := EnvironmentData{
		ID:   e.id,
		Seed: e.metadata.GeneratedAt.UnixNano(), // Use generation timestamp as pseudo-seed
	}

	// Convert rooms to zones
	for roomID, room := range e.orchestrator.GetAllRooms() {
		origin := e.roomPositions[roomID]
		grid := room.GetGrid()
		dims := grid.GetDimensions()

		zone := ZoneData{
			ID:     roomID,
			Type:   string(room.GetType()), // core.EntityType -> string
			Origin: origin,
			Width:  int(dims.Width),
			Height: int(dims.Height),
		}

		// Collect entity IDs for this zone
		for _, entity := range room.GetAllEntities() {
			zone.EntityIDs = append(zone.EntityIDs, entity.GetID())
		}

		data.Zones = append(data.Zones, zone)
	}

	// Convert connections to passages
	for _, conn := range e.orchestrator.GetAllConnections() {
		passage := PassageData{
			ID:            conn.GetID(),
			FromZoneID:    conn.GetFromRoom(),
			ToZoneID:      conn.GetToRoom(),
			Bidirectional: conn.IsReversible(),
		}
		data.Passages = append(data.Passages, passage)
	}

	// Convert entities to placed entities with absolute coordinates
	for roomID, room := range e.orchestrator.GetAllRooms() {
		origin := e.roomPositions[roomID]
		grid := room.GetGrid()

		for _, entity := range room.GetAllEntities() {
			pos, found := room.GetEntityPosition(entity.GetID())
			if !found {
				continue
			}

			// Convert room-local position to absolute
			var absPos spatial.CubeCoordinate
			if grid.GetShape() == spatial.GridShapeHex {
				// For hex grids, convert offset to cube then add origin
				localCube := spatial.OffsetCoordinateToCube(pos)
				absPos = spatial.CubeCoordinate{
					X: origin.X + localCube.X,
					Y: origin.Y + localCube.Y,
					Z: origin.Z + localCube.Z,
				}
			} else {
				// For square/gridless, use X/Z mapping
				absPos = spatial.CubeCoordinate{
					X: origin.X + int(pos.X),
					Y: 0,
					Z: origin.Z + int(pos.Y),
				}
				absPos.Y = -absPos.X - absPos.Z // Maintain cube constraint
			}

			placed := PlacedEntityData{
				ID:       entity.GetID(),
				Type:     string(entity.GetType()),
				Position: absPos,
				ZoneID:   roomID,
			}

			// Extract placeable properties if available
			if placeable, ok := entity.(spatial.Placeable); ok {
				placed.Size = placeable.GetSize()
				placed.BlocksMovement = placeable.BlocksMovement()
				placed.BlocksLoS = placeable.BlocksLineOfSight()
			}

			data.Entities = append(data.Entities, placed)
		}
	}

	// Note: WallSegmentData not populated - walls are entities in current implementation
	data.Walls = []WallSegmentData{}

	return data
}

// LoadFromData creates a BasicEnvironment from EnvironmentData.
// This reconstructs the environment from persisted data.
//
// Purpose: Restore an environment from storage. The data contains absolute
// coordinates, so we convert back to room-local during reconstruction.
func LoadFromData(data EnvironmentData, eventBus events.EventBus) (*BasicEnvironment, error) {
	if data.ID == "" {
		return nil, fmt.Errorf("environment ID is required")
	}

	// Create orchestrator for managing rooms
	orchestrator := spatial.NewBasicRoomOrchestrator(spatial.BasicRoomOrchestratorConfig{
		ID:   spatial.OrchestratorID(data.ID + "-orchestrator"),
		Type: "orchestrator",
	})
	orchestrator.ConnectToEventBus(eventBus)

	// Track room positions for coordinate conversion
	roomPositions := make(map[string]spatial.CubeCoordinate)

	// Create rooms from zones
	for _, zone := range data.Zones {
		roomPositions[zone.ID] = zone.Origin

		// Create hex grid for the room (default to hex, could be configurable)
		grid := spatial.NewHexGrid(spatial.HexGridConfig{
			Width:       float64(zone.Width),
			Height:      float64(zone.Height),
			Orientation: spatial.HexOrientationPointyTop,
		})

		room := spatial.NewBasicRoom(spatial.BasicRoomConfig{
			ID:   zone.ID,
			Type: zone.Type,
			Grid: grid,
		})
		room.ConnectToEventBus(eventBus)

		if err := orchestrator.AddRoom(room); err != nil {
			return nil, fmt.Errorf("failed to add room %s: %w", zone.ID, err)
		}
	}

	// Create connections from passages
	for _, passage := range data.Passages {
		conn := spatial.NewBasicConnection(spatial.BasicConnectionConfig{
			ID:         passage.ID,
			FromRoom:   passage.FromZoneID,
			ToRoom:     passage.ToZoneID,
			Reversible: passage.Bidirectional,
			Passable:   true,
		})

		if err := orchestrator.AddConnection(conn); err != nil {
			return nil, fmt.Errorf("failed to add connection %s: %w", passage.ID, err)
		}
	}

	// Place entities in their rooms
	for _, placed := range data.Entities {
		room, found := orchestrator.GetRoom(placed.ZoneID)
		if !found {
			continue // Skip entities for unknown rooms
		}

		origin := roomPositions[placed.ZoneID]

		// Convert absolute position to room-local
		localCube := spatial.CubeCoordinate{
			X: placed.Position.X - origin.X,
			Y: placed.Position.Y - origin.Y,
			Z: placed.Position.Z - origin.Z,
		}
		localPos := localCube.ToOffsetCoordinate()

		// Create entity from persisted data
		entity := newPersistedEntity(placed)

		if err := room.PlaceEntity(entity, localPos); err != nil {
			// Log but continue - entity might overlap
			continue
		}
	}

	// Calculate blocked hexes from placed entities
	blockedHexes := make(map[spatial.CubeCoordinate]bool)
	for roomID, room := range orchestrator.GetAllRooms() {
		origin := roomPositions[roomID]
		grid := room.GetGrid()

		for _, entity := range room.GetAllEntities() {
			placeable, ok := entity.(spatial.Placeable)
			if !ok || !placeable.BlocksMovement() {
				continue
			}

			pos, found := room.GetEntityPosition(entity.GetID())
			if !found {
				continue
			}

			// Convert to absolute
			var absPos spatial.CubeCoordinate
			if grid.GetShape() == spatial.GridShapeHex {
				localCube := spatial.OffsetCoordinateToCube(pos)
				absPos = spatial.CubeCoordinate{
					X: origin.X + localCube.X,
					Y: origin.Y + localCube.Y,
					Z: origin.Z + localCube.Z,
				}
			} else {
				absPos = spatial.CubeCoordinate{
					X: origin.X + int(pos.X),
					Y: 0,
					Z: origin.Z + int(pos.Y),
				}
				absPos.Y = -absPos.X - absPos.Z
			}

			blockedHexes[absPos] = true
		}
	}

	// Create the environment
	env := NewBasicEnvironment(BasicEnvironmentConfig{
		ID:            data.ID,
		Type:          "environment",
		Orchestrator:  orchestrator,
		RoomPositions: roomPositions,
		BlockedHexes:  blockedHexes,
	})

	env.ConnectToEventBus(eventBus)

	return env, nil
}

// persistedEntity is a minimal entity implementation for loading persisted data.
// Implements both core.Entity and spatial.Placeable.
type persistedEntity struct {
	id             string
	entityType     string
	size           int
	blocksMovement bool
	blocksLoS      bool
}

// newPersistedEntity creates a persistedEntity from PlacedEntityData
func newPersistedEntity(data PlacedEntityData) *persistedEntity {
	return &persistedEntity{
		id:             data.ID,
		entityType:     data.Type,
		size:           max(data.Size, 1),
		blocksMovement: data.BlocksMovement,
		blocksLoS:      data.BlocksLoS,
	}
}

// GetID implements core.Entity
func (e *persistedEntity) GetID() string {
	return e.id
}

// GetType implements core.Entity
func (e *persistedEntity) GetType() core.EntityType {
	return core.EntityType(e.entityType)
}

// GetSize implements spatial.Placeable
func (e *persistedEntity) GetSize() int {
	return e.size
}

// BlocksMovement implements spatial.Placeable
func (e *persistedEntity) BlocksMovement() bool {
	return e.blocksMovement
}

// BlocksLineOfSight implements spatial.Placeable
func (e *persistedEntity) BlocksLineOfSight() bool {
	return e.blocksLoS
}
