// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package encounter_test

// sightpass_scale_test.go is WHAT DISCOVERY BY CELL COSTS ON A KEEP-SIZED
// FLOOR (rpg-project#586). Every sight pass now asks reachesCell of every
// floor cell an observer does not know yet, and seen-now asks it of every
// cell they do — so the open floor rpg-toolkit#2047 measured its compile on
// (73 x 48, 3,504 hexes, the castle keep's size) is the worst case: nothing
// in it stops a sightline, so every cell in range is a ray. Re-run rather
// than quoted:
// `go test . -run '^$' -bench SightPass_Keep -benchtime 20x`.

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter"
)

const (
	keepCols = 73
	keepRows = 48
)

// keepParty is four players in a line near the middle of the floor.
func keepParty() []encounter.MemberInput {
	out := make([]encounter.MemberInput, 0, 4)
	for i := 0; i < 4; i++ {
		out = append(out, encounter.MemberInput{
			ID: encounter.MemberID(fmt.Sprintf("p%d", i)), Kind: encounter.KindPlayer,
			Position: cellAt(30+i, 24),
		})
	}
	return out
}

func keepEncounter(tb testing.TB, sight encounter.Sight) *encounter.Encounter {
	tb.Helper()
	enc, err := encounter.NewEncounter(&encounter.SetupInput{
		Field: encounter.FieldInput{
			Canvas:  pointyCanvas(),
			Regions: []encounter.RegionInput{rectRegion("keep", 0, 0, keepCols, keepRows)},
		},
		Members: keepParty(),
		Endings: []encounter.EndingInput{{Key: "withdrawn", Trigger: encounter.TriggerExternal{}}},
		Capabilities: encounter.Capabilities{
			Sight:         sight,
			Equipment:     encounter.UnobservedEquipment{},
			Conditions:    encounter.UnobservedEquipment{},
			Sheets:        zeroSheets{},
			Participation: everyoneStanding{},
			Initiative:    orderAsGiven{},
			Driver:        passDriver{},
			Actors: encounter.Actors{
				Striker:   passStriker{},
				Mover:     quietMover{},
				Announcer: quietAnnouncer{},
			},
		},
	})
	require.NoError(tb, err)
	return enc
}

// keepSights is sixty feet of darkvision, the rulebook's stated default of
// 120 feet (combat.DefaultSightFeet, 24 cells — what a sheet that states no
// range answers), and a sight that reaches the whole floor.
var keepSights = []struct {
	name  string
	sight encounter.Sight
}{
	{"sight=12", &sightList{fallback: 12}},
	{"sight=24", &sightList{fallback: 24}},
	{"sight=unlimited", everyoneSeesTheWholeMap{}},
}

// BenchmarkSightPass_KeepFirstLight is the learning pass at its largest:
// four observers learning everything they reach at once, plus the rest of
// construction (NewEncounter runs the first pass).
func BenchmarkSightPass_KeepFirstLight(b *testing.B) {
	for _, tc := range keepSights {
		b.Run(tc.name, func(b *testing.B) {
			for i := 0; i < b.N; i++ {
				keepEncounter(b, tc.sight)
			}
		})
	}
}

// BenchmarkSightPass_KeepRecheck is one whole-party sight pass after first
// light: every known cell is skipped by the learning half, so this is what a
// refresh costs once the party has looked around.
func BenchmarkSightPass_KeepRecheck(b *testing.B) {
	for _, tc := range keepSights {
		b.Run(tc.name, func(b *testing.B) {
			enc := keepEncounter(b, tc.sight)
			party := []encounter.MemberID{"p0", "p1", "p2", "p3"}
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				if _, err := enc.Recheck(&encounter.RecheckInput{Members: party}); err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}

// BenchmarkSightPass_KeepStepLearns is one step that shows new floor: the
// step's own sight pass learns a fresh strip, writes its fact, and diffs the
// mover's atlas for the cells-revealed beat.
func BenchmarkSightPass_KeepStepLearns(b *testing.B) {
	for _, tc := range keepSights[:2] {
		b.Run(tc.name, func(b *testing.B) {
			for i := 0; i < b.N; i++ {
				b.StopTimer()
				enc := keepEncounter(b, tc.sight)
				b.StartTimer()
				if _, err := enc.Step(&encounter.StepInput{Member: "p0", To: cellAt(29, 24)}); err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}

// BenchmarkSightPass_KeepAtlasFor is the per-seat fixed map after first
// light.
func BenchmarkSightPass_KeepAtlasFor(b *testing.B) {
	for _, tc := range keepSights {
		b.Run(tc.name, func(b *testing.B) {
			enc := keepEncounter(b, tc.sight)
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				if _, err := enc.AtlasFor("p0"); err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}

// BenchmarkSightPass_KeepVisibleCells is one seat's seen-now read.
func BenchmarkSightPass_KeepVisibleCells(b *testing.B) {
	for _, tc := range keepSights {
		b.Run(tc.name, func(b *testing.B) {
			enc := keepEncounter(b, tc.sight)
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				if _, err := enc.VisibleCellsFor(&encounter.VisibleCellsInput{Member: "p0"}); err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}
