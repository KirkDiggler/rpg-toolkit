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
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/contributions"
	dnd5eEvents "github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/events"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/refs"
)

// FightingStyleArcheryData is the JSON structure for persisting archery condition state
type FightingStyleArcheryData struct {
	Ref      *core.Ref `json:"ref"`
	MemberID string    `json:"member_id"`
}

// FightingStyleArcheryCondition grants +2 to attack rolls with ranged weapons.
type FightingStyleArcheryCondition struct {
	CharacterID     string
	subscriptionIDs []string
	bus             events.EventBus
}

// Ensure FightingStyleArcheryCondition implements dnd5eEvents.ConditionBehavior
var _ dnd5eEvents.ConditionBehavior = (*FightingStyleArcheryCondition)(nil)

var _ contributions.ActionAssessor = (*FightingStyleArcheryCondition)(nil)

// AssessAction answers whether Archery's +2 bears on the framed attack. It
// reads the frame and this style's own owner; it changes nothing.
func (f *FightingStyleArcheryCondition) AssessAction(
	in *contributions.AssessActionInput,
) (*contributions.AssessActionOutput, error) {
	return f.rule().AssessAction(in)
}

func (f *FightingStyleArcheryCondition) rule() archeryRule {
	return archeryRule{owner: f.CharacterID}
}

// archeryRule holds only the facts Archery's predicate uses.
//
// It asks whether the attack is ranged, which is what execution has always
// read. RAW asks for a ranged weapon; the attack chain carries no weapon fact
// to tell a ranged spell attack apart, so this answer and the swing both
// treat any ranged attack roll alike.
type archeryRule struct {
	owner string
}

func (r archeryRule) AssessAction(in *contributions.AssessActionInput) (*contributions.AssessActionOutput, error) {
	frame, err := frameOf(in, "archery")
	if err != nil {
		return nil, err
	}
	if frame.Actor != r.owner {
		return assessed(contributions.DoesNotApply, "Archery affects only its holder's attacks"), nil
	}
	if roll, _ := frame.Action.Roll.Get(); roll != contributions.RollKindAttack {
		return assessed(contributions.DoesNotApply, "Archery affects only attack rolls"), nil
	}
	melee, known := frame.Action.Melee.Get()
	if !known {
		return assessed(contributions.Depends, "Depends on whether the attack is ranged"), nil
	}
	if melee {
		return assessed(contributions.DoesNotApply, "Archery adds only to ranged attacks"), nil
	}
	out := assessed(contributions.Applies, "The attack is ranged")
	out.Answer.Benefit = "+2 to the attack roll"
	return out, nil
}

// Ref returns the canonical ref this condition names itself by — the same ref
// its ToJSON embeds and its loader routes on.
func (f *FightingStyleArcheryCondition) Ref() *core.Ref {
	return refs.Conditions.FightingStyleArchery()
}

// NewFightingStyleArcheryCondition creates a new Archery fighting style condition.
func NewFightingStyleArcheryCondition(memberID string) *FightingStyleArcheryCondition {
	return &FightingStyleArcheryCondition{
		CharacterID: memberID,
	}
}

// IsApplied returns true if this condition is currently applied.
func (f *FightingStyleArcheryCondition) IsApplied() bool {
	return f.bus != nil
}

// Apply subscribes this condition to attack chain events.
func (f *FightingStyleArcheryCondition) Apply(ctx context.Context, bus events.EventBus) error {
	if f.IsApplied() {
		return rpgerr.New(rpgerr.CodeAlreadyExists, "archery fighting style already applied")
	}
	f.bus = bus

	// Subscribe to AttackChain to add +2 bonus for ranged attacks
	attackChain := dnd5eEvents.AttackChain.On(bus)
	subID, err := attackChain.SubscribeWithChain(ctx, f.onAttackChain)
	if err != nil {
		return rpgerr.Wrap(err, "failed to subscribe to attack chain")
	}
	f.subscriptionIDs = append(f.subscriptionIDs, subID)

	return nil
}

// Remove unsubscribes this condition from events.
func (f *FightingStyleArcheryCondition) Remove(ctx context.Context, bus events.EventBus) error {
	if f.bus == nil {
		return nil
	}

	total := len(f.subscriptionIDs)
	var errs []error
	for _, subID := range f.subscriptionIDs {
		if err := bus.Unsubscribe(ctx, subID); err != nil {
			errs = append(errs, fmt.Errorf("unsubscribe %s: %w", subID, err))
		}
	}

	f.subscriptionIDs = nil
	f.bus = nil

	if len(errs) > 0 {
		return fmt.Errorf("failed to unsubscribe %d/%d subscriptions: %w", len(errs), total, errors.Join(errs...))
	}
	return nil
}

// ToJSON converts the condition to JSON for persistence.
func (f *FightingStyleArcheryCondition) ToJSON() (json.RawMessage, error) {
	data := FightingStyleArcheryData{
		Ref:      refs.Conditions.FightingStyleArchery(),
		MemberID: f.CharacterID,
	}
	return json.Marshal(data)
}

// loadJSON loads archery condition state from JSON.
func (f *FightingStyleArcheryCondition) loadJSON(data json.RawMessage) error {
	var archeryData FightingStyleArcheryData
	if err := json.Unmarshal(data, &archeryData); err != nil {
		return rpgerr.Wrap(err, "failed to unmarshal archery data")
	}

	f.CharacterID = archeryData.MemberID
	return nil
}

// onAttackChain adds +2 to the attack roll when archeryRule applies to the
// attack — the same rule information asks.
func (f *FightingStyleArcheryCondition) onAttackChain(
	_ context.Context,
	event dnd5eEvents.AttackChainEvent,
	c chain.Chain[dnd5eEvents.AttackChainEvent],
) (chain.Chain[dnd5eEvents.AttackChainEvent], error) {
	executed, err := executeRule(&executeRuleInput{Name: "archery", Rule: f.rule(), Frame: event.Frame})
	if err != nil {
		return c, err
	}
	if executed.Answer.Decision.Applicability != contributions.Applies {
		return c, nil
	}

	// Add +2 to attack bonus at StageFeatures
	modifyAttack := func(_ context.Context, e dnd5eEvents.AttackChainEvent) (dnd5eEvents.AttackChainEvent, error) {
		e.AttackBonus += 2
		return e, nil
	}

	if err := c.Add(combat.StageFeatures, "archery", modifyAttack); err != nil {
		return c, rpgerr.Wrapf(err, "failed to apply archery bonus for character %s", f.CharacterID)
	}

	return c, nil
}
