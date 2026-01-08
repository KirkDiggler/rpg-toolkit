package environments

import (
	"fmt"
	"sort"

	"github.com/KirkDiggler/rpg-toolkit/core"
	"github.com/KirkDiggler/rpg-toolkit/events"
	"github.com/KirkDiggler/rpg-toolkit/tools/spatial"
)

// ============================================================================
// Grid Shape and Orientation Conversion
// ============================================================================

// gridShapeToValue converts spatial.GridShape to serializable GridShapeValue
func gridShapeToValue(shape spatial.GridShape) GridShapeValue {
	switch shape {
	case spatial.GridShapeHex:
		return GridShapeHex
	case spatial.GridShapeSquare:
		return GridShapeSquare
	case spatial.GridShapeGridless:
		return GridShapeGridless
	default:
		return GridShapeHex // Default to hex
	}
}

// valueToGridShape converts GridShapeValue to spatial.GridShape
func valueToGridShape(value GridShapeValue) (spatial.GridShape, error) {
	switch value {
	case GridShapeHex:
		return spatial.GridShapeHex, nil
	case GridShapeSquare:
		return spatial.GridShapeSquare, nil
	case GridShapeGridless:
		return spatial.GridShapeGridless, nil
	default:
		return spatial.GridShapeHex, fmt.Errorf("unknown grid shape: %s", value)
	}
}

// hexOrientationToValue converts spatial.HexOrientation to serializable HexOrientationValue
func hexOrientationToValue(orientation spatial.HexOrientation) HexOrientationValue {
	switch orientation {
	case spatial.HexOrientationPointyTop:
		return HexOrientationPointy
	case spatial.HexOrientationFlatTop:
		return HexOrientationFlat
	default:
		return HexOrientationPointy // Default to pointy
	}
}

// valueToHexOrientation converts HexOrientationValue to spatial.HexOrientation
func valueToHexOrientation(value HexOrientationValue) spatial.HexOrientation {
	switch value {
	case HexOrientationFlat:
		return spatial.HexOrientationFlatTop
	default:
		return spatial.HexOrientationPointyTop // Default to pointy
	}
}

// ============================================================================
// ToData - Convert BasicEnvironment to EnvironmentData
// ============================================================================

// ToData converts a BasicEnvironment to EnvironmentData for persistence.
// All coordinates are converted to absolute environment space.
// Output is deterministic - slices are sorted by ID.
//
// Purpose: Serialize the environment for storage. The game server receives
// this and passes it through without coordinate conversions.
func (e *BasicEnvironment) ToData() EnvironmentData {
	e.mutex.RLock()
	defer e.mutex.RUnlock()

	data := EnvironmentData{
		ID:       e.id,
		Type:     EnvironmentType(e.typ),
		Theme:    e.theme,
		Metadata: e.metadata,
		Zones:    make([]ZoneData, 0),
		Passages: make([]PassageData, 0),
		Entities: make([]PlacedEntityData, 0),
		Walls:    make([]WallSegmentData, 0),
	}

	if e.orchestrator == nil {
		return data
	}

	// Get all rooms and sort by ID for deterministic output
	allRooms := e.orchestrator.GetAllRooms()
	roomIDs := make([]string, 0, len(allRooms))
	for roomID := range allRooms {
		roomIDs = append(roomIDs, roomID)
	}
	sort.Strings(roomIDs)

	// Convert rooms to zones
	for _, roomID := range roomIDs {
		room := allRooms[roomID]
		origin := e.roomPositions[roomID]

		zone := e.roomToZoneData(roomID, room, origin)
		data.Zones = append(data.Zones, zone)

		// Convert entities in this room
		entities := e.roomEntitiesToData(roomID, room, origin)
		data.Entities = append(data.Entities, entities...)
	}

	// Convert connections to passages (sorted for determinism)
	allConnections := e.orchestrator.GetAllConnections()
	connIDs := make([]string, 0, len(allConnections))
	for connID := range allConnections {
		connIDs = append(connIDs, connID)
	}
	sort.Strings(connIDs)

	for _, connID := range connIDs {
		conn := allConnections[connID]
		passage := PassageData{
			ID:            conn.GetID(),
			FromZoneID:    conn.GetFromRoom(),
			ToZoneID:      conn.GetToRoom(),
			Bidirectional: conn.IsReversible(),
		}
		data.Passages = append(data.Passages, passage)
	}

	// Sort entities by ID for deterministic output
	sort.Slice(data.Entities, func(i, j int) bool {
		return data.Entities[i].ID < data.Entities[j].ID
	})

	return data
}

// roomToZoneData converts a spatial.Room to ZoneData
func (e *BasicEnvironment) roomToZoneData(roomID string, room spatial.Room, origin spatial.CubeCoordinate) ZoneData {
	zone := ZoneData{
		ID:        roomID,
		Type:      string(room.GetType()),
		Origin:    origin,
		EntityIDs: make([]string, 0),
	}

	grid := room.GetGrid()
	if grid == nil {
		// Default to reasonable values if no grid
		zone.Width = 10
		zone.Height = 10
		zone.GridShape = GridShapeHex
		zone.Orientation = HexOrientationPointy
		return zone
	}

	dims := grid.GetDimensions()
	zone.Width = int(dims.Width)
	zone.Height = int(dims.Height)
	zone.GridShape = gridShapeToValue(grid.GetShape())

	// Get orientation if hex grid
	if grid.GetShape() == spatial.GridShapeHex {
		if hexGrid, ok := grid.(*spatial.HexGrid); ok {
			zone.Orientation = hexOrientationToValue(hexGrid.GetOrientation())
		} else {
			zone.Orientation = HexOrientationPointy // Default
		}
	}

	// Collect entity IDs
	for _, entity := range room.GetAllEntities() {
		zone.EntityIDs = append(zone.EntityIDs, entity.GetID())
	}
	sort.Strings(zone.EntityIDs)

	return zone
}

// roomEntitiesToData converts entities in a room to PlacedEntityData with absolute positions
func (e *BasicEnvironment) roomEntitiesToData(
	roomID string, room spatial.Room, origin spatial.CubeCoordinate,
) []PlacedEntityData {
	entities := make([]PlacedEntityData, 0)

	grid := room.GetGrid()
	if grid == nil {
		return entities
	}

	for _, entity := range room.GetAllEntities() {
		pos, found := room.GetEntityPosition(entity.GetID())
		if !found {
			continue
		}

		// Convert room-local position to absolute
		absPos := e.localToAbsolute(pos, origin, grid)

		placed := PlacedEntityData{
			ID:       entity.GetID(),
			Type:     string(entity.GetType()),
			Position: absPos,
			ZoneID:   roomID,
		}

		// Extract Placeable properties if available
		if placeable, ok := entity.(spatial.Placeable); ok {
			placed.Size = placeable.GetSize()
			placed.BlocksMovement = placeable.BlocksMovement()
			placed.BlocksLoS = placeable.BlocksLineOfSight()
		} else {
			placed.Size = 1 // Default
		}

		// Extract Subtype if available
		if subtyped, ok := entity.(interface{ GetSubtype() string }); ok {
			placed.Subtype = subtyped.GetSubtype()
		}

		// Extract Properties if available
		if propped, ok := entity.(interface{ GetProperties() map[PropertyKey]any }); ok {
			placed.Properties = propped.GetProperties()
		}

		entities = append(entities, placed)
	}

	return entities
}

// localToAbsolute converts a room-local position to absolute environment coordinates
func (e *BasicEnvironment) localToAbsolute(
	pos spatial.Position, origin spatial.CubeCoordinate, grid spatial.Grid,
) spatial.CubeCoordinate {
	if grid.GetShape() == spatial.GridShapeHex {
		// For hex grids, convert offset to cube then add origin
		localCube := spatial.OffsetCoordinateToCube(pos)
		return spatial.CubeCoordinate{
			X: origin.X + localCube.X,
			Y: origin.Y + localCube.Y,
			Z: origin.Z + localCube.Z,
		}
	}

	// For square/gridless, use X/Z mapping and derive Y
	x := origin.X + int(pos.X)
	z := origin.Z + int(pos.Y)
	y := -x - z // Maintain cube constraint (x + y + z = 0)

	return spatial.CubeCoordinate{X: x, Y: y, Z: z}
}

// ============================================================================
// LoadFromData - Reconstruct BasicEnvironment from EnvironmentData
// ============================================================================

// LoadFromDataInput contains parameters for loading an environment from data
type LoadFromDataInput struct {
	Data     EnvironmentData
	EventBus events.EventBus
}

// LoadFromDataOutput contains the result of loading an environment
type LoadFromDataOutput struct {
	Environment     *BasicEnvironment
	PlacementErrors []error // Non-fatal errors during entity placement
}

// LoadFromData creates a BasicEnvironment from EnvironmentData.
// This reconstructs the environment from persisted data.
//
// Returns the environment and any non-fatal placement errors.
// Fatal errors (missing ID, invalid grid shape) return an error.
func LoadFromData(input LoadFromDataInput) (*LoadFromDataOutput, error) {
	data := input.Data
	eventBus := input.EventBus

	if data.ID == "" {
		return nil, fmt.Errorf("environment ID is required")
	}

	// Create orchestrator for managing rooms
	orchestrator := spatial.NewBasicRoomOrchestrator(spatial.BasicRoomOrchestratorConfig{
		ID:   spatial.OrchestratorID(data.ID + "-orchestrator"),
		Type: "orchestrator",
	})
	if eventBus != nil {
		orchestrator.ConnectToEventBus(eventBus)
	}

	// Track room positions for coordinate conversion
	roomPositions := make(map[string]spatial.CubeCoordinate)
	placementErrors := make([]error, 0)

	// Create rooms from zones
	for _, zone := range data.Zones {
		roomPositions[zone.ID] = zone.Origin

		grid, err := createGridFromZone(zone)
		if err != nil {
			return nil, fmt.Errorf("failed to create grid for zone %s: %w", zone.ID, err)
		}

		room := spatial.NewBasicRoom(spatial.BasicRoomConfig{
			ID:   zone.ID,
			Type: zone.Type,
			Grid: grid,
		})
		if eventBus != nil {
			room.ConnectToEventBus(eventBus)
		}

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
			placementErrors = append(placementErrors,
				fmt.Errorf("entity %s: zone %s not found", placed.ID, placed.ZoneID))
			continue
		}

		origin := roomPositions[placed.ZoneID]
		grid := room.GetGrid()
		if grid == nil {
			placementErrors = append(placementErrors,
				fmt.Errorf("entity %s: zone %s has no grid", placed.ID, placed.ZoneID))
			continue
		}

		// Convert absolute position to room-local
		localPos := absoluteToLocal(placed.Position, origin, grid)

		// Create entity from persisted data
		entity := newPersistedEntity(placed)

		if err := room.PlaceEntity(entity, localPos); err != nil {
			placementErrors = append(placementErrors,
				fmt.Errorf("entity %s at position %v: %w", placed.ID, localPos, err))
			continue
		}
	}

	// Calculate blocked hexes from placed entities
	blockedHexes := make(map[spatial.CubeCoordinate]bool)
	for roomID, room := range orchestrator.GetAllRooms() {
		origin := roomPositions[roomID]
		grid := room.GetGrid()
		if grid == nil {
			continue
		}

		for _, entity := range room.GetAllEntities() {
			placeable, ok := entity.(spatial.Placeable)
			if !ok || !placeable.BlocksMovement() {
				continue
			}

			pos, found := room.GetEntityPosition(entity.GetID())
			if !found {
				continue
			}

			// Convert to absolute and mark as blocked
			absPos := localToAbsoluteStatic(pos, origin, grid)
			blockedHexes[absPos] = true
		}
	}

	// Create the environment
	env := NewBasicEnvironment(BasicEnvironmentConfig{
		ID:            data.ID,
		Type:          string(data.Type),
		Theme:         data.Theme,
		Metadata:      data.Metadata,
		Orchestrator:  orchestrator,
		RoomPositions: roomPositions,
		BlockedHexes:  blockedHexes,
	})

	if eventBus != nil {
		env.ConnectToEventBus(eventBus)
	}

	return &LoadFromDataOutput{
		Environment:     env,
		PlacementErrors: placementErrors,
	}, nil
}

// createGridFromZone creates the appropriate Grid type based on ZoneData
func createGridFromZone(zone ZoneData) (spatial.Grid, error) {
	shape, err := valueToGridShape(zone.GridShape)
	if err != nil {
		return nil, err
	}

	switch shape {
	case spatial.GridShapeHex:
		return spatial.NewHexGrid(spatial.HexGridConfig{
			Width:       float64(zone.Width),
			Height:      float64(zone.Height),
			Orientation: valueToHexOrientation(zone.Orientation),
		}), nil

	case spatial.GridShapeSquare:
		return spatial.NewSquareGrid(spatial.SquareGridConfig{
			Width:  float64(zone.Width),
			Height: float64(zone.Height),
		}), nil

	case spatial.GridShapeGridless:
		return spatial.NewGridlessRoom(spatial.GridlessConfig{
			Width:  float64(zone.Width),
			Height: float64(zone.Height),
		}), nil

	default:
		return nil, fmt.Errorf("unsupported grid shape: %s", zone.GridShape)
	}
}

// absoluteToLocal converts an absolute environment position to room-local coordinates
func absoluteToLocal(absPos spatial.CubeCoordinate, origin spatial.CubeCoordinate, _ spatial.Grid) spatial.Position {
	localCube := spatial.CubeCoordinate{
		X: absPos.X - origin.X,
		Y: absPos.Y - origin.Y,
		Z: absPos.Z - origin.Z,
	}
	return localCube.ToOffsetCoordinate()
}

// localToAbsoluteStatic is a static version of localToAbsolute for use outside method context
func localToAbsoluteStatic(
	pos spatial.Position, origin spatial.CubeCoordinate, grid spatial.Grid,
) spatial.CubeCoordinate {
	if grid.GetShape() == spatial.GridShapeHex {
		localCube := spatial.OffsetCoordinateToCube(pos)
		return spatial.CubeCoordinate{
			X: origin.X + localCube.X,
			Y: origin.Y + localCube.Y,
			Z: origin.Z + localCube.Z,
		}
	}

	x := origin.X + int(pos.X)
	z := origin.Z + int(pos.Y)
	y := -x - z

	return spatial.CubeCoordinate{X: x, Y: y, Z: z}
}

// ============================================================================
// Persisted Entity
// ============================================================================

// persistedEntity is a minimal entity implementation for loading persisted data.
// Implements both core.Entity and spatial.Placeable.
type persistedEntity struct {
	id             string
	entityType     string
	subtype        string
	size           int
	blocksMovement bool
	blocksLoS      bool
	properties     map[PropertyKey]any
}

// newPersistedEntity creates a persistedEntity from PlacedEntityData
func newPersistedEntity(data PlacedEntityData) *persistedEntity {
	return &persistedEntity{
		id:             data.ID,
		entityType:     data.Type,
		subtype:        data.Subtype,
		size:           max(data.Size, 1),
		blocksMovement: data.BlocksMovement,
		blocksLoS:      data.BlocksLoS,
		properties:     data.Properties,
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

// GetSubtype returns the entity subtype
func (e *persistedEntity) GetSubtype() string {
	return e.subtype
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

// GetProperties returns the entity properties
func (e *persistedEntity) GetProperties() map[PropertyKey]any {
	return e.properties
}
