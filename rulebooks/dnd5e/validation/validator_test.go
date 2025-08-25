package validation

import (
	"testing"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/character"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/classes"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/shared"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/skills"
	"github.com/stretchr/testify/suite"
)

type ValidatorTestSuite struct {
	suite.Suite
}

func TestValidatorSuite(t *testing.T) {
	suite.Run(t, new(ValidatorTestSuite))
}

// Test GetRequiredChoicesForClass
func (s *ValidatorTestSuite) TestGetRequiredChoicesForClass() {
	// Test Bard
	requirements, err := GetRequiredChoicesForClass(classes.Bard)
	s.Require().NoError(err)
	s.Assert().NotNil(requirements)
	s.Assert().Equal("Bard", requirements.ClassName)
	s.Assert().Equal(3, requirements.Level1.Skills.Count)
	s.Assert().Equal(2, requirements.Level1.Cantrips.Count)
	s.Assert().Equal(4, requirements.Level1.Spells.Count)
	s.Assert().Equal(3, requirements.Level1.Instruments.Count)
	
	// Test Fighter
	requirements, err = GetRequiredChoicesForClass(classes.Fighter)
	s.Require().NoError(err)
	s.Assert().NotNil(requirements)
	s.Assert().Equal("Fighter", requirements.ClassName)
	s.Assert().Equal(2, requirements.Level1.Skills.Count)
	s.Assert().Nil(requirements.Level1.Cantrips) // Fighter has no cantrips
	
	// Test non-existent class
	requirements, err = GetRequiredChoicesForClass("NonExistentClass")
	s.Assert().Error(err)
	s.Assert().Nil(requirements)
}

// Test ValidateClassChoices with valid Bard choices
func (s *ValidatorTestSuite) TestValidateClassChoices_Bard_Valid() {
	choices := []character.Choice{
		character.SkillChoice{
			Source:   shared.SourceClass,
			ChoiceID: "bard-skills",
			Skills:   []skills.Skill{skills.Performance, skills.Persuasion, skills.Deception},
		},
		character.CantripChoice{
			Source:   shared.SourceClass,
			ChoiceID: "bard-cantrips",
			Cantrips: []string{"vicious-mockery", "minor-illusion"},
		},
		character.SpellChoice{
			Source:   shared.SourceClass,
			ChoiceID: "bard-spells",
			Spells:   []string{"charm-person", "cure-wounds", "disguise-self", "healing-word"},
		},
		character.EquipmentChoice{
			Source:    shared.SourceClass,
			ChoiceID:  "bard-equipment",
			Equipment: []string{"rapier", "diplomats-pack", "lute"},
		},
		character.InstrumentProficiencyChoice{
			Source:      shared.SourceClass,
			ChoiceID:    "bard-instruments",
			Instruments: []string{"lute", "flute", "drum"},
		},
	}
	
	result, err := ValidateClassChoices(classes.Bard, choices)
	s.Require().NoError(err)
	s.Assert().Empty(result.Errors, "Valid Bard choices should have no errors")
	s.Assert().Empty(result.Warnings, "No warnings expected for single-source choices")
}

// Test ValidateClassChoices with Fighter + Half-Orc duplicate skill warning
func (s *ValidatorTestSuite) TestValidateClassChoices_Fighter_DuplicateSkillWarning() {
	choices := []character.Choice{
		// Fighter chooses Intimidation
		character.SkillChoice{
			Source:   shared.SourceClass,
			ChoiceID: "fighter-skills",
			Skills:   []skills.Skill{skills.Intimidation, skills.Athletics},
		},
		// Half-Orc grants Intimidation (race choice included for warning detection)
		character.SkillChoice{
			Source:   shared.SourceRace,
			ChoiceID: "half-orc-skills",
			Skills:   []skills.Skill{skills.Intimidation},
		},
		character.EquipmentChoice{
			Source:    shared.SourceClass,
			ChoiceID:  "fighter-equipment",
			Equipment: []string{"chain-mail", "longsword", "shield"},
		},
		character.FightingStyleChoice{
			Source:        shared.SourceClass,
			ChoiceID:      "fighter-style",
			FightingStyle: "defense",
		},
	}
	
	result, err := ValidateClassChoices(classes.Fighter, choices)
	s.Require().NoError(err)
	
	// Should have no errors
	s.Assert().Empty(result.Errors, "Valid Fighter choices should have no errors")
	
	// Should have warning about duplicate Intimidation
	s.Require().NotEmpty(result.Warnings, "Should have warnings")
	
	hasIntimidationWarning := false
	for _, w := range result.Warnings {
		if w.Code == WarningDuplicateSkill {
			hasIntimidationWarning = true
			break
		}
	}
	s.Assert().True(hasIntimidationWarning, "Should have warning about duplicate Intimidation")
}

// Test ValidateClassChoices with missing requirements
func (s *ValidatorTestSuite) TestValidateClassChoices_MissingRequirements() {
	// Bard with missing cantrips
	choices := []character.Choice{
		character.SkillChoice{
			Source:   shared.SourceClass,
			ChoiceID: "bard-skills",
			Skills:   []skills.Skill{skills.Performance, skills.Persuasion, skills.Deception},
		},
		// Missing cantrips!
		character.SpellChoice{
			Source:   shared.SourceClass,
			ChoiceID: "bard-spells",
			Spells:   []string{"charm-person", "cure-wounds", "disguise-self", "healing-word"},
		},
		character.EquipmentChoice{
			Source:    shared.SourceClass,
			ChoiceID:  "bard-equipment",
			Equipment: []string{"rapier", "diplomats-pack", "lute"},
		},
		character.InstrumentProficiencyChoice{
			Source:      shared.SourceClass,
			ChoiceID:    "bard-instruments",
			Instruments: []string{"lute", "flute", "drum"},
		},
	}
	
	result, err := ValidateClassChoices(classes.Bard, choices)
	s.Require().NoError(err)
	
	// Should have error about missing cantrips
	s.Require().NotEmpty(result.Errors, "Should have errors for missing cantrips")
	
	hasCantripError := false
	for _, e := range result.Errors {
		if e.Field == "cantrips" {
			hasCantripError = true
			break
		}
	}
	s.Assert().True(hasCantripError, "Should have error about missing cantrips")
}

// Test Rogue with expertise warning
func (s *ValidatorTestSuite) TestValidateClassChoices_Rogue_ExpertiseWarning() {
	choices := []character.Choice{
		character.SkillChoice{
			Source:   shared.SourceClass,
			ChoiceID: "rogue-skills",
			Skills:   []skills.Skill{skills.Stealth, skills.Deception, skills.Investigation, skills.Perception},
		},
		// Expertise in skill not proficient in
		character.ExpertiseChoice{
			Source:    shared.SourceClass,
			ChoiceID:  "rogue-expertise",
			Expertise: []string{"stealth", "persuasion"}, // persuasion not in skills!
		},
		character.EquipmentChoice{
			Source:    shared.SourceClass,
			ChoiceID:  "rogue-equipment",
			Equipment: []string{"shortsword", "shortbow", "burglars-pack"},
		},
	}
	
	result, err := ValidateClassChoices(classes.Rogue, choices)
	s.Require().NoError(err)
	
	// Should have no errors (structurally valid)
	s.Assert().Empty(result.Errors)
	
	// Should have warning about expertise without proficiency
	hasExpertiseWarning := false
	for _, w := range result.Warnings {
		if w.Code == WarningExpertiseWithoutProficiency {
			hasExpertiseWarning = true
			break
		}
	}
	s.Assert().True(hasExpertiseWarning, "Should warn about expertise in non-proficient skill")
}