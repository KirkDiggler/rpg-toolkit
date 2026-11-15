// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package encounter

import (
	"sort"

	"github.com/KirkDiggler/rpg-toolkit/tools/spatial"
)

// floor_topology.go is the compile-time face of WHERE THE AUTHORED REGIONS
// MEET (rpg-project#971): the crossings between two regions' cells that
// nothing fixed closes, the crossings each door stands in, and the cells each
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
// a footprint door's traced crossings ([DoorInput]).
type DoorCrossings struct {
	Door      DoorID
	Crossings []FloorCrossing
}

// PlacedCells is the floor cells one placed prop stands on: those whose
// centre its footprint covers, else the one nearest its centre — the same
// cells play stands it on.
type PlacedCells struct {
	Prop  PropID
	Cells []spatial.Position
}

// FloorTopologyOutput is the field's region seams, door crossings and placed
// props' cells, in the input's authored cell frame. Every list is in a stable
// order: crossings by From then To, doors and props by id.
type FloorTopologyOutput struct {
	// Open is every crossing between a cell of one region and an adjacent
	// cell of ANOTHER region that nothing fixed closes to sight: no wall, no
	// door (every door measured closed, as [PartitionFloor] measures them),
	// no permanent opaque prop. Two regions with an open crossing between
	// them are one wall partition. A crossing inside one region is not
	// listed.
	Open []FloorCrossing

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
			if !owned || other == own || sight.edgeBlocked(cell, next) {
				continue
			}
			if c, ok := crossing(cell, next); ok {
				out.Open = append(out.Open, c)
			}
		}
	}
	sortCrossings(out.Open)

	for _, d := range doors {
		dc := DoorCrossings{Door: d.id}
		for _, edge := range f.measureDoorOpening(d) {
			if c, ok := crossing(edge.From, edge.To); ok {
				dc.Crossings = append(dc.Crossings, c)
			}
		}
		sortCrossings(dc.Crossings)
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

// sortCrossings orders crossings by From, then To, in the map's order.
func sortCrossings(cs []FloorCrossing) {
	sort.Slice(cs, func(i, j int) bool {
		if cs[i].From != cs[j].From {
			return cellBefore(cs[i].From, cs[j].From)
		}
		return cellBefore(cs[i].To, cs[j].To)
	})
}
