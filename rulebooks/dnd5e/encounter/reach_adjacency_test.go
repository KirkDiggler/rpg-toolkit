// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package encounter_test

import (
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter"
	"github.com/KirkDiggler/rpg-toolkit/tools/spatial"
)

// reach_adjacency_test.go pins rpg-toolkit#2017's ruling: two hexes that share
// a side are adjacent, and attack reach agrees with movement on all six of a
// cell's neighbours.
//
// # Two readers, one rule
//
// Reach is [encounter.Encounter.Distance] compared against
// [encounter.CellsFromFeet] — what session's inRange and this module's
// withinReach both compute. Movement is the walk check in session's Move,
// which builds a [spatial.AxialHexGrid] and asks IsAdjacent of each step. Both
// read cells in the SAME frame: the absolute axial cell [encounter.HexCellAt]
// produces, which is also the frame every position on the wire is in.
//
// # The external reference
//
// "Shares a side" is asked in the AUTHORED offset frame, from the standard
// odd-r (pointy-top) and odd-q (flat-top) neighbour tables, not from either
// reader's own formula. A test that took the six neighbours from the grid it
// was checking could not fail.
//
// # What the walk saw
//
// The walk reported (7,4) and (6,3) as neighbours. Those were wire positions,
// which are axial; the walk's own path tool read them as odd-r offsets. In
// axial they are two cells apart, and both readers say so — the pair is
// pinned below so the misreading has a named answer.

type ReachAdjacencySuite struct {
	suite.Suite
}

func TestReachAdjacencySuite(t *testing.T) {
	suite.Run(t, new(ReachAdjacencySuite))
}

// offsetStep is one authored [col,row] neighbour offset.
type offsetStep struct{ dc, dr int }

// The standard offset-coordinate neighbour tables. Odd-r staggers rows, so the
// table depends on the row's parity; odd-q staggers columns, so it depends on
// the column's.
var (
	oddREvenRow = []offsetStep{{+1, 0}, {0, -1}, {-1, -1}, {-1, 0}, {-1, +1}, {0, +1}}
	oddROddRow  = []offsetStep{{+1, 0}, {+1, -1}, {0, -1}, {-1, 0}, {0, +1}, {+1, +1}}
	oddQEvenCol = []offsetStep{{+1, 0}, {+1, -1}, {0, -1}, {-1, -1}, {-1, 0}, {0, +1}}
	oddQOddCol  = []offsetStep{{+1, +1}, {+1, 0}, {0, -1}, {-1, 0}, {-1, +1}, {0, +1}}
)

// fieldFor stands up a plain rectangle floor under the given orientation.
func (s *ReachAdjacencySuite) fieldFor(o encounter.Orientation) *encounter.Encounter {
	enc, err := encounter.NewEncounter(&encounter.SetupInput{
		Field: encounter.FieldInput{
			Canvas:  encounter.CanvasInput{Void: encounter.VoidIsOpaque(), Orientation: o},
			Regions: []encounter.RegionInput{rectRegion("floor", 0, 0, 16, 8)},
		},
		Members: []encounter.MemberInput{{ID: alice, Kind: encounter.KindPlayer, Position: encounter.HexCellAt(o, 1, 1)}},
		Endings: []encounter.EndingInput{{Key: "done", Trigger: encounter.TriggerExternal{}}},
		Capabilities: encounter.Capabilities{
			Sight:         everyoneSeesTheWholeMap{},
			Equipment:     encounter.UnobservedEquipment{},
			Conditions:    encounter.UnobservedEquipment{},
			Sheets:        zeroSheets{},
			Participation: everyoneStanding{},
			Initiative:    orderAsGiven{},
			Driver:        passDriver{},
			Actors: encounter.Actors{
				Striker:   passStriker{},
				Mover:     quietMover{},
				Announcer: quietAnnouncer{},
			},
		},
	})
	s.Require().NoError(err)

	return enc
}

// walkGrid is the grid session's Move asks IsAdjacent of on a hex field,
// built the same way: session's gridOf passes adjacencySpan, which is 1.
// IsAdjacent reads Distance alone and never the span.
func walkGrid() spatial.Grid {
	return spatial.NewAxialHexGrid(spatial.AxialHexGridConfig{SpanWidth: 1, SpanHeight: 1})
}

// assertAgreeOnNeighbours checks every side-sharing neighbour of one authored
// cell is one step for movement and inside 5 ft reach, and that one named
// non-neighbour is neither.
func (s *ReachAdjacencySuite) assertAgreeOnNeighbours(
	o encounter.Orientation, col, row int, table []offsetStep, farCol, farRow int,
) {
	enc := s.fieldFor(o)
	grid := walkGrid()
	reach := float64(encounter.CellsFromFeet(5))
	here := encounter.HexCellAt(o, col, row)

	for _, step := range table {
		there := encounter.HexCellAt(o, col+step.dc, row+step.dr)
		s.True(grid.IsAdjacent(here, there),
			"movement: authored (%d,%d) and (%d,%d) share a side", col, row, col+step.dc, row+step.dr)
		s.Equal(1.0, enc.Distance(here, there),
			"reach: authored (%d,%d) and (%d,%d) share a side", col, row, col+step.dc, row+step.dr)
		s.LessOrEqual(enc.Distance(here, there), reach,
			"a 5 ft attack from authored (%d,%d) reaches (%d,%d)", col, row, col+step.dc, row+step.dr)
	}

	far := encounter.HexCellAt(o, farCol, farRow)
	s.False(grid.IsAdjacent(here, far), "movement: (%d,%d) is not beside (%d,%d)", farCol, farRow, col, row)
	s.Equal(2.0, enc.Distance(here, far), "reach: (%d,%d) is two cells from (%d,%d)", farCol, farRow, col, row)
	s.Greater(enc.Distance(here, far), reach)
}

func (s *ReachAdjacencySuite) TestPointyTopOddRowNeighboursAgree() {
	// (6,4) would be odd-r's up-left of an EVEN row; from odd row 3 it is
	// two cells away.
	s.assertAgreeOnNeighbours(encounter.HexesArePointyTop(), 7, 3, oddROddRow, 6, 4)
}

func (s *ReachAdjacencySuite) TestPointyTopEvenRowNeighboursAgree() {
	// (8,3) would be odd-r's up-right of an ODD row; from even row 4 it is
	// two cells away.
	s.assertAgreeOnNeighbours(encounter.HexesArePointyTop(), 7, 4, oddREvenRow, 8, 3)
}

func (s *ReachAdjacencySuite) TestFlatTopOddColumnNeighboursAgree() {
	s.assertAgreeOnNeighbours(encounter.HexesAreFlatTop(), 7, 3, oddQOddCol, 6, 2)
}

func (s *ReachAdjacencySuite) TestFlatTopEvenColumnNeighboursAgree() {
	s.assertAgreeOnNeighbours(encounter.HexesAreFlatTop(), 6, 3, oddQEvenCol, 7, 4)
}

// TestTheWalksPairIsTwoCellsApartForBoth names rpg-toolkit#2017's evidence.
// The walk's rogue stood on wire cell (7,4) and the zombie on (6,3). Wire
// cells are axial, so these are taken as-is, not through HexCellAt. Axial
// (q,r) and (q-1,r-1) differ by 2 along s: not neighbours, for movement or
// for reach. Axial (7,3), the cell the walk attacked from successfully, is.
func (s *ReachAdjacencySuite) TestTheWalksPairIsTwoCellsApartForBoth() {
	enc := s.fieldFor(encounter.HexesArePointyTop())
	grid := walkGrid()
	reach := float64(encounter.CellsFromFeet(5))

	rogue := spatial.Position{X: 7, Y: 4}
	zombie := spatial.Position{X: 6, Y: 3}
	s.False(grid.IsAdjacent(rogue, zombie), "movement refuses (7,4) to (6,3) as one step")
	s.Equal(2.0, enc.Distance(rogue, zombie))
	s.Greater(enc.Distance(rogue, zombie), reach)

	upLeft := spatial.Position{X: 7, Y: 3}
	s.True(grid.IsAdjacent(rogue, upLeft))
	s.Equal(1.0, enc.Distance(rogue, upLeft))

	// The same two cells, written in the authored odd-r frame the walk's
	// tool assumed, are (9,4) and (7,3): two cells apart.
	s.Equal(rogue, encounter.HexCellAt(encounter.HexesArePointyTop(), 9, 4))
	s.Equal(zombie, encounter.HexCellAt(encounter.HexesArePointyTop(), 7, 3))
}
