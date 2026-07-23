package encounter_test

// crypt_dungeon_test.go is the TDD gate for rpg-toolkit#819's CRYPT-
// SPECIFIC half: encounter.CryptDungeonParams composes the generic,
// content-agnostic InitDungeon + ObstacleSpec mechanism (proven out in
// obstacle_placement_test.go) with the approved physical set-piece
// vocabulary (coffin/tomb, altar, statues, obelisk, pillars) placed by
// region archetype for the first crypt dungeon (entrance -> corridor ->
// boss, the same shape dungeon_test.go's threeRegionDungeonParams already
// established for #814/#817).

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/KirkDiggler/rpg-toolkit/encounter"
	"github.com/KirkDiggler/rpg-toolkit/encounter/core"
)

const (
	cdEntranceDoorID = core.EntityID("crypt-door-entrance-corridor")
	cdBossDoorID     = core.EntityID("crypt-door-corridor-boss")
	cdSeed           = int64(191919)
)

func cdNewEncounter(t *testing.T) *encounter.Encounter {
	t.Helper()
	transport := encounter.NewInMemoryTransport()
	broker := encounter.NewBroker(transport)
	t.Cleanup(func() {
		_ = broker.Close()
		_ = transport.Close()
	})
	return encounter.New(context.Background(), "enc-crypt-dungeon-test", broker)
}

// TestCryptDungeonParams_RegionArchetypeComposition: entrance gets
// obelisk+pillars, corridor gets a sparse pillar only (never coffin/
// altar/statue — corridor must stay monster-free/easily traversable per
// the issue), and boss gets coffin/tomb+altar+the two exact statue
// variants (never obelisk, which is entrance-only per the issue's exact
// list) — the physical semantic refs are limited to the approved set,
// nothing else. Statues are pinned to the two PROMOTED exact asset
// variants (statue-reaper, statue-knight-hooded), never the generic
// "dnd5e:obstacles:statue" — no such key exists in the shipped asset
// contract (verified finding).
func TestCryptDungeonParams_RegionArchetypeComposition(t *testing.T) {
	params := encounter.CryptDungeonParams(cdSeed, cdEntranceDoorID, cdBossDoorID)
	require.Len(t, params.Regions, 3)

	byArchetype := make(map[encounter.RegionArchetype]encounter.DungeonRegionParams, 3)
	for _, r := range params.Regions {
		byArchetype[r.Archetype] = r
	}

	refsOf := func(specs []encounter.ObstacleSpec) map[string]encounter.ObstacleSpec {
		out := make(map[string]encounter.ObstacleSpec, len(specs))
		for _, s := range specs {
			out[s.Ref] = s
		}
		return out
	}

	entranceRefs := refsOf(byArchetype[encounter.ArchetypeEntrance].Obstacles)
	require.Contains(t, entranceRefs, encounter.CryptObstacleRefObelisk)
	require.Contains(t, entranceRefs, encounter.CryptObstacleRefPillar)
	require.NotContains(t, entranceRefs, encounter.CryptObstacleRefCoffin)
	require.NotContains(t, entranceRefs, encounter.CryptObstacleRefAltar)
	require.NotContains(t, entranceRefs, encounter.CryptObstacleRefStatueReaper)
	require.NotContains(t, entranceRefs, encounter.CryptObstacleRefStatueKnightHooded)

	corridorRefs := refsOf(byArchetype[encounter.ArchetypeCorridor].Obstacles)
	require.Contains(t, corridorRefs, encounter.CryptObstacleRefPillar)
	require.NotContains(t, corridorRefs, encounter.CryptObstacleRefObelisk)
	require.NotContains(t, corridorRefs, encounter.CryptObstacleRefCoffin)
	require.NotContains(t, corridorRefs, encounter.CryptObstacleRefAltar)
	require.NotContains(t, corridorRefs, encounter.CryptObstacleRefStatueReaper)
	require.NotContains(t, corridorRefs, encounter.CryptObstacleRefStatueKnightHooded)
	corridorTotal := 0
	for _, s := range byArchetype[encounter.ArchetypeCorridor].Obstacles {
		corridorTotal += s.Count
	}
	require.LessOrEqual(t, corridorTotal, 1, "corridor set pieces must stay sparse")

	bossRefs := refsOf(byArchetype[encounter.ArchetypeBoss].Obstacles)
	require.Contains(t, bossRefs, encounter.CryptObstacleRefCoffin)
	require.Contains(t, bossRefs, encounter.CryptObstacleRefAltar)
	require.Contains(t, bossRefs, encounter.CryptObstacleRefStatueReaper)
	require.Contains(t, bossRefs, encounter.CryptObstacleRefStatueKnightHooded)
	require.NotContains(t, bossRefs, encounter.CryptObstacleRefObelisk)
	require.Equal(t, 1, bossRefs[encounter.CryptObstacleRefStatueReaper].Count,
		"exactly one reaper statue -- the promoted default pairing, not a random-variant policy")
	require.Equal(t, 1, bossRefs[encounter.CryptObstacleRefStatueKnightHooded].Count,
		"exactly one hooded-knight statue -- the promoted default pairing, not a random-variant policy")
}

// TestCryptDungeonParams_NoGenericStatueRefExists: the generic,
// ambiguous "dnd5e:obstacles:statue" key must never appear anywhere in
// CryptDungeonParams' output -- verified finding: no such key exists in
// the shipped asset contract. Only the two promoted exact variants are
// valid statue refs for the first crypt default.
func TestCryptDungeonParams_NoGenericStatueRefExists(t *testing.T) {
	params := encounter.CryptDungeonParams(cdSeed, cdEntranceDoorID, cdBossDoorID)
	for _, r := range params.Regions {
		for _, spec := range r.Obstacles {
			require.NotEqual(t, "dnd5e:obstacles:statue", spec.Ref,
				"the generic ambiguous statue ref must never be used -- exact variants only")
		}
	}
}

// TestCryptDungeonParams_NoObstacleUsesModelFilenamesOrSyntyPaths: every
// approved ref is a plain opaque "dnd5e:obstacles:<kind>" string — no
// model filename or Synty-specific path ever appears in generic encounter
// data (rpg-toolkit#819 scope: "No model filenames/Synty paths").
func TestCryptDungeonParams_NoObstacleUsesModelFilenamesOrSyntyPaths(t *testing.T) {
	for _, ref := range []string{
		encounter.CryptObstacleRefCoffin, encounter.CryptObstacleRefAltar,
		encounter.CryptObstacleRefStatueReaper, encounter.CryptObstacleRefStatueKnightHooded,
		encounter.CryptObstacleRefObelisk, encounter.CryptObstacleRefPillar,
	} {
		require.Regexp(t, `^dnd5e:obstacles:[a-z-]+$`, ref)
	}
}

// cryptBlockingContractTable is the VERIFIED canonical shipped-asset-
// contract blocking table (independent finding, not this file's own
// design guess): coffin/tomb movement=true LoS=false; altar movement=
// true LoS=true (measured 2.057m, role "obstacle"); statues movement=
// true LoS=true; obelisk/pillar movement=true LoS=true.
var cryptBlockingContractTable = []struct {
	ref            string
	blocksMovement bool
	blocksLoS      bool
}{
	{encounter.CryptObstacleRefCoffin, true, false},
	{encounter.CryptObstacleRefAltar, true, true},
	{encounter.CryptObstacleRefStatueReaper, true, true},
	{encounter.CryptObstacleRefStatueKnightHooded, true, true},
	{encounter.CryptObstacleRefObelisk, true, true},
	{encounter.CryptObstacleRefPillar, true, true},
}

// TestCryptDungeonParams_BlockingFlagsMatchVerifiedAssetContract: every
// ObstacleSpec CryptDungeonParams emits for a given ref must carry
// EXACTLY the verified asset-contract BlocksMovement/BlocksLoS pair —
// most pointedly, altar must now block LoS (it did not before this
// fix: the shipped asset contract measures it at 2.057m with role
// "obstacle", not a low walk-around table).
func TestCryptDungeonParams_BlockingFlagsMatchVerifiedAssetContract(t *testing.T) {
	params := encounter.CryptDungeonParams(cdSeed, cdEntranceDoorID, cdBossDoorID)
	seen := make(map[string]encounter.ObstacleSpec)
	for _, r := range params.Regions {
		for _, spec := range r.Obstacles {
			seen[spec.Ref] = spec
		}
	}
	for _, tc := range cryptBlockingContractTable {
		spec, ok := seen[tc.ref]
		require.True(t, ok, "ref %q must appear somewhere in CryptDungeonParams", tc.ref)
		require.Equal(t, tc.blocksMovement, spec.BlocksMovement, "ref %q BlocksMovement mismatch", tc.ref)
		require.Equal(t, tc.blocksLoS, spec.BlocksLoS, "ref %q BlocksLoS mismatch", tc.ref)
	}
}

// TestCryptDungeon_BossStatues_StableExactRefsAndIDs: building a real
// crypt dungeon must place exactly one statue-reaper and one statue-
// knight-hooded instance in the boss region, each with a stable,
// non-empty, unique ID that reproduces identically across independent
// builds with the same seed -- pinning both the exact refs/counts and
// the ID stability #819's done bar requires for any placed obstacle.
func TestCryptDungeon_BossStatues_StableExactRefsAndIDs(t *testing.T) {
	build := func() (reaperID, hoodedID string) {
		enc := cdNewEncounter(t)
		require.NoError(t, enc.InitDungeon(encounter.CryptDungeonParams(cdSeed, cdEntranceDoorID, cdBossDoorID)))
		for _, o := range enc.ToData().Space.Obstacles {
			switch o.Ref {
			case encounter.CryptObstacleRefStatueReaper:
				require.Empty(t, reaperID, "exactly one reaper statue must be placed")
				reaperID = string(o.ID)
			case encounter.CryptObstacleRefStatueKnightHooded:
				require.Empty(t, hoodedID, "exactly one hooded-knight statue must be placed")
				hoodedID = string(o.ID)
			}
		}
		require.NotEmpty(t, reaperID, "a reaper statue must be placed")
		require.NotEmpty(t, hoodedID, "a hooded-knight statue must be placed")
		require.NotEqual(t, reaperID, hoodedID, "the two statue instances must have distinct IDs")
		return reaperID, hoodedID
	}
	r1, h1 := build()
	r2, h2 := build()
	require.Equal(t, r1, r2, "the reaper statue's ID must be stable across independent builds with the same seed")
	require.Equal(t, h1, h2, "the hooded-knight statue's ID must be stable across independent builds with the same seed")
}

// TestCryptDungeon_GeneratesPlaceableDungeon_WithSetPieces: building a
// real Encounter from CryptDungeonParams succeeds and produces a non-empty
// Obstacles list drawing only from the approved refs.
func TestCryptDungeon_GeneratesPlaceableDungeon_WithSetPieces(t *testing.T) {
	enc := cdNewEncounter(t)
	require.NoError(t, enc.InitDungeon(encounter.CryptDungeonParams(cdSeed, cdEntranceDoorID, cdBossDoorID)))

	data := enc.ToData()
	require.NotEmpty(t, data.Space.Obstacles)

	approved := map[string]bool{
		encounter.CryptObstacleRefCoffin:             true,
		encounter.CryptObstacleRefAltar:              true,
		encounter.CryptObstacleRefStatueReaper:       true,
		encounter.CryptObstacleRefStatueKnightHooded: true,
		encounter.CryptObstacleRefObelisk:            true,
		encounter.CryptObstacleRefPillar:             true,
	}
	for _, o := range data.Space.Obstacles {
		require.True(t, approved[o.Ref], "obstacle ref %q is outside the approved crypt vocabulary", o.Ref)
	}
}

// TestCryptDungeon_EntranceToBossConnectivity_SurvivesSetPieces: opening
// both connector doors on the real crypt template (with its real set
// pieces placed) must still connect the entrance to the boss region —
// the crypt-specific end-to-end proof mirroring
// TestInitDungeon_ObstaclePlacement_PreservesEntranceToBossConnectivity's
// generic one.
func TestCryptDungeon_EntranceToBossConnectivity_SurvivesSetPieces(t *testing.T) {
	enc := cdNewEncounter(t)
	require.NoError(t, enc.InitDungeon(encounter.CryptDungeonParams(cdSeed, cdEntranceDoorID, cdBossDoorID)))

	data := enc.ToData()
	entrance := data.Space.Entrance
	require.NoError(t, enc.AddPlayer(encounter.PlayerInput{
		PlayerID: alicePlayerID, EntityID: aliceEntityID, Position: entrance, SightRange: 30,
	}))
	require.NoError(t, enc.OpenDoor(alicePlayerID, cdEntranceDoorID))
	require.NoError(t, enc.OpenDoor(alicePlayerID, cdBossDoorID))

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
	require.True(t, reachedBoss, "the crypt template's real set pieces must never sever entrance->boss connectivity")
}

// TestCryptDungeon_DeterministicSameSeed: same seed reproduces the
// identical crypt layout AND obstacle placement across independent
// Encounters — the crypt-specific instance of #819's determinism
// requirement.
func TestCryptDungeon_DeterministicSameSeed(t *testing.T) {
	build := func() *encounter.Data {
		enc := cdNewEncounter(t)
		require.NoError(t, enc.InitDungeon(encounter.CryptDungeonParams(cdSeed, cdEntranceDoorID, cdBossDoorID)))
		return enc.ToData()
	}
	a := build()
	b := build()
	require.Equal(t, a.Space.Obstacles, b.Space.Obstacles)
	require.Equal(t, a.Space.Walls, b.Space.Walls)
}
