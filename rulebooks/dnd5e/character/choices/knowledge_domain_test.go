package choices_test

import (
	"encoding/json"
	"testing"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/character/choices"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/classes"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/skills"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestClericKnowledgeDomain(t *testing.T) {
	// Get Cleric requirements with Knowledge Domain
	reqs := choices.GetClassRequirementsWithSubclass(classes.Cleric, 1, classes.KnowledgeDomain)
	require.NotNil(t, reqs)

	// Knowledge Domain should add additional skill requirements
	assert.NotNil(t, reqs.AdditionalSkills, "Should have additional skills for Knowledge Domain")
	assert.Len(t, reqs.AdditionalSkills, 1, "Should have one additional skill requirement")

	if len(reqs.AdditionalSkills) > 0 {
		knowledgeSkills := reqs.AdditionalSkills[0]
		assert.Equal(t, choices.ChoiceID("cleric-knowledge-skills"), knowledgeSkills.ID)
		assert.Equal(t, 2, knowledgeSkills.Count, "Should choose 2 skills")
		assert.Equal(t, "Choose 2 Knowledge Domain skills", knowledgeSkills.Label)

		// Verify the skill options
		expectedSkills := []skills.Skill{
			skills.Arcana,
			skills.History,
			skills.Nature,
			skills.Religion,
		}
		assert.Equal(t, expectedSkills, knowledgeSkills.Options)
	}

	// Knowledge Domain should add language requirements
	assert.NotNil(t, reqs.Languages, "Should have language requirements for Knowledge Domain")
	assert.Len(t, reqs.Languages, 1, "Should have one language requirement")

	if len(reqs.Languages) > 0 {
		knowledgeLanguages := reqs.Languages[0]
		assert.Equal(t, choices.ChoiceID("cleric-knowledge-languages"), knowledgeLanguages.ID)
		assert.Equal(t, 2, knowledgeLanguages.Count, "Should choose 2 languages")
		assert.Equal(t, "Choose 2 languages (Knowledge Domain)", knowledgeLanguages.Label)
		assert.Nil(t, knowledgeLanguages.Options, "Should allow any language (nil options)")
	}
}

func TestClericKnowledgeDomainJSON(t *testing.T) {
	// Get Cleric requirements with Knowledge Domain
	reqs := choices.GetClassRequirementsWithSubclass(classes.Cleric, 1, classes.KnowledgeDomain)
	require.NotNil(t, reqs)

	// Convert to JSON to verify structure
	data, err := json.MarshalIndent(reqs, "", "  ")
	require.NoError(t, err)

	// Print for visual inspection
	t.Logf("Knowledge Domain Cleric Requirements:\n%s", string(data))

	// Verify JSON contains expected fields
	var jsonMap map[string]interface{}
	err = json.Unmarshal(data, &jsonMap)
	require.NoError(t, err)

	// Check for additional_skills field
	additionalSkills, ok := jsonMap["additional_skills"]
	assert.True(t, ok, "Should have additional_skills field in JSON")
	assert.NotNil(t, additionalSkills)

	// Check for languages field
	languages, ok := jsonMap["languages"]
	assert.True(t, ok, "Should have languages field in JSON")
	assert.NotNil(t, languages)
}
