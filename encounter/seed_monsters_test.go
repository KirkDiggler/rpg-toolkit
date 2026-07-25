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
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/KirkDiggler/rpg-toolkit/encounter/core"
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
		PlayerID: "alice", EntityID: "char-alice", Position: entrance, SightRange: 30,
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
		PlayerID: "alice", EntityID: "char-alice", Position: entrance, SightRange: 30,
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
