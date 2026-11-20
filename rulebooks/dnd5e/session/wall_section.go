// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package session

import (
	"encoding/json"
	"fmt"
	"math"
	"strings"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter"
)

// WallSection is an immutable visual span carried with its permitted wall.
// Position and Width are canonical feet; Ref is opaque. It supplies no opening,
// blocker, independent prop identity or door state. IDs are wall-local.
type WallSection struct {
	ID       string  `json:"id"`
	Ref      string  `json:"ref"`
	Position float64 `json:"position"`
	Width    float64 `json:"width"`
}

// UnmarshalJSON requires captured numeric facts to be present. An explicit
// zero centre is not the same as an absent/null field; interval validity stays
// with the composition rather than being reconstructed from a live world here.
func (value *WallSection) UnmarshalJSON(data []byte) error {
	var row struct {
		ID       string   `json:"id"`
		Ref      string   `json:"ref"`
		Position *float64 `json:"position"`
		Width    *float64 `json:"width"`
	}
	if err := json.Unmarshal(data, &row); err != nil {
		return err
	}
	if row.Position == nil || row.Width == nil {
		return fmt.Errorf("wall section requires position and width")
	}
	*value = WallSection{ID: row.ID, Ref: row.Ref, Position: *row.Position, Width: *row.Width}
	return nil
}

func projectWallSections(values []encounter.WallSection) []WallSection {
	if len(values) == 0 {
		return nil
	}
	out := make([]WallSection, len(values))
	for i, value := range values {
		out[i] = WallSection{ID: value.ID, Ref: value.Ref, Position: value.Position, Width: value.Width}
	}
	return out
}

func validWallSections(values []WallSection) bool {
	seen := make(map[string]bool, len(values))
	for _, value := range values {
		if strings.TrimSpace(value.ID) == "" || strings.TrimSpace(value.Ref) == "" || seen[value.ID] {
			return false
		}
		if math.IsNaN(value.Position) || math.IsInf(value.Position, 0) || math.IsNaN(value.Width) || math.IsInf(value.Width, 0) || value.Width <= 0 {
			return false
		}
		seen[value.ID] = true
	}
	return true
}
