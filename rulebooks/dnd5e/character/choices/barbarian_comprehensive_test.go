package choices_test

import (
	"testing"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/character/choices"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/classes"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/shared"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/skills"
	"github.com/stretchr/testify/suite"
)

// BarbarianCompleteSuite provides comprehensive testing for Barbarian class
type BarbarianCompleteSuite struct {
	suite.Suite

	validator *choices.Validator
	validBase *choices.Submissions
}

// SetupSuite runs once
func (s *BarbarianCompleteSuite) SetupSuite() {
	s.validator = choices.NewValidator()
}

// SetupTest runs before each test
func (s *BarbarianCompleteSuite) SetupTest() {
	s.validBase = s.createValidBaseSubmissions()
}

// createValidBaseSubmissions creates valid Barbarian submissions
func (s *BarbarianCompleteSuite) createValidBaseSubmissions() *choices.Submissions {
	subs := choices.NewSubmissions()

	// Skills - Barbarian chooses 2 from their list
	subs.Add(choices.Submission{
		Category: shared.ChoiceSkills,
		Source:   shared.SourceClass,
		ChoiceID: choices.BarbarianSkills,
		Values: []shared.SelectionID{
			skills.Athletics,
			skills.Survival,
		},
	})

	// Weapon choice - option a: greataxe
	subs.Add(choices.Submission{
		Category: shared.ChoiceEquipment,
		Source:   shared.SourceClass,
		ChoiceID: choices.BarbarianWeaponsPrimary,
		OptionID: choices.BarbarianWeaponGreataxe,
		Values: []shared.SelectionID{
			choices.BarbarianWeaponGreataxe,
		},
	})

	// Secondary weapon choice - two handaxes
	subs.Add(choices.Submission{
		Category: shared.ChoiceEquipment,
		Source:   shared.SourceClass,
		ChoiceID: choices.BarbarianWeaponsSecondary,
		OptionID: choices.BarbarianSecondaryHandaxes,
		Values: []shared.SelectionID{
			choices.BarbarianSecondaryHandaxes,
		},
	})

	// Pack choice - explorer's pack
	subs.Add(choices.Submission{
		Category: shared.ChoiceEquipment,
		Source:   shared.SourceClass,
		ChoiceID: choices.BarbarianPack,
		OptionID: choices.BarbarianPackExplorer,
		Values: []shared.SelectionID{
			choices.BarbarianPackExplorer,
		},
	})

	return subs
}

// TestBasicBarbarianValidation tests basic Barbarian requirements
func (s *BarbarianCompleteSuite) TestBasicBarbarianValidation() {
	reqs := choices.GetClassRequirementsAtLevel(classes.Barbarian, 1)
	s.Require().NotNil(reqs)

	result := s.validator.Validate(reqs, s.validBase)
	s.Assert().True(result.Valid, "Base submissions should be valid")
	s.Assert().Empty(result.Errors)
}

// TestInvalidSubmissions tests various invalid scenarios
func (s *BarbarianCompleteSuite) TestInvalidSubmissions() {
	reqs := choices.GetClassRequirementsAtLevel(classes.Barbarian, 1)

	s.Run("MissingSkills", func() {
		subs := choices.NewSubmissions()
		for _, sub := range s.validBase.Choices {
			if sub.Category != shared.ChoiceSkills {
				subs.Add(sub)
			}
		}

		result := s.validator.Validate(reqs, subs)
		s.Assert().False(result.Valid, "Should fail without skills")
	})

	s.Run("WrongNumberOfSkills", func() {
		subs := choices.NewSubmissions()
		for _, sub := range s.validBase.Choices {
			if sub.Category == shared.ChoiceSkills {
				// Only choose 1 instead of 2
				sub.Values = []shared.SelectionID{
					skills.Athletics,
				}
			}
			subs.Add(sub)
		}

		result := s.validator.Validate(reqs, subs)
		s.Assert().False(result.Valid, "Should fail with wrong number of skills")
	})

	s.Run("InvalidSkill", func() {
		subs := choices.NewSubmissions()
		for _, sub := range s.validBase.Choices {
			if sub.Category == shared.ChoiceSkills {
				// Try to choose a skill not in Barbarian's list
				sub.Values = []shared.SelectionID{
					skills.Athletics,
					skills.Arcana, // Not a Barbarian skill
				}
			}
			subs.Add(sub)
		}

		result := s.validator.Validate(reqs, subs)
		s.Assert().False(result.Valid, "Should fail with invalid skill")
	})

	s.Run("MissingPrimaryWeapon", func() {
		subs := choices.NewSubmissions()
		for _, sub := range s.validBase.Choices {
			if sub.ChoiceID != choices.BarbarianWeaponsPrimary {
				subs.Add(sub)
			}
		}

		result := s.validator.Validate(reqs, subs)
		s.Assert().False(result.Valid, "Should fail without primary weapon")
	})

	s.Run("MissingSecondaryWeapon", func() {
		subs := choices.NewSubmissions()
		for _, sub := range s.validBase.Choices {
			if sub.ChoiceID != choices.BarbarianWeaponsSecondary {
				subs.Add(sub)
			}
		}

		result := s.validator.Validate(reqs, subs)
		s.Assert().False(result.Valid, "Should fail without secondary weapon")
	})
}

// TestBarbarianSkills tests that Barbarian gets correct skill choices
func (s *BarbarianCompleteSuite) TestBarbarianSkills() {
	reqs := choices.GetClassRequirementsAtLevel(classes.Barbarian, 1)
	s.Require().NotNil(reqs.Skills)

	// Barbarian should choose 2 skills
	s.Assert().Equal(2, reqs.Skills.Count)

	// Check skill list includes expected Barbarian skills
	expectedSkills := []skills.Skill{
		skills.AnimalHandling,
		skills.Athletics,
		skills.Intimidation,
		skills.Nature,
		skills.Perception,
		skills.Survival,
	}

	for _, skill := range expectedSkills {
		s.Assert().Contains(reqs.Skills.Options, skill, "Barbarian should have %s as option", skill)
	}
}

// TestBarbarianNoMagic tests that Barbarian has no magical choices at level 1
func (s *BarbarianCompleteSuite) TestBarbarianNoMagic() {
	reqs := choices.GetClassRequirementsAtLevel(classes.Barbarian, 1)

	// Barbarians don't get cantrips
	s.Assert().Nil(reqs.Cantrips, "Barbarian should not have cantrips")

	// Barbarians don't get spells
	s.Assert().Nil(reqs.Spellbook, "Barbarian should not have spellbook")

	// Barbarians don't choose subclass at level 1 (Path at level 3)
	s.Assert().Nil(reqs.Subclass, "Barbarian should not choose subclass at level 1")
}

// TestEquipmentChoices tests various equipment combinations
func (s *BarbarianCompleteSuite) TestEquipmentChoices() {
	reqs := choices.GetClassRequirementsAtLevel(classes.Barbarian, 1)

	s.Run("GreataxeChoice", func() {
		// Already tested in base - greataxe
		result := s.validator.Validate(reqs, s.validBase)
		s.Assert().True(result.Valid)
	})

	s.Run("MartialWeaponChoice", func() {
		subs := choices.NewSubmissions()
		for _, sub := range s.validBase.Choices {
			if sub.ChoiceID == choices.BarbarianWeaponsPrimary {
				// Choose martial weapon option
				sub.OptionID = choices.BarbarianWeaponMartial
				sub.Values = []shared.SelectionID{
					choices.BarbarianWeaponMartial,
				}
				// TODO: This would also need the actual weapon choice from the category
			}
			subs.Add(sub)
		}

		result := s.validator.Validate(reqs, subs)
		s.Assert().True(result.Valid, "Should be valid with martial weapon choice")
	})

	s.Run("SimpleWeaponSecondary", func() {
		subs := choices.NewSubmissions()
		for _, sub := range s.validBase.Choices {
			if sub.ChoiceID == choices.BarbarianWeaponsSecondary {
				// Choose simple weapon option
				sub.OptionID = choices.BarbarianSecondarySimple
				sub.Values = []shared.SelectionID{
					choices.BarbarianSecondarySimple,
				}
				// TODO: This would also need the actual weapon choice from the category
			}
			subs.Add(sub)
		}

		result := s.validator.Validate(reqs, subs)
		s.Assert().True(result.Valid, "Should be valid with simple weapon choice")
	})
}

// TestBarbarianCompleteSuite runs the suite
func TestBarbarianCompleteSuite(t *testing.T) {
	suite.Run(t, new(BarbarianCompleteSuite))
}
