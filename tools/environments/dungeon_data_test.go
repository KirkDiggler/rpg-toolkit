package environments

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/KirkDiggler/rpg-toolkit/tools/spatial"
)

type DungeonDataTestSuite struct {
	suite.Suite
}

func TestDungeonDataSuite(t *testing.T) {
	suite.Run(t, new(DungeonDataTestSuite))
}

func (s *DungeonDataTestSuite) TestDungeonDataRoundTrip() {
	s.Run("complete dungeon serializes and deserializes correctly", func() {
		original := DungeonData{
			ID:          "dungeon-001",
			Seed:        12345,
			Theme:       "crypt",
			StartRoomID: "room-entrance",
			BossRoomID:  "room-boss",
			Rooms: []DungeonRoomData{
				{
					ID:        "room-entrance",
					Type:      "entrance",
					Origin:    spatial.CubeCoordinate{X: 0, Y: 0, Z: 0},
					Width:     10,
					Height:    10,
					EntityIDs: []string{"monster-1", "obstacle-1"},
				},
				{
					ID:        "room-boss",
					Type:      "boss",
					Origin:    spatial.CubeCoordinate{X: 15, Y: -8, Z: -7},
					Width:     15,
					Height:    15,
					EntityIDs: []string{"monster-2", "door-1"},
				},
			},
			Connections: []DungeonConnectionData{
				{
					ID:            "conn-1",
					FromRoomID:    "room-entrance",
					ToRoomID:      "room-boss",
					DoorEntityID:  "door-1",
					Bidirectional: true,
				},
			},
			Entities: []DungeonEntityData{
				{
					ID:             "monster-1",
					Type:           DungeonEntityTypeMonster,
					Position:       spatial.CubeCoordinate{X: 3, Y: -1, Z: -2},
					Size:           1,
					BlocksMovement: true,
					BlocksLoS:      false,
					RoomID:         "room-entrance",
					VisualType:     "skeleton",
					Properties: map[string]any{
						"cr": 0.25,
						"hp": 13,
					},
				},
				{
					ID:             "obstacle-1",
					Type:           DungeonEntityTypeObstacle,
					Position:       spatial.CubeCoordinate{X: 5, Y: -2, Z: -3},
					Size:           1,
					BlocksMovement: true,
					BlocksLoS:      true,
					RoomID:         "room-entrance",
					VisualType:     "pillar",
				},
				{
					ID:             "door-1",
					Type:           DungeonEntityTypeDoor,
					Position:       spatial.CubeCoordinate{X: 9, Y: -4, Z: -5},
					Size:           1,
					BlocksMovement: true,
					BlocksLoS:      true,
					RoomID:         "room-entrance",
					VisualType:     "wooden_door",
					Properties: map[string]any{
						"open":    false,
						"locked":  true,
						"trap_dc": 15,
					},
				},
			},
			Walls: []DungeonWallData{
				{
					Start:          spatial.CubeCoordinate{X: 0, Y: 0, Z: 0},
					End:            spatial.CubeCoordinate{X: 9, Y: -4, Z: -5},
					BlocksMovement: true,
					BlocksLoS:      true,
				},
				{
					Start:          spatial.CubeCoordinate{X: 9, Y: -4, Z: -5},
					End:            spatial.CubeCoordinate{X: 9, Y: -9, Z: 0},
					BlocksMovement: true,
					BlocksLoS:      true,
					Destructible:   true,
				},
			},
			RevealedRooms: []string{"room-entrance"},
		}

		// Serialize
		data, err := json.Marshal(original)
		s.Require().NoError(err)
		s.Require().NotEmpty(data)

		// Deserialize
		var restored DungeonData
		err = json.Unmarshal(data, &restored)
		s.Require().NoError(err)

		// Verify top-level fields
		s.Assert().Equal(original.ID, restored.ID)
		s.Assert().Equal(original.Seed, restored.Seed)
		s.Assert().Equal(original.Theme, restored.Theme)
		s.Assert().Equal(original.StartRoomID, restored.StartRoomID)
		s.Assert().Equal(original.BossRoomID, restored.BossRoomID)

		// Verify rooms
		s.Require().Len(restored.Rooms, 2)
		s.Assert().Equal(original.Rooms[0].ID, restored.Rooms[0].ID)
		s.Assert().Equal(original.Rooms[0].Origin, restored.Rooms[0].Origin)
		s.Assert().Equal(original.Rooms[1].Origin, restored.Rooms[1].Origin)

		// Verify connections
		s.Require().Len(restored.Connections, 1)
		s.Assert().Equal(original.Connections[0].DoorEntityID, restored.Connections[0].DoorEntityID)
		s.Assert().Equal(original.Connections[0].Bidirectional, restored.Connections[0].Bidirectional)

		// Verify entities
		s.Require().Len(restored.Entities, 3)
		s.Assert().Equal(original.Entities[0].Position, restored.Entities[0].Position)
		// JSON numbers become float64 when unmarshaled into map[string]any
		s.Assert().Equal(float64(15), restored.Entities[2].Properties["trap_dc"])

		// Verify walls
		s.Require().Len(restored.Walls, 2)
		s.Assert().Equal(original.Walls[0].Start, restored.Walls[0].Start)
		s.Assert().Equal(original.Walls[1].Destructible, restored.Walls[1].Destructible)

		// Verify revealed rooms
		s.Assert().Equal(original.RevealedRooms, restored.RevealedRooms)
	})
}

func (s *DungeonDataTestSuite) TestCubeCoordinatesValid() {
	s.Run("positions use valid cube coordinates (x+y+z=0)", func() {
		// All cube coordinates in our test data must satisfy x + y + z = 0
		testCoords := []spatial.CubeCoordinate{
			{X: 0, Y: 0, Z: 0},
			{X: 3, Y: -1, Z: -2},
			{X: 5, Y: -2, Z: -3},
			{X: 9, Y: -4, Z: -5},
			{X: 15, Y: -8, Z: -7},
		}

		for _, coord := range testCoords {
			s.Assert().True(coord.IsValid(), "coordinate %v should be valid", coord)
		}
	})
}

func (s *DungeonDataTestSuite) TestEntityTypes() {
	s.Run("all entity types can be represented", func() {
		entities := []DungeonEntityData{
			{
				ID:         "monster-test",
				Type:       DungeonEntityTypeMonster,
				Position:   spatial.CubeCoordinate{X: 1, Y: -1, Z: 0},
				VisualType: "goblin",
				Properties: map[string]any{
					"cr":         0.5,
					"hp":         7,
					"initiative": 14,
				},
			},
			{
				ID:         "obstacle-test",
				Type:       DungeonEntityTypeObstacle,
				Position:   spatial.CubeCoordinate{X: 2, Y: -1, Z: -1},
				VisualType: "crate",
				Properties: map[string]any{
					"destructible": true,
					"hp":           10,
				},
			},
			{
				ID:         "door-test",
				Type:       DungeonEntityTypeDoor,
				Position:   spatial.CubeCoordinate{X: 3, Y: -2, Z: -1},
				VisualType: "iron_door",
				Properties: map[string]any{
					"open":   false,
					"locked": true,
					"key_id": "key-boss-room",
				},
			},
			{
				ID:         "character-test",
				Type:       DungeonEntityTypeCharacter,
				Position:   spatial.CubeCoordinate{X: 0, Y: 0, Z: 0},
				VisualType: "fighter",
			},
		}

		// Serialize
		data, err := json.Marshal(entities)
		s.Require().NoError(err)

		// Deserialize
		var restored []DungeonEntityData
		err = json.Unmarshal(data, &restored)
		s.Require().NoError(err)

		// Verify types preserved
		s.Require().Len(restored, 4)
		s.Assert().Equal(DungeonEntityTypeMonster, restored[0].Type)
		s.Assert().Equal(DungeonEntityTypeObstacle, restored[1].Type)
		s.Assert().Equal(DungeonEntityTypeDoor, restored[2].Type)
		s.Assert().Equal(DungeonEntityTypeCharacter, restored[3].Type)

		// Verify type-specific properties
		s.Assert().Equal(0.5, restored[0].Properties["cr"])
		s.Assert().Equal(true, restored[1].Properties["destructible"])
		s.Assert().Equal("key-boss-room", restored[2].Properties["key_id"])
	})
}

func (s *DungeonDataTestSuite) TestDoorProperties() {
	s.Run("door properties serialize and deserialize correctly", func() {
		door := DungeonEntityData{
			ID:             "door-complex",
			Type:           DungeonEntityTypeDoor,
			Position:       spatial.CubeCoordinate{X: 5, Y: -3, Z: -2},
			Size:           1,
			BlocksMovement: true,
			BlocksLoS:      true,
			RoomID:         "room-1",
			VisualType:     "magical_barrier",
			Properties: map[string]any{
				"open":                false,
				"locked":              true,
				"trap_dc":             15,
				"trap_damage":         "2d6",
				"key_id":              "skeleton-key",
				"dispel_dc":           17,
				"requires_attunement": true,
			},
		}

		// Serialize
		data, err := json.Marshal(door)
		s.Require().NoError(err)

		// Deserialize
		var restored DungeonEntityData
		err = json.Unmarshal(data, &restored)
		s.Require().NoError(err)

		// Verify all properties round-trip
		s.Assert().Equal(false, restored.Properties["open"])
		s.Assert().Equal(true, restored.Properties["locked"])
		s.Assert().Equal(float64(15), restored.Properties["trap_dc"]) // JSON numbers become float64
		s.Assert().Equal("2d6", restored.Properties["trap_damage"])
		s.Assert().Equal("skeleton-key", restored.Properties["key_id"])
		s.Assert().Equal(float64(17), restored.Properties["dispel_dc"])
		s.Assert().Equal(true, restored.Properties["requires_attunement"])
	})
}

func (s *DungeonDataTestSuite) TestEmptyDungeon() {
	s.Run("empty dungeon serializes correctly", func() {
		empty := DungeonData{
			ID:          "empty-dungeon",
			Seed:        1,
			Theme:       "test",
			StartRoomID: "room-1",
			Rooms:       []DungeonRoomData{},
			Connections: []DungeonConnectionData{},
			Entities:    []DungeonEntityData{},
			Walls:       []DungeonWallData{},
		}

		data, err := json.Marshal(empty)
		s.Require().NoError(err)

		var restored DungeonData
		err = json.Unmarshal(data, &restored)
		s.Require().NoError(err)

		s.Assert().Equal(empty.ID, restored.ID)
		s.Assert().Empty(restored.Rooms)
		s.Assert().Empty(restored.Entities)
	})
}

func (s *DungeonDataTestSuite) TestConnectionBidirectional() {
	s.Run("bidirectional flag defaults correctly", func() {
		conn := DungeonConnectionData{
			ID:            "conn-1",
			FromRoomID:    "room-a",
			ToRoomID:      "room-b",
			Bidirectional: true,
		}

		data, err := json.Marshal(conn)
		s.Require().NoError(err)

		var restored DungeonConnectionData
		err = json.Unmarshal(data, &restored)
		s.Require().NoError(err)

		s.Assert().True(restored.Bidirectional)
	})

	s.Run("one-way connection supported", func() {
		conn := DungeonConnectionData{
			ID:            "conn-stairs",
			FromRoomID:    "room-upper",
			ToRoomID:      "room-lower",
			Bidirectional: false,
		}

		data, err := json.Marshal(conn)
		s.Require().NoError(err)

		var restored DungeonConnectionData
		err = json.Unmarshal(data, &restored)
		s.Require().NoError(err)

		s.Assert().False(restored.Bidirectional)
	})
}
