// Copyright (C) 2024 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package events

import (
	"context"
	"encoding/json"

	"github.com/KirkDiggler/rpg-toolkit/core"
	"github.com/KirkDiggler/rpg-toolkit/events"
)

// ConditionType represents D&D 5e conditions
type ConditionType string

const (
	// ConditionBlinded is a condition that blinds a character
	ConditionBlinded ConditionType = "blinded"
	// ConditionCharmed is a condition that charms a character
	ConditionCharmed ConditionType = "charmed"
	// ConditionDeafened is a condition that deafens a character
	ConditionDeafened ConditionType = "deafened"
	// ConditionFrightened is a condition that frightens a character
	ConditionFrightened ConditionType = "frightened"
	// ConditionGrappled is a condition that grapples a character
	ConditionGrappled ConditionType = "grappled"
	// ConditionIncapacitated is a condition that incapacitates a character
	ConditionIncapacitated ConditionType = "incapacitated"
	// ConditionInvisible is a condition that makes a character invisible
	ConditionInvisible ConditionType = "invisible"
	// ConditionParalyzed is a condition that paralyzes a character
	ConditionParalyzed ConditionType = "paralyzed"
	// ConditionPetrified is a condition that petrifies a character
	ConditionPetrified ConditionType = "petrified"
	// ConditionPoisoned is a condition that poisons a character
	ConditionPoisoned ConditionType = "poisoned"
	// ConditionProne is a condition that makes a character prone
	ConditionProne ConditionType = "prone"
	// ConditionRestrained is a condition that restrains a character
	ConditionRestrained ConditionType = "restrained"
	// ConditionStunned is a condition that stuns a character
	ConditionStunned ConditionType = "stunned"
	// ConditionUnconscious is a condition that makes a character unconscious
	ConditionUnconscious ConditionType = "unconscious"

	// ConditionExhaustion1 is a condition that exhausts a character
	ConditionExhaustion1 ConditionType = "exhaustion_1"
	// ConditionExhaustion2 is a condition that exhausts a character
	ConditionExhaustion2 ConditionType = "exhaustion_2"
	// ConditionExhaustion3 is a condition that exhausts a character
	ConditionExhaustion3 ConditionType = "exhaustion_3"
	// ConditionExhaustion4 is a condition that exhausts a character
	ConditionExhaustion4 ConditionType = "exhaustion_4"
	// ConditionExhaustion5 is a condition that exhausts a character
	ConditionExhaustion5 ConditionType = "exhaustion_5"
	// ConditionExhaustion6 is a condition that exhausts a character
	ConditionExhaustion6 ConditionType = "exhaustion_6"

	// ConditionRaging is a class-specific condition for barbarians
	ConditionRaging ConditionType = "raging"
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

// Event types for D&D 5e gameplay

// TurnStartEvent is published when a character's turn begins
type TurnStartEvent struct {
	CharacterID string // ID of the character whose turn is starting
	Round       int    // Current round number
}

// TurnEndEvent is published when a character's turn ends
type TurnEndEvent struct {
	CharacterID string // ID of the character whose turn is ending
	Round       int    // Current round number
}

// DamageReceivedEvent is published when a character takes damage
type DamageReceivedEvent struct {
	TargetID   string // ID of the character taking damage
	SourceID   string // ID of the damage source
	Amount     int    // Amount of damage
	DamageType string // Type of damage (slashing, fire, etc)
}

// ConditionAppliedEvent is published when a condition is applied to an entity
type ConditionAppliedEvent struct {
	Target    core.Entity       // Entity receiving the condition
	Type      ConditionType     // Type of condition being applied
	Source    string            // What caused this condition
	Condition ConditionBehavior // The condition behavior to apply
}

// ConditionRemovedEvent is published when a condition ends
type ConditionRemovedEvent struct {
	CharacterID  string
	ConditionRef string
	Reason       string
}

// AttackEvent is published when a character makes an attack
type AttackEvent struct {
	AttackerID string // ID of the attacking character
	TargetID   string // ID of the target
	WeaponRef  string // Reference to the weapon used
	IsMelee    bool   // True for melee attacks, false for ranged
}

// Topic definitions for typed event system
var (
	// TurnStartTopic provides typed pub/sub for turn start events
	TurnStartTopic = events.DefineTypedTopic[TurnStartEvent]("dnd5e.turn.start")

	// TurnEndTopic provides typed pub/sub for turn end events
	TurnEndTopic = events.DefineTypedTopic[TurnEndEvent]("dnd5e.turn.end")

	// DamageReceivedTopic provides typed pub/sub for damage received events
	DamageReceivedTopic = events.DefineTypedTopic[DamageReceivedEvent]("dnd5e.combat.damage.received")

	// ConditionAppliedTopic provides typed pub/sub for condition applied events
	ConditionAppliedTopic = events.DefineTypedTopic[ConditionAppliedEvent]("dnd5e.condition.applied")

	// ConditionRemovedTopic provides typed pub/sub for condition removed events
	ConditionRemovedTopic = events.DefineTypedTopic[ConditionRemovedEvent]("dnd5e.condition.removed")

	// AttackTopic provides typed pub/sub for attack events
	AttackTopic = events.DefineTypedTopic[AttackEvent]("dnd5e.combat.attack")
)
