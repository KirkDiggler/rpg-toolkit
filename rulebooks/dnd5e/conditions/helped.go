// Copyright (C) 2024 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package conditions

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/KirkDiggler/rpg-toolkit/core"
	"github.com/KirkDiggler/rpg-toolkit/core/chain"
	"github.com/KirkDiggler/rpg-toolkit/events"
	"github.com/KirkDiggler/rpg-toolkit/rpgerr"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/combat"
	dnd5eEvents "github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/events"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/refs"
)

// HelpedConditionData is the serializable form of the helped condition.
// This is stored by the game server as an opaque JSON blob.
type HelpedConditionData struct {
	Ref         *core.Ref `json:"ref"`
	CharacterID string    `json:"character_id"`
	HelperID    string    `json:"helper_id"`
}

// HelpedCondition grants advantage on the helped ally's next attack roll
// (PHB p.192 Help, attack branch). Applied when a character uses the Help
// combat ability targeting an ally. Beat 2 (rpg-project#75) scopes this to
// the attack branch only — the ability-check branch and its chain subscriber
// are explicitly deferred (R4); see the design doc's Resolved Decisions.
//
// Two removal paths:
//   - Consumed on the ally's next attack (AttackChain, same self-consuming
//     shape as HiddenCondition's attacker branch).
//   - Safety-net removal at the HELPER's next turn if the ally never attacks
//     (TurnStartTopic keyed on HelperID, not CharacterID — this is the one
//     place a condition's own turn-boundary trigger is a DIFFERENT actor than
//     the condition's owner).
type HelpedCondition struct {
	CharacterID     string // the ally who benefits from the advantage
	HelperID        string // the helper; their next turn is the safety-net removal trigger
	bus             events.EventBus
	subscriptionIDs []string
}

// Ensure HelpedCondition implements dnd5eEvents.ConditionBehavior
var _ dnd5eEvents.ConditionBehavior = (*HelpedCondition)(nil)

// Ref returns the canonical ref this condition names itself by — the same ref
// its ToJSON embeds and its loader routes on.
func (h *HelpedCondition) Ref() *core.Ref { return refs.Conditions.Helped() }

// NewHelpedCondition creates a new Helped condition granting the given ally
// advantage, with removal tied to the helper's next turn as a safety net.
func NewHelpedCondition(characterID, helperID string) *HelpedCondition {
	return &HelpedCondition{
		CharacterID: characterID,
		HelperID:    helperID,
	}
}

// IsApplied returns true if this condition is currently applied.
func (h *HelpedCondition) IsApplied() bool {
	return h.bus != nil
}

// Apply subscribes this condition to AttackChain (grants + consumes on the
// ally's next attack) and TurnStartTopic (safety-net removal at the
// helper's next turn if unused). No AbilityCheckChain subscriber this wave
// (R4 — attack branch only).
func (h *HelpedCondition) Apply(ctx context.Context, bus events.EventBus) error {
	if h.IsApplied() {
		return rpgerr.New(rpgerr.CodeAlreadyExists, "helped condition already applied")
	}
	h.bus = bus

	attackChain := dnd5eEvents.AttackChain.On(bus)
	subID1, err := attackChain.SubscribeWithChain(ctx, h.onAttackChain)
	if err != nil {
		h.bus = nil
		return rpgerr.Wrap(err, "failed to subscribe to attack chain")
	}
	h.subscriptionIDs = append(h.subscriptionIDs, subID1)

	turnStartTopic := dnd5eEvents.TurnStartTopic.On(bus)
	subID2, err := turnStartTopic.Subscribe(ctx, h.onTurnStart)
	if err != nil {
		_ = h.Remove(ctx, bus)
		return rpgerr.Wrap(err, "failed to subscribe to turn start topic")
	}
	h.subscriptionIDs = append(h.subscriptionIDs, subID2)

	return nil
}

// Remove unsubscribes this condition from all events.
func (h *HelpedCondition) Remove(ctx context.Context, bus events.EventBus) error {
	if h.bus == nil {
		return nil // Not applied, nothing to remove
	}

	total := len(h.subscriptionIDs)
	var errs []error
	for _, subID := range h.subscriptionIDs {
		if err := bus.Unsubscribe(ctx, subID); err != nil {
			errs = append(errs, fmt.Errorf("unsubscribe %s: %w", subID, err))
		}
	}

	h.subscriptionIDs = nil
	h.bus = nil

	if len(errs) > 0 {
		return fmt.Errorf("failed to unsubscribe %d/%d subscriptions: %w", len(errs), total, errors.Join(errs...))
	}
	return nil
}

// ToJSON converts the condition to JSON for persistence.
func (h *HelpedCondition) ToJSON() (json.RawMessage, error) {
	data := HelpedConditionData{
		Ref:         refs.Conditions.Helped(),
		CharacterID: h.CharacterID,
		HelperID:    h.HelperID,
	}
	return json.Marshal(data)
}

// loadJSON loads helped condition state from JSON.
func (h *HelpedCondition) loadJSON(data json.RawMessage) error {
	var helpedData HelpedConditionData
	if err := json.Unmarshal(data, &helpedData); err != nil {
		return rpgerr.Wrap(err, "failed to unmarshal helped data")
	}

	h.CharacterID = helpedData.CharacterID
	h.HelperID = helpedData.HelperID
	return nil
}

// onAttackChain grants advantage when the helped ally attacks, then consumes
// the condition — mirrors HiddenCondition's self-consuming attacker branch.
// Unsubscribing here is safe mid-dispatch: events.simpleEventBus.Publish
// snapshots subscribers before invoking handlers.
func (h *HelpedCondition) onAttackChain(
	ctx context.Context,
	event dnd5eEvents.AttackChainEvent,
	c chain.Chain[dnd5eEvents.AttackChainEvent],
) (chain.Chain[dnd5eEvents.AttackChainEvent], error) {
	if event.AttackerID != h.CharacterID {
		return c, nil
	}

	modifyAttack := func(_ context.Context, e dnd5eEvents.AttackChainEvent) (dnd5eEvents.AttackChainEvent, error) {
		e.AdvantageSources = append(e.AdvantageSources, dnd5eEvents.AttackModifierSource{
			SourceRef: refs.Conditions.Helped(),
			SourceID:  h.HelperID,
			Reason:    "Helped",
		})
		return e, nil
	}
	if err := c.Add(combat.StageConditions, "helped_advantage", modifyAttack); err != nil {
		return c, rpgerr.Wrapf(err, "failed to add helped advantage modifier for character %s", h.CharacterID)
	}

	if h.bus != nil {
		removals := dnd5eEvents.ConditionRemovedTopic.On(h.bus)
		if err := removals.Publish(ctx, dnd5eEvents.ConditionRemovedEvent{
			CharacterID:  h.CharacterID,
			ConditionRef: refs.Conditions.Helped().String(),
			Reason:       "consumed",
		}); err != nil {
			return c, rpgerr.Wrapf(err, "failed to publish helped removal for character %s", h.CharacterID)
		}
		if err := h.Remove(ctx, h.bus); err != nil {
			return c, rpgerr.Wrapf(err, "failed to remove helped condition for character %s", h.CharacterID)
		}
	}

	return c, nil
}

// onTurnStart is the safety-net removal: if the ally never attacks, Helped
// is removed at the start of the HELPER's next turn (not the ally's —
// PHB p.192: "before the start of your [the helper's] next turn").
func (h *HelpedCondition) onTurnStart(ctx context.Context, event dnd5eEvents.TurnStartEvent) error {
	if event.CharacterID != h.HelperID {
		return nil
	}

	if h.bus == nil {
		return nil
	}

	removals := dnd5eEvents.ConditionRemovedTopic.On(h.bus)
	err := removals.Publish(ctx, dnd5eEvents.ConditionRemovedEvent{
		CharacterID:  h.CharacterID,
		ConditionRef: refs.Conditions.Helped().String(),
		Reason:       "turn_start",
	})
	if err != nil {
		return rpgerr.Wrapf(err, "failed to publish helped removal for character %s", h.CharacterID)
	}

	return h.Remove(ctx, h.bus)
}
