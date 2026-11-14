// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package conditions

import (
	"encoding/json"
	jsonv2 "encoding/json/v2"
	"fmt"

	"github.com/KirkDiggler/rpg-toolkit/core"
	generic "github.com/KirkDiggler/rpg-toolkit/mechanics/effects"
	dndfx "github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/effects"
	dndevents "github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/events"
)

// LocalBindings freezes resolved local part declarations and terminal placement.
// It contains neither a catalogue reference nor another mutable runtime bag.
type LocalBindings struct {
	Parts []dndfx.PartDeclaration `json:"parts"`
}

// ComposedData is the sole current local-composition envelope. Bindings are
// resolved immutable values; Runtime owns independent per-part snapshots. The
// old single state/bag envelope is refused, not normalized on load.
type ComposedData struct {
	Ref      core.Ref                   `json:"ref"`
	MemberID string                     `json:"member_id"`
	Origin   *dndevents.ConditionOrigin `json:"origin"`
	Bindings *LocalBindings             `json:"bindings"`
	Runtime  *generic.AssemblySnapshot  `json:"runtime"`
}

type compositionDefinition struct {
	ref     core.Ref
	display Display
	bind    func(json.RawMessage) (LocalBindings, error)
}

// ValidateBindingsInput names native construction content, not active save data.
type ValidateBindingsInput struct {
	Ref      string
	Bindings json.RawMessage
}

// ValidateBindings checks the native source's construction choices without a
// holder. Active loads validate saved resolved parts instead of rebuilding them.
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
			bindings, err := def.bind(in.Config)
			if err != nil {
				return nil, err
			}
			return createdAs(composeInstance(&def, in.MemberID, in.Origin, &bindings, nil))
		},
		load: func(raw json.RawMessage) (dndevents.ConditionBehavior, error) {
			var data ComposedData
			if err := strictJSON(raw, &data); err != nil {
				return nil, fmt.Errorf("composed condition data: %w", err)
			}
			if data.Ref != def.ref {
				return nil, fmt.Errorf("condition data ref does not match %s", def.ref.String())
			}
			if data.Bindings == nil || data.Runtime == nil {
				return nil, fmt.Errorf("composed condition requires resolved bindings and current assembly runtime")
			}
			return createdAs(composeInstance(&def, data.MemberID, data.Origin, data.Bindings, data.Runtime))
		},
	}
}
