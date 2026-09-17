// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package conditions

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/KirkDiggler/rpg-toolkit/core"
	"github.com/KirkDiggler/rpg-toolkit/events"
	"github.com/KirkDiggler/rpg-toolkit/rpgerr"
	dnd5eEvents "github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/events"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/refs"
)

// SanctuaryImmuneName is the display name for a creature immune to one
// caster's Sanctuary after succeeding a ward save against it.
const SanctuaryImmuneName = "Sanctuary Immune"

// SanctuaryImmuneConditionData is the persisted source-qualified immunity.
type SanctuaryImmuneConditionData struct {
	Ref       *core.Ref `json:"ref"`
	MemberID  string    `json:"member_id"`
	SourceID  string    `json:"source_id"`
	SourceRef *core.Ref `json:"source_ref"`
}

// NewSanctuaryImmuneConditionInput names the attacker who earned the
// immunity, the caster it blocks, and the canonical Sanctuary spell ref that
// justifies the block.
type NewSanctuaryImmuneConditionInput struct {
	MemberID  string
	SourceID  string
	SourceRef *core.Ref
}

// SanctuaryImmuneCondition marks an ATTACKER as immune to one specific
// caster's Sanctuary after succeeding a ward save against it — RAW's "immune
// to your sanctuary spells", not a blanket immunity, so it is source-qualified
// exactly like [BlessedCondition] and its siblings.
//
// # It ends with the fight or a rest, not with a clock
//
// RAW is 24 hours. There is no real-time clock anywhere in this rulebook
// (docs/ideas/cleric/plan.md's Sanctuary section confirmed it: even
// concentration is turn-counted, not minute-timed), so the divergence is
// named rather than approximated — [InspiredCondition]'s own precedent for
// exactly this situation. The immunity lasts until combat ends or the holder
// rests, whichever comes first.
type SanctuaryImmuneCondition struct {
	MemberID  string
	SourceID  string
	SourceRef *core.Ref

	bus             events.EventBus
	subscriptionIDs []string
}

var (
	_ dnd5eEvents.ConditionBehavior        = (*SanctuaryImmuneCondition)(nil)
	_ dnd5eEvents.ConditionAddressProvider = (*SanctuaryImmuneCondition)(nil)
)

// NewSanctuaryImmuneCondition creates one source-qualified immunity.
func NewSanctuaryImmuneCondition(input NewSanctuaryImmuneConditionInput) (*SanctuaryImmuneCondition, error) {
	if input.MemberID == "" {
		return nil, rpgerr.New(rpgerr.CodeInvalidArgument, "sanctuary immune condition requires an attacker member")
	}
	if input.SourceID == "" {
		return nil, rpgerr.New(rpgerr.CodeInvalidArgument,
			"sanctuary immune condition requires the id of the caster it blocks")
	}
	if input.SourceRef == nil || input.SourceRef.String() != refs.Spells.Sanctuary().String() {
		return nil, rpgerr.New(rpgerr.CodeInvalidArgument, "sanctuary immune condition source ref must be Sanctuary")
	}

	return &SanctuaryImmuneCondition{
		MemberID:  input.MemberID,
		SourceID:  input.SourceID,
		SourceRef: refs.Spells.Sanctuary(),
	}, nil
}

// Ref returns the canonical SanctuaryImmune condition ref.
func (s *SanctuaryImmuneCondition) Ref() *core.Ref { return refs.Conditions.SanctuaryImmune() }

// ConditionAddress derives this condition's exact identity from its
// persisted state.
func (s *SanctuaryImmuneCondition) ConditionAddress() dnd5eEvents.ConditionAddress {
	return dnd5eEvents.ConditionAddress{
		MemberID:     s.MemberID,
		ConditionRef: s.Ref().String(),
		SourceID:     s.SourceID,
	}
}

// IsApplied returns true if this condition is currently applied.
func (s *SanctuaryImmuneCondition) IsApplied() bool { return s.bus != nil }

// Apply subscribes the immunity to the two boundaries that end it: combat end
// and any rest.
func (s *SanctuaryImmuneCondition) Apply(ctx context.Context, bus events.EventBus) error {
	if bus == nil {
		return rpgerr.New(rpgerr.CodeInvalidArgument, "event bus is required")
	}
	if s.IsApplied() {
		return rpgerr.New(rpgerr.CodeAlreadyExists, "sanctuary immune condition already applied")
	}
	s.bus = bus

	combatEnds := dnd5eEvents.CombatEndTopic.On(bus)
	combatSub, err := combatEnds.Subscribe(ctx, s.onCombatEnd)
	if err != nil {
		s.bus = nil
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
func (s *SanctuaryImmuneCondition) Remove(ctx context.Context, bus events.EventBus) error {
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
func (s *SanctuaryImmuneCondition) ToJSON() (json.RawMessage, error) {
	return json.Marshal(SanctuaryImmuneConditionData{
		Ref:       refs.Conditions.SanctuaryImmune(),
		MemberID:  s.MemberID,
		SourceID:  s.SourceID,
		SourceRef: refs.Spells.Sanctuary(),
	})
}

func (s *SanctuaryImmuneCondition) loadJSON(data json.RawMessage) error {
	var stored SanctuaryImmuneConditionData
	if err := json.Unmarshal(data, &stored); err != nil {
		return rpgerr.Wrap(err, "failed to unmarshal sanctuary immune data")
	}
	s.MemberID = stored.MemberID
	s.SourceID = stored.SourceID
	s.SourceRef = refs.Spells.Sanctuary()
	return nil
}

// onCombatEnd ends the immunity with the fight, [InspiredCondition]'s own
// divergence from a real-time RAW duration.
func (s *SanctuaryImmuneCondition) onCombatEnd(ctx context.Context, event dnd5eEvents.CombatEndEvent) error {
	if event.SubjectID != s.MemberID {
		return nil
	}
	return s.end(ctx, "combat ended")
}

// onRest ends the immunity on any rest, short or long, for the holder.
func (s *SanctuaryImmuneCondition) onRest(ctx context.Context, event dnd5eEvents.RestEvent) error {
	if event.CharacterID != s.MemberID {
		return nil
	}
	return s.end(ctx, "rest")
}

// end publishes the removal and detaches.
func (s *SanctuaryImmuneCondition) end(ctx context.Context, reason string) error {
	if s.bus == nil {
		return nil
	}
	bus := s.bus
	if err := dnd5eEvents.ConditionRemovedTopic.On(bus).Publish(ctx, dnd5eEvents.ConditionRemovedEvent{
		MemberID:     s.MemberID,
		ConditionRef: refs.Conditions.SanctuaryImmune().String(),
		SourceID:     s.SourceID,
		Reason:       reason,
	}); err != nil {
		return rpgerr.Wrapf(err, "failed to publish sanctuary immune removal for member %s", s.MemberID)
	}
	return s.Remove(ctx, bus)
}
