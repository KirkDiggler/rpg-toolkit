package environments

import (
	"testing"

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
			Orchestrator:  orchestrator,
			RoomPositions: roomPositions,
		})

		// Convert to data
		data := env.ToData()

		// Verify data structure
		s.Assert().Equal("test-env", data.ID)
		s.Require().Len(data.Zones, 2)
		s.Require().Len(data.Passages, 1)
		s.Require().Len(data.Entities, 1)

		// Verify zone data
		var entranceZone, combatZone *ZoneData
		for i := range data.Zones {
			switch data.Zones[i].ID {
			case "room-1":
				entranceZone = &data.Zones[i]
			case "room-2":
				combatZone = &data.Zones[i]
			}
		}
		s.Require().NotNil(entranceZone)
		s.Require().NotNil(combatZone)
		s.Assert().Equal("entrance", entranceZone.Type)
		s.Assert().Equal("combat", combatZone.Type)
		s.Assert().Equal(spatial.CubeCoordinate{X: 0, Y: 0, Z: 0}, entranceZone.Origin)
		s.Assert().Equal(spatial.CubeCoordinate{X: 15, Y: -8, Z: -7}, combatZone.Origin)

		// Verify passage data
		s.Assert().Equal("conn-1", data.Passages[0].ID)
		s.Assert().Equal("room-1", data.Passages[0].FromZoneID)
		s.Assert().Equal("room-2", data.Passages[0].ToZoneID)
		s.Assert().True(data.Passages[0].Bidirectional)

		// Verify entity data (position should be absolute)
		s.Assert().Equal("entity-1", data.Entities[0].ID)
		s.Assert().Equal("monster", data.Entities[0].Type)
		s.Assert().Equal("room-1", data.Entities[0].ZoneID)

		// Load from data
		newEventBus := events.NewEventBus()
		restored, err := LoadFromData(data, newEventBus)
		s.Require().NoError(err)

		// Verify restored environment
		s.Assert().Equal("test-env", restored.GetID())
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

func (s *EnvironmentPersistenceTestSuite) TestLoadFromDataErrors() {
	s.Run("returns error for empty ID", func() {
		data := EnvironmentData{
			ID: "",
		}
		_, err := LoadFromData(data, s.eventBus)
		s.Assert().Error(err)
		s.Assert().Contains(err.Error(), "ID is required")
	})
}

// testEntity is a simple entity for testing
type testEntity struct {
	id             string
	entityType     string
	blocksMovement bool
}

func (e *testEntity) GetID() string {
	return e.id
}

func (e *testEntity) GetType() core.EntityType {
	return core.EntityType(e.entityType)
}

func (e *testEntity) GetSize() int {
	return 1
}

func (e *testEntity) BlocksMovement() bool {
	return e.blocksMovement
}

func (e *testEntity) BlocksLineOfSight() bool {
	return false
}
