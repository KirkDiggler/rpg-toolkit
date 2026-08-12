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

// AbsoluteInput names a room-local position to project into
// dungeon-absolute space.
type AbsoluteInput struct {
	// Room is the room the position is local to.
	Room string

	// Position is the room-local position to project.
	Position spatial.Position
}

// AbsoluteOutput is a position projected into dungeon-absolute space.
type AbsoluteOutput struct {
	// Position is the dungeon-absolute position.
	Position spatial.Position
}

// LocateInput names a dungeon-absolute position to resolve to its
// owning room.
type LocateInput struct {
	// Position is the dungeon-absolute position to resolve.
	Position spatial.Position
}

// LocateOutput is a dungeon-absolute position resolved to its owning
// room and the equivalent room-local position.
type LocateOutput struct {
	// Room is the owning room's ID.
	Room string

	// Position is the room-local position.
	Position spatial.Position
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
// Q-outer/R-inner nesting). Copy-out: every returned slice is freshly
// allocated per call; mutating the result never reaches internal state.
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
// carries the two integers that produce it in 394 bytes — not
// impractically, trivially. Reject-never-crash is module law
// (LoadEncounter's doc comment); this was the trust boundary.
//
// Occluders are map data, not entities: every occluder cell is also a
// Cells entry (Occluders is a SUBSET of Cells —
// TestAtlasRoomCellsAndOccludersAreAbsolute), which is why occluders must
// be integral in every family (encounter.go's occluder-integrality
// check, #929 T3 Opus round F2). A MEMBER's position is different in
// kind — an entity's position, not a map cell — and on a
// fractional-tolerant square grid it may sit anywhere in a room's
// continuous span, coinciding with no integer cell in Cells at all; hex
// forbids fractional member positions entirely
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

		rooms[i] = AtlasRoom{
			ID:        ri.ID,
			Grid:      ri.Grid,
			Origin:    ri.Origin,
			Width:     ri.Width,
			Height:    ri.Height,
			Cells:     cells,
			Occluders: occluders,
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

// Absolute projects a room-local position into dungeon-absolute space.
// Legal for ANY in-bounds position, whether occupied, occluded, or
// empty — hosts project percept ghosts at cells nobody stands on (#929
// T3 ruling 1); it never consults members or occluders.
//
// Returns ErrNilInput for a nil input, ErrNoField for a room ID that
// names no room in this field (the same room-list defect vocabulary
// Setup/Load reject with — #929 T3 ruling), and ErrBadPlacement if
// Position is out of the room's bounds or (hex only) not an integral
// axial cell — the bridge refuses to project fiction.
func (e *Encounter) Absolute(in *AbsoluteInput) (*AbsoluteOutput, error) {
	if in == nil {
		return nil, fmt.Errorf("absolute: %w", ErrNilInput)
	}

	ri, ok := e.roomInput(in.Room)
	if !ok {
		return nil, fmt.Errorf("absolute: unknown room %q: %w", in.Room, ErrNoField)
	}

	grid := buildRoomGrid(ri.Grid, ri.Width, ri.Height)
	if !grid.IsValidPosition(in.Position) {
		return nil, fmt.Errorf("absolute: position out of bounds: %w", ErrBadPlacement)
	}
	if !isIntegralAxialPosition(grid, in.Position) {
		return nil, fmt.Errorf("absolute: position is not an integral axial cell: %w", ErrBadPlacement)
	}

	return &AbsoluteOutput{Position: in.Position.Add(ri.Origin)}, nil
}

// Locate is the reverse bridge: resolves a dungeon-absolute position to
// its owning room and the equivalent room-local position. W2 (rooms
// never overlap) plus integral origins make ownership uniqueness EXACT
// in real (infinite-precision) arithmetic — and exact in float64 too,
// for INTEGRAL cells specifically, since integers stay exact to 2^53,
// far past maxAnchorCoord (1<<30). For a FRACTIONAL square position the
// round trip is only approximate: near an extreme anchor (close to
// maxAnchorCoord), a fraction within roughly one float64 ULP of a room
// edge (~2.4e-7 at that magnitude) can round to the WRONG room — a
// float64 precision limit, not a logic bug, and not chased further here
// (#929 T3 Opus round F3 — an earlier version of this comment
// overclaimed exactness for the fractional case too). The guarantee
// this module DOES make and pins with a test:
// TestLocateAbsoluteRoundTripExactAtExtremeOrigin proves an INTEGRAL
// round trip is exact even at extreme origins (±(2^30−8)). At most one
// room's bounds check can pass for a given input, so iteration order
// never matters — that part holds unconditionally, independent of the
// precision caveat above.
//
// Returns ErrNilInput for a nil input, and ErrBadPlacement if no room
// owns the position — void is not floor. Occluded cells ARE owned:
// Locate returns the owning room for a cell an occluder sits on exactly
// as it would an empty one (#929 T3 ruling 1) — occlusion is
// walkability, not ownership, and this function never consults
// occluders.
func (e *Encounter) Locate(in *LocateInput) (*LocateOutput, error) {
	if in == nil {
		return nil, fmt.Errorf("locate: %w", ErrNilInput)
	}

	for _, ri := range e.fieldInput {
		local := in.Position.Subtract(ri.Origin)
		grid := buildRoomGrid(ri.Grid, ri.Width, ri.Height)
		if !grid.IsValidPosition(local) {
			continue
		}
		if !isIntegralAxialPosition(grid, local) {
			continue
		}
		return &LocateOutput{Room: ri.ID, Position: local}, nil
	}

	return nil, fmt.Errorf("locate: position owned by no room: %w", ErrBadPlacement)
}

// roomInput looks up a room's construction-truth RoomInput by ID —
// shared by Absolute and Locate, both of which need Origin (only ever
// stored here, never on the spatial room itself) alongside grid shape
// and dimensions.
func (e *Encounter) roomInput(id string) (RoomInput, bool) {
	for _, ri := range e.fieldInput {
		if ri.ID == id {
			return ri, true
		}
	}
	return RoomInput{}, false
}

// atlasCells re-derives a room's local, integral cell coordinates for
// Atlas. T1 deleted the original enumeration helper (roomLocalCells)
// when W2 went interval-based (roomAbsoluteBounds' doc comment) — this
// is a fresh implementation, not a resurrection, proven against
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
	// families — this function's doc comment), and width*height is
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
