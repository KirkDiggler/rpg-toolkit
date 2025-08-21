package character

import (
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/abilities"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/backgrounds"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/class"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/classes"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/languages"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/race"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/races"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/shared"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/skills"
)

// Helper function to create skill choices for tests
func makeSkillChoices(className string, skills []skills.Skill) []ChoiceData {
	return []ChoiceData{
		{
			Category:       shared.ChoiceSkills,
			Source:         shared.SourceClass,
			ChoiceID:       className + "_skill_proficiencies",
			SkillSelection: skills,
		},
	}
}

// Helper function to create language choices for tests
func makeLanguageChoices(source shared.ChoiceSource, languages []languages.Language) []ChoiceData {
	return []ChoiceData{
		{
			Category:          shared.ChoiceLanguages,
			Source:            source,
			ChoiceID:          "additional_languages",
			LanguageSelection: languages,
		},
	}
}

// Helper function to combine multiple choice sets
func combineChoices(choiceSets ...[]ChoiceData) []ChoiceData {
	var result []ChoiceData
	for _, choices := range choiceSets {
		result = append(result, choices...)
	}
	return result
}

// Helper function to create fighting style choice
func makeFightingStyleChoice(style string) []ChoiceData {
	return []ChoiceData{
		{
			Category:               shared.ChoiceFightingStyle,
			Source:                 shared.SourceClass,
			ChoiceID:               "fighter_fighting_style",
			FightingStyleSelection: &style,
		},
	}
}

// Helper function to create spell choices
func makeSpellChoices(spells []string) []ChoiceData {
	return []ChoiceData{
		{
			Category:       shared.ChoiceSpells,
			Source:         shared.SourceClass,
			ChoiceID:       "wizard_spells_known",
			SpellSelection: spells,
		},
	}
}

// Helper function to create cantrip choices
func makeCantripChoices(cantrips []string) []ChoiceData {
	return []ChoiceData{
		{
			Category:         shared.ChoiceCantrips,
			Source:           shared.SourceClass,
			ChoiceID:         "wizard_cantrips",
			CantripSelection: cantrips,
		},
	}
}

// Helper function to create equipment choices
func makeEquipmentChoices(equipment []string) []ChoiceData {
	return []ChoiceData{
		{
			Category:           shared.ChoiceEquipment,
			Source:             shared.SourceClass,
			ChoiceID:           "starting_equipment",
			EquipmentSelection: equipment,
		},
	}
}

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
		AbilityScoreIncreases: map[abilities.Ability]int{
			abilities.STR: 1,
			abilities.DEX: 1,
			abilities.CON: 1,
			abilities.INT: 1,
			abilities.WIS: 1,
			abilities.CHA: 1,
		},
		Languages: []languages.Language{languages.Common},
	}

	// Setup Elf race with subraces
	s.elfRace = &race.Data{
		ID:    "elf",
		Name:  "Elf",
		Size:  "Medium",
		Speed: 30,
		AbilityScoreIncreases: map[abilities.Ability]int{
			abilities.DEX: 2,
		},
		Languages:           []languages.Language{languages.Common, languages.Elvish},
		WeaponProficiencies: []string{"Longsword", "Shortsword", "Shortbow", "Longbow"},
		Subraces: []race.SubraceData{
			{
				ID:   "high-elf",
				Name: "High Elf",
				AbilityScoreIncreases: map[abilities.Ability]int{
					abilities.INT: 1,
				},
			},
			{
				ID:   "wood-elf",
				Name: "Wood Elf",
				AbilityScoreIncreases: map[abilities.Ability]int{
					abilities.WIS: 1,
				},
			},
		},
	}

	// Setup Fighter class
	s.fighterClass = &class.Data{
		ID:                    classes.Fighter,
		Name:                  "Fighter",
		HitDice:               10,
		SavingThrows:          []abilities.Ability{abilities.STR, abilities.CON},
		SkillProficiencyCount: 2,
		SkillOptions: []skills.Skill{
			skills.Acrobatics, skills.AnimalHandling, skills.Athletics, skills.History,
			skills.Insight, skills.Intimidation, skills.Perception, skills.Survival,
		},
		ArmorProficiencies:  []string{"Light", "Medium", "Heavy", "Shield"},
		WeaponProficiencies: []string{"Simple", "Martial"},
		ToolProficiencies:   []string{},
		StartingEquipment: []class.EquipmentData{
			{ItemID: "Chain Mail", Quantity: 1},
			{ItemID: "Shield", Quantity: 1},
			{ItemID: "Javelin", Quantity: 5},
		},
	}

	// Setup Wizard class
	s.wizardClass = &class.Data{
		ID:                    classes.Wizard,
		Name:                  "Wizard",
		HitDice:               6,
		SavingThrows:          []abilities.Ability{abilities.INT, abilities.WIS},
		SkillProficiencyCount: 2,
		SkillOptions: []skills.Skill{
			skills.Arcana, skills.History, skills.Insight,
			skills.Investigation, skills.Medicine, skills.Religion,
		},
		ArmorProficiencies:  []string{},
		WeaponProficiencies: []string{"Dagger", "Dart", "Sling", "Quarterstaff", "Light Crossbow"},
		ToolProficiencies:   []string{},
	}

	// Setup Soldier background
	s.soldierBg = &shared.Background{
		ID:                 "soldier",
		Name:               "Soldier",
		SkillProficiencies: []skills.Skill{skills.Athletics, skills.Intimidation},
		Languages:          []languages.Language{languages.Orc},
		ToolProficiencies:  []string{"Gaming set", "Land vehicles"},
		Equipment:          []string{"Insignia of rank", "Trophy", "Deck of cards", "Common clothes"},
	}

	// Setup Hermit background
	s.hermitBg = &shared.Background{
		ID:                 "hermit",
		Name:               "Hermit",
		SkillProficiencies: []skills.Skill{skills.Medicine, skills.Religion},
		Languages:          []languages.Language{languages.Celestial},
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
			RaceID: races.Human,
		},
		ClassChoice: ClassChoice{
			ClassID: classes.Fighter,
		},
		BackgroundChoice: backgrounds.Soldier,
		AbilityScoreChoice: shared.AbilityScores{
			abilities.STR: 15,
			abilities.DEX: 13,
			abilities.CON: 14,
			abilities.INT: 10,
			abilities.WIS: 12,
			abilities.CHA: 8,
		},
		Choices: []ChoiceData{
			{
				Category:       shared.ChoiceSkills,
				Source:         shared.SourceClass,
				ChoiceID:       "ranger_skill_proficiencies",
				SkillSelection: []skills.Skill{skills.Perception, skills.Survival},
			},
			{
				Category:          shared.ChoiceLanguages,
				Source:            shared.SourceRace,
				ChoiceID:          "additional_languages",
				LanguageSelection: []languages.Language{languages.Dwarvish, languages.Giant},
			},
		},
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
	s.Assert().Equal(races.Human, character.raceID)
	s.Assert().Equal(classes.Fighter, character.classID)
	s.Assert().Equal(backgrounds.Soldier, character.backgroundID)

	// Verify ability scores with Human racial bonuses (+1 to all)
	s.Assert().Equal(16, character.abilityScores[abilities.STR]) // 15 + 1
	s.Assert().Equal(14, character.abilityScores[abilities.DEX]) // 13 + 1
	s.Assert().Equal(15, character.abilityScores[abilities.CON]) // 14 + 1
	s.Assert().Equal(11, character.abilityScores[abilities.INT]) // 10 + 1
	s.Assert().Equal(13, character.abilityScores[abilities.WIS]) // 12 + 1
	s.Assert().Equal(9, character.abilityScores[abilities.CHA])  // 8 + 1

	// Verify HP (Fighter d10 + CON modifier of +2)
	s.Assert().Equal(12, character.maxHitPoints)
	s.Assert().Equal(12, character.hitPoints)

	// Verify physical characteristics
	s.Assert().Equal(30, character.speed)
	s.Assert().Equal("Medium", character.size)

	// Verify skills (chosen + background)
	expectedSkills := map[skills.Skill]shared.ProficiencyLevel{
		skills.Perception:   shared.Proficient, // Chosen
		skills.Survival:     shared.Proficient, // Chosen
		skills.Athletics:    shared.Proficient, // Background
		skills.Intimidation: shared.Proficient, // Background
	}
	s.Assert().Equal(expectedSkills, character.skills)

	// Verify languages (race + background + chosen)
	s.Assert().Contains(character.languages, languages.Common)   // Human
	s.Assert().Contains(character.languages, languages.Orc)      // Soldier background
	s.Assert().Contains(character.languages, languages.Dwarvish) // Chosen
	s.Assert().Contains(character.languages, languages.Giant)    // Chosen
	s.Assert().Len(character.languages, 4)

	// Verify proficiencies
	s.Assert().Equal(s.fighterClass.ArmorProficiencies, character.proficiencies.Armor)
	s.Assert().Equal(s.fighterClass.WeaponProficiencies, character.proficiencies.Weapons)
	s.Assert().Equal(s.soldierBg.ToolProficiencies, character.proficiencies.Tools)

	// Verify saving throws
	s.Assert().Equal(shared.Proficient, character.savingThrows[abilities.STR])
	s.Assert().Equal(shared.Proficient, character.savingThrows[abilities.CON])
	s.Assert().Equal(shared.NotProficient, character.savingThrows[abilities.DEX])
	s.Assert().Equal(shared.NotProficient, character.savingThrows[abilities.INT])
	s.Assert().Equal(shared.NotProficient, character.savingThrows[abilities.WIS])
	s.Assert().Equal(shared.NotProficient, character.savingThrows[abilities.CHA])

	// Verify choices were recorded
	s.Assert().True(len(character.choices) > 0)
	hasSkillChoice := false
	hasLanguageChoice := false
	for _, choice := range character.choices {
		if choice.Category == shared.ChoiceSkills && choice.SkillSelection != nil {
			hasSkillChoice = true
			s.Assert().Contains(choice.SkillSelection, skills.Perception)
			s.Assert().Contains(choice.SkillSelection, skills.Survival)
		}
		if choice.Category == shared.ChoiceLanguages && choice.LanguageSelection != nil {
			hasLanguageChoice = true
			s.Assert().Contains(choice.LanguageSelection, languages.Dwarvish)
			s.Assert().Contains(choice.LanguageSelection, languages.Giant)
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
			RaceID:    races.Elf,
			SubraceID: races.HighElf,
		},
		ClassChoice: ClassChoice{
			ClassID: classes.Wizard,
		},
		BackgroundChoice: backgrounds.Hermit,
		AbilityScoreChoice: shared.AbilityScores{
			abilities.STR: 8,
			abilities.DEX: 14,
			abilities.CON: 13,
			abilities.INT: 15,
			abilities.WIS: 12,
			abilities.CHA: 10,
		},
		Choices: []ChoiceData{
			{
				Category:       shared.ChoiceSkills,
				Source:         shared.SourceClass,
				ChoiceID:       "wizard_skill_proficiencies",
				SkillSelection: []skills.Skill{skills.Arcana, skills.Investigation},
			},
			{
				Category:          shared.ChoiceLanguages,
				Source:            shared.SourceRace,
				ChoiceID:          "additional_languages",
				LanguageSelection: []languages.Language{languages.Draconic, languages.Sylvan},
			},
		},
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
	s.Assert().Equal(8, character.abilityScores[abilities.STR])  // No bonus
	s.Assert().Equal(16, character.abilityScores[abilities.DEX]) // 14 + 2 (elf)
	s.Assert().Equal(13, character.abilityScores[abilities.CON]) // No bonus
	s.Assert().Equal(16, character.abilityScores[abilities.INT]) // 15 + 1 (high elf)
	s.Assert().Equal(12, character.abilityScores[abilities.WIS]) // No bonus
	s.Assert().Equal(10, character.abilityScores[abilities.CHA]) // No bonus

	// Verify HP (Wizard d6 + CON modifier of +1)
	s.Assert().Equal(7, character.maxHitPoints)
	s.Assert().Equal(7, character.hitPoints)

	// Verify skills
	expectedSkills := map[skills.Skill]shared.ProficiencyLevel{
		skills.Arcana:        shared.Proficient, // Chosen
		skills.Investigation: shared.Proficient, // Chosen
		skills.Medicine:      shared.Proficient, // Hermit background
		skills.Religion:      shared.Proficient, // Hermit background
	}
	s.Assert().Equal(expectedSkills, character.skills)

	// Verify languages
	expectedLanguages := []languages.Language{
		languages.Common, languages.Elvish, languages.Celestial,
		languages.Draconic, languages.Sylvan,
	}
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
	s.Assert().Equal(shared.Proficient, character.savingThrows[abilities.INT])
	s.Assert().Equal(shared.Proficient, character.savingThrows[abilities.WIS])
}

func (s *DraftConversionTestSuite) TestRaceWithoutCommonLanguage() {
	// Create a race that doesn't include Common
	exoticRace := &race.Data{
		ID:        "exotic",
		Name:      "Exotic Race",
		Size:      "Medium",
		Speed:     30,
		Languages: []languages.Language{languages.Primordial, languages.Abyssal}, // No Common
	}

	draft := &Draft{
		ID:         "test-exotic",
		PlayerID:   "player-789",
		Name:       "Zyx'tar",
		RaceChoice: RaceChoice{RaceID: races.Race("exotic")},
		ClassChoice: ClassChoice{
			ClassID: classes.Fighter,
		},
		BackgroundChoice: backgrounds.Soldier,
		AbilityScoreChoice: shared.AbilityScores{
			abilities.STR: 14,
			abilities.DEX: 12,
			abilities.CON: 15,
			abilities.INT: 10,
			abilities.WIS: 13,
			abilities.CHA: 8,
		},
		Choices: makeSkillChoices(string(classes.Fighter), []skills.Skill{skills.Perception, skills.Survival}),
		Progress: DraftProgress{
			flags: ProgressName | ProgressRace | ProgressClass | ProgressBackground | ProgressAbilityScores,
		},
	}

	// Convert to character
	character, err := draft.ToCharacter(exoticRace, s.fighterClass, s.soldierBg)
	s.Require().NoError(err)
	s.Require().NotNil(character)

	// Verify Common is still included
	s.Assert().Contains(character.languages, languages.Common, "Common should always be included")
	s.Assert().Contains(character.languages, languages.Primordial)
	s.Assert().Contains(character.languages, languages.Abyssal)
	s.Assert().Contains(character.languages, languages.Orc) // From soldier background
}

func (s *DraftConversionTestSuite) TestDuplicateLanguageHandling() {
	// Create a draft where chosen languages overlap with race/background
	draft := &Draft{
		ID:         "test-duplicate-lang",
		PlayerID:   "player-999",
		Name:       "Linguist",
		RaceChoice: RaceChoice{RaceID: races.Elf},
		ClassChoice: ClassChoice{
			ClassID: classes.Wizard,
		},
		BackgroundChoice: backgrounds.Hermit,
		AbilityScoreChoice: shared.AbilityScores{
			abilities.STR: 10,
			abilities.DEX: 14,
			abilities.CON: 12,
			abilities.INT: 15,
			abilities.WIS: 13,
			abilities.CHA: 8,
		},
		Choices: combineChoices(
			makeSkillChoices(string(classes.Wizard), []skills.Skill{skills.Arcana, skills.History}),
			makeLanguageChoices(shared.SourceBackground, []languages.Language{
				languages.Common, languages.Elvish,
				languages.Celestial, languages.Draconic,
			}),
		),
		Progress: DraftProgress{
			flags: ProgressName | ProgressRace | ProgressClass | ProgressBackground | ProgressAbilityScores,
		},
	}

	// Convert to character
	character, err := draft.ToCharacter(s.elfRace, s.wizardClass, s.hermitBg)
	s.Require().NoError(err)
	s.Require().NotNil(character)

	// Count each language occurrence
	languageCount := make(map[languages.Language]int)
	for _, lang := range character.languages {
		languageCount[lang]++
	}

	// Verify no duplicates
	for lang, count := range languageCount {
		s.Assert().Equal(1, count, "Language %s should appear only once, but appears %d times", lang, count)
	}

	// Verify all expected languages are present
	s.Assert().Contains(character.languages, languages.Common)
	s.Assert().Contains(character.languages, languages.Elvish)
	s.Assert().Contains(character.languages, languages.Celestial)
	s.Assert().Contains(character.languages, languages.Draconic)
}

func (s *DraftConversionTestSuite) TestAllProficienciesApplied() {
	// Test that all proficiencies from race, class, and background are applied
	draft := &Draft{
		ID:         "test-all-prof",
		PlayerID:   "player-prof",
		Name:       "Jack of All Trades",
		RaceChoice: RaceChoice{RaceID: races.Elf, SubraceID: races.WoodElf},
		ClassChoice: ClassChoice{
			ClassID: classes.Fighter,
		},
		BackgroundChoice: backgrounds.Soldier,
		AbilityScoreChoice: shared.AbilityScores{
			abilities.STR: 14,
			abilities.DEX: 15,
			abilities.CON: 13,
			abilities.INT: 10,
			abilities.WIS: 12,
			abilities.CHA: 8,
		},
		Choices: makeSkillChoices(string(classes.Fighter), []skills.Skill{skills.Perception, skills.Survival}),
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
			RaceID:    races.Elf,
			SubraceID: races.HighElf,
		},
		ClassChoice: ClassChoice{
			ClassID: classes.Wizard,
		},
		BackgroundChoice: backgrounds.Hermit,
		AbilityScoreChoice: shared.AbilityScores{
			abilities.STR: 8,
			abilities.DEX: 14,
			abilities.CON: 12,
			abilities.INT: 15,
			abilities.WIS: 13,
			abilities.CHA: 10,
		},
		Choices: combineChoices(
			makeSkillChoices(string(classes.Wizard), []skills.Skill{skills.Arcana, skills.History}),
			makeLanguageChoices(shared.SourceRace, []languages.Language{languages.Draconic}),
		),
		Progress: DraftProgress{
			flags: ProgressName | ProgressRace | ProgressClass | ProgressBackground | ProgressAbilityScores,
		},
	}

	// Convert to character
	character, err := draft.ToCharacter(s.elfRace, s.wizardClass, s.hermitBg)
	s.Require().NoError(err)
	s.Require().NotNil(character)

	// Check that all choice categories are stored
	choiceCategories := make(map[shared.ChoiceCategory]bool)
	for _, choice := range character.choices {
		choiceCategories[choice.Category] = true
	}

	expectedCategories := []shared.ChoiceCategory{
		shared.ChoiceName,
		shared.ChoiceRace,
		shared.ChoiceClass,
		shared.ChoiceBackground,
		shared.ChoiceAbilityScores,
		shared.ChoiceSkills,
		shared.ChoiceLanguages,
	}

	for _, cat := range expectedCategories {
		s.Assert().True(choiceCategories[cat], "Choice category %s should be stored", cat)
	}

	// Verify choice sources are set correctly
	for _, choice := range character.choices {
		switch choice.Category {
		case shared.ChoiceLanguages:
			// Language choices from player selection should be race source
			s.Assert().Equal(shared.SourceRace, choice.Source)
		case shared.ChoiceSkills:
			s.Assert().Equal(shared.SourceClass, choice.Source)
		case shared.ChoiceRace, shared.ChoiceClass, shared.ChoiceBackground, shared.ChoiceAbilityScores, shared.ChoiceName:
			// These are fundamental player choices
			s.Assert().Equal(shared.SourcePlayer, choice.Source)
		}
	}
}

func (s *DraftConversionTestSuite) TestFightingStylesStoredCorrectly() {
	// Test that fighting styles are properly stored in choices
	draft := &Draft{
		ID:         "test-fighting-style",
		PlayerID:   "player-fs",
		Name:       "Fighter with Style",
		RaceChoice: RaceChoice{RaceID: races.Human},
		ClassChoice: ClassChoice{
			ClassID: classes.Fighter,
		},
		BackgroundChoice: backgrounds.Soldier,
		AbilityScoreChoice: shared.AbilityScores{
			abilities.STR: 16,
			abilities.DEX: 14,
			abilities.CON: 15,
			abilities.INT: 10,
			abilities.WIS: 12,
			abilities.CHA: 8,
		},
		Choices: combineChoices(
			makeSkillChoices(string(classes.Fighter), []skills.Skill{skills.Perception, skills.Survival}),
			makeFightingStyleChoice("dueling"),
		),
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
		if choice.Category == shared.ChoiceFightingStyle {
			fightingStyleChoice = &choice
			break
		}
	}

	s.Require().NotNil(fightingStyleChoice, "Fighting style choice should be stored")
	s.Require().NotNil(fightingStyleChoice.FightingStyleSelection)
	s.Assert().Equal("dueling", *fightingStyleChoice.FightingStyleSelection)
	s.Assert().Equal(shared.SourceClass, fightingStyleChoice.Source)
}

func (s *DraftConversionTestSuite) TestSpellsAndCantripsStoredCorrectly() {
	// Test that spells and cantrips are properly stored in choices
	draft := &Draft{
		ID:         "test-spells",
		PlayerID:   "player-spells",
		Name:       "Spellcaster Supreme",
		RaceChoice: RaceChoice{RaceID: races.Elf, SubraceID: races.HighElf},
		ClassChoice: ClassChoice{
			ClassID: classes.Wizard,
		},
		BackgroundChoice: backgrounds.Hermit,
		AbilityScoreChoice: shared.AbilityScores{
			abilities.STR: 8,
			abilities.DEX: 14,
			abilities.CON: 13,
			abilities.INT: 15,
			abilities.WIS: 12,
			abilities.CHA: 10,
		},
		Choices: combineChoices(
			makeSkillChoices(string(classes.Wizard), []skills.Skill{skills.Arcana, skills.Investigation}),
			makeCantripChoices([]string{"Mage Hand", "Prestidigitation", "Minor Illusion"}),
			makeSpellChoices([]string{"Magic Missile", "Shield", "Identify", "Detect Magic", "Sleep", "Burning Hands"}),
		),
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
		if choice.Category == shared.ChoiceCantrips {
			cantripChoice = &choice
			break
		}
	}

	s.Require().NotNil(cantripChoice, "Cantrip choice should be stored")
	s.Require().NotNil(cantripChoice.CantripSelection)
	s.Assert().Contains(cantripChoice.CantripSelection, "Mage Hand")
	s.Assert().Contains(cantripChoice.CantripSelection, "Prestidigitation")
	s.Assert().Contains(cantripChoice.CantripSelection, "Minor Illusion")
	s.Assert().Equal(shared.SourceClass, cantripChoice.Source)

	// Find and verify spell choices
	var spellChoice *ChoiceData
	for _, choice := range character.choices {
		if choice.Category == shared.ChoiceSpells {
			spellChoice = &choice
			break
		}
	}

	s.Require().NotNil(spellChoice, "Spell choice should be stored")
	s.Require().NotNil(spellChoice.SpellSelection)
	s.Assert().Contains(spellChoice.SpellSelection, "Magic Missile")
	s.Assert().Contains(spellChoice.SpellSelection, "Shield")
	s.Assert().Contains(spellChoice.SpellSelection, "Identify")
	s.Assert().Contains(spellChoice.SpellSelection, "Detect Magic")
	s.Assert().Contains(spellChoice.SpellSelection, "Sleep")
	s.Assert().Contains(spellChoice.SpellSelection, "Burning Hands")
	s.Assert().Equal(shared.SourceClass, spellChoice.Source)
}

func (s *DraftConversionTestSuite) TestEquipmentChoicesStoredCorrectly() {
	// Test that equipment choices are properly stored in choices
	draft := &Draft{
		ID:         "test-equipment",
		PlayerID:   "player-eq",
		Name:       "Well Equipped",
		RaceChoice: RaceChoice{RaceID: races.Human},
		ClassChoice: ClassChoice{
			ClassID: classes.Fighter,
		},
		BackgroundChoice: backgrounds.Soldier,
		AbilityScoreChoice: shared.AbilityScores{
			abilities.STR: 16,
			abilities.DEX: 14,
			abilities.CON: 15,
			abilities.INT: 10,
			abilities.WIS: 12,
			abilities.CHA: 8,
		},
		Choices: combineChoices(
			makeSkillChoices(string(classes.Fighter), []skills.Skill{skills.Perception, skills.Survival}),
			makeEquipmentChoices([]string{
				"Chain Mail", "Shield", "Longsword", "Javelin (5)",
				"Dungeoneer's Pack", "Explorer's Pack",
			}),
		),
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
		if choice.Category == shared.ChoiceEquipment {
			equipmentChoice = &choice
			break
		}
	}

	s.Require().NotNil(equipmentChoice, "Equipment choice should be stored")
	s.Require().NotNil(equipmentChoice.EquipmentSelection)
	s.Assert().Contains(equipmentChoice.EquipmentSelection, "Chain Mail")
	s.Assert().Contains(equipmentChoice.EquipmentSelection, "Shield")
	s.Assert().Contains(equipmentChoice.EquipmentSelection, "Longsword")
	s.Assert().Contains(equipmentChoice.EquipmentSelection, "Javelin (5)")
	s.Assert().Contains(equipmentChoice.EquipmentSelection, "Dungeoneer's Pack")
	s.Assert().Contains(equipmentChoice.EquipmentSelection, "Explorer's Pack")
	s.Assert().Equal(shared.SourceClass, equipmentChoice.Source)
}

func (s *DraftConversionTestSuite) TestAllChoiceTypesComprehensive() {
	// Comprehensive test with all choice types
	draft := &Draft{
		ID:         "test-comprehensive",
		PlayerID:   "player-all",
		Name:       "Jack of All Trades",
		RaceChoice: RaceChoice{RaceID: races.Elf, SubraceID: races.HighElf},
		ClassChoice: ClassChoice{
			ClassID: classes.Fighter,
		}, // Fighter with spellcasting (e.g., Eldritch Knight)
		BackgroundChoice: backgrounds.Soldier,
		AbilityScoreChoice: shared.AbilityScores{
			abilities.STR: 15,
			abilities.DEX: 14,
			abilities.CON: 13,
			abilities.INT: 12,
			abilities.WIS: 10,
			abilities.CHA: 8,
		},
		Choices: combineChoices(
			makeSkillChoices(string(classes.Fighter), []skills.Skill{skills.Perception, skills.History}),
			makeLanguageChoices(shared.SourceRace, []languages.Language{languages.Draconic, languages.Giant}),
			makeFightingStyleChoice("protection"),
			makeCantripChoices([]string{"Mage Hand", "Minor Illusion"}),
			makeSpellChoices([]string{"Shield", "Magic Missile"}),
			makeEquipmentChoices([]string{"Plate Armor", "Shield", "Longsword", "Shortbow"}),
		),
		Progress: DraftProgress{
			flags: ProgressName | ProgressRace | ProgressClass | ProgressBackground | ProgressAbilityScores,
		},
	}

	character, err := draft.ToCharacter(s.elfRace, s.fighterClass, s.soldierBg)
	s.Require().NoError(err)
	s.Require().NotNil(character)

	// Verify all choice categories are present
	expectedCategories := map[shared.ChoiceCategory]bool{
		shared.ChoiceName:          false,
		shared.ChoiceRace:          false,
		shared.ChoiceClass:         false,
		shared.ChoiceBackground:    false,
		shared.ChoiceAbilityScores: false,
		shared.ChoiceSkills:        false,
		shared.ChoiceLanguages:     false,
		shared.ChoiceFightingStyle: false,
		shared.ChoiceCantrips:      false,
		shared.ChoiceSpells:        false,
		shared.ChoiceEquipment:     false,
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
	sourceMap := map[shared.ChoiceCategory]shared.ChoiceSource{
		shared.ChoiceName:          shared.SourcePlayer,
		shared.ChoiceRace:          shared.SourcePlayer, // Player chooses their race
		shared.ChoiceClass:         shared.SourcePlayer, // Player chooses their class
		shared.ChoiceBackground:    shared.SourcePlayer, // Player chooses their background
		shared.ChoiceAbilityScores: shared.SourcePlayer,
		shared.ChoiceSkills:        shared.SourceClass, // Skills come from class options
		shared.ChoiceLanguages:     shared.SourceRace,  // Extra languages come from race
		shared.ChoiceFightingStyle: shared.SourceClass, // Fighting style from class
		shared.ChoiceCantrips:      shared.SourceClass, // Cantrips from class
		shared.ChoiceSpells:        shared.SourceClass, // Spells from class
		shared.ChoiceEquipment:     shared.SourceClass, // Equipment from class
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
	s.Assert().Equal(16, character.abilityScores[abilities.DEX]) // 14 + 2 (elf)
	s.Assert().Equal(13, character.abilityScores[abilities.INT]) // 12 + 1 (high elf)
	s.Assert().Contains(character.languages, languages.Common)
	s.Assert().Contains(character.languages, languages.Elvish)
	s.Assert().Contains(character.languages, languages.Draconic)
	s.Assert().Contains(character.languages, languages.Giant)
	s.Assert().Equal(shared.Proficient, character.skills[skills.Perception])
	s.Assert().Equal(shared.Proficient, character.skills[skills.History])
}

func (s *DraftConversionTestSuite) TestEquipmentProcessing() {
	// Create a draft with equipment choices including bundles
	draft := &Draft{
		ID:         "test-equipment",
		PlayerID:   "player-eq",
		Name:       "Equipment Tester",
		RaceChoice: RaceChoice{RaceID: races.Human},
		ClassChoice: ClassChoice{
			ClassID: classes.Fighter,
		},
		BackgroundChoice: backgrounds.Soldier,
		AbilityScoreChoice: shared.AbilityScores{
			abilities.STR: 15,
			abilities.DEX: 13,
			abilities.CON: 14,
			abilities.INT: 10,
			abilities.WIS: 12,
			abilities.CHA: 11,
		},
		Choices: combineChoices(
			makeSkillChoices(string(classes.Fighter), []skills.Skill{skills.Perception, skills.Survival}),
			makeEquipmentChoices([]string{"Longsword", "Dungeoneer's Pack"}),
		),
	}
	// Set progress flags
	draft.Progress.flags = ProgressName | ProgressRace | ProgressClass | ProgressBackground |
		ProgressAbilityScores | ProgressSkills | ProgressEquipment

	// Convert to character
	character, err := draft.ToCharacter(s.humanRace, s.fighterClass, s.soldierBg)
	s.Require().NoError(err)

	// Get character equipment
	equipment := character.GetEquipment()

	// Verify starting equipment from class
	s.Assert().Contains(equipment, "Chain Mail")
	s.Assert().Contains(equipment, "Shield")
	s.Assert().Contains(equipment, "Javelin (5)")

	// Verify background equipment
	s.Assert().Contains(equipment, "Insignia of rank")
	s.Assert().Contains(equipment, "Trophy")
	s.Assert().Contains(equipment, "Deck of cards")
	s.Assert().Contains(equipment, "Common clothes")

	// Verify equipment choices
	s.Assert().Contains(equipment, "Longsword")

	// Verify Dungeoneer's Pack was expanded
	s.Assert().Contains(equipment, "Backpack")
	s.Assert().Contains(equipment, "Crowbar")
	s.Assert().Contains(equipment, "Hammer")
	s.Assert().Contains(equipment, "Piton (10)")
	s.Assert().Contains(equipment, "Torch (10)")
	s.Assert().Contains(equipment, "Tinderbox")
	s.Assert().Contains(equipment, "Rations (10 days)")
	s.Assert().Contains(equipment, "Waterskin")
	s.Assert().Contains(equipment, "Hempen Rope (50 feet)")

	// Verify equipment choice was stored
	var equipmentChoice *ChoiceData
	for i := range character.choices {
		if character.choices[i].Category == shared.ChoiceEquipment {
			equipmentChoice = &character.choices[i]
			break
		}
	}
	s.Require().NotNil(equipmentChoice, "Equipment choice should be stored")
	s.Require().NotNil(equipmentChoice.EquipmentSelection)
	s.Assert().Contains(equipmentChoice.EquipmentSelection, "Longsword")
	s.Assert().Contains(equipmentChoice.EquipmentSelection, "Dungeoneer's Pack")
}

func (s *DraftConversionTestSuite) TestClassResourcesInitialization() {
	// Set up fighter with resources
	s.fighterClass.Resources = []class.ResourceData{
		{
			Type:       shared.ClassResourceSecondWind,
			Name:       "Second Wind",
			MaxFormula: "1",
			Resets:     shared.ShortRest,
		},
		{
			Type:       shared.ClassResourceActionSurge,
			Name:       "Action Surge",
			MaxFormula: "1", // Would increase at higher levels
			Resets:     shared.ShortRest,
		},
	}

	// Create a draft
	draft := &Draft{
		ID:         "test-resources",
		PlayerID:   "player-res",
		Name:       "Resource Fighter",
		RaceChoice: RaceChoice{RaceID: races.Human},
		ClassChoice: ClassChoice{
			ClassID: classes.Fighter,
		},
		BackgroundChoice: backgrounds.Soldier,
		AbilityScoreChoice: shared.AbilityScores{
			abilities.STR: 16,
			abilities.DEX: 14,
			abilities.CON: 15,
			abilities.INT: 10,
			abilities.WIS: 13,
			abilities.CHA: 12,
		},
		Choices: makeSkillChoices(string(classes.Fighter), []skills.Skill{skills.History, skills.Perception}),
	}
	draft.Progress.flags = ProgressName | ProgressRace | ProgressClass | ProgressBackground |
		ProgressAbilityScores | ProgressSkills

	// Convert to character
	character, err := draft.ToCharacter(s.humanRace, s.fighterClass, s.soldierBg)
	s.Require().NoError(err)

	// Check resources were initialized
	resources := character.GetClassResources()
	s.Len(resources, 2)

	// Check Second Wind
	secondWind, ok := resources[shared.ClassResourceSecondWind]
	s.Require().True(ok)
	s.Equal("Second Wind", secondWind.Name)
	s.Equal(1, secondWind.Max)
	s.Equal(1, secondWind.Current)
	s.Equal(shared.ShortRest, secondWind.Resets)

	// Check Action Surge
	actionSurge, ok := resources[shared.ClassResourceActionSurge]
	s.Require().True(ok)
	s.Equal("Action Surge", actionSurge.Name)
	s.Equal(1, actionSurge.Max)
	s.Equal(1, actionSurge.Current)
}

func (s *DraftConversionTestSuite) TestSpellSlotsInitialization() {
	// Set up wizard with spell slots
	s.wizardClass.Spellcasting = &class.SpellcastingData{
		Ability: "Intelligence",
		SpellSlots: map[int][]int{
			1: {2},    // Level 1: 2 first-level slots
			2: {3},    // Level 2: 3 first-level slots
			3: {4, 2}, // Level 3: 4 first, 2 second
		},
	}

	// Create a wizard draft
	draft := &Draft{
		ID:         "test-spellslots",
		PlayerID:   "player-spell",
		Name:       "Spell Wizard",
		RaceChoice: RaceChoice{RaceID: races.Elf, SubraceID: races.HighElf},
		ClassChoice: ClassChoice{
			ClassID: classes.Wizard,
		},
		BackgroundChoice: backgrounds.Hermit,
		AbilityScoreChoice: shared.AbilityScores{
			abilities.STR: 8,
			abilities.DEX: 14,
			abilities.CON: 13,
			abilities.INT: 16,
			abilities.WIS: 12,
			abilities.CHA: 10,
		},
		Choices: makeSkillChoices(string(classes.Wizard), []skills.Skill{skills.Arcana, skills.Investigation}),
	}
	draft.Progress.flags = ProgressName | ProgressRace | ProgressClass | ProgressBackground |
		ProgressAbilityScores | ProgressSkills

	// Convert to character
	character, err := draft.ToCharacter(s.elfRace, s.wizardClass, s.hermitBg)
	s.Require().NoError(err)

	// Check spell slots were initialized (level 1 wizard)
	spellSlots := character.GetSpellSlots()
	s.Len(spellSlots, 1)

	slot1, ok := spellSlots[1]
	s.Require().True(ok)
	s.Equal(2, slot1.Max)
	s.Equal(0, slot1.Used)
}

func TestDraftConversionTestSuite(t *testing.T) {
	suite.Run(t, new(DraftConversionTestSuite))
}
