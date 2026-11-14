// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package effects

import (
	"encoding/json"
	jsonv2 "encoding/json/v2"
	"fmt"

	"github.com/KirkDiggler/rpg-toolkit/core"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/refs"
)

// DefinitionReference selects an exact configured value by catalogue identity
// and a separate relative member path. Empty Path selects the root definition.
type DefinitionReference struct {
	Definition core.Ref
	Path       []string
}

// DefinitionUse selects/copies a value, then explicitly replaces complete member
// values. It never merges fields or changes a running instance.
type DefinitionUse struct {
	Reference DefinitionReference
	Replace   []MemberReplacement
}

// MemberReplacement replaces the value at an existing nonroot member path,
// relative to the selected value. Its definition may contain further references.
type MemberReplacement struct {
	Path       []string           `json:"path"`
	Definition AuthoredDefinition `json:"definition"`
}

type definitionUseWire struct {
	Definition string              `json:"definition"`
	Path       []string            `json:"path,omitempty"`
	Replace    []MemberReplacement `json:"replace,omitempty"`
}

// MarshalJSON writes the sole string-ref/array-path authoring shape.
func (u DefinitionUse) MarshalJSON() ([]byte, error) {
	if err := validateDefinitionReference(u.Reference); err != nil {
		return nil, err
	}
	return json.Marshal(definitionUseWire{Definition: u.Reference.Definition.String(), Path: u.Reference.Path, Replace: u.Replace})
}

// UnmarshalJSON refuses object-ref fallback, duplicate/unknown fields and invalid
// ref/path grammar. Reference resolution still validates existence and cycles.
func (u *DefinitionUse) UnmarshalJSON(raw []byte) error {
	var wire definitionUseWire
	if err := jsonv2.Unmarshal(raw, &wire, jsonv2.RejectUnknownMembers(true)); err != nil {
		return err
	}
	ref, err := parseDefinitionRef(wire.Definition)
	if err != nil {
		return err
	}
	reference := DefinitionReference{Definition: ref, Path: wire.Path}
	if err := validateDefinitionReference(reference); err != nil {
		return err
	}
	*u = DefinitionUse{Reference: reference, Replace: wire.Replace}
	return nil
}

func parseDefinitionRef(raw string) (core.Ref, error) {
	ref, err := core.ParseString(raw)
	if err != nil {
		return core.Ref{}, err
	}
	if ref.Type != refs.TypeDefinitions {
		return core.Ref{}, fmt.Errorf("reference %q is not an authored definition", raw)
	}
	return *ref, nil
}
func validateDefinitionReference(reference DefinitionReference) error {
	if _, err := parseDefinitionRef(reference.Definition.String()); err != nil {
		return err
	}
	for _, key := range reference.Path {
		if !validMemberKey(key) {
			return fmt.Errorf("invalid definition member key %q", key)
		}
	}
	return nil
}
