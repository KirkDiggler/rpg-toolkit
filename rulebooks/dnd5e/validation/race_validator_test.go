package validation

import (
	"testing"

	"github.com/KirkDiggler/rpg-toolkit/rpgerr"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/character"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/languages"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/races"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/shared"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/skills"
	"github.com/stretchr/testify/suite"
)

type RaceValidatorTestSuite struct {
	suite.Suite
}

func TestRaceValidatorSuite(t *testing.T) {
	suite.Run(t, new(RaceValidatorTestSuite))
}

// Test that races requiring subraces are validated
func (s *RaceValidatorTestSuite) TestValidateRaceChoices_RequiresSubrace() {
	// Elf requires a subrace
	errors, err := ValidateRaceChoices(races.Elf, "", []character.ChoiceData{})
	s.Require().NoError(err)
	s.Require().NotEmpty(errors)

	hasSubraceError := false
	for _, e := range errors {
		if e.Field == "subrace" {
			s.Assert().Contains(e.Message, "elf requires a subrace selection")
			hasSubraceError = true
		}
	}
	s.Assert().True(hasSubraceError, "Should have error about missing subrace")
}

// Test that invalid subrace combinations are caught
func (s *RaceValidatorTestSuite) TestValidateRaceChoices_InvalidSubrace() {
	// Mountain Dwarf is not a valid subrace for Elf
	errors, err := ValidateRaceChoices(races.Elf, races.MountainDwarf, []character.ChoiceData{})
	s.Require().NoError(err)
	s.Require().NotEmpty(errors)

	hasInvalidError := false
	for _, e := range errors {
		if e.Field == "subrace" {
			s.Assert().Contains(e.Message, "mountain-dwarf is not a valid subrace for elf")
			hasInvalidError = true
		}
	}
	s.Assert().True(hasInvalidError, "Should have error about invalid subrace")
}

// Test Human validation (no choices required)
func (s *RaceValidatorTestSuite) TestValidateHumanChoices_Valid() {
	// Standard Human has no choices
	errors, err := ValidateRaceChoices(races.Human, "", []character.ChoiceData{})
	s.Require().NoError(err)
	s.Assert().Empty(errors)
}

// Test Half-Elf validation
func (s *RaceValidatorTestSuite) TestValidateHalfElfChoices_Valid() {
	choices := []character.ChoiceData{
		{
			Category:       shared.ChoiceSkills,
			Source:         shared.SourceRace,
			ChoiceID:       "half-elf-skills",
			SkillSelection: []skills.Skill{skills.Persuasion, skills.Deception},
		},
		{
			Category:          shared.ChoiceLanguages,
			Source:            shared.SourceRace,
			ChoiceID:          "half-elf-language",
			LanguageSelection: []languages.Language{languages.Dwarvish},
		},
	}

	errors, err := ValidateRaceChoices(races.HalfElf, "", choices)
	s.Require().NoError(err)
	s.Assert().Empty(errors)
}

func (s *RaceValidatorTestSuite) TestValidateHalfElfChoices_MissingSkills() {
	choices := []character.ChoiceData{
		{
			Category:          shared.ChoiceLanguages,
			Source:            shared.SourceRace,
			ChoiceID:          "half-elf-language",
			LanguageSelection: []languages.Language{languages.Dwarvish},
		},
		// Missing skill choice
	}

	errors, err := ValidateRaceChoices(races.HalfElf, "", choices)
	s.Require().NoError(err)
	s.Require().NotEmpty(errors)

	hasSkillError := false
	for _, e := range errors {
		if e.Field == fieldRaceSkills {
			s.Assert().Contains(e.Message, "Half-Elf requires 2 skill proficiency choices")
			hasSkillError = true
		}
	}
	s.Assert().True(hasSkillError, "Should have error about missing skills")
}

func (s *RaceValidatorTestSuite) TestValidateHalfElfChoices_WrongSkillCount() {
	choices := []character.ChoiceData{
		{
			Category:       shared.ChoiceSkills,
			Source:         shared.SourceRace,
			ChoiceID:       "half-elf-skills",
			SkillSelection: []skills.Skill{skills.Persuasion}, // Only 1, needs 2
		},
		{
			Category:          shared.ChoiceLanguages,
			Source:            shared.SourceRace,
			ChoiceID:          "half-elf-language",
			LanguageSelection: []languages.Language{languages.Dwarvish},
		},
	}

	errors, err := ValidateRaceChoices(races.HalfElf, "", choices)
	s.Require().NoError(err)
	s.Require().NotEmpty(errors)

	hasCountError := false
	for _, e := range errors {
		if e.Field == fieldRaceSkills {
			s.Assert().Contains(e.Message, "Half-Elf requires exactly 2 skill proficiencies, 1 selected")
			hasCountError = true
		}
	}
	s.Assert().True(hasCountError, "Should have error about skill count")
}

func (s *RaceValidatorTestSuite) TestValidateHalfElfChoices_MissingLanguage() {
	choices := []character.ChoiceData{
		{
			Category:       shared.ChoiceSkills,
			Source:         shared.SourceRace,
			ChoiceID:       "half-elf-skills",
			SkillSelection: []skills.Skill{skills.Persuasion, skills.Deception},
		},
		// Missing language choice
	}

	errors, err := ValidateRaceChoices(races.HalfElf, "", choices)
	s.Require().NoError(err)
	s.Require().NotEmpty(errors)

	hasLangError := false
	for _, e := range errors {
		if e.Field == fieldLanguages {
			s.Assert().Contains(e.Message, "Half-Elf requires 1 additional language choice")
			hasLangError = true
		}
	}
	s.Assert().True(hasLangError, "Should have error about missing language")
}

// Test High Elf validation
func (s *RaceValidatorTestSuite) TestValidateHighElfChoices_Valid() {
	choices := []character.ChoiceData{
		{
			Category:          shared.ChoiceLanguages,
			Source:            shared.SourceRace,
			ChoiceID:          "high-elf-language",
			LanguageSelection: []languages.Language{languages.Draconic},
		},
		{
			Category:         shared.ChoiceCantrips,
			Source:           shared.SourceRace,
			ChoiceID:         "high-elf-cantrip",
			CantripSelection: []string{"prestidigitation"},
		},
	}

	errors, err := ValidateRaceChoices(races.Elf, races.HighElf, choices)
	s.Require().NoError(err)
	s.Assert().Empty(errors)
}

func (s *RaceValidatorTestSuite) TestValidateHighElfChoices_MissingCantrip() {
	choices := []character.ChoiceData{
		{
			Category:          shared.ChoiceLanguages,
			Source:            shared.SourceRace,
			ChoiceID:          "high-elf-language",
			LanguageSelection: []languages.Language{languages.Draconic},
		},
		// Missing cantrip choice
	}

	errors, err := ValidateRaceChoices(races.Elf, races.HighElf, choices)
	s.Require().NoError(err)
	s.Require().NotEmpty(errors)

	hasCantripError := false
	for _, e := range errors {
		if e.Field == fieldCantrips {
			s.Assert().Contains(e.Message, "High Elf requires 1 wizard cantrip choice")
			hasCantripError = true
		}
	}
	s.Assert().True(hasCantripError, "Should have error about missing cantrip")
}

func (s *RaceValidatorTestSuite) TestValidateHighElfChoices_TooManyCantrips() {
	choices := []character.ChoiceData{
		{
			Category:          shared.ChoiceLanguages,
			Source:            shared.SourceRace,
			ChoiceID:          "high-elf-language",
			LanguageSelection: []languages.Language{languages.Draconic},
		},
		{
			Category:         shared.ChoiceCantrips,
			Source:           shared.SourceRace,
			ChoiceID:         "high-elf-cantrip",
			CantripSelection: []string{"prestidigitation", "mage-hand"}, // 2 cantrips, only 1 allowed
		},
	}

	errors, err := ValidateRaceChoices(races.Elf, races.HighElf, choices)
	s.Require().NoError(err)
	s.Require().NotEmpty(errors)

	hasCantripError := false
	for _, e := range errors {
		if e.Field == fieldCantrips {
			s.Assert().Contains(e.Message, "High Elf requires exactly 1 wizard cantrip, 2 selected")
			hasCantripError = true
		}
	}
	s.Assert().True(hasCantripError, "Should have error about cantrip count")
}

// Test Dragonborn validation
func (s *RaceValidatorTestSuite) TestValidateDragonbornChoices_Valid() {
	choices := []character.ChoiceData{
		{
			Category:       shared.ChoiceTraits,
			Source:         shared.SourceRace,
			ChoiceID:       "dragonborn-ancestry",
			TraitSelection: []string{"draconic-ancestry-red"},
		},
	}

	errors, err := ValidateRaceChoices(races.Dragonborn, "", choices)
	s.Require().NoError(err)
	s.Assert().Empty(errors)
}

func (s *RaceValidatorTestSuite) TestValidateDragonbornChoices_MissingAncestry() {
	choices := []character.ChoiceData{}

	errors, err := ValidateRaceChoices(races.Dragonborn, "", choices)
	s.Require().NoError(err)
	s.Require().NotEmpty(errors)

	hasAncestryError := false
	for _, e := range errors {
		if e.Field == fieldDraconicAncestry {
			s.Assert().Contains(e.Message, "Dragonborn requires draconic ancestry choice")
			hasAncestryError = true
		}
	}
	s.Assert().True(hasAncestryError, "Should have error about missing ancestry")
}

func (s *RaceValidatorTestSuite) TestValidateDragonbornChoices_InvalidAncestry() {
	choices := []character.ChoiceData{
		{
			Category:       shared.ChoiceTraits,
			Source:         shared.SourceRace,
			ChoiceID:       "dragonborn-ancestry",
			TraitSelection: []string{"draconic-ancestry-purple"}, // Invalid color
		},
	}

	errors, err := ValidateRaceChoices(races.Dragonborn, "", choices)
	s.Require().NoError(err)
	s.Require().NotEmpty(errors)

	hasInvalidError := false
	for _, e := range errors {
		if e.Field == fieldDraconicAncestry {
			s.Assert().Contains(e.Message, "Invalid draconic ancestry: purple")
			hasInvalidError = true
		}
	}
	s.Assert().True(hasInvalidError, "Should have error about invalid ancestry")
}

func (s *RaceValidatorTestSuite) TestValidateDragonbornChoices_BadFormat() {
	choices := []character.ChoiceData{
		{
			Category:       shared.ChoiceTraits,
			Source:         shared.SourceRace,
			ChoiceID:       "dragonborn-ancestry",
			TraitSelection: []string{"red-dragon"}, // Wrong format
		},
	}

	errors, err := ValidateRaceChoices(races.Dragonborn, "", choices)
	s.Require().NoError(err)
	s.Require().NotEmpty(errors)

	hasFormatError := false
	for _, e := range errors {
		if e.Field == fieldDraconicAncestry {
			s.Assert().Contains(e.Message, "Invalid draconic ancestry format: red-dragon")
			hasFormatError = true
		}
	}
	s.Assert().True(hasFormatError, "Should have error about ancestry format")
}

func (s *RaceValidatorTestSuite) TestValidateDragonbornChoices_TooManyAncestries() {
	choices := []character.ChoiceData{
		{
			Category:       shared.ChoiceTraits,
			Source:         shared.SourceRace,
			ChoiceID:       "dragonborn-ancestry",
			TraitSelection: []string{"draconic-ancestry-red", "draconic-ancestry-blue"}, // Too many
		},
	}

	errors, err := ValidateRaceChoices(races.Dragonborn, "", choices)
	s.Require().NoError(err)
	s.Require().NotEmpty(errors)

	hasCountError := false
	for _, e := range errors {
		if e.Field == fieldDraconicAncestry {
			s.Assert().Contains(e.Message, "Dragonborn requires exactly 1 draconic ancestry, 2 selected")
			hasCountError = true
		}
	}
	s.Assert().True(hasCountError, "Should have error about ancestry count")
}

// Test Wood Elf (no additional choices beyond base Elf)
func (s *RaceValidatorTestSuite) TestValidateWoodElfChoices_Valid() {
	// Wood Elf has no additional choices
	errors, err := ValidateRaceChoices(races.Elf, races.WoodElf, []character.ChoiceData{})
	s.Require().NoError(err)
	s.Assert().Empty(errors)
}

// Test Dwarf races (no choices)
func (s *RaceValidatorTestSuite) TestValidateDwarfChoices_Valid() {
	// Hill Dwarf has no choices
	errors, err := ValidateRaceChoices(races.Dwarf, races.HillDwarf, []character.ChoiceData{})
	s.Require().NoError(err)
	s.Assert().Empty(errors)

	// Mountain Dwarf has no choices
	errors, err = ValidateRaceChoices(races.Dwarf, races.MountainDwarf, []character.ChoiceData{})
	s.Require().NoError(err)
	s.Assert().Empty(errors)
}

// Test that non-race choices are ignored
func (s *RaceValidatorTestSuite) TestValidateRaceChoices_IgnoresNonRaceChoices() {
	choices := []character.ChoiceData{
		// Race choice
		{
			Category:       shared.ChoiceSkills,
			Source:         shared.SourceRace,
			ChoiceID:       "half-elf-skills",
			SkillSelection: []skills.Skill{skills.Persuasion, skills.Deception},
		},
		{
			Category:          shared.ChoiceLanguages,
			Source:            shared.SourceRace,
			ChoiceID:          "half-elf-language",
			LanguageSelection: []languages.Language{languages.Dwarvish},
		},
		// Non-race choices that should be ignored
		{
			Category:       shared.ChoiceSkills,
			Source:         shared.SourceClass,
			ChoiceID:       "fighter-skills",
			SkillSelection: []skills.Skill{skills.Athletics, skills.Intimidation},
		},
		{
			Category:          shared.ChoiceLanguages,
			Source:            shared.SourceBackground,
			ChoiceID:          "sage-languages",
			LanguageSelection: []languages.Language{languages.Celestial, languages.Infernal},
		},
	}

	errors, err := ValidateRaceChoices(races.HalfElf, "", choices)
	s.Require().NoError(err)
	s.Assert().Empty(errors, "Should not have errors when all race choices are valid")
}

// Test empty race ID
func (s *RaceValidatorTestSuite) TestValidateRaceChoices_EmptyRaceID() {
	errors, err := ValidateRaceChoices("", "", []character.ChoiceData{})
	s.Assert().Error(err)
	s.Assert().Equal(rpgerr.CodeInvalidArgument, err.(*rpgerr.Error).Code)
	s.Assert().Contains(err.Error(), "race ID is required")
	s.Assert().Nil(errors)
}

// Test all valid ancestry colors for Dragonborn
func (s *RaceValidatorTestSuite) TestValidateDragonbornChoices_AllValidColors() {
	validColors := []string{"black", "blue", "brass", "bronze", "copper", "gold", "green", "red", "silver", "white"}

	for _, color := range validColors {
		choices := []character.ChoiceData{
			{
				Category:       shared.ChoiceTraits,
				Source:         shared.SourceRace,
				ChoiceID:       "dragonborn-ancestry",
				TraitSelection: []string{"draconic-ancestry-" + color},
			},
		}

		errors, err := ValidateRaceChoices(races.Dragonborn, "", choices)
		s.Require().NoError(err)
		s.Assert().Empty(errors, "Should accept valid ancestry color: %s", color)
	}
}
