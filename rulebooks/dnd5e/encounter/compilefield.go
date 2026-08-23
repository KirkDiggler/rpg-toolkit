// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package encounter

import (
	"fmt"
	"math"
	"sort"

	"github.com/KirkDiggler/rpg-toolkit/tools/spatial"
)

// compilefield.go is THE REGIONS ARE THE FLOOR (rpg-project#256).
//
// One function, [compileField], turns an authored [FieldInput] into the one
// thing this composition runs on: a validated, absolute picture of the floor
// and everything standing on it. Both construction seams call it — Setup with
// the caller's input, Load with the blob converted back into the same input —
// so every rule about what a field may be is stated exactly once and the two
// seams cannot drift (#929 T2's shared-validator lesson, kept).
//
// # What changed, and why it is simpler
//
// The room chain (RoomInput, Origin, ConnectionInput) described the floor as
// rectangles-plus-anchors and derived the cells from them at every question:
// a mask function, interval runs for overlap, a bounding-box sweep for the
// canvas, and a separate projection for every authored coordinate. A region
// LISTS its cells, so all of that collapses into a map from absolute cell to
// owner, built once. Overlap is a duplicate key. The canvas span is the
// bounding box of the keys. The floor mask is a lookup. There is nothing left
// to derive, and nothing to keep in step.
//
// # Hex only, and one conversion
//
// Every authored coordinate — a region's cells, a prop's cell, a wall's
// endpoints, a member's seat, an ending's target — is an offset [col,row] pair
// under [CanvasInput.Orientation], and [HexCellAt] is the ONE function that
// turns it into the absolute axial cell the canvas runs on. The square family
// left with the room chain: a region is painted on a hex grid, and a second
// family would be a second frame for every coordinate in this file to be
// wrong in (the rpg-toolkit#1141/#1150 lesson).

// field is the compiled field: the authored input, deep-copied and
// validated, plus the absolute facts derived from it exactly once.
type field struct {
	// regions, props and walls are the authored input, deep-copied so the
	// persistence source never aliases caller-owned state (T6 review M4),
	// in authored order. Cells and endpoints stay in the AUTHORED frame here
	// — this is what ToData writes back out.
	regions []RegionInput
	props   []PropInput
	walls   []spatial.Boundary

	void        Void
	orientation Orientation

	// owner is THE MASK: every floor cell, absolute axial, to the region
	// that owns it. A cell absent here is void. Unique by construction —
	// compileField refuses the field otherwise (W2).
	owner map[spatial.Position]RegionID

	// regionCells is each region's cells, absolute axial and sorted by
	// coordinate — the read [Encounter.Atlas] reports, built once rather
	// than re-derived per call.
	regionCells map[RegionID][]spatial.Position

	// cells is the whole floor, absolute axial, sorted by coordinate.
	cells []spatial.Position

	// width and height are the span of the one hex grid that holds the
	// floor's bounding box (W6).
	width, height int
}

// maxAnchorCoord bounds the magnitude of every authored coordinate — pure
// integer-overflow defense, not a gameplay limit: no real dungeon is within
// six orders of magnitude of 1<<30. maxFieldCells bounds the number of cells
// a field may list, which is also the size of the map this package keeps and
// the list the Atlas hands out, so the bound guards a real allocation.
const (
	maxAnchorCoord = 1 << 30
	maxFieldCells  = 1 << 22
)

// compileField validates an authored field and builds the compiled one.
//
// Validation order (first failure wins, R5: no observable state until
// construction succeeds): the canvas's two declarations; the region list
// (empty, then per region: empty or duplicate ID, no cells, no archetype, no
// lighting, an intensity outside [0,1], the field's cell budget, a
// non-integral or out-of-range cell, a cell owned twice — within one region
// or across two); props (no ref, an unsaid blocking answer, a non-integral
// cell, a cell no region owns, two props on one cell); walls (a non-integral
// endpoint, an endpoint that is not floor, endpoints that are not adjacent
// under the orientation, the same edge twice). Doors are checked by the
// caller through validateDoorInputs, against the result.
//
// Error messages carry NO verb prefix: each caller wraps its own at the call
// site, so the same validator is honest about which seam refused.
func compileField(in FieldInput) (*field, error) {
	if in.Canvas.Void == nil {
		return nil, fmt.Errorf("field does not say what its void is (FieldInput.Canvas.Void): %w", ErrNoField)
	}
	if in.Canvas.Orientation == nil {
		return nil, fmt.Errorf(
			"field does not say which way its hexes point (FieldInput.Canvas.Orientation): %w", ErrNoField)
	}
	if len(in.Regions) == 0 {
		return nil, fmt.Errorf("field has no regions: %w", ErrNoField)
	}

	f := &field{
		void:        in.Canvas.Void,
		orientation: in.Canvas.Orientation,
		owner:       make(map[spatial.Position]RegionID),
		regionCells: make(map[RegionID][]spatial.Position, len(in.Regions)),
	}

	if err := f.compileRegions(in.Regions); err != nil {
		return nil, err
	}
	if err := f.compileProps(in.Props); err != nil {
		return nil, err
	}
	if err := f.compileWalls(in.Walls); err != nil {
		return nil, err
	}

	// W6 — the field is one canvas: an origin-centred hex span wide enough to
	// hold the floor's bounding box. A hex grid always fits (negative axial
	// coordinates are ordinary there), so this never refuses.
	qMin, qMax, rMin, rMax := cellBounds(f.cells)
	f.width, f.height = 2*max(-qMin, qMax+1), 2*max(-rMin, rMax+1)

	return f, nil
}

// compileRegions builds the owner map and the per-region cell lists, refusing
// every region defect by name.
func (f *field) compileRegions(regions []RegionInput) error {
	f.regions = make([]RegionInput, len(regions))
	seen := make(map[string]bool, len(regions))
	var total int64

	for i, r := range regions {
		if r.ID == "" {
			return fmt.Errorf("regions[%d] has empty id: %w", i, ErrNoField)
		}
		if seen[r.ID] {
			return fmt.Errorf("duplicate region %q: %w", r.ID, ErrNoField)
		}
		seen[r.ID] = true

		if len(r.Cells) == 0 {
			return fmt.Errorf("region %q: %w", r.ID, ErrRegionEmpty)
		}
		if r.Archetype == "" {
			return fmt.Errorf("region %q: %w", r.ID, ErrRegionArchetypeMissing)
		}
		if r.Lighting == nil {
			return fmt.Errorf("region %q: %w", r.ID, ErrRegionLightingMissing)
		}
		if !(r.Lighting.Intensity >= 0 && r.Lighting.Intensity <= 1) {
			return fmt.Errorf("region %q lighting intensity %g is outside [0,1]: %w",
				r.ID, r.Lighting.Intensity, ErrNoField)
		}

		total += int64(len(r.Cells))
		if total > maxFieldCells {
			return fmt.Errorf("field lists more than %d cells: %w", maxFieldCells, ErrNoField)
		}

		abs := make([]spatial.Position, 0, len(r.Cells))
		for j, c := range r.Cells {
			if !isAuthoredCell(c) {
				return fmt.Errorf("region %q cells[%d] (%g,%g) is not a representable integral cell: %w",
					r.ID, j, c.X, c.Y, ErrNoField)
			}
			cell := f.cellAt(c)
			if owner, taken := f.owner[cell]; taken {
				if owner == r.ID {
					return fmt.Errorf("region %q cells[%d] [%g,%g] is listed twice: %w",
						r.ID, j, c.X, c.Y, ErrRegionOverlap)
				}
				return fmt.Errorf("region %q cells[%d] [%g,%g] already belongs to region %q: %w",
					r.ID, j, c.X, c.Y, owner, ErrRegionOverlap)
			}
			f.owner[cell] = r.ID
			abs = append(abs, cell)
		}
		sortCells(abs)
		f.regionCells[r.ID] = abs
		f.cells = append(f.cells, abs...)

		// Deep copy: Lighting is a pointer, and a caller editing theirs after
		// construction must not change what ToData writes.
		lighting := *r.Lighting
		f.regions[i] = RegionInput{
			ID: r.ID, Name: r.Name, Archetype: r.Archetype, Lighting: &lighting,
			Cells: append([]spatial.Position(nil), r.Cells...),
		}
	}
	sortCells(f.cells)

	return nil
}

// compileProps checks every prop says what it is and what it does, and stands
// on a floor cell of its own.
func (f *field) compileProps(props []PropInput) error {
	f.props = make([]PropInput, len(props))
	seen := make(map[spatial.Position]bool, len(props))

	for i, p := range props {
		if p.Ref == "" {
			return fmt.Errorf("props[%d] at (%g,%g) has no ref: %w", i, p.At.X, p.At.Y, ErrNoField)
		}
		// A PROP MUST SAY WHAT IT DOES (rpg-toolkit#1128, and #1033's law
		// behind it): all four blocking combinations are real content, so no
		// zero value could stand in for a missing answer.
		if p.BlocksMovement == nil {
			return fmt.Errorf("prop %q does not say whether it blocks_movement: %w", p.Ref, ErrNoField)
		}
		if p.BlocksLineOfSight == nil {
			return fmt.Errorf("prop %q does not say whether it blocks_line_of_sight: %w", p.Ref, ErrNoField)
		}
		if !isAuthoredCell(p.At) {
			return fmt.Errorf("prop %q at (%g,%g) is not a representable integral cell: %w",
				p.Ref, p.At.X, p.At.Y, ErrNoField)
		}
		cell := f.cellAt(p.At)
		if _, floor := f.regionOf(cell); !floor {
			return fmt.Errorf("prop %q at [%g,%g] stands on no region's floor: %w",
				p.Ref, p.At.X, p.At.Y, ErrBadPlacement)
		}
		if seen[cell] {
			return fmt.Errorf("two props at [%g,%g]: %w", p.At.X, p.At.Y, ErrNoField)
		}
		seen[cell] = true

		// Fresh pointers: a caller flipping a bool after construction must
		// not change what ToData writes (the T6 defect one indirection down).
		blocksMovement, blocksSight := *p.BlocksMovement, *p.BlocksLineOfSight
		f.props[i] = PropInput{
			Ref: p.Ref, At: p.At, BlocksMovement: &blocksMovement, BlocksLineOfSight: &blocksSight,
		}
	}

	return nil
}

// compileWalls checks every wall is an edge between two adjacent floor cells
// and no edge is drawn twice.
func (f *field) compileWalls(walls []spatial.Boundary) error {
	f.walls = append([]spatial.Boundary(nil), walls...)
	seen := make(map[DoorEdge]bool, len(walls))

	for i, w := range walls {
		edge, err := f.edgeOf(w.From, w.To)
		if err != nil {
			return fmt.Errorf("walls[%d]: %w", i, err)
		}
		if seen[edge] {
			return fmt.Errorf("walls[%d] [%g,%g]-[%g,%g] is listed twice: %w",
				i, w.From.X, w.From.Y, w.To.X, w.To.Y, ErrNoField)
		}
		seen[edge] = true
	}

	return nil
}

// wallEdges is every authored wall as a normalized absolute crossing — what
// validateDoorInputs checks a door against. Only called on a compiled field,
// whose walls have already passed edgeOf.
func (f *field) wallEdges() map[DoorEdge]bool {
	out := make(map[DoorEdge]bool, len(f.walls))
	for _, w := range f.walls {
		out[normalizeDoorEdge(DoorEdge{From: f.cellAt(w.From), To: f.cellAt(w.To)})] = true
	}

	return out
}

// edgeOf converts an authored edge to its normalized absolute crossing,
// refusing one that is not a crossing between two adjacent floor cells.
//
// ADJACENCY IS ASKED UNDER THE ORIENTATION, not computed from the offset
// pair: the two endpoints are converted through [HexCellAt] and spatial
// measures the cube distance between the results. That is what makes the
// same authored pair a legal wall under one layout and a refusal under the
// other, which TestEdges_MustBeAdjacentUnderOrientation pins.
func (f *field) edgeOf(from, to spatial.Position) (DoorEdge, error) {
	for _, end := range []spatial.Position{from, to} {
		if !isAuthoredCell(end) {
			return DoorEdge{}, fmt.Errorf("endpoint (%g,%g) is not a representable integral cell: %w",
				end.X, end.Y, ErrNoField)
		}
	}
	a, b := f.cellAt(from), f.cellAt(to)
	for _, end := range []struct {
		authored, abs spatial.Position
	}{{from, a}, {to, b}} {
		if _, floor := f.regionOf(end.abs); !floor {
			return DoorEdge{}, fmt.Errorf("endpoint [%g,%g] is not floor: %w",
				end.authored.X, end.authored.Y, ErrEdgeOffFloor)
		}
	}
	if adjacencyGrid.Distance(a, b) != 1 {
		return DoorEdge{}, fmt.Errorf("[%g,%g]-[%g,%g] joins cells that are not adjacent under %s: %w",
			from.X, from.Y, to.X, to.Y, f.orientation.Kind(), ErrEdgeNotAdjacent)
	}

	return normalizeDoorEdge(DoorEdge{From: a, To: b}), nil
}

// adjacencyGrid is the calculator every adjacency question in this package
// asks. AxialHexGrid.Distance converts both arguments to cube coordinates and
// subtracts — it never reads the grid's own bounds — so any instance of the
// family measures absolute cells correctly; the span exists only so a future
// caller reaching for IsValidPosition on it is not surprised.
var adjacencyGrid = spatial.NewAxialHexGrid(spatial.AxialHexGridConfig{SpanWidth: 1e6, SpanHeight: 1e6})

// cellAt is the field's one call into [HexCellAt]: the authored offset pair
// to the absolute axial cell it names under this field's orientation.
func (f *field) cellAt(authored spatial.Position) spatial.Position {
	return HexCellAt(f.orientation, int(authored.X), int(authored.Y))
}

// regionOf reports which region owns an absolute cell — THE ownership lookup,
// and the only one.
func (f *field) regionOf(cell spatial.Position) (RegionID, bool) {
	id, ok := f.owner[cell]

	return id, ok
}

// hasVoid reports whether any cell of the canvas belongs to no region —
// purely a cost decision for the sight scan ([canvasRoom]): where there is no
// void, opaque and transparent mean the same thing.
func (f *field) hasVoid() bool {
	return int64(f.width)*int64(f.height) != int64(len(f.cells))
}

// compileCanvas builds the one spatial room the encounter runs on: a grid
// spanning the floor's bounding box, with every prop placed and every wall
// and door registered on it in absolute cells.
func (f *field) compileCanvas(doors []*doorRecord) (*canvasRoom, error) {
	canvas := spatial.NewBasicRoom(spatial.BasicRoomConfig{
		ID:   "canvas",
		Type: "encounter",
		Grid: spatial.NewAxialHexGrid(spatial.AxialHexGridConfig{
			SpanWidth: float64(f.width), SpanHeight: float64(f.height),
		}),
	})

	// Prop entity IDs are index-based: refs legitimately repeat (four
	// pillars in a hall), and a coordinate-derived key is a collision
	// waiting to happen. An index cannot collide on any input.
	for i, p := range f.props {
		prop := &propEntity{
			id:                fmt.Sprintf("prop-%d", i),
			ref:               p.Ref,
			blocksMovement:    *p.BlocksMovement,
			blocksLineOfSight: *p.BlocksLineOfSight,
		}
		if err := canvas.PlaceEntity(prop, f.cellAt(p.At)); err != nil {
			return nil, fmt.Errorf("prop placement: %w: %w", ErrBadPlacement, err)
		}
	}

	for _, w := range f.walls {
		if err := canvas.RegisterBoundary(spatial.Boundary{
			From:              f.cellAt(w.From),
			To:                f.cellAt(w.To),
			BlocksMovement:    w.BlocksMovement,
			BlocksLineOfSight: w.BlocksLineOfSight,
		}); err != nil {
			return nil, fmt.Errorf("boundary: %w: %w", ErrBadPlacement, err)
		}
	}

	// Doors LAST, after the walls, so a field that somehow reached here with
	// a door on a walled crossing would end with the door's answer rather
	// than the wall's. Validation refuses that field outright
	// (validateDoorInputs), which makes this ordering unreachable rather than
	// load-bearing — stated so the next editor knows which it is.
	for _, d := range doors {
		if err := registerDoor(canvas, d); err != nil {
			return nil, err
		}
	}

	return &canvasRoom{BasicRoom: canvas, field: f}, nil
}

// isAuthoredCell reports whether an authored pair names a whole cell within
// the coordinate bound — finite, integral, and at most maxAnchorCoord in
// magnitude, so the int conversion [cellAt] makes is exact.
func isAuthoredCell(pos spatial.Position) bool {
	for _, v := range []float64{pos.X, pos.Y} {
		if math.IsInf(v, 0) || math.IsNaN(v) || float64(int(v)) != v || math.Abs(v) > maxAnchorCoord {
			return false
		}
	}

	return true
}

// cellBounds is the bounding box of a non-empty cell list in axial space.
func cellBounds(cells []spatial.Position) (qMin, qMax, rMin, rMax int) {
	qMin, qMax = int(cells[0].X), int(cells[0].X)
	rMin, rMax = int(cells[0].Y), int(cells[0].Y)
	for _, c := range cells[1:] {
		qMin, qMax = min(qMin, int(c.X)), max(qMax, int(c.X))
		rMin, rMax = min(rMin, int(c.Y)), max(rMax, int(c.Y))
	}

	return qMin, qMax, rMin, rMax
}

// sortCells puts cells in the one coordinate order every sorted list in this
// package shares: by X, then by Y.
func sortCells(cells []spatial.Position) {
	sort.Slice(cells, func(i, j int) bool { return cellBefore(cells[i], cells[j]) })
}

// cellBefore is the map's coordinate order.
func cellBefore(a, b spatial.Position) bool {
	if a.X != b.X {
		return a.X < b.X
	}

	return a.Y < b.Y
}
