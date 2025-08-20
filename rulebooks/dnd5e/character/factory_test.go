package character

import (
	"context"
	"testing"

	"github.com/KirkDiggler/rpg-toolkit/events"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/constants"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/shared"
	"github.com/stretchr/testify/suite"
)

type FactoryTestSuite struct {
	suite.Suite
	factory *CharacterFactory
	bus     events.EventBus
	ctx     context.Context
}

func (s *FactoryTestSuite) SetupTest() {
	s.bus = events.NewEventBus()
	s.factory = NewCharacterFactory(s.bus)
	s.ctx = context.Background()
}

func TestFactoryTestSuite(t *testing.T) {
	suite.Run(t, new(FactoryTestSuite))
}

func (s *FactoryTestSuite) TestCreateBarbarian() {
	input := CreateBarbarianInput{
		ID:       "barb_001",
		PlayerID: "player_001",
		Name:     "Conan",
		AbilityScores: shared.AbilityScores{
			constants.STR: 18,
			constants.DEX: 14,
			constants.CON: 16,
			constants.INT: 9,
			constants.WIS: 11,
			constants.CHA: 12,
		},
		SkillChoices: []string{
			"dnd5e:skill:athletics",
			"dnd5e:skill:survival",
		},
		Background: "dnd5e:background:outlander",
		Equipment: []string{
			"dnd5e:item:greatsword",
			"dnd5e:item:explorer_pack",
		},
	}

	char, err := s.factory.CreateBarbarian(s.ctx, input)
	s.Require().NoError(err)
	s.NotNil(char)

	// Verify basic properties
	s.Equal("barb_001", char.id)
	s.Equal("Conan", char.name)
	s.Equal(constants.Class("barbarian"), char.classID)

	// Verify HP calculation (12 + CON modifier of 3)
	s.Equal(15, char.maxHitPoints)
	s.Equal(15, char.hitPoints)

	// Verify features
	s.Len(char.features, 2) // rage and unarmored defense

	// Verify proficiencies
	s.Contains(char.proficiencies.Weapons, "simple_weapons")
	s.Contains(char.proficiencies.Weapons, "martial_weapons")
	s.Contains(char.proficiencies.Armor, "medium_armor")
	s.Contains(char.proficiencies.Armor, "shields")

	// Verify skills
	s.Equal(shared.Proficient, char.skills[constants.SkillAthletics])
	s.Equal(shared.Proficient, char.skills[constants.SkillSurvival])

	// Verify choices were recorded
	s.Len(char.choices, 1)
	s.Equal("dnd5e:choice:barbarian_skills", char.choices[0].ChoiceID)
}

func (s *FactoryTestSuite) TestCreateWizard() {
	input := CreateWizardInput{
		ID:       "wiz_001",
		PlayerID: "player_002",
		Name:     "Gandalf",
		AbilityScores: shared.AbilityScores{
			constants.STR: 10,
			constants.DEX: 14,
			constants.CON: 15,
			constants.INT: 18,
			constants.WIS: 16,
			constants.CHA: 13,
		},
		SkillChoices: []string{
			"dnd5e:skill:arcana",
			"dnd5e:skill:history",
		},
		Background: "dnd5e:background:sage",
		Cantrips: []string{
			"dnd5e:spell:light",
			"dnd5e:spell:mage_hand",
			"dnd5e:spell:minor_illusion",
		},
		Spells: []string{
			"dnd5e:spell:detect_magic",
			"dnd5e:spell:identify",
			"dnd5e:spell:magic_missile",
			"dnd5e:spell:shield",
			"dnd5e:spell:sleep",
			"dnd5e:spell:thunderwave",
		},
	}

	char, err := s.factory.CreateWizard(s.ctx, input)
	s.Require().NoError(err)
	s.NotNil(char)

	// Verify basic properties
	s.Equal("wiz_001", char.id)
	s.Equal("Gandalf", char.name)
	s.Equal(constants.Class("wizard"), char.classID)

	// Verify HP calculation (6 + CON modifier of 2)
	s.Equal(8, char.maxHitPoints)
	s.Equal(8, char.hitPoints)

	// Verify features
	s.Len(char.features, 2) // arcane recovery and spellcasting

	// Verify limited weapon proficiencies
	s.NotContains(char.proficiencies.Weapons, "martial_weapons")
	s.NotContains(char.proficiencies.Armor, "heavy_armor")

	// Verify skills
	s.Equal(shared.Proficient, char.skills[constants.SkillArcana])
	s.Equal(shared.Proficient, char.skills[constants.SkillHistory])

	// Verify standard wizard equipment
	s.Contains(char.equipment, "dnd5e:item:spellbook")
	s.Contains(char.equipment, "dnd5e:item:component_pouch")

	// Verify choices were recorded (skills, cantrips, spells)
	s.Len(char.choices, 3)
}

func (s *FactoryTestSuite) TestQuickCreate() {
	// Test quick barbarian creation
	barbarian, err := s.factory.QuickCreate(s.ctx, QuickCreateInput{
		Name:  "QuickBarb",
		Class: "barbarian",
	})
	s.Require().NoError(err)
	s.NotNil(barbarian)
	s.Equal("QuickBarb", barbarian.name)
	s.Equal(constants.Class("barbarian"), barbarian.classID)
	s.Equal(14, barbarian.maxHitPoints) // 12 + 2 (15 CON = +2 modifier)

	// Test quick wizard creation
	wizard, err := s.factory.QuickCreate(s.ctx, QuickCreateInput{
		Name:  "QuickWiz",
		Class: "wizard",
	})
	s.Require().NoError(err)
	s.NotNil(wizard)
	s.Equal("QuickWiz", wizard.name)
	s.Equal(constants.Class("wizard"), wizard.classID)
	s.Equal(8, wizard.maxHitPoints) // 6 + 2 (14 CON = +2 modifier)

	// Test unknown class
	_, err = s.factory.QuickCreate(s.ctx, QuickCreateInput{
		Name:  "Unknown",
		Class: "unknown_class",
	})
	s.Error(err)
	s.Contains(err.Error(), "unknown class")
}

func (s *FactoryTestSuite) TestFactoryVsBuilder() {
	// This test demonstrates the simplicity improvement
	
	// OLD WAY with Builder (would be like this):
	// builder, _ := NewCharacterBuilder("draft_001")
	// builder.SetName("Thorin")
	// builder.SetRaceData(raceData, "mountain_dwarf")
	// builder.SetClassData(classData, "")
	// builder.SetBackgroundData(backgroundData)
	// builder.SetAbilityScores(scores)
	// builder.SelectSkills([]string{"athletics", "intimidation"})
	// builder.SelectLanguages([]string{"orcish"})
	// builder.SelectEquipment(equipmentChoices)
	// character, _ := builder.Build()
	// ... lots of validation errors to handle ...

	// NEW WAY with Factory (actual working code):
	char, err := s.factory.CreateBarbarian(s.ctx, CreateBarbarianInput{
		ID:       "simple_001",
		PlayerID: "player_001",
		Name:     "Thorin",
		AbilityScores: shared.AbilityScores{
			constants.STR: 16,
			constants.DEX: 14,
			constants.CON: 15,
			constants.INT: 10,
			constants.WIS: 12,
			constants.CHA: 8,
		},
		SkillChoices: []string{
			"dnd5e:skill:athletics",
			"dnd5e:skill:intimidation",
		},
		Background: "dnd5e:background:soldier",
		Equipment: []string{
			"dnd5e:item:greataxe",
			"dnd5e:item:explorer_pack",
		},
	})

	// Much simpler! One function call, clear input structure, no multi-step process
	s.Require().NoError(err)
	s.NotNil(char)
	s.Equal("Thorin", char.name)
	
	// Character is immediately ready to use
	s.True(char.hitPoints > 0)
	s.NotEmpty(char.features)
}