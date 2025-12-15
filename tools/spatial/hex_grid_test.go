package spatial_test

import (
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/KirkDiggler/rpg-toolkit/tools/spatial"
)

type HexGridTestSuite struct {
	suite.Suite
	grid *spatial.HexGrid
}

// SetupTest runs before EACH test function
func (s *HexGridTestSuite) SetupTest() {
	s.grid = spatial.NewHexGrid(spatial.HexGridConfig{
		Width:     10,
		Height:    10,
		PointyTop: true,
	})
}

// TestHexGridCreation tests basic hex grid creation
func (s *HexGridTestSuite) TestHexGridCreation() {
	s.Require().NotNil(s.grid)
	s.Assert().Equal(spatial.GridShapeHex, s.grid.GetShape())

	dimensions := s.grid.GetDimensions()
	s.Assert().Equal(10.0, dimensions.Width)
	s.Assert().Equal(10.0, dimensions.Height)
}

// TestIsValidPosition tests position validation
func (s *HexGridTestSuite) TestIsValidPosition() {
	spatial.RunPositionValidationTests(s.T(), s.grid)
}

// TestHexDistance tests hex distance calculations using native cube coordinates
// Position is interpreted as: X = cube.x, Y = cube.z, cube.y = -x - z (derived)
func (s *HexGridTestSuite) TestHexDistance() {
	testCases := []struct {
		name     string
		from     spatial.Position
		to       spatial.Position
		expected float64
	}{
		{"same position", spatial.Position{X: 5, Y: 5}, spatial.Position{X: 5, Y: 5}, 0},
		// In native cube: moving +1 in x only is distance 1
		{"adjacent x-direction", spatial.Position{X: 5, Y: 5}, spatial.Position{X: 6, Y: 5}, 1},
		// In native cube: moving +1 in z only is distance 1
		{"adjacent z-direction", spatial.Position{X: 5, Y: 5}, spatial.Position{X: 5, Y: 6}, 1},
		{"two hexes in x", spatial.Position{X: 5, Y: 5}, spatial.Position{X: 7, Y: 5}, 2},
		{"three hexes in x", spatial.Position{X: 5, Y: 5}, spatial.Position{X: 8, Y: 5}, 3},
		// In native cube: moving +1 in both x AND z is distance 2 (not 1)
		// because cube.y changes by -2 when both x and z increase by 1
		{"diagonal x+z movement", spatial.Position{X: 5, Y: 5}, spatial.Position{X: 6, Y: 6}, 2},
		{"longer diagonal", spatial.Position{X: 5, Y: 5}, spatial.Position{X: 7, Y: 7}, 4},
	}

	for _, tc := range testCases {
		s.Run(tc.name, func() {
			result := s.grid.Distance(tc.from, tc.to)
			s.Assert().Equal(tc.expected, result)
		})
	}
}

// TestGetNeighbors tests getting adjacent positions (6 neighbors in hex)
func (s *HexGridTestSuite) TestGetNeighbors() {
	s.Run("center position", func() {
		neighbors := s.grid.GetNeighbors(spatial.Position{X: 5, Y: 5})
		s.Assert().Len(neighbors, 6) // Hex grids have 6 neighbors

		// All neighbors should be distance 1 away
		center := spatial.Position{X: 5, Y: 5}
		for _, neighbor := range neighbors {
			s.Assert().Equal(1.0, s.grid.Distance(center, neighbor))
		}
	})

	s.Run("corner position", func() {
		neighbors := s.grid.GetNeighbors(spatial.Position{X: 0, Y: 0})
		s.Assert().True(len(neighbors) <= 6) // Some neighbors will be out of bounds

		// All returned neighbors should be valid and distance 1
		origin := spatial.Position{X: 0, Y: 0}
		for _, neighbor := range neighbors {
			s.Assert().True(s.grid.IsValidPosition(neighbor))
			s.Assert().Equal(1.0, s.grid.Distance(origin, neighbor))
		}
	})

	s.Run("edge position", func() {
		neighbors := s.grid.GetNeighbors(spatial.Position{X: 0, Y: 5})
		s.Assert().True(len(neighbors) <= 6) // Some neighbors will be out of bounds

		// All returned neighbors should be valid and distance 1
		edge := spatial.Position{X: 0, Y: 5}
		for _, neighbor := range neighbors {
			s.Assert().True(s.grid.IsValidPosition(neighbor))
			s.Assert().Equal(1.0, s.grid.Distance(edge, neighbor))
		}
	})
}

// TestIsAdjacent tests adjacency checking
func (s *HexGridTestSuite) TestIsAdjacent() {
	center := spatial.Position{X: 5, Y: 5}

	testCases := []struct {
		name     string
		position spatial.Position
		expected bool
	}{
		{"same position", spatial.Position{X: 5, Y: 5}, true},
		{"adjacent neighbor", spatial.Position{X: 6, Y: 5}, true},
		{"two hexes away", spatial.Position{X: 7, Y: 5}, false},
		{"three hexes away", spatial.Position{X: 8, Y: 5}, false},
	}

	for _, tc := range testCases {
		s.Run(tc.name, func() {
			result := s.grid.IsAdjacent(center, tc.position)
			s.Assert().Equal(tc.expected, result)
		})
	}
}

// TestGetLineOfSight tests hex line of sight calculations
func (s *HexGridTestSuite) TestGetLineOfSight() {
	s.Run("same position", func() {
		los := s.grid.GetLineOfSight(spatial.Position{X: 5, Y: 5}, spatial.Position{X: 5, Y: 5})
		s.Assert().Len(los, 1)
		s.Assert().Equal(spatial.Position{X: 5, Y: 5}, los[0])
	})

	s.Run("adjacent positions", func() {
		los := s.grid.GetLineOfSight(spatial.Position{X: 5, Y: 5}, spatial.Position{X: 6, Y: 5})
		s.Assert().Len(los, 2)
		s.Assert().Contains(los, spatial.Position{X: 5, Y: 5})
		s.Assert().Contains(los, spatial.Position{X: 6, Y: 5})
	})

	s.Run("longer line", func() {
		los := s.grid.GetLineOfSight(spatial.Position{X: 2, Y: 2}, spatial.Position{X: 5, Y: 5})
		s.Assert().True(len(los) >= 2) // Should have at least start and end
		s.Assert().Contains(los, spatial.Position{X: 2, Y: 2})
		s.Assert().Contains(los, spatial.Position{X: 5, Y: 5})
	})
}

// TestGetPositionsInRange tests hex range queries
func (s *HexGridTestSuite) TestGetPositionsInRange() {
	center := spatial.Position{X: 5, Y: 5}

	s.Run("range 0", func() {
		positions := s.grid.GetPositionsInRange(center, 0)
		s.Assert().Len(positions, 1)
		s.Assert().Contains(positions, center)
	})

	s.Run("range 1", func() {
		positions := s.grid.GetPositionsInRange(center, 1)
		s.Assert().Len(positions, 7) // Center + 6 neighbors
		s.Assert().Contains(positions, center)

		// All positions should be within range 1
		for _, pos := range positions {
			s.Assert().True(s.grid.Distance(center, pos) <= 1)
		}
	})

	s.Run("range 2", func() {
		positions := s.grid.GetPositionsInRange(center, 2)
		s.Assert().Len(positions, 19) // Hex pattern: 1 + 6 + 12 = 19
		s.Assert().Contains(positions, center)

		// All positions should be within range 2
		for _, pos := range positions {
			s.Assert().True(s.grid.Distance(center, pos) <= 2)
		}
	})

	s.Run("range at edge", func() {
		edge := spatial.Position{X: 0, Y: 0}
		positions := s.grid.GetPositionsInRange(edge, 1)
		s.Assert().True(len(positions) <= 7) // Some positions will be out of bounds

		// All returned positions should be valid and within range
		for _, pos := range positions {
			s.Assert().True(s.grid.IsValidPosition(pos))
			s.Assert().True(s.grid.Distance(edge, pos) <= 1)
		}
	})
}

// TestGetPositionsInCircle tests circular area queries
func (s *HexGridTestSuite) TestGetPositionsInCircle() {
	circle := spatial.Circle{
		Center: spatial.Position{X: 5, Y: 5},
		Radius: 2,
	}

	positions := s.grid.GetPositionsInCircle(circle)
	s.Assert().Len(positions, 19) // Same as range 2 test

	// Check center is included
	s.Assert().Contains(positions, spatial.Position{X: 5, Y: 5})

	// All positions should be within the circle using hex distance
	for _, pos := range positions {
		s.Assert().True(s.grid.Distance(circle.Center, pos) <= circle.Radius)
	}
}

// TestGetPositionsInLine tests line queries
func (s *HexGridTestSuite) TestGetPositionsInLine() {
	from := spatial.Position{X: 2, Y: 2}
	to := spatial.Position{X: 5, Y: 5}

	positions := s.grid.GetPositionsInLine(from, to)
	s.Assert().True(len(positions) >= 2)
	s.Assert().Contains(positions, from)
	s.Assert().Contains(positions, to)
}

// TestGetHexRing tests hex-specific ring functionality
func (s *HexGridTestSuite) TestGetHexRing() {
	center := spatial.Position{X: 5, Y: 5}

	s.Run("ring 0", func() {
		ring := s.grid.GetHexRing(center, 0)
		s.Assert().Len(ring, 1)
		s.Assert().Contains(ring, center)
	})

	s.Run("ring 1", func() {
		ring := s.grid.GetHexRing(center, 1)
		s.Assert().Len(ring, 6) // 6 positions in first ring

		// All positions should be exactly distance 1 from center
		for _, pos := range ring {
			s.Assert().Equal(1.0, s.grid.Distance(center, pos))
		}
	})

	s.Run("ring 2", func() {
		ring := s.grid.GetHexRing(center, 2)
		s.Assert().Len(ring, 12) // 12 positions in second ring

		// All positions should be exactly distance 2 from center
		for _, pos := range ring {
			s.Assert().Equal(2.0, s.grid.Distance(center, pos))
		}
	})
}

// TestGetHexSpiral tests hex-specific spiral functionality
func (s *HexGridTestSuite) TestGetHexSpiral() {
	center := spatial.Position{X: 5, Y: 5}

	s.Run("spiral radius 0", func() {
		spiral := s.grid.GetHexSpiral(center, 0)
		s.Assert().Len(spiral, 1)
		s.Assert().Contains(spiral, center)
	})

	s.Run("spiral radius 1", func() {
		spiral := s.grid.GetHexSpiral(center, 1)
		s.Assert().Len(spiral, 7) // 1 + 6 = 7
		s.Assert().Contains(spiral, center)
	})

	s.Run("spiral radius 2", func() {
		spiral := s.grid.GetHexSpiral(center, 2)
		s.Assert().Len(spiral, 19) // 1 + 6 + 12 = 19
		s.Assert().Contains(spiral, center)
	})
}

// TestCubeCoordinateConversion tests conversion between offset and cube coordinates
func (s *HexGridTestSuite) TestCubeCoordinateConversion() {
	testCases := []struct {
		name   string
		offset spatial.Position
	}{
		{"origin", spatial.Position{X: 0, Y: 0}},
		{"center", spatial.Position{X: 5, Y: 5}},
		{"corner", spatial.Position{X: 9, Y: 9}},
		{"edge", spatial.Position{X: 0, Y: 5}},
	}

	for _, tc := range testCases {
		s.Run(tc.name, func() {
			// Convert to cube and back
			cube := spatial.OffsetCoordinateToCube(tc.offset)
			s.Assert().True(cube.IsValid()) // Should be valid cube coordinate

			converted := cube.ToOffsetCoordinate()
			s.Assert().Equal(tc.offset, converted) // Should round-trip correctly
		})
	}
}

// TestSmallHexGrid tests hex grid with small dimensions
func (s *HexGridTestSuite) TestSmallHexGrid() {
	smallGrid := spatial.NewHexGrid(spatial.HexGridConfig{
		Width:     3,
		Height:    3,
		PointyTop: true,
	})

	s.Assert().Equal(spatial.GridShapeHex, smallGrid.GetShape())

	// Test all positions are valid
	s.Assert().True(smallGrid.IsValidPosition(spatial.Position{X: 0, Y: 0}))
	s.Assert().True(smallGrid.IsValidPosition(spatial.Position{X: 2, Y: 2}))
	s.Assert().False(smallGrid.IsValidPosition(spatial.Position{X: 3, Y: 0}))

	// Test neighbors
	neighbors := smallGrid.GetNeighbors(spatial.Position{X: 1, Y: 1})
	s.Assert().True(len(neighbors) <= 6)

	// All neighbors should be valid
	for _, neighbor := range neighbors {
		s.Assert().True(smallGrid.IsValidPosition(neighbor))
	}
}

// TestGetOrientation tests the GetOrientation method
func (s *HexGridTestSuite) TestGetOrientation() {
	s.Run("default is pointy-top", func() {
		grid := spatial.NewHexGrid(spatial.HexGridConfig{
			Width:  5,
			Height: 5,
		})
		s.Assert().Equal(spatial.HexOrientationPointyTop, grid.GetOrientation())
		s.Assert().True(grid.IsPointyTop())
	})

	s.Run("pointy-top orientation via Orientation field", func() {
		pointyGrid := spatial.NewHexGrid(spatial.HexGridConfig{
			Width:       5,
			Height:      5,
			Orientation: spatial.HexOrientationPointyTop,
		})
		s.Assert().Equal(spatial.HexOrientationPointyTop, pointyGrid.GetOrientation())
		s.Assert().True(pointyGrid.IsPointyTop())
	})

	s.Run("flat-top orientation via Orientation field", func() {
		flatGrid := spatial.NewHexGrid(spatial.HexGridConfig{
			Width:       5,
			Height:      5,
			Orientation: spatial.HexOrientationFlatTop,
		})
		s.Assert().Equal(spatial.HexOrientationFlatTop, flatGrid.GetOrientation())
		s.Assert().False(flatGrid.IsPointyTop())
	})
}

// TestCoordinateConversionMethods tests the native cube coordinate methods
func (s *HexGridTestSuite) TestCoordinateConversionMethods() {
	// Position is native cube: X = cube.x, Y = cube.z
	center := spatial.Position{X: 5, Y: 5}

	s.Run("PositionToCube", func() {
		cube := s.grid.PositionToCube(center)
		s.Assert().True(cube.IsValid()) // Should be valid cube coordinate
		// Verify native cube interpretation: X=5 (cube.x), Y=5 (cube.z), derived y = -5-5 = -10
		s.Assert().Equal(5, cube.X)
		s.Assert().Equal(-10, cube.Y)
		s.Assert().Equal(5, cube.Z)
	})

	s.Run("CubeToPosition round-trip", func() {
		cube := s.grid.PositionToCube(center)
		converted := s.grid.CubeToPosition(cube)
		s.Assert().Equal(center, converted)
	})

	s.Run("GetCubeNeighbors", func() {
		cubeNeighbors := s.grid.GetCubeNeighbors(center)
		s.Assert().Len(cubeNeighbors, 6) // Should have 6 cube neighbors

		// All cube neighbors should be valid
		for _, cube := range cubeNeighbors {
			s.Assert().True(cube.IsValid())
		}

		// Convert to positions and verify they match GetNeighbors
		positionNeighbors := s.grid.GetNeighbors(center)
		s.Assert().Len(positionNeighbors, 6) // Should match position neighbors count

		// Each cube neighbor should convert to a valid position neighbor
		for _, cube := range cubeNeighbors {
			pos := s.grid.CubeToPosition(cube)
			if s.grid.IsValidPosition(pos) {
				s.Assert().Contains(positionNeighbors, pos)
			}
		}
	})
}

// Run the test suite
func TestHexGridSuite(t *testing.T) {
	suite.Run(t, new(HexGridTestSuite))
}
