// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package effects

import (
	"fmt"
	"strings"

	generic "github.com/KirkDiggler/rpg-toolkit/mechanics/effects"
)

// Mount selects a definition subtree and supplies its local placement key and
// terminal routing. It contains no creature, source or application identity.
type Mount struct {
	Key        string
	Path       []string
	OnTerminal generic.EndScope
}

// PlaceInput explicitly covers a compiled definition with nonoverlapping mounts.
type PlaceInput struct {
	Definition *CompiledDefinition
	Mounts     []Mount
}

// PlacedPart preserves the mount association for the host that supplies roles.
// Declaration is owned, fully configured local data, not an attached runtime.
type PlacedPart struct {
	MountKey    string
	Declaration PartDeclaration
}

// PlaceOutput owns all placed parts in the definition's authored leaf order.
type PlaceOutput struct{ Parts []PlacedPart }

// Place lowers explicit scope choices without inventing a holder or catalogue.
// It refuses absent definitions, invalid/duplicate mount keys, unknown/overlapping
// paths, uncovered leaves and unsupported terminal scopes with no partial output.
// Mount order cannot reorder the authored definition. An explicit empty
// composition needs no mounts and produces an empty result.
func Place(in *PlaceInput) (*PlaceOutput, error) {
	if in == nil || in.Definition == nil || (in.Definition.data.Effect == nil && in.Definition.data.Composition == nil) {
		return nil, fmt.Errorf("compiled definition is required for placement")
	}
	keys := make(map[string]bool, len(in.Mounts))
	for i, mount := range in.Mounts {
		if !validMemberKey(mount.Key) || keys[mount.Key] {
			return nil, fmt.Errorf("placement requires unique nonempty mount keys without '/': %q", mount.Key)
		}
		keys[mount.Key] = true
		if mount.OnTerminal != generic.EndPart && mount.OnTerminal != generic.EndAttachment && mount.OnTerminal != generic.EndApplication {
			return nil, fmt.Errorf("mount %q has unknown terminal scope %q", mount.Key, mount.OnTerminal)
		}
		if _, err := definitionAtPath(in.Definition.data, mount.Path); err != nil {
			return nil, fmt.Errorf("mount %q: %w", mount.Key, err)
		}
		for _, other := range in.Mounts[:i] {
			if pathPrefix(mount.Path, other.Path) || pathPrefix(other.Path, mount.Path) {
				return nil, fmt.Errorf("mounts %q and %q overlap", mount.Key, other.Key)
			}
		}
	}
	out := &PlaceOutput{Parts: []PlacedPart{}}
	for _, leaf := range in.Definition.Leaves() {
		var chosen *Mount
		for i := range in.Mounts {
			if pathPrefix(in.Mounts[i].Path, leaf.Path) {
				chosen = &in.Mounts[i]
				break
			}
		}
		if chosen == nil {
			return nil, fmt.Errorf("definition leaf %v has no placement", leaf.Path)
		}
		key := chosen.Key
		if suffix := leaf.Path[len(chosen.Path):]; len(suffix) > 0 {
			key += "/" + strings.Join(suffix, "/")
		}
		part := PartDeclaration{Key: key, Kind: leaf.Effect.Kind, Params: leaf.Effect.Params, When: leaf.Effect.When, Amount: leaf.Effect.Amount, OnTerminal: chosen.OnTerminal}
		if err := ValidatePart(&ValidatePartInput{Declaration: part}); err != nil {
			return nil, fmt.Errorf("mount %q leaf %v: %w", chosen.Key, leaf.Path, err)
		}
		out.Parts = append(out.Parts, PlacedPart{MountKey: chosen.Key, Declaration: part.Clone()})
	}
	return out, nil
}

func pathPrefix(prefix, path []string) bool {
	if len(prefix) > len(path) {
		return false
	}
	for i, key := range prefix {
		if key != path[i] {
			return false
		}
	}
	return true
}
func definitionAtPath(data DefinitionData, path []string) (DefinitionData, error) {
	current := data
	for _, key := range path {
		if !validMemberKey(key) {
			return DefinitionData{}, fmt.Errorf("invalid member key %q", key)
		}
		if current.Composition == nil {
			return DefinitionData{}, fmt.Errorf("cannot traverse through an effect at %q", key)
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
	return current, nil
}
