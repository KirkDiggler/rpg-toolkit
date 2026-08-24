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
				Facing: "se", Offset: [2]float64{0.2, -0.1},
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
	require.Equal(t, [2]float64{0.2, -0.1}, out.Props[0].Offset, "the exact authored numbers")

	require.Equal(t, "", out.Props[1].Facing, "said nothing")
	require.Equal(t, [2]float64{0, 0}, out.Props[1].Offset, "and said zero/center: the same fact by design")
}
