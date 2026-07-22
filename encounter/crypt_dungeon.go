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
const (
	// CryptObstacleRefCoffin identifies a stone coffin/sarcophagus set
	// piece — boss-chamber only.
	CryptObstacleRefCoffin = "dnd5e:obstacles:coffin"
	// CryptObstacleRefAltar identifies a ritual altar set piece —
	// boss-chamber only.
	CryptObstacleRefAltar = "dnd5e:obstacles:altar"
	// CryptObstacleRefStatue identifies a standing statue set piece —
	// boss-chamber only (flanks the boss, per the issue).
	CryptObstacleRefStatue = "dnd5e:obstacles:statue"
	// CryptObstacleRefObelisk identifies a standing obelisk set piece —
	// entrance-chamber only.
	CryptObstacleRefObelisk = "dnd5e:obstacles:obelisk"
	// CryptObstacleRefPillar identifies a structural pillar set piece —
	// shared by the entrance chamber and (sparsely) the corridor.
	CryptObstacleRefPillar = "dnd5e:obstacles:pillar"
)

// Crypt set-piece blocking properties (rpg-toolkit#819 design decision,
// documented here since #818's ObstacleData carries no partial-cover
// math — a piece either blocks LoS or it doesn't): pillars/obelisks/
// statues are tall, solid, and block both movement and line of sight;
// a coffin/altar is low enough to walk around but not through — blocks
// movement, but a combatant can see over it.
const (
	cryptBlocksMovementTall = true
	cryptBlocksLoSTall      = true
	cryptBlocksMovementLow  = true
	cryptBlocksLoSLow       = false
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
		{Ref: CryptObstacleRefObelisk, Count: 1, BlocksMovement: cryptBlocksMovementTall, BlocksLoS: cryptBlocksLoSTall},
		{Ref: CryptObstacleRefPillar, Count: 2, BlocksMovement: cryptBlocksMovementTall, BlocksLoS: cryptBlocksLoSTall},
	}
}

// cryptCorridorObstacles: a single sparse pillar — the corridor must
// stay monster-free and easily traversable (#819 scope), so this is
// deliberately the lightest set-piece load of the three regions.
func cryptCorridorObstacles() []ObstacleSpec {
	return []ObstacleSpec{
		{Ref: CryptObstacleRefPillar, Count: 1, BlocksMovement: cryptBlocksMovementTall, BlocksLoS: cryptBlocksLoSTall},
	}
}

// cryptBossObstacles: a coffin/tomb, an altar, and a flanking pair of
// statues — the boss chamber's archetype-appropriate set pieces (#819
// scope: "boss chamber with an altar and flanking statues that don't
// block the path to it").
func cryptBossObstacles() []ObstacleSpec {
	return []ObstacleSpec{
		{Ref: CryptObstacleRefCoffin, Count: 1, BlocksMovement: cryptBlocksMovementLow, BlocksLoS: cryptBlocksLoSLow},
		{Ref: CryptObstacleRefAltar, Count: 1, BlocksMovement: cryptBlocksMovementLow, BlocksLoS: cryptBlocksLoSLow},
		{Ref: CryptObstacleRefStatue, Count: 2, BlocksMovement: cryptBlocksMovementTall, BlocksLoS: cryptBlocksLoSTall},
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
