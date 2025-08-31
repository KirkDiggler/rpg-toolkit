package choices_test

import (
	"testing"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/character/choices"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/classes"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/skills"
	"github.com/stretchr/testify/suite"
)

type NatureDomainTestSuite struct {
	suite.Suite
}

func TestNatureDomainTestSuite(t *testing.T) {
	suite.Run(t, new(NatureDomainTestSuite))
}

func (s *NatureDomainTestSuite) TestNatureDomainRequirements() {
	// Nature Domain should get:
	// - 2 base Cleric skills
	// - 1 additional skill from Animal Handling, Nature, or Survival
	// - 1 additional druid cantrip (4 total cantrips)

	reqs := choices.GetSubclassRequirements(classes.NatureDomain)

	s.NotNil(reqs)
	s.NotNil(reqs.Skills)
	s.NotNil(reqs.Cantrips)

	// Should require 3 skills total (2 base + 1 nature domain)
	s.Equal(3, reqs.Skills.Count, "Nature Domain should require 3 skills")

	// Should have the Nature Domain specific skills in options
	hasAnimalHandling := false
	hasNature := false
	hasSurvival := false
	for _, skill := range reqs.Skills.Options {
		switch skill {
		case skills.AnimalHandling:
			hasAnimalHandling = true
		case skills.Nature:
			hasNature = true
		case skills.Survival:
			hasSurvival = true
		}
	}
	s.True(hasAnimalHandling, "Should have Animal Handling as option")
	s.True(hasNature, "Should have Nature as option")
	s.True(hasSurvival, "Should have Survival as option")

	// Should require 4 cantrips (3 base + 1 druid)
	s.Equal(4, reqs.Cantrips.Count, "Nature Domain should require 4 cantrips")
	s.Contains(reqs.Cantrips.Label, "Druid", "Label should mention Druid cantrip")
}

func (s *NatureDomainTestSuite) TestNatureDomainValidation() {
	context := choices.NewValidationContext()
	submissions := choices.NewTypedSubmissions()

	// Add 3 skills - 2 from base Cleric list + 1 from Nature Domain list
	submissions.AddChoice(choices.ChoiceSubmission{
		Source:   choices.SourceClass,
		Field:    choices.FieldSkills,
		ChoiceID: "cleric_skills",
		Values: []string{
			string(skills.Insight),
			string(skills.Medicine),
			string(skills.Nature), // Nature Domain bonus
		},
	})

	// Add 4 cantrips
	submissions.AddChoice(choices.ChoiceSubmission{
		Source:   choices.SourceClass,
		Field:    choices.FieldCantrips,
		ChoiceID: "cleric_cantrips",
		Values:   []string{"sacred_flame", "guidance", "thaumaturgy", "druidcraft"},
	})

	// Add equipment choices
	for i := 0; i < 4; i++ {
		submissions.AddChoice(choices.ChoiceSubmission{
			Source:   choices.SourceClass,
			Field:    choices.Field("equipment_choice_" + string(rune('0'+i))),
			ChoiceID: "equipment_" + string(rune('0'+i)),
			Values:   []string{"mace", "scale-mail", "light-crossbow-set", "priests-pack"}[i : i+1],
		})
	}

	// Validate with Nature Domain subclass
	result := choices.ValidateClassChoicesWithSubclass(
		classes.Cleric,
		classes.NatureDomain,
		1,
		submissions,
		context,
	)

	// Should be valid
	s.True(result.CanFinalize, "Nature Domain with 3 skills and 4 cantrips should be valid")

	// Check for errors
	for _, issue := range result.AllIssues {
		if issue.Severity == choices.SeverityError {
			s.Fail("Should not have errors: Field=%v, Message=%s", issue.Field, issue.Message)
		}
	}
}
