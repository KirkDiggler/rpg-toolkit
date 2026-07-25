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
// and an ASCII floor plan (walls render as '#') -- everything an author
// needs to eyeball a layout without standing up a server.
//
// placedTombYAML, not referenceYAML/a crypt fixture: referenceYAML's own
// count-based monsters fail Validate in M1 (Task B2's fixture
// consequence, same reason compile_test.go/validate_test.go avoid it for
// their own M1-valid assertions).
func TestWorkbenchReport_ValidSpec(t *testing.T) {
	report, err := dungeonspec.WorkbenchReport([]byte(placedTombYAML), 42)
	require.NoError(t, err)
	assert.Contains(t, report, "VALID")
	assert.Contains(t, report, "seed 42")
	assert.Contains(t, report, "boss") // spawn plan section
	assert.Contains(t, report, "#")    // ASCII map: walls
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
// [6, 3].
func TestWorkbenchReport_PlacedEntriesAtExactCoordinates(t *testing.T) {
	report, err := dungeonspec.WorkbenchReport([]byte(placedTombYAML), 7)
	require.NoError(t, err)
	assert.Contains(t, report, "coffin")
	assert.Contains(t, report, "col=6 row=3")
}
