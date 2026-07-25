package encounter_test

// connector_column_walls_test.go is the TDD gate for rpg-toolkit#848:
// InitDungeon no longer emits a degenerate (Start == End) WallSegmentData
// entry per connector boundary column's flanking (non-door) cell — doing
// so made each flanking cell an independently-classified wall hex
// client-side (rpg-dnd5e-web's buildDungeonWallSegments/collectWallHexes),
// and a hex column's flanking cells expose 4 of their 6 sides to the two
// adjacent regions (not 2, the way a square-grid column would — a hex has
// 6 neighbors, and a north/south column only ever shares 2 of them with
// its own same-column neighbors), rendering as an isolated "chunky rubble
// box" per cell that visually drowned the door immediately beside it
// (rpg-dnd5e-web#562). Every flanking cell is now the End of one or more
// boundary-edge segments (Start != End, exactly one hex step apart) whose
// Start is a real region floor hex — the same technique
// perimeter_edge_walls_test.go already proves for rpg-toolkit#834's
// outer-perimeter case, extended to these interior boundary columns.
//
// The door cell itself never carried a wall entry before this fix and
// still doesn't (TestPerimeterEdgeWalls_NeverAtConnectorDoorCells,
// unchanged) — but this file adds an explicit walkability check anyway
// (TestConnectorColumnWalls_DoorCellStaysWalkable) since this fix also
// touches the room-rebuild blocking path (rebuildRoomFromData) the door
// cell's own passability depends on, not just wall-segment emission.

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/KirkDiggler/rpg-toolkit/encounter"
	"github.com/KirkDiggler/rpg-toolkit/encounter/core"
	"github.com/KirkDiggler/rpg-toolkit/tools/spatial"
)

// TestConnectorColumnWalls_NoLongerDegenerate proves every connector's
// flanking cell has been converted away from the old degenerate
// (Start == End) representation, across the same seed sweep
// perimeter_edge_walls_test.go uses.
func TestConnectorColumnWalls_NoLongerDegenerate(t *testing.T) {
	for _, seed := range perimeterEdgeSeeds {
		enc := newTestEncounter(t)
		require.NoError(t, enc.InitDungeon(threeRegionDungeonParams(seed)), "seed %d", seed)
		data := enc.ToData()
		flanking := connectorFlankingCubes(data, []core.EntityID{dungeonDoor0ID, dungeonDoor1ID})
		require.NotEmpty(t, flanking, "seed %d: fixture must actually have connector flanking cells", seed)

		degenerate := blockedCubesFromDegenerateWalls(data.Space.Walls)
		for cube := range flanking {
			require.False(t, degenerate[cube],
				"seed %d: flanking cell %v must no longer be a degenerate wall entry", seed, cube)
		}
	}
}

// assertConnectorBoundaryCompleteness re-derives, from first principles
// against public data only (RegionData.Hexes + Data.Doors, never
// dungeon.go's own unexported connectorBoundaryEdgeWalls), every
// {Start, End} boundary-edge pair a connector's flanking cells imply — a
// real region floor hex is any cube inside some region's Hexes that isn't
// itself blocked by a degenerate (interior obstacle) wall entry — then
// proves data.Space.Walls' connector-facing (flanking End) entries match
// that set exactly: no extras, no missing edges, no duplicates. Mirrors
// perimeter_edge_walls_test.go's assertPerimeterCompletenessAndNoDuplicates,
// scoped to the complementary (interior, not outer) boundary-edge category.
func assertConnectorBoundaryCompleteness(t *testing.T, label string, data *encounter.Data, doorIDs []core.EntityID) {
	t.Helper()
	flanking := connectorFlankingCubes(data, doorIDs)
	degenerate := blockedCubesFromDegenerateWalls(data.Space.Walls)

	inAnyRegion := make(map[spatial.CubeCoordinate]bool)
	for _, r := range data.Space.Regions {
		for _, h := range r.Hexes.Slice() {
			inAnyRegion[h.ToCube()] = true
		}
	}

	gotEdges := make(map[[2]spatial.CubeCoordinate]int)
	for _, w := range data.Space.Walls {
		if w.Start == w.End {
			continue
		}
		if !flanking[w.End] {
			continue // an outer-perimeter edge (#834) -- not this test's concern
		}
		gotEdges[[2]spatial.CubeCoordinate{w.Start, w.End}]++
	}
	for pair, count := range gotEdges {
		require.Equal(t, 1, count, "%s: duplicate connector boundary edge %v", label, pair)
	}

	expected := make(map[[2]spatial.CubeCoordinate]bool)
	for cube := range inAnyRegion {
		if degenerate[cube] {
			continue // an interior obstacle, not real floor
		}
		for _, n := range cube.GetNeighbors() {
			if !flanking[n] {
				continue
			}
			expected[[2]spatial.CubeCoordinate{cube, n}] = true
		}
	}

	require.Len(t, gotEdges, len(expected), "%s: connector boundary edge count mismatch", label)
	for pair := range expected {
		require.Contains(t, gotEdges, pair, "%s: missing connector boundary edge %v", label, pair)
	}
}

// TestConnectorColumnWalls_BoundaryEdgeCompleteness sweeps the same seeds
// perimeter_edge_walls_test.go uses to prove the connector-facing boundary
// edges are complete and duplicate-free across varying interior-wall
// layouts, not just one lucky seed.
func TestConnectorColumnWalls_BoundaryEdgeCompleteness(t *testing.T) {
	for _, seed := range perimeterEdgeSeeds {
		enc := newTestEncounter(t)
		require.NoError(t, enc.InitDungeon(threeRegionDungeonParams(seed)), "seed %d", seed)
		data := enc.ToData()
		assertConnectorBoundaryCompleteness(t, fmt.Sprintf("seed %d", seed), data,
			[]core.EntityID{dungeonDoor0ID, dungeonDoor1ID})
	}
}

// TestConnectorColumnWalls_FlankingCellsRemainBlocked proves rpg-toolkit#848
// didn't quietly turn off movement/LOS blocking for a connector's flanking
// cells now that they're no longer their own degenerate wall entry —
// rebuildRoomFromData's companion change must still place a blocker at
// each one (see space.go's Start != End / in-grid-End branch).
func TestConnectorColumnWalls_FlankingCellsRemainBlocked(t *testing.T) {
	enc := newTestEncounter(t)
	require.NoError(t, enc.InitDungeon(threeRegionDungeonParams(dungeonSeed)))
	data := enc.ToData()
	flanking := connectorFlankingCubes(data, []core.EntityID{dungeonDoor0ID, dungeonDoor1ID})
	require.NotEmpty(t, flanking, "fixture must actually have connector flanking cells")

	room := enc.Room()
	for cube := range flanking {
		pos := cube.ToOffsetCoordinateWithOrientation(spatial.HexOrientationPointyTop)
		require.False(t, room.CanPlaceEntity(probeEntity{}, pos),
			"flanking cell %v must still block movement/LOS", cube)
	}
}

// TestConnectorColumnWalls_DoorCellStaysWalkable is the explicit
// door-walkability coverage rpg-toolkit#848 calls for: with a connector's
// door OPEN, its own cell must remain enterable. connectorBoundaryEdgeWalls
// never emits an edge whose End is a door cube (flanking excludes doors by
// construction), so rebuildRoomFromData's new Start != End / in-grid-End
// blocking branch never reaches a door cell — this proves that holds
// end-to-end through the actual room rebuild, not just as a claim about
// wall-segment shape.
func TestConnectorColumnWalls_DoorCellStaysWalkable(t *testing.T) {
	enc := newTestEncounter(t)
	require.NoError(t, enc.InitDungeon(threeRegionDungeonParams(dungeonSeed)))
	data := enc.ToData()

	require.NoError(t, enc.AddPlayer(encounter.PlayerInput{
		PlayerID: alicePlayerID, EntityID: aliceEntityID,
		Position: data.Space.Entrance, SightRange: 30,
	}))
	require.NoError(t, enc.OpenDoor(alicePlayerID, dungeonDoor0ID))
	require.NoError(t, enc.OpenDoor(alicePlayerID, dungeonDoor1ID))

	room := enc.Room()
	for _, doorID := range []core.EntityID{dungeonDoor0ID, dungeonDoor1ID} {
		door := enc.ToData().Doors[doorID]
		pos := door.Position.ToPosition()
		require.True(t, room.CanPlaceEntity(probeEntity{}, pos),
			"open door %q's own cell must remain walkable", doorID)
	}
}
