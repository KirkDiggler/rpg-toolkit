package choices_test

import (
	"context"
	"testing"

	"github.com/KirkDiggler/rpg-toolkit/rpgerr"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/choices"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/classes/fighter"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/classes/rogue"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/damage"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/languages"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/weapons"
	"github.com/stretchr/testify/suite"
)

// GameServerSimulationSuite simulates how a game server would use the choice system
type GameServerSimulationSuite struct {
	suite.Suite

	// Context that would be passed through the game server
	ctx context.Context

	// Player data
	playerID    string
	characterID string

	// Character creation state
	selectedClass string
	selectedRace  string
	level         int

	// Choices made by the player
	skillSelections     []string
	languageSelections  []string
	equipmentSelections map[string][]string // choice ID -> selections

	// Available choices based on class/race
	availableChoices []choices.Choice
}

// SetupTest runs before each test method
func (s *GameServerSimulationSuite) SetupTest() {
	// Initialize context with game server metadata
	s.ctx = context.Background()
	s.ctx = rpgerr.WithMetadata(s.ctx,
		rpgerr.Meta("service", "character-creation"),
		rpgerr.Meta("version", "v1"),
	)

	// Initialize player data
	s.playerID = "player-123"
	s.characterID = "char-456"
	s.selectedClass = ""
	s.selectedRace = ""
	s.level = 1

	// Reset all selections
	s.skillSelections = nil
	s.languageSelections = nil
	s.equipmentSelections = make(map[string][]string)
	s.availableChoices = nil
}

// SetupSubTest runs before each s.Run()
func (s *GameServerSimulationSuite) SetupSubTest() {
	// Add sub-test context
	s.ctx = rpgerr.WithMetadata(s.ctx,
		rpgerr.Meta("player_id", s.playerID),
		rpgerr.Meta("character_id", s.characterID),
	)
}

// TestFighterCharacterCreation simulates creating a fighter character
func (s *GameServerSimulationSuite) TestFighterCharacterCreation() {
	s.Run("SelectClass", func() {
		// Player selects Fighter class
		s.selectedClass = "fighter"
		s.ctx = rpgerr.WithMetadata(s.ctx, rpgerr.Meta("class", s.selectedClass))

		// Game server loads fighter choices
		s.availableChoices = []choices.Choice{
			fighter.SkillChoices(),
			fighter.StartingEquipmentChoice1(),
			fighter.StartingEquipmentChoice2(),
		}

		s.Require().Len(s.availableChoices, 3)
	})

	s.Run("PresentSkillChoices", func() {
		// Find skill choice
		var skillChoice *choices.Choice
		for _, c := range s.availableChoices {
			if c.Category == choices.CategorySkill {
				skillChoice = &c
				break
			}
		}
		s.Require().NotNil(skillChoice)

		// Game server would present available options to player
		options, err := choices.GetAvailableOptionsCtx(s.ctx, *skillChoice)
		s.Require().NoError(err)

		// Fighter should have 8 skills to choose from
		s.Assert().Len(options, 8)
		s.Assert().Contains(options, "athletics")
		s.Assert().Contains(options, "perception")

		// Game server would display: "Choose 2 skills from: athletics, animal-handling, ..."
		s.Assert().Equal(2, skillChoice.Choose)
	})

	s.Run("PlayerSelectsSkills", func() {
		skillChoice := s.findChoiceByCategory(choices.CategorySkill)
		s.Require().NotNil(skillChoice)

		// Player selects athletics and perception
		s.skillSelections = []string{"athletics", "perception"}

		// Game server validates the selection
		err := choices.ValidateSelectionCtx(s.ctx, *skillChoice, s.skillSelections)
		s.Assert().NoError(err)
	})

	s.Run("PlayerSelectsInvalidSkills", func() {
		skillChoice := s.findChoiceByCategory(choices.CategorySkill)
		s.Require().NotNil(skillChoice)

		// Player tries to select sleight-of-hand (rogue skill)
		invalidSelections := []string{"athletics", "sleight-of-hand"}

		// Game server validates and gets error
		err := choices.ValidateSelectionCtx(s.ctx, *skillChoice, invalidSelections)
		s.Require().Error(err)

		// Check error has useful context
		var rpgErr *rpgerr.Error
		s.Require().ErrorAs(err, &rpgErr)
		s.Assert().Equal(rpgerr.CodeInvalidArgument, rpgErr.Code)

		// Game server can extract metadata to show helpful error
		meta := rpgerr.GetMeta(err)
		s.Assert().NotNil(meta["choice_id"])
		s.Assert().NotNil(meta["selection"])
	})

	s.Run("PlayerSelectsEquipment", func() {
		// Find equipment choices
		var equipChoice1 *choices.Choice
		for _, c := range s.availableChoices {
			if c.ID == choices.FighterEquipment1 {
				equipChoice1 = &c
				break
			}
		}
		s.Require().NotNil(equipChoice1)

		// Present options to player
		options, err := choices.GetAvailableOptionsCtx(s.ctx, *equipChoice1)
		s.Require().NoError(err)

		// Should have chain-mail or leather-armor-longbow-bundle
		s.Assert().Len(options, 2)
		s.Assert().Contains(options, "chain-mail")
		s.Assert().Contains(options, "leather-armor-longbow-bundle")

		// Player chooses chain mail
		s.equipmentSelections[string(equipChoice1.ID)] = []string{"chain-mail"}

		// Validate
		err = choices.ValidateSelectionCtx(s.ctx, *equipChoice1, s.equipmentSelections[string(equipChoice1.ID)])
		s.Assert().NoError(err)
	})

	s.Run("CompleteCharacterCreation", func() {
		// At this point, game server would:
		// 1. Verify all required choices have been made
		requiredChoices := map[choices.ChoiceID]bool{
			choices.FighterSkills:     true,
			choices.FighterEquipment1: true,
			choices.FighterEquipment2: false, // Not completed yet
		}

		for _, choice := range s.availableChoices {
			if requiredChoices[choice.ID] {
				selections, exists := s.getSelectionsForChoice(choice.ID)
				if !exists || len(selections) == 0 {
					s.T().Logf("Missing required choice: %s", choice.ID)
				}
			}
		}

		// 2. Apply selections to character
		// This would be done in rpg-api
		characterData := s.buildCharacterData()
		s.Assert().Equal("fighter", characterData["class"])
		s.Assert().Contains(characterData["skills"], "athletics")
		s.Assert().Contains(characterData["skills"], "perception")
		s.Assert().Contains(characterData["equipment"], "chain-mail")
	})
}

// TestRogueCharacterCreation simulates creating a rogue character
func (s *GameServerSimulationSuite) TestRogueCharacterCreation() {
	s.Run("RogueGetsMoreSkills", func() {
		// Select rogue class
		s.selectedClass = "rogue"
		s.ctx = rpgerr.WithMetadata(s.ctx, rpgerr.Meta("class", s.selectedClass))

		// Load rogue choices
		rogueSkillChoice := rogue.SkillChoices()

		// Rogue gets 4 skills instead of fighter's 2
		s.Assert().Equal(4, rogueSkillChoice.Choose)

		// And has different skill options
		options, err := choices.GetAvailableOptionsCtx(s.ctx, rogueSkillChoice)
		s.Require().NoError(err)

		s.Assert().Len(options, 11) // Rogues have 11 skill options
		s.Assert().Contains(options, "sleight-of-hand")
		s.Assert().Contains(options, "stealth")
		s.Assert().NotContains(options, "animal-handling")

		// Player selects 4 skills
		s.skillSelections = []string{"stealth", "sleight-of-hand", "perception", "investigation"}

		// Validate
		err = choices.ValidateSelectionCtx(s.ctx, rogueSkillChoice, s.skillSelections)
		s.Assert().NoError(err)
	})

	s.Run("RogueCannotSelectTooManySkills", func() {
		rogueSkillChoice := rogue.SkillChoices()

		// Try to select 5 skills (too many)
		tooManySkills := []string{"stealth", "sleight-of-hand", "perception", "investigation", "acrobatics"}

		err := choices.ValidateSelectionCtx(s.ctx, rogueSkillChoice, tooManySkills)
		s.Require().Error(err)

		// Check error details
		var rpgErr *rpgerr.Error
		s.Require().ErrorAs(err, &rpgErr)
		meta := rpgerr.GetMeta(err)
		s.Assert().Equal(4, meta["expected"])
		s.Assert().Equal(5, meta["got"])
	})
}

// TestWeaponChoiceFlow simulates choosing weapons
func (s *GameServerSimulationSuite) TestWeaponChoiceFlow() {
	s.Run("MartialWeaponChoice", func() {
		// Create a martial weapon choice
		weaponChoice := fighter.MartialWeaponChoice()

		// Get available weapons
		options, err := choices.GetAvailableOptionsCtx(s.ctx, weaponChoice)
		s.Require().NoError(err)

		// Should include all martial weapons we defined
		s.Assert().Contains(options, "longsword")
		s.Assert().Contains(options, "greatsword")
		s.Assert().Contains(options, "longbow")

		// But not simple weapons
		s.Assert().NotContains(options, "club")
		s.Assert().NotContains(options, "dagger")
	})

	s.Run("PlayerSelectsSpecificWeapon", func() {
		weaponChoice := fighter.MartialWeaponChoice()

		// Player selects longsword
		selection := []string{"longsword"}

		// Validate
		err := choices.ValidateSelectionCtx(s.ctx, weaponChoice, selection)
		s.Assert().NoError(err)

		// Game server can now look up weapon details
		weapon, err := weapons.GetByID("longsword")
		s.Require().NoError(err)
		s.Assert().Equal("1d8", weapon.Damage)
		s.Assert().Equal(damage.Slashing, weapon.DamageType)
		s.Assert().True(weapon.HasProperty(weapons.PropertyVersatile))
	})

	s.Run("WeaponCategoryValidation", func() {
		// Player tries to select a simple weapon for martial choice
		weaponChoice := fighter.MartialWeaponChoice()

		// Try to select club (simple weapon)
		invalidSelection := []string{"club"}

		err := choices.ValidateSelectionCtx(s.ctx, weaponChoice, invalidSelection)
		s.Require().Error(err)
		s.Assert().Contains(err.Error(), "selection not found")
	})
}

// TestLanguageChoices simulates language selection
func (s *GameServerSimulationSuite) TestLanguageChoices() {
	s.Run("StandardLanguageChoice", func() {
		// Create a choice for standard languages
		langChoice := choices.Choice{
			ID:          choices.ChoiceID("background-language"),
			Category:    choices.CategoryLanguage,
			Description: "Choose a language",
			Choose:      1,
			Source:      choices.SourceBackground,
			Options: []choices.Option{
				choices.LanguageListOption{
					Languages: languages.StandardLanguages(),
					AllowAny:  false,
				},
			},
		}

		// Get options
		options, err := choices.GetAvailableOptionsCtx(s.ctx, langChoice)
		s.Require().NoError(err)

		// Should have all standard languages
		s.Assert().Contains(options, "common")
		s.Assert().Contains(options, "elvish")
		s.Assert().Contains(options, "dwarvish")

		// But not exotic languages
		s.Assert().NotContains(options, "abyssal")
		s.Assert().NotContains(options, "celestial")
	})

	s.Run("AnyLanguageChoice", func() {
		// Some backgrounds allow ANY language
		anyLangChoice := choices.Choice{
			ID:          choices.ChoiceID("sage-language"),
			Category:    choices.CategoryLanguage,
			Description: "Choose any two languages",
			Choose:      2,
			Source:      choices.SourceBackground,
			Options: []choices.Option{
				choices.LanguageListOption{
					AllowAny: true,
				},
			},
		}

		// Get options - should include exotic
		options, err := choices.GetAvailableOptionsCtx(s.ctx, anyLangChoice)
		s.Require().NoError(err)

		// Should have both standard and exotic
		s.Assert().Contains(options, "common")
		s.Assert().Contains(options, "abyssal")
		s.Assert().Contains(options, "celestial")
		s.Assert().Contains(options, "primordial")

		// Player selects one standard and one exotic
		selections := []string{"elvish", "celestial"}

		err = choices.ValidateSelectionCtx(s.ctx, anyLangChoice, selections)
		s.Assert().NoError(err)
	})
}

// TestErrorContext verifies errors have useful context for the game server
func (s *GameServerSimulationSuite) TestErrorContext() {
	s.Run("DuplicateSelectionError", func() {
		skillChoice := fighter.SkillChoices()

		// Try to select the same skill twice
		duplicateSelections := []string{"athletics", "athletics"}

		err := choices.ValidateSelectionCtx(s.ctx, skillChoice, duplicateSelections)
		s.Require().Error(err)

		// Extract metadata for error reporting
		meta := rpgerr.GetMeta(err)
		s.Assert().Equal("fighter-skills", meta["choice_id"])
		s.Assert().Equal("skill", meta["category"])
		s.Assert().Equal("athletics", meta["selection"])

		// Game server can use this to show:
		// "Error: You selected 'athletics' twice for fighter skills"
	})

	s.Run("InvalidSelectionWithContext", func() {
		// Add more context about the current operation
		ctx := rpgerr.WithMetadata(s.ctx,
			rpgerr.Meta("operation", "character_creation"),
			rpgerr.Meta("step", "skill_selection"),
		)

		skillChoice := fighter.SkillChoices()
		invalidSelections := []string{"athletics", "arcana"} // arcana not available to fighter

		err := choices.ValidateSelectionCtx(ctx, skillChoice, invalidSelections)
		s.Require().Error(err)

		// All context is preserved
		meta := rpgerr.GetMeta(err)
		s.Assert().Equal("character_creation", meta["operation"])
		s.Assert().Equal("skill_selection", meta["step"])
		s.Assert().Equal("fighter-skills", meta["choice_id"])
		s.Assert().Equal("arcana", meta["selection"])
	})
}

// Helper methods

func (s *GameServerSimulationSuite) findChoiceByCategory(category choices.Category) *choices.Choice {
	for _, c := range s.availableChoices {
		if c.Category == category {
			return &c
		}
	}
	return nil
}

func (s *GameServerSimulationSuite) getSelectionsForChoice(choiceID choices.ChoiceID) ([]string, bool) {
	// Check different selection maps based on category
	if choiceID == choices.FighterSkills || choiceID == choices.RogueSkills {
		if s.skillSelections != nil {
			return s.skillSelections, true
		}
	}

	if selections, exists := s.equipmentSelections[string(choiceID)]; exists {
		return selections, true
	}

	return nil, false
}

func (s *GameServerSimulationSuite) buildCharacterData() map[string]interface{} {
	// Simulate building character data from selections
	data := make(map[string]interface{})

	data["class"] = s.selectedClass
	data["race"] = s.selectedRace
	data["level"] = s.level
	data["skills"] = s.skillSelections
	data["languages"] = s.languageSelections

	// Flatten equipment selections
	var equipment []string
	for _, items := range s.equipmentSelections {
		equipment = append(equipment, items...)
	}
	data["equipment"] = equipment

	return data
}

// TestGameServerSimulationSuite runs the suite
func TestGameServerSimulationSuite(t *testing.T) {
	suite.Run(t, new(GameServerSimulationSuite))
}
