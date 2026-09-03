// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package dungeonspec

import (
	"sort"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter"
	"github.com/KirkDiggler/rpg-toolkit/tools/spatial"
)

// walls.go is WHAT A WALL DERIVES (rpg-project#360, wall-geometry design §4.2).
//
// The file holds a line and nothing else. Everything the engine runs on is
// worked out here, exactly once, from that line and the floor:
//
//   - its FOOTPRINT (C8), the floor cells its segment passes through;
//   - the CROSSINGS it blocks (C7), the centre-to-centre steps it stands in
//     the way of;
//   - which footprint cells it SEALS (C10), the ones with too little room
//     left to stand in.
//
// Nothing downstream re-derives any of it. The runtime is handed crossings as
// pairs, sealed cells as a list, and segments as two points and a height; it
// never embeds a hex (design C9, plan §0).

// MinStandable is HOW MUCH OF A CELL A WALL MUST LEAVE for feet to be allowed
// on it: the fraction of its hex still on the near side of every wall through
// it (design C10).
//
// ONE CONSTANT, ONE PLACE, and a number to be tuned in the game rather than
// argued about here. 0.7 rather than 0.75 for a stated reason: a square room's
// inside corner keeps EXACTLY 3/4, and a threshold equal to the answer would
// decide that cell on the last bit of a float. The four calibration numbers it
// sits between are in geometry_internal_test.go — a thin wall alone shaves
// 1/24 or 5/24 and never seals anything, a square corner keeps 3/4 and stands,
// a hexagonal corner keeps 7/12 and does not, a wall through a centre leaves
// half and does not.
const MinStandable = 0.7

// wallGeometry is one authored wall with everything derived from it.
type wallGeometry struct {
	// Index is which walls[] entry this is, for the path a refusal names.
	Index int

	// Name is the author's word for it, used in refusals about the cells it
	// cuts: "north wall" beats "walls[7]" for the streamers this dialect is
	// authored by.
	Name string

	// Height is the authored multiplier, nil when not authored.
	Height *float64

	// Start and End are the wall's two positions as fractional axial points
	// — exact halves, so two walls that join at a corner carry byte-identical
	// endpoints without anything having to recognise the corner (design F5).
	Start, End axialPoint

	// Footprint is every FLOOR cell the segment passes through, absolute
	// axial, in order along the wall (C8). Void cells on the path are not
	// here: there is nothing to cut, and the crossings into them are
	// impassable already (C2).
	Footprint []spatial.Position

	// Crossings is every crossing the wall blocks, absolute axial and
	// normalized (C7).
	Crossings [][2]spatial.Position
}

// wallPasses reports whether a position lies on a wall's closed segment —
// what a door has to be able to say (F10).
//
// Answered in AXIAL space rather than in the embedding: collinearity and
// betweenness survive any linear map, and both the wall's ends and the
// position are exact halves there, so a point an author placed on a wall
// answers yes without a tolerance deciding it.
func wallPasses(w wallGeometry, at axialPoint) bool {
	dq, dr := w.End.Q-w.Start.Q, w.End.R-w.Start.R
	pq, pr := at.Q-w.Start.Q, at.R-w.Start.R
	if dq*pr-dr*pq != 0 {
		return false
	}
	t := pq*dq + pr*dr

	return t >= 0 && t <= dq*dq+dr*dr
}

// wallDerivation is every wall's geometry plus the two answers that need all
// of them together: how much of each cut cell is left, and which cells that
// seals.
type wallDerivation struct {
	// Walls is one entry per authored wall, in authored order.
	Walls []wallGeometry

	// Standing is the fraction of each footprint cell still standable, after
	// EVERY wall through it. A cell cut by two walls is decided once, by both
	// — which is the whole of how a corner seals a cell no single wall would
	// have (design C10).
	Standing map[spatial.Position]float64

	// Sealed is every footprint cell whose fraction is below [MinStandable],
	// to the wall that takes the most from it — the one a refusal names.
	Sealed map[spatial.Position]int
}

// deriveWalls works out every wall's footprint, crossings and cost against a
// floor.
//
// The floor is passed in rather than read from the spec because both callers
// already hold it and they hold it differently: [Validate] builds it while
// reporting the file's own defects, and [Compile] builds it knowing there are
// none. skip names the wall indices whose positions or direction were already
// refused — deriving from a wall nobody could read would report the same
// defect a second time in different words.
func deriveWalls(
	spec *Spec, o encounter.Orientation, floor map[spatial.Position]bool, skip map[int]bool,
) wallDerivation {
	g := geometryOf(o)
	out := wallDerivation{
		Standing: map[spatial.Position]float64{},
		Sealed:   map[spatial.Position]int{},
	}

	cutBy := map[spatial.Position][]int{}
	segments := map[int][2]worldPoint{}

	for i, w := range spec.Walls {
		if skip[i] {
			continue
		}
		// THROUGH HexCellAt, never the authored pair itself: a position names
		// its cell in the OFFSET frame the file is written in, and the
		// geometry runs on absolute axial. They agree only on even rows.
		start, okStart := g.axialAt(encounter.HexCellAt(o, w.Start.Cell[0], w.Start.Cell[1]), w.Start.Offset)
		end, okEnd := g.axialAt(encounter.HexCellAt(o, w.End.Cell[0], w.End.Cell[1]), w.End.Offset)
		if !okStart || !okEnd {
			continue
		}
		from, to := g.world(start), g.world(end)
		wg := wallGeometry{Index: i, Name: w.Name, Height: w.Height, Start: start, End: end}

		// THE CELLS NEAR THE WALL, not every cell of the floor: a wall's
		// footprint should cost what the wall is long, not what the dungeon
		// is big (review round on rpg-toolkit#1477 — measured at 4.8x on a
		// dungeon ten times the reference tomb).
		for _, cell := range g.candidateCells(floor, start, end) {
			if !g.meets(cell, from, to) {
				continue
			}
			wg.Footprint = append(wg.Footprint, cell)
			cutBy[cell] = append(cutBy[cell], i)
		}
		sortAlong(g, wg.Footprint, from, to)

		blocked := map[[2]spatial.Position]bool{}
		for _, cell := range wg.Footprint {
			for _, neighbour := range adjacencyGrid.GetNeighbors(cell) {
				if !floor[neighbour] {
					continue
				}
				if !g.blocks(cell, neighbour, from, to) {
					continue
				}
				blocked[normalizedCrossing(cell, neighbour)] = true
			}
		}
		wg.Crossings = sortedCrossings(blocked)

		segments[i] = [2]worldPoint{from, to}
		out.Walls = append(out.Walls, wg)
	}

	for cell, walls := range cutBy {
		clip := make([][2]worldPoint, 0, len(walls))
		for _, i := range walls {
			clip = append(clip, segments[i])
		}
		fraction := g.standingFraction(cell, clip)
		out.Standing[cell] = fraction
		if fraction < MinStandable {
			out.Sealed[cell] = worstWall(g, cell, walls, segments)
		}
	}

	return out
}

// worstWall is which of the walls through a cell takes the most of it — the
// one a refusal about that cell names, because it is the one an author would
// move. Ties break on authored order so the answer cannot move between runs.
func worstWall(g hexGeom, cell spatial.Position, walls []int, segments map[int][2]worldPoint) int {
	worst, least := walls[0], 2.0
	for _, i := range walls {
		if f := g.standingFraction(cell, [][2]worldPoint{segments[i]}); f < least {
			worst, least = i, f
		}
	}

	return worst
}

// sortAlong puts a footprint in order along its wall (C8): by how far down the
// segment each cell's centre projects, ties by coordinate so the answer is
// deterministic on a wall that runs along a row of side midpoints.
func sortAlong(g hexGeom, cells []spatial.Position, from, to worldPoint) {
	sort.Slice(cells, func(i, j int) bool {
		a, b := g.alongWall(cells[i], from, to), g.alongWall(cells[j], from, to)
		if a != b {
			return a < b
		}

		return cellBefore(cells[i], cells[j])
	})
}

// sortedCrossings puts a wall's blocked crossings in one coordinate order.
func sortedCrossings(set map[[2]spatial.Position]bool) [][2]spatial.Position {
	out := make([][2]spatial.Position, 0, len(set))
	for c := range set {
		out = append(out, c)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i][0] != out[j][0] {
			return cellBefore(out[i][0], out[j][0])
		}

		return cellBefore(out[i][1], out[j][1])
	})

	return out
}

// cellBefore is the one coordinate order this package sorts cells in, the same
// one [encounter.Atlas] uses: by column, then by row.
func cellBefore(a, b spatial.Position) bool {
	if a.X != b.X {
		return a.X < b.X
	}

	return a.Y < b.Y
}
