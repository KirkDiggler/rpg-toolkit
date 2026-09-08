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

// ViciousMockeryName is what a player is shown wherever this condition
// appears.
const ViciousMockeryName = "Vicious Mockery"

// ViciousMockeryConditionData is the serializable form of the vicious mockery
// condition, stored by the game server as an opaque JSON blob.
type ViciousMockeryConditionData struct {
	Ref      *core.Ref `json:"ref"`
	MemberID string    `json:"member_id"`
	SourceID string    `json:"source_id"`
}

// ViciousMockeryCondition is the sting of an insult that landed.
//
// # It sits on the TARGET, and spends itself on their next attack
//
// RAW is "it has disadvantage on the next attack roll it makes before the end
// of its next turn", so the condition is on the mocked creature and reads
// AttackerID alone — every attack it makes is the one that matters, whoever it
// is aimed at. Exactly one attack roll takes the disadvantage; the condition
// consumes itself there, [HelpedCondition]'s shape with the other sign.
//
// # And it expires at the end of that creature's next turn
//
// One turn end, and one is right where True Strike's is two: this is applied on
// the BARD's turn, so the very next turn end for the mocked creature is the end
// of its own next turn — the boundary RAW names.
type ViciousMockeryCondition struct {
	// MemberID is the mocked creature carrying the disadvantage.
	MemberID string

	// SourceID is the bard who imposed it. Recorded rather than inferred, so
	// "whose mockery was that" has an answer.
	SourceID string

	bus             events.EventBus
	subscriptionIDs []string
}

// Ensure ViciousMockeryCondition implements dnd5eEvents.ConditionBehavior
var _ dnd5eEvents.ConditionBehavior = (*ViciousMockeryCondition)(nil)

// Ref returns the canonical ref this condition names itself by — the same ref
// its ToJSON embeds and its loader routes on.
func (v *ViciousMockeryCondition) Ref() *core.Ref { return refs.Conditions.ViciousMockery() }

// NewViciousMockeryCondition creates the disadvantage one creature carries into
// its next attack roll after failing its save.
func NewViciousMockeryCondition(memberID, sourceID string) *ViciousMockeryCondition {
	return &ViciousMockeryCondition{MemberID: memberID, SourceID: sourceID}
}

// IsApplied returns true if this condition is currently applied.
func (v *ViciousMockeryCondition) IsApplied() bool { return v.bus != nil }

// Apply subscribes the disadvantage to the attack chain that spends it and to
// the two boundaries that end it: the mocked creature's turn end and the end of
// the fight.
func (v *ViciousMockeryCondition) Apply(ctx context.Context, bus events.EventBus) error {
	if v.IsApplied() {
		return rpgerr.New(rpgerr.CodeAlreadyExists, "vicious mockery condition already applied")
	}
	v.bus = bus

	attackChain := dnd5eEvents.AttackChain.On(bus)
	attackSub, err := attackChain.SubscribeWithChain(ctx, v.onAttackChain)
	if err != nil {
		v.bus = nil
		return rpgerr.Wrap(err, "failed to subscribe to attack chain")
	}
	v.subscriptionIDs = append(v.subscriptionIDs, attackSub)

	turnEnds := dnd5eEvents.TurnEndTopic.On(bus)
	turnSub, err := turnEnds.Subscribe(ctx, v.onTurnEnd)
	if err != nil {
		_ = v.Remove(ctx, bus)
		return rpgerr.Wrap(err, "failed to subscribe to turn end topic")
	}
	v.subscriptionIDs = append(v.subscriptionIDs, turnSub)

	combatEnds := dnd5eEvents.CombatEndTopic.On(bus)
	combatSub, err := combatEnds.Subscribe(ctx, v.onCombatEnd)
	if err != nil {
		_ = v.Remove(ctx, bus)
		return rpgerr.Wrap(err, "failed to subscribe to combat end topic")
	}
	v.subscriptionIDs = append(v.subscriptionIDs, combatSub)

	// A rest is not one of this condition's own ends — its turn boundary and
	// combat end both fire first in any ordinary fight — but a blob that
	// survived to a long rest must not outlive it, which is the registry every
	// combat-scoped condition here is in.
	restSub, err := subscribeRemoveOnLongRest(ctx, bus, v.MemberID, v.Ref(), v.Remove)
	if err != nil {
		_ = v.Remove(ctx, bus)
		return rpgerr.Wrap(err, "failed to subscribe to long rest")
	}
	v.subscriptionIDs = append(v.subscriptionIDs, restSub)

	return nil
}

// Remove unsubscribes this condition from all events.
func (v *ViciousMockeryCondition) Remove(ctx context.Context, bus events.EventBus) error {
	if v.bus == nil {
		return nil
	}

	total := len(v.subscriptionIDs)
	var errs []error
	for _, subID := range v.subscriptionIDs {
		if err := bus.Unsubscribe(ctx, subID); err != nil {
			errs = append(errs, fmt.Errorf("unsubscribe %s: %w", subID, err))
		}
	}

	v.subscriptionIDs = nil
	v.bus = nil

	if len(errs) > 0 {
		return fmt.Errorf("failed to unsubscribe %d/%d subscriptions: %w", len(errs), total, errors.Join(errs...))
	}
	return nil
}

// ToJSON converts the condition to JSON for persistence.
func (v *ViciousMockeryCondition) ToJSON() (json.RawMessage, error) {
	return json.Marshal(ViciousMockeryConditionData{
		Ref:      refs.Conditions.ViciousMockery(),
		MemberID: v.MemberID,
		SourceID: v.SourceID,
	})
}

// loadJSON loads vicious mockery condition state from JSON.
func (v *ViciousMockeryCondition) loadJSON(data json.RawMessage) error {
	var stored ViciousMockeryConditionData
	if err := json.Unmarshal(data, &stored); err != nil {
		return rpgerr.Wrap(err, "failed to unmarshal vicious mockery data")
	}
	v.MemberID = stored.MemberID
	v.SourceID = stored.SourceID
	return nil
}

// onAttackChain imposes disadvantage on the mocked creature's attack and
// consumes the condition. Anybody else's attack passes through unchanged.
//
// Unsubscribing mid-dispatch is safe: the bus snapshots subscribers before
// invoking handlers (see [HelpedCondition.onAttackChain]).
func (v *ViciousMockeryCondition) onAttackChain(
	ctx context.Context,
	event dnd5eEvents.AttackChainEvent,
	c chain.Chain[dnd5eEvents.AttackChainEvent],
) (chain.Chain[dnd5eEvents.AttackChainEvent], error) {
	if event.AttackerID != v.MemberID {
		return c, nil
	}

	impose := func(_ context.Context, e dnd5eEvents.AttackChainEvent) (dnd5eEvents.AttackChainEvent, error) {
		e.DisadvantageSources = append(e.DisadvantageSources, dnd5eEvents.AttackModifierSource{
			SourceRef: refs.Conditions.ViciousMockery(),
			SourceID:  v.SourceID,
			Reason:    ViciousMockeryName,
		})
		return e, nil
	}
	if err := c.Add(combat.StageConditions, "vicious_mockery_disadvantage", impose); err != nil {
		return c, rpgerr.Wrapf(err, "failed to add vicious mockery disadvantage for member %s", v.MemberID)
	}

	return c, v.end(ctx, "consumed")
}

// onTurnEnd ends the disadvantage at the end of the mocked creature's next
// turn, which is the first turn end it sees.
func (v *ViciousMockeryCondition) onTurnEnd(ctx context.Context, event dnd5eEvents.TurnEndEvent) error {
	if event.SubjectID != v.MemberID {
		return nil
	}
	return v.end(ctx, "expired")
}

// onCombatEnd ends the disadvantage with the fight.
func (v *ViciousMockeryCondition) onCombatEnd(ctx context.Context, event dnd5eEvents.CombatEndEvent) error {
	if event.SubjectID != v.MemberID {
		return nil
	}
	return v.end(ctx, "combat ended")
}

// end publishes the removal and detaches. The publish comes first so the
// removal is on the bus while this condition still owns the disadvantage, which
// is what an activation's effect collector reads.
func (v *ViciousMockeryCondition) end(ctx context.Context, reason string) error {
	if v.bus == nil {
		return nil
	}
	bus := v.bus
	if err := dnd5eEvents.ConditionRemovedTopic.On(bus).Publish(ctx, dnd5eEvents.ConditionRemovedEvent{
		MemberID:     v.MemberID,
		ConditionRef: refs.Conditions.ViciousMockery().String(),
		Reason:       reason,
	}); err != nil {
		return rpgerr.Wrapf(err, "failed to publish vicious mockery removal for member %s", v.MemberID)
	}
	return v.Remove(ctx, bus)
}
