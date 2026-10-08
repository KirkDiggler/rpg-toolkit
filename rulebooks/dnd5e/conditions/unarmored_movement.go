// Copyright (C) 2024 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package conditions

import (
	"context"
	"encoding/json"

	"github.com/KirkDiggler/rpg-toolkit/core"
	"github.com/KirkDiggler/rpg-toolkit/events"
	"github.com/KirkDiggler/rpg-toolkit/rpgerr"
	dnd5eEvents "github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/events"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/refs"
)

// UnarmoredMovementData is the JSON structure for persisting unarmored
// movement condition state. No monk level is stored; a blob saved with the old
// "monk_level" key loads and the copy is ignored.
type UnarmoredMovementData struct {
	Ref      *core.Ref `json:"ref"`
	MemberID string    `json:"member_id"`
}

// UnarmoredMovementCondition marks that a monk holds the Unarmored Movement
// feature: a speed bonus while wearing no armour and wielding no shield, which
// scales with monk level (+10 ft at 2nd, rising to +30 ft at 18th).
//
// It contributes no speed today. How a speed modifier joins the member's speed
// answer is deferred until a speed modifier ships (design R8, owner unset), and
// the level it scales with is the sheet's, asked at that moment — never a copy
// stored here.
type UnarmoredMovementCondition struct {
	MemberID string
	bus      events.EventBus
}

// Ensure UnarmoredMovementCondition implements dnd5eEvents.ConditionBehavior
var _ dnd5eEvents.ConditionBehavior = (*UnarmoredMovementCondition)(nil)

// Ref returns the canonical ref this condition names itself by — the same ref
// its ToJSON embeds and its loader routes on.
func (u *UnarmoredMovementCondition) Ref() *core.Ref { return refs.Conditions.UnarmoredMovement() }

// UnarmoredMovementInput provides configuration for creating an unarmored
// movement condition. It takes no level.
type UnarmoredMovementInput struct {
	MemberID string // ID of the character
}

// NewUnarmoredMovementCondition creates an unarmored movement condition from input
func NewUnarmoredMovementCondition(input UnarmoredMovementInput) *UnarmoredMovementCondition {
	return &UnarmoredMovementCondition{
		MemberID: input.MemberID,
	}
}

// IsApplied returns true if this condition is currently applied
func (u *UnarmoredMovementCondition) IsApplied() bool {
	return u.bus != nil
}

// Apply registers this condition with the event bus.
// Unarmored Movement is a passive feature that doesn't subscribe to events,
// but we store the bus reference for consistency with the interface.
func (u *UnarmoredMovementCondition) Apply(_ context.Context, bus events.EventBus) error {
	u.bus = bus
	return nil
}

// Remove unregisters this condition from the event bus.
func (u *UnarmoredMovementCondition) Remove(_ context.Context, _ events.EventBus) error {
	u.bus = nil
	return nil
}

// ToJSON converts the condition to JSON for persistence
func (u *UnarmoredMovementCondition) ToJSON() (json.RawMessage, error) {
	data := UnarmoredMovementData{
		Ref:      refs.Conditions.UnarmoredMovement(),
		MemberID: u.MemberID,
	}
	return json.Marshal(data)
}

// loadJSON loads unarmored movement condition state from JSON
//
//nolint:unused // Used by loader.go
func (u *UnarmoredMovementCondition) loadJSON(data json.RawMessage) error {
	var umData UnarmoredMovementData
	if err := json.Unmarshal(data, &umData); err != nil {
		return rpgerr.Wrap(err, "failed to unmarshal unarmored movement data")
	}

	u.MemberID = umData.MemberID

	return nil
}
