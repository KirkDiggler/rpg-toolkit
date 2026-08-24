// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package session

import (
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter"
	"github.com/KirkDiggler/rpg-toolkit/tools/spatial"
)

// rosterPositions indexes a roster read by member id, for the range checks
// below and for Afford's per-target declarations — both need "where does
// this member stand" and both already hold the roster read that answers it,
// so this is a lookup rather than a second fetch.
func rosterPositions(roster []encounter.Member) map[string]spatial.Position {
	out := make(map[string]spatial.Position, len(roster))
	for _, m := range roster {
		out[string(m.ID)] = m.Position
	}
	return out
}

// inRange reports whether to is within the declared maximum range in feet of
// from, on enc's own grid (Encounter.Distance — the same primitive
// refreshSight's sight check uses internally, exposed minimally:
// rpg-toolkit#1010). Range is converted to cells once, here, via
// encounter.CellsFromFeet; a cell is five feet.
func inRange(enc *encounter.Encounter, from, to spatial.Position, rangeFeet int) bool {
	return enc.Distance(from, to) <= float64(encounter.CellsFromFeet(rangeFeet))
}
