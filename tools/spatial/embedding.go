package spatial

import (
	"errors"
	"math"
)

// Point is a location in the plane, in whatever unit the caller measured
// CellWidth in. Position addresses a cell; Point addresses the continuous
// space that cell occupies, and only geometry that needs an area or an angle
// ever leaves the one for the other.
type Point struct {
	X float64 `json:"x"`
	Y float64 `json:"y"`
}

// ErrBadCellWidth reports a cell width that is not a positive length. A hex
// with no width has no corners and no centre, so an embedding built from one
// answers nothing rather than answering something plausible.
var ErrBadCellWidth = errors.New("spatial: cell width must be positive")

// HexEmbeddingConfig describes one hex grid's place in the plane.
//
// CellWidth is measured ACROSS THE FLATS — the distance between a hex's two
// parallel sides, which is also the distance between the centres of two
// neighbouring cells. It is in the caller's own unit: spatial holds no notion
// of feet, so a rulebook that calls a cell five feet passes 5 and reads
// everything back in feet.
type HexEmbeddingConfig struct {
	// Orientation is which way the hexes are turned. The same lengths hold
	// under both: a flat-top hex is a pointy-top hex rotated thirty degrees.
	Orientation HexOrientation

	// CellWidth is the across-the-flats width of one cell, in the caller's
	// unit. Must be positive.
	CellWidth float64
}

// Validate reports whether the config describes a usable plane.
// Returns ErrBadCellWidth when CellWidth is not positive.
func (c HexEmbeddingConfig) Validate() error {
	if c.CellWidth <= 0 {
		return ErrBadCellWidth
	}

	return nil
}

// HexEmbedding is a hex grid's embedding in the plane: where a cell's centre
// sits, where its six corners are, and the bearing from one cell to another.
//
// It is the frame every areal question is answered in. Distance and adjacency
// are answered in axial coordinates, which are orientation-free and unitless;
// coverage, angles and areas are not, and this is what they need.
//
// The zero value has no frame and answers zero points and no bearings. Build
// one with NewHexEmbedding, and check the config with
// HexEmbeddingConfig.Validate first if the width came from outside.
type HexEmbedding struct {
	// cellWidth is across the flats, in the caller's unit.
	cellWidth float64

	// qx, qy and rx, ry are the plane vectors one axial step moves: a cell at
	// axial (q, r) sits at q*(qx,qy) + r*(rx,ry). Position.X is Q and
	// Position.Y is R, matching AxialHexGrid.
	qx, qy, rx, ry float64

	// corner is a hex's six corners relative to its own centre, in boundary
	// order.
	corner [6]Point
}

// NewHexEmbedding builds the embedding for one orientation at one cell width.
//
// X runs east and Y runs south, the screen's axes and the axes the authored
// grids already run on. A cell's circumradius — the distance from its centre
// to a corner — is CellWidth/sqrt(3) under both orientations, because across
// the flats is sqrt(3) circumradii either way.
//
// A CellWidth that is not positive yields an embedding with no frame, whose
// methods return zero points and report no bearing. That is deliberate: the
// alternative is a panic in the middle of a raster, or a silent frame of some
// invented size. Callers that take a width from content validate the config.
func NewHexEmbedding(c HexEmbeddingConfig) HexEmbedding {
	if c.Validate() != nil {
		return HexEmbedding{}
	}

	// circumradius: across the flats is sqrt(3) circumradii.
	r := c.CellWidth / math.Sqrt(3)

	e := HexEmbedding{cellWidth: c.CellWidth}
	cornerPhase := math.Pi / 6 // pointy-top: corners north and south
	if c.Orientation == HexOrientationFlatTop {
		// The same hex turned thirty degrees: wider than it is tall, and one
		// axial step along Q moves diagonally rather than due east.
		e.qx, e.qy = 1.5*r, math.Sqrt(3)/2*r
		e.rx, e.ry = 0, math.Sqrt(3)*r
		cornerPhase = 0
	} else {
		e.qx, e.qy = math.Sqrt(3)*r, 0
		e.rx, e.ry = math.Sqrt(3)/2*r, 1.5*r
	}
	for i := range e.corner {
		a := float64(i)*math.Pi/3 + cornerPhase
		e.corner[i] = Point{X: r * math.Cos(a), Y: r * math.Sin(a)}
	}

	return e
}

// CellCentre is the point at the middle of a cell.
func (e HexEmbedding) CellCentre(cell Position) Point {
	return Point{
		X: cell.X*e.qx + cell.Y*e.rx,
		Y: cell.X*e.qy + cell.Y*e.ry,
	}
}

// CellCorners is a cell's six corners in the plane, in boundary order.
// The polygon is convex and closed by its first point.
func (e HexEmbedding) CellCorners(cell Position) [6]Point {
	c := e.CellCentre(cell)
	var out [6]Point
	for i, k := range e.corner {
		out[i] = Point{X: c.X + k.X, Y: c.Y + k.Y}
	}

	return out
}

// Bearing reports the direction from one cell's centre to another's, in
// degrees within [0, 360), measured counter-clockwise from east.
//
// Reports false when the two cells are the same, and when the embedding has
// no frame: neither has a direction at all, and a caller that wants to say
// "toward yourself" words that refusal for itself.
func (e HexEmbedding) Bearing(from, to Position) (degrees float64, ok bool) {
	a, b := e.CellCentre(from), e.CellCentre(to)
	dx, dy := b.X-a.X, b.Y-a.Y
	if dx == 0 && dy == 0 {
		return 0, false
	}
	deg := math.Atan2(dy, dx) * 180 / math.Pi

	return math.Mod(math.Mod(deg, 360)+360, 360), true
}
