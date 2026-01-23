// Copyright (C) 2024 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package combatabilities

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/KirkDiggler/rpg-toolkit/core"
	coreCombat "github.com/KirkDiggler/rpg-toolkit/core/combat"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/refs"
)

// Attack represents the Attack combat ability.
// When activated, it consumes 1 action and sets AttacksRemaining based on
// the character's Extra Attack feature (1 normally, 2+ with Extra Attack).
// The actual Strike actions will be granted in Phase 4.
type Attack struct {
	*BaseCombatAbility
}

// AttackData is the JSON structure for persisting Attack ability state
type AttackData struct {
	Ref *core.Ref `json:"ref"`
	ID  string    `json:"id"`
}

// NewAttack creates a new Attack combat ability with the given ID.
// This is the standard Attack action available to all characters.
func NewAttack(id string) *Attack {
	return &Attack{
		BaseCombatAbility: NewBaseCombatAbility(BaseCombatAbilityConfig{
			ID:          id,
			Name:        "Attack",
			Description: "Make weapon attacks against enemies.",
			ActionType:  coreCombat.ActionStandard,
			Ref:         refs.CombatAbilities.Attack(),
		}),
	}
}

// CanActivate checks if the Attack ability can be activated.
// Requires an available standard action.
func (a *Attack) CanActivate(ctx context.Context, owner core.Entity, input CombatAbilityInput) error {
	return a.BaseCombatAbility.CanActivate(ctx, owner, input)
}

// Activate consumes 1 action and sets AttacksRemaining.
// The number of attacks is: 1 + input.ExtraAttacks
// - Normal characters: 1 attack (ExtraAttacks = 0)
// - Level 5+ Fighter: 2 attacks (ExtraAttacks = 1)
// - Level 11 Fighter: 3 attacks (ExtraAttacks = 2)
// - Level 20 Fighter: 4 attacks (ExtraAttacks = 3)
func (a *Attack) Activate(ctx context.Context, owner core.Entity, input CombatAbilityInput) error {
	// First, consume the action via base implementation
	if err := a.BaseCombatAbility.Activate(ctx, owner, input); err != nil {
		return err
	}

	// Set attacks remaining: 1 base + ExtraAttacks from features
	attackCount := 1 + input.ExtraAttacks
	input.ActionEconomy.SetAttacks(attackCount)

	// Note: Strike actions will be granted in Phase 4
	// For now, the character can use ActionEconomy.UseAttack() to consume attacks

	return nil
}

// ToJSON converts the Attack ability to JSON for persistence
func (a *Attack) ToJSON() (json.RawMessage, error) {
	data := AttackData{
		Ref: a.Ref(),
		ID:  a.GetID(),
	}

	bytes, err := json.Marshal(data)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal attack ability data: %w", err)
	}

	return bytes, nil
}

// loadJSON deserializes an Attack ability from JSON
func (a *Attack) loadJSON(data json.RawMessage) error {
	var attackData AttackData
	if err := json.Unmarshal(data, &attackData); err != nil {
		return fmt.Errorf("failed to unmarshal attack ability data: %w", err)
	}

	a.BaseCombatAbility = NewBaseCombatAbility(BaseCombatAbilityConfig{
		ID:          attackData.ID,
		Name:        "Attack",
		Description: "Make weapon attacks against enemies.",
		ActionType:  coreCombat.ActionStandard,
		Ref:         refs.CombatAbilities.Attack(),
	})

	return nil
}
