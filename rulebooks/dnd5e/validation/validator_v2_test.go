package validation

import (
	"testing"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/character"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/classes"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/shared"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/skills"
	"github.com/stretchr/testify/suite"
)

type ValidatorV2TestSuite struct {
	suite.Suite
}

func TestValidatorV2Suite(t *testing.T) {
	suite.Run(t, new(ValidatorV2TestSuite))
}

// Test that Bard validation works with new Choice types
func (s *ValidatorV2TestSuite) TestValidateBardChoicesV2_Valid() {
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

	errors, err := ValidateClassChoicesV2(classes.Bard, choices)
	s.Require().NoError(err)
	s.Assert().Empty(errors, "Valid Bard choices should have no errors")
}

// Test that Bard validation ignores non-class choices
func (s *ValidatorV2TestSuite) TestValidateBardChoicesV2_IgnoresNonClassSources() {
	choices := []character.Choice{
		// Class choices
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
		// Race choice (should be ignored)
		character.SkillChoice{
			Source:   shared.SourceRace,
			ChoiceID: "half-elf-skills",
			Skills:   []skills.Skill{skills.Persuasion, skills.Deception},
		},
		// Background choice (should be ignored)
		character.LanguageChoice{
			Source:    shared.SourceBackground,
			ChoiceID:  "sage-languages",
			Languages: nil, // Doesn't matter, should be ignored
		},
	}

	errors, err := ValidateClassChoicesV2(classes.Bard, choices)
	s.Require().NoError(err)
	s.Require().NotEmpty(errors, "Should have errors for missing class choices")

	// Should be missing skills, equipment, and instruments
	hasSkillError := false
	hasEquipmentError := false
	hasInstrumentError := false

	for _, e := range errors {
		if e.Field == fieldSkills {
			hasSkillError = true
		}
		if e.Field == "equipment" {
			hasEquipmentError = true
		}
		if e.Field == "instruments" {
			hasInstrumentError = true
		}
	}

	s.Assert().True(hasSkillError, "Should have error about missing skills")
	s.Assert().True(hasEquipmentError, "Should have error about missing equipment")
	s.Assert().True(hasInstrumentError, "Should have error about missing instruments")
}

// Test Fighter validation with new Choice types
func (s *ValidatorV2TestSuite) TestValidateFighterChoicesV2_Valid() {
	choices := []character.Choice{
		character.SkillChoice{
			Source:   shared.SourceClass,
			ChoiceID: "fighter-skills",
			Skills:   []skills.Skill{skills.Athletics, skills.Intimidation},
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

	errors, err := ValidateClassChoicesV2(classes.Fighter, choices)
	s.Require().NoError(err)
	s.Assert().Empty(errors, "Valid Fighter choices should have no errors")
}

// Test Fighter with invalid skill choices
func (s *ValidatorV2TestSuite) TestValidateFighterChoicesV2_InvalidSkills() {
	choices := []character.Choice{
		character.SkillChoice{
			Source:   shared.SourceClass,
			ChoiceID: "fighter-skills",
			// Performance is not a valid Fighter skill
			Skills: []skills.Skill{skills.Athletics, skills.Performance},
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

	errors, err := ValidateClassChoicesV2(classes.Fighter, choices)
	s.Require().NoError(err)
	s.Require().NotEmpty(errors, "Should have error for invalid skill")

	hasInvalidSkillError := false
	for _, e := range errors {
		s.T().Logf("Error: Field=%s, Message=%s", e.Field, e.Message)
		// The error message uses lowercase "performance"
		if e.Field == fieldSkills && (findSubstring(e.Message, "Performance") || findSubstring(e.Message, "performance")) {
			hasInvalidSkillError = true
		}
	}
	s.Assert().True(hasInvalidSkillError, "Should have error about Performance not being a valid Fighter skill")
}

// Test Rogue validation with new Choice types
func (s *ValidatorV2TestSuite) TestValidateRogueChoicesV2_Valid() {
	choices := []character.Choice{
		character.SkillChoice{
			Source:   shared.SourceClass,
			ChoiceID: "rogue-skills",
			Skills:   []skills.Skill{skills.Stealth, skills.Deception, skills.Investigation, skills.Perception},
		},
		character.ExpertiseChoice{
			Source:    shared.SourceClass,
			ChoiceID:  "rogue-expertise",
			Expertise: []string{"stealth", "thieves-tools"},
		},
		character.EquipmentChoice{
			Source:    shared.SourceClass,
			ChoiceID:  "rogue-equipment",
			Equipment: []string{"shortsword", "shortbow", "burglars-pack", "leather-armor", "dagger", "thieves-tools"},
		},
	}

	errors, err := ValidateClassChoicesV2(classes.Rogue, choices)
	s.Require().NoError(err)
	s.Assert().Empty(errors, "Valid Rogue choices should have no errors")
}

// Test Rogue missing expertise
func (s *ValidatorV2TestSuite) TestValidateRogueChoicesV2_MissingExpertise() {
	choices := []character.Choice{
		character.SkillChoice{
			Source:   shared.SourceClass,
			ChoiceID: "rogue-skills",
			Skills:   []skills.Skill{skills.Stealth, skills.Deception, skills.Investigation, skills.Perception},
		},
		character.EquipmentChoice{
			Source:    shared.SourceClass,
			ChoiceID:  "rogue-equipment",
			Equipment: []string{"shortsword", "shortbow", "burglars-pack", "leather-armor", "dagger", "thieves-tools"},
		},
		// Missing expertise choice
	}

	errors, err := ValidateClassChoicesV2(classes.Rogue, choices)
	s.Require().NoError(err)
	s.Require().NotEmpty(errors, "Should have error for missing expertise")

	hasExpertiseError := false
	for _, e := range errors {
		if e.Field == fieldExpertise {
			hasExpertiseError = true
		}
	}
	s.Assert().True(hasExpertiseError, "Should have error about missing expertise")
}

// Helper function
func findSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
