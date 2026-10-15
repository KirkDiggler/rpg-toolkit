// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package encounter

import (
	"testing"

	"github.com/stretchr/testify/require"
)

// TestVisibleCellsOfAnUnplacedMemberIsEmptyNotNil: every verb keeps members
// on the map, so an unplaced member is reachable only from inside. The
// contract still answers it — an empty, non-nil list, and no error — because
// "not on the map" is not a refusal and nil would read as "not sent".
func TestVisibleCellsOfAnUnplacedMemberIsEmptyNotNil(t *testing.T) {
	enc := failClosedEncounter(t)
	placed, err := enc.VisibleCellsFor(&VisibleCellsInput{Member: "walker"})
	require.NoError(t, err)
	require.NotEmpty(t, placed.Cells, "a placed member sees at least their own cell")

	require.NoError(t, enc.canvas.RemoveEntity("walker"))
	out, err := enc.VisibleCellsFor(&VisibleCellsInput{Member: "walker"})
	require.NoError(t, err)
	require.NotNil(t, out.Cells)
	require.Empty(t, out.Cells)
}
