// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package encounter

import (
	"fmt"
	"sort"

	"github.com/KirkDiggler/rpg-toolkit/tools/spatial"
)

// projection.go is THE NEVER-AUTHORED PROJECTION (rpg-project#351, review
// findings ratified; boundary rule revised by rpg-toolkit#1419): the field as
// ONE MEMBER knows it. [Encounter.Atlas] and [Encounter.Doors] stay what
// they were — the whole truth, the author's and the test's view — and these
// two reads apply the absence law on top:
//
//   - ABSENT FROM EVERY DOOR-LIST: an unfound hidden door's doorways do not
//     appear in the atlas, nor the door in [Encounter.DoorsFor] — presence
//     in any list, even marked, leaks the secret. A hidden PROP and a hidden
//     FOOTPRINT DOOR are withheld the same way, from [Atlas.Props] and
//     [Atlas.Placed], wherever they stand.
//
//   - EVERY CROSSING INTO HIDDEN SPACE READS AS A WALL: an authored wall
//     with one endpoint in hidden space is presented rather than dropped, a
//     bare visible/hidden adjacency with nothing authored on it gets a
//     synthesized ordinary one, and an unfound hidden door's edge is
//     masked the same way regardless of which side is hidden — the
//     door-between-two-visible-spaces case this always covered, and the
//     door-into-a-hidden-room case that used to drop the wall along with
//     the door. Both masks and synthesized walls stand at the neighbouring
//     authored run's height ([Encounter.maskHeight]) — standard height
//     inside a height-2 run would be a visible notch exactly where the
//     secret is (the Wave 1b pin). Kirk's ruling, hit concretely while
//     authoring: "if there is no wall but I cannot walk through it that is
//     a tell. a wall is a wall is a wall." Every room has walls; floor that
//     ends in nothing and still refuses a step is the anomaly the
//     never-authored yardstick exists to prevent, reintroduced by the
//     yardstick itself. A crossing wholly INSIDE hidden space — both
//     endpoints hidden — stays withheld: nobody standing in visible space
//     borders it, so there is nothing there for it to disguise.
//
//   - A PRESENTED WALL STANDS ON FLOOR THE RECIPIENT CAN SEE: every cell a
//     presented segment passes through is in the recipient's atlas as floor
//     nobody owns, even when the region that owns it is hidden
//     (rpg-project#360, design C18). Floor that stops one cell short of a wall
//     is a black sliver exactly where the secret is — the same tell the
//     boundary rule above exists to remove, one layer down. Only PRESENTED
//     walls foot: the footing of a withheld wall would trace the secret
//     itself.
//
//   - THE ROOM ITSELF STILL HIDES WITH ITS DOOR: the yardstick governs
//     SPACE AND CONTENTS, unmoved by the boundary rule above — a
//     non-knower's cells and props are byte-identical to an atlas in which
//     the hidden floor was never authored. Only the BOUNDARY with visible
//     space now differs from that never-authored twin, because a wall cannot
//     be authored with an off-floor endpoint (the twin has nowhere to hang
//     one), and an honestly authored dungeon would still have walled the
//     room that IS there.
//
//   - A REGION IS TRIMMED, NOT DROPPED (rpg-project#490). A region carried a
//     `concealed` flag until the concealment primitive landed, so hiding one
//     meant withholding its whole entry. A concealment hides CELLS, which
//     may be a region's or a slice of one, so the entry is rebuilt with its
//     hidden cells removed — and withheld entirely only when none survive,
//     which is exactly what the flag used to mean.
//
// One accepted disclosure, named so it is never mistaken for a bug: a FOUND
// door's doorways name one cell of hidden floor per entrance — knowing
// where a door is includes knowing it leads somewhere.

// AtlasFor returns the field snapshot as one member knows it: the same
// deterministic, construction-time answer [Encounter.Atlas] gives, with the
// concealed structure this member has not had revealed withheld under the
// absence law above. For a field with no concealment it IS Atlas —
// byte-identical, no world machinery consulted, because none was built.
//
// Door STATE is [Encounter.DoorsFor]'s business, exactly as it is for the
// unscoped pair; a member's knowledge changes which doors are listed, never
// what a snapshot promises.
//
// Returns ErrNotMember for an ID this encounter does not hold — a
// member-scoped answer for nobody is a question with no honest answer.
//
// # What is withheld
//
// [Atlas.Placed] IS FILTERED, and it used to be withheld wholesale
// (rpg-project#490, E1). The old rule was right for the shape it had: a
// footprint is a continuous rectangle with no anchor cell, and slicing one at
// a hidden region's edge would have invented geometry the author never drew,
// so the whole list went rather than be approximated. Nothing is sliced now.
// A placement is WITHHELD WHOLE when it belongs to an unfound concealment, or
// when any cell it stands on is hidden; every other placement is presented
// exactly as [Encounter.Atlas] reports it. Withholding the list wholesale
// stopped being honest the moment a footprint DOOR could be a secret: a
// dungeon whose tables all vanished the instant anything anywhere was hidden
// is itself a tell, and a bigger one than the rectangle it was protecting.
func (e *Encounter) AtlasFor(member MemberID) (Atlas, error) {
	if _, ok := e.members[member]; !ok {
		return Atlas{}, fmt.Errorf("atlas for %q: %w", member, ErrNotMember)
	}
	full, err := e.Atlas()
	if err != nil {
		return Atlas{}, err
	}
	if !e.world.conceals() {
		return full, nil
	}

	hidden := e.hiddenFrom(member)
	hiddenCells, unknownDoors := hidden.cells, hidden.doors

	out := Atlas{
		Orientation: full.Orientation,
		// Carried through UNFILTERED: a way out is structure on the truth
		// grain, the same for every member (rpg-project#368). Every other
		// list below is rebuilt because concealment withholds part of it;
		// this one has nothing to withhold.
		Exits: full.Exits,
		// The way in, carried through unfiltered for Exits' own reason: it
		// is structure, the same for every member. The pointer is the
		// snapshot's own — Atlas built it fresh — so sharing it here hands
		// nobody a route back into the field.
		Start:      full.Start,
		Cells:      make([]spatial.Position, 0, len(full.Cells)),
		Regions:    make([]AtlasRegion, 0, len(full.Regions)),
		Props:      make([]AtlasProp, 0, len(full.Props)),
		Placed:     make([]AtlasPlacedProp, 0, len(full.Placed)),
		Boundaries: make([]AtlasBoundary, 0, len(full.Boundaries)),
		Doorways:   make([]AtlasDoorway, 0, len(full.Doorways)),
		Segments:   make([]AtlasSegment, 0, len(full.Segments)),
	}

	// C18: a wall wholly inside hidden space is withheld with the room, and
	// every other wall is presented — standing on its own footprint, which
	// enters this atlas as floor nobody owns whatever the recipient may know
	// about the region underneath.
	footing := make(map[spatial.Position]bool)
	for i, seg := range e.field.segments {
		if e.field.segmentHidden(seg, hiddenCells) {
			continue
		}
		out.Segments = append(out.Segments, full.Segments[i])
		for _, c := range seg.Footprint {
			footing[e.field.cellAt(c)] = true
		}
	}

	// ONE PASS, TWO ANSWERS. Which cells this recipient is shown, and which of
	// those they cannot stand on: the ones nobody can, and the footing of a
	// presented wall whose owner they cannot see — ownerless floor to them,
	// which is exactly what scenery is. Both questions are asked of the same
	// cell in the same visit, because a second walk over the survivors asked
	// the map the same coordinates a second time (measured at a third of
	// AtlasFor on a dungeon ten times the reference tomb, with the two walks
	// between them).
	for _, c := range full.Cells {
		hidden := hiddenCells[c]
		if hidden && !footing[c] {
			continue
		}
		out.Cells = append(out.Cells, c)
		if hidden || !e.field.isStandable(c) {
			out.Sealed = append(out.Sealed, c)
		}
	}
	// A REGION TRIMMED TO WHAT THIS RECIPIENT CAN SEE, and dropped when
	// nothing of it survives — the never-authored twin of a room that is
	// wholly a secret has no entry for it at all.
	for _, r := range full.Regions {
		cells := make([]spatial.Position, 0, len(r.Cells))
		for _, c := range r.Cells {
			if !hiddenCells[c] {
				cells = append(cells, c)
			}
		}
		if len(cells) == 0 {
			continue
		}
		entry := r
		entry.Cells = cells
		out.Regions = append(out.Regions, entry)
	}
	for _, p := range full.Props {
		if !hiddenCells[p.At] && !hidden.props[p.ID] {
			out.Props = append(out.Props, p)
		}
	}
	// A PLACEMENT GOES WHOLE OR STAYS WHOLE. Withheld when it belongs to an
	// unfound concealment — which includes a hidden footprint DOOR, whose
	// rectangle rides this list under its own id — and withheld when any
	// cell it stands on is hidden, because a rectangle presented over a hole
	// in the floor marks the hole.
	for _, p := range full.Placed {
		if hidden.props[p.ID] || hidden.doors[p.ID] || e.placedTouchesHidden(p, hiddenCells) {
			continue
		}
		out.Placed = append(out.Placed, p)
	}

	// Boundaries, in three passes, then restored to the atlas's own sort — a
	// mask or a synthesized wall that sorted differently from an authored
	// one would mark itself by position in the list (rpg-toolkit#1419):
	//
	//  1. Every authored wall stands UNLESS it is wholly inside hidden
	//     space (both endpoints hidden) — the never-authored yardstick
	//     still governing a room's interior nobody visible borders.
	//  2. Every unfound hidden door's edge is masked as an ordinary
	//     wall unless it too is wholly inside hidden space, regardless of
	//     which single side is hidden — the fix: this used to mask only
	//     the two-visible-sides case and silently drop the rest.
	//  3. Every crossing the first two passes left untouched — a bare
	//     visible/hidden adjacency with nothing authored on it at all — is
	//     synthesized as an ordinary wall at THE SAME neighbouring run's
	//     height maskHeight gives pass 2's masks: a raised wall on one
	//     row of a seam and a bare gap on the next, both bordering the
	//     same hidden room, must read as one continuous run — a
	//     standard-height patch beside a height-2 neighbour would be the
	//     notch exactly where the secret is, on the very boundary this
	//     rule exists to make ordinary.
	//
	// doorEdge excludes every door's own crossing from pass 3 — found or
	// not, hidden or not — so a real doorway a member already knows about
	// never grows a phantom wall beside it, and a still-unfound hidden
	// door's edge is masked exactly once, by pass 2, never twice.
	for _, b := range full.Boundaries {
		if hiddenCells[b.From] && hiddenCells[b.To] {
			continue
		}
		out.Boundaries = append(out.Boundaries, b)
	}
	doorEdge := make(map[DoorEdge]bool, len(full.Doorways))
	for _, dw := range full.Doorways {
		doorEdge[normalizeDoorEdge(DoorEdge{From: dw.From, To: dw.To})] = true
	}
	for _, id := range sortedDoorIDs(unknownDoors) {
		d, ok := e.doorsByID[id]
		if !ok {
			continue
		}
		for _, edge := range d.edges {
			if hiddenCells[edge.From] && hiddenCells[edge.To] {
				continue
			}
			out.Boundaries = append(out.Boundaries, AtlasBoundary{
				From:              edge.From,
				To:                edge.To,
				BlocksMovement:    true,
				BlocksLineOfSight: true,
				Height:            e.maskHeight(edge),
			})
		}
	}
	authoredEdge := make(map[DoorEdge]bool, len(full.Boundaries))
	for _, b := range full.Boundaries {
		authoredEdge[normalizeDoorEdge(DoorEdge{From: b.From, To: b.To})] = true
	}
	for hidden := range hiddenCells {
		for _, neighbor := range adjacencyGrid.GetNeighbors(hidden) {
			if hiddenCells[neighbor] {
				continue
			}
			if _, floor := e.field.regionOf(neighbor); !floor {
				continue
			}
			edge := normalizeDoorEdge(DoorEdge{From: hidden, To: neighbor})
			if authoredEdge[edge] || doorEdge[edge] {
				continue
			}
			out.Boundaries = append(out.Boundaries, AtlasBoundary{
				From:              edge.From,
				To:                edge.To,
				BlocksMovement:    true,
				BlocksLineOfSight: true,
				Height:            e.maskHeight(edge),
			})
		}
	}
	sort.Slice(out.Boundaries, func(i, j int) bool {
		if out.Boundaries[i].From != out.Boundaries[j].From {
			return cellBefore(out.Boundaries[i].From, out.Boundaries[j].From)
		}
		return cellBefore(out.Boundaries[i].To, out.Boundaries[j].To)
	})

	for _, dw := range full.Doorways {
		if !unknownDoors[dw.Door] {
			out.Doorways = append(out.Doorways, dw)
		}
	}

	return out, nil
}

// DoorsFor reports every door AS ONE MEMBER KNOWS IT, in the same stable ID
// order [Encounter.Doors] uses: a hidden door the member has not had
// revealed is absent, and everything else — every door no concealment holds
// included — is exactly what Doors reports. For a field that hides nothing
// it IS Doors.
//
// Returns ErrNotMember for an ID this encounter does not hold.
func (e *Encounter) DoorsFor(member MemberID) ([]Door, error) {
	if _, ok := e.members[member]; !ok {
		return nil, fmt.Errorf("doors for %q: %w", member, ErrNotMember)
	}

	all := e.Doors()
	if !e.world.conceals() {
		return all, nil
	}

	unknown := e.hiddenFrom(member).doors
	out := make([]Door, 0, len(all))
	for _, d := range all {
		if !unknown[d.ID] {
			out = append(out, d)
		}
	}
	return out, nil
}

// hiddenView is everything one member has not found: the floor, the doors
// and the props. ONE WALK OVER THE CONCEALMENTS produces all three, because
// they are three readings of one fold and a second walk would be a second
// place for them to disagree about which secrets this member holds.
type hiddenView struct {
	cells map[spatial.Position]bool
	doors map[DoorID]bool
	props map[PropID]bool
}

// hiddenFrom folds the member's own knowledge into what their atlas
// withholds. A concealment they have found contributes nothing.
func (e *Encounter) hiddenFrom(member MemberID) hiddenView {
	out := hiddenView{
		cells: make(map[spatial.Position]bool),
		doors: make(map[DoorID]bool),
		props: make(map[PropID]bool),
	}
	for i := range e.field.concealments {
		c := &e.field.concealments[i]
		if e.world.knowsConcealment(member, c.id) {
			continue
		}
		for _, cell := range e.hiddenCellsOf(c) {
			out.cells[cell] = true
		}
		for _, id := range c.doors {
			out.doors[id] = true
		}
		for _, id := range c.props {
			out.props[id] = true
		}
	}

	return out
}

// masqueradeBlocks is THE MASQUERADE AS GEOMETRY (rpg-project#490, E7):
// whether a crossing reads as WALL in this member's own atlas without a wall
// standing there in the field.
//
// "A wall is a wall is a wall" cuts both ways. The rule that made every
// crossing into hidden space read as wall (rpg-toolkit#1419) was written
// against one half of the tell — floor that ends in nothing and still refuses
// a step. The other half is this one: a wall the picture shows and the
// geometry lets through. A client that trusts the atlas never offers the
// step, so the only caller who can take it is one that ignored the picture —
// and the server accepting it is both a tell (walk the perimeter, find the
// wall that is not there) and a cheat.
//
// THE THREE CASES MIRROR [Encounter.AtlasFor]'S OWN PASSES EXACTLY, because
// what this answers is "would that function draw a wall here for this
// member", and two computations of one truth is how a projection and a step
// learn to disagree:
//
//  1. A DOOR STANDS IN THE CROSSING. Masked (pass 2) exactly when the member
//     has not found the concealment holding it — and not masked when they
//     have, whichever side is hidden, because pass 3 excludes every door's
//     own crossing found or not. So a door they know about is the door's own
//     state's business and never this rule's.
//  2. ONE ENDPOINT HIDDEN, ONE NOT, and nothing standing in the crossing.
//     Synthesized as an ordinary wall (pass 3).
//  3. BOTH ENDPOINTS HIDDEN. Withheld, not masked — nobody visible borders
//     it — and the member cannot be standing on a hidden cell without
//     knowing it (presence pierces), so there is no crossing to judge.
//
// An AUTHORED wall on the crossing is real geometry and refuses through the
// canvas before this is ever consulted; answering true for one as well costs
// nothing and says the same thing.
func (e *Encounter) masqueradeBlocks(member MemberID, from, to spatial.Position) bool {
	if member == "" || !e.world.conceals() {
		return false
	}
	hidden := e.hiddenFrom(member)
	if hidden.cells[from] && hidden.cells[to] {
		return false
	}
	if door := e.doorOnEdge(from, to); door != nil {
		return hidden.doors[door.id]
	}

	return hidden.cells[from] != hidden.cells[to]
}

// placedTouchesHidden reports whether a placement stands on any cell this
// recipient cannot see. Asked of the rectangle's own cells
// ([field.placedCells]) — the one derivation reach, the probe law and an
// arrival fact all ask of a footprint, never a second measurement of it.
func (e *Encounter) placedTouchesHidden(p AtlasPlacedProp, hiddenCells map[spatial.Position]bool) bool {
	if len(hiddenCells) == 0 {
		return false
	}
	for _, cell := range e.field.placedCells(p.Placement) {
		if hiddenCells[cell] {
			return true
		}
	}

	return false
}

// sortedDoorIDs orders a door-id set, so a mask's position in the boundary
// list cannot depend on Go's map iteration (C8).
func sortedDoorIDs(ids map[DoorID]bool) []DoorID {
	out := make([]DoorID, 0, len(ids))
	for id := range ids {
		out = append(out, id)
	}
	sort.Strings(out)

	return out
}

// maskHeight is the Height a synthetic boundary carries for one crossing
// AtlasFor is not presenting as authored — an unfound hidden door's edge
// (pass 2) or a bare visible/hidden adjacency (pass 3): the height of the WALL
// STANDING THERE, so the synthetic boundary reads as part of it rather than as
// a notch exactly where the secret is (the Wave 1b pin, generalized by
// rpg-toolkit#1419).
//
// # The run is a first-class thing now, so it is not reconstructed
//
// This used to hunt: it collected every authored crossing separating the same
// two regions the edge does, took the nearest by hex distance, and called that
// "the run". It had to, because a run was not something the map held — the file
// listed crossings and the shape had to be inferred back out of them.
//
// The file holds the LINE now (rpg-project#360), so the answer is a lookup:
//
//  1. the segment the door hides in, when a door stands in this crossing
//     (design C19 — the wall a hidden door punctures is the wall it should
//     masquerade as, and the segment names its own doors);
//  2. otherwise the segment standing on either of the crossing's cells;
//  3. otherwise — a field built from crossings alone, with no lines authored at
//     all — the authored wall standing on either of those cells.
//
// Rule 3 is not a second mechanism for the same question: a field compiled from
// a dungeon always carries segments and never reaches it, and a field a host
// assembled by hand has no line to read a height off. Its answer is the same
// shape as the other two — the wall standing HERE, not the nearest wall
// somewhere else.
//
// A crossing with no wall anywhere near it masks at 0 — not authored, standard
// height — which is what an authored wall there would have said too.
func (e *Encounter) maskHeight(edge DoorEdge) float64 {
	if id, standing := e.doorInEdge(edge); standing {
		for _, seg := range e.field.segments {
			for _, d := range seg.DoorIDs {
				if d == id {
					return seg.Height
				}
			}
		}
	}
	for _, seg := range e.field.segments {
		for _, c := range seg.Footprint {
			if cell := e.field.cellAt(c); cell == edge.From || cell == edge.To {
				return seg.Height
			}
		}
	}
	for _, w := range e.field.walls {
		from, to := e.field.cellAt(w.From), e.field.cellAt(w.To)
		if from == edge.From || from == edge.To || to == edge.From || to == edge.To {
			return w.Height
		}
	}

	return 0
}

// doorInEdge is which door stands in a crossing, if any.
func (e *Encounter) doorInEdge(edge DoorEdge) (DoorID, bool) {
	want := normalizeDoorEdge(edge)
	for _, d := range e.doors {
		for _, have := range d.edges {
			if normalizeDoorEdge(have) == want {
				return d.id, true
			}
		}
	}

	return "", false
}

// segmentHidden reports whether a wall stands WHOLLY inside hidden space —
// every cell it passes through in a room this recipient cannot see. Such a wall
// is withheld with the room it is inside; every other one is presented, because
// a room the recipient CAN see is entitled to its walls.
//
// A segment with no footprint at all is presented: it stands on nothing that
// could be hidden, so there is nothing for it to trace.
func (f *field) segmentHidden(seg SegmentInput, hiddenCells map[spatial.Position]bool) bool {
	if len(seg.Footprint) == 0 {
		return false
	}
	for _, c := range seg.Footprint {
		if !hiddenCells[f.cellAt(c)] {
			return false
		}
	}

	return true
}
