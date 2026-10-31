// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package encounter

import (
	"math"
	"slices"

	"github.com/KirkDiggler/rpg-toolkit/tools/spatial"
)

// placement_index.go is THE FIELD INDEXES ITS PLACEMENTS (rpg-project#753,
// server-at-scale design "The field indexes its placements"): the cells an
// authored placement, a structural wall or a cell prop covers are derived
// ONCE, when the field is built from its data, and read as a lookup after
// that. Nothing scans every cell of the floor to find what stands on one.
//
// TWO PIECES, both built in [compileField] and never written after:
//
//   - [cellCentres], the floor's cell centres bucketed on a square grid one
//     cell wide, so "which cells lie inside this box" and "which cell is
//     nearest this point" cost the box's cells, not the field's. It is what
//     makes [field.placedCells] and [field.footprintObservationCells] linear
//     in the footprint for ANY placement, authored or dropped.
//   - [placementIndex], what those two reads answer for every AUTHORED
//     placement, by id and by cell.
//
// A PLACEMENT THAT MOVED IS ANSWERED FROM ITS RECORD AT THE MOMENT OF ASKING:
// a dropped rectangle's origin is the journal's ([placedFold.stands]), and
// its cells are measured then, through the same bucketed reads. So nothing
// that writes the journal writes the index, and the index cannot go stale.
//
// NOTHING HERE IS STORED. The index is derived from the field's own data on
// every construction and every load, exactly as the wall partitions are.

// cellCentres buckets the floor's cell centres, in the placed plane, on a
// square grid of side [placedPlaneCellWidth].
type cellCentres struct {
	size float64
	// centre is each field cell's plane centre, parallel to field.cells.
	centre []spatial.Point
	// minB and maxB are the occupied buckets' extent on each axis; the
	// buckets are the dense grid between them, row by row.
	minB, maxB [2]int64
	// start and items hold each bucket's cells (indices into field.cells,
	// ascending): bucket k's are items[start[k]:start[k+1]].
	start, items []int32
}

// nearestRingLimit is how many bucket rings the nearest-centre search walks
// before it falls back to the whole floor: a point that far off the floor
// (scenery drawn in the void) is rare, and a linear walk then is the honest
// answer rather than a ring walk across empty space.
const nearestRingLimit = 64

// newCellCentres buckets every cell of the floor. Linear in the cells and
// in the floor's bounding box of buckets.
func newCellCentres(f *field) *cellCentres {
	c := &cellCentres{
		size:   float64(placedPlaneCellWidth),
		centre: make([]spatial.Point, len(f.cells)),
	}
	keys := make([][2]int64, len(f.cells))
	for i, cell := range f.cells {
		at := f.plane.CellCentre(cell)
		c.centre[i] = at
		key := c.keyOf(at)
		keys[i] = key
		if i == 0 {
			c.minB, c.maxB = key, key
			continue
		}
		for axis := 0; axis < 2; axis++ {
			c.minB[axis] = min(c.minB[axis], key[axis])
			c.maxB[axis] = max(c.maxB[axis], key[axis])
		}
	}
	if len(f.cells) == 0 {
		c.start = []int32{0}
		return c
	}
	buckets := int(c.maxB[0]-c.minB[0]+1) * int(c.maxB[1]-c.minB[1]+1)
	c.start = make([]int32, buckets+1)
	for _, key := range keys {
		c.start[c.slot(key)+1]++
	}
	for k := 1; k <= buckets; k++ {
		c.start[k] += c.start[k-1]
	}
	fill := slices.Clone(c.start[:buckets])
	c.items = make([]int32, len(f.cells))
	for i, key := range keys {
		k := c.slot(key)
		c.items[fill[k]] = int32(i)
		fill[k]++
	}

	return c
}

// slot is an occupied-extent bucket's position in the dense grid.
func (c *cellCentres) slot(key [2]int64) int {
	return int(key[0]-c.minB[0])*int(c.maxB[1]-c.minB[1]+1) + int(key[1]-c.minB[1])
}

// in is one bucket's cells, nil for a bucket beyond the extent.
func (c *cellCentres) in(key [2]int64) []int32 {
	for axis := 0; axis < 2; axis++ {
		if key[axis] < c.minB[axis] || key[axis] > c.maxB[axis] {
			return nil
		}
	}
	k := c.slot(key)

	return c.items[c.start[k]:c.start[k+1]]
}

// keyOf is the bucket a plane point falls in.
func (c *cellCentres) keyOf(p spatial.Point) [2]int64 {
	return [2]int64{int64(math.Floor(p.X / c.size)), int64(math.Floor(p.Y / c.size))}
}

// within lists, in field order, every cell whose centre may lie inside the
// box lo..hi — every bucket the box touches, so a SUPERSET of the cells
// inside, for the caller's own exact test to filter. A box beyond the floor's
// buckets is clamped to them.
func (c *cellCentres) within(lo, hi spatial.Point) []int32 {
	a, b := c.keyOf(lo), c.keyOf(hi)
	for axis := 0; axis < 2; axis++ {
		a[axis] = max(a[axis], c.minB[axis])
		b[axis] = min(b[axis], c.maxB[axis])
		if a[axis] > b[axis] {
			return nil
		}
	}
	var out []int32
	for x := a[0]; x <= b[0]; x++ {
		for y := a[1]; y <= b[1]; y++ {
			out = append(out, c.in([2]int64{x, y})...)
		}
	}
	slices.Sort(out)

	return out
}

// nearest is the field cell whose centre is nearest p, ties broken by
// [cellBefore] — [field.placedCells]' clause 2, the same total order the
// whole-floor walk picks by, so the answer is the walk's answer.
//
// THE RING STOPS ONLY ON A PROOF: after ring k every cell not yet measured
// lies in a bucket k+1 or more away on some axis, so more than k cell widths
// from p; once the best squared distance is strictly below that bound (less
// a rounding margin) no unmeasured cell can beat or tie it.
func (c *cellCentres) nearest(f *field, p spatial.Point) (int32, bool) {
	best, bestD2, found := int32(-1), 0.0, false
	consider := func(i int32) {
		at := c.centre[i]
		dx, dy := at.X-p.X, at.Y-p.Y
		if d2 := dx*dx + dy*dy; !found || d2 < bestD2 || (d2 == bestD2 && cellBefore(f.cells[i], f.cells[best])) {
			found, best, bestD2 = true, i, d2
		}
	}
	home := c.keyOf(p)
	outside := int64(0)
	for axis := 0; axis < 2; axis++ {
		outside = max(outside, c.minB[axis]-home[axis], home[axis]-c.maxB[axis])
	}
	span := max(c.maxB[0]-c.minB[0], c.maxB[1]-c.minB[1])
	if outside > nearestRingLimit {
		for i := range c.centre {
			consider(int32(i))
		}
		return best, found
	}
	for k := int64(0); k <= outside+span+1; k++ {
		for x := home[0] - k; x <= home[0]+k; x++ {
			for y := home[1] - k; y <= home[1]+k; y++ {
				if max(abs64(x-home[0]), abs64(y-home[1])) != k {
					continue
				}
				for _, i := range c.in([2]int64{x, y}) {
					consider(i)
				}
			}
		}
		bound := float64(k) * c.size
		if found && bestD2 < bound*bound*(1-1e-9) {
			break
		}
	}

	return best, found
}

func abs64(v int64) int64 {
	if v < 0 {
		return -v
	}
	return v
}

// boxMargin widens a candidate box past [footprintBounds]' rejection
// tolerance, which is relative to the magnitudes in the question: one cell
// width plus a relative term far larger than that tolerance, so the bucketed
// candidates always contain every cell the exact test would admit.
func boxMargin(b footprintBounds, origin spatial.Point, extra float64) float64 {
	return extra + float64(placedPlaneCellWidth) +
		1e-6*(1+b.mag+extra+3*(math.Abs(origin.X)+math.Abs(origin.Y)))
}

// placementRefKind says which kind of thing a [placementRef] names.
type placementRefKind int

const (
	// refPlaced is a placed footprint contributor, standing on the cell.
	refPlaced placementRefKind = iota + 1
	// refWall is a structural wall, whose footprint overlaps the cell.
	refWall
	// refCellProp is a cell prop, authored on the cell.
	refCellProp
)

// placementRef is one authored thing the index lists on a cell.
type placementRef struct {
	id   PropID
	kind placementRefKind
	// at is the thing's position in its own list: field.placed for a
	// placed contributor or a wall's presence, field.props for a cell prop.
	at int
}

// placementIndex is what stands where, AS AUTHORED: built with the field in
// one pass over its placements, walls and cell props, and never written
// after.
type placementIndex struct {
	// authoredAt is each placed contributor's authored placement: a placement
	// asked about in any other pose (origin, facing, offset or box) is not
	// the one the index measured, and is measured at query time.
	authoredAt map[PropID]spatial.FootprintPlacement
	// stands is each placed contributor's [field.placedCells], authored.
	stands map[PropID][]spatial.Position
	// support is each placed contributor's
	// [field.footprintObservationCells], authored — a structural wall's
	// presence included, which is what [field.wallFootprintCells] reads.
	support map[PropID][]spatial.Position
	// at is every authored thing on a cell: placed contributors by the cells
	// they stand on, structural walls by the floor they overlap, and cell
	// props by their authored cell.
	at map[spatial.Position][]placementRef
}

// buildPlacementIndex derives the index from a compiled field. Called last
// in [compileField], after the structural walls resolve their presences.
func (f *field) buildPlacementIndex() {
	idx := &placementIndex{
		authoredAt: make(map[PropID]spatial.FootprintPlacement, len(f.placed)),
		stands:     make(map[PropID][]spatial.Position, len(f.placed)),
		support:    make(map[PropID][]spatial.Position, len(f.placed)),
		at:         make(map[spatial.Position][]placementRef),
	}
	walls := make(map[PropID]bool, len(f.structuralWalls))
	for i := range f.structuralWalls {
		walls[f.structuralWalls[i].id] = true
	}
	for i := range f.placed {
		p := &f.placed[i]
		idx.authoredAt[p.id] = p.placement
		stands := f.placedCells(p.placement)
		idx.stands[p.id] = stands
		for _, cell := range stands {
			idx.at[cell] = append(idx.at[cell], placementRef{id: p.id, kind: refPlaced, at: i})
		}
		// A support the plane cannot measure is left to query time, which
		// returns the measurement's own error to the reader that asks.
		support, err := f.footprintObservationCells(p.placement)
		if err != nil {
			continue
		}
		idx.support[p.id] = support
		if walls[p.id] {
			for _, cell := range support {
				idx.at[cell] = append(idx.at[cell], placementRef{id: p.id, kind: refWall, at: i})
			}
		}
	}
	for i, p := range f.props {
		if p.ID == "" {
			continue
		}
		cell := f.cellAt(p.At)
		idx.at[cell] = append(idx.at[cell], placementRef{id: p.ID, kind: refCellProp, at: i})
	}
	f.index = idx
}

// authored reports whether a placement for id is the one the field was built
// with — the only placement the index answers for. The WHOLE pose is
// compared, not only the origin a drop moves today, so a future change that
// turns or reshapes a placement is measured rather than read stale.
func (idx *placementIndex) authored(id PropID, p spatial.FootprintPlacement) bool {
	if idx == nil {
		return false
	}
	a, ok := idx.authoredAt[id]

	return ok && samePose(a, p)
}

// samePose reports whether two placements are the same rectangle in the same
// place: origin, facing, local offset and footprint, compared by value.
func samePose(a, b spatial.FootprintPlacement) bool {
	if a.Origin != b.Origin || a.Facing != b.Facing || a.LocalOffset != b.LocalOffset {
		return false
	}
	fa, fb := a.Footprint, b.Footprint
	if (fa.Box == nil) != (fb.Box == nil) || (fa.Triangle == nil) != (fb.Triangle == nil) {
		return false
	}
	if fa.Box != nil && *fa.Box != *fb.Box {
		return false
	}
	return fa.Triangle == nil || *fa.Triangle == *fb.Triangle
}

// standsAt reads the authored cells a placed contributor stands on, a copy
// the caller owns; false when the index cannot answer for this placement.
func (idx *placementIndex) standsAt(id PropID, p spatial.FootprintPlacement) ([]spatial.Position, bool) {
	if !idx.authored(id, p) {
		return nil, false
	}
	cells, ok := idx.stands[id]

	return append([]spatial.Position(nil), cells...), ok
}

// supportOf reads the authored observation support of a placed contributor;
// read-only, false when the index cannot answer for this placement.
func (idx *placementIndex) supportOf(id PropID, p spatial.FootprintPlacement) ([]spatial.Position, bool) {
	if !idx.authored(id, p) {
		return nil, false
	}
	cells, ok := idx.support[id]

	return cells, ok
}

// on is every authored thing on one cell, in authored order. Read-only.
func (idx *placementIndex) on(cell spatial.Position) []placementRef {
	if idx == nil {
		return nil
	}
	return idx.at[cell]
}
