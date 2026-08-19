// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package encounter

import (
	"fmt"
	"sort"

	"github.com/KirkDiggler/rpg-toolkit/tools/spatial"
)

// Atlas is a deterministic, construction-time snapshot of the field
// projected into dungeon-absolute space: every room's absolute cell set
// and occluders, and every connection's absolute doorway pair (#929 T3 —
// the read surface promised by RoomInput.Origin's doc comment).
type Atlas struct {
	// Rooms is every room's absolute footprint, sorted by room ID (C8).
	Rooms []AtlasRoom

	// Doorways is every connection's absolute endpoint pair, sorted by
	// connection ID (C8).
	Doorways []AtlasDoorway
}

// AtlasRoom is one room's absolute-space footprint.
type AtlasRoom struct {
	// ID is the room's identifier.
	ID string

	// Grid is the room's coordinate family (GridShapeSquare or GridShapeHex).
	Grid spatial.GridShape

	// Origin is the room's dungeon-absolute anchor (RoomInput.Origin).
	Origin spatial.Position

	// Width is the room's horizontal dimension.
	Width int

	// Height is the room's vertical dimension.
	Height int

	// Cells is every cell of the room, in dungeon-absolute space, in
	// grid-iteration order (atlasCells) — always populated regardless of
	// occupancy or occlusion: occlusion is walkability, not ownership
	// (#929 T3 ruling 1).
	Cells []spatial.Position

	// Occluders is the room's line-of-sight-blocking cells, in
	// dungeon-absolute space. Reported separately from Cells so a host
	// can render them distinctly (#929 T3 ruling 1).
	Occluders []spatial.Position

	// Boundaries is the room's walls (RoomInput.Boundaries), with both
	// endpoints projected into dungeon-absolute space (#929 hardening round
	// A) — the SAME projection compileCanvas registers them by, so what a
	// host draws and what the encounter enforces are the same edges.
	//
	// An endpoint may belong to the room NEXT DOOR: a chamber walls its own
	// edge by naming the cell beyond it, which is what makes a wall between
	// two authored rooms expressible at all (rpg-toolkit#1106,
	// RoomInput.Boundaries' doc comment). Grouping such a wall under the room
	// that DECLARED it is construction truth reported faithfully, not a claim
	// about which room the wall belongs to. In declaration order (RoomInput's own
	// order, the same construction-truth ordering Occluders already
	// uses above) — deterministic given a fixed input (C8), though not
	// independently sorted the way Rooms/Doorways are.
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
type AtlasDoorway struct {
	// Connection is the connection's identifier.
	Connection string

	// From is the source room ID.
	From string

	// FromCell is the connection's endpoint in From, in dungeon-absolute space.
	FromCell spatial.Position

	// To is the destination room ID.
	To string

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
// Deterministic (C8): Rooms sorted by room ID, Doorways sorted by
// connection ID, each room's Cells in grid-iteration order (atlasCells'
// Q-outer/R-inner nesting), each room's Boundaries in declaration order
// (RoomInput's own order, same as Occluders — #929 hardening round A).
// Copy-out: every returned slice is freshly allocated per call; mutating
// the result never reaches internal state.
//
// Cost: O(total cells across all rooms) by contract — Atlas enumerates
// every cell of every room, via a make() sized exactly Width*Height per
// room (atlasCells). This is BOUNDED, not merely documented:
// maxRoomCells/maxFieldCells (encounter.go) reject an oversized room or
// field at room legality — the shared path both NewEncounter and
// LoadEncounter route through — before an Encounter exists to call Atlas
// on. Before that bound existed, this comment claimed a legal-but-absurd
// room would merely "allocate enormously" and that the wire "cannot
// practically carry" one — both wrong (#929 T3 Opus round F1): a
// 2^30 x 2^30 room, legal under maxRoomSpan's per-axis check alone,
// PANICS atlasCells' make() (a 2^60-capacity argument), and the wire
// carries the two integers that produce it in a few hundred bytes — not
// impractically, trivially. Reject-never-crash is module law
// (LoadEncounter's doc comment); this was the trust boundary.
//
// Bounded is not the same as cheap (#929 hardening round I): at the
// field budget — four 1024×1024 rooms, a legal field whose persisted
// blob is well under a kilobyte (measured: 573 bytes) — a single Atlas()
// call allocates on the order of 128 MB (measured via runtime.MemStats'
// TotalAlloc delta: ~134 MB, i.e. exactly maxFieldCells cells at 16
// bytes each, doubled by atlasCells' local enumeration plus its
// Origin-projected copy) and takes tens of milliseconds (measured:
// ~60-65ms cold, ~45-50ms on a repeat call), REPEATABLY — every call
// redoes the same work, nothing is memoized internally. Hosts that call
// Atlas() per request rather than per encounter will feel this; cache
// the returned Atlas per encounter instead. Caching is trivially safe
// here specifically because Atlas is pure construction-truth (ruling 3
// above) — no live state can make a cached snapshot stale; only a
// reload (LoadEncounter, which returns a new *Encounter) requires a
// fresh call.
//
// Occluders are map data, not entities: every occluder cell is also an
// AtlasRoom.Cells entry (AtlasRoom.Occluders is a SUBSET of
// AtlasRoom.Cells — TestAtlasRoomCellsAndOccludersAreAbsolute), which is
// why occluders must be integral in every family (encounter.go's
// occluder-integrality check, #929 T3 Opus round F2). A MEMBER's
// position is different in kind — an entity's position, not a map cell
// — and on a fractional-tolerant square grid it may sit anywhere in a
// room's continuous span, coinciding with no integer cell in
// AtlasRoom.Cells at all; hex forbids fractional member positions entirely
// (isIntegralAxialPosition), so this only affects square hosts (#929 T3
// Opus round F4). The asymmetry — occluders must be integral, member
// positions may be fractional — is deliberate: one is floor/blockage
// data Atlas enumerates, the other is a live position Atlas never
// touches.
func (e *Encounter) Atlas() (Atlas, error) {
	roomsByID := make(map[string]RoomInput, len(e.fieldInput))
	rooms := make([]AtlasRoom, len(e.fieldInput))
	for i, ri := range e.fieldInput {
		roomsByID[ri.ID] = ri

		local := atlasCells(ri.Grid, ri.Width, ri.Height)
		cells := make([]spatial.Position, len(local))
		for j, c := range local {
			cells[j] = c.Add(ri.Origin)
		}

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

		rooms[i] = AtlasRoom{
			ID:         ri.ID,
			Grid:       ri.Grid,
			Origin:     ri.Origin,
			Width:      ri.Width,
			Height:     ri.Height,
			Cells:      cells,
			Occluders:  occluders,
			Boundaries: boundaries,
		}
	}
	// LOAD-BEARING: fieldInput is never sorted anywhere else — this sort is
	// the only thing establishing C8 order for Rooms.
	sort.Slice(rooms, func(i, j int) bool { return rooms[i].ID < rooms[j].ID })

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
	// OTHER files. Unlike the Rooms sort above (load-bearing: fieldInput is
	// never pre-sorted), a future editor deleting this "redundant" sort
	// would find no failing test today — exactly the trap this comment
	// exists to prevent (#929 T3 fix round item 4; mutation evidence: the
	// mutant that removes this line is unobservable through the public API
	// on any normally-constructed encounter).
	sort.Slice(doorways, func(i, j int) bool { return doorways[i].Connection < doorways[j].Connection })

	return Atlas{Rooms: rooms, Doorways: doorways}, nil
}

// Grid reports the field's coordinate family — GridShapeSquare or
// GridShapeHex — in O(1).
//
// A FIELD fact, not a room one: W1 gives every room in a field the same family,
// checked identically at Setup and Load (validateGridFamilies), so there is one
// answer and every room agrees with it.
//
// It exists because the answer was only reachable through [Encounter.Atlas],
// which enumerates every cell of every room to get there — measured at ~128MB
// and tens of milliseconds at the legal field budget, unmemoized (Atlas's own
// doc). A caller that needs the family and nothing else was paying the whole
// map for one enum, on a per-request path (rpg-toolkit#1059 finding 2).
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

// atlasCells re-derives a room's local, integral cell coordinates for
// Atlas. T1 deleted the original enumeration helper (roomLocalCells)
// when W2 went interval-based (axisBounds' doc comment, encounter.go) —
// this is a fresh implementation, not a resurrection, proven against
// spatial's own IsValidPosition by TestAtlasCellsMatchIsValidPosition
// (#929 T3 ruling 4).
//
// Built on axisBounds (encounter.go), the same min/max primitive W2
// already trusts: enumerates every integer in
// [axisBounds(shape,width).min, axisBounds(shape,width).max] crossed
// with [axisBounds(shape,height).min, axisBounds(shape,height).max], Q
// (X) outer, R (Y) inner — deterministic, grid-iteration order.
func atlasCells(shape spatial.GridShape, width, height int) []spatial.Position {
	qMin, qMax := axisBounds(shape, width)
	rMin, rMax := axisBounds(shape, height)

	// SAFE: (qMax-qMin+1)*(rMax-rMin+1) always equals width*height (both
	// families — axisBounds' doc comment, encounter.go), and width*height is
	// bounded by maxRoomCells at room legality (encounter.go) BEFORE any
	// RoomInput reaches here — the only path to a RoomInput is
	// buildValidRoomGrids, which every construction seam routes through.
	// No redundant check here (#929 T3 Opus round F1 ruling): a future
	// editor who relaxes maxRoomCells without reading this comment
	// reintroduces the panic this bound exists to prevent — the
	// doorway-sort lesson, applied to an allocation instead of an
	// ordering invariant.
	cells := make([]spatial.Position, 0, (qMax-qMin+1)*(rMax-rMin+1))
	for q := qMin; q <= qMax; q++ {
		for r := rMin; r <= rMax; r++ {
			cells = append(cells, spatial.Position{X: float64(q), Y: float64(r)})
		}
	}
	return cells
}
