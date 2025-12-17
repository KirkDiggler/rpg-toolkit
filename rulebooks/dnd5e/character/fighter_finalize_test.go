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
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/conditions"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/fightingstyles"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/languages"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/races"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/shared"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/skills"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/weapons"
)

// FighterFinalizeSuite tests Fighter-specific finalization
type FighterFinalizeSuite struct {
	suite.Suite
	eventBus events.EventBus
}

// SetupTest runs before each test
func (s *FighterFinalizeSuite) SetupTest() {
	s.eventBus = events.NewEventBus()
}

// TestFighterWithArcheryFightingStyle tests that a Fighter's fighting style
// is applied as a condition when the character is finalized
func (s *FighterFinalizeSuite) TestFighterWithArcheryFightingStyle() {
	// Create a new draft
	draft, err := NewDraft(&DraftConfig{
		ID:       "test-fighter-draft",
		PlayerID: "player-1",
	})
	s.Require().NoError(err)
	s.Require().NotNil(draft)

	// Set name
	err = draft.SetName(&SetNameInput{
		Name: "Legolas the Archer",
	})
	s.Require().NoError(err)

	// Set race (Human)
	err = draft.SetRace(&SetRaceInput{
		RaceID: races.Human,
		Choices: RaceChoices{
			Languages: []languages.Language{languages.Elvish},
		},
	})
	s.Require().NoError(err)

	// Set class (Fighter with Archery fighting style)
	err = draft.SetClass(&SetClassInput{
		ClassID: classes.Fighter,
		Choices: ClassChoices{
			Skills: []skills.Skill{
				skills.Athletics,
				skills.Perception,
			},
			Equipment: []EquipmentChoiceSelection{
				{ChoiceID: choices.FighterArmor, OptionID: choices.FighterArmorLeather},
				{
					ChoiceID:           choices.FighterWeaponsPrimary,
					OptionID:           choices.FighterWeaponMartialShield,
					CategorySelections: []shared.EquipmentID{weapons.Longsword},
				},
				{ChoiceID: choices.FighterWeaponsSecondary, OptionID: choices.FighterRangedCrossbow},
				{ChoiceID: choices.FighterPack, OptionID: choices.FighterPackExplorer},
			},
			FightingStyle: fightingstyles.Archery,
		},
	})
	s.Require().NoError(err)

	// Verify fighting style choice was recorded
	found := false
	for _, choice := range draft.Choices() {
		if choice.Category == shared.ChoiceFightingStyle {
			found = true
			s.Require().NotNil(choice.FightingStyleSelection)
			s.Equal(fightingstyles.Archery, *choice.FightingStyleSelection)
		}
	}
	s.True(found, "Fighting style choice should be recorded in draft")

	// Set background
	err = draft.SetBackground(&SetBackgroundInput{
		BackgroundID: backgrounds.Soldier,
	})
	s.Require().NoError(err)

	// Set ability scores
	err = draft.SetAbilityScores(&SetAbilityScoresInput{
		Scores: shared.AbilityScores{
			abilities.STR: 14,
			abilities.DEX: 16,
			abilities.CON: 14,
			abilities.INT: 10,
			abilities.WIS: 12,
			abilities.CHA: 8,
		},
		Method: "standard-array",
	})
	s.Require().NoError(err)

	// Finalize to character
	char, err := draft.ToCharacter(context.Background(), "fighter-1", s.eventBus)
	s.Require().NoError(err, "Failed to convert draft to character")
	s.Require().NotNil(char)

	// Verify character was created correctly
	s.Equal("Legolas the Archer", char.GetName())
	data := char.ToData()
	s.Equal(classes.Fighter, data.ClassID)

	// Verify fighting style condition was applied
	charConditions := char.GetConditions()
	s.Require().Len(charConditions, 1, "Character should have exactly 1 condition (Archery)")

	// Verify it's a FightingStyleCondition with Archery
	fsCondition, ok := charConditions[0].(*conditions.FightingStyleCondition)
	s.Require().True(ok, "Condition should be FightingStyleCondition")
	s.Equal(fightingstyles.Archery, fsCondition.Style)
	s.Equal("fighter-1", fsCondition.CharacterID)
}

// TestFighterWithGWFFightingStyle tests Great Weapon Fighting style application
func (s *FighterFinalizeSuite) TestFighterWithGWFFightingStyle() {
	// Create a new draft
	draft, err := NewDraft(&DraftConfig{
		ID:       "test-gwf-fighter",
		PlayerID: "player-2",
	})
	s.Require().NoError(err)

	// Set name
	err = draft.SetName(&SetNameInput{Name: "Guts the Swordsman"})
	s.Require().NoError(err)

	// Set race
	err = draft.SetRace(&SetRaceInput{
		RaceID: races.Human,
		Choices: RaceChoices{
			Languages: []languages.Language{languages.Dwarvish},
		},
	})
	s.Require().NoError(err)

	// Set class with Great Weapon Fighting
	err = draft.SetClass(&SetClassInput{
		ClassID: classes.Fighter,
		Choices: ClassChoices{
			Skills: []skills.Skill{
				skills.Athletics,
				skills.Intimidation,
			},
			Equipment: []EquipmentChoiceSelection{
				{ChoiceID: choices.FighterArmor, OptionID: choices.FighterArmorChainMail},
				{
					ChoiceID:           choices.FighterWeaponsPrimary,
					OptionID:           choices.FighterWeaponTwoMartial,
					CategorySelections: []shared.EquipmentID{weapons.Greatsword, weapons.Battleaxe},
				},
				{ChoiceID: choices.FighterWeaponsSecondary, OptionID: choices.FighterRangedCrossbow},
				{ChoiceID: choices.FighterPack, OptionID: choices.FighterPackDungeoneer},
			},
			FightingStyle: fightingstyles.GreatWeaponFighting,
		},
	})
	s.Require().NoError(err)

	// Set background
	err = draft.SetBackground(&SetBackgroundInput{
		BackgroundID: backgrounds.Soldier,
	})
	s.Require().NoError(err)

	// Set ability scores
	err = draft.SetAbilityScores(&SetAbilityScoresInput{
		Scores: shared.AbilityScores{
			abilities.STR: 16,
			abilities.DEX: 12,
			abilities.CON: 15,
			abilities.INT: 8,
			abilities.WIS: 10,
			abilities.CHA: 14,
		},
		Method: "standard-array",
	})
	s.Require().NoError(err)

	// Finalize to character
	char, err := draft.ToCharacter(context.Background(), "gwf-fighter", s.eventBus)
	s.Require().NoError(err)
	s.Require().NotNil(char)

	// Verify fighting style condition was applied
	charConditions := char.GetConditions()
	s.Require().Len(charConditions, 1, "Character should have exactly 1 condition (GWF)")

	// Verify it's GWF
	fsCondition, ok := charConditions[0].(*conditions.FightingStyleCondition)
	s.Require().True(ok, "Condition should be FightingStyleCondition")
	s.Equal(fightingstyles.GreatWeaponFighting, fsCondition.Style)
}

// TestFighterWithoutFightingStyle tests that a Fighter cannot be created without
// selecting a fighting style (required at Level 1 per D&D 5e PHB)
func (s *FighterFinalizeSuite) TestFighterWithoutFightingStyle() {
	// Create a new draft
	draft, err := NewDraft(&DraftConfig{
		ID:       "test-no-style-fighter",
		PlayerID: "player-3",
	})
	s.Require().NoError(err)

	// Set name
	err = draft.SetName(&SetNameInput{Name: "Generic Fighter"})
	s.Require().NoError(err)

	// Set race
	err = draft.SetRace(&SetRaceInput{
		RaceID: races.Human,
		Choices: RaceChoices{
			Languages: []languages.Language{languages.Dwarvish},
		},
	})
	s.Require().NoError(err)

	// Set class WITHOUT fighting style
	err = draft.SetClass(&SetClassInput{
		ClassID: classes.Fighter,
		Choices: ClassChoices{
			Skills: []skills.Skill{
				skills.Athletics,
				skills.Acrobatics,
			},
			Equipment: []EquipmentChoiceSelection{
				{ChoiceID: choices.FighterArmor, OptionID: choices.FighterArmorChainMail},
				{
					ChoiceID:           choices.FighterWeaponsPrimary,
					OptionID:           choices.FighterWeaponMartialShield,
					CategorySelections: []shared.EquipmentID{weapons.Rapier},
				},
				{ChoiceID: choices.FighterWeaponsSecondary, OptionID: choices.FighterRangedCrossbow},
				{ChoiceID: choices.FighterPack, OptionID: choices.FighterPackDungeoneer},
			},
			// No FightingStyle set - this should cause validation to fail
		},
	})
	s.Require().NoError(err)

	// Set background
	err = draft.SetBackground(&SetBackgroundInput{
		BackgroundID: backgrounds.Soldier,
	})
	s.Require().NoError(err)

	// Set ability scores
	err = draft.SetAbilityScores(&SetAbilityScoresInput{
		Scores: shared.AbilityScores{
			abilities.STR: 15,
			abilities.DEX: 14,
			abilities.CON: 13,
			abilities.INT: 12,
			abilities.WIS: 10,
			abilities.CHA: 8,
		},
		Method: "standard-array",
	})
	s.Require().NoError(err)

	// ValidateChoices should fail because no fighting style was selected
	err = draft.ValidateChoices()
	s.Require().Error(err, "ValidateChoices should fail when Fighter has no fighting style")
	s.Contains(err.Error(), "fighting style", "Error should mention fighting style")

	// ToCharacter should also fail
	char, err := draft.ToCharacter(context.Background(), "no-style-fighter", s.eventBus)
	s.Require().Error(err, "ToCharacter should fail when Fighter has no fighting style")
	s.Nil(char, "Character should not be created without fighting style")
}

// TestFighterWithInvalidStyleFails tests that choosing an invalid
// fighting style fails during validation
func (s *FighterFinalizeSuite) TestFighterWithInvalidStyleFails() {
	// Create a new draft
	draft, err := NewDraft(&DraftConfig{
		ID:       "test-invalid-style-fighter",
		PlayerID: "player-4",
	})
	s.Require().NoError(err)

	// Set name
	err = draft.SetName(&SetNameInput{Name: "Invalid Style Fighter"})
	s.Require().NoError(err)

	// Set race
	err = draft.SetRace(&SetRaceInput{
		RaceID: races.Human,
		Choices: RaceChoices{
			Languages: []languages.Language{languages.Common},
		},
	})
	s.Require().NoError(err)

	// Set class with a fake fighting style that doesn't exist
	err = draft.SetClass(&SetClassInput{
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
			FightingStyle: "mariner", // Fake style that doesn't exist in valid options
		},
	})
	s.Require().NoError(err)

	// Set background
	err = draft.SetBackground(&SetBackgroundInput{
		BackgroundID: backgrounds.Soldier,
	})
	s.Require().NoError(err)

	// Set ability scores
	err = draft.SetAbilityScores(&SetAbilityScoresInput{
		Scores: shared.AbilityScores{
			abilities.STR: 15,
			abilities.DEX: 14,
			abilities.CON: 13,
			abilities.INT: 12,
			abilities.WIS: 10,
			abilities.CHA: 8,
		},
		Method: "standard-array",
	})
	s.Require().NoError(err)

	// Validation should fail because "mariner" is not a valid fighting style option
	err = draft.ValidateChoices()
	s.Require().Error(err, "ValidateChoices should fail for invalid fighting style")
	s.Contains(err.Error(), "mariner", "Error should mention the invalid fighting style")

	// ToCharacter should also fail
	char, err := draft.ToCharacter(context.Background(), "fake-style-fighter", s.eventBus)
	s.Require().Error(err, "ToCharacter should fail for invalid fighting style")
	s.Nil(char, "Character should not be created when validation fails")
}

func TestFighterFinalizeSuite(t *testing.T) {
	suite.Run(t, new(FighterFinalizeSuite))
}
