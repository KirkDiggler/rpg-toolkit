// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package rowruns_test

import (
	"math"
	"math/rand/v2"
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter/internal/rowruns"
)

type RowRunsSuite struct {
	suite.Suite
}

func TestRowRunsSuite(t *testing.T) { suite.Run(t, new(RowRunsSuite)) }

func set(cells []rowruns.Cell) map[rowruns.Cell]bool {
	out := make(map[rowruns.Cell]bool, len(cells))
	for _, c := range cells {
		out[c] = true
	}
	return out
}

// Any cell set, in any order and with repeats, spells as runs that read back
// as the same set, and the spelling is the canonical one the reader accepts.
func (s *RowRunsSuite) TestEncodeExpandRoundTrip() {
	rng := rand.New(rand.NewPCG(7, 11))
	for round := 0; round < 200; round++ {
		n := rng.IntN(300)
		cells := make([]rowruns.Cell, 0, n)
		for i := 0; i < n; i++ {
			cells = append(cells, rowruns.Cell{Q: rng.IntN(40) - 20, R: rng.IntN(12) - 6})
		}
		rows := rowruns.Encode(cells)
		got, err := rowruns.Expand(rows, len(cells))
		s.Require().NoError(err, "round %d: the encoder's spelling is canonical", round)
		s.Equal(set(cells), set(got), "round %d: the same set", round)
		s.Len(got, len(set(cells)), "round %d: each cell once", round)
	}
}

// Touching cells along a row are one span; a gap starts another; a repeat
// is the set's one cell.
func (s *RowRunsSuite) TestEncodeMergesTouchingCells() {
	rows := rowruns.Encode([]rowruns.Cell{{Q: 3, R: 1}, {Q: 1, R: 1}, {Q: 2, R: 1}, {Q: 2, R: 1}, {Q: 6, R: 1}, {Q: 0, R: -2}})
	s.Equal([]rowruns.Row{
		{R: -2, Q: [][2]int{{0, 0}}},
		{R: 1, Q: [][2]int{{1, 3}, {6, 6}}},
	}, rows)
	s.Nil(rowruns.Encode(nil), "no cells, no rows")
}

// Every way a run list is not the one spelling is refused, as is a list
// naming more cells than the reader allows — counted, never listed.
func (s *RowRunsSuite) TestCountRefuses() {
	notCanonical := map[string][]rowruns.Row{
		"a row with no span":          {{R: 0}},
		"a descending span":           {{R: 0, Q: [][2]int{{3, 1}}}},
		"spans that overlap":          {{R: 0, Q: [][2]int{{1, 4}, {3, 6}}}},
		"spans that touch":            {{R: 0, Q: [][2]int{{1, 2}, {3, 6}}}},
		"spans out of order":          {{R: 0, Q: [][2]int{{5, 6}, {1, 2}}}},
		"a row repeated":              {{R: 0, Q: [][2]int{{1, 1}}}, {R: 0, Q: [][2]int{{3, 3}}}},
		"rows out of order":           {{R: 2, Q: [][2]int{{1, 1}}}, {R: 1, Q: [][2]int{{1, 1}}}},
		"spans touching at the floor": {{R: 0, Q: [][2]int{{math.MinInt, math.MinInt}, {math.MinInt + 1, math.MinInt + 1}}}},
	}
	for name, rows := range notCanonical {
		_, err := rowruns.Count(rows, math.MaxInt)
		s.ErrorIs(err, rowruns.ErrNotCanonical, name)
	}

	tooMany := map[string][]rowruns.Row{
		"one span past the limit":   {{R: 0, Q: [][2]int{{0, 10}}}},
		"two rows past the limit":   {{R: 0, Q: [][2]int{{0, 5}}}, {R: 1, Q: [][2]int{{0, 4}}}},
		"the widest span there is":  {{R: 0, Q: [][2]int{{math.MinInt, math.MaxInt}}}},
		"a billion cells on a site": {{R: 0, Q: [][2]int{{-1_000_000_000, 1_000_000_000}}}},
	}
	for name, rows := range tooMany {
		_, err := rowruns.Count(rows, 10)
		s.ErrorIs(err, rowruns.ErrTooMany, name)
		_, err = rowruns.Expand(rows, 10)
		s.ErrorIs(err, rowruns.ErrTooMany, name)
	}

	n, err := rowruns.Count([]rowruns.Row{{R: 0, Q: [][2]int{{0, 5}}}, {R: 1, Q: [][2]int{{0, 3}}}}, 10)
	s.Require().NoError(err, "exactly the limit is allowed")
	s.Equal(10, n)
}
