package encounter

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/KirkDiggler/rpg-toolkit/encounter/core"
	"github.com/KirkDiggler/rpg-toolkit/tools/spatial"
)

type discontinuousRayGrid struct {
	*spatial.HexGrid
	ray []spatial.Position
}

func (g *discontinuousRayGrid) GetLineOfSight(_, _ spatial.Position) []spatial.Position {
	return g.ray
}

// TestTruncateAtWall_FailsClosedOnDiscontinuousRay protects the encounter
// movement layer from a grid implementation that omits interior direct-ray
// cells. A sparse caller request must never turn a malformed ray into a wall
// or boundary bypass.
func TestTruncateAtWall_FailsClosedOnDiscontinuousRay(t *testing.T) {
	from := core.HexFromPosition(spatial.Position{X: 1, Y: 1})
	to := core.HexFromPosition(spatial.Position{X: 3, Y: 1})
	fromPos, toPos := from.ToPosition(), to.ToPosition()
	grid := &discontinuousRayGrid{
		HexGrid: spatial.NewHexGrid(spatial.HexGridConfig{Width: 10, Height: 10}),
		ray:     []spatial.Position{fromPos, toPos},
	}
	room := spatial.NewBasicRoom(spatial.BasicRoomConfig{ID: "malformed-ray", Type: "test", Grid: grid})
	enc := &Encounter{room: room}

	require.Empty(t, enc.truncateAtWall(from, []core.Hex{to}))
}
