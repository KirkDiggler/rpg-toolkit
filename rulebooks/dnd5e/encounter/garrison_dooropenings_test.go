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
func TestTheGarrisonsDoorOpeningsAreTheWholeFloorScans(t *testing.T) {
	if testing.Short() {
		t.Skip("opens the 10k garrison keep")
	}
	enc := openGarrisonKeep(t, false)
	asked, crossings, differ := encounter.FootprintDoorOpeningsAgainstScan(enc)
	require.Positive(t, asked, "precondition: the keep has footprint doors")
	require.Positive(t, crossings, "precondition: they lie across crossings")
	require.Empty(t, differ, "every door's opening is the scan's")
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
