// Copyright (C) 2024 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

// package dungeonspec (not dungeonspec_test): this file calls the
// unexported compile() directly, bypassing Load's Validate call, to test
// compile()'s own nil-boss.At guard independently of whether Validate
// currently forbids that shape (M1 does; M2's Task C0 lifts it).
package dungeonspec

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Room ids shared by both hand-built specs below (goconst) -- kept
// distinct from the archetypeEntrance/archetypeBoss vocabulary constants
// (validate.go) even though entranceRoomID happens to share a string value
// with archetypeEntrance: one names an arbitrary room id, the other the
// fixed archetype vocabulary, and conflating them (reusing one constant
// for both fields) is the exact mistake this split avoids.
const (
	entranceRoomID = "entrance"
	bossRoomID     = "boss-room"
)

// TestCompile_NilBossAtProducesRolledSpawn calls compile() on a hand-built
// spec with Boss.At == nil -- a shape Validate currently rejects in M1
// (the temporary at-pinning restriction, Task B2), so Load itself can
// never reach this path today. compile()'s own doc says it "still returns
// an error rather than panicking... in case that assumption is ever
// violated by a future caller" -- this test is that violation, constructed
// directly at the compile layer since Load's Validate call would otherwise
// make it unreachable. This is also exactly the shape M2's Task C0 lands:
// an unpinned boss becomes a rolled (Count-based, no At) spawn candidate,
// so the assertion below (Count: 1, At: nil) is the permanent contract,
// not a placeholder.
func TestCompile_NilBossAtProducesRolledSpawn(t *testing.T) {
	spec := &DungeonSpec{
		Version: 1,
		Key:     "nil-boss-at-check",
		Name:    "Nil Boss At Check",
		Height:  8,
		Rooms: []RoomSpec{
			{ID: entranceRoomID, Archetype: archetypeEntrance, Width: 6},
			{
				ID: bossRoomID, Archetype: archetypeBoss, Width: 7,
				Boss: &BossEntry{Ref: "dnd5e:monsters:skeleton-captain"}, // At: nil
			},
		},
		Connectors: []ConnectorSpec{{From: entranceRoomID, To: bossRoomID}},
	}

	compiled, err := compile(spec)
	require.NoError(t, err)
	require.NotEmpty(t, compiled.Spawns)
	boss := compiled.Spawns[0]
	assert.Equal(t, "dnd5e:monsters:skeleton-captain", boss.MonsterRef)
	assert.Equal(t, 1, boss.Count)
	assert.Nil(t, boss.At)
}

// TestCompile_RoomMonstersGuardedAgainstSilentDrop mirrors
// TestCompile_NilBossAtProducesRolledSpawn's shape: compileRoom does not
// yet know how to compile count-based room.Monsters entries into rolled
// SpawnInstructions (that's M2's Task C0), so it must error loudly rather
// than silently drop them -- a compiler that just ignored room.Monsters
// would still produce a CompiledDungeon with zero error, quietly losing
// every rolled monster in the process. Constructed directly at the
// compile layer (bypassing Validate, which rejects this shape in M1 via
// validateM1Restrictions) for the same reason as the nil-boss.At test
// above: this is the guard's whole point, independent of whether
// Validate currently makes it unreachable. This test is also the
// forcing function for M2's Task C0: deleting validateM1Restrictions
// alone, without teaching compileRoom to actually compile room.Monsters,
// fails THIS test instead of silently vanishing monsters at runtime.
func TestCompile_RoomMonstersGuardedAgainstSilentDrop(t *testing.T) {
	spec := &DungeonSpec{
		Version: 1,
		Key:     "room-monsters-guard-check",
		Name:    "Room Monsters Guard Check",
		Height:  8,
		Rooms: []RoomSpec{
			{
				ID: entranceRoomID, Archetype: archetypeEntrance, Width: 6,
				Monsters: []MonsterEntry{{Ref: "dnd5e:monsters:skeleton", Count: 2}},
			},
			{
				ID: bossRoomID, Archetype: archetypeBoss, Width: 7,
				Boss: &BossEntry{Ref: "dnd5e:monsters:skeleton-captain", At: &[2]int{3, 5}},
			},
		},
		Connectors: []ConnectorSpec{{From: entranceRoomID, To: bossRoomID}},
	}

	_, err := compile(spec)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "rolled monsters not compiled yet")
}
