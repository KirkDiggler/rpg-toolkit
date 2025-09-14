package choices_test

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/character/choices"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/classes"
	"github.com/stretchr/testify/suite"
)

// APIValidationSuite validates our implementation against the D&D 5e API
type APIValidationSuite struct {
	suite.Suite

	validator *choices.Validator
	apiData   map[string]interface{}
}

// SetupSuite runs once
func (s *APIValidationSuite) SetupSuite() {
	s.validator = choices.NewValidator()
}

// loadAPIData loads API data for a specific class
func (s *APIValidationSuite) loadAPIData(className string) {
	apiPath := filepath.Join("testdata", "api", "classes", fmt.Sprintf("%s.json", strings.ToLower(className)))
	data, err := os.ReadFile(apiPath)
	if err != nil {
		s.T().Skipf("No API data for %s: %v", className, err)
	}

	s.apiData = make(map[string]interface{})
	if err := json.Unmarshal(data, &s.apiData); err != nil {
		s.T().Fatalf("Failed to parse API data for %s: %v", className, err)
	}
}

// TestFighterAPIValidation validates Fighter against API
func (s *APIValidationSuite) TestFighterAPIValidation() {
	s.loadAPIData("fighter")
	reqs := choices.GetClassRequirementsAtLevel(classes.Fighter, 1)
	s.Require().NotNil(reqs)

	// Validate skills
	if profChoices, ok := s.apiData["proficiency_choices"].([]interface{}); ok && len(profChoices) > 0 {
		if firstChoice, ok := profChoices[0].(map[string]interface{}); ok {
			// API says choose 2 skills
			apiSkillCount := int(firstChoice["choose"].(float64))
			s.Assert().Equal(apiSkillCount, reqs.Skills.Count, "Skill count should match API")

			// Check skill options
			if from, ok := firstChoice["from"].(map[string]interface{}); ok {
				if options, ok := from["options"].([]interface{}); ok {
					s.Assert().Equal(8, len(options), "Fighter should have 8 skill options per API")
					s.Assert().Equal(8, len(reqs.Skills.Options), "Our implementation should have 8 skill options")
				}
			}
		}
	}

	// Validate equipment choices
	if equipOptions, ok := s.apiData["starting_equipment_options"].([]interface{}); ok {
		// API has 4 equipment choices for Fighter
		s.Assert().Equal(4, len(equipOptions), "Fighter should have 4 equipment choices per API")
		s.Assert().Equal(4, len(reqs.Equipment), "Our implementation should have 4 equipment choices")

		// First choice: armor
		if armorChoice, ok := equipOptions[0].(map[string]interface{}); ok {
			desc := armorChoice["desc"].(string)
			s.Assert().Contains(desc, "chain mail", "First choice should include chain mail")
			s.Assert().Contains(desc, "leather armor", "First choice should include leather armor option")
		}
	}

	// Validate hit die
	if hitDie, ok := s.apiData["hit_die"].(float64); ok {
		s.Assert().Equal(10, int(hitDie), "Fighter should have d10 hit die")
	}

	// Validate proficiencies
	if profs, ok := s.apiData["proficiencies"].([]interface{}); ok {
		profNames := make([]string, 0)
		for _, prof := range profs {
			if p, ok := prof.(map[string]interface{}); ok {
				profNames = append(profNames, p["name"].(string))
			}
		}
		s.Assert().Contains(profNames, "All armor")
		s.Assert().Contains(profNames, "Shields")
		s.Assert().Contains(profNames, "Simple Weapons")
		s.Assert().Contains(profNames, "Martial Weapons")
	}
}

// TestBardAPIValidation validates Bard against API
func (s *APIValidationSuite) TestBardAPIValidation() {
	s.loadAPIData("bard")
	reqs := choices.GetClassRequirementsAtLevel(classes.Bard, 1)
	s.Require().NotNil(reqs)

	// Validate skills - Bards choose ANY 3
	if profChoices, ok := s.apiData["proficiency_choices"].([]interface{}); ok && len(profChoices) > 0 {
		if firstChoice, ok := profChoices[0].(map[string]interface{}); ok {
			apiSkillCount := int(firstChoice["choose"].(float64))
			s.Assert().Equal(apiSkillCount, reqs.Skills.Count, "Skill count should match API")
			s.Assert().Equal(3, apiSkillCount, "Bard should choose 3 skills")
			s.Assert().Nil(reqs.Skills.Options, "Bard can choose from ANY skills")
		}
	}

	// Validate musical instrument proficiencies
	if profChoices, ok := s.apiData["proficiency_choices"].([]interface{}); ok && len(profChoices) > 1 {
		if instrumentChoice, ok := profChoices[1].(map[string]interface{}); ok {
			apiInstrumentCount := int(instrumentChoice["choose"].(float64))
			s.Assert().Equal(apiInstrumentCount, reqs.Tools.Count, "Instrument count should match API")
			s.Assert().Equal(3, apiInstrumentCount, "Bard should choose 3 instruments")
		}
	}

	// Validate hit die
	if hitDie, ok := s.apiData["hit_die"].(float64); ok {
		s.Assert().Equal(8, int(hitDie), "Bard should have d8 hit die")
	}

	// Bards are spellcasters
	s.Assert().NotNil(reqs.Cantrips, "Bard should have cantrips")
	s.Assert().NotNil(reqs.Spellbook, "Bard should have spells")
	s.Assert().Equal(2, reqs.Cantrips.Count, "Bard should choose 2 cantrips at level 1")
	s.Assert().Equal(4, reqs.Spellbook.Count, "Bard should know 4 spells at level 1")
}

// TestWizardAPIValidation validates Wizard against API
func (s *APIValidationSuite) TestWizardAPIValidation() {
	s.loadAPIData("wizard")
	reqs := choices.GetClassRequirementsAtLevel(classes.Wizard, 1)
	s.Require().NotNil(reqs)

	// Validate skills
	if profChoices, ok := s.apiData["proficiency_choices"].([]interface{}); ok && len(profChoices) > 0 {
		if firstChoice, ok := profChoices[0].(map[string]interface{}); ok {
			apiSkillCount := int(firstChoice["choose"].(float64))
			s.Assert().Equal(apiSkillCount, reqs.Skills.Count, "Skill count should match API")
		}
	}

	// Validate hit die
	if hitDie, ok := s.apiData["hit_die"].(float64); ok {
		s.Assert().Equal(6, int(hitDie), "Wizard should have d6 hit die")
	}

	// Wizards have cantrips and spellbook
	s.Assert().NotNil(reqs.Cantrips, "Wizard should have cantrips")
	s.Assert().NotNil(reqs.Spellbook, "Wizard should have spellbook")
	s.Assert().Equal(3, reqs.Cantrips.Count, "Wizard should choose 3 cantrips at level 1")
	s.Assert().Equal(6, reqs.Spellbook.Count, "Wizard should choose 6 spells at level 1")
}

// TestRogueAPIValidation validates Rogue against API
func (s *APIValidationSuite) TestRogueAPIValidation() {
	s.loadAPIData("rogue")
	reqs := choices.GetClassRequirementsAtLevel(classes.Rogue, 1)
	s.Require().NotNil(reqs)

	// Validate skills - Rogue chooses 4
	if profChoices, ok := s.apiData["proficiency_choices"].([]interface{}); ok && len(profChoices) > 0 {
		if firstChoice, ok := profChoices[0].(map[string]interface{}); ok {
			apiSkillCount := int(firstChoice["choose"].(float64))
			s.Assert().Equal(apiSkillCount, reqs.Skills.Count, "Skill count should match API")
			s.Assert().Equal(4, apiSkillCount, "Rogue should choose 4 skills")
		}
	}

	// Validate expertise
	s.Assert().NotNil(reqs.Expertise, "Rogue should have expertise at level 1")
	s.Assert().Equal(2, reqs.Expertise.Count, "Rogue should choose 2 for expertise")

	// Validate hit die
	if hitDie, ok := s.apiData["hit_die"].(float64); ok {
		s.Assert().Equal(8, int(hitDie), "Rogue should have d8 hit die")
	}
}

// TestBarbarianAPIValidation validates Barbarian against API
func (s *APIValidationSuite) TestBarbarianAPIValidation() {
	s.loadAPIData("barbarian")
	reqs := choices.GetClassRequirementsAtLevel(classes.Barbarian, 1)
	s.Require().NotNil(reqs)

	// Validate skills
	if profChoices, ok := s.apiData["proficiency_choices"].([]interface{}); ok && len(profChoices) > 0 {
		if firstChoice, ok := profChoices[0].(map[string]interface{}); ok {
			apiSkillCount := int(firstChoice["choose"].(float64))
			s.Assert().Equal(apiSkillCount, reqs.Skills.Count, "Skill count should match API")
			s.Assert().Equal(2, apiSkillCount, "Barbarian should choose 2 skills")
		}
	}

	// Validate hit die
	if hitDie, ok := s.apiData["hit_die"].(float64); ok {
		s.Assert().Equal(12, int(hitDie), "Barbarian should have d12 hit die")
	}

	// Barbarians don't get spells
	s.Assert().Nil(reqs.Cantrips, "Barbarian should not have cantrips")
	s.Assert().Nil(reqs.Spellbook, "Barbarian should not have spellbook")
}

// TestAllClassesHaveAPIData ensures we have API data for all classes
func (s *APIValidationSuite) TestAllClassesHaveAPIData() {
	classNames := []string{
		"barbarian", "bard", "cleric", "druid",
		"fighter", "monk", "paladin", "ranger",
		"rogue", "sorcerer", "warlock", "wizard",
	}

	for _, className := range classNames {
		apiPath := filepath.Join("testdata", "api", "classes", fmt.Sprintf("%s.json", className))
		_, err := os.Stat(apiPath)
		s.Assert().NoError(err, "Should have API data for %s", className)

		// Check file is not empty
		info, _ := os.Stat(apiPath)
		s.Assert().Greater(info.Size(), int64(100), "%s API data should not be empty", className)
	}
}

// TestDruidAPIValidation validates Druid against API
func (s *APIValidationSuite) TestDruidAPIValidation() {
	s.loadAPIData("druid")
	reqs := choices.GetClassRequirementsAtLevel(classes.Druid, 1)
	s.Require().NotNil(reqs)

	// Validate skills - Druids choose 2
	if profChoices, ok := s.apiData["proficiency_choices"].([]interface{}); ok && len(profChoices) > 0 {
		if firstChoice, ok := profChoices[0].(map[string]interface{}); ok {
			apiSkillCount := int(firstChoice["choose"].(float64))
			s.Assert().Equal(apiSkillCount, reqs.Skills.Count, "Skill count should match API")
			s.Assert().Equal(2, apiSkillCount, "Druid should choose 2 skills")
		}
	}

	// Validate hit die
	if hitDie, ok := s.apiData["hit_die"].(float64); ok {
		s.Assert().Equal(8, int(hitDie), "Druid should have d8 hit die")
	}

	// Druids are spellcasters
	s.Assert().NotNil(reqs.Cantrips, "Druid should have cantrips")
	s.Assert().Equal(2, reqs.Cantrips.Count, "Druid should choose 2 cantrips at level 1")
	// Druids prepare spells, don't have a spellbook like Wizards
}

// TestMonkAPIValidation validates Monk against API
func (s *APIValidationSuite) TestMonkAPIValidation() {
	s.loadAPIData("monk")
	reqs := choices.GetClassRequirementsAtLevel(classes.Monk, 1)
	s.Require().NotNil(reqs)

	// Validate skills - Monks choose 2
	if profChoices, ok := s.apiData["proficiency_choices"].([]interface{}); ok && len(profChoices) > 0 {
		if firstChoice, ok := profChoices[0].(map[string]interface{}); ok {
			apiSkillCount := int(firstChoice["choose"].(float64))
			s.Assert().Equal(apiSkillCount, reqs.Skills.Count, "Skill count should match API")
			s.Assert().Equal(2, apiSkillCount, "Monk should choose 2 skills")
		}
	}

	// Validate tool proficiency choice
	s.Assert().NotNil(reqs.Tools, "Monk should have tool proficiency choice")
	s.Assert().Equal(1, reqs.Tools.Count, "Monk should choose 1 tool proficiency")

	// Validate hit die
	if hitDie, ok := s.apiData["hit_die"].(float64); ok {
		s.Assert().Equal(8, int(hitDie), "Monk should have d8 hit die")
	}

	// Monks don't get spells
	s.Assert().Nil(reqs.Cantrips, "Monk should not have cantrips")
	s.Assert().Nil(reqs.Spellbook, "Monk should not have spellbook")
}

// TestPaladinAPIValidation validates Paladin against API
func (s *APIValidationSuite) TestPaladinAPIValidation() {
	s.loadAPIData("paladin")
	reqs := choices.GetClassRequirementsAtLevel(classes.Paladin, 1)
	s.Require().NotNil(reqs)

	// Validate skills - Paladins choose 2
	if profChoices, ok := s.apiData["proficiency_choices"].([]interface{}); ok && len(profChoices) > 0 {
		if firstChoice, ok := profChoices[0].(map[string]interface{}); ok {
			apiSkillCount := int(firstChoice["choose"].(float64))
			s.Assert().Equal(apiSkillCount, reqs.Skills.Count, "Skill count should match API")
			s.Assert().Equal(2, apiSkillCount, "Paladin should choose 2 skills")
		}
	}

	// Validate hit die
	if hitDie, ok := s.apiData["hit_die"].(float64); ok {
		s.Assert().Equal(10, int(hitDie), "Paladin should have d10 hit die")
	}

	// Paladins get spells at level 2, not level 1
	s.Assert().Nil(reqs.Cantrips, "Paladin should not have cantrips at level 1")
	s.Assert().Nil(reqs.Spellbook, "Paladin should not have spells at level 1")
}

// TestRangerAPIValidation validates Ranger against API
func (s *APIValidationSuite) TestRangerAPIValidation() {
	s.loadAPIData("ranger")
	reqs := choices.GetClassRequirementsAtLevel(classes.Ranger, 1)
	s.Require().NotNil(reqs)

	// Validate skills - Rangers choose 3
	if profChoices, ok := s.apiData["proficiency_choices"].([]interface{}); ok && len(profChoices) > 0 {
		if firstChoice, ok := profChoices[0].(map[string]interface{}); ok {
			apiSkillCount := int(firstChoice["choose"].(float64))
			s.Assert().Equal(apiSkillCount, reqs.Skills.Count, "Skill count should match API")
			s.Assert().Equal(3, apiSkillCount, "Ranger should choose 3 skills")
		}
	}

	// Validate fighting style
	s.Assert().NotNil(reqs.FightingStyle, "Ranger should have fighting style choice")
	s.Assert().Equal(4, len(reqs.FightingStyle.Options), "Ranger should have 4 fighting style options")

	// Validate hit die
	if hitDie, ok := s.apiData["hit_die"].(float64); ok {
		s.Assert().Equal(10, int(hitDie), "Ranger should have d10 hit die")
	}

	// Rangers get spells at level 2, not level 1
	s.Assert().Nil(reqs.Cantrips, "Ranger should not have cantrips at level 1")
	s.Assert().Nil(reqs.Spellbook, "Ranger should not have spells at level 1")
}

// TestSorcererAPIValidation validates Sorcerer against API
func (s *APIValidationSuite) TestSorcererAPIValidation() {
	s.loadAPIData("sorcerer")
	reqs := choices.GetClassRequirementsAtLevel(classes.Sorcerer, 1)
	s.Require().NotNil(reqs)

	// Validate skills - Sorcerers choose 2
	if profChoices, ok := s.apiData["proficiency_choices"].([]interface{}); ok && len(profChoices) > 0 {
		if firstChoice, ok := profChoices[0].(map[string]interface{}); ok {
			apiSkillCount := int(firstChoice["choose"].(float64))
			s.Assert().Equal(apiSkillCount, reqs.Skills.Count, "Skill count should match API")
			s.Assert().Equal(2, apiSkillCount, "Sorcerer should choose 2 skills")
		}
	}

	// Validate hit die
	if hitDie, ok := s.apiData["hit_die"].(float64); ok {
		s.Assert().Equal(6, int(hitDie), "Sorcerer should have d6 hit die")
	}

	// Sorcerers are spellcasters
	s.Assert().NotNil(reqs.Cantrips, "Sorcerer should have cantrips")
	s.Assert().NotNil(reqs.Spellbook, "Sorcerer should have spells")
	s.Assert().Equal(4, reqs.Cantrips.Count, "Sorcerer should choose 4 cantrips at level 1")
	s.Assert().Equal(2, reqs.Spellbook.Count, "Sorcerer should know 2 spells at level 1")
}

// TestWarlockAPIValidation validates Warlock against API
func (s *APIValidationSuite) TestWarlockAPIValidation() {
	s.loadAPIData("warlock")
	reqs := choices.GetClassRequirementsAtLevel(classes.Warlock, 1)
	s.Require().NotNil(reqs)

	// Validate skills - Warlocks choose 2
	if profChoices, ok := s.apiData["proficiency_choices"].([]interface{}); ok && len(profChoices) > 0 {
		if firstChoice, ok := profChoices[0].(map[string]interface{}); ok {
			apiSkillCount := int(firstChoice["choose"].(float64))
			s.Assert().Equal(apiSkillCount, reqs.Skills.Count, "Skill count should match API")
			s.Assert().Equal(2, apiSkillCount, "Warlock should choose 2 skills")
		}
	}

	// Validate hit die
	if hitDie, ok := s.apiData["hit_die"].(float64); ok {
		s.Assert().Equal(8, int(hitDie), "Warlock should have d8 hit die")
	}

	// Warlocks are spellcasters
	s.Assert().NotNil(reqs.Cantrips, "Warlock should have cantrips")
	s.Assert().NotNil(reqs.Spellbook, "Warlock should have spells")
	s.Assert().Equal(2, reqs.Cantrips.Count, "Warlock should choose 2 cantrips at level 1")
	s.Assert().Equal(2, reqs.Spellbook.Count, "Warlock should know 2 spells at level 1")
}

// TestAPIValidationSuite runs the suite
func TestAPIValidationSuite(t *testing.T) {
	suite.Run(t, new(APIValidationSuite))
}
