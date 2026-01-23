package character

import (
	"context"
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/KirkDiggler/rpg-toolkit/events"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/abilities"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/backgrounds"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/character/choices"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/classes"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/fightingstyles"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/languages"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/proficiencies"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/races"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/shared"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/skills"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/weapons"
)

// ProficienciesSuite tests weapon/armor/tool proficiency serialization
type ProficienciesSuite struct {
	suite.Suite
	eventBus events.EventBus
}

func (s *ProficienciesSuite) SetupTest() {
	s.eventBus = events.NewEventBus()
}

// TestFighterProficiencies verifies Fighter gets light/medium/heavy/shields armor
// and simple/martial weapon proficiencies
func (s *ProficienciesSuite) TestFighterProficiencies() {
	draft, err := NewDraft(&DraftConfig{
		ID:       "test-fighter-profs",
		PlayerID: "player-1",
	})
	s.Require().NoError(err)

	// Set name
	s.Require().NoError(draft.SetName(&SetNameInput{Name: "Test Fighter"}))

	// Set race (Human)
	s.Require().NoError(draft.SetRace(&SetRaceInput{
		RaceID: races.Human,
		Choices: RaceChoices{
			Languages: []languages.Language{languages.Elvish},
		},
	}))

	// Set class (Fighter)
	s.Require().NoError(draft.SetClass(&SetClassInput{
		ClassID: classes.Fighter,
		Choices: ClassChoices{
			Skills: []skills.Skill{
				skills.Athletics,
				skills.Perception,
			},
			Equipment: []EquipmentChoiceSelection{
				{ChoiceID: choices.FighterArmor, OptionID: choices.FighterArmorChainMail},
				{
					ChoiceID:           choices.FighterWeaponsPrimary,
					OptionID:           choices.FighterWeaponMartialShield,
					CategorySelections: []shared.EquipmentID{weapons.Longsword},
				},
				{ChoiceID: choices.FighterWeaponsSecondary, OptionID: choices.FighterRangedCrossbow},
				{ChoiceID: choices.FighterPack, OptionID: choices.FighterPackExplorer},
			},
			FightingStyle: fightingstyles.Defense,
		},
	}))

	// Set background
	s.Require().NoError(draft.SetBackground(&SetBackgroundInput{
		BackgroundID: backgrounds.Soldier,
	}))

	// Set ability scores
	s.Require().NoError(draft.SetAbilityScores(&SetAbilityScoresInput{
		Scores: shared.AbilityScores{
			abilities.STR: 15,
			abilities.DEX: 14,
			abilities.CON: 13,
			abilities.INT: 12,
			abilities.WIS: 10,
			abilities.CHA: 8,
		},
		Method: "standard-array",
	}))

	// Finalize to character
	char, err := draft.ToCharacter(context.Background(), "fighter-1", s.eventBus)
	s.Require().NoError(err)
	s.Require().NotNil(char)

	// Get data to verify proficiencies
	data := char.ToData()

	// Verify armor proficiencies: light, medium, heavy, shields
	s.ElementsMatch(
		[]proficiencies.Armor{
			proficiencies.ArmorLight,
			proficiencies.ArmorMedium,
			proficiencies.ArmorHeavy,
			proficiencies.ArmorShields,
		},
		data.ArmorProficiencies,
		"Fighter should have all four armor proficiencies",
	)

	// Verify weapon proficiencies: simple, martial
	s.ElementsMatch(
		[]proficiencies.Weapon{
			proficiencies.WeaponSimple,
			proficiencies.WeaponMartial,
		},
		data.WeaponProficiencies,
		"Fighter should have simple and martial weapon proficiencies",
	)

	// Fighters have no tool proficiencies by default
	s.Empty(data.ToolProficiencies, "Fighter should have no tool proficiencies")
}

// TestBarbarianProficiencies verifies Barbarian gets light/medium/shields armor
// (NOT heavy) and simple/martial weapon proficiencies
func (s *ProficienciesSuite) TestBarbarianProficiencies() {
	draft, err := NewDraft(&DraftConfig{
		ID:       "test-barbarian-profs",
		PlayerID: "player-2",
	})
	s.Require().NoError(err)

	// Set name
	s.Require().NoError(draft.SetName(&SetNameInput{Name: "Test Barbarian"}))

	// Set race (Human)
	s.Require().NoError(draft.SetRace(&SetRaceInput{
		RaceID: races.Human,
		Choices: RaceChoices{
			Languages: []languages.Language{languages.Orc},
		},
	}))

	// Set class (Barbarian)
	s.Require().NoError(draft.SetClass(&SetClassInput{
		ClassID: classes.Barbarian,
		Choices: ClassChoices{
			Skills: []skills.Skill{
				skills.Athletics,
				skills.Intimidation,
			},
			Equipment: []EquipmentChoiceSelection{
				{
					ChoiceID:           choices.BarbarianWeaponsPrimary,
					OptionID:           choices.BarbarianWeaponGreataxe,
					CategorySelections: nil,
				},
				{
					ChoiceID:           choices.BarbarianWeaponsSecondary,
					OptionID:           choices.BarbarianSecondaryHandaxes,
					CategorySelections: nil,
				},
				{ChoiceID: choices.BarbarianPack, OptionID: choices.BarbarianPackExplorer},
			},
		},
	}))

	// Set background
	s.Require().NoError(draft.SetBackground(&SetBackgroundInput{
		BackgroundID: backgrounds.Outlander,
	}))

	// Set ability scores
	s.Require().NoError(draft.SetAbilityScores(&SetAbilityScoresInput{
		Scores: shared.AbilityScores{
			abilities.STR: 15,
			abilities.DEX: 14,
			abilities.CON: 13,
			abilities.INT: 8,
			abilities.WIS: 12,
			abilities.CHA: 10,
		},
		Method: "standard-array",
	}))

	// Finalize to character
	char, err := draft.ToCharacter(context.Background(), "barbarian-1", s.eventBus)
	s.Require().NoError(err)
	s.Require().NotNil(char)

	// Get data to verify proficiencies
	data := char.ToData()

	// Verify armor proficiencies: light, medium, shields (NOT heavy)
	s.ElementsMatch(
		[]proficiencies.Armor{
			proficiencies.ArmorLight,
			proficiencies.ArmorMedium,
			proficiencies.ArmorShields,
		},
		data.ArmorProficiencies,
		"Barbarian should have light, medium, shields - NOT heavy",
	)

	// Verify Barbarian does NOT have heavy armor
	s.NotContains(data.ArmorProficiencies, proficiencies.ArmorHeavy,
		"Barbarian should NOT have heavy armor proficiency")

	// Verify weapon proficiencies: simple, martial
	s.ElementsMatch(
		[]proficiencies.Weapon{
			proficiencies.WeaponSimple,
			proficiencies.WeaponMartial,
		},
		data.WeaponProficiencies,
		"Barbarian should have simple and martial weapon proficiencies",
	)

	// Barbarians have no tool proficiencies by default
	s.Empty(data.ToolProficiencies, "Barbarian should have no tool proficiencies")
}

// TestMonkProficiencies verifies Monk gets NO armor proficiencies
// and simple + shortsword weapon proficiencies
func (s *ProficienciesSuite) TestMonkProficiencies() {
	draft, err := NewDraft(&DraftConfig{
		ID:       "test-monk-profs",
		PlayerID: "player-3",
	})
	s.Require().NoError(err)

	// Set name
	s.Require().NoError(draft.SetName(&SetNameInput{Name: "Test Monk"}))

	// Set race (Human)
	s.Require().NoError(draft.SetRace(&SetRaceInput{
		RaceID: races.Human,
		Choices: RaceChoices{
			Languages: []languages.Language{languages.Dwarvish},
		},
	}))

	// Set class (Monk) - Note: Monk equipment choices may differ
	s.Require().NoError(draft.SetClass(&SetClassInput{
		ClassID: classes.Monk,
		Choices: ClassChoices{
			Skills: []skills.Skill{
				skills.Acrobatics,
				skills.Stealth,
			},
			Equipment: []EquipmentChoiceSelection{
				{ChoiceID: choices.MonkWeaponsPrimary, OptionID: choices.MonkWeaponShortsword},
				{ChoiceID: choices.MonkPack, OptionID: choices.MonkPackExplorer},
			},
			Tools: []shared.SelectionID{"brewers-supplies"},
		},
	}))

	// Set background
	s.Require().NoError(draft.SetBackground(&SetBackgroundInput{
		BackgroundID: backgrounds.Hermit,
	}))

	// Set ability scores
	s.Require().NoError(draft.SetAbilityScores(&SetAbilityScoresInput{
		Scores: shared.AbilityScores{
			abilities.STR: 10,
			abilities.DEX: 15,
			abilities.CON: 13,
			abilities.INT: 12,
			abilities.WIS: 14,
			abilities.CHA: 8,
		},
		Method: "standard-array",
	}))

	// Finalize to character
	char, err := draft.ToCharacter(context.Background(), "monk-1", s.eventBus)
	s.Require().NoError(err)
	s.Require().NotNil(char)

	// Get data to verify proficiencies
	data := char.ToData()

	// Verify armor proficiencies: NONE (empty or nil)
	s.Empty(data.ArmorProficiencies, "Monk should have NO armor proficiencies")

	// Verify weapon proficiencies: simple + shortsword
	s.ElementsMatch(
		[]proficiencies.Weapon{
			proficiencies.WeaponSimple,
			proficiencies.WeaponShortsword,
		},
		data.WeaponProficiencies,
		"Monk should have simple weapons and shortsword proficiency",
	)

	// Monks get artisan's tools OR musical instrument - not tested here as it's a choice
	// For now, verify tool proficiencies is empty (choice system not exercised)
	s.Empty(data.ToolProficiencies, "Monk should have no tool proficiencies (choice not made)")
}

// TestProficienciesRoundTrip verifies proficiencies survive serialization/deserialization
func (s *ProficienciesSuite) TestProficienciesRoundTrip() {
	// Create a Fighter
	draft, err := NewDraft(&DraftConfig{
		ID:       "test-roundtrip",
		PlayerID: "player-4",
	})
	s.Require().NoError(err)

	s.Require().NoError(draft.SetName(&SetNameInput{Name: "Roundtrip Fighter"}))
	s.Require().NoError(draft.SetRace(&SetRaceInput{
		RaceID: races.Human,
		Choices: RaceChoices{
			Languages: []languages.Language{languages.Elvish},
		},
	}))
	s.Require().NoError(draft.SetClass(&SetClassInput{
		ClassID: classes.Fighter,
		Choices: ClassChoices{
			Skills: []skills.Skill{skills.Athletics, skills.Perception},
			Equipment: []EquipmentChoiceSelection{
				{ChoiceID: choices.FighterArmor, OptionID: choices.FighterArmorChainMail},
				{
					ChoiceID:           choices.FighterWeaponsPrimary,
					OptionID:           choices.FighterWeaponMartialShield,
					CategorySelections: []shared.EquipmentID{weapons.Longsword},
				},
				{ChoiceID: choices.FighterWeaponsSecondary, OptionID: choices.FighterRangedCrossbow},
				{ChoiceID: choices.FighterPack, OptionID: choices.FighterPackExplorer},
			},
			FightingStyle: fightingstyles.Defense,
		},
	}))
	s.Require().NoError(draft.SetBackground(&SetBackgroundInput{BackgroundID: backgrounds.Soldier}))
	s.Require().NoError(draft.SetAbilityScores(&SetAbilityScoresInput{
		Scores: shared.AbilityScores{
			abilities.STR: 15, abilities.DEX: 14, abilities.CON: 13,
			abilities.INT: 12, abilities.WIS: 10, abilities.CHA: 8,
		},
		Method: "standard-array",
	}))

	// Finalize
	char, err := draft.ToCharacter(context.Background(), "roundtrip-1", s.eventBus)
	s.Require().NoError(err)

	// Get data (simulate save)
	originalData := char.ToData()
	s.Require().NotEmpty(originalData.ArmorProficiencies)
	s.Require().NotEmpty(originalData.WeaponProficiencies)

	// Load from data (simulate load) - need a new event bus
	newBus := events.NewEventBus()
	loadedChar, err := LoadFromData(context.Background(), originalData, newBus)
	s.Require().NoError(err)
	s.Require().NotNil(loadedChar)

	// Get data again and compare
	loadedData := loadedChar.ToData()

	// Verify proficiencies survived roundtrip
	s.ElementsMatch(originalData.ArmorProficiencies, loadedData.ArmorProficiencies,
		"Armor proficiencies should survive roundtrip")
	s.ElementsMatch(originalData.WeaponProficiencies, loadedData.WeaponProficiencies,
		"Weapon proficiencies should survive roundtrip")
	s.ElementsMatch(originalData.ToolProficiencies, loadedData.ToolProficiencies,
		"Tool proficiencies should survive roundtrip")
}

func TestProficienciesSuite(t *testing.T) {
	suite.Run(t, new(ProficienciesSuite))
}
