package spatial

import (
	"math"
)

// HexGrid implements a hexagonal grid system using cube coordinates
type HexGrid struct {
	dimensions  Dimensions
	orientation HexOrientation
}

// HexGridConfig holds configuration for creating a hex grid
type HexGridConfig struct {
	Width       float64
	Height      float64
	PointyTop   bool           // Deprecated: use Orientation instead. true for pointy-top, false for flat-top
	Orientation HexOrientation // The hex orientation (pointy-top or flat-top)
}

// NewHexGrid creates a new hex grid with the given dimensions
// Defaults to pointy-top orientation for D&D 5e compatibility
func NewHexGrid(config HexGridConfig) *HexGrid {
	// Use the Orientation field directly - it defaults to pointy-top (zero value)
	// The legacy PointyTop field is ignored in favor of the clearer Orientation field
	// Callers using the old API should migrate to: Orientation: HexOrientationFlatTop
	return &HexGrid{
		dimensions: Dimensions{
			Width:  config.Width,
			Height: config.Height,
		},
		orientation: config.Orientation,
	}
}

// GetShape returns the grid shape type
func (hg *HexGrid) GetShape() GridShape {
	return GridShapeHex
}

// GetOrientation returns the hex grid orientation (pointy-top or flat-top)
func (hg *HexGrid) GetOrientation() HexOrientation {
	return hg.orientation
}

// IsPointyTop returns true if the grid uses pointy-top orientation
func (hg *HexGrid) IsPointyTop() bool {
	return hg.orientation == HexOrientationPointyTop
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
// Position is interpreted as native cube coordinates: X = cube.x, Y = cube.z
func (hg *HexGrid) Distance(from, to Position) float64 {
	fromCube := hg.positionToCube(from)
	toCube := hg.positionToCube(to)
	return float64(fromCube.Distance(toCube))
}

// GetNeighbors returns all 6 adjacent positions in hex grid
func (hg *HexGrid) GetNeighbors(pos Position) []Position {
	cube := hg.positionToCube(pos)
	neighborCubes := cube.GetNeighbors()

	neighbors := make([]Position, 0, 6)
	for _, neighborCube := range neighborCubes {
		neighborPos := hg.cubeToPosition(neighborCube)
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

	fromCube := hg.positionToCube(from)
	toCube := hg.positionToCube(to)

	distance := fromCube.Distance(toCube)
	positions := make([]Position, 0, distance+1)

	for i := 0; i <= distance; i++ {
		t := float64(i) / float64(distance)
		lerpedCube := hg.lerpCube(fromCube, toCube, t)
		roundedCube := hg.roundCube(lerpedCube)
		pos := hg.cubeToPosition(roundedCube)

		if hg.IsValidPosition(pos) {
			positions = append(positions, pos)
		}
	}

	return positions
}

// GetPositionsInRange returns all positions within a given range using hex distance
func (hg *HexGrid) GetPositionsInRange(center Position, radius float64) []Position {
	positions := make([]Position, 0)
	centerCube := hg.positionToCube(center)

	// Calculate bounding box in cube coordinates
	iRadius := int(radius)
	for x := -iRadius; x <= iRadius; x++ {
		for y := math.Max(float64(-iRadius), float64(-x-iRadius)); y <= math.Min(float64(iRadius), float64(-x+iRadius)); y++ {
			z := -x - int(y)
			cube := CubeCoordinate{X: centerCube.X + x, Y: centerCube.Y + int(y), Z: centerCube.Z + z}

			if cube.Distance(centerCube) <= iRadius {
				pos := hg.cubeToPosition(cube)
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
	centerCube := hg.positionToCube(center)

	// Iterate over native cube coordinates (x, z) in the bounding box
	for x := int(center.X) - radius; x <= int(center.X)+radius; x++ {
		for z := int(center.Y) - radius; z <= int(center.Y)+radius; z++ {
			pos := Position{X: float64(x), Y: float64(z)}
			if hg.IsValidPosition(pos) {
				posCube := hg.positionToCube(pos)
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

// PositionToCube converts a Position to CubeCoordinate
// Position is interpreted as native cube: X = cube.x, Y = cube.z, cube.y is derived
func (hg *HexGrid) PositionToCube(pos Position) CubeCoordinate {
	return hg.positionToCube(pos)
}

// CubeToPosition converts a CubeCoordinate to Position
// Returns Position where X = cube.x, Y = cube.z
func (hg *HexGrid) CubeToPosition(cube CubeCoordinate) Position {
	return hg.cubeToPosition(cube)
}

// GetCubeNeighbors returns the 6 cube coordinate neighbors of a position
func (hg *HexGrid) GetCubeNeighbors(pos Position) []CubeCoordinate {
	cube := hg.positionToCube(pos)
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

// positionToCube converts a Position to CubeCoordinate using native cube interpretation
// Position.X = cube.x, Position.Y = cube.z, cube.y is derived as -x - z
func (hg *HexGrid) positionToCube(pos Position) CubeCoordinate {
	x := int(pos.X)
	z := int(pos.Y) // Position.Y stores cube.z
	y := -x - z     // Derived from cube constraint: x + y + z = 0
	return CubeCoordinate{X: x, Y: y, Z: z}
}

// cubeToPosition converts a CubeCoordinate to Position using native cube interpretation
// Returns Position where X = cube.x, Y = cube.z (cube.y is not stored, it's derived)
func (hg *HexGrid) cubeToPosition(cube CubeCoordinate) Position {
	return Position{X: float64(cube.X), Y: float64(cube.Z)}
}
