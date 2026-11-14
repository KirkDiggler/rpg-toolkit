// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package effects

// DefinitionData is the resolved authoring value. Exactly one form is present;
// a zero value is absent, not an empty composition. It contains no references,
// runtime identity, terminal placement or state.
type DefinitionData struct {
	Effect      *EffectSpec      `json:"effect,omitempty"`
	Composition *CompositionData `json:"composition,omitempty"`
}

// CompositionData groups ordered named definition values. An explicit empty
// composition is valid; a host separately decides whether it may grant nothing.
type CompositionData struct {
	Members []MemberData `json:"members"`
}

// MemberData gives a child its local key and optional descriptive metadata.
// Keys are unique among siblings; names and descriptions never dispatch rules.
type MemberData struct {
	Key         string         `json:"key"`
	Name        string         `json:"name,omitempty"`
	Description string         `json:"description,omitempty"`
	Definition  DefinitionData `json:"definition"`
}

// DefinitionData projects already canonical data without interpretation. Compile
// validates structure/cycles before taking an owned copy; this projection itself
// borrows its nested values and never constructs an instance.
func (d DefinitionData) DefinitionData() (DefinitionData, error) { return d, nil }

// Composition is the Go authoring surface over typed or canonical children.
// It is structure, not a running scope or a collection of shared live bags.
type Composition struct{ Members []Member }

// Member names one typed or canonical child in a Go-authored composition.
type Member struct {
	Key         string
	Name        string
	Description string
	Definition  Definition
}

// DefinitionData projects the ordered members without binding or validation of
// mechanical meaning. Generic Go cycles and absent children return errors.
func (c Composition) DefinitionData() (DefinitionData, error) {
	return projectDefinition(&projectionInput{Definition: c, Active: map[*Composition]bool{}})
}
