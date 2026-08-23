// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package encounter

import (
	"fmt"

	"github.com/KirkDiggler/rpg-toolkit/tools/spatial"
)

// orientation.go is WHICH WAY THE HEXES POINT (rpg-toolkit#1127,
// rpg-project#256).
//
// Every authored cell in a field is an OFFSET [col,row] pair, and an offset
// pair means nothing until the orientation is known: the same [2,3] is a
// different hex under each layout, with different neighbours. The declaration
// lives here, and [HexCellAt] — the ONE conversion from the authored frame to
// the absolute axial cells the canvas runs on — lives beside it.
//
// # Why the composition is allowed to know this
//
// Orientation looks like a rendering fact, and it is not. Pointy versus flat
// is how the grid is laid out, which is what a grid IS. It is not fiction and
// not a picture; it is the difference between two cells being neighbours and
// not — the reason a wall authored as the same [col,row] pair is a legal edge
// under one layout and a refusal under the other.
//
// # There is exactly one conversion, and it is not reversible here
//
// The room chain needed both directions: authored cells were rectangles, so
// the mask converted an absolute cell BACK to offset space to bounds-check it.
// A region lists its cells, so the mask is a map lookup and the reverse
// conversion is gone (rpg-toolkit#1141 and #1150 were both bugs in having two
// readings of one basis). What remains is HexCellAt, asked by compileField
// and by nothing else.

// OrientationKind names which way a field's hexes point, in the form the blob
// carries it. See [Orientation].
type OrientationKind string

const (
	// OrientationPointyTop is the pointy-top layout, whose offset form is
	// odd-r: rows run straight and columns stagger.
	OrientationPointyTop OrientationKind = "pointy"

	// OrientationFlatTop is the flat-top layout, whose offset form is odd-q:
	// columns run straight and rows stagger.
	OrientationFlatTop OrientationKind = "flat"
)

// Orientation is which way a field's cells point: a closed set, sealed the
// way [Void] and [DoorState] are and for the same reason.
//
// REQUIRED on every field. Every authored cell is an offset [col,row] pair
// that means nothing until this is known, so it is construction data and
// round-trips through persistence; a field that does not declare one is
// refused (ErrNoField) rather than read under a guess.
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
// odd-r. The reference tomb's layout.
//
// A function rather than a package-level variable so nothing can reassign what
// it means at runtime — [VoidIsOpaque]'s reasoning, and the save gate's before
// it.
func HexesArePointyTop() Orientation { return pointyTop{} }

type pointyTop struct{}

func (pointyTop) Kind() OrientationKind           { return OrientationPointyTop }
func (pointyTop) spatial() spatial.HexOrientation { return spatial.HexOrientationPointyTop }

// HexesAreFlatTop declares a field's hexes flat-top, whose offset form is odd-q.
func HexesAreFlatTop() Orientation { return flatTop{} }

type flatTop struct{}

func (flatTop) Kind() OrientationKind           { return OrientationFlatTop }
func (flatTop) spatial() spatial.HexOrientation { return spatial.HexOrientationFlatTop }

// orientationFromData resolves the persisted word back to the declaration it
// names.
//
// Refusals by name and loud, per the standing precedent (rpg-toolkit#1053/#1068:
// fail loudly, no migration). An ABSENT word is a blob written before the
// field declared anything, and loading it under a guess would read every
// stored cell in the wrong frame — a dungeon drawn correctly and played
// wrong. An UNKNOWN word is a dialect this build does not speak.
func orientationFromData(name string) (Orientation, error) {
	switch OrientationKind(name) {
	case OrientationPointyTop:
		return HexesArePointyTop(), nil
	case OrientationFlatTop:
		return HexesAreFlatTop(), nil
	case "":
		return nil, fmt.Errorf(
			"field does not say which way its hexes point (canvas.orientation): %w", ErrNoField)
	default:
		return nil, fmt.Errorf(
			"field declares orientation %q, which this build does not know (canvas.orientation): %w",
			name, ErrNoField)
	}
}

// orientationName renders a declaration for persistence.
func orientationName(o Orientation) string {
	return string(o.Kind())
}

// HexCellAt returns the dungeon-absolute cell an authored offset [col,row] pair
// names, under the given orientation.
//
// EXPORTED BECAUSE CONTENT NEEDS IT. A compiler turning an authored dungeon
// into a field has to put things where the author said, and "where the author
// said" is an offset pair — so the conversion cannot be private to this package
// without every caller reimplementing it, which is how two answers to one
// question get born. Inside this package, compileField is the only caller.
//
// The conversion itself is spatial's, not this module's: offset -> cube through
// [spatial.OffsetCoordinateToCubeWithOrientation], then [spatial.CubeCoordinate.ToAxial]
// reads the cube as axial — Q is cube X and R is cube Z (rpg-toolkit#1150: the
// pair every pixel formula and [spatial.AxialHexGrid] assume). This function
// used to rebuild that reading by hand, X and Y rather than X and Z, which is
// the bug #1150 fixed: two definitions of the same basis, silently disagreeing.
// There is now exactly one, and this asks for it — pinned by
// TestOffsetAndAxialAgreeWithSpatial.
func HexCellAt(o Orientation, col, row int) spatial.Position {
	cube := spatial.OffsetCoordinateToCubeWithOrientation(
		spatial.Position{X: float64(col), Y: float64(row)}, o.spatial())

	return cube.ToAxial()
}
