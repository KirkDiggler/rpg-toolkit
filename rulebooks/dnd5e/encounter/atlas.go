// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package encounter

import (
	"fmt"
	"sort"

	"github.com/KirkDiggler/rpg-toolkit/tools/spatial"
)

// Atlas is a deterministic, construction-time snapshot of the field
// projected into dungeon-absolute space: every region's absolute footprint
// and occluders, and every connection's absolute doorway pair (#929 T3 —
// the read surface promised by RoomInput.Origin's doc comment).
type Atlas struct {
	// Regions is every region, sorted by region ID (C8).
	Regions []AtlasRegion

	// Doorways is every connection's absolute endpoint pair, sorted by
	// connection ID (C8).
	Doorways []AtlasDoorway
}

// AtlasRegion is one region: a NAMED SET OF CELLS, described rather than
// enumerated (rpg-toolkit#1108).
//
// Grid, Origin, Width and Height ARE the cell set — they say exactly which
// cells belong to this region, in the same terms the region's own grid uses to
// decide it, and [Encounter.RegionAt] answers membership from them without
// anybody building a list. A host that genuinely wants the cells (a renderer
// laying out a floor) can walk that rectangle itself; every other caller was
// paying for one.
type AtlasRegion struct {
	// ID is the region's identifier — the authored room's own ID.
	ID RegionID

	// Grid is the region's coordinate family (GridShapeSquare or GridShapeHex).
	Grid spatial.GridShape

	// Origin is the region's dungeon-absolute anchor (RoomInput.Origin).
	Origin spatial.Position

	// Width is the region's horizontal dimension, in cells.
	Width int

	// Height is the region's vertical dimension, in cells.
	Height int

	// Occluders is the region's line-of-sight-blocking cells, in
	// dungeon-absolute space. An occluder is still a cell OF the region —
	// occlusion is walkability, not ownership (#929 T3 ruling 1), and
	// RegionAt names an occluded cell's region like any other.
	Occluders []spatial.Position

	// Boundaries is the region's walls (RoomInput.Boundaries), with both
	// endpoints projected into dungeon-absolute space (#929 hardening round
	// A) — the SAME projection compileCanvas registers them by, so what a
	// host draws and what the encounter enforces are the same edges.
	//
	// An endpoint may belong to the region NEXT DOOR: a chamber walls its own
	// edge by naming the cell beyond it, which is what makes a wall between
	// two authored rooms expressible at all (rpg-toolkit#1106,
	// RoomInput.Boundaries' doc comment). Grouping such a wall under the region
	// that DECLARED it is construction truth reported faithfully, not a claim
	// about which region the wall belongs to. In declaration order (RoomInput's
	// own order, the same construction-truth ordering Occluders already
	// uses above) — deterministic given a fixed input (C8), though not
	// independently sorted the way Regions/Doorways are.
	Boundaries []AtlasBoundary
}

// AtlasBoundary is one wall or barrier crossing, with both endpoints
// projected into dungeon-absolute space (#929 hardening round A;
// spatial.Boundary's doc comment for the room-local fields this mirrors).
type AtlasBoundary struct {
	// From is one endpoint of the crossing, in dungeon-absolute space.
	From spatial.Position

	// To is the other endpoint of the crossing, in dungeon-absolute space.
	To spatial.Position

	// BlocksMovement reports whether an entity may cross this boundary.
	BlocksMovement bool

	// BlocksLineOfSight reports whether line of sight may cross this boundary.
	BlocksLineOfSight bool
}

// AtlasDoorway is one connection's absolute endpoint pair.
//
// Both endpoint cells belong to their own regions and nothing sits between
// them (W3) — a doorway is an opening in a wall, not a cell of its own. See
// [Encounter.RegionAt] for what that means for somebody standing in one.
type AtlasDoorway struct {
	// Connection is the connection's identifier.
	Connection string

	// From is the source region ID.
	From RegionID

	// FromCell is the connection's endpoint in From, in dungeon-absolute space.
	FromCell spatial.Position

	// To is the destination region ID.
	To RegionID

	// ToCell is the connection's endpoint in To, in dungeon-absolute space.
	ToCell spatial.Position
}

// Atlas returns a deterministic, construction-time snapshot of the field
// projected into dungeon-absolute space. Computed ONLY from construction
// data — fieldInput and connectionsInput, the same snapshot ToData
// persists — never from live member placement or clock state, so
// placing/moving a member or Pumping a tick never changes it (#929 T3
// ruling 3).
//
// O(REGIONS + CONNECTIONS, rpg-toolkit#1108), and that is the whole point of
// the shape it has. It used to enumerate every cell of every region: at the
// legal field budget — four 1024x1024 rooms, a blob well under a kilobyte —
// one call allocated 128 MB and took 43 ms, repeatably, because nothing was
// memoized and every call redid it (measured again on the post-S0 shape:
// 128.02 MB cold, 43.3 ms; 128.01 MB and 20.9 ms on a repeat call). #1059
// found that cost paying for a caller who wanted the grid FAMILY and nothing
// else, and added [Encounter.Grid] beside it; this is the other half of that
// finding. A region reports what it IS — anchor, span, family — and
// [Encounter.RegionAt] answers membership from that in O(regions) without
// anybody building a list. The same field now measures 624 bytes and 8
// microseconds per call (TestAtlasReportsRegionsWithoutEnumeratingThem).
//
// Deterministic (C8): Regions sorted by region ID, Doorways sorted by
// connection ID, each region's Boundaries and Occluders in declaration order
// (RoomInput's own order — #929 hardening round A). Copy-out: every returned
// slice is freshly allocated per call; mutating the result never reaches
// internal state.
//
// Occluders are map data, not entities: an occluder cell is a cell OF its
// region (RegionAt names it like any other), which is why occluders must be
// integral in every family (encounter.go's occluder-integrality check, #929 T3
// Opus round F2). A MEMBER's position is different in kind — an entity's
// position, not a map cell — and on a fractional-tolerant square grid it may
// sit anywhere in a region's continuous span, coinciding with no integer cell
// at all; hex forbids fractional member positions entirely
// (isIntegralAxialPosition), so this only affects square hosts (#929 T3 Opus
// round F4). The asymmetry — occluders must be integral, member positions may
// be fractional — is deliberate: one is floor/blockage data, the other is a
// live position the Atlas never touches.
func (e *Encounter) Atlas() (Atlas, error) {
	roomsByID := make(map[string]RoomInput, len(e.fieldInput))
	regions := make([]AtlasRegion, len(e.fieldInput))
	for i, ri := range e.fieldInput {
		roomsByID[ri.ID] = ri

		occluders := make([]spatial.Position, len(ri.Occluders))
		for j, o := range ri.Occluders {
			occluders[j] = o.Add(ri.Origin)
		}

		boundaries := make([]AtlasBoundary, len(ri.Boundaries))
		for j, b := range ri.Boundaries {
			boundaries[j] = AtlasBoundary{
				From:              b.From.Add(ri.Origin),
				To:                b.To.Add(ri.Origin),
				BlocksMovement:    b.BlocksMovement,
				BlocksLineOfSight: b.BlocksLineOfSight,
			}
		}

		regions[i] = AtlasRegion{
			ID:         ri.ID,
			Grid:       ri.Grid,
			Origin:     ri.Origin,
			Width:      ri.Width,
			Height:     ri.Height,
			Occluders:  occluders,
			Boundaries: boundaries,
		}
	}
	// LOAD-BEARING: fieldInput is never sorted anywhere else — this sort is
	// the only thing establishing C8 order for Regions.
	sort.Slice(regions, func(i, j int) bool { return regions[i].ID < regions[j].ID })

	doorways := make([]AtlasDoorway, len(e.connectionsInput))
	for i, ci := range e.connectionsInput {
		doorways[i] = AtlasDoorway{
			Connection: ci.ID,
			From:       ci.From,
			FromCell:   ci.FromPosition.Add(roomsByID[ci.From].Origin),
			To:         ci.To,
			ToCell:     ci.ToPosition.Add(roomsByID[ci.To].Origin),
		}
	}
	// Currently redundant — e.connectionsInput is already sorted by ID at
	// both NewEncounter and LoadEncounter (encounter.go, data.go) before
	// Atlas ever runs — but kept anyway so Atlas's own C8 determinism is
	// self-contained rather than coupled to an invariant maintained in two
	// OTHER files. Unlike the Regions sort above (load-bearing: fieldInput is
	// never pre-sorted), a future editor deleting this "redundant" sort
	// would find no failing test today — exactly the trap this comment
	// exists to prevent (#929 T3 fix round item 4; mutation evidence: the
	// mutant that removes this line is unobservable through the public API
	// on any normally-constructed encounter).
	sort.Slice(doorways, func(i, j int) bool { return doorways[i].Connection < doorways[j].Connection })

	return Atlas{Regions: regions, Doorways: doorways}, nil
}

// Grid reports the field's coordinate family — GridShapeSquare or
// GridShapeHex — in O(1).
//
// A FIELD fact, not a room one: W1 gives every room in a field the same family,
// checked identically at Setup and Load (validateGridFamilies), so there is one
// answer and every room agrees with it.
//
// It exists because the answer was only reachable through [Encounter.Atlas],
// which enumerated every cell of every room to get there — measured at ~128MB
// and tens of milliseconds at the legal field budget, unmemoized. A caller that
// needed the family and nothing else was paying the whole map for one enum, on
// a per-request path (rpg-toolkit#1059 finding 2). The Atlas stopped
// enumerating in rpg-toolkit#1108, which is the other half of that same
// finding; this stays because O(1) is still the right cost for one enum, and
// because a caller doing grid arithmetic should not have to read a map to
// learn which arithmetic to do.
//
// WHO NEEDS IT: anything doing grid arithmetic of its own, and the reason that
// is not merely a convenience is that the two families disagree about what one
// step means. Chebyshev distance on axial hex coordinates agrees with cube
// distance everywhere except the diagonals, so substituting one formula for the
// other passes almost every fixture — a real, previously-shipped defect class.
// The cure is to ask spatial for a grid of the right family, and this is how a
// caller learns which one to ask for. SPAN IS NOT REPORTED and is not needed
// for that: adjacency is Distance <= 1 in both families and neither Distance
// consults the grid's dimensions.
//
// Returns ErrNoField if the field holds no rooms, which construction forbids —
// but the zero GridShape IS GridShapeSquare, so answering "square" for a field
// that cannot have one would be a wrong answer rather than an absent one.
func (e *Encounter) Grid() (spatial.GridShape, error) {
	if len(e.fieldInput) == 0 {
		var unknown spatial.GridShape
		return unknown, fmt.Errorf("grid: %w", ErrNoField)
	}
	return e.fieldInput[0].Grid, nil
}
