package choices_test

import (
	"context"
	"testing"

	"github.com/KirkDiggler/rpg-toolkit/rpgerr"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/armor"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/backgrounds"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/choices"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/classes"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/classes/fighter"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/classes/rogue"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/languages"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/races"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/skills"
	"github.com/stretchr/testify/suite"
)

// TypedCharacterCreationSession represents what the game server would track with proper types
type TypedCharacterCreationSession struct {
	PlayerID    string
	CharacterID string

	// Use the actual types instead of strings!
	Class      classes.Class
	Race       races.Race
	Background backgrounds.Background

	// Track all choices and selections with proper types
	PendingChoices   map[choices.ChoiceID]choices.Choice
	CompletedChoices map[choices.ChoiceID][]string // selections are still strings (IDs)

	// Final character data with proper types
	Skills    []skills.Skill
	Languages []languages.Language
	Equipment struct {
		Armor   []armor.ArmorID
		Weapons []string // weapon IDs
		Gear    []string // gear IDs (future: gear.ItemID)
	}
}

// TypedGameServerSuite shows how the game server would use typed values
type TypedGameServerSuite struct {
	suite.Suite

	ctx     context.Context
	session *TypedCharacterCreationSession
}

func (s *TypedGameServerSuite) SetupTest() {
	s.ctx = context.Background()
	s.ctx = rpgerr.WithMetadata(s.ctx,
		rpgerr.Meta("service", "typed-character-creation"),
		rpgerr.Meta("test", "typed-demo"),
	)

	s.session = &TypedCharacterCreationSession{
		PlayerID:         "test-player",
		CharacterID:      "test-character",
		PendingChoices:   make(map[choices.ChoiceID]choices.Choice),
		CompletedChoices: make(map[choices.ChoiceID][]string),
		Skills:           []skills.Skill{},
		Languages:        []languages.Language{},
	}
	s.session.Equipment.Armor = []armor.ArmorID{}
	s.session.Equipment.Weapons = []string{}
	s.session.Equipment.Gear = []string{}
}

func (s *TypedGameServerSuite) SetupSubTest() {
	// Add session context for each subtest
	s.ctx = rpgerr.WithMetadata(s.ctx,
		rpgerr.Meta("player_id", s.session.PlayerID),
		rpgerr.Meta("character_id", s.session.CharacterID),
	)
}

// TestTypedCharacterCreation demonstrates the benefits of using types
func (s *TypedGameServerSuite) TestTypedCharacterCreation() {
	s.Run("TypeSafetyPreventsErrors", func() {
		// COMPILE-TIME SAFETY - These won't compile:
		// s.session.Class = races.Human        // Wrong type!
		// s.session.Race = classes.Fighter     // Wrong type!
		// s.session.Class = "fightr"           // Typo caught at compile time!

		// Correct usage - compiler enforces this
		s.session.Class = classes.Fighter
		s.session.Race = races.Human
		s.session.Background = backgrounds.Soldier

		// Context can use the string representation when needed
		s.ctx = rpgerr.WithMetadata(s.ctx,
			rpgerr.Meta("class", string(s.session.Class)),
			rpgerr.Meta("race", string(s.session.Race)),
			rpgerr.Meta("background", string(s.session.Background)),
		)

		// Assertions work naturally
		s.Assert().Equal(classes.Fighter, s.session.Class)
		s.Assert().NotEqual(classes.Rogue, s.session.Class)
	})

	s.Run("LoadChoicesBasedOnClass", func() {
		// Switch on typed class - exhaustive and clear
		switch s.session.Class {
		case classes.Fighter:
			s.session.PendingChoices[choices.FighterSkills] = fighter.SkillChoices()
			s.session.PendingChoices[choices.FighterEquipment1] = fighter.StartingEquipmentChoice1()
			s.session.PendingChoices[choices.FighterEquipment2] = fighter.StartingEquipmentChoice2()

		case classes.Rogue:
			s.session.PendingChoices[choices.RogueSkills] = rogue.SkillChoices()
			// Load rogue equipment choices...

		case classes.Wizard:
			// Load wizard choices...
			s.T().Log("Wizard choices not yet implemented")

		default:
			// Compiler helps ensure we handle all cases
			s.T().Logf("Class %s not yet implemented", s.session.Class)
		}

		s.Assert().NotEmpty(s.session.PendingChoices)
	})

	s.Run("ProcessSkillSelections", func() {
		// When player selects skills, we get strings from the UI
		uiSelections := []string{"athletics", "intimidation"}

		// Convert and validate
		selectedSkills := []skills.Skill{}
		for _, selection := range uiSelections {
			skill, err := skills.GetByID(selection)
			if err != nil {
				s.T().Errorf("Invalid skill: %s", selection)
				continue
			}
			selectedSkills = append(selectedSkills, skill)
		}

		// Store as proper types
		s.session.Skills = selectedSkills

		// Now we have type safety - can't accidentally add a language to skills
		s.Assert().Contains(s.session.Skills, skills.Athletics)
		s.Assert().Contains(s.session.Skills, skills.Intimidation)

		// Can't do this - compile error:
		// s.session.Skills = append(s.session.Skills, languages.Elvish)
	})

	s.Run("ProcessEquipmentSelections", func() {
		// Player selects chain mail
		armorSelection := string(armor.ChainMail)

		// Validate it's actually armor
		armorID := armor.ArmorID(armorSelection)
		_, err := armor.GetByID(armorID)
		s.Require().NoError(err)

		// Store with proper type
		s.session.Equipment.Armor = append(s.session.Equipment.Armor, armorID)

		// Add shield
		s.session.Equipment.Armor = append(s.session.Equipment.Armor, armor.Shield)

		// Now we can work with armor data directly
		for _, armorID := range s.session.Equipment.Armor {
			armorData, _ := armor.GetByID(armorID)
			s.T().Logf("Equipped: %s (AC: %d)", armorData.Name, armorData.AC)
		}
	})

	s.Run("RaceSpecificLogic", func() {
		// Type-safe race checks
		switch s.session.Race {
		case races.Human:
			// Humans get +1 to all abilities
			s.T().Log("Applying human ability score improvements")

		case races.Elf, races.HighElf, races.WoodElf:
			// All elves get Darkvision
			s.T().Log("Adding Darkvision feature")

		case races.MountainDwarf:
			// Mountain dwarves get armor proficiencies
			s.T().Log("Adding medium armor proficiency")

		default:
			s.T().Logf("No special logic for race: %s", s.session.Race)
		}

		// Check if it's a subrace
		if s.session.Race.IsSubrace() {
			parentRace := s.session.Race.ParentRace()
			s.T().Logf("Subrace %s of parent race %s", s.session.Race, parentRace)
		}
	})

	s.Run("LanguageProficiencies", func() {
		// Type-safe language handling
		s.session.Languages = []languages.Language{
			languages.Common, // All characters know Common
		}

		// Add racial language
		switch s.session.Race {
		case races.Elf, races.HighElf, races.WoodElf:
			s.session.Languages = append(s.session.Languages, languages.Elvish)
		case races.Dwarf, races.MountainDwarf, races.HillDwarf:
			s.session.Languages = append(s.session.Languages, languages.Dwarvish)
		case races.Human:
			// Human gets to choose one
			s.session.Languages = append(s.session.Languages, languages.Elvish) // player choice
		}

		// Can check if character knows a language
		knowsElvish := s.characterKnowsLanguage(languages.Elvish)
		s.Assert().True(knowsElvish)

		// Can't accidentally check a skill as a language - compile error:
		// s.characterKnowsLanguage(skills.Athletics)
	})

	s.Run("BackgroundBenefits", func() {
		// Type-safe background checks
		switch s.session.Background {
		case backgrounds.Soldier:
			// Military rank feature
			s.T().Log("Adding Military Rank feature")

		case backgrounds.Criminal, backgrounds.Spy:
			// Criminal Contact feature
			s.T().Log("Adding Criminal Contact feature")

		case backgrounds.Noble, backgrounds.Knight:
			// Position of Privilege
			s.T().Log("Adding Position of Privilege feature")
		}

		// Check if it's a variant
		if s.session.Background.IsVariant() {
			base := s.session.Background.BaseBackground()
			s.T().Logf("Variant %s of base background %s", s.session.Background, base)
		}
	})

	s.Run("ValidateCompletedCharacter", func() {
		// All required fields must be set - compiler helps ensure this
		s.Require().NotEqual(classes.Class(""), s.session.Class, "Class must be set")
		s.Require().NotEqual(races.Race(""), s.session.Race, "Race must be set")
		s.Require().NotEqual(backgrounds.Background(""), s.session.Background, "Background must be set")

		// Validate we have the right number of skills for the class
		expectedSkills := s.getExpectedSkillCount(s.session.Class)
		s.Assert().Len(s.session.Skills, expectedSkills)

		// Validate armor makes sense for class
		for _, armorID := range s.session.Equipment.Armor {
			s.validateArmorForClass(armorID, s.session.Class)
		}

		s.T().Log("Character validation complete!")
	})
}

// TestTypeSafetyBenefits demonstrates why types are better than strings
func (s *TypedGameServerSuite) TestTypeSafetyBenefits() {
	s.Run("NoMagicStrings", func() {
		// With strings, these bugs are runtime errors:
		// if session.Class == "fighter" { }  // What if typo?
		// if session.Class == "Fighter" { }  // Capitalization error
		// if session.Class == "warrior" { }  // Wrong game system?

		// With types, these are compile-time checks:
		if s.session.Class == classes.Fighter {
			s.T().Log("This is guaranteed to be correct")
		}

		// IDE autocomplete shows all valid options
		// classes.Fighter, classes.Rogue, classes.Wizard, etc.
	})

	s.Run("RefactoringIsSafe", func() {
		// If we rename a constant:
		// - Change: Fighter Class = "fighter" -> Fighter Class = "fighter-class"
		// - All code using classes.Fighter still works!
		// - Only the string representation changes

		// With strings, we'd have to find/replace everywhere
		// and hope we don't miss any or change the wrong strings
	})

	s.Run("CrossPackageTypeSafety", func() {
		// Can't mix up types between packages
		myClass := classes.Fighter
		myRace := races.Human
		myBackground := backgrounds.Soldier

		// These won't compile - catches errors immediately:
		// myClass = myRace
		// myRace = myBackground
		// myBackground = myClass

		s.Assert().Equal(classes.Fighter, myClass)
		s.Assert().Equal(races.Human, myRace)
		s.Assert().Equal(backgrounds.Soldier, myBackground)
	})

	s.Run("MethodsOnTypes", func() {
		// Types can have methods!
		if s.session.Race.IsSubrace() {
			parent := s.session.Race.ParentRace()
			s.T().Logf("Character is subrace %s of %s", s.session.Race, parent)
		}

		if s.session.Background.IsVariant() {
			base := s.session.Background.BaseBackground()
			s.T().Logf("Background %s is variant of %s", s.session.Background, base)
		}

		// Can't do this with plain strings!
	})
}

// Helper methods

func (s *TypedGameServerSuite) characterKnowsLanguage(lang languages.Language) bool {
	for _, l := range s.session.Languages {
		if l == lang {
			return true
		}
	}
	return false
}

func (s *TypedGameServerSuite) getExpectedSkillCount(class classes.Class) int {
	switch class {
	case classes.Fighter:
		return 2
	case classes.Rogue:
		return 4
	case classes.Ranger:
		return 3
	case classes.Bard:
		return 3
	default:
		return 2
	}
}

func (s *TypedGameServerSuite) validateArmorForClass(armorID armor.ArmorID, class classes.Class) {
	armorData, err := armor.GetByID(armorID)
	s.Require().NoError(err)

	// Check proficiency based on class
	switch class {
	case classes.Fighter, classes.Paladin:
		// Can wear any armor
		s.T().Logf("Fighter/Paladin can wear %s", armorData.Name)

	case classes.Rogue, classes.Bard:
		// Light armor only
		if armorData.Category != armor.CategoryLight && armorData.Category != armor.CategoryShield {
			s.T().Errorf("Rogue/Bard cannot wear %s armor", armorData.Category)
		}

	case classes.Wizard, classes.Sorcerer:
		// No armor proficiency
		if armorData.Category != armor.CategoryShield {
			s.T().Errorf("Wizard/Sorcerer cannot wear armor")
		}
	}
}

// TestGetClassChoices shows how to load choices based on typed class
func (s *TypedGameServerSuite) TestGetClassChoices() {
	// This function would be in the game server
	getChoicesForClass := func(class classes.Class) []choices.Choice {
		switch class {
		case classes.Fighter:
			return []choices.Choice{
				fighter.SkillChoices(),
				fighter.StartingEquipmentChoice1(),
				fighter.StartingEquipmentChoice2(),
			}
		case classes.Rogue:
			return []choices.Choice{
				rogue.SkillChoices(),
				// rogue equipment choices...
			}
		default:
			return []choices.Choice{}
		}
	}

	// Type-safe usage
	fighterChoices := getChoicesForClass(classes.Fighter)
	s.Assert().Len(fighterChoices, 3)

	rogueChoices := getChoicesForClass(classes.Rogue)
	s.Assert().NotEmpty(rogueChoices)

	// Can't pass wrong type - won't compile:
	// getChoicesForClass(races.Human)
	// getChoicesForClass("fighter")
}

// TestTypedGameServerSuite runs the suite
func TestTypedGameServerSuite(t *testing.T) {
	suite.Run(t, new(TypedGameServerSuite))
}
