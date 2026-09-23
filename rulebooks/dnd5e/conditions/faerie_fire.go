// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package conditions

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math"

	"github.com/KirkDiggler/rpg-toolkit/core"
	"github.com/KirkDiggler/rpg-toolkit/core/chain"
	"github.com/KirkDiggler/rpg-toolkit/events"
	"github.com/KirkDiggler/rpg-toolkit/rpgerr"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/combat"
	dnd5eEvents "github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/events"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/gamectx"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/refs"
)

// FaerieFireName is the displayed name of the protective spell.
const FaerieFireName = "Faerie Fire"

// FaerieFireConditionData is the persisted source-qualified FaerieFire condition.
type FaerieFireConditionData struct {
	Ref       *core.Ref `json:"ref"`
	MemberID  string    `json:"member_id"`
	SourceID  string    `json:"source_id"`
	SourceRef *core.Ref `json:"source_ref"`
}

// NewFaerieFireConditionInput names the recipient, caster, and canonical spell
// that created a FaerieFire condition.
type NewFaerieFireConditionInput struct {
	MemberID  string
	SourceID  string
	SourceRef *core.Ref
}

// FaerieFireCondition grants advantage to attackers who can see its holder while its caster
// maintains concentration. Concentration owns duration and teardown; the
// recipient keeps provenance so overlapping casters can end independently.
type FaerieFireCondition struct {
	MemberID  string
	SourceID  string
	SourceRef *core.Ref

	bus             events.EventBus
	restSubID       string
	subscriptionIDs []string
}

var (
	_ dnd5eEvents.ConditionBehavior        = (*FaerieFireCondition)(nil)
	_ dnd5eEvents.ConditionAddressProvider = (*FaerieFireCondition)(nil)
)

// NewFaerieFireCondition creates one source-qualified FaerieFire effect.
func NewFaerieFireCondition(input NewFaerieFireConditionInput) (*FaerieFireCondition, error) {
	if input.MemberID == "" {
		return nil, rpgerr.New(rpgerr.CodeInvalidArgument, "faerie_fire condition requires a recipient member")
	}
	if input.SourceID == "" {
		return nil, rpgerr.New(rpgerr.CodeInvalidArgument, "faerie_fire condition requires a caster source id")
	}
	if input.SourceRef == nil || input.SourceRef.String() != refs.Spells.FaerieFire().String() {
		return nil, rpgerr.New(rpgerr.CodeInvalidArgument, "faerie_fire condition source ref must be FaerieFire")
	}

	return &FaerieFireCondition{
		MemberID:  input.MemberID,
		SourceID:  input.SourceID,
		SourceRef: refs.Spells.FaerieFire(),
	}, nil
}

// Ref returns the canonical FaerieFire condition ref.
func (g *FaerieFireCondition) Ref() *core.Ref { return refs.Conditions.FaerieFire() }

// ConditionAddress derives this condition's exact identity from its
// persisted state.
func (g *FaerieFireCondition) ConditionAddress() dnd5eEvents.ConditionAddress {
	return dnd5eEvents.ConditionAddress{
		MemberID:     g.MemberID,
		ConditionRef: g.Ref().String(),
		SourceID:     g.SourceID,
	}
}

// IsApplied returns true if this condition is currently applied.
func (g *FaerieFireCondition) IsApplied() bool { return g.bus != nil }

// Apply subscribes to attack rolls and long-rest cleanup.
func (g *FaerieFireCondition) Apply(ctx context.Context, bus events.EventBus) error {
	if bus == nil {
		return rpgerr.New(rpgerr.CodeInvalidArgument, "event bus is required")
	}
	if g.IsApplied() {
		return rpgerr.New(rpgerr.CodeAlreadyExists, "faerie_fire condition already applied")
	}
	g.bus = bus

	acSub, err := dnd5eEvents.AttackChain.On(bus).SubscribeWithChain(ctx, g.onAttackChain)
	if err != nil {
		g.bus = nil
		return rpgerr.Wrap(err, "failed to subscribe Faerie Fire to attack chain")
	}
	g.subscriptionIDs = append(g.subscriptionIDs, acSub)

	restSubID, err := subscribeRemoveOnLongRest(ctx, bus, subscribeRemoveOnLongRestInput{
		Address: g.ConditionAddress(), Remove: g.Remove,
	})
	if err != nil {
		_ = g.Remove(ctx, bus)
		return rpgerr.Wrap(err, "failed to subscribe faerie_fire condition to long rest")
	}
	g.restSubID = restSubID

	return nil
}

// Remove unsubscribes this condition from every event it joined.
func (g *FaerieFireCondition) Remove(ctx context.Context, bus events.EventBus) error {
	if g.bus == nil {
		return nil
	}

	ids := g.subscriptionIDs
	if g.restSubID != "" {
		ids = append(ids, g.restSubID)
	}
	total := len(ids)
	var errs []error
	for _, subID := range ids {
		if err := bus.Unsubscribe(ctx, subID); err != nil {
			errs = append(errs, fmt.Errorf("unsubscribe %s: %w", subID, err))
		}
	}

	g.subscriptionIDs = nil
	g.restSubID = ""
	g.bus = nil

	if len(errs) > 0 {
		return fmt.Errorf("failed to unsubscribe %d/%d subscriptions: %w", len(errs), total, errors.Join(errs...))
	}
	return nil
}

// ToJSON serializes the source-qualified condition without runtime state.
func (g *FaerieFireCondition) ToJSON() (json.RawMessage, error) {
	return json.Marshal(FaerieFireConditionData{
		Ref:       refs.Conditions.FaerieFire(),
		MemberID:  g.MemberID,
		SourceID:  g.SourceID,
		SourceRef: refs.Spells.FaerieFire(),
	})
}

func (g *FaerieFireCondition) loadJSON(data json.RawMessage) error {
	var stored FaerieFireConditionData
	if err := json.Unmarshal(data, &stored); err != nil {
		return rpgerr.Wrap(err, "failed to unmarshal faerie_fire data")
	}
	loaded, err := NewFaerieFireCondition(NewFaerieFireConditionInput{
		MemberID: stored.MemberID, SourceID: stored.SourceID, SourceRef: stored.SourceRef,
	})
	if err != nil {
		return err
	}
	*g = *loaded
	return nil
}

// onAttackChain checks current sight on every attack. The condition is not
// consumed by attacks, and visibility loss does not remove it.
func (g *FaerieFireCondition) onAttackChain(
	ctx context.Context, event dnd5eEvents.AttackChainEvent, c chain.Chain[dnd5eEvents.AttackChainEvent],
) (chain.Chain[dnd5eEvents.AttackChainEvent], error) {
	if event.TargetID != g.MemberID {
		return c, nil
	}
	sight, ok := gamectx.Visibility(ctx)
	if !ok {
		return c, nil
	}
	// Faerie Fire imposes no attack-distance cap: the attack owns its range.
	// The supplied visibility provider still checks actual sight range and fog.
	visible, known := sight.SeesWithin(event.AttackerID, g.MemberID, math.MaxInt)
	if !known || !visible {
		return c, nil
	}
	err := c.Add(combat.StageConditions, "faerie_fire_"+g.SourceID, func(_ context.Context, e dnd5eEvents.AttackChainEvent) (dnd5eEvents.AttackChainEvent, error) {
		e.AdvantageSources = append(e.AdvantageSources, dnd5eEvents.AttackModifierSource{
			SourceRef: g.Ref(), SourceID: g.SourceID, Reason: FaerieFireName,
		})
		return e, nil
	})
	return c, err
}
