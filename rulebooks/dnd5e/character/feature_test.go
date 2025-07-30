package character

import (
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/class"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/constants"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/race"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/shared"
)

type FeatureTestSuite struct {
	suite.Suite
}

func (s *FeatureTestSuite) TestFighterFeatures() {
	// Create fighter class data with features
	fighterClass := &class.Data{
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
				{
					ID:          "fighting_style",
					Name:        "Fighting Style",
					Level:       1,
					Description: "Adopt a particular style of fighting",
					Choice: &class.ChoiceData{
						ID:     "fighting_style_choice",
						Type:   "fighting_style",
						Choose: 1,
						From:   []string{"archery", "defense", "dueling", "great_weapon_fighting"},
					},
				},
				{
					ID:          "second_wind",
					Name:        "Second Wind",
					Level:       1,
					Description: "Regain hit points as a bonus action",
				},
			},
			2: {
				{
					ID:          "action_surge",
					Name:        "Action Surge",
					Level:       2,
					Description: "Take an additional action",
				},
			},
		},
	}

	// Create human race data
	humanRace := &race.Data{
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

	// Create background
	soldierBackground := &shared.Background{
		ID:                 "soldier",
		Name:               "Soldier",
		SkillProficiencies: []constants.Skill{constants.SkillAthletics, constants.SkillIntimidation},
		Languages:          []constants.Language{constants.LanguageDwarvish},
		ToolProficiencies:  []string{"Gaming set"},
	}

	// Test creating level 1 fighter
	s.Run("Level1Fighter", func() {
		// Create draft with fighting style choice
		draft := &Draft{
			ID:       "test-fighter-1",
			PlayerID: "player-123",
			Name:     "Test Fighter",
			RaceChoice: RaceChoice{
				RaceID: constants.RaceHuman,
			},
			ClassChoice: ClassChoice{
				ClassID: constants.ClassFighter,
			},
			BackgroundChoice: constants.BackgroundSoldier,
			AbilityScoreChoice: shared.AbilityScores{
				constants.STR: 16,
				constants.DEX: 14,
				constants.CON: 15,
				constants.INT: 10,
				constants.WIS: 12,
				constants.CHA: 8,
			},
			Choices: []ChoiceData{
				{
					Category:  shared.ChoiceSkills,
					Source:    shared.SourceClass,
					ChoiceID:  "fighter_skill_proficiencies",
					Selection: []constants.Skill{constants.SkillPerception, constants.SkillSurvival},
				},
				{
					Category:  shared.ChoiceFightingStyle,
					Source:    shared.SourceClass,
					ChoiceID:  "fighter_fighting_style",
					Selection: "defense",
				},
			},
			Progress: DraftProgress{
				flags: ProgressName | ProgressRace | ProgressClass | ProgressBackground | ProgressAbilityScores,
			},
		}

		// Convert to character
		character, err := draft.ToCharacter(humanRace, fighterClass, soldierBackground)
		s.Require().NoError(err)
		s.Assert().NotNil(character)

		// Check basic fighter features
		s.Assert().Equal(constants.ClassFighter, character.classID)
		s.Assert().Equal(10, character.hitDice)

		// Check features - should have fighting_style and second_wind
		s.Assert().Contains(character.features, "fighting_style")
		s.Assert().Contains(character.features, "second_wind")
		s.Assert().Len(character.features, 2, "Level 1 fighter should have 2 features")

		// Check if fighting style choice is stored
		hasDefenseChoice := false
		for _, choice := range character.choices {
			if choice.Category == shared.ChoiceFightingStyle && choice.Selection == "defense" {
				hasDefenseChoice = true
				s.Assert().Equal(shared.SourceClass, choice.Source, "Fighting style should come from class")
				break
			}
		}
		s.Assert().True(hasDefenseChoice, "Fighting style choice should be stored")
	})

	// Test creating level 2 fighter
	s.Run("Level2Fighter", func() {
		// Create level 2 character data
		charData := Data{
			ID:           "test-fighter-2",
			PlayerID:     "player-123",
			Name:         "Level 2 Fighter",
			Level:        2, // Level 2
			RaceID:       constants.RaceHuman,
			ClassID:      constants.ClassFighter,
			BackgroundID: constants.BackgroundSoldier,
			AbilityScores: shared.AbilityScores{
				constants.STR: 17, // With racial bonus
				constants.DEX: 15,
				constants.CON: 16,
				constants.INT: 11,
				constants.WIS: 13,
				constants.CHA: 9,
			},
			MaxHitPoints: 20, // 10 + 3 (con) at level 1, +6 +3 at level 2
			HitPoints:    20,
			Speed:        30,
			Size:         "Medium",
			Skills: map[constants.Skill]shared.ProficiencyLevel{
				constants.SkillPerception:   shared.Proficient,
				constants.SkillSurvival:     shared.Proficient,
				constants.SkillAthletics:    shared.Proficient,
				constants.SkillIntimidation: shared.Proficient,
			},
			SavingThrows: map[constants.Ability]shared.ProficiencyLevel{
				constants.STR: shared.Proficient,
				constants.CON: shared.Proficient,
			},
			Languages: []string{"Common", "dwarvish"},
			Proficiencies: shared.Proficiencies{
				Armor:   fighterClass.ArmorProficiencies,
				Weapons: fighterClass.WeaponProficiencies,
				Tools:   soldierBackground.ToolProficiencies,
			},
			Choices: []ChoiceData{
				{
					Category:  "fighting_style",
					Source:    "class",
					Selection: "defense",
				},
			},
		}

		// Load character from data
		character, err := LoadCharacterFromData(charData, humanRace, fighterClass, soldierBackground)
		s.Require().NoError(err)
		s.Assert().NotNil(character)

		// Check level 2
		s.Assert().Equal(2, character.level)

		// Check features - should have all level 1 and level 2 features
		s.Assert().Contains(character.features, "fighting_style")
		s.Assert().Contains(character.features, "second_wind")
		s.Assert().Contains(character.features, "action_surge")
		s.Assert().Len(character.features, 3, "Level 2 fighter should have 3 features")
	})

	// Test that all classes and races preserve their choices
	s.Run("AllClassChoicesPreserved", func() {
		// Create a wizard for variety
		wizardClass := &class.Data{
			ID:                    "wizard",
			Name:                  "Wizard",
			HitDice:               6,
			SavingThrows:          []constants.Ability{constants.INT, constants.WIS},
			SkillProficiencyCount: 2,
			SkillOptions: []constants.Skill{
				constants.SkillArcana, constants.SkillHistory, constants.SkillInsight,
				constants.SkillInvestigation, constants.SkillMedicine, constants.SkillReligion,
			},
			ArmorProficiencies:  []string{},
			WeaponProficiencies: []string{"Dagger", "Dart", "Sling", "Quarterstaff", "Light crossbow"},
			Features: map[int][]class.FeatureData{
				1: {
					{
						ID:          "spellcasting",
						Name:        "Spellcasting",
						Level:       1,
						Description: "Cast wizard spells",
					},
					{
						ID:          "arcane_recovery",
						Name:        "Arcane Recovery",
						Level:       1,
						Description: "Recover spell slots on short rest",
					},
				},
			},
			Spellcasting: &class.SpellcastingData{
				Ability:       constants.INT,
				RitualCasting: true,
				SpellsKnown: map[int]int{
					1: 6, // 6 spells in spellbook at level 1
				},
				CantripsKnown: map[int]int{
					1: 3,
				},
			},
		}

		// Create elf race with subrace
		elfRace := &race.Data{
			ID:    "elf",
			Name:  "Elf",
			Size:  "Medium",
			Speed: 30,
			AbilityScoreIncreases: map[constants.Ability]int{
				constants.DEX: 2,
			},
			Languages: []constants.Language{constants.LanguageCommon, constants.LanguageElvish},
			Subraces: []race.SubraceData{
				{
					ID:   "high-elf",
					Name: "High Elf",
					AbilityScoreIncreases: map[constants.Ability]int{
						constants.INT: 1,
					},
				},
			},
		}

		// Create sage background
		sageBackground := &shared.Background{
			ID:                 "sage",
			Name:               "Sage",
			SkillProficiencies: []constants.Skill{constants.SkillArcana, constants.SkillHistory},
			Languages:          []constants.Language{constants.LanguageCelestial, constants.LanguageDraconic},
		}

		// Create wizard draft
		wizardDraft := &Draft{
			ID:       "test-wizard",
			PlayerID: "player-456",
			Name:     "Test Wizard",
			RaceChoice: RaceChoice{
				RaceID:    constants.RaceElf,
				SubraceID: constants.SubraceHighElf,
			},
			ClassChoice: ClassChoice{
				ClassID: constants.ClassWizard,
			},
			BackgroundChoice: constants.BackgroundSage,
			AbilityScoreChoice: shared.AbilityScores{
				constants.STR: 8,
				constants.DEX: 14,
				constants.CON: 13,
				constants.INT: 15,
				constants.WIS: 12,
				constants.CHA: 10,
			},
			Choices: []ChoiceData{
				{
					Category:  shared.ChoiceSkills,
					Source:    shared.SourceClass,
					ChoiceID:  "wizard_skill_proficiencies",
					Selection: []constants.Skill{constants.SkillInvestigation, constants.SkillInsight},
				},
				{
					Category:  shared.ChoiceCantrips,
					Source:    shared.SourceClass,
					ChoiceID:  "wizard_cantrips",
					Selection: []string{"fire_bolt", "mage_hand", "prestidigitation"},
				},
				{
					Category: shared.ChoiceSpells,
					Source:   shared.SourceClass,
					ChoiceID: "wizard_spells_known",
					Selection: []string{
						"shield", "magic_missile", "detect_magic",
						"identify", "sleep", "charm_person",
					},
				},
			},
			Progress: DraftProgress{
				flags: ProgressName | ProgressRace | ProgressClass | ProgressBackground | ProgressAbilityScores,
			},
		}

		// Convert to character
		character, err := wizardDraft.ToCharacter(elfRace, wizardClass, sageBackground)
		s.Require().NoError(err)
		s.Assert().NotNil(character)

		// Check wizard features
		s.Assert().Contains(character.features, "spellcasting")
		s.Assert().Contains(character.features, "arcane_recovery")

		// Check if spell choices are preserved with correct sources
		hasCantripChoice := false
		hasSpellChoice := false
		for _, choice := range character.choices {
			if choice.Category == shared.ChoiceCantrips {
				hasCantripChoice = true
				cantrips, ok := choice.Selection.([]string)
				s.Assert().True(ok)
				s.Assert().Len(cantrips, 3)
				s.Assert().Equal(shared.SourceClass, choice.Source, "Cantrips should come from class")
			}
			if choice.Category == shared.ChoiceSpells {
				hasSpellChoice = true
				spells, ok := choice.Selection.([]string)
				s.Assert().True(ok)
				s.Assert().Len(spells, 6)
				s.Assert().Equal(shared.SourceClass, choice.Source, "Spells should come from class")
			}
		}
		s.Assert().True(hasCantripChoice, "Cantrip choices should be stored")
		s.Assert().True(hasSpellChoice, "Spell choices should be stored")

		// Check ability scores with racial bonuses
		s.Assert().Equal(8, character.abilityScores[constants.STR])
		s.Assert().Equal(16, character.abilityScores[constants.DEX]) // 14 + 2 (elf)
		s.Assert().Equal(13, character.abilityScores[constants.CON])
		s.Assert().Equal(16, character.abilityScores[constants.INT]) // 15 + 1 (high elf)
		s.Assert().Equal(12, character.abilityScores[constants.WIS])
		s.Assert().Equal(10, character.abilityScores[constants.CHA])
	})
}

func TestFeatureTestSuite(t *testing.T) {
	suite.Run(t, new(FeatureTestSuite))
}
