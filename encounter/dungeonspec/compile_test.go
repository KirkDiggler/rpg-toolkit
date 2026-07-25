// Copyright (C) 2024 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package dungeonspec_test

import (
	"testing"

	"github.com/KirkDiggler/rpg-toolkit/encounter"
	"github.com/KirkDiggler/rpg-toolkit/encounter/core"
	"github.com/KirkDiggler/rpg-toolkit/encounter/dungeonspec"
	"github.com/KirkDiggler/rpg-toolkit/tools/environments"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLoad_GeneratesDeterministicDoorIDs(t *testing.T) {
	// placedTombYAML: key "reference-tomb", 2 rooms (entrance, tomb), 1 connector —
	// the M1-valid fixture (referenceYAML's own count-based monsters fail Validate
	// in M1; see Task B2's fixture consequence). N-connector coverage (3+ doors in
	// one file) resumes in M2 once referenceYAML/cryptYAML are loadable again.
	compiled, err := dungeonspec.Load([]byte(placedTombYAML))
	require.NoError(t, err)
	require.Len(t, compiled.Params.Connectors, 1)
	assert.Equal(t, core.EntityID("reference-tomb-door-entrance-tomb"),
		compiled.Params.Connectors[0].DoorID)
}

// PATTERN-DEFAULT TRAP (advisory made explicit): the engine treats an
// empty DungeonRegionParams.Pattern as PatternRandom. The SPEC's default
// is empty (design.md). The compiler must therefore map "" and "empty" ->
// environments.PatternEmpty and "scattered" -> environments.PatternRandom
// EXPLICITLY -- never pass the zero value through. placedTombYAML's own
// rooms are both empty-pattern (no room in it is scattered -- Task B2's
// scattered-rejection rows exercise scattered ONLY in combination with
// place/boss.at, never the plain mapping), so the scattered->PatternRandom
// direction needs its OWN small fixture, inline here, decoupled from
// placedTombYAML/validM1YAML entirely.
func TestLoad_ScatteredPatternMapsToPatternRandom(t *testing.T) {
	// Throwaway 2-room fixture threading all three interacting rules
	// correctly (a fixture missing any one of them fails Validate for an
	// UNRELATED reason before this test's actual assertion ever runs):
	// exactly one boss-archetype room is required (v1's own rule) with a
	// non-nil boss: entry (M1's permanent boss-required rule, Task B2) and
	// a pinned at: (M1's temporary at-pinning rule, lifted in M2's Task
	// C0) -- so the boss room carries `pattern` empty/default (NOT
	// scattered; the scattered-rejection rule only fires on a room that
	// HAS place/pinned boss.at, and this room does), width 7 (height 8,
	// boss primary axis needs min(width,height) > 6). The entrance room
	// is the one under test: pattern: scattered, no place/boss.at, so
	// none of the pinning rules apply to it at all.
	const scatteredYAML = `
version: 1
key: pattern-mapping-check
name: Pattern Mapping Check
height: 8
rooms:
  - {id: entrance, archetype: entrance, width: 6, pattern: scattered}
  - {id: boss-room, archetype: boss, width: 7, boss: {ref: "dnd5e:monsters:skeleton-captain", at: [3, 5]}}
connectors:
  - {from: entrance, to: boss-room}
`
	// scattered is legal on its own -- design.md's clarification.
	compiled, err := dungeonspec.Load([]byte(scatteredYAML))
	require.NoError(t, err)
	// entrance: scattered -> PatternRandom.
	assert.Equal(t, environments.PatternRandom, compiled.Params.Regions[0].Pattern)
	// boss-room: default -> PatternEmpty, same test.
	assert.Equal(t, environments.PatternEmpty, compiled.Params.Regions[1].Pattern)
}

// regionByID finds a compiled region by id, failing the test loudly if
// absent, so callers don't have to repeat a linear scan + nil check.
func regionByID(t *testing.T, regions []encounter.DungeonRegionParams, id string) *encounter.DungeonRegionParams {
	t.Helper()
	for i := range regions {
		if regions[i].ID == id {
			return &regions[i]
		}
	}
	t.Fatalf("no region %q in compiled regions", id)
	return nil
}

func TestLoad_PlaceRoutesByRefType(t *testing.T) {
	compiled, err := dungeonspec.Load([]byte(placedTombYAML))
	require.NoError(t, err)
	tombRegion := regionByID(t, compiled.Params.Regions, "tomb")
	require.Len(t, tombRegion.PlacedObstacles, 5) // coffin, altar, statue-reaper, brazier x2
	assert.Equal(t, "dnd5e:props:coffin", tombRegion.PlacedObstacles[0].Ref)
	assert.Equal(t, encounter.LocalHex{Col: 6, Row: 3}, tombRegion.PlacedObstacles[0].At)
	assert.False(t, tombRegion.PlacedObstacles[0].BlocksLoS)

	// BLOCKING-DEFAULT TRAP (advisory made explicit, same class as the
	// PATTERN-DEFAULT TRAP above): PlacedEntry.BlocksMovement/BlocksLoS are
	// *bool with nil => true (design.md: "same defaults/semantics as
	// ObstacleEntry"), but PlacedObstacleSpec.BlocksMovement/BlocksLoS
	// (engine-side, Task N1) are plain bool, whose Go zero value is FALSE.
	// A compiler that just dereferences a nil *bool panics; one that
	// silently treats nil as the zero value inverts the default (an
	// unflagged placed prop would compile to "blocks nothing," backwards
	// from the design's intent). The altar entry in placedTombYAML sets
	// NEITHER flag, so it's the direct test of this: it must compile to
	// BlocksMovement: true, BlocksLoS: true (matching the existing
	// crypt_dungeon.go precedent: altar is a structural piece, blocks both).
	altar := tombRegion.PlacedObstacles[1]
	assert.Equal(t, "dnd5e:props:altar", altar.Ref)
	assert.True(t, altar.BlocksMovement)
	assert.True(t, altar.BlocksLoS)

	var skeletonSpawn *dungeonspec.SpawnInstruction
	for i := range compiled.Spawns {
		if compiled.Spawns[i].MonsterRef == "dnd5e:monsters:skeleton" {
			skeletonSpawn = &compiled.Spawns[i]
		}
	}
	require.NotNil(t, skeletonSpawn)
	require.NotNil(t, skeletonSpawn.At)
	assert.Equal(t, encounter.LocalHex{Col: 4, Row: 2}, *skeletonSpawn.At)
	assert.Equal(t, 1, skeletonSpawn.Count)
}

func TestLoad_BossAtCompilesToSpawnPosition(t *testing.T) {
	compiled, err := dungeonspec.Load([]byte(placedTombYAML))
	require.NoError(t, err)
	require.NotEmpty(t, compiled.Spawns)
	boss := compiled.Spawns[0] // boss-first ordering unchanged
	assert.Equal(t, "dnd5e:monsters:skeleton-captain", boss.MonsterRef)
	require.NotNil(t, boss.At)
	assert.Equal(t, encounter.LocalHex{Col: 7, Row: 5}, *boss.At)
}

// TestLoad_SameBytesProduceIdenticalCompiledDungeon: Load has no source of
// randomness of its own (RandomSeed is left at its zero value -- entropy
// seeding happens later, at InitDungeon time, never inside the compiler) --
// calling it twice on the same bytes must yield byte-for-byte identical
// output, including the placed obstacles and positioned spawns from the
// place block, not just the top-level scalars.
func TestLoad_SameBytesProduceIdenticalCompiledDungeon(t *testing.T) {
	first, err := dungeonspec.Load([]byte(placedTombYAML))
	require.NoError(t, err)
	second, err := dungeonspec.Load([]byte(placedTombYAML))
	require.NoError(t, err)
	assert.Equal(t, first, second)
}
