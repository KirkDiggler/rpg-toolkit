// Package shared provides common types used across the D&D 5e rulebook
package shared

import (
	"fmt"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/constants"
)

// AbilityScores maps ability constants to their scores (includes all bonuses)
type AbilityScores map[constants.Ability]int

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
		constants.STR: config.STR,
		constants.DEX: config.DEX,
		constants.CON: config.CON,
		constants.INT: config.INT,
		constants.WIS: config.WIS,
		constants.CHA: config.CHA,
	}, nil
}

// Increase increments a specific ability, enforcing the maximum of 20
func (a AbilityScores) Increase(ability constants.Ability, amount int) error {
	newValue := a[ability] + amount
	if newValue > 20 {
		return fmt.Errorf("ability %s cannot exceed 20 (current: %d, increase: %d)", ability.Display(), a[ability], amount)
	}
	a[ability] = newValue
	return nil
}

// ApplyIncreases applies multiple increases at once (e.g., racial bonuses)
func (a AbilityScores) ApplyIncreases(increases map[constants.Ability]int) error {
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
func (a AbilityScores) Modifier(ability constants.Ability) int {
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
	// ShortRest resets on a short rest
	ShortRest ResetType = "short_rest"
	// LongRest resets on a long rest
	LongRest ResetType = "long_rest"
	// Dawn resets at dawn
	Dawn ResetType = "dawn"
	// None never recharges (consumable)
	None ResetType = "none"
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

// String returns the string representation of ClassResourceType
func (c ClassResourceType) String() string {
	switch c {
	case ClassResourceRage:
		return "rage"
	case ClassResourceBardicInspiration:
		return "bardic_inspiration"
	case ClassResourceChannelDivinity:
		return "channel_divinity"
	case ClassResourceWildShape:
		return "wild_shape"
	case ClassResourceSecondWind:
		return "second_wind"
	case ClassResourceActionSurge:
		return "action_surge"
	case ClassResourceKiPoints:
		return "ki_points"
	case ClassResourceDivineSense:
		return "divine_sense"
	case ClassResourceLayOnHands:
		return "lay_on_hands"
	case ClassResourceSorceryPoints:
		return "sorcery_points"
	case ClassResourceArcaneRecovery:
		return "arcane_recovery"
	case ClassResourceIndomitable:
		return "indomitable"
	case ClassResourceSuperiorityDice:
		return "superiority_dice"
	default:
		return "unspecified"
	}
}

// ParseClassResourceType parses a string into a ClassResourceType
func ParseClassResourceType(s string) ClassResourceType {
	switch s {
	case "rage":
		return ClassResourceRage
	case "bardic_inspiration":
		return ClassResourceBardicInspiration
	case "channel_divinity":
		return ClassResourceChannelDivinity
	case "wild_shape":
		return ClassResourceWildShape
	case "second_wind":
		return ClassResourceSecondWind
	case "action_surge":
		return ClassResourceActionSurge
	case "ki_points", "ki":
		return ClassResourceKiPoints
	case "divine_sense":
		return ClassResourceDivineSense
	case "lay_on_hands":
		return ClassResourceLayOnHands
	case "sorcery_points":
		return ClassResourceSorceryPoints
	case "arcane_recovery":
		return ClassResourceArcaneRecovery
	case "indomitable":
		return ClassResourceIndomitable
	case "superiority_dice":
		return ClassResourceSuperiorityDice
	default:
		return ClassResourceUnspecified
	}
}

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
type Proficiencies struct {
	Armor   []string `json:"armor,omitempty"`
	Weapons []string `json:"weapons,omitempty"`
	Tools   []string `json:"tools,omitempty"`
}

// DeathSaves tracks death saving throws
type DeathSaves struct {
	Successes int
	Failures  int
}

// Background represents character background
type Background struct {
	ID          constants.Background `json:"id"`
	Name        string               `json:"name"`
	Description string               `json:"description"`

	// Skill proficiencies (usually 2)
	SkillProficiencies []constants.Skill `json:"skill_proficiencies"`

	// Languages
	Languages      []constants.Language `json:"languages,omitempty"`
	LanguageChoice *ChoiceData          `json:"language_choice,omitempty"`

	// Tool proficiencies
	ToolProficiencies []string    `json:"tool_proficiencies,omitempty"`
	ToolChoice        *ChoiceData `json:"tool_choice,omitempty"`

	// Starting equipment
	Equipment []string `json:"equipment"`

	// Feature
	Feature FeatureData `json:"feature"`
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
