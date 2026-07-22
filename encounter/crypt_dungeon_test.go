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
// the issue), and boss gets coffin/tomb+altar+statues (never obelisk,
// which is entrance-only per the issue's exact list) — the physical
// semantic refs are limited to the approved set, nothing else.
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
	require.NotContains(t, entranceRefs, encounter.CryptObstacleRefStatue)

	corridorRefs := refsOf(byArchetype[encounter.ArchetypeCorridor].Obstacles)
	require.Contains(t, corridorRefs, encounter.CryptObstacleRefPillar)
	require.NotContains(t, corridorRefs, encounter.CryptObstacleRefObelisk)
	require.NotContains(t, corridorRefs, encounter.CryptObstacleRefCoffin)
	require.NotContains(t, corridorRefs, encounter.CryptObstacleRefAltar)
	require.NotContains(t, corridorRefs, encounter.CryptObstacleRefStatue)
	corridorTotal := 0
	for _, s := range byArchetype[encounter.ArchetypeCorridor].Obstacles {
		corridorTotal += s.Count
	}
	require.LessOrEqual(t, corridorTotal, 1, "corridor set pieces must stay sparse")

	bossRefs := refsOf(byArchetype[encounter.ArchetypeBoss].Obstacles)
	require.Contains(t, bossRefs, encounter.CryptObstacleRefCoffin)
	require.Contains(t, bossRefs, encounter.CryptObstacleRefAltar)
	require.Contains(t, bossRefs, encounter.CryptObstacleRefStatue)
	require.NotContains(t, bossRefs, encounter.CryptObstacleRefObelisk)
	require.Equal(t, 2, bossRefs[encounter.CryptObstacleRefStatue].Count, "statues flank in a pair")
}

// TestCryptDungeonParams_NoObstacleUsesModelFilenamesOrSyntyPaths: every
// approved ref is a plain opaque "dnd5e:obstacles:<kind>" string — no
// model filename or Synty-specific path ever appears in generic encounter
// data (rpg-toolkit#819 scope: "No model filenames/Synty paths").
func TestCryptDungeonParams_NoObstacleUsesModelFilenamesOrSyntyPaths(t *testing.T) {
	for _, ref := range []string{
		encounter.CryptObstacleRefCoffin, encounter.CryptObstacleRefAltar,
		encounter.CryptObstacleRefStatue, encounter.CryptObstacleRefObelisk, encounter.CryptObstacleRefPillar,
	} {
		require.Regexp(t, `^dnd5e:obstacles:[a-z-]+$`, ref)
	}
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
		encounter.CryptObstacleRefCoffin:  true,
		encounter.CryptObstacleRefAltar:   true,
		encounter.CryptObstacleRefStatue:  true,
		encounter.CryptObstacleRefObelisk: true,
		encounter.CryptObstacleRefPillar:  true,
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
