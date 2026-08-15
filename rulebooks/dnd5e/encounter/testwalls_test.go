// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package encounter_test

import (
	"github.com/KirkDiggler/rpg-toolkit/tools/spatial"
)

// A LONE PILLAR NO LONGER BLOCKS ANYTHING, and these helpers exist because of
// it.
//
// Sight used to be one rasterized line, so a single occluding cell sitting on
// it stopped the sightline dead. That was the defect behind rpg-toolkit#1022:
// measured against the rule 5e actually states — visible if a line from any
// corner of your space to any corner of theirs is unobstructed — the old rule
// blocked 4.5x too many pairs on hexes and hid about one in seven things a
// player should have seen. spatial v0.9.1 asks for a LANE instead: blocked
// only when the direct lane AND every lane from a neighbour closer to the
// target are obstructed. You lean around a pillar, exactly as you would at the
// table.
//
// So a fixture that wants sight BLOCKED needs a wall, not a pillar. That is
// not a workaround — the old fixtures were describing a rule that was wrong,
// and reading one of them now ("the pillar blocks sight") tells you something
// the engine no longer believes.
//
// The width is not decoration either. A wall must be wide enough that the
// neighbour lanes are obstructed too, which means covering the cells a viewer
// would lean toward, not just the one on the centre line.

// wallRow returns a horizontal run of occluder cells at the given row,
// inclusive of both ends.
func wallRow(y, fromX, toX int) []spatial.Position {
	out := make([]spatial.Position, 0, toX-fromX+1)
	for x := fromX; x <= toX; x++ {
		out = append(out, spatial.Position{X: float64(x), Y: float64(y)})
	}
	return out
}

// wallColumn returns a vertical run of occluder cells in the given column,
// inclusive of both ends.
func wallColumn(x, fromY, toY int) []spatial.Position {
	out := make([]spatial.Position, 0, toY-fromY+1)
	for y := fromY; y <= toY; y++ {
		out = append(out, spatial.Position{X: float64(x), Y: float64(y)})
	}
	return out
}
