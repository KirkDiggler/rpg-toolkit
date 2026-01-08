package environments

import (
	"testing"
	"time"

	"github.com/stretchr/testify/suite"

	"github.com/KirkDiggler/rpg-toolkit/core"
	"github.com/KirkDiggler/rpg-toolkit/events"
	"github.com/KirkDiggler/rpg-toolkit/tools/spatial"
)

type EnvironmentPersistenceTestSuite struct {
	suite.Suite
	eventBus events.EventBus
}

func TestEnvironmentPersistenceSuite(t *testing.T) {
	suite.Run(t, new(EnvironmentPersistenceTestSuite))
}

func (s *EnvironmentPersistenceTestSuite) SetupTest() {
	s.eventBus = events.NewEventBus()
}

func (s *EnvironmentPersistenceTestSuite) TestToDataAndLoadFromData() {
	s.Run("round trip preserves environment structure", func() {
		// Create a simple environment with two rooms and one connection
		orchestrator := spatial.NewBasicRoomOrchestrator(spatial.BasicRoomOrchestratorConfig{
			ID:   "test-orch",
			Type: "orchestrator",
		})
		orchestrator.ConnectToEventBus(s.eventBus)

		// Create rooms
		grid1 := spatial.NewHexGrid(spatial.HexGridConfig{
			Width:  10,
			Height: 10,
		})
		room1 := spatial.NewBasicRoom(spatial.BasicRoomConfig{
			ID:   "room-1",
			Type: "entrance",
			Grid: grid1,
		})
		room1.ConnectToEventBus(s.eventBus)

		grid2 := spatial.NewHexGrid(spatial.HexGridConfig{
			Width:  15,
			Height: 15,
		})
		room2 := spatial.NewBasicRoom(spatial.BasicRoomConfig{
			ID:   "room-2",
			Type: "combat",
			Grid: grid2,
		})
		room2.ConnectToEventBus(s.eventBus)

		s.Require().NoError(orchestrator.AddRoom(room1))
		s.Require().NoError(orchestrator.AddRoom(room2))

		// Add connection
		conn := spatial.NewBasicConnection(spatial.BasicConnectionConfig{
			ID:         "conn-1",
			FromRoom:   "room-1",
			ToRoom:     "room-2",
			Reversible: true,
			Passable:   true,
		})
		s.Require().NoError(orchestrator.AddConnection(conn))

		// Place an entity
		entity := &testEntity{
			id:             "entity-1",
			entityType:     "monster",
			blocksMovement: true,
		}
		s.Require().NoError(room1.PlaceEntity(entity, spatial.Position{X: 3, Y: 2}))

		// Create environment with room positions
		roomPositions := map[string]spatial.CubeCoordinate{
			"room-1": {X: 0, Y: 0, Z: 0},
			"room-2": {X: 15, Y: -8, Z: -7},
		}

		env := NewBasicEnvironment(BasicEnvironmentConfig{
			ID:            "test-env",
			Type:          "dungeon",
			Theme:         "dark-crypt",
			Orchestrator:  orchestrator,
			RoomPositions: roomPositions,
			Metadata: EnvironmentMetadata{
				Name:        "Test Environment",
				Description: "A test dungeon",
				GeneratedAt: time.Date(2026, 1, 8, 12, 0, 0, 0, time.UTC),
				GeneratedBy: "test",
				Version:     "1.0.0",
			},
		})

		// Convert to data
		data := env.ToData()

		// Verify data structure
		s.Assert().Equal("test-env", data.ID)
		s.Assert().Equal(EnvironmentType("dungeon"), data.Type)
		s.Assert().Equal("dark-crypt", data.Theme)
		s.Assert().Equal("Test Environment", data.Metadata.Name)
		s.Require().Len(data.Zones, 2)
		s.Require().Len(data.Passages, 1)
		s.Require().Len(data.Entities, 1)

		// Verify zone data (sorted by ID)
		s.Assert().Equal("room-1", data.Zones[0].ID)
		s.Assert().Equal("entrance", data.Zones[0].Type)
		s.Assert().Equal(GridShapeHex, data.Zones[0].GridShape)
		s.Assert().Equal("room-2", data.Zones[1].ID)
		s.Assert().Equal("combat", data.Zones[1].Type)

		// Verify passage data
		s.Assert().Equal("conn-1", data.Passages[0].ID)
		s.Assert().Equal("room-1", data.Passages[0].FromZoneID)
		s.Assert().Equal("room-2", data.Passages[0].ToZoneID)
		s.Assert().True(data.Passages[0].Bidirectional)

		// Verify entity data
		s.Assert().Equal("entity-1", data.Entities[0].ID)
		s.Assert().Equal("monster", data.Entities[0].Type)
		s.Assert().Equal("room-1", data.Entities[0].ZoneID)

		// Load from data
		newEventBus := events.NewEventBus()
		output, err := LoadFromData(LoadFromDataInput{
			Data:     data,
			EventBus: newEventBus,
		})
		s.Require().NoError(err)
		s.Require().NotNil(output)
		s.Assert().Empty(output.PlacementErrors)

		restored := output.Environment

		// Verify restored environment
		s.Assert().Equal("test-env", restored.GetID())
		s.Assert().Equal(core.EntityType("dungeon"), restored.GetType())
		s.Assert().Equal("dark-crypt", restored.GetTheme())
		s.Assert().Equal("Test Environment", restored.GetMetadata().Name)
		s.Assert().Len(restored.GetRooms(), 2)
		s.Assert().Len(restored.GetConnections(), 1)

		// Verify room positions preserved
		pos1, found1 := restored.GetRoomPosition("room-1")
		s.Assert().True(found1)
		s.Assert().Equal(spatial.CubeCoordinate{X: 0, Y: 0, Z: 0}, pos1)

		pos2, found2 := restored.GetRoomPosition("room-2")
		s.Assert().True(found2)
		s.Assert().Equal(spatial.CubeCoordinate{X: 15, Y: -8, Z: -7}, pos2)
	})
}

func (s *EnvironmentPersistenceTestSuite) TestDeterministicOutput() {
	s.Run("ToData produces identical output on repeated calls", func() {
		// Create environment with multiple rooms
		orchestrator := spatial.NewBasicRoomOrchestrator(spatial.BasicRoomOrchestratorConfig{
			ID:   "test-orch",
			Type: "orchestrator",
		})

		// Add rooms in non-alphabetical order
		for _, id := range []string{"room-c", "room-a", "room-b"} {
			grid := spatial.NewHexGrid(spatial.HexGridConfig{Width: 10, Height: 10})
			room := spatial.NewBasicRoom(spatial.BasicRoomConfig{ID: id, Type: "room", Grid: grid})
			s.Require().NoError(orchestrator.AddRoom(room))
		}

		env := NewBasicEnvironment(BasicEnvironmentConfig{
			ID:           "test-env",
			Type:         "test",
			Orchestrator: orchestrator,
			RoomPositions: map[string]spatial.CubeCoordinate{
				"room-a": {X: 0, Y: 0, Z: 0},
				"room-b": {X: 10, Y: -5, Z: -5},
				"room-c": {X: 20, Y: -10, Z: -10},
			},
		})

		// Call ToData multiple times
		data1 := env.ToData()
		data2 := env.ToData()
		data3 := env.ToData()

		// Zones should be in alphabetical order
		s.Require().Len(data1.Zones, 3)
		s.Assert().Equal("room-a", data1.Zones[0].ID)
		s.Assert().Equal("room-b", data1.Zones[1].ID)
		s.Assert().Equal("room-c", data1.Zones[2].ID)

		// All calls should produce same order
		s.Assert().Equal(data1.Zones[0].ID, data2.Zones[0].ID)
		s.Assert().Equal(data1.Zones[0].ID, data3.Zones[0].ID)
	})
}

func (s *EnvironmentPersistenceTestSuite) TestLoadFromDataErrors() {
	s.Run("returns error for empty ID", func() {
		data := EnvironmentData{
			ID: "",
		}
		_, err := LoadFromData(LoadFromDataInput{Data: data, EventBus: s.eventBus})
		s.Assert().Error(err)
		s.Assert().Contains(err.Error(), "ID is required")
	})

	s.Run("returns error for invalid grid shape", func() {
		data := EnvironmentData{
			ID: "test-env",
			Zones: []ZoneData{
				{
					ID:        "zone-1",
					Type:      "room",
					GridShape: "invalid-shape",
					Width:     10,
					Height:    10,
				},
			},
		}
		_, err := LoadFromData(LoadFromDataInput{Data: data, EventBus: s.eventBus})
		s.Assert().Error(err)
		s.Assert().Contains(err.Error(), "unknown grid shape")
	})
}

func (s *EnvironmentPersistenceTestSuite) TestPlacementErrorsCollected() {
	s.Run("collects placement errors without failing", func() {
		data := EnvironmentData{
			ID:   "test-env",
			Type: "test",
			Zones: []ZoneData{
				{
					ID:        "zone-1",
					Type:      "room",
					GridShape: GridShapeHex,
					Width:     10,
					Height:    10,
					Origin:    spatial.CubeCoordinate{X: 0, Y: 0, Z: 0},
				},
			},
			Entities: []PlacedEntityData{
				{
					ID:       "entity-1",
					Type:     "monster",
					ZoneID:   "nonexistent-zone", // Invalid zone
					Position: spatial.CubeCoordinate{X: 5, Y: -2, Z: -3},
				},
			},
		}

		output, err := LoadFromData(LoadFromDataInput{Data: data, EventBus: s.eventBus})
		s.Require().NoError(err) // Should not fail
		s.Require().NotNil(output)
		s.Assert().Len(output.PlacementErrors, 1)
		s.Assert().Contains(output.PlacementErrors[0].Error(), "zone nonexistent-zone not found")
	})
}

func (s *EnvironmentPersistenceTestSuite) TestAllGridShapes() {
	s.Run("hex grid round trips correctly", func() {
		s.verifyGridShapeRoundTrip(GridShapeHex, HexOrientationPointy)
	})

	s.Run("hex grid flat orientation round trips correctly", func() {
		s.verifyGridShapeRoundTrip(GridShapeHex, HexOrientationFlat)
	})

	s.Run("square grid round trips correctly", func() {
		s.verifyGridShapeRoundTrip(GridShapeSquare, "")
	})

	s.Run("gridless round trips correctly", func() {
		s.verifyGridShapeRoundTrip(GridShapeGridless, "")
	})
}

func (s *EnvironmentPersistenceTestSuite) verifyGridShapeRoundTrip(
	shape GridShapeValue, orientation HexOrientationValue,
) {
	data := EnvironmentData{
		ID:   "test-env",
		Type: "test",
		Zones: []ZoneData{
			{
				ID:          "zone-1",
				Type:        "room",
				GridShape:   shape,
				Orientation: orientation,
				Width:       10,
				Height:      10,
				Origin:      spatial.CubeCoordinate{X: 0, Y: 0, Z: 0},
			},
		},
	}

	output, err := LoadFromData(LoadFromDataInput{Data: data, EventBus: s.eventBus})
	s.Require().NoError(err)
	s.Require().NotNil(output)

	// Convert back to data
	restoredData := output.Environment.ToData()
	s.Require().Len(restoredData.Zones, 1)
	s.Assert().Equal(shape, restoredData.Zones[0].GridShape)

	if shape == GridShapeHex && orientation != "" {
		s.Assert().Equal(orientation, restoredData.Zones[0].Orientation)
	}
}

func (s *EnvironmentPersistenceTestSuite) TestNilOrchestratorSafe() {
	s.Run("ToData handles nil orchestrator", func() {
		env := NewBasicEnvironment(BasicEnvironmentConfig{
			ID:   "test-env",
			Type: "test",
			// No orchestrator
		})

		// Should not panic
		data := env.ToData()
		s.Assert().Equal("test-env", data.ID)
		s.Assert().Empty(data.Zones)
		s.Assert().Empty(data.Passages)
		s.Assert().Empty(data.Entities)
	})
}

// testEntity is a simple entity for testing
type testEntity struct {
	id             string
	entityType     string
	subtype        string
	blocksMovement bool
	blocksLoS      bool
}

func (e *testEntity) GetID() string {
	return e.id
}

func (e *testEntity) GetType() core.EntityType {
	return core.EntityType(e.entityType)
}

func (e *testEntity) GetSubtype() string {
	return e.subtype
}

func (e *testEntity) GetSize() int {
	return 1
}

func (e *testEntity) BlocksMovement() bool {
	return e.blocksMovement
}

func (e *testEntity) BlocksLineOfSight() bool {
	return e.blocksLoS
}
