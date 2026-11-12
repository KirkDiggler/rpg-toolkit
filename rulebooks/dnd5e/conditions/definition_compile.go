// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package conditions

import (
	"fmt"
	"maps"
	"slices"
	"strings"

	generic "github.com/KirkDiggler/rpg-toolkit/mechanics/effects"
	dndfx "github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/effects"
)

func validDefinitionKey(key string) bool {
	return strings.TrimSpace(key) != "" && !strings.Contains(key, "/")
}

// CompileDefinitions validates every library entry, resolves ordered selections
// and takes ownership of copied data. Unknown fields/references/kinds, duplicate
// JSON members, invalid guards/amounts, misplaced classes and duplicate claims
// refuse the complete library. It constructs no runtime and performs no I/O.
func CompileDefinitions(in *CompileDefinitionsInput) (*CompileDefinitionsOutput, error) {
	if in == nil {
		return nil, fmt.Errorf("definition library input is required")
	}
	var library DefinitionLibrary
	if err := strictJSON(in.Data, &library); err != nil {
		return nil, fmt.Errorf("definition library: %w", err)
	}
	if len(library.Parts) == 0 || len(library.Definitions) == 0 {
		return nil, fmt.Errorf("definition library requires parts and definitions")
	}
	info := make(map[string]dndfx.ValidatePartTemplateOutput, len(library.Parts))
	for _, id := range slices.Sorted(maps.Keys(library.Parts)) {
		if !validDefinitionKey(id) {
			return nil, fmt.Errorf("invalid part id %q", id)
		}
		checked, err := dndfx.ValidatePartTemplate(&dndfx.ValidatePartTemplateInput{Template: library.Parts[id]})
		if err != nil {
			return nil, fmt.Errorf("parts[%q]: %w", id, err)
		}
		info[id] = *checked
	}
	for _, id := range slices.Sorted(maps.Keys(library.BenefitSets)) {
		if !validDefinitionKey(id) || len(library.BenefitSets[id]) == 0 {
			return nil, fmt.Errorf("benefit_sets[%q] requires a valid id and leaves", id)
		}
		seen := map[string]bool{}
		for _, leaf := range library.BenefitSets[id] {
			entry, ok := info[leaf]
			if !ok || entry.Class != dndfx.PartClassBenefit || seen[leaf] {
				return nil, fmt.Errorf("benefit_sets[%q]: invalid, non-benefit or repeated part %q", id, leaf)
			}
			seen[leaf] = true
		}
	}
	compiled := &CompiledDefinitions{definitions: make(map[string]compiledDefinition, len(library.Definitions))}
	for _, id := range slices.Sorted(maps.Keys(library.Definitions)) {
		def := library.Definitions[id]
		if !validDefinitionKey(id) || strings.TrimSpace(def.Name) == "" {
			return nil, fmt.Errorf("definitions[%q] requires a valid id and name", id)
		}
		if def.Coordinator != DefinitionUser && def.Coordinator != DefinitionRecipient {
			return nil, fmt.Errorf("definitions[%q].coordinator is not user or recipient", id)
		}
		maintenance, err := expandDefinitionParts(library, info, def.Maintenance, dndfx.PartClassMaintenance)
		if err != nil {
			return nil, fmt.Errorf("definitions[%q].maintenance: %w", id, err)
		}
		benefits, err := expandDefinitionParts(library, info, def.Benefits, dndfx.PartClassBenefit)
		if err != nil {
			return nil, fmt.Errorf("definitions[%q].benefits: %w", id, err)
		}
		compiled.definitions[id] = compiledDefinition{name: def.Name, coordinator: def.Coordinator, maintenance: maintenance, benefits: benefits}
	}
	return &CompileDefinitionsOutput{Definitions: compiled}, nil
}

func expandDefinitionParts(library DefinitionLibrary, info map[string]dndfx.ValidatePartTemplateOutput, selections []PartSelection, class dndfx.PartClass) ([]dndfx.PartDeclaration, error) {
	if len(selections) == 0 {
		return nil, fmt.Errorf("requires nonempty selections")
	}
	scope := generic.EndPart
	if class == dndfx.PartClassMaintenance {
		scope = generic.EndApplication
	}
	result := []dndfx.PartDeclaration{}
	keys, leaves := map[string]bool{}, map[string]bool{}
	concentration := 0
	for _, selection := range selections {
		if !validDefinitionKey(selection.Key) || (selection.Part == "") == (selection.Set == "") {
			return nil, fmt.Errorf("selection %q requires a local key and exactly one part/set", selection.Key)
		}
		selected := []string{selection.Part}
		if selection.Set != "" {
			var ok bool
			selected, ok = library.BenefitSets[selection.Set]
			if class != dndfx.PartClassBenefit || !ok {
				return nil, fmt.Errorf("selection %q has an unknown or misplaced benefit set %q", selection.Key, selection.Set)
			}
		}
		for _, leaf := range selected {
			entry, ok := info[leaf]
			if !ok || entry.Class != class {
				return nil, fmt.Errorf("selection %q has an unknown or misplaced part %q", selection.Key, leaf)
			}
			key := selection.Key
			if selection.Set != "" {
				key += "/" + leaf
			}
			if keys[key] || leaves[leaf] {
				return nil, fmt.Errorf("selection %q repeats a key or leaf %q", selection.Key, leaf)
			}
			keys[key], leaves[leaf] = true, true
			if entry.Capabilities.Concentration {
				concentration++
				if concentration > 1 {
					return nil, fmt.Errorf("multiple concentration claims")
				}
			}
			part := library.Parts[leaf].Clone()
			result = append(result, dndfx.PartDeclaration{Key: key, Kind: part.Kind, Params: part.Params, When: part.When, Amount: part.Amount, OnTerminal: scope})
		}
	}
	return result, nil
}

// ValidateDefinitionReference checks a source's selection without participant IDs,
// runtime construction or a mutable catalogue accessor. Unknown/nil input refuses.
func ValidateDefinitionReference(in *ValidateDefinitionReferenceInput) error {
	if in == nil || in.Definitions == nil {
		return fmt.Errorf("compiled definitions are required")
	}
	if _, ok := in.Definitions.definitions[in.DefinitionID]; !ok {
		return fmt.Errorf("unknown definition %q", in.DefinitionID)
	}
	return nil
}
