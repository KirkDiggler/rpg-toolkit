package environments

import (
	"testing"

	"github.com/stretchr/testify/suite"
)

// RoomBuilderTestSuite tests room builder functionality
type RoomBuilderTestSuite struct {
	suite.Suite
}

func (s *RoomBuilderTestSuite) TestBasicRoomBuilding() {
	s.Run("builds room with valid dimensions", func() {
		config := BasicRoomBuilderConfig{}
		builder := NewBasicRoomBuilder(config)

		room, err := builder.
			WithSize(10, 10).
			WithTheme("dungeon").
			Build()

		s.Assert().NoError(err)
		s.Assert().NotNil(room)
	})

	s.Run("handles different sizes", func() {
		sizes := [][2]int{
			{5, 5},
			{15, 10},
			{20, 15},
		}

		for _, size := range sizes {
			config := BasicRoomBuilderConfig{}
			builder := NewBasicRoomBuilder(config) // Create fresh builder each time
			room, err := builder.
				WithSize(size[0], size[1]).
				WithTheme("test").
				Build()

			s.Assert().NoError(err, "Size %dx%d should build successfully", size[0], size[1])
			s.Assert().NotNil(room)
		}
	})
}

func (s *RoomBuilderTestSuite) TestWallPatterns() {
	s.Run("builds with different wall patterns", func() {
		patterns := []string{"empty", "random"}

		for _, pattern := range patterns {
			config := BasicRoomBuilderConfig{}
			builder := NewBasicRoomBuilder(config) // Create fresh builder each time
			room, err := builder.
				WithSize(10, 10).
				WithWallPattern(pattern).
				WithTheme("test").
				Build()

			s.Assert().NoError(err, "Pattern %s should build successfully", pattern)
			s.Assert().NotNil(room)
		}
	})
}

func TestRoomBuilderSuite(t *testing.T) {
	suite.Run(t, new(RoomBuilderTestSuite))
}
