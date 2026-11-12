// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package dungeonspec

import (
	"strings"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter"
	"gopkg.in/yaml.v3"
)

// readWallFaceFinishes reads explicit native wall-face choices, not regional
// defaults or asset bindings. Unknown editor-only keys remain in the source node.
func readWallFaceFinishes(scene *yaml.Node, walls []RoomWall, add errSink) map[string][]encounter.WallFaceFinish {
	raw := childNode(scene, "wallFinishes")
	if raw == nil {
		return nil
	}
	const path = "room.scene.wallFinishes"
	node := resolveNode(raw)
	if node == nil || isNull(node) {
		add(path, errNotNull)
		return nil
	}
	if node.Kind != yaml.MappingNode || len(node.Content)%2 != 0 {
		add(path, errNotAMapping)
		return nil
	}
	known := make(map[string]bool, len(walls))
	for _, wall := range walls {
		known[wall.ID] = true
	}
	out := make(map[string][]encounter.WallFaceFinish, len(node.Content)/2)
	seen := make(map[string]bool, len(node.Content)/2)
	for i := 0; i < len(node.Content); i += 2 {
		key := resolveNode(node.Content[i])
		if key == nil || key.Kind != yaml.ScalarNode || key.Tag != "!!str" || strings.TrimSpace(key.Value) == "" {
			add(path, "wall target must be a nonblank string")
			continue
		}
		id := key.Value
		at := path + "." + id
		if seen[id] {
			add(at, "duplicate wall finish target")
			continue
		}
		seen[id] = true
		if !known[id] {
			add(at, "wall target does not exist")
			continue
		}
		out[id] = readWallFaceFinishRow(resolveNode(node.Content[i+1]), at, add)
	}
	return out
}

func readWallFaceFinishRow(row *yaml.Node, path string, add errSink) []encounter.WallFaceFinish {
	if row == nil || isNull(row) {
		add(path, errNotNull)
		return nil
	}
	if row.Kind != yaml.MappingNode || len(row.Content)%2 != 0 {
		add(path, errNotAMapping)
		return nil
	}
	if len(row.Content) == 0 {
		add(path, "omit an empty wall finish override")
		return nil
	}
	var out []encounter.WallFaceFinish
	for _, side := range []encounter.WallFaceSide{encounter.WallFacePositiveZ, encounter.WallFaceNegativeZ} {
		count := 0
		for i := 0; i < len(row.Content); i += 2 {
			key := resolveNode(row.Content[i])
			if key != nil && key.Kind == yaml.ScalarNode && key.Value == string(side) {
				count++
			}
		}
		if count == 0 {
			continue
		}
		if count > 1 {
			add(path+"."+string(side), "duplicate wall face finish")
			continue
		}
		ref := requireString(row, string(side), path, add)
		if ref == nil {
			continue
		}
		if strings.TrimSpace(ref.Value) == "" {
			add(path+"."+string(side), errRequired)
			continue
		}
		out = append(out, encounter.WallFaceFinish{Side: side, MaterialRef: ref.Value})
	}
	return out
}
