// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package encounter

import (
	"fmt"

	"github.com/KirkDiggler/rpg-toolkit/tools/spatial"
)

// orientation.go is A CHAMBER IS THE RECTANGLE SOMEBODY DREW
// (rpg-toolkit#1127).
//
// # What was wrong
//
// A hex room's Width and Height were read as an AXIAL span — a rhombus, and an
// origin-centred one. Authors do not draw rhombi. Every authored dungeon in
// this project is a run of rectangular chambers in OFFSET coordinates
// ([col,row] pairs), and an offset rectangle SHEARS when it becomes axial: its
// cells stay a contiguous, non-overlapping set, but the smallest rhombus
// containing them is strictly bigger than they are.
//
// Measured on the reference tomb's own three chambers, against the constructor:
//
//   - pointy-top built, and then RegionAt reported 380 cells as floor against
//     the 224 somebody drew. 156 places a member could stand, and be reported
//     standing, that nobody put there.
//   - flat-top DID NOT BUILD AT ALL — `room "entrance" and room "hall" overlap
//     at absolute cell (3, -9)` — because W2 compared bounding boxes and the
//     sheared rhombi intersect where the chambers do not.
//
// So Kirk's ruling that "flat and pointy top are both valid and should be
// settable" was not merely awkward to honour, it was unreachable. This file is
// what delivers it.
//
// A third thing fell out of the measurement and is worth recording because it
// is the sharpest form of the argument: an origin-centred axial span can only
// be EVEN (axisBounds gives [-dim/2, dim/2-1]), so RoomInput{Width, Height}
// could not express an 11-cell rhombus at all. The tomb's chambers were not
// merely approximated by the old reading — several of them were not
// expressible under it.
//
// # The mask is a FUNCTION, not a set
//
// Nothing here stores a cell list, and that answers the first of the two
// questions Kirk's ruling (4) on rpg-toolkit#1105 left riding on the floor mask
// ("computed per load, or persisted?"). A cell is floor iff converting it back
// to offset space lands inside [0,Width) x [0,Height): one conversion and two
// comparisons, O(1), derived from four numbers RoomData already carries —
// width, height, origin, and the field's orientation. There is no cell list on
// the wire and none in memory.
//
// The second question ("does it belong in tools/spatial?") answers itself the
// same way: there is no new grid type here. spatial already owns both
// directions of the offset<->cube conversion, and this is the composition
// asking them. What lives here is the RULE about which cells a chamber owns,
// and that rule's other half is [regionAt] — a second answer to "who owns this
// cell" is exactly what region.go exists to prevent.
//
// # Why the composition is allowed to know this
//
// Orientation looks like a rendering fact, and it is not. The composition
// already knows the grid FAMILY — square versus hex is a thing it holds and
// reasons with — and pointy versus flat is the same species one level finer:
// how the grid is laid out, which is what a grid IS. It is not fiction and not
// a picture; it is the difference between two cells being neighbours and not.

// OrientationKind names which way a field's hexes point, in the form the blob
// carries it. See [Orientation].
type OrientationKind string

const (
	// OrientationPointyTop is the pointy-top layout, whose offset form is
	// odd-q: columns run straight and rows stagger.
	OrientationPointyTop OrientationKind = "pointy"

	// OrientationFlatTop is the flat-top layout, whose offset form is odd-r:
	// rows run straight and columns stagger.
	OrientationFlatTop OrientationKind = "flat"
)

// Orientation is which way a hex field's cells point: a closed set, sealed the
// way [Void] and [DoorState] are and for the same reason.
//
// # Required exactly where it means something
//
// A HEX field must declare one, and a SQUARE field must not. Both halves are
// refusals rather than one being a default, and the second is the interesting
// one: a square grid has no orientation, so a square field that declares one
// has an author believing something that cannot be true about their own
// dungeon. Ignoring it would be this module deciding that a statement with no
// meaning is harmless, which is the same move as inventing a default — see
// [Void]'s doc comment for the argument at length.
//
// # What it decides
//
// Every authored cell in a hex field is an OFFSET [col,row] pair, and an offset
// pair means nothing until the orientation is known: the same [2,3] is a
// different hex under each layout, with different neighbours. So this is not a
// presentational preference — it is the frame every other coordinate in the
// field is written in, which is why it is construction data and why it
// round-trips through persistence.
//
// It does NOT change any distance, adjacency or sight rule. Those run on cube
// coordinates, which are orientation-free; spatial's AxialHexGrid carries no
// orientation at all, correctly. Orientation lives at exactly one seam — the
// conversion between what an author wrote and what the canvas runs on — and
// this type is that seam.
type Orientation interface {
	// Kind names which layout this is: the word the blob carries, and the
	// word an error quotes back.
	Kind() OrientationKind

	// spatial reports the tools/spatial orientation this corresponds to.
	//
	// Unexported, which SEALS the set: a third layout cannot be declared
	// outside this package, and adding one means editing this file with the
	// caller that forces it in hand.
	spatial() spatial.HexOrientation
}

// HexesArePointyTop declares a field's hexes pointy-top, whose offset form is
// odd-q. The reference tomb's layout.
//
// A function rather than a package-level variable so nothing can reassign what
// it means at runtime — [VoidIsOpaque]'s reasoning, and the save gate's before
// it.
func HexesArePointyTop() Orientation { return pointyTop{} }

type pointyTop struct{}

func (pointyTop) Kind() OrientationKind           { return OrientationPointyTop }
func (pointyTop) spatial() spatial.HexOrientation { return spatial.HexOrientationPointyTop }

// HexesAreFlatTop declares a field's hexes flat-top, whose offset form is odd-r.
func HexesAreFlatTop() Orientation { return flatTop{} }

type flatTop struct{}

func (flatTop) Kind() OrientationKind           { return OrientationFlatTop }
func (flatTop) spatial() spatial.HexOrientation { return spatial.HexOrientationFlatTop }

// orientationFromData resolves the persisted word back to the declaration it
// names, or reports that a square field carries one it should not.
//
// Refusals by name and loud, per the standing precedent (rpg-toolkit#1053/#1068:
// fail loudly, no migration). An ABSENT word on a hex field is a blob written
// before the field declared anything, and loading it under a guess would read
// every stored cell in the wrong frame — a dungeon drawn correctly and played
// wrong. An UNKNOWN word is a dialect this build does not speak.
func orientationFromData(shape spatial.GridShape, name string) (Orientation, error) {
	if shape != spatial.GridShapeHex {
		if name != "" {
			return nil, fmt.Errorf(
				"a square field declares orientation %q, and a square grid has none (canvas.orientation): %w",
				name, ErrNoField)
		}

		return nil, nil
	}

	switch OrientationKind(name) {
	case OrientationPointyTop:
		return HexesArePointyTop(), nil
	case OrientationFlatTop:
		return HexesAreFlatTop(), nil
	case "":
		return nil, fmt.Errorf(
			"hex field does not say which way its hexes point (canvas.orientation): %w", ErrNoField)
	default:
		return nil, fmt.Errorf(
			"field declares orientation %q, which this build does not know (canvas.orientation): %w",
			name, ErrNoField)
	}
}

// orientationName renders a declaration for persistence, or empty for a square
// field.
func orientationName(o Orientation) string {
	if o == nil {
		return ""
	}

	return string(o.Kind())
}

// HexCellAt returns the dungeon-absolute cell an authored offset [col,row] pair
// names, under the given orientation.
//
// EXPORTED BECAUSE CONTENT NEEDS IT. A compiler turning an authored dungeon
// into a field has to put things where the author said, and "where the author
// said" is an offset pair — so the conversion cannot be private to this package
// without every caller reimplementing it, which is how two answers to one
// question get born. It is the same function [regionAt] runs backwards.
//
// The conversion itself is spatial's, not this module's: offset -> cube through
// [spatial.OffsetCoordinateToCubeWithOrientation], then cube read as axial with
// Q = X and R = Y, which is the reading [spatial.AxialHexGrid] uses (its
// axialToCube derives Z = -Q-R, so the two agree by construction rather than by
// coincidence — pinned by TestOffsetAndAxialAgreeWithSpatial).
func HexCellAt(o Orientation, col, row int) spatial.Position {
	cube := spatial.OffsetCoordinateToCubeWithOrientation(
		spatial.Position{X: float64(col), Y: float64(row)}, o.spatial())

	return spatial.Position{X: float64(cube.X), Y: float64(cube.Y)}
}

// hexOffsetOf is HexCellAt run backwards: the authored [col,row] a
// dungeon-absolute cell came from.
func hexOffsetOf(o Orientation, cell spatial.Position) (col, row int) {
	q, r := int(cell.X), int(cell.Y)
	offset := spatial.CubeCoordinate{X: q, Y: r, Z: -q - r}.
		ToOffsetCoordinateWithOrientation(o.spatial())

	return int(offset.X), int(offset.Y)
}

// footprintHolds reports whether a room owns a dungeon-absolute cell — THE
// MASK, and the only implementation of it.
//
// For a SQUARE field this is the rectangle it always was, asked through the
// room's own grid so square behaviour is untouched by any of this. For a HEX
// field it is the authored offset rectangle: convert the cell back to offset
// space and check the bounds. Note what is NOT here — no rhombus, no
// axisBounds, no origin-centred span. Those were the approximation.
//
// The origin is subtracted in OFFSET space for hex, which is the other half of
// the same change: a chamber's anchor says where its rectangle starts, in the
// columns and rows the author counts in. Anchoring in axial would put the
// shear back, one level up.
func footprintHolds(r RoomInput, o Orientation, grids map[string]spatial.Grid, cell spatial.Position) bool {
	if r.Grid != spatial.GridShapeHex {
		grid, ok := grids[r.ID]
		if !ok {
			return false
		}
		local := cell.Subtract(r.Origin)

		return grid.IsValidPosition(local)
	}

	col, row := hexOffsetOf(o, cell)
	col -= int(r.Origin.X)
	row -= int(r.Origin.Y)

	return col >= 0 && col < r.Width && row >= 0 && row < r.Height
}

// hexRuns decomposes a hex chamber's footprint into contiguous runs, which is
// what lets W2 and the canvas bounds be computed WITHOUT enumerating cells.
//
// # Why runs rather than a cell set
//
// maxFieldCells allows four million cells, so "build both footprints and
// intersect" is a real cost on a legal field, paid at every construction. But a
// sheared rectangle is not shapeless: along one axis it decomposes into
// contiguous intervals, one per authored column (pointy) or row (flat), and two
// intervals intersect in O(1). So the whole disjointness question costs
// O(Width) or O(Height) rather than O(Width x Height).
//
// The key is the coordinate that does NOT shear:
//
//   - POINTY-TOP is odd-q, so Q = col exactly and only R staggers. Key by Q;
//     for a fixed column, R decreases by exactly one per row, so the run is
//     [r(row=Height-1), r(row=0)].
//   - FLAT-TOP is odd-r, so the authored ROW is recoverable as -(Q+R) and only
//     Q staggers. Key by that; for a fixed row, Q increases by exactly one per
//     column.
//
// Both facts are read off spatial's own conversion rather than assumed, and
// pinned by TestRunsAgreeWithEnumeration, which compares this against a full
// cell-by-cell sweep over a spread of shapes and both orientations.
func hexRuns(r RoomInput, o Orientation) map[int][2]int {
	runs := make(map[int][2]int, max(r.Width, r.Height))

	oc, or := int(r.Origin.X), int(r.Origin.Y)
	if o.Kind() == OrientationPointyTop {
		for col := 0; col < r.Width; col++ {
			top := HexCellAt(o, col+oc, or)
			bottom := HexCellAt(o, col+oc, r.Height-1+or)
			runs[int(top.X)] = [2]int{int(bottom.Y), int(top.Y)}
		}

		return runs
	}

	for row := 0; row < r.Height; row++ {
		left := HexCellAt(o, oc, row+or)
		right := HexCellAt(o, r.Width-1+oc, row+or)
		runs[-(int(left.X) + int(left.Y))] = [2]int{int(left.X), int(right.X)}
	}

	return runs
}

// hexFootprintBounds returns a hex chamber's absolute axial bounding box, from
// its runs rather than from an assumed rhombus.
func hexFootprintBounds(r RoomInput, o Orientation) (qMin, qMax, rMin, rMax int) {
	first := true
	for key, run := range hexRuns(r, o) {
		var q0, q1, r0, r1 int
		if o.Kind() == OrientationPointyTop {
			q0, q1, r0, r1 = key, key, run[0], run[1]
		} else {
			// key is the authored row; the run is a Q interval, and R is
			// -Q-row on each end.
			q0, q1 = run[0], run[1]
			r0, r1 = -run[1]-key, -run[0]-key
		}
		if first {
			qMin, qMax, rMin, rMax = q0, q1, r0, r1
			first = false
			continue
		}
		qMin, qMax = min(qMin, q0), max(qMax, q1)
		rMin, rMax = min(rMin, r0), max(rMax, r1)
	}

	return
}

// hexFootprintsOverlap reports whether two hex chambers share a cell — W2, on
// the authored shape rather than on the box that contains it.
//
// THIS IS WHAT MAKES FLAT-TOP CONSTRUCTIBLE. The interval test on bounding
// boxes is exact for a rectangle and merely conservative for a sheared one, and
// "conservative" here meant refusing the reference tomb outright: its chambers
// share no cell in either orientation (measured: 0 shared cells across all three
// pairs, union exactly 224) while their flat-top boxes intersect.
func hexFootprintsOverlap(a, b RoomInput, o Orientation) bool {
	runsA, runsB := hexRuns(a, o), hexRuns(b, o)
	if len(runsB) < len(runsA) {
		runsA, runsB = runsB, runsA
	}

	for key, ra := range runsA {
		rb, shared := runsB[key]
		if !shared {
			continue
		}
		if ra[0] <= rb[1] && rb[0] <= ra[1] {
			return true
		}
	}

	return false
}

// absoluteOf projects an authored room-local cell onto the canvas.
//
// THERE IS EXACTLY ONE OF THESE, and that is the point. rpg-toolkit#1106
// deleted a function of this name for a good reason — it was a RUNTIME bridge,
// run on every query, and the wave's whole argument was that a field should
// stop projecting a world model it already has. This is the other kind: it runs
// at CONSTRUCTION only, spending the anchor once, exactly as
// RoomInput.Origin's doc comment says an origin is spent. No verb calls it.
//
// For a SQUARE room, local plus origin, which is what it always was. For a HEX
// room, both the local cell and the origin are counted in OFFSET columns and
// rows — the frame an author writes in — and the sum is converted once. Adding
// in axial instead would put the shear back one level up: two chambers anchored
// six columns apart would land at axial distances that vary by row, and the
// seam between them would stop being a straight line.
func absoluteOf(r RoomInput, o Orientation, local spatial.Position) spatial.Position {
	if r.Grid != spatial.GridShapeHex {
		return local.Add(r.Origin)
	}

	return HexCellAt(o, int(local.X)+int(r.Origin.X), int(local.Y)+int(r.Origin.Y))
}

// localIsInRoom reports whether an authored room-local cell is inside its
// chamber — the bounds check that used to be the room grid's.
//
// A hex room's grid is a spatial.AxialHexGrid whose bounds are an
// origin-centred axial span, and that is no longer what a chamber is, so asking
// it would refuse every legal offset cell of a chamber anchored anywhere but
// the middle of nowhere. Square still asks the grid, because for square the
// grid's rectangle IS the chamber and nothing about that changed.
func localIsInRoom(r RoomInput, grids map[string]spatial.Grid, local spatial.Position) bool {
	if r.Grid != spatial.GridShapeHex {
		grid, ok := grids[r.ID]

		return ok && grid.IsValidPosition(local)
	}

	return local.X >= 0 && local.X < float64(r.Width) &&
		local.Y >= 0 && local.Y < float64(r.Height)
}

// roomByID returns the room with the given ID, or a zero RoomInput.
//
// The zero value is deliberate rather than an error return: every caller here
// has already had its room name validated (an unknown room is a construction
// defect refused long before this), so a not-found is unreachable and returning
// an error would be a branch no test could reach and no caller could act on.
func roomByID(rooms []RoomInput, id string) RoomInput {
	for _, r := range rooms {
		if r.ID == id {
			return r
		}
	}

	return RoomInput{}
}

// fieldGridShape reports the grid family a field is drawn on.
//
// ANY hex room makes this a hex field, rather than reading the family off the
// first room, and that is about ERROR ORDER rather than about grids. W1 gives
// every room in a valid field the same family, so on a valid field the two
// readings are identical. On an INVALID one they differ, and reading the first
// room would make a field of [square, hex] answer "square" and get refused for
// declaring an orientation — burying the real defect, which is that it mixes
// families, under a complaint about a field that is not square either. Asking
// "is there a hex room here" lets a mixed field satisfy this check and reach
// W1, which is the rule that has something useful to say about it.
//
// An empty field answers square, and is refused a moment later for being empty.
func fieldGridShape(rooms []RoomInput) spatial.GridShape {
	for _, r := range rooms {
		if r.Grid == spatial.GridShapeHex {
			return spatial.GridShapeHex
		}
	}

	return spatial.GridShapeSquare
}

// fieldOrientation checks a declared orientation against the field it describes.
//
// The mirror of [orientationFromData] for the Setup seam, where the declaration
// arrives typed rather than as a word. Both seams end at the same two rules —
// a hex field must declare one, a square field must not — because #929 T2's
// standing arrangement is that Load routes through the SAME validators Setup
// uses rather than a parallel reimplementation, and two copies of a rule are
// two rules waiting to disagree.
func fieldOrientation(rooms []RoomInput, declared Orientation) (Orientation, error) {
	if fieldGridShape(rooms) != spatial.GridShapeHex {
		if declared != nil {
			return nil, fmt.Errorf(
				"a square field declares orientation %q, and a square grid has none "+
					"(FieldInput.Canvas.Orientation): %w", declared.Kind(), ErrNoField)
		}

		return nil, nil
	}

	if declared == nil {
		return nil, fmt.Errorf(
			"hex field does not say which way its hexes point "+
				"(FieldInput.Canvas.Orientation): %w", ErrNoField)
	}

	return declared, nil
}
