// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package effects

import (
	"fmt"
	"slices"
	"strings"
)

// CompileInput supplies typed or canonical unbound content. No catalogue,
// participant or terminal routing is required to compile a definition value.
type CompileInput struct{ Definition Definition }

// CompileOutput contains a complete owned definition, never a partial result.
type CompileOutput struct{ Definition *CompiledDefinition }

// CompiledDefinition owns validated resolved data. Its methods return detached
// views; the source definition and later callers cannot rewrite this value.
type CompiledDefinition struct{ data DefinitionData }

// CompiledLeaf is an ordered derived view of one configured effect. Path is
// relative to its definition root and is not an instance/attachment identity.
type CompiledLeaf struct {
	Path   []string
	Effect EffectSpec
}

// Compile projects and validates the complete definition before returning an
// owned result. Absent/ambiguous forms, malformed meanings, invalid sibling
// keys and cyclic structure refuse without binding, activation or partial output.
func Compile(in *CompileInput) (*CompileOutput, error) {
	if in == nil {
		return nil, fmt.Errorf("definition compilation input is required")
	}
	data, err := projectDefinition(&projectionInput{Definition: in.Definition, Active: map[*Composition]bool{}})
	if err != nil {
		return nil, err
	}
	owned, err := compileData(&compileDataInput{Data: data, Active: map[*CompositionData]bool{}})
	if err != nil {
		return nil, err
	}
	return &CompileOutput{Definition: &CompiledDefinition{data: owned}}, nil
}

type compileDataInput struct {
	Data   DefinitionData
	Path   []string
	Active map[*CompositionData]bool
}

func validMemberKey(key string) bool {
	return strings.TrimSpace(key) != "" && !strings.Contains(key, "/")
}
func compileData(in *compileDataInput) (DefinitionData, error) {
	data := in.Data
	if (data.Effect != nil) == (data.Composition != nil) {
		return DefinitionData{}, fmt.Errorf("definition at %v requires exactly one effect or composition", in.Path)
	}
	if data.Effect != nil {
		if _, err := ValidateEffect(&ValidateEffectInput{Effect: *data.Effect}); err != nil {
			return DefinitionData{}, fmt.Errorf("effect at %v: %w", in.Path, err)
		}
		effect := data.Effect.Clone()
		return DefinitionData{Effect: &effect}, nil
	}
	if in.Active[data.Composition] {
		return DefinitionData{}, fmt.Errorf("cyclic composition at %v", in.Path)
	}
	in.Active[data.Composition] = true
	defer delete(in.Active, data.Composition)
	members := make([]MemberData, 0, len(data.Composition.Members))
	keys := make(map[string]bool, len(data.Composition.Members))
	for _, member := range data.Composition.Members {
		if !validMemberKey(member.Key) || keys[member.Key] {
			return DefinitionData{}, fmt.Errorf("composition at %v requires unique nonempty sibling keys without '/': %q", in.Path, member.Key)
		}
		keys[member.Key] = true
		path := append(slices.Clone(in.Path), member.Key)
		child, err := compileData(&compileDataInput{Data: member.Definition, Path: path, Active: in.Active})
		if err != nil {
			return DefinitionData{}, err
		}
		members = append(members, MemberData{Key: member.Key, Name: member.Name, Description: member.Description, Definition: child})
	}
	return DefinitionData{Composition: &CompositionData{Members: members}}, nil
}

// Data returns detached canonical data. An uninitialized handle returns the
// absent form, not a fabricated empty composition; placement refuses it.
func (c *CompiledDefinition) Data() DefinitionData {
	if c == nil {
		return DefinitionData{}
	}
	return cloneDefinitionData(c.data)
}
func cloneDefinitionData(data DefinitionData) DefinitionData {
	if data.Effect != nil {
		effect := data.Effect.Clone()
		return DefinitionData{Effect: &effect}
	}
	if data.Composition == nil {
		return DefinitionData{}
	}
	members := make([]MemberData, len(data.Composition.Members))
	for i, member := range data.Composition.Members {
		members[i] = MemberData{Key: member.Key, Name: member.Name, Description: member.Description, Definition: cloneDefinitionData(member.Definition)}
	}
	return DefinitionData{Composition: &CompositionData{Members: members}}
}

// Leaves returns owned effects in depth-first authored order. An empty root
// composition has no leaves; a root effect has an empty relative path.
func (c *CompiledDefinition) Leaves() []CompiledLeaf {
	out := []CompiledLeaf{}
	if c != nil {
		appendDefinitionLeaves(c.data, nil, &out)
	}
	return out
}
func appendDefinitionLeaves(data DefinitionData, path []string, out *[]CompiledLeaf) {
	if data.Effect != nil {
		*out = append(*out, CompiledLeaf{Path: slices.Clone(path), Effect: data.Effect.Clone()})
		return
	}
	if data.Composition == nil {
		return
	}
	for _, member := range data.Composition.Members {
		appendDefinitionLeaves(member.Definition, append(slices.Clone(path), member.Key), out)
	}
}
