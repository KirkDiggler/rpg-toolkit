// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package dungeonspec

import (
	"fmt"
	"sort"
)

// single_room_cells.go reads a region's cells (rpg-project#725, design R2).
//
// A region's cells are written as ROW RUNS: for each axial row r, the
// inclusive [qStart, qEnd] spans of q that are floor. The runs are canonical —
// rows ascend by r with no repeat, spans ascend and neither overlap nor touch —
// so one cell set has exactly one spelling and a repaint diffs per row. They
// are what the compiler reads; nothing here infers a cell the author did not
// list, and there is no bitmap, outline-fill or hole grammar.
//
// THE TWO BOUNDS THE WORKSPACE USED TO BE. The workspace radius bounded every
// authored cell, and it is gone (R3: the canvas is unbounded). What it was
// also doing — keeping a coordinate inside the range the encounter's integer
// conversion is exact in, and keeping a twelve-byte span from naming a
// billion cells — is held here, by two bounds that are not gameplay limits:
// a coordinate magnitude ([maxSiteCoord]) and a whole-site cell budget
// ([maxSiteCells], the encounter field's own cell budget).

const (
	// maxSiteCoord bounds the magnitude of every authored q and r. Integer
	// defense, not a map size: the encounter refuses an offset cell beyond
	// 1<<30, and no site is within five orders of magnitude of this.
	maxSiteCoord = 1 << 24
	// maxSiteCells is the most cells a site's regions may list together —
	// the encounter field's own budget (compilefield.go maxFieldCells), so a
	// site the decoder accepts is never one the compiler refuses for size.
	maxSiteCells = 1 << 22

	errBeyondCoordBound   = "is beyond the coordinate bound"
	errSiteTooLarge       = "the site lists more cells than one field holds"
	errSpanDescending     = "must not be descending"
	errSpansNotCanonical  = "spans must be ascending and merged"
	errRowsNotCanonical   = "rows must be ascending by r"
	errMustNotBeEmpty     = "must not be empty"
	errCellInTwoRegions   = "already belongs to region %q"
	errNoRegionOwnsACell  = "no region owns a cell"
	errLightingOutOfRange = "must be between 0 and 1"
)

// ExpandRows returns the cells a region's rows list, in the order they are
// written (ascending r, then ascending q), reporting at path — the region's
// `cells` path — every way the rows are not canonical: a row with no span, a
// coordinate beyond the bound, a descending span, a span that does not start
// after the previous one ends plus one, and a row that does not come after
// the previous row. A span that is refused contributes no cell; a row out of
// order still contributes its cells, so a later duplicate is still found.
//
// The rows are counted before a cell is listed: rows naming more cells than
// one field holds are refused at the span that overdraws, and expand to
// nothing.
func ExpandRows(rows []RoomCellRow, path string, add errSink) []RoomCell {
	region := []RoomRegion{{Cells: rows}}
	if !withinBudget(region, func(int) string { return path }, add) {
		return nil
	}
	return expandRows(rows, path, add)
}

// CellRows spells a cell set as canonical row runs — the one spelling
// [ExpandRows] reads back: rows ascending by r with no repeat, each row's
// inclusive [qStart, qEnd] spans ascending and merged, so cells that touch
// along a row are one span. The input may be in any order and may repeat a
// cell; the set is spelled once. Empty input is nil. The input is not
// modified.
//
// It is the encoder every writer of a region uses — the v4 converter and the
// keep generator (dungeonspec/keepgen) — so a generated site and a converted
// one cannot spell the same floor two ways.
func CellRows(cells []RoomCell) []RoomCellRow {
	sorted := append([]RoomCell(nil), cells...)
	sort.Slice(sorted, func(i, j int) bool {
		if sorted[i].R != sorted[j].R {
			return sorted[i].R < sorted[j].R
		}
		return sorted[i].Q < sorted[j].Q
	})
	var rows []RoomCellRow
	for _, c := range sorted {
		if n := len(rows); n > 0 && rows[n-1].R == c.R {
			last := &rows[n-1].Q[len(rows[n-1].Q)-1]
			switch c.Q {
			case last[1]:
				// a duplicate cell; the set has it once
			case last[1] + 1:
				last[1] = c.Q
			default:
				rows[n-1].Q = append(rows[n-1].Q, [2]int{c.Q, c.Q})
			}
			continue
		}
		rows = append(rows, RoomCellRow{R: c.R, Q: [][2]int{{c.Q, c.Q}}})
	}
	return rows
}

// withinBudget counts every well-formed span of every region against the
// site's cell budget without listing a cell, and refuses the span that
// overdraws it. pathOf is a region's `cells` path.
func withinBudget(regions []RoomRegion, pathOf func(i int) string, add errSink) bool {
	budget := maxSiteCells
	for i, region := range regions {
		for j, row := range region.Cells {
			for k, span := range row.Q {
				if !inBound(span[0]) || !inBound(span[1]) || span[0] > span[1] {
					continue
				}
				if budget -= span[1] - span[0] + 1; budget < 0 {
					add(fmt.Sprintf("%s[%d].q[%d]", pathOf(i), j, k), errSiteTooLarge)
					return false
				}
			}
		}
	}
	return true
}

func inBound(v int) bool { return v <= maxSiteCoord && v >= -maxSiteCoord }

// expandRows is [ExpandRows] after the count: the canonical-form refusals and
// the cells of every span that passes them.
func expandRows(rows []RoomCellRow, path string, add errSink) []RoomCell {
	var out []RoomCell
	expandRowsAt(rows, path, add, func(c RoomCell, _, _ int) { out = append(out, c) })
	return out
}

// expandRowsAt is the expansion itself: every canonical-form refusal, and
// each cell of every span that passes them handed to visit with the row and
// span that list it, so a later refusal can name the exact listing.
func expandRowsAt(rows []RoomCellRow, path string, add errSink, visit func(c RoomCell, row, span int)) {
	for j, row := range rows {
		rowPath := fmt.Sprintf("%s[%d]", path, j)
		if j > 0 && row.R <= rows[j-1].R {
			add(rowPath, errRowsNotCanonical)
		}
		if !inBound(row.R) {
			add(rowPath+".r", errBeyondCoordBound)
			continue
		}
		if len(row.Q) == 0 {
			add(rowPath+".q", errMustNotBeEmpty)
			continue
		}
		for k, span := range row.Q {
			spanPath := fmt.Sprintf("%s.q[%d]", rowPath, k)
			start, end := span[0], span[1]
			switch {
			case !inBound(start) || !inBound(end):
				add(spanPath, errBeyondCoordBound)
				continue
			case start > end:
				add(spanPath, errSpanDescending)
				continue
			case k > 0 && start <= row.Q[k-1][1]+1:
				add(spanPath, errSpansNotCanonical)
				continue
			}
			for q := start; q <= end; q++ {
				visit(RoomCell{Q: q, R: row.R}, j, k)
			}
		}
	}
}

// regionCells expands every region's rows, in authored order, after counting
// the whole site against one budget, and reports a cell a second region lists
// at the span that lists it again, naming the region that owns it first. The
// result is indexed like regions; an empty region, and every region of a site
// over budget, expands to nothing.
func regionCells(regions []RoomRegion, add errSink) [][]RoomCell {
	out := make([][]RoomCell, len(regions))
	pathOf := func(i int) string { return fmt.Sprintf("room.room.regions[%d].cells", i) }
	if !withinBudget(regions, pathOf, add) {
		return out
	}
	owner := map[RoomCell]int{}
	for i, region := range regions {
		path := pathOf(i)
		expandRowsAt(region.Cells, path, add, func(c RoomCell, j, k int) {
			first, taken := owner[c]
			switch {
			case !taken:
				owner[c] = i
				out[i] = append(out[i], c)
			case first != i:
				add(fmt.Sprintf("%s[%d].q[%d]", path, j, k), fmt.Sprintf(errCellInTwoRegions, regions[first].ID))
			}
			// A cell the SAME region lists twice is a repeated row, already
			// refused at the second listing as "rows must be ascending by
			// r"; the region keeps it once and names no second owner.
		})
	}
	return out
}

// regionValues judges the authored regions: a non-empty list, each with an
// id no other region has, a lighting intensity in [0, 1], and canonical rows;
// no cell in two regions; and at least one cell on the floor. It returns
// every region's expanded cells, indexed like the list, for the readers that
// need the floor.
func regionValues(regions []RoomRegion, add errSink) [][]RoomCell {
	if len(regions) == 0 {
		add("room.room.regions", errMustNotBeEmpty)
		return nil
	}
	seen := map[string]bool{}
	for i, region := range regions {
		p := fmt.Sprintf("room.room.regions[%d]", i)
		if region.ID == "" {
			add(p+".id", errRequired)
		} else if seen[region.ID] {
			add(p+".id", errDuplicateID)
		}
		seen[region.ID] = true
		if v := region.Lighting.Intensity; !(v >= 0 && v <= 1) {
			add(p+".lighting.intensity", errLightingOutOfRange)
		}
	}
	cells := regionCells(regions, add)
	for _, c := range cells {
		if len(c) > 0 {
			return cells
		}
	}
	add("room.room.regions", errNoRegionOwnsACell)
	return cells
}

// floorCells is the union of the regions' cells — the site's floor — for a
// reader that runs on a document validation has already judged. Defects are
// not its business: they were reported where the document was judged.
func (gp *RoomGameplaySource) floorCells() []RoomCell {
	var out []RoomCell
	for _, cells := range regionCells(gp.Regions, func(string, string) {}) {
		out = append(out, cells...)
	}
	return out
}
