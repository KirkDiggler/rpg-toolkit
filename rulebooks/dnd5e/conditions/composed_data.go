// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package conditions

import (
	"encoding/json"
	jsonv2 "encoding/json/v2"
	"fmt"

	"github.com/KirkDiggler/rpg-toolkit/core"
	generic "github.com/KirkDiggler/rpg-toolkit/mechanics/effects"
	"github.com/KirkDiggler/rpg-toolkit/mechanics/effects/machine"
	dndevents "github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/events"
)

// ComposedData is the current persisted representation of a named rulebook
// composition. Origin and Bindings are immutable instance inputs; Runtime is the
// only mutable lifecycle snapshot. Nil sections are missing data, not creation.
type ComposedData struct {
	Ref      core.Ref                   `json:"ref"`
	MemberID string                     `json:"member_id"`
	Origin   *dndevents.ConditionOrigin `json:"origin"`
	Bindings json.RawMessage            `json:"bindings"`
	Runtime  *generic.Snapshot          `json:"runtime"`
}

type boundComposition struct {
	bindings json.RawMessage
	recipe   machine.Recipe
}

type compositionDefinition struct {
	ref           core.Ref
	display       Display
	bind          func(json.RawMessage) (boundComposition, error)
	validateState func(generic.Snapshot) error
}

// ValidateBindingsInput names a composed definition and the authored binding
// payload to validate without inventing a holder or constructing a runtime.
type ValidateBindingsInput struct {
	Ref      string
	Bindings json.RawMessage
}

// ValidateBindings checks the same binding contract used by create and load.
// It refuses nil input, unknown/non-composed refs and invalid binding data.
func ValidateBindings(in *ValidateBindingsInput) error {
	if in == nil {
		return fmt.Errorf("binding validation requires input")
	}
	entry, err := lookup(in.Ref)
	if err != nil {
		return err
	}
	if entry.composition == nil {
		return fmt.Errorf("condition %s is not composed", in.Ref)
	}
	_, err = entry.composition.bind(in.Bindings)
	return err
}

func strictJSON(data json.RawMessage, out any) error {
	if len(data) == 0 {
		return fmt.Errorf("JSON value is required")
	}
	return jsonv2.Unmarshal(data, out, jsonv2.RejectUnknownMembers(true))
}

func composedDescriptor(def compositionDefinition) descriptor {
	return descriptor{ref: &def.ref, display: def.display, ownAction: answersNow, againstHolder: notBearing, composition: &def, acceptsOrigin: true,
		create: func(in *CreateFromRefInput) (dndevents.ConditionBehavior, error) {
			if in.SourceRef != "" {
				return nil, fmt.Errorf("composed condition requires Origin, not legacy SourceRef")
			}
			return createdAs(composeInstance(&def, in.MemberID, in.Origin, in.Config, nil))
		},
		load: func(raw json.RawMessage) (dndevents.ConditionBehavior, error) {
			var data ComposedData
			if err := strictJSON(raw, &data); err != nil {
				return nil, fmt.Errorf("composed condition data: %w", err)
			}
			if data.Ref != def.ref {
				return nil, fmt.Errorf("condition data ref does not match %s", def.ref.String())
			}
			if data.Runtime == nil {
				return nil, fmt.Errorf("composed condition runtime is required; recreate obsolete characters")
			}
			return createdAs(composeInstance(&def, data.MemberID, data.Origin, data.Bindings, data.Runtime))
		},
	}
}

func validateRestoredRuntime(recipe machine.Recipe, state *generic.Snapshot) error {
	if state.State == "" {
		return fmt.Errorf("saved runtime state is required")
	}
	declared, ok := recipe.States[state.State]
	if !ok || declared.Terminal {
		return fmt.Errorf("saved runtime state %q is not live", state.State)
	}
	if state.Bag == nil || len(state.Bag) != len(recipe.Bag) {
		return fmt.Errorf("saved runtime bag must contain every declared key exactly once")
	}
	for key := range recipe.Bag {
		if _, ok := state.Bag[key]; !ok {
			return fmt.Errorf("saved runtime bag is missing %q", key)
		}
	}
	return nil
}
