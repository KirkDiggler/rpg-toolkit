package environments

import (
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/KirkDiggler/rpg-toolkit/events"
	"github.com/KirkDiggler/rpg-toolkit/tools/spatial"
)

type WallEntitiesTestSuite struct {
	suite.Suite
	eventBus *events.Bus
	room     spatial.Room
}

func (s *WallEntitiesTestSuite) SetupTest() {
	s.eventBus = events.NewBus()

	// Create a basic room for testing
	gridConfig := spatial.SquareGridConfig{
		Width:  20,
		Height: 15,
	}
	grid := spatial.NewSquareGrid(gridConfig)

	roomConfig := spatial.BasicRoomConfig{
		ID:       "test_room",
		Type:     "test",
		Grid:     grid,
		EventBus: s.eventBus,
	}
	s.room = spatial.NewBasicRoom(roomConfig)
}

func (s *WallEntitiesTestSuite) SetupSubTest() {
	// Reset room state for each subtest
	s.SetupTest()
}

func (s *WallEntitiesTestSuite) TestNewWallEntity() {
	s.Run("creates wall entity with proper defaults", func() {
		config := WallEntityConfig{
			SegmentID: "wall_segment_1",
			WallType:  WallTypeDestructible,
			Properties: WallProperties{
				HP:             20,
				Material:       "stone",
				Thickness:      0.5,
				Height:         3.0,
				BlocksLoS:      true,
				BlocksMovement: true,
				ProvidesCover:  true,
			},
			Position: spatial.Position{X: 5, Y: 5},
		}

		entity := NewWallEntity(config)

		s.Assert().NotNil(entity)
		s.Assert().Equal("wall_segment_1", entity.segmentID)
		s.Assert().Equal(WallTypeDestructible, entity.wallType)
		s.Assert().Equal(config.Properties, entity.properties)
		s.Assert().Equal(config.Position, entity.position)
		s.Assert().Equal(20, entity.currentHP)
		s.Assert().False(entity.destroyed)
		s.Assert().Contains(entity.id, "wall_segment_1_5_5")
	})

	s.Run("sets default HP when not specified", func() {
		config := WallEntityConfig{
			SegmentID: "wall_segment_2",
			WallType:  WallTypeDestructible,
			Properties: WallProperties{
				HP: 0, // No HP specified
			},
			Position: spatial.Position{X: 3, Y: 3},
		}

		entity := NewWallEntity(config)

		s.Assert().Equal(10, entity.currentHP) // Default HP
	})
}

func (s *WallEntitiesTestSuite) TestCoreEntityInterface() {
	s.Run("implements core entity interface", func() {
		entity := NewWallEntity(WallEntityConfig{
			SegmentID: "test_segment",
			WallType:  WallTypeDestructible,
			Properties: WallProperties{
				HP:             15,
				BlocksMovement: true,
			},
			Position: spatial.Position{X: 2, Y: 2},
		})

		s.Assert().Contains(entity.GetID(), "test_segment")
		s.Assert().Equal("wall", entity.GetType())
	})
}

func (s *WallEntitiesTestSuite) TestPlaceableInterface() {
	s.Run("implements placeable interface", func() {
		entity := NewWallEntity(WallEntityConfig{
			SegmentID: "test_segment",
			WallType:  WallTypeDestructible,
			Properties: WallProperties{
				HP:             15,
				BlocksMovement: true,
				BlocksLoS:      true,
			},
			Position: spatial.Position{X: 2, Y: 2},
		})

		s.Assert().Equal(1, entity.GetSize())
		s.Assert().True(entity.BlocksMovement())
		s.Assert().True(entity.BlocksLineOfSight())
	})

	s.Run("destroyed walls don't block", func() {
		entity := NewWallEntity(WallEntityConfig{
			SegmentID: "test_segment",
			WallType:  WallTypeDestructible,
			Properties: WallProperties{
				HP:             15,
				BlocksMovement: true,
				BlocksLoS:      true,
			},
			Position: spatial.Position{X: 2, Y: 2},
		})

		// Initially should block
		s.Assert().True(entity.BlocksMovement())
		s.Assert().True(entity.BlocksLineOfSight())

		// Destroy the wall
		entity.Destroy()

		// Should no longer block
		s.Assert().False(entity.BlocksMovement())
		s.Assert().False(entity.BlocksLineOfSight())
	})
}

func (s *WallEntitiesTestSuite) TestWallDamage() {
	s.Run("takes damage correctly", func() {
		entity := NewWallEntity(WallEntityConfig{
			SegmentID: "test_segment",
			WallType:  WallTypeDestructible,
			Properties: WallProperties{
				HP:             20,
				BlocksMovement: true,
			},
			Position: spatial.Position{X: 2, Y: 2},
		})

		// Take some damage
		destroyed := entity.TakeDamage(5, "physical")
		s.Assert().False(destroyed)
		s.Assert().Equal(15, entity.currentHP)
		s.Assert().False(entity.destroyed)

		// Take enough damage to destroy
		destroyed = entity.TakeDamage(15, "physical")
		s.Assert().True(destroyed)
		s.Assert().Equal(0, entity.currentHP)
		s.Assert().True(entity.destroyed)
	})

	s.Run("indestructible walls don't take damage", func() {
		entity := NewWallEntity(WallEntityConfig{
			SegmentID: "test_segment",
			WallType:  WallTypeIndestructible,
			Properties: WallProperties{
				HP:             20,
				BlocksMovement: true,
			},
			Position: spatial.Position{X: 2, Y: 2},
		})

		destroyed := entity.TakeDamage(100, "physical")
		s.Assert().False(destroyed)
		s.Assert().Equal(20, entity.currentHP)
		s.Assert().False(entity.destroyed)
	})

	s.Run("already destroyed walls don't take damage", func() {
		entity := NewWallEntity(WallEntityConfig{
			SegmentID: "test_segment",
			WallType:  WallTypeDestructible,
			Properties: WallProperties{
				HP:             20,
				BlocksMovement: true,
			},
			Position: spatial.Position{X: 2, Y: 2},
		})

		// Destroy the wall
		entity.Destroy()

		// Try to damage destroyed wall
		destroyed := entity.TakeDamage(10, "physical")
		s.Assert().False(destroyed)
		s.Assert().Equal(0, entity.currentHP)
	})

	s.Run("applies resistances and weaknesses", func() {
		entity := NewWallEntity(WallEntityConfig{
			SegmentID: "test_segment",
			WallType:  WallTypeDestructible,
			Properties: WallProperties{
				HP:             20,
				Resistance:     []string{"fire"},
				Weakness:       []string{"ice"},
				BlocksMovement: true,
			},
			Position: spatial.Position{X: 2, Y: 2},
		})

		// Test resistance (50% damage)
		entity.TakeDamage(10, "fire")
		s.Assert().Equal(15, entity.currentHP) // 20 - 5 = 15

		// Reset HP
		entity.currentHP = 20

		// Test weakness (200% damage)
		entity.TakeDamage(5, "ice")
		s.Assert().Equal(10, entity.currentHP) // 20 - 10 = 10

		// Reset HP
		entity.currentHP = 20

		// Test normal damage
		entity.TakeDamage(6, "physical")
		s.Assert().Equal(14, entity.currentHP) // 20 - 6 = 14
	})

	s.Run("minimum 1 damage", func() {
		entity := NewWallEntity(WallEntityConfig{
			SegmentID: "test_segment",
			WallType:  WallTypeDestructible,
			Properties: WallProperties{
				HP:             20,
				Resistance:     []string{"fire"},
				BlocksMovement: true,
			},
			Position: spatial.Position{X: 2, Y: 2},
		})

		// Even with resistance, should take at least 1 damage
		entity.TakeDamage(1, "fire")
		s.Assert().Equal(19, entity.currentHP) // 20 - 1 = 19
	})
}

func (s *WallEntitiesTestSuite) TestWallRepair() {
	s.Run("repairs wall correctly", func() {
		entity := NewWallEntity(WallEntityConfig{
			SegmentID: "test_segment",
			WallType:  WallTypeDestructible,
			Properties: WallProperties{
				HP:             20,
				BlocksMovement: true,
			},
			Position: spatial.Position{X: 2, Y: 2},
		})

		// Damage the wall
		entity.TakeDamage(10, "physical")
		s.Assert().Equal(10, entity.currentHP)

		// Repair the wall
		entity.Repair(5)
		s.Assert().Equal(15, entity.currentHP)

		// Can't repair beyond max HP
		entity.Repair(10)
		s.Assert().Equal(20, entity.currentHP)
	})

	s.Run("can't repair destroyed walls", func() {
		entity := NewWallEntity(WallEntityConfig{
			SegmentID: "test_segment",
			WallType:  WallTypeDestructible,
			Properties: WallProperties{
				HP:             20,
				BlocksMovement: true,
			},
			Position: spatial.Position{X: 2, Y: 2},
		})

		// Destroy the wall
		entity.Destroy()

		// Try to repair
		entity.Repair(10)
		s.Assert().Equal(0, entity.currentHP)
		s.Assert().True(entity.destroyed)
	})
}

func (s *WallEntitiesTestSuite) TestWallSegmentConversion() {
	s.Run("creates wall entities from segments", func() {
		segments := []WallSegment{
			{
				Start: spatial.Position{X: 0, Y: 0},
				End:   spatial.Position{X: 3, Y: 0},
				Type:  WallTypeDestructible,
				Properties: WallProperties{
					HP:             15,
					Material:       "stone",
					Thickness:      0.5,
					BlocksMovement: true,
				},
			},
			{
				Start: spatial.Position{X: 5, Y: 2},
				End:   spatial.Position{X: 5, Y: 5},
				Type:  WallTypeIndestructible,
				Properties: WallProperties{
					Material:       "metal",
					Thickness:      1.0,
					BlocksMovement: true,
				},
			},
		}

		entities := CreateWallEntities(segments)

		s.Assert().NotEmpty(entities)

		// All entities should be wall entities
		for _, entity := range entities {
			wallEntity, ok := entity.(*WallEntity)
			s.Assert().True(ok)
			s.Assert().Equal("wall", wallEntity.GetType())
		}
	})

	s.Run("discretizes wall segments correctly", func() {
		segment := WallSegment{
			Start: spatial.Position{X: 0, Y: 0},
			End:   spatial.Position{X: 4, Y: 0},
			Type:  WallTypeDestructible,
			Properties: WallProperties{
				HP:             15,
				Thickness:      0.5,
				BlocksMovement: true,
			},
		}

		entities := discretizeWallSegment(segment)

		s.Assert().NotEmpty(entities)
		s.Assert().GreaterOrEqual(len(entities), 1)

		// All entities should have the same properties
		for _, entity := range entities {
			s.Assert().Equal(segment.Type, entity.wallType)
			s.Assert().Equal(segment.Properties, entity.properties)
		}
	})
}

func (s *WallEntitiesTestSuite) TestSpatialIntegration() {
	s.Run("wall entities integrate with spatial room", func() {
		// Create a wall entity
		entity := NewWallEntity(WallEntityConfig{
			SegmentID: "test_segment",
			WallType:  WallTypeDestructible,
			Properties: WallProperties{
				HP:             20,
				BlocksMovement: true,
				BlocksLoS:      true,
			},
			Position: spatial.Position{X: 5, Y: 5},
		})

		// Place wall entity in room
		err := s.room.PlaceEntity(entity, entity.GetPosition())
		s.Require().NoError(err)

		// Check that wall is in room
		entitiesAtPos := s.room.GetEntitiesAt(entity.GetPosition())
		s.Assert().Len(entitiesAtPos, 1)
		s.Assert().Equal(entity.GetID(), entitiesAtPos[0].GetID())

		// Check that wall blocks movement - try to place another entity at the same position
		anotherEntity := NewWallEntity(WallEntityConfig{
			SegmentID: "test_segment_2",
			WallType:  WallTypeDestructible,
			Properties: WallProperties{
				HP:             20,
				BlocksMovement: true,
				BlocksLoS:      true,
			},
			Position: spatial.Position{X: 5, Y: 5},
		})
		canPlace := s.room.CanPlaceEntity(anotherEntity, entity.GetPosition())
		s.Assert().False(canPlace) // Should be blocked by the existing wall

		// Check line of sight blocking
		fromPos := spatial.Position{X: 4, Y: 5}
		toPos := spatial.Position{X: 6, Y: 5}
		blocked := s.room.IsLineOfSightBlocked(fromPos, toPos)
		s.Assert().True(blocked) // Should be blocked by the wall
	})

	s.Run("destroyed walls don't block in spatial room", func() {
		// Create a wall entity
		entity := NewWallEntity(WallEntityConfig{
			SegmentID: "test_segment",
			WallType:  WallTypeDestructible,
			Properties: WallProperties{
				HP:             20,
				BlocksMovement: true,
				BlocksLoS:      true,
			},
			Position: spatial.Position{X: 5, Y: 5},
		})

		// Place wall entity in room
		err := s.room.PlaceEntity(entity, entity.GetPosition())
		s.Require().NoError(err)

		// Initially should block
		blocked := s.room.IsLineOfSightBlocked(
			spatial.Position{X: 4, Y: 5},
			spatial.Position{X: 6, Y: 5},
		)
		s.Assert().True(blocked)

		// Destroy the wall
		entity.Destroy()

		// Should no longer block
		blocked = s.room.IsLineOfSightBlocked(
			spatial.Position{X: 4, Y: 5},
			spatial.Position{X: 6, Y: 5},
		)
		s.Assert().False(blocked)
	})
}

func (s *WallEntitiesTestSuite) TestWallManagement() {
	s.Run("finds wall entities by segment", func() {
		// Create multiple wall entities from same segment
		entities := []spatial.Placeable{
			NewWallEntity(WallEntityConfig{
				SegmentID:  "segment_1",
				WallType:   WallTypeDestructible,
				Properties: WallProperties{HP: 10, BlocksMovement: true},
				Position:   spatial.Position{X: 1, Y: 1},
			}),
			NewWallEntity(WallEntityConfig{
				SegmentID:  "segment_1",
				WallType:   WallTypeDestructible,
				Properties: WallProperties{HP: 10, BlocksMovement: true},
				Position:   spatial.Position{X: 2, Y: 1},
			}),
			NewWallEntity(WallEntityConfig{
				SegmentID:  "segment_2",
				WallType:   WallTypeDestructible,
				Properties: WallProperties{HP: 10, BlocksMovement: true},
				Position:   spatial.Position{X: 3, Y: 1},
			}),
		}

		wallEntities := FindWallEntitiesBySegment(entities, "segment_1")
		s.Assert().Len(wallEntities, 2)

		for _, entity := range wallEntities {
			s.Assert().Equal("segment_1", entity.GetSegmentID())
		}
	})

	s.Run("gets wall entities in room", func() {
		// Place some wall entities in room
		wall1 := NewWallEntity(WallEntityConfig{
			SegmentID:  "segment_1",
			WallType:   WallTypeDestructible,
			Properties: WallProperties{HP: 10, BlocksMovement: true},
			Position:   spatial.Position{X: 1, Y: 1},
		})

		wall2 := NewWallEntity(WallEntityConfig{
			SegmentID:  "segment_2",
			WallType:   WallTypeDestructible,
			Properties: WallProperties{HP: 10, BlocksMovement: true},
			Position:   spatial.Position{X: 2, Y: 2},
		})

		err := s.room.PlaceEntity(wall1, wall1.GetPosition())
		s.Require().NoError(err)
		err = s.room.PlaceEntity(wall2, wall2.GetPosition())
		s.Require().NoError(err)

		wallEntities := GetWallEntitiesInRoom(s.room)
		s.Assert().Len(wallEntities, 2)
	})

	s.Run("gets wall segment health", func() {
		// Create multiple wall entities for same segment
		wall1 := NewWallEntity(WallEntityConfig{
			SegmentID:  "segment_1",
			WallType:   WallTypeDestructible,
			Properties: WallProperties{HP: 20, BlocksMovement: true},
			Position:   spatial.Position{X: 1, Y: 1},
		})

		wall2 := NewWallEntity(WallEntityConfig{
			SegmentID:  "segment_1",
			WallType:   WallTypeDestructible,
			Properties: WallProperties{HP: 20, BlocksMovement: true},
			Position:   spatial.Position{X: 2, Y: 1},
		})

		err := s.room.PlaceEntity(wall1, wall1.GetPosition())
		s.Require().NoError(err)
		err = s.room.PlaceEntity(wall2, wall2.GetPosition())
		s.Require().NoError(err)

		// Damage one wall
		wall1.TakeDamage(5, "physical")

		current, maximum, destroyed := GetWallSegmentHealth(s.room, "segment_1")
		s.Assert().Equal(35, current) // 15 + 20
		s.Assert().Equal(40, maximum) // 20 + 20
		s.Assert().False(destroyed)

		// Destroy both walls
		wall1.Destroy()
		wall2.Destroy()

		current, maximum, destroyed = GetWallSegmentHealth(s.room, "segment_1")
		s.Assert().Equal(0, current)
		s.Assert().Equal(40, maximum)
		s.Assert().True(destroyed)
	})
}

func TestWallEntitiesTestSuite(t *testing.T) {
	suite.Run(t, new(WallEntitiesTestSuite))
}
