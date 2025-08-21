package character

import (
	"testing"

	"github.com/stretchr/testify/suite"

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

type FeatureTestSuite struct {
	suite.Suite
}

func (s *FeatureTestSuite) TestFighterFeatures() {
	// Create fighter class data with features
	fighterClass := &class.Data{
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

	// Create background
	soldierBackground := &shared.Background{
		ID:                 "soldier",
		Name:               "Soldier",
		SkillProficiencies: []skills.Skill{skills.Athletics, skills.Intimidation},
		Languages:          []languages.Language{languages.Dwarvish},
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
				RaceID: races.Human,
			},
			ClassChoice: ClassChoice{
				ClassID: classes.Fighter,
			},
			BackgroundChoice: backgrounds.Soldier,
			AbilityScoreChoice: shared.AbilityScores{
				abilities.STR: 16,
				abilities.DEX: 14,
				abilities.CON: 15,
				abilities.INT: 10,
				abilities.WIS: 12,
				abilities.CHA: 8,
			},
			Choices: []ChoiceData{
				{
					Category:       shared.ChoiceSkills,
					Source:         shared.SourceClass,
					ChoiceID:       "fighter_skill_proficiencies",
					SkillSelection: []skills.Skill{skills.Perception, skills.Survival},
				},
				{
					Category:               shared.ChoiceFightingStyle,
					Source:                 shared.SourceClass,
					ChoiceID:               "fighter_fighting_style",
					FightingStyleSelection: func(s string) *string { return &s }("defense"),
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
		s.Assert().Equal(classes.Fighter, character.classID)
		s.Assert().Equal(10, character.hitDice)

		// Check features - should have fighting_style and second_wind
		s.Assert().True(character.HasFeatureID("fighting_style"), "Should have fighting_style feature")
		s.Assert().True(character.HasFeatureID("second_wind"), "Should have second_wind feature")
		s.Assert().Len(character.GetFeatures(), 2, "Level 1 fighter should have 2 features")

		// Check if fighting style choice is stored
		hasDefenseChoice := false
		for _, choice := range character.choices {
			if choice.Category == shared.ChoiceFightingStyle &&
				choice.FightingStyleSelection != nil &&
				*choice.FightingStyleSelection == "defense" {
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
			RaceID:       races.Human,
			ClassID:      classes.Fighter,
			BackgroundID: backgrounds.Soldier,
			AbilityScores: shared.AbilityScores{
				abilities.STR: 17, // With racial bonus
				abilities.DEX: 15,
				abilities.CON: 16,
				abilities.INT: 11,
				abilities.WIS: 13,
				abilities.CHA: 9,
			},
			MaxHitPoints: 20, // 10 + 3 (con) at level 1, +6 +3 at level 2
			HitPoints:    20,
			Speed:        30,
			Size:         "Medium",
			Skills: map[skills.Skill]shared.ProficiencyLevel{
				skills.Perception:   shared.Proficient,
				skills.Survival:     shared.Proficient,
				skills.Athletics:    shared.Proficient,
				skills.Intimidation: shared.Proficient,
			},
			SavingThrows: map[abilities.Ability]shared.ProficiencyLevel{
				abilities.STR: shared.Proficient,
				abilities.CON: shared.Proficient,
			},
			Languages: []string{"Common", "dwarvish"},
			Proficiencies: shared.Proficiencies{
				Armor:   fighterClass.ArmorProficiencies,
				Weapons: fighterClass.WeaponProficiencies,
				Tools:   soldierBackground.ToolProficiencies,
			},
			Choices: []ChoiceData{
				{
					Category:               shared.ChoiceFightingStyle,
					Source:                 shared.SourceClass,
					FightingStyleSelection: func(s string) *string { return &s }("defense"),
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
		s.Assert().True(character.HasFeatureID("fighting_style"), "Should have fighting_style feature")
		s.Assert().True(character.HasFeatureID("second_wind"), "Should have second_wind feature")
		s.Assert().True(character.HasFeatureID("action_surge"), "Should have action_surge feature")
		s.Assert().Len(character.GetFeatures(), 3, "Level 2 fighter should have 3 features")
	})

	// Test that all classes and races preserve their choices
	s.Run("AllClassChoicesPreserved", func() {
		// Create a wizard for variety
		wizardClass := &class.Data{
			ID:                    classes.Wizard,
			Name:                  "Wizard",
			HitDice:               6,
			SavingThrows:          []abilities.Ability{abilities.INT, abilities.WIS},
			SkillProficiencyCount: 2,
			SkillOptions: []skills.Skill{
				skills.Arcana, skills.History, skills.Insight,
				skills.Investigation, skills.Medicine, skills.Religion,
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
				Ability:       abilities.INT,
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
			AbilityScoreIncreases: map[abilities.Ability]int{
				abilities.DEX: 2,
			},
			Languages: []languages.Language{languages.Common, languages.Elvish},
			Subraces: []race.SubraceData{
				{
					ID:   "high-elf",
					Name: "High Elf",
					AbilityScoreIncreases: map[abilities.Ability]int{
						abilities.INT: 1,
					},
				},
			},
		}

		// Create sage background
		sageBackground := &shared.Background{
			ID:                 "sage",
			Name:               "Sage",
			SkillProficiencies: []skills.Skill{skills.Arcana, skills.History},
			Languages:          []languages.Language{languages.Celestial, languages.Draconic},
		}

		// Create wizard draft
		wizardDraft := &Draft{
			ID:       "test-wizard",
			PlayerID: "player-456",
			Name:     "Test Wizard",
			RaceChoice: RaceChoice{
				RaceID:    races.Elf,
				SubraceID: races.HighElf,
			},
			ClassChoice: ClassChoice{
				ClassID: classes.Wizard,
			},
			BackgroundChoice: backgrounds.Sage,
			AbilityScoreChoice: shared.AbilityScores{
				abilities.STR: 8,
				abilities.DEX: 14,
				abilities.CON: 13,
				abilities.INT: 15,
				abilities.WIS: 12,
				abilities.CHA: 10,
			},
			Choices: []ChoiceData{
				{
					Category:       shared.ChoiceSkills,
					Source:         shared.SourceClass,
					ChoiceID:       "wizard_skill_proficiencies",
					SkillSelection: []skills.Skill{skills.Investigation, skills.Insight},
				},
				{
					Category:         shared.ChoiceCantrips,
					Source:           shared.SourceClass,
					ChoiceID:         "wizard_cantrips",
					CantripSelection: []string{"fire_bolt", "mage_hand", "prestidigitation"},
				},
				{
					Category: shared.ChoiceSpells,
					Source:   shared.SourceClass,
					ChoiceID: "wizard_spells_known",
					SpellSelection: []string{
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
		s.Assert().True(character.HasFeatureID("spellcasting"), "Should have spellcasting feature")
		s.Assert().True(character.HasFeatureID("arcane_recovery"), "Should have arcane_recovery feature")

		// Check if spell choices are preserved with correct sources
		hasCantripChoice := false
		hasSpellChoice := false
		for _, choice := range character.choices {
			if choice.Category == shared.ChoiceCantrips && choice.CantripSelection != nil {
				hasCantripChoice = true
				s.Assert().Len(choice.CantripSelection, 3)
				s.Assert().Equal(shared.SourceClass, choice.Source, "Cantrips should come from class")
			}
			if choice.Category == shared.ChoiceSpells && choice.SpellSelection != nil {
				hasSpellChoice = true
				s.Assert().Len(choice.SpellSelection, 6)
				s.Assert().Equal(shared.SourceClass, choice.Source, "Spells should come from class")
			}
		}
		s.Assert().True(hasCantripChoice, "Cantrip choices should be stored")
		s.Assert().True(hasSpellChoice, "Spell choices should be stored")

		// Check ability scores with racial bonuses
		s.Assert().Equal(8, character.abilityScores[abilities.STR])
		s.Assert().Equal(16, character.abilityScores[abilities.DEX]) // 14 + 2 (elf)
		s.Assert().Equal(13, character.abilityScores[abilities.CON])
		s.Assert().Equal(16, character.abilityScores[abilities.INT]) // 15 + 1 (high elf)
		s.Assert().Equal(12, character.abilityScores[abilities.WIS])
		s.Assert().Equal(10, character.abilityScores[abilities.CHA])
	})
}

func TestFeatureTestSuite(t *testing.T) {
	suite.Run(t, new(FeatureTestSuite))
}
