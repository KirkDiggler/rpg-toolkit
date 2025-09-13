package choices_test

import (
	"testing"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/character/choices"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/classes"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/shared"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/skills"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/spells"
	"github.com/stretchr/testify/suite"
)

// WizardCompleteSuite provides comprehensive testing for Wizard class
type WizardCompleteSuite struct {
	suite.Suite

	validator *choices.Validator
	validBase *choices.Submissions
}

// SetupSuite runs once
func (s *WizardCompleteSuite) SetupSuite() {
	s.validator = choices.NewValidator()
}

// SetupTest runs before each test
func (s *WizardCompleteSuite) SetupTest() {
	s.validBase = s.createValidBaseSubmissions()
}

// createValidBaseSubmissions creates valid Wizard submissions
func (s *WizardCompleteSuite) createValidBaseSubmissions() *choices.Submissions {
	subs := choices.NewSubmissions()

	// Skills - Wizard chooses 2 from their list
	subs.Add(choices.Submission{
		Category: shared.ChoiceSkills,
		Source:   shared.SourceClass,
		ChoiceID: choices.ChoiceID("wizard-skills"),
		Values: []shared.SelectionID{
			shared.SelectionID(skills.Arcana),
			shared.SelectionID(skills.Investigation),
		},
	})

	// Weapon choice - option a: quarterstaff
	subs.Add(choices.Submission{
		Category: shared.ChoiceEquipment,
		Source:   shared.SourceClass,
		ChoiceID: choices.WizardWeaponsPrimary,
		OptionID: choices.WizardWeaponQuarterstaff,
		Values: []shared.SelectionID{
			shared.SelectionID(choices.WizardWeaponQuarterstaff),
		},
	})

	// Focus choice - option a: component pouch
	subs.Add(choices.Submission{
		Category: shared.ChoiceEquipment,
		Source:   shared.SourceClass,
		ChoiceID: choices.WizardFocus,
		OptionID: choices.WizardFocusComponent,
		Values: []shared.SelectionID{
			shared.SelectionID(choices.WizardFocusComponent),
		},
	})

	// Pack choice - scholar's pack
	subs.Add(choices.Submission{
		Category: shared.ChoiceEquipment,
		Source:   shared.SourceClass,
		ChoiceID: choices.WizardPack,
		OptionID: choices.WizardPackScholar,
		Values: []shared.SelectionID{
			shared.SelectionID(choices.WizardPackScholar),
		},
	})

	// Cantrips - 3 wizard cantrips
	subs.Add(choices.Submission{
		Category: shared.ChoiceCantrips,
		Source:   shared.SourceClass,
		ChoiceID: choices.WizardCantrips1,
		Values: []shared.SelectionID{
			shared.SelectionID(spells.FireBolt),
			shared.SelectionID(spells.MageHand),
			shared.SelectionID(spells.Prestidigitation),
		},
	})

	// Spellbook - 6 1st-level spells
	subs.Add(choices.Submission{
		Category: shared.ChoiceSpells,
		Source:   shared.SourceClass,
		ChoiceID: choices.WizardSpells1,
		Values: []shared.SelectionID{
			shared.SelectionID(spells.MagicMissile),
			shared.SelectionID(spells.Shield),
			shared.SelectionID(spells.Sleep),
			shared.SelectionID(spells.DetectMagic),
			shared.SelectionID(spells.Identify),
			shared.SelectionID(spells.CharmPerson),
		},
	})

	return subs
}

// TestBasicWizardValidation tests basic Wizard requirements
func (s *WizardCompleteSuite) TestBasicWizardValidation() {
	reqs := choices.GetClassRequirementsAtLevel(classes.Wizard, 1)
	s.Require().NotNil(reqs)

	result := s.validator.Validate(reqs, s.validBase)
	s.Assert().True(result.Valid, "Base submissions should be valid")
	s.Assert().Empty(result.Errors)
}

// TestInvalidSubmissions tests various invalid scenarios
func (s *WizardCompleteSuite) TestInvalidSubmissions() {
	reqs := choices.GetClassRequirementsAtLevel(classes.Wizard, 1)

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
				// Try to choose a skill not in Wizard's list
				sub.Values = []shared.SelectionID{
					shared.SelectionID(skills.Arcana),
					shared.SelectionID(skills.Athletics), // Not a Wizard skill
				}
			}
			subs.Add(sub)
		}

		result := s.validator.Validate(reqs, subs)
		s.Assert().False(result.Valid, "Should fail with invalid skill")
	})

	s.Run("MissingCantrips", func() {
		subs := choices.NewSubmissions()
		for _, sub := range s.validBase.Choices {
			if sub.Category != shared.ChoiceCantrips {
				subs.Add(sub)
			}
		}

		result := s.validator.Validate(reqs, subs)
		s.Assert().False(result.Valid, "Should fail without cantrips")
	})

	s.Run("WrongNumberOfCantrips", func() {
		subs := choices.NewSubmissions()
		for _, sub := range s.validBase.Choices {
			if sub.Category == shared.ChoiceCantrips {
				// Only choose 2 instead of 3
				sub.Values = []shared.SelectionID{
					shared.SelectionID(spells.FireBolt),
					shared.SelectionID(spells.MageHand),
				}
			}
			subs.Add(sub)
		}

		result := s.validator.Validate(reqs, subs)
		s.Assert().False(result.Valid, "Should fail with wrong number of cantrips")
	})

	s.Run("MissingSpellbook", func() {
		subs := choices.NewSubmissions()
		for _, sub := range s.validBase.Choices {
			if sub.Category != shared.ChoiceSpells {
				subs.Add(sub)
			}
		}

		result := s.validator.Validate(reqs, subs)
		s.Assert().False(result.Valid, "Should fail without spellbook")
	})

	s.Run("WrongNumberOfSpells", func() {
		subs := choices.NewSubmissions()
		for _, sub := range s.validBase.Choices {
			if sub.Category == shared.ChoiceSpells {
				// Only choose 4 instead of 6
				sub.Values = []shared.SelectionID{
					shared.SelectionID(spells.MagicMissile),
					shared.SelectionID(spells.Shield),
					shared.SelectionID(spells.Sleep),
					shared.SelectionID(spells.DetectMagic),
				}
			}
			subs.Add(sub)
		}

		result := s.validator.Validate(reqs, subs)
		s.Assert().False(result.Valid, "Should fail with wrong number of spells")
	})

	s.Run("InvalidSpell", func() {
		subs := choices.NewSubmissions()
		for _, sub := range s.validBase.Choices {
			if sub.Category == shared.ChoiceSpells {
				// Try to add a spell not in the wizard list
				sub.Values = []shared.SelectionID{
					shared.SelectionID(spells.MagicMissile),
					shared.SelectionID(spells.Shield),
					shared.SelectionID(spells.Sleep),
					shared.SelectionID(spells.DetectMagic),
					shared.SelectionID(spells.Identify),
					shared.SelectionID(spells.CureWounds), // Cleric spell, not wizard
				}
			}
			subs.Add(sub)
		}

		result := s.validator.Validate(reqs, subs)
		s.Assert().False(result.Valid, "Should fail with invalid spell")
	})
}

// TestWizardCantrips tests wizard cantrip selection
func (s *WizardCompleteSuite) TestWizardCantrips() {
	reqs := choices.GetClassRequirementsAtLevel(classes.Wizard, 1)
	s.Require().NotNil(reqs.Cantrips)

	// Wizard should choose 3 cantrips
	s.Assert().Equal(3, reqs.Cantrips.Count)

	// Check cantrip list includes expected wizard cantrips
	expectedCantrips := []spells.Spell{
		spells.FireBolt,
		spells.RayOfFrost,
		spells.ShockingGrasp,
		spells.AcidSplash,
		spells.PoisonSpray,
		spells.ChillTouch,
		spells.MageHand,
		spells.MinorIllusion,
		spells.Prestidigitation,
		spells.Light,
	}

	for _, cantrip := range expectedCantrips {
		s.Assert().Contains(reqs.Cantrips.Options, cantrip, "Wizard should have %s as option", cantrip)
	}
}

// TestWizardSpellbook tests wizard spellbook selection
func (s *WizardCompleteSuite) TestWizardSpellbook() {
	reqs := choices.GetClassRequirementsAtLevel(classes.Wizard, 1)
	s.Require().NotNil(reqs.Spellbook)

	// Wizard should choose 6 1st-level spells
	s.Assert().Equal(6, reqs.Spellbook.Count)
	s.Assert().Equal(1, reqs.Spellbook.SpellLevel)

	// Check spell list includes expected wizard spells
	expectedSpells := []spells.Spell{
		spells.MagicMissile,
		spells.BurningHands,
		spells.ChromaticOrb,
		spells.Thunderwave,
		spells.Shield,
		spells.Sleep,
		spells.CharmPerson,
		spells.DetectMagic,
		spells.Identify,
	}

	for _, spell := range expectedSpells {
		s.Assert().Contains(reqs.Spellbook.Options, spell, "Wizard should have %s as option", spell)
	}
}

// TestWizardSkills tests that Wizard gets correct skill choices
func (s *WizardCompleteSuite) TestWizardSkills() {
	reqs := choices.GetClassRequirementsAtLevel(classes.Wizard, 1)
	s.Require().NotNil(reqs.Skills)

	// Wizard should choose 2 skills
	s.Assert().Equal(2, reqs.Skills.Count)

	// Check skill list includes expected Wizard skills
	expectedSkills := []skills.Skill{
		skills.Arcana,
		skills.History,
		skills.Insight,
		skills.Investigation,
		skills.Medicine,
		skills.Religion,
	}

	for _, skill := range expectedSkills {
		s.Assert().Contains(reqs.Skills.Options, skill, "Wizard should have %s as option", skill)
	}
}

// TestEquipmentChoices tests various equipment combinations
func (s *WizardCompleteSuite) TestEquipmentChoices() {
	reqs := choices.GetClassRequirementsAtLevel(classes.Wizard, 1)

	s.Run("QuarterstaffChoice", func() {
		// Already tested in base - quarterstaff
		result := s.validator.Validate(reqs, s.validBase)
		s.Assert().True(result.Valid)
	})

	s.Run("DaggerChoice", func() {
		subs := choices.NewSubmissions()
		for _, sub := range s.validBase.Choices {
			if sub.ChoiceID == choices.WizardWeaponsPrimary {
				// Choose dagger
				sub.OptionID = choices.WizardWeaponDagger
				sub.Values = []shared.SelectionID{
					shared.SelectionID(choices.WizardWeaponDagger),
				}
			}
			subs.Add(sub)
		}

		result := s.validator.Validate(reqs, subs)
		s.Assert().True(result.Valid, "Should be valid with dagger choice")
	})

	s.Run("ArcaneStaffChoice", func() {
		subs := choices.NewSubmissions()
		for _, sub := range s.validBase.Choices {
			if sub.ChoiceID == choices.WizardFocus {
				// Choose arcane staff
				sub.OptionID = choices.WizardFocusStaff
				sub.Values = []shared.SelectionID{
					shared.SelectionID(choices.WizardFocusStaff),
				}
			}
			subs.Add(sub)
		}

		result := s.validator.Validate(reqs, subs)
		s.Assert().True(result.Valid, "Should be valid with arcane staff")
	})

	s.Run("ExplorerPackChoice", func() {
		subs := choices.NewSubmissions()
		for _, sub := range s.validBase.Choices {
			if sub.ChoiceID == choices.WizardPack {
				// Choose explorer's pack
				sub.OptionID = choices.WizardPackExplorer
				sub.Values = []shared.SelectionID{
					shared.SelectionID(choices.WizardPackExplorer),
				}
			}
			subs.Add(sub)
		}

		result := s.validator.Validate(reqs, subs)
		s.Assert().True(result.Valid, "Should be valid with explorer's pack")
	})
}

// TestWizardCompleteSuite runs the suite
func TestWizardCompleteSuite(t *testing.T) {
	suite.Run(t, new(WizardCompleteSuite))
}