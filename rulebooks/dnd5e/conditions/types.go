// Package conditions provides D&D 5e condition types and effects
package conditions

import (
	"context"
	"encoding/json"

	"github.com/KirkDiggler/rpg-toolkit/core"
	"github.com/KirkDiggler/rpg-toolkit/events"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e"
)

// ConditionBehavior represents the behavior of an active condition.
// Conditions subscribe to events to modify game mechanics.
type ConditionBehavior interface {
	// Apply subscribes this condition to relevant events on the bus
	Apply(ctx context.Context, bus events.EventBus) error

	// Remove unsubscribes this condition from events
	Remove(ctx context.Context, bus events.EventBus) error

	// ToJSON converts the condition to JSON for persistence
	ToJSON() (json.RawMessage, error)
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
	Type         dnd5e.ConditionType `json:"type"`
	Source       string              `json:"source,omitempty"`        // What caused this - TODO: find proper type
	SourceEntity core.Entity         `json:"source_entity,omitempty"` // Entity that applied it
	Duration     string              `json:"duration,omitempty"`      // "1_hour", "until_rest", etc
	DurationType DurationType        `json:"duration_type,omitempty"` // How duration is tracked
	Remaining    int                 `json:"remaining,omitempty"`     // Remaining duration units
	Metadata     map[string]any      `json:"metadata,omitempty"`      // Condition-specific data
}
