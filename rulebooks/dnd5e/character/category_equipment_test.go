package character_test

import (
	"context"
	"fmt"
	"sync"
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

func (s *CategoryBasedEquipmentTestSuite) TestMonkCategoryChoiceRejectsEverySpecialWeaponViaSetClass() {
	requirements := choices.GetClassRequirements(classes.Monk)
	_, monkSimple := monkWeaponSimpleOptionFromRequirements(requirements)
	advertised := make(map[shared.EquipmentID]struct{}, len(monkSimple.CategoryChoices[0].Options))
	for _, item := range monkSimple.CategoryChoices[0].Options {
		advertised[item.ID] = struct{}{}
	}

	for specialID := range weapons.SpecialWeapons {
		s.Run(string(specialID), func() {
			_, offered := advertised[specialID]
			s.False(offered, "special weapons must never be advertised as category choices")

			draft := character.LoadDraftFromData(&character.DraftData{
				ID:       "draft-monk-special-" + string(specialID),
				PlayerID: "player-test",
			})
			err := draft.SetClass(monkClassInput(specialID))

			s.Require().Error(err)
			s.ErrorContains(err, "Invalid equipment choice '"+string(specialID)+"'")
			for _, choice := range draft.Choices() {
				s.NotContains(choice.EquipmentSelection, specialID,
					"rejected category equipment must not be recorded in the draft")
			}
		})
	}
}

func (s *CategoryBasedEquipmentTestSuite) TestMonkCategoryChoiceAppliesExactlyAdvertisedWeaponOptions() {
	requirements := choices.GetClassRequirements(classes.Monk)
	_, monkSimple := monkWeaponSimpleOptionFromRequirements(requirements)
	advertised := make(map[shared.EquipmentID]struct{}, len(monkSimple.CategoryChoices[0].Options))
	for _, item := range monkSimple.CategoryChoices[0].Options {
		advertised[item.ID] = struct{}{}
	}

	for weaponID := range weapons.All {
		s.Run(string(weaponID), func() {
			_, offered := advertised[weaponID]
			draft := character.LoadDraftFromData(&character.DraftData{
				ID:       "draft-monk-" + string(weaponID),
				PlayerID: "player-test",
			})
			err := draft.SetClass(monkClassInput(weaponID))

			if offered {
				s.Require().NoError(err, "advertised equipment must be accepted through Draft.SetClass")
				return
			}
			s.Require().Error(err, "unadvertised equipment must be rejected through Draft.SetClass")
		})
	}
}

func (s *CategoryBasedEquipmentTestSuite) TestMonkCategoryChoiceAppliesAdvertisedOptionsConcurrently() {
	_, monkSimple := monkWeaponSimpleOptionFromRequirements(choices.GetClassRequirements(classes.Monk))
	options := monkSimple.CategoryChoices[0].Options
	const workers = 64

	errors := make(chan string, workers)
	var waitGroup sync.WaitGroup
	for worker := range workers {
		waitGroup.Add(1)
		go func() {
			defer waitGroup.Done()
			item := options[worker%len(options)]
			draft := character.LoadDraftFromData(&character.DraftData{
				ID:       fmt.Sprintf("draft-monk-concurrent-%d", worker),
				PlayerID: "player-test",
			})
			if err := draft.SetClass(monkClassInput(item.ID)); err != nil {
				errors <- fmt.Sprintf("%s: %v", item.ID, err)
			}
		}()
	}
	waitGroup.Wait()
	close(errors)
	for err := range errors {
		s.Fail(err)
	}
}

func monkClassInput(weaponID shared.EquipmentID) *character.SetClassInput {
	return &character.SetClassInput{
		ClassID: classes.Monk,
		Choices: character.ClassChoices{
			Skills: []skills.Skill{skills.Acrobatics, skills.Athletics},
			Equipment: []character.EquipmentChoiceSelection{
				{
					ChoiceID:           choices.MonkWeaponsPrimary,
					OptionID:           choices.MonkWeaponSimple,
					CategorySelections: []shared.EquipmentID{weaponID},
				},
				{
					ChoiceID: choices.MonkPack,
					OptionID: choices.MonkPackExplorer,
				},
			},
		},
	}
}

func monkWeaponSimpleOptionFromRequirements(requirements *choices.Requirements) (*choices.EquipmentRequirement, *choices.EquipmentOption) {
	for _, requirement := range requirements.Equipment {
		if requirement.ID != choices.MonkWeaponsPrimary {
			continue
		}
		for i := range requirement.Options {
			if requirement.Options[i].ID == choices.MonkWeaponSimple {
				return requirement, &requirement.Options[i]
			}
		}
	}
	panic("MonkWeaponSimple should exist")
}

func TestCategoryBasedEquipmentTestSuite(t *testing.T) {
	suite.Run(t, new(CategoryBasedEquipmentTestSuite))
}
