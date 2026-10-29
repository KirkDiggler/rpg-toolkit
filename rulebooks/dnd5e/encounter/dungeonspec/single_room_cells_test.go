// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package dungeonspec

import (
	"testing"

	"github.com/stretchr/testify/require"
)

// expand runs ExpandRows at a fixed region path and hands back the cells and
// every defect it reported.
func expand(rows []RoomCellRow) ([]RoomCell, []FieldError) {
	var errs []FieldError
	cells := ExpandRows(rows, "room.room.regions[0].cells", func(p, m string) {
		errs = append(errs, FieldError{Path: p, Message: m})
	})
	return cells, errs
}

// TestExpandRows_CanonicalCastleRow is the gatehouse's first row: one span,
// inclusive at both ends, five cells.
func TestExpandRows_CanonicalCastleRow(t *testing.T) {
	cells, errs := expand([]RoomCellRow{{R: -6, Q: [][2]int{{-2, 2}}}})
	require.Empty(t, errs)
	require.Equal(t, []RoomCell{{Q: -2, R: -6}, {Q: -1, R: -6}, {Q: 0, R: -6}, {Q: 1, R: -6}, {Q: 2, R: -6}}, cells)
}

// TestExpandRows_ReadsSeveralRowsAndSpans: a gap between two spans is not
// floor, and the rows come back in the order they are written (ascending r).
func TestExpandRows_ReadsSeveralRowsAndSpans(t *testing.T) {
	cells, errs := expand([]RoomCellRow{
		{R: -5, Q: [][2]int{{-3, -2}, {6, 6}}},
		{R: 2, Q: [][2]int{{0, 0}}},
	})
	require.Empty(t, errs)
	require.Equal(t, []RoomCell{{Q: -3, R: -5}, {Q: -2, R: -5}, {Q: 6, R: -5}, {Q: 0, R: 2}}, cells)
}

// TestExpandRows_RefusesTouchingSpans: [1,3] and [4,6] are one run, [1,6].
// Two spellings of one cell set would make a repaint diff on nothing.
func TestExpandRows_RefusesTouchingSpans(t *testing.T) {
	_, errs := expand([]RoomCellRow{{R: 0, Q: [][2]int{{1, 3}, {4, 6}}}})
	require.Equal(t, []FieldError{{Path: "room.room.regions[0].cells[0].q[1]", Message: "spans must be ascending and merged"}}, errs)
}

// TestExpandRows_RefusesOverlappingAndUnsortedSpans: the same refusal for a
// span that starts inside, or before, the one written before it.
func TestExpandRows_RefusesOverlappingAndUnsortedSpans(t *testing.T) {
	for name, q := range map[string][][2]int{
		"overlapping": {{1, 5}, {3, 8}},
		"unsorted":    {{6, 8}, {1, 3}},
	} {
		t.Run(name, func(t *testing.T) {
			_, errs := expand([]RoomCellRow{{R: 0, Q: q}})
			require.Equal(t, []FieldError{{Path: "room.room.regions[0].cells[0].q[1]", Message: "spans must be ascending and merged"}}, errs)
		})
	}
}

func TestExpandRows_RefusesDescendingSpan(t *testing.T) {
	cells, errs := expand([]RoomCellRow{{R: 0, Q: [][2]int{{3, 1}}}})
	require.Equal(t, []FieldError{{Path: "room.room.regions[0].cells[0].q[0]", Message: "must not be descending"}}, errs)
	require.Empty(t, cells, "a descending span names no cell")
}

func TestExpandRows_RefusesUnsortedRows(t *testing.T) {
	_, errs := expand([]RoomCellRow{{R: 2, Q: [][2]int{{0, 0}}}, {R: 1, Q: [][2]int{{0, 0}}}})
	require.Equal(t, []FieldError{{Path: "room.room.regions[0].cells[1]", Message: "rows must be ascending by r"}}, errs)
}

func TestExpandRows_RefusesRepeatedRow(t *testing.T) {
	_, errs := expand([]RoomCellRow{{R: 1, Q: [][2]int{{0, 0}}}, {R: 1, Q: [][2]int{{4, 4}}}})
	require.Equal(t, []FieldError{{Path: "room.room.regions[0].cells[1]", Message: "rows must be ascending by r"}}, errs)
}

func TestExpandRows_RefusesARowWithNoSpan(t *testing.T) {
	_, errs := expand([]RoomCellRow{{R: 1}})
	require.Equal(t, []FieldError{{Path: "room.room.regions[0].cells[0].q", Message: "must not be empty"}}, errs)
}

// TestExpandRows_RefusesACoordinateBeyondTheBound: the workspace radius was
// the only bound on a cell's coordinates and it is gone (R3). A span that far
// out is refused by name before anything counts its cells.
func TestExpandRows_RefusesACoordinateBeyondTheBound(t *testing.T) {
	_, errs := expand([]RoomCellRow{{R: 0, Q: [][2]int{{0, maxSiteCoord + 1}}}})
	require.Equal(t, []FieldError{{Path: "room.room.regions[0].cells[0].q[0]", Message: errBeyondCoordBound}}, errs)
	_, errs = expand([]RoomCellRow{{R: -maxSiteCoord - 1, Q: [][2]int{{0, 0}}}})
	require.Equal(t, []FieldError{{Path: "room.room.regions[0].cells[0].r", Message: errBeyondCoordBound}}, errs)
}

// TestRegionCells_RefusesASiteBeyondTheCellBudget: one span can name
// millions of cells in a dozen bytes, so the count is judged before a single
// cell is listed.
func TestRegionCells_RefusesASiteBeyondTheCellBudget(t *testing.T) {
	var errs []FieldError
	add := func(p, m string) { errs = append(errs, FieldError{Path: p, Message: m}) }
	regions := []RoomRegion{
		{ID: "a", Lighting: RoomLighting{Intensity: 1}, Cells: []RoomCellRow{{R: 0, Q: [][2]int{{0, maxSiteCells - 1}}}}},
		{ID: "b", Lighting: RoomLighting{Intensity: 1}, Cells: []RoomCellRow{{R: 1, Q: [][2]int{{0, 0}}}}},
	}
	cells := regionCells(regions, add)
	require.Equal(t, []FieldError{{Path: "room.room.regions[1].cells[0].q[0]", Message: errSiteTooLarge}}, errs)
	require.Equal(t, [][]RoomCell{nil, nil}, cells, "a site over budget lists no cell at all")

	// Exactly the budget is a site; one span over it, on its own, is not.
	errs = nil
	_, errs2 := expand([]RoomCellRow{{R: 0, Q: [][2]int{{0, maxSiteCells}}}})
	require.Equal(t, []FieldError{{Path: "room.room.regions[0].cells[0].q[0]", Message: errSiteTooLarge}}, errs2)
	require.Empty(t, errs)
}

// TestRegionCells_NamesTheSecondOwner: a cell two regions list is refused at
// the span of the later region that lists it, naming the first owner, and
// stays the first region's.
func TestRegionCells_NamesTheSecondOwner(t *testing.T) {
	var errs []FieldError
	add := func(p, m string) { errs = append(errs, FieldError{Path: p, Message: m}) }
	cells := regionCells([]RoomRegion{
		{ID: "region-yard", Cells: []RoomCellRow{{R: 0, Q: [][2]int{{0, 3}}}}},
		{ID: "region-gate", Cells: []RoomCellRow{{R: -1, Q: [][2]int{{0, 0}}}, {R: 0, Q: [][2]int{{3, 3}, {5, 6}}}}},
	}, add)
	require.Contains(t, errs, FieldError{Path: "room.room.regions[1].cells[1].q[0]", Message: `already belongs to region "region-yard"`})
	require.Equal(t, []RoomCell{{Q: 0, R: 0}, {Q: 1, R: 0}, {Q: 2, R: 0}, {Q: 3, R: 0}}, cells[0])
	require.NotContains(t, cells[1], RoomCell{Q: 3, R: 0})
}
