// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package dungeonspec_test

import "github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter"

// zeroSheets is the Sheets capability these tests install: every member's
// sheet states no speed, no actions and no strategy. Geometry tests may
// explicitly Step, but these sheets pace no world-clock rounds and supply no
// driven movement or attack budget. Installing it states that fixture contract.
type zeroSheets struct{}

func (zeroSheets) Sheets(members []encounter.MemberID) (map[encounter.MemberID]encounter.SheetFacts, error) {
	out := make(map[encounter.MemberID]encounter.SheetFacts, len(members))
	for _, id := range members {
		out[id] = encounter.SheetFacts{}
	}

	return out, nil
}
