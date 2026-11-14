// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package effects

import (
	"encoding/json"
	jsonv2 "encoding/json/v2"

	"github.com/KirkDiggler/rpg-toolkit/core"
)

// AuthoredDefinition is the configuration form before references are resolved.
// Exactly one effect/composition/use is required by resolution; canonical
// DefinitionData never retains a Use node.
type AuthoredDefinition struct {
	Effect      *EffectSpec          `json:"effect,omitempty"`
	Composition *AuthoredComposition `json:"composition,omitempty"`
	Use         *DefinitionUse       `json:"use,omitempty"`
}

// AuthoredComposition supplies ordered members, which may reference other content.
type AuthoredComposition struct {
	Members []AuthoredMember `json:"members"`
}

// AuthoredMember names a definition value; display metadata does not dispatch it.
type AuthoredMember struct {
	Key         string             `json:"key"`
	Name        string             `json:"name,omitempty"`
	Description string             `json:"description,omitempty"`
	Definition  AuthoredDefinition `json:"definition"`
}

// UnmarshalJSON refuses unsupported/duplicate configuration fields, including
// nested definitions. It does not resolve references or create a runtime.
func (d *AuthoredDefinition) UnmarshalJSON(raw []byte) error {
	type wireDefinition AuthoredDefinition
	var wire wireDefinition
	if err := jsonv2.Unmarshal(raw, &wire, jsonv2.RejectUnknownMembers(true)); err != nil {
		return err
	}
	*d = AuthoredDefinition(wire)
	return nil
}

// CatalogueEntry registers descriptive authored content under a definition ref.
// It grants no running condition identity and owns no source budget or state.
type CatalogueEntry struct {
	Ref         core.Ref
	Name        string
	Description string
	Definition  AuthoredDefinition
}

type catalogueEntryWire struct {
	Ref         string             `json:"ref"`
	Name        string             `json:"name,omitempty"`
	Description string             `json:"description,omitempty"`
	Definition  AuthoredDefinition `json:"definition"`
}

// MarshalJSON writes an entry with its explicit string definition ref.
func (e CatalogueEntry) MarshalJSON() ([]byte, error) {
	if _, err := parseDefinitionRef(e.Ref.String()); err != nil {
		return nil, err
	}
	return json.Marshal(catalogueEntryWire{Ref: e.Ref.String(), Name: e.Name, Description: e.Description, Definition: e.Definition})
}

// UnmarshalJSON refuses historical object refs and unsupported entry fields.
func (e *CatalogueEntry) UnmarshalJSON(raw []byte) error {
	var wire catalogueEntryWire
	if err := jsonv2.Unmarshal(raw, &wire, jsonv2.RejectUnknownMembers(true)); err != nil {
		return err
	}
	ref, err := parseDefinitionRef(wire.Ref)
	if err != nil {
		return err
	}
	*e = CatalogueEntry{Ref: ref, Name: wire.Name, Description: wire.Description, Definition: wire.Definition}
	return nil
}

// CatalogueData is the strict file shape. BuildCatalogue validates every entry
// and reference before publishing a usable immutable catalogue.
type CatalogueData struct {
	Entries []CatalogueEntry `json:"entries"`
}

// UnmarshalJSON refuses extra/duplicate document fields instead of ignoring them.
func (d *CatalogueData) UnmarshalJSON(raw []byte) error {
	type wireCatalogue CatalogueData
	var wire wireCatalogue
	if err := jsonv2.Unmarshal(raw, &wire, jsonv2.RejectUnknownMembers(true)); err != nil {
		return err
	}
	*d = CatalogueData(wire)
	return nil
}

func authoredFromCanonical(data DefinitionData) AuthoredDefinition {
	if data.Effect != nil {
		effect := data.Effect.Clone()
		return AuthoredDefinition{Effect: &effect}
	}
	if data.Composition == nil {
		return AuthoredDefinition{}
	}
	members := make([]AuthoredMember, len(data.Composition.Members))
	for i, member := range data.Composition.Members {
		members[i] = AuthoredMember{Key: member.Key, Name: member.Name, Description: member.Description, Definition: authoredFromCanonical(member.Definition)}
	}
	return AuthoredDefinition{Composition: &AuthoredComposition{Members: members}}
}
