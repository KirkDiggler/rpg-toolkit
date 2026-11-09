// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package encounter_test

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter"
)

// TestTheGarrisonsDoorOpeningsAreTheWholeFloorScans holds every footprint
// door of the garrison keep to the whole-floor scan (rpg-project#879 S9): the
// field's answer, read from the cells near each door, is the scan's.
//
// THE KEEP IS THE SMALL ONE (rpg-project#909). The fast path is
// [field.doorCrossings]' window of cells around a door's reach; the oracle
// asks every floor cell. What the comparison needs is every kind of door the
// keep cuts, a room to a room, to the courtyard and to the grounds, in walls
// running both ways, each with floor on both sides and beyond its window:
// [smallKeepPlan] cuts all 36 of the 10k keep's doors in 1,230 hexes. A
// door's crossings are floor and placement alone, so the keep opens with two
// players and no garrison. The 10k garrison keep's comparison is
// [BenchmarkGarrisonDoorOpeningsAgainstScan].
func TestTheGarrisonsDoorOpeningsAreTheWholeFloorScans(t *testing.T) {
	compiled := smallKeep(t, false)
	enc := openCompiled(t, compiled, seatedParty(compiled)[:2], false, findsNothing{})
	asked, crossings, differ := encounter.FootprintDoorOpeningsAgainstScan(enc)
	require.Equal(t, 36, asked, "precondition: every door of the plan is a footprint door")
	require.Positive(t, crossings, "precondition: they lie across crossings")
	require.Empty(t, differ, "every door's opening is the scan's")
}

// BenchmarkGarrisonDoorOpeningsAgainstScan is the comparison on the 10k
// garrison keep (rpg-project#909): what it was gated on before the small
// keep, kept as a number a measurement asks for by name, never a CI gate. Each
// iteration asks every door and scans the whole floor; it fails if one door
// differs.
//
//	go test -run '^$' -bench BenchmarkGarrisonDoorOpeningsAgainstScan -benchtime 1x .
func BenchmarkGarrisonDoorOpeningsAgainstScan(b *testing.B) {
	enc := openGarrisonKeep(b, false)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		asked, crossings, differ := encounter.FootprintDoorOpeningsAgainstScan(enc)
		if asked == 0 || crossings == 0 || len(differ) > 0 {
			b.Fatalf("doors %d, crossings %d, differ from the scan %v", asked, crossings, differ)
		}
	}
}

// BenchmarkGarrisonLoadAtlas is what the door openings cost a verb that loads
// the garrison keep (rpg-project#879 S9, F17): the saved run read back and
// its full atlas built, which asks every footprint door for its crossings.
// The keep's saved bytes are unmarshalled outside the timer. Public API only,
// so it runs on a head before S9 as well.
//
//	go test -run '^$' -bench BenchmarkGarrisonLoadAtlas -benchtime 10x .
func BenchmarkGarrisonLoadAtlas(b *testing.B) {
	enc := openGarrisonKeep(b, false)
	raw, err := json.Marshal(enc.ToData())
	require.NoError(b, err)
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		b.StopTimer()
		var data encounter.EncounterData
		if err := json.Unmarshal(raw, &data); err != nil {
			b.Fatal(err)
		}
		b.StartTimer()
		loaded, err := encounter.LoadEncounter(&encounter.LoadEncounterInput{
			Data: data, Capabilities: knowledgeCapabilities(&sightList{fallback: 24}),
		})
		if err != nil {
			b.Fatal(err)
		}
		if _, err := loaded.Atlas(); err != nil {
			b.Fatal(err)
		}
	}
}
