package encounter

import (
	"math"

	"github.com/KirkDiggler/rpg-toolkit/tools/spatial"
)

// footprintBounds is a placed rectangle's axis-aligned REACH, measured once
// where the footprint is placed, so a query can skip a trace that cannot
// touch it (rpg-toolkit#2049).
//
// RELATIVE TO THE ORIGIN, not absolute. Footprint, facing and local offset are
// construction truth; the origin is the one thing [placedFold.stands] moves
// (a dropped thing lands on a new cell, shape and facing untouched). Holding
// the reach as an offset from the origin keeps one cached answer correct for
// every pose a contributor can take.
//
// A PRE-FILTER, NEVER A SECOND ANSWER. [footprintBounds.mayTouch] only ever
// says "this trace cannot make contact"; every "maybe" is still asked of
// [spatial.TraceFootprint], which stays the one measure of contact and
// interior. Rejection is conservative by [footprintBoundsSlack], so rounding
// in this module's arithmetic and in spatial's local-frame transform cannot
// turn a contact into a miss: results are identical to tracing everything.
//
// The ZERO VALUE TRACES EVERYTHING (ok false). [boundsOf] returns it for every
// placement spatial would refuse to trace — no box, a triangle, a side that
// is not a positive finite number, or a reach that is not finite — so such a
// placement is never filtered and its trace still returns spatial's own error
// to the caller that reads it. (Construction validation refuses all of these
// for a compiled field already; the check here keeps the filter exact without
// relying on that.)
type footprintBounds struct {
	ok                     bool
	minX, maxX, minY, maxY float64
	// mag is the largest magnitude among the four, folded into the
	// rejection tolerance's scale.
	mag float64
}

// footprintBoundsSlack is the relative margin rejection must clear: a
// segment is skipped only when it misses the reach by more than this share of
// the coordinate magnitudes in the question (plus one foot's share, so the
// margin never vanishes near the plane's origin). Float64 rounding in either
// side's arithmetic is around 1e-16 of that magnitude; 1e-9 leaves seven
// orders of headroom and still rejects everything a cell away.
//
// RELATIVE BECAUSE ROUNDING IS. An absolute margin small enough to reject a
// lane a foot away near the origin is smaller than one float64 step far from
// it. TestMayTouchAgreesWithSpatial draws placements out to 1e8 feet, where a
// flat 1e-9 margin drops contacts spatial reports.
const footprintBoundsSlack = 1e-9

// boundsOf measures a placement's reach relative to its origin: the centre's
// offset (the same along/across transform [placedCentre] mirrors from
// spatial) widened by the rotated rectangle's half extents on each axis. A
// rectangle's axis-aligned bounds are exactly |along|·halfDepth +
// |across|·halfWidth either side of its centre.
func boundsOf(p spatial.FootprintPlacement) footprintBounds {
	if p.Footprint.Box == nil || p.Footprint.Triangle != nil {
		return footprintBounds{}
	}
	if w, d := p.Footprint.Box.W, p.Footprint.Box.D; !(w > 0) || !(d > 0) || !finite(w) || !finite(d) {
		return footprintBounds{}
	}
	a := math.Mod(p.Facing, 360) * math.Pi / 180
	along := spatial.Point{X: math.Cos(a), Y: math.Sin(a)}
	across := spatial.Point{X: -math.Sin(a), Y: math.Cos(a)}
	halfDepth, halfWidth := p.Footprint.Box.D/2, p.Footprint.Box.W/2

	cx := along.X*p.LocalOffset.X + across.X*p.LocalOffset.Y
	cy := along.Y*p.LocalOffset.X + across.Y*p.LocalOffset.Y
	hx := math.Abs(along.X)*halfDepth + math.Abs(across.X)*halfWidth
	hy := math.Abs(along.Y)*halfDepth + math.Abs(across.Y)*halfWidth

	b := footprintBounds{ok: true, minX: cx - hx, maxX: cx + hx, minY: cy - hy, maxY: cy + hy}
	for _, v := range []float64{b.minX, b.maxX, b.minY, b.maxY} {
		if !finite(v) {
			return footprintBounds{}
		}
		b.mag = math.Max(b.mag, math.Abs(v))
	}

	return b
}

// mayTouch reports whether the closed segment from..to can reach the
// rectangle these bounds were measured for, placed at origin. False is a
// proof of no contact; true means "ask spatial". A zero-length segment is the
// stationary question the standing reads ask. It is [footprintBounds.mayOverlap]
// asked of the segment's own bounding box, which contains the segment.
//
// HOT: every lane, crossing and standing read asks it of every contributor,
// so it is straight-line arithmetic — no loops, no math.Max.
func (b footprintBounds) mayTouch(origin, from, to spatial.Point) bool {
	lo, hi := from, to
	if lo.X > hi.X {
		lo.X, hi.X = hi.X, lo.X
	}
	if lo.Y > hi.Y {
		lo.Y, hi.Y = hi.Y, lo.Y
	}

	return b.mayOverlap(origin, lo, hi)
}

// mayOverlap reports whether the closed axis-aligned box lo..hi (lo the
// lower corner on both axes) can overlap the rectangle these bounds were
// measured for, placed at origin. False is a proof that nothing inside the
// box touches the rectangle; true means "ask spatial".
//
// NON-FINITE INPUT IS NEVER REJECTED, so spatial — not this filter — is the
// one that refuses it with [spatial.ErrBadFootprintTrace]. Each test below is
// written as "reject when strictly beyond", and every comparison involving NaN
// is false; an infinite coordinate makes the tolerance infinite (or NaN), so
// no rejection can hold either.
func (b footprintBounds) mayOverlap(origin, lo, hi spatial.Point) bool {
	if !b.ok {
		return true
	}
	// The tolerance's scale is a SUM of magnitudes, which is at least their
	// maximum: a wider margin only ever traces more.
	scale := 1 + b.mag + math.Abs(origin.X) + math.Abs(origin.Y) +
		math.Abs(lo.X) + math.Abs(lo.Y) + math.Abs(hi.X) + math.Abs(hi.Y)
	tol := footprintBoundsSlack * scale

	return !(hi.X < origin.X+b.minX-tol || lo.X > origin.X+b.maxX+tol ||
		hi.Y < origin.Y+b.minY-tol || lo.Y > origin.Y+b.maxY+tol)
}
