// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package conditions

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/KirkDiggler/rpg-toolkit/core"
	"github.com/KirkDiggler/rpg-toolkit/events"
	generic "github.com/KirkDiggler/rpg-toolkit/mechanics/effects"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/contributions"
	dndfx "github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/effects"
	dndevents "github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/events"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/refs"
)

type linkedEndRequest struct {
	Condition *LinkedCondition
	PartKey   string
	Scope     generic.EndScope
	Reason    string
}

type linkedTransport interface {
	endCondition(context.Context, linkedEndRequest) error
}

// LinkedCondition is one entity-local assembly in a shared application. Its
// coordinator alone owns lifetime membership. Origin, address and application
// identity are separate; no runtime pointer or transport enters its saved form.
type LinkedCondition struct {
	ref           core.Ref
	member        string
	addressSource string
	application   string
	role          LinkedRole
	origin        dndevents.ConditionOrigin
	bindings      LinkedBindings
	assembly      *generic.Assembly
	lifetime      *generic.Lifetime
	definition    generic.AssemblyInput
	transport     linkedTransport
}

var (
	_ dndevents.ConditionBehavior         = (*LinkedCondition)(nil)
	_ dndevents.ConditionAddressProvider  = (*LinkedCondition)(nil)
	_ dndevents.ConditionOriginProvider   = (*LinkedCondition)(nil)
	_ contributions.ActionEffectsAssessor = (*LinkedCondition)(nil)
)

func buildLinked(data LinkedConditionData, restored bool) (*LinkedCondition, error) {
	if err := validateLinkedData(data); err != nil {
		return nil, err
	}
	c := &LinkedCondition{ref: data.Ref, member: data.MemberID, addressSource: data.AddressSourceID,
		application: data.ApplicationID, role: data.Role, origin: *data.Origin,
		bindings: cloneLinkedBindings(*data.Bindings)}
	owner := c.ConditionAddress()
	if c.role == LinkedRecipient {
		owner = *c.bindings.Owner
	}
	definition := generic.AssemblyInput{Key: fmt.Sprintf("%q/%q/%q/%q", c.application, c.member, c.ref.String(), c.addressSource),
		Lifecycle: generic.AssemblyLifecycle{End: c.partEnded, Changed: c.partChanged}}
	enders := []string{"external"}
	concentrationCount := 0
	for _, declaration := range c.bindings.Parts {
		bound, err := dndfx.BindPart(&dndfx.BindPartInput{
			Declaration: declaration, Ref: c.ref, HolderID: c.member,
			ApplicationID: c.application, DisplayName: c.bindings.Name, Origin: c.origin, Owner: owner,
		})
		if err != nil {
			return nil, fmt.Errorf("linked part %q: %w", declaration.Key, err)
		}
		if bound.Capabilities.Concentration {
			concentrationCount++
		}
		if declaration.OnTerminal == generic.EndApplication {
			enders = append(enders, "part:"+declaration.Key)
		}
		definition.Parts = append(definition.Parts, bound.Part)
	}
	if concentrationCount > 1 {
		return nil, fmt.Errorf("linked application declares multiple concentration claims")
	}
	c.definition = definition
	var err error
	if restored {
		c.assembly, err = generic.RestoreAssembly(generic.RestoreAssemblyInput{Definition: definition, Snapshot: data.Runtime.Parts})
	} else {
		c.assembly, err = generic.Assemble(definition)
	}
	if err != nil {
		return nil, fmt.Errorf("linked assembly: %w", err)
	}
	if c.role == LinkedCoordinator {
		members := make([]string, 0, len(c.bindings.Members))
		for _, binding := range c.bindings.Members {
			members = append(members, binding.Key)
		}
		input := generic.LifetimeInput{ID: c.application, Members: members, Enders: enders}
		if restored {
			c.lifetime, err = generic.RestoreLifetime(generic.RestoreLifetimeInput{Definition: input, Snapshot: data.Runtime.Lifetime})
		} else {
			c.lifetime, err = generic.NewLifetime(input)
		}
		if err != nil {
			return nil, fmt.Errorf("linked lifetime: %w", err)
		}
	} else if restored && data.Runtime.Lifetime != nil {
		return nil, fmt.Errorf("recipient cannot carry lifetime authority")
	}
	if _, err := c.MaintenanceInfo(); err != nil {
		return nil, err
	}
	return c, nil
}

// Ref returns the registered attachment ref as a detached value.
func (c *LinkedCondition) Ref() *core.Ref { ref := c.ref; return &ref }

// ConditionAddress returns exact replacement identity, not origin provenance.
func (c *LinkedCondition) ConditionAddress() dndevents.ConditionAddress {
	return dndevents.ConditionAddress{MemberID: c.member, ConditionRef: c.ref.String(), SourceID: c.addressSource}
}

// ConditionOrigin returns immutable source attribution by value.
func (c *LinkedCondition) ConditionOrigin() dndevents.ConditionOrigin { return c.origin }

// IsApplied reports local interaction attachment.
func (c *LinkedCondition) IsApplied() bool { return c.assembly.IsApplied() }

// Apply attaches local behavior. Application graph admission/transport belongs
// to BindApplications in the caller's complete interaction; a local-only
// projection may attach payloads but cannot successfully end an unbound application.
func (c *LinkedCondition) Apply(ctx context.Context, bus events.EventBus) error {
	return c.assembly.Apply(ctx, bus)
}

// Remove is local teardown only; it does not turn detachment into a gameplay end.
func (c *LinkedCondition) Remove(ctx context.Context, bus events.EventBus) error {
	return c.assembly.Remove(ctx, bus)
}

// ToJSON writes the current linked representation. Ending/ended coordinator
// state is not saveable as a live application; failed transport cannot be hidden
// by serializing an owner whose required maintenance has already terminated.
func (c *LinkedCondition) ToJSON() (json.RawMessage, error) {
	parts := c.assembly.Snapshot()
	var lifetime *generic.LifetimeSnapshot
	if c.lifetime != nil {
		snapshot := c.lifetime.Snapshot()
		if snapshot.Phase != generic.LifetimeLive {
			return nil, fmt.Errorf("cannot save a non-live linked lifetime")
		}
		for _, declaration := range c.definition.Parts {
			state := parts.Parts[declaration.Key].State
			if declaration.OnTerminal == generic.EndApplication && declaration.Compose.Recipe.States[state].Terminal {
				return nil, fmt.Errorf("cannot save ended required maintenance %q", declaration.Key)
			}
		}
		lifetime = &snapshot
	}
	bindings := cloneLinkedBindings(c.bindings)
	origin := c.origin
	return json.Marshal(LinkedConditionData{Ref: c.ref, MemberID: c.member,
		AddressSourceID: c.addressSource, ApplicationID: c.application, Role: c.role,
		Origin: &origin, Bindings: &bindings, Runtime: &LinkedRuntimeData{Parts: &parts, Lifetime: lifetime}})
}

func (c *LinkedCondition) partChanged(ctx context.Context, bus events.EventBus, _ generic.AssemblyChange) error {
	return dndevents.ConditionStateChangedTopic.On(bus).Publish(ctx, dndevents.ConditionStateChangedEvent{
		MemberID: c.member, ConditionRef: c.Ref(),
	})
}

func (c *LinkedCondition) partEnded(ctx context.Context, _ events.EventBus, end generic.AssemblyEnd) error {
	if end.Scope == generic.EndPart {
		snapshot := c.assembly.Snapshot()
		for _, part := range c.definition.Parts {
			if !part.Compose.Recipe.States[snapshot.Parts[part.Key].State].Terminal {
				return nil
			}
		}
	}
	if c.transport == nil {
		return fmt.Errorf("linked application %q has no interaction transport", c.application)
	}
	return c.transport.endCondition(ctx, linkedEndRequest{Condition: c, PartKey: end.PartKey, Scope: end.Scope, Reason: end.Reason})
}

// AssessActionEffects describes local parts without interpreting lifetime links
// or inventing another holder's state. The outer selector owns group shadowing.
func (c *LinkedCondition) AssessActionEffects(in *contributions.AssessActionInput) (*contributions.AssessEffectsOutput, error) {
	if in == nil {
		return nil, fmt.Errorf("linked action assessment requires a frame")
	}
	frame := in.Frame.Clone()
	out := &contributions.AssessEffectsOutput{}
	for _, part := range c.assembly.CurrentOutputs() {
		answer, err := dndfx.Assess(&dndfx.AssessInput{Output: part.Output, Query: dndfx.Query{Action: &frame}, Origin: c.origin, DisplayName: c.bindings.Name})
		if err != nil {
			return nil, err
		}
		out.Effects = append(out.Effects, answer.Effect)
	}
	return out, nil
}

var linkedDescriptor = descriptor{
	ref:     refs.Conditions.LinkedEffects(),
	display: Display{Name: "Linked effects", Detail: "Contributes its configured effects under one linked application."},
	load:    loadLinked, create: createLinked, acceptsOrigin: true, ownAction: answersNow, againstHolder: notBearing,
}
