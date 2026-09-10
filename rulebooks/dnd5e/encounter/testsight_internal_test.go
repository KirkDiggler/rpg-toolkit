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

// noHandsAreObserved answers the equipment question for tests that are not
// about equipment: every member is present in the answer, and every answer is
// "no hands to observe". That is the honest default for a fixture roster of
// bare member IDs with no sheets behind them — and it is deliberately NOT
// "everybody is empty-handed", which would be testimony this fixture has no
// standing to give. See [encounter.Equipment].
type noHandsAreObserved struct{}

func (noHandsAreObserved) Equipment(members []MemberID) (map[MemberID]*HeldEquipment, error) {
	out := make(map[MemberID]*HeldEquipment, len(members))
	for _, id := range members {
		out[id] = nil
	}
	return out, nil
}
