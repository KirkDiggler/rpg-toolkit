// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package encounter

// door_openings_internal_test.go pins DOOR CROSSINGS ARE SITE DATA
// (rpg-project#879 S9, F17): a footprint door's crossings depend on the floor
// and the door's placement, so they are computed once on the field and read
// after that. The field's answer is the whole-floor scan's answer, a door the
// content adds or removes changes it, a door re-authored under the same id is
// asked again, and a shallow copy of the field never writes the live field's
// memo.

import (
	"reflect"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/KirkDiggler/rpg-toolkit/tools/spatial"
)

// wholeFloorDoorCrossings is the oracle: [field.doorCrossings] as it was
// before S9, every cell of the floor asked of the door.
func wholeFloorDoorCrossings(f *field, d *doorRecord) [][2]spatial.Position {
	if d.placement == nil {
		return nil
	}
	var out [][2]spatial.Position
	for _, cell := range f.cells {
		a := f.plane.CellCentre(cell)
		for _, next := range adjacencyGrid.GetNeighbors(cell) {
			if !cellBefore(cell, next) || !f.isFloor(next) {
				continue
			}
			b := f.plane.CellCentre(next)
			trace, err := spatial.TraceFootprint(spatial.FootprintTraceInput{
				Placement: *d.placement, From: a, To: b,
			})
			if err == nil && trace.Interior {
				out = append(out, [2]spatial.Position{cell, next})
			}
		}
	}

	return out
}

// wholeFloorOpening is the oracle's crossings normalized, as
// [field.doorOpening] holds them.
func wholeFloorOpening(f *field, d *doorRecord) []DoorEdge {
	out := make([]DoorEdge, 0, len(d.edges))
	for _, edge := range d.edges {
		out = append(out, normalizeDoorEdge(edge))
	}
	for _, crossing := range wholeFloorDoorCrossings(f, d) {
		out = append(out, normalizeDoorEdge(DoorEdge{From: crossing[0], To: crossing[1]}))
	}

	return out
}

// leafAt is a footprint door record under id at origin and facing, its
// reach measured as construction measures it.
func leafAt(id DoorID, origin spatial.Point, facing float64, box spatial.Box) *doorRecord {
	p := spatial.FootprintPlacement{Footprint: spatial.Footprint{Box: &box}, Origin: origin, Facing: facing}

	return &doorRecord{id: id, placement: &p, reach: boundsOf(p), state: DoorIsClosed()}
}

func TestTheFieldsCrossingsAreTheWholeFloorScans(t *testing.T) {
	enc := holdHall(t, holdLeafDoor(DoorIsClosed()), holdSouthEast)
	f := enc.field
	asked, crossed := 0, 0
	for x := -5.0; x <= 60; x += 3.7 {
		for y := -5.0; y <= 30; y += 2.9 {
			for _, facing := range []float64{0, 30, 90, 135} {
				for _, box := range []spatial.Box{{W: 1, D: 6}, {W: 2, D: 11}, {W: 0.5, D: 3}, {W: 1, D: 40}} {
					d := leafAt("probe", spatial.Point{X: x, Y: y}, facing, box)
					want := wholeFloorDoorCrossings(f, d)
					require.Equal(t, want, f.doorCrossings(d),
						"a leaf at (%v,%v) facing %v, %vx%v: the cells near the door answer as the whole floor did, in its order",
						x, y, facing, box.W, box.D)
					asked++
					if len(want) > 0 {
						crossed++
					}
				}
			}
		}
	}
	require.Positive(t, crossed, "precondition: some probes lie across crossings")
	require.Less(t, crossed, asked, "precondition: some probes lie across none")
}

func TestADoorInTheContentChangesTheOpenings(t *testing.T) {
	with := holdHall(t, holdLeafDoor(DoorIsClosed()), holdSouthEast)
	require.Equal(t, wholeFloorOpening(with.field, with.doorsByID[holdLeaf]), with.doorOpening(holdLeaf),
		"the leaf's opening is the scan's")
	require.NotEmpty(t, with.doorOpening(holdLeaf), "precondition: the leaf lies across crossings")

	without := holdHall(t, holdEdgeDoor(DoorIsClosed()), holdSouthEast)
	require.Empty(t, without.doorOpening(holdLeaf), "a door the content does not author opens nothing")

	atlas, err := without.Atlas()
	require.NoError(t, err)
	for _, dw := range atlas.Doorways {
		require.False(t, dw.Crossing, "no footprint door, no crossing doorway: %+v", dw)
	}
}

// unsited is enc's field as a field compiled outside a site — a validation
// seam's — whose openings are its own to measure when doors attach.
func unsited(enc *Encounter) *field {
	f := *enc.field
	f.openingsAreSite = false
	return &f
}

func TestAReAuthoredDoorIsAskedAgain(t *testing.T) {
	enc := holdHall(t, holdLeafDoor(DoorIsClosed()), holdSouthEast)
	f := unsited(enc)
	before := f.doorOpening(enc.doorsByID[holdLeaf])
	require.NotEmpty(t, before, "precondition: the leaf lies across crossings")

	// The same id, a rectangle two cells east: new door content.
	moved := leafAt(holdLeaf, spatial.Point{X: 21.25 + 2*placedPlaneCellWidth*0.866, Y: 2.165}, 0, spatial.Box{W: 1, D: 6})
	f.attachDoorFootprints([]*doorRecord{moved})
	after := f.doorOpening(moved)
	require.Equal(t, wholeFloorOpening(f, moved), after, "the re-authored door's opening is its own scan's")
	require.NotEmpty(t, after, "precondition: the moved leaf lies across crossings")
	require.NotEqual(t, before, after, "not the opening the old rectangle had")
}

func TestAShallowCopyNeverWritesTheFieldsOpenings(t *testing.T) {
	enc := holdHall(t, holdLeafDoor(DoorIsClosed()), holdSouthEast)
	f := unsited(enc)
	want := f.doorOpenings[holdLeaf]
	require.NotEmpty(t, want, "precondition: the leaf's opening was built when the doors attached")

	copied := *f
	other := leafAt("other", spatial.Point{X: 30, Y: 10}, 90, spatial.Box{W: 1, D: 6})
	copied.attachDoorFootprints([]*doorRecord{other})

	require.Contains(t, copied.doorOpenings, DoorID("other"), "precondition: the copy holds the doors it attached")
	require.NotContains(t, f.doorOpenings, DoorID("other"), "the live field's openings are not the copy's to write")
	require.Equal(t, want, f.doorOpenings[holdLeaf], "and the live field still holds its own door's")
}

func TestADoorsOpeningIsBuiltWhenTheDoorsAttach(t *testing.T) {
	enc := holdHall(t, holdLeafDoor(DoorIsClosed()), holdSouthEast)
	f := enc.field
	d := enc.doorsByID[holdLeaf]
	built, ok := f.doorOpenings[holdLeaf]
	require.True(t, ok, "the opening is in the field before anybody asks")
	require.Equal(t, wholeFloorOpening(f, d), built, "and it is the scan's")

	// A sentinel in the built map is what a read returns: the read does not
	// scan again.
	sentinel := []DoorEdge{{From: spatial.Position{X: 99, Y: 99}, To: spatial.Position{X: 100, Y: 99}}}
	f.doorOpenings = map[DoorID][]DoorEdge{holdLeaf: sentinel}
	require.Equal(t, sentinel, f.doorOpening(d), "a read is the built map's")
}

func TestAnUnattachedDoorIsMeasuredAndNothingIsWritten(t *testing.T) {
	enc := holdHall(t, holdLeafDoor(DoorIsClosed()), holdSouthEast)
	f := enc.field
	stray := leafAt("stray", spatial.Point{X: 30, Y: 10}, 90, spatial.Box{W: 1, D: 6})

	require.Equal(t, wholeFloorOpening(f, stray), f.doorOpening(stray), "a door never attached is answered by measuring it")
	require.NotContains(t, f.doorOpenings, DoorID("stray"), "and the read writes nothing")
}

// TestASitesOpeningsAreKeptByEveryInstance (rpg-project#879 S5): the site
// measures its doors once; an encounter attaching its own door records keeps
// the site's map, the same map, and two encounters on one site share it.
// Mutant: an attach that measures again (the instance's map is a new one).
func TestASitesOpeningsAreKeptByEveryInstance(t *testing.T) {
	a := holdHall(t, holdLeafDoor(DoorIsClosed()), holdSouthEast)
	b := holdHall(t, holdLeafDoor(DoorIsOpen()), holdSouthEast)
	require.Same(t, a.site, b.site, "precondition: one content, one site, whatever the doors' states")
	require.NotEmpty(t, a.site.field.doorOpenings[holdLeaf], "precondition: the site measured the leaf")
	require.Equal(t, reflect.ValueOf(a.site.field.doorOpenings).Pointer(), reflect.ValueOf(a.field.doorOpenings).Pointer(),
		"the instance reads the site's map")
	require.Equal(t, reflect.ValueOf(a.field.doorOpenings).Pointer(), reflect.ValueOf(b.field.doorOpenings).Pointer(),
		"and so does every other instance on the site")
}
