package choices_test

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/character/choices"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/races"
)

// RaceAPIValidationSuite validates our race implementation against the D&D 5e API
type RaceAPIValidationSuite struct {
	suite.Suite

	validator *choices.Validator
	apiData   map[string]interface{}
}

// SetupSuite runs once
func (s *RaceAPIValidationSuite) SetupSuite() {
	s.validator = choices.NewValidator()
}

// loadRaceAPIData loads API data for a specific race
func (s *RaceAPIValidationSuite) loadRaceAPIData(raceName string) {
	apiPath := filepath.Join("testdata", "api", "races", fmt.Sprintf("%s.json", strings.ToLower(raceName)))
	data, err := os.ReadFile(apiPath) //nolint:gosec
	if err != nil {
		s.T().Skipf("No API data for %s: %v", raceName, err)
	}

	s.apiData = make(map[string]interface{})
	if err := json.Unmarshal(data, &s.apiData); err != nil {
		s.T().Fatalf("Failed to parse API data for %s: %v", raceName, err)
	}
}

// TestDwarfAPIValidation validates Dwarf against API
func (s *RaceAPIValidationSuite) TestDwarfAPIValidation() {
	s.loadRaceAPIData("dwarf")
	reqs := choices.GetRaceRequirements(races.Dwarf)
	s.Require().NotNil(reqs)

	// Validate tool proficiency choice
	if profOptions, ok := s.apiData["starting_proficiency_options"].(map[string]interface{}); ok {
		apiToolCount := int(profOptions["choose"].(float64))
		s.Assert().NotNil(reqs.Tools, "Dwarf should have tool choices")
		if reqs.Tools != nil {
			s.Assert().Equal(apiToolCount, reqs.Tools.Count, "Tool proficiency count should match API")
			s.Assert().Equal(1, apiToolCount, "Dwarf should choose 1 tool proficiency")

			// Check tool options
			if from, ok := profOptions["from"].(map[string]interface{}); ok {
				if options, ok := from["options"].([]interface{}); ok {
					s.Assert().Equal(3, len(options), "Dwarf should have 3 tool options per API")
					s.Assert().Equal(3, len(reqs.Tools.Options), "Our implementation should have 3 tool options")
				}
			}
		}
	}

	// Validate ability scores
	if abilityBonuses, ok := s.apiData["ability_bonuses"].([]interface{}); ok {
		s.Assert().Equal(1, len(abilityBonuses), "Dwarf should have 1 ability bonus")
		if bonus, ok := abilityBonuses[0].(map[string]interface{}); ok {
			if score, ok := bonus["ability_score"].(map[string]interface{}); ok {
				s.Assert().Equal("con", score["index"], "Dwarf should have CON bonus")
			}
			s.Assert().Equal(float64(2), bonus["bonus"], "Dwarf should have +2 CON")
		}
	}

	// Validate speed
	if speed, ok := s.apiData["speed"].(float64); ok {
		s.Assert().Equal(25, int(speed), "Dwarf should have 25 feet speed")
	}

	// Validate languages
	if langs, ok := s.apiData["languages"].([]interface{}); ok {
		s.Assert().Equal(2, len(langs), "Dwarf should know 2 languages")
		langNames := make([]string, 0)
		for _, lang := range langs {
			if l, ok := lang.(map[string]interface{}); ok {
				langNames = append(langNames, l["name"].(string))
			}
		}
		s.Assert().Contains(langNames, "Common")
		s.Assert().Contains(langNames, "Dwarvish")
	}
}

// TestElfAPIValidation validates Elf against API
func (s *RaceAPIValidationSuite) TestElfAPIValidation() {
	s.loadRaceAPIData("elf")
	reqs := choices.GetRaceRequirements(races.Elf)
	s.Require().NotNil(reqs)

	// Validate ability scores
	if abilityBonuses, ok := s.apiData["ability_bonuses"].([]interface{}); ok {
		s.Assert().Equal(1, len(abilityBonuses), "Elf should have 1 ability bonus")
		if bonus, ok := abilityBonuses[0].(map[string]interface{}); ok {
			if score, ok := bonus["ability_score"].(map[string]interface{}); ok {
				s.Assert().Equal("dex", score["index"], "Elf should have DEX bonus")
			}
			s.Assert().Equal(float64(2), bonus["bonus"], "Elf should have +2 DEX")
		}
	}

	// Validate speed
	if speed, ok := s.apiData["speed"].(float64); ok {
		s.Assert().Equal(30, int(speed), "Elf should have 30 feet speed")
	}

	// Validate starting proficiencies (Perception skill)
	if profs, ok := s.apiData["starting_proficiencies"].([]interface{}); ok {
		s.Assert().Equal(1, len(profs), "Elf should have 1 starting proficiency")
		if prof, ok := profs[0].(map[string]interface{}); ok {
			s.Assert().Equal("skill-perception", prof["index"], "Elf should have Perception proficiency")
		}
	}

	// Validate languages
	if langs, ok := s.apiData["languages"].([]interface{}); ok {
		s.Assert().Equal(2, len(langs), "Elf should know 2 languages")
		langNames := make([]string, 0)
		for _, lang := range langs {
			if l, ok := lang.(map[string]interface{}); ok {
				langNames = append(langNames, l["name"].(string))
			}
		}
		s.Assert().Contains(langNames, "Common")
		s.Assert().Contains(langNames, "Elvish")
	}
}

// TestHumanAPIValidation validates Human against API
func (s *RaceAPIValidationSuite) TestHumanAPIValidation() {
	s.loadRaceAPIData("human")
	reqs := choices.GetRaceRequirements(races.Human)
	s.Require().NotNil(reqs)

	// Validate ability scores - humans get +1 to all
	if abilityBonuses, ok := s.apiData["ability_bonuses"].([]interface{}); ok {
		s.Assert().Equal(6, len(abilityBonuses), "Human should have 6 ability bonuses")
		for _, bonus := range abilityBonuses {
			if b, ok := bonus.(map[string]interface{}); ok {
				s.Assert().Equal(float64(1), b["bonus"], "Human should have +1 to each ability")
			}
		}
	}

	// Validate speed
	if speed, ok := s.apiData["speed"].(float64); ok {
		s.Assert().Equal(30, int(speed), "Human should have 30 feet speed")
	}

	// Validate language choice
	if langOptions, ok := s.apiData["language_options"].(map[string]interface{}); ok {
		apiLangCount := int(langOptions["choose"].(float64))
		s.Assert().NotNil(reqs.Languages, "Human should have language choices")
		if len(reqs.Languages) > 0 {
			s.Assert().Equal(apiLangCount, reqs.Languages[0].Count, "Language count should match API")
			s.Assert().Equal(1, apiLangCount, "Human should choose 1 extra language")

			// Check language options
			if from, ok := langOptions["from"].(map[string]interface{}); ok {
				if options, ok := from["options"].([]interface{}); ok {
					s.Assert().Equal(15, len(options), "Human should have 15 language options per API")
					s.Assert().Equal(15, len(reqs.Languages[0].Options), "Our implementation should have 15 language options")
				}
			}
		}
	}
}

// TestHalflingAPIValidation validates Halfling against API
func (s *RaceAPIValidationSuite) TestHalflingAPIValidation() {
	s.loadRaceAPIData("halfling")
	reqs := choices.GetRaceRequirements(races.Halfling)
	s.Require().NotNil(reqs)

	// Load API data to check basic race properties
	apiPath := filepath.Join("testdata", "api", "races", "halfling.json")
	_, err := os.Stat(apiPath)
	if err != nil {
		s.T().Skipf("No API data for halfling: %v", err)
		return
	}

	// Validate ability scores
	if abilityBonuses, ok := s.apiData["ability_bonuses"].([]interface{}); ok {
		s.Assert().Equal(1, len(abilityBonuses), "Halfling should have 1 ability bonus")
		if bonus, ok := abilityBonuses[0].(map[string]interface{}); ok {
			if score, ok := bonus["ability_score"].(map[string]interface{}); ok {
				s.Assert().Equal("dex", score["index"], "Halfling should have DEX bonus")
			}
			s.Assert().Equal(float64(2), bonus["bonus"], "Halfling should have +2 DEX")
		}
	}

	// Validate speed
	if speed, ok := s.apiData["speed"].(float64); ok {
		s.Assert().Equal(25, int(speed), "Halfling should have 25 feet speed")
	}
}

// TestAllRacesHaveAPIData ensures we have API data for all races
func (s *RaceAPIValidationSuite) TestAllRacesHaveAPIData() {
	raceNames := []string{
		"dragonborn", "dwarf", "elf", "gnome",
		"half-elf", "half-orc", "halfling", "human", "tiefling",
	}

	for _, raceName := range raceNames {
		apiPath := filepath.Join("testdata", "api", "races", fmt.Sprintf("%s.json", raceName))
		_, err := os.Stat(apiPath)
		s.Assert().NoError(err, "Should have API data for %s", raceName)

		// Check file is not empty
		info, _ := os.Stat(apiPath)
		s.Assert().Greater(info.Size(), int64(100), "%s API data should not be empty", raceName)
	}
}

// TestRaceAPIValidationSuite runs the suite
func TestRaceAPIValidationSuite(t *testing.T) {
	suite.Run(t, new(RaceAPIValidationSuite))
}
