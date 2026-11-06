// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package encounter

import (
	"math"

	"github.com/KirkDiggler/rpg-toolkit/tools/spatial"
)

// route_search.go is A ROUTE COSTS ITS REACH (rpg-project#762, R10 on lane
// rpg-project#754): a route to one goal searches toward it, settling the cells
// near its path, rather than flooding the whole floor and then scanning the
// flood for the nearest stop. On the 10k keep the flood was ~89% of a
// patrol's turn; the search settles what lies between the walker and its goal.
//
// THE SAME ANSWER AS THE FLOOD, NOT MERELY AN EQUALLY GOOD ONE (C8). The
// search walks by the flood's own edge rule ([cellQuery.walkerEdge]), picks
// the flood's stop (fewest steps, then X, then Y — [cellQuery.nearestStop]'s
// order) and reads back the flood's path: [spatial.Field] enters each cell
// from the first neighbour it settles one step nearer the source, which on a
// unit-cost grid is the X-then-Y least of them, and the search keeps exactly
// that predecessor. It can, because it settles every cell whose best-case
// route to the goal (steps so far plus the heuristic) is no longer than the
// route it found: with a consistent heuristic every cell on any shortest
// route to any nearest stop is among those, with its true distance, so the
// flood's stop and every predecessor the flood would read are in hand when
// the search ends.
//
// THE HEURISTIC is the grid's own distance to the anchor, less the reach,
// rounded up and never below zero: a step changes a hex (or Chebyshev)
// distance by at most one, so it never overestimates the steps left and never
// drops by more than one across a step — admissible and consistent. A
// gridless floor has no such step and keeps the flood.
//
// THE FLOOD STAYS for every question that is not one goal: a budgeted reach
// ([cellQuery.routeToward], a dash), a field read whole (a search's trails,
// a threat zone), and any goal predicate without an anchor
// ([cellQuery.routeTo]). Where a read has already flooded from this walker's
// cell, the route reads that flood rather than searching again.

// routeWithin is [cellQuery.routeTo] for the goal "within `within` cells of
// `anchor`", cell for cell the same answer: the shortest route for `mover`
// from `from` to the nearest cell it may STOP on whose distance to anchor is
// at most within, ties by X then Y; an empty path with ok=true when from
// already is; ok=false when no reachable stop is. Exact-cell routes pass zero.
func (e *cellQuery) routeWithin(
	mover MemberID, from, anchor spatial.Position, within float64,
) ([]spatial.Position, bool, error) {
	goal := func(cell spatial.Position) bool { return e.Distance(cell, anchor) <= within }
	if goal(from) {
		return nil, true, nil
	}
	if _, flooded := e.floods[floodKey{mover: mover, walker: mover, from: from, opens: true}]; flooded || !e.searchable() {
		return e.routeTo(mover, from, goal)
	}

	return e.searchToward(mover, mover, from, anchor, within, nil, true, func(cell spatial.Position) (bool, error) {
		if cell == from || !goal(cell) {
			return false, nil
		}
		// May cross an ally, may not stop on one.
		fact, err := e.cellAt(CellAtInput{Cell: cell, Mover: mover})
		if err != nil {
			return false, err
		}
		return fact.Passage == PassageStandable, nil
	})
}

// searchable is whether this floor's grid has the step the search's heuristic
// rests on: one step, one cell of the grid's distance.
func (e *cellQuery) searchable() bool {
	switch e.canvas.GetGrid().GetShape() {
	case spatial.GridShapeHex, spatial.GridShapeSquare:
		return true
	default:
		return false
	}
}

// searchToward is the goal-directed search (A*) the single-goal routes run:
// from `from` by [cellQuery.walkerEdge]'s rule (mover, walker, forgiven,
// opens as the flood takes them), toward `anchor`, to the cell `stop` accepts
// in the fewest steps, ties by X then Y. Returns the path EXCLUDING from and
// INCLUDING the stop — the flood's path to the flood's stop (see the file's
// doc) — or ok=false when no reachable cell is a stop, which it learns the
// way the flood does: by settling everything reachable.
//
// `stop` must only accept cells within `within` of anchor; the heuristic is
// a promise about where the stops are.
func (e *cellQuery) searchToward(
	mover, walker MemberID, from, anchor spatial.Position, within float64,
	forgiven func(spatial.Position) (bool, error), opens bool,
	stop func(spatial.Position) (bool, error),
) ([]spatial.Position, bool, error) {
	path, ok, _, err := e.searchTowardField(mover, walker, from, anchor, within, forgiven, opens, stop)
	return path, ok, err
}

// searchTowardField is [cellQuery.searchToward] that also hands back what it
// settled. When it finds NO stop it has settled every cell the walk reaches,
// each at its true distance and entered from the flood's own predecessor
// (see the file's doc), so the field it returns then IS the unbounded flood
// from `from` by the same edge rule — a caller asking more of that floor
// reads it rather than flooding again. When it finds a stop the field is
// partial and means nothing to anyone else.
func (e *cellQuery) searchTowardField(
	mover, walker MemberID, from, anchor spatial.Position, within float64,
	forgiven func(spatial.Position) (bool, error), opens bool,
	stop func(spatial.Position) (bool, error),
) ([]spatial.Position, bool, spatial.FieldOutput, error) {
	grid := e.canvas.GetGrid()
	passable, edgeErr := e.walkerEdge(mover, walker, forgiven, opens)
	heuristic := func(cell spatial.Position) int {
		left := math.Ceil(grid.Distance(cell, anchor) - within)
		if left <= 0 {
			return 0
		}
		return int(left)
	}

	dist := map[spatial.Position]int{from: 0}
	prev := map[spatial.Position]spatial.Position{}
	queue := searchQueue{{pos: from, g: 0, f: heuristic(from)}}
	var best spatial.Position
	bestDist, found := 0, false

	for len(queue) > 0 {
		cur := queue.pop()
		// Every cell no worse than the stop found is settled; past that,
		// nothing can be a nearer stop or a predecessor the path reads.
		if found && cur.f > bestDist {
			break
		}
		if cur.g > dist[cur.pos] {
			continue // a stale entry, left behind when a shorter route arrived
		}
		e.cellsSettled++
		ok, err := stop(cur.pos)
		if err != nil {
			return nil, false, spatial.FieldOutput{}, err
		}
		if ok && (!found || cur.g < bestDist || (cur.g == bestDist && beforeInScanOrder(cur.pos, best))) {
			best, bestDist, found = cur.pos, cur.g, true
		}
		// A cell as far as the stop leads only to cells farther than it.
		if found && cur.g >= bestDist {
			continue
		}
		// In whatever order the grid lists them: the predecessor below is
		// the least of the nearer neighbours however they arrive.
		for _, n := range grid.GetNeighbors(cur.pos) {
			if !passable(cur.pos, n) {
				continue
			}
			d := cur.g + 1
			if old, seen := dist[n]; seen {
				// THE FLOOD'S PREDECESSOR: of the neighbours one step
				// nearer, the X-then-Y least — the one the flood settles
				// first and enters this cell from.
				if d == old && beforeInScanOrder(cur.pos, prev[n]) {
					prev[n] = cur.pos
				}
				if d >= old {
					continue
				}
			}
			dist[n], prev[n] = d, cur.pos
			queue.push(searchItem{pos: n, g: d, f: d + heuristic(n)})
		}
	}
	if *edgeErr != nil {
		return nil, false, spatial.FieldOutput{}, *edgeErr
	}
	field := spatial.FieldOutput{Dist: dist, Prev: prev}
	if !found {
		return nil, false, field, nil
	}

	var rev []spatial.Position
	for cell := best; cell != from; cell = prev[cell] {
		rev = append(rev, cell)
	}
	for i, j := 0, len(rev)-1; i < j; i, j = i+1, j-1 {
		rev[i], rev[j] = rev[j], rev[i]
	}
	return rev, true, field, nil
}

// searchItem is one cell waiting to be settled: g its steps from the source,
// f those plus the heuristic.
type searchItem struct {
	pos  spatial.Position
	g, f int
}

// searchQueue is a min-heap of searchItem by f, then the deeper cell first
// (larger g, so the search runs down its path rather than across the band of
// equal f), then X, then Y. The order decides only how soon the answer is
// found, never which answer: the search settles every cell of the stop's f
// before it ends.
type searchQueue []searchItem

func (q searchQueue) less(i, j int) bool {
	a, b := q[i], q[j]
	if a.f != b.f {
		return a.f < b.f
	}
	if a.g != b.g {
		return a.g > b.g
	}
	return beforeInScanOrder(a.pos, b.pos)
}

func (q *searchQueue) push(item searchItem) {
	*q = append(*q, item)
	h := *q
	for i := len(h) - 1; i > 0; {
		parent := (i - 1) / 2
		if !h.less(i, parent) {
			break
		}
		h[i], h[parent] = h[parent], h[i]
		i = parent
	}
}

func (q *searchQueue) pop() searchItem {
	h := *q
	top := h[0]
	last := len(h) - 1
	h[0] = h[last]
	h = h[:last]
	for i := 0; ; {
		least, l, r := i, 2*i+1, 2*i+2
		if l < len(h) && h.less(l, least) {
			least = l
		}
		if r < len(h) && h.less(r, least) {
			least = r
		}
		if least == i {
			break
		}
		h[i], h[least] = h[least], h[i]
		i = least
	}
	*q = h
	return top
}
