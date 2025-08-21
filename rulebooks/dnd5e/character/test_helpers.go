package character

import (
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/abilities"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/backgrounds"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/class"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/classes"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/languages"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/race"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/races"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/shared"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/skills"
)

// createTestRaceData creates standard test race data for tests
func createTestRaceData() *race.Data {
	return &race.Data{
		ID:    races.Human,
		Name:  "Human",
		Size:  "Medium",
		Speed: 30,
		AbilityScoreIncreases: map[abilities.Ability]int{
			abilities.STR: 1,
			abilities.DEX: 1,
			abilities.CON: 1,
			abilities.INT: 1,
			abilities.WIS: 1,
			abilities.CHA: 1,
		},
		Languages: []languages.Language{languages.Common},
	}
}

// createTestClassData creates standard test class data for tests
func createTestClassData() *class.Data {
	return &class.Data{
		ID:                    classes.Fighter,
		Name:                  "Fighter",
		HitDice:               10,
		SavingThrows:          []abilities.Ability{abilities.STR, abilities.CON},
		SkillProficiencyCount: 2,
		SkillOptions: []skills.Skill{
			skills.Acrobatics, skills.AnimalHandling, skills.Athletics, skills.History,
			skills.Insight, skills.Intimidation, skills.Perception, skills.Survival,
		},
		ArmorProficiencies:  []string{"Light", "Medium", "Heavy", "Shield"},
		WeaponProficiencies: []string{"Simple", "Martial"},
		Features: map[int][]class.FeatureData{
			1: {
				{ID: "fighting-style", Name: "Fighting Style", Level: 1},
				{ID: "second-wind", Name: "Second Wind", Level: 1},
			},
		},
	}
}

// createTestBackgroundData creates standard test background data for tests
func createTestBackgroundData() *shared.Background {
	return &shared.Background{
		ID:                 backgrounds.Soldier,
		Name:               "Soldier",
		SkillProficiencies: []skills.Skill{skills.Athletics, skills.Intimidation},
		Languages:          []languages.Language{languages.Dwarvish},
		ToolProficiencies:  []string{"Gaming set", "Land vehicles"},
	}
}
