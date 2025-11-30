// Package conditions provides D&D 5e condition types and effects
package conditions

import (
	"github.com/KirkDiggler/rpg-toolkit/core"
	"github.com/KirkDiggler/rpg-toolkit/dice"
	"github.com/KirkDiggler/rpg-toolkit/events"
	dnd5eEvents "github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/events"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/fightingstyles"
)

// Type is the content type for conditions in refs (e.g., "dnd5e:conditions:raging")
const Type core.Type = "conditions"

// Condition IDs - these are the known condition identifiers
const (
	RagingID                    core.ID = "raging"
	BrutalCriticalID            core.ID = "brutal_critical"
	UnarmoredDefenseBarbarianID core.ID = "unarmored_defense_barbarian"
	UnarmoredDefenseMonkID      core.ID = "unarmored_defense_monk"
	FightingStyleID             core.ID = "fighting_style"
)

// Grant represents a condition grant from a class, race, or background.
// Grant = what you get (the condition ID to be granted)
type Grant struct {
	ID core.ID
}

// NewInput provides parameters for creating a new condition instance
type NewInput struct {
	CharacterID   string                       // Character or entity that owns this condition
	Level         int                          // Class level for scaling conditions
	Source        string                       // What granted this condition
	FightingStyle fightingstyles.FightingStyle // Only for fighting style conditions
	Roller        dice.Roller                  // Optional dice roller
}

// New creates a new condition instance based on the grant ID.
// Returns nil if the condition ID is not recognized (fail loudly pattern).
func New(grant Grant, input NewInput) dnd5eEvents.ConditionBehavior {
	switch grant.ID {
	case RagingID:
		return &RagingCondition{
			CharacterID: input.CharacterID,
			DamageBonus: calculateRageDamage(input.Level),
			Level:       input.Level,
			Source:      input.Source,
		}
	case BrutalCriticalID:
		return NewBrutalCriticalCondition(BrutalCriticalInput{
			CharacterID: input.CharacterID,
			Level:       input.Level,
			Roller:      input.Roller,
		})
	case UnarmoredDefenseBarbarianID:
		return NewUnarmoredDefenseCondition(UnarmoredDefenseInput{
			CharacterID: input.CharacterID,
			Type:        UnarmoredDefenseBarbarian,
			Source:      input.Source,
		})
	case UnarmoredDefenseMonkID:
		return NewUnarmoredDefenseCondition(UnarmoredDefenseInput{
			CharacterID: input.CharacterID,
			Type:        UnarmoredDefenseMonk,
			Source:      input.Source,
		})
	case FightingStyleID:
		return NewFightingStyleCondition(FightingStyleConditionConfig{
			CharacterID: input.CharacterID,
			Style:       input.FightingStyle,
			Roller:      input.Roller,
		})
	default:
		return nil // Unmigrated condition - fail loudly
	}
}

// calculateRageDamage determines rage damage bonus based on barbarian level
func calculateRageDamage(level int) int {
	switch {
	case level < 9:
		return 2
	case level < 16:
		return 3
	default:
		return 4
	}
}

// ApplyInput provides parameters for applying conditions
type ApplyInput struct {
	Bus events.EventBus
}

// DurationType defines how a condition's duration is tracked
type DurationType string

const (
	// DurationRounds tracks duration in combat rounds
	DurationRounds DurationType = "rounds"
	// DurationMinutes tracks duration in minutes
	DurationMinutes DurationType = "minutes"
	// DurationHours tracks duration in hours
	DurationHours DurationType = "hours"
	// DurationUntilRest lasts until a short or long rest
	DurationUntilRest DurationType = "until_rest"
	// DurationPermanent has no expiration
	DurationPermanent DurationType = "permanent"
)

// Condition represents an active condition on a character
type Condition struct {
	Type         dnd5eEvents.ConditionType `json:"type"`
	Source       string                    `json:"source,omitempty"`        // What caused this - TODO: find proper type
	SourceEntity core.Entity               `json:"source_entity,omitempty"` // Entity that applied it
	Duration     string                    `json:"duration,omitempty"`      // "1_hour", "until_rest", etc
	DurationType DurationType              `json:"duration_type,omitempty"` // How duration is tracked
	Remaining    int                       `json:"remaining,omitempty"`     // Remaining duration units
	Metadata     map[string]any            `json:"metadata,omitempty"`      // Condition-specific data
}
