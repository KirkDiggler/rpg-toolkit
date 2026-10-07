// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package encounter_test

import "github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter"

// sheetFacts is the Sheets capability these tests install: a fixed table of
// each member's speed, actions and targeting, answered for every member asked.
//
// A member the table does not list is answered the zero value — no speed, no
// actions, no strategy — which is what a player is to a driver and what every
// scene that never set a speed was already assuming. A test about the
// capability's refusals installs its own capability rather than this one.
type sheetFacts map[encounter.MemberID]encounter.SheetFacts

func (f sheetFacts) Sheets(members []encounter.MemberID) (map[encounter.MemberID]encounter.SheetFacts, error) {
	out := make(map[encounter.MemberID]encounter.SheetFacts, len(members))
	for _, id := range members {
		out[id] = f[id]
	}

	return out, nil
}
