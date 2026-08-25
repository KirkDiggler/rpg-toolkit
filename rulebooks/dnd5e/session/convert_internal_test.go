// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package session

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter"
	"github.com/KirkDiggler/rpg-toolkit/tools/spatial"
)

// convert_internal_test.go tests projectAtlas directly, white-box, because it
// is unexported and convert_test.go's external-package audit is deliberately
// structural (field NAMES match, by reflection) rather than a value check —
// it would pass even if a field were added to both types' definitions and
// never actually wired into the copy loop below. This file is the value half:
// proof that the bytes projectAtlas returns are the bytes it was handed, not
// just that the outer type has somewhere to put them.
//
// TestPropFacingAndOffsetCrossTheSeam pins rpg-project#261's projection: a
// straight copy, by design, of the two additive presentational fields —
// carried exactly as the composition reports them, including the "said
// nothing" zero values on a prop that authored neither.
func TestPropFacingAndOffsetCrossTheSeam(t *testing.T) {
	in := encounter.Atlas{
		Orientation: encounter.HexesArePointyTop(),
		Props: []encounter.AtlasProp{
			{
				Ref: "dnd5e:props:statue-reaper", At: spatial.Position{X: 3, Y: 4},
				BlocksMovement: true, BlocksLineOfSight: true,
				Facing: "se", Offset: [3]float64{0.2, -0.1, 0.6},
			},
			{
				Ref: "dnd5e:props:candles", At: spatial.Position{X: 5, Y: 6},
				BlocksMovement: false, BlocksLineOfSight: false,
			},
		},
	}

	out := projectAtlas(in)

	require.Len(t, out.Props, 2)
	require.Equal(t, "se", out.Props[0].Facing, "the exact authored word, uninterpreted")
	require.Equal(t, [3]float64{0.2, -0.1, 0.6}, out.Props[0].Offset, "the exact authored numbers, height included")

	require.Equal(t, "", out.Props[1].Facing, "said nothing")
	require.Equal(t, [3]float64{0, 0, 0}, out.Props[1].Offset, "and said zero/center-on-the-floor: the same fact by design")
}

// TestWallHeightCrossesTheSeam — the authored multiplier is copied verbatim
// (rpg-project#273), and a wall that authored none carries 0: the reader's
// word for "render the standard height", never a number to multiply by.
func TestWallHeightCrossesTheSeam(t *testing.T) {
	in := encounter.Atlas{
		Orientation: encounter.HexesArePointyTop(),
		Boundaries: []encounter.AtlasBoundary{
			{From: spatial.Position{X: 2, Y: 3}, To: spatial.Position{X: 3, Y: 3},
				BlocksMovement: true, BlocksLineOfSight: true, Height: 2.5},
			{From: spatial.Position{X: 2, Y: 4}, To: spatial.Position{X: 3, Y: 4},
				BlocksMovement: true, BlocksLineOfSight: true},
		},
	}

	out := projectAtlas(in)

	require.Len(t, out.Boundaries, 2)
	require.Equal(t, 2.5, out.Boundaries[0].Height, "the exact authored multiplier")
	require.Zero(t, out.Boundaries[1].Height, "no height authored: the standard height")
}
