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

// sheetSpeeds asks the members' own sheets for the one fact a directed move
// needs and the content cannot supply: how far this creature's own legs carry
// it, in feet.
//
// THE SAME ANSWER THE COMPOSITION GETS. A push budgeted by the mover's speed
// asks the [sheetSeam] the encounter's own pace and turn budget ask, at the
// moment the push runs — never a copy taken at Join, which is the record
// rpg-project#538 retired. A member the verb holds no sheet for is refused by
// the seam, not answered with zero; a sheet that answers zero is a true speed
// of zero, and the caller turns that into a refusal with a sentence on it
// (routeCastPushes, noSpeedToRunWith).
func sheetSpeeds(sheets encounter.Sheets, members []encounter.MemberID) (map[string]int, error) {
	if len(members) == 0 {
		return map[string]int{}, nil
	}
	facts, err := sheets.Sheets(members)
	if err != nil {
		return nil, err
	}
	out := make(map[string]int, len(facts))
	for id, f := range facts {
		out[string(id)] = f.SpeedFeet
	}
	return out, nil
}

// inRange reports whether to is within the declared maximum range in feet of
// from, on enc's own grid (Encounter.Distance — the same primitive
// refreshSight's sight check uses internally, exposed minimally:
// rpg-toolkit#1010). Range is converted to cells once, here, via
// encounter.CellsFromFeet; a cell is five feet.
func inRange(enc *encounter.Encounter, from, to spatial.Position, rangeFeet int) bool {
	return enc.Distance(from, to) <= float64(encounter.CellsFromFeet(rangeFeet))
}
