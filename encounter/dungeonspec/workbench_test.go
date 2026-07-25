// Copyright (C) 2024 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package dungeonspec_test

import (
	"testing"

	"github.com/KirkDiggler/rpg-toolkit/encounter/dungeonspec"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestWorkbenchReport_ValidSpec pins the fast-authoring-loop contract
// (design.md's observability addendum, rpg-toolkit#842): a valid spec
// produces a VALID verdict naming the seed, the boss-first spawn plan,
// and an ASCII floor plan -- everything an author needs to eyeball a
// layout without standing up a server.
//
// Assertions target generated CONTENT, not static header/legend text a
// report could satisfy while broken: "verdict: VALID" (not bare "VALID",
// which "verdict: INVALID" also contains -- an always-INVALID mutant must
// fail this), the boss spawn's actual monster ref (not just the word
// "boss", which the region legend/section headers also contain
// regardless of whether any spawn printed), and a specific floor-plan
// row -- see the comment on that assertion below.
//
// placedTombYAML, not referenceYAML/a crypt fixture: referenceYAML's own
// count-based monsters fail Validate in M1 (Task B2's fixture
// consequence, same reason compile_test.go/validate_test.go avoid it for
// their own M1-valid assertions).
func TestWorkbenchReport_ValidSpec(t *testing.T) {
	report, err := dungeonspec.WorkbenchReport([]byte(placedTombYAML), 42)
	require.NoError(t, err)
	assert.Contains(t, report, "verdict: VALID")
	assert.Contains(t, report, "seed 42")
	assert.Contains(t, report, "dnd5e:monsters:skeleton-captain (boss)") // spawn plan: actual boss spawn
	// entrance (width 6) + a real wall at the region-boundary column (col
	// 6) + tomb (width 12, no obstacle on this particular row): a mutant
	// that draws no walls, or shifts any coordinate by even one column,
	// cannot produce this exact string.
	assert.Contains(t, report, "......#............")
}

// TestWorkbenchReport_InvalidSpecShowsVerdict pins the other half of the
// contract: an invalid spec still returns a report (not just an error) so
// the CLI has something to print, and that report says INVALID rather
// than silently looking like a truncated valid one.
func TestWorkbenchReport_InvalidSpecShowsVerdict(t *testing.T) {
	report, err := dungeonspec.WorkbenchReport([]byte("version: 1\n"), 1)
	require.Error(t, err)
	assert.Contains(t, report, "INVALID")
}

// TestWorkbenchReport_PlacedEntriesAtExactCoordinates pins the delta
// addition (design.md §Design delta): a placed prop must show up in the
// report at its exact compiled coordinate, not just "an obstacle
// somewhere in the room" -- placedTombYAML's tomb room places a coffin at
// [6, 3]. Asserted as ONE paired string (ref and coordinate together, not
// two separate Contains checks) so a mutant that prints the right refs
// and the right coordinates but MIS-PAIRS them (e.g. coffin's ref next to
// altar's coordinate) still fails this test.
func TestWorkbenchReport_PlacedEntriesAtExactCoordinates(t *testing.T) {
	report, err := dungeonspec.WorkbenchReport([]byte(placedTombYAML), 7)
	require.NoError(t, err)
	assert.Contains(t, report, "dnd5e:props:coffin at col=6 row=3")
}

// TestWorkbenchReport_FloorPlanRendersEntranceAndWalkablePerimeter pins
// the floor plan's honesty contract: SpaceData.Walls carries a
// render-only "boundary-edge" shape (rpg-toolkit#834) whose Start is real
// WALKABLE floor, distinct from an actual blocking wall (see
// writeFloorPlan's doc) -- the floor plan must not draw those as '#'.
func TestWorkbenchReport_FloorPlanRendersEntranceAndWalkablePerimeter(t *testing.T) {
	report, err := dungeonspec.WorkbenchReport([]byte(placedTombYAML), 42)
	require.NoError(t, err)
	// The entrance sits at the room's far edge, on the SAME row as the
	// door joining entrance->tomb (design.md/dungeon.go: "just inside the
	// entrance, not center") -- '@' at col 0, 'D' at col 6, floor between
	// and after. A renderer that folds boundary-edge segments into '#'
	// (as an earlier version of this one did) draws the party's own spawn
	// cell as a wall instead.
	assert.Contains(t, report, "@.....D............")
	// A door-free row elsewhere in the entrance region: its own left edge
	// (col 0) is a walkable PERIMETER cell -- under the old (incorrect)
	// boundary-edge-folding renderer this rendered as a wall too, even
	// though nothing ever blocks movement there.
	assert.Contains(t, report, "......#............")
}
