// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package session_test

import (
	"context"
	"fmt"
	"testing"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter/dungeonspec"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/session"
	"github.com/KirkDiggler/rpg-toolkit/tools/spatial"
)

// castle_sheet_bench_test.go measures what one verb pays to read sheets in
// the castle (rpg-toolkit#2493): one player step, and a walk of one world
// round (six cells of a thirty-foot walker), each reported with the
// character records read per verb and, where counted, the monster sheets
// loaded for the encounter's sheet facts. A benchmark, never a CI gate.

// sheetBenchCastle launches the castle for alice and the cleric and returns
// it with a free-roam path of cells cells out from alice's start.
func sheetBenchCastle(b *testing.B, cells int) (*freeRoamCastle, []spatial.Position, spatial.Position) {
	b.Helper()
	ctx := context.Background()
	raw, err := castleSourceOrError()
	if err != nil {
		b.Fatal(err)
	}
	compiled, err := dungeonspec.Load(raw)
	if err != nil {
		b.Fatal(err)
	}
	c := &freeRoamCastle{
		live: &actCapture{}, dice: &castleDice{d20s: []int{18}},
		characters: newFakeCharacters(armedFighter("alice"), healingCleric()),
		encounters: newFakeEncounters(), sessions: newFakeSessions(), seats: newFakeSeats(),
	}
	if c.mgr, err = c.newManager(); err != nil {
		b.Fatal(err)
	}
	if _, err := c.mgr.Launch(ctx, &session.LaunchInput{Session: "sess", Dungeon: &compiled, Party: []string{"alice", "cleric"}}); err != nil {
		b.Fatal(err)
	}
	enc, err := c.mgr.LoadWorldForTest(ctx, "sess")
	if err != nil {
		b.Fatal(err)
	}
	start := standingAt(enc, "alice")
	// The first creature a free walk of cells cells toward reaches is
	// whatever it is: the walk is the measure, not the destination.
	members, err := enc.Members()
	if err != nil {
		b.Fatal(err)
	}
	var route encounter.RouteOutput
	for _, m := range members {
		if m.ID == "alice" || m.ID == "cleric" {
			continue
		}
		if route, err = enc.Route(encounter.RouteInput{Mover: "alice", Policy: encounter.MoveToward, Anchor: m.Position, Budget: cells}); err != nil {
			b.Fatal(err)
		}
		if len(route.Path) >= cells {
			break
		}
	}
	if len(route.Path) < cells {
		b.Fatalf("nowhere to walk %d cells", cells)
	}
	return c, route.Path[:cells], start
}

// benchWalk walks alice out along path and back to start on alternate
// iterations, reporting reads per verb.
func benchWalk(b *testing.B, cells int) {
	c, out, start := sheetBenchCastle(b, cells)
	back := make([]spatial.Position, 0, cells)
	for i := len(out) - 2; i >= 0; i-- {
		back = append(back, out[i])
	}
	back = append(back, start)
	ctx := context.Background()
	loads, monsters := c.characters.loads, countMonsterSheetLoads()
	verbs := 0
	for b.Loop() {
		path := out
		if verbs%2 == 1 {
			path = back
		}
		if _, err := c.mgr.Move(ctx, &session.MoveInput{Session: "sess", Member: "alice", Path: path}); err != nil {
			b.Fatal(fmt.Errorf("walk %d: %w", verbs, err))
		}
		verbs++
	}
	b.ReportMetric(float64(c.characters.loads-loads)/float64(verbs), "character-reads/verb")
	b.ReportMetric(float64(countMonsterSheetLoads()-monsters)/float64(verbs), "monster-loads/verb")
}

// BenchmarkCastleSheets_OneStep is one player step in free roam.
func BenchmarkCastleSheets_OneStep(b *testing.B) { benchWalk(b, 1) }

// BenchmarkCastleSheets_OneWorldRound is a six-cell walk: one round of the
// world, in which every creature that acts asks for its sheet.
func BenchmarkCastleSheets_OneWorldRound(b *testing.B) { benchWalk(b, 6) }

// countMonsterSheetLoads is the sheet seam's monster loads so far.
func countMonsterSheetLoads() int { return session.MonsterSheetLoadsForTest() }
