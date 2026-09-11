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

// rosterSpeeds indexes a roster read by member id for the one fact a directed
// move needs and the content cannot supply: how far this creature's own legs
// carry it, in feet.
//
// OFF THE SAME READ AS THE POSITIONS, and beside them for that reason. A cast
// that budgets a push by the mover's own speed asks two questions about the
// same roster row — where it stands and how fast it is — and taking them from
// one read is what keeps the two answers describing one moment.
//
// A member with no speed on its row answers zero, which is the honest reading
// of a fact nobody supplied rather than a walk of unknown length. The caller
// turns that zero into a refusal with a sentence on it rather than a silent
// distance of nothing — see routeCastPushes and noSpeedToRunWith.
func rosterSpeeds(roster []encounter.Member) map[string]int {
	out := make(map[string]int, len(roster))
	for _, m := range roster {
		out[string(m.ID)] = m.SpeedFeet
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
