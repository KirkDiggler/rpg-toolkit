// Copyright (C) 2024 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package encounter

import (
	"encoding/json"
	"fmt"
	"math"

	"github.com/KirkDiggler/rpg-toolkit/encounter/core"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/monster/monsters"
	"github.com/KirkDiggler/rpg-toolkit/tools/spatial"
)

// SpawnInstruction describes one monster spawn compiled from a dungeon
// spec — encounter-owned (not dungeonspec-owned; dungeonspec re-exports
// it via a plain type alias) so a host can call
// enc.SeedMonsters(compiled.Spawns) directly, with zero conversion.
type SpawnInstruction struct {
	// RoomID names the region (DungeonRegionParams.ID / RegionData.ID)
	// this spawn targets.
	RoomID string

	// MonsterRef is the canonical monster ref (e.g.
	// "dnd5e:monsters:skeleton") resolved via monsters.ByRef.
	MonsterRef string

	// Count is how many instances to spawn. M1 only supports At-bearing
	// instructions, which are always exactly one instance at a fixed
	// cell — see At's doc.
	Count int

	// At is this spawn's room-local cell: nil means rolled (count-based,
	// safe-cell selection — M2 only, SeedMonsters rejects it in M1);
	// non-nil means placed (M1) at this exact cell, translated by the
	// target region's offsetX exactly like Task N1's PlacedObstacleSpec.
	At *LocalHex
}

// SeedMonsters places a compiled spawn plan (e.g.
// dungeonspec.CompiledDungeon.Spawns) into an already-InitDungeon'd
// encounter, under the atomic combat-entry invariant: every spawn in the
// batch is staged with combat-entry evaluation suppressed
// (addMonsterNoCombatCheck), and exactly ONE checkCombatEntry pass runs
// after the whole batch commits. This matters because AddMonster's own
// per-call checkCombatEntry would otherwise let a monster added mid-batch
// while combat is already running (triggered by an EARLIER spawn's own
// visibility) join initiative unconditionally, regardless of that later
// monster's own visibility (see combat_entry_test.go's
// TestIdempotent_AddMonsterAndMoveAfterCombatStarted) — exactly the
// partial-roster bug class the invariant exists to prevent, for spawn
// orders (boss-first, per the compiler) where the boss is seeded before
// an already-visible entrance monster.
//
// M1 scope (this plan's Task N2 scoping call): only At-bearing (placed)
// instructions are supported — an At == nil (rolled, count-based)
// instruction returns the scope-boundary error below regardless of
// Count. M2's Slice C extends this same function with safe-cell rolling
// for At == nil instructions. dungeonspec's own Validate (Task B2)
// already rejects any M1 spec that would produce one, so this is
// unreachable via the content-hosting path today — this check is
// defense-in-depth for SeedMonsters' other, non-dungeonspec callers.
func (e *Encounter) SeedMonsters(spawns []SpawnInstruction) error {
	roomCounts := make(map[string]int, len(spawns))
	for _, spawn := range spawns {
		if spawn.At == nil {
			return fmt.Errorf("room %q: monster %q: rolled (count-based) monster placement lands in M2",
				spawn.RoomID, spawn.MonsterRef)
		}
		ctor, ok := monsters.ByRef(spawn.MonsterRef)
		if !ok {
			return fmt.Errorf("room %q: unknown monster ref %q", spawn.RoomID, spawn.MonsterRef)
		}
		region, err := e.regionByID(spawn.RoomID)
		if err != nil {
			return err
		}
		offsetX := regionOffsetX(region.Hexes)
		position := core.HexFromPosition(spatial.Position{
			X: float64(offsetX + spawn.At.Col), Y: float64(spawn.At.Row),
		})

		id := core.EntityID(fmt.Sprintf("monster-%s-%d", spawn.RoomID, roomCounts[spawn.RoomID]))
		roomCounts[spawn.RoomID]++

		mon := ctor(string(id))
		dataJSON, err := json.Marshal(mon.ToData())
		if err != nil {
			return fmt.Errorf("room %q: monster %q: marshal monster data: %w", spawn.RoomID, spawn.MonsterRef, err)
		}

		if err := e.addMonsterNoCombatCheck(MonsterInput{
			ID:         id,
			Position:   position,
			HP:         mon.HP(),
			MaxHP:      mon.MaxHP(),
			AC:         mon.AC(),
			Speed:      mon.Speed().Walk,
			MonsterRef: spawn.MonsterRef,
			DataJSON:   dataJSON,
		}); err != nil {
			return fmt.Errorf("room %q: monster %q: %w", spawn.RoomID, spawn.MonsterRef, err)
		}
	}
	return e.checkCombatEntry()
}

// regionByID finds a region by its ID (RegionData.ID) in the encounter's
// current Space — the lookup direction SpaceData.RegionAt doesn't
// provide (that's hex -> region id; this is region id -> RegionData).
func (e *Encounter) regionByID(id string) (*RegionData, error) {
	if e.data.Space == nil {
		return nil, fmt.Errorf("room %q: no space initialized (call InitDungeon first)", id)
	}
	for i := range e.data.Space.Regions {
		if e.data.Space.Regions[i].ID == id {
			return &e.data.Space.Regions[i], nil
		}
	}
	return nil, fmt.Errorf("room %q: no such region", id)
}

// regionOffsetX recovers a region's generateDungeonLayout offsetX
// (starts[i]) from its persisted RegionData.Hexes — InitDungeon discards
// DungeonParams once the layout is built, and RegionData carries no
// offset field (data.go), so this is the only source left. Every
// region's local x=0 column is a member of Hexes for every row (see
// regionCubes), so the minimum absolute X across the set IS offsetX.
func regionOffsetX(hexes core.HexSet) int {
	minX := 0
	first := true
	for h := range hexes {
		x := int(math.Round(h.ToPosition().X))
		if first || x < minX {
			minX = x
			first = false
		}
	}
	return minX
}
