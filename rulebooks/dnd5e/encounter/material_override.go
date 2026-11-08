// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package encounter

import (
	"fmt"
	"strings"
)

// MaterialOverride is an opaque visual choice for one provider-declared scope.
// It is captured with a prop or wall, never interpreted as a rule or resolved
// against an asset catalog here. SurfaceID is unique within its owner; both
// values must be nonblank. An absent list retains original authored materials.
type MaterialOverride struct {
	SurfaceID   string
	MaterialRef string
}

// MaterialOverrideData persists a captured visual choice without resource bytes
// or editor ownership. The containing owner decides its disclosure/lifetime.
type MaterialOverrideData struct {
	SurfaceID   string `json:"surface_id"`
	MaterialRef string `json:"material_ref"`
}

func validateMaterialOverrides(values []MaterialOverride) error {
	seen := make(map[string]bool, len(values))
	for _, value := range values {
		if strings.TrimSpace(value.SurfaceID) == "" || strings.TrimSpace(value.MaterialRef) == "" {
			return fmt.Errorf("material override needs nonblank surface and material identities: %w", ErrNoField)
		}
		if seen[value.SurfaceID] {
			return fmt.Errorf("duplicate material override surface %q: %w", value.SurfaceID, ErrNoField)
		}
		seen[value.SurfaceID] = true
	}
	return nil
}

func cloneMaterialOverrides(values []MaterialOverride) []MaterialOverride {
	return append([]MaterialOverride(nil), values...)
}

func materialOverridesData(values []MaterialOverride) []MaterialOverrideData {
	if len(values) == 0 {
		return nil
	}
	result := make([]MaterialOverrideData, len(values))
	for i, value := range values {
		result[i] = MaterialOverrideData(value)
	}
	return result
}

func materialOverridesFromData(values []MaterialOverrideData) []MaterialOverride {
	if len(values) == 0 {
		return nil
	}
	result := make([]MaterialOverride, len(values))
	for i, value := range values {
		result[i] = MaterialOverride(value)
	}
	return result
}
