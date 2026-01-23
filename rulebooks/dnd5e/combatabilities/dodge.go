// Copyright (C) 2024 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package combatabilities

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/KirkDiggler/rpg-toolkit/core"
	coreCombat "github.com/KirkDiggler/rpg-toolkit/core/combat"
	"github.com/KirkDiggler/rpg-toolkit/rpgerr"
	dnd5eEvents "github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/events"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/refs"
)

// Dodge represents the Dodge combat ability.
// When activated, it consumes 1 action and publishes a DodgeActivatedEvent.
// The Dodging condition (to be implemented) will:
// - Give attackers disadvantage on attack rolls against the dodging character
// - Grant advantage on DEX saving throws
// The effect lasts until the start of the character's next turn.
type Dodge struct {
	*BaseCombatAbility
}

// DodgeData is the JSON structure for persisting Dodge ability state
type DodgeData struct {
	Ref *core.Ref `json:"ref"`
	ID  string    `json:"id"`
}

// NewDodge creates a new Dodge combat ability that uses a standard action.
// This is the default Dodge action available to all characters.
func NewDodge(id string) *Dodge {
	return &Dodge{
		BaseCombatAbility: NewBaseCombatAbility(BaseCombatAbilityConfig{
			ID:          id,
			Name:        "Dodge",
			Description: "Attackers have disadvantage against you until your next turn.",
			ActionType:  coreCombat.ActionStandard,
			Ref:         refs.CombatAbilities.Dodge(),
		}),
	}
}

// NewBonusDodge creates a Dodge ability that uses a bonus action.
// This is used by features like Monk's Patient Defense.
func NewBonusDodge(id string) *Dodge {
	return &Dodge{
		BaseCombatAbility: NewBaseCombatAbility(BaseCombatAbilityConfig{
			ID:          id,
			Name:        "Dodge",
			Description: "Attackers have disadvantage against you until your next turn.",
			ActionType:  coreCombat.ActionBonus,
			Ref:         refs.CombatAbilities.Dodge(),
		}),
	}
}

// CanActivate checks if the Dodge ability can be activated.
// Requires an available action and an event bus.
func (d *Dodge) CanActivate(ctx context.Context, owner core.Entity, input CombatAbilityInput) error {
	// Check base requirements (action economy)
	if err := d.BaseCombatAbility.CanActivate(ctx, owner, input); err != nil {
		return err
	}

	// Dodge requires event bus to publish the activation event
	if input.Bus == nil {
		return rpgerr.New(rpgerr.CodeInvalidArgument, "event bus required for Dodge")
	}

	return nil
}

// Activate consumes 1 action and publishes a DodgeActivatedEvent.
// Subscribers to the event can apply the Dodging condition.
func (d *Dodge) Activate(ctx context.Context, owner core.Entity, input CombatAbilityInput) error {
	// Validate event bus before consuming action
	if input.Bus == nil {
		return rpgerr.New(rpgerr.CodeInvalidArgument, "event bus required for Dodge")
	}

	// Consume the action via base implementation
	if err := d.BaseCombatAbility.Activate(ctx, owner, input); err != nil {
		return err
	}

	// Publish the dodge activated event
	// A DodgingCondition (to be implemented in a future phase) will subscribe
	// to this event and apply the actual mechanical effects
	if err := dnd5eEvents.DodgeActivatedTopic.On(input.Bus).Publish(ctx, dnd5eEvents.DodgeActivatedEvent{
		CharacterID: owner.GetID(),
	}); err != nil {
		return fmt.Errorf("failed to publish dodge activated event: %w", err)
	}

	return nil
}

// ToJSON converts the Dodge ability to JSON for persistence
func (d *Dodge) ToJSON() (json.RawMessage, error) {
	data := DodgeData{
		Ref: d.Ref(),
		ID:  d.GetID(),
	}

	bytes, err := json.Marshal(data)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal dodge ability data: %w", err)
	}

	return bytes, nil
}

// loadJSON deserializes a Dodge ability from JSON
func (d *Dodge) loadJSON(data json.RawMessage) error {
	var dodgeData DodgeData
	if err := json.Unmarshal(data, &dodgeData); err != nil {
		return fmt.Errorf("failed to unmarshal dodge ability data: %w", err)
	}

	d.BaseCombatAbility = NewBaseCombatAbility(BaseCombatAbilityConfig{
		ID:          dodgeData.ID,
		Name:        "Dodge",
		Description: "Attackers have disadvantage against you until your next turn.",
		ActionType:  coreCombat.ActionStandard,
		Ref:         refs.CombatAbilities.Dodge(),
	})

	return nil
}
