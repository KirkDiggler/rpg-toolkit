package choices_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/armor"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/character/choices"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/classes"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/languages"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/shared"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/skills"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/spells"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/weapons"
	"github.com/stretchr/testify/suite"
)

// ClericCompleteSuite provides comprehensive testing for Cleric class
type ClericCompleteSuite struct {
	suite.Suite

	validator *choices.Validator
	apiData   map[string]interface{}

	// Base valid submissions that get reset each test
	validBase *choices.Submissions
}

// SetupSuite runs once - load API data
func (s *ClericCompleteSuite) SetupSuite() {
	s.validator = choices.NewValidator()
	s.loadAPIData()
}

// SetupTest runs before each test method
func (s *ClericCompleteSuite) SetupTest() {
	s.validBase = s.createValidBaseSubmissions()
}

// SetupSubTest runs before each s.Run()
func (s *ClericCompleteSuite) SetupSubTest() {
	// Reset to clean base for each subtest
	s.validBase = s.createValidBaseSubmissions()
}

// loadAPIData loads the cached D&D 5e API data
func (s *ClericCompleteSuite) loadAPIData() {
	apiPath := filepath.Join("testdata", "api", "classes", "cleric.json")
	if data, err := os.ReadFile(apiPath); err == nil {
		json.Unmarshal(data, &s.apiData)
	}
}

// createValidBaseSubmissions creates a valid set of base Cleric submissions
func (s *ClericCompleteSuite) createValidBaseSubmissions() *choices.Submissions {
	subs := choices.NewSubmissions()

	// Skills - Cleric chooses 2 from their list
	subs.Add(choices.Submission{
		Category: shared.ChoiceSkills,
		Source:   shared.SourceClass,
		ChoiceID: choices.ClericSkills,
		Values: []shared.SelectionID{
			shared.SelectionID(skills.Insight),
			shared.SelectionID(skills.Medicine),
		},
	})

	// Weapon choice - option a: mace
	// When selecting a predefined option, Values contains the option ID
	subs.Add(choices.Submission{
		Category: shared.ChoiceEquipment,
		Source:   shared.SourceClass,
		ChoiceID: choices.ClericWeapons,
		OptionID: "cleric-weapon-a",
		Values: []shared.SelectionID{
			shared.SelectionID("cleric-weapon-a"), // The option chosen
		},
	})

	// Armor choice - option a: scale mail
	subs.Add(choices.Submission{
		Category: shared.ChoiceEquipment,
		Source:   shared.SourceClass,
		ChoiceID: choices.ClericArmor,
		OptionID: "cleric-armor-a",
		Values: []shared.SelectionID{
			shared.SelectionID("cleric-armor-a"),
		},
	})

	// Secondary weapon - option a: light crossbow and bolts
	subs.Add(choices.Submission{
		Category: shared.ChoiceEquipment,
		Source:   shared.SourceClass,
		ChoiceID: choices.ClericSecondaryWeapon,
		OptionID: "cleric-secondary-a",
		Values: []shared.SelectionID{
			shared.SelectionID("cleric-secondary-a"),
		},
	})

	// Pack choice - option a: priest's pack
	subs.Add(choices.Submission{
		Category: shared.ChoiceEquipment,
		Source:   shared.SourceClass,
		ChoiceID: choices.ClericPack,
		OptionID: "cleric-pack-a",
		Values: []shared.SelectionID{
			shared.SelectionID("cleric-pack-a"),
		},
	})

	// Holy symbol
	subs.Add(choices.Submission{
		Category: shared.ChoiceEquipment,
		Source:   shared.SourceClass,
		ChoiceID: choices.ClericHolySymbol,
		OptionID: "cleric-holy-symbol",
		Values: []shared.SelectionID{
			shared.SelectionID("cleric-holy-symbol"),
		},
	})

	// Cantrips - 3 cleric cantrips
	subs.Add(choices.Submission{
		Category: shared.ChoiceCantrips,
		Source:   shared.SourceClass,
		ChoiceID: choices.ChoiceID("cleric-cantrips-1"),
		Values: []shared.SelectionID{
			shared.SelectionID(spells.SacredFlame),
			shared.SelectionID(spells.Guidance),
			shared.SelectionID(spells.Light),
		},
	})

	// Subclass - default to Life Domain
	// Note: Subclass uses ChoiceClass category since it's a class choice
	subs.Add(choices.Submission{
		Category: shared.ChoiceClass,
		Source:   shared.SourceClass,
		ChoiceID: choices.ClericDomain,
		Values: []shared.SelectionID{
			shared.SelectionID(classes.LifeDomain),
		},
	})

	return subs
}

// TestBasicClericValidation tests that our base submissions are valid
func (s *ClericCompleteSuite) TestBasicClericValidation() {
	reqs := choices.GetClassRequirementsAtLevel(classes.Cleric, 1)
	s.Require().NotNil(reqs)

	result := s.validator.Validate(reqs, s.validBase)
	s.Assert().True(result.Valid, "Base submissions should be valid")
	s.Assert().Empty(result.Errors)
}

// TestLifeDomain tests Life Domain specific features
func (s *ClericCompleteSuite) TestLifeDomain() {
	reqs := choices.GetClassRequirementsWithSubclass(classes.Cleric, 1, classes.LifeDomain)
	s.Require().NotNil(reqs)

	s.Run("ValidWithBasicEquipment", func() {
		// Base submissions already have Life Domain selected
		result := s.validator.Validate(reqs, s.validBase)
		s.Assert().True(result.Valid)
	})

	s.Run("ValidWithHeavyArmor", func() {
		// Life Domain can choose chain mail
		// Update armor choice to Life Domain option
		subs := choices.NewSubmissions()

		// Copy all non-armor submissions
		for _, sub := range s.validBase.Choices {
			if sub.ChoiceID != choices.ClericArmor {
				subs.Add(sub)
			}
		}

		// Add Life Domain chain mail choice
		subs.Add(choices.Submission{
			Category: shared.ChoiceEquipment,
			Source:   shared.SourceClass,
			ChoiceID: choices.ClericArmor,
			OptionID: "cleric-armor-life",
			Values: []shared.SelectionID{
				shared.SelectionID(armor.ChainMail),
			},
		})

		result := s.validator.Validate(reqs, subs)
		s.Assert().True(result.Valid, "Should be able to choose Life Domain chain mail")
	})

	s.Run("HasDomainSpells", func() {
		mods := choices.GetSubclassModifications(classes.LifeDomain)
		s.Require().NotNil(mods)
		s.Assert().NotEmpty(mods.GrantedSpells)

		// Check level 1 spells
		found := false
		for _, grant := range mods.GrantedSpells {
			if grant.Level == 1 {
				found = true
				s.Assert().Len(grant.Spells, 2, "Should have 2 domain spells at level 1")
				// Should have bless and cure-wounds
				s.Assert().Contains(grant.Spells, spells.Bless)
				s.Assert().Contains(grant.Spells, spells.CureWounds)
			}
		}
		s.Assert().True(found, "Should have level 1 domain spells")
	})
}

// TestKnowledgeDomain tests Knowledge Domain with additional skills and languages
func (s *ClericCompleteSuite) TestKnowledgeDomain() {
	reqs := choices.GetClassRequirementsWithSubclass(classes.Cleric, 1, classes.KnowledgeDomain)
	s.Require().NotNil(reqs)

	s.Run("RequiresAdditionalSkills", func() {
		// Change to Knowledge Domain but don't add extra skills
		subs := choices.NewSubmissions()
		for _, sub := range s.validBase.Choices {
			if sub.ChoiceID == choices.ClericDomain {
				// Switch to Knowledge Domain
				sub.Values = []shared.SelectionID{
					shared.SelectionID(classes.KnowledgeDomain),
				}
			}
			subs.Add(sub)
		}

		result := s.validator.Validate(reqs, subs)
		s.Assert().False(result.Valid, "Should require Knowledge Domain skills")
	})

	s.Run("ValidWithAllChoices", func() {
		// Create complete Knowledge Domain submissions
		subs := choices.NewSubmissions()

		// Copy base submissions, changing subclass
		for _, sub := range s.validBase.Choices {
			if sub.ChoiceID == choices.ClericDomain {
				sub.Values = []shared.SelectionID{
					shared.SelectionID(classes.KnowledgeDomain),
				}
			}
			subs.Add(sub)
		}

		// Add Knowledge Domain skills
		subs.Add(choices.Submission{
			Category: shared.ChoiceSkills,
			Source:   shared.SourceSubclass,
			ChoiceID: choices.ChoiceID("cleric-knowledge-skills"),
			Values: []shared.SelectionID{
				shared.SelectionID(skills.Arcana),
				shared.SelectionID(skills.History),
			},
		})

		// Add Knowledge Domain languages
		subs.Add(choices.Submission{
			Category: shared.ChoiceLanguages,
			Source:   shared.SourceSubclass,
			ChoiceID: choices.ChoiceID("cleric-knowledge-languages"),
			Values: []shared.SelectionID{
				shared.SelectionID(languages.Elvish),
				shared.SelectionID(languages.Dwarvish),
			},
		})

		result := s.validator.Validate(reqs, subs)
		if !result.Valid {
			for _, err := range result.Errors {
				s.T().Logf("Validation error: %+v", err)
			}
		}
		s.Assert().True(result.Valid, "Should be valid with all Knowledge choices")
	})

	s.Run("InvalidSkillChoice", func() {
		subs := choices.NewSubmissions()

		// Copy base with Knowledge Domain
		for _, sub := range s.validBase.Choices {
			if sub.ChoiceID == choices.ClericDomain {
				sub.Values = []shared.SelectionID{
					shared.SelectionID(classes.KnowledgeDomain),
				}
			}
			subs.Add(sub)
		}

		// Try to choose invalid skills for Knowledge Domain
		subs.Add(choices.Submission{
			Category: shared.ChoiceSkills,
			Source:   shared.SourceSubclass,
			ChoiceID: choices.ChoiceID("cleric-knowledge-skills"),
			Values: []shared.SelectionID{
				shared.SelectionID(skills.Acrobatics), // Not valid for Knowledge
				shared.SelectionID(skills.Athletics),  // Not valid for Knowledge
			},
		})

		// Add languages (valid)
		subs.Add(choices.Submission{
			Category: shared.ChoiceLanguages,
			Source:   shared.SourceSubclass,
			ChoiceID: choices.ChoiceID("cleric-knowledge-languages"),
			Values: []shared.SelectionID{
				shared.SelectionID(languages.Elvish),
				shared.SelectionID(languages.Dwarvish),
			},
		})

		result := s.validator.Validate(reqs, subs)
		s.Assert().False(result.Valid, "Should reject invalid skill choices")
	})
}

// TestWarDomain tests War Domain with martial weapons
func (s *ClericCompleteSuite) TestWarDomain() {
	reqs := choices.GetClassRequirementsWithSubclass(classes.Cleric, 1, classes.WarDomain)
	s.Require().NotNil(reqs)

	s.Run("CanChooseMartialWeapon", func() {
		subs := choices.NewSubmissions()

		// Copy non-weapon submissions, switch to War Domain
		for _, sub := range s.validBase.Choices {
			if sub.ChoiceID == choices.ClericDomain {
				sub.Values = []shared.SelectionID{
					shared.SelectionID(classes.WarDomain),
				}
				subs.Add(sub)
			} else if sub.ChoiceID != choices.ClericWeapons {
				subs.Add(sub)
			}
		}

		// Choose martial weapon option
		subs.Add(choices.Submission{
			Category: shared.ChoiceEquipment,
			Source:   shared.SourceClass,
			ChoiceID: choices.ClericWeapons,
			OptionID: "cleric-weapon-war",
			Values: []shared.SelectionID{
				shared.SelectionID(weapons.Longsword),
			},
		})

		result := s.validator.Validate(reqs, subs)
		s.Assert().True(result.Valid, "Should be able to choose martial weapon")
	})

	s.Run("CanChooseHeavyArmor", func() {
		subs := choices.NewSubmissions()

		// Copy non-armor submissions, switch to War Domain
		for _, sub := range s.validBase.Choices {
			if sub.ChoiceID == choices.ClericDomain {
				sub.Values = []shared.SelectionID{
					shared.SelectionID(classes.WarDomain),
				}
				subs.Add(sub)
			} else if sub.ChoiceID != choices.ClericArmor {
				subs.Add(sub)
			}
		}

		// Choose War Domain chain mail
		subs.Add(choices.Submission{
			Category: shared.ChoiceEquipment,
			Source:   shared.SourceClass,
			ChoiceID: choices.ClericArmor,
			OptionID: "cleric-armor-war",
			Values: []shared.SelectionID{
				shared.SelectionID(armor.ChainMail),
			},
		})

		result := s.validator.Validate(reqs, subs)
		s.Assert().True(result.Valid, "Should be able to choose heavy armor")
	})
}

// TestInvalidSubmissions tests various invalid scenarios
func (s *ClericCompleteSuite) TestInvalidSubmissions() {
	reqs := choices.GetClassRequirementsAtLevel(classes.Cleric, 1)

	s.Run("MissingSkills", func() {
		subs := choices.NewSubmissions()
		// Add everything except skills
		for _, sub := range s.validBase.Choices {
			if sub.ChoiceID != choices.ClericSkills {
				subs.Add(sub)
			}
		}

		result := s.validator.Validate(reqs, subs)
		s.Assert().False(result.Valid, "Should fail without skills")
	})

	s.Run("TooManySkills", func() {
		subs := choices.NewSubmissions()
		for _, sub := range s.validBase.Choices {
			if sub.ChoiceID == choices.ClericSkills {
				// Try to choose 3 skills instead of 2
				sub.Values = []shared.SelectionID{
					shared.SelectionID(skills.History),
					shared.SelectionID(skills.Insight),
					shared.SelectionID(skills.Medicine),
				}
			}
			subs.Add(sub)
		}

		result := s.validator.Validate(reqs, subs)
		s.Assert().False(result.Valid, "Should fail with too many skills")
	})

	s.Run("InvalidSkill", func() {
		subs := choices.NewSubmissions()
		for _, sub := range s.validBase.Choices {
			if sub.ChoiceID == choices.ClericSkills {
				// Try to choose skills not in Cleric list
				sub.Values = []shared.SelectionID{
					shared.SelectionID(skills.Acrobatics),
					shared.SelectionID(skills.Stealth),
				}
			}
			subs.Add(sub)
		}

		result := s.validator.Validate(reqs, subs)
		s.Assert().False(result.Valid, "Should fail with invalid skills")
	})

	s.Run("MissingSubclass", func() {
		subs := choices.NewSubmissions()
		// Add everything except subclass
		for _, sub := range s.validBase.Choices {
			if sub.ChoiceID != choices.ClericDomain {
				subs.Add(sub)
			}
		}

		result := s.validator.Validate(reqs, subs)
		s.Assert().False(result.Valid, "Should fail without subclass at level 1")
	})

	s.Run("WrongNumberOfCantrips", func() {
		subs := choices.NewSubmissions()
		for _, sub := range s.validBase.Choices {
			if sub.Category == shared.ChoiceCantrips {
				// Only choose 1 cantrip instead of 3
				sub.Values = []shared.SelectionID{
					shared.SelectionID(spells.SacredFlame),
				}
			}
			subs.Add(sub)
		}

		result := s.validator.Validate(reqs, subs)
		s.Assert().False(result.Valid, "Should fail with wrong number of cantrips")
	})

	s.Run("InvalidEquipmentOption", func() {
		subs := choices.NewSubmissions()
		for _, sub := range s.validBase.Choices {
			if sub.ChoiceID == choices.ClericWeapons {
				// Use invalid option ID
				sub.OptionID = "invalid-option"
			}
			subs.Add(sub)
		}

		result := s.validator.Validate(reqs, subs)
		s.Assert().False(result.Valid, "Should fail with invalid equipment option")
	})
}

// TestAllDomains verifies all domains are properly configured
func (s *ClericCompleteSuite) TestAllDomains() {
	domains := []classes.Subclass{
		classes.LifeDomain,
		classes.LightDomain,
		classes.NatureDomain,
		classes.TempestDomain,
		classes.TrickeryDomain,
		classes.WarDomain,
		classes.KnowledgeDomain,
		classes.DeathDomain,
	}

	for _, domain := range domains {
		s.Run(string(domain), func() {
			// Get requirements with this domain
			reqs := choices.GetClassRequirementsWithSubclass(classes.Cleric, 1, domain)
			s.Assert().NotNil(reqs, "Should have requirements for %s", domain)

			// Get modifications
			mods := choices.GetSubclassModifications(domain)
			s.Assert().NotNil(mods, "Should have modifications for %s", domain)

			// All domains should have domain spells
			s.Assert().NotEmpty(mods.GrantedSpells, "%s should have domain spells", domain)
		})
	}
}

// TestAPIComparison compares our implementation against D&D 5e API
func (s *ClericCompleteSuite) TestAPIComparison() {
	if s.apiData == nil {
		s.T().Skip("No API data loaded")
	}

	reqs := choices.GetClassRequirementsAtLevel(classes.Cleric, 1)

	s.Run("SkillChoices", func() {
		// Check API proficiency choices
		if profChoices, ok := s.apiData["proficiency_choices"].([]interface{}); ok {
			s.Assert().NotEmpty(profChoices)

			if firstChoice, ok := profChoices[0].(map[string]interface{}); ok {
				// Should choose 2
				if choose, ok := firstChoice["choose"].(float64); ok {
					s.Assert().Equal(2, int(choose))
				}

				// Check our requirement matches
				s.Assert().Equal(2, reqs.Skills.Count)
			}
		}
	})

	s.Run("StartingEquipment", func() {
		// Check equipment options match
		if eqOptions, ok := s.apiData["starting_equipment_options"].([]interface{}); ok {
			// We should have same number of equipment choices
			s.Assert().Equal(len(eqOptions), len(reqs.Equipment))
		}
	})

	s.Run("Cantrips", func() {
		// Check spellcasting info
		if spellcasting, ok := s.apiData["spellcasting"].(map[string]interface{}); ok {
			if info, ok := spellcasting["info"].([]interface{}); ok {
				// First info should be about cantrips
				if len(info) > 0 {
					if cantripInfo, ok := info[0].(map[string]interface{}); ok {
						if name, ok := cantripInfo["name"].(string); ok {
							s.Assert().Equal("Cantrips", name)
						}
					}
				}
			}
		}

		// We start with 3 cantrips
		s.Assert().Equal(3, reqs.Cantrips.Count)
	})
}

// TestClericCompleteSuite runs the suite
func TestClericCompleteSuite(t *testing.T) {
	suite.Run(t, new(ClericCompleteSuite))
}
