package validation

import (
	"testing"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/character"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/classes"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/languages"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/shared"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/skills"
	"github.com/stretchr/testify/suite"
)

type BardValidatorTestSuite struct {
	suite.Suite
}

func TestBardValidatorSuite(t *testing.T) {
	suite.Run(t, new(BardValidatorTestSuite))
}

// Test that Bard only validates class-sourced choices
func (s *BardValidatorTestSuite) TestValidateBardChoices_IgnoresNonClassSources() {
	// Create choices with mixed sources
	choices := []character.ChoiceData{
		// Class choices (should be validated)
		{
			Category:         shared.ChoiceCantrips,
			Source:           shared.SourceClass,
			ChoiceID:         "bard-cantrips",
			CantripSelection: []string{"vicious-mockery", "minor-illusion"},
		},
		{
			Category:       shared.ChoiceSpells,
			Source:         shared.SourceClass,
			ChoiceID:       "bard-spells",
			SpellSelection: []string{"charm-person", "cure-wounds", "disguise-self", "healing-word"},
		},
		// Race choice (should be IGNORED)
		{
			Category:       shared.ChoiceSkills,
			Source:         shared.SourceRace,
			ChoiceID:       "half-elf-skills",
			SkillSelection: []skills.Skill{skills.Persuasion, skills.Deception},
		},
		// Background choice (should be IGNORED)
		{
			Category:          shared.ChoiceLanguages,
			Source:            shared.SourceBackground,
			ChoiceID:          "sage-languages",
			LanguageSelection: []languages.Language{languages.Elvish, languages.Dwarvish},
		},
	}

	errors, err := ValidateClassChoices(classes.Bard, choices)
	s.Require().NoError(err)

	// Should have errors about missing class choices (skills, equipment, tools)
	// but NOT complain about the race/background choices
	s.Require().NotEmpty(errors)

	// Check that it's missing the class skills, not complaining about race skills
	hasClassSkillError := false
	for _, e := range errors {
		if e.Field == fieldClassChoices {
			s.Assert().Contains(e.Message, "skills")
			s.Assert().Contains(e.Message, "equipment")
			s.Assert().Contains(e.Message, "tool_proficiency")
			hasClassSkillError = true
		}
	}
	s.Assert().True(hasClassSkillError, "Should have error about missing class choices")
}

// Test that Bard validates all required choices including musical instruments
func (s *BardValidatorTestSuite) TestValidateBardChoices_RequiresMusicalInstruments() {
	choices := []character.ChoiceData{
		{
			Category:       shared.ChoiceSkills,
			Source:         shared.SourceClass,
			ChoiceID:       "bard-skills",
			SkillSelection: []skills.Skill{skills.Performance, skills.Persuasion, skills.Deception},
		},
		{
			Category:         shared.ChoiceCantrips,
			Source:           shared.SourceClass,
			ChoiceID:         "bard-cantrips",
			CantripSelection: []string{"vicious-mockery", "minor-illusion"},
		},
		{
			Category:       shared.ChoiceSpells,
			Source:         shared.SourceClass,
			ChoiceID:       "bard-spells",
			SpellSelection: []string{"charm-person", "cure-wounds", "disguise-self", "healing-word"},
		},
		{
			Category:           shared.ChoiceEquipment,
			Source:             shared.SourceClass,
			ChoiceID:           "bard-equipment",
			EquipmentSelection: []string{"rapier", "diplomats-pack", "lute"},
		},
		// Missing musical instrument proficiencies!
	}

	errors, err := ValidateClassChoices(classes.Bard, choices)
	s.Require().NoError(err)
	s.Require().NotEmpty(errors)

	// Should have error about missing musical instruments
	hasInstrumentError := false
	for _, e := range errors {
		if e.Field == fieldClassChoices {
			s.Assert().Contains(e.Message, "tool_proficiency")
			hasInstrumentError = true
		}
	}
	s.Assert().True(hasInstrumentError, "Should have error about missing musical instruments")
}

// Test valid Bard with all choices including instruments
func (s *BardValidatorTestSuite) TestValidateBardChoices_ValidWithInstruments() {
	choices := []character.ChoiceData{
		{
			Category:       shared.ChoiceSkills,
			Source:         shared.SourceClass,
			ChoiceID:       "bard-skills",
			SkillSelection: []skills.Skill{skills.Performance, skills.Persuasion, skills.Deception},
		},
		{
			Category:         shared.ChoiceCantrips,
			Source:           shared.SourceClass,
			ChoiceID:         "bard-cantrips",
			CantripSelection: []string{"vicious-mockery", "minor-illusion"},
		},
		{
			Category:       shared.ChoiceSpells,
			Source:         shared.SourceClass,
			ChoiceID:       "bard-spells",
			SpellSelection: []string{"charm-person", "cure-wounds", "disguise-self", "healing-word"},
		},
		{
			Category:           shared.ChoiceEquipment,
			Source:             shared.SourceClass,
			ChoiceID:           "bard-equipment",
			EquipmentSelection: []string{"rapier", "diplomats-pack", "lute"},
		},
		{
			Category:                 shared.ChoiceToolProficiency,
			Source:                   shared.SourceClass,
			ChoiceID:                 "bard-instruments",
			ToolProficiencySelection: []string{"lute", "flute", "drum"},
		},
	}

	errors, err := ValidateClassChoices(classes.Bard, choices)
	s.Require().NoError(err)
	s.Assert().Empty(errors, "Should have no errors with all choices provided")
}
