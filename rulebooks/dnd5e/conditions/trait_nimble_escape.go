// Copyright (C) 2026 Kirk Diggler
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

// NimbleEscapeData is the JSON structure for persisting the nimble escape trait.
type NimbleEscapeData struct {
	Ref     *core.Ref `json:"ref"`
	OwnerID string    `json:"owner_id"`
}

// nimbleEscapeCondition is a creature's Nimble Escape trait, as far as this
// engine honours it: the creature may take the Disengage action as a bonus
// action on each of its turns (rpg-toolkit#2032).
//
// IT STATES A FACT AND DOES NOTHING. Carrying the ref IS the fact: a seam
// deciding a turn reads it off the stored sheet with
// HoldsRef(data.Conditions, refs.MonsterTraits.NimbleEscape()), without
// loading or attaching anything. Spending the bonus action, and applying the
// Disengaging condition that actually stops opportunity attacks, are the
// encounter's and the session's; this trait subscribes to nothing.
//
// The SRD trait also lets the creature Hide as a bonus action. That half is
// deliberately not represented: there is no monster Hide verb.
type nimbleEscapeCondition struct {
	ownerID string
	bus     events.EventBus
}

// Ensure nimbleEscapeCondition implements dnd5eEvents.ConditionBehavior
var _ dnd5eEvents.ConditionBehavior = (*nimbleEscapeCondition)(nil)

// Ref returns the canonical ref this trait names itself by — the same ref its
// ToJSON embeds and its loader routes on.
func (n *nimbleEscapeCondition) Ref() *core.Ref { return refs.MonsterTraits.NimbleEscape() }

// NimbleEscape creates a new nimble escape trait for the given owner.
func NimbleEscape(ownerID string) dnd5eEvents.ConditionBehavior {
	return &nimbleEscapeCondition{ownerID: ownerID}
}

// NimbleEscapeJSON creates the JSON representation of a nimble escape trait,
// for factory functions that add trait data before a bus is available.
func NimbleEscapeJSON(ownerID string) (json.RawMessage, error) {
	return json.Marshal(NimbleEscapeData{
		Ref:     refs.MonsterTraits.NimbleEscape(),
		OwnerID: ownerID,
	})
}

// MustNimbleEscapeJSON is [NimbleEscapeJSON] for factory functions, where a
// marshal failure is a programming bug. It panics if marshaling fails.
func MustNimbleEscapeJSON(ownerID string) json.RawMessage {
	data, err := NimbleEscapeJSON(ownerID)
	if err != nil {
		panic("conditions: failed to marshal nimble escape JSON: " + err.Error())
	}
	return data
}

// IsApplied returns true if this condition is currently applied
func (n *nimbleEscapeCondition) IsApplied() bool {
	return n.bus != nil
}

// Apply marks the trait active. It installs no subscriber: the trait is a
// fact read off the sheet, not a rule that reacts to events. Applying it
// twice is refused.
func (n *nimbleEscapeCondition) Apply(_ context.Context, bus events.EventBus) error {
	if n.IsApplied() {
		return rpgerr.New(rpgerr.CodeAlreadyExists, "nimble escape condition already applied")
	}
	n.bus = bus
	return nil
}

// Remove marks the trait inactive.
func (n *nimbleEscapeCondition) Remove(_ context.Context, _ events.EventBus) error {
	n.bus = nil
	return nil
}

// ToJSON converts the condition to JSON for persistence
func (n *nimbleEscapeCondition) ToJSON() (json.RawMessage, error) {
	return NimbleEscapeJSON(n.ownerID)
}

// loadJSON loads nimble escape trait state from JSON
func (n *nimbleEscapeCondition) loadJSON(data json.RawMessage) error {
	var escapeData NimbleEscapeData
	if err := json.Unmarshal(data, &escapeData); err != nil {
		return rpgerr.Wrap(err, "failed to unmarshal nimble escape data")
	}

	n.ownerID = escapeData.OwnerID

	return nil
}

// nimbleEscapeDescriptor is the one entry for [nimbleEscapeCondition]; see [descriptors].
//
// It bears on no attack: Disengage changes what a walk provokes, not a roll.
// The detail names only the half the engine honours.
var nimbleEscapeDescriptor = descriptor{
	ref:           refs.MonsterTraits.NimbleEscape(),
	display:       Display{Name: "Nimble Escape", Detail: "Can take the Disengage action as a bonus action on each of its turns."},
	load:          loaderOf[nimbleEscapeCondition](),
	ownAction:     notBearing,
	againstHolder: notBearing,
}
