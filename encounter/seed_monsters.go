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

	// AbsoluteAt is a canvas-mode placed spawn. It deliberately has no RoomID:
	// canvas has an explicit floor source rather than synthetic regions.
	// Exactly one of At and AbsoluteAt must be present for a placed spawn.
	AbsoluteAt *core.Hex
}

// resolvedSpawn is one SpawnInstruction after it has passed every
// validation check AND had its full MonsterInput derived — everything
// the commit pass needs to call addMonsterNoCombatCheck directly, with
// no remaining derivation or fallible step (monster construction,
// DataJSON marshaling, attack-snapshot extraction) left for commit time.
type resolvedSpawn struct {
	roomID string
	input  MonsterInput
}

// spawnBatch holds one SeedMonsters call's shared validation state: the
// room geometry (doorRow, wallCubes) and the evolving collision domain
// (claimed) that every instruction in the batch is checked against, in
// order. Kept as its own type (rather than local variables in
// validateSpawnBatch) so M2's Slice C count-based safe-cell roller has
// the same seam to extend — it needs this exact geometry/claimed shape
// to pick safe cells for At == nil instructions, not just to reject bad
// At-bearing ones.
type spawnBatch struct {
	doorRow   int
	wallCubes map[spatial.CubeCoordinate]struct{}
	claimed   map[core.Hex]string
}

// newSpawnBatch seeds a spawnBatch's collision domain from the
// encounter's CURRENT players, monsters, and blocking obstacles, so a
// spawn can't land on any existing occupant either — not just on another
// spawn within this same batch. Non-blocking obstacles (candles,
// bone-pile, chain — data.go's ObstacleData.BlocksMovement doc) are
// walkable-past floor dressing, so they don't reserve a cell here,
// matching room-rebuild's own walkability semantics (rebuildRoomFromData
// only ever rejects a BlocksMovement entity's cell as occupied).
func newSpawnBatch(e *Encounter, spawnCountHint int) *spawnBatch {
	b := &spawnBatch{
		doorRow:   e.data.Space.Height / 2,
		wallCubes: make(map[spatial.CubeCoordinate]struct{}, len(e.data.Space.Walls)),
		claimed: make(map[core.Hex]string,
			spawnCountHint+len(e.data.Players)+len(e.data.Monsters)+len(e.data.Space.Obstacles)),
	}
	for _, w := range e.data.Space.Walls {
		if w.Start != w.End {
			continue // boundary-edge perimeter segment (rpg-toolkit#834), not a blocking wall cell
		}
		b.wallCubes[w.Start] = struct{}{}
	}
	for id, p := range e.data.Players {
		if p.View == nil {
			continue // no persisted position to check against (shouldn't happen for a real seat, but be defensive)
		}
		b.claimed[p.View.Position] = fmt.Sprintf("player %q", id)
	}
	for id, m := range e.data.Monsters {
		b.claimed[m.Position] = fmt.Sprintf("monster %q", id)
	}
	for _, o := range e.data.Space.Obstacles {
		if !o.BlocksMovement {
			continue
		}
		b.claimed[o.Position] = fmt.Sprintf("obstacle %q", o.ID)
	}
	return b
}

// isWall reports whether position is a blocking wall cell.
func (b *spawnBatch) isWall(position core.Hex) bool {
	_, wall := b.wallCubes[position.ToCube()]
	return wall
}

// claim reports the existing occupant's descriptor if position is
// already taken; otherwise it reserves position for descriptor and
// reports ("", false).
func (b *spawnBatch) claim(position core.Hex, descriptor string) (occupant string, alreadyTaken bool) {
	if occ, taken := b.claimed[position]; taken {
		return occ, true
	}
	b.claimed[position] = descriptor
	return "", false
}

// SeedMonsters places a compiled spawn plan (e.g.
// dungeonspec.CompiledDungeon.Spawns) into an already-InitDungeon'd
// encounter under two invariants:
//
//  1. All-or-nothing batch validation: EVERY instruction is validated —
//     At non-nil, Count==1, MonsterRef resolves, RoomID names a real
//     region, the cell is in-bounds/not-doorRow/not-a-wall-cell/not
//     already claimed by this batch or an existing player/monster/
//     blocking obstacle, and its minted ID isn't already in the
//     encounter — and has its full MonsterInput derived (monster
//     construction, DataJSON, attack snapshot), BEFORE any monster is
//     added. The ONE honest exception to "no partial commit": once
//     validation passes, the only way a commit-phase call can still fail
//     is addMonsterNoCombatCheck's own EntityAppearedEvent broker
//     publish (an I/O-shaped failure, not a spec error) — every
//     REALISTIC failure mode (bad ref, bad room, OOB, collision, wrong
//     Count, duplicate ID) is caught in validation and can never reach
//     commit. This mirrors placeVerbatimObstacles' own contract one file
//     over (dungeon.go).
//  2. Atomic combat-entry evaluation: every validated spawn is staged
//     with combat-entry evaluation suppressed (addMonsterNoCombatCheck),
//     and exactly ONE checkCombatEntry pass runs after the whole batch
//     commits (including when spawns is empty — SeedMonsters(nil) still
//     runs this pass; harmless and intended, not worth special-casing
//     away). This matters because AddMonster's own per-call
//     checkCombatEntry would otherwise let a monster added mid-batch
//     while combat is already running (triggered by an EARLIER spawn's
//     own visibility) join initiative unconditionally, regardless of
//     that later monster's own visibility (see combat_entry_test.go's
//     TestIdempotent_AddMonsterAndMoveAfterCombatStarted) — the
//     partial-roster bug this invariant exists to prevent for spawn
//     orders (boss-first, per the compiler) where a monster is added
//     AFTER an earlier one's own visibility already flipped the mode.
//
// Caller ordering (Slice E and any other host): players must be added
// (AddPlayer) BEFORE calling SeedMonsters — combat-entry evaluation only
// ever sees whoever is already in e.data.Players at call time, the same
// as any other checkCombatEntry-driven check in this package.
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
// IDs this call already assigned — validated and rejected outright
// rather than silently colliding, but M2's Slice C, whichever caller
// ends up needing repeat calls, must carry the counters forward or use a
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
		if err := e.addMonsterNoCombatCheck(r.input); err != nil {
			return fmt.Errorf("room %q: monster %q: %w", r.roomID, r.input.MonsterRef, err)
		}
	}
	// No trigger entity (rpg-toolkit#865): a batch spawn has no acting player
	// to privilege into initiative slot 0, so this stays plain roll order.
	return e.checkCombatEntry("", 0)
}

// validateSpawnBatch checks every instruction in spawns and resolves it
// to a resolvedSpawn — including constructing the monster, marshaling
// its DataJSON, and extracting its attack snapshot — WITHOUT mutating
// the encounter. SeedMonsters only starts committing once every
// instruction has passed here, so a single bad instruction anywhere in
// the batch never leaves an earlier one committed.
func (e *Encounter) validateSpawnBatch(spawns []SpawnInstruction) ([]resolvedSpawn, error) {
	batch := newSpawnBatch(e, len(spawns))

	roomCounts := make(map[string]int, len(spawns))
	canvasCount := 0
	resolved := make([]resolvedSpawn, 0, len(spawns))
	for _, spawn := range spawns {
		if spawn.AbsoluteAt != nil {
			resolvedSpawn, err := e.validateAbsoluteCanvasSpawn(batch, spawn, canvasCount)
			if err != nil {
				return nil, err
			}
			canvasCount++
			resolved = append(resolved, resolvedSpawn)
			continue
		}
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
			return nil, fmt.Errorf("room %q: monster %q: unknown monster ref", spawn.RoomID, spawn.MonsterRef)
		}
		region, err := e.regionByID(spawn.RoomID)
		if err != nil {
			return nil, fmt.Errorf("room %q: monster %q: %w", spawn.RoomID, spawn.MonsterRef, err)
		}
		offsetX := regionOffsetX(region.Hexes)
		position := core.HexFromPosition(spatial.Position{
			X: float64(offsetX + spawn.At.Col), Y: float64(spawn.At.Row),
		})
		if !region.Hexes.Has(position) {
			return nil, fmt.Errorf("room %q: monster %q: at %v is out of bounds for this region (width=%d, height=%d)",
				spawn.RoomID, spawn.MonsterRef, spawn.At, regionWidth(region.Hexes), e.data.Space.Height)
		}
		if spawn.At.Row == batch.doorRow {
			return nil, fmt.Errorf("room %q: monster %q: at %v is on the reserved row (doorRow=%d)",
				spawn.RoomID, spawn.MonsterRef, spawn.At, batch.doorRow)
		}
		if batch.isWall(position) {
			return nil, fmt.Errorf("room %q: monster %q: at %v is on a wall cell",
				spawn.RoomID, spawn.MonsterRef, spawn.At)
		}

		// Minted here, not just at commit time: the whole point of
		// validating the ENTIRE batch before committing any of it is that
		// an ID collision — e.g. a second SeedMonsters call re-minting
		// "monster-entrance-0" against an encounter that already has one
		// from a prior call — must be caught here too, not discovered
		// mid-commit by addMonsterNoCombatCheck's own "already in
		// encounter" check after earlier spawns in THIS batch already
		// committed.
		id := core.EntityID(fmt.Sprintf("monster-%s-%d", spawn.RoomID, roomCounts[spawn.RoomID]))
		roomCounts[spawn.RoomID]++
		if _, exists := e.data.Monsters[id]; exists {
			return nil, fmt.Errorf("room %q: monster %q: minted id %q already in encounter",
				spawn.RoomID, spawn.MonsterRef, id)
		}

		if occupant, taken := batch.claim(position, fmt.Sprintf("monster %q", id)); taken {
			return nil, fmt.Errorf("room %q: monster %q: at %v collides with %s",
				spawn.RoomID, spawn.MonsterRef, spawn.At, occupant)
		}

		mon := ctor(string(id))
		dataJSON, err := json.Marshal(mon.ToData())
		if err != nil {
			return nil, fmt.Errorf("room %q: monster %q: marshal monster data: %w", spawn.RoomID, spawn.MonsterRef, err)
		}
		attackBonus, damageDice, damageType := primaryAttackSnapshot(mon)

		resolved = append(resolved, resolvedSpawn{
			roomID: spawn.RoomID,
			input: MonsterInput{
				ID:          id,
				Position:    position,
				HP:          mon.HP(),
				MaxHP:       mon.MaxHP(),
				AC:          mon.AC(),
				Speed:       mon.Speed().Walk / 5, // feet -> hexes (npc.go/action.go consume Speed in hexes)
				MonsterRef:  spawn.MonsterRef,
				DataJSON:    dataJSON,
				AttackBonus: attackBonus,
				DamageDice:  damageDice,
				DamageType:  damageType,
			},
		})
	}
	return resolved, nil
}

// regionByID finds a region by its ID (RegionData.ID) in the encounter's
// current Space — the lookup direction SpaceData.RegionAt doesn't
// provide (that's hex -> region id; this is region id -> RegionData).
// Its only caller (validateSpawnBatch, via SeedMonsters) already checked
// e.data.Space != nil before this can ever be reached, so there's no
// nil-Space branch here; callers wrap the returned error with their own
// room/monster context.
// validateAbsoluteCanvasSpawn resolves one absolute canvas instruction without
// inventing a region name or applying room-chain door-row rules.
func (e *Encounter) validateAbsoluteCanvasSpawn(
	batch *spawnBatch, spawn SpawnInstruction, index int,
) (resolvedSpawn, error) {
	if spawn.At != nil || spawn.RoomID != "" {
		return resolvedSpawn{}, fmt.Errorf(
			"canvas monster %q: absolute spawn must not carry room-local coordinates or room id", spawn.MonsterRef)
	}
	if spawn.Count != 1 {
		return resolvedSpawn{}, fmt.Errorf(
			"canvas monster %q: placed instructions must have Count 1, got %d", spawn.MonsterRef, spawn.Count)
	}
	if e.data.Space == nil || e.data.Space.Canvas == nil {
		return resolvedSpawn{}, fmt.Errorf("canvas monster %q: no canvas dungeon initialized", spawn.MonsterRef)
	}
	if !canvasFloorContainsHex(e.data.Space.Canvas, *spawn.AbsoluteAt) {
		return resolvedSpawn{}, fmt.Errorf(
			"canvas monster %q: absolute position %v is outside canvas floor", spawn.MonsterRef, *spawn.AbsoluteAt)
	}
	if batch.isWall(*spawn.AbsoluteAt) {
		return resolvedSpawn{}, fmt.Errorf(
			"canvas monster %q: absolute position %v is on a wall cell", spawn.MonsterRef, *spawn.AbsoluteAt)
	}
	ctor, ok := monsters.ByRef(spawn.MonsterRef)
	if !ok {
		return resolvedSpawn{}, fmt.Errorf("canvas monster %q: unknown monster ref", spawn.MonsterRef)
	}
	id := core.EntityID(fmt.Sprintf("monster-canvas-%d", index))
	if _, exists := e.data.Monsters[id]; exists {
		return resolvedSpawn{}, fmt.Errorf("canvas monster %q: minted id %q already in encounter", spawn.MonsterRef, id)
	}
	if occupant, taken := batch.claim(*spawn.AbsoluteAt, fmt.Sprintf("monster %q", id)); taken {
		return resolvedSpawn{}, fmt.Errorf(
			"canvas monster %q: absolute position %v collides with %s", spawn.MonsterRef, *spawn.AbsoluteAt, occupant)
	}
	mon := ctor(string(id))
	dataJSON, err := json.Marshal(mon.ToData())
	if err != nil {
		return resolvedSpawn{}, fmt.Errorf("canvas monster %q: marshal monster data: %w", spawn.MonsterRef, err)
	}
	attackBonus, damageDice, damageType := primaryAttackSnapshot(mon)
	return resolvedSpawn{
		roomID: "canvas",
		input: MonsterInput{
			ID: id, Position: *spawn.AbsoluteAt, HP: mon.HP(), MaxHP: mon.MaxHP(), AC: mon.AC(),
			Speed: mon.Speed().Walk / 5, MonsterRef: spawn.MonsterRef, DataJSON: dataJSON,
			AttackBonus: attackBonus, DamageDice: damageDice, DamageType: damageType,
		},
	}, nil
}

func (e *Encounter) regionByID(id string) (*RegionData, error) {
	for i := range e.data.Space.Regions {
		if e.data.Space.Regions[i].ID == id {
			return &e.data.Space.Regions[i], nil
		}
	}
	return nil, errors.New("no such region")
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

// regionWidth recovers a region's Width the same way regionOffsetX
// recovers its offsetX: RegionData carries no Width field, but Hexes is
// the region's full local rectangle (regionCubes), so max absolute X -
// min absolute X + 1 IS the width. Used only for an author-facing
// out-of-bounds error detail — not on any hot path.
func regionWidth(hexes core.HexSet) int {
	minX, maxX := 0, 0
	first := true
	for h := range hexes {
		x := int(math.Round(h.ToPosition().X))
		if first || x < minX {
			minX = x
		}
		if first || x > maxX {
			maxX = x
		}
		first = false
	}
	return maxX - minX + 1
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
// correctly, which is what OA readiness actually needs. A broader gap —
// a ranged-only monster's OA readiness assumes a melee-shaped attack —
// is tracked separately as rpg-toolkit#844 (a typed attack-snapshot
// accessor belongs in rulebooks/dnd5e, not patched here).
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
