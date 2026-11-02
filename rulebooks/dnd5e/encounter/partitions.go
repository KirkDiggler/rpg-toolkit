// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package encounter

import (
	"errors"
	"fmt"
	"sort"

	"github.com/KirkDiggler/rpg-toolkit/tools/spatial"
)

// partitions.go is the field's SECOND DIVISION (rpg-project#725, design R4
// and R8).
//
// A field's floor is divided twice, and the two divisions are independent.
// The AUTHORED REGIONS ([RegionInput]) own presentation: a name, a lighting,
// what the atlas groups a cell under and what [Encounter.RegionAt] answers.
// The WALL PARTITIONS own mechanics: the floor split wherever sight cannot
// cross — walls, every door treated as closed, permanent opaque props — so a
// partition is a room as the walls make it. Hearing hops, the party's seats,
// where a blocked arrival looks for room and the intel flip read the
// partition; nothing reads an authored region to decide an outcome, so an
// author who splits an open hall into two named places changes how it is lit
// and labelled and nothing about how sound or sight crosses it.
//
// DERIVED, NEVER STORED (R8). The partitions are computed from the authored
// regions, walls, props and doors at construction ([NewEncounter]) and, after
// a load ([LoadEncounter]), on the first read that needs them
// ([partitionMemo], rpg-project#762 T13), and [FieldData] carries none of it: its
// charter is authored truth only, and a derived division written beside the
// thing it is derived from is a second answer waiting to disagree. Their ids
// ([PartitionID]) are internal for the same reason — no atlas, beat or wire
// type carries one.

// PartitionID names one wall partition of a field: `p0`, `p1`, … in the
// canonical axial order of each partition's first cell. Internal and derived
// — it is never persisted and never sent — so it is stable only for the life
// of one compiled field.
type PartitionID string

// PartitionFloorInput names the complete static field whose floor is to be
// partitioned. No live encounter, roster or knowledge is needed. Door
// geometry is treated as closed regardless of its initial state.
type PartitionFloorInput struct {
	Field FieldInput
}

// PartitionFloorOutput contains the floor's partitions, with unambiguous
// opaque footing assigned to its adjoining partition, in the INPUT'S authored
// cell frame. Components and their cells are ordered by canonical axial cell;
// returned slices do not alias the input.
type PartitionFloorOutput struct {
	Components [][]spatial.Position
	// Footing is permanently occupied opaque floor that cannot be assigned
	// to one adjoining partition. A compiler carries it as scenery, not as
	// an undiscoverable room. It excludes doors and movable/reserved props.
	Footing []spatial.Position
}

// PartitionFloor groups every region's cells — the whole floor, across
// authored region boundaries — by clear adjacent sight crossings through the
// same compiled canvas geometry used in play. It is the compile-time face of
// the division the field derives for itself at construction and load: a
// builder asks it for the footing to carry as scenery and for the partition
// the party's seats stay in.
//
// Doors are topology boundaries even when authored open, so opening or
// closing one cannot change a partition. Input geometry and state are
// untouched. Invalid field or door declarations are refused by the normal
// construction gates. Any failure returns a zero output.
func PartitionFloor(in PartitionFloorInput) (PartitionFloorOutput, error) {
	f, err := compileField(in.Field)
	if err != nil {
		return PartitionFloorOutput{}, err
	}
	if err := validateDoorInputs(f, in.Field.Doors); err != nil {
		return PartitionFloorOutput{}, err
	}
	groups, footing, err := f.partitionFloor(closedDoors(in.Field.Doors))
	if err != nil {
		return PartitionFloorOutput{}, err
	}
	authored := make(map[spatial.Position]spatial.Position)
	for _, region := range in.Field.Regions {
		for _, cell := range region.Cells {
			authored[f.cellAt(cell)] = cell
		}
	}
	out := PartitionFloorOutput{Components: make([][]spatial.Position, 0, len(groups))}
	for _, cell := range footing {
		out.Footing = append(out.Footing, authored[cell])
	}
	for _, group := range groups {
		cells := make([]spatial.Position, 0, len(group))
		for _, cell := range group {
			cells = append(cells, authored[cell])
		}
		out.Components = append(out.Components, cells)
	}
	return out, nil
}

// fieldPartitions is the division itself as the field reads it: every owned
// cell to the wall partition holding it, and each partition's cells,
// absolute axial and sorted.
type fieldPartitions struct {
	of    map[spatial.Position]PartitionID
	cells map[PartitionID][]spatial.Position
}

// partitionMemo is the field's SECOND DIVISION, DERIVED WHEN ASKED
// (rpg-project#762 T13). A load used to pay the whole partition pass —
// ~45 ms of ~87 on the 10k keep — before the verb it served had asked
// anything of it, and a plain Move asks nothing: the partitions are read
// where a sound hops, a mind is in a carrier's presence, an arrival is
// bumped off its cell, a listener is told how far away a beat landed, or a
// secret threshold is presented. So the memo holds what the pass needs —
// the field's door geometry, every door closed, snapshot at construction —
// and derives on the first read, ONCE for the instance's life.
//
// IN THE INSTANCE, NEVER IN THE RECORD. The law is unchanged: the field
// indexes its placements and stores no derived data. [FieldData] carries
// none of this; the memo dies with the instance (today, the verb), and a
// longer-lived instance (rpg-project#770) keeps it longer for free.
//
// NOTHING INVALIDATES IT. The division reads only the field's fixed
// geometry — immutable placed contributors and props, and every door as
// closed whatever its state — so opening, closing or locking a door, a
// member's walk, or a holdable prop picked up cannot change a partition
// ([PartitionFloor]'s own contract). What a door's state does change is how
// many hops a sound pays crossing it, which [Encounter.hopsFrom] reads live.
//
// NO LOCK. The encounter is one instance driven by one verb on one goroutine;
// a lazily derived value there needs no mutex. Were an encounter ever read
// from two goroutines at once, that would be a finding about the caller, not
// a reason to lock here.
//
// A FAILED DERIVATION IS REMEMBERED and returned to every reader: the pass is
// deterministic over the same geometry, so asking again would fail the same
// way. Every verb that decides on a partition surfaces it.
type partitionMemo struct {
	doors       []*doorRecord // the construction-time doors, every one closed
	derived     *fieldPartitions
	err         error
	derivations int // how many times the pass has run: once, or never
}

// closedDoors is the door geometry the partition pass reads: fresh records
// (the input's edges and placement copied), every one closed.
func closedDoors(doors []DoorInput) []*doorRecord {
	records, _ := doorRecordsFrom(doors)
	for _, door := range records {
		door.state = DoorIsClosed()
	}
	return records
}

// deferPartitions gives the field its partition memo without running the
// pass: the load seam ([LoadEncounter]) calls it after the doors are
// validated, and the first reader derives ([field.partitionsNow]).
func (f *field) deferPartitions(doors []DoorInput) {
	f.partitions = &partitionMemo{doors: closedDoors(doors)}
}

// derivePartitions gives the field its partition memo and runs the pass now,
// returning its error. The construction seam ([NewEncounter]) calls it so a
// field that cannot be divided is refused where it is authored; the same
// field loaded from its data defers the same pass and answers
// [field.partitionAt] identically.
func (f *field) derivePartitions(doors []DoorInput) error {
	f.deferPartitions(doors)
	if _, err := f.partitionsNow(); err != nil {
		return err
	}
	return nil
}

// errNoPartitionMemo is a field asked for its partitions that was compiled
// for a question that does not need them (validation, static placements):
// a bug in the asker, refused rather than answered "nowhere".
var errNoPartitionMemo = errors.New("the field was compiled without its partitions")

// partitionsNow is THE ONE ACCESSOR to the field's wall partitions: derived
// on the first call, memoized for the instance's life ([partitionMemo]).
//
// Footing — permanent opaque floor bordering no single partition — is its own
// partition, cell by cell: nobody stands there, so it can neither hear nor be
// arrived on, and it is still owned floor. A compiled site has none (the
// builder moved it to scenery); a field authored in code may.
func (f *field) partitionsNow() (*fieldPartitions, error) {
	m := f.partitions
	if m == nil {
		return nil, errNoPartitionMemo
	}
	if m.derived != nil || m.err != nil {
		return m.derived, m.err
	}
	m.derivations++
	groups, footing, err := f.partitionFloor(m.doors)
	if err != nil {
		m.err = fmt.Errorf("partition the floor: %w", err)
		return nil, m.err
	}
	for _, cell := range footing {
		groups = append(groups, []spatial.Position{cell})
	}
	sort.Slice(groups, func(i, j int) bool { return cellBefore(groups[i][0], groups[j][0]) })
	out := &fieldPartitions{
		of:    make(map[spatial.Position]PartitionID),
		cells: make(map[PartitionID][]spatial.Position, len(groups)),
	}
	for i, group := range groups {
		id := PartitionID(fmt.Sprintf("p%d", i))
		out.cells[id] = group
		for _, cell := range group {
			out.of[cell] = id
		}
	}
	m.derived = out
	return out, nil
}

// partitionAt reports which wall partition holds an absolute cell — THE
// partition lookup, the mechanical twin of [field.regionOf]. False for a cell
// that is not owned floor; an error only when the partitions cannot be
// derived ([field.partitionsNow]).
func (f *field) partitionAt(cell spatial.Position) (PartitionID, bool, error) {
	parts, err := f.partitionsNow()
	if err != nil {
		return "", false, err
	}
	id, ok := parts.of[cell]
	return id, ok, nil
}

// partitionFloor is the division itself, in ABSOLUTE cells: every owned cell
// grouped by clear sight crossings over a canvas built from the field's FIXED
// geometry — immutable placed contributors and props only, every door closed —
// then opaque footing joined to its one clear neighbour or returned apart.
// The field itself is not edited: the canvas is built on a shallow copy whose
// contributor lists are fresh.
func (f *field) partitionFloor(doors []*doorRecord) ([][]spatial.Position, []spatial.Position, error) {
	canvas, err := f.partitionCanvas(doors)
	if err != nil {
		return nil, nil, err
	}
	// THE LOCAL INDEX (rpg-project#725, R8). Built once over the fixed
	// contributors this canvas holds, so each edge and each cell asks only
	// the few whose reach is near it; see [partitionSight].
	sight := newPartitionSight(canvas)
	var floor []spatial.Position
	for _, cells := range f.regionCells {
		floor = append(floor, cells...)
	}
	sortCells(floor)
	visited := make(map[spatial.Position]bool, len(floor))
	groups := make([][]spatial.Position, 0)
	for _, seed := range floor {
		if visited[seed] {
			continue
		}
		component, err := spatial.Field(canvas.GetGrid(), spatial.FieldInput{
			Sources: []spatial.Position{seed},
			Passable: func(from, to spatial.Position) bool {
				_, owned := f.regionOf(to)
				return owned && !sight.edgeBlocked(from, to)
			},
		})
		if err != nil {
			return nil, nil, err
		}
		group := make([]spatial.Position, 0, len(component.Dist))
		for cell := range component.Dist {
			group = append(group, cell)
			visited[cell] = true
		}
		sortCells(group)
		groups = append(groups, group)
	}
	return joinOpaqueFooting(canvas, groups, sight.opaqueAt)
}

// partitionCanvas is the canvas the partition pass reads: built on a shallow
// copy of the field whose contributor lists are fresh and hold only the
// FIXED ones — immutable placed contributors and props — with the doors it
// is given, every one closed ([closedDoors]). The field itself is not edited.
func (f *field) partitionCanvas(doors []*doorRecord) (*canvasRoom, error) {
	fixed := *f
	fixed.holdings = nil
	fixed.sightPass = nil
	fixed.placed = make([]placedContributor, 0, len(f.placed))
	for _, p := range f.placed {
		if !p.mutable() {
			fixed.placed = append(fixed.placed, p)
		}
	}
	// The reach buckets name contributors by their position in the list, so
	// the copy's shorter list gets its own; the rest of the index is by id.
	if f.index != nil {
		idx := *f.index
		idx.reach = fixed.buildPlacedReach()
		fixed.index = &idx
	}
	fixed.props = make([]PropInput, 0, len(f.props))
	for _, p := range f.props {
		if !p.Holdable && p.Arrives == nil {
			fixed.props = append(fixed.props, p)
		}
	}
	return fixed.compileCanvas(doors, nil)
}

// permanentOpaqueFootingAt uses the same centre-contact standing rule as the
// field, restricted to immutable opaque contributors. Door states are excluded:
// their floor must remain owned so it can be stood on after opening.
func (f *field) permanentOpaqueFootingAt(cell spatial.Position) (bool, error) {
	for _, p := range f.props {
		if !p.Holdable && p.Arrives == nil && p.BlocksMovement != nil && *p.BlocksMovement &&
			p.BlocksLineOfSight != nil && *p.BlocksLineOfSight && f.cellAt(p.At) == cell {
			return true, nil
		}
	}
	centre := f.plane.CellCentre(cell)
	for _, p := range f.placed {
		if p.mutable() || !p.blocksMovement || !p.blocksLineOfSight {
			continue
		}
		if !p.reach.mayTouch(p.placement.Origin, centre, centre) {
			continue
		}
		contact, err := spatial.TraceFootprint(spatial.FootprintTraceInput{Placement: p.placement, From: centre, To: centre})
		if err != nil {
			return false, err
		}
		if contact.Contact {
			return true, nil
		}
	}
	return false, nil
}

// An opaque pillar's own cell is not another room. Attach completely opaque
// footing only when it borders ONE clear component; never join two rooms through
// a wall/closed door. Permanently occupied ambiguous footing is scenery, never
// a phantom discovery region. Doors retain owned floor because they can open.
//
// opaqueAt is [canvasSightObstructions.At]'s answer for a cell — the indexed
// [partitionSight.opaqueAt] in the one caller.
func joinOpaqueFooting(
	canvas *canvasRoom, groups [][]spatial.Position, opaqueAt func(spatial.Position) (bool, error),
) ([][]spatial.Position, []spatial.Position, error) {
	owner := make(map[spatial.Position]int)
	opaque := make([]bool, len(groups))
	for i, cells := range groups {
		opaque[i] = true
		for _, cell := range cells {
			owner[cell] = i
			if !opaque[i] {
				continue
			}
			blocked, err := opaqueAt(cell)
			if err != nil {
				return nil, nil, err
			}
			opaque[i] = blocked
		}
	}
	joined := make([][]spatial.Position, len(groups))
	footing := make([]spatial.Position, 0)
	for i, cells := range groups {
		target, ambiguous := -1, false
		if opaque[i] {
			for _, cell := range cells {
				for _, neighbor := range canvas.GetGrid().GetNeighbors(cell) {
					other, owned := owner[neighbor]
					if !owned || opaque[other] {
						continue
					}
					if target >= 0 && target != other {
						ambiguous = true
					}
					target = other
				}
			}
		}
		if target < 0 || ambiguous {
			// A group with any clear cell is not permanent whatever its
			// cells hold, so nothing more is asked of it.
			permanent := opaque[i]
			for _, cell := range cells {
				if !permanent {
					break
				}
				fixed, err := canvas.field.permanentOpaqueFootingAt(cell)
				if err != nil {
					return nil, nil, err
				}
				permanent = permanent && fixed
			}
			if permanent {
				footing = append(footing, cells...)
				continue
			}
			target = i
		}
		joined[target] = append(joined[target], cells...)
	}
	out := make([][]spatial.Position, 0, len(joined))
	for _, cells := range joined {
		if len(cells) > 0 {
			sortCells(cells)
			out = append(out, cells)
		}
	}
	sort.Slice(out, func(i, j int) bool { return cellBefore(out[i][0], out[j][0]) })
	sortCells(footing)
	return out, footing, nil
}
