package choices_test

import (
	"testing"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/character/choices"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/classes"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/shared"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/skills"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/tools"
	"github.com/stretchr/testify/suite"
)

// RogueCompleteSuite provides comprehensive testing for Rogue class
type RogueCompleteSuite struct {
	suite.Suite

	validator *choices.Validator
	validBase *choices.Submissions
}

// SetupSuite runs once
func (s *RogueCompleteSuite) SetupSuite() {
	s.validator = choices.NewValidator()
}

// SetupTest runs before each test
func (s *RogueCompleteSuite) SetupTest() {
	s.validBase = s.createValidBaseSubmissions()
}

// createValidBaseSubmissions creates valid Rogue submissions
func (s *RogueCompleteSuite) createValidBaseSubmissions() *choices.Submissions {
	subs := choices.NewSubmissions()

	// Skills - Rogue chooses 4 from their list
	subs.Add(choices.Submission{
		Category: shared.ChoiceSkills,
		Source:   shared.SourceClass,
		ChoiceID: choices.RogueSkills,
		Values: []shared.SelectionID{
			skills.Stealth,
			skills.Acrobatics,
			skills.Investigation,
			skills.Perception,
		},
	})

	// Weapon choice - option a: rapier
	subs.Add(choices.Submission{
		Category: shared.ChoiceEquipment,
		Source:   shared.SourceClass,
		ChoiceID: choices.RogueWeaponsPrimary,
		OptionID: choices.RogueWeaponRapier,
		Values: []shared.SelectionID{
			choices.RogueWeaponRapier,
		},
	})

	// Secondary weapon choice - shortbow and arrows
	subs.Add(choices.Submission{
		Category: shared.ChoiceEquipment,
		Source:   shared.SourceClass,
		ChoiceID: choices.RogueWeaponsSecondary,
		OptionID: choices.RogueSecondaryShortbow,
		Values: []shared.SelectionID{
			choices.RogueSecondaryShortbow,
		},
	})

	// Pack choice - burglar's pack
	subs.Add(choices.Submission{
		Category: shared.ChoiceEquipment,
		Source:   shared.SourceClass,
		ChoiceID: choices.RoguePack,
		OptionID: choices.RoguePackBurglar,
		Values: []shared.SelectionID{
			choices.RoguePackBurglar,
		},
	})

	// Expertise - 2 skills or thieves' tools
	subs.Add(choices.Submission{
		Category: shared.ChoiceExpertise,
		Source:   shared.SourceClass,
		ChoiceID: choices.RogueExpertise1,
		Values: []shared.SelectionID{
			skills.Stealth,
			tools.ThievesTools,
		},
	})

	return subs
}

// TestBasicRogueValidation tests basic Rogue requirements
func (s *RogueCompleteSuite) TestBasicRogueValidation() {
	reqs := choices.GetClassRequirementsAtLevel(classes.Rogue, 1)
	s.Require().NotNil(reqs)

	result := s.validator.Validate(reqs, s.validBase)
	s.Assert().True(result.Valid, "Base submissions should be valid")
	s.Assert().Empty(result.Errors)
}

// TestInvalidSubmissions tests various invalid scenarios
func (s *RogueCompleteSuite) TestInvalidSubmissions() {
	reqs := choices.GetClassRequirementsAtLevel(classes.Rogue, 1)

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
				// Only choose 2 instead of 4
				sub.Values = []shared.SelectionID{
					skills.Stealth,
					skills.Acrobatics,
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
				// Try to choose a skill not in Rogue's list
				sub.Values = []shared.SelectionID{
					skills.Stealth,
					skills.Acrobatics,
					skills.Investigation,
					skills.Arcana, // Not a Rogue skill
				}
			}
			subs.Add(sub)
		}

		result := s.validator.Validate(reqs, subs)
		s.Assert().False(result.Valid, "Should fail with invalid skill")
	})

	s.Run("MissingExpertise", func() {
		subs := choices.NewSubmissions()
		for _, sub := range s.validBase.Choices {
			if sub.Category != shared.ChoiceExpertise {
				subs.Add(sub)
			}
		}

		result := s.validator.Validate(reqs, subs)
		s.Assert().False(result.Valid, "Should fail without expertise")
	})

	s.Run("WrongNumberOfExpertise", func() {
		subs := choices.NewSubmissions()
		for _, sub := range s.validBase.Choices {
			if sub.Category == shared.ChoiceExpertise {
				// Only choose 1 instead of 2
				sub.Values = []shared.SelectionID{
					skills.Stealth,
				}
			}
			subs.Add(sub)
		}

		result := s.validator.Validate(reqs, subs)
		s.Assert().False(result.Valid, "Should fail with wrong number of expertise choices")
	})
}

// TestRogueSkills tests that Rogue gets correct skill choices
func (s *RogueCompleteSuite) TestRogueSkills() {
	reqs := choices.GetClassRequirementsAtLevel(classes.Rogue, 1)
	s.Require().NotNil(reqs.Skills)

	// Rogue should choose 4 skills
	s.Assert().Equal(4, reqs.Skills.Count)

	// Check skill list includes expected Rogue skills
	expectedSkills := []skills.Skill{
		skills.Acrobatics,
		skills.Athletics,
		skills.Deception,
		skills.Insight,
		skills.Intimidation,
		skills.Investigation,
		skills.Perception,
		skills.Performance,
		skills.Persuasion,
		skills.SleightOfHand,
		skills.Stealth,
	}

	for _, skill := range expectedSkills {
		s.Assert().Contains(reqs.Skills.Options, skill, "Rogue should have %s as option", skill)
	}
}

// TestRogueExpertise tests that Rogue gets expertise choices
func (s *RogueCompleteSuite) TestRogueExpertise() {
	reqs := choices.GetClassRequirementsAtLevel(classes.Rogue, 1)
	s.Require().NotNil(reqs.Expertise)

	// Rogue should choose 2 for expertise at level 1
	s.Assert().Equal(2, reqs.Expertise.Count)
	s.Assert().Equal(choices.RogueExpertise1, reqs.Expertise.ID)
}

// TestEquipmentChoices tests various equipment combinations
func (s *RogueCompleteSuite) TestEquipmentChoices() {
	reqs := choices.GetClassRequirementsAtLevel(classes.Rogue, 1)

	s.Run("RapierChoice", func() {
		// Already tested in base - rapier
		result := s.validator.Validate(reqs, s.validBase)
		s.Assert().True(result.Valid)
	})

	s.Run("ShortswordChoice", func() {
		subs := choices.NewSubmissions()
		for _, sub := range s.validBase.Choices {
			if sub.ChoiceID == choices.RogueWeaponsPrimary {
				// Choose shortsword
				sub.OptionID = choices.RogueWeaponShortsword
				sub.Values = []shared.SelectionID{
					choices.RogueWeaponShortsword,
				}
			}
			subs.Add(sub)
		}

		result := s.validator.Validate(reqs, subs)
		s.Assert().True(result.Valid, "Should be valid with shortsword choice")
	})

	s.Run("SecondaryShortsword", func() {
		subs := choices.NewSubmissions()
		for _, sub := range s.validBase.Choices {
			if sub.ChoiceID == choices.RogueWeaponsSecondary {
				// Choose shortsword
				sub.OptionID = choices.RogueSecondaryShortsword
				sub.Values = []shared.SelectionID{
					choices.RogueSecondaryShortsword,
				}
			}
			subs.Add(sub)
		}

		result := s.validator.Validate(reqs, subs)
		s.Assert().True(result.Valid, "Should be valid with secondary shortsword")
	})

	s.Run("DungeoneerPack", func() {
		subs := choices.NewSubmissions()
		for _, sub := range s.validBase.Choices {
			if sub.ChoiceID == choices.RoguePack {
				// Choose dungeoneer's pack
				sub.OptionID = choices.RoguePackDungeoneer
				sub.Values = []shared.SelectionID{
					choices.RoguePackDungeoneer,
				}
			}
			subs.Add(sub)
		}

		result := s.validator.Validate(reqs, subs)
		s.Assert().True(result.Valid, "Should be valid with dungeoneer's pack")
	})

	s.Run("ExplorerPack", func() {
		subs := choices.NewSubmissions()
		for _, sub := range s.validBase.Choices {
			if sub.ChoiceID == choices.RoguePack {
				// Choose explorer's pack
				sub.OptionID = choices.RoguePackExplorer
				sub.Values = []shared.SelectionID{
					choices.RoguePackExplorer,
				}
			}
			subs.Add(sub)
		}

		result := s.validator.Validate(reqs, subs)
		s.Assert().True(result.Valid, "Should be valid with explorer's pack")
	})
}

// TestExpertiseOptions tests various expertise combinations
func (s *RogueCompleteSuite) TestExpertiseOptions() {
	reqs := choices.GetClassRequirementsAtLevel(classes.Rogue, 1)

	s.Run("TwoSkills", func() {
		subs := choices.NewSubmissions()
		for _, sub := range s.validBase.Choices {
			if sub.Category == shared.ChoiceExpertise {
				// Choose 2 skills
				sub.Values = []shared.SelectionID{
					skills.Stealth,
					skills.Investigation,
				}
			}
			subs.Add(sub)
		}

		result := s.validator.Validate(reqs, subs)
		s.Assert().True(result.Valid, "Should be valid with 2 skills for expertise")
	})

	s.Run("SkillAndThievesTools", func() {
		// Already tested in base - 1 skill and thieves' tools
		result := s.validator.Validate(reqs, s.validBase)
		s.Assert().True(result.Valid)
	})

	// TODO: Add test for invalid expertise choices once validation is implemented
	// Currently the expertise validation only checks count, not validity of choices
}

// TestRogueCompleteSuite runs the suite
func TestRogueCompleteSuite(t *testing.T) {
	suite.Run(t, new(RogueCompleteSuite))
}
