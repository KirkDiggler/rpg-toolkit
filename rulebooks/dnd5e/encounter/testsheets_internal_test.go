// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package encounter

import "fmt"

// sheetFacts is a Sheets capability answered from a fixed table. STRICT: a
// member the table does not list is refused with ErrNoSheets, never answered
// zero. A scene whose every member states nothing installs [zeroSheets].
type sheetFacts map[MemberID]SheetFacts

func (f sheetFacts) Sheets(members []MemberID) (map[MemberID]SheetFacts, error) {
	out := make(map[MemberID]SheetFacts, len(members))
	for _, id := range members {
		facts, ok := f[id]
		if !ok {
			return nil, fmt.Errorf("test fixture has no sheet for %q: %w", id, ErrNoSheets)
		}
		out[id] = facts
	}

	return out, nil
}

// zeroSheets answers the zero sheet — no speed, no actions, no strategy — for
// every member asked: a stated claim about a scene's members, installed out
// loud, not a default for a missing one.
type zeroSheets struct{}

func (zeroSheets) Sheets(members []MemberID) (map[MemberID]SheetFacts, error) {
	out := make(map[MemberID]SheetFacts, len(members))
	for _, id := range members {
		out[id] = SheetFacts{}
	}

	return out, nil
}
