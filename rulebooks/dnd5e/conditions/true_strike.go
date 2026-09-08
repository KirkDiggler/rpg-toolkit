// Copyright (C) 2026 Kirk Diggler
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

// TrueStrikeName is what a player is shown wherever this condition appears.
const TrueStrikeName = "True Strike"

// TrueStrikeConditionData is the serializable form of the true strike
// condition, stored by the game server as an opaque JSON blob.
type TrueStrikeConditionData struct {
	Ref       *core.Ref `json:"ref"`
	MemberID  string    `json:"member_id"`
	TargetID  string    `json:"target_id"`
	SourceRef string    `json:"source_ref"`
}

// TrueStrikeCondition is the caster's foreknowledge of one creature's defenses.
//
// # It sits on the CASTER, keyed to the target
//
// The advantage is the caster's, and it is only good against the creature they
// pointed at, so the condition is applied to the caster and carries the
// target's member id. That is why it reads AttackerID and TargetID both: a
// condition on the target could not tell whose attack it was improving.
//
// # It is consumed by the first matching attack
//
// One attack, then gone — [HelpedCondition]'s self-consuming shape, with a
// matching target as the extra test. An attack against anybody else passes
// through untouched and leaves the condition in place.
//
// # It does not hold its own clock
//
// True Strike is a concentration cantrip, so the duration belongs to the
// caster's [ConcentratingCondition] and this condition ends when that one does.
// One spell, one duration, one answer: a count here beside the owner's would be
// two clocks answering the same question.
//
// # Divergence from RAW, named rather than hidden
//
// RAW's True Strike grants its advantage on your NEXT turn. Here the advantage
// applies to the caster's next attack against that target.
type TrueStrikeCondition struct {
	// MemberID is the caster who holds the advantage.
	MemberID string

	// TargetID is the only creature the advantage is good against.
	TargetID string

	// SourceRef is what granted this, as a ref string — the spell, not the
	// caster. Named apart from MemberID because "who holds it" and "what
	// applied it" are different questions, and a display that wants to say
	// "True Strike" reads this rather than inferring it.
	SourceRef string

	bus             events.EventBus
	subscriptionIDs []string
}

// Ensure TrueStrikeCondition implements dnd5eEvents.ConditionBehavior
var _ dnd5eEvents.ConditionBehavior = (*TrueStrikeCondition)(nil)

// Ref returns the canonical ref this condition names itself by — the same ref
// its ToJSON embeds and its loader routes on.
func (t *TrueStrikeCondition) Ref() *core.Ref { return refs.Conditions.TrueStrike() }

// NewTrueStrikeCondition creates the caster's advantage against one named
// creature, good until it is used or the caster's concentration ends.
//
// An empty sourceRef falls back to the True Strike spell, which is the only
// thing in this rulebook that applies this condition. A condition that recorded
// "" as what granted it would be a blank where the answer was never in doubt.
func NewTrueStrikeCondition(casterID, targetID, sourceRef string) *TrueStrikeCondition {
	if sourceRef == "" {
		sourceRef = refs.Spells.TrueStrike().String()
	}
	return &TrueStrikeCondition{
		MemberID:  casterID,
		TargetID:  targetID,
		SourceRef: sourceRef,
	}
}

// IsApplied returns true if this condition is currently applied.
func (t *TrueStrikeCondition) IsApplied() bool { return t.bus != nil }

// Apply subscribes the advantage to the attack chain that spends it and to the
// end of the fight. NOT to turn ends: the spell's duration is the caster's
// concentration, and this condition ends when that one publishes its removal.
func (t *TrueStrikeCondition) Apply(ctx context.Context, bus events.EventBus) error {
	if t.IsApplied() {
		return rpgerr.New(rpgerr.CodeAlreadyExists, "true strike condition already applied")
	}
	t.bus = bus

	attackChain := dnd5eEvents.AttackChain.On(bus)
	attackSub, err := attackChain.SubscribeWithChain(ctx, t.onAttackChain)
	if err != nil {
		t.bus = nil
		return rpgerr.Wrap(err, "failed to subscribe to attack chain")
	}
	t.subscriptionIDs = append(t.subscriptionIDs, attackSub)

	combatEnds := dnd5eEvents.CombatEndTopic.On(bus)
	combatSub, err := combatEnds.Subscribe(ctx, t.onCombatEnd)
	if err != nil {
		_ = t.Remove(ctx, bus)
		return rpgerr.Wrap(err, "failed to subscribe to combat end topic")
	}
	t.subscriptionIDs = append(t.subscriptionIDs, combatSub)

	// A rest is not one of this condition's own ends — its owner's clock and
	// combat end both fire first in any ordinary fight — but a blob that
	// survived to a long rest must not outlive it, which is the registry every
	// combat-scoped condition here is in.
	restSub, err := subscribeRemoveOnLongRest(ctx, bus, t.MemberID, t.Ref(), t.Remove)
	if err != nil {
		_ = t.Remove(ctx, bus)
		return rpgerr.Wrap(err, "failed to subscribe to long rest")
	}
	t.subscriptionIDs = append(t.subscriptionIDs, restSub)

	return nil
}

// Remove unsubscribes this condition from all events.
func (t *TrueStrikeCondition) Remove(ctx context.Context, bus events.EventBus) error {
	if t.bus == nil {
		return nil
	}

	total := len(t.subscriptionIDs)
	var errs []error
	for _, subID := range t.subscriptionIDs {
		if err := bus.Unsubscribe(ctx, subID); err != nil {
			errs = append(errs, fmt.Errorf("unsubscribe %s: %w", subID, err))
		}
	}

	t.subscriptionIDs = nil
	t.bus = nil

	if len(errs) > 0 {
		return fmt.Errorf("failed to unsubscribe %d/%d subscriptions: %w", len(errs), total, errors.Join(errs...))
	}
	return nil
}

// ToJSON converts the condition to JSON for persistence.
func (t *TrueStrikeCondition) ToJSON() (json.RawMessage, error) {
	return json.Marshal(TrueStrikeConditionData{
		Ref:       refs.Conditions.TrueStrike(),
		MemberID:  t.MemberID,
		TargetID:  t.TargetID,
		SourceRef: t.SourceRef,
	})
}

// loadJSON loads true strike condition state from JSON.
func (t *TrueStrikeCondition) loadJSON(data json.RawMessage) error {
	var stored TrueStrikeConditionData
	if err := json.Unmarshal(data, &stored); err != nil {
		return rpgerr.Wrap(err, "failed to unmarshal true strike data")
	}
	t.MemberID = stored.MemberID
	t.TargetID = stored.TargetID
	t.SourceRef = stored.SourceRef
	if t.SourceRef == "" {
		t.SourceRef = refs.Spells.TrueStrike().String()
	}
	return nil
}

// onAttackChain grants the caster advantage on their attack against the named
// creature, then consumes the condition. Somebody else's attack, or an attack
// on anybody else, passes through unchanged.
//
// Unsubscribing mid-dispatch is safe: the bus snapshots subscribers before
// invoking handlers (see [HelpedCondition.onAttackChain]).
func (t *TrueStrikeCondition) onAttackChain(
	ctx context.Context,
	event dnd5eEvents.AttackChainEvent,
	c chain.Chain[dnd5eEvents.AttackChainEvent],
) (chain.Chain[dnd5eEvents.AttackChainEvent], error) {
	if event.AttackerID != t.MemberID || event.TargetID != t.TargetID {
		return c, nil
	}

	grant := func(_ context.Context, e dnd5eEvents.AttackChainEvent) (dnd5eEvents.AttackChainEvent, error) {
		e.AdvantageSources = append(e.AdvantageSources, dnd5eEvents.AttackModifierSource{
			SourceRef: refs.Conditions.TrueStrike(),
			SourceID:  t.MemberID,
			Reason:    TrueStrikeName,
		})
		return e, nil
	}
	if err := c.Add(combat.StageConditions, "true_strike_advantage", grant); err != nil {
		return c, rpgerr.Wrapf(err, "failed to add true strike advantage for member %s", t.MemberID)
	}

	return c, t.end(ctx, "consumed")
}

// onCombatEnd ends the advantage with the fight.
func (t *TrueStrikeCondition) onCombatEnd(ctx context.Context, event dnd5eEvents.CombatEndEvent) error {
	if event.SubjectID != t.MemberID {
		return nil
	}
	return t.end(ctx, "combat ended")
}

// end publishes the removal and detaches. The publish comes first so the
// removal is on the bus while this condition still owns the advantage, which is
// what an activation's effect collector reads.
func (t *TrueStrikeCondition) end(ctx context.Context, reason string) error {
	if t.bus == nil {
		return nil
	}
	bus := t.bus
	if err := dnd5eEvents.ConditionRemovedTopic.On(bus).Publish(ctx, dnd5eEvents.ConditionRemovedEvent{
		MemberID:     t.MemberID,
		ConditionRef: refs.Conditions.TrueStrike().String(),
		Reason:       reason,
	}); err != nil {
		return rpgerr.Wrapf(err, "failed to publish true strike removal for member %s", t.MemberID)
	}
	return t.Remove(ctx, bus)
}
