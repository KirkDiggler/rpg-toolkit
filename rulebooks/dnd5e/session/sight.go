// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package session

import "github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter"

// sightRangeCells is how far a member can see, in cells — a torch's bright
// light, and no further.
//
// # This is a policy, not a default
//
// The composition REFUSES to answer this itself (encounter.ErrNoSight): sight
// is per-creature and per-light-source, so a number living down there would be
// that module inventing a rule 5e does not have. It asks whoever holds the
// sheets, which is this package, and this constant is that answer written down
// where it can be argued with.
//
// # Why a number, and why this one
//
// Four cells is TWENTY FEET at five feet to the cell, which is a torch's bright
// radius. Nothing on a character sheet says how far anybody sees yet — no
// darkvision, no senses, no light sources — so this is the party's own light
// standing in for all of it, uniformly, until sheets can say better.
//
// Unbounded was tried first and is wrong, for a reason worth keeping: with no
// range, ONE OPEN DOOR MAKES THE WHOLE DUNGEON ONE FIGHT. Sight becomes purely
// geometric, an open archway is a sightline into every cell of the room beyond,
// and everyone who can see anything is in contact with it. The workbench scene
// is the demonstration — its whole point is that a fight is localised to the
// members in contact while the rest of the party keeps exploring, and at
// unlimited range every member joined every fight.
//
// So the number is doing real work, and it is Kirk's to move. When light and
// darkvision land this becomes per-member and reads the sheet; the seam is
// already shaped for that, since the capability is asked per member and answers
// per member, so only this file changes.
const sightRangeCells = 4

// sightSeam answers the composition's sight question for every member.
//
// Deliberately stateless and deliberately not caching: the composition asks at
// every refresh including first light, and an answer cached here would go stale
// the moment sight becomes a function of anything that moves — which is the
// whole point of it being asked rather than configured.
type sightSeam struct{}

// Sight reports how far each member can see, in cells.
//
// Every member asked about appears in the answer and nobody else does, which is
// the capability's stated contract. There is no per-member variation yet, so
// the loop is uniform — see [sightRangeCells] for why that is a position rather
// than a shortcut.
func (sightSeam) Sight(members []encounter.MemberID) (map[encounter.MemberID]int, error) {
	out := make(map[encounter.MemberID]int, len(members))
	for _, id := range members {
		out[id] = sightRangeCells
	}

	return out, nil
}
