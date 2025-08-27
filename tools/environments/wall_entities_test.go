package environments

import (
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/KirkDiggler/rpg-toolkit/tools/spatial"
)

// WallEntitiesTestSuite tests wall segment functionality
type WallEntitiesTestSuite struct {
	suite.Suite
}

func (s *WallEntitiesTestSuite) TestWallSegmentCreation() {
	s.Run("creates wall segment with valid properties", func() {
		segment := WallSegment{
			Start: spatial.Position{X: 0, Y: 0},
			End:   spatial.Position{X: 5, Y: 0},
			Type:  WallTypeDestructible,
			Properties: WallProperties{
				HP:       100,
				Material: "stone",
			},
		}

		s.Assert().Equal(WallTypeDestructible, segment.Type)
		s.Assert().Equal(100, segment.Properties.HP)
		s.Assert().Equal("stone", segment.Properties.Material)
	})
}

func (s *WallEntitiesTestSuite) TestWallTypes() {
	s.Run("wall types are properly defined", func() {
		// Test that wall type constants exist
		s.Assert().Equal(WallType(0), WallTypeIndestructible)
		s.Assert().Equal(WallType(1), WallTypeDestructible)
		s.Assert().Equal(WallType(2), WallTypeTemporary)
		s.Assert().Equal(WallType(3), WallTypeConditional)
	})
}

func (s *WallEntitiesTestSuite) TestWallProperties() {
	s.Run("wall has correct properties", func() {
		properties := WallProperties{
			HP:             50,
			Material:       "wood",
			BlocksLoS:      true,
			BlocksMovement: true,
			ProvidesCover:  true,
		}

		segment := WallSegment{
			Start:      spatial.Position{X: 1, Y: 1},
			End:        spatial.Position{X: 6, Y: 1},
			Type:       WallTypeDestructible,
			Properties: properties,
		}

		s.Assert().Equal("wood", segment.Properties.Material)
		s.Assert().Equal(50, segment.Properties.HP)
		s.Assert().True(segment.Properties.BlocksLoS)
		s.Assert().True(segment.Properties.BlocksMovement)
		s.Assert().True(segment.Properties.ProvidesCover)
	})
}

func TestWallEntitiesSuite(t *testing.T) {
	suite.Run(t, new(WallEntitiesTestSuite))
}
