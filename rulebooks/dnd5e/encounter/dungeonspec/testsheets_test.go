// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package dungeonspec_test

import "github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter"

// zeroSheets is the Sheets capability these tests install: every member's
// sheet states no speed, no actions and no strategy. Nothing in this
// package's tests walks or drives anybody, and installing it says so.
type zeroSheets struct{}

func (zeroSheets) Sheets(members []encounter.MemberID) (map[encounter.MemberID]encounter.SheetFacts, error) {
	out := make(map[encounter.MemberID]encounter.SheetFacts, len(members))
	for _, id := range members {
		out[id] = encounter.SheetFacts{}
	}

	return out, nil
}
