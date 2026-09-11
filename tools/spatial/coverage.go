package spatial

import (
	"errors"
	"math"
)

// Box is a rectangular footprint: W across the bearing it is drawn at, D along
// it. Both are in the caller's unit, the same one the embedding's cell width
// was measured in.
type Box struct {
	W float64 `json:"w"`
	D float64 `json:"d"`
}

// Footprint is a shape in the plane. Today a box is the only one; a polygon
// joins it when something in the game is shaped like one.
type Footprint struct {
	Box *Box `json:"box,omitempty"`
}

// AnchorRule says where a footprint sits relative to the cell it is placed at.
type AnchorRule int

const (
	// AnchorAtCentre puts the footprint's centre on the anchor cell's centre.
	// The anchor cell is under the footprint.
	AnchorAtCentre AnchorRule = iota

	// AnchorAtEdge puts the footprint's near edge on the anchor cell's
	// boundary along Facing, so the anchor cell is never under it and the
	// footprint's depth is measured from that boundary outward.
	AnchorAtEdge
)

// CoverageInput places a footprint on the plane: which cell it is anchored at,
// which way it faces, and whether it sits on that cell or in front of it.
//
// Facing is in degrees, counter-clockwise from east, as HexEmbedding.Bearing
// reports it. Any angle is legal: nothing here snaps to a grid axis.
type CoverageInput struct {
	Footprint Footprint
	At        Position
	Facing    float64
	Anchor    AnchorRule
}

// CoverageOutput is which cells the footprint lies on, and how much of each.
//
// Cells holds the fraction of each cell's area under the footprint, in (0, 1].
// A cell the footprint misses is absent rather than zero. Edges — the cell
// boundaries the footprint's outline crosses, which is how a thin thing that
// covers under half of everything still blocks something — arrive with the
// first thin prop that needs them.
type CoverageOutput struct {
	Cells map[Position]float64
}

// ErrNoFootprint reports coverage asked for with no shape to rasterise.
var ErrNoFootprint = errors.New("spatial: coverage needs a footprint")

// ErrBadFootprint reports a footprint whose sides are not positive lengths.
var ErrBadFootprint = errors.New("spatial: footprint sides must be positive")

// coverageEpsilon is the fraction below which an overlap is arithmetic noise
// rather than area. A footprint whose edge lies exactly on a cell's boundary
// clips to a degenerate sliver, and a sliver a billionth of a cell wide is the
// rounding in a cosine, not coverage.
const coverageEpsilon = 1e-9

// Coverage rasterises a footprint at a transform onto a grid, returning the
// fraction of each cell's area under it.
//
// Fractions come out; thresholds go in above. What a covered cell means —
// caught by the blast, blocked, difficult — is the game's, and a rule that
// counts a cell at half would be a different number in a different rulebook
// without this changing. Only cells the grid considers valid are reported.
//
// Returns ErrNoFootprint when no shape was given, ErrBadFootprint when its
// sides are not positive, and ErrBadCellWidth when the embedding has no frame.
func Coverage(emb HexEmbedding, g Grid, in CoverageInput) (CoverageOutput, error) {
	if in.Footprint.Box == nil {
		return CoverageOutput{}, ErrNoFootprint
	}
	b := *in.Footprint.Box
	if b.W <= 0 || b.D <= 0 {
		return CoverageOutput{}, ErrBadFootprint
	}
	if emb.cellWidth <= 0 {
		return CoverageOutput{}, ErrBadCellWidth
	}

	rect := boxPolygon(emb, in, b)
	radius := math.Ceil((b.D+b.W/2)/emb.cellWidth) + 1

	out := CoverageOutput{Cells: map[Position]float64{}}
	for _, cell := range g.GetPositionsInRange(in.At, radius) {
		hex := emb.CellCorners(cell)
		whole := polygonArea(hex[:])
		if whole <= 0 {
			continue
		}
		clipped := clipConvex(hex[:], rect)
		f := polygonArea(clipped) / whole
		if f > coverageEpsilon {
			out.Cells[cell] = math.Min(f, 1)
		}
	}

	return out, nil
}

// boxPolygon is the footprint's four corners in the plane, in boundary order.
//
// The near edge sits on the anchor cell's boundary for AnchorAtEdge — half a
// cell width along the bearing, which is the inradius — and the box is centred
// on the cell for AnchorAtCentre.
func boxPolygon(emb HexEmbedding, in CoverageInput, b Box) []Point {
	rad := in.Facing * math.Pi / 180
	ux, uy := math.Cos(rad), math.Sin(rad)
	nx, ny := -uy, ux

	c := emb.CellCentre(in.At)
	near, far := -b.D/2, b.D/2
	if in.Anchor == AnchorAtEdge {
		near, far = emb.cellWidth/2, emb.cellWidth/2+b.D
	}
	half := b.W / 2

	at := func(along, across float64) Point {
		return Point{X: c.X + ux*along + nx*across, Y: c.Y + uy*along + ny*across}
	}

	return []Point{at(near, -half), at(far, -half), at(far, half), at(near, half)}
}

// clipConvex is the part of subject inside the convex clipper, both given in
// boundary order. Sutherland–Hodgman against each of the clipper's edges.
func clipConvex(subject, clipper []Point) []Point {
	if len(clipper) < 3 {
		return nil
	}
	inside := centroid(clipper)
	out := subject
	for i := range clipper {
		a, b := clipper[i], clipper[(i+1)%len(clipper)]
		nx, ny, c := nearHalfPlane(a, b, inside)
		out = clipHalfPlane(out, nx, ny, c)
		if len(out) < 3 {
			return nil
		}
	}

	return out
}

// centroid is the average of a polygon's vertices — inside any convex one.
func centroid(poly []Point) Point {
	var sx, sy float64
	for _, p := range poly {
		sx += p.X
		sy += p.Y
	}
	n := float64(len(poly))

	return Point{X: sx / n, Y: sy / n}
}

// nearHalfPlane is the half-plane {p : n·p <= c} bounded by the line through
// a and b and containing near.
func nearHalfPlane(a, b, near Point) (nx, ny, c float64) {
	nx, ny = -(b.Y - a.Y), b.X-a.X
	c = nx*a.X + ny*a.Y
	if nx*near.X+ny*near.Y > c {
		return -nx, -ny, -c
	}

	return nx, ny, c
}

// clipHalfPlane is Sutherland–Hodgman against one half-plane {p : n·p <= c}.
//
// The polygon is convex and the clipper is a half-plane, so the result is
// convex and the algorithm's degenerate cases (a concave polygon re-entering
// the clip region) cannot arise. A polygon wholly outside comes back empty.
func clipHalfPlane(poly []Point, nx, ny, c float64) []Point {
	if len(poly) == 0 {
		return nil
	}
	out := make([]Point, 0, len(poly)+1)
	for i, cur := range poly {
		prev := poly[(i+len(poly)-1)%len(poly)]
		dCur := nx*cur.X + ny*cur.Y - c
		dPrev := nx*prev.X + ny*prev.Y - c
		if dCur <= 0 {
			if dPrev > 0 {
				out = append(out, lerpTo(prev, cur, dPrev/(dPrev-dCur)))
			}
			out = append(out, cur)
			continue
		}
		if dPrev <= 0 {
			out = append(out, lerpTo(prev, cur, dPrev/(dPrev-dCur)))
		}
	}

	return out
}

// lerpTo is the point t of the way from a to b.
func lerpTo(a, b Point, t float64) Point {
	return Point{X: a.X + (b.X-a.X)*t, Y: a.Y + (b.Y-a.Y)*t}
}

// polygonArea is the shoelace area of a simple polygon, sign discarded so
// winding does not matter.
func polygonArea(poly []Point) float64 {
	if len(poly) < 3 {
		return 0
	}
	var twice float64
	for i, cur := range poly {
		next := poly[(i+1)%len(poly)]
		twice += cur.X*next.Y - next.X*cur.Y
	}

	return math.Abs(twice) / 2
}
