// Package character provides D&D 5e character creation and management functionality
package character

import (
	"encoding/json"
	"errors"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/abilities"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/class"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/effects"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/languages"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/race"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/shared"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/skills"
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
	conMod := abilityScores.Modifier(abilities.CON)
	maxHP := data.ClassData.HitDice + conMod

	// Build skills map
	skillProfs := make(map[skills.Skill]shared.ProficiencyLevel)

	// Add background skills
	for _, skill := range data.BackgroundData.SkillProficiencies {
		skillProfs[skill] = shared.Proficient
	}

	// Add chosen skills
	if chosenSkills, ok := data.Choices["skills"].([]string); ok {
		for _, skillStr := range chosenSkills {
			skill := skills.Skill(skillStr)
			skillProfs[skill] = shared.Proficient
		}
	}

	// Build saving throws
	saves := make(map[abilities.Ability]shared.ProficiencyLevel)
	for _, save := range data.ClassData.SavingThrows {
		saves[save] = shared.Proficient
	}

	// Compile languages - ensure Common is always included
	languageSet := make(map[languages.Language]bool)
	languageSet[languages.Common] = true

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
			lang := languages.Language(langStr)
			languageSet[lang] = true
		}
	}

	// Convert set to slice
	languages := make([]languages.Language, 0, len(languageSet))
	for lang := range languageSet {
		languages = append(languages, lang)
	}

	// Compile proficiencies
	proficiencies := shared.Proficiencies{
		Armor:   data.ClassData.ArmorProficiencies,
		Weapons: append(data.ClassData.WeaponProficiencies, data.RaceData.WeaponProficiencies...),
		Tools:   append(data.ClassData.ToolProficiencies, data.BackgroundData.ToolProficiencies...),
	}

	// Extract features (store as JSON for persistence)
	level1Features := data.ClassData.Features[1]
	features := make([]json.RawMessage, 0, len(level1Features))
	for _, feature := range level1Features { // Level 1 features
		// For now, store feature ID as simple JSON
		featureJSON, _ := json.Marshal(map[string]string{"id": feature.ID})
		features = append(features, featureJSON)
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
		armorClass:       10 + abilityScores.Modifier(abilities.DEX),
		initiative:       abilityScores.Modifier(abilities.DEX),
		hitDice:          data.ClassData.HitDice,
		skills:           skillProfs,
		savingThrows:     saves,
		languages:        languages,
		proficiencies:    proficiencies,
		features:         features,
		conditions:       []json.RawMessage{},
		effects:          []effects.Effect{},
		exhaustion:       0,
		deathSaves:       shared.DeathSaves{},
		spellSlots:       make(SpellSlots),
		classResources:   make(map[shared.ClassResourceType]Resource),
		choices:          buildChoiceData(data),
	}

	return char, nil
}

func buildChoiceData(data CreationData) []ChoiceData {
	choices := make([]ChoiceData, 0, len(data.Choices))

	// Record all choices made - need to handle the legacy map[string]any format
	for category, selection := range data.Choices {
		choiceData := ChoiceData{
			Category: shared.ChoiceCategory(category),
			Source:   shared.SourcePlayer,
			ChoiceID: "", // No specific choice ID for legacy creation data
		}

		// Convert the selection based on category
		convertLegacyChoice(&choiceData, selection)

		choices = append(choices, choiceData)
	}

	return choices
}

// convertLegacyChoice converts legacy selection data to the appropriate typed field
func convertLegacyChoice(choiceData *ChoiceData, selection any) {
	switch choiceData.Category {
	case shared.ChoiceName:
		if name, ok := selection.(string); ok {
			choiceData.NameSelection = &name
		}
	case shared.ChoiceSkills:
		choiceData.SkillSelection = convertToSkills(selection)
	case shared.ChoiceLanguages:
		choiceData.LanguageSelection = convertToLanguages(selection)
	case shared.ChoiceEquipment:
		choiceData.EquipmentSelection = convertToStringSlice(selection)
	case shared.ChoiceFightingStyle:
		if style, ok := selection.(string); ok {
			choiceData.FightingStyleSelection = &style
		}
	case shared.ChoiceSpells:
		choiceData.SpellSelection = convertToStringSlice(selection)
	case shared.ChoiceCantrips:
		choiceData.CantripSelection = convertToStringSlice(selection)
		// Complex types (AbilityScores, Race, Class, Background) are not supported in legacy creation
	}
}

// convertToSkills converts various formats to []skills.Skill
func convertToSkills(selection any) []skills.Skill {
	switch v := selection.(type) {
	case []string:
		result := make([]skills.Skill, len(v))
		for i, s := range v {
			result[i] = skills.Skill(s)
		}
		return result
	case []interface{}:
		result := make([]skills.Skill, 0, len(v))
		for _, item := range v {
			if s, ok := item.(string); ok {
				result = append(result, skills.Skill(s))
			}
		}
		return result
	}
	return nil
}

// convertToLanguages converts various formats to []languages.Language
func convertToLanguages(selection any) []languages.Language {
	switch v := selection.(type) {
	case []string:
		langs := make([]languages.Language, len(v))
		for i, l := range v {
			langs[i] = languages.Language(l)
		}
		return langs
	case []interface{}:
		langs := make([]languages.Language, 0, len(v))
		for _, item := range v {
			if l, ok := item.(string); ok {
				langs = append(langs, languages.Language(l))
			}
		}
		return langs
	}
	return nil
}

// convertToStringSlice converts various formats to []string
func convertToStringSlice(selection any) []string {
	switch v := selection.(type) {
	case []string:
		return v
	case []interface{}:
		result := make([]string, 0, len(v))
		for _, item := range v {
			if s, ok := item.(string); ok {
				result = append(result, s)
			}
		}
		return result
	}
	return nil
}
