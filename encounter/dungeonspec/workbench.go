// Copyright (C) 2024 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package dungeonspec

import (
	"context"
	"fmt"
	"strings"

	"github.com/KirkDiggler/rpg-toolkit/encounter"
	"github.com/KirkDiggler/rpg-toolkit/encounter/core"
)

// WorkbenchReport is the fast-authoring-loop entry point (design.md's
// observability addendum, rpg-toolkit#842): compile raw spec bytes at a
// given seed and describe the result as human-readable text, without a
// server. An author edits a YAML file, runs the workbench CLI
// (encounter/cmd/dungeonspec-workbench), and sees the layout in seconds.
//
// A spec that fails to Load (Decode or Validate) still returns a non-empty
// report -- naming the verdict INVALID and the error -- alongside the
// error itself, so a caller (the CLI) always has something to print. A
// spec that Loads successfully is then run through a throwaway
// Encounter.InitDungeon at seed so the report's floor plan reflects the
// SAME wall/door/obstacle generation a real encounter would produce at
// that seed -- not a re-derivation of the compiler's own output -- and
// then through SeedMonsters (no AddPlayer call first: this is a
// throwaway single-report encounter, not a live one, and SeedMonsters'
// combat-entry check is harmless with zero players seated), the third leg
// of the same pipeline Load's own doc names (InitDungeon -> AddPlayer ->
// SeedMonsters). Running it here is not optional: rpg-toolkit#842's gate
// finding was a spec that Loaded and InitDungeon'd cleanly could still
// fail at THIS step, on some fraction of seeds, because a placed
// monster's cell hadn't been reserved from the rolled-obstacle draw --
// without this call, the workbench would report VALID on a spec that
// SeedMonsters actually rejects, exactly the failure mode this tool
// exists to catch before an author ships a spec.
//
// seed 0 is entropy-seeded (matches DungeonParams.RandomSeed's own zero-
// value contract) -- pass a non-zero seed for a reproducible report.
func WorkbenchReport(raw []byte, seed int64) (string, error) {
	compiled, err := Load(raw)
	if err != nil {
		return fmt.Sprintf("verdict: INVALID\nseed: %d\nerror: %s\n", seed, err), err
	}
	compiled.Params.RandomSeed = seed

	transport := encounter.NewInMemoryTransport()
	broker := encounter.NewBroker(transport)
	enc := encounter.New(context.Background(), "workbench", broker)
	if err := enc.InitDungeon(compiled.Params); err != nil {
		return fmt.Sprintf("verdict: INVALID\nseed: %d\nerror: init dungeon: %s\n", seed, err), err
	}
	if err := enc.SeedMonsters(compiled.Spawns); err != nil {
		return fmt.Sprintf("verdict: INVALID\nseed: %d\nerror: seed monsters: %s\n", seed, err), err
	}

	var b strings.Builder
	fmt.Fprintf(&b, "verdict: VALID\nseed %d\n\n", seed)
	writeRegions(&b, compiled.Params.Regions)
	writeConnectors(&b, compiled.Params.Connectors)
	writeSpawnPlan(&b, compiled.Spawns)
	writePlacedObstacles(&b, compiled.Params.Regions)
	fmt.Fprintf(&b, "seeded %d monster(s)\n\n", len(enc.ToData().Monsters))
	writeFloorPlan(&b, enc.ToData())
	return b.String(), nil
}

// writeRegions lists every region's id/archetype/width -- the "region ids
// as a legend" the floor plan itself doesn't spell out per-cell.
func writeRegions(b *strings.Builder, regions []encounter.DungeonRegionParams) {
	fmt.Fprintln(b, "regions:")
	for _, r := range regions {
		fmt.Fprintf(b, "  %s [%s] width=%d\n", r.ID, r.Archetype, r.Width)
	}
	fmt.Fprintln(b)
}

// writeConnectors names each door in chain order, including lock state --
// an author checking "is this door supposed to be locked" shouldn't have
// to cross-reference the source YAML.
func writeConnectors(b *strings.Builder, connectors []encounter.DungeonConnectorParams) {
	fmt.Fprintln(b, "connectors:")
	for i, c := range connectors {
		lock := ""
		if c.Locked {
			lock = fmt.Sprintf(" locked dc=%d ability=%s", c.LockDC, c.LockAbility)
		}
		fmt.Fprintf(b, "  connector %d: door %s%s\n", i, c.DoorID, lock)
	}
	fmt.Fprintln(b)
}

// writeSpawnPlan renders the compiled spawn plan boss-first (compile()'s
// own ordering contract) -- rolled (At == nil, M2's Task C0) spawns print
// "rolled" in place of a coordinate rather than a misleading col=0 row=0.
func writeSpawnPlan(b *strings.Builder, spawns []SpawnInstruction) {
	fmt.Fprintln(b, "spawn plan (boss first):")
	for i, s := range spawns {
		role := ""
		if i == 0 {
			role = " (boss)"
		}
		at := "rolled"
		if s.At != nil {
			at = s.At.String()
		}
		fmt.Fprintf(b, "  %d. %s%s in %s at %s\n", i+1, s.MonsterRef, role, s.RoomID, at)
	}
	fmt.Fprintln(b)
}

// writePlacedObstacles lists every PlacedObstacleSpec at its exact
// room-local coordinate -- the delta addition's own requirement (design.md
// §Design delta): a placed prop must be nameable at its exact compiled
// cell, not just "somewhere in the room." Read directly off
// compiled.Params rather than re-derived from the encounter's absolute
// Space coordinates, since LocalHex is already the same room-local frame
// the author's own place: block used.
func writePlacedObstacles(b *strings.Builder, regions []encounter.DungeonRegionParams) {
	hasPlaced := false
	for _, r := range regions {
		if len(r.PlacedObstacles) > 0 {
			hasPlaced = true
			break
		}
	}
	if !hasPlaced {
		return // nothing placed in this spec -- an empty header/section would just be noise
	}
	fmt.Fprintln(b, "placed props (exact coordinates):")
	for _, r := range regions {
		for _, p := range r.PlacedObstacles {
			fmt.Fprintf(b, "  %s: %s at %s\n", r.ID, p.Ref, p.At.String())
		}
	}
	fmt.Fprintln(b)
}

// floorPlanLegend documents the ASCII floor plan's marker vocabulary,
// printed once ahead of the grid so the symbols are self-describing.
const floorPlanLegend = "floor plan (. floor, # wall, D door, o obstacle, @ entrance):"

// writeFloorPlan renders the encounter's committed Space (plus its Doors
// and Entrance, which live on Data/SpaceData respectively) as an ASCII
// grid: '.' floor, '#' an actual wall cell, 'D' a door, 'o' an obstacle
// (rolled or placed alike -- writePlacedObstacles already calls out
// placed ones by name and exact coordinate separately, so the map doesn't
// need a second, distinct marker to carry that distinction too), '@' the
// designated entrance cell (SpaceData.Entrance).
//
// SpaceData.Walls carries two shapes (see that field's doc): degenerate
// (Start == End) is an actual blocked interior hex, marked at Start;
// boundary-edge (Start != End) always has a real WALKABLE floor Start
// (never marked '#' -- an earlier version of this renderer folded both
// shapes into the same marker, which wrongly rendered the entrance
// region's own spawn column, and SpaceData.Entrance itself, as a wall,
// since most of a dungeon's outer edge is boundary-edge-only), but its End
// is one of two cases: genuinely outside the room (the outer-perimeter
// case, rpg-toolkit#834 -- nothing to mark, `set`'s own bounds check
// silently no-ops) or a connector column's flanking cell, still inside
// the room (rpg-toolkit#848) -- a real blocked interior cell, marked '#'
// exactly like a degenerate entry, just discretized as an edge rather
// than a hex.
func writeFloorPlan(b *strings.Builder, data *encounter.Data) {
	fmt.Fprintln(b, floorPlanLegend)
	sd := data.Space
	if sd == nil {
		fmt.Fprintln(b, "(no space)")
		return
	}

	grid := make([][]rune, sd.Height)
	for row := range grid {
		grid[row] = make([]rune, sd.Width)
		for col := range grid[row] {
			grid[row][col] = '.'
		}
	}
	// set is defensive, not load-bearing: every coordinate passed to it
	// below is already guaranteed in-bounds -- walls/doors/obstacles by
	// InitDungeon's own validation, Entrance by construction. The bounds
	// check exists so a violation of that assumption drops the mark
	// silently instead of panicking the whole report on a slice index.
	set := func(h core.Hex, r rune) {
		pos := h.ToPosition()
		col, row := int(pos.X), int(pos.Y)
		if row < 0 || row >= sd.Height || col < 0 || col >= sd.Width {
			return
		}
		grid[row][col] = r
	}

	for _, w := range sd.Walls {
		if w.Start == w.End {
			set(core.HexFromCube(w.Start), '#')
			continue
		}
		// Boundary-edge: Start is real floor, never marked -- see doc
		// above. End is marked when it's a real in-grid blocked cell
		// (rpg-toolkit#848); `set`'s own bounds check silently no-ops for
		// the outer-perimeter case, where End is genuinely outside the
		// room.
		set(core.HexFromCube(w.End), '#')
	}
	for _, door := range data.Doors {
		set(door.Position, 'D')
	}
	for _, o := range sd.Obstacles {
		set(o.Position, 'o')
	}
	set(sd.Entrance, '@')

	rows := make([]string, sd.Height)
	for row := range grid {
		rows[row] = string(grid[row])
	}
	fmt.Fprintln(b, strings.Join(rows, "\n"))
	fmt.Fprintln(b)
}
