// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package session

import "github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter"

// encNoHandsObserved answers the equipment question for fixtures that are not
// about equipment: every member is answered for, every answer is "no hands to
// observe" — deliberately NOT "everybody is empty-handed", which would be
// testimony this fixture has no standing to give.
type encNoHandsObserved struct{}

func (encNoHandsObserved) Equipment(
	members []encounter.MemberID,
) (map[encounter.MemberID]*encounter.HeldEquipment, error) {
	out := make(map[encounter.MemberID]*encounter.HeldEquipment, len(members))
	for _, id := range members {
		out[id] = nil
	}
	return out, nil
}
