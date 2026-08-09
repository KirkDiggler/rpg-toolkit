package dungeonspec

import (
	"context"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/monster"
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
	require.Empty(t, plan.Connectors)
	require.Zero(t, plan.DoorRow)
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

func Test886RoomChainProviderProjectsCanonicalMetadata(t *testing.T) {
	const (
		galleryRoomID  = "gallery"
		crossingRoomID = "crossing"
	)
	const roomChain = `version: 1
key: room-provider
name: Room Provider
height: 8
rooms:
  - { id: entrance, archetype: entrance, width: 6 }
  - { id: gallery, archetype: chamber, width: 8 }
  - { id: crossing, archetype: corridor, width: 10 }
  - { id: boss, archetype: boss, width: 12, boss: { ref: "dnd5e:monsters:skeleton-captain", at: [4, 2] } }
connectors:
  - { from: entrance, to: gallery }
  - { from: gallery, to: crossing, locked: { dc: 12, ability: dex } }
  - { from: crossing, to: boss }
walls:
  - { from: [0, 0], to: [1, 0], kind: door }
`
	compiled, err := Load([]byte(roomChain))
	require.NoError(t, err)
	plan, err := BuildFloorPlan(context.Background(), BuildFloorPlanInput{Compiled: compiled, Seed: 3})
	require.NoError(t, err)

	require.Zero(t, plan.Width, "width is a canvas-only wire field")
	require.Equal(t, 8, plan.Height)
	require.Equal(t, 4, plan.DoorRow)
	require.Nil(t, plan.FloorCells,
		"room-chain floor_cells must remain absent; region-only membership omits connector cells")
	require.Equal(t, []FloorPlanRoom{
		{ID: archetypeEntrance, Archetype: archetypeEntrance, StartColumn: 0, Width: 6},
		{ID: galleryRoomID, Archetype: archetypeChamber, StartColumn: 7, Width: 8},
		{ID: crossingRoomID, Archetype: archetypeCorridor, StartColumn: 16, Width: 10},
		{ID: archetypeBoss, Archetype: archetypeBoss, StartColumn: 27, Width: 12},
	}, plan.Rooms)
	require.Equal(t, []FloorPlanConnector{
		{
			DoorID: "room-provider-door-entrance-gallery", FromRoomID: archetypeEntrance, ToRoomID: galleryRoomID, Column: 6,
		},
		{
			DoorID: "room-provider-door-gallery-crossing", Locked: true,
			FromRoomID: galleryRoomID, ToRoomID: crossingRoomID, Column: 15,
		},
		{
			DoorID: "room-provider-door-crossing-boss", FromRoomID: crossingRoomID, ToRoomID: archetypeBoss, Column: 26,
		},
	}, plan.Connectors)
	require.Equal(t, FloorPlanCell{Column: 0, Row: 4}, plan.Entrance)

	doorIDs := make(map[string]bool, len(plan.Edges))
	for _, edge := range plan.Edges {
		doorIDs[edge.DoorID] = edge.Kind == FloorPlanEdgeKindDoor
	}
	require.True(t, doorIDs["room-provider-door-entrance-gallery"], "generated connector edge remains projected")
	require.True(t, doorIDs["room-provider-authored-door-0-0-0--1--1-0"], "authored edge remains projected")
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

// TestCanvasMonsterPlacementTargeting covers rpg-toolkit#895's targeting
// field on canvas-mode monster placements: rejected on a props ref (mirrors
// TestCanvasMonsterPlacementRejectsPropOnlyFlagsAtFieldPath's own props/
// monsters-only pattern, inverted), rejected when unparseable, and — when
// valid — threaded all the way into the compiled SpawnInstruction.
func TestCanvasMonsterPlacementTargeting(t *testing.T) {
	t.Run("rejected on a props ref", func(t *testing.T) {
		yaml := strings.Replace(canvasFixture,
			"  - { ref: \"dnd5e:props:altar\", at: [1, 0], facing: W }",
			"  - { ref: \"dnd5e:props:altar\", at: [1, 0], targeting: closest }", 1)
		_, err := Load([]byte(yaml))
		require.ErrorContains(t, err, "place[0].targeting")
		require.ErrorContains(t, err, "only valid on monsters")
	})

	t.Run("unparseable value rejected", func(t *testing.T) {
		yaml := strings.Replace(canvasFixture,
			"  - { ref: \"dnd5e:props:altar\", at: [1, 0], facing: W }",
			"  - { ref: \"dnd5e:monsters:skeleton\", at: [1, 0], targeting: nearest }", 1)
		_, err := Load([]byte(yaml))
		require.ErrorContains(t, err, "place[0].targeting")
		require.ErrorContains(t, err, `invalid targeting strategy "nearest"`)
	})

	t.Run("valid value threads into the compiled SpawnInstruction", func(t *testing.T) {
		yaml := strings.Replace(canvasFixture,
			"  - { ref: \"dnd5e:props:altar\", at: [1, 0], facing: W }",
			"  - { ref: \"dnd5e:monsters:skeleton\", at: [1, 0], targeting: lowest-ac }", 1)
		compiled, err := Load([]byte(yaml))
		require.NoError(t, err)
		require.Len(t, compiled.Spawns, 1)
		require.Equal(t, monster.TargetLowestAC, compiled.Spawns[0].Targeting)
	})
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

func TestWallLockSourceRoundTripRetainsOptionsWithoutRuntimeBinding(t *testing.T) {
	raw := []byte(strings.Replace(
		canvasFixture,
		"kind: door",
		"kind: door, lock: { options: [ { ability: dex, dc: 15 }, { ability: str, dc: 10 } ] }",
		1,
	))
	decoded, err := Decode(raw)
	require.NoError(t, err)
	require.NotNil(t, decoded.Walls[0].Lock)
	require.Equal(t, []LockOptionSpec{{Ability: "dex", DC: 15}, {Ability: "str", DC: 10}}, decoded.Walls[0].Lock.Options)

	marshaled, err := yaml.Marshal(decoded)
	require.NoError(t, err)
	roundTripped, err := Decode(marshaled)
	require.NoError(t, err)
	require.NotNil(t, roundTripped.Walls[0].Lock)
	require.Equal(t, decoded.Walls[0].Lock.Options, roundTripped.Walls[0].Lock.Options)

	compiled, err := Load(raw)
	require.NoError(t, err)
	require.Len(t, compiled.Params.AuthoredEdges, 1)
	// AuthoredEdge intentionally has no lock metadata: authored doors start unlocked.
}
