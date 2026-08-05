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
	previous, err := Load([]byte(canvasFixture))
	require.NoError(t, err)
	shrunk := strings.Replace(canvasFixture, "width: 4", "width: 1", 1)
	_, err = LoadWithPrevious([]byte(shrunk), LoadConfig{PartyStartSeatCount: 4}, previous)
	require.ErrorContains(t, err, "place[0]")
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
