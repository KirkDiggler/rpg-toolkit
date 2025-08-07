package spatial

import (
	"math"
)

// HexGrid implements a hexagonal grid system using cube coordinates
type HexGrid struct {
	dimensions Dimensions
	// Hex grids can be pointy-top or flat-top oriented
	pointyTop bool
}

// HexGridConfig holds configuration for creating a hex grid
type HexGridConfig struct {
	Width     float64
	Height    float64
	PointyTop bool // true for pointy-top, false for flat-top
}

// NewHexGrid creates a new hex grid with the given dimensions
// Defaults to pointy-top orientation for D&D 5e compatibility
func NewHexGrid(config HexGridConfig) *HexGrid {
	return &HexGrid{
		dimensions: Dimensions{
			Width:  config.Width,
			Height: config.Height,
		},
		pointyTop: config.PointyTop,
	}
}

// GetShape returns the grid shape type
func (hg *HexGrid) GetShape() GridShape {
	return GridShapeHex
}

// GetOrientation returns true for pointy-top, false for flat-top orientation
func (hg *HexGrid) GetOrientation() bool {
	return hg.pointyTop
}

// IsValidPosition checks if a position is valid within the grid bounds
func (hg *HexGrid) IsValidPosition(pos Position) bool {
	return pos.X >= 0 && pos.X < hg.dimensions.Width &&
		pos.Y >= 0 && pos.Y < hg.dimensions.Height
}

// GetDimensions returns the grid dimensions
func (hg *HexGrid) GetDimensions() Dimensions {
	return hg.dimensions
}

// Distance calculates the distance between two positions using hex grid rules
// Converts to cube coordinates and uses hex distance formula
func (hg *HexGrid) Distance(from, to Position) float64 {
	fromCube := OffsetCoordinateToCube(from)
	toCube := OffsetCoordinateToCube(to)
	return float64(fromCube.Distance(toCube))
}

// GetNeighbors returns all 6 adjacent positions in hex grid
func (hg *HexGrid) GetNeighbors(pos Position) []Position {
	cube := OffsetCoordinateToCube(pos)
	neighborCubes := cube.GetNeighbors()

	neighbors := make([]Position, 0, 6)
	for _, neighborCube := range neighborCubes {
		neighborPos := neighborCube.ToOffsetCoordinate()
		if hg.IsValidPosition(neighborPos) {
			neighbors = append(neighbors, neighborPos)
		}
	}

	return neighbors
}

// IsAdjacent checks if two positions are adjacent (within 1 hex)
func (hg *HexGrid) IsAdjacent(pos1, pos2 Position) bool {
	return hg.Distance(pos1, pos2) <= 1
}

// GetLineOfSight returns positions along the line of sight between two positions
// Uses cube coordinate lerp for hex line drawing
func (hg *HexGrid) GetLineOfSight(from, to Position) []Position {
	if from.Equals(to) {
		return []Position{from}
	}

	fromCube := OffsetCoordinateToCube(from)
	toCube := OffsetCoordinateToCube(to)

	distance := fromCube.Distance(toCube)
	positions := make([]Position, 0, distance+1)

	for i := 0; i <= distance; i++ {
		t := float64(i) / float64(distance)
		lerpedCube := hg.lerpCube(fromCube, toCube, t)
		roundedCube := hg.roundCube(lerpedCube)
		pos := roundedCube.ToOffsetCoordinate()

		if hg.IsValidPosition(pos) {
			positions = append(positions, pos)
		}
	}

	return positions
}

// GetPositionsInRange returns all positions within a given range using hex distance
func (hg *HexGrid) GetPositionsInRange(center Position, radius float64) []Position {
	positions := make([]Position, 0)
	centerCube := OffsetCoordinateToCube(center)

	// Calculate bounding box in cube coordinates
	iRadius := int(radius)
	for x := -iRadius; x <= iRadius; x++ {
		for y := math.Max(float64(-iRadius), float64(-x-iRadius)); y <= math.Min(float64(iRadius), float64(-x+iRadius)); y++ {
			z := -x - int(y)
			cube := CubeCoordinate{X: centerCube.X + x, Y: centerCube.Y + int(y), Z: centerCube.Z + z}

			if cube.Distance(centerCube) <= iRadius {
				pos := cube.ToOffsetCoordinate()
				if hg.IsValidPosition(pos) {
					positions = append(positions, pos)
				}
			}
		}
	}

	return positions
}

// GetPositionsInRectangle returns all positions within a rectangular area
// Note: This is approximate for hex grids since rectangles don't align perfectly with hex geometry
func (hg *HexGrid) GetPositionsInRectangle(rect Rectangle) []Position {
	positions := make([]Position, 0)

	minX := math.Max(0, rect.Position.X)
	maxX := math.Min(hg.dimensions.Width-1, rect.Position.X+rect.Dimensions.Width-1)
	minY := math.Max(0, rect.Position.Y)
	maxY := math.Min(hg.dimensions.Height-1, rect.Position.Y+rect.Dimensions.Height-1)

	for x := minX; x <= maxX; x++ {
		for y := minY; y <= maxY; y++ {
			pos := Position{X: x, Y: y}
			if hg.IsValidPosition(pos) {
				positions = append(positions, pos)
			}
		}
	}

	return positions
}

// GetPositionsInCircle returns all positions within a circular area using hex distance
func (hg *HexGrid) GetPositionsInCircle(circle Circle) []Position {
	return hg.GetPositionsInRange(circle.Center, circle.Radius)
}

// GetPositionsInLine returns positions along a line from start to end
func (hg *HexGrid) GetPositionsInLine(from, to Position) []Position {
	return hg.GetLineOfSight(from, to)
}

// GetPositionsInCone returns positions within a cone shape
// This is more complex for hex grids due to the 6-sided nature
func (hg *HexGrid) GetPositionsInCone(origin Position, direction Position, length float64, angle float64) []Position {
	positions := make([]Position, 0)

	// Convert to cube coordinates for easier math
	_ = OffsetCoordinateToCube(origin)

	// Normalize direction vector
	dirLength := math.Sqrt(direction.X*direction.X + direction.Y*direction.Y)
	if dirLength == 0 {
		return positions
	}

	dirX := direction.X / dirLength
	dirY := direction.Y / dirLength

	// Get all positions in range and filter by angle
	rangePositions := hg.GetPositionsInRange(origin, length)

	for _, pos := range rangePositions {
		if pos.Equals(origin) {
			positions = append(positions, pos)
			continue
		}

		// Calculate angle between direction and position vector
		posX := pos.X - origin.X
		posY := pos.Y - origin.Y
		posLength := math.Sqrt(posX*posX + posY*posY)

		if posLength == 0 {
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

	return positions
}

// GetHexRing returns positions forming a ring at a specific distance from center
// This is a hex-specific function that's useful for spell effects
func (hg *HexGrid) GetHexRing(center Position, radius int) []Position {
	if radius == 0 {
		return []Position{center}
	}

	positions := make([]Position, 0)
	centerCube := OffsetCoordinateToCube(center)

	// Simple approach: get all positions at exactly the specified distance
	for x := int(center.X) - radius; x <= int(center.X)+radius; x++ {
		for y := int(center.Y) - radius; y <= int(center.Y)+radius; y++ {
			pos := Position{X: float64(x), Y: float64(y)}
			if hg.IsValidPosition(pos) {
				posCube := OffsetCoordinateToCube(pos)
				if centerCube.Distance(posCube) == radius {
					positions = append(positions, pos)
				}
			}
		}
	}

	return positions
}

// GetHexSpiral returns positions in a spiral pattern from center outward
// Useful for area effects that expand outward
func (hg *HexGrid) GetHexSpiral(center Position, radius int) []Position {
	positions := make([]Position, 0)

	for r := 0; r <= radius; r++ {
		ring := hg.GetHexRing(center, r)
		positions = append(positions, ring...)
	}

	return positions
}

// OffsetToCube converts an offset coordinate to cube coordinate using this grid's settings
func (hg *HexGrid) OffsetToCube(pos Position) CubeCoordinate {
	return OffsetCoordinateToCube(pos)
}

// CubeToOffset converts a cube coordinate to offset coordinate using this grid's settings
func (hg *HexGrid) CubeToOffset(cube CubeCoordinate) Position {
	return cube.ToOffsetCoordinate()
}

// GetCubeNeighbors returns the 6 cube coordinate neighbors of a position
func (hg *HexGrid) GetCubeNeighbors(pos Position) []CubeCoordinate {
	cube := OffsetCoordinateToCube(pos)
	return cube.GetNeighbors()
}

// lerpCube performs linear interpolation between two cube coordinates
func (hg *HexGrid) lerpCube(from, to CubeCoordinate, t float64) CubeCoordinate {
	return CubeCoordinate{
		X: int(float64(from.X) + t*float64(to.X-from.X)),
		Y: int(float64(from.Y) + t*float64(to.Y-from.Y)),
		Z: int(float64(from.Z) + t*float64(to.Z-from.Z)),
	}
}

// roundCube rounds a cube coordinate to the nearest valid hex
func (hg *HexGrid) roundCube(cube CubeCoordinate) CubeCoordinate {
	rx := math.Round(float64(cube.X))
	ry := math.Round(float64(cube.Y))
	rz := math.Round(float64(cube.Z))

	xDiff := math.Abs(rx - float64(cube.X))
	yDiff := math.Abs(ry - float64(cube.Y))
	zDiff := math.Abs(rz - float64(cube.Z))

	switch {
	case xDiff > yDiff && xDiff > zDiff:
		rx = -ry - rz
	case yDiff > zDiff:
		ry = -rx - rz
	default:
		rz = -rx - ry
	}

	return CubeCoordinate{X: int(rx), Y: int(ry), Z: int(rz)}
}
