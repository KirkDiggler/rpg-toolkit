// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package session

import "github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter"

// defaultSightFeet is the stated default absent a stated number
// (rpg-project#254 design §5, Kirk's ruling) — 120 feet, 24 cells: normal
// vision on a lit dungeon floor, this build's answer until a light model
// exists to do better (see encounter/sight.go's own doc on this
// capability).
//
// ONE CONSTANT, BOTH KINDS. A member's own stated SensesData wins when it
// has one — a monster's real darkvision, once a stat block authors it —
// and this is the fallback for EITHER kind whose stored SightFeet is zero:
// a character with none stated, or a monster whose stat block has not had
// Senses authored onto it yet. Silence means the same thing regardless of
// who is silent; a narrower monster-only floor was tried and retired
// (rpg-project#254 review) — it read as a fact about darkness this build
// does not model, when it was really standing in for "nobody has written
// this stat block's Senses yet". Lighting is a later slice, and a
// genuinely narrower number belongs there, read off content, not guessed
// at here by kind.
const defaultSightFeet = 120

// sightSeam answers the composition's sight question for every member, from
// a snapshot of the world's own persisted member facts (encounter.MemberData,
// specifically ID and SightFeet — filled at Join/Spawn from the sheet or
// monster.Data and round-tripped by the composition itself,
// rpg-toolkit#1187).
//
// A SNAPSHOT, NOT A LIVE *encounter.Encounter — deliberately, to avoid the
// circular dependency a live reference would create: this capability is
// itself an input to LoadEncounter, so it cannot hold the encounter that
// call is still constructing. The snapshot is seeded from the SAME
// encounter.EncounterData a write scope already read (or already holds, for
// Manager.adopt) before calling LoadEncounter, so this costs no second
// fetch.
//
// A POINTER, NOT A VALUE, and that is load-bearing rather than a style
// choice. A member being placed by THIS SAME call — Join, Spawn — triggers
// its own sight refresh as part of joining (does it see anyone; is it seen)
// before this seam's snapshot has any way to know it exists: the snapshot
// was taken from the world as it stood before the placement, and the
// placement itself is what teaches the world about the new member. [place]
// closes that gap by calling add on the SAME *sightSeam the live encounter
// already holds, immediately before the Join call that needs the answer —
// which only works because every holder of this capability shares the one
// pointer, never a copy.
//
// STILL LOS-BOUNDED, unchanged: this answers RANGE alone, in cells — the
// composition's own rebuildPercepts separately walls off anything a wall or
// door blocks, exactly as it always has. Nothing about that changes here.
type sightSeam struct {
	members []encounter.MemberData
}

// add records one member's sight-relevant facts — ID and SightFeet are all
// Sight ever reads — so a member placed mid-call is known to THIS seam
// before its own placement asks about it. See the type's own doc.
func (s *sightSeam) add(id encounter.MemberID, sightFeet int) {
	s.members = append(s.members, encounter.MemberData{ID: id, SightFeet: sightFeet})
}

// Sight reports how far each member can see, in cells — read off each
// member's own stored SightFeet (feet), converted once via
// encounter.CellsFromFeet, defaulting to defaultSightFeet for a member with
// no stored value.
func (s *sightSeam) Sight(members []encounter.MemberID) (map[encounter.MemberID]int, error) {
	feet := make(map[encounter.MemberID]int, len(s.members))
	for _, m := range s.members {
		feet[m.ID] = m.SightFeet
	}

	out := make(map[encounter.MemberID]int, len(members))
	for _, id := range members {
		f := feet[id]
		if f <= 0 {
			f = defaultSightFeet
		}
		out[id] = encounter.CellsFromFeet(f)
	}

	return out, nil
}
