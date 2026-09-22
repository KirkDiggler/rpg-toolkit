// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package encounter

import "github.com/KirkDiggler/rpg-toolkit/tools/spatial"

// PointReachable reports whether a selected floor cell is within the supplied
// range and has a clear geometric path from the origin. Runtime sight-only
// volumes do not obstruct this path; they are not solid cover.
func (e *Encounter) PointReachable(from, to spatial.Position, rangeFeet int) bool {
	if e.canvas == nil || rangeFeet <= 0 || !e.canvas.field.isFloor(from) || !e.canvas.field.isFloor(to) {
		return false
	}
	return e.Distance(from, to) <= float64(CellsFromFeet(rangeFeet)) && !e.canvas.IsLineOfSightBlocked(from, to)
}

// RefreshPerception recomputes every roster member's sight after geometry has
// changed. This updates perception only: the caller must record the outcome
// that caused the change before the normal participation pass may close a
// fight or drive another turn. A pre-record refresh must never do that work.
func (e *Encounter) RefreshPerception() error {
	if e.outcome != nil {
		return nil
	}
	_, err := e.rebuildPercepts(e.rosterIDs())
	return err
}

// SeesWithin answers a pointed visibility and reach question from current
// placements, sight ranges and active volumes. Unknown members are distinguished
// from a known pair outside sight or reach.
func (e *Encounter) SeesWithin(observer, subject MemberID, rangeFeet int) (bool, bool) {
	from, ok := e.canvas.GetEntityPosition(string(observer))
	if !ok {
		return false, false
	}
	to, ok := e.canvas.GetEntityPosition(string(subject))
	if !ok {
		return false, false
	}
	if e.Distance(from, to) > float64(CellsFromFeet(rangeFeet)) {
		return false, true
	}
	ranges, err := e.sightNow()
	if err != nil {
		return false, false
	}
	reach := sightReach{positions: map[MemberID]spatial.Position{observer: from, subject: to}, cells: ranges, canvas: e.canvas, areas: e.sightAreas}
	return reach.Reaches("sight", observer, subject), true
}
