// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package dungeonspec

import (
	"math"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter"
	"github.com/KirkDiggler/rpg-toolkit/tools/spatial"
)

// geometry.go is THE TOOLKIT'S FIRST WORLD-UNIT GEOMETRY (rpg-project#360,
// wall-geometry design §4.2 C9).
//
// A wall is a straight line between two picked positions, and everything the
// engine runs on — which crossings it blocks, which cells it cuts, which of
// those cells still have room to stand in — is DERIVED from that line. Deriving
// it means embedding cells in the plane, which nothing in this workspace had
// ever needed: a hex grid answers distance and adjacency in cube coordinates,
// which are orientation-free and unitless, and every question before this one
// was answerable there.
//
// # Where it lives, and when it moves
//
// Here, package-private, and NOT in [tools/spatial], deliberately. There is
// exactly one customer today — the wall derivations in this file's package —
// and a shared package with one caller is a guess about the second one. The
// second customer is named and real: `tools/spatial/room.go`'s corner-rule
// sight test measures whether a sightline clears a hex corner and reimplements
// its own notion of where a corner is. When that test is rewritten against a
// real embedding, THIS moves to tools/spatial and both call it. Not before:
// promoting it now would publish an API shaped by one caller and freeze it
// before the caller that would have changed it arrives.
//
// # The frame
//
// One embedding, stated once so nothing re-derives it:
//
//   - CIRCUMRADIUS 1. A hex's corners are one unit from its centre. Every
//     length here is in those units, and no length ever leaves this file —
//     areas leave as RATIOS and positions leave as fractional axial.
//   - X EAST, Y SOUTH. The screen's axes, and the axes the authored offset
//     pair already runs on: row 1 is south of row 0.
//   - The BOUNDING BOX is the unit the file's offsets are written in — x in
//     widths, y in heights (design §3.3) — so the same seven numbers describe
//     the same seven points whatever the circumradius is.
//
// # What a position is, exactly
//
// A position is a cell and one of seven offsets: the six side midpoints and
// the centre. A side midpoint is HALF THE STEP TO THE NEIGHBOUR ACROSS THAT
// SIDE, which is why [hexGeom.axialAt] is exact rational arithmetic and never
// touches the embedding: the midpoint of the side between axial (q,r) and its
// neighbour (q+dq, r+dr) is axial (q+dq/2, r+dr/2), and every dq/2 is a half.
// The embedding is needed only for ANGLES and AREAS, which is what the rest of
// this file is.
//
// THAT SPLIT IS WHY A CORNER NEEDS NO TOLERANCE. Halves are exact in binary,
// so two walls written to end at the same position arrive at the far end
// carrying byte-identical endpoints — and two positions that name the same
// physical point from different cells (the midpoint of one hex's east side is
// the midpoint of its neighbour's west side) compare equal on the nose. If a
// position had been rounded through the embedding instead, every join would
// have needed an epsilon, and the size of that epsilon would have decided
// where one wall ended and the next began — which is precisely the
// straightness-tolerance problem this design exists to delete, moved one layer
// down.

// sqrt3 is the one irrational this embedding uses: the ratio between a hex's
// two spans.
var sqrt3 = math.Sqrt(3)

// hexArea is the area of a hex of circumradius 1. Every area this file
// reports is a fraction of it, so the constant itself never leaves.
var hexArea = 3 * sqrt3 / 2

// axialPoint is a point in FRACTIONAL axial coordinates — the frame
// [spatial.Position] already carries whole cells in, with the halves a wall
// endpoint needs.
//
// This is what leaves the compiler: [encounter.SegmentInput] carries wall
// endpoints as axial points because the client's axial-to-world formula
// already accepts fractions (design §5.2), so no second basis and no unit
// crosses the wire.
type axialPoint struct {
	Q, R float64
}

// worldPoint is a point in the plane of the embedding: x east, y south,
// circumradius 1. It never leaves this file.
type worldPoint struct {
	X, Y float64
}

// sidePosition is one of a hex's six side midpoints: the offset the file
// writes for it, and the neighbour whose side it is the midpoint of.
//
// The two halves are the same fact twice, which is the point — the offset is
// what an author reads and a picker snaps to, and the step is what a door's
// crossing IS (design F11: "the crossing of the side whose midpoint the
// position is"). Pinned against the embedding by
// TestTheSevenPositionsAreTheMidpointsAndTheCentre.
type sidePosition struct {
	// Offset is the position in BOUNDING-BOX FRACTIONS, exactly the pair the
	// file carries (design §3.3). Every value is dyadic, so the set compares
	// as exact floats and no tolerance is needed to recognise a position.
	Offset [2]float64

	// Step is the axial delta to the neighbour across this side.
	Step [2]int
}

// centreOffset is the seventh position: the middle of the hex. Written [0,0]
// in every orientation, and the only position that is not on a side.
var centreOffset = [2]float64{0, 0}

// pointySides and flatSides are THE CLOSED SET (design F8): six side
// midpoints per orientation, plus [centreOffset], and nothing else. An offset
// outside the set for the file's orientation is refused naming the wall and
// the value — never snapped to the nearest member, on [facings]' law.
//
// Ordered by the axial step so the two tables read as the same six
// neighbours under two layouts rather than as two unrelated lists.
var (
	pointySides = []sidePosition{
		{Offset: [2]float64{0.5, 0}, Step: [2]int{1, 0}},
		{Offset: [2]float64{-0.5, 0}, Step: [2]int{-1, 0}},
		{Offset: [2]float64{0.25, 0.375}, Step: [2]int{0, 1}},
		{Offset: [2]float64{-0.25, -0.375}, Step: [2]int{0, -1}},
		{Offset: [2]float64{0.25, -0.375}, Step: [2]int{1, -1}},
		{Offset: [2]float64{-0.25, 0.375}, Step: [2]int{-1, 1}},
	}

	flatSides = []sidePosition{
		{Offset: [2]float64{0.375, 0.25}, Step: [2]int{1, 0}},
		{Offset: [2]float64{-0.375, -0.25}, Step: [2]int{-1, 0}},
		{Offset: [2]float64{0, 0.5}, Step: [2]int{0, 1}},
		{Offset: [2]float64{0, -0.5}, Step: [2]int{0, -1}},
		{Offset: [2]float64{0.375, -0.25}, Step: [2]int{1, -1}},
		{Offset: [2]float64{-0.375, 0.25}, Step: [2]int{-1, 1}},
	}
)

// hexGeom is one orientation's embedding: how an axial coordinate becomes a
// point, where a hex's corners are, and which seven offsets its positions are
// written as.
type hexGeom struct {
	kind encounter.OrientationKind

	// qx, qy and rx, ry are the world vectors one axial step moves: a point
	// at axial (q,r) sits at q*(qx,qy) + r*(rx,ry).
	qx, qy, rx, ry float64

	// width and height are the hex's bounding box — the unit the file's
	// offsets are fractions of.
	width, height float64

	// corner is the six corners of a hex, relative to its centre.
	corner [6]worldPoint

	// sides is the six side midpoints in this layout. See [sidePosition].
	sides []sidePosition
}

// geometryOf is the embedding for one declared orientation.
//
// POINTY-TOP is odd-r: rows run straight, a hex is taller than it is wide
// (width sqrt(3), height 2), and its corners point north and south.
// FLAT-TOP is odd-q: the same hex turned 30°, wider than it is tall (width 2,
// height sqrt(3)).
func geometryOf(o encounter.Orientation) hexGeom {
	if o.Kind() == encounter.OrientationFlatTop {
		g := hexGeom{
			kind:   encounter.OrientationFlatTop,
			qx:     1.5,
			qy:     sqrt3 / 2,
			rx:     0,
			ry:     sqrt3,
			width:  2,
			height: sqrt3,
			sides:  flatSides,
		}
		for i := range g.corner {
			a := float64(i) * math.Pi / 3
			g.corner[i] = worldPoint{X: math.Cos(a), Y: math.Sin(a)}
		}

		return g
	}

	g := hexGeom{
		kind:   encounter.OrientationPointyTop,
		qx:     sqrt3,
		qy:     0,
		rx:     sqrt3 / 2,
		ry:     1.5,
		width:  sqrt3,
		height: 2,
		sides:  pointySides,
	}
	for i := range g.corner {
		a := float64(i)*math.Pi/3 + math.Pi/6
		g.corner[i] = worldPoint{X: math.Cos(a), Y: math.Sin(a)}
	}

	return g
}

// axialAt is the fractional axial point a position names: the cell, plus half
// the step to the neighbour across the named side.
//
// EXACT: every step component is 0, 1 or -1, so every result is a whole or a
// half, and two positions naming the same physical point from different cells
// compare equal without a tolerance. Reports false for an offset outside the
// seven.
func (g hexGeom) axialAt(cell spatial.Position, offset [2]float64) (axialPoint, bool) {
	if offset == centreOffset {
		return axialPoint{Q: cell.X, R: cell.Y}, true
	}
	for _, s := range g.sides {
		if s.Offset == offset {
			return axialPoint{
				Q: cell.X + float64(s.Step[0])/2,
				R: cell.Y + float64(s.Step[1])/2,
			}, true
		}
	}

	return axialPoint{}, false
}

// stepAt is the axial neighbour whose side an offset is the midpoint of —
// what a door's crossing is (design F11). Reports false for the centre and
// for any offset outside the six.
func (g hexGeom) stepAt(offset [2]float64) ([2]int, bool) {
	for _, s := range g.sides {
		if s.Offset == offset {
			return s.Step, true
		}
	}

	return [2]int{}, false
}

// world embeds a fractional axial point in the plane.
func (g hexGeom) world(a axialPoint) worldPoint {
	return worldPoint{
		X: a.Q*g.qx + a.R*g.rx,
		Y: a.Q*g.qy + a.R*g.ry,
	}
}

// centreOf is the world point at the middle of a whole cell.
func (g hexGeom) centreOf(cell spatial.Position) worldPoint {
	return g.world(axialPoint{Q: cell.X, R: cell.Y})
}

// hexOf is a cell's six corners in the plane, in boundary order.
func (g hexGeom) hexOf(cell spatial.Position) []worldPoint {
	c := g.centreOf(cell)
	out := make([]worldPoint, 6)
	for i, k := range g.corner {
		out[i] = worldPoint{X: c.X + k.X, Y: c.Y + k.Y}
	}

	return out
}

// directionTolerance is how far off a multiple of 30° a wall may be and still
// be read as that multiple: floating-point slack, nothing more. The positions
// are exact halves of axial steps, so a legal wall's angle lands within a few
// ulp of its multiple, and the nearest ILLEGAL direction between two
// positions is degrees away — there is no case this tolerance decides.
const directionTolerance = 1e-9

// directionOf reports the wall's bearing in degrees within [0,360), and
// whether it is one of the twelve (design F13).
//
// The twelve are the six along rows of neighbours and the six along hex
// sides, together — 30° apart, and the same twelve under both layouts because
// the embedding turns with the hexes. A zero-length wall reports false: it has
// no direction at all, which is a different defect the caller words for itself.
func (g hexGeom) directionOf(from, to axialPoint) (float64, bool) {
	a, b := g.world(from), g.world(to)
	dx, dy := b.X-a.X, b.Y-a.Y
	if dx == 0 && dy == 0 {
		return 0, false
	}
	rad := math.Atan2(dy, dx)
	step := math.Pi / 6
	k := math.Round(rad / step)
	deg := math.Mod(math.Mod(rad*180/math.Pi, 360)+360, 360)

	return deg, math.Abs(rad-k*step) <= directionTolerance
}

// touchTolerance is how close a point may be to a hex's edge and still count
// as on it. A thick wall runs ALONG a flat side (design C10's table), so
// "does this segment meet this hex" has to answer yes for a segment lying
// exactly on the boundary — the wall stands on those cells and foots on them,
// it just takes no area from them.
const touchTolerance = 1e-9

// candidateCells is every floor cell a wall COULD stand on: the cells inside
// the segment's own axial bounding box, grown by one.
//
// WHY THE BOX IS ENOUGH. A cell the segment touches has its centre within a
// circumradius of some point ON the segment, and a segment's points interpolate
// its two ends linearly in axial space — so every candidate's centre is within
// one circumradius, in axial terms, of the box the ends span. One circumradius
// of world distance is under one axial unit in either coordinate under either
// layout: pointy-top gives |dq| <= 0.91 and |dr| <= 2/3, flat-top the same pair
// the other way round. Since a position is a whole or a half, rounding the box
// outward to integers already absorbs all of that.
//
// THE EXTRA UNIT IS SLACK, NOT A REQUIREMENT, and it is worth saying which.
// Measured over 35,352 wall shapes — every direction, from every position, at
// every length up to nine cells, in both layouts — the rounded box alone is
// sufficient every time, so a mutant that removes the +1 changes no answer.
// It stays because the argument above is exactly tight: it holds for a
// SEVEN-position set whose offsets are halves, and would need re-deriving the
// day the set grows (F9 says it may). Slack costs a ring of map lookups; a
// silently-too-small box costs a wall that stops blocking a crossing.
//
// What actually guarantees this is right is not the argument but
// TestCandidateCellsAgreeWithScanningTheWholeFloor, which compares the narrow
// answer against the exhaustive one over every shape the dialect can express.
//
// FALLS BACK TO THE WHOLE FLOOR when the box would be bigger than the floor is,
// so this is never worse than the scan it replaces — a wall drawn from one
// corner of the coordinate space to the other spans a box no dungeon fills.
func (g hexGeom) candidateCells(floor map[spatial.Position]bool, from, to axialPoint) []spatial.Position {
	qLo := int(math.Floor(math.Min(from.Q, to.Q))) - 1
	qHi := int(math.Ceil(math.Max(from.Q, to.Q))) + 1
	rLo := int(math.Floor(math.Min(from.R, to.R))) - 1
	rHi := int(math.Ceil(math.Max(from.R, to.R))) + 1

	if int64(qHi-qLo+1)*int64(rHi-rLo+1) >= int64(len(floor)) {
		out := make([]spatial.Position, 0, len(floor))
		for cell := range floor {
			out = append(out, cell)
		}

		return out
	}

	var out []spatial.Position
	for q := qLo; q <= qHi; q++ {
		for r := rLo; r <= rHi; r++ {
			cell := spatial.Position{X: float64(q), Y: float64(r)}
			if floor[cell] {
				out = append(out, cell)
			}
		}
	}

	return out
}

// meets reports whether the closed wall segment touches the closed hex of a
// cell — the footprint test (design C8).
func (g hexGeom) meets(cell spatial.Position, from, to worldPoint) bool {
	hex := g.hexOf(cell)
	if pointInHex(hex, from) || pointInHex(hex, to) {
		return true
	}
	for i := range hex {
		if segmentsMeet(from, to, hex[i], hex[(i+1)%len(hex)]) {
			return true
		}
	}

	return false
}

// blocks reports whether a wall segment blocks the crossing between two
// adjacent cells (design C7): the closed centre-to-centre segment meets the
// closed wall segment.
//
// CENTRE TO CENTRE, not "the shared side", because a wall need not lie on a
// side — a thin line runs a quarter of a width inside the cells it shaves, and
// a thick one runs through their middles. What a crossing IS, physically, is
// the step from one cell's middle to the next one's, and the wall blocks it if
// it stands in the way.
func (g hexGeom) blocks(a, b spatial.Position, from, to worldPoint) bool {
	return segmentsMeet(g.centreOf(a), g.centreOf(b), from, to)
}

// standingFraction is A CELL'S STANDABLE FRACTION: the part of it inside the
// space the walls bound, as a fraction of the whole hex (design C10).
//
// EVERY WALL IN WHOSE FOOTPRINT THE CELL SITS CONTRIBUTES ITS HALF-PLANE —
// including a wall that only touches the cell at a corner position, which is
// the ordinary shape of a room's corner: one wall runs through the cell and
// the other leaves from the point where they meet. The half-plane is the
// wall's LINE, not its segment, because what is being measured is the room the
// walls enclose and not the stone's own length.
//
// That reading is not a convenience; it is what produces the design's two
// corner numbers. A square room's inside corner — a quarter line meeting a
// midpoint line — keeps exactly 3/4, and a hexagonal room's corner — two
// quarter lines at 60° — keeps 7/12. Both are pinned from the design's own
// arithmetic in geometry_internal_test.go, and 3/4 is why [MinStandable] is
// 0.7 rather than 0.75.
//
// NEAR SIDE means the side the cell's own centre is on: a wall takes the far
// half-plane away and leaves what an author could still stand in. A wall
// through the centre leaves exactly half either way, which is the whole
// mechanism behind "a thick wall seals the cells it centres" (design F15) —
// no special case, just a number below the threshold.
func (g hexGeom) standingFraction(cell spatial.Position, walls [][2]worldPoint) float64 {
	poly := g.hexOf(cell)
	centre := g.centreOf(cell)
	for _, w := range walls {
		nx, ny, c := nearHalfPlane(w[0], w[1], centre)
		poly = clipHalfPlane(poly, nx, ny, c)
		if len(poly) < 3 {
			return 0
		}
	}

	return polygonArea(poly) / hexArea
}

// nearHalfPlane is the half-plane {p : n·p <= c} bounded by the line through
// a and b and containing the point near. When near is ON the line — a wall
// through a cell's centre — either side is as true as the other and the
// answer is the one the normal already points away from; both leave half the
// hex, which is what makes that case need no special handling.
func nearHalfPlane(a, b, near worldPoint) (nx, ny, c float64) {
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
func clipHalfPlane(poly []worldPoint, nx, ny, c float64) []worldPoint {
	if len(poly) == 0 {
		return nil
	}
	out := make([]worldPoint, 0, len(poly)+1)
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
func lerpTo(a, b worldPoint, t float64) worldPoint {
	return worldPoint{X: a.X + (b.X-a.X)*t, Y: a.Y + (b.Y-a.Y)*t}
}

// polygonArea is the shoelace area of a simple polygon, sign discarded so
// winding does not matter.
func polygonArea(poly []worldPoint) float64 {
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

// pointInHex reports whether a point is inside or on a convex polygon given
// in boundary order, within [touchTolerance].
func pointInHex(poly []worldPoint, p worldPoint) bool {
	var sign int
	for i, cur := range poly {
		next := poly[(i+1)%len(poly)]
		d := cross(cur, next, p)
		switch {
		case d > touchTolerance:
			if sign < 0 {
				return false
			}
			sign = 1
		case d < -touchTolerance:
			if sign > 0 {
				return false
			}
			sign = -1
		}
	}

	return true
}

// cross is the z of (b-a)×(p-a): positive when p is left of a→b.
func cross(a, b, p worldPoint) float64 {
	return (b.X-a.X)*(p.Y-a.Y) - (b.Y-a.Y)*(p.X-a.X)
}

// segmentsMeet reports whether two CLOSED segments share at least one point,
// touching and collinear overlap included.
func segmentsMeet(p1, p2, p3, p4 worldPoint) bool {
	d1 := cross(p3, p4, p1)
	d2 := cross(p3, p4, p2)
	d3 := cross(p1, p2, p3)
	d4 := cross(p1, p2, p4)
	if ((d1 > touchTolerance && d2 < -touchTolerance) || (d1 < -touchTolerance && d2 > touchTolerance)) &&
		((d3 > touchTolerance && d4 < -touchTolerance) || (d3 < -touchTolerance && d4 > touchTolerance)) {
		return true
	}

	return (math.Abs(d1) <= touchTolerance && onSegment(p3, p4, p1)) ||
		(math.Abs(d2) <= touchTolerance && onSegment(p3, p4, p2)) ||
		(math.Abs(d3) <= touchTolerance && onSegment(p1, p2, p3)) ||
		(math.Abs(d4) <= touchTolerance && onSegment(p1, p2, p4))
}

// onSegment reports whether a point already known to be collinear with a→b
// lies within its bounds.
func onSegment(a, b, p worldPoint) bool {
	return p.X >= math.Min(a.X, b.X)-touchTolerance && p.X <= math.Max(a.X, b.X)+touchTolerance &&
		p.Y >= math.Min(a.Y, b.Y)-touchTolerance && p.Y <= math.Max(a.Y, b.Y)+touchTolerance
}

// alongWall is how far down a wall a cell's centre falls, as the parameter of
// the projection onto the segment — what puts a footprint "in order along the
// wall" (design C8). Degenerate walls never reach here; Validate refuses a
// wall with no direction first.
func (g hexGeom) alongWall(cell spatial.Position, from, to worldPoint) float64 {
	c := g.centreOf(cell)
	dx, dy := to.X-from.X, to.Y-from.Y
	den := dx*dx + dy*dy
	if den == 0 {
		return 0
	}

	return ((c.X-from.X)*dx + (c.Y-from.Y)*dy) / den
}
