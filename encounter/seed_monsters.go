// Copyright (C) 2024 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package encounter

import (
	"encoding/json"
	"errors"
	"fmt"
	"math"

	"github.com/KirkDiggler/rpg-toolkit/encounter/core"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/monster"
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
	// instructions, which SeedMonsters requires to have Count exactly 1
	// (a placed instruction pins ONE instance to ONE cell — Count>1 with
	// At set is rejected outright, never silently truncated) — see At's
	// doc.
	Count int

	// At is this spawn's room-local cell: nil means rolled (count-based,
	// safe-cell selection — M2 only, SeedMonsters rejects it in M1);
	// non-nil means placed (M1) at this exact cell, translated by the
	// target region's offsetX exactly like Task N1's PlacedObstacleSpec.
	At *LocalHex
}

// resolvedSpawn is one SpawnInstruction after it has passed every
// validation check in SeedMonsters' first pass — everything the commit
// pass needs, so committing never has to re-derive or re-check anything
// the validation pass already established.
type resolvedSpawn struct {
	id         core.EntityID
	roomID     string
	monsterRef string
	position   core.Hex
	ctor       monsters.Constructor
}

// SeedMonsters places a compiled spawn plan (e.g.
// dungeonspec.CompiledDungeon.Spawns) into an already-InitDungeon'd
// encounter under two invariants:
//
//  1. All-or-nothing batch validation: EVERY instruction is validated
//     (At non-nil, Count==1, MonsterRef resolves, RoomID names a real
//     region, the cell is in-bounds/not-doorRow/not-a-wall-cell/not
//     already claimed by this batch or an existing occupant) BEFORE any
//     monster is added. A single bad instruction anywhere in the batch
//     leaves the encounter exactly as it was before the call — no
//     partial commit, no monsters added, mode unchanged — mirroring
//     placeVerbatimObstacles' own contract one file over (dungeon.go)
//     and closing the class of bug where an early instruction commits
//     and only a LATER one fails.
//  2. Atomic combat-entry evaluation: every validated spawn is staged
//     with combat-entry evaluation suppressed (addMonsterNoCombatCheck),
//     and exactly ONE checkCombatEntry pass runs after the whole batch
//     commits. This matters because AddMonster's own per-call
//     checkCombatEntry would otherwise let a monster added mid-batch
//     while combat is already running (triggered by an EARLIER spawn's
//     own visibility) join initiative unconditionally, regardless of
//     that later monster's own visibility (see combat_entry_test.go's
//     TestIdempotent_AddMonsterAndMoveAfterCombatStarted) — the
//     partial-roster bug this invariant exists to prevent for spawn
//     orders (boss-first, per the compiler) where a monster is added
//     AFTER an earlier one's own visibility already flipped the mode.
//
// M1 scope (this plan's Task N2 scoping call): only At-bearing (placed)
// instructions are supported — an At == nil (rolled, count-based)
// instruction returns the scope-boundary error below regardless of
// Count. M2's Slice C extends this same function with safe-cell rolling
// for At == nil instructions. dungeonspec's own Validate (Task B2)
// already rejects any M1 spec that would produce one, so this is
// unreachable via the content-hosting path today — this check is
// defense-in-depth for SeedMonsters' other, non-dungeonspec callers.
//
// ID numbering ("monster-<roomID>-<n>", per-room, 0-based) is scoped to
// ONE SeedMonsters call: M1 only ever calls this once per encounter (at
// initial dungeon setup), so this is never an issue today. A second call
// against the SAME encounter (M2 territory, e.g. re-seeding after a
// pocket clears) would restart each room's counter at 0 and collide with
// IDs this call already assigned — M2's Slice C, whichever caller ends
// up needing repeat calls, must carry the counters forward or use a
// different scheme; not needed for M1's single-call acceptance path.
func (e *Encounter) SeedMonsters(spawns []SpawnInstruction) error {
	if e.data.Space == nil {
		return errors.New("seed monsters: no space initialized (call InitDungeon first)")
	}

	resolved, err := e.validateSpawnBatch(spawns)
	if err != nil {
		return err
	}

	for _, r := range resolved {
		mon := r.ctor(string(r.id))
		dataJSON, err := json.Marshal(mon.ToData())
		if err != nil {
			return fmt.Errorf("room %q: monster %q: marshal monster data: %w", r.roomID, r.monsterRef, err)
		}
		attackBonus, damageDice, damageType := primaryAttackSnapshot(mon)

		if err := e.addMonsterNoCombatCheck(MonsterInput{
			ID:          r.id,
			Position:    r.position,
			HP:          mon.HP(),
			MaxHP:       mon.MaxHP(),
			AC:          mon.AC(),
			Speed:       mon.Speed().Walk / 5, // feet -> hexes (npc.go/action.go consume Speed in hexes)
			MonsterRef:  r.monsterRef,
			DataJSON:    dataJSON,
			AttackBonus: attackBonus,
			DamageDice:  damageDice,
			DamageType:  damageType,
		}); err != nil {
			return fmt.Errorf("room %q: monster %q: %w", r.roomID, r.monsterRef, err)
		}
	}
	return e.checkCombatEntry()
}

// validateSpawnBatch checks every instruction in spawns and resolves it
// to a resolvedSpawn, WITHOUT mutating the encounter — SeedMonsters only
// starts committing once every instruction has passed here, so a single
// bad instruction anywhere in the batch never leaves an earlier one
// committed.
func (e *Encounter) validateSpawnBatch(spawns []SpawnInstruction) ([]resolvedSpawn, error) {
	doorRow := e.data.Space.Height / 2
	wallCubes := make(map[spatial.CubeCoordinate]struct{}, len(e.data.Space.Walls))
	for _, w := range e.data.Space.Walls {
		if w.Start != w.End {
			continue // boundary-edge perimeter segment (rpg-toolkit#834), not a blocking wall cell
		}
		wallCubes[w.Start] = struct{}{}
	}

	// Seeded from any monsters already in the encounter, so a spawn can't
	// land on an existing occupant either — not just on another spawn
	// within this same batch.
	claimed := make(map[core.Hex]bool, len(spawns)+len(e.data.Monsters))
	for _, m := range e.data.Monsters {
		claimed[m.Position] = true
	}

	roomCounts := make(map[string]int, len(spawns))
	resolved := make([]resolvedSpawn, 0, len(spawns))
	for _, spawn := range spawns {
		if spawn.At == nil {
			return nil, fmt.Errorf("room %q: monster %q: rolled (count-based) monster placement lands in M2",
				spawn.RoomID, spawn.MonsterRef)
		}
		if spawn.Count != 1 {
			return nil, fmt.Errorf("room %q: monster %q: placed (At-bearing) instructions must have Count 1, got %d",
				spawn.RoomID, spawn.MonsterRef, spawn.Count)
		}
		ctor, ok := monsters.ByRef(spawn.MonsterRef)
		if !ok {
			return nil, fmt.Errorf("room %q: unknown monster ref %q", spawn.RoomID, spawn.MonsterRef)
		}
		region, err := e.regionByID(spawn.RoomID)
		if err != nil {
			return nil, err
		}
		offsetX := regionOffsetX(region.Hexes)
		position := core.HexFromPosition(spatial.Position{
			X: float64(offsetX + spawn.At.Col), Y: float64(spawn.At.Row),
		})
		if !region.Hexes.Has(position) {
			return nil, fmt.Errorf("room %q: monster %q: at %v is out of bounds for this region",
				spawn.RoomID, spawn.MonsterRef, spawn.At)
		}
		if spawn.At.Row == doorRow {
			return nil, fmt.Errorf("room %q: monster %q: at %v is on the reserved row (doorRow=%d)",
				spawn.RoomID, spawn.MonsterRef, spawn.At, doorRow)
		}
		if _, wall := wallCubes[position.ToCube()]; wall {
			return nil, fmt.Errorf("room %q: monster %q: at %v is on a wall cell",
				spawn.RoomID, spawn.MonsterRef, spawn.At)
		}
		if claimed[position] {
			return nil, fmt.Errorf("room %q: monster %q: at %v collides with another monster",
				spawn.RoomID, spawn.MonsterRef, spawn.At)
		}
		claimed[position] = true

		id := core.EntityID(fmt.Sprintf("monster-%s-%d", spawn.RoomID, roomCounts[spawn.RoomID]))
		roomCounts[spawn.RoomID]++

		resolved = append(resolved, resolvedSpawn{
			id: id, roomID: spawn.RoomID, monsterRef: spawn.MonsterRef, position: position, ctor: ctor,
		})
	}
	return resolved, nil
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

// attackSnapshot mirrors the JSON shape shared by every damage-dealing
// MonsterAction's Config (MeleeConfig, RangedConfig, BiteConfig —
// monster/actions/*.go): attack_bonus/damage_dice/damage_type. Decoding
// into this one small struct rather than each action's own concrete
// Config type means no type switch on the action's Ref is needed —
// Multiattack (whose Config only ever carries an Attacks []string) and
// any non-damage action (e.g. Nimble Escape) simply decode to zero
// values here and get skipped by primaryAttackSnapshot below.
type attackSnapshot struct {
	AttackBonus int    `json:"attack_bonus"`
	DamageDice  string `json:"damage_dice"`
	DamageType  string `json:"damage_type"`
}

// primaryAttackSnapshot extracts a flat combat-snapshot (AttackBonus,
// DamageDice, DamageType) from the first of a monster's actions that
// actually carries one — MonsterInput's own flat-field shape,
// combat_resolver.go's documented "stat-snapshot stand-in path" for
// callers with no rehydratable DataJSON.
//
// This is NOT redundant with DataJSON. Real combat resolution runs
// through the resolved monster's DataJSON-hydrated Attacker/Defender
// regardless of these fields (combat_resolver.go's own doc) — but
// encounter.go's Opportunity Attack readiness seeding
// (addMonsterNoCombatCheck: "if input.DamageDice != \"\" { seedOAReadiness }")
// gates ONLY on this flat DamageDice field, and never consults DataJSON.
// Leaving it empty (an earlier version of this function did, reasoning
// DataJSON alone was sufficient) silently starves every dungeon-seeded
// monster's OA reaction of readiness forever.
//
// One known, accepted gap: the goblin's hardcoded ScimitarConfig
// (monster/scimitar_action.go, predating the generic actions.MeleeAction)
// never serializes a damage type at all (damage_bonus instead) — its
// DamageType comes back empty here; AttackBonus/DamageDice still populate
// correctly, which is what OA readiness actually needs.
func primaryAttackSnapshot(mon *monster.Monster) (attackBonus int, damageDice, damageType string) {
	for _, action := range mon.Actions() {
		var snap attackSnapshot
		if err := json.Unmarshal(action.ToData().Config, &snap); err != nil {
			continue
		}
		if snap.DamageDice == "" {
			continue
		}
		return snap.AttackBonus, snap.DamageDice, snap.DamageType
	}
	return 0, "", ""
}
