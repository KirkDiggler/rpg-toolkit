// Package fighter provides fighter class-specific choices and features
package fighter

import (
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/choices"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/skills"
)

// SkillChoices returns the fighter's skill proficiency choices
func SkillChoices() choices.Choice {
	return choices.Choice{
		ID:          "fighter-skills",
		Category:    choices.CategorySkill,
		Description: "Choose two skills from Acrobatics, Animal Handling, Athletics, History, Insight, Intimidation, Perception, and Survival",
		Choose:      2,
		Source:      choices.SourceClass,
		Options: []choices.Option{
			choices.SkillListOption{
				Skills: []skills.Skill{
					skills.Acrobatics,
					skills.AnimalHandling,
					skills.Athletics,
					skills.History,
					skills.Insight,
					skills.Intimidation,
					skills.Perception,
					skills.Survival,
				},
			},
		},
	}
}

// EquipmentChoices returns the fighter's starting equipment choices
// For now, we'll return empty until weapons are implemented
func EquipmentChoices() []choices.Choice {
	// TODO: Add equipment choices once weapons/armor are implemented
	return []choices.Choice{}
}

// AllChoices returns all fighter choices
func AllChoices() []choices.Choice {
	choices := []choices.Choice{
		SkillChoices(),
	}
	choices = append(choices, EquipmentChoices()...)
	return choices
}