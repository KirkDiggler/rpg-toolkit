package encounter_test

// perimeter_edge_walls_test.go is the TDD gate for rpg-toolkit#834:
// InitDungeon now emits one boundary-edge WallSegmentData (Start != End,
// exactly one hex step apart) per room-facing edge of the combined
// dungeon space's outer perimeter, in addition to the existing degenerate
// (Start == End) interior/connector wall entries, which are left
// untouched. Reuses dungeon_test.go's shared threeRegionDungeonParams /
// dungeonHeight / dungeonRegionIDs fixture and dungeon_completion_test.go's
// dcDungeonParams (all-PatternEmpty) fixture — both already establish this
// package's shared conventions (newTestEncounter, dungeonDoor0ID/1ID) —
// rather than inventing a third one.
//
// Every check here is derived independently of dungeon.go's own unexported
// perimeterEdgeWalls/wallCubeSet: "floor" is reconstructed from the public
// Data.Space.Walls shape (a degenerate entry marks a blocked cell), and
// "outside the room" is queried against the LIVE room's own
// GetGrid().IsValidPosition — the exact same grid movement/LoS already use
// — rather than a hand-rolled bounds recheck that could share a bug with
// the generator.

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/KirkDiggler/rpg-toolkit/tools/environments"
	"github.com/KirkDiggler/rpg-toolkit/tools/spatial"
)

// perimeterEdgeSeeds sweeps a handful of seeds across the shared
// three-region dungeon fixture (entrance/boss PatternRandom, corridor
// PatternEmpty) so perimeter-edge behavior is proven across varying
// interior-wall layouts, not just one lucky seed.
var perimeterEdgeSeeds = []int64{dungeonSeed, 1, 2, 3, 4, 5, 100, 909091}

// blockedCubesFromDegenerateWalls returns every cube coordinate marked
// blocked by a degenerate (Start == End) wall entry — interior pattern
// walls and connector boundary columns alike. Boundary-edge entries
// (Start != End) never mark a cube blocked; a real floor hex is exactly
// the complement of this set within the space's bounds.
func blockedCubesFromDegenerateWalls(walls []environments.WallSegmentData) map[spatial.CubeCoordinate]bool {
	out := make(map[spatial.CubeCoordinate]bool, len(walls))
	for _, w := range walls {
		if w.Start == w.End {
			out[w.Start] = true
		}
	}
	return out
}

// TestPerimeterEdgeWalls_Completeness proves, independently of dungeon.go's
// own perimeterEdgeWalls, that every non-wall (floor) hex in the combined
// space whose neighbor lies outside the room's ACTUAL grid bounds (queried
// via the live room's own GetGrid().IsValidPosition, not a hand-rolled
// recheck) has exactly one matching Start != End boundary-edge segment in
// Data.Space.Walls — no extras, no duplicates, across a spread of seeds.
func TestPerimeterEdgeWalls_Completeness(t *testing.T) {
	for _, seed := range perimeterEdgeSeeds {
		enc := newTestEncounter(t)
		require.NoError(t, enc.InitDungeon(threeRegionDungeonParams(seed)), "seed %d", seed)
		data := enc.ToData()
		grid := enc.Room().GetGrid()

		blocked := blockedCubesFromDegenerateWalls(data.Space.Walls)
		gotEdges := make(map[[2]spatial.CubeCoordinate]int)
		for _, w := range data.Space.Walls {
			if w.Start == w.End {
				continue
			}
			gotEdges[[2]spatial.CubeCoordinate{w.Start, w.End}]++
		}
		for pair, count := range gotEdges {
			require.Equal(t, 1, count, "seed %d: duplicate perimeter edge segment %v", seed, pair)
		}

		expected := make(map[[2]spatial.CubeCoordinate]bool)
		for x := 0; x < data.Space.Width; x++ {
			for y := 0; y < data.Space.Height; y++ {
				pos := spatial.Position{X: float64(x), Y: float64(y)}
				cube := spatial.OffsetCoordinateToCubeWithOrientation(pos, spatial.HexOrientationPointyTop)
				if blocked[cube] {
					continue
				}
				for _, n := range cube.GetNeighbors() {
					npos := n.ToOffsetCoordinateWithOrientation(spatial.HexOrientationPointyTop)
					if grid.IsValidPosition(npos) {
						continue // neighbor is inside the room -- not a perimeter edge
					}
					expected[[2]spatial.CubeCoordinate{cube, n}] = true
				}
			}
		}

		require.Len(t, gotEdges, len(expected), "seed %d: perimeter edge segment count mismatch", seed)
		for pair := range expected {
			require.Contains(t, gotEdges, pair, "seed %d: missing perimeter edge %v", seed, pair)
		}
	}
}

// TestPerimeterEdgeWalls_StartInsideEndOutsideDistanceOne pins the shape
// contract every boundary-edge segment must satisfy: Start and End are
// exactly one hex step apart, Start is a real position inside the live
// room's grid, and End is outside it — plus the BlocksMovement/BlocksLoS
// semantics match existing (degenerate) walls, per the issue's ask.
func TestPerimeterEdgeWalls_StartInsideEndOutsideDistanceOne(t *testing.T) {
	enc := newTestEncounter(t)
	require.NoError(t, enc.InitDungeon(threeRegionDungeonParams(dungeonSeed)))
	data := enc.ToData()
	grid := enc.Room().GetGrid()

	found := false
	for _, w := range data.Space.Walls {
		if w.Start == w.End {
			continue
		}
		found = true
		require.Equal(t, 1, w.Start.Distance(w.End),
			"boundary-edge segment %+v must be exactly one hex step apart", w)

		startPos := w.Start.ToOffsetCoordinateWithOrientation(spatial.HexOrientationPointyTop)
		endPos := w.End.ToOffsetCoordinateWithOrientation(spatial.HexOrientationPointyTop)
		require.True(t, grid.IsValidPosition(startPos),
			"boundary-edge segment %+v: Start must be inside the room", w)
		require.False(t, grid.IsValidPosition(endPos),
			"boundary-edge segment %+v: End must be outside the room", w)

		require.True(t, w.BlocksMovement, "boundary-edge segment %+v must carry BlocksMovement like existing walls", w)
		require.True(t, w.BlocksLoS, "boundary-edge segment %+v must carry BlocksLoS like existing walls", w)
	}
	require.True(t, found, "fixture must actually produce at least one boundary-edge segment")
}

// TestPerimeterEdgeWalls_EntranceHexHasWestFacingEdge is a concrete,
// human-checkable pin alongside the from-first-principles completeness
// test above: the entrance hex sits on the required entrance-to-door
// path, so it is guaranteed floor regardless of seed, and it sits at the
// space's own column 0 -- its westward neighbor is always outside the
// room. It must therefore always carry a west-facing boundary-edge
// segment, for every seed.
func TestPerimeterEdgeWalls_EntranceHexHasWestFacingEdge(t *testing.T) {
	for _, seed := range perimeterEdgeSeeds {
		enc := newTestEncounter(t)
		require.NoError(t, enc.InitDungeon(threeRegionDungeonParams(seed)), "seed %d", seed)
		data := enc.ToData()
		entranceCube := data.Space.Entrance.ToCube()

		found := false
		for _, w := range data.Space.Walls {
			if w.Start != entranceCube || w.Start == w.End {
				continue
			}
			endPos := w.End.ToOffsetCoordinateWithOrientation(spatial.HexOrientationPointyTop)
			if endPos.X < 0 {
				found = true
				break
			}
		}
		require.True(t, found, "seed %d: the entrance hex must carry a west-facing boundary-edge segment", seed)
	}
}

// TestPerimeterEdgeWalls_NeverAtConnectorDoorCells pins a geometric fact
// dungeon.go's perimeterEdgeWalls doc explains rather than works around: a
// connector's door always sits at doorRow within an INTERIOR boundary
// column (never the space's own x=0/x=width-1 edge), and doorRow=height/2
// is itself never y=0/y=height-1 given validateDungeonParams' >=4 height
// floor -- so a door cell can never be the Start of a boundary-edge
// segment. The door-passage exclusion the issue anticipated turns out to
// be geometrically moot; this test proves that fact directly rather than
// just trusting the reasoning.
func TestPerimeterEdgeWalls_NeverAtConnectorDoorCells(t *testing.T) {
	for _, seed := range perimeterEdgeSeeds {
		enc := newTestEncounter(t)
		require.NoError(t, enc.InitDungeon(threeRegionDungeonParams(seed)), "seed %d", seed)
		data := enc.ToData()

		doorCubes := map[spatial.CubeCoordinate]bool{
			data.Doors[dungeonDoor0ID].Position.ToCube(): true,
			data.Doors[dungeonDoor1ID].Position.ToCube(): true,
		}
		for _, w := range data.Space.Walls {
			if w.Start == w.End {
				continue
			}
			require.False(t, doorCubes[w.Start],
				"seed %d: a connector door cell must never be the Start of a boundary-edge perimeter segment", seed)
		}
	}
}

// TestPerimeterEdgeWalls_DeterministicAcrossSeeds proves perimeter-edge
// emission is a pure function of the floor set, not of the seed itself.
// dungeon_completion_test.go's dcDungeonParams fixture uses PatternEmpty
// for every region, so its floor set is entirely seed-independent — under
// it, Data.Space.Walls (connector boundary columns AND the new perimeter
// edges alike) must be byte-for-byte identical across every seed tried,
// not merely "the same shape."
func TestPerimeterEdgeWalls_DeterministicAcrossSeeds(t *testing.T) {
	var first []environments.WallSegmentData
	for i, seed := range []int64{1, 2, 3, 42, 909090, 123456789} {
		enc := newTestEncounter(t)
		require.NoError(t, enc.InitDungeon(dcDungeonParams(seed)), "seed %d", seed)
		walls := enc.ToData().Space.Walls
		if i == 0 {
			first = walls
			continue
		}
		require.Equal(t, first, walls,
			"seed %d: an all-PatternEmpty fixture's wall layout (including perimeter edges) must be "+
				"identical across seeds", seed)
	}
}
