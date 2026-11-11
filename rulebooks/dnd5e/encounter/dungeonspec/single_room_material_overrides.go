// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package dungeonspec

import (
	"fmt"
	"strings"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter"
	"gopkg.in/yaml.v3"
)

// RoomMaterialOverride names one visual scope and shared material in native
// authoring data. It supplies no asset lookup, unit conversion or game rule.
type RoomMaterialOverride struct {
	SurfaceID   string `yaml:"surfaceId" json:"surfaceId"`
	MaterialRef string `yaml:"materialRef" json:"materialRef"`
}

func readMaterialOverrides(owner *yaml.Node, path string, add errSink) []RoomMaterialOverride {
	raw := childNode(owner, "materialOverrides")
	if raw == nil {
		return nil
	}
	path += ".materialOverrides"
	node := resolveNode(raw)
	if node == nil || isNull(node) {
		add(path, errNotNull)
		return nil
	}
	if node.Kind != yaml.SequenceNode {
		add(path, errNotAList)
		return nil
	}
	result := make([]RoomMaterialOverride, 0, len(node.Content))
	seen := make(map[string]bool, len(node.Content))
	for i, item := range node.Content {
		at := fmt.Sprintf("%s[%d]", path, i)
		item = resolveNode(item)
		if item == nil || isNull(item) {
			add(at, errNotNull)
			continue
		}
		if item.Kind != yaml.MappingNode {
			add(at, errNotAMapping)
			continue
		}
		surface := requireString(item, "surfaceId", at, add)
		material := requireString(item, "materialRef", at, add)
		if surface == nil || material == nil {
			continue
		}
		if strings.TrimSpace(surface.Value) == "" {
			add(at+".surfaceId", errRequired)
		}
		if strings.TrimSpace(material.Value) == "" {
			add(at+".materialRef", errRequired)
		}
		if seen[surface.Value] {
			add(at+".surfaceId", "duplicate material override surface")
		}
		seen[surface.Value] = true
		result = append(result, RoomMaterialOverride{SurfaceID: surface.Value, MaterialRef: material.Value})
	}
	return result
}

func materialOverridesOf(values []RoomMaterialOverride) []encounter.MaterialOverride {
	if len(values) == 0 {
		return nil
	}
	out := make([]encounter.MaterialOverride, len(values))
	for i, value := range values {
		out[i] = encounter.MaterialOverride{SurfaceID: value.SurfaceID, MaterialRef: value.MaterialRef}
	}
	return out
}
