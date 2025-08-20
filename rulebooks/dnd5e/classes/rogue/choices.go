// Package rogue provides rogue class-specific choices and features
package rogue

import (
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/choices"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/skills"
)

// SkillChoices returns the rogue's skill proficiency choices
func SkillChoices() choices.Choice {
	return choices.Choice{
		ID:          "rogue-skills",
		Category:    choices.CategorySkill,
		Description: "Choose four skills from Acrobatics, Athletics, Deception, Insight, Intimidation, Investigation, Perception, Performance, Persuasion, Sleight of Hand, and Stealth",
		Choose:      4, // Rogues get more skills!
		Source:      choices.SourceClass,
		Options: []choices.Option{
			choices.SkillListOption{
				Skills: []skills.Skill{
					skills.Acrobatics,
					skills.Athletics,
					skills.Deception,
					skills.Insight,
					skills.Intimidation,
					skills.Investigation,
					skills.Perception,
					skills.Performance,
					skills.Persuasion,
					skills.SleightOfHand,
					skills.Stealth,
				},
			},
		},
	}
}

// AllChoices returns all rogue choices
func AllChoices() []choices.Choice {
	return []choices.Choice{
		SkillChoices(),
	}
}