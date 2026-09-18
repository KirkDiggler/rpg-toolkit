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

// GuidingBoltName labels the light that grants advantage to the next attack.
const GuidingBoltName = "Guiding Bolt"

// GuidingBoltTurnEnds counts the casting turn and the caster's next turn.
// The supported action cast occurs on the caster's turn.
const GuidingBoltTurnEnds = 2

// GuidingBoltConditionData is the persisted source-qualified light.
type GuidingBoltConditionData struct {
	Ref          *core.Ref `json:"ref"`
	MemberID     string    `json:"member_id"`
	SourceID     string    `json:"source_id"`
	SourceRef    *core.Ref `json:"source_ref"`
	TurnEndsLeft int       `json:"turn_ends_left"`
}

// NewGuidingBoltConditionInput names the recipient and the caster who
// applied the light. SourceID owns its expiry clock and attribution.
type NewGuidingBoltConditionInput struct {
	MemberID  string
	SourceID  string
	SourceRef *core.Ref
}

// GuidingBoltCondition grants advantage on the next attack roll against its
// holder. It ends when consumed or at the end of the caster's next turn and
// does not require concentration.
type GuidingBoltCondition struct {
	MemberID  string
	SourceID  string
	SourceRef *core.Ref

	// TurnEndsLeft is how many of the CASTER's turn ends remain —
	// [BladeWardCondition.TurnEndsLeft]'s own reasoning: counted here rather
	// than derived from a round, because a TurnEndEvent may carry no round
	// at all.
	TurnEndsLeft int

	bus             events.EventBus
	subscriptionIDs []string
}

var (
	_ dnd5eEvents.ConditionBehavior        = (*GuidingBoltCondition)(nil)
	_ dnd5eEvents.ConditionAddressProvider = (*GuidingBoltCondition)(nil)
)

// NewGuidingBoltCondition creates one source-qualified light, lasting
// GuidingBoltTurnEnds of the caster's turn ends.
func NewGuidingBoltCondition(input NewGuidingBoltConditionInput) (*GuidingBoltCondition, error) {
	if input.MemberID == "" {
		return nil, rpgerr.New(rpgerr.CodeInvalidArgument, "guiding bolt condition requires a recipient member")
	}
	if input.SourceID == "" {
		return nil, rpgerr.New(rpgerr.CodeInvalidArgument,
			"guiding bolt condition requires the id of the originating caster")
	}
	if input.SourceRef == nil || input.SourceRef.String() != refs.Spells.GuidingBolt().String() {
		return nil, rpgerr.New(rpgerr.CodeInvalidArgument, "guiding bolt condition source ref must be Guiding Bolt")
	}

	return &GuidingBoltCondition{
		MemberID:     input.MemberID,
		SourceID:     input.SourceID,
		SourceRef:    refs.Spells.GuidingBolt(),
		TurnEndsLeft: GuidingBoltTurnEnds,
	}, nil
}

// Ref returns the canonical GuidingBolt condition ref.
func (s *GuidingBoltCondition) Ref() *core.Ref { return refs.Conditions.GuidingBolt() }

// ConditionAddress derives this condition's exact identity from its
// persisted state.
func (s *GuidingBoltCondition) ConditionAddress() dnd5eEvents.ConditionAddress {
	return dnd5eEvents.ConditionAddress{
		MemberID:     s.MemberID,
		ConditionRef: s.Ref().String(),
		SourceID:     s.SourceID,
	}
}

// IsApplied returns true if this condition is currently applied.
func (s *GuidingBoltCondition) IsApplied() bool { return s.bus != nil }

// Apply subscribes the light to attacks, caster turn ends, combat end, and rest.
func (s *GuidingBoltCondition) Apply(ctx context.Context, bus events.EventBus) error {
	if bus == nil {
		return rpgerr.New(rpgerr.CodeInvalidArgument, "event bus is required")
	}
	if s.IsApplied() {
		return rpgerr.New(rpgerr.CodeAlreadyExists, "guiding bolt condition already applied")
	}
	s.bus = bus
	attackSub, err := dnd5eEvents.AttackChain.On(bus).SubscribeWithChain(ctx, s.onAttackChain)
	if err != nil {
		s.bus = nil
		return err
	}
	s.subscriptionIDs = append(s.subscriptionIDs, attackSub)

	turnEnds := dnd5eEvents.TurnEndTopic.On(bus)
	turnSub, err := turnEnds.Subscribe(ctx, s.onTurnEnd)
	if err != nil {
		_ = s.Remove(ctx, bus)
		return rpgerr.Wrap(err, "failed to subscribe to turn end topic")
	}
	s.subscriptionIDs = append(s.subscriptionIDs, turnSub)

	combatEnds := dnd5eEvents.CombatEndTopic.On(bus)
	combatSub, err := combatEnds.Subscribe(ctx, s.onCombatEnd)
	if err != nil {
		_ = s.Remove(ctx, bus)
		return rpgerr.Wrap(err, "failed to subscribe to combat end topic")
	}
	s.subscriptionIDs = append(s.subscriptionIDs, combatSub)

	rests := dnd5eEvents.RestTopic.On(bus)
	restSub, err := rests.Subscribe(ctx, s.onRest)
	if err != nil {
		_ = s.Remove(ctx, bus)
		return rpgerr.Wrap(err, "failed to subscribe to rest topic")
	}
	s.subscriptionIDs = append(s.subscriptionIDs, restSub)

	return nil
}

// Remove unsubscribes this condition from every event it joined.
func (s *GuidingBoltCondition) Remove(ctx context.Context, bus events.EventBus) error {
	if s.bus == nil {
		return nil
	}

	total := len(s.subscriptionIDs)
	var errs []error
	for _, subID := range s.subscriptionIDs {
		if err := bus.Unsubscribe(ctx, subID); err != nil {
			errs = append(errs, fmt.Errorf("unsubscribe %s: %w", subID, err))
		}
	}

	s.subscriptionIDs = nil
	s.bus = nil

	if len(errs) > 0 {
		return fmt.Errorf("failed to unsubscribe %d/%d subscriptions: %w", len(errs), total, errors.Join(errs...))
	}
	return nil
}

// ToJSON serializes the source-qualified condition without runtime state.
func (s *GuidingBoltCondition) ToJSON() (json.RawMessage, error) {
	return json.Marshal(GuidingBoltConditionData{
		Ref:          refs.Conditions.GuidingBolt(),
		MemberID:     s.MemberID,
		SourceID:     s.SourceID,
		SourceRef:    refs.Spells.GuidingBolt(),
		TurnEndsLeft: s.TurnEndsLeft,
	})
}

func (s *GuidingBoltCondition) loadJSON(data json.RawMessage) error {
	var stored GuidingBoltConditionData
	if err := json.Unmarshal(data, &stored); err != nil {
		return rpgerr.Wrap(err, "failed to unmarshal guiding bolt data")
	}
	s.MemberID = stored.MemberID
	s.SourceID = stored.SourceID
	s.SourceRef = refs.Spells.GuidingBolt()
	s.TurnEndsLeft = stored.TurnEndsLeft
	return nil
}

// onTurnEnd spends one of the caster's turn ends and ends the light
// when the count runs out. Other members' turns do not own this clock.
func (s *GuidingBoltCondition) onTurnEnd(ctx context.Context, event dnd5eEvents.TurnEndEvent) error {
	if event.SubjectID != s.SourceID {
		return nil
	}
	s.TurnEndsLeft--
	if s.TurnEndsLeft > 0 {
		return dnd5eEvents.ConditionStateChangedTopic.On(s.bus).Publish(ctx, dnd5eEvents.ConditionStateChangedEvent{
			MemberID: s.MemberID, ConditionRef: s.Ref(),
		})
	}
	return s.end(ctx, "expired")
}

// onCombatEnd ends the light with the fight, in case combat ends before
// the turn-end count runs out.
func (s *GuidingBoltCondition) onCombatEnd(ctx context.Context, event dnd5eEvents.CombatEndEvent) error {
	if event.SubjectID != s.MemberID {
		return nil
	}
	return s.end(ctx, "combat ended")
}

// onRest ends the light on any rest, short or long, for the holder.
func (s *GuidingBoltCondition) onRest(ctx context.Context, event dnd5eEvents.RestEvent) error {
	if event.CharacterID != s.MemberID {
		return nil
	}
	return s.end(ctx, "rest")
}

// end publishes the removal and detaches.
func (s *GuidingBoltCondition) end(ctx context.Context, reason string) error {
	if s.bus == nil {
		return nil
	}
	bus := s.bus
	if err := dnd5eEvents.ConditionRemovedTopic.On(bus).Publish(ctx, dnd5eEvents.ConditionRemovedEvent{
		MemberID:     s.MemberID,
		ConditionRef: refs.Conditions.GuidingBolt().String(),
		SourceID:     s.SourceID,
		Reason:       reason,
	}); err != nil {
		return rpgerr.Wrapf(err, "failed to publish guiding bolt removal for member %s", s.MemberID)
	}
	return s.Remove(ctx, bus)
}

// onAttackChain consumes the target's light only when an attack roll against it
// is actually assembled. Other targets and saving throws cannot consume it.
func (s *GuidingBoltCondition) onAttackChain(ctx context.Context, event dnd5eEvents.AttackChainEvent, c chain.Chain[dnd5eEvents.AttackChainEvent]) (chain.Chain[dnd5eEvents.AttackChainEvent], error) {
	if event.TargetID != s.MemberID {
		return c, nil
	}
	err := c.Add(combat.StageConditions, "guiding_bolt_advantage_"+s.SourceID, func(_ context.Context, e dnd5eEvents.AttackChainEvent) (dnd5eEvents.AttackChainEvent, error) {
		e.AdvantageSources = append(e.AdvantageSources, dnd5eEvents.AttackModifierSource{SourceRef: s.Ref(), SourceID: s.SourceID, Reason: GuidingBoltName})
		return e, nil
	})
	if err != nil {
		return c, err
	}
	return c, s.end(ctx, "consumed")
}
