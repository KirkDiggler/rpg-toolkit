// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package dungeonspec

import (
	"sort"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter"
	"github.com/KirkDiggler/rpg-toolkit/tools/spatial"
)

// concealments.go is THE v2 LOWERING ONTO THE ONE CONCEALMENT PRIMITIVE
// (rpg-project#490, E5).
//
// The v2 dialect keeps the two words it always had — `regions[].concealed:
// true` and `doors[].concealed: [...]` — and that is deliberate rather than
// unfinished. They are the author's vocabulary for this dialect's geometry: a
// room made of painted cells, and a door standing in a crossing between two
// of them. What CHANGED is the thing underneath, and one lowering is where
// the two words meet the one noun.
//
// # The three rules, and the one thing they are all saying
//
//  1. A CONCEALED REGION becomes a concealment whose cells are the region's,
//     whose members are every concealed door with an edge touching it, and
//     whose checks are the union of those doors' approaches in authored
//     order — "beaten by any listed route" ([CheckApproach]), so a vault with
//     two ways in is found by either.
//  2. A CONCEALED DOOR TOUCHING NO CONCEALED REGION becomes a cell-less
//     concealment holding just that door — a hidden crossing, which is what
//     the author drew.
//  3. CONCEALED REGIONS JOINED BY A WAY THROUGH HIDDEN SPACE MERGE. This is
//     the case the two flags could express and one noun cannot: a secret
//     SUITE — several hidden rooms with ordinary doors and gaps between
//     them, reached from visible space through one hidden door — is ONE
//     secret, because a cell, a door and a prop belong to at most one
//     concealment (R4). The alternative was refusing a document v2 accepts
//     today, and the v2 validator says so in as many words: "everything
//     wholly inside hidden space is nobody's business".
//
// Rule 3 collapses a knowledge moment, and that is the primitive's own
// ruling rather than an accident here: finding any way into the suite reveals
// the suite. Under the two flags, each room arrived separately as its own
// door was perceived open. "One noun, everything hidden belongs to it."
//
// # Ids
//
// `<key>/<id>` throughout, the minting a door and an intel record already
// use, so two dungeons in one process cannot collide: a region concealment
// takes the FIRST merged region's id in authored order, and a lone door's
// takes the door's own. A file whose region and whose lone concealed door
// share a name earns the engine's duplicate-concealment refusal, by name.

// concealmentsOf lowers this document's two concealment words into the
// field's one concealment list, in authored region order and then authored
// door order. Nil when the file hides nothing, which is the same fact the
// absent keys are.
func concealmentsOf(spec *Spec, o encounter.Orientation, ways crossingWays) []encounter.ConcealmentInput {
	groups := mergeConcealedRegions(spec, o, ways)

	var out []encounter.ConcealmentInput
	claimed := make(map[int]bool, len(spec.Doors))
	for _, g := range groups {
		c := encounter.ConcealmentInput{ID: encounter.ConcealmentID(spec.Key + "/" + spec.Regions[g.regions[0]].ID)}
		for _, ri := range g.regions {
			for _, row := range spec.Regions[ri].Cells {
				for _, at := range row {
					c.Cells = append(c.Cells, authored(at))
				}
			}
		}
		for _, di := range g.doors {
			claimed[di] = true
			c.Doors = append(c.Doors, encounter.DoorID(spec.Key+"/"+spec.Doors[di].ID))
			c.Checks = append(c.Checks, approachesOf(spec.Doors[di].Concealed)...)
		}
		out = append(out, c)
	}

	// AND THE LONE HIDDEN CROSSINGS, after every room, so a file's
	// concealments read in the order its author would list them.
	for i, d := range spec.Doors {
		if d.Concealed == nil || claimed[i] {
			continue
		}
		id := encounter.DoorID(spec.Key + "/" + d.ID)
		out = append(out, encounter.ConcealmentInput{
			ID:     encounter.ConcealmentID(id),
			Checks: approachesOf(d.Concealed),
			Doors:  []encounter.DoorID{id},
		})
	}

	return out
}

// concealmentGroup is one lowered concealment before it is minted: the
// authored regions it covers and the authored doors that hide them, both by
// index and both in authored order.
type concealmentGroup struct {
	regions []int
	doors   []int
}

// crossingWays answers WHICH CROSSINGS ARE WAYS rather than walls — the one
// fact the suite merge needs about geometry, supplied by whichever caller
// already derived it ([Compile] from its wall derivation, [Validate] from the
// crossing map it built while reporting the file's own defects).
//
// A crossing is a way when no wall blocks it, or when a DOOR stands in it: a
// v2 door is a position on a wall, so the wall's crossing list still names the
// crossing the door opens.
type crossingWays struct {
	walled map[[2]spatial.Position]bool
	doors  map[[2]spatial.Position]bool
}

// open reports whether a crossing is a way.
func (w crossingWays) open(c [2]spatial.Position) bool { return !w.walled[c] || w.doors[c] }

// waysOf builds [crossingWays] from a wall derivation and the door crossings
// beside it — what [Compile] holds.
func waysOf(derived wallDerivation, doors map[[2]spatial.Position]encounter.DoorID) crossingWays {
	out := crossingWays{walled: map[[2]spatial.Position]bool{}, doors: map[[2]spatial.Position]bool{}}
	for _, w := range derived.Walls {
		for _, c := range w.Crossings {
			out.walled[c] = true
		}
	}
	for c := range doors {
		out.doors[c] = true
	}

	return out
}

// mergeConcealedRegions groups the concealed regions a way through hidden
// space joins, in authored order — rule 3.
//
// THE FLOOD ONLY EVER ENTERS HIDDEN SPACE: a concealed region's cell, or a
// scenery cell, which belongs to nobody and therefore hides nothing of its
// own but can still be the corridor between two secrets. It never steps onto
// a VISIBLE room's floor, so two secrets either side of a public hall stay two
// secrets — which is exactly what an author who walled them apart drew.
func mergeConcealedRegions(spec *Spec, o encounter.Orientation, ways crossingWays) []concealmentGroup {
	owner := concealedRegionOwners(spec, o)
	scenery := sceneryOwners(spec, o)

	// parent[i] is the region index i's group representative: itself until a
	// way merges it into an earlier one.
	parent := make([]int, len(spec.Regions))
	for i := range parent {
		parent[i] = i
	}
	var find func(int) int
	find = func(i int) int {
		if parent[i] != i {
			parent[i] = find(parent[i])
		}

		return parent[i]
	}
	union := func(a, b int) {
		// MERGE TOWARD THE EARLIER REGION, always, so the group's id is the
		// first authored one however the file was written down.
		ra, rb := find(a), find(b)
		if ra < rb {
			parent[rb] = ra
		} else {
			parent[ra] = rb
		}
	}

	// The flood: from every concealed cell, over every way into hidden
	// space, merging the concealed regions it reaches.
	for cell, home := range owner {
		floodHidden(cell, home, owner, scenery, ways, union)
	}

	at := map[int]int{}
	var out []concealmentGroup
	for i, r := range spec.Regions {
		if !r.Concealed {
			continue
		}
		root := find(i)
		if slot, seen := at[root]; seen {
			out[slot].regions = append(out[slot].regions, i)
			continue
		}
		at[root] = len(out)
		out = append(out, concealmentGroup{regions: []int{i}})
	}
	for i := range spec.Doors {
		if spec.Doors[i].Concealed == nil {
			continue
		}
		touched := concealedRegionsOfDoor(spec, o, owner, scenery, ways, i)
		if len(touched) == 0 {
			continue
		}
		slot := at[find(touched[0])]
		out[slot].doors = append(out[slot].doors, i)
	}

	return out
}

// floodHidden walks out from one cell of hidden space over ways into more of
// it, calling union with every concealed region it reaches.
//
// Bounded by the floor: scenery and concealed cells only, each visited once
// per start cell. A dungeon's hidden space is a handful of rooms, so this is
// cheap and, more to the point, it is the same reachability the v2 validator
// already reasons about — stated once here rather than re-derived.
func floodHidden(
	start spatial.Position, home int,
	owner map[spatial.Position]int, scenery map[spatial.Position]bool,
	ways crossingWays, union func(a, b int),
) {
	seen := map[spatial.Position]bool{start: true}
	queue := []spatial.Position{start}
	for len(queue) > 0 {
		cell := queue[0]
		queue = queue[1:]
		for _, next := range concealAdjacency.GetNeighbors(cell) {
			if seen[next] || !ways.open(normalizedCrossing(cell, next)) {
				continue
			}
			region, hidden := owner[next]
			if !hidden && !scenery[next] {
				continue
			}
			seen[next] = true
			queue = append(queue, next)
			if hidden {
				union(home, region)
			}
		}
	}
}

// concealAdjacency is the hex grid the flood asks for neighbours, span-free
// because the only thing asked of it is which six cells touch this one.
var concealAdjacency = spatial.NewAxialHexGrid(spatial.AxialHexGridConfig{SpanWidth: 1e6, SpanHeight: 1e6})

// sceneryOwners is every scenery cell, absolute — floor that belongs to
// nobody, which hides nothing of its own and can still join two secrets.
func sceneryOwners(spec *Spec, o encounter.Orientation) map[spatial.Position]bool {
	out := map[spatial.Position]bool{}
	for _, row := range spec.Scenery {
		for _, at := range row {
			out[encounter.HexCellAt(o, at[0], at[1])] = true
		}
	}

	return out
}

// concealedRegionOwners is every CONCEALED region's cells, absolute, to that
// region's index — the map a door's endpoints are looked up in. Unconcealed
// regions are deliberately absent: a door into visible space joins nothing.
func concealedRegionOwners(spec *Spec, o encounter.Orientation) map[spatial.Position]int {
	out := map[spatial.Position]int{}
	for i, r := range spec.Regions {
		if !r.Concealed {
			continue
		}
		for _, row := range r.Cells {
			for _, at := range row {
				out[encounter.HexCellAt(o, at[0], at[1])] = i
			}
		}
	}

	return out
}

// concealedRegionsOfDoor is the concealed regions one door GUARDS,
// deduplicated and in ascending index order — empty for a door that guards
// none, which is rule 2's lone hidden crossing.
//
// A DOOR GUARDS THE HIDDEN SPACE IT OPENS ONTO, not only a room its own
// crossing lands in. An endpoint standing in a concealed room is the ordinary
// secret door; an endpoint standing on SCENERY is the strip case the v2
// coherence walk already reasons about — "a way that runs THROUGH scenery"
// ([wayIn]) — and the room at the far end of that strip is the room this door
// hides. Reading only the two endpoint cells would leave such a room with no
// check anywhere and hand the composition a secret nobody could ever find.
//
// THE SAME CROSSING [doorsOf] COMPILES, arrived at by the same two lines, so
// a door cannot be lowered into one concealment and placed on another cell.
func concealedRegionsOfDoor(
	spec *Spec, o encounter.Orientation,
	owner map[spatial.Position]int, scenery map[spatial.Position]bool, ways crossingWays, i int,
) []int {
	g := geometryOf(o)
	d := spec.Doors[i]
	step, ok := g.stepAt(d.At.Offset)
	if !ok {
		return nil
	}
	here := encounter.HexCellAt(o, d.At.Cell[0], d.At.Cell[1])
	there := spatial.Position{X: here.X + float64(step[0]), Y: here.Y + float64(step[1])}

	seen := map[int]bool{}
	var out []int
	reach := func(_, region int) {
		if !seen[region] {
			seen[region] = true
			out = append(out, region)
		}
	}
	for _, cell := range []spatial.Position{here, there} {
		if r, hidden := owner[cell]; hidden {
			reach(0, r)
			continue
		}
		if scenery[cell] {
			floodHidden(cell, 0, owner, scenery, ways, reach)
		}
	}
	sort.Ints(out)

	return out
}

// concealmentHoldingDoor is every concealed door's AUTHORED id to the
// compiled concealment holding it — what a v2 `reveals: { door }` resolves
// through ([intelOf]).
//
// Built from the lowering itself rather than beside it, so a record and the
// field cannot disagree about which secret a door is part of.
func concealmentHoldingDoor(
	spec *Spec, o encounter.Orientation, ways crossingWays,
) map[string]encounter.ConcealmentID {
	out := map[string]encounter.ConcealmentID{}
	compiled := concealmentsOf(spec, o, ways)
	byDoorID := map[encounter.DoorID]encounter.ConcealmentID{}
	for _, c := range compiled {
		for _, id := range c.Doors {
			byDoorID[id] = c.ID
		}
	}
	for _, d := range spec.Doors {
		if id, held := byDoorID[encounter.DoorID(spec.Key+"/"+d.ID)]; held {
			out[d.ID] = id
		}
	}

	return out
}
