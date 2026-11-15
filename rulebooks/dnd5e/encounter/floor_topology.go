// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package encounter

import (
	"sort"

	"github.com/KirkDiggler/rpg-toolkit/tools/spatial"
)

// floor_topology.go is the compile-time face of WHERE THE AUTHORED REGIONS
// MEET (rpg-project#971): the crossings between two regions' cells that
// fixed geometry leaves open to feet, to sight or both, the crossings each door stands in, and the cells each
// placed prop stands on — measured on the same geometry play measures them
// on, and answered without a live encounter, the way [PartitionFloor] answers
// the partitions.
//
// It exists for a reader that says the site back (dungeonspec/readback): a
// footprint door's crossings and a placed prop's cells are traced in the
// field's own continuous plane, which no caller outside this package can
// build, so without this answer that reader would have to construct an
// [Encounter] — roster, sight, initiative and all — to learn which two rooms
// a door stands between.
//
// It decides nothing and is not read by play: what a crossing MEANS (a join,
// a room, a way through) is the caller's.

// FloorTopologyInput names the complete static field to measure. No live
// encounter, roster or knowledge is needed. Doors are measured as geometry,
// whatever state they start in.
type FloorTopologyInput struct {
	Field FieldInput
}

// FloorCrossing is one crossing between two adjacent floor cells, in the
// INPUT'S authored cell frame, From before To in the map's coordinate order.
type FloorCrossing struct {
	From spatial.Position
	To   spatial.Position
}

// DoorCrossings is the crossings one door stands in: an edge door's edges,
// a footprint door's traced crossings ([DoorInput]) — only those between two
// floor cells — and the floor cells the door stands on, so a door whose
// other side is off the floor still names the floor it is on.
type DoorCrossings struct {
	Door      DoorID
	Crossings []FloorCrossing
	// Cells is the floor the door stands on: an edge door's edge cells, a
	// footprint door's cells as a placed prop's are measured ([PlacedCells]).
	Cells []spatial.Position
}

// PlacedCells is the floor cells one placed prop stands on: those whose
// centre its footprint covers, else the one nearest its centre — the same
// cells play stands it on.
type PlacedCells struct {
	Prop  PropID
	Cells []spatial.Position
}

// RegionSeam is one crossing between a cell of one region and an adjacent
// cell of ANOTHER region, and what the field's FIXED geometry leaves open
// across it — walls, every door measured closed (as [PartitionFloor]
// measures them), and placed props and cell props that neither arrive nor
// can be carried. The two answers are independent because a wall or prop
// states them independently: a window lets sight across and not feet, a
// curtain feet and not sight.
type RegionSeam struct {
	FloorCrossing
	// Moves is whether a step can cross: no movement-blocking boundary on
	// it and no movement-blocking footprint across it — the fold a step and
	// a route ask, on the fixed geometry.
	Moves bool
	// Sees is whether sight crosses: the partition pass's own question, so
	// two regions with a seam that Sees are one wall partition.
	Sees bool
}

// FloorTopologyOutput is the field's region seams, door crossings and placed
// props' cells, in the input's authored cell frame. Every list is in a stable
// order: crossings by From then To, doors and props by id.
type FloorTopologyOutput struct {
	// Seams is every crossing between two different regions' adjacent cells
	// that fixed geometry leaves open to feet, to sight, or to both. A
	// crossing closed to both, and a crossing inside one region, is not
	// listed.
	Seams []RegionSeam

	// Doors is every door's crossings, whichever regions its two sides are
	// in — the same region on both sides included.
	Doors []DoorCrossings

	// Placed is every placed prop's cells ([FieldInput.Placed]). A cell
	// prop ([FieldInput.Props]) stands on its own authored cell and is not
	// listed.
	Placed []PlacedCells
}

// FloorTopology measures where a static field's regions meet. Invalid field
// or door declarations are refused by the normal construction gates, and any
// failure returns a zero output.
func FloorTopology(in FloorTopologyInput) (FloorTopologyOutput, error) {
	f, err := compileField(in.Field)
	if err != nil {
		return FloorTopologyOutput{}, err
	}
	if err := validateDoorInputs(f, in.Field.Doors); err != nil {
		return FloorTopologyOutput{}, err
	}
	doors := closedDoors(in.Field.Doors)
	canvas, err := f.partitionCanvas(doors)
	if err != nil {
		return FloorTopologyOutput{}, err
	}
	sight := newPartitionSight(canvas)

	// THE INPUT FRAME, as [PartitionFloor] returns it: every floor cell the
	// author wrote, region or scenery, keyed by its absolute cell.
	authored := make(map[spatial.Position]spatial.Position)
	for _, region := range in.Field.Regions {
		for _, cell := range region.Cells {
			authored[f.cellAt(cell)] = cell
		}
	}
	for _, cell := range in.Field.Scenery {
		authored[f.cellAt(cell)] = cell
	}
	crossing := func(a, b spatial.Position) (FloorCrossing, bool) {
		from, okFrom := authored[a]
		to, okTo := authored[b]
		if !okFrom || !okTo {
			return FloorCrossing{}, false
		}
		if cellBefore(to, from) {
			from, to = to, from
		}
		return FloorCrossing{From: from, To: to}, true
	}

	var out FloorTopologyOutput
	var floor []spatial.Position
	for _, cells := range f.regionCells {
		floor = append(floor, cells...)
	}
	sortCells(floor)
	for _, cell := range floor {
		own, _ := f.regionOf(cell)
		for _, next := range adjacencyGrid.GetNeighbors(cell) {
			if !cellBefore(cell, next) {
				continue
			}
			other, owned := f.regionOf(next)
			if !owned || other == own {
				continue
			}
			sees := !sight.edgeBlocked(cell, next)
			moves, err := movesAcross(canvas, cell, next)
			if err != nil {
				return FloorTopologyOutput{}, err
			}
			if !sees && !moves {
				continue
			}
			if c, ok := crossing(cell, next); ok {
				out.Seams = append(out.Seams, RegionSeam{FloorCrossing: c, Moves: moves, Sees: sees})
			}
		}
	}
	sort.Slice(out.Seams, func(i, j int) bool { return crossingBefore(out.Seams[i].FloorCrossing, out.Seams[j].FloorCrossing) })

	for _, d := range doors {
		dc := DoorCrossings{Door: d.id}
		for _, edge := range f.measureDoorOpening(d) {
			if c, ok := crossing(edge.From, edge.To); ok {
				dc.Crossings = append(dc.Crossings, c)
			}
		}
		sortCrossings(dc.Crossings)
		var on []spatial.Position
		for _, e := range d.edges {
			on = append(on, e.From, e.To)
		}
		if d.placement != nil {
			on = append(on, f.placedCells(*d.placement)...)
		}
		sortCells(on)
		for _, cell := range on {
			if at, ok := authored[cell]; ok && (len(dc.Cells) == 0 || dc.Cells[len(dc.Cells)-1] != at) {
				dc.Cells = append(dc.Cells, at)
			}
		}
		out.Doors = append(out.Doors, dc)
	}
	sort.Slice(out.Doors, func(i, j int) bool { return out.Doors[i].Door < out.Doors[j].Door })

	for _, p := range f.placed {
		pc := PlacedCells{Prop: p.id}
		for _, cell := range f.placedCells(p.placement) {
			if at, ok := authored[cell]; ok {
				pc.Cells = append(pc.Cells, at)
			}
		}
		out.Placed = append(out.Placed, pc)
	}
	sort.Slice(out.Placed, func(i, j int) bool { return out.Placed[i].Prop < out.Placed[j].Prop })

	return out, nil
}

// movesAcross is whether a step crosses between two adjacent cells on the
// fixed canvas: the fold [Encounter.crossingBlocked] makes for a step and a
// route — a movement-blocking boundary, or a movement-blocking footprint
// (a shut footprint door included) across the crossing — asked of the
// partition canvas, where every door is shut and only fixed things stand.
func movesAcross(canvas *canvasRoom, from, to spatial.Position) (bool, error) {
	if canvas.IsBoundaryMovementBlocked(from, to) {
		return false, nil
	}
	_, blocked, err := canvas.field.crossingBlocks(from, to)
	if err != nil {
		return false, err
	}
	return !blocked, nil
}

// crossingBefore orders crossings by From, then To, in the map's order.
func crossingBefore(a, b FloorCrossing) bool {
	if a.From != b.From {
		return cellBefore(a.From, b.From)
	}
	return cellBefore(a.To, b.To)
}

// sortCrossings orders crossings by From, then To, in the map's order.
func sortCrossings(cs []FloorCrossing) {
	sort.Slice(cs, func(i, j int) bool { return crossingBefore(cs[i], cs[j]) })
}
