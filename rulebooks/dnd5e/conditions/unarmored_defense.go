// Copyright (C) 2024 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package conditions

import (
	"context"
	"encoding/json"

	"github.com/KirkDiggler/rpg-toolkit/core"
	"github.com/KirkDiggler/rpg-toolkit/events"
	"github.com/KirkDiggler/rpg-toolkit/rpgerr"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/abilities"
	dnd5eEvents "github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/events"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/shared"
)

// UnarmoredDefenseType distinguishes between class variants of Unarmored Defense
type UnarmoredDefenseType string

const (
	// UnarmoredDefenseBarbarian uses CON modifier (10 + DEX + CON)
	UnarmoredDefenseBarbarian UnarmoredDefenseType = "barbarian"
	// UnarmoredDefenseMonk uses WIS modifier (10 + DEX + WIS)
	UnarmoredDefenseMonk UnarmoredDefenseType = "monk"
)

// UnarmoredDefenseData is the JSON structure for persisting unarmored defense condition state
type UnarmoredDefenseData struct {
	Ref         core.Ref `json:"ref"`
	Type        string   `json:"type"` // "barbarian" or "monk"
	CharacterID string   `json:"character_id"`
	Source      string   `json:"source"`
}

// UnarmoredDefenseCondition represents the Unarmored Defense feature.
// Barbarian: AC = 10 + DEX modifier + CON modifier
// Monk: AC = 10 + DEX modifier + WIS modifier
// Only applies when not wearing armor. Shields can still be used.
type UnarmoredDefenseCondition struct {
	CharacterID string
	Type        UnarmoredDefenseType
	Source      string // e.g., "barbarian:unarmored_defense"
	bus         events.EventBus
}

// Ensure UnarmoredDefenseCondition implements dnd5eEvents.ConditionBehavior
var _ dnd5eEvents.ConditionBehavior = (*UnarmoredDefenseCondition)(nil)

// UnarmoredDefenseInput provides configuration for creating an unarmored defense condition
type UnarmoredDefenseInput struct {
	CharacterID string               // ID of the character
	Type        UnarmoredDefenseType // Barbarian (CON) or Monk (WIS)
	Source      string               // What granted this feature
}

// NewUnarmoredDefenseCondition creates an unarmored defense condition from input
func NewUnarmoredDefenseCondition(input UnarmoredDefenseInput) *UnarmoredDefenseCondition {
	return &UnarmoredDefenseCondition{
		CharacterID: input.CharacterID,
		Type:        input.Type,
		Source:      input.Source,
	}
}

// IsApplied returns true if this condition is currently applied
func (u *UnarmoredDefenseCondition) IsApplied() bool {
	return u.bus != nil
}

// Apply registers this condition with the event bus.
// Unarmored Defense is a passive feature that doesn't subscribe to events,
// but we store the bus reference for consistency with the interface.
func (u *UnarmoredDefenseCondition) Apply(_ context.Context, bus events.EventBus) error {
	u.bus = bus
	return nil
}

// Remove unregisters this condition from the event bus.
func (u *UnarmoredDefenseCondition) Remove(_ context.Context, _ events.EventBus) error {
	u.bus = nil
	return nil
}

// ToJSON converts the condition to JSON for persistence
func (u *UnarmoredDefenseCondition) ToJSON() (json.RawMessage, error) {
	data := UnarmoredDefenseData{
		Ref: core.Ref{
			Module: "dnd5e",
			Type:   "conditions",
			ID:     "unarmored_defense",
		},
		Type:        string(u.Type),
		CharacterID: u.CharacterID,
		Source:      u.Source,
	}
	return json.Marshal(data)
}

// loadJSON loads unarmored defense condition state from JSON
func (u *UnarmoredDefenseCondition) loadJSON(data json.RawMessage) error {
	var udData UnarmoredDefenseData
	if err := json.Unmarshal(data, &udData); err != nil {
		return rpgerr.Wrap(err, "failed to unmarshal unarmored defense data")
	}

	u.CharacterID = udData.CharacterID
	u.Type = UnarmoredDefenseType(udData.Type)
	u.Source = udData.Source

	return nil
}

// CalculateAC computes the AC for this unarmored defense type given ability scores.
// Returns the unarmored AC (10 + DEX + secondary ability).
func (u *UnarmoredDefenseCondition) CalculateAC(scores shared.AbilityScores) int {
	baseAC := 10
	dexMod := scores.Modifier(abilities.DEX)

	var secondaryMod int
	switch u.Type {
	case UnarmoredDefenseBarbarian:
		secondaryMod = scores.Modifier(abilities.CON)
	case UnarmoredDefenseMonk:
		secondaryMod = scores.Modifier(abilities.WIS)
	}

	return baseAC + dexMod + secondaryMod
}

// SecondaryAbility returns the secondary ability used for this type of unarmored defense
func (u *UnarmoredDefenseCondition) SecondaryAbility() abilities.Ability {
	switch u.Type {
	case UnarmoredDefenseBarbarian:
		return abilities.CON
	case UnarmoredDefenseMonk:
		return abilities.WIS
	default:
		return abilities.CON
	}
}
