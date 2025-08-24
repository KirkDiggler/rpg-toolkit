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
		if e.Field == fieldCantrips {
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

	// Test with a class that doesn't have validation yet (artificer doesn't exist in constants)
	errors, err := ValidateClassChoices("artificer", choices)
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
		if e.Field == fieldCantrips {
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

func (s *ClassValidatorTestSuite) TestValidateClericChoices_Valid() {
	choices := []character.ChoiceData{
		{
			Source:         shared.SourceClass,
			Category:       shared.ChoiceSkills,
			SkillSelection: []skills.Skill{skills.History, skills.Medicine},
		},
		{
			Source:           shared.SourceClass,
			Category:         shared.ChoiceCantrips,
			CantripSelection: []string{"sacred-flame", "guidance", "thaumaturgy"},
		},
		{
			Source:             shared.SourceClass,
			Category:           shared.ChoiceEquipment,
			ChoiceID:           "cleric-equipment-primary-weapon",
			EquipmentSelection: []string{"mace"},
		},
		{
			Source:             shared.SourceClass,
			Category:           shared.ChoiceEquipment,
			ChoiceID:           "cleric-equipment-armor",
			EquipmentSelection: []string{"scale-mail"},
		},
		{
			Source:             shared.SourceClass,
			Category:           shared.ChoiceEquipment,
			ChoiceID:           "cleric-equipment-ranged",
			EquipmentSelection: []string{"crossbow-light"},
		},
		{
			Source:             shared.SourceClass,
			Category:           shared.ChoiceEquipment,
			ChoiceID:           "cleric-equipment-pack",
			EquipmentSelection: []string{"priests-pack"},
		},
		{
			Source:             shared.SourceClass,
			Category:           shared.ChoiceEquipment,
			ChoiceID:           "cleric-equipment-holy-symbol",
			EquipmentSelection: []string{"amulet"},
		},
	}

	errors, err := ValidateClassChoices(classes.Cleric, choices)
	s.Require().NoError(err)
	s.Assert().Empty(errors)
}

func (s *ClassValidatorTestSuite) TestValidateClericChoices_InvalidSkill() {
	choices := []character.ChoiceData{
		{
			Source:         shared.SourceClass,
			Category:       shared.ChoiceSkills,
			SkillSelection: []skills.Skill{skills.Athletics, skills.Medicine}, // Athletics not valid for Cleric
		},
		{
			Source:           shared.SourceClass,
			Category:         shared.ChoiceCantrips,
			CantripSelection: []string{"sacred-flame", "guidance", "thaumaturgy"},
		},
	}

	errors, err := ValidateClassChoices(classes.Cleric, choices)
	s.Require().NoError(err)
	s.Require().NotEmpty(errors)

	hasInvalidSkillError := false
	for _, e := range errors {
		if e.Field == fieldSkills {
			s.Assert().Contains(e.Message, "Invalid cleric skill: athletics")
			s.Assert().Contains(e.Message, "Must choose from")
			hasInvalidSkillError = true
		}
	}
	s.Assert().True(hasInvalidSkillError, "Should have error about invalid skill")
}

func (s *ClassValidatorTestSuite) TestValidateClericChoices_InsufficientCantrips() {
	choices := []character.ChoiceData{
		{
			Source:         shared.SourceClass,
			Category:       shared.ChoiceSkills,
			SkillSelection: []skills.Skill{skills.History, skills.Medicine},
		},
		{
			Source:           shared.SourceClass,
			Category:         shared.ChoiceCantrips,
			CantripSelection: []string{"sacred-flame", "guidance"}, // Only 2, need 3
		},
	}

	errors, err := ValidateClassChoices(classes.Cleric, choices)
	s.Require().NoError(err)
	s.Require().NotEmpty(errors)

	hasCantripError := false
	for _, e := range errors {
		if e.Field == fieldCantrips {
			s.Assert().Contains(e.Message, "Cleric requires 3 cantrips at level 1, only 2 selected")
			hasCantripError = true
		}
	}
	s.Assert().True(hasCantripError, "Should have error about insufficient cantrips")
}

func (s *ClassValidatorTestSuite) TestValidateDruidChoices_Valid() {
	choices := []character.ChoiceData{
		{
			Source:         shared.SourceClass,
			Category:       shared.ChoiceSkills,
			SkillSelection: []skills.Skill{skills.Nature, skills.Perception},
		},
		{
			Source:           shared.SourceClass,
			Category:         shared.ChoiceCantrips,
			CantripSelection: []string{"druidcraft", "guidance"},
		},
		{
			Source:             shared.SourceClass,
			Category:           shared.ChoiceEquipment,
			ChoiceID:           "druid-equipment-shield-weapon",
			EquipmentSelection: []string{"shield"},
		},
		{
			Source:             shared.SourceClass,
			Category:           shared.ChoiceEquipment,
			ChoiceID:           "druid-equipment-melee",
			EquipmentSelection: []string{"scimitar"},
		},
		{
			Source:             shared.SourceClass,
			Category:           shared.ChoiceEquipment,
			ChoiceID:           "druid-equipment-focus",
			EquipmentSelection: []string{"druidcraft-focus"},
		},
	}

	errors, err := ValidateClassChoices(classes.Druid, choices)
	s.Require().NoError(err)
	s.Assert().Empty(errors)
}

func (s *ClassValidatorTestSuite) TestValidateDruidChoices_InvalidSkill() {
	choices := []character.ChoiceData{
		{
			Source:         shared.SourceClass,
			Category:       shared.ChoiceSkills,
			SkillSelection: []skills.Skill{skills.Deception, skills.Perception}, // Deception not valid for Druid
		},
		{
			Source:           shared.SourceClass,
			Category:         shared.ChoiceCantrips,
			CantripSelection: []string{"druidcraft", "guidance"},
		},
	}

	errors, err := ValidateClassChoices(classes.Druid, choices)
	s.Require().NoError(err)
	s.Require().NotEmpty(errors)

	hasInvalidSkillError := false
	for _, e := range errors {
		if e.Field == fieldSkills {
			s.Assert().Contains(e.Message, "Invalid druid skill: deception")
			s.Assert().Contains(e.Message, "Must choose from")
			hasInvalidSkillError = true
		}
	}
	s.Assert().True(hasInvalidSkillError, "Should have error about invalid skill")
}

func (s *ClassValidatorTestSuite) TestValidateDruidChoices_InsufficientCantrips() {
	choices := []character.ChoiceData{
		{
			Source:         shared.SourceClass,
			Category:       shared.ChoiceSkills,
			SkillSelection: []skills.Skill{skills.Nature, skills.Perception},
		},
		{
			Source:           shared.SourceClass,
			Category:         shared.ChoiceCantrips,
			CantripSelection: []string{"druidcraft"}, // Only 1, need 2
		},
	}

	errors, err := ValidateClassChoices(classes.Druid, choices)
	s.Require().NoError(err)
	s.Require().NotEmpty(errors)

	hasCantripError := false
	for _, e := range errors {
		if e.Field == fieldCantrips {
			s.Assert().Contains(e.Message, "Druid requires 2 cantrips at level 1, only 1 selected")
			hasCantripError = true
		}
	}
	s.Assert().True(hasCantripError, "Should have error about insufficient cantrips")
}

func (s *ClassValidatorTestSuite) TestValidateRogueChoices_Valid() {
	choices := []character.ChoiceData{
		{
			Source:         shared.SourceClass,
			Category:       shared.ChoiceSkills,
			SkillSelection: []skills.Skill{skills.Stealth, skills.Acrobatics, skills.Deception, skills.Perception},
		},
		{
			Source:             shared.SourceClass,
			Category:           shared.ChoiceExpertise,
			ExpertiseSelection: []string{"stealth", "perception"},
		},
		{
			Source:             shared.SourceClass,
			Category:           shared.ChoiceEquipment,
			ChoiceID:           "rogue-equipment-primary-weapon",
			EquipmentSelection: []string{"rapier"},
		},
		{
			Source:             shared.SourceClass,
			Category:           shared.ChoiceEquipment,
			ChoiceID:           "rogue-equipment-secondary",
			EquipmentSelection: []string{"shortbow"},
		},
		{
			Source:             shared.SourceClass,
			Category:           shared.ChoiceEquipment,
			ChoiceID:           "rogue-equipment-pack",
			EquipmentSelection: []string{"burglars-pack"},
		},
	}

	errors, err := ValidateClassChoices(classes.Rogue, choices)
	s.Require().NoError(err)
	s.Assert().Empty(errors)
}

func (s *ClassValidatorTestSuite) TestValidateRogueChoices_InvalidSkill() {
	choices := []character.ChoiceData{
		{
			Source:   shared.SourceClass,
			Category: shared.ChoiceSkills,
			// History is not a valid rogue skill
			SkillSelection: []skills.Skill{skills.Stealth, skills.Acrobatics, skills.Deception, skills.History},
		},
		{
			Source:             shared.SourceClass,
			Category:           shared.ChoiceExpertise,
			ExpertiseSelection: []string{"stealth", "perception"},
		},
	}

	errors, err := ValidateClassChoices(classes.Rogue, choices)
	s.Require().NoError(err)
	s.Require().NotEmpty(errors)

	hasInvalidSkillError := false
	for _, e := range errors {
		if e.Field == fieldSkills {
			s.Assert().Contains(e.Message, "Invalid rogue skill: history")
			s.Assert().Contains(e.Message, "Must choose from")
			hasInvalidSkillError = true
		}
	}
	s.Assert().True(hasInvalidSkillError, "Should have error about invalid skill")
}

func (s *ClassValidatorTestSuite) TestValidateRogueChoices_InsufficientSkills() {
	choices := []character.ChoiceData{
		{
			Source:         shared.SourceClass,
			Category:       shared.ChoiceSkills,
			SkillSelection: []skills.Skill{skills.Stealth, skills.Acrobatics}, // Only 2, needs 4
		},
		{
			Source:             shared.SourceClass,
			Category:           shared.ChoiceExpertise,
			ExpertiseSelection: []string{"stealth", "acrobatics"},
		},
	}

	errors, err := ValidateClassChoices(classes.Rogue, choices)
	s.Require().NoError(err)
	s.Require().NotEmpty(errors)

	hasSkillError := false
	for _, e := range errors {
		if e.Field == fieldSkills {
			s.Assert().Contains(e.Message, "Rogue requires 4 skill proficiencies, only 2 selected")
			hasSkillError = true
		}
	}
	s.Assert().True(hasSkillError, "Should have error about insufficient skills")
}

func (s *ClassValidatorTestSuite) TestValidateRogueChoices_InsufficientExpertise() {
	choices := []character.ChoiceData{
		{
			Source:         shared.SourceClass,
			Category:       shared.ChoiceSkills,
			SkillSelection: []skills.Skill{skills.Stealth, skills.Acrobatics, skills.Deception, skills.Perception},
		},
		{
			Source:             shared.SourceClass,
			Category:           shared.ChoiceExpertise,
			ExpertiseSelection: []string{"stealth"}, // Only 1, needs 2
		},
	}

	errors, err := ValidateClassChoices(classes.Rogue, choices)
	s.Require().NoError(err)
	s.Require().NotEmpty(errors)

	hasExpertiseError := false
	for _, e := range errors {
		if e.Field == fieldExpertise {
			s.Assert().Contains(e.Message, "Rogue requires 2 expertise choices at level 1, only 1 selected")
			hasExpertiseError = true
		}
	}
	s.Assert().True(hasExpertiseError, "Should have error about insufficient expertise")
}

func (s *ClassValidatorTestSuite) TestValidateRogueChoices_MissingExpertise() {
	choices := []character.ChoiceData{
		{
			Source:         shared.SourceClass,
			Category:       shared.ChoiceSkills,
			SkillSelection: []skills.Skill{skills.Stealth, skills.Acrobatics, skills.Deception, skills.Perception},
		},
		// No expertise choice at all
	}

	errors, err := ValidateClassChoices(classes.Rogue, choices)
	s.Require().NoError(err)
	s.Require().NotEmpty(errors)

	hasMissingError := false
	for _, e := range errors {
		if e.Field == "class_choices" {
			s.Assert().Contains(e.Message, "Missing required choices")
			s.Assert().Contains(e.Message, "expertise")
			hasMissingError = true
		}
	}
	s.Assert().True(hasMissingError, "Should have error about missing expertise")
}

// Barbarian validation tests
func (s *ClassValidatorTestSuite) TestValidateBarbarianChoices_Valid() {
	choices := []character.ChoiceData{
		{
			Category:       shared.ChoiceSkills,
			Source:         shared.SourceClass,
			ChoiceID:       "barbarian-skills",
			SkillSelection: []skills.Skill{skills.Athletics, skills.Survival},
		},
		{
			Category:           shared.ChoiceEquipment,
			Source:             shared.SourceClass,
			ChoiceID:           "barbarian-weapon-choice",
			EquipmentSelection: []string{"greataxe"},
		},
	}

	errors, err := ValidateClassChoices(classes.Barbarian, choices)
	s.Require().NoError(err)
	s.Assert().Empty(errors)
}

func (s *ClassValidatorTestSuite) TestValidateBarbarianChoices_InvalidSkill() {
	choices := []character.ChoiceData{
		{
			Category:       shared.ChoiceSkills,
			Source:         shared.SourceClass,
			ChoiceID:       "barbarian-skills",
			SkillSelection: []skills.Skill{skills.Athletics, skills.Arcana}, // Arcana is invalid
		},
		{
			Category:           shared.ChoiceEquipment,
			Source:             shared.SourceClass,
			ChoiceID:           "barbarian-weapon-choice",
			EquipmentSelection: []string{"greataxe"},
		},
	}

	errors, err := ValidateClassChoices(classes.Barbarian, choices)
	s.Require().NoError(err)
	s.Require().NotEmpty(errors)

	hasInvalidSkillError := false
	for _, e := range errors {
		if e.Field == fieldSkills {
			s.Assert().Contains(e.Message, "Invalid barbarian skill: arcana")
			s.Assert().Contains(e.Message, "Must choose from")
			hasInvalidSkillError = true
		}
	}
	s.Assert().True(hasInvalidSkillError, "Should have error about invalid skill")
}

// Monk validation tests
func (s *ClassValidatorTestSuite) TestValidateMonkChoices_Valid() {
	choices := []character.ChoiceData{
		{
			Category:       shared.ChoiceSkills,
			Source:         shared.SourceClass,
			ChoiceID:       "monk-skills",
			SkillSelection: []skills.Skill{skills.Acrobatics, skills.Stealth},
		},
		{
			Category:           shared.ChoiceEquipment,
			Source:             shared.SourceClass,
			ChoiceID:           "monk-weapon-choice",
			EquipmentSelection: []string{"shortsword"},
		},
	}

	errors, err := ValidateClassChoices(classes.Monk, choices)
	s.Require().NoError(err)
	s.Assert().Empty(errors)
}

func (s *ClassValidatorTestSuite) TestValidateMonkChoices_TooFewSkills() {
	choices := []character.ChoiceData{
		{
			Category:       shared.ChoiceSkills,
			Source:         shared.SourceClass,
			ChoiceID:       "monk-skills",
			SkillSelection: []skills.Skill{skills.Acrobatics}, // Only 1, needs 2
		},
		{
			Category:           shared.ChoiceEquipment,
			Source:             shared.SourceClass,
			ChoiceID:           "monk-weapon-choice",
			EquipmentSelection: []string{"shortsword"},
		},
	}

	errors, err := ValidateClassChoices(classes.Monk, choices)
	s.Require().NoError(err)
	s.Require().NotEmpty(errors)

	hasSkillCountError := false
	for _, e := range errors {
		if e.Field == fieldSkills && e.Code == rpgerr.CodeInvalidArgument {
			s.Assert().Contains(e.Message, "Monk requires 2 skill proficiencies")
			hasSkillCountError = true
		}
	}
	s.Assert().True(hasSkillCountError, "Should have error about skill count")
}

// Ranger validation tests
func (s *ClassValidatorTestSuite) TestValidateRangerChoices_Valid() {
	choices := []character.ChoiceData{
		{
			Category:       shared.ChoiceSkills,
			Source:         shared.SourceClass,
			ChoiceID:       "ranger-skills",
			SkillSelection: []skills.Skill{skills.AnimalHandling, skills.Survival, skills.Perception},
		},
		{
			Category:           shared.ChoiceEquipment,
			Source:             shared.SourceClass,
			ChoiceID:           "ranger-armor-choice",
			EquipmentSelection: []string{"leather-armor"},
		},
	}

	errors, err := ValidateClassChoices(classes.Ranger, choices)
	s.Require().NoError(err)
	s.Assert().Empty(errors)
}

func (s *ClassValidatorTestSuite) TestValidateRangerChoices_InvalidSkillCount() {
	choices := []character.ChoiceData{
		{
			Category:       shared.ChoiceSkills,
			Source:         shared.SourceClass,
			ChoiceID:       "ranger-skills",
			SkillSelection: []skills.Skill{skills.AnimalHandling, skills.Survival}, // Only 2, needs 3
		},
		{
			Category:           shared.ChoiceEquipment,
			Source:             shared.SourceClass,
			ChoiceID:           "ranger-armor-choice",
			EquipmentSelection: []string{"leather-armor"},
		},
	}

	errors, err := ValidateClassChoices(classes.Ranger, choices)
	s.Require().NoError(err)
	s.Require().NotEmpty(errors)

	hasSkillCountError := false
	for _, e := range errors {
		if e.Field == fieldSkills {
			s.Assert().Contains(e.Message, "Ranger requires 3 skill proficiencies")
			hasSkillCountError = true
		}
	}
	s.Assert().True(hasSkillCountError, "Should have error about skill count")
}

// Paladin validation tests
func (s *ClassValidatorTestSuite) TestValidatePaladinChoices_Valid() {
	choices := []character.ChoiceData{
		{
			Category:       shared.ChoiceSkills,
			Source:         shared.SourceClass,
			ChoiceID:       "paladin-skills",
			SkillSelection: []skills.Skill{skills.Athletics, skills.Persuasion},
		},
		{
			Category:           shared.ChoiceEquipment,
			Source:             shared.SourceClass,
			ChoiceID:           "paladin-weapon-choice",
			EquipmentSelection: []string{"longsword", "shield"},
		},
	}

	errors, err := ValidateClassChoices(classes.Paladin, choices)
	s.Require().NoError(err)
	s.Assert().Empty(errors)
}

func (s *ClassValidatorTestSuite) TestValidatePaladinChoices_MissingEquipment() {
	choices := []character.ChoiceData{
		{
			Category:       shared.ChoiceSkills,
			Source:         shared.SourceClass,
			ChoiceID:       "paladin-skills",
			SkillSelection: []skills.Skill{skills.Athletics, skills.Persuasion},
		},
		// No equipment choice
	}

	errors, err := ValidateClassChoices(classes.Paladin, choices)
	s.Require().NoError(err)
	s.Require().NotEmpty(errors)

	hasMissingError := false
	for _, e := range errors {
		if e.Field == "choices" {
			s.Assert().Contains(e.Message, "Missing required choices")
			s.Assert().Contains(e.Message, "equipment")
			hasMissingError = true
		}
	}
	s.Assert().True(hasMissingError, "Should have error about missing equipment")
}

// Bard validation tests
func (s *ClassValidatorTestSuite) TestValidateBardChoices_Valid() {
	choices := []character.ChoiceData{
		{
			Category:       shared.ChoiceSkills,
			Source:         shared.SourceClass,
			ChoiceID:       "bard-skills",
			SkillSelection: []skills.Skill{skills.Performance, skills.Deception, skills.Persuasion},
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
			ChoiceID:       "bard-spells-level-1",
			SpellSelection: []string{"charm-person", "healing-word", "thunderwave", "disguise-self"},
		},
		{
			Category:           shared.ChoiceEquipment,
			Source:             shared.SourceClass,
			ChoiceID:           "bard-equipment-primary-weapon",
			EquipmentSelection: []string{"rapier"},
		},
		{
			Category:           shared.ChoiceEquipment,
			Source:             shared.SourceClass,
			ChoiceID:           "bard-equipment-instrument",
			EquipmentSelection: []string{"lute"},
		},
		{
			Category:           shared.ChoiceEquipment,
			Source:             shared.SourceClass,
			ChoiceID:           "bard-equipment-pack",
			EquipmentSelection: []string{"entertainers-pack"},
		},
	}

	errors, err := ValidateClassChoices(classes.Bard, choices)
	s.Require().NoError(err)
	s.Assert().Empty(errors)
}

func (s *ClassValidatorTestSuite) TestValidateBardChoices_AnySkillValid() {
	// Bard can choose ANY 3 skills
	choices := []character.ChoiceData{
		{
			Category:       shared.ChoiceSkills,
			Source:         shared.SourceClass,
			ChoiceID:       "bard-skills",
			// Testing with skills that aren't typical "bard" skills
			SkillSelection: []skills.Skill{skills.Athletics, skills.Survival, skills.Medicine},
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
			ChoiceID:       "bard-spells-level-1",
			SpellSelection: []string{"charm-person", "healing-word", "thunderwave", "disguise-self"},
		},
		{
			Category:           shared.ChoiceEquipment,
			Source:             shared.SourceClass,
			ChoiceID:           "bard-equipment-primary-weapon",
			EquipmentSelection: []string{"rapier"},
		},
	}

	errors, err := ValidateClassChoices(classes.Bard, choices)
	s.Require().NoError(err)
	s.Assert().Empty(errors, "Bard should be able to choose any skills")
}

func (s *ClassValidatorTestSuite) TestValidateBardChoices_InsufficientSkills() {
	choices := []character.ChoiceData{
		{
			Category:       shared.ChoiceSkills,
			Source:         shared.SourceClass,
			ChoiceID:       "bard-skills",
			SkillSelection: []skills.Skill{skills.Performance, skills.Deception}, // Only 2, needs 3
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
			ChoiceID:       "bard-spells-level-1",
			SpellSelection: []string{"charm-person", "healing-word", "thunderwave", "disguise-self"},
		},
	}

	errors, err := ValidateClassChoices(classes.Bard, choices)
	s.Require().NoError(err)
	s.Require().NotEmpty(errors)

	hasSkillError := false
	for _, e := range errors {
		if e.Field == fieldSkills {
			s.Assert().Contains(e.Message, "Bard requires 3 skill proficiencies, only 2 selected")
			hasSkillError = true
		}
	}
	s.Assert().True(hasSkillError, "Should have error about insufficient skills")
}

func (s *ClassValidatorTestSuite) TestValidateBardChoices_InsufficientCantrips() {
	choices := []character.ChoiceData{
		{
			Category:       shared.ChoiceSkills,
			Source:         shared.SourceClass,
			ChoiceID:       "bard-skills",
			SkillSelection: []skills.Skill{skills.Performance, skills.Deception, skills.Persuasion},
		},
		{
			Category:         shared.ChoiceCantrips,
			Source:           shared.SourceClass,
			ChoiceID:         "bard-cantrips",
			CantripSelection: []string{"vicious-mockery"}, // Only 1, needs 2
		},
		{
			Category:       shared.ChoiceSpells,
			Source:         shared.SourceClass,
			ChoiceID:       "bard-spells-level-1",
			SpellSelection: []string{"charm-person", "healing-word", "thunderwave", "disguise-self"},
		},
	}

	errors, err := ValidateClassChoices(classes.Bard, choices)
	s.Require().NoError(err)
	s.Require().NotEmpty(errors)

	hasCantripError := false
	for _, e := range errors {
		if e.Field == fieldCantrips {
			s.Assert().Contains(e.Message, "Bard requires 2 cantrips at level 1, only 1 selected")
			hasCantripError = true
		}
	}
	s.Assert().True(hasCantripError, "Should have error about insufficient cantrips")
}

func (s *ClassValidatorTestSuite) TestValidateBardChoices_InsufficientSpells() {
	choices := []character.ChoiceData{
		{
			Category:       shared.ChoiceSkills,
			Source:         shared.SourceClass,
			ChoiceID:       "bard-skills",
			SkillSelection: []skills.Skill{skills.Performance, skills.Deception, skills.Persuasion},
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
			ChoiceID:       "bard-spells-level-1",
			SpellSelection: []string{"charm-person", "healing-word"}, // Only 2, needs 4
		},
	}

	errors, err := ValidateClassChoices(classes.Bard, choices)
	s.Require().NoError(err)
	s.Require().NotEmpty(errors)

	hasSpellError := false
	for _, e := range errors {
		if e.Field == "spells" {
			s.Assert().Contains(e.Message, "Bard spells known: requires 4 spells at level 1, only 2 selected")
			hasSpellError = true
		}
	}
	s.Assert().True(hasSpellError, "Should have error about insufficient spells")
}

func (s *ClassValidatorTestSuite) TestValidateBardChoices_MissingCantrips() {
	choices := []character.ChoiceData{
		{
			Category:       shared.ChoiceSkills,
			Source:         shared.SourceClass,
			ChoiceID:       "bard-skills",
			SkillSelection: []skills.Skill{skills.Performance, skills.Deception, skills.Persuasion},
		},
		{
			Category:       shared.ChoiceSpells,
			Source:         shared.SourceClass,
			ChoiceID:       "bard-spells-level-1",
			SpellSelection: []string{"charm-person", "healing-word", "thunderwave", "disguise-self"},
		},
		{
			Category:           shared.ChoiceEquipment,
			Source:             shared.SourceClass,
			ChoiceID:           "bard-equipment-primary-weapon",
			EquipmentSelection: []string{"rapier"},
		},
		// Missing cantrips choice
	}

	errors, err := ValidateClassChoices(classes.Bard, choices)
	s.Require().NoError(err)
	s.Require().NotEmpty(errors)

	hasMissingError := false
	for _, e := range errors {
		if e.Field == "class_choices" {
			s.Assert().Contains(e.Message, "Missing required choices")
			s.Assert().Contains(e.Message, "cantrips")
			hasMissingError = true
		}
	}
	s.Assert().True(hasMissingError, "Should have error about missing cantrips")
}

// Test that Bard doesn't require expertise at level 1
func (s *ClassValidatorTestSuite) TestValidateBardChoices_NoExpertiseRequired() {
	// Valid Bard choices without expertise (since Bards get expertise at level 3)
	choices := []character.ChoiceData{
		{
			Category:       shared.ChoiceSkills,
			Source:         shared.SourceClass,
			ChoiceID:       "bard-skills",
			SkillSelection: []skills.Skill{skills.Performance, skills.Deception, skills.Persuasion},
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
			ChoiceID:       "bard-spells-level-1",
			SpellSelection: []string{"charm-person", "healing-word", "thunderwave", "disguise-self"},
		},
		{
			Category:           shared.ChoiceEquipment,
			Source:             shared.SourceClass,
			ChoiceID:           "bard-equipment-primary-weapon",
			EquipmentSelection: []string{"rapier"},
		},
		// No expertise choice - this should be valid for level 1 Bard
	}

	errors, err := ValidateClassChoices(classes.Bard, choices)
	s.Require().NoError(err)
	s.Assert().Empty(errors, "Level 1 Bard should not require expertise")
}
