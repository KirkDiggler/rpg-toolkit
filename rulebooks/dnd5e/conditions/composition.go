// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package conditions

import (
	"fmt"
	"strings"

	generic "github.com/KirkDiggler/rpg-toolkit/mechanics/effects"
	dndfx "github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/effects"
)

// CompositionSpec selects typed recipient benefits and independent coordinator
// maintenance. It is unbound content, not an instance, source budget or runtime.
type CompositionSpec struct {
	Name        string
	Coordinator DefinitionRole
	Benefits    []dndfx.PartSpec
	Maintenance []dndfx.PartSpec
}

// CompileCompositionInput supplies one named code-authored composition.
type CompileCompositionInput struct {
	ID   string
	Spec CompositionSpec
}

// CompileComposition lowers code-authored content to the same validated expanded
// definitions as CompileDefinitions. The returned one-entry catalogue uses the
// existing BindDefinition/application protocol. Nil input, invalid identity/role,
// empty or misplaced parts, duplicate keys and invalid capabilities are errors.
// Compilation owns its data and performs no attachment, payment or world lookup.
func CompileComposition(in *CompileCompositionInput) (*CompileDefinitionsOutput, error) {
	if in == nil {
		return nil, fmt.Errorf("composition input is required")
	}
	benefits, err := dndfx.CompileParts(&dndfx.CompilePartsInput{Parts: in.Spec.Benefits})
	if err != nil {
		return nil, fmt.Errorf("composition benefits: %w", err)
	}
	maintenance, err := dndfx.CompileParts(&dndfx.CompilePartsInput{Parts: in.Spec.Maintenance})
	if err != nil {
		return nil, fmt.Errorf("composition maintenance: %w", err)
	}
	definition, err := compileExpandedDefinition(expandedDefinitionInput{ID: in.ID, Name: in.Spec.Name, Coordinator: in.Spec.Coordinator, Benefits: benefits.Parts, Maintenance: maintenance.Parts})
	if err != nil {
		return nil, err
	}
	return &CompileDefinitionsOutput{Definitions: &CompiledDefinitions{definitions: map[string]compiledDefinition{in.ID: definition}}}, nil
}

type expandedDefinitionInput struct {
	ID          string
	Name        string
	Coordinator DefinitionRole
	Maintenance []dndfx.PartDeclaration
	Benefits    []dndfx.PartDeclaration
}

// Both authoring adapters converge here; neither owns another binder/runtime.
func compileExpandedDefinition(in expandedDefinitionInput) (compiledDefinition, error) {
	if !validDefinitionKey(in.ID) || strings.TrimSpace(in.Name) == "" {
		return compiledDefinition{}, fmt.Errorf("definitions[%q] requires a valid id and name", in.ID)
	}
	if in.Coordinator != DefinitionUser && in.Coordinator != DefinitionRecipient {
		return compiledDefinition{}, fmt.Errorf("definitions[%q].coordinator is not user or recipient", in.ID)
	}
	maintenance, err := validateExpandedParts(expandedPartsInput{Parts: in.Maintenance, Class: dndfx.PartClassMaintenance})
	if err != nil {
		return compiledDefinition{}, fmt.Errorf("definitions[%q].maintenance: %w", in.ID, err)
	}
	benefits, err := validateExpandedParts(expandedPartsInput{Parts: in.Benefits, Class: dndfx.PartClassBenefit})
	if err != nil {
		return compiledDefinition{}, fmt.Errorf("definitions[%q].benefits: %w", in.ID, err)
	}
	return compiledDefinition{name: in.Name, coordinator: in.Coordinator, maintenance: maintenance, benefits: benefits}, nil
}

type expandedPartsInput struct {
	Parts []dndfx.PartDeclaration
	Class dndfx.PartClass
}

func validateExpandedParts(in expandedPartsInput) ([]dndfx.PartDeclaration, error) {
	if len(in.Parts) == 0 {
		return nil, fmt.Errorf("requires nonempty selections")
	}
	scope := generic.EndPart
	if in.Class == dndfx.PartClassMaintenance {
		scope = generic.EndApplication
	}
	keys := make(map[string]bool, len(in.Parts))
	concentration := 0
	out := make([]dndfx.PartDeclaration, 0, len(in.Parts))
	for _, part := range in.Parts {
		if err := dndfx.ValidatePart(&dndfx.ValidatePartInput{Declaration: part}); err != nil {
			return nil, err
		}
		if keys[part.Key] {
			return nil, fmt.Errorf("repeated part key %q", part.Key)
		}
		keys[part.Key] = true
		if part.OnTerminal != scope {
			return nil, fmt.Errorf("part %q has the wrong terminal scope", part.Key)
		}
		checked, err := dndfx.ValidatePartTemplate(&dndfx.ValidatePartTemplateInput{Template: dndfx.PartTemplate{Kind: part.Kind, Params: part.Params, When: part.When, Amount: part.Amount}})
		if err != nil {
			return nil, err
		}
		if checked.Class != in.Class {
			return nil, fmt.Errorf("part %q has the wrong class", part.Key)
		}
		if checked.Capabilities.Concentration {
			concentration++
			if concentration > 1 {
				return nil, fmt.Errorf("multiple concentration claims")
			}
		}
		out = append(out, part.Clone())
	}
	return out, nil
}
