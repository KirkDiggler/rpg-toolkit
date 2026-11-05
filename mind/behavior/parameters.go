// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package behavior

import (
	"errors"
	"fmt"
	"maps"
	"slices"

	"github.com/KirkDiggler/rpg-toolkit/mind/machine"
)

// ErrBadParameter reports invalid parameter metadata or a value that its
// metadata refuses. The wrapped message names the parameter and the reason.
var ErrBadParameter = errors.New("behavior: invalid parameter")

// ParameterSpec describes mechanical constraints for a producer's scalar
// parameter. It does not sample values, bind templates or interpret Unit.
// Values are typed Go bool, int or string; JSON producers must decode their
// values according to Kind rather than rely on untyped float coercion.
type ParameterSpec struct {
	// Name identifies a parameter within one schema; it is nonempty and unique.
	Name string `json:"name"`
	// Description is explanatory metadata for the authoring surface.
	Description string `json:"description,omitzero"`
	// Unit is an opaque authoring label, such as rounds, not a conversion rule.
	Unit string `json:"unit,omitzero"`
	// Kind is the scalar kind the producer supplies.
	Kind machine.ScalarKind `json:"kind"`
	// Required distinguishes a missing value from a supplied zero value.
	Required bool `json:"required,omitzero"`
	// Min and Max are inclusive integer bounds; nil means unbounded.
	Min *int `json:"min,omitzero"`
	Max *int `json:"max,omitzero"`
	// Choices restricts a string to this set; empty leaves it unrestricted.
	Choices []string `json:"choices,omitzero"`
}

// CheckParametersInput pairs a schema with producer-supplied values. Absent
// optional parameters remain absent; this operation supplies no defaults.
type CheckParametersInput struct {
	Schema []ParameterSpec
	Values map[string]any
}

// CheckParametersOutput contains a detached copy of accepted scalar values.
// It is a producer result, not a new persistence or runtime binding authority.
type CheckParametersOutput struct {
	Values map[string]any
}

// CheckParameters validates metadata first, then refuses unknown, missing,
// wrong-kind or out-of-constraint values by name. It returns ErrNilInput for
// nil input and ErrBadParameter for any schema/value refusal. No input changes
// on success or failure, and a supplied false, zero or empty string is present.
func CheckParameters(in *CheckParametersInput) (*CheckParametersOutput, error) {
	if in == nil {
		return nil, ErrNilInput
	}
	schema := make(map[string]ParameterSpec, len(in.Schema))
	for _, spec := range in.Schema {
		if err := checkParameterSpec(spec); err != nil {
			return nil, err
		}
		if _, exists := schema[spec.Name]; exists {
			return nil, fmt.Errorf("%w: duplicate name %q", ErrBadParameter, spec.Name)
		}
		schema[spec.Name] = spec
	}
	for _, name := range slices.Sorted(maps.Keys(in.Values)) {
		if _, exists := schema[name]; !exists {
			return nil, fmt.Errorf("%w: unknown parameter %q", ErrBadParameter, name)
		}
	}
	for _, spec := range in.Schema {
		value, present := in.Values[spec.Name]
		if !present {
			if spec.Required {
				return nil, fmt.Errorf("%w: required parameter %q is missing", ErrBadParameter, spec.Name)
			}
			continue
		}
		if err := checkParameterValue(spec, value); err != nil {
			return nil, err
		}
	}
	return &CheckParametersOutput{Values: maps.Clone(in.Values)}, nil
}

func checkParameterSpec(spec ParameterSpec) error {
	if spec.Name == "" {
		return fmt.Errorf("%w: parameter has no name", ErrBadParameter)
	}
	switch spec.Kind {
	case machine.KindBool, machine.KindInt, machine.KindString:
	default:
		return fmt.Errorf("%w: %q has unsupported kind %q", ErrBadParameter, spec.Name, spec.Kind)
	}
	if (spec.Min != nil || spec.Max != nil) && spec.Kind != machine.KindInt {
		return fmt.Errorf("%w: %q has integer bounds on %s", ErrBadParameter, spec.Name, spec.Kind)
	}
	if spec.Min != nil && spec.Max != nil && *spec.Min > *spec.Max {
		return fmt.Errorf("%w: %q has minimum above maximum", ErrBadParameter, spec.Name)
	}
	if len(spec.Choices) > 0 && spec.Kind != machine.KindString {
		return fmt.Errorf("%w: %q has string choices on %s", ErrBadParameter, spec.Name, spec.Kind)
	}
	seen := make(map[string]bool, len(spec.Choices))
	for _, choice := range spec.Choices {
		if seen[choice] {
			return fmt.Errorf("%w: %q repeats choice %q", ErrBadParameter, spec.Name, choice)
		}
		seen[choice] = true
	}
	return nil
}

func checkParameterValue(spec ParameterSpec, value any) error {
	matches := false
	switch spec.Kind {
	case machine.KindBool:
		_, matches = value.(bool)
	case machine.KindInt:
		n, ok := value.(int)
		matches = ok
		if ok && ((spec.Min != nil && n < *spec.Min) || (spec.Max != nil && n > *spec.Max)) {
			return fmt.Errorf("%w: %q is outside its integer bounds", ErrBadParameter, spec.Name)
		}
	case machine.KindString:
		text, ok := value.(string)
		matches = ok
		if ok && len(spec.Choices) > 0 && !slices.Contains(spec.Choices, text) {
			return fmt.Errorf("%w: %q is not one of its choices", ErrBadParameter, spec.Name)
		}
	}
	if !matches {
		return fmt.Errorf("%w: %q wants %s, got %T", ErrBadParameter, spec.Name, spec.Kind, value)
	}
	return nil
}
