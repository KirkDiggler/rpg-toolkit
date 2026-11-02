// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package encounter_test

// route_search_test.go is A ROUTE COSTS ITS REACH (rpg-project#762, R10): a
// route to one goal is searched toward, and the search is held to the flood
// it replaced — the same stop, the same path, cell for cell — on the castle
// and on the 10k keep, and to its reach: the cells it settles stay near the
// path it finds.

import (
	"fmt"
	"math/rand/v2"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter"
	"github.com/KirkDiggler/rpg-toolkit/tools/spatial"
)

// routeAsk is one route question: a mover from its own cell toward a goal —
// within `within` of anchor, or (remembered) exactly onto anchor with a
// creature there forgiven.
type routeAsk struct {
	mover      encounter.MemberID
	from       spatial.Position
	anchor     spatial.Position
	within     float64
	remembered bool
}

func (a routeAsk) String() string {
	if a.remembered {
		return fmt.Sprintf("%s from %v onto remembered %v", a.mover, a.from, a.anchor)
	}
	return fmt.Sprintf("%s from %v within %v of %v", a.mover, a.from, a.within, a.anchor)
}

// randomAsks draws n questions on enc: a random member from its own cell,
// toward a random cell its walk reaches (one in ten from anywhere on the
// floor, reachable or not, and one in five onto another member's cell, so
// the forgiven target and the creature-closed goal are both asked), at a
// reach of an exact cell, a step, two steps, or a bow's six.
func randomAsks(t *testing.T, enc *encounter.Encounter, seed uint64, n int) []routeAsk {
	t.Helper()
	members, err := enc.Members()
	require.NoError(t, err)
	require.NotEmpty(t, members)
	rng := rand.New(rand.NewPCG(seed, 762)) //nolint:gosec // a seeded property, not a secret
	reach := map[encounter.MemberID][]spatial.Position{}
	var floor []spatial.Position
	asks := make([]routeAsk, 0, n)
	for len(asks) < n {
		m := members[rng.IntN(len(members))]
		cells, ok := reach[m.ID]
		if !ok {
			cells, _, err = enc.ExportedReachable(m.ID, m.Position)
			require.NoError(t, err)
			reach[m.ID] = cells
			if len(cells) > len(floor) {
				floor = cells
			}
		}
		pool := cells
		if rng.IntN(10) == 0 {
			pool = floor
		}
		anchor := pool[rng.IntN(len(pool))]
		if rng.IntN(5) == 0 {
			anchor = members[rng.IntN(len(members))].Position
		}
		ask := routeAsk{mover: m.ID, from: m.Position, anchor: anchor}
		switch rng.IntN(5) {
		case 0:
			ask.remembered = true
		case 1:
			ask.within = 1
		case 2:
			ask.within = 2
		case 3:
			ask.within = 6
		}
		asks = append(asks, ask)
	}

	return asks
}

// searchAnswer and floodAnswer are one question asked of the search and of
// the flood it replaced.
func searchAnswer(t *testing.T, enc *encounter.Encounter, a routeAsk) ([]spatial.Position, bool, int) {
	t.Helper()
	var (
		path    []spatial.Position
		ok      bool
		settled int
		err     error
	)
	if a.remembered {
		path, ok, settled, err = enc.ExportedRouteToRemembered(a.mover, a.from, a.anchor)
	} else {
		path, ok, settled, err = enc.ExportedRouteWithin(a.mover, a.from, a.anchor, a.within)
	}
	require.NoError(t, err, "search: %s", a)

	return path, ok, settled
}

func floodAnswer(t *testing.T, enc *encounter.Encounter, a routeAsk) ([]spatial.Position, bool) {
	t.Helper()
	var (
		path []spatial.Position
		ok   bool
		err  error
	)
	if a.remembered {
		path, ok, err = enc.ExportedFloodToRemembered(a.mover, a.from, a.anchor)
	} else {
		path, ok, err = enc.ExportedFloodWithin(a.mover, a.from, a.anchor, a.within)
	}
	require.NoError(t, err, "flood: %s", a)

	return path, ok
}

// TestRouteTo_EqualsFlood is the property: for random movers and goals on
// the castle and the 10k garrison keep, the search answers what the flood
// answers. Both find a route or neither does; both paths satisfy the goal,
// walk one step at a time and are the same length; and the paths are
// IDENTICAL — not only where the flood's path is unique, but everywhere,
// because the search keeps the flood's own predecessor (route_search.go's
// doc says why it can). A tie broken differently would fail here, and would
// have shown as a changed golden.
func TestRouteTo_EqualsFlood(t *testing.T) {
	for _, tc := range []struct {
		name   string
		open   func(t *testing.T) *encounter.Encounter
		asks   int
		noRace bool
	}{
		{"castle", func(t *testing.T) *encounter.Encounter { enc, _ := openSite(t, 0, true); return enc }, underRace(600, 60), false},
		// The keep's oracle floods 10k hexes per question: ~30x slower
		// under -race, so plain go test runs it and -race runs the castle.
		{"keep-10k", func(t *testing.T) *encounter.Encounter { return openDriven(t, garrisonKeep(t)) }, 300, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if tc.noRace && raceDetector {
				t.Skip("the 10k keep's flood oracle is ~30x slower under -race; plain go test runs it")
			}
			enc := tc.open(t)
			routed, unique := 0, 0
			for _, a := range randomAsks(t, enc, 762, tc.asks) {
				got, gotOK, _ := searchAnswer(t, enc, a)
				want, wantOK := floodAnswer(t, enc, a)
				require.Equal(t, wantOK, gotOK, "found a route: %s", a)
				require.Len(t, got, len(want), "path length: %s", a)
				require.Equal(t, want, got, "the flood's path: %s", a)
				if !gotOK || len(got) == 0 {
					continue
				}
				routed++
				if a.within == 0 {
					unique++
				}
				end := got[len(got)-1]
				if a.remembered {
					require.Equal(t, a.anchor, end, "a remembered route ends on its cell: %s", a)
				} else {
					require.LessOrEqual(t, enc.Distance(end, a.anchor), a.within, "the route ends within reach: %s", a)
				}
				prev := a.from
				for _, cell := range got {
					require.Equal(t, 1.0, enc.Distance(prev, cell), "one step at a time: %s at %v", a, cell)
					prev = cell
				}
			}
			require.Greater(t, routed, tc.asks/2, "precondition: most questions have a route")
			require.Positive(t, unique, "precondition: some questions are exact-cell routes")
		})
	}
}

// TestRouteTo_Deterministic is C8: the same question, asked twice of one
// encounter and once of a second opened from the same document, gets the
// same answer — the same path, and the same cells settled finding it.
func TestRouteTo_Deterministic(t *testing.T) {
	enc, compiled := openSite(t, 0, true)
	twin := openCompiled(t, compiled, seatedParty(compiled), true, findsNothing{})
	for _, a := range randomAsks(t, enc, 8, underRace(200, 30)) {
		first, firstOK, firstSettled := searchAnswer(t, enc, a)
		again, againOK, againSettled := searchAnswer(t, enc, a)
		other, otherOK, otherSettled := searchAnswer(t, twin, a)
		require.Equal(t, firstOK, againOK, "asked twice: %s", a)
		require.Equal(t, first, again, "asked twice: %s", a)
		require.Equal(t, firstSettled, againSettled, "asked twice, the same search: %s", a)
		require.Equal(t, firstOK, otherOK, "asked of a twin: %s", a)
		require.Equal(t, first, other, "asked of a twin: %s", a)
		require.Equal(t, firstSettled, otherSettled, "asked of a twin, the same search: %s", a)
	}
}

// underRace is n questions, or under -race (~30x slower) the fewer that
// keep the package inside CI's timeout: the mutants this file was proved
// against fail at both.
func underRace(n, race int) int {
	if raceDetector {
		return race
	}
	return n
}

// routeReachK is the bound a route's reach is held to on the open keep: the
// cells one search settles, at most this many per step of the path it finds.
// Measured over 1000 random exact-cell routes from the party's seat on the
// 10k keep (rpg-project#762): median 10, 90th percentile 26, worst 44 — a
// route that detours round a wing's wall settles the wing's near side — and
// 15.2 across all of them, against a flood's 9540 cells for every one.
const routeReachK = 48

// routeReachAggregateK is the same bound over all the routes together.
const routeReachAggregateK = 20

// TestRouteTo_VisitsNearThePath is the probe counter: on the open 10k keep
// (no creatures), every route to a random reachable cell settles at most
// routeReachK cells per step of its path, the routes together at most
// routeReachAggregateK, and none settles half the floor a flood would.
func TestRouteTo_VisitsNearThePath(t *testing.T) {
	enc := sharedKeep10k(t)
	p := keepPlayers[0]
	from := whereIs(t, enc, p)
	cells, _, err := enc.ExportedReachable(p, from)
	require.NoError(t, err)
	require.Greater(t, len(cells), 9000, "precondition: the open keep is the whole 10k floor to a flood")
	rng := rand.New(rand.NewPCG(11, 1)) //nolint:gosec // a seeded property, not a secret
	settledAll, stepsAll := 0, 0
	for i := 0; i < underRace(400, 60); i++ {
		goal := cells[rng.IntN(len(cells))]
		path, ok, settled, err := enc.ExportedRouteWithin(p, from, goal, 0)
		require.NoError(t, err)
		if goal == from {
			continue
		}
		require.True(t, ok, "precondition: %v is reachable", goal)
		require.LessOrEqual(t, settled, routeReachK*len(path), "route to %v: %d settled for %d steps", goal, settled, len(path))
		require.Less(t, settled, len(cells)*6/10, "route to %v settles less than the floor", goal)
		settledAll += settled
		stepsAll += len(path)
	}
	require.LessOrEqual(t, settledAll, routeReachAggregateK*stepsAll, "all routes: %d settled for %d steps", settledAll, stepsAll)
}
