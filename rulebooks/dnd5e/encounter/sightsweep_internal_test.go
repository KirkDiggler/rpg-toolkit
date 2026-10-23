// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package encounter

// sightsweep_internal_test.go is the sight sweep skip (rpg-project#664): an
// observer whose sweep key is unchanged does not re-sweep the floor, a door
// change re-sweeps everyone who could see through it, and a save without
// keys sweeps once and then holds. Every test also asks the ORACLE — no
// observer is ever left not knowing a cell its sight reaches now — which is
// the claim the skip must never break: same known cells as sweeping always.
//
// White-box because the sweep count is the encounter's own business: from
// outside, a skipped sweep and a sweep that learned nothing look the same.

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/KirkDiggler/rpg-toolkit/tools/spatial"
)

// The sweep scene's members: two watchers and a walker in the west room, a
// watcher in the east room, and a shut door in the one-cell neck between.
const (
	sweepWatchA  = MemberID("watch-a")
	sweepWatchB  = MemberID("watch-b")
	sweepEast    = MemberID("watch-east")
	sweepWalker  = MemberID("walker")
	sweepNeckDor = DoorID("neck-door")
)

var sweepMembers = []MemberID{sweepWatchA, sweepWatchB, sweepEast, sweepWalker}

// sweepField is a west room and an east room joined by a one-cell-wide neck
// in opaque void, with a shut door on the neck's one crossing: shut, neither
// room sees the other at all; open, each sees down the neck into the other.
func sweepField() FieldInput {
	return FieldInput{
		Canvas: CanvasInput{Void: VoidIsOpaque(), Orientation: HexesArePointyTop()},
		Regions: []RegionInput{
			rectRegion("west", 0, 0, 4, 3),
			rectRegion("neck", 4, 1, 3, 1),
			rectRegion("east", 7, 0, 4, 3),
		},
		Doors: []DoorInput{{ID: sweepNeckDor, State: DoorIsClosed(),
			Edges: []DoorEdge{{From: spatial.Position{X: 4, Y: 1}, To: spatial.Position{X: 5, Y: 1}}}}},
	}
}

func sweepCapabilities() Capabilities {
	speed := SheetFacts{SpeedFeet: 30}
	return Capabilities{
		Sight:         everyoneSeesTheWholeMap{},
		Equipment:     UnobservedEquipment{},
		Conditions:    UnobservedEquipment{},
		Sheets:        sheetFacts{sweepWatchA: speed, sweepWatchB: speed, sweepEast: speed, sweepWalker: speed},
		Participation: everyoneStanding{},
		Initiative:    orderAsGiven{},
		Driver:        passDriver{},
		Actors:        Actors{Striker: passStriker{}, Mover: quietMover{}, Announcer: quietAnnouncer{}},
	}
}

func sweepScene(t *testing.T) *Encounter {
	t.Helper()
	enc, err := NewEncounter(&SetupInput{
		Field: sweepField(),
		Members: []MemberInput{
			{ID: sweepWatchA, Kind: KindPlayer, Position: spatial.Position{X: 0, Y: 1}},
			{ID: sweepWatchB, Kind: KindPlayer, Position: spatial.Position{X: 1, Y: 0}},
			{ID: sweepWalker, Kind: KindPlayer, Position: spatial.Position{X: 0, Y: 2}},
			{ID: sweepEast, Kind: KindPlayer, Position: spatial.Position{X: 10, Y: 1}},
		},
		Endings:      []EndingInput{{Key: "withdrawn", Trigger: TriggerExternal{}}},
		Capabilities: sweepCapabilities(),
	})
	require.NoError(t, err)

	return enc
}

// walkAround steps the walker n times round the west room's south-west
// corner and back, never into anybody's cell.
func walkAround(t *testing.T, enc *Encounter, n int) {
	t.Helper()
	there := enc.field.cellAt(spatial.Position{X: 1, Y: 2})
	home := enc.field.cellAt(spatial.Position{X: 0, Y: 2})
	for i := 0; i < n; i++ {
		to := there
		if i%2 == 1 {
			to = home
		}
		_, err := enc.Step(&StepInput{Member: sweepWalker, To: to, EndWalk: true})
		require.NoError(t, err, "step %d", i)
	}
}

// unlearned is THE ORACLE: every floor cell an observer's sight reaches now
// — the full sweep's own test, asked of every cell — that it does not know.
// Empty for every observer after every verb is "the skip changed nothing".
func unlearned(t *testing.T, enc *Encounter, observer MemberID) []spatial.Position {
	t.Helper()
	cells, err := enc.sightNow()
	require.NoError(t, err)
	positions := map[MemberID]spatial.Position{}
	for id := range enc.members {
		if at, ok := enc.canvas.GetEntityPosition(string(id)); ok {
			positions[id] = at
		}
	}
	reach := enc.newSightReach(positions, cells)
	hidden := enc.hiddenFrom(observer)
	known := enc.world.knownCells(observer)
	var out []spatial.Position
	for _, cell := range enc.field.cells {
		if !known[cell] && reach.seesCell(observer, cell, hidden) {
			out = append(out, cell)
		}
	}

	return out
}

func requireNothingUnlearned(t *testing.T, enc *Encounter, when string) {
	t.Helper()
	for _, m := range sweepMembers {
		require.Empty(t, unlearned(t, enc, m), "%s: %s knows every cell its sight reaches", when, m)
	}
}

// TestAnObserverWhoHasNotMovedSweepsOnce: the walker takes N steps round the
// west room; the watchers, who stand still, swept at first light and never
// again. Mutant (the skip removed): every step's sight passes sweep them
// again, N times or more.
func TestAnObserverWhoHasNotMovedSweepsOnce(t *testing.T) {
	enc := sweepScene(t)
	for _, m := range []MemberID{sweepWatchA, sweepWatchB, sweepEast} {
		require.Equal(t, 1, enc.sightSweepsRun[m], "precondition: %s swept at first light", m)
	}
	walker := enc.sightSweepsRun[sweepWalker]

	const n = 6
	walkAround(t, enc, n)

	for _, m := range []MemberID{sweepWatchA, sweepWatchB, sweepEast} {
		require.Equal(t, 1, enc.sightSweepsRun[m], "%s has not moved and does not sweep again", m)
	}
	require.GreaterOrEqual(t, enc.sightSweepsRun[sweepWalker]-walker, n, "the walker sweeps from every cell it steps to")
	requireNothingUnlearned(t, enc, "after the walk")
}

// TestADoorChangeReSweepsEveryObserverWhoCouldSeeIt: shut, the west
// watchers know nothing east of the neck door; opened by nobody — a
// world-side open teaches no opener — every watcher who did not move sweeps
// again and learns the far room down the neck, and the east watcher learns
// the west. Mutant (the door left out of the opacity): nobody's key changes,
// nobody sweeps, and the far room stays unknown.
func TestADoorChangeReSweepsEveryObserverWhoCouldSeeIt(t *testing.T) {
	enc := sweepScene(t)
	farEast := enc.field.cellAt(spatial.Position{X: 9, Y: 1})
	farWest := enc.field.cellAt(spatial.Position{X: 2, Y: 1})
	for _, m := range []MemberID{sweepWatchA, sweepWatchB, sweepWalker} {
		require.False(t, enc.world.knownCells(m)[farEast], "precondition: shut, %s knows nothing of the east room", m)
	}
	require.False(t, enc.world.knownCells(sweepEast)[farWest], "precondition: shut, the east watcher knows nothing of the west room")
	before := map[MemberID]int{}
	for _, m := range sweepMembers {
		before[m] = enc.sightSweepsRun[m]
	}

	_, err := enc.OpenDoor(&OpenDoorInput{Door: sweepNeckDor})
	require.NoError(t, err)

	require.True(t, enc.world.knownCells(sweepWatchA)[farEast], "watch-a sees down the neck into the east room")
	require.True(t, enc.world.knownCells(sweepEast)[farWest], "the east watcher sees down the neck into the west room")
	requireNothingUnlearned(t, enc, "after the open")
	for _, m := range sweepMembers {
		require.Greater(t, enc.sightSweepsRun[m], before[m], "the open re-sweeps %s, who has not moved", m)
	}

	// And then holds: nothing moved since the open.
	after := enc.sightSweepsRun[sweepWatchA]
	walkAround(t, enc, 2)
	require.Equal(t, after, enc.sightSweepsRun[sweepWatchA], "an open door, once swept, is not swept again")
	requireNothingUnlearned(t, enc, "after the walk")
}

// TestASaveWithoutSweepKeysSweepsOnceThenHolds: a blob written before the
// keys existed loads, and each watcher sweeps once on the first verb and
// then holds; a blob written with them loads and nobody who stood still
// sweeps at all. Mutants: keys not written by ToData (the second blob sweeps
// once); a missing key read as a match (the first blob never sweeps, which
// the oracle catches only if something changed, so the count pins it).
func TestASaveWithoutSweepKeysSweepsOnceThenHolds(t *testing.T) {
	t.Run("a save without keys", func(t *testing.T) {
		data := sweepScene(t).ToData()
		require.NotEmpty(t, data.SightSweeps, "precondition: a run with sight writes its keys")
		data.SightSweeps = nil
		enc, err := LoadEncounter(&LoadEncounterInput{Data: data, Capabilities: sweepCapabilities()})
		require.NoError(t, err)

		walkAround(t, enc, 4)
		for _, m := range []MemberID{sweepWatchA, sweepWatchB, sweepEast} {
			require.Equal(t, 1, enc.sightSweepsRun[m], "%s sweeps once on the first verb and then holds", m)
		}
		requireNothingUnlearned(t, enc, "after the walk")
	})

	t.Run("a save with keys", func(t *testing.T) {
		data := sweepScene(t).ToData()
		enc, err := LoadEncounter(&LoadEncounterInput{Data: data, Capabilities: sweepCapabilities()})
		require.NoError(t, err)

		walkAround(t, enc, 4)
		for _, m := range []MemberID{sweepWatchA, sweepWatchB, sweepEast} {
			require.Zero(t, enc.sightSweepsRun[m], "%s stood still across the save and does not sweep", m)
		}
		requireNothingUnlearned(t, enc, "after the walk")
	})
}

// TestAClearedAreaReSweepsOnTheNextVerb: an obscuring area stands in the
// neck when the door opens, so the west watchers learn nothing past it; the
// area clears — which refreshes no sight of its own — and on the walker's
// next step watch-a, who has not moved, sweeps and learns the east room.
// Mutant (the areas left out of the opacity): watch-a's key never changes
// and the east room stays unknown for as long as it stands still.
func TestAClearedAreaReSweepsOnTheNextVerb(t *testing.T) {
	enc := sweepScene(t)
	farEast := enc.field.cellAt(spatial.Position{X: 9, Y: 1})
	require.NoError(t, enc.AddSightArea(&SightAreaInput{
		ID: "fog", SourceID: "a-spell", Name: "fog", Center: enc.field.cellAt(spatial.Position{X: 5, Y: 1}), RadiusFeet: 5,
	}))
	_, err := enc.OpenDoor(&OpenDoorInput{Door: sweepNeckDor})
	require.NoError(t, err)
	require.False(t, enc.world.knownCells(sweepWatchA)[farEast], "precondition: the fog in the neck hides the east room")

	removed, err := enc.RemoveSightArea("a-spell")
	require.NoError(t, err)
	require.True(t, removed)
	walkAround(t, enc, 1)

	require.True(t, enc.world.knownCells(sweepWatchA)[farEast], "with the fog gone, watch-a sees down the neck")
	requireNothingUnlearned(t, enc, "after the fog clears")
}
