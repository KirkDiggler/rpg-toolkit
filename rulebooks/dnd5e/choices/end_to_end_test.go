package choices_test

import (
	"context"
	"testing"

	"github.com/KirkDiggler/rpg-toolkit/rpgerr"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/choices"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/classes/fighter"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/languages"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/weapons"
	"github.com/stretchr/testify/suite"
)

// Test constants for repeated strings
const (
	fighterClass = "fighter"
)

// CharacterCreationSession represents what the game server would track
type CharacterCreationSession struct {
	PlayerID    string
	CharacterID string
	Class       string
	Race        string
	Background  string

	// Track all choices and selections
	PendingChoices   map[choices.ChoiceID]choices.Choice
	CompletedChoices map[choices.ChoiceID][]string

	// Final character data
	Skills    []string
	Languages []string
	Equipment []string
	Weapons   []string
}

// EndToEndTestSuite tests the complete character creation flow
type EndToEndTestSuite struct {
	suite.Suite

	ctx     context.Context
	session *CharacterCreationSession
}

func (s *EndToEndTestSuite) SetupTest() {
	// Initialize context with tracing
	s.ctx = context.Background()
	s.ctx = rpgerr.WithMetadata(s.ctx,
		rpgerr.Meta("service", "character-creation-e2e"),
		rpgerr.Meta("test", "end-to-end"),
	)

	// Initialize session
	s.session = &CharacterCreationSession{
		PlayerID:         "test-player",
		CharacterID:      "test-character",
		PendingChoices:   make(map[choices.ChoiceID]choices.Choice),
		CompletedChoices: make(map[choices.ChoiceID][]string),
		Skills:           []string{},
		Languages:        []string{},
		Equipment:        []string{},
		Weapons:          []string{},
	}
}

func (s *EndToEndTestSuite) SetupSubTest() {
	// Add session context for each subtest
	s.ctx = rpgerr.WithMetadata(s.ctx,
		rpgerr.Meta("player_id", s.session.PlayerID),
		rpgerr.Meta("character_id", s.session.CharacterID),
	)
}

// TestCompleteHumanFighterCreation tests creating a Human Fighter from start to finish
func (s *EndToEndTestSuite) TestCompleteHumanFighterCreation() {
	s.Run("Step1_SelectRaceClassBackground", func() {
		// Player selects Human Fighter with Soldier background
		s.session.Race = "human"
		s.session.Class = fighterClass
		s.session.Background = "soldier"

		s.ctx = rpgerr.WithMetadata(s.ctx,
			rpgerr.Meta("race", s.session.Race),
			rpgerr.Meta("class", s.session.Class),
			rpgerr.Meta("background", s.session.Background),
		)

		// Game server loads all choices for this combination
		s.loadChoicesForCharacter()

		// Should have multiple pending choices
		s.Assert().NotEmpty(s.session.PendingChoices)
		s.Assert().Contains(s.session.PendingChoices, choices.FighterSkills)
		s.Assert().Contains(s.session.PendingChoices, choices.FighterEquipment1)
	})

	s.Run("Step2_SelectFighterSkills", func() {
		skillChoice, exists := s.session.PendingChoices[choices.FighterSkills]
		s.Require().True(exists)

		// Present available skills to player
		availableSkills, err := choices.GetAvailableOptionsCtx(s.ctx, skillChoice)
		s.Require().NoError(err)
		s.T().Logf("Available fighter skills: %v", availableSkills)

		// Player selects athletics and intimidation
		selections := []string{"athletics", "intimidation"}

		// Validate selection
		err = choices.ValidateSelectionCtx(s.ctx, skillChoice, selections)
		s.Require().NoError(err)

		// Mark as completed
		s.completeChoice(choices.FighterSkills, selections)
		s.session.Skills = append(s.session.Skills, selections...)
	})

	s.Run("Step3_SelectStartingEquipment", func() {
		// Equipment choice 1: armor
		armorChoice, exists := s.session.PendingChoices[choices.FighterEquipment1]
		s.Require().True(exists)

		// Player chooses chain mail
		armorSelection := []string{"chain-mail"}
		err := choices.ValidateSelectionCtx(s.ctx, armorChoice, armorSelection)
		s.Require().NoError(err)

		s.completeChoice(choices.FighterEquipment1, armorSelection)
		s.session.Equipment = append(s.session.Equipment, "chain-mail")

		// Equipment choice 2: weapons
		weaponChoice, exists := s.session.PendingChoices[choices.FighterEquipment2]
		s.Require().True(exists)

		// Player chooses martial weapon and shield
		weaponSelection := []string{"martial-weapon-and-shield"}
		err = choices.ValidateSelectionCtx(s.ctx, weaponChoice, weaponSelection)
		s.Require().NoError(err)

		s.completeChoice(choices.FighterEquipment2, weaponSelection)

		// This choice leads to another choice - picking the specific martial weapon
		s.addMartialWeaponChoice()
	})

	s.Run("Step4_SelectSpecificWeapon", func() {
		weaponChoice, exists := s.session.PendingChoices[choices.ChoiceID("martial-weapon-selection")]
		s.Require().True(exists)

		// Get available martial weapons
		availableWeapons, err := choices.GetAvailableOptionsCtx(s.ctx, weaponChoice)
		s.Require().NoError(err)
		s.T().Logf("Available martial weapons: %v", availableWeapons)

		// Player selects longsword
		weaponSelection := []string{"longsword"}
		err = choices.ValidateSelectionCtx(s.ctx, weaponChoice, weaponSelection)
		s.Require().NoError(err)

		s.completeChoice(choices.ChoiceID("martial-weapon-selection"), weaponSelection)
		s.session.Weapons = append(s.session.Weapons, "longsword")
		s.session.Equipment = append(s.session.Equipment, "shield")
	})

	s.Run("Step5_SelectLanguages", func() {
		// Human gets one extra language
		humanLangChoice := s.createHumanLanguageChoice()
		s.session.PendingChoices[choices.ChoiceID("human-language")] = humanLangChoice

		// Player selects Elvish
		langSelection := []string{"elvish"}
		err := choices.ValidateSelectionCtx(s.ctx, humanLangChoice, langSelection)
		s.Require().NoError(err)

		s.completeChoice(choices.ChoiceID("human-language"), langSelection)
		s.session.Languages = append(s.session.Languages, "common") // All humans know Common
		s.session.Languages = append(s.session.Languages, langSelection...)
	})

	s.Run("Step6_VerifyCompletion", func() {
		// Check all required choices are completed
		s.Assert().Empty(s.session.PendingChoices, "All choices should be completed")

		// Verify final character state
		s.Assert().Equal("human", s.session.Race)
		s.Assert().Equal(fighterClass, s.session.Class)
		s.Assert().Equal("soldier", s.session.Background)

		// Skills
		s.Assert().Contains(s.session.Skills, "athletics")
		s.Assert().Contains(s.session.Skills, "intimidation")

		// Languages
		s.Assert().Contains(s.session.Languages, "common")
		s.Assert().Contains(s.session.Languages, "elvish")

		// Equipment
		s.Assert().Contains(s.session.Equipment, "chain-mail")
		s.Assert().Contains(s.session.Equipment, "shield")
		s.Assert().Contains(s.session.Weapons, "longsword")

		s.T().Log("Character creation completed successfully!")
		s.T().Logf("Final character: %+v", s.session)
	})
}

// TestValidationScenarios tests various validation edge cases
func (s *EndToEndTestSuite) TestValidationScenarios() {
	s.Run("CannotSkipRequiredChoices", func() {
		// Set up a fighter character
		s.session.Class = fighterClass
		s.loadChoicesForCharacter()

		// Character should NOT be complete without making choices
		isComplete := s.validateCharacterCompletion()
		s.Assert().False(isComplete, "Character should not be complete without making choices")

		// Should tell us what's missing
		missing := s.getMissingChoices()
		s.Assert().Contains(missing, choices.FighterSkills)
		s.Assert().Contains(missing, choices.FighterEquipment1)
		s.Assert().Contains(missing, choices.FighterEquipment2)
	})

	s.Run("CannotMakeSameChoiceTwice", func() {
		skillChoice := fighter.SkillChoices()

		// Make initial selection
		firstSelection := []string{"athletics", "perception"}
		err := choices.ValidateSelectionCtx(s.ctx, skillChoice, firstSelection)
		s.Require().NoError(err)
		s.completeChoice(choices.FighterSkills, firstSelection)

		// Try to change selection (should be prevented by game server)
		s.Assert().NotContains(s.session.PendingChoices, choices.FighterSkills)
		s.Assert().Contains(s.session.CompletedChoices, choices.FighterSkills)
	})

	s.Run("HandlesInvalidWeaponCategory", func() {
		// Create a choice for martial weapons
		martialChoice := fighter.MartialWeaponChoice()

		// Try to select a simple weapon
		err := choices.ValidateSelectionCtx(s.ctx, martialChoice, []string{"club"})
		s.Require().Error(err)

		// Error should have helpful context
		meta := rpgerr.GetMeta(err)
		s.Assert().NotNil(meta["selection"])
		s.Assert().Equal("club", meta["selection"])
	})

	s.Run("ValidatesWeaponExists", func() {
		martialChoice := fighter.MartialWeaponChoice()

		// Try to select non-existent weapon
		err := choices.ValidateSelectionCtx(s.ctx, martialChoice, []string{"lightsaber"})
		s.Require().Error(err)
		s.Assert().Contains(err.Error(), "selection not found")
	})
}

// TestChoiceInterdependencies tests choices that depend on other choices
func (s *EndToEndTestSuite) TestChoiceInterdependencies() {
	s.Run("BundleChoiceCreatesFollowupChoices", func() {
		// Equipment choice with bundle
		equipChoice := fighter.StartingEquipmentChoice2()

		// Select "two martial weapons" option
		err := choices.ValidateSelectionCtx(s.ctx, equipChoice, []string{"two-martial-weapons"})
		s.Require().NoError(err)

		// This should create TWO follow-up choices for specific weapons
		// Game server would need to handle this
		s.T().Log("Bundle selection would create follow-up choices for:")
		s.T().Log("- First martial weapon selection")
		s.T().Log("- Second martial weapon selection")
	})

	s.Run("ConditionalChoicesBasedOnLevel", func() {
		// At higher levels, fighters get additional choices
		// This is where the game server would check level
		level := 3

		if level >= 3 {
			// Fighter gets a martial archetype choice at level 3
			s.T().Log("Level 3 fighter would get martial archetype choice")
		}
	})
}

// Helper methods for the test suite

func (s *EndToEndTestSuite) loadChoicesForCharacter() {
	// Load all choices based on race/class/background
	if s.session.Class == fighterClass {
		s.session.PendingChoices[choices.FighterSkills] = fighter.SkillChoices()
		s.session.PendingChoices[choices.FighterEquipment1] = fighter.StartingEquipmentChoice1()
		s.session.PendingChoices[choices.FighterEquipment2] = fighter.StartingEquipmentChoice2()
	}
}

func (s *EndToEndTestSuite) completeChoice(choiceID choices.ChoiceID, selections []string) {
	s.session.CompletedChoices[choiceID] = selections
	delete(s.session.PendingChoices, choiceID)
}

func (s *EndToEndTestSuite) addMartialWeaponChoice() {
	martialWeaponChoice := choices.Choice{
		ID:          choices.ChoiceID("martial-weapon-selection"),
		Category:    choices.CategoryEquipment,
		Description: "Choose your martial weapon",
		Choose:      1,
		Source:      choices.SourceClass,
		Options: []choices.Option{
			choices.WeaponCategoryOption{
				Category: weapons.CategoryMartialMelee,
			},
			choices.WeaponCategoryOption{
				Category: weapons.CategoryMartialRanged,
			},
		},
	}
	s.session.PendingChoices[martialWeaponChoice.ID] = martialWeaponChoice
}

func (s *EndToEndTestSuite) createHumanLanguageChoice() choices.Choice {
	return choices.Choice{
		ID:          choices.ChoiceID("human-language"),
		Category:    choices.CategoryLanguage,
		Description: "Humans learn one additional language",
		Choose:      1,
		Source:      choices.SourceRace,
		Options: []choices.Option{
			choices.LanguageListOption{
				Languages: languages.StandardLanguages()[1:], // Exclude Common
				AllowAny:  false,
			},
		},
	}
}

func (s *EndToEndTestSuite) validateCharacterCompletion() bool {
	// Check if all required choices are completed
	return len(s.session.PendingChoices) == 0
}

func (s *EndToEndTestSuite) getMissingChoices() []choices.ChoiceID {
	missing := []choices.ChoiceID{}
	for id := range s.session.PendingChoices {
		missing = append(missing, id)
	}
	return missing
}

// TestEndToEndTestSuite runs the suite
func TestEndToEndTestSuite(t *testing.T) {
	suite.Run(t, new(EndToEndTestSuite))
}
