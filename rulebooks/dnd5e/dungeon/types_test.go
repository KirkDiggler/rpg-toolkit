package dungeon

import (
	"testing"

	"github.com/stretchr/testify/suite"
)

type TypesTestSuite struct {
	suite.Suite
}

func TestTypesSuite(t *testing.T) {
	suite.Run(t, new(TypesTestSuite))
}

func (s *TypesTestSuite) TestDirectionOpposite() {
	s.Run("returns opposite for all cardinal directions", func() {
		testCases := []struct {
			direction Direction
			expected  Direction
		}{
			{DirectionNorth, DirectionSouth},
			{DirectionSouth, DirectionNorth},
			{DirectionEast, DirectionWest},
			{DirectionWest, DirectionEast},
			{DirectionUp, DirectionDown},
			{DirectionDown, DirectionUp},
		}

		for _, tc := range testCases {
			s.Assert().Equal(tc.expected, tc.direction.Opposite(),
				"Opposite of %s should be %s", tc.direction, tc.expected)
		}
	})

	s.Run("returns empty string for invalid direction", func() {
		invalid := Direction("invalid")
		s.Assert().Equal(Direction(""), invalid.Opposite())
	})

	s.Run("returns empty string for empty direction", func() {
		empty := Direction("")
		s.Assert().Equal(Direction(""), empty.Opposite())
	})

	s.Run("opposite of opposite returns original", func() {
		directions := []Direction{
			DirectionNorth,
			DirectionSouth,
			DirectionEast,
			DirectionWest,
			DirectionUp,
			DirectionDown,
		}

		for _, dir := range directions {
			s.Assert().Equal(dir, dir.Opposite().Opposite(),
				"Opposite of opposite of %s should be %s", dir, dir)
		}
	})
}
