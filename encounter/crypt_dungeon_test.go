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
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/KirkDiggler/rpg-toolkit/encounter"
	"github.com/KirkDiggler/rpg-toolkit/encounter/core"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/abilities"
	"github.com/KirkDiggler/rpg-toolkit/tools/spatial"
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

// TestCryptDungeonParams_BossConnectorUsesApprovedLockConfig pins the first
// crypt's approved door contract: its entrance stays a plain connector while
// the boss connector requires a DC 12 Dexterity check without a tool.
func TestCryptDungeonParams_BossConnectorUsesApprovedLockConfig(t *testing.T) {
	params := encounter.CryptDungeonParams(cdSeed, cdEntranceDoorID, cdBossDoorID)
	require.Len(t, params.Connectors, 2)

	entrance := params.Connectors[0]
	require.Equal(t, cdEntranceDoorID, entrance.DoorID)
	require.False(t, entrance.Locked)
	require.Zero(t, entrance.LockDC)
	require.Empty(t, entrance.LockAbility)
	require.Empty(t, entrance.LockTool)

	boss := params.Connectors[1]
	require.Equal(t, cdBossDoorID, boss.DoorID)
	require.True(t, boss.Locked)
	require.Equal(t, 12, boss.LockDC)
	require.Equal(t, string(abilities.DEX), boss.LockAbility)
	require.Empty(t, boss.LockTool)
}

// cryptDexResolver accepts only the canonical D&D Dexterity identifier. It
// proves the crypt's generated prompt reaches the existing unlock rules with
// no tool contribution.
type cryptDexResolver struct {
	ability string
}

func (r *cryptDexResolver) AbilityModifier(_ core.PlayerID, ability string) (int, bool) {
	r.ability = ability
	return 0, ability == string(abilities.DEX)
}

func (*cryptDexResolver) ToolProficiencyBonus(_ core.PlayerID, _ string) (int, bool) {
	return 0, false
}

// TestCryptDungeon_BossDoorLockPersistsAndUnlocks verifies the crypt preset
// config drives the existing generated-door state, persistence, and
// AttemptUnlock/SubmitCheck recovery contract without introducing a new verb.
func TestCryptDungeon_BossDoorLockPersistsAndUnlocks(t *testing.T) {
	transport := encounter.NewInMemoryTransport()
	broker := encounter.NewBroker(transport)
	t.Cleanup(func() {
		_ = broker.Close()
		_ = transport.Close()
	})
	resolver := &cryptDexResolver{}
	enc := encounter.New(
		context.Background(), "enc-crypt-locked-door", broker, encounter.WithCharacterResolver(resolver),
	)
	require.NoError(t, enc.InitDungeon(encounter.CryptDungeonParams(cdSeed, cdEntranceDoorID, cdBossDoorID)))

	data := enc.ToData()
	require.False(t, data.Doors[cdEntranceDoorID].Open)
	require.False(t, data.Doors[cdEntranceDoorID].Locked)
	require.True(t, data.Doors[cdBossDoorID].Locked)
	require.False(t, data.Doors[cdBossDoorID].Open)
	require.Equal(t, 12, data.Doors[cdBossDoorID].LockDC)
	require.Equal(t, string(abilities.DEX), data.Doors[cdBossDoorID].LockAbility)
	require.Empty(t, data.Doors[cdBossDoorID].LockTool)

	payload, err := json.Marshal(data)
	require.NoError(t, err)
	var loaded encounter.Data
	require.NoError(t, json.Unmarshal(payload, &loaded))
	reloaded, err := encounter.LoadFromData(
		context.Background(), &loaded, broker, encounter.WithCharacterResolver(resolver),
	)
	require.NoError(t, err)

	require.NoError(t, reloaded.AddPlayer(encounter.PlayerInput{
		PlayerID: alicePlayerID, EntityID: aliceEntityID,
		Position: reloaded.ToData().Space.Entrance, SightRange: 30,
	}))
	require.NoError(t, reloaded.OpenDoor(alicePlayerID, cdEntranceDoorID))

	issued, err := reloaded.AttemptUnlock(alicePlayerID, cdBossDoorID)
	require.NoError(t, err)
	require.Equal(t, 12, issued.DC)
	require.Equal(t, string(abilities.DEX), issued.Ability)
	require.Empty(t, issued.Tool)
	failed, err := reloaded.SubmitCheck(alicePlayerID, 11)
	require.NoError(t, err)
	require.False(t, failed.Success)
	require.True(t, reloaded.ToData().Doors[cdBossDoorID].Locked)
	require.False(t, reloaded.ToData().Doors[cdBossDoorID].Open)

	_, err = reloaded.AttemptUnlock(alicePlayerID, cdBossDoorID)
	require.NoError(t, err, "a failed lock check must remain recoverable")
	succeeded, err := reloaded.SubmitCheck(alicePlayerID, 12)
	require.NoError(t, err)
	require.True(t, succeeded.Success)
	require.Equal(t, string(abilities.DEX), resolver.ability)
	require.False(t, reloaded.ToData().Doors[cdBossDoorID].Locked)
	require.True(t, reloaded.ToData().Doors[cdBossDoorID].Open)
}

// TestCryptDungeonParams_RegionArchetypeComposition: entrance gets
// obelisk+pillars, corridor gets a sparse pillar only (never coffin/
// altar/statue — corridor must stay monster-free/easily traversable per
// the issue), and boss gets coffin/tomb+altar+the two exact statue
// variants (never obelisk, which is entrance-only per the issue's exact
// list) — the physical semantic refs are limited to the approved set,
// nothing else. Statues are pinned to the two PROMOTED exact asset
// variants (statue-reaper, statue-knight-hooded), never a generic
// "dnd5e:props:statue" — no such key exists in the shipped asset
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
// ambiguous "dnd5e:props:statue" key must never appear anywhere in
// CryptDungeonParams' output -- verified finding: no such key exists in
// the shipped asset contract. Only the two promoted exact variants are
// valid statue refs for the first crypt default. Also guards against a
// regression to this file's now-corrected "dnd5e:obstacles:..." (an
// invented, non-canonical namespace, PR #826 review second pass).
func TestCryptDungeonParams_NoGenericStatueRefExists(t *testing.T) {
	params := encounter.CryptDungeonParams(cdSeed, cdEntranceDoorID, cdBossDoorID)
	for _, r := range params.Regions {
		for _, spec := range r.Obstacles {
			require.NotEqual(t, "dnd5e:props:statue", spec.Ref,
				"the generic ambiguous statue ref must never be used -- exact variants only")
			require.NotContains(t, spec.Ref, "obstacles",
				"refs must use the canonical \"dnd5e:props:...\" namespace, not the invented \"obstacles\" one")
		}
	}
}

// TestCryptDungeonParams_NoObstacleUsesModelFilenamesOrSyntyPaths: every
// approved ref is a plain opaque "dnd5e:props:<kind>" string — no
// model filename or Synty-specific path ever appears in generic encounter
// data (rpg-toolkit#819 scope: "No model filenames/Synty paths").
func TestCryptDungeonParams_NoObstacleUsesModelFilenamesOrSyntyPaths(t *testing.T) {
	for _, ref := range []string{
		encounter.CryptObstacleRefCoffin, encounter.CryptObstacleRefAltar,
		encounter.CryptObstacleRefStatueReaper, encounter.CryptObstacleRefStatueKnightHooded,
		encounter.CryptObstacleRefObelisk, encounter.CryptObstacleRefPillar,
	} {
		require.Regexp(t, `^dnd5e:props:[a-z-]+$`, ref)
	}
}

// cryptCanonicalAssetManifestKeys pins the EXACT full canonical ref
// string every crypt obstacle constant must equal -- not merely a tail
// ID or a shape-matching regex (PR #826 review: "assert exact full
// canonical keys, not merely tail IDs"). These are the shipped asset
// manifest's own keys, verified independently: module "dnd5e", type
// "props" (NOT the invented "obstacles" this file used before this
// fix), one entry per approved crypt set piece.
var cryptCanonicalAssetManifestKeys = map[string]string{
	"CryptObstacleRefCoffin":             "dnd5e:props:coffin",
	"CryptObstacleRefAltar":              "dnd5e:props:altar",
	"CryptObstacleRefStatueReaper":       "dnd5e:props:statue-reaper",
	"CryptObstacleRefStatueKnightHooded": "dnd5e:props:statue-knight-hooded",
	"CryptObstacleRefObelisk":            "dnd5e:props:obelisk",
	"CryptObstacleRefPillar":             "dnd5e:props:pillar",
}

// TestCryptObstacleRefs_MatchCanonicalAssetManifestKeysExactly: each
// exported CryptObstacleRef* constant must equal, byte for byte, the
// asset manifest's own canonical key -- module "dnd5e", type "props"
// (verified finding, PR #826 review: the earlier "dnd5e:obstacles:..."
// namespace was invented, not the shipped contract's actual key). A
// test that only checked the SHAPE of the ref (e.g. a regex matching
// any "dnd5e:<type>:<id>" string) would pass even if this constant
// silently drifted to the wrong type segment -- this test pins the
// literal value instead.
func TestCryptObstacleRefs_MatchCanonicalAssetManifestKeysExactly(t *testing.T) {
	actual := map[string]string{
		"CryptObstacleRefCoffin":             encounter.CryptObstacleRefCoffin,
		"CryptObstacleRefAltar":              encounter.CryptObstacleRefAltar,
		"CryptObstacleRefStatueReaper":       encounter.CryptObstacleRefStatueReaper,
		"CryptObstacleRefStatueKnightHooded": encounter.CryptObstacleRefStatueKnightHooded,
		"CryptObstacleRefObelisk":            encounter.CryptObstacleRefObelisk,
		"CryptObstacleRefPillar":             encounter.CryptObstacleRefPillar,
	}
	for name, want := range cryptCanonicalAssetManifestKeys {
		require.Equal(t, want, actual[name], "%s must equal the exact canonical asset-manifest key", name)
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

// cdHeight/cdEntranceWidth/cdCorridorWidth/cdBossWidth duplicate
// crypt_dungeon.go's own unexported cryptHeight/cryptEntranceWidth/
// cryptCorridorWidth/cryptBossWidth dimensions (this file lives in
// package encounter_test and can't see those directly) — the same
// "assert the generator's construction guarantee actually holds"
// convention dungeon_test.go's dungeonRegionStarts and
// two_chamber_test.go's doorAdjacentChamber2Hex already use, not a public
// API this package exports.
const (
	cdHeight        = 8
	cdEntranceWidth = 10
	cdCorridorWidth = 5
	cdBossWidth     = 10
	cryptDoorRow    = cdHeight / 2
)

var cryptRegionWidths = []int{cdEntranceWidth, cdCorridorWidth, cdBossWidth}

func cryptRegionStarts() []int {
	starts := make([]int, len(cryptRegionWidths))
	x := 0
	for i, w := range cryptRegionWidths {
		starts[i] = x
		x += w + 1 // +1 reserves the boundary/door column after this region
	}
	return starts
}

// cryptBoundaryColumnHexes returns every hex the connector boundary column
// between region connectorIdx and connectorIdx+1 must carry a blocked wall
// segment at — every row except cryptDoorRow (that cell is the connector's
// door), mirroring generateDungeonLayout's own boundary-column construction
// (dungeon.go). Unaffected by rpg-toolkit#835's Pattern change: this column
// is emitted unconditionally, independent of either neighboring region's
// interior wall pattern.
func cryptBoundaryColumnHexes(connectorIdx int) []core.Hex {
	starts := cryptRegionStarts()
	x := starts[connectorIdx] + cryptRegionWidths[connectorIdx]
	out := make([]core.Hex, 0, cdHeight-1)
	for y := 0; y < cdHeight; y++ {
		if y == cryptDoorRow {
			continue
		}
		out = append(out, core.HexFromPosition(spatial.Position{X: float64(x), Y: float64(y)}))
	}
	return out
}

// TestCryptDungeon_EntranceAndBossPatternEmpty_NoInteriorWallsAcrossSeeds
// pins rpg-toolkit#835: the entrance and boss regions now use
// environments.PatternEmpty instead of PatternRandom (set pieces carry the
// cover role per #819; PatternRandom's scattered wall cells rendered as
// rubble client-side, rpg-dnd5e-web#562), so building the crypt across a
// spread of seeds must roll ZERO interior wall cells in either region.
// PatternEmpty never generates interior walls by construction (see
// environments.EmptyPattern), but this proves that holds for the crypt's
// actual dimensions/archetypes/obstacle placement, not just in isolation —
// while the connector boundary columns between regions (always emitted
// regardless of either neighbor's interior pattern) must still be present,
// completely unaffected by this change.
func TestCryptDungeon_EntranceAndBossPatternEmpty_NoInteriorWallsAcrossSeeds(t *testing.T) {
	for seed := int64(1); seed <= 50; seed++ {
		enc := cdNewEncounter(t)
		require.NoError(t, enc.InitDungeon(encounter.CryptDungeonParams(seed, cdEntranceDoorID, cdBossDoorID)),
			"seed %d", seed)

		data := enc.ToData()
		entranceHexes := regionHexSet(data.Space, "entrance")
		bossHexes := regionHexSet(data.Space, "boss")
		require.NotEmpty(t, entranceHexes, "seed %d: entrance region must have hexes", seed)
		require.NotEmpty(t, bossHexes, "seed %d: boss region must have hexes", seed)

		for _, w := range data.Space.Walls {
			// rpg-toolkit#834: a boundary-edge segment (Start != End) is a
			// non-blocking render-contract entry for a REAL floor hex on
			// the space's outer perimeter, not a generated interior
			// wall -- the entrance's west edge and the boss region's own
			// outer edge legitimately carry one of these even under
			// PatternEmpty. This test's invariant is about GENERATED
			// (blocking) interior walls only; only degenerate
			// (Start == End) entries are ever candidates (same fix as
			// boss_primary_axis_test.go's analogous filter).
			if w.Start != w.End {
				continue
			}
			h := core.HexFromCube(w.Start)
			require.False(t, entranceHexes[h],
				"seed %d: entrance region must roll zero interior walls with PatternEmpty; found one at %v", seed, h)
			require.False(t, bossHexes[h],
				"seed %d: boss region must roll zero interior walls with PatternEmpty; found one at %v", seed, h)
		}

		wallSet := make(map[core.Hex]bool, len(data.Space.Walls))
		for _, w := range data.Space.Walls {
			wallSet[core.HexFromCube(w.Start)] = true
		}
		for connectorIdx := 0; connectorIdx < 2; connectorIdx++ {
			for _, h := range cryptBoundaryColumnHexes(connectorIdx) {
				require.True(t, wallSet[h],
					"seed %d: connector %d boundary column must still carry a wall at %v", seed, connectorIdx, h)
			}
		}
	}
}
