// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package conditions

import (
	"fmt"
	"strings"

	generic "github.com/KirkDiggler/rpg-toolkit/mechanics/effects"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/definitions"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/effects"
)

// RagingBenefitsInput is the first-cut composition adapter's construction input.
// New authoring uses definitions.Raging and its actual named fields directly.
type RagingBenefitsInput struct{ DamageBonus RagingDamageBonus }

// RagingBenefitsOutput carries first-cut part specs during host-adapter migration.
// No mechanical content is defined in this compatibility source adapter.
type RagingBenefitsOutput struct{ Parts []effects.PartSpec }

// RagingBenefits projects the complete named definition's benefit group for
// first-cut callers. It neither copies a Rage recipe nor changes its meanings.
// Nil/invalid input returns errors from the same named constructor/compiler.
func RagingBenefits(in *RagingBenefitsInput) (*RagingBenefitsOutput, error) {
	if in == nil {
		return nil, fmt.Errorf("raging benefits input is required")
	}
	named, err := definitions.Raging(&definitions.RagingInput{DamageBonus: in.DamageBonus})
	if err != nil {
		return nil, err
	}
	compiled, err := effects.Compile(&effects.CompileInput{Definition: named.Benefits})
	if err != nil {
		return nil, err
	}
	parts := make([]effects.PartSpec, 0)
	for _, leaf := range compiled.Definition.Leaves() {
		spec := leaf.Effect
		parts = append(parts, effects.PartSpec{Key: strings.Join(leaf.Path, "/"), Effect: effects.DefinitionData{Effect: &spec}, OnTerminal: generic.EndPart})
	}
	return &RagingBenefitsOutput{Parts: parts}, nil
}
