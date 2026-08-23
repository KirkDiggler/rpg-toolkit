// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package session_test

import (
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter"
	"github.com/KirkDiggler/rpg-toolkit/tools/spatial"
)

// regionfixtures_test.go is how this package's scenes paint a floor since
// regions replaced rooms in the composition (rpg-project#256). A scene that
// used to say "an 8x8 room" now says rectRegion("hall", 0, 0, 8, 8): the
// same cells, listed rather than described, with the archetype and lighting
// every region must carry supplied once here so no scene is secretly about
// them. Mirrors the composition's own fixture helpers on purpose: a scene
// that reads the same in both packages is one fewer translation to doubt.

// testArchetype is the presentation ref every fixture region carries. Never
// read by anything under test.
const testArchetype = "crypt"

// fullLight is the lighting every fixture region carries unless a scene is
// about light, in which case it says so itself.
func fullLight() *encounter.Lighting { return &encounter.Lighting{Intensity: 1} }

// rectRegion paints a w x h rectangle of authored offset cells whose top-left
// corner is [col,row], in row-major order.
func rectRegion(id string, col, row, w, h int) encounter.RegionInput {
	return encounter.RegionInput{
		ID:        id,
		Name:      id,
		Cells:     rectCells(col, row, w, h),
		Archetype: testArchetype,
		Lighting:  fullLight(),
	}
}

// rectCells is the authored offset cells of a w x h rectangle at [col,row].
func rectCells(col, row, w, h int) []spatial.Position {
	cells := make([]spatial.Position, 0, w*h)
	for r := 0; r < h; r++ {
		for c := 0; c < w; c++ {
			cells = append(cells, spatial.Position{X: float64(col + c), Y: float64(row + r)})
		}
	}
	return cells
}

// pointyCanvas is the declaration every fixture field makes unless a scene is
// about the void or the layout: opaque void, pointy-top hexes.
func pointyCanvas() encounter.CanvasInput {
	return encounter.CanvasInput{Void: encounter.VoidIsOpaque(), Orientation: encounter.HexesArePointyTop()}
}
