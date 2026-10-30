// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package encounter

import (
	"fmt"
	"math"

	"github.com/KirkDiggler/rpg-toolkit/tools/spatial"
)

// partitionSight is THE PARTITION PASS'S SIGHT, LOCALLY INDEXED
// (rpg-project#725, R8). Partitioning floods the whole floor and asks one
// question per edge — can sight cross from this cell to its neighbour — and
// one per cell — is its centre covered. Asked through
// [canvasRoom.IsLineOfSightBlocked] and [canvasSightObstructions.At], each
// answer scans every contributor on the field, so the flood was
// O(edges x contributors): two seconds on a 30,000-hex site.
//
// THE SAME ANSWER, NOT A SECOND RULE. For two ADJACENT cells
// [spatial.SightLanes] never takes an alternate lane — the only neighbour of
// either end nearer the other end is the other end itself, whose lane is the
// direct one — so its answer is exactly the direct lane's: a sight-blocking
// boundary on the crossing (hard), an occluding entity on the ray's interior,
// the void, or a sight-blocking contributor's INTERIOR on the segment between
// the centres ([field.sightBlocksAlong]). [partitionSight.edgeBlocked] asks
// those same facts in that order, and a pair that is not adjacent goes to
// IsLineOfSightBlocked unchanged.
//
// THE INDEX IS A SUPERSET. Each contributor is filed under every bucket its
// reach ([footprintBounds], rpg-toolkit#2049) overlaps, widened by a bucket
// on each side; a query collects the buckets its segment's box overlaps and
// still asks each candidate [footprintBounds.mayTouch] and then
// [spatial.TraceFootprint], the one measure of contact. A contributor whose
// reach cannot be measured is asked by every query. So nothing the full scan
// would trace is skipped, and every answer is the full scan's.
//
// FIXED CONTRIBUTORS ONLY, as the pass is: the canvas this is built on holds
// the immutable placed contributors and every door shut ([field.partitionFloor]).
type partitionSight struct {
	canvas *canvasRoom
	// items are the sight-blocking contributors, placed then doors — the
	// order [field.sightBlocksAlongExcept] asks them in.
	items []partitionSightItem
	// buckets files item indexes by bucket; always is the items with no
	// measurable reach, asked by every query.
	buckets map[[2]int][]int
	always  []int
	// seen stamps the items one query has asked, so an item filed under two
	// buckets is traced once.
	seen  []int
	stamp int
}

// partitionSightItem is one contributor as the pass sees it.
type partitionSightItem struct {
	name      string
	placement spatial.FootprintPlacement
	reach     footprintBounds
}

// partitionBucketFeet is the index's bucket side: two cells.
const partitionBucketFeet = 2 * FeetPerCell

func newPartitionSight(canvas *canvasRoom) *partitionSight {
	f := canvas.field
	s := &partitionSight{canvas: canvas, buckets: map[[2]int][]int{}}
	now := f.placedNow()
	for i := range f.placed {
		p := &f.placed[i]
		if !p.blocksLineOfSight {
			continue
		}
		placement, standing := now.stands(p)
		if !standing {
			continue
		}
		s.add(fmt.Sprintf("placed prop %q", p.id), placement, p.reach)
	}
	for _, d := range f.blockingDoorFootprints() {
		s.add(fmt.Sprintf("door %q", d.id), *d.placement, d.reach)
	}
	s.seen = make([]int, len(s.items))
	return s
}

// add files one contributor under every bucket its reach overlaps, one
// bucket wider on each side than the reach itself.
func (s *partitionSight) add(name string, placement spatial.FootprintPlacement, reach footprintBounds) {
	i := len(s.items)
	s.items = append(s.items, partitionSightItem{name: name, placement: placement, reach: reach})
	o := placement.Origin
	if !reach.ok || !finite(o.X) || !finite(o.Y) {
		s.always = append(s.always, i)
		return
	}
	x0, x1 := bucketOf(o.X+reach.minX)-1, bucketOf(o.X+reach.maxX)+1
	y0, y1 := bucketOf(o.Y+reach.minY)-1, bucketOf(o.Y+reach.maxY)+1
	if (x1-x0+1)*(y1-y0+1) > 1<<16 {
		// A reach this large is cheaper asked every time than filed.
		s.always = append(s.always, i)
		return
	}
	for x := x0; x <= x1; x++ {
		for y := y0; y <= y1; y++ {
			k := [2]int{x, y}
			s.buckets[k] = append(s.buckets[k], i)
		}
	}
}

func bucketOf(v float64) int { return int(math.Floor(v / partitionBucketFeet)) }

// candidates calls ask with every item that may touch the box lo..hi, each
// once, stopping at the first that answers true or errs.
func (s *partitionSight) candidates(lo, hi spatial.Point, ask func(partitionSightItem) (bool, error)) (bool, error) {
	s.stamp++
	try := func(i int) (bool, error) {
		if s.seen[i] == s.stamp {
			return false, nil
		}
		s.seen[i] = s.stamp
		return ask(s.items[i])
	}
	for _, i := range s.always {
		if hit, err := try(i); hit || err != nil {
			return hit, err
		}
	}
	if !finite(lo.X) || !finite(lo.Y) || !finite(hi.X) || !finite(hi.Y) {
		for i := range s.items {
			if hit, err := try(i); hit || err != nil {
				return hit, err
			}
		}
		return false, nil
	}
	for x := bucketOf(lo.X); x <= bucketOf(hi.X); x++ {
		for y := bucketOf(lo.Y); y <= bucketOf(hi.Y); y++ {
			for _, i := range s.buckets[[2]int{x, y}] {
				if hit, err := try(i); hit || err != nil {
					return hit, err
				}
			}
		}
	}
	return false, nil
}

// edgeBlocked is [canvasRoom.IsLineOfSightBlocked] for two adjacent cells,
// read through the index; any other pair is asked of the canvas itself. A
// trace error fails closed, as the canvas does.
func (s *partitionSight) edgeBlocked(from, to spatial.Position) bool {
	c := s.canvas
	grid := c.GetGrid()
	if grid.Distance(from, to) != 1 {
		return c.IsLineOfSightBlocked(from, to)
	}
	ray := spatial.CanonicalBoundaryRay(grid, from, to)
	if c.field.hasVoid() && c.field.void.blocksSight() {
		for _, cell := range ray {
			if !c.field.isFloor(cell) {
				return true
			}
		}
	}
	for i := 1; i < len(ray); i++ {
		if b, ok := c.GetBoundary(ray[i-1], ray[i]); ok && b.BlocksLineOfSight {
			return true
		}
	}
	for i := 1; i < len(ray)-1; i++ {
		for _, ent := range c.GetEntitiesAt(ray[i]) {
			if placeable, ok := ent.(spatial.Placeable); ok && placeable.BlocksLineOfSight() {
				return true
			}
		}
	}
	a, b := c.field.plane.CellCentre(from), c.field.plane.CellCentre(to)
	lo, hi := a, b
	if lo.X > hi.X {
		lo.X, hi.X = hi.X, lo.X
	}
	if lo.Y > hi.Y {
		lo.Y, hi.Y = hi.Y, lo.Y
	}
	blocked, err := s.candidates(lo, hi, func(it partitionSightItem) (bool, error) {
		if !it.reach.mayTouch(it.placement.Origin, a, b) {
			return false, nil
		}
		trace, err := spatial.TraceFootprint(spatial.FootprintTraceInput{Placement: it.placement, From: a, To: b})
		if err != nil {
			return false, fmt.Errorf("%s: %w", it.name, err)
		}
		return trace.Interior, nil
	})
	return err != nil || blocked
}

// opaqueAt is [canvasSightObstructions.At] read through the index: an
// occluding entity on the cell, or a sight-blocking contributor whose
// stationary contact covers its centre.
func (s *partitionSight) opaqueAt(cell spatial.Position) (bool, error) {
	for _, ent := range s.canvas.GetEntitiesAt(cell) {
		if placeable, ok := ent.(spatial.Placeable); ok && placeable.BlocksLineOfSight() {
			return true, nil
		}
	}
	centre := s.canvas.field.plane.CellCentre(cell)
	return s.candidates(centre, centre, func(it partitionSightItem) (bool, error) {
		if !it.reach.mayTouch(it.placement.Origin, centre, centre) {
			return false, nil
		}
		contact, err := spatial.TraceFootprint(spatial.FootprintTraceInput{Placement: it.placement, From: centre, To: centre})
		if err != nil {
			return false, fmt.Errorf("%s: %w", it.name, err)
		}
		return contact.Contact, nil
	})
}
