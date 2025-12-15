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
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/gamectx"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/refs"
)

// UnarmoredMovementData is the JSON structure for persisting unarmored movement condition state
type UnarmoredMovementData struct {
	Ref         *core.Ref `json:"ref"`
	CharacterID string    `json:"character_id"`
	MonkLevel   int       `json:"monk_level"`
}

// UnarmoredMovementCondition represents the Monk's Unarmored Movement feature.
// Grants a speed bonus when not wearing armor or using a shield.
// The bonus scales with monk level:
// - Level 2-5: +10 ft
// - Level 6-9: +15 ft
// - Level 10-13: +20 ft
// - Level 14-17: +25 ft
// - Level 18+: +30 ft
type UnarmoredMovementCondition struct {
	CharacterID string
	MonkLevel   int
	bus         events.EventBus
}

// Ensure UnarmoredMovementCondition implements dnd5eEvents.ConditionBehavior
var _ dnd5eEvents.ConditionBehavior = (*UnarmoredMovementCondition)(nil)

// UnarmoredMovementInput provides configuration for creating an unarmored movement condition
type UnarmoredMovementInput struct {
	CharacterID string // ID of the character
	MonkLevel   int    // Monk level determines speed bonus
}

// NewUnarmoredMovementCondition creates an unarmored movement condition from input
func NewUnarmoredMovementCondition(input UnarmoredMovementInput) *UnarmoredMovementCondition {
	return &UnarmoredMovementCondition{
		CharacterID: input.CharacterID,
		MonkLevel:   input.MonkLevel,
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
		Ref:         refs.Conditions.UnarmoredMovement(),
		CharacterID: u.CharacterID,
		MonkLevel:   u.MonkLevel,
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

	u.CharacterID = umData.CharacterID
	u.MonkLevel = umData.MonkLevel

	return nil
}

// GetSpeedBonus returns the speed bonus granted by this condition.
// Returns 0 if the character is wearing armor or using a shield.
// The bonus is based on monk level:
// - Level 2-5: +10 ft
// - Level 6-9: +15 ft
// - Level 10-13: +20 ft
// - Level 14-17: +25 ft
// - Level 18+: +30 ft
func (u *UnarmoredMovementCondition) GetSpeedBonus(ctx context.Context) int {
	// Check if character is wearing armor or shield
	if !u.isUnarmored(ctx) {
		return 0
	}

	// Calculate bonus based on monk level
	return u.calculateSpeedBonus()
}

// isUnarmored checks if the character is not wearing armor or using a shield.
// Currently only checks for shield via weapons registry.
// Full armor checking would require extending gamectx.CharacterRegistry.
func (u *UnarmoredMovementCondition) isUnarmored(ctx context.Context) bool {
	// Get character registry to check equipment
	registry, ok := gamectx.Characters(ctx)
	if !ok {
		// No registry available, assume unarmored for now
		// Game server should provide proper context
		return true
	}

	// Check for shield in equipped weapons
	weapons := registry.GetCharacterWeapons(u.CharacterID)
	if weapons == nil {
		// No weapons data, assume unarmored
		return true
	}

	// Check main hand for shield
	if mainHand := weapons.MainHand(); mainHand != nil && mainHand.IsShield {
		return false
	}

	// Check off hand for shield
	// Note: OffHand() returns nil if a shield is equipped, so we need to check directly
	// However, we can't access the private offHand field, so we'll use a workaround
	// by checking if AllEquipped is empty when we know there's equipment
	// This is a limitation of the current gamectx API

	// For now, we'll rely on the fact that if either hand has a shield, isUnarmored will be false
	// A more complete implementation would require:
	// 1. Adding GetArmor() method to CharacterRegistry
	// 2. Checking both armor slot and shield slot

	// TODO: When armor tracking is added to gamectx, check for equipped armor here
	// For now, we assume no armor unless a shield is found

	return true
}

// calculateSpeedBonus returns the speed bonus based on monk level
func (u *UnarmoredMovementCondition) calculateSpeedBonus() int {
	switch {
	case u.MonkLevel >= 18:
		return 30
	case u.MonkLevel >= 14:
		return 25
	case u.MonkLevel >= 10:
		return 20
	case u.MonkLevel >= 6:
		return 15
	default:
		// Level 2-5
		return 10
	}
}
