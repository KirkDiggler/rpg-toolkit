package choices_test

import (
	"testing"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/character/choices"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/classes"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/fightingstyles"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/shared"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/skills"
	"github.com/stretchr/testify/suite"
)

// FighterCompleteSuite provides comprehensive testing for Fighter class
type FighterCompleteSuite struct {
	suite.Suite

	validator *choices.Validator
	validBase *choices.Submissions
}

// SetupSuite runs once
func (s *FighterCompleteSuite) SetupSuite() {
	s.validator = choices.NewValidator()
}

// SetupTest runs before each test
func (s *FighterCompleteSuite) SetupTest() {
	s.validBase = s.createValidBaseSubmissions()
}

// createValidBaseSubmissions creates valid Fighter submissions
func (s *FighterCompleteSuite) createValidBaseSubmissions() *choices.Submissions {
	subs := choices.NewSubmissions()

	// Skills - Fighter chooses 2 from their list
	subs.Add(choices.Submission{
		Category: shared.ChoiceSkills,
		Source:   shared.SourceClass,
		ChoiceID: choices.ChoiceID("fighter-skills"),
		Values: []shared.SelectionID{
			shared.SelectionID(skills.Athletics),
			shared.SelectionID(skills.Survival),
		},
	})

	// Armor choice - option a: chain mail
	subs.Add(choices.Submission{
		Category: shared.ChoiceEquipment,
		Source:   shared.SourceClass,
		ChoiceID: choices.FighterArmor,
		OptionID: choices.FighterArmorChainMail,
		Values: []shared.SelectionID{
			shared.SelectionID(choices.FighterArmorChainMail),
		},
	})

	// Weapon choice - option a: martial weapon and shield
	// For equipment with OptionID, only the option goes in Values
	subs.Add(choices.Submission{
		Category: shared.ChoiceEquipment,
		Source:   shared.SourceClass,
		ChoiceID: choices.FighterWeaponsPrimary,
		OptionID: choices.FighterWeaponMartialShield,
		Values: []shared.SelectionID{
			shared.SelectionID(choices.FighterWeaponMartialShield),
		},
		// TODO: Need to track the actual weapon choice separately
	})

	// Secondary weapon choice - light crossbow and bolts
	subs.Add(choices.Submission{
		Category: shared.ChoiceEquipment,
		Source:   shared.SourceClass,
		ChoiceID: choices.FighterWeaponsSecondary,
		OptionID: choices.FighterRangedCrossbow,
		Values: []shared.SelectionID{
			shared.SelectionID(choices.FighterRangedCrossbow),
		},
	})

	// Pack choice - dungeoneer's pack
	subs.Add(choices.Submission{
		Category: shared.ChoiceEquipment,
		Source:   shared.SourceClass,
		ChoiceID: choices.FighterPack,
		OptionID: choices.FighterPackDungeoneer,
		Values: []shared.SelectionID{
			shared.SelectionID(choices.FighterPackDungeoneer),
		},
	})

	// Fighting style - Defense
	subs.Add(choices.Submission{
		Category: shared.ChoiceFightingStyle,
		Source:   shared.SourceClass,
		ChoiceID: choices.FighterFightingStyle,
		Values: []shared.SelectionID{
			shared.SelectionID(fightingstyles.Defense),
		},
	})

	return subs
}

// TestBasicFighterValidation tests basic Fighter requirements
func (s *FighterCompleteSuite) TestBasicFighterValidation() {
	reqs := choices.GetClassRequirementsAtLevel(classes.Fighter, 1)
	s.Require().NotNil(reqs)

	result := s.validator.Validate(reqs, s.validBase)
	s.Assert().True(result.Valid, "Base submissions should be valid")
	s.Assert().Empty(result.Errors)
}

// TestInvalidSubmissions tests various invalid scenarios
func (s *FighterCompleteSuite) TestInvalidSubmissions() {
	reqs := choices.GetClassRequirementsAtLevel(classes.Fighter, 1)

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

	s.Run("InvalidSkill", func() {
		subs := choices.NewSubmissions()
		for _, sub := range s.validBase.Choices {
			if sub.Category == shared.ChoiceSkills {
				// Try to choose a skill not in Fighter's list
				sub.Values = []shared.SelectionID{
					shared.SelectionID(skills.Athletics),
					shared.SelectionID(skills.Arcana), // Not a Fighter skill
				}
			}
			subs.Add(sub)
		}

		result := s.validator.Validate(reqs, subs)
		s.Assert().False(result.Valid, "Should fail with invalid skill")
	})

	s.Run("MissingFightingStyle", func() {
		subs := choices.NewSubmissions()
		for _, sub := range s.validBase.Choices {
			if sub.Category != shared.ChoiceFightingStyle {
				subs.Add(sub)
			}
		}

		result := s.validator.Validate(reqs, subs)
		s.Assert().False(result.Valid, "Should fail without fighting style")
	})

	s.Run("InvalidFightingStyle", func() {
		subs := choices.NewSubmissions()
		for _, sub := range s.validBase.Choices {
			if sub.Category == shared.ChoiceFightingStyle {
				// Try to use an invalid fighting style
				sub.Values = []shared.SelectionID{
					shared.SelectionID("invalid-style"),
				}
			}
			subs.Add(sub)
		}

		result := s.validator.Validate(reqs, subs)
		s.Assert().False(result.Valid, "Should fail with invalid fighting style")
	})
}

// TestEquipmentChoices tests various equipment combinations
func (s *FighterCompleteSuite) TestEquipmentChoices() {
	reqs := choices.GetClassRequirementsAtLevel(classes.Fighter, 1)

	s.Run("ChainMailChoice", func() {
		// Already tested in base - chain mail
		result := s.validator.Validate(reqs, s.validBase)
		s.Assert().True(result.Valid)
	})

	s.Run("LeatherArmorChoice", func() {
		subs := choices.NewSubmissions()
		for _, sub := range s.validBase.Choices {
			if sub.ChoiceID == choices.FighterArmor {
				// Choose leather armor, longbow, and arrows
				sub.OptionID = choices.FighterArmorLeather
				sub.Values = []shared.SelectionID{
					shared.SelectionID(choices.FighterArmorLeather),
				}
			}
			subs.Add(sub)
		}

		result := s.validator.Validate(reqs, subs)
		s.Assert().True(result.Valid, "Should be valid with leather armor choice")
	})

	s.Run("TwoMartialWeapons", func() {
		subs := choices.NewSubmissions()
		for _, sub := range s.validBase.Choices {
			if sub.ChoiceID == choices.FighterWeaponsPrimary {
				// Choose two martial weapons
				sub.OptionID = choices.FighterWeaponTwoMartial
				sub.Values = []shared.SelectionID{
					shared.SelectionID(choices.FighterWeaponTwoMartial),
				}
				// TODO: Need to handle category weapon choices
			}
			subs.Add(sub)
		}

		result := s.validator.Validate(reqs, subs)
		s.Assert().True(result.Valid, "Should be valid with two martial weapons")
	})

	s.Run("HandcrossbowChoice", func() {
		subs := choices.NewSubmissions()
		for _, sub := range s.validBase.Choices {
			if sub.ChoiceID == choices.FighterWeaponsSecondary {
				// Choose handaxes
				sub.OptionID = choices.FighterRangedHandaxes
				sub.Values = []shared.SelectionID{
					shared.SelectionID(choices.FighterRangedHandaxes),
				}
			}
			subs.Add(sub)
		}

		result := s.validator.Validate(reqs, subs)
		s.Assert().True(result.Valid, "Should be valid with handcrossbows")
	})
}

// TestFightingStyles tests all fighting style options
func (s *FighterCompleteSuite) TestFightingStyles() {
	reqs := choices.GetClassRequirementsAtLevel(classes.Fighter, 1)

	styles := []fightingstyles.FightingStyle{
		fightingstyles.Archery,
		fightingstyles.Defense,
		fightingstyles.Dueling,
		fightingstyles.GreatWeaponFighting,
		fightingstyles.Protection,
		fightingstyles.TwoWeaponFighting,
	}

	for _, style := range styles {
		s.Run(string(style), func() {
			subs := choices.NewSubmissions()
			for _, sub := range s.validBase.Choices {
				if sub.Category == shared.ChoiceFightingStyle {
					sub.Values = []shared.SelectionID{
						shared.SelectionID(style),
					}
				}
				subs.Add(sub)
			}

			result := s.validator.Validate(reqs, subs)
			s.Assert().True(result.Valid, "Should be valid with %s style", style)
		})
	}
}

// TestFighterSkills tests that Fighter gets correct skill choices
func (s *FighterCompleteSuite) TestFighterSkills() {
	reqs := choices.GetClassRequirementsAtLevel(classes.Fighter, 1)
	s.Require().NotNil(reqs.Skills)

	// Fighter should choose 2 skills
	s.Assert().Equal(2, reqs.Skills.Count)

	// Check skill list includes expected Fighter skills
	expectedSkills := []skills.Skill{
		skills.Acrobatics,
		skills.AnimalHandling,
		skills.Athletics,
		skills.History,
		skills.Insight,
		skills.Intimidation,
		skills.Perception,
		skills.Survival,
	}

	for _, skill := range expectedSkills {
		s.Assert().Contains(reqs.Skills.Options, skill, "Fighter should have %s as option", skill)
	}
}

// TestFighterCompleteSuite runs the suite
func TestFighterCompleteSuite(t *testing.T) {
	suite.Run(t, new(FighterCompleteSuite))
}