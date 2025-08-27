package spatial

import (
	"math"
)

// SquareGrid implements a square grid system with D&D 5e distance rules
type SquareGrid struct {
	dimensions Dimensions
}

// SquareGridConfig holds configuration for creating a square grid
type SquareGridConfig struct {
	Width  float64
	Height float64
}

// NewSquareGrid creates a new square grid with the given dimensions
func NewSquareGrid(config SquareGridConfig) *SquareGrid {
	return &SquareGrid{
		dimensions: Dimensions(config),
	}
}

// GetShape returns the grid shape type
func (sg *SquareGrid) GetShape() GridShape {
	return GridShapeSquare
}

// IsValidPosition checks if a position is valid within the grid bounds
func (sg *SquareGrid) IsValidPosition(pos Position) bool {
	return pos.X >= 0 && pos.X < sg.dimensions.Width &&
		pos.Y >= 0 && pos.Y < sg.dimensions.Height
}

// GetDimensions returns the grid dimensions
func (sg *SquareGrid) GetDimensions() Dimensions {
	return sg.dimensions
}

// Distance calculates the distance between two positions using D&D 5e rules
// D&D 5e uses Chebyshev distance: max(|x2-x1|, |y2-y1|)
// This means diagonals cost the same as orthogonal movement
func (sg *SquareGrid) Distance(from, to Position) float64 {
	dx := math.Abs(to.X - from.X)
	dy := math.Abs(to.Y - from.Y)
	return math.Max(dx, dy)
}

// GetNeighbors returns all 8 adjacent positions (including diagonals)
func (sg *SquareGrid) GetNeighbors(pos Position) []Position {
	neighbors := make([]Position, 0, 8)

	// All 8 directions: orthogonal + diagonal
	directions := []Position{
		{-1, -1}, {-1, 0}, {-1, 1},
		{0, -1}, {0, 1},
		{1, -1}, {1, 0}, {1, 1},
	}

	for _, dir := range directions {
		neighbor := pos.Add(dir)
		if sg.IsValidPosition(neighbor) {
			neighbors = append(neighbors, neighbor)
		}
	}

	return neighbors
}

// IsAdjacent checks if two positions are adjacent (within 1 square, including diagonals)
func (sg *SquareGrid) IsAdjacent(pos1, pos2 Position) bool {
	return sg.Distance(pos1, pos2) <= 1
}

// GetLineOfSight returns positions along the line of sight between two positions
// Uses Bresenham's line algorithm for grid-based line drawing
func (sg *SquareGrid) GetLineOfSight(from, to Position) []Position {
	if from.Equals(to) {
		return []Position{from}
	}

	// Convert to integer coordinates for Bresenham's algorithm
	x0, y0 := int(from.X), int(from.Y)
	x1, y1 := int(to.X), int(to.Y)

	positions := make([]Position, 0)

	dx := abs(x1 - x0)
	dy := abs(y1 - y0)

	var sx, sy int
	if x0 < x1 {
		sx = 1
	} else {
		sx = -1
	}
	if y0 < y1 {
		sy = 1
	} else {
		sy = -1
	}

	err := dx - dy
	x, y := x0, y0

	for {
		pos := Position{X: float64(x), Y: float64(y)}
		if sg.IsValidPosition(pos) {
			positions = append(positions, pos)
		}

		if x == x1 && y == y1 {
			break
		}

		e2 := 2 * err
		if e2 > -dy {
			err -= dy
			x += sx
		}
		if e2 < dx {
			err += dx
			y += sy
		}
	}

	return positions
}

// GetPositionsInRange returns all positions within a given range using D&D 5e distance
func (sg *SquareGrid) GetPositionsInRange(center Position, radius float64) []Position {
	positions := make([]Position, 0)

	// Calculate bounding box to avoid checking every position in the grid
	minX := math.Max(0, center.X-radius)
	maxX := math.Min(sg.dimensions.Width-1, center.X+radius)
	minY := math.Max(0, center.Y-radius)
	maxY := math.Min(sg.dimensions.Height-1, center.Y+radius)

	for x := minX; x <= maxX; x++ {
		for y := minY; y <= maxY; y++ {
			pos := Position{X: x, Y: y}
			if sg.IsValidPosition(pos) && sg.Distance(center, pos) <= radius {
				positions = append(positions, pos)
			}
		}
	}

	return positions
}

// GetPositionsInRectangle returns all positions within a rectangular area
func (sg *SquareGrid) GetPositionsInRectangle(rect Rectangle) []Position {
	positions := make([]Position, 0)

	minX := math.Max(0, rect.Position.X)
	maxX := math.Min(sg.dimensions.Width-1, rect.Position.X+rect.Dimensions.Width-1)
	minY := math.Max(0, rect.Position.Y)
	maxY := math.Min(sg.dimensions.Height-1, rect.Position.Y+rect.Dimensions.Height-1)

	for x := minX; x <= maxX; x++ {
		for y := minY; y <= maxY; y++ {
			pos := Position{X: x, Y: y}
			if sg.IsValidPosition(pos) {
				positions = append(positions, pos)
			}
		}
	}

	return positions
}

// GetPositionsInCircle returns all positions within a circular area using D&D 5e distance
func (sg *SquareGrid) GetPositionsInCircle(circle Circle) []Position {
	return sg.GetPositionsInRange(circle.Center, circle.Radius)
}

// GetPositionsInLine returns positions along a line from start to end
func (sg *SquareGrid) GetPositionsInLine(from, to Position) []Position {
	return sg.GetLineOfSight(from, to)
}

// GetPositionsInCone returns positions within a cone shape
// This is a simplified cone implementation - games may need more sophisticated cone logic
func (sg *SquareGrid) GetPositionsInCone(
	origin Position, direction Position, length float64, angle float64,
) []Position {
	positions := make([]Position, 0)

	// Normalize direction vector
	dirLength := math.Sqrt(direction.X*direction.X + direction.Y*direction.Y)
	if dirLength == 0 {
		return positions
	}

	dirX := direction.X / dirLength
	dirY := direction.Y / dirLength

	// Check positions within the cone's bounding area
	for x := origin.X - length; x <= origin.X+length; x++ {
		for y := origin.Y - length; y <= origin.Y+length; y++ {
			pos := Position{X: x, Y: y}
			if !sg.IsValidPosition(pos) {
				continue
			}

			// Check if position is within cone distance
			if sg.Distance(origin, pos) > length {
				continue
			}

			// Calculate angle between direction and position vector
			posX := x - origin.X
			posY := y - origin.Y
			posLength := math.Sqrt(posX*posX + posY*posY)

			if posLength == 0 {
				positions = append(positions, pos) // Origin is always included
				continue
			}

			// Dot product to get cosine of angle
			dot := (dirX*posX + dirY*posY) / posLength
			angleToPos := math.Acos(math.Max(-1, math.Min(1, dot)))

			// Check if within cone angle
			if angleToPos <= angle/2 {
				positions = append(positions, pos)
			}
		}
	}

	return positions
}
