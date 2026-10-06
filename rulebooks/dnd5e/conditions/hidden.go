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
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/contributions"
	dnd5eEvents "github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/events"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/refs"
)

// HiddenConditionData is the serializable form of the hidden condition.
// This is stored by the game server as an opaque JSON blob.
type HiddenConditionData struct {
	Ref      *core.Ref `json:"ref"`
	MemberID string    `json:"member_id"`
}

// HiddenCondition grants advantage on the hidden character's attacks and
// disadvantage on attacks against them. This condition is applied when a
// character succeeds a Hide stealth check and removes itself when the
// hidden character makes their own attack (PHB p.192: Hidden ends when you
// attack). Both directions subscribe to the same AttackChain — mirrors
// DodgingCondition's registration shape but adds a self-consuming attacker
// branch DodgingCondition doesn't need.
type HiddenCondition struct {
	MemberID        string
	bus             events.EventBus
	subscriptionIDs []string
}

// Ensure HiddenCondition implements dnd5eEvents.ConditionBehavior
var _ dnd5eEvents.ConditionBehavior = (*HiddenCondition)(nil)

var _ contributions.ActionAssessor = (*HiddenCondition)(nil)

// AssessAction answers whether being hidden bears on the framed attack: every
// attack roll its holder makes has advantage. Attacking ends Hidden; reading
// this answer ends nothing.
func (h *HiddenCondition) AssessAction(in *contributions.AssessActionInput) (*contributions.AssessActionOutput, error) {
	return h.attackRule().AssessAction(in)
}

// heldRule is the by-reference rule for this Hidden on its holder — the one
// information asks for a candidate. Attacks against the hidden holder have
// disadvantage.
func (h *HiddenCondition) heldRule() contributions.ActionAssessor {
	return newHiddenHeldRule(h.MemberID, heldAddress(h.MemberID, h))
}

func (h *HiddenCondition) attackRule() attackRollRule {
	return attackRollRule{name: "hidden", owner: h.MemberID, text: attackRollText{
		NotOwner:    "Hidden affects only its holder's attacks",
		OnlyAttacks: "Hidden affects only attack rolls",
		Applies:     "You are hidden; attacking ends it",
		Benefit:     advantageBenefit,
	}, mode: contributions.AttackAdvantage}
}

// Ref returns the canonical ref this condition names itself by — the same ref
// its ToJSON embeds and its loader routes on.
func (h *HiddenCondition) Ref() *core.Ref { return refs.Conditions.Hidden() }

// NewHiddenCondition creates a new Hidden condition for the specified character.
func NewHiddenCondition(characterID string) *HiddenCondition {
	return &HiddenCondition{
		MemberID: characterID,
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

	longRestSubID, err := subscribeRemoveOnLongRest(ctx, bus, subscribeRemoveOnLongRestInput{
		Address: ConditionAddressOf(h.MemberID, h), Remove: h.Remove,
	})
	if err != nil {
		_ = h.Remove(ctx, bus)
		return rpgerr.Wrap(err, "failed to subscribe to long rest")
	}
	h.subscriptionIDs = append(h.subscriptionIDs, longRestSubID)

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
		Ref:      refs.Conditions.Hidden(),
		MemberID: h.MemberID,
	}
	return json.Marshal(data)
}

// loadJSON loads hidden condition state from JSON.
func (h *HiddenCondition) loadJSON(data json.RawMessage) error {
	var hiddenData HiddenConditionData
	if err := json.Unmarshal(data, &hiddenData); err != nil {
		return rpgerr.Wrap(err, "failed to unmarshal hidden data")
	}

	h.MemberID = hiddenData.MemberID
	return nil
}

// onAttackChain handles attack events in both directions, each through the
// rule information asks:
//   - When the hidden character is the attacker, its attack rule answers and
//     its mode — advantage — is applied; then Hidden is removed (PHB p.192:
//     attacking ends Hidden). The removal happens after the modifier is
//     queued on the chain, so this attack still gets its advantage — Remove
//     only stops FUTURE events from reaching this condition (see
//     events.simpleEventBus.Publish, which snapshots subscribers before
//     invoking handlers, so unsubscribing here is safe mid-dispatch).
//   - When the hidden character is the target, its by-reference held rule
//     answers through applyHeldAttack, imposing disadvantage on the attacker.
//     This does not end Hidden: nothing yet models being seen.
func (h *HiddenCondition) onAttackChain(
	ctx context.Context,
	event dnd5eEvents.AttackChainEvent,
	c chain.Chain[dnd5eEvents.AttackChainEvent],
) (chain.Chain[dnd5eEvents.AttackChainEvent], error) {
	switch h.MemberID {
	case event.AttackerID:
		executed, err := executeRule(&executeRuleInput{Name: "hidden", Rule: h.attackRule(), Frame: event.Frame})
		if err != nil {
			return c, err
		}
		if executed.Answer.Decision.Applicability != contributions.Applies {
			return c, nil
		}
		c, err = applyAttackMode(&attackModeInput{
			Name: "hidden", Answer: executed.Answer, Chain: c,
			SourceRef: refs.Conditions.Hidden(), SourceID: h.MemberID,
			Label: fixedLabel("hidden_attacker_advantage", "Hidden"),
		})
		if err != nil {
			return c, err
		}

		// Hidden ends when the hidden character attacks. Publish the removal
		// signal and unsubscribe now — the advantage modifier above is already
		// queued on the chain and will still apply to this attack.
		if h.bus != nil {
			removals := dnd5eEvents.ConditionRemovedTopic.On(h.bus)
			if err := removals.Publish(ctx, dnd5eEvents.ConditionRemovedEvent{
				MemberID:     h.MemberID,
				ConditionRef: refs.Conditions.Hidden().String(),
				Reason:       "attacked",
			}); err != nil {
				return c, rpgerr.Wrapf(err, "failed to publish hidden removal for character %s", h.MemberID)
			}
			if err := h.Remove(ctx, h.bus); err != nil {
				return c, rpgerr.Wrapf(err, "failed to remove hidden condition for character %s", h.MemberID)
			}
		}

	case event.TargetID:
		return applyHeldAttack(&heldAttackInput{
			Name: "hidden", Holder: h.MemberID, Held: heldAddress(h.MemberID, h), Rule: h.heldRule(), Event: event, Chain: c,
			SourceRef: refs.Conditions.Hidden(), SourceID: h.MemberID,
			Label: fixedLabel("hidden_target_disadvantage", "Hidden"),
		})
	}

	return c, nil
}
