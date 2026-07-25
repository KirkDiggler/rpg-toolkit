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
	const entranceRoomID = "entrance"
	spec := &DungeonSpec{
		Version: 1,
		Key:     "nil-boss-at-check",
		Name:    "Nil Boss At Check",
		Height:  8,
		Rooms: []RoomSpec{
			{ID: entranceRoomID, Archetype: entranceRoomID, Width: 6},
			{
				ID: "boss-room", Archetype: "boss", Width: 7,
				Boss: &BossEntry{Ref: "dnd5e:monsters:skeleton-captain"}, // At: nil
			},
		},
		Connectors: []ConnectorSpec{{From: entranceRoomID, To: "boss-room"}},
	}

	compiled, err := compile(spec)
	require.NoError(t, err)
	require.NotEmpty(t, compiled.Spawns)
	boss := compiled.Spawns[0]
	assert.Equal(t, "dnd5e:monsters:skeleton-captain", boss.MonsterRef)
	assert.Equal(t, 1, boss.Count)
	assert.Nil(t, boss.At)
}
