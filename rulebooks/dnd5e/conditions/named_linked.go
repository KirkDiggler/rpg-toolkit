// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package conditions

import (
	"bytes"
	"encoding/json"
	"encoding/json/jsontext"
	"fmt"

	"github.com/KirkDiggler/rpg-toolkit/core"
	generic "github.com/KirkDiggler/rpg-toolkit/mechanics/effects"
	dndfx "github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/effects"
	dndevents "github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/events"
)

func namedRollParts(group string, subtract bool) []dndfx.PartDeclaration {
	params, err := json.Marshal(struct {
		Dice     string   `json:"dice"`
		Group    string   `json:"group"`
		Rolls    []string `json:"rolls"`
		Subtract bool     `json:"subtract"`
	}{Dice: "1d4", Group: group, Rolls: []string{"attack", "saving_throw"}, Subtract: subtract})
	if err != nil {
		panic(err) // static, typed rulebook declaration; no runtime input
	}
	return []dndfx.PartDeclaration{
		{Key: "roll", Kind: dndfx.KindRollModifier, Params: params, OnTerminal: generic.EndPart},
		{Key: "rest", Kind: dndfx.KindEndOn, Params: json.RawMessage(`{"beat":"rest","reason":"rest"}`), OnTerminal: generic.EndAttachment},
	}
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

func namedLinkedDescriptor(ref *core.Ref, display Display, parts []dndfx.PartDeclaration) descriptor {
	fixed := cloneDeclarations(parts)
	return descriptor{ref: ref, display: display, acceptsOrigin: true, linked: true, linkedParts: fixed,
		ownAction: answersNow, againstHolder: notBearing,
		create: func(in *CreateFromRefInput) (dndevents.ConditionBehavior, error) {
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
			var data LinkedConditionData
			if err := strictJSON(raw, &data); err != nil {
				return nil, err
			}
			if data.Ref != *ref || data.Role != LinkedRecipient || data.Bindings == nil {
				return nil, fmt.Errorf("named linked data has wrong ref, role or bindings")
			}
			if err := sameLinkedParts(data.Bindings.Parts, fixed); err != nil {
				return nil, err
			}
			return loadLinked(raw)
		},
	}
}
