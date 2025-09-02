package choices_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/character/choices"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/classes"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/shared"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/weapons"
)

type ValidationTestSuite struct {
	suite.Suite
	validator *choices.Validator
}

func (s *ValidationTestSuite) SetupTest() {
	s.validator = choices.NewValidator()
}

func (s *ValidationTestSuite) TestValidateEquipmentCategory() {
	// Get fighter requirements
	reqs := choices.GetClassRequirements(classes.Fighter)
	require.NotNil(s.T(), reqs)
	require.NotEmpty(s.T(), reqs.EquipmentCategories)

	// Create submissions for equipment category choice
	submissions := choices.NewSubmissions()

	// Test case 1: No submission for required category choice
	result := s.validator.Validate(reqs, submissions)
	assert.False(s.T(), result.Valid, "Should be invalid when category choice is missing")
	assert.NotEmpty(s.T(), result.Errors, "Should have validation errors")

	// Test case 2: Valid martial weapon selection
	submissions.Add(choices.Submission{
		Category: shared.ChoiceEquipment,
		Source:   shared.SourceClass,
		ChoiceID: choices.FighterMartialWeapon1,
		Values:   []string{string(weapons.Longsword)}, // Valid martial weapon
	})

	result = s.validator.Validate(reqs, submissions)
	// Will still be invalid because other required choices are missing,
	// but should not have errors for this specific choice
	foundError := false
	for _, err := range result.Errors {
		if err.ChoiceID == choices.FighterMartialWeapon1 {
			foundError = true
			break
		}
	}
	assert.False(s.T(), foundError, "Should not have error for valid martial weapon choice")

	// Test case 3: Invalid weapon (not martial)
	badSubmissions := choices.NewSubmissions()
	badSubmissions.Add(choices.Submission{
		Category: shared.ChoiceEquipment,
		Source:   shared.SourceClass,
		ChoiceID: choices.FighterMartialWeapon1,
		Values:   []string{string(weapons.Club)}, // Simple weapon, not martial
	})

	result = s.validator.Validate(reqs, badSubmissions)
	assert.False(s.T(), result.Valid, "Should be invalid with non-martial weapon")
	foundError = false
	for _, err := range result.Errors {
		if err.ChoiceID == choices.FighterMartialWeapon1 {
			foundError = true
			assert.Contains(s.T(), err.Message, "must be from specified categories")
			break
		}
	}
	assert.True(s.T(), foundError, "Should have error for invalid weapon category")

	// Test case 4: Wrong number of choices
	wrongCountSubmissions := choices.NewSubmissions()
	wrongCountSubmissions.Add(choices.Submission{
		Category: shared.ChoiceEquipment,
		Source:   shared.SourceClass,
		ChoiceID: choices.FighterMartialWeapon1,
		Values: []string{
			string(weapons.Longsword),
			string(weapons.Shortsword),
		}, // Too many choices
	})

	result = s.validator.Validate(reqs, wrongCountSubmissions)
	assert.False(s.T(), result.Valid, "Should be invalid with wrong number of choices")
	foundError = false
	for _, err := range result.Errors {
		if err.ChoiceID == choices.FighterMartialWeapon1 {
			foundError = true
			assert.Contains(s.T(), err.Message, "Must choose exactly")
			break
		}
	}
	assert.True(s.T(), foundError, "Should have error for wrong number of choices")
}

func (s *ValidationTestSuite) TestValidateEquipmentBundles() {
	// Get fighter requirements
	reqs := choices.GetClassRequirements(classes.Fighter)
	require.NotNil(s.T(), reqs)
	require.NotEmpty(s.T(), reqs.Equipment)

	// Find the armor choice requirement
	var armorReq *choices.EquipmentRequirement
	for _, req := range reqs.Equipment {
		if req.ID == choices.FighterArmor {
			armorReq = req
			break
		}
	}
	require.NotNil(s.T(), armorReq, "Should have fighter armor requirement")

	// Create submissions
	submissions := choices.NewSubmissions()

	// Test case 1: Valid armor bundle choice
	submissions.Add(choices.Submission{
		Category: shared.ChoiceEquipment,
		Source:   shared.SourceClass,
		ChoiceID: choices.FighterArmor,
		Values:   []string{"fighter-armor-b"}, // Leather + bow + arrows bundle
	})

	// Create a requirement with just armor for isolated testing
	isolatedReq := &choices.Requirements{
		Equipment: []*choices.EquipmentRequirement{armorReq},
	}

	result := s.validator.Validate(isolatedReq, submissions)
	assert.True(s.T(), result.Valid, "Should be valid with proper bundle selection")
	assert.Empty(s.T(), result.Errors, "Should have no errors")
}

func (s *ValidationTestSuite) TestDuplicateWeaponChoice() {
	// Create a requirement that allows choosing 2 martial weapons
	req := &choices.Requirements{
		EquipmentCategories: []*choices.EquipmentCategoryRequirement{
			{
				ID:     "test-two-martial",
				Choose: 2,
				Type:   shared.EquipmentTypeWeapon,
				Categories: []shared.EquipmentCategory{
					weapons.CategoryMartialMelee,
					weapons.CategoryMartialRanged,
				},
				Label: "Choose 2 martial weapons",
			},
		},
	}

	// Test case 1: Same weapon chosen twice
	submissions := choices.NewSubmissions()
	submissions.Add(choices.Submission{
		Category: shared.ChoiceEquipment,
		Source:   shared.SourceClass,
		ChoiceID: "test-two-martial",
		Values: []string{
			string(weapons.Longsword),
			string(weapons.Longsword), // Duplicate
		},
	})

	result := s.validator.Validate(req, submissions)
	assert.False(s.T(), result.Valid, "Should be invalid with duplicate weapons")
	assert.NotEmpty(s.T(), result.Errors)
	assert.Contains(s.T(), result.Errors[0].Message, "Cannot choose the same item")

	// Test case 2: Different weapons chosen
	goodSubmissions := choices.NewSubmissions()
	goodSubmissions.Add(choices.Submission{
		Category: shared.ChoiceEquipment,
		Source:   shared.SourceClass,
		ChoiceID: "test-two-martial",
		Values: []string{
			string(weapons.Longsword),
			string(weapons.Shortsword), // Different weapon
		},
	})

	result = s.validator.Validate(req, goodSubmissions)
	assert.True(s.T(), result.Valid, "Should be valid with different weapons")
	assert.Empty(s.T(), result.Errors)
}

func TestValidationSuite(t *testing.T) {
	suite.Run(t, new(ValidationTestSuite))
}
