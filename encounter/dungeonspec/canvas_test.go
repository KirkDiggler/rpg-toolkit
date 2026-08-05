package dungeonspec

import (
	"context"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

const canvasFixture = `version: 1
key: canvas-provider-contract
name: Canvas Provider Contract
height: 1
canvas: { width: 4, height: 2 }
rooms: []
start: [1, 1]
place:
  - { ref: "dnd5e:props:altar", at: [1, 0], facing: W }
walls:
  - { from: [1, 0], to: [1, 1], kind: door }
`

func Test883CanvasModeAndProviderContract(t *testing.T) {
	compiled, err := LoadWithConfig([]byte(canvasFixture), LoadConfig{PartyStartSeatCount: 4})
	require.NoError(t, err)
	plan, err := BuildFloorPlan(context.Background(), BuildFloorPlanInput{Compiled: compiled, Seed: 1})
	require.NoError(t, err)
	require.Empty(t, plan.Rooms)
	require.Equal(t, 4, plan.Width)
	require.Equal(t, 2, plan.Height)
	require.Equal(t, []FloorPlanCell{{0, 0}, {0, 1}, {1, 0}, {1, 1}, {2, 0}, {2, 1}, {3, 0}, {3, 1}}, plan.FloorCells)
	require.Equal(t, FloorPlanCell{1, 1}, plan.Entrance)
	require.Equal(t, []FloorPlanEdge{{
		From: FloorPlanCell{1, 0}, To: FloorPlanCell{1, 1},
		Kind:   FloorPlanEdgeKindDoor,
		DoorID: "canvas-provider-contract-authored-door-1--2-1--1--1-0",
	}}, plan.Edges)
}

func Test883CanvasModeMatrixAndShrink(t *testing.T) {
	_, err := Load([]byte(strings.Replace(canvasFixture, "rooms: []", "", 1)))
	require.ErrorContains(t, err, "rooms: []")
	_, err = Load([]byte(strings.Replace(
		canvasFixture, "rooms: []", "rooms:\n  - { id: x, archetype: entrance, width: 4 }", 1,
	)))
	require.ErrorContains(t, err, "rooms: []")
	invalidWall := strings.Replace(canvasFixture, "to: [1, 1]", "to: [8, 1]", 1)
	_, err = Load([]byte(invalidWall))
	require.ErrorContains(t, err, "out of dungeon floor footprint")
	previous, err := Load([]byte(canvasFixture))
	require.NoError(t, err)
	shrunk := strings.Replace(canvasFixture, "width: 4", "width: 1", 1)
	_, err = LoadWithPrevious([]byte(shrunk), LoadConfig{PartyStartSeatCount: 4}, previous)
	require.ErrorContains(t, err, "place[0]")
}

func Test883RoomChainProviderRetainsRuntimeProjection(t *testing.T) {
	const roomChain = `version: 1
key: room-provider
name: Room Provider
height: 8
rooms:
  - { id: entrance, archetype: entrance, width: 6 }
  - { id: boss, archetype: boss, width: 8, boss: { ref: "dnd5e:monsters:skeleton-captain", at: [4, 2] } }
connectors:
  - { from: entrance, to: boss }
`
	compiled, err := Load([]byte(roomChain))
	require.NoError(t, err)
	plan, err := BuildFloorPlan(context.Background(), BuildFloorPlanInput{Compiled: compiled, Seed: 3})
	require.NoError(t, err)
	require.Equal(t, 15, plan.Width)
	require.Equal(t, 8, plan.Height)
	require.Empty(t, plan.FloorCells,
		"room-chain floor_cells must remain absent; region-only membership omits connector cells")
	require.Equal(t, []FloorPlanRoom{
		{ID: "entrance", StartColumn: 0, Width: 6},
		{ID: "boss", StartColumn: 7, Width: 8},
	}, plan.Rooms)
	require.Equal(t, []FloorPlanConnector{{DoorID: "room-provider-door-entrance-boss"}}, plan.Connectors)
	require.Equal(t, FloorPlanCell{Column: 0, Row: 4}, plan.Entrance)
	require.NotEmpty(t, plan.Edges, "runtime generated and connector edges must be projected")
}

func TestCanvasMonsterPlacementRejectsPropOnlyFlagsAtFieldPath(t *testing.T) {
	for _, field := range []string{"blocks_movement", "blocks_los"} {
		t.Run(field, func(t *testing.T) {
			yaml := strings.Replace(canvasFixture,
				"  - { ref: \"dnd5e:props:altar\", at: [1, 0], facing: W }",
				"  - { ref: \"dnd5e:monsters:skeleton\", at: [1, 0], "+field+": false }", 1)
			_, err := Load([]byte(yaml))
			require.ErrorContains(t, err, "place[0]."+field)
			require.ErrorContains(t, err, "only valid on props")
		})
	}
}

func Test883CanvasFacingAndLockGrammar(t *testing.T) {
	for _, facing := range []string{"E", "NE", "NW", "W", "SW", "SE"} {
		_, err := Load([]byte(strings.Replace(canvasFixture, "facing: W", "facing: "+facing, 1)))
		require.NoError(t, err, facing)
	}
	locked := strings.Replace(
		canvasFixture, "kind: door", "kind: door, lock: { options: [ { ability: dex, dc: 15 } ] }", 1,
	)
	_, err := Load([]byte(locked))
	require.NoError(t, err)
}
