// Copyright (C) 2024 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package dnd5e

import (
	"github.com/KirkDiggler/rpg-toolkit/events"
)

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

// Topic definitions for typed event system
var (
	// TurnStartTopic provides typed pub/sub for turn start events
	TurnStartTopic = events.DefineTypedTopic[TurnStartEvent]("dnd5e.turn.start")

	// TurnEndTopic provides typed pub/sub for turn end events
	TurnEndTopic = events.DefineTypedTopic[TurnEndEvent]("dnd5e.turn.end")

	// DamageReceivedTopic provides typed pub/sub for damage received events
	DamageReceivedTopic = events.DefineTypedTopic[DamageReceivedEvent]("dnd5e.combat.damage.received")
)
