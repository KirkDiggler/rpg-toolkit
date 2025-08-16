// Copyright (C) 2024 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package dnd5e

import (
	"github.com/KirkDiggler/rpg-toolkit/core"
	"github.com/KirkDiggler/rpg-toolkit/core/damage"
	"github.com/KirkDiggler/rpg-toolkit/events"
)

// Event type refs for D&D 5e combat
var (
	EventRefAttack         = core.MustNewRef(core.RefInput{Module: "dnd5e", Type: "events", Value: "attack"})
	EventRefDamageReceived = core.MustNewRef(core.RefInput{Module: "dnd5e", Type: "events", Value: "damage_received"})
	EventRefTurnEnd        = core.MustNewRef(core.RefInput{Module: "dnd5e", Type: "events", Value: "turn_end"})
	EventRefRoundEnd       = core.MustNewRef(core.RefInput{Module: "dnd5e", Type: "events", Value: "round_end"})
	EventRefRageStarted    = core.MustNewRef(core.RefInput{Module: "dnd5e", Type: "events", Value: "rage_started"})
	EventRefRageEnded      = core.MustNewRef(core.RefInput{Module: "dnd5e", Type: "events", Value: "rage_ended"})
)

// AbilityType identifies which ability is used
type AbilityType string

// Ability type constants for D&D 5e
const (
	AbilityStrength     AbilityType = "strength"     // Strength ability score
	AbilityDexterity    AbilityType = "dexterity"    // Dexterity ability score
	AbilityConstitution AbilityType = "constitution" // Constitution ability score
	AbilityIntelligence AbilityType = "intelligence" // Intelligence ability score
	AbilityWisdom       AbilityType = "wisdom"       // Wisdom ability score
	AbilityCharisma     AbilityType = "charisma"     // Charisma ability score
)

// AttackEvent is published when an entity makes an attack
type AttackEvent struct {
	ctx      *events.EventContext
	Attacker core.Entity
	Target   core.Entity
	IsMelee  bool
	Ability  AbilityType
	Damage   int // Base damage before modifiers
}

// EventRef returns the event reference for attack events
func (e *AttackEvent) EventRef() *core.Ref { return EventRefAttack }

// Context returns the event context for modifiers
func (e *AttackEvent) Context() *events.EventContext { return e.ctx }

// DamageReceivedEvent is published when an entity takes damage
type DamageReceivedEvent struct {
	ctx        *events.EventContext
	Target     core.Entity
	Source     core.Entity
	Amount     int
	DamageType damage.Type
}

// EventRef returns the event reference for damage received events
func (e *DamageReceivedEvent) EventRef() *core.Ref { return EventRefDamageReceived }

// Context returns the event context for modifiers
func (e *DamageReceivedEvent) Context() *events.EventContext { return e.ctx }

// TurnEndEvent is published when an entity's turn ends
type TurnEndEvent struct {
	ctx    *events.EventContext
	Entity core.Entity
}

// EventRef returns the event reference for turn end events
func (e *TurnEndEvent) EventRef() *core.Ref { return EventRefTurnEnd }

// Context returns the event context for modifiers
func (e *TurnEndEvent) Context() *events.EventContext { return e.ctx }

// RoundEndEvent is published when a combat round ends
type RoundEndEvent struct {
	ctx   *events.EventContext
	Round int
}

// EventRef returns the event reference for round end events
func (e *RoundEndEvent) EventRef() *core.Ref { return EventRefRoundEnd }

// Context returns the event context for modifiers
func (e *RoundEndEvent) Context() *events.EventContext { return e.ctx }

// RageStartedEvent is published when rage begins
type RageStartedEvent struct {
	ctx         *events.EventContext
	Owner       core.Entity
	DamageBonus int
}

// EventRef returns the event reference for rage started events
func (e *RageStartedEvent) EventRef() *core.Ref { return EventRefRageStarted }

// Context returns the event context for modifiers
func (e *RageStartedEvent) Context() *events.EventContext { return e.ctx }

// RageEndedEvent is published when rage ends
type RageEndedEvent struct {
	ctx   *events.EventContext
	Owner core.Entity
}

// EventRef returns the event reference for rage ended events
func (e *RageEndedEvent) EventRef() *core.Ref { return EventRefRageEnded }

// Context returns the event context for modifiers
func (e *RageEndedEvent) Context() *events.EventContext { return e.ctx }

// NewAttackEvent creates a new attack event with the given parameters
func NewAttackEvent(attacker, target core.Entity, isMelee bool, ability AbilityType, damage int) *AttackEvent {
	return &AttackEvent{
		ctx:      events.NewEventContext(),
		Attacker: attacker,
		Target:   target,
		IsMelee:  isMelee,
		Ability:  ability,
		Damage:   damage,
	}
}

// NewDamageReceivedEvent creates a new damage received event with the given parameters
func NewDamageReceivedEvent(target, source core.Entity, amount int, damageType damage.Type) *DamageReceivedEvent {
	return &DamageReceivedEvent{
		ctx:        events.NewEventContext(),
		Target:     target,
		Source:     source,
		Amount:     amount,
		DamageType: damageType,
	}
}
