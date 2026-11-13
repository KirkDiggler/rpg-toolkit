// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package effects

import (
	"fmt"
	"reflect"
	"strings"

	generic "github.com/KirkDiggler/rpg-toolkit/mechanics/effects"
)

// PartSpec gives a typed effect its local identity and explicit terminal scope.
// Neither the definition nor its enclosing spec contains live participant IDs.
type PartSpec struct {
	Key        string
	Effect     Definition
	OnTerminal generic.EndScope
}

// CompilePartsInput supplies ordered typed content for canonical validation.
type CompilePartsInput struct{ Parts []PartSpec }

// CompilePartsOutput owns detached canonical declarations in authored order.
type CompilePartsOutput struct{ Parts []PartDeclaration }

// CompileParts lowers typed definitions through the same kind validation as
// authored data. Nil input/definitions, blank/duplicate keys, invalid scopes and
// malformed or unsupported meaning are errors with no partial output. It never
// binds, attaches, rolls, spends or mutates. An empty part list is valid here;
// the enclosing composition owns whether it requires benefits or maintenance.
func CompileParts(in *CompilePartsInput) (*CompilePartsOutput, error) {
	if in == nil {
		return nil, fmt.Errorf("typed parts input is required")
	}
	out := &CompilePartsOutput{Parts: make([]PartDeclaration, 0, len(in.Parts))}
	keys := make(map[string]bool, len(in.Parts))
	for i, part := range in.Parts {
		if strings.TrimSpace(part.Key) == "" || keys[part.Key] {
			return nil, fmt.Errorf("typed part %d requires a unique nonempty key", i)
		}
		keys[part.Key] = true
		if part.Effect == nil {
			return nil, fmt.Errorf("typed part %q requires an effect definition", part.Key)
		}
		value := reflect.ValueOf(part.Effect)
		if value.Kind() == reflect.Pointer && value.IsNil() {
			return nil, fmt.Errorf("typed part %q has a nil effect definition", part.Key)
		}
		template, err := part.Effect.effectTemplate()
		if err != nil {
			return nil, fmt.Errorf("typed part %q: %w", part.Key, err)
		}
		declaration := PartDeclaration{Key: part.Key, Kind: template.Kind, Params: template.Params, When: template.When, Amount: template.Amount, OnTerminal: part.OnTerminal}
		if err := ValidatePart(&ValidatePartInput{Declaration: declaration}); err != nil {
			return nil, fmt.Errorf("typed part %q: %w", part.Key, err)
		}
		out.Parts = append(out.Parts, declaration.Clone())
	}
	return out, nil
}
