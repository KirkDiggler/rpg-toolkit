// Package character provides D&D 5e character creation and management functionality
package character

import (
	"errors"

	"github.com/google/uuid"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/class"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/conditions"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/effects"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/race"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/shared"
)

// CreationData contains all data needed to create a character
type CreationData struct {
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
	if data.Name == "" {
		return nil, errors.New("name is required")
	}
	if data.RaceData == nil || data.ClassData == nil || data.BackgroundData == nil {
		return nil, errors.New("race, class, and background data are required")
	}

	// Apply racial ability score improvements
	abilityScores := data.AbilityScores
	for ability, bonus := range data.RaceData.AbilityScoreIncreases {
		switch ability {
		case shared.AbilityStrength:
			abilityScores.Strength += bonus
		case shared.AbilityDexterity:
			abilityScores.Dexterity += bonus
		case shared.AbilityConstitution:
			abilityScores.Constitution += bonus
		case shared.AbilityIntelligence:
			abilityScores.Intelligence += bonus
		case shared.AbilityWisdom:
			abilityScores.Wisdom += bonus
		case shared.AbilityCharisma:
			abilityScores.Charisma += bonus
		}
	}

	// Calculate HP
	conMod := (abilityScores.Constitution - 10) / 2
	maxHP := data.ClassData.HitPointsAt1st + conMod

	// Build skills map
	skills := make(map[string]shared.ProficiencyLevel)

	// Add background skills
	for _, skill := range data.BackgroundData.SkillProficiencies {
		skills[skill] = shared.Proficient
	}

	// Add chosen skills
	if chosenSkills, ok := data.Choices["skills"].([]string); ok {
		for _, skill := range chosenSkills {
			skills[skill] = shared.Proficient
		}
	}

	// Build saving throws
	saves := make(map[string]shared.ProficiencyLevel)
	for _, save := range data.ClassData.SavingThrows {
		saves[save] = shared.Proficient
	}

	// Compile languages
	languages := append([]string{}, data.RaceData.Languages...)
	languages = append(languages, data.BackgroundData.Languages...)
	if chosenLangs, ok := data.Choices["languages"].([]string); ok {
		languages = append(languages, chosenLangs...)
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
		id:               generateID(), // You implement this
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
		armorClass:       10 + ((abilityScores.Dexterity - 10) / 2),
		initiative:       (abilityScores.Dexterity - 10) / 2,
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

func generateID() string {
	// Use UUID for unique ID generation to avoid collisions
	return uuid.New().String()
}

func buildChoiceData(data CreationData) []ChoiceData {
	choices := make([]ChoiceData, 0, len(data.Choices))

	// Record all choices made
	for category, selection := range data.Choices {
		choices = append(choices, ChoiceData{
			Category:  category,
			Source:    "creation",
			Selection: selection,
		})
	}

	return choices
}
