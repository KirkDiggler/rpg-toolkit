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

// Help represents the Help combat ability (PHB p.192).
// When activated, it consumes 1 action, publishes a HelpActivatedEvent, and
// grants the targeted ally advantage on their next attack roll via
// HelpedCondition.
//
// Beat 2 (rpg-project#75) scopes Help to the ATTACK branch only — advantage
// on an ally's next ability check (the "helping a task" branch of PHB p.192)
// is explicitly deferred (R4): the fixture that would exercise it doesn't
// exist yet, and HelpedCondition ships with no AbilityCheckChain subscriber
// this wave.
type Help struct {
	*BaseCombatAbility
}

// HelpData is the JSON structure for persisting Help ability state.
type HelpData struct {
	Ref *core.Ref `json:"ref"`
	ID  string    `json:"id"`
}

// NewHelp creates a new Help combat ability that uses a standard action.
// This is the default Help action available to all characters.
func NewHelp(id string) *Help {
	return &Help{
		BaseCombatAbility: NewBaseCombatAbility(BaseCombatAbilityConfig{
			ID:          id,
			Name:        "Help",
			Description: "Aid an ally: their next check or attack against a foe near you gains advantage.",
			ActionType:  coreCombat.ActionStandard,
			Ref:         refs.CombatAbilities.Help(),
		}),
	}
}

// CanActivate checks if the Help ability can be activated.
// Requires an available action, an event bus, and a target ally.
func (h *Help) CanActivate(ctx context.Context, owner core.Entity, input CombatAbilityInput) error {
	if err := h.BaseCombatAbility.CanActivate(ctx, owner, input); err != nil {
		return err
	}
	if input.Bus == nil {
		return rpgerr.New(rpgerr.CodeInvalidArgument, "event bus required for Help")
	}
	if input.Target == nil {
		return rpgerr.New(rpgerr.CodeInvalidArgument, "target ally required for Help")
	}
	return nil
}

// Activate consumes 1 action, publishes a HelpActivatedEvent, and applies
// HelpedCondition to the targeted ally via a cross-entity
// ConditionAppliedEvent{Target: input.Target} — the same
// ConditionAppliedTopic path Rage uses for self-targeting, extended to a
// target other than the activator.
func (h *Help) Activate(ctx context.Context, owner core.Entity, input CombatAbilityInput) error {
	if input.Bus == nil {
		return rpgerr.New(rpgerr.CodeInvalidArgument, "event bus required for Help")
	}
	if input.Target == nil {
		return rpgerr.New(rpgerr.CodeInvalidArgument, "target ally required for Help")
	}
	if err := h.BaseCombatAbility.Activate(ctx, owner, input); err != nil {
		return err
	}

	allyID := input.Target.GetID()

	if err := dnd5eEvents.HelpActivatedTopic.On(input.Bus).Publish(ctx, dnd5eEvents.HelpActivatedEvent{
		CharacterID: owner.GetID(),
		AllyID:      allyID,
	}); err != nil {
		return fmt.Errorf("failed to publish help activated event: %w", err)
	}

	helpedCondition := conditions.NewHelpedCondition(allyID, owner.GetID())
	if err := dnd5eEvents.ConditionAppliedTopic.On(input.Bus).Publish(ctx, dnd5eEvents.ConditionAppliedEvent{
		Target:    input.Target,
		Type:      dnd5eEvents.ConditionHelped,
		Source:    dnd5eEvents.ConditionSourceCombatAbility,
		Condition: helpedCondition,
	}); err != nil {
		return fmt.Errorf("failed to publish helped condition applied event: %w", err)
	}

	return nil
}

// ToJSON converts the Help ability to JSON for persistence.
func (h *Help) ToJSON() (json.RawMessage, error) {
	data := HelpData{Ref: h.Ref(), ID: h.GetID()}
	bytes, err := json.Marshal(data)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal help ability data: %w", err)
	}
	return bytes, nil
}

// loadJSON deserializes a Help ability from JSON.
func (h *Help) loadJSON(data json.RawMessage) error {
	var helpData HelpData
	if err := json.Unmarshal(data, &helpData); err != nil {
		return fmt.Errorf("failed to unmarshal help ability data: %w", err)
	}
	h.BaseCombatAbility = NewBaseCombatAbility(BaseCombatAbilityConfig{
		ID:          helpData.ID,
		Name:        "Help",
		Description: "Aid an ally: their next check or attack against a foe near you gains advantage.",
		ActionType:  coreCombat.ActionStandard,
		Ref:         refs.CombatAbilities.Help(),
	})
	return nil
}
