// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package resolution

import (
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter"
	"github.com/KirkDiggler/rpg-toolkit/tools/spatial"
)

// rectRegion paints a rectangular floor in the encounter's one absolute frame,
// the same way the composition's own suites do.
//
// These fixtures used to say Rooms + RoomInput{Width, Height} and give each
// member a Room, because this package's tests were pinned to an encounter two
// world-models old (v0.30.6) while its non-test code was already compatible
// with the current one. Nothing caught it: a module's tests compile against its
// OWN pin, and only a consumer linking everything together picks the newer
// version by MVS. So the suite was green against a shape the product had
// stopped building — which is this slice's recurring theme, one layer over.
func rectRegion(id string, col, row, w, h int) encounter.RegionInput {
	cells := make([]spatial.Position, 0, w*h)
	for r := 0; r < h; r++ {
		for c := 0; c < w; c++ {
			cells = append(cells, spatial.Position{X: float64(col + c), Y: float64(row + r)})
		}
	}
	return encounter.RegionInput{
		ID: id, Name: id, Cells: cells, Archetype: "crypt",
		Lighting: &encounter.Lighting{Intensity: 1},
	}
}

// hexCanvas is the orientation the product actually runs on. Square grids stay
// an option in the toolkit; the game is hex exclusively.
func hexCanvas() encounter.CanvasInput {
	return encounter.CanvasInput{Void: encounter.VoidIsOpaque(), Orientation: encounter.HexesArePointyTop()}
}
