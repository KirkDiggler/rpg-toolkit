package encounter_test

// boss_primary_axis_test.go is the regression gate for a hard invariant
// #819's done bar requires but #814's original generator did not fully
// guarantee: a boss-archetype region's PRIMARY PLAYABLE AXIS (the full
// doorRow row spanning the region's entire width, its >6-hex-step size
// floor already enforced by validateDungeonParams) must stay clear of
// GENERATED WALLS, not just the pre-#819 required path (which only ran
// from the incoming door to the region's local CENTER, not its far edge).
//
// Found by independent seed sampling (335+/3000 boss regions at this
// exact 10x8 fixture shape land at least one PatternRandom wall on the
// doorRow beyond local center): tools/environments' path-safety
// validation (validatePathSafety/pathExists, a SEPARATE module —
// verified read-only, not modified here) operates on the RAW, continuous-
// position wall segments BEFORE this package's own hex-cell rounding
// (regionWallSegments) — a wall segment slightly off the exact required-
// path line in continuous space can still round onto the doorRow hex
// cell after discretization, slipping through even where a required
// path nominally already covers that span. Reproduced below against a
// bounded table of concretely-failing seeds (not a live 3000-seed scan)
// so this test stays fast and deterministic.
//
// See dungeon.go's per-region requiredPaths switch (now extended to
// farEdge for any ArchetypeBoss region, matching entrance/interior
// regions) and stripReservedAxisWalls (the discrete, by-construction
// guarantee that closes the continuous/discrete rounding gap the
// required-path extension alone cannot).

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/KirkDiggler/rpg-toolkit/encounter"
	"github.com/KirkDiggler/rpg-toolkit/tools/environments"
	"github.com/KirkDiggler/rpg-toolkit/tools/spatial"
)

// bossAxisFailingSeeds are concrete seeds independently verified (via a
// throwaway 1..3000 scan against this exact fixture shape) to place a
// PatternRandom wall on the boss region's doorRow BEFORE this file's
// fix — several strictly beyond local center (width/2 == 5): seed 4
// alone lands walls at local columns 6 and 7. Kept as a literal table
// (not regenerated at test time) so the RED/GREEN evidence is pinned to
// specific, reproducible seeds rather than "whatever the scan finds
// today."
var bossAxisFailingSeeds = []int64{1, 4, 9, 10, 19, 30, 35, 63, 67, 68}

// bpaDungeonParams builds the same entrance/corridor/boss shape
// dungeon_test.go's threeRegionDungeonParams uses, but with PatternRandom
// on the boss region specifically (threeRegionDungeonParams already does
// this) — kept local so this file's seed table and assertions are
// self-contained and don't risk perturbing dungeon_test.go's own fixture.
func bpaDungeonParams(seed int64) encounter.DungeonParams {
	return encounter.DungeonParams{
		Height:     dungeonHeight,
		RandomSeed: seed,
		Regions: []encounter.DungeonRegionParams{
			{
				ID: dungeonRegionIDEntrance, Archetype: encounter.ArchetypeEntrance,
				Width: dungeonEntranceWidth, Pattern: environments.PatternEmpty,
			},
			{
				ID: dungeonRegionIDCorridor, Archetype: encounter.ArchetypeCorridor,
				Width: dungeonCorridorWidth, Pattern: environments.PatternEmpty,
			},
			{
				ID: dungeonRegionIDBoss, Archetype: encounter.ArchetypeBoss,
				Width: dungeonBossWidth, Pattern: environments.PatternRandom,
			},
		},
		Connectors: []encounter.DungeonConnectorParams{
			{DoorID: dungeonDoor0ID},
			{DoorID: dungeonDoor1ID},
		},
	}
}

// TestInitDungeon_BossPrimaryAxis_NeverBlockedByGeneratedWalls: for every
// seed in the failing table, InitDungeon must place NO wall on the boss
// region's doorRow anywhere across its FULL local width — not just up to
// center. This is the #819 hard invariant, checked directly against the
// generated Data.Space.Walls (ground truth), not the upstream required-
// path declaration (which, per this file's header, is necessary but not
// sufficient on its own).
func TestInitDungeon_BossPrimaryAxis_NeverBlockedByGeneratedWalls(t *testing.T) {
	starts := dungeonRegionStarts()
	bossStart := starts[2]

	for _, seed := range bossAxisFailingSeeds {
		enc := newTestEncounter(t)
		require.NoError(t, enc.InitDungeon(bpaDungeonParams(seed)))
		data := enc.ToData()

		for _, w := range data.Space.Walls {
			// rpg-toolkit#834: a boundary-edge segment (Start != End) is a
			// non-blocking render-contract entry for a REAL floor hex on
			// the space's outer perimeter, not a generated interior wall —
			// the boss region's far (rightmost) column sits on the whole
			// dungeon's own right edge, so it legitimately gets one of
			// these on its primary-axis cell too. This test's invariant is
			// about GENERATED (blocking) walls only; only degenerate
			// (Start == End) entries are ever candidates.
			if w.Start != w.End {
				continue
			}
			pos := w.Start.ToOffsetCoordinateWithOrientation(spatial.HexOrientationPointyTop)
			localX := int(pos.X) - bossStart
			if localX < 0 || localX >= dungeonBossWidth {
				continue // not this region
			}
			require.NotEqual(t, dungeonHeight/2, int(pos.Y),
				"seed %d: generated wall at boss local column %d sits on the primary-axis doorRow; must never happen",
				seed, localX)
		}
	}
}

// TestInitDungeon_BossPrimaryAxis_FullRowWalkable: beyond "no wall sits
// exactly on this hex" (the previous test), the entire doorRow row across
// the boss region's FULL local width must be an actually walkable line —
// opening the connector door and querying room.CanPlaceEntity end to end,
// the same check movement itself uses (mirrors reachableFrom/
// truncateAtWall's own CanPlaceEntity-based ground truth elsewhere in
// this package).
func TestInitDungeon_BossPrimaryAxis_FullRowWalkable(t *testing.T) {
	starts := dungeonRegionStarts()
	bossStart := starts[2]

	for _, seed := range bossAxisFailingSeeds {
		enc := newTestEncounter(t)
		require.NoError(t, enc.InitDungeon(bpaDungeonParams(seed)))

		entrance := enc.ToData().Space.Entrance
		require.NoError(t, enc.AddPlayer(encounter.PlayerInput{
			PlayerID: alicePlayerID, EntityID: aliceEntityID,
			Position: entrance, SightRange: 30,
		}))
		// rpg-toolkit#864: OpenDoor requires adjacency — walk alice up to
		// each door first (mirrors dungeon_test.go's identical fix).
		require.NoError(t, enc.Move(alicePlayerID, straightRowPath(entrance, dungeonRegionFarEdgeHex(0))))
		require.NoError(t, enc.OpenDoor(alicePlayerID, dungeonDoor0ID))
		require.NoError(t, enc.Move(alicePlayerID, straightRowPath(dungeonRegionFarEdgeHex(0), dungeonRegionFarEdgeHex(1))))
		require.NoError(t, enc.OpenDoor(alicePlayerID, dungeonDoor1ID))

		for localX := 0; localX < dungeonBossWidth; localX++ {
			pos := spatial.Position{X: float64(bossStart + localX), Y: float64(dungeonHeight / 2)}
			require.True(t, enc.Room().CanPlaceEntity(probeEntity{}, pos),
				"seed %d: boss primary-axis column %d must be walkable end to end", seed, localX)
		}
	}
}

// TestInitDungeon_BossPrimaryAxis_ClearWithObstaclesToo: the #819 done
// bar is that the boss primary axis stays clear of BOTH generated walls
// AND placed obstacles. Attaches a heavy ObstacleSpec load (Count far
// exceeding the boss region's non-axis floor) to every failing seed and
// re-proves the row is walkable — obstacle placement's own doorRow
// exclusion (placeRegionObstacles) and this file's wall-level fix must
// hold simultaneously, not just in isolation.
func TestInitDungeon_BossPrimaryAxis_ClearWithObstaclesToo(t *testing.T) {
	starts := dungeonRegionStarts()
	bossStart := starts[2]

	for _, seed := range bossAxisFailingSeeds {
		params := bpaDungeonParams(seed)
		params.Regions[2].Obstacles = []encounter.ObstacleSpec{
			{Ref: "test:obstacles:heavy-load", Count: 60, BlocksMovement: true, BlocksLoS: true},
		}
		enc := newTestEncounter(t)
		require.NoError(t, enc.InitDungeon(params))

		entrance := enc.ToData().Space.Entrance
		require.NoError(t, enc.AddPlayer(encounter.PlayerInput{
			PlayerID: alicePlayerID, EntityID: aliceEntityID,
			Position: entrance, SightRange: 30,
		}))
		// rpg-toolkit#864: OpenDoor requires adjacency — walk alice up to
		// each door first (mirrors dungeon_test.go's identical fix).
		require.NoError(t, enc.Move(alicePlayerID, straightRowPath(entrance, dungeonRegionFarEdgeHex(0))))
		require.NoError(t, enc.OpenDoor(alicePlayerID, dungeonDoor0ID))
		require.NoError(t, enc.Move(alicePlayerID, straightRowPath(dungeonRegionFarEdgeHex(0), dungeonRegionFarEdgeHex(1))))
		require.NoError(t, enc.OpenDoor(alicePlayerID, dungeonDoor1ID))

		for localX := 0; localX < dungeonBossWidth; localX++ {
			pos := spatial.Position{X: float64(bossStart + localX), Y: float64(dungeonHeight / 2)}
			require.True(t, enc.Room().CanPlaceEntity(probeEntity{}, pos),
				"seed %d: boss primary-axis column %d must stay walkable even under heavy obstacle load", seed, localX)
		}
	}
}
