package spatial

import (
	"fmt"
	"math"
)

// Position represents a spatial position in 2D space
// NOTE: Distance calculations are grid-dependent and handled by Grid implementations
type Position struct {
	X float64 `json:"x"`
	Y float64 `json:"y"`
}

// String returns a string representation of the position
func (p Position) String() string {
	return fmt.Sprintf("(%g, %g)", p.X, p.Y)
}

// Equals checks if two positions are equal
func (p Position) Equals(other Position) bool {
	return p.X == other.X && p.Y == other.Y
}

// Add adds another position to this position
func (p Position) Add(other Position) Position {
	return Position{X: p.X + other.X, Y: p.Y + other.Y}
}

// Subtract subtracts another position from this position
func (p Position) Subtract(other Position) Position {
	return Position{X: p.X - other.X, Y: p.Y - other.Y}
}

// Scale scales the position by a factor
func (p Position) Scale(factor float64) Position {
	return Position{X: p.X * factor, Y: p.Y * factor}
}

// Normalize returns a normalized version of the position (for vector math)
func (p Position) Normalize() Position {
	length := math.Sqrt(p.X*p.X + p.Y*p.Y)
	if length == 0 {
		return Position{X: 0, Y: 0}
	}
	return Position{X: p.X / length, Y: p.Y / length}
}

// IsZero checks if the position is at the origin
func (p Position) IsZero() bool {
	return p.X == 0 && p.Y == 0
}

// Dimensions represents the size of a spatial area
type Dimensions struct {
	Width  float64 `json:"width"`
	Height float64 `json:"height"`
}

// String returns a string representation of the dimensions
func (d Dimensions) String() string {
	return fmt.Sprintf("%gx%g", d.Width, d.Height)
}

// Area calculates the area of the dimensions
func (d Dimensions) Area() float64 {
	return d.Width * d.Height
}

// Contains checks if a position is within the dimensions (assuming origin at 0,0)
func (d Dimensions) Contains(pos Position) bool {
	return pos.X >= 0 && pos.X < d.Width && pos.Y >= 0 && pos.Y < d.Height
}

// HexOrientation represents the orientation of a hexagonal grid
type HexOrientation int

const (
	// HexOrientationPointyTop is the default orientation where hexes have a pointed top
	// This is the standard D&D 5e hex grid orientation
	HexOrientationPointyTop HexOrientation = iota
	// HexOrientationFlatTop is an alternative orientation where hexes have a flat top
	HexOrientationFlatTop
)

// String returns the string representation of the hex orientation
func (o HexOrientation) String() string {
	switch o {
	case HexOrientationPointyTop:
		return "pointy-top"
	case HexOrientationFlatTop:
		return "flat-top"
	default:
		return "unknown"
	}
}

// CubeCoordinate represents a position in cube coordinate system (for hex grids)
// In hex grids, cube coordinates simplify distance and neighbor calculations
type CubeCoordinate struct {
	X int `json:"x"`
	Y int `json:"y"`
	Z int `json:"z"`
}

// String returns a string representation of the cube coordinate
func (c CubeCoordinate) String() string {
	return fmt.Sprintf("(%d, %d, %d)", c.X, c.Y, c.Z)
}

// Equals checks if two cube coordinates are equal
func (c CubeCoordinate) Equals(other CubeCoordinate) bool {
	return c.X == other.X && c.Y == other.Y && c.Z == other.Z
}

// IsValid checks if the cube coordinate is valid (x + y + z == 0)
func (c CubeCoordinate) IsValid() bool {
	return c.X+c.Y+c.Z == 0
}

// Distance calculates the hex distance between two cube coordinates
// This is specific to hex grids and uses cube coordinate math
func (c CubeCoordinate) Distance(other CubeCoordinate) int {
	return (abs(c.X-other.X) + abs(c.Y-other.Y) + abs(c.Z-other.Z)) / 2
}

// Add adds another cube coordinate to this one
func (c CubeCoordinate) Add(other CubeCoordinate) CubeCoordinate {
	return CubeCoordinate{
		X: c.X + other.X,
		Y: c.Y + other.Y,
		Z: c.Z + other.Z,
	}
}

// Subtract subtracts another cube coordinate from this one
func (c CubeCoordinate) Subtract(other CubeCoordinate) CubeCoordinate {
	return CubeCoordinate{
		X: c.X - other.X,
		Y: c.Y - other.Y,
		Z: c.Z - other.Z,
	}
}

// Scale scales the cube coordinate by a factor
func (c CubeCoordinate) Scale(factor int) CubeCoordinate {
	return CubeCoordinate{
		X: c.X * factor,
		Y: c.Y * factor,
		Z: c.Z * factor,
	}
}

// GetNeighbors returns all 6 neighboring cube coordinates
func (c CubeCoordinate) GetNeighbors() []CubeCoordinate {
	directions := []CubeCoordinate{
		{1, -1, 0}, {1, 0, -1}, {0, 1, -1},
		{-1, 1, 0}, {-1, 0, 1}, {0, -1, 1},
	}

	neighbors := make([]CubeCoordinate, 6)
	for i, dir := range directions {
		neighbors[i] = c.Add(dir)
	}
	return neighbors
}

// ToOffsetCoordinate converts cube coordinate to offset coordinate (for display)
// Uses pointy-top orientation by default. Use ToOffsetCoordinateWithOrientation for flat-top.
func (c CubeCoordinate) ToOffsetCoordinate() Position {
	return c.ToOffsetCoordinateWithOrientation(HexOrientationPointyTop)
}

// ToOffsetCoordinateWithOrientation converts cube coordinate to offset coordinate
// using the specified hex orientation
func (c CubeCoordinate) ToOffsetCoordinateWithOrientation(orientation HexOrientation) Position {
	switch orientation {
	case HexOrientationFlatTop:
		// Flat-top: odd-r offset coordinates
		col := float64(c.X + (c.Z-(c.Z&1))/2)
		row := float64(c.Z)
		return Position{X: col, Y: row}
	default:
		// Pointy-top: odd-q offset coordinates (default)
		col := float64(c.X)
		row := float64(c.Z + (c.X-(c.X&1))/2)
		return Position{X: col, Y: row}
	}
}

// OffsetCoordinateToCube converts offset coordinate to cube coordinate
// Uses pointy-top orientation by default. Use OffsetCoordinateToCubeWithOrientation for flat-top.
func OffsetCoordinateToCube(pos Position) CubeCoordinate {
	return OffsetCoordinateToCubeWithOrientation(pos, HexOrientationPointyTop)
}

// OffsetCoordinateToCubeWithOrientation converts offset coordinate to cube coordinate
// using the specified hex orientation
func OffsetCoordinateToCubeWithOrientation(pos Position, orientation HexOrientation) CubeCoordinate {
	col := int(pos.X)
	row := int(pos.Y)

	switch orientation {
	case HexOrientationFlatTop:
		// Flat-top: odd-r offset coordinates
		x := col - (row-(row&1))/2
		z := row
		y := -x - z
		return CubeCoordinate{X: x, Y: y, Z: z}
	default:
		// Pointy-top: odd-q offset coordinates (default)
		x := col
		z := row - (col-(col&1))/2
		y := -x - z
		return CubeCoordinate{X: x, Y: y, Z: z}
	}
}

// Rectangle represents a rectangular area
type Rectangle struct {
	Position   Position   `json:"position"`
	Dimensions Dimensions `json:"dimensions"`
}

// String returns a string representation of the rectangle
func (r Rectangle) String() string {
	return fmt.Sprintf("Rect[%s %s]", r.Position, r.Dimensions)
}

// Contains checks if a position is within the rectangle
func (r Rectangle) Contains(pos Position) bool {
	return pos.X >= r.Position.X && pos.X < r.Position.X+r.Dimensions.Width &&
		pos.Y >= r.Position.Y && pos.Y < r.Position.Y+r.Dimensions.Height
}

// Center returns the center position of the rectangle
func (r Rectangle) Center() Position {
	return Position{
		X: r.Position.X + r.Dimensions.Width/2,
		Y: r.Position.Y + r.Dimensions.Height/2,
	}
}

// Intersects checks if this rectangle intersects with another rectangle
func (r Rectangle) Intersects(other Rectangle) bool {
	return r.Position.X < other.Position.X+other.Dimensions.Width &&
		r.Position.X+r.Dimensions.Width > other.Position.X &&
		r.Position.Y < other.Position.Y+other.Dimensions.Height &&
		r.Position.Y+r.Dimensions.Height > other.Position.Y
}

// Circle represents a circular area
type Circle struct {
	Center Position `json:"center"`
	Radius float64  `json:"radius"`
}

// String returns a string representation of the circle
func (c Circle) String() string {
	return fmt.Sprintf("Circle[%s r:%g]", c.Center, c.Radius)
}

// Contains checks if a position is within the circle
func (c Circle) Contains(pos Position) bool {
	// Note: This uses Euclidean distance - actual distance rules depend on grid type
	dx := c.Center.X - pos.X
	dy := c.Center.Y - pos.Y
	return math.Sqrt(dx*dx+dy*dy) <= c.Radius
}

// Intersects checks if this circle intersects with another circle
func (c Circle) Intersects(other Circle) bool {
	// Note: This uses Euclidean distance - actual distance rules depend on grid type
	dx := c.Center.X - other.Center.X
	dy := c.Center.Y - other.Center.Y
	return math.Sqrt(dx*dx+dy*dy) <= c.Radius+other.Radius
}

// abs returns the absolute value of an integer
func abs(x int) int {
	if x < 0 {
		return -x
	}
	return x
}
