// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package conditions

import (
	"context"
	"encoding/json"

	"github.com/KirkDiggler/rpg-toolkit/core"
	"github.com/KirkDiggler/rpg-toolkit/events"
	"github.com/KirkDiggler/rpg-toolkit/rpgerr"
	dnd5eEvents "github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/events"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/refs"
)

// SanctuaryName is the display name for a creature warded by Sanctuary.
const SanctuaryName = "Sanctuary"

// SanctuaryConditionData is the persisted source-qualified Sanctuary ward.
type SanctuaryConditionData struct {
	Ref       *core.Ref `json:"ref"`
	MemberID  string    `json:"member_id"`
	SourceID  string    `json:"source_id"`
	SourceRef *core.Ref `json:"source_ref"`
}

// NewSanctuaryConditionInput names the warded creature, the caster, and the
// canonical spell that created a Sanctuary condition.
type NewSanctuaryConditionInput struct {
	MemberID  string
	SourceID  string
	SourceRef *core.Ref
}

// SanctuaryCondition marks its holder as warded. It stores no die, offers
// nothing and subscribes to no roll event — [BlessedCondition]'s shape, not
// [GuidedCondition]'s: what Sanctuary protects against happens to a
// DIFFERENT creature (whoever targets the ward) at a point this condition
// itself never observes, so resolution asks "is my target warded, and by
// whom" directly, the same way it already reads other passive markers,
// rather than this condition publishing an offer nobody but resolution would
// answer. See docs/ideas/cleric/plan.md's Sanctuary section for why the ward
// save, the retarget, and the holder's own self-break belong in resolution
// rather than as event subscriptions here.
//
// # It ends like Bless, Guidance and Resistance
//
// Concentration, up to one minute — the same duration category as
// [BlessedCondition], [GuidedCondition] and [ResistanceCondition]. This
// condition subscribes to long-rest cleanup and relies on the existing
// concentration teardown for everything else.
type SanctuaryCondition struct {
	MemberID  string
	SourceID  string
	SourceRef *core.Ref

	bus       events.EventBus
	restSubID string
}

var (
	_ dnd5eEvents.ConditionBehavior        = (*SanctuaryCondition)(nil)
	_ dnd5eEvents.ConditionAddressProvider = (*SanctuaryCondition)(nil)
)

// NewSanctuaryCondition creates one source-qualified Sanctuary ward.
func NewSanctuaryCondition(input NewSanctuaryConditionInput) (*SanctuaryCondition, error) {
	if input.MemberID == "" {
		return nil, rpgerr.New(rpgerr.CodeInvalidArgument, "sanctuary condition requires a warded member")
	}
	if input.SourceID == "" {
		return nil, rpgerr.New(rpgerr.CodeInvalidArgument, "sanctuary condition requires a caster source id")
	}
	if input.SourceRef == nil || input.SourceRef.String() != refs.Spells.Sanctuary().String() {
		return nil, rpgerr.New(rpgerr.CodeInvalidArgument, "sanctuary condition source ref must be Sanctuary")
	}

	return &SanctuaryCondition{
		MemberID:  input.MemberID,
		SourceID:  input.SourceID,
		SourceRef: refs.Spells.Sanctuary(),
	}, nil
}

// Ref returns the canonical Sanctuary condition ref.
func (s *SanctuaryCondition) Ref() *core.Ref { return refs.Conditions.Sanctuary() }

// ConditionAddress derives this condition's exact identity from its
// persisted state.
func (s *SanctuaryCondition) ConditionAddress() dnd5eEvents.ConditionAddress {
	return dnd5eEvents.ConditionAddress{
		MemberID:     s.MemberID,
		ConditionRef: s.Ref().String(),
		SourceID:     s.SourceID,
	}
}

// IsApplied reports whether the condition has joined an interaction bus.
func (s *SanctuaryCondition) IsApplied() bool { return s.bus != nil }

// Apply marks the condition active and subscribes only to long-rest cleanup.
// Sanctuary contributes nothing to any roll and installs no roll subscriber.
func (s *SanctuaryCondition) Apply(ctx context.Context, bus events.EventBus) error {
	if bus == nil {
		return rpgerr.New(rpgerr.CodeInvalidArgument, "event bus is required")
	}
	if s.IsApplied() {
		return rpgerr.New(rpgerr.CodeAlreadyExists, "sanctuary condition already applied")
	}
	s.bus = bus
	restSubID, err := subscribeRemoveOnLongRest(ctx, bus, subscribeRemoveOnLongRestInput{
		Address: s.ConditionAddress(), Remove: s.Remove,
	})
	if err != nil {
		s.bus = nil
		return rpgerr.Wrap(err, "failed to subscribe sanctuary condition to long rest")
	}
	s.restSubID = restSubID
	return nil
}

// Remove marks the condition inactive and revokes its cleanup subscription.
func (s *SanctuaryCondition) Remove(ctx context.Context, bus events.EventBus) error {
	if s.restSubID != "" {
		if err := bus.Unsubscribe(ctx, s.restSubID); err != nil {
			return rpgerr.Wrap(err, "failed to unsubscribe sanctuary condition")
		}
	}
	s.restSubID = ""
	s.bus = nil
	return nil
}

// ToJSON serializes the source-qualified condition without runtime state.
func (s *SanctuaryCondition) ToJSON() (json.RawMessage, error) {
	return json.Marshal(SanctuaryConditionData{
		Ref:       refs.Conditions.Sanctuary(),
		MemberID:  s.MemberID,
		SourceID:  s.SourceID,
		SourceRef: refs.Spells.Sanctuary(),
	})
}

func (s *SanctuaryCondition) loadJSON(data json.RawMessage) error {
	var stored SanctuaryConditionData
	if err := json.Unmarshal(data, &stored); err != nil {
		return rpgerr.Wrap(err, "failed to unmarshal sanctuary data")
	}
	s.MemberID = stored.MemberID
	s.SourceID = stored.SourceID
	s.SourceRef = refs.Spells.Sanctuary()
	return nil
}
