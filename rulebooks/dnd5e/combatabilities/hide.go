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

// Hide represents the Hide combat ability (PHB p.192).
// When activated, it consumes 1 action and publishes a HideActivatedEvent.
// The character attempts to become hidden — a Dexterity (Stealth) check against
// observers' passive Perception. On success the character is Hidden (unseen and
// unheard) until they are discovered, make noise, attack, or move into the open.
//
// Mirrors the Dodge/Disengage bar: this consumes the action and emits the
// activation signal. Resolving the Stealth check and applying the Hidden
// condition is a later beat (the toolkit has the proficiency/check machinery;
// nothing yet wires HideActivated → a check → the Hidden condition).
type Hide struct {
	*BaseCombatAbility
}

// HideData is the JSON structure for persisting Hide ability state.
type HideData struct {
	Ref *core.Ref `json:"ref"`
	ID  string    `json:"id"`
}

// NewHide creates a new Hide combat ability that uses a standard action.
// This is the default Hide action available to all characters.
func NewHide(id string) *Hide {
	return &Hide{
		BaseCombatAbility: NewBaseCombatAbility(BaseCombatAbilityConfig{
			ID:          id,
			Name:        "Hide",
			Description: "Make a Stealth check to become hidden from creatures that can't see you.",
			ActionType:  coreCombat.ActionStandard,
			Ref:         refs.CombatAbilities.Hide(),
		}),
	}
}

// CanActivate checks if the Hide ability can be activated.
// Requires an available action and an event bus.
func (h *Hide) CanActivate(ctx context.Context, owner core.Entity, input CombatAbilityInput) error {
	if err := h.BaseCombatAbility.CanActivate(ctx, owner, input); err != nil {
		return err
	}
	if input.Bus == nil {
		return rpgerr.New(rpgerr.CodeInvalidArgument, "event bus required for Hide")
	}
	return nil
}

// Activate consumes 1 action and publishes a HideActivatedEvent.
// A subscriber in a later beat resolves the Stealth check and applies Hidden.
func (h *Hide) Activate(ctx context.Context, owner core.Entity, input CombatAbilityInput) error {
	if input.Bus == nil {
		return rpgerr.New(rpgerr.CodeInvalidArgument, "event bus required for Hide")
	}
	if err := h.BaseCombatAbility.Activate(ctx, owner, input); err != nil {
		return err
	}
	if err := dnd5eEvents.HideActivatedTopic.On(input.Bus).Publish(ctx, dnd5eEvents.HideActivatedEvent{
		CharacterID: owner.GetID(),
	}); err != nil {
		return fmt.Errorf("failed to publish hide activated event: %w", err)
	}
	return nil
}

// ToJSON converts the Hide ability to JSON for persistence.
func (h *Hide) ToJSON() (json.RawMessage, error) {
	data := HideData{Ref: h.Ref(), ID: h.GetID()}
	bytes, err := json.Marshal(data)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal hide ability data: %w", err)
	}
	return bytes, nil
}

// loadJSON deserializes a Hide ability from JSON.
func (h *Hide) loadJSON(data json.RawMessage) error {
	var hideData HideData
	if err := json.Unmarshal(data, &hideData); err != nil {
		return fmt.Errorf("failed to unmarshal hide ability data: %w", err)
	}
	h.BaseCombatAbility = NewBaseCombatAbility(BaseCombatAbilityConfig{
		ID:          hideData.ID,
		Name:        "Hide",
		Description: "Make a Stealth check to become hidden from creatures that can't see you.",
		ActionType:  coreCombat.ActionStandard,
		Ref:         refs.CombatAbilities.Hide(),
	})
	return nil
}
