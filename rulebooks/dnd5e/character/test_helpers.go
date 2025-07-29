package character

import (
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/class"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/constants"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/race"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/shared"
)

// createTestRaceData creates standard test race data for tests
func createTestRaceData() *race.Data {
	return &race.Data{
		ID:    "human",
		Name:  "Human",
		Size:  "Medium",
		Speed: 30,
		AbilityScoreIncreases: map[constants.Ability]int{
			constants.STR: 1,
			constants.DEX: 1,
			constants.CON: 1,
			constants.INT: 1,
			constants.WIS: 1,
			constants.CHA: 1,
		},
		Languages: []constants.Language{constants.LanguageCommon},
	}
}

// createTestClassData creates standard test class data for tests
func createTestClassData() *class.Data {
	return &class.Data{
		ID:                    "fighter",
		Name:                  "Fighter",
		HitDice:               10,
		SavingThrows:          []constants.Ability{constants.STR, constants.CON},
		SkillProficiencyCount: 2,
		SkillOptions: []constants.Skill{
			constants.SkillAcrobatics, constants.SkillAnimalHandling, constants.SkillAthletics, constants.SkillHistory,
			constants.SkillInsight, constants.SkillIntimidation, constants.SkillPerception, constants.SkillSurvival,
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
		ID:                 "soldier",
		Name:               "Soldier",
		SkillProficiencies: []constants.Skill{constants.SkillAthletics, constants.SkillIntimidation},
		Languages:          []constants.Language{constants.LanguageDwarvish},
		ToolProficiencies:  []string{"Gaming set", "Land vehicles"},
	}
}
