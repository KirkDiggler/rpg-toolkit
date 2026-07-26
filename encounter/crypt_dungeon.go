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
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/abilities"
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

// Crypt dressing + light-anchor refs (rpg-toolkit#839, "the depth pass"):
// the crypt renders correct-but-flat with zero light sources; these feed
// the existing light pipeline (client mood-light derivation lights props
// by ref, capped at 8 — rpg-dnd5e-web#585) and carry decay/dressing that
// the art target wants instead of broken wall geometry (rpg-dnd5e-
// web#469). Already promoted + client-registered (rpg-game-assets#22/#23,
// rpg-dnd5e-web#567) — same "dnd5e:props:..." namespace and opaque-ref
// convention as the #819 vocabulary above.
const (
	// CryptObstacleRefBrazier identifies a standing brazier — a warm
	// light anchor, shared by the entrance and boss chambers.
	CryptObstacleRefBrazier = "dnd5e:props:brazier"
	// CryptObstacleRefBonePile identifies a scattered bone-pile floor
	// dressing piece — entrance-chamber only.
	CryptObstacleRefBonePile = "dnd5e:props:bone-pile"
	// CryptObstacleRefTorchOrnate identifies a wall-mount ornate torch —
	// the corridor's single light anchor (must stay easily traversable).
	CryptObstacleRefTorchOrnate = "dnd5e:props:torch-ornate"
	// CryptObstacleRefCandles identifies a paired-candle floor dressing
	// piece flanking the boss chamber's coffin.
	CryptObstacleRefCandles = "dnd5e:props:candles"
	// CryptObstacleRefChain identifies a hanging-chain dressing piece —
	// boss-chamber only.
	CryptObstacleRefChain = "dnd5e:props:chain"
	// CryptObstacleRefSkeletonRemains identifies a skeletal-remains floor
	// dressing piece — boss-chamber only.
	CryptObstacleRefSkeletonRemains = "dnd5e:props:skeleton-remains"
)

// Wave-1 promotion refs (rpg-toolkit#854, part of rpg-project#132's
// look-and-feel wave): 9 of a 16-ref promotion batch — asset-w is
// promoting matching meshes for these exact names in parallel (the name
// IS the contract; do not rename without coordinating). Vocabulary only —
// not yet wired into cryptEntranceObstacles/cryptCorridorObstacles/
// cryptBossObstacles' fixed room composition, which room gets which of
// these is a separate, later authoring/composition decision.
//
// Resolved, no renames: asset-w reconciled the shipped manifest TO this
// exact 16-ref contract (rpg-game-assets#36) rather than the other way
// around — every key here matches the promoted asset names verbatim.
// #36 ships real meshes for 15 of the 16; CryptObstacleRefRibcage below
// is the one exception (still blocked on an asset defect, see its own
// doc). See the blocking-properties table below for each ref's promotion
// status relative to the pre-#854 verified shipped-asset contract.
const (
	// CryptObstacleRefRug identifies a floor rug — walkable-past dressing.
	CryptObstacleRefRug = "dnd5e:props:rug"
	// CryptObstacleRefWallBanner identifies a hanging wall banner —
	// walkable-past dressing.
	CryptObstacleRefWallBanner = "dnd5e:props:wall-banner"
	// CryptObstacleRefCandleStand identifies a small candle stand —
	// walkable-past dressing (distinct from CryptObstacleRefStoneLantern's
	// physical stone pedestal). rpg-game-assets#36 ships the base mesh;
	// its flame companion is blocked by rpg-game-assets#35 — candle
	// clusters (CryptObstacleRefCandles) carry the flames this wave.
	CryptObstacleRefCandleStand = "dnd5e:props:candle-stand"
	// CryptObstacleRefLantern identifies a hanging/table lantern —
	// walkable-past dressing.
	CryptObstacleRefLantern = "dnd5e:props:lantern"
	// CryptObstacleRefStoneLantern identifies a stone pedestal lantern — a
	// physical light anchor, same flag shape as CryptObstacleRefBrazier/
	// CryptObstacleRefTorchOrnate (blocks movement, see over/around it).
	CryptObstacleRefStoneLantern = "dnd5e:props:stone-lantern"
	// CryptObstacleRefTortureTable identifies a torture table set piece —
	// a physical obstruction, same flag shape as CryptObstacleRefCoffin
	// (blocks movement, see over/around it).
	CryptObstacleRefTortureTable = "dnd5e:props:torture-table"
	// CryptObstacleRefRibcage identifies a ribcage floor dressing piece —
	// walkable-past dressing. No shipped mesh yet as of rpg-game-assets#36
	// — every pack candidate is scale-broken, blocked by
	// rpg-game-assets#21. The ref/vocabulary entry stays regardless; a
	// caller placing it will get correct blocking behavior once a mesh
	// ships, this is purely an asset-availability gap.
	CryptObstacleRefRibcage = "dnd5e:props:ribcage"
	// CryptObstacleRefBoneScatter identifies a scattered-bones floor
	// dressing piece — walkable-past dressing.
	CryptObstacleRefBoneScatter = "dnd5e:props:bone-scatter"
	// CryptObstacleRefRubbleScatter identifies a scattered-rubble floor
	// dressing piece — walkable-past dressing.
	CryptObstacleRefRubbleScatter = "dnd5e:props:rubble-scatter"
)

// Wave-1 promotion refs, addendum (rpg-toolkit#854): the asset
// inventory's crypt reprioritization added 7 more candidates to the same
// batch, bringing the total to 16 -- same vocabulary-only scope and
// pattern as the const block above; resolved without renames the same
// way (see that block's doc). Only 6 are new here:
// "dnd5e:props:skeleton-remains" (false/false, "lying/shackled skeleton
// dressing") was also in this addendum's list, but it's byte-for-byte the
// same ref string and flags as CryptObstacleRefSkeletonRemains above
// (rpg-toolkit#839) -- not re-added, to avoid a duplicate constant for
// an identical value.
const (
	// CryptObstacleRefChains identifies a hanging-chains dressing piece —
	// walkable-past dressing (distinct from CryptObstacleRefChain,
	// singular, rpg-toolkit#839).
	CryptObstacleRefChains = "dnd5e:props:chains"
	// CryptObstacleRefGlowingOrb identifies a glowing-orb light/dressing
	// piece — walkable-past dressing.
	CryptObstacleRefGlowingOrb = "dnd5e:props:glowing-orb"
	// CryptObstacleRefRuneMarker identifies a floor/small rune-marker
	// dressing piece — walkable-past dressing.
	CryptObstacleRefRuneMarker = "dnd5e:props:rune-marker"
	// CryptObstacleRefRunePillar identifies a rune-carved pillar — a
	// structural piece, same flag shape as CryptObstacleRefPillar (blocks
	// both movement and LoS).
	CryptObstacleRefRunePillar = "dnd5e:props:rune-pillar"
	// CryptObstacleRefTombOpen identifies an opened sarcophagus/tomb — a
	// physical obstruction, same flag shape as CryptObstacleRefCoffin
	// (blocks movement, see over/around it).
	CryptObstacleRefTombOpen = "dnd5e:props:tomb-open"
	// CryptObstacleRefPillarBroken identifies a broken-pillar stump — a
	// physical obstruction, same flag shape as CryptObstacleRefCoffin/
	// CryptObstacleRefTombOpen (blocks movement, LoS clears over it).
	CryptObstacleRefPillarBroken = "dnd5e:props:pillar-broken"
)

// Crypt set-piece blocking properties: the VERIFIED canonical shipped-
// asset-contract table (independent finding, PR #826 review — supersedes
// this file's earlier "low vs tall" design guess, which had altar wrong),
// spanning #819's structural vocabulary and rpg-toolkit#839's depth-pass
// dressing/light-anchor additions. Every ref below already has a shipped,
// measured mesh in the asset manifest — see the SEPARATE table below this
// one for rpg-toolkit#854's wave-1 promotion refs, which started pending
// and are tracked distinctly until rpg-game-assets#36 (now merged, see
// #854's own doc above) fully collapses that distinction:
//
//	ref                    BlocksMovement  BlocksLoS
//	altar                  true            true   (measured 2.057m, role "obstacle")
//	statue-reaper          true            true
//	statue-knight-hooded   true            true
//	obelisk                true            true
//	pillar                 true            true
//	coffin/tomb            true            false  (walk around, see over)
//	brazier                true            false  (physical, but see over the flame)
//	torch-ornate           true            false  (same shape as brazier)
//	candles                false           false  (walkable-past floor dressing)
//	bone-pile              false           false  (walkable-past floor dressing)
//	chain                  false           false  (walkable-past floor dressing)
//	skeleton-remains       false           false  (walkable-past floor dressing)
//
// rpg-toolkit#854's wave-1 promotion refs (pending promotion —
// rpg-game-assets#36 has now shipped meshes reconciled to this exact
// 16-ref contract, resolving 15 of the 16; ribcage remains the one
// exception, blocked by a separate asset defect — see its own doc
// comment above):
//
//	ref                    BlocksMovement  BlocksLoS
//	rune-pillar            true            true   (#854 addendum, same shape as pillar)
//	tomb-open              true            false  (same shape as coffin, #854 addendum)
//	pillar-broken          true            false  (stump, same shape as coffin, #854 addendum)
//	stone-lantern          true            false  (same shape as brazier, #854)
//	torture-table          true            false  (same shape as coffin, #854)
//	chains                 false           false  (walkable-past floor dressing, #854 addendum)
//	rug                    false           false  (walkable-past floor dressing, #854)
//	wall-banner            false           false  (walkable-past floor dressing, #854)
//	candle-stand           false           false  (walkable-past floor dressing, #854; flame
//	                                               companion pending, see its own doc comment)
//	lantern                false           false  (walkable-past floor dressing, #854)
//	ribcage                false           false  (walkable-past floor dressing, #854; no
//	                                               shipped mesh yet, see its own doc comment)
//	bone-scatter           false           false  (walkable-past floor dressing, #854)
//	rubble-scatter         false           false  (walkable-past floor dressing, #854)
//	glowing-orb            false           false  (walkable-past floor dressing, #854 addendum)
//	rune-marker            false           false  (walkable-past floor dressing, #854 addendum)
//
// Three flag shapes, not one, across BOTH tables: structural pieces
// (obelisk/pillar/altar/statues, and #854 addendum's rune-pillar) block
// both movement and LoS; a see-over class (coffin/tomb, brazier,
// torch-ornate, #854's stone-lantern/torture-table, and #854 addendum's
// tomb-open/pillar-broken) is a physical obstruction that never blocks
// sightline over or around it (coffin/torture-table/tomb-open/
// pillar-broken: low enough or broken enough to see over; brazier/
// torch-ornate/stone-lantern: see over the flame); dressing (candles/
// bone-pile/chain/skeleton-remains, #854's rug/wall-banner/candle-stand/
// lantern/ribcage/bone-scatter/rubble-scatter, and #854 addendum's
// chains/glowing-orb/rune-marker) blocks neither.
const (
	cryptBlocksMovement = true
	cryptBlocksLoS      = true
	// cryptSeeOverBlocksLoS is BlocksLoS for a piece that's a physical
	// obstruction (BlocksMovement true) but never blocks sightline over
	// or around it: the coffin/tomb (table above), rpg-toolkit#839's
	// light anchors (brazier, torch-ornate — "you can see over flame"),
	// rpg-toolkit#854's stone-lantern/torture-table, and #854's addendum
	// tomb-open/pillar-broken.
	cryptSeeOverBlocksLoS = false
)

// Crypt floor-dressing blocking properties (rpg-toolkit#839): candles,
// bone-pile, chain, and skeleton-remains are walkable-past floor
// dressing, never a physical obstruction or sightline blocker — same
// shape rpg-toolkit#854's rug/wall-banner/candle-stand/lantern/ribcage/
// bone-scatter/rubble-scatter and #854's addendum chains/glowing-orb/
// rune-marker reuse — verified
// against this package's placement (placeRegionObstacles: each instance
// still consumes its own unique candidate cell regardless of blocking
// flags — no collision risk), room-rebuild (rebuildRoomFromData places a
// WallEntity per obstacle with these exact flags copied verbatim — a
// BlocksMovement=false entity places but never blocks CanPlaceEntity),
// and reveal (publishRevealedObstacles keys purely on hex-position
// overlap with a viewer's newly revealed hexes, never on either blocking
// flag) machinery — see obstacle_test.go's existing
// TestObstacle_BlocksMovement_False for the end-to-end proof this
// already holds. No fallback to blocking=true needed.
const (
	cryptDressingBlocksMovement = false
	cryptDressingBlocksLoS      = false
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

// cryptEntranceObstacles: an obelisk plus flanking pillars (#819), now
// joined by rpg-toolkit#839's depth-pass dressing — 2 braziers as the
// warm light anchors, plus 2 bone-piles as floor dressing. Judgment call
// on the issue's "1-2x bone-pile": chose 2, matching the fuller "gives
// the room a depth" composition target rather than the sparser end of
// the range — flagged in the PR body, easy to dial back to 1 if it reads
// too busy once rendered.
func cryptEntranceObstacles() []ObstacleSpec {
	return []ObstacleSpec{
		{Ref: CryptObstacleRefObelisk, Count: 1, BlocksMovement: cryptBlocksMovement, BlocksLoS: cryptBlocksLoS},
		{Ref: CryptObstacleRefPillar, Count: 2, BlocksMovement: cryptBlocksMovement, BlocksLoS: cryptBlocksLoS},
		{
			Ref: CryptObstacleRefBrazier, Count: 2, PreferBorder: true,
			BlocksMovement: cryptBlocksMovement, BlocksLoS: cryptSeeOverBlocksLoS,
		},
		{
			Ref: CryptObstacleRefBonePile, Count: 2, PreferBorder: true,
			BlocksMovement: cryptDressingBlocksMovement, BlocksLoS: cryptDressingBlocksLoS,
		},
	}
}

// cryptCorridorObstacles: a single sparse pillar (#819), plus
// rpg-toolkit#839's one light anchor — the corridor must stay
// monster-free and easily traversable, so this stays deliberately the
// lightest set-piece load of the three regions; a single torch is enough
// to carry a light pool through it without competing for floor space.
func cryptCorridorObstacles() []ObstacleSpec {
	return []ObstacleSpec{
		{Ref: CryptObstacleRefPillar, Count: 1, BlocksMovement: cryptBlocksMovement, BlocksLoS: cryptBlocksLoS},
		{
			Ref: CryptObstacleRefTorchOrnate, Count: 1, PreferBorder: true,
			BlocksMovement: cryptBlocksMovement, BlocksLoS: cryptSeeOverBlocksLoS,
		},
	}
}

// cryptBossObstacles: a coffin/tomb, an altar, and one of each of the
// two promoted exact statue variants (#819's fixed reaper+hooded-knight
// pairing, not a random-variant policy), now joined by rpg-toolkit#839's
// depth-pass dressing: 2 candles flanking the coffin, 2 braziers as
// light anchors, 1 hanging chain, and 1 skeleton-remains pile. Keeping
// coffin as coffin only — no tomb/sarcophagus variant is added here; see
// the PR body for why (#818 v1's single-anchor-per-instance limitation
// and rpg-dnd5e-web#577's rendering/composition ownership).
func cryptBossObstacles() []ObstacleSpec {
	return []ObstacleSpec{
		{Ref: CryptObstacleRefCoffin, Count: 1, BlocksMovement: cryptBlocksMovement, BlocksLoS: cryptSeeOverBlocksLoS},
		{Ref: CryptObstacleRefAltar, Count: 1, BlocksMovement: cryptBlocksMovement, BlocksLoS: cryptBlocksLoS},
		{Ref: CryptObstacleRefStatueReaper, Count: 1, BlocksMovement: cryptBlocksMovement, BlocksLoS: cryptBlocksLoS},
		{
			Ref: CryptObstacleRefStatueKnightHooded, Count: 1,
			BlocksMovement: cryptBlocksMovement, BlocksLoS: cryptBlocksLoS,
		},
		{
			Ref: CryptObstacleRefCandles, Count: 2, PreferBorder: true,
			BlocksMovement: cryptDressingBlocksMovement, BlocksLoS: cryptDressingBlocksLoS,
		},
		{
			Ref: CryptObstacleRefBrazier, Count: 2, PreferBorder: true,
			BlocksMovement: cryptBlocksMovement, BlocksLoS: cryptSeeOverBlocksLoS,
		},
		{
			Ref: CryptObstacleRefChain, Count: 1, PreferBorder: true,
			BlocksMovement: cryptDressingBlocksMovement, BlocksLoS: cryptDressingBlocksLoS,
		},
		{
			Ref: CryptObstacleRefSkeletonRemains, Count: 1, PreferBorder: true,
			BlocksMovement: cryptDressingBlocksMovement, BlocksLoS: cryptDressingBlocksLoS,
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
				// PatternEmpty (rpg-toolkit#835): PatternRandom's scattered
				// wall cells were designed as tactical cover before the
				// obstacle layer existed (#819) and now double-book that
				// role — the entrance's obelisk+pillar set pieces (#819)
				// already provide cover, and each isolated scattered wall
				// cell renders client-side as 3-5 crisscrossed slabs (one
				// per exposed face), reading as rubble rather than the
				// approved crypt art target of intact walls with decay
				// carried by dressing, not broken geometry (rpg-dnd5e-
				// web#469, rpg-dnd5e-web#562). Cover now comes from set
				// pieces; interior walls stay empty.
				Width: cryptEntranceWidth, Pattern: environments.PatternEmpty, Obstacles: cryptEntranceObstacles(),
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
				// PatternEmpty (rpg-toolkit#835): same reasoning as the
				// entrance region above — the boss chamber's coffin,
				// altar, and paired statues (#819) already carry the cover
				// role, so PatternRandom's scattered wall cells are now
				// redundant rubble-rendering noise rather than needed
				// tactical cover. Side benefit: shrinks this region's
				// exposure to the doorRow-collision bug class (#826,
				// #833) since fewer interior walls get rolled at all.
				Width: cryptBossWidth, Pattern: environments.PatternEmpty, Obstacles: cryptBossObstacles(),
			},
		},
		Connectors: []DungeonConnectorParams{
			{DoorID: entranceDoorID},
			{
				DoorID: bossDoorID, Locked: true, LockDC: 12,
				LockAbility: string(abilities.DEX),
			},
		},
	}
}
