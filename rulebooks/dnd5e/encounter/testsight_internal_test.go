// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package encounter

// everyoneSeesTheWholeMap is the Sight capability the internal tests install.
//
// Nobody is ever bounded by distance: these tests are about clocks, not about
// light, and the capability is required at construction so they have to say so.
// The rulebook's real answer is the production consumer's business.
type everyoneSeesTheWholeMap struct{}

func (everyoneSeesTheWholeMap) Sight(members []MemberID) (map[MemberID]int, error) {
	out := make(map[MemberID]int, len(members))
	for _, id := range members {
		out[id] = 1_000_000
	}

	return out, nil
}
