package character

import (
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/class"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/constants"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/race"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/shared"
)

// DraftConversionTestSuite comprehensively tests the draft-to-character conversion process
type DraftConversionTestSuite struct {
	suite.Suite
	humanRace    *race.Data
	elfRace      *race.Data
	fighterClass *class.Data
	wizardClass  *class.Data
	soldierBg    *shared.Background
	hermitBg     *shared.Background
}

func (s *DraftConversionTestSuite) SetupTest() {
	// Setup Human race
	s.humanRace = &race.Data{
		ID:    "human",
		Name:  "Human",
		Size:  "Medium",
		Speed: 30,
		AbilityScoreIncreases: map[string]int{
			shared.AbilityStrength:     1,
			shared.AbilityDexterity:    1,
			shared.AbilityConstitution: 1,
			shared.AbilityIntelligence: 1,
			shared.AbilityWisdom:       1,
			shared.AbilityCharisma:     1,
		},
		Languages: []string{"Common"},
	}

	// Setup Elf race with subraces
	s.elfRace = &race.Data{
		ID:    "elf",
		Name:  "Elf",
		Size:  "Medium",
		Speed: 30,
		AbilityScoreIncreases: map[string]int{
			shared.AbilityDexterity: 2,
		},
		Languages:           []string{"Common", "Elvish"},
		WeaponProficiencies: []string{"Longsword", "Shortsword", "Shortbow", "Longbow"},
		Subraces: []race.SubraceData{
			{
				ID:   "high-elf",
				Name: "High Elf",
				AbilityScoreIncreases: map[string]int{
					shared.AbilityIntelligence: 1,
				},
			},
			{
				ID:   "wood-elf",
				Name: "Wood Elf",
				AbilityScoreIncreases: map[string]int{
					shared.AbilityWisdom: 1,
				},
			},
		},
	}

	// Setup Fighter class
	s.fighterClass = &class.Data{
		ID:                    "fighter",
		Name:                  "Fighter",
		HitDice:               10,
		SavingThrows:          []string{shared.AbilityStrength, shared.AbilityConstitution},
		SkillProficiencyCount: 2,
		SkillOptions: []string{
			"Acrobatics", "Animal Handling", "Athletics", "History",
			"Insight", "Intimidation", "Perception", "Survival",
		},
		ArmorProficiencies:  []string{"Light", "Medium", "Heavy", "Shield"},
		WeaponProficiencies: []string{"Simple", "Martial"},
		ToolProficiencies:   []string{},
	}

	// Setup Wizard class
	s.wizardClass = &class.Data{
		ID:                    "wizard",
		Name:                  "Wizard",
		HitDice:               6,
		SavingThrows:          []string{shared.AbilityIntelligence, shared.AbilityWisdom},
		SkillProficiencyCount: 2,
		SkillOptions: []string{
			"Arcana", "History", "Insight", "Investigation", "Medicine", "Religion",
		},
		ArmorProficiencies:  []string{},
		WeaponProficiencies: []string{"Dagger", "Dart", "Sling", "Quarterstaff", "Light Crossbow"},
		ToolProficiencies:   []string{},
	}

	// Setup Soldier background
	s.soldierBg = &shared.Background{
		ID:                 "soldier",
		Name:               "Soldier",
		SkillProficiencies: []string{"Athletics", "Intimidation"},
		Languages:          []string{"Orc"},
		ToolProficiencies:  []string{"Gaming set", "Land vehicles"},
	}

	// Setup Hermit background
	s.hermitBg = &shared.Background{
		ID:                 "hermit",
		Name:               "Hermit",
		SkillProficiencies: []string{"Medicine", "Religion"},
		Languages:          []string{"Celestial"},
		ToolProficiencies:  []string{"Herbalism kit"},
	}
}

func (s *DraftConversionTestSuite) TestCompleteHumanFighterConversion() {
	// Create a complete draft for a Human Fighter
	draft := &Draft{
		ID:       "test-human-fighter",
		PlayerID: "player-123",
		Name:     "Garen the Bold",
		RaceChoice: RaceChoice{
			RaceID: "human",
		},
		ClassChoice:      "fighter",
		BackgroundChoice: "soldier",
		AbilityScoreChoice: shared.AbilityScores{
			constants.STR: 15,
			constants.DEX: 13,
			constants.CON: 14,
			constants.INT: 10,
			constants.WIS: 12,
			constants.CHA: 8,
		},
		SkillChoices:    []string{"Perception", "Survival"},
		LanguageChoices: []string{"Dwarvish", "Giant"},
		Progress: DraftProgress{
			flags: ProgressName | ProgressRace | ProgressClass | ProgressBackground | ProgressAbilityScores,
		},
	}

	// Convert to character
	character, err := draft.ToCharacter(s.humanRace, s.fighterClass, s.soldierBg)
	s.Require().NoError(err)
	s.Require().NotNil(character)

	// Verify basic info
	s.Assert().Equal("test-human-fighter", character.id)
	s.Assert().Equal("player-123", character.playerID)
	s.Assert().Equal("Garen the Bold", character.name)
	s.Assert().Equal(1, character.level)
	s.Assert().Equal("human", character.raceID)
	s.Assert().Equal("fighter", character.classID)
	s.Assert().Equal("soldier", character.backgroundID)

	// Verify ability scores with Human racial bonuses (+1 to all)
	s.Assert().Equal(16, character.abilityScores[constants.STR]) // 15 + 1
	s.Assert().Equal(14, character.abilityScores[constants.DEX]) // 13 + 1
	s.Assert().Equal(15, character.abilityScores[constants.CON]) // 14 + 1
	s.Assert().Equal(11, character.abilityScores[constants.INT]) // 10 + 1
	s.Assert().Equal(13, character.abilityScores[constants.WIS]) // 12 + 1
	s.Assert().Equal(9, character.abilityScores[constants.CHA])  // 8 + 1

	// Verify HP (Fighter d10 + CON modifier of +2)
	s.Assert().Equal(12, character.maxHitPoints)
	s.Assert().Equal(12, character.hitPoints)

	// Verify physical characteristics
	s.Assert().Equal(30, character.speed)
	s.Assert().Equal("Medium", character.size)

	// Verify skills (chosen + background)
	expectedSkills := map[string]shared.ProficiencyLevel{
		"Perception":   shared.Proficient, // Chosen
		"Survival":     shared.Proficient, // Chosen
		"Athletics":    shared.Proficient, // Background
		"Intimidation": shared.Proficient, // Background
	}
	s.Assert().Equal(expectedSkills, character.skills)

	// Verify languages (race + background + chosen)
	s.Assert().Contains(character.languages, "Common")   // Human
	s.Assert().Contains(character.languages, "Orc")      // Soldier background
	s.Assert().Contains(character.languages, "Dwarvish") // Chosen
	s.Assert().Contains(character.languages, "Giant")    // Chosen
	s.Assert().Len(character.languages, 4)

	// Verify proficiencies
	s.Assert().Equal(s.fighterClass.ArmorProficiencies, character.proficiencies.Armor)
	s.Assert().Equal(s.fighterClass.WeaponProficiencies, character.proficiencies.Weapons)
	s.Assert().Equal(s.soldierBg.ToolProficiencies, character.proficiencies.Tools)

	// Verify saving throws
	s.Assert().Equal(shared.Proficient, character.savingThrows[shared.AbilityStrength])
	s.Assert().Equal(shared.Proficient, character.savingThrows[shared.AbilityConstitution])
	s.Assert().Equal(shared.NotProficient, character.savingThrows[shared.AbilityDexterity])
	s.Assert().Equal(shared.NotProficient, character.savingThrows[shared.AbilityIntelligence])
	s.Assert().Equal(shared.NotProficient, character.savingThrows[shared.AbilityWisdom])
	s.Assert().Equal(shared.NotProficient, character.savingThrows[shared.AbilityCharisma])

	// Verify choices were recorded
	s.Assert().True(len(character.choices) > 0)
	hasSkillChoice := false
	hasLanguageChoice := false
	for _, choice := range character.choices {
		if choice.Category == string(shared.ChoiceSkills) {
			hasSkillChoice = true
			skills, ok := choice.Selection.([]string)
			s.Assert().True(ok)
			s.Assert().Contains(skills, "Perception")
			s.Assert().Contains(skills, "Survival")
		}
		if choice.Category == string(shared.ChoiceLanguages) {
			hasLanguageChoice = true
			langs, ok := choice.Selection.([]string)
			s.Assert().True(ok)
			s.Assert().Contains(langs, "Dwarvish")
			s.Assert().Contains(langs, "Giant")
		}
	}
	s.Assert().True(hasSkillChoice, "Should have recorded skill choices")
	s.Assert().True(hasLanguageChoice, "Should have recorded language choices")
}

func (s *DraftConversionTestSuite) TestHighElfWizardConversion() {
	// Create a complete draft for a High Elf Wizard
	draft := &Draft{
		ID:       "test-elf-wizard",
		PlayerID: "player-456",
		Name:     "Elaria Moonshadow",
		RaceChoice: RaceChoice{
			RaceID:    "elf",
			SubraceID: "high-elf",
		},
		ClassChoice:      "wizard",
		BackgroundChoice: "hermit",
		AbilityScoreChoice: shared.AbilityScores{
			constants.STR: 8,
			constants.DEX: 14,
			constants.CON: 13,
			constants.INT: 15,
			constants.WIS: 12,
			constants.CHA: 10,
		},
		SkillChoices:    []string{"Arcana", "Investigation"},
		LanguageChoices: []string{"Draconic", "Sylvan"},
		Progress: DraftProgress{
			flags: ProgressName | ProgressRace | ProgressClass | ProgressBackground | ProgressAbilityScores,
		},
	}

	// Convert to character
	character, err := draft.ToCharacter(s.elfRace, s.wizardClass, s.hermitBg)
	s.Require().NoError(err)
	s.Require().NotNil(character)

	// Verify basic info
	s.Assert().Equal("test-elf-wizard", character.id)
	s.Assert().Equal("Elaria Moonshadow", character.name)
	// Note: subraceID is stored in the character data but not exposed on the Character struct

	// Verify ability scores with Elf/High Elf bonuses
	s.Assert().Equal(8, character.abilityScores[constants.STR])  // No bonus
	s.Assert().Equal(16, character.abilityScores[constants.DEX]) // 14 + 2 (elf)
	s.Assert().Equal(13, character.abilityScores[constants.CON]) // No bonus
	s.Assert().Equal(16, character.abilityScores[constants.INT]) // 15 + 1 (high elf)
	s.Assert().Equal(12, character.abilityScores[constants.WIS]) // No bonus
	s.Assert().Equal(10, character.abilityScores[constants.CHA]) // No bonus

	// Verify HP (Wizard d6 + CON modifier of +1)
	s.Assert().Equal(7, character.maxHitPoints)
	s.Assert().Equal(7, character.hitPoints)

	// Verify skills
	expectedSkills := map[string]shared.ProficiencyLevel{
		"Arcana":        shared.Proficient, // Chosen
		"Investigation": shared.Proficient, // Chosen
		"Medicine":      shared.Proficient, // Hermit background
		"Religion":      shared.Proficient, // Hermit background
	}
	s.Assert().Equal(expectedSkills, character.skills)

	// Verify languages
	expectedLanguages := []string{"Common", "Elvish", "Celestial", "Draconic", "Sylvan"}
	s.Assert().Len(character.languages, len(expectedLanguages))
	for _, lang := range expectedLanguages {
		s.Assert().Contains(character.languages, lang)
	}

	// Verify weapon proficiencies include both class and race
	for _, weapon := range s.wizardClass.WeaponProficiencies {
		s.Assert().Contains(character.proficiencies.Weapons, weapon)
	}
	for _, weapon := range s.elfRace.WeaponProficiencies {
		s.Assert().Contains(character.proficiencies.Weapons, weapon)
	}

	// Verify saving throws
	s.Assert().Equal(shared.Proficient, character.savingThrows[shared.AbilityIntelligence])
	s.Assert().Equal(shared.Proficient, character.savingThrows[shared.AbilityWisdom])
}

func (s *DraftConversionTestSuite) TestRaceWithoutCommonLanguage() {
	// Create a race that doesn't include Common
	exoticRace := &race.Data{
		ID:        "exotic",
		Name:      "Exotic Race",
		Size:      "Medium",
		Speed:     30,
		Languages: []string{"Primordial", "Abyssal"}, // No Common
	}

	draft := &Draft{
		ID:               "test-exotic",
		PlayerID:         "player-789",
		Name:             "Zyx'tar",
		RaceChoice:       RaceChoice{RaceID: "exotic"},
		ClassChoice:      "fighter",
		BackgroundChoice: "soldier",
		AbilityScoreChoice: shared.AbilityScores{
			constants.STR: 14,
			constants.DEX: 12,
			constants.CON: 15,
			constants.INT: 10,
			constants.WIS: 13,
			constants.CHA: 8,
		},
		SkillChoices: []string{"Perception", "Survival"},
		Progress: DraftProgress{
			flags: ProgressName | ProgressRace | ProgressClass | ProgressBackground | ProgressAbilityScores,
		},
	}

	// Convert to character
	character, err := draft.ToCharacter(exoticRace, s.fighterClass, s.soldierBg)
	s.Require().NoError(err)
	s.Require().NotNil(character)

	// Verify Common is still included
	s.Assert().Contains(character.languages, "Common", "Common should always be included")
	s.Assert().Contains(character.languages, "Primordial")
	s.Assert().Contains(character.languages, "Abyssal")
	s.Assert().Contains(character.languages, "Orc") // From soldier background
}

func (s *DraftConversionTestSuite) TestDuplicateLanguageHandling() {
	// Create a draft where chosen languages overlap with race/background
	draft := &Draft{
		ID:               "test-duplicate-lang",
		PlayerID:         "player-999",
		Name:             "Linguist",
		RaceChoice:       RaceChoice{RaceID: "elf"},
		ClassChoice:      "wizard",
		BackgroundChoice: "hermit",
		AbilityScoreChoice: shared.AbilityScores{
			constants.STR: 10,
			constants.DEX: 14,
			constants.CON: 12,
			constants.INT: 15,
			constants.WIS: 13,
			constants.CHA: 8,
		},
		SkillChoices: []string{"Arcana", "History"},
		// Choosing languages that overlap with race/background
		LanguageChoices: []string{"Common", "Elvish", "Celestial", "Draconic"},
		Progress: DraftProgress{
			flags: ProgressName | ProgressRace | ProgressClass | ProgressBackground | ProgressAbilityScores,
		},
	}

	// Convert to character
	character, err := draft.ToCharacter(s.elfRace, s.wizardClass, s.hermitBg)
	s.Require().NoError(err)
	s.Require().NotNil(character)

	// Count each language occurrence
	languageCount := make(map[string]int)
	for _, lang := range character.languages {
		languageCount[lang]++
	}

	// Verify no duplicates
	for lang, count := range languageCount {
		s.Assert().Equal(1, count, "Language %s should appear only once, but appears %d times", lang, count)
	}

	// Verify all expected languages are present
	s.Assert().Contains(character.languages, "Common")
	s.Assert().Contains(character.languages, "Elvish")
	s.Assert().Contains(character.languages, "Celestial")
	s.Assert().Contains(character.languages, "Draconic")
}

func (s *DraftConversionTestSuite) TestAllProficienciesApplied() {
	// Test that all proficiencies from race, class, and background are applied
	draft := &Draft{
		ID:               "test-all-prof",
		PlayerID:         "player-prof",
		Name:             "Jack of All Trades",
		RaceChoice:       RaceChoice{RaceID: "elf", SubraceID: "wood-elf"},
		ClassChoice:      "fighter",
		BackgroundChoice: "soldier",
		AbilityScoreChoice: shared.AbilityScores{
			constants.STR: 14,
			constants.DEX: 15,
			constants.CON: 13,
			constants.INT: 10,
			constants.WIS: 12,
			constants.CHA: 8,
		},
		SkillChoices: []string{"Perception", "Survival"},
		Progress: DraftProgress{
			flags: ProgressName | ProgressRace | ProgressClass | ProgressBackground | ProgressAbilityScores,
		},
	}

	// Convert to character
	character, err := draft.ToCharacter(s.elfRace, s.fighterClass, s.soldierBg)
	s.Require().NoError(err)
	s.Require().NotNil(character)

	// Verify armor proficiencies (from class)
	s.Assert().Equal(s.fighterClass.ArmorProficiencies, character.proficiencies.Armor)

	// Verify weapon proficiencies (from both class and race)
	// Should have all fighter weapons
	for _, weapon := range s.fighterClass.WeaponProficiencies {
		s.Assert().Contains(character.proficiencies.Weapons, weapon)
	}
	// Should also have elf weapon proficiencies
	for _, weapon := range s.elfRace.WeaponProficiencies {
		s.Assert().Contains(character.proficiencies.Weapons, weapon)
	}

	// Verify tool proficiencies (from background)
	s.Assert().Equal(s.soldierBg.ToolProficiencies, character.proficiencies.Tools)
}

func (s *DraftConversionTestSuite) TestChoiceDataStorage() {
	// Verify that all choices are properly stored in the character
	draft := &Draft{
		ID:       "test-choices",
		PlayerID: "player-choices",
		Name:     "Choice Tracker",
		RaceChoice: RaceChoice{
			RaceID:    "elf",
			SubraceID: "high-elf",
		},
		ClassChoice:      "wizard",
		BackgroundChoice: "hermit",
		AbilityScoreChoice: shared.AbilityScores{
			constants.STR: 8,
			constants.DEX: 14,
			constants.CON: 12,
			constants.INT: 15,
			constants.WIS: 13,
			constants.CHA: 10,
		},
		SkillChoices:    []string{"Arcana", "History"},
		LanguageChoices: []string{"Draconic"},
		Progress: DraftProgress{
			flags: ProgressName | ProgressRace | ProgressClass | ProgressBackground | ProgressAbilityScores,
		},
	}

	// Convert to character
	character, err := draft.ToCharacter(s.elfRace, s.wizardClass, s.hermitBg)
	s.Require().NoError(err)
	s.Require().NotNil(character)

	// Check that all choice categories are stored
	choiceCategories := make(map[string]bool)
	for _, choice := range character.choices {
		choiceCategories[choice.Category] = true
	}

	expectedCategories := []string{
		string(shared.ChoiceName),
		string(shared.ChoiceRace),
		string(shared.ChoiceClass),
		string(shared.ChoiceBackground),
		string(shared.ChoiceAbilityScores),
		string(shared.ChoiceSkills),
		string(shared.ChoiceLanguages),
	}

	for _, cat := range expectedCategories {
		s.Assert().True(choiceCategories[cat], "Choice category %s should be stored", cat)
	}

	// Verify choice sources are set correctly
	for _, choice := range character.choices {
		switch shared.ChoiceCategory(choice.Category) {
		case shared.ChoiceRace, shared.ChoiceSubrace, shared.ChoiceLanguages:
			if choice.Category == string(shared.ChoiceLanguages) {
				// Language choices from player selection should be "race" source
				s.Assert().Equal("race", choice.Source)
			}
		case shared.ChoiceClass, shared.ChoiceSkills:
			s.Assert().Equal("class", choice.Source)
		case shared.ChoiceBackground:
			s.Assert().Equal("background", choice.Source)
		case shared.ChoiceAbilityScores, shared.ChoiceName:
			s.Assert().Equal("player", choice.Source)
		}
	}
}

func (s *DraftConversionTestSuite) TestFightingStylesStoredCorrectly() {
	// Test that fighting styles are properly stored in choices
	draft := &Draft{
		ID:               "test-fighting-style",
		PlayerID:         "player-fs",
		Name:             "Fighter with Style",
		RaceChoice:       RaceChoice{RaceID: "human"},
		ClassChoice:      "fighter",
		BackgroundChoice: "soldier",
		AbilityScoreChoice: shared.AbilityScores{
			constants.STR: 16,
			constants.DEX: 14,
			constants.CON: 15,
			constants.INT: 10,
			constants.WIS: 12,
			constants.CHA: 8,
		},
		SkillChoices:        []string{"Perception", "Survival"},
		FightingStyleChoice: "dueling", // Fighting style choice
		Progress: DraftProgress{
			flags: ProgressName | ProgressRace | ProgressClass | ProgressBackground | ProgressAbilityScores,
		},
	}

	character, err := draft.ToCharacter(s.humanRace, s.fighterClass, s.soldierBg)
	s.Require().NoError(err)
	s.Require().NotNil(character)

	// Find and verify fighting style choice
	var fightingStyleChoice *ChoiceData
	for _, choice := range character.choices {
		if choice.Category == string(shared.ChoiceFightingStyle) {
			fightingStyleChoice = &choice
			break
		}
	}

	s.Require().NotNil(fightingStyleChoice, "Fighting style choice should be stored")
	s.Assert().Equal("dueling", fightingStyleChoice.Selection)
	s.Assert().Equal("class", fightingStyleChoice.Source)
}

func (s *DraftConversionTestSuite) TestSpellsAndCantripsStoredCorrectly() {
	// Test that spells and cantrips are properly stored in choices
	draft := &Draft{
		ID:               "test-spells",
		PlayerID:         "player-spells",
		Name:             "Spellcaster Supreme",
		RaceChoice:       RaceChoice{RaceID: "elf", SubraceID: "high-elf"},
		ClassChoice:      "wizard",
		BackgroundChoice: "hermit",
		AbilityScoreChoice: shared.AbilityScores{
			constants.STR: 8,
			constants.DEX: 14,
			constants.CON: 13,
			constants.INT: 15,
			constants.WIS: 12,
			constants.CHA: 10,
		},
		SkillChoices:   []string{"Arcana", "Investigation"},
		CantripChoices: []string{"Mage Hand", "Prestidigitation", "Minor Illusion"},
		SpellChoices:   []string{"Magic Missile", "Shield", "Identify", "Detect Magic", "Sleep", "Burning Hands"},
		Progress: DraftProgress{
			flags: ProgressName | ProgressRace | ProgressClass | ProgressBackground | ProgressAbilityScores,
		},
	}

	character, err := draft.ToCharacter(s.elfRace, s.wizardClass, s.hermitBg)
	s.Require().NoError(err)
	s.Require().NotNil(character)

	// Find and verify cantrip choices
	var cantripChoice *ChoiceData
	for _, choice := range character.choices {
		if choice.Category == string(shared.ChoiceCantrips) {
			cantripChoice = &choice
			break
		}
	}

	s.Require().NotNil(cantripChoice, "Cantrip choice should be stored")
	cantrips, ok := cantripChoice.Selection.([]string)
	s.Require().True(ok, "Cantrip selection should be []string")
	s.Assert().Contains(cantrips, "Mage Hand")
	s.Assert().Contains(cantrips, "Prestidigitation")
	s.Assert().Contains(cantrips, "Minor Illusion")
	s.Assert().Equal("class", cantripChoice.Source)

	// Find and verify spell choices
	var spellChoice *ChoiceData
	for _, choice := range character.choices {
		if choice.Category == string(shared.ChoiceSpells) {
			spellChoice = &choice
			break
		}
	}

	s.Require().NotNil(spellChoice, "Spell choice should be stored")
	spells, ok := spellChoice.Selection.([]string)
	s.Require().True(ok, "Spell selection should be []string")
	s.Assert().Contains(spells, "Magic Missile")
	s.Assert().Contains(spells, "Shield")
	s.Assert().Contains(spells, "Identify")
	s.Assert().Contains(spells, "Detect Magic")
	s.Assert().Contains(spells, "Sleep")
	s.Assert().Contains(spells, "Burning Hands")
	s.Assert().Equal("class", spellChoice.Source)
}

func (s *DraftConversionTestSuite) TestEquipmentChoicesStoredCorrectly() {
	// Test that equipment choices are properly stored in choices
	draft := &Draft{
		ID:               "test-equipment",
		PlayerID:         "player-eq",
		Name:             "Well Equipped",
		RaceChoice:       RaceChoice{RaceID: "human"},
		ClassChoice:      "fighter",
		BackgroundChoice: "soldier",
		AbilityScoreChoice: shared.AbilityScores{
			constants.STR: 16,
			constants.DEX: 14,
			constants.CON: 15,
			constants.INT: 10,
			constants.WIS: 12,
			constants.CHA: 8,
		},
		SkillChoices: []string{"Perception", "Survival"},
		EquipmentChoices: []string{
			"Chain Mail", "Shield", "Longsword", "Javelin (5)",
			"Dungeoneer's Pack", "Explorer's Pack",
		},
		Progress: DraftProgress{
			flags: ProgressName | ProgressRace | ProgressClass | ProgressBackground | ProgressAbilityScores,
		},
	}

	character, err := draft.ToCharacter(s.humanRace, s.fighterClass, s.soldierBg)
	s.Require().NoError(err)
	s.Require().NotNil(character)

	// Find and verify equipment choices
	var equipmentChoice *ChoiceData
	for _, choice := range character.choices {
		if choice.Category == string(shared.ChoiceEquipment) {
			equipmentChoice = &choice
			break
		}
	}

	s.Require().NotNil(equipmentChoice, "Equipment choice should be stored")
	equipment, ok := equipmentChoice.Selection.([]string)
	s.Require().True(ok, "Equipment selection should be []string")
	s.Assert().Contains(equipment, "Chain Mail")
	s.Assert().Contains(equipment, "Shield")
	s.Assert().Contains(equipment, "Longsword")
	s.Assert().Contains(equipment, "Javelin (5)")
	s.Assert().Contains(equipment, "Dungeoneer's Pack")
	s.Assert().Contains(equipment, "Explorer's Pack")
	s.Assert().Equal("class", equipmentChoice.Source)
}

func (s *DraftConversionTestSuite) TestAllChoiceTypesComprehensive() {
	// Comprehensive test with all choice types
	draft := &Draft{
		ID:               "test-comprehensive",
		PlayerID:         "player-all",
		Name:             "Jack of All Trades",
		RaceChoice:       RaceChoice{RaceID: "elf", SubraceID: "high-elf"},
		ClassChoice:      "fighter", // Fighter with spellcasting (e.g., Eldritch Knight)
		BackgroundChoice: "soldier",
		AbilityScoreChoice: shared.AbilityScores{
			constants.STR: 15,
			constants.DEX: 14,
			constants.CON: 13,
			constants.INT: 12,
			constants.WIS: 10,
			constants.CHA: 8,
		},
		SkillChoices:        []string{"Perception", "History"},
		LanguageChoices:     []string{"Draconic", "Giant"},
		FightingStyleChoice: "protection",
		CantripChoices:      []string{"Mage Hand", "Minor Illusion"},
		SpellChoices:        []string{"Shield", "Magic Missile"},
		EquipmentChoices:    []string{"Plate Armor", "Shield", "Longsword", "Shortbow"},
		Progress: DraftProgress{
			flags: ProgressName | ProgressRace | ProgressClass | ProgressBackground | ProgressAbilityScores,
		},
	}

	character, err := draft.ToCharacter(s.elfRace, s.fighterClass, s.soldierBg)
	s.Require().NoError(err)
	s.Require().NotNil(character)

	// Verify all choice categories are present
	expectedCategories := map[string]bool{
		string(shared.ChoiceName):          false,
		string(shared.ChoiceRace):          false,
		string(shared.ChoiceClass):         false,
		string(shared.ChoiceBackground):    false,
		string(shared.ChoiceAbilityScores): false,
		string(shared.ChoiceSkills):        false,
		string(shared.ChoiceLanguages):     false,
		string(shared.ChoiceFightingStyle): false,
		string(shared.ChoiceCantrips):      false,
		string(shared.ChoiceSpells):        false,
		string(shared.ChoiceEquipment):     false,
	}

	for _, choice := range character.choices {
		if _, exists := expectedCategories[choice.Category]; exists {
			expectedCategories[choice.Category] = true
		}
	}

	// Verify all categories were found
	for category, found := range expectedCategories {
		s.Assert().True(found, "Choice category %s should be present", category)
	}

	// Verify sources are correctly assigned
	sourceMap := map[string]string{
		string(shared.ChoiceName):          "player",
		string(shared.ChoiceRace):          "race",
		string(shared.ChoiceClass):         "class",
		string(shared.ChoiceBackground):    "background",
		string(shared.ChoiceAbilityScores): "player",
		string(shared.ChoiceSkills):        "class",
		string(shared.ChoiceLanguages):     "race",
		string(shared.ChoiceFightingStyle): "class",
		string(shared.ChoiceCantrips):      "class",
		string(shared.ChoiceSpells):        "class",
		string(shared.ChoiceEquipment):     "class",
	}

	for _, choice := range character.choices {
		if expectedSource, exists := sourceMap[choice.Category]; exists {
			s.Assert().Equal(expectedSource, choice.Source,
				"Choice category %s should have source %s, got %s",
				choice.Category, expectedSource, choice.Source)
		}
	}

	// Verify character stats still work correctly
	s.Assert().Equal("Jack of All Trades", character.name)
	s.Assert().Equal(16, character.abilityScores[constants.DEX]) // 14 + 2 (elf)
	s.Assert().Equal(13, character.abilityScores[constants.INT]) // 12 + 1 (high elf)
	s.Assert().Contains(character.languages, "Common")
	s.Assert().Contains(character.languages, "Elvish")
	s.Assert().Contains(character.languages, "Draconic")
	s.Assert().Contains(character.languages, "Giant")
	s.Assert().Equal(shared.Proficient, character.skills["Perception"])
	s.Assert().Equal(shared.Proficient, character.skills["History"])
}

func TestDraftConversionTestSuite(t *testing.T) {
	suite.Run(t, new(DraftConversionTestSuite))
}
