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
//   - ABSENT FROM EVERY DOOR-LIST: a concealed unfound door's doorways do
//     not appear in the atlas, nor the door in [Encounter.DoorsFor] —
//     presence in any list, even marked, leaks the secret.
//
//   - EVERY CROSSING INTO HIDDEN SPACE READS AS A WALL: an authored wall
//     with one endpoint in hidden space is presented rather than dropped, a
//     bare visible/hidden adjacency with nothing authored on it gets a
//     synthesized ordinary one, and a concealed unfound door's edge is
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
//     non-knower's cells, region entries and props are byte-identical to an
//     atlas in which the concealed region was never authored. Only the
//     BOUNDARY with visible space now differs from that never-authored
//     twin, because a wall cannot be authored with an off-floor endpoint
//     (the twin has nowhere to hang one), and an honestly authored dungeon
//     would still have walled the room that IS there.
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
func (e *Encounter) AtlasFor(member MemberID) (Atlas, error) {
	if _, ok := e.members[member]; !ok {
		return Atlas{}, fmt.Errorf("atlas for %q: %w", member, ErrNotMember)
	}

	full, err := e.Atlas()
	if err != nil {
		return Atlas{}, err
	}
	if e.world == nil {
		return full, nil
	}

	hiddenCells, hiddenRegions := e.hiddenFrom(member)
	unknownDoors := e.unknownDoorsFor(member)

	out := Atlas{
		Orientation: full.Orientation,
		// Carried through UNFILTERED: a way out is structure on the truth
		// grain, the same for every member (rpg-project#368). Every other
		// list below is rebuilt because concealment withholds part of it;
		// this one has nothing to withhold.
		Exits:      full.Exits,
		Cells:      make([]spatial.Position, 0, len(full.Cells)),
		Regions:    make([]AtlasRegion, 0, len(full.Regions)),
		Props:      make([]AtlasProp, 0, len(full.Props)),
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
	for _, r := range full.Regions {
		if !hiddenRegions[r.ID] {
			out.Regions = append(out.Regions, r)
		}
	}
	for _, p := range full.Props {
		if !hiddenCells[p.At] {
			out.Props = append(out.Props, p)
		}
	}

	// Boundaries, in three passes, then restored to the atlas's own sort — a
	// mask or a synthesized wall that sorted differently from an authored
	// one would mark itself by position in the list (rpg-toolkit#1419):
	//
	//  1. Every authored wall stands UNLESS it is wholly inside hidden
	//     space (both endpoints hidden) — the never-authored yardstick
	//     still governing a room's interior nobody visible borders.
	//  2. Every concealed unfound door's edge is masked as an ordinary
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
	// not, concealed or not — so a real doorway a member already knows
	// about never grows a phantom wall beside it, and a still-unfound
	// concealed door's edge is masked exactly once, by pass 2, never
	// twice.
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
	for _, id := range e.world.concealedDoors {
		if !unknownDoors[id] {
			continue
		}
		d := e.doorsByID[id]
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
// order [Encounter.Doors] uses: a concealed door the member has not had
// revealed is absent, and everything else — never-concealed doors included —
// is exactly what Doors reports. For a field with no concealment it IS
// Doors.
//
// Returns ErrNotMember for an ID this encounter does not hold.
func (e *Encounter) DoorsFor(member MemberID) ([]Door, error) {
	if _, ok := e.members[member]; !ok {
		return nil, fmt.Errorf("doors for %q: %w", member, ErrNotMember)
	}

	all := e.Doors()
	if e.world == nil {
		return all, nil
	}

	unknown := e.unknownDoorsFor(member)
	out := make([]Door, 0, len(all))
	for _, d := range all {
		if !unknown[d.ID] {
			out = append(out, d)
		}
	}
	return out, nil
}

// hiddenFrom is the member's hidden floor: every concealed region their fold
// does not show, and the union of those regions' cells.
func (e *Encounter) hiddenFrom(member MemberID) (map[spatial.Position]bool, map[RegionID]bool) {
	hiddenCells := make(map[spatial.Position]bool)
	hiddenRegions := make(map[RegionID]bool)
	for r := range e.world.concealedRegions {
		if e.world.knowsRegion(member, r) {
			continue
		}
		hiddenRegions[r] = true
		for _, c := range e.field.regionCells[r] {
			hiddenCells[c] = true
		}
	}
	return hiddenCells, hiddenRegions
}

// unknownDoorsFor is every concealed door the member's fold does not show.
func (e *Encounter) unknownDoorsFor(member MemberID) map[DoorID]bool {
	out := make(map[DoorID]bool)
	for _, id := range e.world.concealedDoors {
		if !e.world.knowsDoor(member, id) {
			out[id] = true
		}
	}
	return out
}

// maskHeight is the Height a synthetic boundary carries for one crossing
// AtlasFor is not presenting as authored — a concealed unfound door's edge
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
//     (design C19 — the wall a concealed door punctures is the wall it should
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
