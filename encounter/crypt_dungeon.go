package encounter

// crypt_dungeon.go composes the generic, content-agnostic InitDungeon +
// ObstacleSpec mechanism (dungeon.go, rpg-toolkit#814/#819) with the
// approved physical set-piece vocabulary for the first crypt dungeon —
// rpg-toolkit#819: "encounter: crypt generator places path-safe physical
// set pieces by region archetype (coffin/altar/statues/obelisk/pillars)."
//
// Layer ownership (see .claude/progress.json's design_decision for the
// full inspection this rested on): this file is the crypt-specific
// CONTENT layer — the only file in this module that names "crypt",
// "coffin", "altar", "statue", "obelisk", or "pillar". dungeon.go's
// generic geometry code (InitDungeon, generateDungeonLayout,
// placeRegionObstacles) never references any of it, mirroring the
// file's existing Theme/Archetype-are-opaque discipline. It lives in
// THIS module rather than rulebooks/dnd5e because:
//   - ADR-0034 already classifies the `encounter` module as the (migration-
//     pending) dnd5e rulebook loop, and encounter/go.mod already depends
//     on rulebooks/dnd5e by pseudo-version — the reverse dependency (which
//     does not exist today) would need publishing + tagging a whole new
//     encounter release purely to host 5 string constants and a params
//     builder, for a rulebook that has no other reason yet to depend on
//     this module.
//   - The repo's own existing fixtures (dungeon_test.go's dungeonThemeCrypt
//     / threeRegionDungeonParams) already treat "the first crypt
//     template"'s concrete shape as encounter-owned content — this file
//     promotes that same shape from a test-only fixture to a real,
//     production, host-callable constructor (CryptDungeonParams).
//   - Content refs (ObstacleData.Ref, mirroring MonsterData.MonsterRef)
//     are, by this module's own established convention, plain opaque
//     literal strings (e.g. "dnd5e:monsters:goblin" in combat_test.go),
//     not typed refs.* singletons — so no new cross-module refs package
//     dependency is needed either.

import (
	"github.com/KirkDiggler/rpg-toolkit/encounter/core"
	"github.com/KirkDiggler/rpg-toolkit/tools/environments"
)

// Crypt obstacle refs: the approved physical set-piece vocabulary for the
// first crypt dungeon (rpg-toolkit#819) — coffin/tomb, altar, statues,
// obelisk, pillars. Plain opaque content identifiers (this module's
// established Ref convention — see MonsterData.MonsterRef); the toolkit
// never interprets them. Deliberately NOT model filenames or Synty-
// specific paths (#819 scope) — a client resolves theme-appropriate
// meshes off these refs downstream.
//
// Module "dnd5e", type "props" (PR #826 review, second pass): these are
// the EXACT keys the shipped asset manifest itself uses — an earlier
// revision of this file invented a "dnd5e:obstacles:..." namespace that
// does not exist in the asset contract; "dnd5e:props:..." is what's
// actually there. The full canonical string (module:type:id) is what
// this toolkit persists in ObstacleData.Ref -- a future #692 API
// projection may reuse whatever existing splitRef-style helper already
// decomposes a Ref into module/type/id for the wire, and
// rpg-dnd5e-web#577 (client rendering) only needs the tail id (e.g.
// "coffin") to resolve a mesh, but the value THIS package persists must
// match the manifest exactly, not a truncated or reconstructed form.
//
// Statue refs are the two EXACT promoted asset variants (verified
// finding, PR #826 review): no generic "dnd5e:props:statue" key
// exists in the shipped asset contract. Two exact variants are promoted
// — statue-reaper and statue-knight-hooded — both role "obstacle" with
// BlocksLoS=true, intended for visual pairing. This file's default boss
// composition uses exactly one of each (a fixed pairing, not a random-
// variant policy); see cryptBossObstacles.
const (
	// CryptObstacleRefCoffin identifies a stone coffin/sarcophagus set
	// piece — boss-chamber only.
	CryptObstacleRefCoffin = "dnd5e:props:coffin"
	// CryptObstacleRefAltar identifies a ritual altar set piece —
	// boss-chamber only.
	CryptObstacleRefAltar = "dnd5e:props:altar"
	// CryptObstacleRefStatueReaper identifies the "reaper" statue
	// variant — one of the two promoted exact statue assets,
	// boss-chamber only.
	CryptObstacleRefStatueReaper = "dnd5e:props:statue-reaper"
	// CryptObstacleRefStatueKnightHooded identifies the "hooded knight"
	// statue variant — the other promoted exact statue asset,
	// boss-chamber only.
	CryptObstacleRefStatueKnightHooded = "dnd5e:props:statue-knight-hooded"
	// CryptObstacleRefObelisk identifies a standing obelisk set piece —
	// entrance-chamber only.
	CryptObstacleRefObelisk = "dnd5e:props:obelisk"
	// CryptObstacleRefPillar identifies a structural pillar set piece —
	// shared by the entrance chamber and (sparsely) the corridor.
	CryptObstacleRefPillar = "dnd5e:props:pillar"
)

// Crypt set-piece blocking properties: the VERIFIED canonical shipped-
// asset-contract table (independent finding, PR #826 review — supersedes
// this file's earlier "low vs tall" design guess, which had altar wrong):
//
//	ref                    BlocksMovement  BlocksLoS
//	coffin/tomb            true            false  (walk around, see over)
//	altar                  true            true   (measured 2.057m, role "obstacle")
//	statue-reaper          true            true
//	statue-knight-hooded   true            true
//	obelisk                true            true
//	pillar                 true            true
//
// Every approved piece blocks movement; only the coffin/tomb does not
// also block line of sight.
const (
	cryptBlocksMovement  = true
	cryptBlocksLoS       = true
	cryptCoffinBlocksLoS = false
)

// The first crypt dungeon's fixed shape: entrance -> corridor -> boss,
// the same dimensions dungeon_test.go's threeRegionDungeonParams already
// established for #814/#817's done bars. Unexported — a caller only
// needs CryptDungeonParams' result, not these internals.
const (
	cryptHeight        = 8
	cryptEntranceWidth = 10
	cryptCorridorWidth = 5
	cryptBossWidth     = 10
	cryptTheme         = "crypt"

	cryptRegionIDEntrance = "entrance"
	cryptRegionIDCorridor = "corridor"
	cryptRegionIDBoss     = "boss"
)

// cryptEntranceObstacles: an obelisk plus flanking pillars — the
// entrance chamber's archetype-appropriate set pieces (#819 scope).
func cryptEntranceObstacles() []ObstacleSpec {
	return []ObstacleSpec{
		{Ref: CryptObstacleRefObelisk, Count: 1, BlocksMovement: cryptBlocksMovement, BlocksLoS: cryptBlocksLoS},
		{Ref: CryptObstacleRefPillar, Count: 2, BlocksMovement: cryptBlocksMovement, BlocksLoS: cryptBlocksLoS},
	}
}

// cryptCorridorObstacles: a single sparse pillar — the corridor must
// stay monster-free and easily traversable (#819 scope), so this is
// deliberately the lightest set-piece load of the three regions.
func cryptCorridorObstacles() []ObstacleSpec {
	return []ObstacleSpec{
		{Ref: CryptObstacleRefPillar, Count: 1, BlocksMovement: cryptBlocksMovement, BlocksLoS: cryptBlocksLoS},
	}
}

// cryptBossObstacles: a coffin/tomb, an altar, and one of each of the
// two promoted exact statue variants (a fixed reaper+hooded-knight
// pairing, not a random-variant policy) — the boss chamber's archetype-
// appropriate set pieces (#819 scope: "boss chamber with an altar and
// flanking statues that don't block the path to it"). Keeping coffin as
// coffin only — no tomb/sarcophagus variant is added here; see the PR
// body for why (#818 v1's single-anchor-per-instance limitation and
// rpg-dnd5e-web#577's rendering/composition ownership).
func cryptBossObstacles() []ObstacleSpec {
	return []ObstacleSpec{
		{Ref: CryptObstacleRefCoffin, Count: 1, BlocksMovement: cryptBlocksMovement, BlocksLoS: cryptCoffinBlocksLoS},
		{Ref: CryptObstacleRefAltar, Count: 1, BlocksMovement: cryptBlocksMovement, BlocksLoS: cryptBlocksLoS},
		{Ref: CryptObstacleRefStatueReaper, Count: 1, BlocksMovement: cryptBlocksMovement, BlocksLoS: cryptBlocksLoS},
		{
			Ref: CryptObstacleRefStatueKnightHooded, Count: 1,
			BlocksMovement: cryptBlocksMovement, BlocksLoS: cryptBlocksLoS,
		},
	}
}

// CryptDungeonParams builds the first crypt dungeon's DungeonParams
// (rpg-toolkit#819): the entrance -> corridor -> boss chain #814/#817
// already proved out (dungeon_test.go's threeRegionDungeonParams), with
// archetype-appropriate physical set pieces attached to each region via
// the generic ObstacleSpec mechanism. seed reproduces the WHOLE
// dungeon — geometry and set-piece placement alike (see
// generateDungeonLayout's doc); entranceDoorID/bossDoorID are the caller-
// assigned entity IDs for the two connector doors (mirrors every other
// InitDungeon caller's DungeonConnectorParams.DoorID).
func CryptDungeonParams(seed int64, entranceDoorID, bossDoorID core.EntityID) DungeonParams {
	return DungeonParams{
		Height:     cryptHeight,
		RandomSeed: seed,
		Theme:      cryptTheme,
		Regions: []DungeonRegionParams{
			{
				ID: cryptRegionIDEntrance, Archetype: ArchetypeEntrance,
				Width: cryptEntranceWidth, Pattern: environments.PatternRandom, Obstacles: cryptEntranceObstacles(),
			},
			{
				ID: cryptRegionIDCorridor, Archetype: ArchetypeCorridor,
				// PatternEmpty: the corridor must stay monster-free and
				// easily traversable (#819 scope) — no interior walls
				// competing with its one sparse pillar for floor space.
				Width: cryptCorridorWidth, Pattern: environments.PatternEmpty, Obstacles: cryptCorridorObstacles(),
			},
			{
				ID: cryptRegionIDBoss, Archetype: ArchetypeBoss,
				Width: cryptBossWidth, Pattern: environments.PatternRandom, Obstacles: cryptBossObstacles(),
			},
		},
		Connectors: []DungeonConnectorParams{
			{DoorID: entranceDoorID},
			{DoorID: bossDoorID},
		},
	}
}
