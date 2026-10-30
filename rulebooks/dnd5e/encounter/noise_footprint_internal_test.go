// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package encounter

import (
	"math"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/KirkDiggler/rpg-toolkit/tools/spatial"
)

// TestADiagonalDoorJoinsOnlyTheCrossingItLiesAcross pins the interior trace
// behind a footprint door's hearing links (rpg-toolkit#2066), which the
// bounds pre-filter alone cannot answer for a door that is not axis-aligned.
//
// Four floor cells in one row, x0 x1 x2 x3, and a solid wall on the x2–x3
// edge: the walls make three rooms — a is x0 (the door, closed for topology,
// cuts it off), b is x1 and x2, c is x3 (rpg-project#725: hearing hops are
// wall partitions). The door is a long thin rectangle at 45° through the
// midpoint of the x0–x1 crossing. It lies across that crossing and no other,
// but its axis-aligned bounds reach out over the x2–x3 crossing too. Counting
// every crossing the bounds admit would join b to c through a door that
// stands nowhere near their seam.
func TestADiagonalDoorJoinsOnlyTheCrossingItLiesAcross(t *testing.T) {
	region := func(id string, cols ...int) RegionInput {
		r := rectRegion(id, 0, 0, 0, 0)
		for _, c := range cols {
			r.Cells = append(r.Cells, spatial.Position{X: float64(c), Y: 0})
		}
		return r
	}
	in := FieldInput{
		Canvas:  openAir(),
		Regions: []RegionInput{region("a", 0), region("b", 1, 2), region("c", 3)},
		Walls: []WallInput{{Boundary: spatial.Boundary{
			From: spatial.Position{X: 2, Y: 0}, To: spatial.Position{X: 3, Y: 0},
			BlocksMovement: true, BlocksLineOfSight: true,
		}}},
	}
	f, err := compileField(in)
	require.NoError(t, err)

	x := make([]spatial.Point, 4)
	for col := range x {
		x[col] = f.plane.CellCentre(f.cellAt(spatial.Position{X: float64(col), Y: 0}))
	}
	spacing := math.Hypot(x[1].X-x[0].X, x[1].Y-x[0].Y)
	mid := spatial.Point{X: (x[0].X + x[1].X) / 2, Y: (x[0].Y + x[1].Y) / 2}
	placement := spatial.FootprintPlacement{
		Footprint: spatial.Footprint{Box: &spatial.Box{W: spacing / 10, D: 5 * spacing}},
		Origin:    mid,
		Facing:    45,
	}

	// The fixture's premise: the bounds admit the x2–x3 crossing, and the
	// rectangle does not lie across it.
	reach := boundsOf(placement)
	require.True(t, reach.mayTouch(placement.Origin, x[2], x[3]), "precondition: the bounds reach the b–c crossing")
	trace, err := spatial.TraceFootprint(spatial.FootprintTraceInput{Placement: placement, From: x[2], To: x[3]})
	require.NoError(t, err)
	require.False(t, trace.Interior, "precondition: the rectangle does not lie across it")

	door := []DoorInput{{ID: "diagonal", Placement: &placement, State: DoorIsOpen()}}
	require.NoError(t, f.derivePartitions(door))
	partition := func(col int) PartitionID {
		p, ok := f.partitionAt(f.cellAt(spatial.Position{X: float64(col), Y: 0}))
		require.True(t, ok)
		return p
	}
	a, b, c := partition(0), partition(1), partition(3)
	require.Equal(t, b, partition(2), "precondition: x1 and x2 are one room")
	require.Len(t, map[PartitionID]bool{a: true, b: true, c: true}, 3, "precondition: three rooms")

	doors, _ := doorRecordsFrom(door)
	e := &Encounter{field: f, doors: doors}
	links := e.links()

	var fromB []PartitionID
	for _, l := range links[b] {
		fromB = append(fromB, l.to)
	}
	require.Contains(t, fromB, a, "the door joins the two partitions either side of the crossing it lies across")
	require.NotContains(t, fromB, c, "and not a pair only its bounds reach")
	require.Empty(t, links[c], "c has no door at all")
}
