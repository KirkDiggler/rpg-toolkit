// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

// Package rowruns is THE ONE SPELLING OF A HEX CELL SET AS ROW RUNS
// (rpg-project#725 R2, rpg-project#753 S4): for each axial row r, the
// inclusive [qStart, qEnd] spans of q in the set. The runs are canonical —
// rows ascend by r with no repeat, spans ascend and neither overlap nor touch
// — so one cell set has exactly one spelling.
//
// Two writers share it: a site's regions (dungeonspec's CellRows) and a
// member's `known:cells` facts (the encounter's journal). Both read back what
// they wrote through the same rules, so the floor and the knowledge of it
// cannot be spelled two ways.
package rowruns

import (
	"errors"
	"fmt"
	"sort"
)

// Cell is one axial cell: q along the row, r the row.
type Cell struct {
	Q, R int
}

// Row is one axial row's runs: R, and the inclusive [qStart, qEnd] spans of
// q, ascending and merged.
type Row struct {
	R int      `yaml:"r" json:"r"`
	Q [][2]int `yaml:"q" json:"q"`
}

// ErrNotCanonical is every way a run list is not the one spelling: a row
// with no span, a descending span, spans that overlap or touch, rows out of
// order or repeated.
var ErrNotCanonical = errors.New("row runs are not canonical")

// ErrTooMany is a run list naming more cells than the reader's limit.
var ErrTooMany = errors.New("row runs name more cells than allowed")

// Encode spells a cell set as canonical row runs. The input may be in any
// order and may repeat a cell; the set is spelled once. Empty input is nil.
// The input is not modified.
func Encode(cells []Cell) []Row {
	sorted := append([]Cell(nil), cells...)
	sort.Slice(sorted, func(i, j int) bool {
		if sorted[i].R != sorted[j].R {
			return sorted[i].R < sorted[j].R
		}
		return sorted[i].Q < sorted[j].Q
	})
	var rows []Row
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
		rows = append(rows, Row{R: c.R, Q: [][2]int{{c.Q, c.Q}}})
	}
	return rows
}

// Count is how many cells canonical rows name, refusing rows that are not
// canonical and a count above limit — before a cell is listed, so a span of
// a billion cells costs nothing to refuse.
func Count(rows []Row, limit int) (int, error) {
	total := 0
	for i, row := range rows {
		if i > 0 && row.R <= rows[i-1].R {
			return 0, fmt.Errorf("row %d (r=%d) does not come after r=%d: %w", i, row.R, rows[i-1].R, ErrNotCanonical)
		}
		if len(row.Q) == 0 {
			return 0, fmt.Errorf("row %d (r=%d) has no span: %w", i, row.R, ErrNotCanonical)
		}
		for k, span := range row.Q {
			if span[0] > span[1] {
				return 0, fmt.Errorf("row %d span %d %v is descending: %w", i, k, span, ErrNotCanonical)
			}
			if k > 0 && (span[0] <= row.Q[k-1][1] || span[0]-1 == row.Q[k-1][1]) {
				return 0, fmt.Errorf("row %d span %d %v does not start after %v: %w", i, k, span, row.Q[k-1], ErrNotCanonical)
			}
			// The span's width less one, exact in unsigned arithmetic for any
			// non-descending pair; width > room is less-one >= room.
			less := uint64(span[1]) - uint64(span[0])
			if limit-total <= 0 || less >= uint64(limit-total) {
				return 0, fmt.Errorf("more than %d cells: %w", limit, ErrTooMany)
			}
			total += int(less) + 1
		}
	}
	return total, nil
}

// Expand lists the cells canonical rows name, ascending r then ascending q,
// refusing what [Count] refuses.
func Expand(rows []Row, limit int) ([]Cell, error) {
	n, err := Count(rows, limit)
	if err != nil {
		return nil, err
	}
	out := make([]Cell, 0, n)
	for _, row := range rows {
		for _, span := range row.Q {
			for q := span[0]; ; q++ {
				out = append(out, Cell{Q: q, R: row.R})
				if q == span[1] {
					break
				}
			}
		}
	}
	return out, nil
}
