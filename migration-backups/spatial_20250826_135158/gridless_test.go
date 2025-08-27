package spatial_test

import (
	"math"
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/KirkDiggler/rpg-toolkit/tools/spatial"
)

type GridlessTestSuite struct {
	suite.Suite
	room *spatial.GridlessRoom
}

// SetupTest runs before EACH test function
func (s *GridlessTestSuite) SetupTest() {
	s.room = spatial.NewGridlessRoom(spatial.GridlessConfig{
		Width:  100.0,
		Height: 100.0,
	})
}

// TestGridlessRoomCreation tests basic gridless room creation
func (s *GridlessTestSuite) TestGridlessRoomCreation() {
	s.Require().NotNil(s.room)
	s.Assert().Equal(spatial.GridShapeGridless, s.room.GetShape())

	dimensions := s.room.GetDimensions()
	s.Assert().Equal(100.0, dimensions.Width)
	s.Assert().Equal(100.0, dimensions.Height)
}

// TestIsValidPosition tests position validation
func (s *GridlessTestSuite) TestIsValidPosition() {
	testCases := []struct {
		name     string
		position spatial.Position
		expected bool
	}{
		{"origin", spatial.Position{X: 0, Y: 0}, true},
		{"center", spatial.Position{X: 50, Y: 50}, true},
		{"corner", spatial.Position{X: 100, Y: 100}, true},
		{"decimal position", spatial.Position{X: 25.5, Y: 75.7}, true},
		{"negative x", spatial.Position{X: -1, Y: 50}, false},
		{"negative y", spatial.Position{X: 50, Y: -1}, false},
		{"x too large", spatial.Position{X: 101, Y: 50}, false},
		{"y too large", spatial.Position{X: 50, Y: 101}, false},
		{"both too large", spatial.Position{X: 101, Y: 101}, false},
	}

	for _, tc := range testCases {
		s.Run(tc.name, func() {
			result := s.room.IsValidPosition(tc.position)
			s.Assert().Equal(tc.expected, result)
		})
	}
}

// TestEuclideanDistance tests Euclidean distance calculations
func (s *GridlessTestSuite) TestEuclideanDistance() {
	testCases := []struct {
		name     string
		from     spatial.Position
		to       spatial.Position
		expected float64
	}{
		{"same position", spatial.Position{X: 50, Y: 50}, spatial.Position{X: 50, Y: 50}, 0},
		{"horizontal movement", spatial.Position{X: 50, Y: 50}, spatial.Position{X: 53, Y: 50}, 3},
		{"vertical movement", spatial.Position{X: 50, Y: 50}, spatial.Position{X: 50, Y: 54}, 4},
		{"diagonal movement", spatial.Position{X: 50, Y: 50}, spatial.Position{X: 53, Y: 54}, 5}, // 3-4-5 triangle
		{"pythagorean triple", spatial.Position{X: 0, Y: 0}, spatial.Position{X: 5, Y: 12}, 13},  // 5-12-13 triangle
		{"decimal positions", spatial.Position{X: 10.5, Y: 20.5}, spatial.Position{X: 11.5, Y: 21.5}, math.Sqrt(2)},
	}

	for _, tc := range testCases {
		s.Run(tc.name, func() {
			result := s.room.Distance(tc.from, tc.to)
			s.Assert().InDelta(tc.expected, result, 0.0001) // Allow small floating point error
		})
	}
}

// TestGetNeighbors tests getting neighbor positions
func (s *GridlessTestSuite) TestGetNeighbors() {
	s.Run("center position", func() {
		neighbors := s.room.GetNeighbors(spatial.Position{X: 50, Y: 50})
		s.Assert().Len(neighbors, 8) // 8 directions at 45-degree intervals

		// All neighbors should be distance 1 away
		center := spatial.Position{X: 50, Y: 50}
		for _, neighbor := range neighbors {
			s.Assert().InDelta(1.0, s.room.Distance(center, neighbor), 0.0001)
		}
	})

	s.Run("corner position", func() {
		neighbors := s.room.GetNeighbors(spatial.Position{X: 0, Y: 0})
		s.Assert().True(len(neighbors) <= 8) // Some neighbors will be out of bounds

		// All returned neighbors should be valid and distance 1
		origin := spatial.Position{X: 0, Y: 0}
		for _, neighbor := range neighbors {
			s.Assert().True(s.room.IsValidPosition(neighbor))
			s.Assert().InDelta(1.0, s.room.Distance(origin, neighbor), 0.0001)
		}
	})

	s.Run("edge position", func() {
		neighbors := s.room.GetNeighbors(spatial.Position{X: 0, Y: 50})
		s.Assert().True(len(neighbors) <= 8) // Some neighbors will be out of bounds

		// All returned neighbors should be valid and distance 1
		edge := spatial.Position{X: 0, Y: 50}
		for _, neighbor := range neighbors {
			s.Assert().True(s.room.IsValidPosition(neighbor))
			s.Assert().InDelta(1.0, s.room.Distance(edge, neighbor), 0.0001)
		}
	})
}

// TestIsAdjacent tests adjacency checking
func (s *GridlessTestSuite) TestIsAdjacent() {
	center := spatial.Position{X: 50, Y: 50}

	testCases := []struct {
		name     string
		position spatial.Position
		expected bool
	}{
		{"same position", spatial.Position{X: 50, Y: 50}, true},
		{"exactly 1 unit away", spatial.Position{X: 51, Y: 50}, true},
		{"diagonal 1 unit", spatial.Position{X: 50.7, Y: 50.7}, true}, // sqrt(0.7^2 + 0.7^2) ≈ 0.99
		{"just over 1 unit", spatial.Position{X: 51.1, Y: 50}, false},
		{"far away", spatial.Position{X: 55, Y: 55}, false},
	}

	for _, tc := range testCases {
		s.Run(tc.name, func() {
			result := s.room.IsAdjacent(center, tc.position)
			s.Assert().Equal(tc.expected, result)
		})
	}
}

// TestGetLineOfSight tests line of sight calculations
func (s *GridlessTestSuite) TestGetLineOfSight() {
	s.Run("same position", func() {
		los := s.room.GetLineOfSight(spatial.Position{X: 50, Y: 50}, spatial.Position{X: 50, Y: 50})
		s.Assert().Len(los, 1)
		s.Assert().Equal(spatial.Position{X: 50, Y: 50}, los[0])
	})

	s.Run("horizontal line", func() {
		los := s.room.GetLineOfSight(spatial.Position{X: 20, Y: 50}, spatial.Position{X: 25, Y: 50})
		s.Assert().True(len(los) >= 2) // Should have at least start and end
		s.Assert().Contains(los, spatial.Position{X: 20, Y: 50})
		s.Assert().Contains(los, spatial.Position{X: 25, Y: 50})
	})

	s.Run("diagonal line", func() {
		los := s.room.GetLineOfSight(spatial.Position{X: 20, Y: 20}, spatial.Position{X: 25, Y: 25})
		s.Assert().True(len(los) >= 2) // Should have at least start and end
		s.Assert().Contains(los, spatial.Position{X: 20, Y: 20})
		s.Assert().Contains(los, spatial.Position{X: 25, Y: 25})
	})
}

// TestGetPositionsInRange tests range queries
func (s *GridlessTestSuite) TestGetPositionsInRange() {
	center := spatial.Position{X: 50, Y: 50}

	s.Run("range 0", func() {
		positions := s.room.GetPositionsInRange(center, 0)
		s.Assert().Len(positions, 1)
		s.Assert().Contains(positions, center)
	})

	s.Run("range 1", func() {
		positions := s.room.GetPositionsInRange(center, 1)
		s.Assert().True(len(positions) > 1) // Should have multiple positions
		s.Assert().Contains(positions, center)

		// All positions should be within range 1
		for _, pos := range positions {
			s.Assert().True(s.room.Distance(center, pos) <= 1.0)
		}
	})

	s.Run("range 5", func() {
		positions := s.room.GetPositionsInRange(center, 5)
		s.Assert().True(len(positions) > 1) // Should have many positions
		s.Assert().Contains(positions, center)

		// All positions should be within range 5
		for _, pos := range positions {
			s.Assert().True(s.room.Distance(center, pos) <= 5.0)
		}
	})

	s.Run("range at edge", func() {
		edge := spatial.Position{X: 0, Y: 0}
		positions := s.room.GetPositionsInRange(edge, 5)
		s.Assert().True(len(positions) > 0)

		// All returned positions should be valid and within range
		for _, pos := range positions {
			s.Assert().True(s.room.IsValidPosition(pos))
			s.Assert().True(s.room.Distance(edge, pos) <= 5.0)
		}
	})
}

// TestGetPositionsInCircle tests circular area queries
func (s *GridlessTestSuite) TestGetPositionsInCircle() {
	circle := spatial.Circle{
		Center: spatial.Position{X: 50, Y: 50},
		Radius: 5,
	}

	positions := s.room.GetPositionsInCircle(circle)
	s.Assert().True(len(positions) > 0)

	// Check center is included
	s.Assert().Contains(positions, spatial.Position{X: 50, Y: 50})

	// All positions should be within the circle
	for _, pos := range positions {
		s.Assert().True(s.room.Distance(circle.Center, pos) <= circle.Radius)
	}
}

// TestGetPositionsInRectangle tests rectangular area queries
func (s *GridlessTestSuite) TestGetPositionsInRectangle() {
	rect := spatial.Rectangle{
		Position:   spatial.Position{X: 20, Y: 20},
		Dimensions: spatial.Dimensions{Width: 10, Height: 10},
	}

	positions := s.room.GetPositionsInRectangle(rect)
	s.Assert().True(len(positions) > 0)

	// All positions should be within the rectangle
	for _, pos := range positions {
		s.Assert().True(rect.Contains(pos))
	}
}

// TestGetPositionsInArc tests arc queries (gridless-specific)
func (s *GridlessTestSuite) TestGetPositionsInArc() {
	center := spatial.Position{X: 50, Y: 50}
	radius := 10.0
	startAngle := 0.0       // 0 radians (east)
	endAngle := math.Pi / 2 // π/2 radians (north)

	positions := s.room.GetPositionsInArc(center, radius, startAngle, endAngle)
	s.Assert().True(len(positions) > 0)

	// All positions should be within the arc
	for _, pos := range positions {
		distance := s.room.Distance(center, pos)
		s.Assert().True(distance <= radius)

		// Check angle is within arc (for non-center positions)
		if !pos.Equals(center) {
			angle := math.Atan2(pos.Y-center.Y, pos.X-center.X)
			if angle < 0 {
				angle += 2 * math.Pi
			}
			s.Assert().True(angle >= startAngle && angle <= endAngle)
		}
	}
}

// TestGetNearestPosition tests position snapping
func (s *GridlessTestSuite) TestGetNearestPosition() {
	testCases := []struct {
		name     string
		position spatial.Position
		expected spatial.Position
	}{
		{"valid position", spatial.Position{X: 50, Y: 50}, spatial.Position{X: 50, Y: 50}},
		{"negative x", spatial.Position{X: -5, Y: 50}, spatial.Position{X: 0, Y: 50}},
		{"negative y", spatial.Position{X: 50, Y: -5}, spatial.Position{X: 50, Y: 0}},
		{"x too large", spatial.Position{X: 105, Y: 50}, spatial.Position{X: 100, Y: 50}},
		{"y too large", spatial.Position{X: 50, Y: 105}, spatial.Position{X: 50, Y: 100}},
		{"both out of bounds", spatial.Position{X: -5, Y: 105}, spatial.Position{X: 0, Y: 100}},
	}

	for _, tc := range testCases {
		s.Run(tc.name, func() {
			result := s.room.GetNearestPosition(tc.position)
			s.Assert().Equal(tc.expected, result)
		})
	}
}

// TestSmallGridlessRoom tests gridless room with small dimensions
func (s *GridlessTestSuite) TestSmallGridlessRoom() {
	smallRoom := spatial.NewGridlessRoom(spatial.GridlessConfig{
		Width:  5.0,
		Height: 5.0,
	})

	s.Assert().Equal(spatial.GridShapeGridless, smallRoom.GetShape())

	// Test positions are valid
	s.Assert().True(smallRoom.IsValidPosition(spatial.Position{X: 0, Y: 0}))
	s.Assert().True(smallRoom.IsValidPosition(spatial.Position{X: 2.5, Y: 2.5}))
	s.Assert().True(smallRoom.IsValidPosition(spatial.Position{X: 5, Y: 5}))
	s.Assert().False(smallRoom.IsValidPosition(spatial.Position{X: 6, Y: 0}))

	// Test neighbors
	neighbors := smallRoom.GetNeighbors(spatial.Position{X: 2.5, Y: 2.5})
	s.Assert().True(len(neighbors) <= 8)

	// All neighbors should be valid
	for _, neighbor := range neighbors {
		s.Assert().True(smallRoom.IsValidPosition(neighbor))
	}
}

// Run the test suite
func TestGridlessRoomSuite(t *testing.T) {
	suite.Run(t, new(GridlessTestSuite))
}
