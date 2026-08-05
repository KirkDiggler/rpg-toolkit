package dungeonspec_test

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/KirkDiggler/rpg-toolkit/encounter"
	"github.com/KirkDiggler/rpg-toolkit/encounter/core"
	"github.com/KirkDiggler/rpg-toolkit/encounter/dungeonspec"
	"github.com/KirkDiggler/rpg-toolkit/tools/spatial"
	"github.com/stretchr/testify/require"
)

const canvasRuntimeYAML = `version: 1
key: canvas-runtime
name: Canvas Runtime
height: 1
canvas: { width: 5, height: 3 }
rooms: []
start: [0, 0]
place:
  - { ref: "dnd5e:props:altar", at: [1, 0], facing: E }
  - { ref: "dnd5e:props:brazier", at: [2, 0], facing: NE, blocks_movement: false }
  - { ref: "dnd5e:props:coffin", at: [3, 0], facing: NW }
  - { ref: "dnd5e:props:statue-reaper", at: [4, 0], facing: W }
  - { ref: "dnd5e:props:pillar", at: [1, 1], facing: SW }
  - { ref: "dnd5e:props:candles", at: [2, 1], facing: SE }
  - { ref: "dnd5e:props:bone-pile", at: [3, 1] }
  - { ref: "dnd5e:monsters:skeleton", at: [4, 2] }
walls:
  - { from: [1, 2], to: [1, 1], kind: door, lock: { options: [ { ability: dex, dc: 15 }, { ability: str, dc: 10 } ] } }
`

func TestCanvasRuntime_InitializesPersistsAndSeedsAbsoluteContent(t *testing.T) {
	compiled, err := dungeonspec.LoadWithConfig([]byte(canvasRuntimeYAML), dungeonspec.LoadConfig{PartyStartSeatCount: 4})
	require.NoError(t, err)
	require.NotNil(t, compiled.Params.Canvas)
	require.Empty(t, compiled.Params.Regions)
	require.Len(t, compiled.Params.Canvas.Cells, 15)
	require.Len(t, compiled.Params.CanvasPlacedObstacles, 7)
	require.Len(t, compiled.Spawns, 1)
	require.Empty(t, compiled.Spawns[0].RoomID)
	require.Nil(t, compiled.Spawns[0].At)
	require.NotNil(t, compiled.Spawns[0].AbsoluteAt)

	transport := encounter.NewInMemoryTransport()
	broker := encounter.NewBroker(transport)
	t.Cleanup(func() { _ = broker.Close(); _ = transport.Close() })
	enc := encounter.New(context.Background(), "canvas-runtime", broker)
	compiled.Params.RandomSeed = 27
	require.NoError(t, enc.InitDungeon(compiled.Params))
	require.NoError(t, enc.SeedMonsters(compiled.Spawns))

	data := enc.ToData()
	require.NotNil(t, data.Space.Canvas)
	require.Equal(t, compiled.Params.Canvas, data.Space.Canvas)
	require.Empty(t, data.Space.Regions, "canvas mode must not infer regions")
	require.Equal(t, core.HexFromPosition(spatial.Position{X: 0, Y: 0}), data.Space.Entrance)
	require.Len(t, data.Space.Obstacles, 7)
	require.Len(t, data.Monsters, 1)
	require.Contains(t, data.Monsters, core.EntityID("monster-canvas-0"))
	require.Equal(t, core.HexFromPosition(spatial.Position{X: 4, Y: 2}), data.Monsters["monster-canvas-0"].Position)

	for index, obstacle := range data.Space.Obstacles {
		if index == 6 {
			require.Nil(t, obstacle.Facing, "omitted facing remains absent")
			continue
		}
		require.NotNil(t, obstacle.Facing)
		require.Equal(t, uint32(index), *obstacle.Facing)
	}
	door := data.Doors["canvas-runtime-authored-door-1--3-2--1--2-1"]
	require.NotNil(t, door)
	require.False(t, door.Locked, "lock grammar is metadata, never executable lock state")
	require.Equal(t,
		[]encounter.AuthoredLockOption{{Ability: "dex", DC: 15}, {Ability: "str", DC: 10}},
		data.Space.AuthoredEdges[0].LockOptions,
	)

	positions, err := enc.ResolvePartySpawnPositions(encounter.ResolvePartySpawnPositionsInput{Count: 4})
	require.NoError(t, err)
	require.Equal(t, data.Space.Entrance, positions.Positions[0])
	_, err = enc.ResolvePartySpawnPositions(encounter.ResolvePartySpawnPositionsInput{Count: 5})
	require.ErrorContains(t, err, "5 position(s) exceeds 4")

	plan, err := dungeonspec.BuildFloorPlan(context.Background(), dungeonspec.BuildFloorPlanInput{
		Compiled: compiled, Seed: 27,
	})
	require.NoError(t, err)
	require.Equal(t, dungeonspec.FloorPlanCell{Column: 0, Row: 0}, plan.Entrance)
	require.Len(t, plan.FloorCells, 15)
	require.Equal(t, "canvas-runtime-authored-door-1--3-2--1--2-1", plan.Edges[0].DoorID)

	payload, err := json.Marshal(data)
	require.NoError(t, err)
	var persisted encounter.Data
	require.NoError(t, json.Unmarshal(payload, &persisted))
	reloaded, err := encounter.LoadFromData(context.Background(), &persisted, broker)
	require.NoError(t, err)
	require.Equal(t, data.Space.Canvas, reloaded.ToData().Space.Canvas)
	require.Equal(t, positions.Positions, reloaded.ToData().Space.PartyStartPositions)
	require.Equal(t, data.Space.AuthoredEdges, reloaded.ToData().Space.AuthoredEdges)
}

func TestCanvasRuntime_StaticFacingAndCollisionDefenses(t *testing.T) {
	for _, replacement := range []string{"E", "NE", "NW", "W", "SW", "SE", "null"} {
		yaml := strings.Replace(canvasRuntimeYAML, "facing: E", "facing: "+replacement, 1)
		_, err := dungeonspec.Load([]byte(yaml))
		require.NoError(t, err, replacement)
	}
	monsterFacing := strings.Replace(canvasRuntimeYAML, "at: [4, 2] }", "at: [4, 2], facing: E }", 1)
	_, err := dungeonspec.Load([]byte(monsterFacing))
	require.ErrorContains(t, err, "place[7].facing")
	require.ErrorContains(t, err, "facing only supported on floor props")
	floorMounted := strings.Replace(canvasRuntimeYAML, "facing: E", "facing: E, mount: floor", 1)
	_, err = dungeonspec.Load([]byte(floorMounted))
	require.NoError(t, err)
	wallFacing := strings.Replace(canvasRuntimeYAML, "facing: E", "facing: E, mount: wall", 1)
	_, err = dungeonspec.Load([]byte(wallFacing))
	require.ErrorContains(t, err, "facing only supported on floor props")
	startCollision := strings.Replace(canvasRuntimeYAML, "start: [0, 0]", "start: [1, 0]", 1)
	_, err = dungeonspec.Load([]byte(startCollision))
	require.ErrorContains(t, err, "start [1 0] conflicts")
}
