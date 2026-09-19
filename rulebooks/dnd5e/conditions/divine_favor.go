// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package conditions

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/KirkDiggler/rpg-toolkit/dice"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/damage"
	"slices"

	"github.com/KirkDiggler/rpg-toolkit/core"
	"github.com/KirkDiggler/rpg-toolkit/core/chain"
	"github.com/KirkDiggler/rpg-toolkit/events"
	"github.com/KirkDiggler/rpg-toolkit/rpgerr"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/combat"
	dnd5eEvents "github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/events"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/refs"
)

// DivineFavorName is the displayed name of the protective spell.
const DivineFavorName = "Divine Favor"

// DivineFavorConditionData is the persisted source-qualified DivineFavor condition.
type DivineFavorConditionData struct {
	Ref       *core.Ref `json:"ref"`
	MemberID  string    `json:"member_id"`
	SourceID  string    `json:"source_id"`
	SourceRef *core.Ref `json:"source_ref"`
}

// NewDivineFavorConditionInput names the recipient, caster, and canonical spell
// that created a DivineFavor condition.
type NewDivineFavorConditionInput struct {
	MemberID  string
	SourceID  string
	SourceRef *core.Ref
}

// DivineFavorCondition adds radiant weapon damage while its caster
// maintains concentration. Concentration owns duration and teardown; the
// recipient keeps provenance so overlapping casters can end independently.
type DivineFavorCondition struct {
	MemberID  string
	SourceID  string
	SourceRef *core.Ref

	roller          dice.Roller
	bus             events.EventBus
	restSubID       string
	subscriptionIDs []string
}

var (
	_ dnd5eEvents.ConditionBehavior        = (*DivineFavorCondition)(nil)
	_ dnd5eEvents.ConditionAddressProvider = (*DivineFavorCondition)(nil)
)

// NewDivineFavorCondition creates one source-qualified DivineFavor effect.
func NewDivineFavorCondition(input NewDivineFavorConditionInput) (*DivineFavorCondition, error) {
	if input.MemberID == "" {
		return nil, rpgerr.New(rpgerr.CodeInvalidArgument, "divine_favor condition requires a recipient member")
	}
	if input.SourceID != input.MemberID {
		return nil, rpgerr.New(rpgerr.CodeInvalidArgument, "divine_favor condition requires the caster as its recipient")
	}
	if input.SourceRef == nil || input.SourceRef.String() != refs.Spells.DivineFavor().String() {
		return nil, rpgerr.New(rpgerr.CodeInvalidArgument, "divine_favor condition source ref must be DivineFavor")
	}

	return &DivineFavorCondition{
		MemberID:  input.MemberID,
		SourceID:  input.SourceID,
		SourceRef: refs.Spells.DivineFavor(),
	}, nil
}

// Ref returns the canonical DivineFavor condition ref.
func (g *DivineFavorCondition) Ref() *core.Ref { return refs.Conditions.DivineFavor() }

// ConditionAddress derives this condition's exact identity from its
// persisted state.
func (g *DivineFavorCondition) ConditionAddress() dnd5eEvents.ConditionAddress {
	return dnd5eEvents.ConditionAddress{
		MemberID:     g.MemberID,
		ConditionRef: g.Ref().String(),
		SourceID:     g.SourceID,
	}
}

// IsApplied returns true if this condition is currently applied.
func (g *DivineFavorCondition) IsApplied() bool { return g.bus != nil }

// Apply subscribes to weapon damage and long-rest cleanup.
func (g *DivineFavorCondition) Apply(ctx context.Context, bus events.EventBus) error {
	if g.IsApplied() {
		return rpgerr.New(rpgerr.CodeAlreadyExists, "divine_favor condition already applied")
	}
	g.bus = bus

	acSub, err := dnd5eEvents.DamageChain.On(bus).SubscribeWithChain(ctx, g.onDamageChain)
	if err != nil {
		g.bus = nil
		return rpgerr.Wrap(err, "failed to subscribe Divine Favor to damage chain")
	}
	g.subscriptionIDs = append(g.subscriptionIDs, acSub)

	restSubID, err := subscribeRemoveOnLongRest(ctx, bus, subscribeRemoveOnLongRestInput{
		Address: g.ConditionAddress(), Remove: g.Remove,
	})
	if err != nil {
		_ = g.Remove(ctx, bus)
		return rpgerr.Wrap(err, "failed to subscribe divine_favor condition to long rest")
	}
	g.restSubID = restSubID

	return nil
}

// Remove unsubscribes this condition from every event it joined.
func (g *DivineFavorCondition) Remove(ctx context.Context, bus events.EventBus) error {
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
func (g *DivineFavorCondition) ToJSON() (json.RawMessage, error) {
	return json.Marshal(DivineFavorConditionData{
		Ref:       refs.Conditions.DivineFavor(),
		MemberID:  g.MemberID,
		SourceID:  g.SourceID,
		SourceRef: refs.Spells.DivineFavor(),
	})
}

func (g *DivineFavorCondition) loadJSON(data json.RawMessage) error {
	var stored DivineFavorConditionData
	if err := json.Unmarshal(data, &stored); err != nil {
		return rpgerr.Wrap(err, "failed to unmarshal divine_favor data")
	}
	loaded, err := NewDivineFavorCondition(NewDivineFavorConditionInput{
		MemberID: stored.MemberID, SourceID: stored.SourceID, SourceRef: stored.SourceRef,
	})
	if err != nil {
		return err
	}
	*g = *loaded
	return nil
}

// BindRoller uses the interaction's dice source, including after rehydration.
func (g *DivineFavorCondition) BindRoller(roller dice.Roller) {
	if roller != nil {
		g.roller = roller
	}
}

var _ RollerBinder = (*DivineFavorCondition)(nil)

func (g *DivineFavorCondition) onDamageChain(
	_ context.Context, event *dnd5eEvents.DamageChainEvent, c chain.Chain[*dnd5eEvents.DamageChainEvent],
) (chain.Chain[*dnd5eEvents.DamageChainEvent], error) {
	if event == nil || event.AttackerID != g.MemberID {
		return c, nil
	}
	modify := func(ctx context.Context, e *dnd5eEvents.DamageChainEvent) (*dnd5eEvents.DamageChainEvent, error) {
		if primaryWeaponComponent(e) == nil {
			return e, nil
		}
		// Repeated instances of the same spell never stack or roll extra dice.
		for _, component := range e.Components {
			if component.Roll.Source.Ref != nil && component.Roll.Source.Ref.String() == refs.Spells.DivineFavor().String() {
				return e, nil
			}
		}
		roller := g.roller
		if roller == nil {
			roller = dice.NewRoller()
		}
		count := 1
		if e.IsCritical {
			count = 2
		}
		faces, err := roller.RollN(ctx, count, 4)
		if err != nil {
			return nil, rpgerr.Wrap(err, "failed to roll Divine Favor damage")
		}
		total := 0
		for _, face := range faces {
			total += face
		}
		e.Components = append(e.Components, dnd5eEvents.DamageComponent{
			Source: dnd5eEvents.DamageSourceSpell,
			Roll: dnd5eEvents.RollComponent{
				Source: dnd5eEvents.RollSource{Ref: refs.Spells.DivineFavor(), Name: DivineFavorName, SourceID: g.SourceID},
				Dice:   &dnd5eEvents.DiceTrace{Notation: dice.SimplePool(count, 4, 0).Notation(), DieSize: 4, OriginalRolls: faces, FinalRolls: slices.Clone(faces), Subtotal: total},
			},
			DamageType: damage.Radiant, IsCritical: e.IsCritical,
		})
		return e, nil
	}
	if err := c.Add(combat.StageConditions, "divine_favor_"+g.SourceID, modify); err != nil {
		return c, rpgerr.Wrap(err, "failed to add Divine Favor damage")
	}
	return c, nil
}
