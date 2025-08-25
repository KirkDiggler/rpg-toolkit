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

type WarningsTestSuite struct {
	suite.Suite
}

func TestWarningsTestSuite(t *testing.T) {
	suite.Run(t, new(WarningsTestSuite))
}

// Test duplicate skill detection across sources
func (s *WarningsTestSuite) TestDetectCrossSourceDuplicates_Skills() {
	choices := []character.Choice{
		// Fighter chooses Intimidation
		character.SkillChoice{
			Source:   shared.SourceClass,
			ChoiceID: "fighter-skills",
			Skills:   []skills.Skill{skills.Intimidation, skills.Athletics},
		},
		// Half-Orc grants Intimidation
		character.SkillChoice{
			Source:   shared.SourceRace,
			ChoiceID: "half-orc-skills",
			Skills:   []skills.Skill{skills.Intimidation},
		},
	}

	warnings := DetectCrossSourceDuplicates(choices)

	s.Require().Len(warnings, 1, "Should have one warning for duplicate Intimidation")
	s.Assert().Equal(WarningDuplicateSkill, warnings[0].Code)
	s.Assert().Contains(warnings[0].Message, "intimidation")
	s.Assert().Contains(warnings[0].Message, "multiple sources")
}

// Test duplicate language detection
func (s *WarningsTestSuite) TestDetectCrossSourceDuplicates_Languages() {
	choices := []character.Choice{
		// Race grants Elvish
		character.LanguageChoice{
			Source:    shared.SourceRace,
			ChoiceID:  "elf-languages",
			Languages: []languages.Language{languages.Common, languages.Elvish},
		},
		// Background also grants Elvish
		character.LanguageChoice{
			Source:    shared.SourceBackground,
			ChoiceID:  "sage-languages",
			Languages: []languages.Language{languages.Elvish, languages.Draconic},
		},
	}

	warnings := DetectCrossSourceDuplicates(choices)

	s.Require().Len(warnings, 1, "Should have one warning for duplicate Elvish")
	s.Assert().Equal(WarningDuplicateLanguage, warnings[0].Code)
	s.Assert().Contains(warnings[0].Message, "elvish") // Language string is lowercase
}

// Test expertise without proficiency warning
func (s *WarningsTestSuite) TestValidateExpertisePrerequisites_MissingProficiency() {
	choices := []character.Choice{
		// Rogue chooses skills
		character.SkillChoice{
			Source:   shared.SourceClass,
			ChoiceID: "rogue-skills",
			Skills:   []skills.Skill{skills.Stealth, skills.Deception, skills.Investigation, skills.Perception},
		},
		// Rogue chooses expertise in skills they don't have
		character.ExpertiseChoice{
			Source:    shared.SourceClass,
			ChoiceID:  "rogue-expertise",
			Expertise: []string{"stealth", "persuasion"}, // persuasion not in skills!
		},
	}

	warnings := ValidateExpertisePrerequisites(choices)

	s.Require().Len(warnings, 1, "Should have warning for expertise without proficiency")
	s.Assert().Equal(WarningExpertiseWithoutProficiency, warnings[0].Code)
	s.Assert().Contains(warnings[0].Message, "persuasion")
	s.Assert().Contains(warnings[0].Message, "requires proficiency")
}

// Test expertise with proficiency (no warning)
func (s *WarningsTestSuite) TestValidateExpertisePrerequisites_ValidExpertise() {
	choices := []character.Choice{
		// Rogue chooses skills
		character.SkillChoice{
			Source:   shared.SourceClass,
			ChoiceID: "rogue-skills",
			Skills:   []skills.Skill{skills.Stealth, skills.Deception, skills.Investigation, skills.Perception},
		},
		// Rogue chooses expertise in skills they have
		character.ExpertiseChoice{
			Source:    shared.SourceClass,
			ChoiceID:  "rogue-expertise",
			Expertise: []string{"stealth", "deception"},
		},
	}

	warnings := ValidateExpertisePrerequisites(choices)

	s.Assert().Empty(warnings, "Should have no warnings for valid expertise choices")
}

// Test expertise in thieves' tools
func (s *WarningsTestSuite) TestValidateExpertisePrerequisites_ThievesTools() {
	choices := []character.Choice{
		// Rogue gets thieves' tools proficiency
		character.ToolProficiencyChoice{
			Source:   shared.SourceClass,
			ChoiceID: "rogue-tools",
			Tools:    []string{"thieves-tools"},
		},
		// Rogue chooses expertise in thieves' tools
		character.ExpertiseChoice{
			Source:    shared.SourceClass,
			ChoiceID:  "rogue-expertise",
			Expertise: []string{"thieves-tools", "stealth"},
		},
		// But no stealth proficiency!
		character.SkillChoice{
			Source:   shared.SourceClass,
			ChoiceID: "rogue-skills",
			Skills:   []skills.Skill{skills.Deception, skills.Investigation, skills.Perception, skills.Acrobatics},
		},
	}

	warnings := ValidateExpertisePrerequisites(choices)

	s.Require().Len(warnings, 1, "Should warn about stealth expertise without proficiency")
	s.Assert().Contains(warnings[0].Message, "stealth")
	s.Assert().NotContains(warnings[0].Message, "thieves-tools", "Should not warn about thieves' tools")
}

// Test full validation with Fighter + Half-Orc
func (s *WarningsTestSuite) TestValidateClassChoicesV3_FighterHalfOrc() {
	choices := []character.Choice{
		// Fighter chooses Intimidation and Athletics
		character.SkillChoice{
			Source:   shared.SourceClass,
			ChoiceID: "fighter-skills",
			Skills:   []skills.Skill{skills.Intimidation, skills.Athletics},
		},
		// Half-Orc grants Intimidation
		character.SkillChoice{
			Source:   shared.SourceRace,
			ChoiceID: "half-orc-skills",
			Skills:   []skills.Skill{skills.Intimidation},
		},
		// Fighter equipment and fighting style
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

	result, err := ValidateClassChoicesV3(classes.Fighter, choices)
	s.Require().NoError(err)

	// Should have no errors (valid Fighter choices)
	s.Assert().Empty(result.Errors, "Valid Fighter choices should have no errors")

	// Should have warning about duplicate Intimidation
	s.Require().NotEmpty(result.Warnings, "Should have warnings")

	hasIntimidationWarning := false
	for _, w := range result.Warnings {
		if w.Code == WarningDuplicateSkill {
			// Check for either case since different sources may format differently
			if containsString(w.Message, "intimidation") || containsString(w.Message, "Intimidation") {
				hasIntimidationWarning = true
			}
		}
	}
	s.Assert().True(hasIntimidationWarning, "Should have warning about duplicate Intimidation")
}

// Test Bard with overlapping background skills
func (s *WarningsTestSuite) TestValidateClassChoicesV3_BardBackgroundOverlap() {
	choices := []character.Choice{
		// Bard chooses Performance (which background also provides)
		character.SkillChoice{
			Source:   shared.SourceClass,
			ChoiceID: "bard-skills",
			Skills:   []skills.Skill{skills.Performance, skills.Persuasion, skills.Deception},
		},
		// Entertainer background grants Performance
		character.SkillChoice{
			Source:   shared.SourceBackground,
			ChoiceID: "entertainer-skills",
			Skills:   []skills.Skill{skills.Performance, skills.Acrobatics},
		},
		// Other required Bard choices
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

	result, err := ValidateClassChoicesV3(classes.Bard, choices)
	s.Require().NoError(err)

	// Should have no errors
	s.Assert().Empty(result.Errors, "Valid Bard choices should have no errors")

	// Should have warning about Performance overlap
	hasPerformanceWarning := false
	for _, w := range result.Warnings {
		if w.Code == WarningDuplicateSkill && containsString(w.Message, "performance") {
			hasPerformanceWarning = true
		}
	}
	s.Assert().True(hasPerformanceWarning, "Should warn about Performance from both sources")
}

// Test Rogue with invalid expertise
func (s *WarningsTestSuite) TestValidateClassChoicesV3_RogueInvalidExpertise() {
	choices := []character.Choice{
		// Rogue chooses 4 skills
		character.SkillChoice{
			Source:   shared.SourceClass,
			ChoiceID: "rogue-skills",
			Skills:   []skills.Skill{skills.Stealth, skills.Deception, skills.Investigation, skills.Perception},
		},
		// Rogue chooses expertise in skill they don't have
		character.ExpertiseChoice{
			Source:    shared.SourceClass,
			ChoiceID:  "rogue-expertise",
			Expertise: []string{"stealth", "persuasion"}, // persuasion not chosen!
		},
		character.EquipmentChoice{
			Source:    shared.SourceClass,
			ChoiceID:  "rogue-equipment",
			Equipment: []string{"shortsword", "shortbow", "burglars-pack"},
		},
	}

	result, err := ValidateClassChoicesV3(classes.Rogue, choices)
	s.Require().NoError(err)

	// Should have no errors (structurally valid)
	s.Assert().Empty(result.Errors)

	// Should have warning about expertise without proficiency
	hasExpertiseWarning := false
	for _, w := range result.Warnings {
		if w.Code == WarningExpertiseWithoutProficiency {
			s.Assert().Contains(w.Message, "persuasion")
			hasExpertiseWarning = true
		}
	}
	s.Assert().True(hasExpertiseWarning, "Should warn about expertise in non-proficient skill")
}

// Helper function
func containsString(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
