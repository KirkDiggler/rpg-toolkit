// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package encounter

import (
	"github.com/KirkDiggler/rpg-toolkit/core"
	"github.com/KirkDiggler/rpg-toolkit/mind/perception"
	"github.com/KirkDiggler/rpg-toolkit/tools/spatial"
)

// sightReach is the geometry [Encounter.rebuildPercepts] hands to
// mind/perception: the whole of what "can this observer see that subject"
// means here, and nothing else.
//
// A VALUE BUILT FOR ONE PASS, not a field on [Encounter]. Every term in it is
// a READING — where everybody stands, how far each of them can see this
// refresh — and a reach assembled from readings taken at different moments
// would be internally inconsistent: a stale distance judged against a world
// that has since moved on. That internal consistency is the property that
// matters; nothing here bounds how many times the capability may be asked,
// only that the readings inside one pass agree with each other. A reach that
// outlived its own pass would be the smallest possible version of the dual
// state [Sight] exists to avoid.
//
// Positions are looked up ONCE PER MEMBER, before the pass, rather than once
// per (observer, subject) pair — the N² that rpg-toolkit#1691 removed.
type sightReach struct {
	// positions is every PLACED member's cell. An absent key means the
	// member is not on the canvas, which is not a distance question.
	positions map[MemberID]spatial.Position
	// cells is how far each member can see this pass, from
	// [Encounter.sightNow].
	cells map[MemberID]int
	// canvas is THE MAP, for the grid's distance and the void-aware line of
	// sight — the same room every other geometry question in this pass is
	// asked of.
	canvas *canvasRoom
}

// Reaches answers with exactly the two filters the nested loop applied, in
// the same order.
//
// Too far BEFORE blocked: both filters are geometric, and this one is
// arithmetic while the next one walks a ray. Order is a cost decision, not a
// correctness one — either filter alone drops the subject. Strictly greater,
// because a member exactly at the edge of your sight is inside it.
//
// Two members can answer differently, so A may reach B without B reaching A.
// That asymmetry is real and it is NOT rpg-toolkit#1020: geometry stays
// mutual (spatial v0.9.1 pins it), and what differs is the per-member
// distance [Sight] supplied.
//
// The channel is ignored because this composition has exactly one — sight —
// and a second one would bring its own physics rather than a branch here.
//
// AN UNPLACED OBSERVER OR SUBJECT REACHES NOTHING. For a subject that is the
// same answer the old loop gave by skipping it. For an observer it is a
// backstop and nothing more: an unplaced observer never enters
// [perception.Pass.Observers] at all, because an observer that is IN the pass
// with no reach fades everything it holds (R4) — which is the opposite of
// what this composition has always done for one it could not place. See
// [Encounter.rebuildPercepts].
func (s sightReach) Reaches(_ perception.Channel, observer, subject core.EntityID) bool {
	observerCell, placed := s.positions[observer]
	if !placed {
		return false
	}
	subjectCell, placed := s.positions[subject]
	if !placed {
		return false
	}

	if s.canvas.GetGrid().Distance(observerCell, subjectCell) > float64(s.cells[observer]) {
		return false
	}

	return !s.canvas.IsLineOfSightBlocked(observerCell, subjectCell)
}
