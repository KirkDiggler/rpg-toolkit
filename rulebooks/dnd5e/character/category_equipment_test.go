package character_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/KirkDiggler/rpg-toolkit/events"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/abilities"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/backgrounds"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/character"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/character/choices"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/classes"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/languages"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/races"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/shared"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/skills"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/weapons"
)

// CategoryBasedEquipmentTestSuite tests the fix for issue #346
type CategoryBasedEquipmentTestSuite struct {
	suite.Suite
	ctx   context.Context
	bus   events.EventBus
	draft *character.Draft
}

func (s *CategoryBasedEquipmentTestSuite) SetupTest() {
	s.ctx = context.Background()
	s.bus = events.NewEventBus()
	s.draft = character.LoadDraftFromData(&character.DraftData{
		ID:       "draft-category-test",
		PlayerID: "player-test",
	})

	// Set up minimal character
	err := s.draft.SetName(&character.SetNameInput{Name: "Test Barbarian"})
	s.Require().NoError(err)

	err = s.draft.SetAbilityScores(&character.SetAbilityScoresInput{
		Scores: shared.AbilityScores{
			abilities.STR: 16,
			abilities.DEX: 14,
			abilities.CON: 15,
			abilities.INT: 10,
			abilities.WIS: 12,
			abilities.CHA: 8,
		},
		Method: "standard",
	})
	s.Require().NoError(err)

	err = s.draft.SetRace(&character.SetRaceInput{
		RaceID:    races.Human,
		SubraceID: "",
		Choices: character.RaceChoices{
			Languages: []languages.Language{languages.Orc},
		},
	})
	s.Require().NoError(err)

	err = s.draft.SetBackground(&character.SetBackgroundInput{
		BackgroundID: backgrounds.Outlander,
	})
	s.Require().NoError(err)
}

// TestBarbarianSecondaryWeaponCategoryChoice tests the bug scenario from issue #346
// Barbarian secondary weapon option B has CategoryChoices for "any simple weapon"
func (s *CategoryBasedEquipmentTestSuite) TestBarbarianSecondaryWeaponCategoryChoice() {
	// Set class with secondary weapon option B (any simple weapon)
	err := s.draft.SetClass(&character.SetClassInput{
		ClassID: classes.Barbarian,
		Choices: character.ClassChoices{
			Skills: []skills.Skill{skills.Athletics, skills.Intimidation},
			Equipment: []character.EquipmentChoiceSelection{
				{
					ChoiceID: choices.BarbarianWeaponsPrimary,
					OptionID: choices.BarbarianWeaponGreataxe,
				},
				{
					ChoiceID:           choices.BarbarianWeaponsSecondary,
					OptionID:           choices.BarbarianSecondarySimple,    // Option B with CategoryChoices
					CategorySelections: []shared.EquipmentID{weapons.Spear}, // Player chooses a spear
				},
				{
					ChoiceID: choices.BarbarianPack,
					OptionID: choices.BarbarianPackExplorer,
				},
			},
		},
	})
	s.Require().NoError(err, "Setting Barbarian class with category weapon choice should succeed")

	// Verify choices were recorded
	draftChoices := s.draft.Choices()
	s.Require().NotEmpty(draftChoices, "Should have recorded choices")

	// Find the secondary weapon choice
	var secondaryWeaponChoice *choices.ChoiceData
	for _, choice := range draftChoices {
		if choice.ChoiceID == choices.BarbarianWeaponsSecondary {
			secondaryWeaponChoice = &choice
			break
		}
	}

	s.Require().NotNil(secondaryWeaponChoice, "Should have recorded secondary weapon choice")
	s.Assert().Equal(shared.ChoiceEquipment, secondaryWeaponChoice.Category)
	s.Assert().Equal(shared.SourceClass, secondaryWeaponChoice.Source)
	s.Assert().Equal(choices.BarbarianSecondarySimple, secondaryWeaponChoice.OptionID)
	s.Require().NotEmpty(secondaryWeaponChoice.EquipmentSelection, "Should have equipment selection")
	s.Assert().Contains(secondaryWeaponChoice.EquipmentSelection, weapons.Spear,
		"Should have the selected spear in equipment selection")
}

func TestCategoryBasedEquipmentTestSuite(t *testing.T) {
	suite.Run(t, new(CategoryBasedEquipmentTestSuite))
}
