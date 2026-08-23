// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package encounter

import "github.com/KirkDiggler/rpg-toolkit/tools/spatial"

// The internal twin of regionfixtures_test.go: test helpers do not cross a
// package boundary, and the internal suites need the same way to paint a floor.

func rectRegion(id string, col, row, w, h int) RegionInput {
	cells := make([]spatial.Position, 0, w*h)
	for r := 0; r < h; r++ {
		for c := 0; c < w; c++ {
			cells = append(cells, spatial.Position{X: float64(col + c), Y: float64(row + r)})
		}
	}
	return RegionInput{ID: id, Name: id, Cells: cells, Archetype: "crypt", Lighting: &Lighting{Intensity: 1}}
}

func openAir() CanvasInput {
	return CanvasInput{Void: VoidIsTransparent(), Orientation: HexesArePointyTop()}
}
