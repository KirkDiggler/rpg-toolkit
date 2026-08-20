// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package dungeonspec_test

// The three capabilities every encounter requires, supplied explicitly.
//
// Stated out loud rather than defaulted, which is the whole point of them being
// required (rpg-toolkit#1033 — capabilities are supplied, never defaulted). This
// package's scenes are about GEOMETRY: what the compiler built, and whether a
// player can walk it. So each of these answers the least interesting thing it
// can, and says so — an unbounded sight range and a roster where nobody is down
// mean no assertion in tomb_test.go is secretly about light or hit points.
//
// They are copies of the encounter package's own test fixtures rather than
// shared ones, because test helpers do not cross a package boundary and
// exporting them to share here would put a test-only surface on a published
// module.

import "github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter"

// unlimitedSight is far enough that no scene here is bounded by it.
const unlimitedSight = 1_000_000

// everyoneSeesTheWholeMap installs a sight model with no distance term, so
// every sighting in these scenes is decided by geometry alone — walls, doors,
// props and void — which is exactly what the world-model wave was about.
type everyoneSeesTheWholeMap struct{}

func (everyoneSeesTheWholeMap) Sight(members []encounter.MemberID) (map[encounter.MemberID]int, error) {
	out := make(map[encounter.MemberID]int, len(members))
	for _, id := range members {
		out[id] = unlimitedSight
	}

	return out, nil
}

// everyoneStanding reports nobody down. The tomb's scenes never drop anybody,
// and a body that was silently treated as an enemy would form fights this file
// does not mean to be about.
type everyoneStanding struct{}

func (everyoneStanding) Standing([]encounter.MemberID) ([]encounter.MemberID, error) {
	return nil, nil
}

// orderAsGiven returns the roster untouched. These scenes assert that a fight
// FORMED, never what order it formed in, and a shuffle would make the assertion
// depend on a roll nobody is testing.
type orderAsGiven struct{}

func (orderAsGiven) RollInitiative(members []encounter.MemberID) ([]encounter.MemberID, error) {
	return members, nil
}
