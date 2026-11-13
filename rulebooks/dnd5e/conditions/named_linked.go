// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package conditions

import (
	"bytes"
	"encoding/json"
	"encoding/json/jsontext"
	"fmt"

	"github.com/KirkDiggler/rpg-toolkit/core"
	dndfx "github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/effects"
	dndevents "github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/events"
)

type namedLinkedDescriptorInput struct {
	Ref     *core.Ref
	Display Display
	Parts   func() ([]dndfx.PartDeclaration, error)
}

func sameLinkedParts(got, want []dndfx.PartDeclaration) error {
	left, err := json.Marshal(got)
	if err != nil {
		return err
	}
	right, err := json.Marshal(want)
	if err != nil {
		return err
	}
	a, b := jsontext.Value(left), jsontext.Value(right)
	if err := a.Canonicalize(jsontext.CanonicalizeRawInts(false)); err != nil {
		return err
	}
	if err := b.Canonicalize(jsontext.CanonicalizeRawInts(false)); err != nil {
		return err
	}
	if !bytes.Equal(a, b) {
		return fmt.Errorf("named linked condition parts disagree with its definition")
	}
	return nil
}

func namedLinkedDescriptor(definition namedLinkedDescriptorInput) descriptor {
	// Registration is inert. Definition errors reach prepare/create/load rather
	// than becoming package-initialization panics or absent registered content.
	parts := func() ([]dndfx.PartDeclaration, error) {
		if definition.Parts == nil {
			return nil, fmt.Errorf("named linked definition requires a parts factory")
		}
		built, err := definition.Parts()
		if err != nil {
			return nil, err
		}
		if len(built) == 0 {
			return nil, fmt.Errorf("named linked definition requires nonempty parts")
		}
		return cloneDeclarations(built), nil
	}
	return descriptor{ref: definition.Ref, display: definition.Display, acceptsOrigin: true, linked: true, linkedParts: parts,
		ownAction: answersNow, againstHolder: notBearing,
		create: func(in *CreateFromRefInput) (dndevents.ConditionBehavior, error) {
			fixed, err := parts()
			if err != nil {
				return nil, err
			}
			var config LinkedConditionConfig
			if err := strictJSON(in.Config, &config); err != nil {
				return nil, err
			}
			if config.Role != LinkedRecipient || config.Bindings == nil {
				return nil, fmt.Errorf("named linked payload requires current recipient bindings")
			}
			if config.Bindings.Parts != nil {
				if err := sameLinkedParts(config.Bindings.Parts, fixed); err != nil {
					return nil, err
				}
			}
			bindings := cloneLinkedBindings(*config.Bindings)
			bindings.Parts = cloneDeclarations(fixed)
			config.Bindings = &bindings
			raw, err := json.Marshal(config)
			if err != nil {
				return nil, err
			}
			input := *in
			input.Config = raw
			return createLinked(&input)
		},
		load: func(raw json.RawMessage) (dndevents.ConditionBehavior, error) {
			fixed, err := parts()
			if err != nil {
				return nil, err
			}
			var data LinkedConditionData
			if err := strictJSON(raw, &data); err != nil {
				return nil, err
			}
			if data.Ref != *definition.Ref || data.Role != LinkedRecipient || data.Bindings == nil {
				return nil, fmt.Errorf("named linked data has wrong ref, role or bindings")
			}
			if err := sameLinkedParts(data.Bindings.Parts, fixed); err != nil {
				return nil, err
			}
			return loadLinked(raw)
		},
	}
}
