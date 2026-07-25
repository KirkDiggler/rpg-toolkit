// Copyright (C) 2024 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package encounter

// seed_monsters_test.go is package encounter (not encounter_test): the
// regionOffsetX helper it tests in isolation is unexported, and this
// codebase's convention otherwise (every other _test.go file here is
// package encounter_test) is to test through the public API — this file
// is the one deliberate exception, needed for the same reason
// checkCombatEntry's own tests never need one (that logic is entirely
// reachable through AddMonster/AddPlayer/Move).

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/KirkDiggler/rpg-toolkit/encounter/core"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/damage"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/monster"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/monster/actions"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/refs"
	"github.com/KirkDiggler/rpg-toolkit/tools/environments"
	"github.com/KirkDiggler/rpg-toolkit/tools/spatial"
)

const (
	smHeight        = 8
	smEntranceWidth = 8
	smBossWidth     = 10
	smSeed          = 1

	smRoomIDEntrance = "entrance"
	smRoomIDBoss     = "boss"

	smRefSkeletonCaptain = "dnd5e:monsters:skeleton-captain"
	smRefSkeleton        = "dnd5e:monsters:skeleton"
	smRefZombie          = "dnd5e:monsters:zombie"

	smObstacleRefCrate  = "test:obstacles:crate"
	smObstacleRefCandle = "test:obstacles:candle"

	smAlicePlayerID = "alice"
	smAliceEntityID = "char-alice"

	smLongswordActionName = "longsword"
)

// smDungeonParams builds a minimal 2-region (entrance -> boss) dungeon,
// fixed at smSeed (no test in this file needs a different one — only the
// wiring/logic these tests exercise, not seed-dependent RNG behavior):
// the entrance holds the player's spawn, the boss region sits behind a
// closed connector door — closed doors block LoS the same as a solid
// wall (rebuildRoomFromData), so a monster placed there is guaranteed NOT
// visible from the entrance regardless of SightRange, no distance tuning
// needed.
func smDungeonParams() DungeonParams {
	return DungeonParams{
		Height:     smHeight,
		RandomSeed: smSeed,
		Regions: []DungeonRegionParams{
			{ID: smRoomIDEntrance, Archetype: ArchetypeEntrance, Width: smEntranceWidth, Pattern: environments.PatternEmpty},
			{ID: smRoomIDBoss, Archetype: ArchetypeBoss, Width: smBossWidth, Pattern: environments.PatternEmpty},
		},
		Connectors: []DungeonConnectorParams{{DoorID: "door-0"}},
	}
}

func smNewEncounter(t *testing.T) *Encounter {
	t.Helper()
	transport := NewInMemoryTransport()
	broker := NewBroker(transport)
	t.Cleanup(func() {
		_ = broker.Close()
		_ = transport.Close()
	})
	return New(context.Background(), "enc-seed-monsters-test", broker)
}

// marshalData snapshots enc's current state as JSON bytes. ToData()
// returns e.data directly (the SAME pointer every call, never a copy) —
// require.Equal on two ToData() results compares a pointer to itself and
// can never fail regardless of what happened in between. Marshaling to
// bytes at each call site captures a genuine point-in-time snapshot, so
// comparing before/after actually proves nothing changed.
func marshalData(t *testing.T, enc *Encounter) []byte {
	t.Helper()
	b, err := json.Marshal(enc.ToData())
	require.NoError(t, err)
	return b
}

// TestSeedMonsters_PlacedMonstersSpawnAtTheirCells: an At-bearing
// SpawnInstruction resolves its MonsterRef via monsters.ByRef and lands
// at exactly its declared region-local cell, translated by
// regionOffsetX — same conversion as Task N1's placed obstacles, minus
// the shortcut N1 gets from already being inside generateDungeonLayout.
func TestSeedMonsters_PlacedMonstersSpawnAtTheirCells(t *testing.T) {
	enc := smNewEncounter(t)
	require.NoError(t, enc.InitDungeon(smDungeonParams()))

	bossAt := LocalHex{Col: 7, Row: 5}
	entranceAt := LocalHex{Col: 4, Row: 2}
	spawns := []SpawnInstruction{
		{RoomID: smRoomIDBoss, MonsterRef: smRefSkeletonCaptain, Count: 1, At: &bossAt},
		{RoomID: smRoomIDEntrance, MonsterRef: smRefSkeleton, Count: 1, At: &entranceAt},
	}
	require.NoError(t, enc.SeedMonsters(spawns))

	data := enc.ToData()
	require.Len(t, data.Monsters, 2)

	bossOffsetX := smEntranceWidth + 1 // generateDungeonLayout's "+1 reserves the boundary/door column"
	wantBoss := core.HexFromPosition(spatial.Position{X: float64(bossOffsetX + bossAt.Col), Y: float64(bossAt.Row)})
	wantEntrance := core.HexFromPosition(spatial.Position{X: float64(entranceAt.Col), Y: float64(entranceAt.Row)})

	var sawBoss, sawEntrance bool
	for _, m := range data.Monsters {
		switch m.MonsterRef {
		case smRefSkeletonCaptain:
			sawBoss = true
			require.Equal(t, wantBoss, m.Position, "boss must land at exactly its declared cell")
			require.NotEmpty(t, m.DataJSON, "a resolved monster must carry rehydratable DataJSON")
			require.Positive(t, m.HP, "a resolved monster must carry its real stat block, not zero values")
		case smRefSkeleton:
			sawEntrance = true
			require.Equal(t, wantEntrance, m.Position, "entrance monster must land at exactly its declared cell")
		}
	}
	require.True(t, sawBoss, "boss spawn missing")
	require.True(t, sawEntrance, "entrance spawn missing")
}

// TestRegionOffsetX_RecoversStartsIFromPersistedHexes tests the helper in
// isolation from SeedMonsters' other machinery: build a 2-region
// InitDungeon, read back e.data.Space.Regions, assert regionOffsetX
// recovers the SAME offsetX generateDungeonLayout used internally
// (region[0].Width + 1, per its own doc).
func TestRegionOffsetX_RecoversStartsIFromPersistedHexes(t *testing.T) {
	enc := smNewEncounter(t)
	require.NoError(t, enc.InitDungeon(smDungeonParams()))

	data := enc.ToData()
	require.Len(t, data.Space.Regions, 2)
	require.Equal(t, smRoomIDEntrance, data.Space.Regions[0].ID)
	require.Equal(t, smRoomIDBoss, data.Space.Regions[1].ID)

	require.Equal(t, 0, regionOffsetX(data.Space.Regions[0].Hexes), "the first region always starts at column 0")
	require.Equal(t, smEntranceWidth+1, regionOffsetX(data.Space.Regions[1].Hexes))
}

// TestSeedMonsters_CombatEntryNeverSeesPartialRoster: boss placed at a
// cell NOT visible from the party's spawn (behind the closed connector
// door), entrance monster placed at a cell that IS. SeedMonsters must add
// both under suppressed combat-entry evaluation, then run ONE
// checkCombatEntry pass — initiative must never contain the boss if only
// the entrance monster was actually visible when the batch committed. A
// naive per-spawn AddMonster loop would NOT guarantee this: see
// combat_entry_test.go's TestIdempotent_AddMonsterAndMoveAfterCombatStarted,
// which proves a monster added AFTER combat has already started (from an
// earlier add's own visibility) gets appended to initiative unconditionally,
// regardless of its own visibility — exactly the bug this batch exists to
// prevent for boss-first spawn ordering.
func TestSeedMonsters_CombatEntryNeverSeesPartialRoster(t *testing.T) {
	enc := smNewEncounter(t)
	require.NoError(t, enc.InitDungeon(smDungeonParams()))

	entrance := enc.ToData().Space.Entrance
	require.NoError(t, enc.AddPlayer(PlayerInput{
		PlayerID: smAlicePlayerID, EntityID: smAliceEntityID, Position: entrance, SightRange: 30,
	}))
	require.Equal(t, core.ModeFreeRoam, enc.Mode())

	bossAt := LocalHex{Col: 7, Row: 5}
	entranceAt := LocalHex{Col: 1, Row: 2}
	bossMinionAt := LocalHex{Col: 3, Row: 1}
	// Order matters, and a boss+entrance-only fixture would NOT catch a
	// missing batch here: the entrance monster's add is the only one
	// that can trigger the FreeRoam->TurnBased flip, and by then the
	// boss is already resident in e.data.Monsters, so rollInitiative's
	// own per-monster visibility check already excludes it correctly
	// either way — verified empirically (a 2-monster version of this
	// test still passed with SeedMonsters calling plain AddMonster
	// instead of the suppressed addMonsterNoCombatCheck). The bug
	// AddMonster's "mode already TURN_BASED -> append unconditionally"
	// branch introduces (combat_entry_test.go's
	// TestIdempotent_AddMonsterAndMoveAfterCombatStarted) only bites a
	// monster added AFTER the triggering add — bossMinion, spawned
	// THIRD (after the entrance monster flips the mode), is that
	// monster, and is what actually isolates the invariant.
	spawns := []SpawnInstruction{
		{RoomID: smRoomIDBoss, MonsterRef: smRefSkeletonCaptain, Count: 1, At: &bossAt},
		{RoomID: smRoomIDEntrance, MonsterRef: smRefSkeleton, Count: 1, At: &entranceAt},
		{RoomID: smRoomIDBoss, MonsterRef: smRefZombie, Count: 1, At: &bossMinionAt},
	}
	require.NoError(t, enc.SeedMonsters(spawns))

	require.Equal(t, core.ModeTurnBased, enc.Mode(), "the visible entrance monster must have started combat")

	data := enc.ToData()
	var bossID, entranceID, minionID core.EntityID
	for id, m := range data.Monsters {
		switch m.MonsterRef {
		case smRefSkeletonCaptain:
			bossID = id
		case smRefSkeleton:
			entranceID = id
		case smRefZombie:
			minionID = id
		}
	}
	require.NotEmpty(t, entranceID)
	require.NotEmpty(t, bossID)
	require.NotEmpty(t, minionID)
	require.Contains(t, data.Initiative, entranceID, "the actually-visible entrance monster must be in the roster")
	require.NotContains(t, data.Initiative, bossID,
		"the boss (not visible, spawned before the flip) must NOT be in initiative")
	require.NotContains(t, data.Initiative, minionID,
		"the boss-room minion (not visible, spawned AFTER the flip-triggering entrance monster) must NOT be "+
			"in initiative — this is the case a naive per-spawn AddMonster loop gets wrong: once combat has "+
			"already started, AddMonster appends every subsequent add to initiative unconditionally, "+
			"regardless of that monster's own visibility")
}

// TestSeedMonsters_SpawnVisibleStillStartsCombat: the AddPlayer-doesn't-
// check trap, reverified on today's origin/main (encounter.go's AddPlayer
// never calls checkCombatEntry; only AddMonster/Move do) — batching must
// not lose "combat starts immediately if you spawn already visible."
func TestSeedMonsters_SpawnVisibleStillStartsCombat(t *testing.T) {
	enc := smNewEncounter(t)
	require.NoError(t, enc.InitDungeon(smDungeonParams()))

	entrance := enc.ToData().Space.Entrance
	require.NoError(t, enc.AddPlayer(PlayerInput{
		PlayerID: smAlicePlayerID, EntityID: smAliceEntityID, Position: entrance, SightRange: 30,
	}))
	require.Equal(t, core.ModeFreeRoam, enc.Mode())

	at := LocalHex{Col: 1, Row: 2}
	spawns := []SpawnInstruction{
		{RoomID: smRoomIDEntrance, MonsterRef: smRefSkeleton, Count: 1, At: &at},
	}
	require.NoError(t, enc.SeedMonsters(spawns))

	require.Equal(t, core.ModeTurnBased, enc.Mode(),
		"a monster placed already visible must start combat, exactly like a direct AddMonster would — "+
			"batching/suppression must not swallow this")
	require.NotEmpty(t, enc.ToData().Initiative)
}

// TestSeedMonsters_UnpinnedInstructionReturnsNotYetSupported: At == nil,
// REGARDLESS of Count (including Count == 1 — an unpinned single-monster
// room is just as unimplemented in M1 as a Count > 1 room; there's no
// safe-cell-rolling machinery for either yet). M2's Slice C replaces this
// error path with real safe-cell rolling for every At == nil instruction.
func TestSeedMonsters_UnpinnedInstructionReturnsNotYetSupported(t *testing.T) {
	cases := []struct {
		name  string
		count int
	}{
		{"count 1", 1},
		{"count 3", 3},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			enc := smNewEncounter(t)
			require.NoError(t, enc.InitDungeon(smDungeonParams()))

			spawns := []SpawnInstruction{
				{RoomID: smRoomIDEntrance, MonsterRef: smRefSkeleton, Count: tc.count, At: nil},
			}
			err := enc.SeedMonsters(spawns)
			require.Error(t, err)
			require.Contains(t, err.Error(), "M2")
		})
	}
}

// smSeedOneSkeleton spawns a single skeleton at a fixed, valid entrance
// cell and returns its entity ID.
func smSeedOneSkeleton(t *testing.T, enc *Encounter) core.EntityID {
	t.Helper()
	at := LocalHex{Col: 1, Row: 2}
	require.NoError(t, enc.SeedMonsters([]SpawnInstruction{
		{RoomID: smRoomIDEntrance, MonsterRef: smRefSkeleton, Count: 1, At: &at},
	}))
	data := enc.ToData()
	require.Len(t, data.Monsters, 1)
	var id core.EntityID
	for mid := range data.Monsters {
		id = mid
	}
	return id
}

// TestSeedMonsters_SpeedConvertedFromFeetToHexes: monster.Monster.Speed()
// reports Walk in FEET (e.g. 30 for a skeleton); npc.go/action.go consume
// MonsterData.Speed as HEXES directly (movement remaining for the turn).
// Storing the raw feet value gives a seeded skeleton 30 hexes (150ft) of
// movement per turn instead of 6 (30ft) — the same feet->hexes conversion
// devseed/main.go and rpg-api's crypt_monster_seed.go both apply
// (cryptMinionSpeed = 6 // 30ft / 5ft per hex) must happen here too.
func TestSeedMonsters_SpeedConvertedFromFeetToHexes(t *testing.T) {
	enc := smNewEncounter(t)
	require.NoError(t, enc.InitDungeon(smDungeonParams()))

	id := smSeedOneSkeleton(t, enc)
	m := enc.ToData().Monsters[id]
	require.Equal(t, 6, m.Speed, "skeleton's 30ft walk speed must convert to 6 hexes (30/5), not store raw feet")
}

// TestSeedMonsters_OpportunityAttackReadySeeded: encounter.go only seeds
// OA readiness when MonsterInput.DamageDice is non-empty, and never
// derives readiness from DataJSON — rpg-api's gamectx readiness map is
// built straight from data.ReactionReadiness. A SeedMonsters caller that
// leaves the flat AttackBonus/DamageDice/DamageType fields empty
// (reasoning DataJSON alone carries the monster's real stats) silently
// starves every seeded monster's OA reaction of readiness forever.
func TestSeedMonsters_OpportunityAttackReadySeeded(t *testing.T) {
	enc := smNewEncounter(t)
	require.NoError(t, enc.InitDungeon(smDungeonParams()))

	id := smSeedOneSkeleton(t, enc)
	require.True(t, enc.IsReactionReady(id, OAReactionRef),
		"a seeded monster with a real attack action must have its Opportunity Attack reaction seeded ready")

	m := enc.ToData().Monsters[id]
	require.NotZero(t, m.AttackBonus,
		"the flat combat-snapshot AttackBonus must be populated from the monster's own attack action")
	require.NotEmpty(t, m.DamageDice, "the flat combat-snapshot DamageDice must be populated")
	require.NotEmpty(t, m.DamageType, "the flat combat-snapshot DamageType must be populated")
}

// TestSeedMonsters_MidBatchInvalidInstructionLeavesEncounterUnchanged:
// an invalid instruction ANYWHERE in the batch (here: the second of two)
// must commit NOTHING — no monsters added, mode unchanged, encounter
// byte-identical to before the call. This is the all-or-nothing batch
// validation contract (mirrors placeVerbatimObstacles' own contract one
// file over, dungeon.go): SeedMonsters validates the ENTIRE batch before
// committing any of it, rather than committing earlier instructions and
// only failing on a later one.
func TestSeedMonsters_MidBatchInvalidInstructionLeavesEncounterUnchanged(t *testing.T) {
	enc := smNewEncounter(t)
	require.NoError(t, enc.InitDungeon(smDungeonParams()))
	before := marshalData(t, enc)

	validAt := LocalHex{Col: 1, Row: 2}
	badAt := LocalHex{Col: 2, Row: 2}
	spawns := []SpawnInstruction{
		{RoomID: smRoomIDEntrance, MonsterRef: smRefSkeleton, Count: 1, At: &validAt},
		{RoomID: smRoomIDEntrance, MonsterRef: "dnd5e:monsters:beholder", Count: 1, At: &badAt}, // unresolvable ref
	}
	err := enc.SeedMonsters(spawns)
	require.Error(t, err)

	require.Empty(t, enc.ToData().Monsters, "a bad instruction anywhere in the batch must commit NOTHING, "+
		"not just skip the failing one")
	require.Equal(t, core.ModeFreeRoam, enc.Mode())
	after := marshalData(t, enc)
	require.Equal(t, before, after, "encounter must be byte-identical to before the call")
}

// TestSeedMonsters_OutOfBoundsAtRejected: a cell outside the target
// region's own extent (here: one column past its right edge, landing in
// the connector's boundary/door column) is rejected before any commit.
func TestSeedMonsters_OutOfBoundsAtRejected(t *testing.T) {
	enc := smNewEncounter(t)
	require.NoError(t, enc.InitDungeon(smDungeonParams()))

	at := LocalHex{Col: smEntranceWidth, Row: 2}
	err := enc.SeedMonsters([]SpawnInstruction{
		{RoomID: smRoomIDEntrance, MonsterRef: smRefSkeleton, Count: 1, At: &at},
	})
	require.Error(t, err)
	require.Contains(t, err.Error(), "out of bounds")
	require.Contains(t, err.Error(), "width=8") // pins the region's own width in the detail, not just "out of bounds"
	require.Empty(t, enc.ToData().Monsters)
}

// TestSeedMonsters_TwoInstructionsOnOneCellRejected: two instructions
// targeting the exact same cell (whether in the same room or not) is
// rejected before any commit — the batch-internal collision half of the
// "not already claimed by this batch or existing occupant" check.
func TestSeedMonsters_TwoInstructionsOnOneCellRejected(t *testing.T) {
	enc := smNewEncounter(t)
	require.NoError(t, enc.InitDungeon(smDungeonParams()))

	at := LocalHex{Col: 1, Row: 2}
	spawns := []SpawnInstruction{
		{RoomID: smRoomIDEntrance, MonsterRef: smRefSkeleton, Count: 1, At: &at},
		{RoomID: smRoomIDEntrance, MonsterRef: smRefZombie, Count: 1, At: &at},
	}
	err := enc.SeedMonsters(spawns)
	require.Error(t, err)
	require.Contains(t, err.Error(), "collides")
	require.Empty(t, enc.ToData().Monsters)
}

// TestSeedMonsters_CountGreaterThanOneWithAtRejected: a placed
// (At-bearing) instruction must have Count exactly 1 — a caller asking
// for 5 placed instances at one cell is rejected outright, never
// silently truncated to 1.
func TestSeedMonsters_CountGreaterThanOneWithAtRejected(t *testing.T) {
	enc := smNewEncounter(t)
	require.NoError(t, enc.InitDungeon(smDungeonParams()))

	at := LocalHex{Col: 1, Row: 2}
	err := enc.SeedMonsters([]SpawnInstruction{
		{RoomID: smRoomIDEntrance, MonsterRef: smRefSkeleton, Count: 5, At: &at},
	})
	require.Error(t, err)
	require.Contains(t, err.Error(), "Count")
	require.Empty(t, enc.ToData().Monsters)
}

// TestSeedMonsters_ObstacleCollisionRejected: a BLOCKING obstacle already
// in the encounter's Space joins the collision domain — a monster pinned
// onto its exact cell is rejected before any commit, the same as
// colliding with another monster. Non-blocking obstacles (candles,
// bone-pile, chain — data.go's ObstacleData.BlocksMovement doc) do NOT
// reserve a cell here, matching room-rebuild's own walkability semantics.
// Rolls a full spread of blocking obstacles (rolled placement is
// seed-dependent, so the exact cell can't be hardcoded) and reads one
// back to target directly.
func TestSeedMonsters_ObstacleCollisionRejected(t *testing.T) {
	enc := smNewEncounter(t)
	params := smDungeonParams()
	params.Regions[0].Obstacles = []ObstacleSpec{
		{Ref: smObstacleRefCrate, Count: 6, BlocksMovement: true, BlocksLoS: false},
	}
	require.NoError(t, enc.InitDungeon(params))

	obstacles := enc.ToData().Space.Obstacles
	require.NotEmpty(t, obstacles, "fixture must actually roll obstacles for this to be a real proof")
	target := obstacles[0]

	// Entrance is region 0 (offsetX 0), so absolute position IS local
	// position directly — no offsetX translation needed.
	pos := target.Position.ToPosition()
	at := LocalHex{Col: int(pos.X), Row: int(pos.Y)}

	err := enc.SeedMonsters([]SpawnInstruction{
		{RoomID: smRoomIDEntrance, MonsterRef: smRefSkeleton, Count: 1, At: &at},
	})
	require.Error(t, err)
	require.Contains(t, err.Error(), "collides with obstacle")
	require.Empty(t, enc.ToData().Monsters)
}

// TestSeedMonsters_SecondCallMintedIDCollisionRejectsNothingCommitted:
// SeedMonsters mints "monster-<roomID>-<n>" IDs starting at 0 for each
// room on every call — a second call touching a room an earlier call
// already seeded would re-mint an ID already in the encounter. This is
// caught in the VALIDATION pass (not deferred to commit-time, where it
// would only be caught mid-commit by addMonsterNoCombatCheck's own
// "already in encounter" check, after any earlier-in-THIS-batch spawns
// already committed) — a batch mixing a fresh room and a colliding room
// must commit NOTHING from the second call, including the fresh room's
// otherwise-valid spawn.
func TestSeedMonsters_SecondCallMintedIDCollisionRejectsNothingCommitted(t *testing.T) {
	enc := smNewEncounter(t)
	require.NoError(t, enc.InitDungeon(smDungeonParams()))

	firstAt := LocalHex{Col: 1, Row: 2}
	require.NoError(t, enc.SeedMonsters([]SpawnInstruction{
		{RoomID: smRoomIDEntrance, MonsterRef: smRefSkeleton, Count: 1, At: &firstAt},
	}))
	before := marshalData(t, enc)

	// Second call: a fresh room (boss, untouched by the first call) plus
	// a spawn back in entrance -- which would re-mint "monster-entrance-0",
	// colliding with the first call's monster.
	bossAt := LocalHex{Col: 7, Row: 5}
	secondEntranceAt := LocalHex{Col: 3, Row: 1}
	err := enc.SeedMonsters([]SpawnInstruction{
		{RoomID: smRoomIDBoss, MonsterRef: smRefSkeletonCaptain, Count: 1, At: &bossAt},
		{RoomID: smRoomIDEntrance, MonsterRef: smRefZombie, Count: 1, At: &secondEntranceAt},
	})
	require.Error(t, err)
	require.Contains(t, err.Error(), "already in encounter")

	after := marshalData(t, enc)
	require.Equal(t, before, after,
		"the second call's failure must commit NOTHING -- not even the boss spawn that would have succeeded alone")
}

// TestSeedMonsters_DoorRowRejected: a cell on the shared doorRow
// (height/2) is rejected before any commit — belt-and-suspenders with
// dungeonspec's own load-time check, since SeedMonsters is reachable by
// callers other than the content-hosting path. Verified by temporarily
// deleting the check: confirmed this test fails (no error) before
// restoring it — same discipline as N1's mutation testing.
func TestSeedMonsters_DoorRowRejected(t *testing.T) {
	enc := smNewEncounter(t)
	require.NoError(t, enc.InitDungeon(smDungeonParams()))

	at := LocalHex{Col: 1, Row: smHeight / 2}
	err := enc.SeedMonsters([]SpawnInstruction{
		{RoomID: smRoomIDEntrance, MonsterRef: smRefSkeleton, Count: 1, At: &at},
	})
	require.Error(t, err)
	require.Contains(t, err.Error(), "reserved row")
	require.Empty(t, enc.ToData().Monsters)
}

// TestSeedMonsters_WallCellRejected guards the non-obvious Start != End
// skip in validateSpawnBatch's wallCubes build: SpaceData.Walls also
// carries boundary-edge perimeter segments (rpg-toolkit#834, Start !=
// End) that are NOT blocking wall cells — Start is real walkable floor,
// End is the adjacent hex outside the grid. A naive "every Walls entry
// blocks" reading would wrongly reject a perimeter floor cell (a boss
// pinned against the back wall is ordinary content, not a rejected
// placement). This test proves BOTH directions on the SAME seed: a real
// interior wall cell (Start == End) is rejected, AND a perimeter floor
// cell (which also appears in Walls, but only via its Start != End
// boundary segment) is accepted — only the acceptance half actually
// depends on the skip; deleting it leaves the rejection half green on
// its own (verified: the skip's removal doesn't touch Start == End
// entries at all, so the interior-wall assertion alone can't catch it).
func TestSeedMonsters_WallCellRejected(t *testing.T) {
	var wallCell LocalHex
	var seedUsed int64
	found := false
	for seed := int64(1); seed <= 50 && !found; seed++ {
		probe := smNewEncounter(t)
		params := smDungeonParams()
		params.RandomSeed = seed
		params.Regions[0].Pattern = environments.PatternRandom
		require.NoError(t, probe.InitDungeon(params), "seed %d", seed)
		for _, w := range probe.ToData().Space.Walls {
			if w.Start != w.End {
				continue // boundary-edge perimeter segment, not an interior wall cell
			}
			pos := w.Start.ToOffsetCoordinateWithOrientation(spatial.HexOrientationPointyTop)
			col, row := int(pos.X), int(pos.Y) // entrance is region 0, offsetX 0
			if col >= 0 && col < smEntranceWidth && row != smHeight/2 {
				wallCell = LocalHex{Col: col, Row: row}
				seedUsed = seed
				found = true
				break
			}
		}
	}
	require.True(t, found, "expected at least one seed in [1,50] to produce an interior wall cell in the entrance")

	enc := smNewEncounter(t)
	params := smDungeonParams()
	params.RandomSeed = seedUsed
	params.Regions[0].Pattern = environments.PatternRandom
	require.NoError(t, enc.InitDungeon(params))

	err := enc.SeedMonsters([]SpawnInstruction{
		{RoomID: smRoomIDEntrance, MonsterRef: smRefSkeleton, Count: 1, At: &wallCell},
	})
	require.Error(t, err)
	require.Contains(t, err.Error(), "wall cell")
	require.Empty(t, enc.ToData().Monsters)

	// Perimeter acceptance: Col 0 sits on the whole space's own left edge,
	// so this cell ALSO appears in Space.Walls — but only via a Start !=
	// End boundary-edge segment (its west-facing side lands outside the
	// grid); it is real walkable floor. This is what the Start != End
	// skip actually guards: without it, this cell would wrongly land in
	// wallCubes too and get rejected, even though it's ordinary content.
	perimeterAt := LocalHex{Col: 0, Row: 1}
	require.NotEqual(t, wallCell, perimeterAt,
		"the discovered interior wall and the perimeter probe must be distinct cells")
	require.NoError(t, enc.SeedMonsters([]SpawnInstruction{
		{RoomID: smRoomIDEntrance, MonsterRef: smRefZombie, Count: 1, At: &perimeterAt},
	}), "a perimeter floor cell (boundary-edge segment only, no interior wall) must be accepted")
}

// TestSeedMonsters_PlayerCollisionRejected: a player already occupying a
// cell joins the collision domain, the same as a monster or blocking
// obstacle — without this, a spawn could silently co-locate with a party
// member.
func TestSeedMonsters_PlayerCollisionRejected(t *testing.T) {
	enc := smNewEncounter(t)
	require.NoError(t, enc.InitDungeon(smDungeonParams()))

	at := LocalHex{Col: 1, Row: 2}
	pos := core.HexFromPosition(spatial.Position{X: float64(at.Col), Y: float64(at.Row)}) // entrance offsetX=0
	require.NoError(t, enc.AddPlayer(PlayerInput{
		PlayerID: smAlicePlayerID, EntityID: smAliceEntityID, Position: pos, SightRange: 5,
	}))

	err := enc.SeedMonsters([]SpawnInstruction{
		{RoomID: smRoomIDEntrance, MonsterRef: smRefSkeleton, Count: 1, At: &at},
	})
	require.Error(t, err)
	require.Contains(t, err.Error(), "collides with player")
	require.Empty(t, enc.ToData().Monsters)
}

// TestSeedMonsters_ExistingMonsterCollisionRejected: a monster already in
// the encounter (added directly via AddMonster, not SeedMonsters — so no
// minted-ID collision is possible here, isolating the position-collision
// check specifically) joins the collision domain the same as a player or
// blocking obstacle.
func TestSeedMonsters_ExistingMonsterCollisionRejected(t *testing.T) {
	enc := smNewEncounter(t)
	require.NoError(t, enc.InitDungeon(smDungeonParams()))

	at := LocalHex{Col: 1, Row: 2}
	pos := core.HexFromPosition(spatial.Position{X: float64(at.Col), Y: float64(at.Row)})
	require.NoError(t, enc.AddMonster(MonsterInput{ID: "hand-placed-goblin", Position: pos, HP: 7, MaxHP: 7}))

	err := enc.SeedMonsters([]SpawnInstruction{
		{RoomID: smRoomIDEntrance, MonsterRef: smRefSkeleton, Count: 1, At: &at},
	})
	require.Error(t, err)
	require.Contains(t, err.Error(), "collides with monster")
	require.Len(t, enc.ToData().Monsters, 1,
		"only the hand-placed monster must remain -- SeedMonsters must not have committed anything")
}

// TestSeedMonsters_UnknownRoomIDRejected: a RoomID that names no real
// region is rejected before any commit.
func TestSeedMonsters_UnknownRoomIDRejected(t *testing.T) {
	enc := smNewEncounter(t)
	require.NoError(t, enc.InitDungeon(smDungeonParams()))

	at := LocalHex{Col: 1, Row: 2}
	err := enc.SeedMonsters([]SpawnInstruction{
		{RoomID: "nonexistent-room", MonsterRef: smRefSkeleton, Count: 1, At: &at},
	})
	require.Error(t, err)
	require.Contains(t, err.Error(), "no such region")
	require.Empty(t, enc.ToData().Monsters)
}

// TestSeedMonsters_BeforeInitDungeonRejected: SeedMonsters on a fresh
// encounter with no Space initialized yet is rejected outright, not a
// panic or a nil-pointer crash deeper in validateSpawnBatch.
func TestSeedMonsters_BeforeInitDungeonRejected(t *testing.T) {
	enc := smNewEncounter(t)
	at := LocalHex{Col: 1, Row: 2}
	err := enc.SeedMonsters([]SpawnInstruction{
		{RoomID: smRoomIDEntrance, MonsterRef: smRefSkeleton, Count: 1, At: &at},
	})
	require.Error(t, err)
	require.Contains(t, err.Error(), "no space initialized")
}

// TestSeedMonsters_NonBlockingObstacleDoesNotReserveCell: a non-blocking
// obstacle (BlocksMovement false — candles, bone-pile, chain in the real
// crypt content) does NOT reserve its cell; a monster may be pinned
// there. Rolls a spread of non-blocking obstacles and targets one back.
func TestSeedMonsters_NonBlockingObstacleDoesNotReserveCell(t *testing.T) {
	enc := smNewEncounter(t)
	params := smDungeonParams()
	params.Regions[0].Obstacles = []ObstacleSpec{
		{Ref: smObstacleRefCandle, Count: 6, BlocksMovement: false, BlocksLoS: false},
	}
	require.NoError(t, enc.InitDungeon(params))

	obstacles := enc.ToData().Space.Obstacles
	require.NotEmpty(t, obstacles, "fixture must actually roll obstacles for this to be a real proof")
	target := obstacles[0]
	pos := target.Position.ToPosition()
	at := LocalHex{Col: int(pos.X), Row: int(pos.Y)} // entrance offsetX=0

	err := enc.SeedMonsters([]SpawnInstruction{
		{RoomID: smRoomIDEntrance, MonsterRef: smRefSkeleton, Count: 1, At: &at},
	})
	require.NoError(t, err, "a non-blocking obstacle must not reserve its cell")
	require.Len(t, enc.ToData().Monsters, 1)
}

// TestPrimaryAttackSnapshot_SkipsMultiattackAction proves the Multiattack
// skip directly: every real 11-constructor monster with a Multiattack
// action happens to register its individual attack action(s) FIRST
// (skeleton-captain: longsword, then multiattack), so primaryAttackSnapshot
// never actually needs to skip past one in practice — that ordering
// coincidence would let a bug (e.g. accidentally treating Multiattack as
// if it had its own damage_dice) hide. Building a monster with Multiattack
// registered FIRST isolates the skip from that coincidence.
func TestPrimaryAttackSnapshot_SkipsMultiattackAction(t *testing.T) {
	mon := monster.New(monster.Config{
		ID: "test-multiattack-first", Name: "Test", Ref: refs.Monsters.SkeletonCaptain(), HP: 10, AC: 10,
	})
	mon.AddAction(actions.NewMultiattackAction(actions.MultiattackConfig{
		Attacks: []string{smLongswordActionName, smLongswordActionName},
	}))
	mon.AddAction(actions.NewMeleeAction(actions.MeleeConfig{
		Name: smLongswordActionName, AttackBonus: 5, DamageDice: "1d8+3", Reach: 1, DamageType: damage.Slashing,
	}))

	attackBonus, damageDice, damageType := primaryAttackSnapshot(mon)
	require.Equal(t, 5, attackBonus)
	require.Equal(t, "1d8+3", damageDice)
	require.Equal(t, string(damage.Slashing), damageType)
}
