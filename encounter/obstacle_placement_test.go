package encounter_test

// obstacle_placement_test.go is the TDD gate for rpg-toolkit#819's GENERIC
// half: a content-agnostic mechanism on InitDungeon (rpg-toolkit#814) that
// places rpg-toolkit#818 ObstacleData instances into a region's floor from
// caller-supplied ObstacleSpec values, safely (never on a required-path or
// primary-combat-axis cell, never colliding with a wall/door/other
// obstacle), deterministically by seed, and skipping instances that don't
// fit rather than failing generation.
//
// This file never references crypt/coffin/altar/statue/obelisk/pillar —
// that content lives in crypt_dungeon_test.go, exercising the SAME
// mechanism this file proves out. See dungeon.go's ObstacleSpec/
// DungeonRegionParams.Obstacles doc for why the split: the geometry code
// must stay themeless, same discipline as Theme/Archetype already
// documented there.

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/KirkDiggler/rpg-toolkit/encounter"
	"github.com/KirkDiggler/rpg-toolkit/encounter/core"
	"github.com/KirkDiggler/rpg-toolkit/tools/environments"
	"github.com/KirkDiggler/rpg-toolkit/tools/spatial"
)

const (
	opSpecRefA = "test:obstacles:blocker-a"
	opSpecRefB = "test:obstacles:blocker-b"
)

// opTwoRegionParams builds a minimal, otherwise-valid 2-region DungeonParams
// (entrance -> chamber, PatternEmpty so wall placement never competes with
// obstacle placement for candidate cells) with obstacleSpecs attached to
// region 1 (the chamber) — the baseline every test in this file starts
// from and mutates.
func opTwoRegionParams(seed int64, specs []encounter.ObstacleSpec) encounter.DungeonParams {
	return encounter.DungeonParams{
		Height:     8,
		RandomSeed: seed,
		Regions: []encounter.DungeonRegionParams{
			{ID: "entrance", Archetype: encounter.ArchetypeEntrance, Width: 8, Pattern: environments.PatternEmpty},
			{
				ID: "chamber", Archetype: encounter.ArchetypeChamber, Width: 10, Pattern: environments.PatternEmpty,
				Obstacles: specs,
			},
		},
		Connectors: []encounter.DungeonConnectorParams{{DoorID: "door-0"}},
	}
}

// opNewEncounter mirrors newTestEncounter (dungeon_test.go) — a bare
// Encounter for tests that don't need the full suite fixture machinery.
func opNewEncounter(t *testing.T) *encounter.Encounter {
	t.Helper()
	transport := encounter.NewInMemoryTransport()
	broker := encounter.NewBroker(transport)
	t.Cleanup(func() {
		_ = broker.Close()
		_ = transport.Close()
	})
	return encounter.New(context.Background(), "enc-obstacle-placement-test", broker)
}

// TestInitDungeon_NoObstacleSpecs_PlacesNoObstacles: a DungeonRegionParams
// with a nil/empty Obstacles field (every existing InitDungeon caller,
// including InitTwoChamberRoom and every #814/#817/#818 fixture) must get
// exactly zero obstacles — the new mechanism is opt-in and fully backward
// compatible for generic, non-crypt callers.
func TestInitDungeon_NoObstacleSpecs_PlacesNoObstacles(t *testing.T) {
	enc := opNewEncounter(t)
	require.NoError(t, enc.InitDungeon(opTwoRegionParams(1, nil)))
	data := enc.ToData()
	require.Empty(t, data.Space.Obstacles, "no ObstacleSpec anywhere must yield zero placed obstacles")
}

// TestInitDungeon_ObstacleSpec_PlacesRequestedCountWithVerbatimFields:
// a single ObstacleSpec with Count N places exactly N ObstacleData
// entries, each carrying the spec's Ref/BlocksMovement/BlocksLoS verbatim
// — the mechanism never interprets or mutates caller-supplied content.
func TestInitDungeon_ObstacleSpec_PlacesRequestedCountWithVerbatimFields(t *testing.T) {
	enc := opNewEncounter(t)
	specs := []encounter.ObstacleSpec{
		{Ref: opSpecRefA, Count: 3, BlocksMovement: true, BlocksLoS: false},
	}
	require.NoError(t, enc.InitDungeon(opTwoRegionParams(1, specs)))
	data := enc.ToData()
	require.Len(t, data.Space.Obstacles, 3)
	for _, o := range data.Space.Obstacles {
		require.Equal(t, opSpecRefA, o.Ref)
		require.True(t, o.BlocksMovement)
		require.False(t, o.BlocksLoS)
	}
}

// TestInitDungeon_MultipleSpecsInOneRegion_AllPlacedWithoutCollision: two
// distinct specs in the SAME region must place their combined instance
// count with no two instances sharing a hex (validateObstacles/
// rebuildRoomFromData's occupancy check would otherwise reject the whole
// InitDungeon call — this test's NoError already proves no collision, but
// asserts the position-uniqueness directly too, for a discriminating
// failure message).
func TestInitDungeon_MultipleSpecsInOneRegion_AllPlacedWithoutCollision(t *testing.T) {
	enc := opNewEncounter(t)
	specs := []encounter.ObstacleSpec{
		{Ref: opSpecRefA, Count: 2, BlocksMovement: true, BlocksLoS: true},
		{Ref: opSpecRefB, Count: 2, BlocksMovement: true, BlocksLoS: false},
	}
	require.NoError(t, enc.InitDungeon(opTwoRegionParams(2, specs)))
	data := enc.ToData()
	require.Len(t, data.Space.Obstacles, 4)

	seen := make(map[core.Hex]bool, 4)
	for _, o := range data.Space.Obstacles {
		require.False(t, seen[o.Position], "obstacle %q collides with another at %v", o.ID, o.Position)
		seen[o.Position] = true
	}
}

// TestInitDungeon_ObstacleIDs_StableAndUnique: every placed obstacle's ID
// is non-empty and unique within the dungeon, and (same seed, same call
// shape) reproduces the IDENTICAL ID set across two independent Encounters
// — a host or client keys a future interaction verb off these IDs across
// ticks/reloads (data.go's ObstacleData.ID doc), so they must not be
// randomly generated.
func TestInitDungeon_ObstacleIDs_StableAndUnique(t *testing.T) {
	build := func() []core.EntityID {
		enc := opNewEncounter(t)
		specs := []encounter.ObstacleSpec{{Ref: opSpecRefA, Count: 4, BlocksMovement: true}}
		require.NoError(t, enc.InitDungeon(opTwoRegionParams(42, specs)))
		ids := make([]core.EntityID, 0, 4)
		seen := make(map[core.EntityID]bool, 4)
		for _, o := range enc.ToData().Space.Obstacles {
			require.NotEmpty(t, o.ID)
			require.False(t, seen[o.ID], "duplicate obstacle ID %q", o.ID)
			seen[o.ID] = true
			ids = append(ids, o.ID)
		}
		return ids
	}
	a := build()
	b := build()
	require.Equal(t, a, b, "the same seed and params must reproduce the identical obstacle ID set")
}

// TestInitDungeon_ObstaclePlacement_DeterministicSameSeed: the same seed
// reproduces byte-identical obstacle placement (ID, Ref, Position,
// BlocksMovement, BlocksLoS for every entry) across independent Encounters
// — mirrors TestLayout_DeterministicSeedVsEntropyDefault's wall-layout gate
// for obstacles specifically.
func TestInitDungeon_ObstaclePlacement_DeterministicSameSeed(t *testing.T) {
	specs := []encounter.ObstacleSpec{{Ref: opSpecRefA, Count: 5, BlocksMovement: true, BlocksLoS: true}}
	build := func() *encounter.Data {
		enc := opNewEncounter(t)
		require.NoError(t, enc.InitDungeon(opTwoRegionParams(909, specs)))
		return enc.ToData()
	}
	a := build()
	b := build()
	require.Equal(t, a.Space.Obstacles, b.Space.Obstacles, "identical seed must reproduce identical obstacle placement")
}

// TestInitDungeon_ObstaclePlacement_VariesAcrossSeeds: given a candidate
// pool much larger than the requested count (so the seeded shuffle order
// actually matters — "where guaranteed" per #819's done bar), different
// seeds must select different positions at least some of the time. Tries
// a handful of seed pairs, like TestLayout_DeterministicSeedVsEntropyDefault,
// to avoid flakiness from an unlucky pair landing on the same shuffle
// prefix.
func TestInitDungeon_ObstaclePlacement_VariesAcrossSeeds(t *testing.T) {
	specs := []encounter.ObstacleSpec{{Ref: opSpecRefA, Count: 2, BlocksMovement: true}}
	build := func(seed int64) *encounter.Data {
		enc := opNewEncounter(t)
		require.NoError(t, enc.InitDungeon(opTwoRegionParams(seed, specs)))
		return enc.ToData()
	}
	varied := false
	for seed := int64(1); seed < 20 && !varied; seed++ {
		a := build(seed)
		b := build(seed + 1000)
		if a.Space.Obstacles[0].Position != b.Space.Obstacles[0].Position ||
			a.Space.Obstacles[1].Position != b.Space.Obstacles[1].Position {
			varied = true
		}
	}
	require.True(t, varied, "different seeds must vary obstacle placement at least once across the sampled pairs")
}

// TestInitDungeon_ObstaclePlacement_NeverOnReservedRow: no placed obstacle
// may sit on the shared doorRow (Height/2) — the row generateDungeonLayout
// already reserves as every region's required connectivity path, and
// which (spanning the region's FULL width) also satisfies the boss
// archetype's primary-playable-axis invariant (#819's hard invariant).
// Uses a PatternRandom region (unlike this file's other tests) so the
// reserved-row exclusion is proven against real interior walls too, not
// just an empty floor.
func TestInitDungeon_ObstaclePlacement_NeverOnReservedRow(t *testing.T) {
	enc := opNewEncounter(t)
	params := encounter.DungeonParams{
		Height:     8,
		RandomSeed: 7,
		Regions: []encounter.DungeonRegionParams{
			{ID: "entrance", Archetype: encounter.ArchetypeEntrance, Width: 10, Pattern: environments.PatternEmpty},
			{
				ID: "boss", Archetype: encounter.ArchetypeBoss, Width: 10, Pattern: environments.PatternRandom,
				Obstacles: []encounter.ObstacleSpec{{Ref: opSpecRefA, Count: 40, BlocksMovement: true, BlocksLoS: true}},
			},
		},
		Connectors: []encounter.DungeonConnectorParams{{DoorID: "door-0"}},
	}
	require.NoError(t, enc.InitDungeon(params))
	data := enc.ToData()
	require.NotEmpty(t, data.Space.Obstacles, "fixture must actually place obstacles for this to be a real proof")

	const reservedDoorRowY = 4 // Height/2 = 8/2 = 4
	for _, o := range data.Space.Obstacles {
		pos := o.Position.ToPosition()
		require.NotEqual(t, float64(reservedDoorRowY), pos.Y,
			"obstacle %q at %v sits on the reserved doorRow (y=4); must never happen", o.ID, o.Position)
	}
}

// TestInitDungeon_ObstaclePlacement_PreservesEntranceToBossConnectivity:
// after placing a heavy obstacle load into the boss region, opening both
// doors must still connect the entrance all the way to the boss region —
// direct end-to-end proof (not just "not on the reserved row" in
// isolation) that obstacle placement never breaks the connectivity
// InitDungeon's wall generator already guarantees.
func TestInitDungeon_ObstaclePlacement_PreservesEntranceToBossConnectivity(t *testing.T) {
	enc := opNewEncounter(t)
	params := encounter.DungeonParams{
		Height:     8,
		RandomSeed: 77,
		Regions: []encounter.DungeonRegionParams{
			{
				ID: "entrance", Archetype: encounter.ArchetypeEntrance, Width: 10, Pattern: environments.PatternEmpty,
				Obstacles: []encounter.ObstacleSpec{{Ref: opSpecRefA, Count: 20, BlocksMovement: true, BlocksLoS: true}},
			},
			{
				ID: "corridor", Archetype: encounter.ArchetypeCorridor, Width: 5, Pattern: environments.PatternEmpty,
				Obstacles: []encounter.ObstacleSpec{{Ref: opSpecRefA, Count: 10, BlocksMovement: true, BlocksLoS: true}},
			},
			{
				ID: "boss", Archetype: encounter.ArchetypeBoss, Width: 10, Pattern: environments.PatternEmpty,
				Obstacles: []encounter.ObstacleSpec{{Ref: opSpecRefA, Count: 20, BlocksMovement: true, BlocksLoS: true}},
			},
		},
		Connectors: []encounter.DungeonConnectorParams{{DoorID: "door-0"}, {DoorID: "door-1"}},
	}
	require.NoError(t, enc.InitDungeon(params))

	data := enc.ToData()
	entrance := data.Space.Entrance
	require.NoError(t, enc.AddPlayer(encounter.PlayerInput{
		PlayerID: alicePlayerID, EntityID: aliceEntityID, Position: entrance, SightRange: 30,
	}))
	require.NoError(t, enc.OpenDoor(alicePlayerID, "door-0"))
	require.NoError(t, enc.OpenDoor(alicePlayerID, "door-1"))

	reachable := reachableFrom(enc.Room(), entrance)
	boss := regionHexSet(enc.ToData().Space, "boss")
	require.NotEmpty(t, boss)
	reachedBoss := false
	for h := range reachable {
		if boss[h] {
			reachedBoss = true
			break
		}
	}
	require.True(t, reachedBoss, "obstacle-heavy entrance/corridor/boss regions must still connect end to end")
}

// TestInitDungeon_ObstaclePlacement_SkipsWhenNoSafeHexRemains: a spec
// requesting far more instances than the region has safe candidate cells
// must place as many as fit and DROP the rest — InitDungeon must still
// succeed (no error), matching #819's "a crypt missing one statue is
// fine; a crypt that fails to generate ... is not" done-bar requirement.
func TestInitDungeon_ObstaclePlacement_SkipsWhenNoSafeHexRemains(t *testing.T) {
	enc := opNewEncounter(t)
	// A 4x4 region (the generator's own floor) has 16 cells; one row (4
	// cells) is the reserved doorRow, leaving at most 12 candidates. Ask
	// for far more than could ever fit.
	params := encounter.DungeonParams{
		Height:     4,
		RandomSeed: 3,
		Regions: []encounter.DungeonRegionParams{
			{ID: "entrance", Archetype: encounter.ArchetypeEntrance, Width: 4, Pattern: environments.PatternEmpty},
			{
				ID: "chamber", Archetype: encounter.ArchetypeChamber, Width: 4, Pattern: environments.PatternEmpty,
				Obstacles: []encounter.ObstacleSpec{{Ref: opSpecRefA, Count: 1000, BlocksMovement: true}},
			},
		},
		Connectors: []encounter.DungeonConnectorParams{{DoorID: "door-0"}},
	}
	err := enc.InitDungeon(params)
	require.NoError(t, err, "InitDungeon must never fail because an obstacle spec could not fully fit")

	data := enc.ToData()
	require.NotEmpty(t, data.Space.Obstacles, "some obstacles should still have fit")
	require.Less(t, len(data.Space.Obstacles), 1000, "an oversubscribed spec must place fewer than requested, not fail")
}

// TestInitDungeon_ObstaclePlacement_RoundTripsThroughToDataLoadFromData:
// placed obstacles survive a ToData -> LoadFromData round trip byte for
// byte, and the reloaded encounter's room re-derives the exact same
// movement/LoS blocking at each obstacle's position that the original
// (pre-reload) room had — proving rebuildRoomFromData (already exercised
// by #818's own tests) agrees with THIS mechanism's output on replay, not
// just on first build.
func TestInitDungeon_ObstaclePlacement_RoundTripsThroughToDataLoadFromData(t *testing.T) {
	enc := opNewEncounter(t)
	specs := []encounter.ObstacleSpec{
		{Ref: opSpecRefA, Count: 2, BlocksMovement: true, BlocksLoS: true},
		{Ref: opSpecRefB, Count: 2, BlocksMovement: false, BlocksLoS: false},
	}
	require.NoError(t, enc.InitDungeon(opTwoRegionParams(55, specs)))
	before := enc.ToData()

	transport2 := encounter.NewInMemoryTransport()
	broker2 := encounter.NewBroker(transport2)
	t.Cleanup(func() { _ = broker2.Close(); _ = transport2.Close() })
	reloaded, err := encounter.LoadFromData(context.Background(), before, broker2)
	require.NoError(t, err)

	after := reloaded.ToData()
	require.Equal(t, before.Space.Obstacles, after.Space.Obstacles, "obstacles must round-trip byte-for-byte")

	for _, o := range after.Space.Obstacles {
		blocked := !reloaded.Room().CanPlaceEntity(probeEntity{}, o.Position.ToPosition())
		require.Equal(t, o.BlocksMovement, blocked,
			"obstacle %q BlocksMovement=%v must match the reloaded room's actual movement block", o.ID, o.BlocksMovement)
	}
}

// opChamberOffsetX is opTwoRegionParams' chamber region's global column
// offset (entrance width 8 + 1 boundary column) -- needed to convert a
// placed obstacle's global Position back to the chamber's own LOCAL
// coordinates for the border-preference tests below. opChamberWidth/
// opChamberHeight mirror that same fixture's chamber region dimensions.
const (
	opChamberOffsetX = 8 + 1
	opChamberWidth   = 10
	opChamberHeight  = 8
)

// opIsBorderLocal reports whether local (x,y) sits on the chamber
// region's own four edges -- the "hugs walls/corners" preference
// placeRegionObstacles now draws from first (rpg-toolkit#839, composition
// ask from rpg-dnd5e-web#469).
func opIsBorderLocal(x, y int) bool {
	return x == 0 || x == opChamberWidth-1 || y == 0 || y == opChamberHeight-1
}

// TestInitDungeon_ObstaclePlacement_PrefersBorderCellsWhenTheyFit pins
// rpg-toolkit#839's composition ask ("dressing hugs walls/corners; floor
// centers stay clear") as implemented: a PER-SPEC, opt-in (PreferBorder
// bool) DRAW-ORDER preference in placeRegionObstacles, where border
// candidates (local x==0, x==width-1, y==0, y==height-1) are shuffled
// and drawn before interior candidates -- but ONLY for a spec that sets
// PreferBorder (rpg-toolkit#840 gate finding: an earlier revision
// applied this to every spec regardless of intent, which forced FOCAL
// pieces onto the border ahead of the dressing that actually wanted it).
// When a PreferBorder=true spec's Count comfortably fits within the
// region's border-cell capacity, every placed instance must land on the
// border -- across a spread of seeds, not just one lucky roll.
func TestInitDungeon_ObstaclePlacement_PrefersBorderCellsWhenTheyFit(t *testing.T) {
	for _, seed := range []int64{1, 2, 3, 4, 5, 42, 100, 909091} {
		enc := opNewEncounter(t)
		specs := []encounter.ObstacleSpec{
			{Ref: opSpecRefA, Count: 5, BlocksMovement: true, BlocksLoS: true, PreferBorder: true},
		}
		require.NoError(t, enc.InitDungeon(opTwoRegionParams(seed, specs)), "seed %d", seed)

		data := enc.ToData()
		require.Len(t, data.Space.Obstacles, 5, "seed %d", seed)
		for _, o := range data.Space.Obstacles {
			pos := o.Position.ToPosition()
			localX := int(pos.X) - opChamberOffsetX
			localY := int(pos.Y)
			require.True(t, opIsBorderLocal(localX, localY),
				"seed %d: obstacle %q at local (%d,%d) must be a border cell when a PreferBorder=true spec's "+
					"Count comfortably fits the requested count", seed, o.ID, localX, localY)
		}
	}
}

// TestInitDungeon_ObstaclePlacement_DefaultDoesNotPreferBorder proves
// PreferBorder's zero value (false) is genuinely UNBIASED, not merely
// "less biased" -- rpg-toolkit#840's whole reason to make this per-spec
// and opt-in. Requests the SAME Count (5) the biased test above proves
// always lands on the border when PreferBorder=true -- but here every
// spec leaves PreferBorder at its zero value. If this path were still
// secretly border-biased, every seed would place all 5 on the border,
// exactly like the test above. A genuine uniform draw must place at
// least one instance on an interior cell somewhere across this seed
// spread.
func TestInitDungeon_ObstaclePlacement_DefaultDoesNotPreferBorder(t *testing.T) {
	sawInterior := false
	for _, seed := range []int64{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 42, 100, 909091} {
		enc := opNewEncounter(t)
		specs := []encounter.ObstacleSpec{{Ref: opSpecRefA, Count: 5, BlocksMovement: true, BlocksLoS: true}}
		require.NoError(t, enc.InitDungeon(opTwoRegionParams(seed, specs)), "seed %d", seed)
		for _, o := range enc.ToData().Space.Obstacles {
			pos := o.Position.ToPosition()
			localX := int(pos.X) - opChamberOffsetX
			localY := int(pos.Y)
			if !opIsBorderLocal(localX, localY) {
				sawInterior = true
			}
		}
		if sawInterior {
			break
		}
	}
	require.True(t, sawInterior,
		"PreferBorder's zero value must be genuinely unbiased -- at least one seed in this spread must place "+
			"an instance on an interior cell when no spec opts into the border preference")
}

// TestInitDungeon_ObstaclePlacement_SpillsToInteriorWhenBorderPoolExhausted:
// the border preference above is a DRAW-ORDER bias, not a hard
// requirement on which cells are eligible -- a PreferBorder=true spec
// whose Count exceeds the region's border-cell capacity must still place
// its full best-effort count (never fail generation), with the overflow
// landing on interior cells. The chamber's border pool is the full-grid
// perimeter (2*10 + 2*8 - 4 corners = 32 cells) minus the two border
// cells doorRow's exclusion removes (local (0,4) and (9,4), the only
// border cells whose row is the reserved doorRow) = 30; total floor
// candidates are width*height minus one full doorRow (10*8 - 10 = 70).
// Requesting 40 (comfortably under 70, over 30) must place all 40, with
// exactly 30 on the border and the remaining 10 spilling to interior.
func TestInitDungeon_ObstaclePlacement_SpillsToInteriorWhenBorderPoolExhausted(t *testing.T) {
	enc := opNewEncounter(t)
	specs := []encounter.ObstacleSpec{
		{Ref: opSpecRefA, Count: 40, BlocksMovement: true, BlocksLoS: true, PreferBorder: true},
	}
	require.NoError(t, enc.InitDungeon(opTwoRegionParams(7, specs)))

	data := enc.ToData()
	require.Len(t, data.Space.Obstacles, 40, "40 comfortably fits the region's 70 total floor candidates")

	var borderCount, interiorCount int
	for _, o := range data.Space.Obstacles {
		pos := o.Position.ToPosition()
		localX := int(pos.X) - opChamberOffsetX
		localY := int(pos.Y)
		if opIsBorderLocal(localX, localY) {
			borderCount++
		} else {
			interiorCount++
		}
	}
	require.Equal(t, 30, borderCount, "every border cell must be exhausted before any interior cell is used")
	require.Equal(t, 10, interiorCount, "the remaining instances must spill to interior cells")
}

// TestInitDungeon_ObstaclePlacement_PreferBorderSpecsDrawBeforeNormalSpecs
// pins rpg-toolkit#840's exact gate finding and fix: a region mixing a
// PreferBorder=true spec with a PreferBorder=false spec (mirroring
// CryptDungeonParams' own composition -- e.g. entrance's obelisk/pillar
// left false, brazier/bone-pile set true) must place the border-
// preferring spec's instances on the border regardless of which spec is
// declared FIRST in p.specs -- the earlier revision's bug was that list
// order (not PreferBorder) decided who got first crack at the border,
// which put a region's always-first-listed FOCAL piece there instead of
// its dressing. Declares the NON-preferring spec first (matching every
// crypt region's own declaration order: focal piece first, dressing
// after), with a Count (28) deliberately large enough to nearly exhaust
// the 30-cell border pool on its own -- if list order still controlled
// draw order (the bug), the non-preferring spec would claim almost all
// of the border before the preferring spec ever got a turn, forcing most
// of its 5 instances into the interior. A Count of 1 here would NOT
// discriminate (confirmed: re-run against a temporarily-reintroduced
// version of the bug with Count:1 and it stayed green by coincidence --
// the border pool is large enough that one focal instance never
// contests it. 28 does.).
func TestInitDungeon_ObstaclePlacement_PreferBorderSpecsDrawBeforeNormalSpecs(t *testing.T) {
	for _, seed := range []int64{1, 2, 3, 4, 5, 42} {
		enc := opNewEncounter(t)
		specs := []encounter.ObstacleSpec{
			// Declared FIRST but PreferBorder=false -- mirrors a focal
			// piece (e.g. crypt's obelisk) declared ahead of its dressing.
			// Count=28 nearly exhausts the 30-cell border pool on its own.
			{Ref: opSpecRefB, Count: 28, BlocksMovement: true, BlocksLoS: true},
			// Declared SECOND but PreferBorder=true -- mirrors dressing
			// declared after its region's focal piece.
			{Ref: opSpecRefA, Count: 5, BlocksMovement: true, BlocksLoS: true, PreferBorder: true},
		}
		require.NoError(t, enc.InitDungeon(opTwoRegionParams(seed, specs)), "seed %d", seed)

		for _, o := range enc.ToData().Space.Obstacles {
			if o.Ref != opSpecRefA {
				continue // only asserting the PreferBorder=true spec's placements
			}
			pos := o.Position.ToPosition()
			localX := int(pos.X) - opChamberOffsetX
			localY := int(pos.Y)
			require.True(t, opIsBorderLocal(localX, localY),
				"seed %d: obstacle %q at local (%d,%d) must be a border cell -- PreferBorder must win over "+
					"list order, even though the non-preferring spec (Count=28, nearly exhausting the border "+
					"pool on its own) was declared first", seed, o.ID, localX, localY)
		}
	}
}

// opTwoRegionParamsWithPlaced mirrors opTwoRegionParams but also attaches
// PlacedObstacleSpecs to the chamber region alongside its rolled specs —
// proving the two placement mechanisms coexist in the same region
// (design.md §Design delta: "place coexists with the count-based
// obstacles... list in the same room; placed entries are honored first,
// then count-based entries roll into the remaining safe cells").
func opTwoRegionParamsWithPlaced(
	seed int64, placed []encounter.PlacedObstacleSpec, specs []encounter.ObstacleSpec,
) encounter.DungeonParams {
	params := opTwoRegionParams(seed, specs)
	params.Regions[1].PlacedObstacles = placed
	return params
}

// TestInitDungeon_PlacedObstaclesLandVerbatim proves a PlacedObstacleSpec
// lands at EXACTLY its declared room-local cell, translated by the
// region's offsetX — not just "an obstacle with this ref exists somewhere
// in the region" (design.md §Design delta).
func TestInitDungeon_PlacedObstaclesLandVerbatim(t *testing.T) {
	enc := opNewEncounter(t)
	placed := []encounter.PlacedObstacleSpec{
		{Ref: opSpecRefA, At: encounter.LocalHex{Col: 6, Row: 3}, BlocksMovement: true, BlocksLoS: false},
	}
	require.NoError(t, enc.InitDungeon(opTwoRegionParamsWithPlaced(1, placed, nil)))

	data := enc.ToData()
	require.Len(t, data.Space.Obstacles, 1)

	want := core.HexFromPosition(spatial.Position{X: float64(opChamberOffsetX + 6), Y: float64(3)})
	got := data.Space.Obstacles[0]
	require.Equal(t, opSpecRefA, got.Ref)
	require.Equal(t, want, got.Position, "placed obstacle must land at exactly its declared cell, translated by offsetX")
	require.True(t, got.BlocksMovement)
	require.False(t, got.BlocksLoS)
}

// TestInitDungeon_RolledObstaclesNeverUsePlacedCells proves placed cells
// are excluded from the rolled candidate pool: across a seed sweep, no
// rolled ObstacleData ever lands on a placed cell's cube coordinate.
func TestInitDungeon_RolledObstaclesNeverUsePlacedCells(t *testing.T) {
	placed := []encounter.PlacedObstacleSpec{
		{Ref: opSpecRefA, At: encounter.LocalHex{Col: 6, Row: 3}},
		{Ref: opSpecRefA, At: encounter.LocalHex{Col: 2, Row: 2}},
		{Ref: opSpecRefA, At: encounter.LocalHex{Col: 8, Row: 6}},
	}
	rolled := []encounter.ObstacleSpec{{Ref: opSpecRefB, Count: 20, BlocksMovement: true}}

	placedCells := make(map[core.Hex]bool, len(placed))
	for _, p := range placed {
		placedCells[core.HexFromPosition(spatial.Position{
			X: float64(opChamberOffsetX + p.At.Col), Y: float64(p.At.Row),
		})] = true
	}

	for seed := int64(1); seed <= 20; seed++ {
		enc := opNewEncounter(t)
		require.NoError(t, enc.InitDungeon(opTwoRegionParamsWithPlaced(seed, placed, rolled)), "seed %d", seed)
		for _, o := range enc.ToData().Space.Obstacles {
			if o.Ref != opSpecRefB {
				continue // only the rolled spec's instances are under test here
			}
			require.False(t, placedCells[o.Position],
				"seed %d: rolled obstacle %q landed on a placed cell %v", seed, o.ID, o.Position)
		}
	}
}

// TestInitDungeon_PlacedObstacleOnReservedRowRejected: InitDungeon rejects
// a PlacedObstacleSpec whose At.Row == height/2 — belt-and-suspenders with
// dungeonspec's own load-time check (Task B2), since InitDungeon is a
// public toolkit entry point other callers besides dungeonspec could reach
// directly.
func TestInitDungeon_PlacedObstacleOnReservedRowRejected(t *testing.T) {
	enc := opNewEncounter(t)
	placed := []encounter.PlacedObstacleSpec{
		{Ref: opSpecRefA, At: encounter.LocalHex{Col: 5, Row: opChamberHeight / 2}},
	}
	err := enc.InitDungeon(opTwoRegionParamsWithPlaced(1, placed, nil))
	require.Error(t, err)
	require.Contains(t, err.Error(), "reserved row")
}

// TestInitDungeon_PlacedObstacleCollisionRejected: two PlacedObstacleSpecs
// at the same At, or one at a wall cell (PatternRandom region), is a hard
// InitDungeon error — placed entries are guarantees, not best-effort
// (design.md §Validation).
func TestInitDungeon_PlacedObstacleCollisionRejected(t *testing.T) {
	t.Run("two placed obstacles at the same cell", func(t *testing.T) {
		enc := opNewEncounter(t)
		placed := []encounter.PlacedObstacleSpec{
			{Ref: opSpecRefA, At: encounter.LocalHex{Col: 6, Row: 3}},
			{Ref: opSpecRefB, At: encounter.LocalHex{Col: 6, Row: 3}},
		}
		err := enc.InitDungeon(opTwoRegionParamsWithPlaced(1, placed, nil))
		require.Error(t, err)
		require.Contains(t, err.Error(), "collides with placed obstacle")
		// Names BOTH obstacles involved, not just the later (colliding) one.
		require.Contains(t, err.Error(), opSpecRefA)
		require.Contains(t, err.Error(), opSpecRefB)
	})

	t.Run("placed obstacle on a wall cell is rejected", func(t *testing.T) {
		// PatternRandom's interior walls are seed-dependent -- discover an
		// actual wall cell for some seed first (a plain probe run, no
		// placed obstacles), then re-run WITH a PlacedObstacleSpec at that
		// exact cell using the SAME seed, expecting rejection. Wall layout
		// depends only on RandomSeed + region shape, never on
		// PlacedObstacles content, so the discovered cell is still a real
		// wall cell in the second run.
		var wallCell encounter.LocalHex
		var seedUsed int64
		found := false
		for seed := int64(1); seed <= 50 && !found; seed++ {
			probeParams := opTwoRegionParams(seed, nil)
			probeParams.Regions[1].Pattern = environments.PatternRandom
			probe := opNewEncounter(t)
			require.NoError(t, probe.InitDungeon(probeParams), "seed %d", seed)
			for _, w := range probe.ToData().Space.Walls {
				if w.Start != w.End {
					continue // boundary-edge perimeter segment, not an interior wall cell
				}
				pos := w.Start.ToOffsetCoordinateWithOrientation(spatial.HexOrientationPointyTop)
				localCol := int(pos.X) - opChamberOffsetX
				localRow := int(pos.Y)
				if localCol >= 0 && localCol < opChamberWidth && localRow != opChamberHeight/2 {
					wallCell = encounter.LocalHex{Col: localCol, Row: localRow}
					seedUsed = seed
					found = true
					break
				}
			}
		}
		require.True(t, found, "expected at least one seed in [1,50] to produce an interior wall cell in the chamber")

		enc := opNewEncounter(t)
		placed := []encounter.PlacedObstacleSpec{{Ref: opSpecRefA, At: wallCell}}
		params := opTwoRegionParamsWithPlaced(seedUsed, placed, nil)
		params.Regions[1].Pattern = environments.PatternRandom
		err := enc.InitDungeon(params)
		require.Error(t, err)
		require.Contains(t, err.Error(), "wall cell")
	})
}

// TestInitDungeon_PlacedObstaclesDeterministicSameSeed: the same seed
// reproduces byte-identical placement (placed AND rolled obstacles
// together) across independent Encounters — mirrors
// TestInitDungeon_ObstaclePlacement_DeterministicSameSeed's rolled-only
// gate; determinism must hold with placements present too.
func TestInitDungeon_PlacedObstaclesDeterministicSameSeed(t *testing.T) {
	placed := []encounter.PlacedObstacleSpec{
		{Ref: opSpecRefA, At: encounter.LocalHex{Col: 6, Row: 3}, BlocksMovement: true},
		{Ref: opSpecRefA, At: encounter.LocalHex{Col: 2, Row: 2}, BlocksMovement: true},
	}
	rolled := []encounter.ObstacleSpec{{Ref: opSpecRefB, Count: 5, BlocksMovement: true, BlocksLoS: true}}
	build := func() *encounter.Data {
		enc := opNewEncounter(t)
		require.NoError(t, enc.InitDungeon(opTwoRegionParamsWithPlaced(909, placed, rolled)))
		return enc.ToData()
	}
	a := build()
	b := build()
	require.Equal(t, a.Space.Obstacles, b.Space.Obstacles,
		"identical seed must reproduce identical placement, placed and rolled together")
}

// TestInitDungeon_PlacedObstacleOutOfBoundsRejected: InitDungeon rejects a
// PlacedObstacleSpec whose At falls outside [0,width)x[0,height) — the
// same defense-in-depth rationale as the reserved-row check, since
// InitDungeon is a public toolkit entry point other callers besides
// dungeonspec's own load-time Validate could reach directly. Without this
// check, an out-of-bounds col lands in the connector's boundary/door
// column or an adjacent region's own span (failing downstream in
// rebuildRoomFromData with a message naming neither region nor local
// cell), and an out-of-bounds row degrades to a bare "hex already
// occupied" the moment it happens to collide with something.
func TestInitDungeon_PlacedObstacleOutOfBoundsRejected(t *testing.T) {
	cases := []struct {
		name string
		at   encounter.LocalHex
	}{
		{"col at the door/boundary column (one past the right edge)", encounter.LocalHex{Col: opChamberWidth, Row: 2}},
		{"col inside the next region's span", encounter.LocalHex{Col: opChamberWidth + 3, Row: 2}},
		{"col before the left edge", encounter.LocalHex{Col: -1, Row: 2}},
		{"row below the floor", encounter.LocalHex{Col: 2, Row: opChamberHeight}},
		{"row above the floor", encounter.LocalHex{Col: 2, Row: -1}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			enc := opNewEncounter(t)
			placed := []encounter.PlacedObstacleSpec{{Ref: opSpecRefA, At: tc.at}}
			err := enc.InitDungeon(opTwoRegionParamsWithPlaced(7, placed, nil))
			require.Error(t, err)
			require.Contains(t, err.Error(), "out of bounds")
		})
	}
}

// TestInitDungeon_PreferBorderExcludesPlacedBorderCell proves the
// PreferBorder border/interior partition (dungeon.go's second draw-order
// branch, distinct from the flat pool
// TestInitDungeon_RolledObstaclesNeverUsePlacedCells already exercises)
// ALSO excludes placed cells. A placed obstacle sits ON a border cell,
// with a PreferBorder rolled spec sized to exactly saturate the
// unreduced border pool: if the partition's placed-cell exclusion were
// missing, the border-preferring spec's shuffle would eventually draw
// the placed cell too, since border+placed together exceed the pool by
// exactly one — a deterministic collision, not a rare one, so a single
// seed sweep per cell is enough to catch a missing exclusion.
func TestInitDungeon_PreferBorderExcludesPlacedBorderCell(t *testing.T) {
	cases := []encounter.LocalHex{
		{Col: 0, Row: 0}, // corner
		{Col: opChamberWidth - 1, Row: opChamberHeight - 1}, // opposite corner
		{Col: 5, Row: 0}, // top edge
		{Col: 0, Row: 3}, // left edge, adjacent to doorRow
		{Col: 4, Row: 2}, // interior (control)
	}
	for _, at := range cases {
		for seed := int64(1); seed <= 30; seed++ {
			placed := []encounter.PlacedObstacleSpec{{Ref: opSpecRefA, At: at, BlocksMovement: true}}
			// Border pool for a 10x8 region: perimeter 2*10+2*8-4=32 cells,
			// minus the two doorRow (y=4) border cells = 30. Count=30
			// exactly saturates the unreduced pool, so a missing
			// exclusion forces a deterministic collision with the placed
			// cell whenever it sits on the border.
			rolled := []encounter.ObstacleSpec{{Ref: opSpecRefB, Count: 30, PreferBorder: true}}

			enc := opNewEncounter(t)
			require.NoError(t, enc.InitDungeon(opTwoRegionParamsWithPlaced(seed, placed, rolled)),
				"at=%v seed=%d", at, seed)

			want := core.HexFromPosition(spatial.Position{X: float64(opChamberOffsetX + at.Col), Y: float64(at.Row)})
			sawPlaced := false
			for _, o := range enc.ToData().Space.Obstacles {
				switch o.Ref {
				case opSpecRefA:
					sawPlaced = true
					require.Equal(t, want, o.Position,
						"at=%v seed=%d: placed obstacle must land at its declared cell", at, seed)
				case opSpecRefB:
					require.NotEqual(t, want, o.Position,
						"at=%v seed=%d: rolled obstacle %q must never land on the placed cell", at, seed, o.ID)
				}
			}
			require.True(t, sawPlaced, "at=%v seed=%d: placed obstacle missing", at, seed)
		}
	}
}

// TestInitDungeon_PreferBorderRemainderPathExcludesPlacedCells proves the
// remainder-reshuffle path — normalSpecs drawing from whatever the
// PreferBorder specs left over, via drawObstaclesFrom's idOffset+len(out)
// ID continuation — still produces unique IDs, no cell collisions, and
// never draws a placed cell, across a mix of PreferBorder and normal
// specs and several placed entries at once (mirrors rpg-toolkit#840's own
// PreferBorder-vs-normal-spec gate, extended to placed cells).
func TestInitDungeon_PreferBorderRemainderPathExcludesPlacedCells(t *testing.T) {
	placed := []encounter.PlacedObstacleSpec{
		{Ref: opSpecRefA, At: encounter.LocalHex{Col: 0, Row: 0}},
		{Ref: opSpecRefA, At: encounter.LocalHex{Col: opChamberWidth - 1, Row: opChamberHeight - 1}},
		{Ref: opSpecRefA, At: encounter.LocalHex{Col: 3, Row: 5}},
		{Ref: opSpecRefA, At: encounter.LocalHex{Col: 6, Row: 1}},
	}
	rolled := []encounter.ObstacleSpec{
		{Ref: opSpecRefB, Count: 20, PreferBorder: true},
		{Ref: opSpecRefB, Count: 25},
		{Ref: opSpecRefB, Count: 15, PreferBorder: true},
	}
	placedCells := make(map[core.Hex]bool, len(placed))
	for _, p := range placed {
		placedCells[core.HexFromPosition(spatial.Position{
			X: float64(opChamberOffsetX + p.At.Col), Y: float64(p.At.Row),
		})] = true
	}

	for seed := int64(1); seed <= 40; seed++ {
		enc := opNewEncounter(t)
		require.NoError(t, enc.InitDungeon(opTwoRegionParamsWithPlaced(seed, placed, rolled)), "seed %d", seed)

		ids := make(map[core.EntityID]bool)
		cells := make(map[core.Hex]core.EntityID)
		nPlaced := 0
		for _, o := range enc.ToData().Space.Obstacles {
			require.False(t, ids[o.ID], "seed %d: duplicate obstacle ID %q", seed, o.ID)
			ids[o.ID] = true
			if prev, dup := cells[o.Position]; dup {
				t.Fatalf("seed %d: obstacles %q and %q collide at %v", seed, prev, o.ID, o.Position)
			}
			cells[o.Position] = o.ID
			if o.Ref == opSpecRefA {
				nPlaced++
				continue
			}
			require.False(t, placedCells[o.Position],
				"seed %d: rolled obstacle %q landed on a placed cell %v", seed, o.ID, o.Position)
		}
		require.Equal(t, len(placed), nPlaced, "seed %d", seed)
	}
}
