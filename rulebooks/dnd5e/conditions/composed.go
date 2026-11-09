// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package conditions

import (
	"context"
	"encoding/json"
	"fmt"
	"slices"

	"github.com/KirkDiggler/rpg-toolkit/core"
	"github.com/KirkDiggler/rpg-toolkit/events"
	generic "github.com/KirkDiggler/rpg-toolkit/mechanics/effects"
	"github.com/KirkDiggler/rpg-toolkit/mechanics/effects/machine"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/contributions"
	ruleeffects "github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/effects"
	dndevents "github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/events"
)

// ComposedCondition implements the rulebook's protocols around one generic
// composition. It owns immutable origin/bindings and its saved representation,
// not another state runner or a copy of the mutable lifecycle bag.
type ComposedCondition struct {
	runtime  *generic.Composed
	memberID string
	origin   dndevents.ConditionOrigin
	bindings json.RawMessage
	display  Display
}

var _ dndevents.ConditionBehavior = (*ComposedCondition)(nil)
var _ dndevents.ConditionOriginProvider = (*ComposedCondition)(nil)
var _ contributions.ActionEffectsAssessor = (*ComposedCondition)(nil)

func composeInstance(def *compositionDefinition, member string, origin *dndevents.ConditionOrigin, bindings json.RawMessage, saved *generic.Snapshot) (*ComposedCondition, error) {
	if member == "" {
		return nil, fmt.Errorf("composed condition member_id is required")
	}
	if origin == nil {
		return nil, fmt.Errorf("composed condition origin is required")
	}
	bound, err := def.bind(bindings)
	if err != nil {
		return nil, err
	}
	vocab, err := ruleeffects.Bind(&ruleeffects.BindInput{Origin: *origin, DisplayName: def.display.Name})
	if err != nil {
		return nil, err
	}
	c := &ComposedCondition{memberID: member, origin: *origin, bindings: slices.Clone(bound.bindings), display: def.display}
	vocab.Outputs.End = func(ctx context.Context, bus events.EventBus, host generic.Host, reason string) error {
		return dndevents.ConditionRemovedTopic.On(bus).Publish(ctx, dndevents.ConditionRemovedEvent{MemberID: host.Holder, ConditionRef: host.Ref.String(), Reason: reason})
	}
	vocab.Outputs.Changed = func(ctx context.Context, bus events.EventBus, host generic.Host) error {
		return dndevents.ConditionStateChangedTopic.On(bus).Publish(ctx, dndevents.ConditionStateChangedEvent{MemberID: host.Holder, ConditionRef: host.Ref})
	}
	var state string
	var bag machine.Bag
	if saved != nil {
		if err := validateRestoredRuntime(bound.recipe, saved); err != nil {
			return nil, err
		}
		state, bag = saved.State, saved.Bag
	}
	c.runtime, err = generic.Compose(generic.ComposeInput{Recipe: bound.recipe, MachineVocabulary: vocab.Machine, OutputVocabulary: vocab.Outputs, HolderID: member, SourceRef: &c.origin.Ref, State: state, Parameters: bag})
	if err != nil {
		return nil, err
	}
	if def.validateState != nil {
		if err := def.validateState(c.runtime.Snapshot()); err != nil {
			return nil, err
		}
	}
	return c, nil
}

// Ref returns a detached copy of the canonical definition ref.
func (c *ComposedCondition) Ref() *core.Ref { return c.runtime.Ref() }

// IsApplied reports runtime attachment, not item ownership or a budget.
func (c *ComposedCondition) IsApplied() bool { return c.runtime.IsApplied() }

// Apply attaches the composition to this interaction's bus and returns its errors.
func (c *ComposedCondition) Apply(ctx context.Context, bus events.EventBus) error {
	return c.runtime.Apply(ctx, bus)
}

// Remove detaches subscriptions without declaring a gameplay end or refund.
func (c *ComposedCondition) Remove(ctx context.Context, bus events.EventBus) error {
	return c.runtime.Remove(ctx, bus)
}

// ConditionOrigin returns immutable provenance by value, not a stacking address.
func (c *ComposedCondition) ConditionOrigin() dndevents.ConditionOrigin { return c.origin }

// ConditionAddress retains by-ref identity for this cut's composed definitions.
// Source provenance must never silently enable another simultaneous Raging.
func (c *ComposedCondition) ConditionAddress() dndevents.ConditionAddress {
	return dndevents.ConditionAddress{MemberID: c.memberID, ConditionRef: c.Ref().String()}
}

// ToJSON writes the sole current composed-instance shape using a typed snapshot,
// without encoding and decoding another runtime representation first.
func (c *ComposedCondition) ToJSON() (json.RawMessage, error) {
	snapshot := c.runtime.Snapshot()
	origin := c.origin
	return json.Marshal(&ComposedData{Ref: *c.Ref(), MemberID: c.memberID, Origin: &origin, Bindings: slices.Clone(c.bindings), Runtime: &snapshot})
}

// InspectEffectsInput selects a typed context, or descriptions only when empty.
type InspectEffectsInput struct{ Query ruleeffects.Query }

// InspectEffectsOutput lists one typed, domain-honest row per current output.
type InspectEffectsOutput struct{ Effects []contributions.Effect }

// InspectEffects describes/evaluates current outputs without attachment, payment,
// publication or state mutation. It refuses nil input and invalid query contexts.
func (c *ComposedCondition) InspectEffects(in *InspectEffectsInput) (*InspectEffectsOutput, error) {
	if in == nil {
		return nil, fmt.Errorf("effect inspection requires input")
	}
	if err := in.Query.Validate(); err != nil {
		return nil, err
	}
	rows := make([]contributions.Effect, 0)
	for _, out := range c.runtime.CurrentOutputs() {
		answer, err := ruleeffects.Assess(&ruleeffects.AssessInput{Query: in.Query, Output: out, Origin: c.origin, DisplayName: c.display.Name})
		if err != nil {
			return nil, err
		}
		rows = append(rows, answer.Effect)
	}
	return &InspectEffectsOutput{Effects: rows}, nil
}

// AssessActionEffects supplies plural information through the native attack/save
// frame boundary. Other effect domains remain explicitly not applicable there.
func (c *ComposedCondition) AssessActionEffects(in *contributions.AssessActionInput) (*contributions.AssessEffectsOutput, error) {
	if in == nil {
		return nil, fmt.Errorf("action effect assessment requires input")
	}
	frame := in.Frame.Clone()
	out, err := c.InspectEffects(&InspectEffectsInput{Query: ruleeffects.Query{Action: &frame}})
	if err != nil {
		return nil, err
	}
	return &contributions.AssessEffectsOutput{Effects: out.Effects}, nil
}
