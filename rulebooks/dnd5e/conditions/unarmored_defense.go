// Copyright (C) 2024 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package conditions

import (
	"context"
	"encoding/json"

	"github.com/KirkDiggler/rpg-toolkit/events"
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

// UnarmoredDefenseCondition represents the Unarmored Defense feature.
// Barbarian: AC = 10 + DEX modifier + CON modifier
// Monk: AC = 10 + DEX modifier + WIS modifier
// Only applies when not wearing armor. Shields can still be used.
type UnarmoredDefenseCondition struct {
	CharacterID string               `json:"character_id"`
	Type        UnarmoredDefenseType `json:"type"`
	Source      string               `json:"source"` // e.g., "barbarian:unarmored_defense"
	bus         events.EventBus      `json:"-"`
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
	data := map[string]interface{}{
		"ref":          "dnd5e:conditions:unarmored_defense",
		"type":         string(u.Type),
		"character_id": u.CharacterID,
		"source":       u.Source,
	}
	return json.Marshal(data)
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
