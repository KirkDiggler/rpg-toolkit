// Package conditions provides D&D 5e condition types and effects
package conditions

// ConditionType represents D&D 5e conditions
type ConditionType string

const (
	// Blinded condition - creature can't see and automatically fails sight checks
	Blinded ConditionType = "blinded"
	// Charmed condition - creature can't attack charmer, charmer has advantage on social checks
	Charmed ConditionType = "charmed"
	// Deafened condition - creature can't hear and automatically fails hearing checks
	Deafened ConditionType = "deafened"
	// Frightened condition - creature can't willingly move closer to source of fear
	Frightened ConditionType = "frightened"
	// Grappled condition - creature's speed becomes 0
	Grappled ConditionType = "grappled"
	// Incapacitated condition - creature can't take actions or reactions
	Incapacitated ConditionType = "incapacitated"
	// Invisible condition - creature can't be seen without special sense
	Invisible ConditionType = "invisible"
	// Paralyzed condition - creature is incapacitated and can't move or speak
	Paralyzed ConditionType = "paralyzed"
	// Petrified condition - creature is transformed into stone
	Petrified ConditionType = "petrified"
	// Poisoned condition - disadvantage on attack rolls and ability checks
	Poisoned ConditionType = "poisoned"
	// Prone condition - creature is lying on the ground
	Prone ConditionType = "prone"
	// Restrained condition - creature's speed becomes 0, disadvantage on Dex saves
	Restrained ConditionType = "restrained"
	// Stunned condition - creature is incapacitated, can't move, and fails Str/Dex saves
	Stunned ConditionType = "stunned"
	// Unconscious condition - creature is incapacitated, can't move or speak, unaware
	Unconscious ConditionType = "unconscious"

	// Exhaustion1 represents exhaustion level 1 - disadvantage on ability checks
	Exhaustion1 ConditionType = "exhaustion_1"
	// Exhaustion2 represents exhaustion level 2 - speed halved
	Exhaustion2 ConditionType = "exhaustion_2"
	// Exhaustion3 represents exhaustion level 3 - disadvantage on attacks and saves
	Exhaustion3 ConditionType = "exhaustion_3"
	// Exhaustion4 represents exhaustion level 4 - hit point maximum halved
	Exhaustion4 ConditionType = "exhaustion_4"
	// Exhaustion5 represents exhaustion level 5 - speed reduced to 0
	Exhaustion5 ConditionType = "exhaustion_5"
	// Exhaustion6 represents exhaustion level 6 - death
	Exhaustion6 ConditionType = "exhaustion_6"

	// Raging represents the barbarian rage state - damage bonus and resistance
	Raging ConditionType = "raging"
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
	Type         ConditionType          `json:"type"`
	Source       string                 `json:"source,omitempty"`         // What caused this
	SourceEntity string                 `json:"source_entity,omitempty"`  // Entity ID that applied it
	Duration     string                 `json:"duration,omitempty"`       // "1_hour", "until_rest", etc
	DurationType DurationType           `json:"duration_type,omitempty"`  // How duration is tracked
	Remaining    int                    `json:"remaining,omitempty"`      // Remaining duration units
	Metadata     map[string]interface{} `json:"metadata,omitempty"`       // Condition-specific data
}
