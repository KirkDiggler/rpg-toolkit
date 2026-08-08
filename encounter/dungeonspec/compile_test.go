// Copyright (C) 2024 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package dungeonspec_test

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/KirkDiggler/rpg-toolkit/encounter"
	"github.com/KirkDiggler/rpg-toolkit/encounter/core"
	"github.com/KirkDiggler/rpg-toolkit/encounter/dungeonspec"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/monster"
	"github.com/KirkDiggler/rpg-toolkit/tools/environments"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const facingCoffinRef = "dnd5e:props:coffin"

func TestLoad_GeneratesDeterministicDoorIDs(t *testing.T) {
	// placedTombYAML: key "reference-tomb", 2 rooms (entrance, tomb), 1 connector —
	// the M1-valid fixture (referenceYAML's own count-based monsters fail Validate
	// in M1; see Task B2's fixture consequence). N-connector coverage (3+ doors in
	// one file) resumes in M2 once referenceYAML/cryptYAML are loadable again.
	compiled, err := dungeonspec.Load([]byte(placedTombYAML))
	require.NoError(t, err)
	require.Len(t, compiled.Params.Connectors, 1)
	assert.Equal(t, core.EntityID("reference-tomb-door-entrance-tomb"),
		compiled.Params.Connectors[0].DoorID)

	// Plain scalar carry-throughs -- currently swappable with the suite
	// green if compile() dropped or mis-wired either field.
	assert.Equal(t, 8, compiled.Params.Height)
	assert.Equal(t, "crypt", compiled.Params.Theme)
}

// TestLoad_ScatteredPatternMapsToPatternRandom exercises the
// scattered->PatternRandom half of the PATTERN-DEFAULT TRAP (see
// compilePattern's doc, compile.go, for what the trap is and why it
// matters). placedTombYAML's own rooms are both empty-pattern (no room
// in it is scattered -- Task B2's scattered-rejection rows exercise
// scattered ONLY in combination with place/boss.at, never the plain
// mapping), so this direction needs its own small fixture, decoupled
// from placedTombYAML/validM1YAML entirely.
func TestLoad_ScatteredPatternMapsToPatternRandom(t *testing.T) {
	// Throwaway 2-room fixture threading all three interacting rules
	// correctly (a fixture missing any one of them fails Validate for an
	// UNRELATED reason before this test's actual assertion ever runs):
	// exactly one boss-archetype room is required (v1's own rule) with a
	// non-nil boss: entry (M1's permanent boss-required rule, Task B2) and
	// a pinned at: (M1's temporary at-pinning rule, lifted in M2's Task
	// C0) -- so the boss room carries `pattern` empty/default (NOT
	// scattered; the scattered-rejection rule only fires on a room that
	// HAS place/pinned boss.at, and this room does), width 7 (height 8,
	// boss primary axis needs min(width,height) > 6). The entrance room
	// is the one under test: pattern: scattered, no place/boss.at, so
	// none of the pinning rules apply to it at all.
	const scatteredYAML = `
version: 1
key: pattern-mapping-check
name: Pattern Mapping Check
height: 8
rooms:
  - {id: entrance, archetype: entrance, width: 6, pattern: scattered}
  - {id: boss-room, archetype: boss, width: 7, boss: {ref: "dnd5e:monsters:skeleton-captain", at: [3, 5]}}
connectors:
  - {from: entrance, to: boss-room}
`
	// scattered is legal on its own -- design.md's clarification.
	compiled, err := dungeonspec.Load([]byte(scatteredYAML))
	require.NoError(t, err)
	// entrance: scattered -> PatternRandom.
	assert.Equal(t, environments.PatternRandom, compiled.Params.Regions[0].Pattern)
	// boss-room: default -> PatternEmpty, same test.
	assert.Equal(t, environments.PatternEmpty, compiled.Params.Regions[1].Pattern)
}

// regionByID finds a compiled region by id, failing the test loudly if
// absent, so callers don't have to repeat a linear scan + nil check.
func regionByID(t *testing.T, regions []encounter.DungeonRegionParams, id string) *encounter.DungeonRegionParams {
	t.Helper()
	for i := range regions {
		if regions[i].ID == id {
			return &regions[i]
		}
	}
	t.Fatalf("no region %q in compiled regions", id)
	return nil
}

// TestLoad_FloorPropFacingCompilesEveryCanonicalDirection covers Slice #178's
// canonical YAML mapping. The six labels must compile in their fixed
// E,NE,NW,W,SW,SE -> 0..5 order; no current compiler path may strip them.
func TestLoad_FloorPropFacingCompilesEveryCanonicalDirection(t *testing.T) {
	const facingYAML = `
version: 1
key: facing-check
name: Facing Check
height: 8
rooms:
  - {id: entrance, archetype: entrance, width: 6}
  - id: tomb
    archetype: boss
    width: 8
    boss: {ref: "dnd5e:monsters:skeleton-captain", at: [4, 5]}
    place:
      - {ref: "` + facingCoffinRef + `", at: [0, 0], facing: E}
      - {ref: "dnd5e:props:altar", at: [1, 0], facing: NE}
      - {ref: "dnd5e:props:statue-reaper", at: [2, 0], facing: NW}
      - {ref: "dnd5e:props:brazier", at: [3, 0], facing: W}
      - {ref: "dnd5e:props:candles", at: [4, 0], facing: SW}
      - {ref: "dnd5e:props:wall-banner", at: [5, 0], facing: SE}
connectors:
  - {from: entrance, to: tomb}
`

	compiled, err := dungeonspec.Load([]byte(facingYAML))
	require.NoError(t, err)
	tomb := regionByID(t, compiled.Params.Regions, "tomb")
	require.Len(t, tomb.PlacedObstacles, 6)
	wantFacings := []uint32{
		encounter.FacingEast,
		encounter.FacingNortheast,
		encounter.FacingNorthwest,
		encounter.FacingWest,
		encounter.FacingSouthwest,
		encounter.FacingSoutheast,
	}
	for index, obstacle := range tomb.PlacedObstacles {
		require.NotNil(t, obstacle.Facing, "placement %d must retain facing presence", index)
		assert.Equal(t, wantFacings[index], *obstacle.Facing)
	}
}

// TestLoad_FloorPropFacingPreservesEastAndAbsence runs the actual compiler and
// dungeon initializer so E=0 remains a present override while omitted/null
// stay absent through the persisted obstacle state. Rolled obstacles must
// never gain a facing from this metadata path.
func TestLoad_FloorPropFacingPreservesEastAndAbsence(t *testing.T) {
	withEast := strings.Replace(placedTombYAML,
		"at: [6, 3], blocks_los: false }",
		"at: [6, 3], blocks_los: false, facing: E }", 1)
	withNull := strings.Replace(placedTombYAML,
		"at: [6, 3], blocks_los: false }",
		"at: [6, 3], blocks_los: false, facing: null }", 1)

	omitted, err := dungeonspec.Load([]byte(placedTombYAML))
	require.NoError(t, err)
	nullFacing, err := dungeonspec.Load([]byte(withNull))
	require.NoError(t, err)
	east, err := dungeonspec.Load([]byte(withEast))
	require.NoError(t, err)

	omittedProp := regionByID(t, omitted.Params.Regions, "tomb").PlacedObstacles[0]
	nullProp := regionByID(t, nullFacing.Params.Regions, "tomb").PlacedObstacles[0]
	eastProp := regionByID(t, east.Params.Regions, "tomb").PlacedObstacles[0]
	assert.Nil(t, omittedProp.Facing)
	assert.Nil(t, nullProp.Facing)
	require.NotNil(t, eastProp.Facing)
	assert.Equal(t, encounter.FacingEast, *eastProp.Facing)

	east.Params.Regions[0].Obstacles = []encounter.ObstacleSpec{{
		Ref: "dnd5e:props:pillar", Count: 1, BlocksMovement: true, BlocksLoS: true,
	}}
	east.Params.RandomSeed = 19
	transport := encounter.NewInMemoryTransport()
	broker := encounter.NewBroker(transport)
	t.Cleanup(func() { _ = broker.Close(); _ = transport.Close() })
	enc := encounter.New(context.Background(), "facing-persistence", broker)
	require.NoError(t, enc.InitDungeon(east.Params))

	var authored, rolled *encounter.ObstacleData
	for index := range enc.ToData().Space.Obstacles {
		obstacle := &enc.ToData().Space.Obstacles[index]
		switch obstacle.Ref {
		case facingCoffinRef:
			authored = obstacle
		case "dnd5e:props:pillar":
			rolled = obstacle
		}
	}
	require.NotNil(t, authored)
	require.NotNil(t, authored.Facing)
	assert.Equal(t, encounter.FacingEast, *authored.Facing)
	require.NotNil(t, rolled)
	assert.Nil(t, rolled.Facing)

	payload, err := json.Marshal(enc.ToData())
	require.NoError(t, err)
	var persisted encounter.Data
	require.NoError(t, json.Unmarshal(payload, &persisted))
	reloaded, err := encounter.LoadFromData(context.Background(), &persisted, broker)
	require.NoError(t, err)
	var reloadedAuthored *encounter.ObstacleData
	for index := range reloaded.ToData().Space.Obstacles {
		obstacle := &reloaded.ToData().Space.Obstacles[index]
		if obstacle.Ref == facingCoffinRef {
			reloadedAuthored = obstacle
			break
		}
	}
	require.NotNil(t, reloadedAuthored)
	require.NotNil(t, reloadedAuthored.Facing)
	assert.Equal(t, encounter.FacingEast, *reloadedAuthored.Facing)
}

func TestLoad_PlaceRoutesByRefType(t *testing.T) {
	compiled, err := dungeonspec.Load([]byte(placedTombYAML))
	require.NoError(t, err)
	tombRegion := regionByID(t, compiled.Params.Regions, "tomb")

	// Plain scalar carry-throughs -- currently swappable with the suite
	// green if compileRoom dropped or mis-wired either field.
	assert.Equal(t, 12, tombRegion.Width)
	assert.Equal(t, encounter.ArchetypeBoss, tombRegion.Archetype)

	require.Len(t, tombRegion.PlacedObstacles, 5) // coffin, altar, statue-reaper, brazier x2
	assert.Equal(t, "dnd5e:props:coffin", tombRegion.PlacedObstacles[0].Ref)
	assert.Equal(t, encounter.LocalHex{Col: 6, Row: 3}, tombRegion.PlacedObstacles[0].At)
	assert.False(t, tombRegion.PlacedObstacles[0].BlocksLoS)

	// BLOCKING-DEFAULT TRAP (advisory made explicit, same class as the
	// PATTERN-DEFAULT TRAP above): PlacedEntry.BlocksMovement/BlocksLoS are
	// *bool with nil => true (design.md: "same defaults/semantics as
	// ObstacleEntry"), but PlacedObstacleSpec.BlocksMovement/BlocksLoS
	// (engine-side, Task N1) are plain bool, whose Go zero value is FALSE.
	// A compiler that just dereferences a nil *bool panics; one that
	// silently treats nil as the zero value inverts the default (an
	// unflagged placed prop would compile to "blocks nothing," backwards
	// from the design's intent). The altar entry in placedTombYAML sets
	// NEITHER flag, so it's the direct test of this: it must compile to
	// BlocksMovement: true, BlocksLoS: true (matching the existing
	// crypt_dungeon.go precedent: altar is a structural piece, blocks both).
	altar := tombRegion.PlacedObstacles[1]
	assert.Equal(t, "dnd5e:props:altar", altar.Ref)
	assert.True(t, altar.BlocksMovement)
	assert.True(t, altar.BlocksLoS)

	var skeletonSpawn *dungeonspec.SpawnInstruction
	for i := range compiled.Spawns {
		if compiled.Spawns[i].MonsterRef == "dnd5e:monsters:skeleton" {
			skeletonSpawn = &compiled.Spawns[i]
		}
	}
	require.NotNil(t, skeletonSpawn)
	require.NotNil(t, skeletonSpawn.At)
	assert.Equal(t, encounter.LocalHex{Col: 4, Row: 2}, *skeletonSpawn.At)
	assert.Equal(t, 1, skeletonSpawn.Count)
	assert.Equal(t, "tomb", skeletonSpawn.RoomID)
}

func TestLoad_BossAtCompilesToSpawnPosition(t *testing.T) {
	compiled, err := dungeonspec.Load([]byte(placedTombYAML))
	require.NoError(t, err)
	require.NotEmpty(t, compiled.Spawns)
	boss := compiled.Spawns[0] // boss-first ordering unchanged
	assert.Equal(t, "dnd5e:monsters:skeleton-captain", boss.MonsterRef)
	require.NotNil(t, boss.At)
	assert.Equal(t, encounter.LocalHex{Col: 7, Row: 5}, *boss.At)
}

// TestLoad_TargetingCompilesAtBothWiringSites (rpg-toolkit#895): a
// SpawnInstruction.Targeting override reaches the compiled dungeon from
// BOTH sites that build one — the boss spawn (compileWithConfig) and a
// room-chain place entry's refTypeMonsters case (compileRoom) — and a
// spawn whose author left targeting unset compiles with a nil Targeting
// (never a zero-value TargetClosest standing in for "unset", which would
// silently stomp a ctor's own non-closest default at seed time).
// placedTombWithTargetingYAML is placedTombYAML plus `targeting:` on the
// boss and the skeleton place entry — kept as its own fixture rather than
// mutating placedTombYAML itself, which several other tests in this file
// assert exact PlacedObstacles/Spawns shapes against.
func TestLoad_TargetingCompilesAtBothWiringSites(t *testing.T) {
	const placedTombWithTargetingYAML = `
version: 1
key: reference-tomb
name: The Reference Tomb
theme: crypt
height: 8
rooms:
  - id: entrance
    archetype: entrance
    width: 6
  - id: tomb
    archetype: boss
    width: 12
    boss: { ref: "dnd5e:monsters:skeleton-captain", at: [7, 5], targeting: lowest-health }
    place:
      - { ref: "dnd5e:props:coffin",        at: [6, 3], blocks_los: false }
      - { ref: "dnd5e:props:altar",         at: [9, 3] }
      - { ref: "dnd5e:props:statue-reaper", at: [1, 1] }
      - { ref: "dnd5e:props:brazier",       at: [3, 1] }
      - { ref: "dnd5e:props:brazier",       at: [3, 6] }
      - { ref: "dnd5e:monsters:skeleton",   at: [4, 2], targeting: closest }
connectors:
  - { from: entrance, to: tomb }
`
	compiled, err := dungeonspec.Load([]byte(placedTombWithTargetingYAML))
	require.NoError(t, err)
	require.Len(t, compiled.Spawns, 2)

	boss := compiled.Spawns[0] // boss-first ordering unchanged
	assert.Equal(t, "dnd5e:monsters:skeleton-captain", boss.MonsterRef)
	assert.Equal(t, monster.TargetLowestHP, boss.Targeting,
		"boss spawn (compileWithConfig) must thread its targeting override")

	var skeletonSpawn *dungeonspec.SpawnInstruction
	for i := range compiled.Spawns {
		if compiled.Spawns[i].MonsterRef == "dnd5e:monsters:skeleton" {
			skeletonSpawn = &compiled.Spawns[i]
		}
	}
	require.NotNil(t, skeletonSpawn)
	assert.Equal(t, monster.TargetClosest, skeletonSpawn.Targeting,
		"place-block refTypeMonsters spawn (compileRoom) must thread its targeting override")

	// The unmodified placedTombYAML fixture (no targeting: anywhere) must
	// still compile both spawns with TargetingUnspecified -- omitted, not a
	// zero-value TargetClosest standing in for "unset" (rpg-toolkit#895
	// gate-review hardening moved TargetClosest off the zero value
	// specifically so this distinction holds by construction).
	unset, err := dungeonspec.Load([]byte(placedTombYAML))
	require.NoError(t, err)
	for i := range unset.Spawns {
		assert.Equal(t, monster.TargetingUnspecified, unset.Spawns[i].Targeting,
			"spawn %q: omitted targeting must compile to TargetingUnspecified, not TargetClosest",
			unset.Spawns[i].MonsterRef)
	}
}

// TestLoad_SameBytesProduceIdenticalCompiledDungeon: Load has no source of
// randomness of its own (RandomSeed is left at its zero value -- entropy
// seeding happens later, at InitDungeon time, never inside the compiler) --
// calling it twice on the same bytes must yield byte-for-byte identical
// output, including the placed obstacles and positioned spawns from the
// place block, not just the top-level scalars.
func TestLoad_SameBytesProduceIdenticalCompiledDungeon(t *testing.T) {
	first, err := dungeonspec.Load([]byte(placedTombYAML))
	require.NoError(t, err)
	second, err := dungeonspec.Load([]byte(placedTombYAML))
	require.NoError(t, err)
	assert.Equal(t, first, second)
}

// TestLoad_PropagatesValidateErrors pins Load's Decode -> Validate ->
// compile wiring: a Load that skipped the Validate call (calling compile
// directly on whatever Decode returned) would still pass every other test
// in this file, since none of their fixtures are M1-invalid. referenceYAML
// (decode_test.go) IS M1-invalid on two counts at once (count-based
// monsters: on its entrance/gallery rooms, Task B2's validateM1Restrictions)
// -- so Load must surface that error, not a CompiledDungeon.
func TestLoad_PropagatesValidateErrors(t *testing.T) {
	_, err := dungeonspec.Load([]byte(referenceYAML))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "rolled monster placement lands in M2")
}

func TestLoad_CountBasedObstaclesCompileWithDefaultsAndPreferBorder(t *testing.T) {
	// obstacleDefaultsYAML exercises the count-based obstacle loop's
	// nil->true blocking defaults AND explicit-false/PreferBorder carry-
	// through, independent of placedTombYAML (which only has an unflagged
	// place entry, never a count-based obstacle with flags set).
	const obstacleDefaultsYAML = `
version: 1
key: obstacle-defaults-check
name: Obstacle Defaults Check
height: 8
rooms:
  - id: entrance
    archetype: entrance
    width: 6
    obstacles:
      - { ref: "dnd5e:props:pillar", count: 2, blocks_movement: false, prefer_border: true }
      - { ref: "dnd5e:props:obelisk", count: 1 }
  - id: boss-room
    archetype: boss
    width: 7
    boss: { ref: "dnd5e:monsters:skeleton-captain", at: [3, 5] }
connectors:
  - { from: entrance, to: boss-room }
`
	compiled, err := dungeonspec.Load([]byte(obstacleDefaultsYAML))
	require.NoError(t, err)
	entrance := regionByID(t, compiled.Params.Regions, "entrance")
	require.Len(t, entrance.Obstacles, 2)

	pillar := entrance.Obstacles[0]
	assert.Equal(t, "dnd5e:props:pillar", pillar.Ref)
	assert.Equal(t, 2, pillar.Count)
	assert.False(t, pillar.BlocksMovement) // explicit false carried through, not defaulted
	assert.True(t, pillar.BlocksLoS)       // unset -> nil -> true
	assert.True(t, pillar.PreferBorder)

	obelisk := entrance.Obstacles[1]
	assert.Equal(t, "dnd5e:props:obelisk", obelisk.Ref)
	assert.True(t, obelisk.BlocksMovement) // unset -> nil -> true
	assert.True(t, obelisk.BlocksLoS)      // unset -> nil -> true
	assert.False(t, obelisk.PreferBorder)  // unset, no flip-to-true default for this field
}

func TestLoad_LockedConnectorCompilesLockFields(t *testing.T) {
	// lockedConnectorYAML exercises locked-connector compilation independent
	// of placedTombYAML (which has no lock at all) -- Locked/LockDC/
	// LockAbility must all copy through from the spec's locked: block.
	const lockedConnectorYAML = `
version: 1
key: locked-connector-check
name: Locked Connector Check
height: 8
rooms:
  - {id: entrance, archetype: entrance, width: 6}
  - {id: boss-room, archetype: boss, width: 7, boss: {ref: "dnd5e:monsters:skeleton-captain", at: [3, 5]}}
connectors:
  - {from: entrance, to: boss-room, locked: {dc: 15, ability: str}}
`
	compiled, err := dungeonspec.Load([]byte(lockedConnectorYAML))
	require.NoError(t, err)
	require.Len(t, compiled.Params.Connectors, 1)
	conn := compiled.Params.Connectors[0]
	assert.True(t, conn.Locked)
	assert.Equal(t, 15, conn.LockDC)
	assert.Equal(t, "str", conn.LockAbility)
}

// TestLoad_DoorIDsComposeHyphenatedRoomIDs pins the door-id rule's
// composition against a room id that itself contains a hyphen
// ("trap-crossing") -- plain concatenation must not get confused by it.
// validM1YAML (validate_test.go): key "sunken-crypt", chain entrance ->
// gallery -> trap-crossing -> tomb.
func TestLoad_DoorIDsComposeHyphenatedRoomIDs(t *testing.T) {
	compiled, err := dungeonspec.Load([]byte(validM1YAML))
	require.NoError(t, err)
	require.Len(t, compiled.Params.Connectors, 3)
	assert.Equal(t, core.EntityID("sunken-crypt-door-trap-crossing-tomb"),
		compiled.Params.Connectors[2].DoorID)
}

// TestLoad_CryptKeyDoorIDsMatchAPIConstants pins the M2 parity requirement
// the Door-ID rule exists for: once the crypt migrates to content under
// key: crypt, the compiler must reproduce rpg-api's own hardcoded test
// constants byte-for-byte -- cryptEntranceDoorID =
// "crypt-door-entrance-corridor" and cryptBossDoorID =
// "crypt-door-corridor-boss" (internal/integration/dungeon_crypt_test.go,
// internal/handlers/dnd5e/v2/encounter/project_test.go on rpg-api
// origin/main) -- with zero special-casing in the compiler.
func TestLoad_CryptKeyDoorIDsMatchAPIConstants(t *testing.T) {
	// cryptKeyYAML is the fourth minimal entrance+boss-room-plus-one-more,
	// single-linear-chain throwaway fixture in this file (after
	// scatteredYAML, obstacleDefaultsYAML, lockedConnectorYAML) built just
	// to isolate one compile rule. If a test needs a fifth, that's the
	// signal to extract a shared minimal-dungeon builder helper instead of
	// copy-pasting another one.
	const cryptKeyYAML = `
version: 1
key: crypt
name: Crypt Key Door ID Check
height: 8
rooms:
  - {id: entrance, archetype: entrance, width: 6}
  - {id: corridor, archetype: corridor, width: 6}
  - {id: boss, archetype: boss, width: 7, boss: {ref: "dnd5e:monsters:skeleton-captain", at: [3, 5]}}
connectors:
  - {from: entrance, to: corridor}
  - {from: corridor, to: boss}
`
	compiled, err := dungeonspec.Load([]byte(cryptKeyYAML))
	require.NoError(t, err)
	require.Len(t, compiled.Params.Connectors, 2)
	assert.Equal(t, core.EntityID("crypt-door-entrance-corridor"), compiled.Params.Connectors[0].DoorID)
	assert.Equal(t, core.EntityID("crypt-door-corridor-boss"), compiled.Params.Connectors[1].DoorID)
}

// TestLoad_PlacedMonsterCellsReservedFromRolledObstacles is the regression
// test for rpg-toolkit#842's gate-blocking cross-task seam bug: a placed
// monster -- a place: monster entry, or the boss's own pinned boss.at --
// compiles to a SpawnInstruction, never a PlacedObstacleSpec, so
// InitDungeon's rolled-obstacle draw had no idea that cell was already
// spoken for and could roll a count-based obstacle directly onto it. The
// bug never showed up at compile time (compile()/compileRoom's own output
// was correct in isolation) -- it only surfaced later, and only on SOME
// seeds, as Encounter.SeedMonsters failing to place the monster into a
// cell an obstacle already occupies. Exercising the FULL pipeline (Load ->
// InitDungeon -> SeedMonsters), not just the compiler or the engine in
// isolation, is the point: the seam between two independently-correct
// pieces is exactly what broke.
//
// reservedCellRegressionYAML is single-use and deliberately NOT
// placedTombYAML/validM1YAML: this fixture's tomb room needs BOTH a
// count-based obstacles: block (to create rollable candidates) and a
// place: monster entry alongside its boss.at (to create reserved cells to
// collide with) in the SAME room -- a combination the shared fixtures
// don't have, and adding one to them would perturb OTHER tests' exact
// obstacle-position/floor-plan assertions (compile_test.go's own
// TestLoad_GeneratesDeterministicDoorIDs and workbench_test.go's floor-plan
// row pins, in particular).
//
// Count: 40 count-based obstacles crammed into the tomb room's ~84
// non-doorRow cells (12x8, minus the shared doorRow row) makes a missing
// reservation collide on a large fraction of seeds -- the gate's own
// empirical finding was roughly a third of seeds at a comparable
// room/count ratio -- so a 50-seed sweep is well past enough to catch a
// fix that only closes PART of the gap (e.g. the boss.at half but not the
// place: monster half, or vice versa).
func TestLoad_PlacedMonsterCellsReservedFromRolledObstacles(t *testing.T) {
	const reservedCellRegressionYAML = `
version: 1
key: reserved-cell-regression
name: Reserved Cell Regression Check
height: 8
rooms:
  - id: entrance
    archetype: entrance
    width: 6
  - id: tomb
    archetype: boss
    width: 12
    boss: { ref: "dnd5e:monsters:skeleton-captain", at: [7, 5] }
    obstacles:
      - { ref: "dnd5e:props:pillar", count: 40 }
    place:
      - { ref: "dnd5e:monsters:skeleton", at: [4, 2] }
connectors:
  - { from: entrance, to: tomb }
`
	compiled, err := dungeonspec.Load([]byte(reservedCellRegressionYAML))
	require.NoError(t, err)

	for seed := int64(1); seed <= 50; seed++ {
		params := compiled.Params
		params.RandomSeed = seed

		transport := encounter.NewInMemoryTransport()
		broker := encounter.NewBroker(transport)
		enc := encounter.New(context.Background(), "reserved-cell-regression-test", broker)
		require.NoError(t, enc.InitDungeon(params), "seed %d: InitDungeon", seed)
		require.NoError(t, enc.SeedMonsters(compiled.Spawns), "seed %d: SeedMonsters", seed)
	}
}
