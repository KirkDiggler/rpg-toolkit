// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package effects

import (
	"fmt"

	generic "github.com/KirkDiggler/rpg-toolkit/mechanics/effects"
)

// CompileOutputsInput supplies validated-shape payload declarations, not runtime
// bindings. The function still validates each entry and refuses maintenance.
type CompileOutputsInput struct{ Parts []PartDeclaration }

// CompileOutputsOutput owns the mechanical outputs in declaration order, ready
// for an existing recipe to use without fabricating an actor or application ID.
type CompileOutputsOutput struct{ Outputs []generic.Output }

// CompileOutputs uses each kind's existing payload builder, rather than copying
// mechanical definitions into a second recipe path. Nil input, invalid parts,
// duplicate keys and maintenance/no-payload kinds are errors with no partial
// result. It preserves quantity/guard ownership and causes no runtime effects.
func CompileOutputs(in *CompileOutputsInput) (*CompileOutputsOutput, error) {
	if in == nil {
		return nil, fmt.Errorf("payload output input is required")
	}
	out := &CompileOutputsOutput{Outputs: make([]generic.Output, 0, len(in.Parts))}
	keys := make(map[string]bool, len(in.Parts))
	for _, part := range in.Parts {
		if err := ValidatePart(&ValidatePartInput{Declaration: part}); err != nil {
			return nil, err
		}
		if keys[part.Key] {
			return nil, fmt.Errorf("repeated payload part %q", part.Key)
		}
		keys[part.Key] = true
		kind := kindTable[part.Kind]
		if kind.class != PartClassBenefit || kind.payload == nil {
			return nil, fmt.Errorf("part %q is not a payload output", part.Key)
		}
		output, err := kind.payload(part)
		if err != nil {
			return nil, err
		}
		out.Outputs = append(out.Outputs, output.Clone())
	}
	return out, nil
}
