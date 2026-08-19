// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package encounter_test

import (
	"runtime"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter"
	"github.com/KirkDiggler/rpg-toolkit/tools/spatial"
)

// budgetField is the LARGEST LEGAL field: four 1024x1024 rooms, exactly
// maxFieldCells (1<<22) cells between them — which the encounter below persists
// in 601 bytes, measured. The gap between what a field COSTS TO SAY and what it
// used to cost to READ is the whole subject of the test.
func budgetField() encounter.FieldInput {
	const dim = 1024
	return encounter.FieldInput{
		Rooms: []encounter.RoomInput{
			{ID: "a", Width: dim, Height: dim, Origin: spatial.Position{X: 0, Y: 0}},
			{ID: "b", Width: dim, Height: dim, Origin: spatial.Position{X: dim, Y: 0}},
			{ID: "c", Width: dim, Height: dim, Origin: spatial.Position{X: 0, Y: dim}},
			{ID: "d", Width: dim, Height: dim, Origin: spatial.Position{X: dim, Y: dim}},
		},
	}
}

// TestAtlasReportsRegionsWithoutEnumeratingThem is the cost half of
// rpg-toolkit#1108, pinned as a property rather than left as a claim in a doc
// comment.
//
// The Atlas used to materialize every cell of every room. At this field —
// legal, and small to say — one call allocated 128.02 MB and took 43.3 ms,
// repeatably, because nothing memoized it (measured on the post-S0 shape, which
// is where this slice starts). #1059 found that cost being paid by callers who
// wanted the grid FAMILY and nothing else and added Encounter.Grid beside it;
// this is the other half of the same finding.
//
// A region reports what it IS — anchor, span, family — and RegionAt answers
// membership from that. So the report is O(regions), and a host that genuinely
// wants cells walks a rectangle it was handed rather than one the module built
// for it. The bound is deliberately loose (1 MB against a measured 624 bytes): this
// is pinning an ORDER OF GROWTH, and a tight bound would fail on an unrelated
// allocation without saying anything true.
func TestAtlasReportsRegionsWithoutEnumeratingThem(t *testing.T) {
	t.Run("a region describes its cells rather than listing them", func(t *testing.T) {
		require.Equal(t,
			[]string{"ID", "Grid", "Origin", "Width", "Height", "Occluders", "Boundaries"},
			structFieldNames(encounter.AtlasRegion{}),
			"an AtlasRegion is an anchor and a span; a cell list would be the enumeration this slice deleted")
	})

	t.Run("and reporting them costs O(regions), not O(cells)", func(t *testing.T) {
		enc, err := encounter.NewEncounter(&encounter.SetupInput{
			Standing: everyoneStanding{}, Initiative: orderAsGiven{},
			Field: budgetField(),
			Members: []encounter.MemberInput{
				{ID: alice, Kind: encounter.KindPlayer, Room: "a", Position: spatial.Position{X: 1, Y: 1}},
			},
			Endings: []encounter.EndingInput{{Key: "done", Trigger: encounter.TriggerExternal{}}},
		})
		require.NoError(t, err)

		var before, after runtime.MemStats
		runtime.GC()
		runtime.ReadMemStats(&before)
		atlas, err := enc.Atlas()
		runtime.ReadMemStats(&after)
		require.NoError(t, err)
		require.Len(t, atlas.Regions, 4)

		allocated := after.TotalAlloc - before.TotalAlloc
		t.Logf("Atlas at the legal field budget (%d cells): %d bytes allocated", 4*1024*1024, allocated)
		require.Less(t, allocated, uint64(1<<20),
			"one Atlas call allocated %d bytes for a 4-region field — an enumeration has grown back "+
				"(it was 134,225,920 bytes before rpg-toolkit#1108)", allocated)
	})
}
