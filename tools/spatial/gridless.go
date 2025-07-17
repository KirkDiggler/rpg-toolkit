package spatial

import (
	"math"
)

// GridlessRoom implements a gridless spatial system for theater-of-mind play
// Uses Euclidean distance and allows approximate positioning
type GridlessRoom struct {
	dimensions Dimensions
}

// GridlessConfig holds configuration for creating a gridless room
type GridlessConfig struct {
	Width  float64
	Height float64
}

// NewGridlessRoom creates a new gridless room with the given dimensions
func NewGridlessRoom(config GridlessConfig) *GridlessRoom {
	return &GridlessRoom{
		dimensions: Dimensions(config),
	}
}

// GetShape returns the grid shape type
func (gr *GridlessRoom) GetShape() GridShape {
	return GridShapeGridless
}

// IsValidPosition checks if a position is within the room bounds
// In gridless rooms, any position within the dimensions is valid
func (gr *GridlessRoom) IsValidPosition(pos Position) bool {
	return pos.X >= 0 && pos.X <= gr.dimensions.Width &&
		pos.Y >= 0 && pos.Y <= gr.dimensions.Height
}

// GetDimensions returns the room dimensions
func (gr *GridlessRoom) GetDimensions() Dimensions {
	return gr.dimensions
}

// Distance calculates the Euclidean distance between two positions
// This is the true geometric distance, not constrained by grid
func (gr *GridlessRoom) Distance(from, to Position) float64 {
	dx := to.X - from.X
	dy := to.Y - from.Y
	return math.Sqrt(dx*dx + dy*dy)
}

// GetNeighbors returns positions in a circle around the given position
// Since there's no grid, we return positions at various angles
func (gr *GridlessRoom) GetNeighbors(pos Position) []Position {
	neighbors := make([]Position, 0)
	radius := 1.0 // Standard neighbor distance

	// 8 directions at 45-degree intervals
	angles := []float64{
		0, math.Pi / 4, math.Pi / 2, 3 * math.Pi / 4,
		math.Pi, 5 * math.Pi / 4, 3 * math.Pi / 2, 7 * math.Pi / 4,
	}

	for _, angle := range angles {
		x := pos.X + radius*math.Cos(angle)
		y := pos.Y + radius*math.Sin(angle)
		neighbor := Position{X: x, Y: y}

		if gr.IsValidPosition(neighbor) {
			neighbors = append(neighbors, neighbor)
		}
	}

	return neighbors
}

// IsAdjacent checks if two positions are adjacent (within distance 1)
func (gr *GridlessRoom) IsAdjacent(pos1, pos2 Position) bool {
	return gr.Distance(pos1, pos2) <= 1.0
}

// GetLineOfSight returns positions along the line of sight
// Since there's no grid, we sample points along the line
func (gr *GridlessRoom) GetLineOfSight(from, to Position) []Position {
	if from.Equals(to) {
		return []Position{from}
	}

	distance := gr.Distance(from, to)
	// Sample every 0.5 units along the line
	sampleInterval := 0.5
	numSamples := int(math.Ceil(distance / sampleInterval))

	positions := make([]Position, 0, numSamples+1)

	for i := 0; i <= numSamples; i++ {
		t := float64(i) / float64(numSamples)
		x := from.X + t*(to.X-from.X)
		y := from.Y + t*(to.Y-from.Y)
		pos := Position{X: x, Y: y}

		if gr.IsValidPosition(pos) {
			positions = append(positions, pos)
		}
	}

	return positions
}

// GetPositionsInRange returns all positions within range
// Since there's no grid, we sample in a circular pattern
func (gr *GridlessRoom) GetPositionsInRange(center Position, radius float64) []Position {
	positions := make([]Position, 0)

	// Adaptive sampling based on area to prevent excessive sample points
	// Target ~10,000 samples maximum for performance
	maxSamples := 10000.0
	area := math.Pi * radius * radius
	sampleSize := math.Max(0.1, math.Sqrt(area/maxSamples))

	// For small areas, maintain reasonable precision
	if sampleSize > 0.5 {
		sampleSize = 0.5
	}

	// Create a bounding box around the circle
	minX := math.Max(0, center.X-radius)
	maxX := math.Min(gr.dimensions.Width, center.X+radius)
	minY := math.Max(0, center.Y-radius)
	maxY := math.Min(gr.dimensions.Height, center.Y+radius)

	// Always include the center position if it's within the radius and valid
	if gr.IsValidPosition(center) && gr.Distance(center, center) <= radius {
		positions = append(positions, center)
	}

	// Sample points in the bounding box
	for x := minX; x <= maxX; x += sampleSize {
		for y := minY; y <= maxY; y += sampleSize {
			pos := Position{X: x, Y: y}
			// Skip if this is very close to center (already added)
			if gr.Distance(center, pos) < sampleSize/2 {
				continue
			}
			if gr.IsValidPosition(pos) && gr.Distance(center, pos) <= radius {
				positions = append(positions, pos)
			}
		}
	}

	return positions
}

// GetPositionsInRectangle returns positions within a rectangular area
func (gr *GridlessRoom) GetPositionsInRectangle(rect Rectangle) []Position {
	positions := make([]Position, 0)

	// Adaptive sampling based on rectangle area
	maxSamples := 10000.0
	area := rect.Dimensions.Width * rect.Dimensions.Height
	sampleSize := math.Max(0.1, math.Sqrt(area/maxSamples))

	// For small areas, maintain reasonable precision
	if sampleSize > 0.5 {
		sampleSize = 0.5
	}

	minX := math.Max(0, rect.Position.X)
	maxX := math.Min(gr.dimensions.Width, rect.Position.X+rect.Dimensions.Width)
	minY := math.Max(0, rect.Position.Y)
	maxY := math.Min(gr.dimensions.Height, rect.Position.Y+rect.Dimensions.Height)

	for x := minX; x <= maxX; x += sampleSize {
		for y := minY; y <= maxY; y += sampleSize {
			pos := Position{X: x, Y: y}
			if gr.IsValidPosition(pos) && rect.Contains(pos) {
				positions = append(positions, pos)
			}
		}
	}

	return positions
}

// GetPositionsInCircle returns positions within a circular area
func (gr *GridlessRoom) GetPositionsInCircle(circle Circle) []Position {
	return gr.GetPositionsInRange(circle.Center, circle.Radius)
}

// GetPositionsInLine returns positions along a line
func (gr *GridlessRoom) GetPositionsInLine(from, to Position) []Position {
	return gr.GetLineOfSight(from, to)
}

// GetPositionsInCone returns positions within a cone shape
func (gr *GridlessRoom) GetPositionsInCone(
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

	// Adaptive sampling based on cone area
	maxSamples := 10000.0
	area := math.Pi * length * length * (angle / (2 * math.Pi)) // Cone sector area
	sampleSize := math.Max(0.1, math.Sqrt(area/maxSamples))

	// For small areas, maintain reasonable precision
	if sampleSize > 0.5 {
		sampleSize = 0.5
	}

	// Check positions within the cone's bounding area
	for x := origin.X - length; x <= origin.X+length; x += sampleSize {
		for y := origin.Y - length; y <= origin.Y+length; y += sampleSize {
			pos := Position{X: x, Y: y}
			if !gr.IsValidPosition(pos) {
				continue
			}

			// Check if position is within cone distance
			if gr.Distance(origin, pos) > length {
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

// GetPositionsInArc returns positions within an arc (portion of a circle)
// This is useful for gridless rooms where you want spell effects in arcs
func (gr *GridlessRoom) GetPositionsInArc(center Position, radius float64, startAngle, endAngle float64) []Position {
	positions := make([]Position, 0)

	// Adaptive sampling based on arc area
	maxSamples := 10000.0
	arcAngle := math.Abs(endAngle - startAngle)
	area := 0.5 * radius * radius * arcAngle // Arc sector area
	sampleSize := math.Max(0.1, math.Sqrt(area/maxSamples))

	// For small areas, maintain reasonable precision
	if sampleSize > 0.5 {
		sampleSize = 0.5
	}

	// Normalize angles to [0, 2π)
	for startAngle < 0 {
		startAngle += 2 * math.Pi
	}
	for endAngle < 0 {
		endAngle += 2 * math.Pi
	}

	// Sample points in the arc
	for x := center.X - radius; x <= center.X+radius; x += sampleSize {
		for y := center.Y - radius; y <= center.Y+radius; y += sampleSize {
			pos := Position{X: x, Y: y}
			if !gr.IsValidPosition(pos) {
				continue
			}

			distance := gr.Distance(center, pos)
			if distance > radius {
				continue
			}

			// Calculate angle from center to position
			angle := math.Atan2(y-center.Y, x-center.X)
			if angle < 0 {
				angle += 2 * math.Pi
			}

			// Check if angle is within arc
			if endAngle >= startAngle {
				if angle >= startAngle && angle <= endAngle {
					positions = append(positions, pos)
				}
			} else {
				// Arc crosses 0 radians
				if angle >= startAngle || angle <= endAngle {
					positions = append(positions, pos)
				}
			}
		}
	}

	return positions
}

// GetNearestPosition returns the nearest valid position to the given position
// Useful for "snapping" entities to valid positions
func (gr *GridlessRoom) GetNearestPosition(pos Position) Position {
	if gr.IsValidPosition(pos) {
		return pos
	}

	// Clamp to room bounds
	x := math.Max(0, math.Min(gr.dimensions.Width, pos.X))
	y := math.Max(0, math.Min(gr.dimensions.Height, pos.Y))

	return Position{X: x, Y: y}
}
