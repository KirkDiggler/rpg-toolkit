// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package effects

import (
	"fmt"
	"slices"
	"strings"

	"github.com/KirkDiggler/rpg-toolkit/core"
)

// ResolveInput supplies inline or reference-bearing content. Catalogue is only
// needed when the authored value contains references; no runtime facts enter.
type ResolveInput struct {
	Definition AuthoredDefinition
	Catalogue  *Catalogue
}

// ResolveOutput owns a canonical definition with no unresolved references.
type ResolveOutput struct{ Definition DefinitionData }

// Resolve expands explicit selections/whole-member replacements and validates
// the result through the intrinsic compiler. Missing refs/paths, malformed forms,
// cycles or invalid content refuse without partial output or catalogue mutation.
func Resolve(in *ResolveInput) (*ResolveOutput, error) {
	if in == nil {
		return nil, fmt.Errorf("definition resolution input is required")
	}
	resolver := newDefinitionResolver()
	resolver.catalogue = in.Catalogue
	data, err := resolver.resolve(in.Definition)
	if err != nil {
		return nil, err
	}
	compiled, err := Compile(&CompileInput{Definition: data})
	if err != nil {
		return nil, err
	}
	return &ResolveOutput{Definition: compiled.Definition.Data()}, nil
}

type definitionReferenceKey struct {
	Ref  core.Ref
	Path string
}
type definitionResolver struct {
	entries      map[core.Ref]CatalogueEntry
	catalogue    *Catalogue
	references   map[definitionReferenceKey]bool
	compositions map[*AuthoredComposition]bool
	uses         map[*DefinitionUse]bool
}

func newDefinitionResolver() *definitionResolver {
	return &definitionResolver{references: map[definitionReferenceKey]bool{}, compositions: map[*AuthoredComposition]bool{}, uses: map[*DefinitionUse]bool{}}
}

func (r *definitionResolver) resolve(in AuthoredDefinition) (DefinitionData, error) {
	forms := 0
	if in.Effect != nil {
		forms++
	}
	if in.Composition != nil {
		forms++
	}
	if in.Use != nil {
		forms++
	}
	if forms != 1 {
		return DefinitionData{}, fmt.Errorf("authored definition requires exactly one effect, composition or use")
	}
	if in.Effect != nil {
		compiled, err := Compile(&CompileInput{Definition: DefinitionData{Effect: in.Effect}})
		if err != nil {
			return DefinitionData{}, err
		}
		return compiled.Definition.Data(), nil
	}
	if in.Composition != nil {
		if r.compositions[in.Composition] {
			return DefinitionData{}, fmt.Errorf("cyclic authored composition")
		}
		r.compositions[in.Composition] = true
		defer delete(r.compositions, in.Composition)
		if err := validateAuthoredKeys(in.Composition.Members); err != nil {
			return DefinitionData{}, err
		}
		members := make([]MemberData, 0, len(in.Composition.Members))
		for _, member := range in.Composition.Members {
			data, err := r.resolve(member.Definition)
			if err != nil {
				return DefinitionData{}, fmt.Errorf("member %q: %w", member.Key, err)
			}
			members = append(members, MemberData{Key: member.Key, Name: member.Name, Description: member.Description, Definition: data})
		}
		return DefinitionData{Composition: &CompositionData{Members: members}}, nil
	}
	use := in.Use
	if r.uses[use] {
		return DefinitionData{}, fmt.Errorf("cyclic definition use")
	}
	r.uses[use] = true
	defer delete(r.uses, use)
	selected, err := r.reference(use.Reference)
	if err != nil {
		return DefinitionData{}, err
	}
	replacements := make([]ResolvedMemberReplacement, 0, len(use.Replace))
	for _, replacement := range use.Replace {
		data, err := r.resolve(replacement.Definition)
		if err != nil {
			return DefinitionData{}, fmt.Errorf("replacement %v: %w", replacement.Path, err)
		}
		replacements = append(replacements, ResolvedMemberReplacement{Path: slices.Clone(replacement.Path), Definition: data})
	}
	derived, err := Derive(&DeriveInput{Definition: selected, Replace: replacements})
	if err != nil {
		return DefinitionData{}, err
	}
	return derived.Definition, nil
}

func (r *definitionResolver) reference(reference DefinitionReference) (DefinitionData, error) {
	if err := validateDefinitionReference(reference); err != nil {
		return DefinitionData{}, err
	}
	key := definitionReferenceKey{Ref: reference.Definition, Path: strings.Join(reference.Path, "/")}
	if r.references[key] {
		return DefinitionData{}, fmt.Errorf("cyclic definition reference %s at %v", reference.Definition.String(), reference.Path)
	}
	r.references[key] = true
	defer delete(r.references, key)
	if r.entries != nil {
		entry, ok := r.entries[reference.Definition]
		if !ok {
			return DefinitionData{}, fmt.Errorf("unknown definition %s", reference.Definition.String())
		}
		data, err := r.selectAuthored(entry.Definition, reference.Path)
		if err != nil {
			return DefinitionData{}, fmt.Errorf("definition %s path %v: %w", reference.Definition.String(), reference.Path, err)
		}
		return data, nil
	}
	if r.catalogue == nil {
		return DefinitionData{}, fmt.Errorf("catalogue is required for reference %s", reference.Definition.String())
	}
	entry, ok := r.catalogue.values[reference.Definition]
	if !ok {
		return DefinitionData{}, fmt.Errorf("unknown definition %s", reference.Definition.String())
	}
	selected, err := definitionAtPath(entry.definition, reference.Path)
	if err != nil {
		return DefinitionData{}, fmt.Errorf("definition %s path %v: %w", reference.Definition.String(), reference.Path, err)
	}
	return cloneDefinitionData(selected), nil
}

// Select raw members before expanding their siblings. That permits an acyclic
// reference to a concrete member of this same entry, while active expansion
// paths still reject actual cycles. Whole-catalogue construction checks all roots.
func (r *definitionResolver) selectAuthored(in AuthoredDefinition, path []string) (DefinitionData, error) {
	current := in
	for i, key := range path {
		if current.Use != nil {
			expanded, err := r.resolve(current)
			if err != nil {
				return DefinitionData{}, err
			}
			selected, err := definitionAtPath(expanded, path[i:])
			if err != nil {
				return DefinitionData{}, err
			}
			return cloneDefinitionData(selected), nil
		}
		if current.Composition == nil || current.Effect != nil {
			return DefinitionData{}, fmt.Errorf("cannot traverse definition at member %q", key)
		}
		if err := validateAuthoredKeys(current.Composition.Members); err != nil {
			return DefinitionData{}, err
		}
		found := false
		for _, member := range current.Composition.Members {
			if member.Key == key {
				current = member.Definition
				found = true
				break
			}
		}
		if !found {
			return DefinitionData{}, fmt.Errorf("unknown member %q", key)
		}
	}
	return r.resolve(current)
}

func validateAuthoredKeys(members []AuthoredMember) error {
	keys := make(map[string]bool, len(members))
	for _, member := range members {
		if !validMemberKey(member.Key) || keys[member.Key] {
			return fmt.Errorf("composition requires unique nonempty keys without '/': %q", member.Key)
		}
		keys[member.Key] = true
	}
	return nil
}
