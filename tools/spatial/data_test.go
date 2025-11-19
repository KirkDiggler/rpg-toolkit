package spatial

import (
	"context"
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/KirkDiggler/rpg-toolkit/core"
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

func (m *MockEntity) GetType() core.EntityType {
	return core.EntityType(m.entityType)
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
	s.eventBus = events.NewEventBus()
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
	})
	room.ConnectToEventBus(s.eventBus)

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
	})
	room.ConnectToEventBus(s.eventBus)

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
	})
	room.ConnectToEventBus(s.eventBus)

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
	s.Equal(core.EntityType("tavern"), room.GetType())

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
	s.Equal(core.EntityType("npc"), bartenderEntity.GetType())

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
	})
	originalRoom.ConnectToEventBus(s.eventBus)

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
	})
	room.ConnectToEventBus(s.eventBus)

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

// TestHexOrientationPersistence tests that hex orientation is properly persisted and loaded
func (s *RoomDataTestSuite) TestHexOrientationPersistence() {
	// Helper function to test hex orientation persistence
	testHexOrientation := func(roomID, roomType string, width, height int, pointyTop bool, label string) {
		// Create hex room with specified orientation
		room := NewBasicRoom(BasicRoomConfig{
			ID:   roomID,
			Type: roomType,
			Grid: NewHexGrid(HexGridConfig{
				Width:     float64(width),
				Height:    float64(height),
				PointyTop: pointyTop,
			}),
		})
		room.ConnectToEventBus(s.eventBus)

		// Convert to data
		data := room.ToData()

		// Verify hex orientation is captured
		s.Equal("hex", data.GridType)
		s.Require().NotNil(data.HexOrientation)
		s.Equal(pointyTop, *data.HexOrientation)

		// Load from data
		gameCtx, err := game.NewContext(s.eventBus, data)
		s.Require().NoError(err)
		loadedRoom, err := LoadRoomFromContext(context.Background(), gameCtx)
		s.Require().NoError(err)

		// Verify loaded grid has correct orientation
		grid := loadedRoom.GetGrid()
		s.Equal(GridShapeHex, grid.GetShape())
		hexGrid, ok := grid.(*HexGrid)
		s.Require().True(ok)
		s.Equal(pointyTop, hexGrid.GetOrientation(), label)
	}

	s.Run("pointy-top hex grid persistence", func() {
		testHexOrientation("pointy-hex", "battlefield", 8, 8, true, "Loaded grid should be pointy-top")
	})

	s.Run("flat-top hex grid persistence", func() {
		testHexOrientation("flat-hex", "campaign", 6, 6, false, "Loaded grid should be flat-top")
	})

	s.Run("legacy hex grid defaults to pointy-top", func() {
		// Create room data without hex orientation (legacy format)
		roomData := RoomData{
			ID:       "legacy-hex",
			Type:     "dungeon",
			Width:    10,
			Height:   10,
			GridType: "hex",
			// HexOrientation is nil (legacy)
		}

		// Load from data
		gameCtx, err := game.NewContext(s.eventBus, roomData)
		s.Require().NoError(err)
		loadedRoom, err := LoadRoomFromContext(context.Background(), gameCtx)
		s.Require().NoError(err)

		// Verify defaults to pointy-top
		grid := loadedRoom.GetGrid()
		s.Equal(GridShapeHex, grid.GetShape())
		hexGrid, ok := grid.(*HexGrid)
		s.Require().True(ok)
		s.True(hexGrid.GetOrientation(), "Legacy hex grid should default to pointy-top for D&D 5e")
	})

	s.Run("non-hex grids don't have orientation", func() {
		// Create square room
		room := NewBasicRoom(BasicRoomConfig{
			ID:   "square-room",
			Type: "chamber",
			Grid: NewSquareGrid(SquareGridConfig{
				Width:  10,
				Height: 10,
			}),
		})
		room.ConnectToEventBus(s.eventBus)

		// Convert to data
		data := room.ToData()

		// Verify no hex orientation for square grid
		s.Equal("square", data.GridType)
		s.Nil(data.HexOrientation)
	})
}

// TestGridTypeFromHexOrientation tests the helper function that converts orientation to grid type
func (s *RoomDataTestSuite) TestGridTypeFromHexOrientation() {
	s.Run("pointy-top returns hex_pointy", func() {
		gridType := GridTypeFromHexOrientation(true)
		s.Equal(GridTypeHexPointy, gridType)
		s.Equal("hex_pointy", gridType)
	})

	s.Run("flat-top returns hex_flat", func() {
		gridType := GridTypeFromHexOrientation(false)
		s.Equal(GridTypeHexFlat, gridType)
		s.Equal("hex_flat", gridType)
	})
}

// TestParseHexGridType tests parsing of hex grid type strings
func (s *RoomDataTestSuite) TestParseHexGridType() {
	s.Run("parse hex_pointy", func() {
		orientation, err := ParseHexGridType(GridTypeHexPointy)
		s.NoError(err)
		s.True(orientation)
	})

	s.Run("parse hex_flat", func() {
		orientation, err := ParseHexGridType(GridTypeHexFlat)
		s.NoError(err)
		s.False(orientation)
	})

	s.Run("parse generic hex defaults to pointy", func() {
		orientation, err := ParseHexGridType(GridTypeHex)
		s.NoError(err)
		s.True(orientation, "Generic 'hex' should default to pointy-top for D&D 5e compatibility")
	})

	s.Run("error on non-hex grid type", func() {
		_, err := ParseHexGridType(GridTypeSquare)
		s.Error(err)
		s.Contains(err.Error(), "not a hex grid type")
	})

	s.Run("error on gridless type", func() {
		_, err := ParseHexGridType(GridTypeGridless)
		s.Error(err)
		s.Contains(err.Error(), "not a hex grid type")
	})

	s.Run("error on invalid type", func() {
		_, err := ParseHexGridType("invalid")
		s.Error(err)
		s.Contains(err.Error(), "not a hex grid type: invalid")
	})
}

// TestLoadRoomWithOrientationSpecificGridTypes tests loading rooms with hex_pointy and hex_flat
func (s *RoomDataTestSuite) TestLoadRoomWithOrientationSpecificGridTypes() {
	s.Run("load room with hex_pointy grid type", func() {
		roomData := RoomData{
			ID:       "pointy-hex-room",
			Type:     "battlefield",
			Width:    8,
			Height:   8,
			GridType: GridTypeHexPointy,
			// Note: HexOrientation field is not set, orientation comes from GridType
		}

		gameCtx, err := game.NewContext(s.eventBus, roomData)
		s.Require().NoError(err)
		room, err := LoadRoomFromContext(context.Background(), gameCtx)
		s.Require().NoError(err)

		// Verify grid is hex with pointy-top orientation
		grid := room.GetGrid()
		s.Equal(GridShapeHex, grid.GetShape())
		hexGrid, ok := grid.(*HexGrid)
		s.Require().True(ok)
		s.True(hexGrid.GetOrientation(), "hex_pointy should create pointy-top grid")
	})

	s.Run("load room with hex_flat grid type", func() {
		roomData := RoomData{
			ID:       "flat-hex-room",
			Type:     "battlefield",
			Width:    8,
			Height:   8,
			GridType: GridTypeHexFlat,
			// Note: HexOrientation field is not set, orientation comes from GridType
		}

		gameCtx, err := game.NewContext(s.eventBus, roomData)
		s.Require().NoError(err)
		room, err := LoadRoomFromContext(context.Background(), gameCtx)
		s.Require().NoError(err)

		// Verify grid is hex with flat-top orientation
		grid := room.GetGrid()
		s.Equal(GridShapeHex, grid.GetShape())
		hexGrid, ok := grid.(*HexGrid)
		s.Require().True(ok)
		s.False(hexGrid.GetOrientation(), "hex_flat should create flat-top grid")
	})

	s.Run("orientation-specific type overrides HexOrientation field", func() {
		// GridType says pointy, but HexOrientation says flat - GridType wins
		flatBool := false
		roomData := RoomData{
			ID:             "conflict-room",
			Type:           "test",
			Width:          6,
			Height:         6,
			GridType:       GridTypeHexPointy, // Says pointy
			HexOrientation: &flatBool,         // Says flat
		}

		gameCtx, err := game.NewContext(s.eventBus, roomData)
		s.Require().NoError(err)
		room, err := LoadRoomFromContext(context.Background(), gameCtx)
		s.Require().NoError(err)

		// GridType should win
		grid := room.GetGrid()
		hexGrid, ok := grid.(*HexGrid)
		s.Require().True(ok)
		s.True(hexGrid.GetOrientation(), "GridType hex_pointy should override HexOrientation field")
	})

	s.Run("generic hex type still uses HexOrientation field", func() {
		flatBool := false
		roomData := RoomData{
			ID:             "generic-hex-room",
			Type:           "test",
			Width:          6,
			Height:         6,
			GridType:       GridTypeHex, // Generic hex
			HexOrientation: &flatBool,   // Specifies flat
		}

		gameCtx, err := game.NewContext(s.eventBus, roomData)
		s.Require().NoError(err)
		room, err := LoadRoomFromContext(context.Background(), gameCtx)
		s.Require().NoError(err)

		// HexOrientation field should be used for generic "hex"
		grid := room.GetGrid()
		hexGrid, ok := grid.(*HexGrid)
		s.Require().True(ok)
		s.False(hexGrid.GetOrientation(), "Generic 'hex' should use HexOrientation field")
	})
}
