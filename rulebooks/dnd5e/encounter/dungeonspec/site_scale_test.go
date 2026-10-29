// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package dungeonspec_test

// site_scale_test.go is THE SITE FORMAT'S YARDSTICK (rpg-project#725, R1 and
// R8): one site of 30,000 hexes, no fade, generated rather than authored, and
// measured against the file budget R1 sets for this story — under 100 KB of
// geometry — with the compile time and the per-load cost of deriving the
// wall partitions (R8) beside it. Re-run rather than quoted:
//
//	go test ./dungeonspec/ -run 'TestBigSite' -bench 'CompileSingleRoom_30k|FieldLoad' -benchtime 3x
//
// The shape is O4's open question answered the simplest honest way: rooms of
// cols x rows tiled in a grid, one authored region per room, alternate rooms
// lit at 1 and 0.4, and a wall with one shut door between every two
// neighbours — so every room is its own partition and the partitioning has
// as many walls and doors to read as a site that size is likely to carry.

import (
	"fmt"
	"math"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/KirkDiggler/rpg-toolkit/core"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter/dungeonspec"
)

// The 30k fixture: 60 rooms of 25 x 20, ten across and six down.
const (
	bigSiteAcross = 10
	bigSiteDown   = 6
	bigSiteCols   = 25
	bigSiteRows   = 20
)

// bigSiteDoc is a generated site and the bytes of its geometry: the
// `regions` block and the `walls` block exactly as they appear in the file.
type bigSiteDoc struct {
	yaml                   string
	regionBytes, wallBytes int
	cells, rooms, walls    int
	doors                  int
}

// The two ways bigSite walls its rooms. Both put a wall and one shut door
// between every two neighbours; they differ in how many wall lines carry them.
const (
	// seamWalls is one wall line per seam across the whole site, with one
	// door-carrying opening per pair of rooms it divides — how the castle's
	// author draws a long wall.
	seamWalls = "seam"
	// roomWalls is a wall line per pair of neighbours (each room carries its
	// east and its south wall, wallsPerRegion = 2) with its one door.
	roomWalls = "room"
)

// bigSite writes a version 5 site of across x down rooms, each cols x rows
// hexes of odd-r offset floor written as axial row runs, walled per shape.
//
// THE WALLS ARE QUARTER LINES, the reference tomb's: a vertical seam at
// boundary column c runs at scene x = sqrt(3)(c - 1/4), between the centres
// of both row parities, and a horizontal seam between rows R-1 and R runs at
// z = 1.5(R - 1/2). Neither covers a cell centre, so no floor turns to
// footing and every crossing between two rooms is cut.
func bigSite(across, down, cols, rows int, shape string) bigSiteDoc {
	sqrt3 := math.Sqrt(3)
	var regions, walls, bindings strings.Builder
	doc := bigSiteDoc{rooms: across * down}
	regions.WriteString("    regions:\n")
	for j := 0; j < down; j++ {
		for i := 0; i < across; i++ {
			lighting := "1"
			if (i+j)%2 == 1 {
				lighting = "0.4"
			}
			fmt.Fprintf(&regions, "      - id: room-%d-%d\n        name: Room %d-%d\n        lighting: { intensity: %s }\n        cells:\n",
				i, j, i, j, lighting)
			for row := j * rows; row < (j+1)*rows; row++ {
				// odd-r offset column to axial: q = col - (row - (row&1))/2.
				shift := (row - (row & 1)) / 2
				fmt.Fprintf(&regions, "          - r: %d\n            q: [[%d, %d]]\n",
					row, i*cols-shift, (i+1)*cols-1-shift)
				doc.cells += cols
			}
		}
	}
	// wall writes one line from (sx, sz) to (ex, ez) with a door-carrying
	// opening centred at each of the given distances along it.
	wall := func(id string, sx, sz, ex, ez float64, doorsAt ...float64) {
		length := math.Hypot(ex-sx, ez-sz)
		fmt.Fprintf(&walls, "      - id: %s\n        label: %s\n        line: { start: { x: %s, z: %s }, end: { x: %s, z: %s } }\n"+
			"        appearance: { assetRef: 'dnd5e:env:test:wall', height: 3, thickness: 0.2, elevation: 0 }\n"+
			"        blocker: { blocksMovement: true, blocksLineOfSight: true, footprint: { width: %s, depth: 0.2, offsetX: 0, offsetZ: 0 } }\n"+
			"        openings:\n", id, id, num(sx), num(sz), num(ex), num(ez), num(length))
		for k, at := range doorsAt {
			door := fmt.Sprintf("%s-%d", id, k)
			fmt.Fprintf(&walls, "          - { id: %s-gap, position: %s, width: 2, door: { id: %s, assetRef: 'dnd5e:env:test:door' } }\n",
				door, num(at), door)
			fmt.Fprintf(&bindings, "      %s: { closed: true }\n", door)
			doc.doors++
		}
		doc.walls++
	}
	x := func(c float64) float64 { return sqrt3 * (c - 0.25) } // a vertical seam before column c
	z := func(r float64) float64 { return 1.5*r - 0.75 }       // a horizontal seam before row r
	walls.WriteString("    walls:\n")
	switch shape {
	case seamWalls:
		top, bottom := z(0), z(float64(down*rows))
		for i := 1; i < across; i++ {
			var doors []float64
			for j := 0; j < down; j++ {
				doors = append(doors, z(float64(j*rows)+float64(rows)/2)-top)
			}
			wall(fmt.Sprintf("v%d", i), x(float64(i*cols)), top, x(float64(i*cols)), bottom, doors...)
		}
		west, east := x(0), x(float64(across*cols))
		for j := 1; j < down; j++ {
			var doors []float64
			for i := 0; i < across; i++ {
				doors = append(doors, x(float64(i*cols)+float64(cols)/2)-west)
			}
			wall(fmt.Sprintf("h%d", j), west, z(float64(j*rows)), east, z(float64(j*rows)), doors...)
		}
	case roomWalls:
		for j := 0; j < down; j++ {
			for i := 0; i < across; i++ {
				top, bottom := z(float64(j*rows)), z(float64((j+1)*rows))
				west, east := x(float64(i*cols)), x(float64((i+1)*cols))
				if i+1 < across {
					wall(fmt.Sprintf("e-%d-%d", i, j), east, top, east, bottom, (bottom-top)/2)
				}
				if j+1 < down {
					wall(fmt.Sprintf("s-%d-%d", i, j), west, bottom, east, bottom, (east-west)/2)
				}
			}
		}
	}
	doc.regionBytes, doc.wallBytes = regions.Len(), walls.Len()
	doc.yaml = "version: 5\nkey: big-site\nplay: { void: transparent, lighting: bright, standing: centre-covered }\n" +
		"room:\n  version: 3\n  id: room-1\n  name: Big Site\n  coordinateFrame: { hexRadius: 1 }\n" +
		"  scene: { version: 1, id: scene-1, name: Big Site, items: [], groups: [] }\n" +
		"  room:\n    implicitRegionId: room-1-region\n" + regions.String() +
		"    propDeclarations: {}\n    arrangementDeclarations: {}\n    partyStart: { q: 2, r: 2 }\n    monsterDeclarations: []\n" +
		walls.String() + "    doorBindings:\n" + bindings.String()
	return doc
}

// num writes a scene coordinate with enough digits to land where it was
// computed and no more.
func num(v float64) string { return fmt.Sprintf("%.4g", v) }

// TestBigSite_GeometryUnder100KB is R1's file budget: 30,000 hexes of floor
// in sixty regions, with their walls and doors, in under 100,000 bytes of
// geometry — and the site compiles, every room its own partition.
//
// THE BUDGET IS ASSERTED ON THE SEAM-WALLED SITE. The same rooms walled pair
// by pair are measured and logged beside it, and are over the budget: a wall
// line costs about 460 bytes before its first opening, and a hundred and four
// of them are half the file. The regions — this story's encoding — are the
// same 60 KB in both.
func TestBigSite_GeometryUnder100KB(t *testing.T) {
	perRoom := bigSite(bigSiteAcross, bigSiteDown, bigSiteCols, bigSiteRows, roomWalls)
	t.Logf("30k site, a wall per neighbour pair: regions %d bytes, walls %d bytes (%d walls, %d doors), geometry %d bytes",
		perRoom.regionBytes, perRoom.wallBytes, perRoom.walls, perRoom.doors, perRoom.regionBytes+perRoom.wallBytes)

	doc := bigSite(bigSiteAcross, bigSiteDown, bigSiteCols, bigSiteRows, seamWalls)
	require.Equal(t, 30000, doc.cells)
	require.Equal(t, 60, doc.rooms)
	require.Equal(t, perRoom.doors, doc.doors, "the same doors, one between every two neighbours")
	geometry := doc.regionBytes + doc.wallBytes
	t.Logf("30k site, a wall per seam: regions %d bytes, walls %d bytes (%d walls, %d doors), geometry %d bytes, whole file %d bytes",
		doc.regionBytes, doc.wallBytes, doc.walls, doc.doors, geometry, len(doc.yaml))
	require.Less(t, geometry, 100_000, "R1: under 100 KB of geometry at 30k hexes")

	// The site compiles — the measured one; the per-pair twin's bytes are
	// what this test is about, and compiling it too doubles a test that is
	// already the package's slowest.
	compiled, err := dungeonspec.Load([]byte(doc.yaml))
	require.NoError(t, err)
	cells := 0
	for _, r := range compiled.Field.Regions {
		cells += len(r.Cells)
	}
	require.Equal(t, 30000, cells, "every hex is floor: no wall turned any to footing")
	require.Len(t, compiled.Field.Regions, 60)
	require.Len(t, compiled.PartyStart, bigSiteCols*bigSiteRows, "the party's seats are its whole starting room")
	for _, seat := range compiled.PartyStart {
		require.Equal(t, "room-0-0", seat.Region, "and no other")
	}
	out, err := encounter.PartitionFloor(encounter.PartitionFloorInput{Field: compiled.Field})
	require.NoError(t, err)
	require.Len(t, out.Components, 60, "the walls make sixty rooms")
	require.Empty(t, out.Footing)
}

func BenchmarkCompileSingleRoom_30k(b *testing.B) {
	raw := []byte(bigSite(bigSiteAcross, bigSiteDown, bigSiteCols, bigSiteRows, seamWalls).yaml)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := dungeonspec.Load(raw); err != nil {
			b.Fatal(err)
		}
	}
}

// BenchmarkFieldLoad is R8's measurement: what loading a persisted encounter
// costs with the wall partitions derived, on the castle and on the 30k site.
//
//   - load is [encounter.LoadEncounter] from the encounter's own data: the
//     whole load, partition derivation included.
//   - field is the field compile alone ([encounter.CompileStaticPlacements]):
//     the floor, props, walls and doors, no partitions.
//   - partitions is [encounter.PartitionFloor]: the same compile plus the
//     door validation and the partition derivation.
//
// So the derivation's own cost is partitions - field, and a load without it
// is load - (partitions - field). Both are read off the three, because the
// derivation is not something a caller can switch off.
func BenchmarkFieldLoad(b *testing.B) {
	for _, site := range []struct {
		name string
		raw  []byte
	}{
		{"castle", readFixture(b, "testdata/world-builder-v4-castle.yaml")},
		{"30k", []byte(bigSite(bigSiteAcross, bigSiteDown, bigSiteCols, bigSiteRows, seamWalls).yaml)},
	} {
		compiled, err := dungeonspec.Load(site.raw)
		require.NoError(b, err)
		enc := siteEncounter(b, compiled)
		data := enc.ToData()
		caps := siteCapabilities()
		b.Run(site.name+"/load", func(b *testing.B) {
			for i := 0; i < b.N; i++ {
				if _, err := encounter.LoadEncounter(&encounter.LoadEncounterInput{Data: data, Capabilities: caps}); err != nil {
					b.Fatal(err)
				}
			}
		})
		b.Run(site.name+"/field", func(b *testing.B) {
			for i := 0; i < b.N; i++ {
				if _, err := encounter.CompileStaticPlacements(compiled.Field); err != nil {
					b.Fatal(err)
				}
			}
		})
		b.Run(site.name+"/partitions", func(b *testing.B) {
			for i := 0; i < b.N; i++ {
				if _, err := encounter.PartitionFloor(encounter.PartitionFloorInput{Field: compiled.Field}); err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}

// siteEncounter sets a compiled site up with one watcher on the party's
// first seat.
func siteEncounter(tb testing.TB, compiled dungeonspec.Compiled) *encounter.Encounter {
	tb.Helper()
	enc, err := encounter.NewEncounter(&encounter.SetupInput{
		Field: compiled.Field,
		Members: []encounter.MemberInput{
			{ID: core.EntityID("watcher"), Kind: encounter.KindPlayer, Position: compiled.PartyStart[0].At},
		},
		Endings:      []encounter.EndingInput{{Key: "withdrawn", Trigger: encounter.TriggerExternal{}}},
		Capabilities: siteCapabilities(),
	})
	require.NoError(tb, err)
	return enc
}

// siteCapabilities is everything a site's encounter asks of its host, the
// quietest answer to each.
func siteCapabilities() encounter.Capabilities {
	return encounter.Capabilities{
		Sight:         everyoneSeesTheWholeMap{},
		Equipment:     encounter.UnobservedEquipment{},
		Conditions:    encounter.UnobservedEquipment{},
		Sheets:        zeroSheets{},
		Participation: everyoneStanding{},
		Initiative:    orderAsGiven{},
		Driver:        passDriver{},
		CheckResolver: nobodyFindsAnything{},
		Witness:       nobodyIsWatching{},
		Actors: encounter.Actors{
			Striker:   noAttacksExpected{},
			Mover:     quietMover{},
			Announcer: quietAnnouncer{},
		},
	}
}
