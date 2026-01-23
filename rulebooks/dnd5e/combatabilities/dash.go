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

// Dash represents the Dash combat ability.
// When activated, it consumes 1 action and adds the character's speed to
// their movement remaining. This effectively doubles their movement for the turn.
type Dash struct {
	*BaseCombatAbility
}

// DashData is the JSON structure for persisting Dash ability state
type DashData struct {
	Ref *core.Ref `json:"ref"`
	ID  string    `json:"id"`
}

// NewDash creates a new Dash combat ability that uses a standard action.
// This is the default Dash action available to all characters.
func NewDash(id string) *Dash {
	return &Dash{
		BaseCombatAbility: NewBaseCombatAbility(BaseCombatAbilityConfig{
			ID:          id,
			Name:        "Dash",
			Description: "Double your movement speed for this turn.",
			ActionType:  coreCombat.ActionStandard,
			Ref:         refs.CombatAbilities.Dash(),
		}),
	}
}

// NewBonusDash creates a Dash ability that uses a bonus action.
// This is used by features like Rogue's Cunning Action or Monk's Step of the Wind.
func NewBonusDash(id string) *Dash {
	return &Dash{
		BaseCombatAbility: NewBaseCombatAbility(BaseCombatAbilityConfig{
			ID:          id,
			Name:        "Dash",
			Description: "Double your movement speed for this turn.",
			ActionType:  coreCombat.ActionBonus,
			Ref:         refs.CombatAbilities.Dash(),
		}),
	}
}

// CanActivate checks if the Dash ability can be activated.
// Requires an available action (standard or bonus depending on construction).
func (d *Dash) CanActivate(ctx context.Context, owner core.Entity, input CombatAbilityInput) error {
	return d.BaseCombatAbility.CanActivate(ctx, owner, input)
}

// Activate consumes 1 action and adds the character's speed to movement remaining.
// If the character has 30ft speed and has already moved 10ft (20ft remaining),
// after Dash they would have 50ft remaining (20 + 30).
func (d *Dash) Activate(ctx context.Context, owner core.Entity, input CombatAbilityInput) error {
	// First, consume the action via base implementation
	if err := d.BaseCombatAbility.Activate(ctx, owner, input); err != nil {
		return err
	}

	// Add the character's speed to movement remaining
	input.ActionEconomy.AddMovement(input.Speed)

	return nil
}

// ToJSON converts the Dash ability to JSON for persistence
func (d *Dash) ToJSON() (json.RawMessage, error) {
	data := DashData{
		Ref: d.Ref(),
		ID:  d.GetID(),
	}

	bytes, err := json.Marshal(data)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal dash ability data: %w", err)
	}

	return bytes, nil
}

// loadJSON deserializes a Dash ability from JSON
func (d *Dash) loadJSON(data json.RawMessage) error {
	var dashData DashData
	if err := json.Unmarshal(data, &dashData); err != nil {
		return fmt.Errorf("failed to unmarshal dash ability data: %w", err)
	}

	d.BaseCombatAbility = NewBaseCombatAbility(BaseCombatAbilityConfig{
		ID:          dashData.ID,
		Name:        "Dash",
		Description: "Double your movement speed for this turn.",
		ActionType:  coreCombat.ActionStandard,
		Ref:         refs.CombatAbilities.Dash(),
	})

	return nil
}
