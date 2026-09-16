// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package choices

import (
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/classes"
)

// LevelRequirements is what one class level ADDS to what a character is asked.
//
// It is the same shape [classes.Grant] already had for what a level gives: a
// level-tagged row, authored as data (design R4.1). Requirements used to be one
// value per class with no level in it at all, so the only thing any class could
// gain above level 1 was its subclass — a level-up screen driven by that
// surface would have rendered an empty form forever.
//
// A row says what THIS level adds, never a running total. Rogue expertise at 1
// and again at 6 is two rows of the same kind, which a cumulative total could
// not hold in one singular field without losing one of them.
type LevelRequirements struct {
	// Level is the class level these requirements are gained at.
	Level int

	// Requirements is what the level asks for.
	Requirements
}

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

// GetClassRequirementsGainedAtLevel returns everything a class asks for AT a
// class level and did not ask for before it.
//
// This is the primitive both creation and advancement read (design R4.2).
// Creation asks at level 1, where "gained at" and "has in total" are the same
// answer; advancement asks at level N, where every choice an earlier level
// required has already been made and only the difference is a question.
//
// Three sources compose into the answer, each owning exactly one fact:
//
//   - the class's authored row for that level, which is data;
//   - the spell and cantrip questions, DERIVED from the class's progression
//     table rather than authored beside it (R4.6);
//   - the subclass, derived from [classes.Data].SubclassLevel rather than
//     restated per class (R4.3: one fact, one home).
//
// Returns an empty (non-nil) Requirements for a level that asks nothing, which
// is most levels of most classes and is a valid level, not an error.
func GetClassRequirementsGainedAtLevel(classID classes.Class, classLevel int) *Requirements {
	reqs := &Requirements{}
	if classLevel < 1 {
		return reqs
	}

	for _, row := range classLevelRequirements(classID) {
		if row.Level == classLevel {
			*reqs = row.Requirements
			break
		}
	}

	addDerivedSpellRequirements(reqs, classID, classLevel)

	// The subclass is chosen at exactly one level, and that level is a field on
	// the class rather than a row here: cleric at 1, wizard at 2, fighter at 3,
	// one fact with three answers and no branching (design §2).
	classData := classes.ClassData[classID]
	if classData != nil && classData.SubclassLevel == classLevel {
		reqs.Subclass = &SubclassRequirement{
			ID:      ChoiceID(classData.SubclassChoiceID),
			Options: classData.Subclasses,
			Label:   classData.SubclassLabel,
		}
	}

	return reqs
}

// GetClassChoiceIDsGainedAtLevel names the choices a class asks for at a class
// level and did not ask for before it.
//
// It was the difference between two cumulative totals while requirements were
// level-blind; now the level's own row IS the answer, so the subtraction is
// gone along with the function it subtracted.
func GetClassChoiceIDsGainedAtLevel(classID classes.Class, classLevel int) []ChoiceID {
	if classLevel < 1 {
		return nil
	}
	return GetClassRequirementsGainedAtLevel(classID, classLevel).ChoiceIDs()
}
