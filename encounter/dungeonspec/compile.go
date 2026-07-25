// Copyright (C) 2024 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package dungeonspec

import (
	"fmt"

	"github.com/KirkDiggler/rpg-toolkit/encounter"
	"github.com/KirkDiggler/rpg-toolkit/encounter/core"
	"github.com/KirkDiggler/rpg-toolkit/tools/environments"
)

// SpawnInstruction is encounter's OWN type (defined in Task N2's
// encounter/seed_monsters.go, alongside SeedMonsters) -- dungeonspec
// re-exports it via a plain type ALIAS, not a second struct, so
// CompiledDungeon.Spawns can be passed directly to
// enc.SeedMonsters(compiled.Spawns) with zero conversion.
type SpawnInstruction = encounter.SpawnInstruction

// CompiledDungeon is a dungeon spec compiled down to what the engine and
// SeedMonsters actually consume: engine params ready for InitDungeon, and
// a spawn plan ready for SeedMonsters.
type CompiledDungeon struct {
	Params encounter.DungeonParams
	Spawns []SpawnInstruction // boss first, then entrance->boss chain order
}

// Load decodes, validates, and compiles a spec in one call -- the only
// entry point callers use. Door entity ids are COMPILER-GENERATED from
// content ("<key>-door-<from>-<to>" per connector), so callers supply
// nothing but the bytes.
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
		}
	}
	if bossRoom == nil {
		return CompiledDungeon{}, fmt.Errorf("compile: no boss room (Validate should have rejected this spec)")
	}

	// Boss spawn goes first, regardless of the boss room's position in the
	// chain (design.md, defense-in-depth ahead of Slice C's own invariant).
	bossAt := encounter.LocalHex{Col: bossRoom.Boss.At[0], Row: bossRoom.Boss.At[1]}
	spawns := []SpawnInstruction{{
		RoomID:     bossRoom.ID,
		MonsterRef: bossRoom.Boss.Ref,
		Count:      1,
		At:         &bossAt,
	}}

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
	region := encounter.DungeonRegionParams{
		ID:        room.ID,
		Archetype: encounter.RegionArchetype(room.Archetype),
		Width:     room.Width,
		Pattern:   compilePattern(room.Pattern),
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
			spawns = append(spawns, SpawnInstruction{
				RoomID:     room.ID,
				MonsterRef: entry.Ref,
				Count:      1,
				At:         &at,
			})
		default:
			return encounter.DungeonRegionParams{}, nil, fmt.Errorf("place %q: ref type %q must be props or monsters",
				entry.Ref, refType)
		}
	}

	return region, spawns, nil
}

// compilePattern maps the spec's author-facing pattern vocabulary onto the
// engine's own (design.md's PATTERN-DEFAULT TRAP: the engine treats an
// empty DungeonRegionParams.Pattern as PatternRandom, but the spec's
// default -- "" or "empty" -- means no interior walls at all, so the zero
// value must never pass through unmapped).
func compilePattern(pattern string) string {
	if pattern == patternScattered {
		return environments.PatternRandom
	}
	return environments.PatternEmpty
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
