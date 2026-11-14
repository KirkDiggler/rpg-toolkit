// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package conditions

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/KirkDiggler/rpg-toolkit/core"
	"github.com/KirkDiggler/rpg-toolkit/events"
	generic "github.com/KirkDiggler/rpg-toolkit/mechanics/effects"
	"github.com/KirkDiggler/rpg-toolkit/mechanics/effects/machine"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/contributions"
	ruleeffects "github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/effects"
	dndevents "github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/events"
)

// ComposedCondition is a local assembly-backed condition. It owns frozen resolved
// bindings, origin and one per-part snapshot, not another state runner or clock.
type ComposedCondition struct {
	runtime  *generic.Assembly
	ref      core.Ref
	memberID string
	origin   dndevents.ConditionOrigin
	bindings LocalBindings
	display  Display
}

var _ dndevents.ConditionBehavior = (*ComposedCondition)(nil)
var _ dndevents.ConditionOriginProvider = (*ComposedCondition)(nil)
var _ contributions.ActionEffectsAssessor = (*ComposedCondition)(nil)

func composeInstance(def *compositionDefinition, member string, origin *dndevents.ConditionOrigin, bindings *LocalBindings, saved *generic.AssemblySnapshot) (*ComposedCondition, error) {
	if strings.TrimSpace(member) == "" {
		return nil, fmt.Errorf("composed condition member_id is required")
	}
	if origin == nil {
		return nil, fmt.Errorf("composed condition origin is required")
	}
	if bindings == nil || len(bindings.Parts) == 0 {
		return nil, fmt.Errorf("local condition requires resolved parts")
	}
	c := &ComposedCondition{ref: def.ref, memberID: member, origin: *origin, bindings: LocalBindings{Parts: cloneDeclarations(bindings.Parts)}, display: def.display}
	parts := make([]generic.PartInput, 0, len(c.bindings.Parts))
	for _, declaration := range c.bindings.Parts {
		if declaration.OnTerminal == generic.EndApplication {
			return nil, fmt.Errorf("local condition cannot route an application ending")
		}
		bound, err := ruleeffects.BindPart(&ruleeffects.BindPartInput{Declaration: declaration, Ref: c.ref, HolderID: member, Origin: c.origin, DisplayName: c.display.Name})
		if err != nil {
			return nil, fmt.Errorf("local part %q: %w", declaration.Key, err)
		}
		parts = append(parts, bound.Part)
	}
	assembly := generic.AssemblyInput{Key: fmt.Sprintf("%d:%s/%s", len(member), member, c.ref.String()), Parts: parts, Lifecycle: generic.AssemblyLifecycle{
		End: func(ctx context.Context, bus events.EventBus, end generic.AssemblyEnd) error {
			return dndevents.ConditionRemovedTopic.On(bus).Publish(ctx, dndevents.ConditionRemovedEvent{MemberID: member, ConditionRef: c.ref.String(), Reason: end.Reason})
		},
		Changed: func(ctx context.Context, bus events.EventBus, _ generic.AssemblyChange) error {
			return dndevents.ConditionStateChangedTopic.On(bus).Publish(ctx, dndevents.ConditionStateChangedEvent{MemberID: member, ConditionRef: &c.ref})
		},
	}}
	var err error
	if saved == nil {
		c.runtime, err = generic.Assemble(assembly)
	} else {
		c.runtime, err = generic.RestoreAssembly(generic.RestoreAssemblyInput{Definition: assembly, Snapshot: saved})
	}
	if err != nil {
		return nil, err
	}
	if err := validateAssemblyRuntime(&assemblyRuntimeInput{Parts: c.bindings.Parts, Snapshot: c.runtime.Snapshot()}); err != nil {
		return nil, err
	}
	return c, nil
}

// Ref returns the unchanged native condition identity as a detached value.
func (c *ComposedCondition) Ref() *core.Ref { ref := c.ref; return &ref }

// IsApplied reports runtime attachment, not ownership or selection.
func (c *ComposedCondition) IsApplied() bool { return c.runtime.IsApplied() }

// Apply attaches the existing local assembly to this interaction.
func (c *ComposedCondition) Apply(ctx context.Context, bus events.EventBus) error {
	if c.runtime.IsApplied() {
		return machine.ErrAlreadyApplied
	}
	return c.runtime.Apply(ctx, bus)
}

// Remove detaches without declaring another gameplay ending or refund.
func (c *ComposedCondition) Remove(ctx context.Context, bus events.EventBus) error {
	return c.runtime.Remove(ctx, bus)
}

// ConditionOrigin returns immutable activation provenance by value.
func (c *ComposedCondition) ConditionOrigin() dndevents.ConditionOrigin { return c.origin }

// ConditionAddress preserves native by-ref admission; provenance does not enable
// another simultaneous native Raging instance.
func (c *ComposedCondition) ConditionAddress() dndevents.ConditionAddress {
	return dndevents.ConditionAddress{MemberID: c.memberID, ConditionRef: c.ref.String()}
}

// ToJSON writes the sole current resolved-local/assembly-state representation.
// No catalogue selector or duplicate single-machine bag is written.
func (c *ComposedCondition) ToJSON() (json.RawMessage, error) {
	snapshot := c.runtime.Snapshot()
	origin := c.origin
	return json.Marshal(&ComposedData{Ref: c.ref, MemberID: c.memberID, Origin: &origin, Bindings: &LocalBindings{Parts: cloneDeclarations(c.bindings.Parts)}, Runtime: &snapshot})
}

// InspectEffectsInput selects a typed query or descriptions only when empty.
type InspectEffectsInput struct{ Query ruleeffects.Query }

// InspectEffectsOutput retains one domain-honest row per available output.
type InspectEffectsOutput struct{ Effects []contributions.Effect }

// InspectEffects asks the same kind meanings as execution without attachment,
// rolling, payment or state mutation. Invalid query/meaning returns an error.
func (c *ComposedCondition) InspectEffects(in *InspectEffectsInput) (*InspectEffectsOutput, error) {
	if in == nil {
		return nil, fmt.Errorf("effect inspection requires input")
	}
	if err := in.Query.Validate(); err != nil {
		return nil, err
	}
	rows := make([]contributions.Effect, 0)
	for _, part := range c.runtime.CurrentOutputs() {
		answer, err := ruleeffects.Assess(&ruleeffects.AssessInput{Query: in.Query, Output: part.Output, Origin: c.origin, DisplayName: c.display.Name})
		if err != nil {
			return nil, err
		}
		rows = append(rows, answer.Effect)
	}
	return &InspectEffectsOutput{Effects: rows}, nil
}

// AssessActionEffects supplies plural current output rows through the native frame.
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

// MaintenanceInfo asks part-owned inspection for local progress. It does not
// interpret private bags or report concentration owned by another attachment.
func (c *ComposedCondition) MaintenanceInfo() (*MaintenanceInfo, error) {
	return inspectMaintenance(&inspectMaintenanceInput{Parts: c.bindings.Parts, Snapshot: c.runtime.Snapshot()})
}
