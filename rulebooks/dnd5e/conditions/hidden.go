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

// HiddenConditionData is the serializable form of the hidden condition.
// This is stored by the game server as an opaque JSON blob.
type HiddenConditionData struct {
	Ref         *core.Ref `json:"ref"`
	CharacterID string    `json:"character_id"`
}

// HiddenCondition grants advantage on the hidden character's attacks and
// disadvantage on attacks against them. This condition is applied when a
// character succeeds a Hide stealth check and removes itself when the
// hidden character makes their own attack (PHB p.192: Hidden ends when you
// attack). Both directions subscribe to the same AttackChain — mirrors
// DodgingCondition's registration shape but adds a self-consuming attacker
// branch DodgingCondition doesn't need.
type HiddenCondition struct {
	CharacterID     string
	bus             events.EventBus
	subscriptionIDs []string
}

// Ensure HiddenCondition implements dnd5eEvents.ConditionBehavior
var _ dnd5eEvents.ConditionBehavior = (*HiddenCondition)(nil)

// Ref returns the canonical ref this condition names itself by — the same ref
// its ToJSON embeds and its loader routes on.
func (h *HiddenCondition) Ref() *core.Ref { return refs.Conditions.Hidden() }

// NewHiddenCondition creates a new Hidden condition for the specified character.
func NewHiddenCondition(characterID string) *HiddenCondition {
	return &HiddenCondition{
		CharacterID: characterID,
	}
}

// IsApplied returns true if this condition is currently applied.
func (h *HiddenCondition) IsApplied() bool {
	return h.bus != nil
}

// Apply subscribes this condition to AttackChain in both directions:
// advantage when the hidden character attacks (which also ends Hidden),
// disadvantage when the hidden character is attacked.
func (h *HiddenCondition) Apply(ctx context.Context, bus events.EventBus) error {
	if h.IsApplied() {
		return rpgerr.New(rpgerr.CodeAlreadyExists, "hidden condition already applied")
	}
	h.bus = bus

	attackChain := dnd5eEvents.AttackChain.On(bus)
	subID, err := attackChain.SubscribeWithChain(ctx, h.onAttackChain)
	if err != nil {
		h.bus = nil
		return rpgerr.Wrap(err, "failed to subscribe to attack chain")
	}
	h.subscriptionIDs = append(h.subscriptionIDs, subID)

	return nil
}

// Remove unsubscribes this condition from all events.
func (h *HiddenCondition) Remove(ctx context.Context, bus events.EventBus) error {
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
func (h *HiddenCondition) ToJSON() (json.RawMessage, error) {
	data := HiddenConditionData{
		Ref:         refs.Conditions.Hidden(),
		CharacterID: h.CharacterID,
	}
	return json.Marshal(data)
}

// loadJSON loads hidden condition state from JSON.
func (h *HiddenCondition) loadJSON(data json.RawMessage) error {
	var hiddenData HiddenConditionData
	if err := json.Unmarshal(data, &hiddenData); err != nil {
		return rpgerr.Wrap(err, "failed to unmarshal hidden data")
	}

	h.CharacterID = hiddenData.CharacterID
	return nil
}

// onAttackChain handles attack events in both directions:
//   - When the hidden character is the attacker, grants advantage on that
//     attack, then removes Hidden (PHB p.192: attacking ends Hidden). The
//     removal happens after the modifier is queued on the chain, so this
//     attack still gets its advantage — Remove only stops FUTURE events from
//     reaching this condition (see events.simpleEventBus.Publish, which
//     snapshots subscribers before invoking handlers, so unsubscribing here
//     is safe mid-dispatch).
//   - When the hidden character is the target, imposes disadvantage on the
//     attacker. This does not end Hidden (no "seen" break trigger this wave).
func (h *HiddenCondition) onAttackChain(
	ctx context.Context,
	event dnd5eEvents.AttackChainEvent,
	c chain.Chain[dnd5eEvents.AttackChainEvent],
) (chain.Chain[dnd5eEvents.AttackChainEvent], error) {
	switch h.CharacterID {
	case event.AttackerID:
		modifyAttack := func(_ context.Context, e dnd5eEvents.AttackChainEvent) (dnd5eEvents.AttackChainEvent, error) {
			e.AdvantageSources = append(e.AdvantageSources, dnd5eEvents.AttackModifierSource{
				SourceRef: refs.Conditions.Hidden(),
				SourceID:  h.CharacterID,
				Reason:    "Hidden",
			})
			return e, nil
		}
		if err := c.Add(combat.StageConditions, "hidden_attacker_advantage", modifyAttack); err != nil {
			return c, rpgerr.Wrapf(err, "failed to add hidden advantage modifier for character %s", h.CharacterID)
		}

		// Hidden ends when the hidden character attacks. Publish the removal
		// signal and unsubscribe now — the advantage modifier above is already
		// queued on the chain and will still apply to this attack.
		if h.bus != nil {
			removals := dnd5eEvents.ConditionRemovedTopic.On(h.bus)
			if err := removals.Publish(ctx, dnd5eEvents.ConditionRemovedEvent{
				MemberID:     h.CharacterID,
				ConditionRef: refs.Conditions.Hidden().String(),
				Reason:       "attacked",
			}); err != nil {
				return c, rpgerr.Wrapf(err, "failed to publish hidden removal for character %s", h.CharacterID)
			}
			if err := h.Remove(ctx, h.bus); err != nil {
				return c, rpgerr.Wrapf(err, "failed to remove hidden condition for character %s", h.CharacterID)
			}
		}

	case event.TargetID:
		modifyAttack := func(_ context.Context, e dnd5eEvents.AttackChainEvent) (dnd5eEvents.AttackChainEvent, error) {
			e.DisadvantageSources = append(e.DisadvantageSources, dnd5eEvents.AttackModifierSource{
				SourceRef: refs.Conditions.Hidden(),
				SourceID:  h.CharacterID,
				Reason:    "Hidden",
			})
			return e, nil
		}
		if err := c.Add(combat.StageConditions, "hidden_target_disadvantage", modifyAttack); err != nil {
			return c, rpgerr.Wrapf(err, "failed to add hidden disadvantage modifier for character %s", h.CharacterID)
		}
	}

	return c, nil
}
