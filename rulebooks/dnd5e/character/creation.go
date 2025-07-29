// Package character provides D&D 5e character creation and management functionality
package character

import (
	"errors"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/class"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/conditions"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/constants"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/effects"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/race"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/shared"
)

// CreationData contains all data needed to create a character
type CreationData struct {
	ID             string // Required - must be provided by caller
	PlayerID       string
	Name           string
	RaceData       *race.Data
	SubraceID      string
	ClassData      *class.Data
	BackgroundData *shared.Background
	AbilityScores  shared.AbilityScores
	Choices        map[string]any // skill selections, language choices, etc.
}

// NewFromCreationData creates a character from creation data
// This is a simpler alternative to the builder pattern
func NewFromCreationData(data CreationData) (*Character, error) {
	// Validate required fields
	if data.ID == "" {
		return nil, errors.New("character ID is required")
	}
	if data.Name == "" {
		return nil, errors.New("name is required")
	}
	if data.RaceData == nil || data.ClassData == nil || data.BackgroundData == nil {
		return nil, errors.New("race, class, and background data are required")
	}

	// Apply racial ability score improvements
	abilityScores := data.AbilityScores
	// Ignore errors about exceeding 20 during creation
	_ = abilityScores.ApplyIncreases(data.RaceData.AbilityScoreIncreases)

	// Calculate HP
	conMod := abilityScores.Modifier(constants.CON)
	maxHP := data.ClassData.HitDice + conMod

	// Build skills map
	skills := make(map[constants.Skill]shared.ProficiencyLevel)

	// Add background skills
	for _, skill := range data.BackgroundData.SkillProficiencies {
		skills[skill] = shared.Proficient
	}

	// Add chosen skills
	if chosenSkills, ok := data.Choices["skills"].([]string); ok {
		for _, skillStr := range chosenSkills {
			skill := constants.Skill(skillStr)
			skills[skill] = shared.Proficient
		}
	}

	// Build saving throws
	saves := make(map[constants.Ability]shared.ProficiencyLevel)
	for _, save := range data.ClassData.SavingThrows {
		saves[save] = shared.Proficient
	}

	// Compile languages - ensure Common is always included
	languageSet := make(map[constants.Language]bool)
	languageSet[constants.LanguageCommon] = true

	// Add race languages
	for _, lang := range data.RaceData.Languages {
		languageSet[lang] = true
	}

	// Add background languages
	for _, lang := range data.BackgroundData.Languages {
		languageSet[lang] = true
	}

	// Add chosen languages
	if chosenLangs, ok := data.Choices["languages"].([]string); ok {
		for _, langStr := range chosenLangs {
			lang := constants.Language(langStr)
			languageSet[lang] = true
		}
	}

	// Convert set to slice
	languages := make([]constants.Language, 0, len(languageSet))
	for lang := range languageSet {
		languages = append(languages, lang)
	}

	// Compile proficiencies
	proficiencies := shared.Proficiencies{
		Armor:   data.ClassData.ArmorProficiencies,
		Weapons: append(data.ClassData.WeaponProficiencies, data.RaceData.WeaponProficiencies...),
		Tools:   append(data.ClassData.ToolProficiencies, data.BackgroundData.ToolProficiencies...),
	}

	// Extract features
	level1Features := data.ClassData.Features[1]
	features := make([]string, 0, len(level1Features))
	for _, feature := range level1Features { // Level 1 features
		features = append(features, feature.ID)
	}

	// Build character
	char := &Character{
		id:               data.ID,
		playerID:         data.PlayerID,
		name:             data.Name,
		level:            1,
		proficiencyBonus: 2,
		raceID:           data.RaceData.ID,
		classID:          data.ClassData.ID,
		backgroundID:     data.BackgroundData.ID,
		abilityScores:    abilityScores,
		size:             data.RaceData.Size,
		speed:            data.RaceData.Speed,
		hitPoints:        maxHP,
		maxHitPoints:     maxHP,
		tempHitPoints:    0,
		armorClass:       10 + abilityScores.Modifier(constants.DEX),
		initiative:       abilityScores.Modifier(constants.DEX),
		hitDice:          data.ClassData.HitDice,
		skills:           skills,
		savingThrows:     saves,
		languages:        languages,
		proficiencies:    proficiencies,
		features:         features,
		conditions:       []conditions.Condition{},
		effects:          []effects.Effect{},
		exhaustion:       0,
		deathSaves:       shared.DeathSaves{},
		spellSlots:       make(SpellSlots),
		classResources:   make(map[string]Resource),
		choices:          buildChoiceData(data),
	}

	return char, nil
}

func buildChoiceData(data CreationData) []ChoiceData {
	choices := make([]ChoiceData, 0, len(data.Choices))

	// Record all choices made
	for category, selection := range data.Choices {
		choiceData := ChoiceData{
			Category:  category,
			Source:    "creation",
			ChoiceID:  "", // No specific choice ID for legacy creation data
			Selection: selection,
		}
		choices = append(choices, choiceData)
	}

	return choices
}
