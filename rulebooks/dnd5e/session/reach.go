// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package session

import (
	"fmt"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter"
	"github.com/KirkDiggler/rpg-toolkit/tools/spatial"
)

// rosterPositions indexes a roster read by member id, for the reach checks
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

// inReach reports whether to is within reach cells of from, on enc's own
// grid (Encounter.Distance — the same primitive refreshSight's sight check
// uses internally, exposed minimally: rpg-toolkit#1010).
func inReach(enc *encounter.Encounter, from, to spatial.Position, reach int) bool {
	return enc.Distance(from, to) <= float64(reach)
}

// refuseOutOfReach refuses target unless it is within reach cells of
// attacker's current position — session.Attack's own gate, ported from the
// retired top-level encounter module's checkReach (encounter/range_gate.go):
// melee one cell, the Reach property two, read off the compiled profile
// rather than re-derived here.
//
// Both ids are already known to be members of positions by the time this is
// called — Attack's own roster checks run first — so a missing entry would
// be this function being asked about someone that earlier check should have
// refused, not a case this seam narrates to a caller.
func refuseOutOfReach(enc *encounter.Encounter, roster []encounter.Member, attacker, target string, reach int) error {
	positions := rosterPositions(roster)
	if !inReach(enc, positions[attacker], positions[target], reach) {
		return fmt.Errorf("target %q: %w", target, ErrOutOfReach)
	}
	return nil
}
