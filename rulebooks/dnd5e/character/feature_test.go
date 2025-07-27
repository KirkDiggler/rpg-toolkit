package character

import (
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/class"
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
		SavingThrows:          []string{shared.AbilityStrength, shared.AbilityConstitution},
		SkillProficiencyCount: 2,
		SkillOptions: []string{
			"Acrobatics", "Animal Handling", "Athletics", "History",
			"Insight", "Intimidation", "Perception", "Survival",
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
		AbilityScoreIncreases: map[string]int{
			shared.AbilityStrength:     1,
			shared.AbilityDexterity:    1,
			shared.AbilityConstitution: 1,
			shared.AbilityIntelligence: 1,
			shared.AbilityWisdom:       1,
			shared.AbilityCharisma:     1,
		},
		Languages: []string{"Common"},
	}

	// Create background
	soldierBackground := &shared.Background{
		ID:                 "soldier",
		Name:               "Soldier",
		SkillProficiencies: []string{"Athletics", "Intimidation"},
		Languages:          []string{"Dwarvish"},
		ToolProficiencies:  []string{"Gaming set"},
	}

	// Test creating level 1 fighter
	s.Run("Level1Fighter", func() {
		// Create draft with fighting style choice
		draft := &Draft{
			ID:       "test-fighter-1",
			PlayerID: "player-123",
			Name:     "Test Fighter",
			Choices: map[shared.ChoiceCategory]any{
				shared.ChoiceName: "Test Fighter",
				shared.ChoiceRace: RaceChoice{
					RaceID: "human",
				},
				shared.ChoiceClass:      "fighter",
				shared.ChoiceBackground: "soldier",
				shared.ChoiceAbilityScores: shared.AbilityScores{
					Strength:     16,
					Dexterity:    14,
					Constitution: 15,
					Intelligence: 10,
					Wisdom:       12,
					Charisma:     8,
				},
				shared.ChoiceSkills:        []string{"Perception", "Survival"},
				shared.ChoiceFightingStyle: "defense", // Fighter-specific choice
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
		s.Assert().Equal("fighter", character.classID)
		s.Assert().Equal(10, character.hitDice)

		// Check features - should have fighting_style and second_wind
		s.Assert().Contains(character.features, "fighting_style")
		s.Assert().Contains(character.features, "second_wind")
		s.Assert().Len(character.features, 2, "Level 1 fighter should have 2 features")

		// Check if fighting style choice is stored
		hasDefenseChoice := false
		for _, choice := range character.choices {
			if choice.Category == string(shared.ChoiceFightingStyle) && choice.Selection == "defense" {
				hasDefenseChoice = true
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
			RaceID:       "human",
			ClassID:      "fighter",
			BackgroundID: "soldier",
			AbilityScores: shared.AbilityScores{
				Strength:     17, // With racial bonus
				Dexterity:    15,
				Constitution: 16,
				Intelligence: 11,
				Wisdom:       13,
				Charisma:     9,
			},
			MaxHitPoints: 20, // 10 + 3 (con) at level 1, +6 +3 at level 2
			HitPoints:    20,
			Speed:        30,
			Size:         "Medium",
			Skills: map[string]int{
				"Perception":   int(shared.Proficient),
				"Survival":     int(shared.Proficient),
				"Athletics":    int(shared.Proficient),
				"Intimidation": int(shared.Proficient),
			},
			SavingThrows: map[string]int{
				shared.AbilityStrength:     int(shared.Proficient),
				shared.AbilityConstitution: int(shared.Proficient),
			},
			Languages: []string{"Common", "Dwarvish"},
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
			SavingThrows:          []string{shared.AbilityIntelligence, shared.AbilityWisdom},
			SkillProficiencyCount: 2,
			SkillOptions: []string{
				"Arcana", "History", "Insight", "Investigation", "Medicine", "Religion",
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
				Ability:       shared.AbilityIntelligence,
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
			AbilityScoreIncreases: map[string]int{
				shared.AbilityDexterity: 2,
			},
			Languages: []string{"Common", "Elvish"},
			Subraces: []race.SubraceData{
				{
					ID:   "high-elf",
					Name: "High Elf",
					AbilityScoreIncreases: map[string]int{
						shared.AbilityIntelligence: 1,
					},
				},
			},
		}

		// Create sage background
		sageBackground := &shared.Background{
			ID:                 "sage",
			Name:               "Sage",
			SkillProficiencies: []string{"Arcana", "History"},
			Languages:          []string{"Celestial", "Draconic"},
		}

		// Create wizard draft
		wizardDraft := &Draft{
			ID:       "test-wizard",
			PlayerID: "player-456",
			Name:     "Test Wizard",
			Choices: map[shared.ChoiceCategory]any{
				shared.ChoiceName: "Test Wizard",
				shared.ChoiceRace: RaceChoice{
					RaceID:    "elf",
					SubraceID: "high-elf",
				},
				shared.ChoiceClass:      "wizard",
				shared.ChoiceBackground: "sage",
				shared.ChoiceAbilityScores: shared.AbilityScores{
					Strength:     8,
					Dexterity:    14,
					Constitution: 13,
					Intelligence: 15,
					Wisdom:       12,
					Charisma:     10,
				},
				shared.ChoiceSkills:   []string{"Investigation", "Insight"},
				shared.ChoiceCantrips: []string{"fire_bolt", "mage_hand", "prestidigitation"},
				shared.ChoiceSpells: []string{
					"shield", "magic_missile", "detect_magic",
					"identify", "sleep", "charm_person",
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

		// Check if spell choices are preserved
		hasCantripChoice := false
		hasSpellChoice := false
		for _, choice := range character.choices {
			if choice.Category == string(shared.ChoiceCantrips) {
				hasCantripChoice = true
				cantrips, ok := choice.Selection.([]string)
				s.Assert().True(ok)
				s.Assert().Len(cantrips, 3)
			}
			if choice.Category == string(shared.ChoiceSpells) {
				hasSpellChoice = true
				spells, ok := choice.Selection.([]string)
				s.Assert().True(ok)
				s.Assert().Len(spells, 6)
			}
		}
		s.Assert().True(hasCantripChoice, "Cantrip choices should be stored")
		s.Assert().True(hasSpellChoice, "Spell choices should be stored")

		// Check ability scores with racial bonuses
		s.Assert().Equal(8, character.abilityScores.Strength)
		s.Assert().Equal(16, character.abilityScores.Dexterity) // 14 + 2 (elf)
		s.Assert().Equal(13, character.abilityScores.Constitution)
		s.Assert().Equal(16, character.abilityScores.Intelligence) // 15 + 1 (high elf)
		s.Assert().Equal(12, character.abilityScores.Wisdom)
		s.Assert().Equal(10, character.abilityScores.Charisma)
	})
}

func TestFeatureTestSuite(t *testing.T) {
	suite.Run(t, new(FeatureTestSuite))
}
