// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package dungeonspec_test

import "github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter"

// sheetFacts is the Sheets capability these tests install: the zero value —
// no speed, no actions, no strategy — for every member asked, or a listed
// member's own facts. Nothing in this package's tests walks or drives anybody.
type sheetFacts map[encounter.MemberID]encounter.SheetFacts

func (f sheetFacts) Sheets(members []encounter.MemberID) (map[encounter.MemberID]encounter.SheetFacts, error) {
	out := make(map[encounter.MemberID]encounter.SheetFacts, len(members))
	for _, id := range members {
		out[id] = f[id]
	}

	return out, nil
}
