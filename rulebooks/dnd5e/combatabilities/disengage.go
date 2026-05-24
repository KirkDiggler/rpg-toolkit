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
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/conditions"
	dnd5eEvents "github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/events"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/refs"
)

// Disengage represents the Disengage combat ability.
// When activated, it consumes 1 action and publishes a DisengageActivatedEvent.
// The Disengaging condition (to be implemented) will:
// - Prevent the character's movement from provoking opportunity attacks
// The effect lasts for the rest of the character's turn.
type Disengage struct {
	*BaseCombatAbility
}

// DisengageData is the JSON structure for persisting Disengage ability state
type DisengageData struct {
	Ref *core.Ref `json:"ref"`
	ID  string    `json:"id"`
}

// NewDisengage creates a new Disengage combat ability that uses a standard action.
// This is the default Disengage action available to all characters.
func NewDisengage(id string) *Disengage {
	return &Disengage{
		BaseCombatAbility: NewBaseCombatAbility(BaseCombatAbilityConfig{
			ID:          id,
			Name:        "Disengage",
			Description: "Your movement doesn't provoke opportunity attacks this turn.",
			ActionType:  coreCombat.ActionStandard,
			Ref:         refs.CombatAbilities.Disengage(),
		}),
	}
}

// NewBonusDisengage creates a Disengage ability that uses a bonus action.
// This is used by features like Rogue's Cunning Action or Monk's Step of the Wind.
func NewBonusDisengage(id string) *Disengage {
	return &Disengage{
		BaseCombatAbility: NewBaseCombatAbility(BaseCombatAbilityConfig{
			ID:          id,
			Name:        "Disengage",
			Description: "Your movement doesn't provoke opportunity attacks this turn.",
			ActionType:  coreCombat.ActionBonus,
			Ref:         refs.CombatAbilities.Disengage(),
		}),
	}
}

// CanActivate checks if the Disengage ability can be activated.
// Requires an available action and an event bus.
func (d *Disengage) CanActivate(ctx context.Context, owner core.Entity, input CombatAbilityInput) error {
	// Check base requirements (action economy)
	if err := d.BaseCombatAbility.CanActivate(ctx, owner, input); err != nil {
		return err
	}

	// Disengage requires event bus to publish the activation event
	if input.Bus == nil {
		return rpgerr.New(rpgerr.CodeInvalidArgument, "event bus required for Disengage")
	}

	return nil
}

// Activate consumes 1 action (or bonus action for NewBonusDisengage),
// applies the DisengagingCondition to the owner on input.Bus, and
// publishes a DisengageActivatedEvent for game-server telemetry.
//
// Wave 2.11e (#666): toolkit-side rule application per
// project_toolkit_as_product framing. Before this change, the comment
// here said "A DisengagingCondition (to be implemented in a future
// phase) will subscribe to this event and apply the actual mechanical
// effects" — but the condition has always been ready; the gap was that
// no Activate path applied it. Now the condition gets Apply'd here, so
// the next MovementChain for the owner emits OAPreventionSources and
// OpportunityAttackCondition.onMovementChain skips publishing a trigger.
//
// The condition removes itself automatically on the owner's TurnEnd.
func (d *Disengage) Activate(ctx context.Context, owner core.Entity, input CombatAbilityInput) error {
	// Validate event bus before consuming action
	if input.Bus == nil {
		return rpgerr.New(rpgerr.CodeInvalidArgument, "event bus required for Disengage")
	}

	// Consume the action via base implementation
	if err := d.BaseCombatAbility.Activate(ctx, owner, input); err != nil {
		return err
	}

	// Apply the DisengagingCondition. The condition subscribes to
	// MovementChain (adds OAPreventionSources when the owner moves) and
	// TurnEndTopic (self-removes when the owner's turn ends).
	condition := conditions.NewDisengagingCondition(owner.GetID())
	if err := condition.Apply(ctx, input.Bus); err != nil {
		return fmt.Errorf("failed to apply disengaging condition: %w", err)
	}

	// Telemetry event for the game server. The condition is already
	// applied; this is the activation signal for stream consumers.
	if err := dnd5eEvents.DisengageActivatedTopic.On(input.Bus).Publish(ctx, dnd5eEvents.DisengageActivatedEvent{
		CharacterID: owner.GetID(),
	}); err != nil {
		return fmt.Errorf("failed to publish disengage activated event: %w", err)
	}

	return nil
}

// ToJSON converts the Disengage ability to JSON for persistence
func (d *Disengage) ToJSON() (json.RawMessage, error) {
	data := DisengageData{
		Ref: d.Ref(),
		ID:  d.GetID(),
	}

	bytes, err := json.Marshal(data)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal disengage ability data: %w", err)
	}

	return bytes, nil
}

// loadJSON deserializes a Disengage ability from JSON
func (d *Disengage) loadJSON(data json.RawMessage) error {
	var disengageData DisengageData
	if err := json.Unmarshal(data, &disengageData); err != nil {
		return fmt.Errorf("failed to unmarshal disengage ability data: %w", err)
	}

	d.BaseCombatAbility = NewBaseCombatAbility(BaseCombatAbilityConfig{
		ID:          disengageData.ID,
		Name:        "Disengage",
		Description: "Your movement doesn't provoke opportunity attacks this turn.",
		ActionType:  coreCombat.ActionStandard,
		Ref:         refs.CombatAbilities.Disengage(),
	})

	return nil
}
