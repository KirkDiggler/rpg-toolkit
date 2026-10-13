// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package encounter

import (
	"github.com/KirkDiggler/rpg-toolkit/tools/spatial"
)

// patrol.go is A PATROL, DERIVED FROM WHERE THE CREATURE STANDS
// (rpg-project#569 R7, as refined twice on the review of rpg-toolkit#2044).
//
// # A leg is a line, not a band
//
// `toward: { path: [w0, w1, …] }` walks a loop of LEGS, w0 to w1, w1 to w2,
// and the last back to the first. Each leg is ONE deterministic run of
// cells: the hex line between its two waypoints ([LegLine], the cube-lerp
// line spatial draws for sight). "Any shortest walk" between two hexes is a
// band, and a band smears neighbouring legs into one another; a line does
// not. And the creature walks THAT line — the same cells, a Move cut to its
// budget — so the cell it ends a round on is a cell of its leg, and the next
// round knows which leg it is on with nothing remembered.
//
// The live pathfinder ([Encounter.Route]) is deliberately NOT the leg: its
// answer reads who is standing where, who is down and which doors are open
// at that moment, so it is not the same route twice and could not be judged
// at the door. It is how a creature knocked OFF its route gets back to it.
//
// # A route that retraces itself is refused
//
// If two legs share a cell (other than the waypoint one ends and the next
// begins on), a creature standing there is on both, and which way it walks is
// a guess. So such a route is refused at every door ([RetracedCell]): the
// dialect's form, the member door, and a loaded blob. That refuses the
// two-waypoint corridor — A to B and B back to A are the same cells — which
// is the named next use case for a direction derivation, not a thing this
// slice reads by guessing.

// LegLine is the cells of the patrol leg from a to b, both ends included, in
// walking order: the hex line ([spatial.AxialHexGrid.GetLineOfSight]), each
// cell adjacent to the one before it. The cells a creature on that leg walks.
//
// EXPORTED FOR THE DIALECT, which asks whether every cell of a leg is floor
// before a route reaches the board — the same line, so the form and the walk
// cannot disagree about where a patrol goes.
func LegLine(a, b spatial.Position) []spatial.Position {
	return adjacencyGrid.GetLineOfSight(a, b)
}

// RetracedCell is the first cell two legs of a patrol route share — the
// reason the route is refused — or ok=false for a route whose legs cross
// nowhere. The waypoint where one leg ends and the next begins is not a
// crossing; for a route of three or more waypoints the first waypoint is
// also where the last leg ends. A route of two waypoints retraces itself by
// construction, and one of a single waypoint has nothing to cross.
//
// EXPORTED FOR THE DIALECT, which refuses the same route on the author's form
// in the author's coordinates ([OffsetOf]) — one statement of the rule.
func RetracedCell(path []spatial.Position) (spatial.Position, bool) {
	n := len(path)
	if n < 2 {
		return spatial.Position{}, false
	}
	legs := make([][]spatial.Position, n)
	for i := range path {
		legs[i] = LegLine(path[i], path[(i+1)%n])
	}
	for i := 0; i < n; i++ {
		for j := i + 1; j < n; j++ {
			for _, cell := range legs[j] {
				if !onLine(legs[i], cell) {
					continue
				}
				// The joint between consecutive legs is where the route turns,
				// not where it crosses itself.
				if j == i+1 && cell == path[j] {
					continue
				}
				if n > 2 && i == 0 && j == n-1 && cell == path[0] {
					continue
				}

				return cell, true
			}
		}
	}

	return spatial.Position{}, false
}

// onLine reports whether a cell is one of a leg's cells.
func onLine(line []spatial.Position, cell spatial.Position) bool {
	for _, c := range line {
		if c == cell {
			return true
		}
	}

	return false
}

// patrolStep is what a creature standing at from does next on its route:
// the cells of its leg still ahead of it, or — off the route — the waypoint
// it goes back to.
//
//   - ON A WAYPOINT, or partway along a leg: the rest of that leg's line, to
//     the next waypoint. A waypoint is the first cell of the leg it starts,
//     so standing on one walks the leg after it, and the last leg leads back
//     to the first waypoint: a loop.
//   - OFF THE ROUTE: the nearest waypoint, ties to the earlier in the
//     author's order, walked to by the live pathfinder.
//
// A route of one waypoint is a post: the creature goes to it and stays.
func patrolStep(from spatial.Position, path []spatial.Position) (ahead []spatial.Position, back spatial.Position) {
	n := len(path)
	if n > 1 {
		for i := range path {
			line := LegLine(path[i], path[(i+1)%n])
			for k := 0; k < len(line)-1; k++ {
				if line[k] == from {
					return line[k+1:], spatial.Position{}
				}
			}
		}
	}
	best := 0
	for i := range path {
		if adjacencyGrid.Distance(from, path[i]) < adjacencyGrid.Distance(from, path[best]) {
			best = i
		}
	}

	return nil, path[best]
}
