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

// InFogName labels membership in one active Fog Cloud volume.
const InFogName = "In Fog"

// InFogConditionData persists a membership qualified by its area identity.
// The area owns its lifetime; this condition has no independent timer.
type InFogConditionData struct {
	Ref       *core.Ref `json:"ref"`
	MemberID  string    `json:"member_id"`
	SourceID  string    `json:"source_id"`
	SourceRef *core.Ref `json:"source_ref"`
}

// NewInFogConditionInput names the recipient and the exact cloud area.
// SourceID is an area ID, not a caster ID, so overlapping areas stay distinct.
type NewInFogConditionInput struct {
	MemberID  string
	SourceID  string
	SourceRef *core.Ref
}

// InFogCondition records membership only. Geometry determines visibility;
// this condition never adds Blinded or an attack-roll modifier. Resolution
// reconciles it from current positions and active areas, including on reload.
type InFogCondition struct {
	MemberID  string
	SourceID  string
	SourceRef *core.Ref
	bus       events.EventBus
}

var (
	_ dnd5eEvents.ConditionBehavior        = (*InFogCondition)(nil)
	_ dnd5eEvents.ConditionAddressProvider = (*InFogCondition)(nil)
)

// NewInFogCondition constructs membership in exactly one Fog Cloud area.
func NewInFogCondition(in NewInFogConditionInput) (*InFogCondition, error) {
	if in.MemberID == "" || in.SourceID == "" {
		return nil, rpgerr.New(rpgerr.CodeInvalidArgument, "in fog requires member and area identity")
	}
	if in.SourceRef == nil || in.SourceRef.String() != refs.Spells.FogCloud().String() {
		return nil, rpgerr.New(rpgerr.CodeInvalidArgument, "in fog source must be Fog Cloud")
	}
	return &InFogCondition{MemberID: in.MemberID, SourceID: in.SourceID, SourceRef: refs.Spells.FogCloud()}, nil
}

// Ref returns the membership condition's canonical reference.
func (f *InFogCondition) Ref() *core.Ref { return refs.Conditions.InFog() }

// ConditionAddress keeps overlapping cloud memberships independently removable.
func (f *InFogCondition) ConditionAddress() dnd5eEvents.ConditionAddress {
	return dnd5eEvents.ConditionAddress{MemberID: f.MemberID, ConditionRef: f.Ref().String(), SourceID: f.SourceID}
}

// IsApplied reports whether this membership is attached to the current fold.
func (f *InFogCondition) IsApplied() bool { return f.bus != nil }

// Apply attaches membership without subscribing any combat modifiers.
func (f *InFogCondition) Apply(_ context.Context, bus events.EventBus) error {
	if bus == nil {
		return rpgerr.New(rpgerr.CodeInvalidArgument, "event bus is required")
	}
	if f.IsApplied() {
		return rpgerr.New(rpgerr.CodeAlreadyExists, "in fog already applied")
	}
	f.bus = bus
	return nil
}

// Remove detaches membership; the authoritative area remains owned by resolution.
func (f *InFogCondition) Remove(_ context.Context, _ events.EventBus) error {
	f.bus = nil
	return nil
}

// ToJSON serializes the source-qualified membership without runtime bindings.
func (f *InFogCondition) ToJSON() (json.RawMessage, error) {
	return json.Marshal(InFogConditionData{Ref: f.Ref(), MemberID: f.MemberID, SourceID: f.SourceID, SourceRef: f.SourceRef})
}

func (f *InFogCondition) loadJSON(data json.RawMessage) error {
	var stored InFogConditionData
	if err := json.Unmarshal(data, &stored); err != nil {
		return rpgerr.Wrap(err, "load in fog")
	}
	if stored.Ref == nil || stored.Ref.String() != refs.Conditions.InFog().String() {
		return rpgerr.New(rpgerr.CodeInvalidArgument, "invalid in fog condition reference")
	}
	loaded, err := NewInFogCondition(NewInFogConditionInput{MemberID: stored.MemberID, SourceID: stored.SourceID, SourceRef: stored.SourceRef})
	if err != nil {
		return err
	}
	*f = *loaded
	return nil
}
