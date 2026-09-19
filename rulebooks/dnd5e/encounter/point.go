// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package encounter

import "github.com/KirkDiggler/rpg-toolkit/tools/spatial"

// PointReachable reports whether a selected floor cell is within the supplied
// range and has a clear geometric path from the origin. Runtime sight-only
// volumes do not obstruct this path; they are not solid cover.
func (e *Encounter) PointReachable(from, to spatial.Position, rangeFeet int) bool {
 if e.canvas == nil || rangeFeet <= 0 || !e.canvas.field.isFloor(from) || !e.canvas.field.isFloor(to) { return false }
 return e.Distance(from,to) <= float64(CellsFromFeet(rangeFeet)) && !e.canvas.IsLineOfSightBlocked(from,to)
}

// RefreshPerception recomputes every roster member's sight after geometry has
// changed. The encounter remains the sole owner of perception and contact.
func (e *Encounter) RefreshPerception() error {
 _, _, err := e.refreshSight(e.rosterIDs())
 return err
}
