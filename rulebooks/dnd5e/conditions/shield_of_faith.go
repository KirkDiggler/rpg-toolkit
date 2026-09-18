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

// ShieldOfFaithName is the displayed name of the protective spell.
const ShieldOfFaithName = "Shield of Faith"

// ShieldOfFaithACBonus is the spell's fixed armor class bonus.
const ShieldOfFaithACBonus = 2

// ShieldOfFaithConditionData is the persisted source-qualified ShieldOfFaith condition.
type ShieldOfFaithConditionData struct {
	Ref       *core.Ref `json:"ref"`
	MemberID  string    `json:"member_id"`
	SourceID  string    `json:"source_id"`
	SourceRef *core.Ref `json:"source_ref"`
}

// NewShieldOfFaithConditionInput names the recipient, caster, and canonical spell
// that created a ShieldOfFaith condition.
type NewShieldOfFaithConditionInput struct {
	MemberID  string
	SourceID  string
	SourceRef *core.Ref
}

// ShieldOfFaithCondition grants a source-qualified +2 AC while its caster
// maintains concentration. Concentration owns duration and teardown; the
// recipient keeps provenance so overlapping casters can end independently.
type ShieldOfFaithCondition struct {
	MemberID  string
	SourceID  string
	SourceRef *core.Ref

	bus             events.EventBus
	restSubID       string
	subscriptionIDs []string
}

var (
	_ dnd5eEvents.ConditionBehavior        = (*ShieldOfFaithCondition)(nil)
	_ dnd5eEvents.ConditionAddressProvider = (*ShieldOfFaithCondition)(nil)
)

// NewShieldOfFaithCondition creates one source-qualified ShieldOfFaith effect.
func NewShieldOfFaithCondition(input NewShieldOfFaithConditionInput) (*ShieldOfFaithCondition, error) {
	if input.MemberID == "" {
		return nil, rpgerr.New(rpgerr.CodeInvalidArgument, "shield_of_faith condition requires a recipient member")
	}
	if input.SourceID == "" {
		return nil, rpgerr.New(rpgerr.CodeInvalidArgument, "shield_of_faith condition requires a caster source id")
	}
	if input.SourceRef == nil || input.SourceRef.String() != refs.Spells.ShieldOfFaith().String() {
		return nil, rpgerr.New(rpgerr.CodeInvalidArgument, "shield_of_faith condition source ref must be ShieldOfFaith")
	}

	return &ShieldOfFaithCondition{
		MemberID:  input.MemberID,
		SourceID:  input.SourceID,
		SourceRef: refs.Spells.ShieldOfFaith(),
	}, nil
}

// Ref returns the canonical ShieldOfFaith condition ref.
func (g *ShieldOfFaithCondition) Ref() *core.Ref { return refs.Conditions.ShieldOfFaith() }

// ConditionAddress derives this condition's exact identity from its
// persisted state.
func (g *ShieldOfFaithCondition) ConditionAddress() dnd5eEvents.ConditionAddress {
	return dnd5eEvents.ConditionAddress{
		MemberID:     g.MemberID,
		ConditionRef: g.Ref().String(),
		SourceID:     g.SourceID,
	}
}

// IsApplied returns true if this condition is currently applied.
func (g *ShieldOfFaithCondition) IsApplied() bool { return g.bus != nil }

// Apply subscribes to AC calculation and long-rest cleanup.
func (g *ShieldOfFaithCondition) Apply(ctx context.Context, bus events.EventBus) error {
	if g.IsApplied() {
		return rpgerr.New(rpgerr.CodeAlreadyExists, "shield_of_faith condition already applied")
	}
	g.bus = bus

	acSub, err := combat.ACChain.On(bus).SubscribeWithChain(ctx, g.onACChain)
	if err != nil {
		g.bus = nil
		return rpgerr.Wrap(err, "failed to subscribe Shield of Faith to AC chain")
	}
	g.subscriptionIDs = append(g.subscriptionIDs, acSub)

	restSubID, err := subscribeRemoveOnLongRest(ctx, bus, subscribeRemoveOnLongRestInput{
		Address: g.ConditionAddress(), Remove: g.Remove,
	})
	if err != nil {
		_ = g.Remove(ctx, bus)
		return rpgerr.Wrap(err, "failed to subscribe shield_of_faith condition to long rest")
	}
	g.restSubID = restSubID

	return nil
}

// Remove unsubscribes this condition from every event it joined.
func (g *ShieldOfFaithCondition) Remove(ctx context.Context, bus events.EventBus) error {
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
func (g *ShieldOfFaithCondition) ToJSON() (json.RawMessage, error) {
	return json.Marshal(ShieldOfFaithConditionData{
		Ref:       refs.Conditions.ShieldOfFaith(),
		MemberID:  g.MemberID,
		SourceID:  g.SourceID,
		SourceRef: refs.Spells.ShieldOfFaith(),
	})
}

func (g *ShieldOfFaithCondition) loadJSON(data json.RawMessage) error {
	var stored ShieldOfFaithConditionData
	if err := json.Unmarshal(data, &stored); err != nil {
		return rpgerr.Wrap(err, "failed to unmarshal shield_of_faith data")
	}
	loaded, err := NewShieldOfFaithCondition(NewShieldOfFaithConditionInput{
		MemberID: stored.MemberID, SourceID: stored.SourceID, SourceRef: stored.SourceRef,
	})
	if err != nil {
		return err
	}
	*g = *loaded
	return nil
}

func (g *ShieldOfFaithCondition) onACChain(
	_ context.Context, event *combat.ACChainEvent, c chain.Chain[*combat.ACChainEvent],
) (chain.Chain[*combat.ACChainEvent], error) {
	if event == nil || event.CharacterID != g.MemberID {
		return c, nil
	}
	modify := func(_ context.Context, e *combat.ACChainEvent) (*combat.ACChainEvent, error) {
		// Equal-potency castings of the same spell do not stack. Each caster
		// remains subscribed, so removing either leaves the surviving protection.
		for _, component := range e.Breakdown.Components {
			if component.Source != nil && component.Source.String() == refs.Spells.ShieldOfFaith().String() {
				return e, nil
			}
		}
		e.Breakdown.AddComponent(combat.ACComponent{
			Type: combat.ACSourceSpell, Source: refs.Spells.ShieldOfFaith(), Value: ShieldOfFaithACBonus,
		})
		return e, nil
	}
	if err := c.Add(combat.StageConditions, "shield_of_faith_"+g.SourceID, modify); err != nil {
		return c, rpgerr.Wrap(err, "failed to add Shield of Faith AC bonus")
	}
	return c, nil
}
