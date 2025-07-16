package spatial_test

import (
	"testing"

	"github.com/stretchr/testify/suite"
	"github.com/KirkDiggler/rpg-toolkit/tools/spatial"
)

type SquareGridTestSuite struct {
	suite.Suite
	grid *spatial.SquareGrid
}

// SetupTest runs before EACH test function
func (s *SquareGridTestSuite) SetupTest() {
	s.grid = spatial.NewSquareGrid(spatial.SquareGridConfig{
		Width:  10,
		Height: 10,
	})
}

// TestSquareGridCreation tests basic grid creation
func (s *SquareGridTestSuite) TestSquareGridCreation() {
	s.Require().NotNil(s.grid)
	s.Assert().Equal(spatial.GridShapeSquare, s.grid.GetShape())
	
	dimensions := s.grid.GetDimensions()
	s.Assert().Equal(10.0, dimensions.Width)
	s.Assert().Equal(10.0, dimensions.Height)
}

// TestIsValidPosition tests position validation
func (s *SquareGridTestSuite) TestIsValidPosition() {
	testCases := []struct {
		name     string
		position spatial.Position
		expected bool
	}{
		{"origin", spatial.Position{X: 0, Y: 0}, true},
		{"center", spatial.Position{X: 5, Y: 5}, true},
		{"top-right corner", spatial.Position{X: 9, Y: 9}, true},
		{"negative x", spatial.Position{X: -1, Y: 5}, false},
		{"negative y", spatial.Position{X: 5, Y: -1}, false},
		{"x too large", spatial.Position{X: 10, Y: 5}, false},
		{"y too large", spatial.Position{X: 5, Y: 10}, false},
		{"both too large", spatial.Position{X: 10, Y: 10}, false},
	}
	
	for _, tc := range testCases {
		s.Run(tc.name, func() {
			result := s.grid.IsValidPosition(tc.position)
			s.Assert().Equal(tc.expected, result)
		})
	}
}

// TestDND5eDistance tests D&D 5e distance calculations (Chebyshev distance)
func (s *SquareGridTestSuite) TestDND5eDistance() {
	testCases := []struct {
		name     string
		from     spatial.Position
		to       spatial.Position
		expected float64
	}{
		{"same position", spatial.Position{X: 5, Y: 5}, spatial.Position{X: 5, Y: 5}, 0},
		{"orthogonal 1", spatial.Position{X: 5, Y: 5}, spatial.Position{X: 6, Y: 5}, 1},
		{"orthogonal 3", spatial.Position{X: 5, Y: 5}, spatial.Position{X: 8, Y: 5}, 3},
		{"diagonal 1", spatial.Position{X: 5, Y: 5}, spatial.Position{X: 6, Y: 6}, 1},
		{"diagonal 3", spatial.Position{X: 5, Y: 5}, spatial.Position{X: 8, Y: 8}, 3},
		{"mixed diagonal", spatial.Position{X: 5, Y: 5}, spatial.Position{X: 8, Y: 6}, 3},
		{"mixed orthogonal", spatial.Position{X: 5, Y: 5}, spatial.Position{X: 6, Y: 8}, 3},
		{"knight's move", spatial.Position{X: 5, Y: 5}, spatial.Position{X: 7, Y: 6}, 2},
	}
	
	for _, tc := range testCases {
		s.Run(tc.name, func() {
			result := s.grid.Distance(tc.from, tc.to)
			s.Assert().Equal(tc.expected, result)
		})
	}
}

// TestGetNeighbors tests getting adjacent positions
func (s *SquareGridTestSuite) TestGetNeighbors() {
	s.Run("center position", func() {
		neighbors := s.grid.GetNeighbors(spatial.Position{X: 5, Y: 5})
		s.Assert().Len(neighbors, 8)
		
		// Should have all 8 neighbors
		expected := []spatial.Position{
			{X: 4, Y: 4}, {X: 4, Y: 5}, {X: 4, Y: 6},
			{X: 5, Y: 4},                 {X: 5, Y: 6},
			{X: 6, Y: 4}, {X: 6, Y: 5}, {X: 6, Y: 6},
		}
		
		for _, exp := range expected {
			s.Assert().Contains(neighbors, exp)
		}
	})
	
	s.Run("corner position", func() {
		neighbors := s.grid.GetNeighbors(spatial.Position{X: 0, Y: 0})
		s.Assert().Len(neighbors, 3)
		
		expected := []spatial.Position{
			{X: 0, Y: 1}, {X: 1, Y: 0}, {X: 1, Y: 1},
		}
		
		for _, exp := range expected {
			s.Assert().Contains(neighbors, exp)
		}
	})
	
	s.Run("edge position", func() {
		neighbors := s.grid.GetNeighbors(spatial.Position{X: 0, Y: 5})
		s.Assert().Len(neighbors, 5)
		
		expected := []spatial.Position{
			{X: 0, Y: 4}, {X: 0, Y: 6},
			{X: 1, Y: 4}, {X: 1, Y: 5}, {X: 1, Y: 6},
		}
		
		for _, exp := range expected {
			s.Assert().Contains(neighbors, exp)
		}
	})
}

// TestIsAdjacent tests adjacency checking
func (s *SquareGridTestSuite) TestIsAdjacent() {
	center := spatial.Position{X: 5, Y: 5}
	
	testCases := []struct {
		name     string
		position spatial.Position
		expected bool
	}{
		{"same position", spatial.Position{X: 5, Y: 5}, true},
		{"orthogonal adjacent", spatial.Position{X: 6, Y: 5}, true},
		{"diagonal adjacent", spatial.Position{X: 6, Y: 6}, true},
		{"two squares away", spatial.Position{X: 7, Y: 5}, false},
		{"knight's move", spatial.Position{X: 7, Y: 6}, false},
	}
	
	for _, tc := range testCases {
		s.Run(tc.name, func() {
			result := s.grid.IsAdjacent(center, tc.position)
			s.Assert().Equal(tc.expected, result)
		})
	}
}

// TestGetLineOfSight tests line of sight calculations
func (s *SquareGridTestSuite) TestGetLineOfSight() {
	s.Run("same position", func() {
		los := s.grid.GetLineOfSight(spatial.Position{X: 5, Y: 5}, spatial.Position{X: 5, Y: 5})
		s.Assert().Len(los, 1)
		s.Assert().Equal(spatial.Position{X: 5, Y: 5}, los[0])
	})
	
	s.Run("horizontal line", func() {
		los := s.grid.GetLineOfSight(spatial.Position{X: 2, Y: 5}, spatial.Position{X: 5, Y: 5})
		s.Assert().Len(los, 4)
		s.Assert().Contains(los, spatial.Position{X: 2, Y: 5})
		s.Assert().Contains(los, spatial.Position{X: 3, Y: 5})
		s.Assert().Contains(los, spatial.Position{X: 4, Y: 5})
		s.Assert().Contains(los, spatial.Position{X: 5, Y: 5})
	})
	
	s.Run("vertical line", func() {
		los := s.grid.GetLineOfSight(spatial.Position{X: 5, Y: 2}, spatial.Position{X: 5, Y: 5})
		s.Assert().Len(los, 4)
		s.Assert().Contains(los, spatial.Position{X: 5, Y: 2})
		s.Assert().Contains(los, spatial.Position{X: 5, Y: 3})
		s.Assert().Contains(los, spatial.Position{X: 5, Y: 4})
		s.Assert().Contains(los, spatial.Position{X: 5, Y: 5})
	})
	
	s.Run("diagonal line", func() {
		los := s.grid.GetLineOfSight(spatial.Position{X: 2, Y: 2}, spatial.Position{X: 5, Y: 5})
		s.Assert().Len(los, 4)
		s.Assert().Contains(los, spatial.Position{X: 2, Y: 2})
		s.Assert().Contains(los, spatial.Position{X: 5, Y: 5})
	})
}

// TestGetPositionsInRange tests range queries
func (s *SquareGridTestSuite) TestGetPositionsInRange() {
	center := spatial.Position{X: 5, Y: 5}
	
	s.Run("range 0", func() {
		positions := s.grid.GetPositionsInRange(center, 0)
		s.Assert().Len(positions, 1)
		s.Assert().Contains(positions, center)
	})
	
	s.Run("range 1", func() {
		positions := s.grid.GetPositionsInRange(center, 1)
		s.Assert().Len(positions, 9) // 3x3 area
		s.Assert().Contains(positions, center)
		s.Assert().Contains(positions, spatial.Position{X: 4, Y: 4})
		s.Assert().Contains(positions, spatial.Position{X: 6, Y: 6})
	})
	
	s.Run("range 2", func() {
		positions := s.grid.GetPositionsInRange(center, 2)
		s.Assert().Len(positions, 25) // 5x5 area
		s.Assert().Contains(positions, center)
		s.Assert().Contains(positions, spatial.Position{X: 3, Y: 3})
		s.Assert().Contains(positions, spatial.Position{X: 7, Y: 7})
	})
	
	s.Run("range at edge", func() {
		edge := spatial.Position{X: 0, Y: 0}
		positions := s.grid.GetPositionsInRange(edge, 1)
		s.Assert().Len(positions, 4) // Only positions within grid bounds
		s.Assert().Contains(positions, edge)
		s.Assert().Contains(positions, spatial.Position{X: 0, Y: 1})
		s.Assert().Contains(positions, spatial.Position{X: 1, Y: 0})
		s.Assert().Contains(positions, spatial.Position{X: 1, Y: 1})
	})
}

// TestGetPositionsInRectangle tests rectangular area queries
func (s *SquareGridTestSuite) TestGetPositionsInRectangle() {
	rect := spatial.Rectangle{
		Position:   spatial.Position{X: 2, Y: 2},
		Dimensions: spatial.Dimensions{Width: 3, Height: 3},
	}
	
	positions := s.grid.GetPositionsInRectangle(rect)
	s.Assert().Len(positions, 9) // 3x3 area
	
	// Check corners
	s.Assert().Contains(positions, spatial.Position{X: 2, Y: 2})
	s.Assert().Contains(positions, spatial.Position{X: 4, Y: 2})
	s.Assert().Contains(positions, spatial.Position{X: 2, Y: 4})
	s.Assert().Contains(positions, spatial.Position{X: 4, Y: 4})
	
	// Check center
	s.Assert().Contains(positions, spatial.Position{X: 3, Y: 3})
}

// TestGetPositionsInCircle tests circular area queries
func (s *SquareGridTestSuite) TestGetPositionsInCircle() {
	circle := spatial.Circle{
		Center: spatial.Position{X: 5, Y: 5},
		Radius: 2,
	}
	
	positions := s.grid.GetPositionsInCircle(circle)
	s.Assert().Len(positions, 25) // 5x5 area with D&D 5e distance
	
	// Check center
	s.Assert().Contains(positions, spatial.Position{X: 5, Y: 5})
	
	// Check positions at radius 2
	s.Assert().Contains(positions, spatial.Position{X: 3, Y: 5})
	s.Assert().Contains(positions, spatial.Position{X: 7, Y: 5})
	s.Assert().Contains(positions, spatial.Position{X: 5, Y: 3})
	s.Assert().Contains(positions, spatial.Position{X: 5, Y: 7})
	
	// Check diagonal positions at radius 2
	s.Assert().Contains(positions, spatial.Position{X: 3, Y: 3})
	s.Assert().Contains(positions, spatial.Position{X: 7, Y: 7})
}

// TestGetPositionsInLine tests line queries
func (s *SquareGridTestSuite) TestGetPositionsInLine() {
	from := spatial.Position{X: 2, Y: 2}
	to := spatial.Position{X: 5, Y: 5}
	
	positions := s.grid.GetPositionsInLine(from, to)
	s.Assert().Len(positions, 4)
	s.Assert().Contains(positions, from)
	s.Assert().Contains(positions, to)
}

// TestSmallGrid tests grid with small dimensions
func (s *SquareGridTestSuite) TestSmallGrid() {
	smallGrid := spatial.NewSquareGrid(spatial.SquareGridConfig{
		Width:  2,
		Height: 2,
	})
	
	s.Assert().Equal(spatial.GridShapeSquare, smallGrid.GetShape())
	
	// Test all positions are valid
	s.Assert().True(smallGrid.IsValidPosition(spatial.Position{X: 0, Y: 0}))
	s.Assert().True(smallGrid.IsValidPosition(spatial.Position{X: 1, Y: 1}))
	s.Assert().False(smallGrid.IsValidPosition(spatial.Position{X: 2, Y: 0}))
	
	// Test neighbors
	neighbors := smallGrid.GetNeighbors(spatial.Position{X: 0, Y: 0})
	s.Assert().Len(neighbors, 3)
}

// Run the test suite
func TestSquareGridSuite(t *testing.T) {
	suite.Run(t, new(SquareGridTestSuite))
}