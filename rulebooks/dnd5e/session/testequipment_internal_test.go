// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package session

import (
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/combat"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter"
)

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

// Conditions answers "nothing to observe" for every member asked, for the same
// reason Equipment does: this fixture has no sheets to report from.
func (encNoHandsObserved) Conditions(
	members []encounter.MemberID,
) (map[encounter.MemberID]*encounter.ConditionSet, error) {
	out := make(map[encounter.MemberID]*encounter.ConditionSet, len(members))
	for _, id := range members {
		out[id] = nil
	}
	return out, nil
}

var _ encounter.EquipmentWithConditions = encNoHandsObserved{}

// encStandStill answers the sheet question for a scene being ASSEMBLED, never
// played: every member asked stands still with nothing to swing and no
// strategy — the facts these authored members always carried — so a sight pass
// during construction that forms a fight paces and budgets nobody. Once the
// session loads the world it answers from each member's own sheet (sheets.go).
type encStandStill struct{}

func (encStandStill) Sheets(members []encounter.MemberID) (map[encounter.MemberID]encounter.SheetFacts, error) {
	out := make(map[encounter.MemberID]encounter.SheetFacts, len(members))
	for _, id := range members {
		out[id] = encounter.SheetFacts{}
	}
	return out, nil
}

// encStatedSight answers the sight question for a scene being assembled: the
// rulebook's stated default range for every member, which is what a member
// whose sheet states no range answers once the session plays the world.
type encStatedSight struct{}

func (encStatedSight) Sight(members []encounter.MemberID) (map[encounter.MemberID]int, error) {
	out := make(map[encounter.MemberID]int, len(members))
	for _, id := range members {
		out[id] = encounter.CellsFromFeet(combat.DefaultSightFeet)
	}
	return out, nil
}
