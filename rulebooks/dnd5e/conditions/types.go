// Package conditions provides D&D 5e condition types and effects
package conditions

import (
	"github.com/KirkDiggler/rpg-toolkit/core"
	dnd5eEvents "github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/events"
)

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
	Source       string                    `json:"source,omitempty"`        // Ref string in "module:type:value" format (e.g., "dnd5e:classes:barbarian")
	SourceEntity core.Entity               `json:"source_entity,omitempty"` // Entity that applied it
	Duration     string                    `json:"duration,omitempty"`      // "1_hour", "until_rest", etc
	DurationType DurationType              `json:"duration_type,omitempty"` // How duration is tracked
	Remaining    int                       `json:"remaining,omitempty"`     // Remaining duration units
	Metadata     map[string]any            `json:"metadata,omitempty"`      // Condition-specific data
}
