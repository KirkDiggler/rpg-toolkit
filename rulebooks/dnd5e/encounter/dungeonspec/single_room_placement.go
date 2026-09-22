// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package dungeonspec

import (
	"fmt"
	"math"
	"sort"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter"
	"github.com/KirkDiggler/rpg-toolkit/tools/spatial"
)

// single_room_placement.go is THE SOURCE-TO-CANONICAL PLACEMENT ADAPTER
// (single-room-play.md §3, issue #1753).
//
// The source scene keeps its declared portable authoring frame: editor units
// on the world X/Z plane, Three Y yaw in radians, hexes of circumradius 1.
// The encounter's placed facts live in the CANONICAL plane: spatial XY feet,
// one hex cell FeetPerCell across the flats. Exactly one adapter converts
// the one into the other, and it lives HERE, at the construction boundary —
// the encounter consumes canonical geometry and never re-converts, and no
// second authored pose exists on either side of the line.
//
// The arithmetic is the approved one, and the tests pin it term by term:
//
//	k = FeetPerCell/sqrt(3) feet per editor unit (across the flats)
//	spatial origin = (transform.x*k, transform.z*k)
//	facing degrees = -transform.rotationY*180/pi
//	Box.D = footprint.width*k   (D lies along Facing — the name swap)
//	Box.W = footprint.depth*k
//	LocalOffset = (footprint.offsetX*k, footprint.offsetZ*k)
//
// Two signs are load-bearing rather than cosmetic. Positive Three Y yaw
// turns +X toward -Z, and the canonical plane's Y runs south, so a source
// yaw arrives as the opposite plane angle. And spatial's D runs along the
// facing while the editor's "width" runs along the owner's own, hence D
// taking the width and W the depth. `coordinateFrame.hexRadius` is REQUIRED
// to be 1 by the lowering (single_room_lowering.go), so the scale needs no
// second factor; a scene the editor never authored is not this function's
// input.
//
// THIS IS NOT THE COMPILE. A decoded spec's placed contributors are one
// input the full v3 Load/Compile (the next milestone) hands to the encounter
// field; until then nothing here claims a playable result, and Load still
// refuses v3 outright.

// feetPerSourceUnit is k: the canonical scale one portable source unit
// converts to, on the approved across-the-flats equivalence (an editor hex
// of circumradius 1 is sqrt(3) editor units across the flats, which is
// exactly one cell's FeetPerCell).
var feetPerSourceUnit = encounter.FeetPerCell / math.Sqrt(3)

// CanonicalPlacedProps converts one room source's prop declarations into the
// canonical placed contributors the encounter field consumes: every declared
// prop paired with the pose the lowering read for its live scene item
// (single_room_lowering.go), converted by the ONE adapter this file documents.
// Group poses apply no further render transform — item transforms are
// already world-posed — and only DECLARED props contribute: an undeclared
// prop is visual dressing, never inferred blocking.
//
// The result is sorted by prop ID (C8: a decode is a pure function of its
// source, and a map must not leak iteration order into geometry). A
// declaration naming no live prop item, or one whose pose the scene never
// authored, is refused — both unreachable from a successful decode (the
// lowering refuses each at its own path) and named, not skipped, for a
// hand-assembled source.
//
// The flags and the local rectangle ride through unchanged; only the frame
// is converted. Doubles stay doubles: no float32 narrowing anywhere in the
// path.
func (r *RoomSource) CanonicalPlacedProps() ([]encounter.PlacedPropInput, error) {
	if len(r.Gameplay.PropDeclarations) == 0 {
		return []encounter.PlacedPropInput{}, nil
	}

	read, _ := readRoom(r)

	out := make([]encounter.PlacedPropInput, 0, len(r.Gameplay.PropDeclarations))
	for id, decl := range r.Gameplay.PropDeclarations {
		pose, posed := read.Poses[id]
		if !posed {
			if read.ItemIDs[id] {
				return nil, fmt.Errorf("prop declaration %q names a scene prop whose transform is not authored", id)
			}

			return nil, fmt.Errorf("prop declaration %q names no live scene prop", id)
		}
		if decl.BlocksMovement == nil || decl.BlocksLineOfSight == nil {
			return nil, fmt.Errorf("prop declaration %q does not say both blocking answers", id)
		}
		out = append(out, placedPropFrom(id, decl, pose))
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })

	return out, nil
}

// placedPropFrom is the adapter itself, one declaration and one authored pose
// to one canonical placement — the exact mapping this file's doc pins.
func placedPropFrom(id string, decl RoomPropDeclaration, t scenePose) encounter.PlacedPropInput {
	return encounter.PlacedPropInput{
		ID:                id,
		Placement:         placedFootprintFrom(decl, t),
		BlocksMovement:    *decl.BlocksMovement,
		BlocksLineOfSight: *decl.BlocksLineOfSight,
	}
}

// placedFootprintFrom is the GEOMETRY HALF of the adapter: the rectangle and
// the pose, with no opinion about what the thing blocks.
//
// Split out because a DOOR is the same rectangle with a different answer to
// that question (single_room_doors.go): its blocking follows its state, so it
// needs the shape without the flags — and it must be the SAME shape, arrived
// at by the same arithmetic, or a door would sit somewhere its own prop
// declaration does not.
func placedFootprintFrom(decl RoomPropDeclaration, t scenePose) spatial.FootprintPlacement {
	k := feetPerSourceUnit

	return spatial.FootprintPlacement{
		Footprint: spatial.Footprint{Box: &spatial.Box{
			D: decl.Footprint.Width * k, // D lies along the facing: the name swap
			W: decl.Footprint.Depth * k,
		}},
		Origin:      spatial.Point{X: t.X * k, Y: t.Z * k},
		Facing:      -t.RotationY * 180 / math.Pi, // positive Three Y yaw turns +X toward -Z
		LocalOffset: spatial.Point{X: decl.Footprint.OffsetX * k, Y: decl.Footprint.OffsetZ * k},
	}
}
