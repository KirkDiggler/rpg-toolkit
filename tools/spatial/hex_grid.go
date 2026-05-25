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
// Converts to cube coordinates and uses hex distance formula
func (hg *HexGrid) Distance(from, to Position) float64 {
	fromCube := OffsetCoordinateToCubeWithOrientation(from, hg.orientation)
	toCube := OffsetCoordinateToCubeWithOrientation(to, hg.orientation)
	return float64(fromCube.Distance(toCube))
}

// GetNeighbors returns all 6 adjacent positions in hex grid
func (hg *HexGrid) GetNeighbors(pos Position) []Position {
	cube := OffsetCoordinateToCubeWithOrientation(pos, hg.orientation)
	neighborCubes := cube.GetNeighbors()

	neighbors := make([]Position, 0, 6)
	for _, neighborCube := range neighborCubes {
		neighborPos := neighborCube.ToOffsetCoordinateWithOrientation(hg.orientation)
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

	fromCube := OffsetCoordinateToCubeWithOrientation(from, hg.orientation)
	toCube := OffsetCoordinateToCubeWithOrientation(to, hg.orientation)

	distance := fromCube.Distance(toCube)
	positions := make([]Position, 0, distance+1)

	for i := 0; i <= distance; i++ {
		t := float64(i) / float64(distance)
		lerpedCube := hg.lerpCube(fromCube, toCube, t)
		roundedCube := hg.roundCube(lerpedCube)
		pos := roundedCube.ToOffsetCoordinateWithOrientation(hg.orientation)

		if hg.IsValidPosition(pos) {
			positions = append(positions, pos)
		}
	}

	return positions
}

// GetPositionsInRange returns all positions within a given range using hex distance
func (hg *HexGrid) GetPositionsInRange(center Position, radius float64) []Position {
	positions := make([]Position, 0)
	centerCube := OffsetCoordinateToCubeWithOrientation(center, hg.orientation)

	// Calculate bounding box in cube coordinates
	iRadius := int(radius)
	for x := -iRadius; x <= iRadius; x++ {
		for y := math.Max(float64(-iRadius), float64(-x-iRadius)); y <= math.Min(float64(iRadius), float64(-x+iRadius)); y++ {
			z := -x - int(y)
			cube := CubeCoordinate{X: centerCube.X + x, Y: centerCube.Y + int(y), Z: centerCube.Z + z}

			if cube.Distance(centerCube) <= iRadius {
				pos := cube.ToOffsetCoordinateWithOrientation(hg.orientation)
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
	_ = OffsetCoordinateToCubeWithOrientation(origin, hg.orientation)

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
	centerCube := OffsetCoordinateToCubeWithOrientation(center, hg.orientation)

	// Simple approach: get all positions at exactly the specified distance
	for x := int(center.X) - radius; x <= int(center.X)+radius; x++ {
		for y := int(center.Y) - radius; y <= int(center.Y)+radius; y++ {
			pos := Position{X: float64(x), Y: float64(y)}
			if hg.IsValidPosition(pos) {
				posCube := OffsetCoordinateToCubeWithOrientation(pos, hg.orientation)
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

// OffsetToCube converts an offset coordinate to cube coordinate using this grid's orientation
func (hg *HexGrid) OffsetToCube(pos Position) CubeCoordinate {
	return OffsetCoordinateToCubeWithOrientation(pos, hg.orientation)
}

// CubeToOffset converts a cube coordinate to offset coordinate using this grid's orientation
func (hg *HexGrid) CubeToOffset(cube CubeCoordinate) Position {
	return cube.ToOffsetCoordinateWithOrientation(hg.orientation)
}

// GetCubeNeighbors returns the 6 cube coordinate neighbors of a position
func (hg *HexGrid) GetCubeNeighbors(pos Position) []CubeCoordinate {
	cube := OffsetCoordinateToCubeWithOrientation(pos, hg.orientation)
	return cube.GetNeighbors()
}

// AxialHexGrid implements Grid for hexagonal maps that store positions in axial
// coordinates: Position.X = Q (column), Position.Y = R (row), S = -(Q+R).
//
// This is the correct grid for encounter SDKs and tactical maps that use axial
// hex coordinates natively (e.g. rpg-api's encountercore.Hex). The existing
// HexGrid expects offset coordinates and would produce wrong distances when fed
// axial positions — AxialHexGrid eliminates that mismatch.
//
// Distance uses the cube formula: (|ΔQ| + |ΔR| + |ΔS|) / 2, which correctly
// counts adjacent hexes as distance 1 regardless of their direction relative to
// the X/Y axes. This matches D&D 5e adjacency rules for hex maps.
type AxialHexGrid struct {
	dimensions Dimensions
}

// AxialHexGridConfig holds configuration for creating an AxialHexGrid.
// Width and Height set the bounding box for IsValidPosition checks; pass
// large values (e.g. 1000×1000) for encounter rooms where position bounds
// are not meaningful.
type AxialHexGridConfig struct {
	Width  float64
	Height float64
}

// NewAxialHexGrid creates a new hex grid that treats Position.X/Y as axial
// Q/R coordinates. Use this instead of NewHexGrid when positions are stored
// in axial (not offset) form.
func NewAxialHexGrid(config AxialHexGridConfig) *AxialHexGrid {
	return &AxialHexGrid{
		dimensions: Dimensions(config),
	}
}

// GetShape returns GridShapeHex.
func (a *AxialHexGrid) GetShape() GridShape {
	return GridShapeHex
}

// GetDimensions returns the configured bounding-box dimensions.
func (a *AxialHexGrid) GetDimensions() Dimensions {
	return a.dimensions
}

// IsValidPosition reports whether the position falls within the configured
// bounding box. Axial Q/R can be negative, so the check is symmetric around
// the origin: |X| < Width/2 and |Y| < Height/2. Pass Width=Height=1000 (or
// similar) when you do not want meaningful bounds.
func (a *AxialHexGrid) IsValidPosition(pos Position) bool {
	half := a.dimensions.Width / 2
	halfH := a.dimensions.Height / 2
	return pos.X >= -half && pos.X < half && pos.Y >= -halfH && pos.Y < halfH
}

// Distance returns the hex distance between two axial positions.
// Interprets Position.X as Q and Position.Y as R; derives S = -(Q+R).
// Uses the cube formula: (|ΔQ| + |ΔR| + |ΔS|) / 2.
func (a *AxialHexGrid) Distance(from, to Position) float64 {
	fromCube := axialToCube(from)
	toCube := axialToCube(to)
	return float64(fromCube.Distance(toCube))
}

// GetNeighbors returns the 6 axial positions adjacent to pos.
func (a *AxialHexGrid) GetNeighbors(pos Position) []Position {
	cube := axialToCube(pos)
	neighborCubes := cube.GetNeighbors()
	neighbors := make([]Position, 0, 6)
	for _, nc := range neighborCubes {
		np := cubeToAxial(nc)
		if a.IsValidPosition(np) {
			neighbors = append(neighbors, np)
		}
	}
	return neighbors
}

// IsAdjacent reports whether two positions are adjacent (hex distance == 1).
func (a *AxialHexGrid) IsAdjacent(pos1, pos2 Position) bool {
	return a.Distance(pos1, pos2) <= 1
}

// GetLineOfSight returns hex positions along the line from from to to using
// cube-coordinate linear interpolation (standard hex line algorithm).
func (a *AxialHexGrid) GetLineOfSight(from, to Position) []Position {
	if from.Equals(to) {
		return []Position{from}
	}
	fromCube := axialToCube(from)
	toCube := axialToCube(to)
	dist := fromCube.Distance(toCube)
	positions := make([]Position, 0, dist+1)
	for i := 0; i <= dist; i++ {
		t := float64(i) / float64(dist)
		lerped := lerpCubeAxial(fromCube, toCube, t)
		rounded := roundCubeAxial(lerped)
		pos := cubeToAxial(rounded)
		if a.IsValidPosition(pos) {
			positions = append(positions, pos)
		}
	}
	return positions
}

// GetPositionsInRange returns all valid axial positions within radius hex
// steps of center.
func (a *AxialHexGrid) GetPositionsInRange(center Position, radius float64) []Position {
	centerCube := axialToCube(center)
	iRadius := int(radius)
	positions := make([]Position, 0)
	for dq := -iRadius; dq <= iRadius; dq++ {
		rMin := intMax(-iRadius, -dq-iRadius)
		rMax := intMin(iRadius, -dq+iRadius)
		for dr := rMin; dr <= rMax; dr++ {
			ds := -dq - dr
			candidate := CubeCoordinate{X: centerCube.X + dq, Y: centerCube.Y + dr, Z: centerCube.Z + ds}
			pos := cubeToAxial(candidate)
			if a.IsValidPosition(pos) {
				positions = append(positions, pos)
			}
		}
	}
	return positions
}

// GetPositionsInRectangle returns all valid axial positions within the given
// rectangle (treating X as Q and Y as R for the bounding box).
func (a *AxialHexGrid) GetPositionsInRectangle(rect Rectangle) []Position {
	positions := make([]Position, 0)
	minQ := rect.Position.X
	maxQ := rect.Position.X + rect.Dimensions.Width - 1
	minR := rect.Position.Y
	maxR := rect.Position.Y + rect.Dimensions.Height - 1
	for q := minQ; q <= maxQ; q++ {
		for r := minR; r <= maxR; r++ {
			pos := Position{X: q, Y: r}
			if a.IsValidPosition(pos) {
				positions = append(positions, pos)
			}
		}
	}
	return positions
}

// GetPositionsInCircle returns all valid axial positions within the given circle.
func (a *AxialHexGrid) GetPositionsInCircle(circle Circle) []Position {
	return a.GetPositionsInRange(circle.Center, circle.Radius)
}

// GetPositionsInLine returns the positions along a line from from to to.
func (a *AxialHexGrid) GetPositionsInLine(from, to Position) []Position {
	return a.GetLineOfSight(from, to)
}

// GetPositionsInCone returns axial positions within a cone.
func (a *AxialHexGrid) GetPositionsInCone(origin, direction Position, length, angle float64) []Position {
	dirLength := math.Sqrt(direction.X*direction.X + direction.Y*direction.Y)
	if dirLength == 0 {
		return nil
	}
	dirX := direction.X / dirLength
	dirY := direction.Y / dirLength
	candidates := a.GetPositionsInRange(origin, length)
	positions := make([]Position, 0, len(candidates))
	for _, pos := range candidates {
		if pos.Equals(origin) {
			positions = append(positions, pos)
			continue
		}
		posX := pos.X - origin.X
		posY := pos.Y - origin.Y
		posLen := math.Sqrt(posX*posX + posY*posY)
		if posLen == 0 {
			continue
		}
		dot := (dirX*posX + dirY*posY) / posLen
		if math.Acos(math.Max(-1, math.Min(1, dot))) <= angle/2 {
			positions = append(positions, pos)
		}
	}
	return positions
}

// axialToCube converts an axial Position (X=Q, Y=R) to a CubeCoordinate.
func axialToCube(pos Position) CubeCoordinate {
	q := int(pos.X)
	r := int(pos.Y)
	return CubeCoordinate{X: q, Y: r, Z: -q - r}
}

// cubeToAxial converts a CubeCoordinate back to an axial Position (X=Q, Y=R).
func cubeToAxial(c CubeCoordinate) Position {
	return Position{X: float64(c.X), Y: float64(c.Y)}
}

// lerpCubeAxial linearly interpolates between two CubeCoordinates.
// Uses float arithmetic before rounding so the cube invariant is preserved.
func lerpCubeAxial(from, to CubeCoordinate, t float64) cubeFloat {
	return cubeFloat{
		x: float64(from.X) + t*float64(to.X-from.X),
		y: float64(from.Y) + t*float64(to.Y-from.Y),
		z: float64(from.Z) + t*float64(to.Z-from.Z),
	}
}

// cubeFloat is a floating-point cube coordinate used only during line
// interpolation to avoid premature integer truncation.
type cubeFloat struct{ x, y, z float64 }

// roundCubeAxial rounds a floating-point cube to the nearest valid hex,
// preserving the cube constraint x+y+z=0.
func roundCubeAxial(c cubeFloat) CubeCoordinate {
	rx := math.Round(c.x)
	ry := math.Round(c.y)
	rz := math.Round(c.z)
	xDiff := math.Abs(rx - c.x)
	yDiff := math.Abs(ry - c.y)
	zDiff := math.Abs(rz - c.z)
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

// intMax returns the larger of two ints.
func intMax(a, b int) int {
	if a > b {
		return a
	}
	return b
}

// intMin returns the smaller of two ints.
func intMin(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// compile-time check: AxialHexGrid satisfies the Grid interface.
var _ Grid = (*AxialHexGrid)(nil)

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
