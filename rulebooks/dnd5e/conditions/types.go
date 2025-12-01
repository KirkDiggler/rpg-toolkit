// Package conditions provides D&D 5e condition types and effects
package conditions

import (
	"github.com/KirkDiggler/rpg-toolkit/core"
	dnd5eEvents "github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/events"
)

// Type is the content type for conditions within the dnd5e module
const Type core.Type = "conditions"

// Condition ID constants for type-safe references
const (
	RagingID           core.ID = "raging"
	BrutalCriticalID   core.ID = "brutal_critical"
	UnarmoredDefenseID core.ID = "unarmored_defense"
	FightingStyleID    core.ID = "fighting_style"
)

// Grant represents a condition granted to a character (e.g., from a class level)
type Grant struct {
	ID core.ID
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
	Type dnd5eEvents.ConditionType `json:"type"`
	// Source is a ref string in "module:type:value" format (e.g., "dnd5e:classes:barbarian")
	Source       string         `json:"source,omitempty"`
	SourceEntity core.Entity    `json:"source_entity,omitempty"` // Entity that applied it
	Duration     string         `json:"duration,omitempty"`      // "1_hour", "until_rest", etc
	DurationType DurationType   `json:"duration_type,omitempty"` // How duration is tracked
	Remaining    int            `json:"remaining,omitempty"`     // Remaining duration units
	Metadata     map[string]any `json:"metadata,omitempty"`      // Condition-specific data
}
