// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package encounter

import (
	"fmt"
	"sort"

	"github.com/KirkDiggler/rpg-toolkit/tools/spatial"
)

// projection.go is THE NEVER-AUTHORED PROJECTION (rpg-project#351, review
// findings ratified): the field as ONE MEMBER knows it. [Encounter.Atlas]
// and [Encounter.Doors] stay what they were — the whole truth, the author's
// and the test's view — and these two reads apply the absence law on top:
//
//   - ABSENT FROM EVERY DOOR-LIST: a concealed unfound door's doorways do
//     not appear in the atlas, nor the door in [Encounter.DoorsFor] —
//     presence in any list, even marked, leaks the secret.
//
//   - MASKED AS WALL, IN ONE CASE ONLY: for a concealed unfound door
//     between two spaces the member can see, the atlas gains a synthetic
//     ordinary boundary at the door's edges, indistinguishable from an
//     authored wall (nothing marks it), at the neighbouring authored run's
//     height — a standard-height mask inside a height-2 run would be a
//     visible notch exactly where the secret is (the Wave 1b pin).
//
//   - THE ROOM HIDES WITH ITS DOOR, by the never-authored yardstick: a
//     non-knower's answer is byte-identical to an atlas in which the
//     concealed region was never authored — its cells, its region entry,
//     its props, and EVERY boundary touching its cells withheld, border
//     walls included. No mask at a hidden region's border: the space
//     already reads as solid mass, and a synthetic wall touching void would
//     carry the same self-marking signature the yardstick exists to remove.
//     A boundary shared with a still-hidden neighbour stays withheld even
//     after this region is revealed — the member-scoped answer governs.
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
		Cells:       make([]spatial.Position, 0, len(full.Cells)),
		Regions:     make([]AtlasRegion, 0, len(full.Regions)),
		Props:       make([]AtlasProp, 0, len(full.Props)),
		Boundaries:  make([]AtlasBoundary, 0, len(full.Boundaries)),
		Doorways:    make([]AtlasDoorway, 0, len(full.Doorways)),
	}

	for _, c := range full.Cells {
		if !hiddenCells[c] {
			out.Cells = append(out.Cells, c)
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

	// Boundaries: withhold every one touching hidden floor, then mask the
	// concealed unfound doors that stand between two visible spaces, then
	// restore the atlas's own sort — a mask that sorted differently from an
	// authored wall would mark itself by position in the list.
	for _, b := range full.Boundaries {
		if hiddenCells[b.From] || hiddenCells[b.To] {
			continue
		}
		out.Boundaries = append(out.Boundaries, b)
	}
	for _, id := range e.world.concealedDoors {
		if !unknownDoors[id] {
			continue
		}
		d := e.doorsByID[id]
		for _, edge := range d.edges {
			if hiddenCells[edge.From] || hiddenCells[edge.To] {
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

// maskHeight is the Height the synthetic mask carries for one concealed
// unfound door edge: the neighbouring authored run's, so the mask reads as a
// continuation of the wall it hides in (the Wave 1b pin — walls carry
// per-edge height, and a standard-height mask inside a height-2 run is a
// visible notch exactly where the secret is).
//
// The mechanism, since "the run" is not a first-class thing on this map: the
// run a door punctures is the authored walls separating the same two regions
// its edge does, and the mask takes the height of the NEAREST of them —
// nearest by hex distance between crossing endpoints, ties broken by the
// atlas's own boundary order so the answer cannot move between calls. A door
// with no such wall (a crossing in an unwalled seam) masks at 0 — not
// authored, standard height — which is what an authored wall there would
// have said too.
func (e *Encounter) maskHeight(edge DoorEdge) float64 {
	pairA, okA := e.field.regionOf(edge.From)
	pairB, okB := e.field.regionOf(edge.To)
	if !okA || !okB {
		return 0
	}

	type candidate struct {
		from, to spatial.Position
		height   float64
	}
	var run []candidate
	for _, w := range e.field.walls {
		wFrom, wTo := e.field.cellAt(w.From), e.field.cellAt(w.To)
		rFrom, okF := e.field.regionOf(wFrom)
		rTo, okT := e.field.regionOf(wTo)
		if !okF || !okT {
			continue
		}
		sameRun := (rFrom == pairA && rTo == pairB) || (rFrom == pairB && rTo == pairA)
		if !sameRun {
			continue
		}
		norm := normalizeDoorEdge(DoorEdge{From: wFrom, To: wTo})
		run = append(run, candidate{from: norm.From, to: norm.To, height: w.Height})
	}
	if len(run) == 0 {
		return 0
	}

	sort.Slice(run, func(i, j int) bool {
		if run[i].from != run[j].from {
			return cellBefore(run[i].from, run[j].from)
		}
		return cellBefore(run[i].to, run[j].to)
	})

	best, bestDist := 0, -1.0
	for i, c := range run {
		d := crossingDistance(edge, DoorEdge{From: c.from, To: c.to})
		if bestDist < 0 || d < bestDist {
			best, bestDist = i, d
		}
	}
	return run[best].height
}

// crossingDistance is how far apart two crossings stand: the smallest hex
// distance between any endpoint of one and any endpoint of the other.
func crossingDistance(a, b DoorEdge) float64 {
	dist := adjacencyGrid.Distance(a.From, b.From)
	for _, pair := range [][2]spatial.Position{{a.From, b.To}, {a.To, b.From}, {a.To, b.To}} {
		if d := adjacencyGrid.Distance(pair[0], pair[1]); d < dist {
			dist = d
		}
	}
	return dist
}
