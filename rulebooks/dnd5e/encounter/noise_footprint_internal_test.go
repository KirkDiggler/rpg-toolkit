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
// Four floor cells in one row, x0 x1 x2 x3: region a is x0, b is x1 and x2,
// c is x3. The door is a long thin rectangle at 45° through the midpoint of
// the x0–x1 crossing. It lies across that crossing and no other, but its
// axis-aligned bounds reach out over the x2–x3 crossing too. Counting every
// crossing the bounds admit would join b to c through a door that stands
// nowhere near their seam.
func TestADiagonalDoorJoinsOnlyTheCrossingItLiesAcross(t *testing.T) {
	region := func(id string, cols ...int) RegionInput {
		r := rectRegion(id, 0, 0, 0, 0)
		for _, c := range cols {
			r.Cells = append(r.Cells, spatial.Position{X: float64(c), Y: 0})
		}
		return r
	}
	f, err := compileField(FieldInput{
		Canvas:  openAir(),
		Regions: []RegionInput{region("a", 0), region("b", 1, 2), region("c", 3)},
	})
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

	doors, _ := doorRecordsFrom([]DoorInput{{ID: "diagonal", Placement: &placement, State: DoorIsOpen()}})
	e := &Encounter{field: f, doors: doors}
	links := e.links()

	var fromB []RegionID
	for _, l := range links["b"] {
		fromB = append(fromB, l.to)
	}
	require.Contains(t, fromB, RegionID("a"), "the door joins the two regions either side of the crossing it lies across")
	require.NotContains(t, fromB, RegionID("c"), "and not a pair only its bounds reach")
	require.Empty(t, links["c"], "c has no door at all")
}
