// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package encounter

import (
	"sort"

	"github.com/KirkDiggler/rpg-toolkit/tools/spatial"
)

// export_route_test.go is the test-only door into the goal-directed route
// (rpg-project#762, R10) for the external package's keep and castle
// properties, and the flood it is held to. In a _test.go file, so nothing
// outside this package reaches it at build time.

// ExportedRouteWithin is [cellQuery.routeWithin] on a fresh read, with the
// cells its search settled.
func (e *Encounter) ExportedRouteWithin(
	mover MemberID, from, anchor spatial.Position, within float64,
) (path []spatial.Position, ok bool, settled int, err error) {
	q, err := e.cellQuery()
	if err != nil {
		return nil, false, 0, err
	}
	before := e.cellsSettled
	path, ok, err = q.routeWithin(mover, from, anchor, within)
	return path, ok, e.cellsSettled - before, err
}

// ExportedFloodWithin is the oracle: [cellQuery.routeTo] — the whole-floor
// flood and its nearest-stop scan — for the same goal on a fresh read.
func (e *Encounter) ExportedFloodWithin(
	mover MemberID, from, anchor spatial.Position, within float64,
) ([]spatial.Position, bool, error) {
	q, err := e.cellQuery()
	if err != nil {
		return nil, false, err
	}
	return q.routeTo(mover, from, func(cell spatial.Position) bool { return e.Distance(cell, anchor) <= within })
}

// ExportedRouteToRemembered is [cellQuery.routeToRemembered] on a fresh read,
// with the cells its search settled.
func (e *Encounter) ExportedRouteToRemembered(
	mover MemberID, from, target spatial.Position,
) (path []spatial.Position, ok bool, settled int, err error) {
	q, err := e.cellQuery()
	if err != nil {
		return nil, false, 0, err
	}
	before := e.cellsSettled
	path, ok, err = q.routeToRemembered(mover, from, target)
	return path, ok, e.cellsSettled - before, err
}

// ExportedFloodToRemembered is the oracle for [cellQuery.routeToRemembered]:
// the flood it ran before rpg-project#762, the target forgiven a creature.
func (e *Encounter) ExportedFloodToRemembered(mover MemberID, from, target spatial.Position) ([]spatial.Position, bool, error) {
	if from == target {
		return nil, true, nil
	}
	q, err := e.cellQuery()
	if err != nil {
		return nil, false, err
	}
	field, err := q.floodFrom(mover, mover, from, func(cell spatial.Position) (bool, error) {
		if cell != target {
			return false, nil
		}
		return q.blockedOnlyByCreatures(mover, cell)
	}, 0, true)
	if err != nil {
		return nil, false, err
	}
	path, ok := field.PathTo(target)
	return path, ok, nil
}

// ExportedReachable is every cell mover's own walk reaches from `from`, X then
// Y, each with its steps: the cells a property draws its goals from.
func (e *Encounter) ExportedReachable(mover MemberID, from spatial.Position) ([]spatial.Position, map[spatial.Position]int, error) {
	q, err := e.cellQuery()
	if err != nil {
		return nil, nil, err
	}
	field, err := q.floodFrom(mover, mover, from, nil, 0, true)
	if err != nil {
		return nil, nil, err
	}
	cells := make([]spatial.Position, 0, len(field.Dist))
	for cell := range field.Dist {
		cells = append(cells, cell)
	}
	sort.Slice(cells, func(i, j int) bool { return beforeInScanOrder(cells[i], cells[j]) })
	return cells, field.Dist, nil
}

// ExportedWalkFlood is the flood a player's own walk is held to: the
// mover's own picture, no door opened, unbounded — the oracle
// [Encounter.Approaches] is checked against.
func (e *Encounter) ExportedWalkFlood(mover MemberID, from spatial.Position) (map[spatial.Position]int, error) {
	q, err := e.cellQuery()
	if err != nil {
		return nil, err
	}
	field, err := q.floodFrom(mover, mover, from, nil, 0, false)
	if err != nil {
		return nil, err
	}
	return field.Dist, nil
}

// ExportedSeesFrom is the one sight test asked as if observer stood on cell
// and subject where it stands.
func (e *Encounter) ExportedSeesFrom(observer MemberID, cell spatial.Position, subject MemberID) (bool, error) {
	at, ok := e.canvas.GetEntityPosition(string(subject))
	if !ok {
		return false, nil
	}
	ranges, err := e.sightOf([]MemberID{observer})
	if err != nil {
		return false, err
	}
	positions := map[MemberID]spatial.Position{observer: cell, subject: at}
	return e.newSightReach(positions, ranges).Reaches("sight", observer, subject), nil
}

// ExportedFloodsComputed is how many distance fields this run has flooded.
func (e *Encounter) ExportedFloodsComputed() int { return e.floodsComputed }
