// Copyright (C) 2024 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package dungeonspec

import (
	"fmt"

	"github.com/KirkDiggler/rpg-toolkit/encounter"
	"github.com/KirkDiggler/rpg-toolkit/encounter/core"
	"github.com/KirkDiggler/rpg-toolkit/tools/environments"
)

// SpawnInstruction is encounter's OWN type (defined in
// encounter/seed_monsters.go, alongside SeedMonsters -- rpg-toolkit#842)
// -- dungeonspec re-exports it via a plain type ALIAS, not a second
// struct, so CompiledDungeon.Spawns can be passed directly to
// enc.SeedMonsters(compiled.Spawns) with zero conversion.
type SpawnInstruction = encounter.SpawnInstruction

// CompiledDungeon is a dungeon spec compiled down to what the engine and
// SeedMonsters actually consume: engine params ready for InitDungeon, and
// a spawn plan ready for SeedMonsters. It is a plain value (no internal
// state, no mutable shared pointers into the source spec) -- safe to
// cache and share across concurrent requests, since the engine only ever
// reads Params/Spawns off of it.
type CompiledDungeon struct {
	// Params feeds Encounter.InitDungeon directly. Params.RandomSeed is
	// deliberately left at its zero value -- Load has no opinion on
	// reproducibility, so the caller MUST set it before calling
	// InitDungeon, or repeated encounters compiled from the same spec
	// bytes won't roll the same layout.
	Params encounter.DungeonParams
	Spawns []SpawnInstruction // boss first, then entrance->boss chain order
}

// Load decodes, validates, and compiles a spec in one call -- the only
// entry point callers use. Door entity ids are COMPILER-GENERATED from
// content ("<key>-door-<from>-<to>" per connector), so callers supply
// nothing but the bytes.
//
// Expected call sequence once Load returns: enc.InitDungeon(compiled.Params)
// (after setting Params.RandomSeed, see CompiledDungeon.Params), then
// enc.AddPlayer for each player, then enc.SeedMonsters(compiled.Spawns) --
// in that order. SeedMonsters' combat-entry check only evaluates players
// already added; calling it before every AddPlayer silently skips combat
// entry for that seeding pass rather than erroring, so an author-placed
// monster in a party's starting sightline needs the party added first.
func Load(raw []byte) (CompiledDungeon, error) {
	spec, err := Decode(raw)
	if err != nil {
		return CompiledDungeon{}, err
	}
	if err := Validate(spec); err != nil {
		return CompiledDungeon{}, err
	}
	return compile(spec)
}

// compile assumes spec has already passed Validate -- every ref shape,
// cell bound, and boss-cardinality invariant it relies on is guaranteed by
// that pass. It still returns an error rather than panicking, matching
// this package's style elsewhere, in case that assumption is ever violated
// by a future caller.
func compile(spec *DungeonSpec) (CompiledDungeon, error) {
	var bossRoom *RoomSpec
	for i := range spec.Rooms {
		if spec.Rooms[i].Boss != nil {
			bossRoom = &spec.Rooms[i]
			break // validateBossCardinality already guarantees exactly one
		}
	}
	if bossRoom == nil {
		return CompiledDungeon{}, fmt.Errorf("dungeon %q: no boss room (Validate should have rejected this spec)", spec.Key)
	}

	// Boss spawn goes first, regardless of the boss room's position in the
	// chain (design.md, defense-in-depth ahead of Slice C's own invariant).
	//
	// bossRoom.Boss.At is nil only for an unpinned boss -- Validate's M1-only
	// restriction (Task B2) rejects that shape today, so Load can never
	// reach this branch; it stays here so compile() never panics regardless
	// (this function's own doc), and it's exactly where M2's Task C0 lands:
	// an unpinned boss becomes a rolled (Count-based, no At) spawn candidate,
	// which is precisely what Count:1, At:nil already expresses.
	var bossAt *encounter.LocalHex
	if bossRoom.Boss.At != nil {
		at := encounter.LocalHex{Col: bossRoom.Boss.At[0], Row: bossRoom.Boss.At[1]}
		bossAt = &at
	}
	// Capacity is a hint, not a bound: sized for the boss spawn plus one
	// per room, but a room's place list can name several monster entries
	// (growing past the hint) or none at all (falling short of it) --
	// neither direction is guaranteed.
	spawns := make([]SpawnInstruction, 0, len(spec.Rooms)+1)
	spawns = append(spawns, SpawnInstruction{
		RoomID:     bossRoom.ID,
		MonsterRef: bossRoom.Boss.Ref,
		Count:      1,
		At:         bossAt,
	})

	regions := make([]encounter.DungeonRegionParams, len(spec.Rooms))
	for i := range spec.Rooms {
		room := &spec.Rooms[i]
		region, roomSpawns, err := compileRoom(room)
		if err != nil {
			return CompiledDungeon{}, fmt.Errorf("room %q: %w", room.ID, err)
		}
		regions[i] = region
		spawns = append(spawns, roomSpawns...)
	}

	connectors := make([]encounter.DungeonConnectorParams, len(spec.Connectors))
	for i := range spec.Connectors {
		connectors[i] = compileConnector(spec.Key, &spec.Connectors[i])
	}

	return CompiledDungeon{
		Params: encounter.DungeonParams{
			Regions:    regions,
			Connectors: connectors,
			Height:     spec.Height,
			Theme:      spec.Theme,
		},
		Spawns: spawns,
	}, nil
}

// compileRoom compiles one room's geometry/obstacles/place block. It
// returns the room's placed monster spawns separately (never the boss
// spawn, which compile builds directly from room.Boss) so compile can
// prepend the boss spawn once, ahead of every room's placed spawns, rather
// than every call site re-deriving "is this room the boss room."
func compileRoom(room *RoomSpec) (encounter.DungeonRegionParams, []SpawnInstruction, error) {
	// room.Monsters is unreachable via Load today -- Validate's M1-only
	// restriction (validateM1Restrictions) rejects any non-empty
	// room.Monsters -- but compileRoom does not yet know how to compile
	// count-based rolled monsters (M2's Slice C, count-based compiling --
	// see plan), so it errors loudly rather than silently dropping them if
	// that assumption is ever violated by a future caller, mirroring
	// compile()'s own nil-boss.At guard.
	if len(room.Monsters) > 0 {
		return encounter.DungeonRegionParams{}, nil,
			fmt.Errorf("rolled monsters not compiled yet (Validate should have rejected this spec)")
	}

	region := encounter.DungeonRegionParams{
		ID:        room.ID,
		Archetype: encounter.RegionArchetype(room.Archetype),
		Width:     room.Width,
		Pattern:   compilePattern(room.Pattern),
	}

	// The boss room's own pinned boss.at cell is a MONSTER placement exactly
	// like a place: monster entry below -- compile() builds its
	// SpawnInstruction directly from room.Boss, never through this room's
	// Place loop, so without reserving it here separately, InitDungeon's
	// rolled-obstacle draw has no idea that cell is spoken for (rpg-toolkit#842
	// gate finding: see encounter.DungeonRegionParams.ReservedCells' doc).
	if room.Boss != nil && room.Boss.At != nil {
		region.ReservedCells = append(region.ReservedCells,
			encounter.LocalHex{Col: room.Boss.At[0], Row: room.Boss.At[1]})
	}

	if len(room.Obstacles) > 0 {
		region.Obstacles = make([]encounter.ObstacleSpec, 0, len(room.Obstacles))
	}
	for _, o := range room.Obstacles {
		region.Obstacles = append(region.Obstacles, encounter.ObstacleSpec{
			Ref:            o.Ref,
			Count:          o.Count,
			BlocksMovement: boolOrTrue(o.BlocksMovement),
			BlocksLoS:      boolOrTrue(o.BlocksLoS),
			PreferBorder:   o.PreferBorder,
		})
	}

	if len(room.Place) > 0 {
		// Capacity is an upper bound, not exact: place entries split between
		// props (appended below to PlacedObstacles) and monsters (appended
		// to spawns), so a place block that's all monsters never grows
		// PlacedObstacles at all.
		region.PlacedObstacles = make([]encounter.PlacedObstacleSpec, 0, len(room.Place))
	}
	var spawns []SpawnInstruction
	for _, entry := range room.Place {
		refType, err := refParts(entry.Ref)
		if err != nil {
			return encounter.DungeonRegionParams{}, nil, fmt.Errorf("place %q: %w", entry.Ref, err)
		}
		switch refType {
		case refTypeProps:
			region.PlacedObstacles = append(region.PlacedObstacles, encounter.PlacedObstacleSpec{
				Ref:            entry.Ref,
				At:             encounter.LocalHex{Col: entry.At[0], Row: entry.At[1]},
				BlocksMovement: boolOrTrue(entry.BlocksMovement),
				BlocksLoS:      boolOrTrue(entry.BlocksLoS),
			})
		case refTypeMonsters:
			at := encounter.LocalHex{Col: entry.At[0], Row: entry.At[1]}
			// Reserve this cell the same way as the boss's own pinned
			// boss.at above -- a placed monster never becomes a
			// PlacedObstacleSpec, so without this InitDungeon's
			// rolled-obstacle draw would have no idea the cell is taken
			// (rpg-toolkit#842 gate finding: see ReservedCells' doc).
			region.ReservedCells = append(region.ReservedCells, at)
			spawns = append(spawns, SpawnInstruction{
				RoomID:     room.ID,
				MonsterRef: entry.Ref,
				Count:      1,
				At:         &at,
			})
		default:
			// Phrasing matches validatePlaceBlock's "must be props or
			// monsters" message (validate.go) -- this branch is unreachable
			// via Load (Validate already rejects it), but the two should
			// read as the same rule stated twice, not two different ones.
			return encounter.DungeonRegionParams{}, nil, fmt.Errorf("place ref %q must be props or monsters, got type %q",
				entry.Ref, refType)
		}
	}

	return region, spawns, nil
}

// compilePattern maps the spec's author-facing pattern vocabulary
// (patternEmpty/patternScattered, validate.go) onto the engine's own.
//
// PATTERN-DEFAULT TRAP: the engine treats an empty DungeonRegionParams.
// Pattern as PatternRandom, but the spec's default -- "" or patternEmpty
// -- means no interior walls at all, so the zero value must never pass
// through unmapped. This is the one place that mapping happens; anything
// that wants to reason about it again should point back here rather than
// re-deriving it.
func compilePattern(pattern string) string {
	switch pattern {
	case patternScattered:
		return environments.PatternRandom
	default: // "", patternEmpty, and (Validate having already run) nothing else
		return environments.PatternEmpty
	}
}

// compileConnector generates this connector's door id from content
// ("<key>-door-<from>-<to>") and copies its lock config verbatim.
func compileConnector(key string, c *ConnectorSpec) encounter.DungeonConnectorParams {
	params := encounter.DungeonConnectorParams{
		DoorID: core.EntityID(fmt.Sprintf("%s-door-%s-%s", key, c.From, c.To)),
	}
	if c.Locked != nil {
		params.Locked = true
		params.LockDC = c.Locked.DC
		params.LockAbility = c.Locked.Ability
	}
	return params
}

// boolOrTrue returns *b, or true when b is nil -- the nil=>true blocking
// default shared by ObstacleEntry and PlacedEntry's BlocksMovement/
// BlocksLoS flags (design.md: an unflagged entry blocks both).
func boolOrTrue(b *bool) bool {
	if b == nil {
		return true
	}
	return *b
}
