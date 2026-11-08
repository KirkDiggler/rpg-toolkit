// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package session

import (
	"strings"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter"
)

// MaterialOverride is a captured opaque material choice for a declared visual
// scope. It belongs to its prop/wall, not a rule or a source lookup. Empty owner
// lists retain authored materials; both identities are nonblank and scope IDs
// are unique within the owning appearance.
type MaterialOverride struct {
	SurfaceID   string `json:"surface_id"`
	MaterialRef string `json:"material_ref"`
}

func projectMaterialOverrides(values []encounter.MaterialOverride) []MaterialOverride {
	if len(values) == 0 {
		return nil
	}
	out := make([]MaterialOverride, len(values))
	for i, value := range values {
		out[i] = MaterialOverride(value)
	}
	return out
}

func validMaterialOverrides(values []MaterialOverride) bool {
	seen := make(map[string]bool, len(values))
	for _, value := range values {
		if strings.TrimSpace(value.SurfaceID) == "" || strings.TrimSpace(value.MaterialRef) == "" || seen[value.SurfaceID] {
			return false
		}
		seen[value.SurfaceID] = true
	}
	return true
}
