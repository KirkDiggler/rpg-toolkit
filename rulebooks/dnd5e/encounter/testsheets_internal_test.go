// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package encounter

// sheetFacts is the Sheets capability the internal tests install: a fixed
// table of each member's speed, actions and targeting, answered for every
// member asked, and the zero value for a member it does not list.
type sheetFacts map[MemberID]SheetFacts

func (f sheetFacts) Sheets(members []MemberID) (map[MemberID]SheetFacts, error) {
	out := make(map[MemberID]SheetFacts, len(members))
	for _, id := range members {
		out[id] = f[id]
	}

	return out, nil
}
