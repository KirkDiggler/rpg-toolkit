package spatial

import (
	"context"
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/KirkDiggler/rpg-toolkit/events"
	"github.com/KirkDiggler/rpg-toolkit/game"
)

// MockEntity implements core.Entity and Placeable for testing
type MockEntity struct {
	id                string
	entityType        string
	size              int
	blocksMovement    bool
	blocksLineOfSight bool
}

func (m *MockEntity) GetID() string {
	return m.id
}

func (m *MockEntity) GetType() string {
	return m.entityType
}

func (m *MockEntity) GetSize() int {
	if m.size < 1 {
		return 1
	}
	return m.size
}

func (m *MockEntity) BlocksMovement() bool {
	return m.blocksMovement
}

func (m *MockEntity) BlocksLineOfSight() bool {
	return m.blocksLineOfSight
}

// RoomDataTestSuite tests room data persistence functionality
type RoomDataTestSuite struct {
	suite.Suite
	eventBus events.EventBus
}

func (s *RoomDataTestSuite) SetupTest() {
	s.eventBus = events.NewBus()
}

func TestRoomDataSuite(t *testing.T) {
	suite.Run(t, new(RoomDataTestSuite))
}

func (s *RoomDataTestSuite) TestToDataBasicRoom() {
	// Create a basic room
	room := NewBasicRoom(BasicRoomConfig{
		ID:   "test-room",
		Type: "dungeon",
		Grid: NewSquareGrid(SquareGridConfig{
			Width:  10,
			Height: 10,
		}),
		EventBus: s.eventBus,
	})

	// Add some entities
	entity1 := &MockEntity{
		id:                "hero",
		entityType:        "character",
		size:              1,
		blocksMovement:    true,
		blocksLineOfSight: false,
	}
	entity2 := &MockEntity{
		id:                "goblin",
		entityType:        "monster",
		size:              1,
		blocksMovement:    true,
		blocksLineOfSight: true,
	}

	err := room.PlaceEntity(entity1, Position{X: 5, Y: 5})
	s.Require().NoError(err)

	err = room.PlaceEntity(entity2, Position{X: 3, Y: 7})
	s.Require().NoError(err)

	// Convert to data
	data := room.ToData()

	// Verify basic properties
	s.Equal("test-room", data.ID)
	s.Equal("dungeon", data.Type)
	s.Equal(10, data.Width)
	s.Equal(10, data.Height)
	s.Equal("square", data.GridType)

	// Verify entities
	s.Len(data.Entities, 2)

	heroPlacement, exists := data.Entities["hero"]
	s.True(exists)
	s.Equal("hero", heroPlacement.EntityID)
	s.Equal("character", heroPlacement.EntityType)
	s.Equal(Position{X: 5, Y: 5}, heroPlacement.Position)
	s.Equal(1, heroPlacement.Size)
	s.True(heroPlacement.BlocksMovement)
	s.False(heroPlacement.BlocksLineOfSight)

	goblinPlacement, exists := data.Entities["goblin"]
	s.True(exists)
	s.Equal("goblin", goblinPlacement.EntityID)
	s.Equal("monster", goblinPlacement.EntityType)
	s.Equal(Position{X: 3, Y: 7}, goblinPlacement.Position)
	s.Equal(1, goblinPlacement.Size)
	s.True(goblinPlacement.BlocksMovement)
	s.True(goblinPlacement.BlocksLineOfSight)
}

func (s *RoomDataTestSuite) TestToDataHexRoom() {
	// Create a hex room
	room := NewBasicRoom(BasicRoomConfig{
		ID:   "hex-room",
		Type: "outdoor",
		Grid: NewHexGrid(HexGridConfig{
			Width:  8,
			Height: 8,
		}),
		EventBus: s.eventBus,
	})

	// Convert to data
	data := room.ToData()

	// Verify grid type
	s.Equal("hex", data.GridType)
	s.Equal(8, data.Width)
	s.Equal(8, data.Height)
}

func (s *RoomDataTestSuite) TestToDataGridlessRoom() {
	// Create a gridless room
	room := NewBasicRoom(BasicRoomConfig{
		ID:   "gridless-room",
		Type: "theater",
		Grid: NewGridlessRoom(GridlessConfig{
			Width:  20,
			Height: 15,
		}),
		EventBus: s.eventBus,
	})

	// Convert to data
	data := room.ToData()

	// Verify grid type
	s.Equal("gridless", data.GridType)
	s.Equal(20, data.Width)
	s.Equal(15, data.Height)
}

func (s *RoomDataTestSuite) TestLoadRoomFromContext() {
	// Create room data
	roomData := RoomData{
		ID:       "loaded-room",
		Type:     "tavern",
		Width:    12,
		Height:   8,
		GridType: "square",
		Entities: map[string]EntityPlacement{
			"bartender": {
				EntityID:   "bartender",
				EntityType: "npc",
				Position:   Position{X: 6, Y: 2},
			},
		},
	}

	// Create game context
	gameCtx, err := game.NewContext(s.eventBus, roomData)
	s.Require().NoError(err)

	// Load room from context
	room, err := LoadRoomFromContext(context.Background(), gameCtx)
	s.Require().NoError(err)
	s.NotNil(room)

	// Verify room properties
	s.Equal("loaded-room", room.GetID())
	s.Equal("tavern", room.GetType())

	// Verify grid
	grid := room.GetGrid()
	s.Equal(GridShapeSquare, grid.GetShape())
	dims := grid.GetDimensions()
	s.Equal(float64(12), dims.Width)
	s.Equal(float64(8), dims.Height)

	// Verify entities were loaded as PlaceableData
	entities := room.GetAllEntities()
	s.Len(entities, 1)

	// Verify the bartender entity exists
	bartenderEntity, exists := entities["bartender"]
	s.True(exists)
	s.Equal("bartender", bartenderEntity.GetID())
	s.Equal("npc", bartenderEntity.GetType())

	// Verify position
	pos, ok := room.GetEntityPosition("bartender")
	s.True(ok)
	s.Equal(Position{X: 6, Y: 2}, pos)
}

func (s *RoomDataTestSuite) TestLoadRoomFromContextHex() {
	// Create hex room data
	roomData := RoomData{
		ID:       "hex-loaded",
		Type:     "wilderness",
		Width:    10,
		Height:   10,
		GridType: "hex",
	}

	// Create game context
	gameCtx, err := game.NewContext(s.eventBus, roomData)
	s.Require().NoError(err)

	// Load room from context
	room, err := LoadRoomFromContext(context.Background(), gameCtx)
	s.Require().NoError(err)

	// Verify hex grid
	grid := room.GetGrid()
	s.Equal(GridShapeHex, grid.GetShape())
}

func (s *RoomDataTestSuite) TestLoadRoomFromContextGridless() {
	// Create gridless room data
	roomData := RoomData{
		ID:       "gridless-loaded",
		Type:     "abstract",
		Width:    30,
		Height:   20,
		GridType: "gridless",
	}

	// Create game context
	gameCtx, err := game.NewContext(s.eventBus, roomData)
	s.Require().NoError(err)

	// Load room from context
	room, err := LoadRoomFromContext(context.Background(), gameCtx)
	s.Require().NoError(err)

	// Verify gridless grid
	grid := room.GetGrid()
	s.Equal(GridShapeGridless, grid.GetShape())
}

func (s *RoomDataTestSuite) TestLoadRoomFromContextInvalidGridType() {
	// Create room data with invalid grid type
	roomData := RoomData{
		ID:       "invalid-grid",
		Type:     "dungeon",
		Width:    10,
		Height:   10,
		GridType: "invalid",
	}

	// Create game context
	gameCtx, err := game.NewContext(s.eventBus, roomData)
	s.Require().NoError(err)

	// Load should fail
	room, err := LoadRoomFromContext(context.Background(), gameCtx)
	s.Error(err)
	s.Nil(room)
	s.Contains(err.Error(), "unknown grid type")
}

func (s *RoomDataTestSuite) TestRoundTripConversion() {
	// Create original room
	originalRoom := NewBasicRoom(BasicRoomConfig{
		ID:   "round-trip-test",
		Type: "castle",
		Grid: NewSquareGrid(SquareGridConfig{
			Width:  15,
			Height: 12,
		}),
		EventBus: s.eventBus,
	})

	// Add entities
	entity := &MockEntity{
		id:                "knight",
		entityType:        "character",
		size:              1,
		blocksMovement:    true,
		blocksLineOfSight: false,
	}
	err := originalRoom.PlaceEntity(entity, Position{X: 7, Y: 6})
	s.Require().NoError(err)

	// Convert to data
	data := originalRoom.ToData()

	// Create new room from data
	gameCtx, err := game.NewContext(s.eventBus, data)
	s.Require().NoError(err)

	loadedRoom, err := LoadRoomFromContext(context.Background(), gameCtx)
	s.Require().NoError(err)

	// Verify properties match
	s.Equal(originalRoom.GetID(), loadedRoom.GetID())
	s.Equal(originalRoom.GetType(), loadedRoom.GetType())

	// Verify grid dimensions match
	origDims := originalRoom.GetGrid().GetDimensions()
	loadedDims := loadedRoom.GetGrid().GetDimensions()
	s.Equal(origDims.Width, loadedDims.Width)
	s.Equal(origDims.Height, loadedDims.Height)

	// Verify grid type matches
	s.Equal(originalRoom.GetGrid().GetShape(), loadedRoom.GetGrid().GetShape())

	// Verify entity was loaded
	loadedEntities := loadedRoom.GetAllEntities()
	s.Len(loadedEntities, 1)

	// Verify entity position
	pos, ok := loadedRoom.GetEntityPosition("knight")
	s.True(ok)
	s.Equal(Position{X: 7, Y: 6}, pos)
}

func (s *RoomDataTestSuite) TestSpatialPropertiesPreserved() {
	// Create room with entities that have different spatial properties
	room := NewBasicRoom(BasicRoomConfig{
		ID:   "spatial-test",
		Type: "arena",
		Grid: NewSquareGrid(SquareGridConfig{
			Width:  20,
			Height: 20,
		}),
		EventBus: s.eventBus,
	})

	// Add a large creature that blocks LOS
	dragon := &MockEntity{
		id:                "dragon",
		entityType:        "monster",
		size:              3,
		blocksMovement:    true,
		blocksLineOfSight: true,
	}
	err := room.PlaceEntity(dragon, Position{X: 10, Y: 10})
	s.Require().NoError(err)

	// Add a ghost that doesn't block movement
	ghost := &MockEntity{
		id:                "ghost",
		entityType:        "undead",
		size:              1,
		blocksMovement:    false,
		blocksLineOfSight: false,
	}
	err = room.PlaceEntity(ghost, Position{X: 5, Y: 5})
	s.Require().NoError(err)

	// Convert to data
	data := room.ToData()

	// Verify dragon properties
	dragonPlacement := data.Entities["dragon"]
	s.Equal(3, dragonPlacement.Size)
	s.True(dragonPlacement.BlocksMovement)
	s.True(dragonPlacement.BlocksLineOfSight)

	// Verify ghost properties
	ghostPlacement := data.Entities["ghost"]
	s.Equal(1, ghostPlacement.Size)
	s.False(ghostPlacement.BlocksMovement)
	s.False(ghostPlacement.BlocksLineOfSight)

	// Load from data
	gameCtx, err := game.NewContext(s.eventBus, data)
	s.Require().NoError(err)

	loadedRoom, err := LoadRoomFromContext(context.Background(), gameCtx)
	s.Require().NoError(err)

	// Test that spatial queries work correctly
	// Ghost should not block line of sight
	blocked := loadedRoom.IsLineOfSightBlocked(Position{X: 0, Y: 5}, Position{X: 10, Y: 5})
	s.False(blocked, "Ghost should not block line of sight")

	// Dragon should block line of sight
	blocked = loadedRoom.IsLineOfSightBlocked(Position{X: 0, Y: 10}, Position{X: 20, Y: 10})
	s.True(blocked, "Dragon should block line of sight")
}
