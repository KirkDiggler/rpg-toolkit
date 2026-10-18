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
	areas  map[string]SightArea

	// suppressed is every (sneaking subject, watcher) pair the stored
	// verdicts hide — a standing won Stealth verdict (R11), or an armed pair
	// outside the watcher's zone (R9, R14) — sneak.go's
	// [Encounter.standingSuppression]: subject first, then watcher. Seeded from the stored verdicts by [Encounter.newSightReach],
	// re-set for the pass's own pairs by [Encounter.judgeSneakers] on the
	// reach the pass is handed BEFORE anything copies it, and read only by
	// [sightReach.Reaches] — [sightReach.reachesCell] stays the one sight test
	// zones, fog and discovery ask. Nil when no pair is hidden.
	suppressed map[MemberID]map[MemberID]bool

	// cones is each watcher's supplied arc and facing, read by
	// [sightReach.inWatchZone] alone (sneak.go, design R14). Set only on a
	// reach a zone is asked of — the pass's, by [Encounter.judgeSneakers],
	// and [Encounter.watchReach]'s — and never by [Encounter.newSightReach],
	// so no other sight question can learn an arc exists. Nil is everybody
	// watching all round.
	cones map[MemberID]watchCone
}

// newSightReach is THE ONE CONSTRUCTION of a pass's sight geometry: the
// readings the caller took (where each placed member stands, how far each can
// see this refresh) over the live canvas and the sight areas standing now.
// [Encounter.rebuildPercepts] and [Encounter.VisibleCellsFor] both build their
// reach here, so seen-now and learning cannot be asked of two geometries
// (rpg-project#586 R5 — one sight test).
//
// EVERY REACH CARRIES THE STANDING WON VERDICTS (sneaking, lead ruling on
// rpg-toolkit#2078): suppression is ONE answer to "does this watcher see this
// member", so a watcher a sneaker beat is not a seer for any question built
// here — the pass, a door's or a deed's witnesses ([Encounter.subjectSeers]),
// a pointed [Encounter.SeesWithin]. The pass then re-judges its own
// observers' pairs over this ([Encounter.judgeSneakers]).
func (e *Encounter) newSightReach(positions map[MemberID]spatial.Position, cells map[MemberID]int) sightReach {
	return sightReach{
		positions:  positions,
		cells:      cells,
		canvas:     e.canvas,
		areas:      e.sightAreas,
		suppressed: e.standingSuppression(),
	}
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
	subjectCell, placed := s.positions[subject]
	if !placed {
		return false
	}
	// A WON STEALTH ROLL HIDES ONE PAIR, AND ONLY IT (design R11). The pass
	// then fades the watcher's holding of the sneaker to a ghost at the last
	// cell it saw; nothing is erased. An armed pair outside the watcher's
	// zone is hidden the same way (R9, R14): a sneaker behind an arc is in
	// the watcher's sight and not in its zone. Every other answer is the one
	// sight test: a lost pair is ordinary sight (R10), and with no arc
	// supplied a watcher's zone is its sight, so a sneaker outside it is
	// already unreached.
	if s.suppressed[subject][observer] {
		return false
	}

	return s.reachesCell(observer, subjectCell)
}

// reachesCell is the same sight question for floor and remembered locations.
// Discovery uses this answer, not a second visibility algorithm.
func (s sightReach) reachesCell(observer MemberID, subjectCell spatial.Position) bool {
	observerCell, placed := s.positions[observer]
	if !placed {
		return false
	}
	if s.canvas.GetGrid().Distance(observerCell, subjectCell) > float64(s.cells[observer]) {
		return false
	}

	if s.canvas.IsLineOfSightBlocked(observerCell, subjectCell) {
		return false
	}
	for _, area := range s.areas {
		if areaCrosses(area, observerCell, subjectCell, s.canvas.GetGrid()) {
			return false
		}
	}
	return true
}

// seesCell is reachesCell with ONE ADDITION, for a cell whose centre a
// sight-blocking footprint covers — a shut door leaf standing in its own hex,
// a pillar inside its hex. No sightline ever reaches such a centre, because
// the cover itself stands on it, so by reachesCell alone the cell could never
// be learned and the door or pillar on it never drawn (rpg-project#586; the
// law "a door on the edge of a known cell is presented", for a door that has
// no edge).
//
// SUCH A CELL IS SEEN WHEN ITS COVER'S FACE IS: when reachesCell reaches a
// floor neighbour that nothing but that same cover separates from it — no
// sight-blocking boundary on the shared edge, and no OTHER sight-blocking
// contributor across the neighbour-to-cell line. So a pillar is seen from
// beside it, a shut leaf from either side of it, and neither from across a
// wall. The answer is still asked only of reachesCell; this decides which
// cells it is asked of. Everything else answers exactly as reachesCell does.
//
// A COVER THE OBSERVER HAS NOT FOUND TEACHES NOTHING: hidden is that
// observer's own [hiddenView], and an unfound concealed door or prop is not a
// face they can see — otherwise the threshold under a secret leaf would read
// as open floor where the solid-wall twin has a wall's footing, which is
// exactly the tell concealment forbids.
//
// EXCEPT WHERE IT READS AS THE WALL IT STANDS IN (fog-of-war R9). An unfound
// door bound to a structural wall's opening masquerades as that wall, and the
// solid-wall twin's observer learns the hex behind the wall's face. So the
// leaf's face teaches its threshold here too — while the parent wall is not
// itself hidden from the observer — and [Encounter.AtlasFor] presents that
// threshold as the twin's would be until the door is found. Without the face
// the secret would be a hole in the wall's footing the twin does not have.
func (s sightReach) seesCell(observer MemberID, cell spatial.Position, hidden hiddenView) bool {
	if s.reachesCell(observer, cell) {
		return true
	}
	from, placed := s.positions[observer]
	if !placed || s.canvas.GetGrid().Distance(from, cell) > float64(s.cells[observer]+1) {
		return false
	}
	f := s.canvas.field
	coverPlaced, coverDoors, err := f.screensOf(cell)
	if err != nil {
		return false
	}
	// A hidden door is a face only as the wall it masquerades as, and only
	// while that wall is not itself hidden from this observer: a hidden parent
	// is no wall to them, and its twin has nothing there to teach. The door
	// footprint is the leaf that covers the threshold; its placed
	// representation adds no second face.
	masquerades := func(parent PropID, bound bool) bool {
		return bound && !hidden.props[parent]
	}
	faces := 0
	for id := range coverPlaced {
		if !hidden.props[id] {
			faces++
		}
	}
	for id := range coverDoors {
		if !hidden.doors[id] || masquerades(f.boundParentWall("", id)) {
			faces++
		}
	}
	if faces == 0 {
		return false
	}
	for _, neighbor := range adjacencyGrid.GetNeighbors(cell) {
		if !f.isFloor(neighbor) {
			continue
		}
		if b, ok := s.canvas.GetBoundary(neighbor, cell); ok && b.BlocksLineOfSight {
			continue
		}
		blocked, err := f.sightBlocksAlongExcept(neighbor, cell, coverPlaced, coverDoors)
		if err != nil || blocked {
			continue
		}
		if s.reachesCell(observer, neighbor) {
			return true
		}
	}
	return false
}
