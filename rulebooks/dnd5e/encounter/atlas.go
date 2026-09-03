// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package encounter

import (
	"fmt"
	"sort"

	"github.com/KirkDiggler/rpg-toolkit/tools/spatial"
)

// Atlas is a deterministic, construction-time snapshot of the field in
// dungeon-absolute space: every floor cell, every region and the cells it
// owns, and every prop, wall and doorway standing on the floor
// (rpg-project#256).
//
// FLAT, because the field is. Walls, props and doorways are field-level facts
// now (they were grouped under the room that declared them while rooms had
// origins), so the shape a host reads is the shape the session seam used to
// have to build from this one: one sorted list per kind, in one coordinate
// order, with the regions beside them rather than around them.
type Atlas struct {
	// Orientation is which way this field's hexes point.
	//
	// Reported because every cell below is axial, and a host laying the
	// floor out needs the same frame the encounter used to know where on
	// the screen an axial cell lands.
	Orientation Orientation

	// Cells is every floor cell, sorted by coordinate: the union of every
	// region's cells AND the field's scenery (rpg-project#360).
	//
	// A CELL HERE AND IN NO REGION BELOW IS SCENERY — floor nobody owns and
	// nobody stands on. That is the whole of what a host needs to be told
	// about it, which is why nothing else on this snapshot names it: the two
	// lists already say it, and a third statement of the same fact is a third
	// place for it to be wrong.
	Cells []spatial.Position

	// Regions is every region, sorted by region ID (C8), each listing its
	// own cells sorted the same way Cells is.
	Regions []AtlasRegion

	// Props is every authored thing standing on the floor, sorted by cell
	// then ref. See [AtlasProp].
	Props []AtlasProp

	// Boundaries is every authored wall, both endpoints absolute and
	// normalized (From before To in coordinate order), sorted by From then
	// To — the SAME edges compileCanvas registers, so what a host draws and
	// what the encounter enforces are identical.
	Boundaries []AtlasBoundary

	// Doorways is every door's every edge, sorted by door ID then cell. A
	// doorway is two adjacent floor cells with a door on the edge between
	// them; what state that door is in is [Encounter.Doors]' business, not
	// a snapshot's.
	Doorways []AtlasDoorway
}

// AtlasRegion is one region: a NAMED SET OF CELLS, enumerated, with the
// per-area world facts it carries.
type AtlasRegion struct {
	// ID is the region's identifier.
	ID RegionID

	// Name is the region's display name, carried verbatim.
	Name string

	// Cells is every cell the region owns, dungeon-absolute, sorted by
	// coordinate.
	Cells []spatial.Position

	// Archetype is the presentation profile the assets resolve, carried
	// unread. It NEVER decides a mechanic — see [RegionInput].
	Archetype string

	// Lighting is the region's light level, carried unread.
	Lighting Lighting

	// Concealed is whether the region is authored as hidden space, carried
	// unread — [RegionInput.Concealed]. What a non-knower's atlas withholds
	// is the world layer's business (rpg-project#351), not a snapshot's.
	Concealed bool
}

// AtlasProp is one authored thing standing on the floor, as the map reports
// it: what it is, where it stands in dungeon-absolute space, and what it does
// to a step and to a sightline. See [PropInput] for the authoring side.
type AtlasProp struct {
	// Ref is content's identifier for this thing, carried through the
	// compile unchanged and never interpreted by this module.
	Ref string

	// At is where it stands, in dungeon-absolute space.
	At spatial.Position

	// BlocksMovement is whether a member can end a step on this cell.
	BlocksMovement bool

	// BlocksLineOfSight is whether it obstructs a sightline — subject to
	// spatial's lane rule, so one cell of it obstructs nothing on its own
	// ([PropInput]).
	BlocksLineOfSight bool

	// Facing and Offset are the same authored, uninterpreted presentational
	// facts as [PropInput.Facing] and [PropInput.Offset], carried through
	// unread. Neither is validated here either — dungeonspec is the layer
	// that owns the vocabulary and the bounds.
	Facing string
	Offset [3]float64
}

// AtlasBoundary is one wall or barrier crossing, with both endpoints in
// dungeon-absolute space.
type AtlasBoundary struct {
	// From is one endpoint of the crossing, in dungeon-absolute space.
	From spatial.Position

	// To is the other endpoint of the crossing, in dungeon-absolute space.
	To spatial.Position

	// BlocksMovement reports whether an entity may cross this boundary.
	BlocksMovement bool

	// BlocksLineOfSight reports whether line of sight may cross this boundary.
	BlocksLineOfSight bool

	// Height is the authored wall-height multiplier, carried verbatim from
	// [WallInput.Height] and unread by this module. 0 = not authored =
	// standard height; see [WallInput.Height] for the full contract.
	Height float64
}

// AtlasDoorway is one crossable pair of cells a door stands in.
//
// Both cells are floor and adjacent — a doorway is an opening in a wall, not
// a cell of its own. See [Encounter.RegionAt] for what that means for
// somebody standing in one.
type AtlasDoorway struct {
	// Door is the door's identifier.
	Door DoorID

	// From is one of the two cells, in dungeon-absolute space.
	From spatial.Position

	// To is the other, adjacent to From.
	To spatial.Position
}

// Atlas returns a deterministic, construction-time snapshot of the field in
// dungeon-absolute space. Computed ONLY from construction data — the same
// compiled field ToData persists — never from live member placement, door
// state or clock, so placing or moving a member, opening a door or Pumping a
// tick never changes it (#929 T3 ruling 3; [Encounter.Doors] for why door
// state is read elsewhere).
//
// Deterministic (C8): every list sorted in one coordinate order, Regions by
// ID. Copy-out: every returned slice is freshly allocated per call; mutating
// the result never reaches internal state.
//
// O(cells) per call, which is what an enumerated floor costs and is the
// honest shape of it: a region IS its cells now, so the list the host wants
// is the list the composition already holds, copied. The field's cell budget
// (maxFieldCells) is what bounds this.
func (e *Encounter) Atlas() (Atlas, error) {
	f := e.field
	out := Atlas{
		Orientation: f.orientation,
		Cells:       append([]spatial.Position(nil), f.cells...),
		Regions:     make([]AtlasRegion, 0, len(f.regions)),
		Props:       make([]AtlasProp, 0, len(f.props)),
		Boundaries:  make([]AtlasBoundary, 0, len(f.walls)),
		Doorways:    make([]AtlasDoorway, 0, len(e.doors)),
	}

	for _, r := range f.regions {
		out.Regions = append(out.Regions, AtlasRegion{
			ID:        r.ID,
			Name:      r.Name,
			Cells:     append([]spatial.Position(nil), f.regionCells[r.ID]...),
			Archetype: r.Archetype,
			Lighting:  *r.Lighting,
			Concealed: r.Concealed,
		})
	}
	sort.Slice(out.Regions, func(i, j int) bool { return out.Regions[i].ID < out.Regions[j].ID })

	for _, p := range f.props {
		out.Props = append(out.Props, AtlasProp{
			Ref:               p.Ref,
			At:                f.cellAt(p.At),
			BlocksMovement:    *p.BlocksMovement,
			BlocksLineOfSight: *p.BlocksLineOfSight,
			Facing:            p.Facing,
			Offset:            p.Offset,
		})
	}
	sort.Slice(out.Props, func(i, j int) bool {
		if out.Props[i].At != out.Props[j].At {
			return cellBefore(out.Props[i].At, out.Props[j].At)
		}
		return out.Props[i].Ref < out.Props[j].Ref
	})

	for _, w := range f.walls {
		edge := normalizeDoorEdge(DoorEdge{From: f.cellAt(w.From), To: f.cellAt(w.To)})
		out.Boundaries = append(out.Boundaries, AtlasBoundary{
			From:              edge.From,
			To:                edge.To,
			BlocksMovement:    w.BlocksMovement,
			BlocksLineOfSight: w.BlocksLineOfSight,
			Height:            w.Height,
		})
	}
	sort.Slice(out.Boundaries, func(i, j int) bool {
		if out.Boundaries[i].From != out.Boundaries[j].From {
			return cellBefore(out.Boundaries[i].From, out.Boundaries[j].From)
		}
		return cellBefore(out.Boundaries[i].To, out.Boundaries[j].To)
	})

	// e.doors is already sorted by ID (doorRecordsFrom) and every edge
	// normalized; the sort below keeps Atlas's own determinism
	// self-contained rather than coupled to that invariant.
	for _, d := range e.doors {
		for _, edge := range d.edges {
			out.Doorways = append(out.Doorways, AtlasDoorway{Door: d.id, From: edge.From, To: edge.To})
		}
	}
	sort.Slice(out.Doorways, func(i, j int) bool {
		a, b := out.Doorways[i], out.Doorways[j]
		if a.Door != b.Door {
			return a.Door < b.Door
		}
		if a.From != b.From {
			return cellBefore(a.From, b.From)
		}
		return cellBefore(a.To, b.To)
	})

	return out, nil
}

// Grid reports the field's coordinate family, in O(1).
//
// Always [spatial.GridShapeHex] as of rpg-project#256: the square family left
// with the room chain, and a region is painted on a hex grid. Kept as a read
// because a caller doing grid arithmetic of its own should learn which
// arithmetic to do from the field rather than assume it — the two families
// disagree about what one step means, and Chebyshev distance on axial
// coordinates passes almost every fixture while being wrong on the diagonals.
//
// Returns ErrNoField on the zero value, which construction forbids.
func (e *Encounter) Grid() (spatial.GridShape, error) {
	if e.field == nil {
		return spatial.GridShapeHex, fmt.Errorf("grid: %w", ErrNoField)
	}
	return spatial.GridShapeHex, nil
}
