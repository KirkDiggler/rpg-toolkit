// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package effects

import "fmt"

// ResolvedMemberReplacement supplies a complete value at an existing member
// path. It contains no references, field-merge language or runtime mutation.
type ResolvedMemberReplacement struct {
	Path       []string
	Definition DefinitionData
}

// DeriveInput selects an owned copy of Definition with explicit replacements.
type DeriveInput struct {
	Definition DefinitionData
	Replace    []ResolvedMemberReplacement
}

// DeriveOutput owns the resulting canonical definition.
type DeriveOutput struct{ Definition DefinitionData }

// Derive validates the original shape and every replacement before modifying
// its owned copy. Empty/root, missing, duplicate and overlapping paths, or an
// invalid replacement, refuse with no partial result. Keys/order/metadata stay
// where authored; whole definition values replace, never merge, their targets.
func Derive(in *DeriveInput) (*DeriveOutput, error) {
	if in == nil {
		return nil, fmt.Errorf("definition derivation input is required")
	}
	base, err := Compile(&CompileInput{Definition: in.Definition})
	if err != nil {
		return nil, err
	}
	original := base.Definition.Data()
	replacements := make([]DefinitionData, 0, len(in.Replace))
	for i, replacement := range in.Replace {
		if len(replacement.Path) == 0 {
			return nil, fmt.Errorf("replacement requires a nonroot member path")
		}
		if _, err := definitionAtPath(original, replacement.Path); err != nil {
			return nil, fmt.Errorf("replacement %v: %w", replacement.Path, err)
		}
		for _, earlier := range in.Replace[:i] {
			if pathPrefix(earlier.Path, replacement.Path) || pathPrefix(replacement.Path, earlier.Path) {
				return nil, fmt.Errorf("replacement paths %v and %v overlap", earlier.Path, replacement.Path)
			}
		}
		compiled, err := Compile(&CompileInput{Definition: replacement.Definition})
		if err != nil {
			return nil, fmt.Errorf("replacement %v: %w", replacement.Path, err)
		}
		replacements = append(replacements, compiled.Definition.Data())
	}
	for i, replacement := range in.Replace {
		parent, err := definitionAtPath(original, replacement.Path[:len(replacement.Path)-1])
		if err != nil {
			return nil, err
		}
		key := replacement.Path[len(replacement.Path)-1]
		// All paths were checked against the original, with overlaps forbidden.
		// Replacing a sibling cannot invalidate this already-validated parent.
		for j := range parent.Composition.Members {
			if parent.Composition.Members[j].Key == key {
				parent.Composition.Members[j].Definition = replacements[i]
				break
			}
		}
	}
	return &DeriveOutput{Definition: original}, nil
}
