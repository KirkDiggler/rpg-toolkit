// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package session

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter"
	"github.com/KirkDiggler/rpg-toolkit/tools/spatial"
)

// move_internal_test.go pins the two claims that let the walk stop resolving
// rooms for itself (rpg-toolkit#1059).
//
// Both are about what the seam NO LONGER needs to know, which is exactly the
// kind of claim that is invisible from outside: the verb's behaviour is
// identical either way, and only a test aimed at the reasoning can say why the
// deletion was safe rather than lucky.

// walkOrderAsGiven and walkEveryoneStanding are two of the capabilities every
// encounter needs, in their most boring form. The external test package has its
// own; these tests are internal and cannot see them.
type walkOrderAsGiven struct{}

func (walkOrderAsGiven) RollInitiative(members []encounter.MemberID) ([]encounter.MemberID, error) {
	return members, nil
}

type walkEveryoneStanding struct{}

func (walkEveryoneStanding) Standing(_ []encounter.MemberID) ([]encounter.MemberID, error) {
	return nil, nil
}

// passDriver is the third: every unplayed member always passes. Shared across
// this package's internal test files (attack_internal_test.go included)
// rather than "walk"-prefixed, since it answers the same boring question
// regardless of which internal verb's test needs an encounter built.
type passDriver struct{}

func (passDriver) Act(encounter.MonsterView) (encounter.TurnIntent, error) {
	return encounter.Pass{}, nil
}

// walkWorld is two regions of DIFFERENT sizes, painted away from the origin.
// Different sizes on purpose: the grid this seam builds used to take the
// walker's own room's span, so a fixture whose rooms agree could not tell a
// span that mattered from one that never did.
func walkWorld(t *testing.T) *encounter.Encounter {
	t.Helper()

	enc, err := encounter.NewEncounter(&encounter.SetupInput{Striker: encounter.RefusingStriker{}, Mover: encounter.RefusingMover{}, Announcer: encQuietAnnouncer{}, Sight: &sightSeam{},
		Initiative: walkOrderAsGiven{}, TurnDriver: passDriver{}, Standing: walkEveryoneStanding{},
		Field: encounter.FieldInput{Canvas: pointyCanvas(), Regions: []encounter.RegionInput{
			rectRegion("hall", 30, 40, 4, 4),
			rectRegion("annex", 60, 40, 12, 12),
		}},
		Members: []encounter.MemberInput{{
			ID: "alice", Kind: encounter.KindPlayer, Position: spatial.Position{X: 31, Y: 41},
		}},
		Endings: []encounter.EndingInput{{Key: "done", Trigger: encounter.TriggerExternal{}}},
	})
	require.NoError(t, err)
	return enc
}

// TestTheAdjacencyGridIsSpanIndependent is the claim that lets gridOf ask the
// composition for a FAMILY and nothing else.
//
// The seam used to fetch a whole Atlas per Move — documented O(total cells),
// measured at ~128MB and tens of milliseconds at the legal field budget — and
// the only thing it wanted from all that was one room's grid family and its
// width and height. The width and height were never load-bearing: adjacency is
// Distance <= 1 in every family, and no family's Distance consults the grid's
// dimensions. Cells hundreds of units outside the span the grid was built with
// therefore answer exactly as cells inside it do.
//
// If that ever stops being true, this test fails and gridOf has to start
// carrying a real span again — which is the point of asserting it here rather
// than trusting a comment.
func TestTheAdjacencyGridIsSpanIndependent(t *testing.T) {
	grid, err := gridOf(walkWorld(t))
	require.NoError(t, err)

	// Far outside adjacencySpan, in both regions. The grid speaks AXIAL, so
	// the authored [col,row] pairs the fixture paints are converted the one
	// way everything else is (the composition's HexCellAt) before they are
	// asked about — two cells on one authored row are always neighbours.
	at := func(col, row int) spatial.Position {
		return encounter.HexCellAt(encounter.HexesArePointyTop(), col, row)
	}
	require.True(t, grid.IsAdjacent(at(31, 41), at(32, 41)),
		"a step east deep inside the hall")
	require.True(t, grid.IsAdjacent(at(64, 47), at(65, 47)),
		"and one deep inside the annex, whose span is three times the hall's")
	require.False(t, grid.IsAdjacent(at(31, 41), at(33, 41)),
		"two cells apart is still two cells apart")
}

// TestTheAdjacencyGridUsesCubeDistanceOnHex is the discriminating half, and it
// is the reason gridOf asks for the family at all rather than hard-coding one
// distance formula.
//
// Axial (1,1) is cube distance 2 from the origin — two steps, not one — while
// Chebyshev distance calls it 1. Substituting the square formula for the hex
// one passes almost every hex fixture and fails only on the diagonals, which is
// a previously-shipped defect class in this codebase.
func TestTheAdjacencyGridUsesCubeDistanceOnHex(t *testing.T) {
	grid, err := gridOf(walkWorld(t))
	require.NoError(t, err)

	require.True(t, grid.IsAdjacent(spatial.Position{X: 30, Y: 40}, spatial.Position{X: 31, Y: 40}),
		"one step east")
	require.False(t, grid.IsAdjacent(spatial.Position{X: 30, Y: 40}, spatial.Position{X: 31, Y: 41}),
		"axial (1,1) away is TWO hex steps, whatever Chebyshev says")
}

// TestStandsAtReadsTheRosterRow pins the round trip that finding 3 deleted.
//
// Asking where somebody is used to mean: read the roster (which already
// carries a dungeon-absolute cell), Locate that cell down to a room and a
// room-local one, then project it straight back up again through onMap. Two
// composition queries and a silent-fallback helper to return the number the
// first query already had.
func TestStandsAtReadsTheRosterRow(t *testing.T) {
	enc := walkWorld(t)

	at, err := standsAt(enc, "alice")
	require.NoError(t, err)
	require.Equal(t, encounter.HexCellAt(encounter.HexesArePointyTop(), 31, 41), at,
		"authored [31,41] as one axial cell, as the roster already reported it")

	members, err := enc.Members()
	require.NoError(t, err)
	require.Equal(t, members[0].Position, at, "which is the roster row itself, unmodified")
}

// TestStandsAtRefusesSomebodyWhoIsNotHere keeps the verb's own sentinel on the
// path a caller can actually drive.
func TestStandsAtRefusesSomebodyWhoIsNotHere(t *testing.T) {
	_, err := standsAt(walkWorld(t), "nobody")
	require.ErrorIs(t, err, ErrNoMember)
}

// pointyCanvas and rectRegion are the internal package's copies of the
// external fixture helpers (regionfixtures_test.go): a Go test in package
// session cannot see package session_test.
func pointyCanvas() encounter.CanvasInput {
	return encounter.CanvasInput{Void: encounter.VoidIsOpaque(), Orientation: encounter.HexesArePointyTop()}
}

func rectRegion(id string, col, row, w, h int) encounter.RegionInput {
	cells := make([]spatial.Position, 0, w*h)
	for r := 0; r < h; r++ {
		for c := 0; c < w; c++ {
			cells = append(cells, spatial.Position{X: float64(col + c), Y: float64(row + r)})
		}
	}
	return encounter.RegionInput{ID: id, Name: id, Cells: cells, Archetype: "crypt", Lighting: &encounter.Lighting{Intensity: 1}}
}
