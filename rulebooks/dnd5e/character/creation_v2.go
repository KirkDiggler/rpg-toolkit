// Package character provides D&D 5e character creation and management functionality
package character

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/KirkDiggler/rpg-toolkit/core"
	"github.com/KirkDiggler/rpg-toolkit/events"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/constants"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/shared"
)

// CharacterDef represents character data using refs for everything
type CharacterDef struct {
	ID       string `json:"id"`
	PlayerID string `json:"player_id"`
	Name     string `json:"name"`
	Level    int    `json:"level"`

	// Core identity as refs
	RaceRef       string `json:"race_ref"`       // e.g., "dnd5e:race:human"
	ClassRef      string `json:"class_ref"`      // e.g., "dnd5e:class:fighter"
	BackgroundRef string `json:"background_ref"` // e.g., "dnd5e:background:soldier"

	// Subtype refs (optional)
	SubraceRef  string `json:"subrace_ref,omitempty"`  // e.g., "dnd5e:subrace:mountain_dwarf"
	SubclassRef string `json:"subclass_ref,omitempty"` // e.g., "dnd5e:subclass:champion"

	// Core attributes
	AbilityScores shared.AbilityScores `json:"ability_scores"`

	// Combat stats
	HitPoints    int `json:"hit_points"`
	MaxHitPoints int `json:"max_hit_points"`

	// Features as refs
	Features []string `json:"features"` // e.g., ["dnd5e:features:rage", "dnd5e:features:second_wind"]

	// Conditions as JSON (already working)
	Conditions []json.RawMessage `json:"conditions,omitempty"`

	// Skills, languages, and proficiencies using refs
	Skills        []string `json:"skills"`        // e.g., ["dnd5e:skill:athletics"]
	Languages     []string `json:"languages"`     // e.g., ["dnd5e:language:common"]
	Proficiencies []string `json:"proficiencies"` // e.g., ["dnd5e:proficiency:martial_weapons"]

	// Equipment as refs
	Equipment []string `json:"equipment"` // e.g., ["dnd5e:item:longsword"]

	// Choices made during creation (simplified)
	Choices []CharacterChoice `json:"choices"`
}

// CharacterChoice represents a choice made during character creation
type CharacterChoice struct {
	Ref       string   `json:"ref"`       // e.g., "dnd5e:choice:fighter_skills"
	Source    string   `json:"source"`    // What granted this choice (e.g., "dnd5e:class:fighter")
	Selected  []string `json:"selected"`  // What was chosen (as refs)
	Timestamp string   `json:"timestamp"` // When the choice was made
}

// LoadCharacterDef loads a character from JSON data
func LoadCharacterDef(data []byte, bus events.EventBus) (*Character, error) {
	var def CharacterDef
	if err := json.Unmarshal(data, &def); err != nil {
		return nil, fmt.Errorf("failed to unmarshal character def: %w", err)
	}

	return LoadFromDef(&def, bus)
}

// LoadFromDef creates a Character from a CharacterDef
func LoadFromDef(def *CharacterDef, bus events.EventBus) (*Character, error) {
	if def.ID == "" {
		return nil, errors.New("character ID is required")
	}
	if def.Name == "" {
		return nil, errors.New("character name is required")
	}

	// Parse refs to extract IDs for legacy compatibility
	raceID, err := extractIDFromRef(def.RaceRef, "race")
	if err != nil {
		return nil, err
	}

	classID, err := extractIDFromRef(def.ClassRef, "class")
	if err != nil {
		return nil, err
	}

	backgroundID, err := extractIDFromRef(def.BackgroundRef, "background")
	if err != nil {
		return nil, err
	}

	// Build skills map from refs
	skills := make(map[constants.Skill]shared.ProficiencyLevel)
	for _, skillRef := range def.Skills {
		skillID, err := extractIDFromRef(skillRef, "skill")
		if err != nil {
			continue // Skip invalid skill refs
		}
		skills[constants.Skill(skillID)] = shared.Proficient
	}

	// Build languages from refs
	languages := make([]constants.Language, 0, len(def.Languages))
	for _, langRef := range def.Languages {
		langID, err := extractIDFromRef(langRef, "language")
		if err != nil {
			continue // Skip invalid language refs
		}
		languages = append(languages, constants.Language(langID))
	}

	// Load features as JSON
	features := make([]json.RawMessage, 0, len(def.Features))
	for _, featureRef := range def.Features {
		// For now, store feature refs as simple JSON
		// In the future, we'd load the actual feature data
		featureData := map[string]string{
			"ref": featureRef,
			"id":  generateFeatureID(featureRef),
		}
		featureJSON, _ := json.Marshal(featureData)
		features = append(features, featureJSON)
	}

	// Calculate derived stats
	proficiencyBonus := 2 + (def.Level-1)/4

	// Build saving throws (would come from class data)
	saves := make(map[constants.Ability]shared.ProficiencyLevel)
	// For now, hardcode based on class
	if classID == "fighter" || classID == "barbarian" {
		saves[constants.STR] = shared.Proficient
		saves[constants.CON] = shared.Proficient
	} else if classID == "wizard" {
		saves[constants.INT] = shared.Proficient
		saves[constants.WIS] = shared.Proficient
	}

	// Build proficiencies
	proficiencies := shared.Proficiencies{
		Armor:   []string{},
		Weapons: []string{},
		Tools:   []string{},
	}

	// Parse proficiency refs
	for _, profRef := range def.Proficiencies {
		profID, err := extractIDFromRef(profRef, "proficiency")
		if err != nil {
			continue
		}
		// Categorize proficiencies (simplified for now)
		if contains([]string{"light_armor", "medium_armor", "heavy_armor", "shields"}, profID) {
			proficiencies.Armor = append(proficiencies.Armor, profID)
		} else if contains([]string{"simple_weapons", "martial_weapons"}, profID) {
			proficiencies.Weapons = append(proficiencies.Weapons, profID)
		} else {
			proficiencies.Tools = append(proficiencies.Tools, profID)
		}
	}

	// Build simplified choice data
	choices := make([]ChoiceData, 0, len(def.Choices))
	for _, choice := range def.Choices {
		// Convert new choice format to legacy format for compatibility
		choiceData := ChoiceData{
			Category: shared.ChoiceSkills, // Would be determined from ref
			Source:   shared.SourceClass,  // Would be determined from source
			ChoiceID: choice.Ref,
		}
		
		// Set selection based on what was chosen
		if len(choice.Selected) > 0 {
			// Simplified - would need proper conversion based on choice type
			skills := make([]constants.Skill, 0, len(choice.Selected))
			for _, sel := range choice.Selected {
				if id, err := extractIDFromRef(sel, "skill"); err == nil {
					skills = append(skills, constants.Skill(id))
				}
			}
			if len(skills) > 0 {
				choiceData.SkillSelection = skills
			}
		}
		
		choices = append(choices, choiceData)
	}

	// Create character
	char := &Character{
		id:               def.ID,
		playerID:         def.PlayerID,
		name:             def.Name,
		level:            def.Level,
		proficiencyBonus: proficiencyBonus,
		eventBus:         bus,

		// Identity
		raceID:       constants.Race(raceID),
		classID:      constants.Class(classID),
		backgroundID: constants.Background(backgroundID),

		// Core attributes
		abilityScores: def.AbilityScores,

		// Physical (would come from race data)
		size:  "medium", // Would be loaded from race data
		speed: 30,       // Would be loaded from race data

		// Combat
		hitPoints:    def.HitPoints,
		maxHitPoints: def.MaxHitPoints,
		armorClass:   10 + def.AbilityScores.Modifier(constants.DEX),
		initiative:   def.AbilityScores.Modifier(constants.DEX),
		hitDice:      10, // Would come from class data

		// Capabilities
		skills:        skills,
		savingThrows:  saves,
		languages:     languages,
		proficiencies: proficiencies,
		features:      features,

		// State
		conditions:    def.Conditions,
		exhaustion:    0,
		deathSaves:    shared.DeathSaves{},
		tempHitPoints: 0,

		// Resources
		spellSlots:     make(SpellSlots),
		classResources: make(map[shared.ClassResourceType]Resource),

		// Equipment
		equipment: def.Equipment,

		// Choices
		choices: choices,
	}

	// Apply event handlers if we have an event bus
	if bus != nil {
		char.ApplyToEventBus(context.Background(), bus)
	}

	return char, nil
}

// extractIDFromRef extracts the ID from a ref string
func extractIDFromRef(refStr string, expectedType string) (string, error) {
	if refStr == "" {
		return "", fmt.Errorf("empty ref")
	}

	ref, err := core.ParseString(refStr)
	if err != nil {
		return "", fmt.Errorf("invalid ref %q: %w", refStr, err)
	}

	if ref.Type != expectedType {
		return "", fmt.Errorf("expected %s ref, got %s", expectedType, ref.Type)
	}

	return ref.Value, nil
}

// generateFeatureID creates a unique ID for a feature from its ref
func generateFeatureID(featureRef string) string {
	ref, err := core.ParseString(featureRef)
	if err != nil {
		return featureRef // Fallback to the full ref
	}
	return fmt.Sprintf("feature_%s", ref.Value)
}

// contains checks if a string is in a slice
func contains(slice []string, item string) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}

// ToDef converts a Character to a CharacterDef for persistence
func (c *Character) ToDef() *CharacterDef {
	// Build skill refs
	skills := make([]string, 0, len(c.skills))
	for skill, level := range c.skills {
		if level >= shared.Proficient {
			skills = append(skills, fmt.Sprintf("dnd5e:skill:%s", skill))
		}
	}

	// Build language refs
	languages := make([]string, 0, len(c.languages))
	for _, lang := range c.languages {
		languages = append(languages, fmt.Sprintf("dnd5e:language:%s", lang))
	}

	// Build proficiency refs
	proficiencies := make([]string, 0)
	for _, armor := range c.proficiencies.Armor {
		proficiencies = append(proficiencies, fmt.Sprintf("dnd5e:proficiency:%s", armor))
	}
	for _, weapon := range c.proficiencies.Weapons {
		proficiencies = append(proficiencies, fmt.Sprintf("dnd5e:proficiency:%s", weapon))
	}
	for _, tool := range c.proficiencies.Tools {
		proficiencies = append(proficiencies, fmt.Sprintf("dnd5e:proficiency:%s", tool))
	}

	// Build feature refs from stored JSON
	features := make([]string, 0, len(c.features))
	for _, featureData := range c.features {
		var peek struct {
			Ref string `json:"ref"`
		}
		if err := json.Unmarshal(featureData, &peek); err == nil && peek.Ref != "" {
			features = append(features, peek.Ref)
		}
	}

	// Convert choices to new format
	choices := make([]CharacterChoice, 0, len(c.choices))
	for _, oldChoice := range c.choices {
		choice := CharacterChoice{
			Ref:    oldChoice.ChoiceID,
			Source: string(oldChoice.Source),
		}
		
		// Convert selections to refs
		if oldChoice.SkillSelection != nil {
			selected := make([]string, 0, len(oldChoice.SkillSelection))
			for _, skill := range oldChoice.SkillSelection {
				selected = append(selected, fmt.Sprintf("dnd5e:skill:%s", skill))
			}
			choice.Selected = selected
		}
		
		choices = append(choices, choice)
	}

	return &CharacterDef{
		ID:       c.id,
		PlayerID: c.playerID,
		Name:     c.name,
		Level:    c.level,

		// Core identity as refs
		RaceRef:       fmt.Sprintf("dnd5e:race:%s", c.raceID),
		ClassRef:      fmt.Sprintf("dnd5e:class:%s", c.classID),
		BackgroundRef: fmt.Sprintf("dnd5e:background:%s", c.backgroundID),

		// Core attributes
		AbilityScores: c.abilityScores,

		// Combat stats
		HitPoints:    c.hitPoints,
		MaxHitPoints: c.maxHitPoints,

		// Features, conditions, etc.
		Features:      features,
		Conditions:    c.conditions,
		Skills:        skills,
		Languages:     languages,
		Proficiencies: proficiencies,
		Equipment:     c.equipment,
		Choices:       choices,
	}
}