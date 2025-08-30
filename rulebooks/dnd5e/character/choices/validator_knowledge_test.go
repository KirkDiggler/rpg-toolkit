package choices_test

import (
	"testing"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/character/choices"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/classes"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/races"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/skills"
	"github.com/stretchr/testify/suite"
)

type KnowledgeDomainTestSuite struct {
	suite.Suite
}

func TestKnowledgeDomainTestSuite(t *testing.T) {
	suite.Run(t, new(KnowledgeDomainTestSuite))
}

func (s *KnowledgeDomainTestSuite) TestValidateClassChoicesWithSubclass_KnowledgeDomain() {
	// Knowledge Domain Clerics should be able to choose 4 skills total:
	// - 2 from base Cleric list
	// - 2 additional from: Arcana, History, Nature, or Religion

	context := choices.NewValidationContext()
	submissions := choices.NewTypedSubmissions()

	// Add 4 skills as a Knowledge Domain Cleric would
	submissions.AddChoice(choices.ChoiceSubmission{
		Source:   choices.SourceClass,
		Field:    choices.FieldSkills,
		ChoiceID: "cleric_skills",
		Values: []string{
			string(skills.Insight),
			string(skills.Medicine),
			string(skills.Arcana),  // Knowledge Domain bonus
			string(skills.History), // Knowledge Domain bonus
		},
	})

	// Add required equipment choices for Cleric
	// Choice 0: weapon (mace or warhammer)
	submissions.AddChoice(choices.ChoiceSubmission{
		Source:   choices.SourceClass,
		Field:    choices.Field("equipment_choice_0"),
		ChoiceID: "equipment_0",
		Values:   []string{"mace"},
	})
	// Choice 1: armor (scale-mail, leather-armor, or chain-mail)
	submissions.AddChoice(choices.ChoiceSubmission{
		Source:   choices.SourceClass,
		Field:    choices.Field("equipment_choice_1"),
		ChoiceID: "equipment_1",
		Values:   []string{"scale-mail"},
	})
	// Choice 2: secondary weapon (light-crossbow-set or any-simple)
	submissions.AddChoice(choices.ChoiceSubmission{
		Source:   choices.SourceClass,
		Field:    choices.Field("equipment_choice_2"),
		ChoiceID: "equipment_2",
		Values:   []string{"light-crossbow-set"},
	})
	// Choice 3: pack (priests-pack or explorers-pack)
	submissions.AddChoice(choices.ChoiceSubmission{
		Source:   choices.SourceClass,
		Field:    choices.Field("equipment_choice_3"),
		ChoiceID: "equipment_3",
		Values:   []string{"priests-pack"},
	})

	// Add required cantrips (3 for Cleric)
	submissions.AddChoice(choices.ChoiceSubmission{
		Source:   choices.SourceClass,
		Field:    choices.FieldCantrips,
		ChoiceID: "cleric_cantrips",
		Values:   []string{"sacred_flame", "guidance", "thaumaturgy"},
	})

	// Add required languages (2 for Knowledge Domain)
	submissions.AddChoice(choices.ChoiceSubmission{
		Source:   choices.SourceClass,
		Field:    choices.FieldLanguages,
		ChoiceID: "knowledge_languages",
		Values:   []string{"elvish", "dwarvish"},
	})

	// Validate with subclass
	result := choices.ValidateClassChoicesWithSubclass(
		classes.Cleric,
		classes.KnowledgeDomain,
		1, // level
		submissions,
		context,
	)

	// Should be valid - Knowledge Domain allows 4 skills
	s.True(result.CanFinalize, "Knowledge Domain Cleric with 4 skills should be valid")
	
	// Check for skill errors and debug
	for _, issue := range result.AllIssues {
		s.T().Logf("Issue: Field=%v, Severity=%v, Message=%s", issue.Field, issue.Severity, issue.Message)
		if issue.Field == choices.FieldSkills && issue.Severity == choices.SeverityError {
			s.Fail("Should not have skill errors for Knowledge Domain with 4 skills: %s", issue.Message)
		}
	}
}

func (s *KnowledgeDomainTestSuite) TestValidateClassChoicesWithSubclass_RegularCleric() {
	// Regular Cleric (no subclass yet) should only be able to choose 2 skills

	context := choices.NewValidationContext()
	submissions := choices.NewTypedSubmissions()

	// Try to add 4 skills as a regular Cleric
	submissions.AddChoice(choices.ChoiceSubmission{
		Source:   choices.SourceClass,
		Field:    choices.FieldSkills,
		ChoiceID: "cleric_skills",
		Values: []string{
			string(skills.Insight),
			string(skills.Medicine),
			string(skills.History),
			string(skills.Religion),
		},
	})

	// Validate without subclass
	result := choices.ValidateClassChoices(
		classes.Cleric,
		1, // level
		submissions,
		context,
	)

	// Should have an error - regular Cleric only allows 2 skills
	s.False(result.CanFinalize, "Regular Cleric with 4 skills should be invalid")
	
	// Check for skill errors
	hasSkillError := false
	for _, issue := range result.AllIssues {
		if issue.Field == choices.FieldSkills && issue.Severity == choices.SeverityError {
			hasSkillError = true
			s.Contains(issue.Message, "2 skills")
			s.Contains(issue.Message, "4")
		}
	}
	s.True(hasSkillError, "Should have skill error for regular Cleric with 4 skills")
}

func (s *KnowledgeDomainTestSuite) TestValidateAllWithSubclass_CompleteKnowledgeDomainCharacter() {
	// Test complete validation with Knowledge Domain subclass
	
	context := choices.NewValidationContext()
	submissions := choices.NewTypedSubmissions()

	// Class choices - Knowledge Domain gets 4 skills
	submissions.AddChoice(choices.ChoiceSubmission{
		Source:   choices.SourceClass,
		Field:    choices.FieldSkills,
		ChoiceID: "cleric_skills",
		Values: []string{
			string(skills.Insight),
			string(skills.Medicine),
			string(skills.Arcana),
			string(skills.History),
		},
	})

	// Equipment choices
	submissions.AddChoice(choices.ChoiceSubmission{
		Source:   choices.SourceClass,
		Field:    choices.Field("equipment_choice_0"),
		ChoiceID: "equipment_0",
		Values:   []string{"mace"},
	})
	submissions.AddChoice(choices.ChoiceSubmission{
		Source:   choices.SourceClass,
		Field:    choices.Field("equipment_choice_1"),
		ChoiceID: "equipment_1",
		Values:   []string{"scale-mail"},
	})
	submissions.AddChoice(choices.ChoiceSubmission{
		Source:   choices.SourceClass,
		Field:    choices.Field("equipment_choice_2"),
		ChoiceID: "equipment_2",
		Values:   []string{"light-crossbow-set"},
	})
	submissions.AddChoice(choices.ChoiceSubmission{
		Source:   choices.SourceClass,
		Field:    choices.Field("equipment_choice_3"),
		ChoiceID: "equipment_3",
		Values:   []string{"priests-pack"},
	})

	// Cantrips
	submissions.AddChoice(choices.ChoiceSubmission{
		Source:   choices.SourceClass,
		Field:    choices.FieldCantrips,
		ChoiceID: "cleric_cantrips",
		Values:   []string{"sacred_flame", "guidance", "thaumaturgy"},
	})

	// Languages for Knowledge Domain
	submissions.AddChoice(choices.ChoiceSubmission{
		Source:   choices.SourceClass,
		Field:    choices.FieldLanguages,
		ChoiceID: "knowledge_languages",
		Values:   []string{"elvish", "dwarvish"},
	})

	// Race choices (Human for simplicity - no choices needed)
	// Background skills would be separate

	// Validate the complete character
	result := choices.ValidateWithSubclass(
		classes.Cleric,
		classes.KnowledgeDomain,
		races.Human,
		"", // background not relevant for this test
		1,  // level
		submissions,
		context,
	)

	// Should be valid
	s.True(result.CanFinalize, "Complete Knowledge Domain character should be valid")
	
	// No skill errors
	for _, issue := range result.AllIssues {
		if issue.Field == choices.FieldSkills && issue.Severity == choices.SeverityError {
			s.Fail("Should not have skill errors: %s", issue.Message)
		}
	}
}