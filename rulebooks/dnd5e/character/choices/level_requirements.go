// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package choices

import (
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/classes"
)

// ChoiceIDs returns the identifier of every choice these requirements ask for,
// in a stable order.
//
// One list rather than eleven fields, so that a caller comparing two sets of
// requirements — "what does THIS level newly ask for?" — does not have to know
// which kinds of requirement exist, and does not silently miss a kind added
// later.
func (r *Requirements) ChoiceIDs() []ChoiceID {
	if r == nil {
		return nil
	}

	ids := make([]ChoiceID, 0)

	if r.Skills != nil {
		ids = append(ids, r.Skills.ID)
	}
	for _, req := range r.AdditionalSkills {
		if req != nil {
			ids = append(ids, req.ID)
		}
	}
	for _, req := range r.Equipment {
		if req != nil {
			ids = append(ids, req.ID)
		}
	}
	for _, req := range r.EquipmentCategories {
		if req != nil {
			ids = append(ids, req.ID)
		}
	}
	for _, req := range r.Languages {
		if req != nil {
			ids = append(ids, req.ID)
		}
	}
	if r.Tools != nil {
		ids = append(ids, r.Tools.ID)
	}
	if r.FightingStyle != nil {
		ids = append(ids, r.FightingStyle.ID)
	}
	if r.Expertise != nil {
		ids = append(ids, r.Expertise.ID)
	}
	if r.Subclass != nil {
		ids = append(ids, r.Subclass.ID)
	}
	if r.Cantrips != nil {
		ids = append(ids, r.Cantrips.ID)
	}
	if r.Spellbook != nil {
		ids = append(ids, r.Spellbook.ID)
	}

	return ids
}

// GetClassChoiceIDsGainedAtLevel returns the choices a class asks for AT this
// level and did not ask for before it.
//
// [GetClassRequirementsAtLevel] answers what a character of level N must have
// chosen in TOTAL — the right question at creation, and the wrong one at a
// level-up, where every choice an earlier level required has already been
// made. The difference between level N's total and level N-1's is what level N
// itself adds, which is what advancement needs (design §4.1 step 5).
//
// Level 1 has no level before it, so every choice the class asks for is gained
// there; levels below 1 gain nothing.
func GetClassChoiceIDsGainedAtLevel(classID classes.Class, level int) []ChoiceID {
	if level < 1 {
		return nil
	}

	current := GetClassRequirementsAtLevel(classID, level).ChoiceIDs()
	if level == 1 {
		return current
	}

	previous := make(map[ChoiceID]struct{})
	for _, id := range GetClassRequirementsAtLevel(classID, level-1).ChoiceIDs() {
		previous[id] = struct{}{}
	}

	gained := make([]ChoiceID, 0)
	for _, id := range current {
		if _, had := previous[id]; !had {
			gained = append(gained, id)
		}
	}
	return gained
}
