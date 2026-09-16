// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package choices

import (
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/shared"
)

// SubmissionsFrom turns recorded choices into the submissions the validator
// reads.
//
// [ChoiceData] is a sum type — one selection field is populated and which one
// depends on the category — while a [Submission] is one flat list of selection
// ids. Something has to make that translation, and until advancement needed it
// the only copy lived inside the draft. Two copies of this switch is two
// readings of what a category means, and the one advancement wrote would be the
// one nothing at creation exercised (design R4.4b: validate "through the same
// validator").
//
// A choice whose selection field is empty contributes nothing: an empty
// submission would read as an answer of length zero and fail a count check with
// a message about the wrong thing, where no submission at all reads as "not
// answered", which is what it is.
func SubmissionsFrom(data []ChoiceData) *Submissions {
	submissions := NewSubmissions()

	for _, choice := range data {
		switch choice.Category {
		case shared.ChoiceSkills:
			if len(choice.SkillSelection) > 0 {
				values := make([]shared.SelectionID, 0, len(choice.SkillSelection))
				values = append(values, choice.SkillSelection...)
				submissions.Add(newSubmission(choice, values))
			}

		case shared.ChoiceEquipment:
			if len(choice.EquipmentSelection) > 0 {
				// A bundle is answered by naming the option taken; a category
				// choice is answered by naming the items.
				values := choice.EquipmentSelection
				if choice.OptionID != "" {
					values = []shared.SelectionID{choice.OptionID}
				}
				submissions.Add(newSubmission(choice, values))
			}

		case shared.ChoiceLanguages:
			if len(choice.LanguageSelection) > 0 {
				values := make([]shared.SelectionID, 0, len(choice.LanguageSelection))
				values = append(values, choice.LanguageSelection...)
				submissions.Add(newSubmission(choice, values))
			}

		case shared.ChoiceCantrips, shared.ChoiceSpells:
			if len(choice.SpellSelection) > 0 {
				values := make([]shared.SelectionID, 0, len(choice.SpellSelection))
				values = append(values, choice.SpellSelection...)
				submissions.Add(newSubmission(choice, values))
			}

		case shared.ChoiceToolProficiency:
			if len(choice.ToolSelection) > 0 {
				values := make([]shared.SelectionID, len(choice.ToolSelection))
				for i, tool := range choice.ToolSelection {
					values[i] = shared.SelectionID(tool)
				}
				submissions.Add(newSubmission(choice, values))
			}

		case shared.ChoiceFightingStyle:
			if choice.FightingStyleSelection != nil {
				submissions.Add(newSubmission(choice, []shared.SelectionID{*choice.FightingStyleSelection}))
			}

		case shared.ChoiceExpertise:
			if len(choice.ExpertiseSelection) > 0 {
				values := make([]shared.SelectionID, len(choice.ExpertiseSelection))
				copy(values, choice.ExpertiseSelection)
				submissions.Add(newSubmission(choice, values))
			}
		}
	}

	return submissions
}

// newSubmission carries a choice's identity onto its submission unchanged, so
// the validator matches on the id the requirement published.
func newSubmission(choice ChoiceData, values []shared.SelectionID) Submission {
	return Submission{
		Category: choice.Category,
		Source:   choice.Source,
		ChoiceID: choice.ChoiceID,
		OptionID: choice.OptionID,
		Values:   values,
	}
}
