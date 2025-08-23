package validation

import (
	"testing"

	"github.com/KirkDiggler/rpg-toolkit/rpgerr"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/character"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/classes"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/languages"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/shared"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/skills"
	"github.com/stretchr/testify/suite"
)

const (
	fieldClassChoices = "class_choices"
	fieldSkills       = "skills"
)

type ClassValidatorTestSuite struct {
	suite.Suite
}

func TestClassValidatorSuite(t *testing.T) {
	suite.Run(t, new(ClassValidatorTestSuite))
}

// Fighter validation tests
func (s *ClassValidatorTestSuite) TestValidateFighterChoices_Valid() {
	fightingStyle := "archery"
	choices := []character.ChoiceData{
		{
			Category:       shared.ChoiceSkills,
			Source:         shared.SourceClass,
			ChoiceID:       "fighter-skills",
			SkillSelection: []skills.Skill{skills.Athletics, skills.Intimidation},
		},
		{
			Category:               shared.ChoiceFightingStyle,
			Source:                 shared.SourceClass,
			ChoiceID:               "fighter-fighting-style",
			FightingStyleSelection: &fightingStyle,
		},
		{
			Category:           shared.ChoiceEquipment,
			Source:             shared.SourceClass,
			ChoiceID:           "fighter-armor-choice",
			EquipmentSelection: []string{"chain-mail"},
		},
		{
			Category:           shared.ChoiceEquipment,
			Source:             shared.SourceClass,
			ChoiceID:           "fighter-primary-weapon",
			EquipmentSelection: []string{"longsword", "shield"},
		},
		{
			Category:           shared.ChoiceEquipment,
			Source:             shared.SourceClass,
			ChoiceID:           "fighter-secondary-equipment",
			EquipmentSelection: []string{"shield"},
		},
		{
			Category:           shared.ChoiceEquipment,
			Source:             shared.SourceClass,
			ChoiceID:           "fighter-ranged-choice",
			EquipmentSelection: []string{"crossbow-light", "bolts-20"},
		},
		{
			Category:           shared.ChoiceEquipment,
			Source:             shared.SourceClass,
			ChoiceID:           "fighter-pack-choice",
			EquipmentSelection: []string{"dungeoneers-pack"},
		},
	}

	errors, err := ValidateClassChoices(classes.Fighter, choices)
	s.Require().NoError(err)
	s.Assert().Empty(errors)
}

func (s *ClassValidatorTestSuite) TestValidateFighterChoices_MissingSkills() {
	fightingStyle := "defense"
	choices := []character.ChoiceData{
		{
			Category:               shared.ChoiceFightingStyle,
			Source:                 shared.SourceClass,
			ChoiceID:               "fighter-fighting-style",
			FightingStyleSelection: &fightingStyle,
		},
	}

	errors, err := ValidateClassChoices(classes.Fighter, choices)
	s.Require().NoError(err)
	s.Require().NotEmpty(errors)

	// Should have error about missing skills
	hasSkillError := false
	for _, e := range errors {
		if e.Field == fieldClassChoices {
			s.Assert().Contains(e.Message, "skill proficiencies")
			hasSkillError = true
		}
	}
	s.Assert().True(hasSkillError, "Should have error about missing skills")
}

func (s *ClassValidatorTestSuite) TestValidateFighterChoices_InsufficientSkills() {
	fightingStyle := "dueling"
	choices := []character.ChoiceData{
		{
			Category:       shared.ChoiceSkills,
			Source:         shared.SourceClass,
			ChoiceID:       "fighter-skills",
			SkillSelection: []skills.Skill{skills.Athletics}, // Only 1, needs 2
		},
		{
			Category:               shared.ChoiceFightingStyle,
			Source:                 shared.SourceClass,
			ChoiceID:               "fighter-fighting-style",
			FightingStyleSelection: &fightingStyle,
		},
	}

	errors, err := ValidateClassChoices(classes.Fighter, choices)
	s.Require().NoError(err)
	s.Require().NotEmpty(errors)

	hasSkillError := false
	for _, e := range errors {
		if e.Field == fieldSkills {
			s.Assert().Contains(e.Message, "requires 2 skill proficiencies")
			s.Assert().Contains(e.Message, "only 1 selected")
			hasSkillError = true
		}
	}
	s.Assert().True(hasSkillError, "Should have error about insufficient skills")
}

func (s *ClassValidatorTestSuite) TestValidateFighterChoices_InvalidFightingStyle() {
	invalidStyle := "invalid-style"
	choices := []character.ChoiceData{
		{
			Category:       shared.ChoiceSkills,
			Source:         shared.SourceClass,
			ChoiceID:       "fighter-skills",
			SkillSelection: []skills.Skill{skills.Athletics, skills.Survival},
		},
		{
			Category:               shared.ChoiceFightingStyle,
			Source:                 shared.SourceClass,
			ChoiceID:               "fighter-fighting-style",
			FightingStyleSelection: &invalidStyle,
		},
	}

	errors, err := ValidateClassChoices(classes.Fighter, choices)
	s.Require().NoError(err)
	s.Require().NotEmpty(errors)

	hasStyleError := false
	for _, e := range errors {
		if e.Field == "fighting_style" {
			s.Assert().Contains(e.Message, "Invalid fighting style")
			s.Assert().Equal(rpgerr.CodeInvalidArgument, e.Code)
			hasStyleError = true
		}
	}
	s.Assert().True(hasStyleError, "Should have error about invalid fighting style")
}

// Wizard validation tests
func (s *ClassValidatorTestSuite) TestValidateWizardChoices_Valid() {
	choices := []character.ChoiceData{
		{
			Category:       shared.ChoiceSkills,
			Source:         shared.SourceClass,
			ChoiceID:       "wizard-skills",
			SkillSelection: []skills.Skill{skills.Arcana, skills.Investigation},
		},
		{
			Category:         shared.ChoiceCantrips,
			Source:           shared.SourceClass,
			ChoiceID:         "wizard-cantrips",
			CantripSelection: []string{"fire-bolt", "mage-hand", "prestidigitation"},
		},
		{
			Category:       shared.ChoiceSpells,
			Source:         shared.SourceClass,
			ChoiceID:       "wizard-spells-level-1",
			SpellSelection: []string{"magic-missile", "shield", "identify", "detect-magic", "sleep", "burning-hands"},
		},
		{
			Category:           shared.ChoiceEquipment,
			Source:             shared.SourceClass,
			ChoiceID:           "wizard-equipment-primary-weapon",
			EquipmentSelection: []string{"quarterstaff"},
		},
		{
			Category:           shared.ChoiceEquipment,
			Source:             shared.SourceClass,
			ChoiceID:           "wizard-equipment-focus",
			EquipmentSelection: []string{"component-pouch"},
		},
		{
			Category:           shared.ChoiceEquipment,
			Source:             shared.SourceClass,
			ChoiceID:           "wizard-equipment-pack",
			EquipmentSelection: []string{"scholars-pack"},
		},
	}

	errors, err := ValidateClassChoices(classes.Wizard, choices)
	s.Require().NoError(err)
	s.Assert().Empty(errors)
}

func (s *ClassValidatorTestSuite) TestValidateWizardChoices_MissingSkills() {
	choices := []character.ChoiceData{
		{
			Category:         shared.ChoiceCantrips,
			Source:           shared.SourceClass,
			ChoiceID:         "wizard-cantrips",
			CantripSelection: []string{"fire-bolt", "mage-hand", "prestidigitation"},
		},
		{
			Category:       shared.ChoiceSpells,
			Source:         shared.SourceClass,
			ChoiceID:       "wizard-spells-level-1",
			SpellSelection: []string{"magic-missile", "shield", "identify", "detect-magic", "sleep", "burning-hands"},
		},
	}

	errors, err := ValidateClassChoices(classes.Wizard, choices)
	s.Require().NoError(err)
	s.Require().NotEmpty(errors)

	hasSkillError := false
	for _, e := range errors {
		if e.Field == fieldClassChoices {
			s.Assert().Contains(e.Message, "skill proficiencies")
			hasSkillError = true
		}
	}
	s.Assert().True(hasSkillError, "Should have error about missing skills")
}

func (s *ClassValidatorTestSuite) TestValidateWizardChoices_InvalidSkill() {
	choices := []character.ChoiceData{
		{
			Category:       shared.ChoiceSkills,
			Source:         shared.SourceClass,
			ChoiceID:       "wizard-skills",
			SkillSelection: []skills.Skill{skills.Arcana, skills.Athletics}, // Athletics is not valid for wizard
		},
	}

	errors, err := ValidateClassChoices(classes.Wizard, choices)
	s.Require().NoError(err)
	s.Require().NotEmpty(errors)

	hasInvalidSkillError := false
	for _, e := range errors {
		if e.Field == fieldSkills {
			s.Assert().Contains(e.Message, "Invalid wizard skill")
			s.Assert().Contains(e.Message, "athletics")
			hasInvalidSkillError = true
		}
	}
	s.Assert().True(hasInvalidSkillError, "Should have error about invalid skill")
}

func (s *ClassValidatorTestSuite) TestValidateWizardChoices_InsufficientCantrips() {
	choices := []character.ChoiceData{
		{
			Category:       shared.ChoiceSkills,
			Source:         shared.SourceClass,
			ChoiceID:       "wizard-skills",
			SkillSelection: []skills.Skill{skills.Arcana, skills.History},
		},
		{
			Category:         shared.ChoiceCantrips,
			Source:           shared.SourceClass,
			ChoiceID:         "wizard-cantrips",
			CantripSelection: []string{"fire-bolt", "mage-hand"}, // Only 2, needs 3
		},
		{
			Category:       shared.ChoiceSpells,
			Source:         shared.SourceClass,
			ChoiceID:       "wizard-spells-level-1",
			SpellSelection: []string{"magic-missile", "shield", "identify", "detect-magic", "sleep", "burning-hands"},
		},
	}

	errors, err := ValidateClassChoices(classes.Wizard, choices)
	s.Require().NoError(err)
	s.Require().NotEmpty(errors)

	hasCantripError := false
	for _, e := range errors {
		if e.Field == "cantrips" {
			s.Assert().Contains(e.Message, "requires 3 cantrips")
			s.Assert().Contains(e.Message, "only 2 selected")
			hasCantripError = true
		}
	}
	s.Assert().True(hasCantripError, "Should have error about insufficient cantrips")
}

func (s *ClassValidatorTestSuite) TestValidateWizardChoices_InsufficientSpells() {
	choices := []character.ChoiceData{
		{
			Category:       shared.ChoiceSkills,
			Source:         shared.SourceClass,
			ChoiceID:       "wizard-skills",
			SkillSelection: []skills.Skill{skills.Arcana, skills.History},
		},
		{
			Category:         shared.ChoiceCantrips,
			Source:           shared.SourceClass,
			ChoiceID:         "wizard-cantrips",
			CantripSelection: []string{"fire-bolt", "mage-hand", "prestidigitation"},
		},
		{
			Category:       shared.ChoiceSpells,
			Source:         shared.SourceClass,
			ChoiceID:       "wizard-spells-level-1",
			SpellSelection: []string{"magic-missile", "shield"}, // Only 2, needs 6
		},
	}

	errors, err := ValidateClassChoices(classes.Wizard, choices)
	s.Require().NoError(err)
	s.Require().NotEmpty(errors)

	hasSpellError := false
	for _, e := range errors {
		if e.Field == "spells" {
			s.Assert().Contains(e.Message, "Wizard spells for spellbook")
			s.Assert().Contains(e.Message, "only 2 selected")
			hasSpellError = true
		}
	}
	s.Assert().True(hasSpellError, "Should have error about insufficient spells")
}

func (s *ClassValidatorTestSuite) TestValidateWizardChoices_MissingCantrips() {
	choices := []character.ChoiceData{
		{
			Category:       shared.ChoiceSkills,
			Source:         shared.SourceClass,
			ChoiceID:       "wizard-skills",
			SkillSelection: []skills.Skill{skills.Arcana, skills.History},
		},
		{
			Category:       shared.ChoiceSpells,
			Source:         shared.SourceClass,
			ChoiceID:       "wizard-spells-level-1",
			SpellSelection: []string{"magic-missile", "shield", "identify", "detect-magic", "sleep", "burning-hands"},
		},
	}

	errors, err := ValidateClassChoices(classes.Wizard, choices)
	s.Require().NoError(err)
	s.Require().NotEmpty(errors)

	hasCantripError := false
	for _, e := range errors {
		if e.Field == fieldClassChoices {
			s.Assert().Contains(e.Message, "cantrips")
			hasCantripError = true
		}
	}
	s.Assert().True(hasCantripError, "Should have error about missing cantrips")
}

func (s *ClassValidatorTestSuite) TestValidateWizardChoices_MissingSpells() {
	choices := []character.ChoiceData{
		{
			Category:       shared.ChoiceSkills,
			Source:         shared.SourceClass,
			ChoiceID:       "wizard-skills",
			SkillSelection: []skills.Skill{skills.Arcana, skills.History},
		},
		{
			Category:         shared.ChoiceCantrips,
			Source:           shared.SourceClass,
			ChoiceID:         "wizard-cantrips",
			CantripSelection: []string{"fire-bolt", "mage-hand", "prestidigitation"},
		},
	}

	errors, err := ValidateClassChoices(classes.Wizard, choices)
	s.Require().NoError(err)
	s.Require().NotEmpty(errors)

	hasSpellError := false
	for _, e := range errors {
		if e.Field == fieldClassChoices {
			s.Assert().Contains(e.Message, "spells for spellbook")
			hasSpellError = true
		}
	}
	s.Assert().True(hasSpellError, "Should have error about missing spells")
}

func (s *ClassValidatorTestSuite) TestValidateWizardChoices_MissingEquipment() {
	choices := []character.ChoiceData{
		{
			Category:       shared.ChoiceSkills,
			Source:         shared.SourceClass,
			ChoiceID:       "wizard-skills",
			SkillSelection: []skills.Skill{skills.Arcana, skills.History},
		},
		{
			Category:         shared.ChoiceCantrips,
			Source:           shared.SourceClass,
			ChoiceID:         "wizard-cantrips",
			CantripSelection: []string{"fire-bolt", "mage-hand", "prestidigitation"},
		},
		{
			Category:       shared.ChoiceSpells,
			Source:         shared.SourceClass,
			ChoiceID:       "wizard-spells-level-1",
			SpellSelection: []string{"magic-missile", "shield", "identify", "detect-magic", "sleep", "burning-hands"},
		},
		// Missing all equipment choices
	}

	errors, err := ValidateClassChoices(classes.Wizard, choices)
	s.Require().NoError(err)
	s.Require().NotEmpty(errors)

	hasEquipmentError := false
	for _, e := range errors {
		if e.Field == fieldClassChoices {
			// Should mention missing equipment
			s.Assert().Contains(e.Message, "weapon choice")
			hasEquipmentError = true
		}
	}
	s.Assert().True(hasEquipmentError, "Should have error about missing equipment")
}

func (s *ClassValidatorTestSuite) TestValidateWizardChoices_EmptyEquipmentSelection() {
	choices := []character.ChoiceData{
		{
			Category:       shared.ChoiceSkills,
			Source:         shared.SourceClass,
			ChoiceID:       "wizard-skills",
			SkillSelection: []skills.Skill{skills.Arcana, skills.History},
		},
		{
			Category:         shared.ChoiceCantrips,
			Source:           shared.SourceClass,
			ChoiceID:         "wizard-cantrips",
			CantripSelection: []string{"fire-bolt", "mage-hand", "prestidigitation"},
		},
		{
			Category:       shared.ChoiceSpells,
			Source:         shared.SourceClass,
			ChoiceID:       "wizard-spells-level-1",
			SpellSelection: []string{"magic-missile", "shield", "identify", "detect-magic", "sleep", "burning-hands"},
		},
		{
			Category:           shared.ChoiceEquipment,
			Source:             shared.SourceClass,
			ChoiceID:           "wizard-equipment-primary-weapon",
			EquipmentSelection: []string{}, // Empty selection
		},
	}

	errors, err := ValidateClassChoices(classes.Wizard, choices)
	s.Require().NoError(err)
	s.Require().NotEmpty(errors)

	hasEquipmentError := false
	for _, e := range errors {
		if e.Field == "wizard-equipment-primary-weapon" {
			s.Assert().Contains(e.Message, "No selection made for weapon choice")
			hasEquipmentError = true
		}
	}
	s.Assert().True(hasEquipmentError, "Should have error about empty equipment selection")
}

// Test non-class choices are ignored
func (s *ClassValidatorTestSuite) TestValidateWizardChoices_IgnoresNonClassChoices() {
	choices := []character.ChoiceData{
		// Class choices
		{
			Category:       shared.ChoiceSkills,
			Source:         shared.SourceClass,
			ChoiceID:       "wizard-skills",
			SkillSelection: []skills.Skill{skills.Arcana, skills.Investigation},
		},
		{
			Category:         shared.ChoiceCantrips,
			Source:           shared.SourceClass,
			ChoiceID:         "wizard-cantrips",
			CantripSelection: []string{"fire-bolt", "mage-hand", "prestidigitation"},
		},
		{
			Category:       shared.ChoiceSpells,
			Source:         shared.SourceClass,
			ChoiceID:       "wizard-spells-level-1",
			SpellSelection: []string{"magic-missile", "shield", "identify", "detect-magic", "sleep", "burning-hands"},
		},
		{
			Category:           shared.ChoiceEquipment,
			Source:             shared.SourceClass,
			ChoiceID:           "wizard-equipment-primary-weapon",
			EquipmentSelection: []string{"quarterstaff"},
		},
		{
			Category:           shared.ChoiceEquipment,
			Source:             shared.SourceClass,
			ChoiceID:           "wizard-equipment-focus",
			EquipmentSelection: []string{"component-pouch"},
		},
		{
			Category:           shared.ChoiceEquipment,
			Source:             shared.SourceClass,
			ChoiceID:           "wizard-equipment-pack",
			EquipmentSelection: []string{"scholars-pack"},
		},
		// Non-class choices that should be ignored
		{
			Category:       shared.ChoiceSkills,
			Source:         shared.SourceBackground,
			ChoiceID:       "background-skills",
			SkillSelection: []skills.Skill{skills.Stealth}, // Even though stealth is not a wizard skill
		},
		{
			Category:          shared.ChoiceLanguages,
			Source:            shared.SourceRace,
			ChoiceID:          "race-languages",
			LanguageSelection: []languages.Language{languages.Elvish},
		},
	}

	errors, err := ValidateClassChoices(classes.Wizard, choices)
	s.Require().NoError(err)
	s.Assert().Empty(errors, "Should not have errors when non-class choices are present")
}

// Test unknown class returns no errors
func (s *ClassValidatorTestSuite) TestValidateClassChoices_UnknownClass() {
	choices := []character.ChoiceData{}

	// Test with a class that doesn't have validation yet
	errors, err := ValidateClassChoices(classes.Rogue, choices)
	s.Require().NoError(err)
	s.Assert().Nil(errors, "Unknown class should return nil errors")
}

// Sorcerer validation tests
func (s *ClassValidatorTestSuite) TestValidateSorcererChoices_Valid() {
	choices := []character.ChoiceData{
		{
			Category:       shared.ChoiceSkills,
			Source:         shared.SourceClass,
			ChoiceID:       "sorcerer-skills",
			SkillSelection: []skills.Skill{skills.Arcana, skills.Persuasion},
		},
		{
			Category:         shared.ChoiceCantrips,
			Source:           shared.SourceClass,
			ChoiceID:         "sorcerer-cantrips",
			CantripSelection: []string{"fire-bolt", "mage-hand", "prestidigitation", "ray-of-frost"},
		},
		{
			Category:       shared.ChoiceSpells,
			Source:         shared.SourceClass,
			ChoiceID:       "sorcerer-spells-level-1",
			SpellSelection: []string{"magic-missile", "shield"},
		},
		{
			Category:           shared.ChoiceEquipment,
			Source:             shared.SourceClass,
			ChoiceID:           "sorcerer-equipment-primary-weapon",
			EquipmentSelection: []string{"light-crossbow", "bolts-20"},
		},
		{
			Category:           shared.ChoiceEquipment,
			Source:             shared.SourceClass,
			ChoiceID:           "sorcerer-equipment-focus",
			EquipmentSelection: []string{"component-pouch"},
		},
		{
			Category:           shared.ChoiceEquipment,
			Source:             shared.SourceClass,
			ChoiceID:           "sorcerer-equipment-pack",
			EquipmentSelection: []string{"dungeoneers-pack"},
		},
	}

	errors, err := ValidateClassChoices(classes.Sorcerer, choices)
	s.Require().NoError(err)
	s.Assert().Empty(errors)
}

func (s *ClassValidatorTestSuite) TestValidateSorcererChoices_InvalidSkill() {
	choices := []character.ChoiceData{
		{
			Category:       shared.ChoiceSkills,
			Source:         shared.SourceClass,
			ChoiceID:       "sorcerer-skills",
			SkillSelection: []skills.Skill{skills.Arcana, skills.Athletics}, // Athletics not valid for sorcerer
		},
	}

	errors, err := ValidateClassChoices(classes.Sorcerer, choices)
	s.Require().NoError(err)
	s.Require().NotEmpty(errors)

	hasInvalidSkillError := false
	for _, e := range errors {
		if e.Field == fieldSkills {
			s.Assert().Contains(e.Message, "Invalid sorcerer skill")
			s.Assert().Contains(e.Message, "athletics")
			hasInvalidSkillError = true
		}
	}
	s.Assert().True(hasInvalidSkillError, "Should have error about invalid skill")
}

func (s *ClassValidatorTestSuite) TestValidateSorcererChoices_InsufficientCantrips() {
	choices := []character.ChoiceData{
		{
			Category:       shared.ChoiceSkills,
			Source:         shared.SourceClass,
			ChoiceID:       "sorcerer-skills",
			SkillSelection: []skills.Skill{skills.Arcana, skills.Deception},
		},
		{
			Category:         shared.ChoiceCantrips,
			Source:           shared.SourceClass,
			ChoiceID:         "sorcerer-cantrips",
			CantripSelection: []string{"fire-bolt", "mage-hand"}, // Only 2, needs 4
		},
		{
			Category:       shared.ChoiceSpells,
			Source:         shared.SourceClass,
			ChoiceID:       "sorcerer-spells-level-1",
			SpellSelection: []string{"magic-missile", "shield"},
		},
	}

	errors, err := ValidateClassChoices(classes.Sorcerer, choices)
	s.Require().NoError(err)
	s.Require().NotEmpty(errors)

	hasCantripError := false
	for _, e := range errors {
		if e.Field == "cantrips" {
			s.Assert().Contains(e.Message, "requires 4 cantrips")
			s.Assert().Contains(e.Message, "only 2 selected")
			hasCantripError = true
		}
	}
	s.Assert().True(hasCantripError, "Should have error about insufficient cantrips")
}

// Warlock validation tests
func (s *ClassValidatorTestSuite) TestValidateWarlockChoices_Valid() {
	choices := []character.ChoiceData{
		{
			Category:       shared.ChoiceSkills,
			Source:         shared.SourceClass,
			ChoiceID:       "warlock-skills",
			SkillSelection: []skills.Skill{skills.Arcana, skills.Investigation},
		},
		{
			Category:         shared.ChoiceCantrips,
			Source:           shared.SourceClass,
			ChoiceID:         "warlock-cantrips",
			CantripSelection: []string{"eldritch-blast", "minor-illusion"},
		},
		{
			Category:       shared.ChoiceSpells,
			Source:         shared.SourceClass,
			ChoiceID:       "warlock-spells-level-1",
			SpellSelection: []string{"hex", "armor-of-agathys"},
		},
		{
			Category:           shared.ChoiceEquipment,
			Source:             shared.SourceClass,
			ChoiceID:           "warlock-equipment-primary-weapon",
			EquipmentSelection: []string{"light-crossbow", "bolts-20"},
		},
		{
			Category:           shared.ChoiceEquipment,
			Source:             shared.SourceClass,
			ChoiceID:           "warlock-equipment-focus",
			EquipmentSelection: []string{"arcane-focus"},
		},
		{
			Category:           shared.ChoiceEquipment,
			Source:             shared.SourceClass,
			ChoiceID:           "warlock-equipment-pack",
			EquipmentSelection: []string{"scholars-pack"},
		},
	}

	errors, err := ValidateClassChoices(classes.Warlock, choices)
	s.Require().NoError(err)
	s.Assert().Empty(errors)
}

func (s *ClassValidatorTestSuite) TestValidateWarlockChoices_InvalidSkill() {
	choices := []character.ChoiceData{
		{
			Category:       shared.ChoiceSkills,
			Source:         shared.SourceClass,
			ChoiceID:       "warlock-skills",
			SkillSelection: []skills.Skill{skills.Arcana, skills.Athletics}, // Athletics not valid for warlock
		},
	}

	errors, err := ValidateClassChoices(classes.Warlock, choices)
	s.Require().NoError(err)
	s.Require().NotEmpty(errors)

	hasInvalidSkillError := false
	for _, e := range errors {
		if e.Field == fieldSkills {
			s.Assert().Contains(e.Message, "Invalid warlock skill")
			s.Assert().Contains(e.Message, "athletics")
			hasInvalidSkillError = true
		}
	}
	s.Assert().True(hasInvalidSkillError, "Should have error about invalid skill")
}

func (s *ClassValidatorTestSuite) TestValidateWarlockChoices_InsufficientSpells() {
	choices := []character.ChoiceData{
		{
			Category:       shared.ChoiceSkills,
			Source:         shared.SourceClass,
			ChoiceID:       "warlock-skills",
			SkillSelection: []skills.Skill{skills.Deception, skills.Intimidation},
		},
		{
			Category:         shared.ChoiceCantrips,
			Source:           shared.SourceClass,
			ChoiceID:         "warlock-cantrips",
			CantripSelection: []string{"eldritch-blast", "minor-illusion"},
		},
		{
			Category:       shared.ChoiceSpells,
			Source:         shared.SourceClass,
			ChoiceID:       "warlock-spells-level-1",
			SpellSelection: []string{"hex"}, // Only 1, needs 2
		},
	}

	errors, err := ValidateClassChoices(classes.Warlock, choices)
	s.Require().NoError(err)
	s.Require().NotEmpty(errors)

	hasSpellError := false
	for _, e := range errors {
		if e.Field == "spells" {
			s.Assert().Contains(e.Message, "Warlock spells known")
			s.Assert().Contains(e.Message, "only 1 selected")
			hasSpellError = true
		}
	}
	s.Assert().True(hasSpellError, "Should have error about insufficient spells")
}
