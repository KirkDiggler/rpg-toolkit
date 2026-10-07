// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package encounter_test

import (
	"fmt"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter"
)

// sheetFacts is a Sheets capability answered from a fixed table of each
// member's speed, actions and targeting.
//
// STRICT, as a host must be: a member the table does not list is refused
// with ErrNoSheets, never answered zero, so a misspelled or forgotten id
// fails the test that asked instead of passing on an invented sheet. A scene
// whose every member truly states nothing installs [zeroSheets] and says so.
type sheetFacts map[encounter.MemberID]encounter.SheetFacts

func (f sheetFacts) Sheets(members []encounter.MemberID) (map[encounter.MemberID]encounter.SheetFacts, error) {
	out := make(map[encounter.MemberID]encounter.SheetFacts, len(members))
	for _, id := range members {
		facts, ok := f[id]
		if !ok {
			return nil, fmt.Errorf("test fixture has no sheet for %q: %w", id, encounter.ErrNoSheets)
		}
		out[id] = facts
	}

	return out, nil
}

// zeroSheets is the Sheets capability for a scene whose every member's sheet
// states no speed, no actions and no strategy — said out loud by installing
// it, the way everyoneSeesTheWholeMap says what a scene believes about light.
// It is a claim about this scene's members, not a default for a missing one.
type zeroSheets struct{}

func (zeroSheets) Sheets(members []encounter.MemberID) (map[encounter.MemberID]encounter.SheetFacts, error) {
	out := make(map[encounter.MemberID]encounter.SheetFacts, len(members))
	for _, id := range members {
		out[id] = encounter.SheetFacts{}
	}

	return out, nil
}
