// Copyright (C) 2024 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package conditions

import (
	"fmt"

	"github.com/KirkDiggler/rpg-toolkit/events"
)

// PermanentDuration never expires.
type PermanentDuration struct{}

func (d PermanentDuration) IsExpired(event events.Event) bool {
	return false
}

func (d PermanentDuration) Description() string {
	return "Permanent"
}

// RoundsDuration expires after a number of rounds.
type RoundsDuration struct {
	Rounds     int
	StartRound int
}

func NewRoundsDuration(rounds int) *RoundsDuration {
	return &RoundsDuration{
		Rounds: rounds,
		// StartRound will be set when we receive the first round event
	}
}

func (d *RoundsDuration) IsExpired(event events.Event) bool {
	if event.Type() != events.EventRoundEnd {
		return false
	}

	// If we haven't started tracking yet, start now
	if d.StartRound == 0 {
		if round, ok := event.Context().Get("round"); ok {
			if roundNum, ok := round.(int); ok {
				d.StartRound = roundNum
			}
		}
		return false
	}

	// Check if enough rounds have passed
	if round, ok := event.Context().Get("round"); ok {
		if currentRound, ok := round.(int); ok {
			return currentRound >= d.StartRound+d.Rounds
		}
	}

	return false
}

func (d *RoundsDuration) Description() string {
	return fmt.Sprintf("%d rounds", d.Rounds)
}

// TurnsDuration expires after a number of turns.
type TurnsDuration struct {
	Turns      int
	TurnsTaken int
	EntityID   string // Whose turns to count
}

func NewTurnsDuration(turns int, entityID string) *TurnsDuration {
	return &TurnsDuration{
		Turns:    turns,
		EntityID: entityID,
	}
}

func (d *TurnsDuration) IsExpired(event events.Event) bool {
	if event.Type() != events.EventTurnEnd {
		return false
	}

	// Check if it's the right entity's turn
	if event.Source() == nil || event.Source().GetID() != d.EntityID {
		return false
	}

	// Increment turn count
	d.TurnsTaken++

	// Check if we've had enough turns
	return d.TurnsTaken >= d.Turns
}

func (d *TurnsDuration) Description() string {
	return fmt.Sprintf("%d turns", d.Turns)
}

// UntilDamagedDuration expires when the entity takes damage.
type UntilDamagedDuration struct {
	EntityID string
}

func NewUntilDamagedDuration(entityID string) *UntilDamagedDuration {
	return &UntilDamagedDuration{EntityID: entityID}
}

func (d *UntilDamagedDuration) IsExpired(event events.Event) bool {
	if event.Type() != events.EventAfterDamage {
		return false
	}

	// Check if the damaged entity is our target
	return event.Target() != nil && event.Target().GetID() == d.EntityID
}

func (d *UntilDamagedDuration) Description() string {
	return "Until damaged"
}

// EventDuration expires when a specific event occurs.
type EventDuration struct {
	EventType string
	Condition func(events.Event) bool
}

func NewEventDuration(eventType string, condition func(events.Event) bool) *EventDuration {
	return &EventDuration{
		EventType: eventType,
		Condition: condition,
	}
}

func (d *EventDuration) IsExpired(event events.Event) bool {
	if event.Type() != d.EventType {
		return false
	}

	if d.Condition != nil {
		return d.Condition(event)
	}

	return true
}

func (d *EventDuration) Description() string {
	return fmt.Sprintf("Until %s", d.EventType)
}

