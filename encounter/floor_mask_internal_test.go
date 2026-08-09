package encounter

import (
	"context"
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/KirkDiggler/rpg-toolkit/encounter/core"
	"github.com/KirkDiggler/rpg-toolkit/tools/spatial"
)

type FloorMaskReachSuite struct{ suite.Suite }

func TestFloorMaskReachSuite(t *testing.T) { suite.Run(t, new(FloorMaskReachSuite)) }

func (s *FloorMaskReachSuite) TestTargetReachRejectsLineAcrossVoid() {
	ctx := context.Background()
	transport := NewInMemoryTransport()
	broker := NewBroker(transport)
	defer func() { _ = broker.Close(); _ = transport.Close() }()
	enc := New(ctx, "floor-mask-reach", broker)
	cells := [][2]int{{1, 1}, {1, 2}, {1, 3}, {2, 1}, {2, 3}, {3, 1}, {3, 2}, {3, 3}}
	floor := make([]core.Hex, len(cells))
	for index, cell := range cells {
		floor[index] = core.HexFromPosition(spatial.Position{X: float64(cell[0]), Y: float64(cell[1])})
	}
	anchor := floor[0]
	s.Require().NoError(enc.InitDungeon(DungeonParams{
		FloorSource: FloorSourceCanvas, Width: 5, Height: 5, FloorCells: floor,
		RequireConnectedFloor: true, PartyStart: PartyStartParams{Anchor: &anchor, SeatCount: 1},
	}))
	acrossVoid := core.HexFromPosition(spatial.Position{X: 3, Y: 3})
	s.False(enc.hasClearStructuralReach(anchor, acrossVoid))
	s.True(enc.hasClearStructuralReach(anchor, core.HexFromPosition(spatial.Position{X: 1, Y: 2})))
}
