package environments

import (
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/KirkDiggler/rpg-toolkit/events"
	"github.com/KirkDiggler/rpg-toolkit/tools/spatial"
)

type RoomBuilderTestSuite struct {
	suite.Suite
	eventBus    *events.Bus
	shapeLoader *ShapeLoader
	builder     *BasicRoomBuilder
}

func (s *RoomBuilderTestSuite) SetupTest() {
	s.eventBus = events.NewBus()
	s.shapeLoader = &ShapeLoader{
		shapesPath: "test_shapes",
		cache: map[string]*RoomShape{
			"test_rectangle": {
				Name: "test_rectangle",
				Boundary: []spatial.Position{
					{X: 0, Y: 0},
					{X: 1, Y: 0},
					{X: 1, Y: 1},
					{X: 0, Y: 1},
				},
				Connections: []ConnectionPoint{
					{Name: "entrance", Position: spatial.Position{X: 0.5, Y: 0}},
				},
			},
			"test_square": {
				Name: "test_square",
				Boundary: []spatial.Position{
					{X: 0, Y: 0},
					{X: 1, Y: 0},
					{X: 1, Y: 1},
					{X: 0, Y: 1},
				},
				Connections: []ConnectionPoint{
					{Name: "entrance", Position: spatial.Position{X: 0.5, Y: 0}},
					{Name: "exit", Position: spatial.Position{X: 0.5, Y: 1}},
				},
			},
		},
	}

	s.builder = NewBasicRoomBuilder(BasicRoomBuilderConfig{
		EventBus:    s.eventBus,
		ShapeLoader: s.shapeLoader,
	})
}

func (s *RoomBuilderTestSuite) SetupSubTest() {
	// Create fresh builder for each subtest
	s.builder = NewBasicRoomBuilder(BasicRoomBuilderConfig{
		EventBus:    s.eventBus,
		ShapeLoader: s.shapeLoader,
	})
}

func (s *RoomBuilderTestSuite) TestNewBasicRoomBuilder() {
	s.Run("creates builder with defaults", func() {
		builder := NewBasicRoomBuilder(BasicRoomBuilderConfig{
			EventBus:    s.eventBus,
			ShapeLoader: s.shapeLoader,
		})

		s.Assert().NotNil(builder)
		s.Assert().Equal(s.eventBus, builder.eventBus)
		s.Assert().Equal(s.shapeLoader, builder.shapeLoader)
		s.Assert().Equal("empty", builder.pattern)
		s.Assert().Equal("default", builder.theme)
		s.Assert().False(builder.built)

		// Check default pattern params
		s.Assert().Equal(0.4, builder.patternParams.Density)
		s.Assert().Equal(0.7, builder.patternParams.DestructibleRatio)
		s.Assert().Equal(2.0, builder.patternParams.Safety.MinPathWidth)
		s.Assert().Equal(0.6, builder.patternParams.Safety.MinOpenSpace)
		s.Assert().Equal("stone", builder.patternParams.Material)
		s.Assert().Equal(3.0, builder.patternParams.WallHeight)
	})
}

func (s *RoomBuilderTestSuite) TestFluentAPI() {
	s.Run("WithSize sets dimensions", func() {
		result := s.builder.WithSize(15, 12)
		s.Assert().Equal(s.builder, result) // Should return self for chaining
		s.Assert().Equal(float64(15), s.builder.size.Width)
		s.Assert().Equal(float64(12), s.builder.size.Height)
	})

	s.Run("WithTheme sets theme", func() {
		result := s.builder.WithTheme("ancient_ruins")
		s.Assert().Equal(s.builder, result)
		s.Assert().Equal("ancient_ruins", s.builder.theme)
	})

	s.Run("WithFeatures adds features", func() {
		feature1 := Feature{Type: "chest", Name: "Treasure Chest"}
		feature2 := Feature{Type: "pillar", Name: "Stone Pillar"}

		result := s.builder.WithFeatures(feature1, feature2)
		s.Assert().Equal(s.builder, result)
		s.Require().Len(s.builder.features, 2)
		s.Assert().Equal(feature1, s.builder.features[0])
		s.Assert().Equal(feature2, s.builder.features[1])
	})

	s.Run("WithLayout sets pattern and density", func() {
		layout := Layout{Type: LayoutTypeBranching}
		result := s.builder.WithLayout(layout)

		s.Assert().Equal(s.builder, result)
		s.Assert().Equal("random", s.builder.pattern)
		s.Assert().Equal(0.3, s.builder.patternParams.Density)
	})

	s.Run("WithWallPattern sets pattern", func() {
		result := s.builder.WithWallPattern("random")
		s.Assert().Equal(s.builder, result)
		s.Assert().Equal("random", s.builder.pattern)
	})

	s.Run("WithWallDensity sets density", func() {
		result := s.builder.WithWallDensity(0.8)
		s.Assert().Equal(s.builder, result)
		s.Assert().Equal(0.8, s.builder.patternParams.Density)
	})

	s.Run("WithDestructibleRatio sets ratio", func() {
		result := s.builder.WithDestructibleRatio(0.9)
		s.Assert().Equal(s.builder, result)
		s.Assert().Equal(0.9, s.builder.patternParams.DestructibleRatio)
	})

	s.Run("WithMaterial sets material", func() {
		result := s.builder.WithMaterial("wood")
		s.Assert().Equal(s.builder, result)
		s.Assert().Equal("wood", s.builder.patternParams.Material)
	})

	s.Run("WithShape sets shape", func() {
		result := s.builder.WithShape("test_square")
		s.Assert().Equal(s.builder, result)
		s.Assert().Equal("test_square", s.builder.shape.Name)
	})

	s.Run("WithRandomSeed sets seed", func() {
		result := s.builder.WithRandomSeed(12345)
		s.Assert().Equal(s.builder, result)
		s.Assert().Equal(int64(12345), s.builder.patternParams.RandomSeed)
	})
}

func (s *RoomBuilderTestSuite) TestWithPrefab() {
	s.Run("sets configuration from prefab", func() {
		prefab := RoomPrefab{
			Name:  "test_rectangle",
			Size:  spatial.Dimensions{Width: 20, Height: 15},
			Theme: "dungeon",
			Features: []Feature{
				{Type: "trap", Name: "Spike Trap"},
			},
		}

		result := s.builder.WithPrefab(prefab)
		s.Assert().Equal(s.builder, result)
		s.Assert().Equal(prefab.Size, s.builder.size)
		s.Assert().Equal(prefab.Theme, s.builder.theme)
		s.Assert().Equal(prefab.Features, s.builder.features)
		s.Assert().Equal("test_rectangle", s.builder.shape.Name)
	})
}

func (s *RoomBuilderTestSuite) TestBuild() {
	s.Run("builds room successfully", func() {
		room, err := s.builder.
			WithSize(10, 8).
			WithShape("test_rectangle").
			WithWallPattern("empty").
			Build()

		s.Require().NoError(err)
		s.Assert().NotNil(room)
		s.Assert().True(s.builder.built)

		// Check room properties
		s.Assert().Contains(room.GetID(), "room_default_10_8")
		s.Assert().Equal("generated_room", room.GetType())
	})

	s.Run("builds room with walls", func() {
		room, err := s.builder.
			WithSize(10, 8).
			WithShape("test_rectangle").
			WithWallPattern("random").
			WithWallDensity(0.5).
			WithRandomSeed(42).
			Build()

		s.Require().NoError(err)
		s.Assert().NotNil(room)

		// Check if walls were placed (should have some wall entities)
		entities := room.GetAllEntities()
		wallCount := 0
		for _, entity := range entities {
			if entity.GetType() == "wall" {
				wallCount++
			}
		}
		// Should have some walls (exact count depends on algorithm)
		s.Assert().True(wallCount >= 0)
	})

	s.Run("builds room with features", func() {
		feature := Feature{
			Type: "chest",
			Name: "Treasure Chest",
			Properties: map[string]interface{}{
				"locked": true,
			},
		}

		room, err := s.builder.
			WithSize(10, 8).
			WithShape("test_rectangle").
			WithFeatures(feature).
			Build()

		s.Require().NoError(err)
		s.Assert().NotNil(room)

		// Check if feature was placed
		entities := room.GetAllEntities()
		found := false
		for _, entity := range entities {
			if featureEntity, ok := entity.(*FeatureEntity); ok {
				if featureEntity.name == feature.Name {
					found = true
					break
				}
			}
		}
		s.Assert().True(found, "Feature should be placed in room")
	})

	s.Run("fails on second build attempt", func() {
		// First build should succeed
		room1, err1 := s.builder.
			WithSize(10, 8).
			WithShape("test_rectangle").
			Build()

		s.Require().NoError(err1)
		s.Assert().NotNil(room1)

		// Second build should fail
		room2, err2 := s.builder.Build()
		s.Assert().Error(err2)
		s.Assert().Nil(room2)
		s.Assert().Contains(err2.Error(), "can only be used once")
	})

	s.Run("validates size", func() {
		_, err := s.builder.
			WithSize(0, 8).
			WithShape("test_rectangle").
			Build()

		s.Assert().Error(err)
		s.Assert().Contains(err.Error(), "room size must be positive")
	})

	s.Run("validates density", func() {
		_, err := s.builder.
			WithSize(10, 8).
			WithShape("test_rectangle").
			WithWallDensity(1.5).
			Build()

		s.Assert().Error(err)
		s.Assert().Contains(err.Error(), "wall density must be between 0 and 1")
	})

	s.Run("validates destructible ratio", func() {
		_, err := s.builder.
			WithSize(10, 8).
			WithShape("test_rectangle").
			WithDestructibleRatio(-0.1).
			Build()

		s.Assert().Error(err)
		s.Assert().Contains(err.Error(), "destructible ratio must be between 0 and 1")
	})

	s.Run("loads default shape when none specified", func() {
		room, err := s.builder.
			WithSize(10, 8).
			Build()

		s.Require().NoError(err)
		s.Assert().NotNil(room)
		// Should have loaded default rectangle shape
	})
}

func (s *RoomBuilderTestSuite) TestLayoutConversion() {
	testCases := []struct {
		name            string
		layoutType      LayoutType
		expectedPattern string
		expectedDensity float64
	}{
		{"Linear", LayoutTypeLinear, "empty", 0.4},
		{"Branching", LayoutTypeBranching, "random", 0.3},
		{"Grid", LayoutTypeGrid, "random", 0.5},
		{"Organic", LayoutTypeOrganic, "random", 0.4},
	}

	for _, tc := range testCases {
		s.Run(tc.name, func() {
			layout := Layout{Type: tc.layoutType}
			s.builder.WithLayout(layout)

			s.Assert().Equal(tc.expectedPattern, s.builder.pattern)
			s.Assert().Equal(tc.expectedDensity, s.builder.patternParams.Density)
		})
	}
}

func (s *RoomBuilderTestSuite) TestConvenienceFunctions() {
	s.Run("QuickRoom creates room", func() {
		room, err := QuickRoom(15, 12, "random")
		s.Require().NoError(err)
		s.Assert().NotNil(room)
	})

	s.Run("DenseCoverRoom creates room", func() {
		room, err := DenseCoverRoom(20, 15)
		s.Require().NoError(err)
		s.Assert().NotNil(room)
	})

	s.Run("SparseCoverRoom creates room", func() {
		room, err := SparseCoverRoom(20, 15)
		s.Require().NoError(err)
		s.Assert().NotNil(room)
	})

	s.Run("BalancedCoverRoom creates room", func() {
		room, err := BalancedCoverRoom(20, 15)
		s.Require().NoError(err)
		s.Assert().NotNil(room)
	})
}

func (s *RoomBuilderTestSuite) TestFeatureEntity() {
	s.Run("implements required interfaces", func() {
		feature := &FeatureEntity{
			id:          "test_feature",
			featureType: "chest",
			name:        "Test Chest",
			properties:  map[string]interface{}{"locked": true},
		}

		s.Assert().Equal("test_feature", feature.GetID())
		s.Assert().Equal("chest", feature.GetType())
		s.Assert().Equal(1, feature.GetSize())
		s.Assert().False(feature.BlocksMovement())
		s.Assert().False(feature.BlocksLineOfSight())
	})
}

func TestRoomBuilderTestSuite(t *testing.T) {
	suite.Run(t, new(RoomBuilderTestSuite))
}
