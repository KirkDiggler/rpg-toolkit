// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package encounter

import (
	"math"
	"math/bits"
	"slices"

	"github.com/KirkDiggler/rpg-toolkit/tools/spatial"
)

// placed_reach_index.go is A QUERY ASKS THE PLACEMENTS NEAR IT
// (rpg-project#753, S3; the law "The field indexes its placements"). Every
// geometric question about the placed contributors — does sight cross this
// lane ([field.sightBlocksAlongExcept]), does a step cross this edge
// ([field.crossingBlocksExcept]), is this centre covered
// ([field.standingBlocks], [field.screensOf],
// [field.computeSightBlocksOriginAt], the cell fold) — walked every
// placement on the field and let [footprintBounds.mayTouch] reject the far
// ones. On the 10k keep that is 654 placements per lane, and a launch asks a
// few hundred thousand lanes.
//
// THE AUTHORED RECTANGLES ARE FILED ONCE, at compile, in buckets on the
// placed plane by where their bounds reach; a query reads the buckets its own
// box touches. A placement that can MOVE is never filed: it is asked by every
// query, from its record at the moment of asking, exactly as before — so no
// writer maintains this and it cannot go stale.
//
// A SUPERSET, IN COMPILED ORDER. Both boxes are widened by a margin larger
// than mayTouch's own rounding slack, so a placement the buckets do not name
// is one mayTouch would have rejected; every named one is still asked
// mayTouch and then traced by the caller, and the names come back in
// [field.placed] order, so the first blocker a caller reports — a refusal's
// sentence names it — is the walk's first.

// placedReachBucketFeet is one bucket's side: two cells.
const placedReachBucketFeet = 2 * float64(placedPlaneCellWidth)

// placedReachCap bounds the index: a placement spread over more buckets than
// this, or a field whose filed placements span more, is asked by every query
// instead — the walk, which is always right.
const placedReachCap = 1 << 22

// placedReach is the bucketed authored placements.
type placedReach struct {
	minB, maxB [2]int64
	// start and items are each bucket's placements (indices into
	// field.placed, ascending): bucket k's are items[start[k]:start[k+1]].
	start, items []int32
	// always is every placement asked by every query, ascending: the ones
	// that can move, and the ones too wide (or unmeasurable) to file.
	always []int32
	// placed is how many placements the field has: the bitset's width.
	placed int
}

// reachMargin is how far a box is widened before it is bucketed: a foot, plus
// a relative term a thousand times mayOverlap's slack at these magnitudes.
func reachMargin(mag float64, coords ...float64) float64 {
	scale := 1 + mag
	for _, c := range coords {
		scale += math.Abs(c)
	}
	return 1 + 1e-6*scale
}

// span is the buckets the box lo..hi widened by m covers; false when it is
// not finite.
func reachSpan(lo, hi spatial.Point, m float64) (a, b [2]int64, ok bool) {
	vals := [4]float64{
		math.Floor((lo.X - m) / placedReachBucketFeet), math.Floor((lo.Y - m) / placedReachBucketFeet),
		math.Floor((hi.X + m) / placedReachBucketFeet), math.Floor((hi.Y + m) / placedReachBucketFeet),
	}
	for _, v := range vals {
		if math.IsNaN(v) || math.IsInf(v, 0) || math.Abs(v) > 1<<40 {
			return a, b, false
		}
	}
	return [2]int64{int64(vals[0]), int64(vals[1])}, [2]int64{int64(vals[2]), int64(vals[3])}, true
}

func spanCount(a, b [2]int64) int64 { return (b[0] - a[0] + 1) * (b[1] - a[1] + 1) }

// buildPlacedReach files the field's authored placements. Linear in the
// placements and the buckets they cover.
func (f *field) buildPlacedReach() *placedReach {
	r := &placedReach{placed: len(f.placed)}
	type filed struct {
		at   int32
		a, b [2]int64
	}
	var files []filed
	for i := range f.placed {
		p := &f.placed[i]
		if p.mutable() || !p.reach.ok {
			r.always = append(r.always, int32(i))
			continue
		}
		o := p.placement.Origin
		lo := spatial.Point{X: o.X + p.reach.minX, Y: o.Y + p.reach.minY}
		hi := spatial.Point{X: o.X + p.reach.maxX, Y: o.Y + p.reach.maxY}
		a, b, ok := reachSpan(lo, hi, reachMargin(p.reach.mag, o.X, o.Y, lo.X, lo.Y, hi.X, hi.Y))
		if !ok || spanCount(a, b) > placedReachCap {
			r.always = append(r.always, int32(i))
			continue
		}
		if len(files) == 0 {
			r.minB, r.maxB = a, b
		}
		for axis := 0; axis < 2; axis++ {
			r.minB[axis] = min(r.minB[axis], a[axis])
			r.maxB[axis] = max(r.maxB[axis], b[axis])
		}
		files = append(files, filed{at: int32(i), a: a, b: b})
	}
	if len(files) == 0 {
		r.minB, r.maxB = [2]int64{1, 1}, [2]int64{0, 0} // no buckets at all
		r.start = []int32{0}
		return r
	}
	if spanCount(r.minB, r.maxB) > placedReachCap {
		// Filed placements scattered over a plane too wide to grid: ask
		// every one of them, as the walk did.
		r.always = r.always[:0]
		for i := range f.placed {
			r.always = append(r.always, int32(i))
		}
		r.minB, r.maxB = [2]int64{1, 1}, [2]int64{0, 0}
		r.start = []int32{0}
		return r
	}
	buckets := int(spanCount(r.minB, r.maxB))
	r.start = make([]int32, buckets+1)
	for _, fl := range files {
		for x := fl.a[0]; x <= fl.b[0]; x++ {
			for y := fl.a[1]; y <= fl.b[1]; y++ {
				r.start[r.slot(x, y)+1]++
			}
		}
	}
	for k := 1; k <= buckets; k++ {
		r.start[k] += r.start[k-1]
	}
	fill := slices.Clone(r.start[:buckets])
	r.items = make([]int32, r.start[buckets])
	for _, fl := range files { // ascending placement order, so each bucket is sorted
		for x := fl.a[0]; x <= fl.b[0]; x++ {
			for y := fl.a[1]; y <= fl.b[1]; y++ {
				k := r.slot(x, y)
				r.items[fill[k]] = fl.at
				fill[k]++
			}
		}
	}

	return r
}

func (r *placedReach) slot(x, y int64) int {
	return int(x-r.minB[0])*int(r.maxB[1]-r.minB[1]+1) + int(y-r.minB[1])
}

// near is every placement (index into field.placed) whose bounds the
// segment a→b may meet, ascending, each once: the filed ones in the buckets
// the segment passes through, and every always-asked one. False when the segment cannot be bucketed, for the caller
// to walk every placement.
//
// THE SEGMENT, NOT ITS BOX. [footprintBounds.mayTouch] admits a placement
// whose bounds meet the segment's bounding box; the trace then answers for
// the segment itself. A placement whose bounds the segment never meets is one
// the trace answers "no contact, no interior" for, so leaving it out changes
// no answer — and a long diagonal lane's box holds far more placements than
// the lane passes. Each bucket column the segment spans contributes the rows
// the segment occupies inside it, widened by the margin.
func (r *placedReach) near(a, b spatial.Point) ([]int32, bool) {
	if a.X > b.X {
		a, b = b, a
	}
	m := reachMargin(0, a.X, a.Y, b.X, b.Y)
	s, e, ok := reachSpan(spatial.Point{X: a.X, Y: min(a.Y, b.Y)}, spatial.Point{X: b.X, Y: max(a.Y, b.Y)}, m)
	if !ok || spanCount(s, e) > placedReachCap {
		return nil, false
	}
	// Marked on a bitset over field.placed and read back in order: each
	// once, ascending, with no sort.
	var stack [64]uint64
	marks := stack[:]
	if words := (r.placed + 63) / 64; words > len(stack) {
		marks = make([]uint64, words)
	}
	count := 0
	mark := func(i int32) {
		w, bit := i>>6, uint64(1)<<(i&63)
		if marks[w]&bit == 0 {
			marks[w] |= bit
			count++
		}
	}
	for _, i := range r.always {
		mark(i)
	}
	x0, x1 := max(s[0], r.minB[0]), min(e[0], r.maxB[0])
	dx := b.X - a.X
	for x := x0; x <= x1; x++ {
		// The segment's y over this column's x interval, clamped to the
		// segment's own ends.
		yLo, yHi := a.Y, b.Y
		if dx > 0 {
			cx0 := max(a.X, float64(x)*placedReachBucketFeet)
			cx1 := min(b.X, float64(x+1)*placedReachBucketFeet)
			if cx0 > cx1 {
				cx0, cx1 = a.X, b.X // the margin's column beyond an end: the whole segment
			}
			yLo = a.Y + (b.Y-a.Y)*(cx0-a.X)/dx
			yHi = a.Y + (b.Y-a.Y)*(cx1-a.X)/dx
		}
		if yLo > yHi {
			yLo, yHi = yHi, yLo
		}
		y0 := max(int64(math.Floor((yLo-m)/placedReachBucketFeet)), r.minB[1])
		y1 := min(int64(math.Floor((yHi+m)/placedReachBucketFeet)), r.maxB[1])
		for y := y0; y <= y1; y++ {
			k := r.slot(x, y)
			for _, i := range r.items[r.start[k]:r.start[k+1]] {
				mark(i)
			}
		}
	}
	out := make([]int32, 0, count)
	for w, word := range marks[:(r.placed+63)/64] {
		for word != 0 {
			bit := bits.TrailingZeros64(word)
			out = append(out, int32(w*64+bit))
			word &= word - 1
		}
	}

	return out, true
}

// placedNear is every placed contributor (index into f.placed) that may
// touch the box spanned by a and b, in compiled order — the placement index's
// answer, or every placement when the field has no index, the index was
// built for another list, or the box cannot be bucketed. Callers still ask each one mayTouch and the trace.
func (f *field) placedNear(a, b spatial.Point) []int32 {
	if f.index != nil && f.index.reach != nil && f.index.reach.placed == len(f.placed) {
		if near, ok := f.index.reach.near(a, b); ok {
			return near
		}
	}
	all := make([]int32, len(f.placed))
	for i := range all {
		all[i] = int32(i)
	}
	return all
}
