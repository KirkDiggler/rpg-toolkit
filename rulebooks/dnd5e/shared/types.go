// Package shared provides common types used across the D&D 5e rulebook
package shared

import (
	"fmt"

	"github.com/KirkDiggler/rpg-toolkit/core"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/abilities"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/proficiencies"
)

// SelectionID represents any selectable game content ID.
// This is an alias for core.ID, connecting our domain types to the ref system.
type SelectionID = core.ID

// AbilityScores maps ability constants to their scores (includes all bonuses)
type AbilityScores map[abilities.Ability]int

// AbilityScoreConfig provides a clear way to specify all ability scores at creation
type AbilityScoreConfig struct {
	STR int
	DEX int
	CON int
	INT int
	WIS int
	CHA int
}

// Validate ensures all scores are in valid range for character creation (3-18)
func (c *AbilityScoreConfig) Validate() error {
	scores := []struct {
		name  string
		value int
	}{
		{"STR", c.STR},
		{"DEX", c.DEX},
		{"CON", c.CON},
		{"INT", c.INT},
		{"WIS", c.WIS},
		{"CHA", c.CHA},
	}

	for _, score := range scores {
		if score.value < 3 || score.value > 18 {
			return fmt.Errorf("ability %s must be between 3-18 for character creation, got %d", score.name, score.value)
		}
	}
	return nil
}

// NewAbilityScores creates ability scores with validation
func NewAbilityScores(config *AbilityScoreConfig) (AbilityScores, error) {
	if err := config.Validate(); err != nil {
		return nil, err
	}

	return AbilityScores{
		abilities.STR: config.STR,
		abilities.DEX: config.DEX,
		abilities.CON: config.CON,
		abilities.INT: config.INT,
		abilities.WIS: config.WIS,
		abilities.CHA: config.CHA,
	}, nil
}

// Increase increments a specific ability, enforcing the maximum of 20
func (a AbilityScores) Increase(ability abilities.Ability, amount int) error {
	newValue := a[ability] + amount
	if newValue > 20 {
		return fmt.Errorf("ability %s cannot exceed 20 (current: %d, increase: %d)", ability.Display(), a[ability], amount)
	}
	a[ability] = newValue
	return nil
}

// ApplyIncreases applies multiple increases at once (e.g., racial bonuses)
func (a AbilityScores) ApplyIncreases(increases map[abilities.Ability]int) error {
	// Check all increases first
	for ability, bonus := range increases {
		if a[ability]+bonus > 20 {
			return fmt.Errorf("ability %s would exceed 20 with increase", ability.Display())
		}
	}
	// Then apply
	for ability, bonus := range increases {
		a[ability] += bonus
	}
	return nil
}

// Modifier calculates the ability modifier for a given ability
func (a AbilityScores) Modifier(ability abilities.Ability) int {
	return (a[ability] - 10) / 2
}

// ProficiencyLevel represents expertise levels
type ProficiencyLevel int

const (
	// NotProficient indicates no proficiency
	NotProficient ProficiencyLevel = iota
	// Proficient indicates normal proficiency
	Proficient
	// Expertise indicates double proficiency bonus
	Expertise
	// Expert indicates double proficiency bonus (alias for Expertise)
	Expert ProficiencyLevel = 2
)

// ResetType defines when a resource resets
type ResetType string

const (
	// ResetTypeShortRest resets on a short rest
	ResetTypeShortRest ResetType = "short_rest"
	// ResetTypeLongRest resets on a long rest
	ResetTypeLongRest ResetType = "long_rest"
	// ResetTypeDawn resets at dawn
	ResetTypeDawn ResetType = "dawn"
	// ResetTypeNone never recharges (consumable)
	ResetTypeNone ResetType = "none"
)

// ClassResourceType represents different types of class resources
type ClassResourceType int

const (
	// ClassResourceUnspecified is the zero value
	ClassResourceUnspecified ClassResourceType = iota
	// ClassResourceRage for barbarian rage
	ClassResourceRage
	// ClassResourceBardicInspiration for bard
	ClassResourceBardicInspiration
	// ClassResourceChannelDivinity for cleric/paladin
	ClassResourceChannelDivinity
	// ClassResourceWildShape for druid
	ClassResourceWildShape
	// ClassResourceSecondWind for fighter
	ClassResourceSecondWind
	// ClassResourceActionSurge for fighter
	ClassResourceActionSurge
	// ClassResourceKiPoints for monk
	ClassResourceKiPoints
	// ClassResourceDivineSense for paladin
	ClassResourceDivineSense
	// ClassResourceLayOnHands for paladin
	ClassResourceLayOnHands
	// ClassResourceSorceryPoints for sorcerer
	ClassResourceSorceryPoints
	// ClassResourceArcaneRecovery for wizard
	ClassResourceArcaneRecovery
	// ClassResourceIndomitable for fighter
	ClassResourceIndomitable
	// ClassResourceSuperiorityDice for battle master
	ClassResourceSuperiorityDice
)

// Note: No String() or Parse() methods - use the enum directly as the identifier.
// This keeps the system lean and avoids dual representations.

// ChoiceCategory represents different types of choices during creation
type ChoiceCategory string

const (
	// ChoiceName represents character name choice
	ChoiceName ChoiceCategory = "name"
	// ChoiceRace represents race selection
	ChoiceRace ChoiceCategory = "race"
	// ChoiceSubrace represents subrace selection
	ChoiceSubrace ChoiceCategory = "subrace"
	// ChoiceClass represents class selection
	ChoiceClass ChoiceCategory = "class"
	// ChoiceBackground represents background selection
	ChoiceBackground ChoiceCategory = "background"
	// ChoiceAbilityScores represents ability score selection
	ChoiceAbilityScores ChoiceCategory = "ability_scores"
	// ChoiceSkills represents skill proficiency selection
	ChoiceSkills ChoiceCategory = "skills"
	// ChoiceLanguages represents language selection
	ChoiceLanguages ChoiceCategory = "languages"
	// ChoiceEquipment represents equipment selection
	ChoiceEquipment ChoiceCategory = "equipment"
	// ChoiceSpells represents spell selection
	ChoiceSpells ChoiceCategory = "spells"
	// ChoiceCantrips represents cantrip selection
	ChoiceCantrips ChoiceCategory = "cantrips"
	// ChoiceFightingStyle represents fighting style selection
	ChoiceFightingStyle ChoiceCategory = "fighting_style"
	// ChoiceToolProficiency represents tool proficiency selection
	ChoiceToolProficiency ChoiceCategory = "tool_proficiency"
	// ChoiceExpertise represents expertise selection (for rogues and bards)
	ChoiceExpertise ChoiceCategory = "expertise"
	// ChoiceTraits represents racial trait selection (e.g., draconic ancestry)
	ChoiceTraits ChoiceCategory = "traits"
)

// ChoiceSource represents where a choice or grant comes from
type ChoiceSource string

const (
	// SourcePlayer represents a direct player choice
	SourcePlayer ChoiceSource = "player"
	// SourceRace represents a grant from race selection
	SourceRace ChoiceSource = "race"
	// SourceSubrace represents a grant from subrace selection
	SourceSubrace ChoiceSource = "subrace"
	// SourceClass represents a grant or choice from class
	SourceClass ChoiceSource = "class"
	// SourceSubclass represents a grant or choice from subclass
	SourceSubclass ChoiceSource = "subclass"
	// SourceBackground represents a grant from background
	SourceBackground ChoiceSource = "background"
)

// Proficiencies tracks what the character is proficient with
// Now uses typed constants from the proficiencies package for type safety
type Proficiencies struct {
	Armor   []proficiencies.Armor  `json:"armor,omitempty"`
	Weapons []proficiencies.Weapon `json:"weapons,omitempty"`
	Tools   []proficiencies.Tool   `json:"tools,omitempty"`
}

// DeathSaves tracks death saving throws
type DeathSaves struct {
	Successes int
	Failures  int
}

// ChoiceData represents any choice in character creation
type ChoiceData struct {
	ID          string   `json:"id"`
	Type        string   `json:"type"`
	Choose      int      `json:"choose"`
	From        []string `json:"from"`
	Description string   `json:"description,omitempty"`
}

// FeatureData represents a feature or trait
type FeatureData struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description"`
}
