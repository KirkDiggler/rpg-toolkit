// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package session

import (
	"strings"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter"
)

// WallFaceFinish is a captured visual selection in a wall's local frame.
// Side is positive-z or negative-z, unique within its wall; MaterialRef is
// nonblank and opaque. Absent lists apply no explicit face material.
type WallFaceFinish struct {
	Side        string `json:"side"`
	MaterialRef string `json:"material_ref"`
}

func projectWallFaceFinishes(values []encounter.WallFaceFinish) []WallFaceFinish {
	if len(values) == 0 {
		return nil
	}
	out := make([]WallFaceFinish, len(values))
	for i, value := range values {
		out[i] = WallFaceFinish{Side: string(value.Side), MaterialRef: value.MaterialRef}
	}
	return out
}

func validWallFaceFinishes(values []WallFaceFinish) bool {
	var seen uint8
	for _, value := range values {
		var bit uint8
		switch value.Side {
		case string(encounter.WallFacePositiveZ):
			bit = 1
		case string(encounter.WallFaceNegativeZ):
			bit = 2
		default:
			return false
		}
		if strings.TrimSpace(value.MaterialRef) == "" || seen&bit != 0 {
			return false
		}
		seen |= bit
	}
	return true
}
