// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package encounter

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/KirkDiggler/rpg-toolkit/tools/spatial"
)

// TestPlacementIndex_AnswersOnlyTheAuthoredPose: the index answers for a
// placement only in the exact pose it measured. Moved, turned, offset or
// reshaped, the placement is measured at query time instead.
func TestPlacementIndex_AnswersOnlyTheAuthoredPose(t *testing.T) {
	authored := spatial.FootprintPlacement{
		Footprint: spatial.Footprint{Box: &spatial.Box{W: 5, D: 15}}, Origin: spatial.Point{X: 10, Y: 20},
	}
	idx := &placementIndex{
		authoredAt: map[PropID]spatial.FootprintPlacement{"slab": authored},
		stands:     map[PropID][]spatial.Position{"slab": {{X: 1, Y: 1}}},
	}
	same := authored
	same.Footprint.Box = &spatial.Box{W: 5, D: 15}
	_, ok := idx.standsAt("slab", same)
	require.True(t, ok, "the same pose, a different box pointer, is the authored one")

	moved, turned, offset, reshaped := authored, authored, authored, authored
	moved.Origin = spatial.Point{X: 15, Y: 20}
	turned.Facing = 90
	offset.LocalOffset = spatial.Point{X: 1}
	reshaped.Footprint.Box = &spatial.Box{W: 5, D: 10}
	for name, p := range map[string]spatial.FootprintPlacement{
		"moved": moved, "turned": turned, "offset": offset, "reshaped": reshaped,
	} {
		_, ok := idx.standsAt("slab", p)
		require.False(t, ok, "%s: not the pose the index measured", name)
	}
}
