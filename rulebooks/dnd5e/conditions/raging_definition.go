// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package conditions

import (
	"encoding/json"
	"fmt"

	generic "github.com/KirkDiggler/rpg-toolkit/mechanics/effects"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/definitions"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/effects"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/refs"
)

// RagingBindings is the source's native construction input, not active save data.
// The named definition owns its configured effects; the condition owns placement.
type RagingBindings struct {
	DamageBonus RagingDamageBonus `json:"damage_bonus"`
}

// RagingDamageBonus is the same named-definition input type, retained here for
// native creation callers. It is not a second definition or saved-state reader.
type RagingDamageBonus = definitions.RagingDamageBonus

var ragingDescriptor = composedDescriptor(compositionDefinition{
	ref:     *refs.Conditions.Raging(),
	display: Display{Name: "Raging", Detail: "Adds your rage damage bonus to melee weapon attacks that use Strength. Also grants advantage on Strength-based skill checks and Strength saving throws, and resistance to bludgeoning, piercing, and slashing damage."},
	bind:    bindRaging,
})

func bindRaging(raw json.RawMessage) (LocalBindings, error) {
	var input RagingBindings
	if err := strictJSON(raw, &input); err != nil {
		return LocalBindings{}, fmt.Errorf("raging bindings: %w", err)
	}
	definition, err := definitions.Raging(&definitions.RagingInput{DamageBonus: input.DamageBonus})
	if err != nil {
		return LocalBindings{}, err
	}
	compiled, err := effects.Compile(&effects.CompileInput{Definition: definition})
	if err != nil {
		return LocalBindings{}, err
	}
	placed, err := effects.Place(&effects.PlaceInput{Definition: compiled.Definition, Mounts: []effects.Mount{
		{Key: "benefits", Path: []string{"benefits"}, OnTerminal: generic.EndPart},
		{Key: "maintenance", Path: []string{"maintenance"}, OnTerminal: generic.EndAttachment},
	}})
	if err != nil {
		return LocalBindings{}, err
	}
	parts := make([]effects.PartDeclaration, 0, len(placed.Parts))
	for _, part := range placed.Parts {
		parts = append(parts, part.Declaration)
	}
	return LocalBindings{Parts: parts}, nil
}
