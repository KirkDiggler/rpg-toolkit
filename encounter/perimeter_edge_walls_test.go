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
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/KirkDiggler/rpg-toolkit/encounter"
	"github.com/KirkDiggler/rpg-toolkit/encounter/core"
	"github.com/KirkDiggler/rpg-toolkit/tools/environments"
	"github.com/KirkDiggler/rpg-toolkit/tools/spatial"
)

// perimeterEdgeSeeds sweeps a handful of seeds across the shared
// three-region dungeon fixture (entrance/boss PatternRandom, corridor
// PatternEmpty) so perimeter-edge behavior is proven across varying
// interior-wall layouts, not just one lucky seed.
//
// rpg-toolkit#849 gate review finding 2: the original 8-seed list here
// (dungeonSeed, 1, 2, 3, 4, 5, 100, 909091) actually produced interior
// wall cells on only 2 of those 8 seeds (4 and 100, 3 cells each) — the
// other 6 rolled entirely empty entrance/boss interiors, so the sweep
// exercised essentially one non-trivial layout, not "varying interior-wall
// layouts" as the doc above claims. Measured directly (interior wall cell
// counts per seed against this exact fixture): 4→3, 6→7, 17→9, 19→8,
// 100→3, 101→10; dungeonSeed and 909091 stay at 0, kept deliberately for
// the all-empty case (a PatternEmpty-heavy dungeon is itself a real,
// legitimate shape worth covering, not just an accident of a bad seed
// pick).
var perimeterEdgeSeeds = []int64{dungeonSeed, 4, 6, 17, 19, 100, 101, 909091}

// blockedCubesFromDegenerateWalls returns every cube coordinate marked
// blocked by a degenerate (Start == End) wall entry — interior pattern
// walls only, as of rpg-toolkit#848 (connector boundary columns used to
// be degenerate entries too; see connectorFlankingCubes below for their
// replacement). Boundary-edge entries (Start != End) never mark a cube
// blocked via THIS function.
func blockedCubesFromDegenerateWalls(walls []environments.WallSegmentData) map[spatial.CubeCoordinate]bool {
	out := make(map[spatial.CubeCoordinate]bool, len(walls))
	for _, w := range walls {
		if w.Start == w.End {
			out[w.Start] = true
		}
	}
	return out
}

// connectorFlankingCubes reconstructs every connector's boundary-column
// flanking (non-door) cube for a built dungeon, independently of
// dungeon.go's own starts/width arithmetic (rpg-toolkit#848): a
// RegionData.Hexes already covers a region's FULL local rectangle —
// interior pattern walls/obstacles included (rpg-toolkit#814) — so any
// cube within the combined space's [0,Width)x[0,Height) bounds that
// belongs to NO region is exactly a connector's boundary column; the
// door cells themselves (data.Doors, keyed by doorIDs) are excluded,
// leaving only the flanking cells that must remain blocked without any
// longer being their own degenerate wire entry.
func connectorFlankingCubes(data *encounter.Data, doorIDs []core.EntityID) map[spatial.CubeCoordinate]bool {
	doorCubes := make(map[spatial.CubeCoordinate]bool, len(doorIDs))
	for _, id := range doorIDs {
		doorCubes[data.Doors[id].Position.ToCube()] = true
	}
	flanking := make(map[spatial.CubeCoordinate]bool)
	for x := 0; x < data.Space.Width; x++ {
		for y := 0; y < data.Space.Height; y++ {
			cube := spatial.OffsetCoordinateToCubeWithOrientation(
				spatial.Position{X: float64(x), Y: float64(y)}, spatial.HexOrientationPointyTop)
			if doorCubes[cube] {
				continue
			}
			hex := core.HexFromCube(cube)
			inRegion := false
			for _, r := range data.Space.Regions {
				if r.Hexes.Has(hex) {
					inRegion = true
					break
				}
			}
			if !inRegion {
				flanking[cube] = true
			}
		}
	}
	return flanking
}

// assertPerimeterCompletenessAndNoDuplicates re-derives, from first
// principles against public spatial primitives only (never dungeon.go's
// own unexported perimeterEdgeWalls/wallCubeSet), every {Start, End}
// OUTER-PERIMETER boundary-edge pair a SpaceData's floor/wall layout
// implies, then proves data.Space.Walls' non-degenerate, out-of-grid-End
// entries match that set exactly: no extras, no missing edges, no
// duplicates. Shared by every test in this file that builds a dungeon
// (varying seed or shape) and checks the same invariant, rather than
// duplicating the derivation per call site.
//
// doorIDs feeds connectorFlankingCubes (rpg-toolkit#848): a connector's
// flanking cells must count as "not real floor" here too, exactly like an
// interior obstacle wall, even though they're no longer degenerate wire
// entries themselves — otherwise a flanking cell sitting on the space's
// own true y=0/y=height-1 edge row (which always happens: a connector
// column always spans every row, only its doorRow cell is ever excluded)
// would be misidentified as real floor and expected to grow a bogus
// outward-facing perimeter edge of its own. Connector-to-region boundary
// edges are a separate concern with their own completeness test — see
// connector_column_walls_test.go — deliberately excluded from `expected`
// and filtered out of `gotEdges` here by their in-grid End.
func assertPerimeterCompletenessAndNoDuplicates(
	t *testing.T, label string, data *encounter.Data, grid spatial.Grid, doorIDs []core.EntityID,
) {
	t.Helper()
	blocked := blockedCubesFromDegenerateWalls(data.Space.Walls)
	flanking := connectorFlankingCubes(data, doorIDs)
	gotEdges := make(map[[2]spatial.CubeCoordinate]int)
	for _, w := range data.Space.Walls {
		if w.Start == w.End {
			continue
		}
		endPos := w.End.ToOffsetCoordinateWithOrientation(spatial.HexOrientationPointyTop)
		if grid.IsValidPosition(endPos) {
			continue // a connector boundary edge (#848) -- not this test's concern
		}
		gotEdges[[2]spatial.CubeCoordinate{w.Start, w.End}]++
	}
	for pair, count := range gotEdges {
		require.Equal(t, 1, count, "%s: duplicate perimeter edge segment %v", label, pair)
	}

	expected := make(map[[2]spatial.CubeCoordinate]bool)
	for x := 0; x < data.Space.Width; x++ {
		for y := 0; y < data.Space.Height; y++ {
			pos := spatial.Position{X: float64(x), Y: float64(y)}
			cube := spatial.OffsetCoordinateToCubeWithOrientation(pos, spatial.HexOrientationPointyTop)
			if blocked[cube] || flanking[cube] {
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

	require.Len(t, gotEdges, len(expected), "%s: perimeter edge segment count mismatch", label)
	for pair := range expected {
		require.Contains(t, gotEdges, pair, "%s: missing perimeter edge %v", label, pair)
	}
}

// assertNoDoorCellIsPerimeterStart proves a connector door's cell is never
// the Start of a boundary-edge segment, for the given door entity IDs.
// Shared by every test in this file that checks the door-passage-
// exclusion invariant across a seed or shape sweep.
func assertNoDoorCellIsPerimeterStart(t *testing.T, label string, data *encounter.Data, doorIDs []core.EntityID) {
	t.Helper()
	doorCubes := make(map[spatial.CubeCoordinate]bool, len(doorIDs))
	for _, id := range doorIDs {
		doorCubes[data.Doors[id].Position.ToCube()] = true
	}
	for _, w := range data.Space.Walls {
		if w.Start == w.End {
			continue
		}
		require.False(t, doorCubes[w.Start],
			"%s: a connector door cell must never be the Start of a boundary-edge perimeter segment", label)
	}
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
		assertPerimeterCompletenessAndNoDuplicates(t, fmt.Sprintf("seed %d", seed), data, enc.Room().GetGrid(),
			[]core.EntityID{dungeonDoor0ID, dungeonDoor1ID})
	}
}

// TestPerimeterEdgeWalls_StartInsideEndOutsideDistanceOne pins the shape
// contract every boundary-edge segment must satisfy: Start and End are
// exactly one hex step apart, Start is always a real position inside the
// live room's grid, and BlocksMovement/BlocksLoS match existing
// (degenerate) walls, per the issue's ask. End is outside the room for
// the outer-perimeter case (#834); rpg-toolkit#848 adds a second, EQUALLY
// valid shape — End is itself a valid in-grid position that is exactly
// one of this dungeon's connector flanking cells (never anything else,
// e.g. never an interior obstacle cell or a door cell) — see
// connector_column_walls_test.go for that shape's own dedicated coverage.
func TestPerimeterEdgeWalls_StartInsideEndOutsideDistanceOne(t *testing.T) {
	enc := newTestEncounter(t)
	require.NoError(t, enc.InitDungeon(threeRegionDungeonParams(dungeonSeed)))
	data := enc.ToData()
	grid := enc.Room().GetGrid()
	flanking := connectorFlankingCubes(data, []core.EntityID{dungeonDoor0ID, dungeonDoor1ID})

	foundOuter, foundConnector := false, false
	for _, w := range data.Space.Walls {
		if w.Start == w.End {
			continue
		}
		require.Equal(t, 1, w.Start.Distance(w.End),
			"boundary-edge segment %+v must be exactly one hex step apart", w)

		startPos := w.Start.ToOffsetCoordinateWithOrientation(spatial.HexOrientationPointyTop)
		endPos := w.End.ToOffsetCoordinateWithOrientation(spatial.HexOrientationPointyTop)
		require.True(t, grid.IsValidPosition(startPos),
			"boundary-edge segment %+v: Start must be inside the room", w)

		if grid.IsValidPosition(endPos) {
			foundConnector = true
			require.True(t, flanking[w.End],
				"boundary-edge segment %+v: an in-grid End must be a connector flanking cell", w)
		} else {
			foundOuter = true
		}

		require.True(t, w.BlocksMovement, "boundary-edge segment %+v must carry BlocksMovement like existing walls", w)
		require.True(t, w.BlocksLoS, "boundary-edge segment %+v must carry BlocksLoS like existing walls", w)
	}
	require.True(t, foundOuter, "fixture must actually produce at least one outer-perimeter boundary-edge segment")
	require.True(t, foundConnector, "fixture must actually produce at least one connector boundary-edge segment")
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
		assertNoDoorCellIsPerimeterStart(t, fmt.Sprintf("seed %d", seed), data,
			[]core.EntityID{dungeonDoor0ID, dungeonDoor1ID})
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

// perimeterEdgeShape is one SHAPE (region count, height, widths, pattern
// mix) to sweep TestPerimeterEdgeWalls_VariedShapes over.
type perimeterEdgeShape struct {
	name    string
	params  encounter.DungeonParams
	doorIDs []core.EntityID
}

// perimeterEdgeShapeCases sweeps DungeonParams SHAPE (region count, the
// validateDungeonParams height floor, and varied per-region widths) rather
// than just seed. Every other test in this file pins the perimeter-edge
// geometry (completeness, no duplicates, door-passage exclusion) across
// randomness at ONE fixed shape (the 3-region entrance/corridor/boss
// fixture, Height=8, widths 10/5/10) — this proves the same invariants
// hold at shapes that fixture never exercises: a 2-region dungeon at the
// minimum allowed height (4), and a 5-region dungeon with unequal widths
// ending in a boss archetype (exercising the boss room's >6-hex-step
// scale invariant at a non-default height too).
func perimeterEdgeShapeCases() []perimeterEdgeShape {
	twoRegionDoor := core.EntityID("shape-2region-door")
	fiveRegionDoors := []core.EntityID{
		"shape-5region-door-0", "shape-5region-door-1", "shape-5region-door-2", "shape-5region-door-3",
	}
	return []perimeterEdgeShape{
		{
			name: "2-region at the minimum allowed height",
			params: encounter.DungeonParams{
				Height: 4, RandomSeed: 7,
				Regions: []encounter.DungeonRegionParams{
					{ID: "a", Archetype: encounter.ArchetypeEntrance, Width: 5, Pattern: environments.PatternRandom},
					{ID: "b", Archetype: encounter.ArchetypeChamber, Width: 6, Pattern: environments.PatternEmpty},
				},
				Connectors: []encounter.DungeonConnectorParams{{DoorID: twoRegionDoor}},
			},
			doorIDs: []core.EntityID{twoRegionDoor},
		},
		{
			name: "5-region with varied widths ending in boss",
			params: encounter.DungeonParams{
				Height: 8, RandomSeed: 13,
				Regions: []encounter.DungeonRegionParams{
					{ID: "r0", Archetype: encounter.ArchetypeEntrance, Width: 4, Pattern: environments.PatternRandom},
					{ID: "r1", Archetype: encounter.ArchetypeCorridor, Width: 9, Pattern: environments.PatternEmpty},
					{ID: "r2", Archetype: encounter.ArchetypeChamber, Width: 5, Pattern: environments.PatternRandom},
					{ID: "r3", Archetype: encounter.ArchetypeCorridor, Width: 4, Pattern: environments.PatternEmpty},
					{ID: "r4", Archetype: encounter.ArchetypeBoss, Width: 11, Pattern: environments.PatternRandom},
				},
				Connectors: []encounter.DungeonConnectorParams{
					{DoorID: fiveRegionDoors[0]}, {DoorID: fiveRegionDoors[1]},
					{DoorID: fiveRegionDoors[2]}, {DoorID: fiveRegionDoors[3]},
				},
			},
			doorIDs: fiveRegionDoors,
		},
	}
}

// TestPerimeterEdgeWalls_VariedShapes is the gate review's minor finding
// (PR #838): every other test in this file sweeps SEED at one fixed
// 3-region shape, which pins the geometry across randomness but never
// across the shapes the reasoning in dungeon.go's perimeterEdgeWalls doc
// actually depends on (an interior boundary column is never at the
// space's x=0/x=width-1 edge; doorRow is never y=0/y=height-1). This pins
// completeness, no-duplicates, and door-passage exclusion as DATA at a
// 2-region minimum-height shape and a 5-region varied-width shape, not
// just as argument.
func TestPerimeterEdgeWalls_VariedShapes(t *testing.T) {
	for _, tc := range perimeterEdgeShapeCases() {
		t.Run(tc.name, func(t *testing.T) {
			enc := newTestEncounter(t)
			require.NoError(t, enc.InitDungeon(tc.params))
			data := enc.ToData()
			assertPerimeterCompletenessAndNoDuplicates(t, tc.name, data, enc.Room().GetGrid(), tc.doorIDs)
			assertNoDoorCellIsPerimeterStart(t, tc.name, data, tc.doorIDs)
		})
	}
}
