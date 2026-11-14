// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package effects

import (
	"fmt"
	"slices"

	"github.com/KirkDiggler/rpg-toolkit/core"
	generic "github.com/KirkDiggler/rpg-toolkit/mechanics/effects"
)

// BuildCatalogueInput supplies the complete explicit definition universe.
type BuildCatalogueInput struct{ Entries []CatalogueEntry }

// BuildCatalogueOutput publishes only a completely validated immutable catalogue.
type BuildCatalogueOutput struct{ Catalogue *Catalogue }

// Catalogue owns resolved values and descriptive metadata. No running instance
// retains this object, and resolution returns detached values rather than handles.
type Catalogue struct {
	order  []core.Ref
	values map[core.Ref]catalogueValue
}
type catalogueValue struct {
	name        string
	description string
	definition  DefinitionData
}

// BuildCatalogue validates every entry, including unused content, before
// publication. Nil input, duplicate/invalid refs, bad members, unresolved/cyclic
// references or malformed meanings return no partial catalogue. Forward refs are
// valid; there is no network lookup, registration side effect or runtime creation.
func BuildCatalogue(in *BuildCatalogueInput) (*BuildCatalogueOutput, error) {
	if in == nil {
		return nil, fmt.Errorf("catalogue input is required")
	}
	resolver := newDefinitionResolver()
	resolver.entries = make(map[core.Ref]CatalogueEntry, len(in.Entries))
	for _, entry := range in.Entries {
		if _, err := parseDefinitionRef(entry.Ref.String()); err != nil {
			return nil, err
		}
		if _, exists := resolver.entries[entry.Ref]; exists {
			return nil, fmt.Errorf("duplicate definition ref %s", entry.Ref.String())
		}
		resolver.entries[entry.Ref] = entry
	}
	result := &Catalogue{order: make([]core.Ref, 0, len(in.Entries)), values: make(map[core.Ref]catalogueValue, len(in.Entries))}
	for _, entry := range in.Entries {
		data, err := resolver.reference(DefinitionReference{Definition: entry.Ref})
		if err != nil {
			return nil, err
		}
		compiled, err := Compile(&CompileInput{Definition: data})
		if err != nil {
			return nil, fmt.Errorf("definition %s: %w", entry.Ref.String(), err)
		}
		result.order = append(result.order, entry.Ref)
		result.values[entry.Ref] = catalogueValue{name: entry.Name, description: entry.Description, definition: compiled.Definition.Data()}
	}
	return &BuildCatalogueOutput{Catalogue: result}, nil
}

// DefineEntryInput registers a typed definition's actual projected values,
// without a second JSON-only copy of the content or a mutable global registry.
type DefineEntryInput struct {
	Ref         core.Ref
	Name        string
	Description string
	Definition  Definition
}

// DefineEntryOutput contains owned authored data suitable for BuildCatalogue.
type DefineEntryOutput struct{ Entry CatalogueEntry }

// DefineEntry projects/validates typed content into an ordinary catalogue entry.
// Nil input, invalid refs or a malformed definition return an error. This is an
// authoring adapter to the same compiler, not another mechanical evaluator.
func DefineEntry(in *DefineEntryInput) (*DefineEntryOutput, error) {
	if in == nil {
		return nil, fmt.Errorf("typed catalogue entry input is required")
	}
	if _, err := parseDefinitionRef(in.Ref.String()); err != nil {
		return nil, err
	}
	compiled, err := Compile(&CompileInput{Definition: in.Definition})
	if err != nil {
		return nil, err
	}
	return &DefineEntryOutput{Entry: CatalogueEntry{Ref: in.Ref, Name: in.Name, Description: in.Description, Definition: authoredFromCanonical(compiled.Definition.Data())}}, nil
}

// DefinitionForm identifies a catalogue node for discovery, not rule dispatch.
type DefinitionForm string

const (
	// EffectForm describes a configured mechanical leaf.
	EffectForm DefinitionForm = "effect"
	// CompositionForm describes ordered reusable members.
	CompositionForm DefinitionForm = "composition"
)

// DefinitionDescription is an owned discovery view. Order is the depth-first
// ordinal within an entry; Path is separate from its definition ref and any
// runtime instance identity. Kind is present only on an effect leaf.
type DefinitionDescription struct {
	Ref         core.Ref
	Path        []string
	Form        DefinitionForm
	Name        string
	Description string
	Order       int
	Kind        generic.KindName
}

// DescribeCatalogueInput supplies an already validated catalogue.
type DescribeCatalogueInput struct{ Catalogue *Catalogue }

// DescribeCatalogueOutput owns ordered entry/root/member discovery descriptions.
type DescribeCatalogueOutput struct{ Entries []DefinitionDescription }

// DescribeCatalogue enumerates actual resolved structure without granting,
// binding or interpreting runtime state. Missing/uninitialized input is an error;
// an explicitly built empty catalogue yields an empty description list.
func DescribeCatalogue(in *DescribeCatalogueInput) (*DescribeCatalogueOutput, error) {
	if in == nil || in.Catalogue == nil || in.Catalogue.values == nil {
		return nil, fmt.Errorf("validated catalogue is required")
	}
	out := &DescribeCatalogueOutput{Entries: []DefinitionDescription{}}
	for _, ref := range in.Catalogue.order {
		value := in.Catalogue.values[ref]
		order := 0
		describeDefinition(describeDefinitionInput{Ref: ref, Data: value.definition, Name: value.name, Description: value.description}, &order, &out.Entries)
	}
	return out, nil
}

type describeDefinitionInput struct {
	Ref         core.Ref
	Data        DefinitionData
	Path        []string
	Name        string
	Description string
}

func describeDefinition(in describeDefinitionInput, order *int, out *[]DefinitionDescription) {
	node := DefinitionDescription{Ref: in.Ref, Path: slices.Clone(in.Path), Name: in.Name, Description: in.Description, Order: *order, Form: CompositionForm}
	*order = *order + 1
	if in.Data.Effect != nil {
		node.Form = EffectForm
		node.Kind = in.Data.Effect.Kind
	}
	*out = append(*out, node)
	if in.Data.Composition != nil {
		for _, member := range in.Data.Composition.Members {
			describeDefinition(describeDefinitionInput{Ref: in.Ref, Data: member.Definition, Path: append(slices.Clone(in.Path), member.Key), Name: member.Name, Description: member.Description}, order, out)
		}
	}
}
