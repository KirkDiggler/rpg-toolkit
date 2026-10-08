// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package session

import (
	"reflect"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter"
	"github.com/KirkDiggler/rpg-toolkit/tools/spatial"
)

// convert_internal_test.go tests projectAtlas directly, white-box, because it
// is unexported and convert_test.go's external-package audit is deliberately
// structural (field NAMES match, by reflection) rather than a value check —
// it would pass even if a field were added to both types' definitions and
// never actually wired into the copy loop below. This file is the value half:
// proof that the bytes projectAtlas returns are the bytes it was handed, not
// just that the outer type has somewhere to put them.

// TestPropFacingAndOffsetCrossTheSeam pins rpg-project#261's projection: a
// straight copy, by design, of the two additive presentational fields —
// carried exactly as the composition reports them, including the "said
// nothing" zero values on a prop that authored neither.
func TestPropFacingAndOffsetCrossTheSeam(t *testing.T) {
	in := encounter.Atlas{
		Orientation: encounter.HexesArePointyTop(),
		Props: []encounter.AtlasProp{
			{
				Ref: "dnd5e:props:statue-reaper", At: spatial.Position{X: 3, Y: 4},
				BlocksMovement: true, BlocksLineOfSight: true,
				Facing: "se", Offset: [3]float64{0.2, -0.1, 0.6},
			},
			{
				Ref: "dnd5e:props:candles", At: spatial.Position{X: 5, Y: 6},
				BlocksMovement: false, BlocksLineOfSight: false,
			},
		},
	}

	out := projectAtlas(in)

	require.Len(t, out.Props, 2)
	require.Equal(t, "se", out.Props[0].Facing, "the exact authored word, uninterpreted")
	require.Equal(t, [3]float64{0.2, -0.1, 0.6}, out.Props[0].Offset, "the exact authored numbers, height included")

	require.Equal(t, "", out.Props[1].Facing, "said nothing")
	require.Equal(t, [3]float64{0, 0, 0}, out.Props[1].Offset, "and said zero/center-on-the-floor: the same fact by design")
}

// TestWallHeightCrossesTheSeam — the authored multiplier is copied verbatim
// (rpg-project#273), and a wall that authored none carries 0: the reader's
// word for "render the standard height", never a number to multiply by.
func TestWallHeightCrossesTheSeam(t *testing.T) {
	in := encounter.Atlas{
		Orientation: encounter.HexesArePointyTop(),
		Boundaries: []encounter.AtlasBoundary{
			{From: spatial.Position{X: 2, Y: 3}, To: spatial.Position{X: 3, Y: 3},
				BlocksMovement: true, BlocksLineOfSight: true, Height: 2.5},
			{From: spatial.Position{X: 2, Y: 4}, To: spatial.Position{X: 3, Y: 4},
				BlocksMovement: true, BlocksLineOfSight: true},
		},
	}

	out := projectAtlas(in)

	require.Len(t, out.Boundaries, 2)
	require.Equal(t, 2.5, out.Boundaries[0].Height, "the exact authored multiplier")
	require.Zero(t, out.Boundaries[1].Height, "no height authored: the standard height")
}

// TestSegmentsAndSealedCrossTheSeam is the value half for rpg-toolkit#1480:
// the two fields encounter's atlas grew for walls-as-lines, carried across as
// the bytes they are.
//
// This is the test that was missing when the fields arrived. The structural
// audit next door caught that session.Atlas had nowhere to put them the moment
// the pin moved to encounter v0.52.0 — which is how the gap was meant to be
// found, and would have been, had anything bumped the pin before rpg-api did.
// A name match is not a wiring proof, so the values are checked here.
func TestSegmentsAndSealedCrossTheSeam(t *testing.T) {
	in := encounter.Atlas{
		Orientation: encounter.HexesArePointyTop(),
		Segments: []encounter.AtlasSegment{
			{
				From:   encounter.AxialPointF{Q: 2, R: 7.5},
				To:     encounter.AxialPointF{Q: 6, R: -0.5},
				Height: 2.5,
			},
			{
				From: encounter.AxialPointF{Q: -1, R: 0.5},
				To:   encounter.AxialPointF{Q: 3, R: 0.5},
			},
		},
		Sealed: []spatial.Position{{X: 3, Y: 1}, {X: 3, Y: 3}},
	}

	out := projectAtlas(in)

	require.Len(t, out.Segments, 2)
	require.Equal(t, AxialPointF{Q: 2, R: 7.5}, out.Segments[0].From,
		"the halves a wall endpoint needs, exactly — a side midpoint is half a step")
	require.Equal(t, AxialPointF{Q: 6, R: -0.5}, out.Segments[0].To)
	require.Equal(t, 2.5, out.Segments[0].Height, "the exact authored multiplier")
	require.Zero(t, out.Segments[1].Height, "no height authored: the standard height")
	require.Equal(t, AxialPointF{Q: -1, R: 0.5}, out.Segments[1].From)

	require.Equal(t, []spatial.Position{{X: 3, Y: 1}, {X: 3, Y: 3}}, out.Sealed,
		"every cell nobody stands on, in the composition's own order")

	// AND NOTHING MORE. A segment says where the line runs and how tall it is
	// drawn, and deliberately not what stands on it or opens in it — a
	// segment that carried its doors would say through the back door what the
	// doorway list withholds from a non-knower.
	require.Equal(t, []string{"From", "To", "Height"}, fieldsOfSegment(),
		"two ends and a height")
}

// fieldsOfSegment names AtlasSegment's fields in declaration order.
func fieldsOfSegment() []string {
	t := reflect.TypeOf(AtlasSegment{})
	out := make([]string, 0, t.NumField())
	for i := 0; i < t.NumField(); i++ {
		out = append(out, t.Field(i).Name)
	}

	return out
}

// fieldNamesOf names a type's exported fields in declaration order — the
// structural records' half of the "nothing on the wire but the fixed facts"
// claim (rpg-project#169).
func fieldNamesOf(v any) []string {
	t := reflect.TypeOf(v)
	out := make([]string, 0, t.NumField())
	for i := 0; i < t.NumField(); i++ {
		if t.Field(i).IsExported() {
			out = append(out, t.Field(i).Name)
		}
	}

	return out
}

// TestStructuralLayoutCrossesTheSeamWhole is the value half of
// rpg-project#169: every provider field of a wall, its openings and an
// independent door arrives, spelled in this package's own footpoint type, with
// nothing conversion-shaped happening on the way.
//
// The numbers are deliberately all different (a length that is not its height,
// an elevation that is negative, an opening position that is not the midpoint)
// so a converter that swapped or defaulted any of them fails here instead of
// drawing a wall at the wrong place.
func TestStructuralLayoutCrossesTheSeamWhole(t *testing.T) {
	in := encounter.Atlas{
		Orientation: encounter.HexesArePointyTop(),
		StructuralWalls: []encounter.AtlasStructuralWall{{
			ID:        encounter.PropID("wall-presence"),
			Ref:       "dnd5e:env:test:wall",
			From:      spatial.Point{X: 1.5, Y: 2.25},
			To:        spatial.Point{X: 11.5, Y: 2.25},
			Height:    3,
			Thickness: 0.3,
			Elevation: -1.25,
			Openings: []encounter.AtlasStructuralOpening{
				{ID: "bare", Position: 2, Width: 1.5},
				{ID: "gapped", Position: 7.25, Width: 2},
			},
		}},
		StructuralDoors: []encounter.AtlasStructuralDoor{{
			ID:        encounter.DoorID("vault/gate"),
			Ref:       "dnd5e:env:test:door",
			From:      spatial.Point{X: 6.25, Y: 2.25},
			To:        spatial.Point{X: 8.25, Y: 2.25},
			Height:    3,
			Thickness: 0.3,
			Elevation: -1.25,
		}},
	}

	out := projectAtlas(in)

	require.Len(t, out.StructuralWalls, 1)
	wall := out.StructuralWalls[0]
	require.Equal(t, "wall-presence", wall.ID, "the raw static presence id, verbatim")
	require.Equal(t, "dnd5e:env:test:wall", wall.Ref, "opaque, unread")
	require.Equal(t, FootprintPoint{X: 1.5, Y: 2.25}, wall.From, "canonical feet, one frame lower")
	require.Equal(t, FootprintPoint{X: 11.5, Y: 2.25}, wall.To)
	require.Equal(t, 3.0, wall.Height)
	require.Equal(t, 0.3, wall.Thickness)
	require.Equal(t, -1.25, wall.Elevation, "a structural line may be sunk; negative is an authored fact")
	require.Equal(t, []AtlasStructuralOpening{
		{ID: "bare", Position: 2, Width: 1.5},
		{ID: "gapped", Position: 7.25, Width: 2},
	}, wall.Openings, "the permitted cuts, in authored order, id/position/width only")

	require.Len(t, out.StructuralDoors, 1)
	door := out.StructuralDoors[0]
	require.Equal(t, "vault/gate", door.ID, "the actual canonical gameplay door id")
	require.Equal(t, "dnd5e:env:test:door", door.Ref)
	require.Equal(t, FootprintPoint{X: 6.25, Y: 2.25}, door.From, "the resolved visual opening endpoints")
	require.Equal(t, FootprintPoint{X: 8.25, Y: 2.25}, door.To)
	require.Equal(t, 3.0, door.Height)
	require.Equal(t, 0.3, door.Thickness)
	require.Equal(t, -1.25, door.Elevation)

	// NOTHING MORE AND NOTHING ELSEWHERE. A structural row is fixed layout:
	// no state, no private placed id and no parent association may ride it,
	// because a client that could read those would be reading world truth this
	// seam is forbidden to send.
	require.Equal(t, []string{"ID", "Ref", "From", "To", "Height", "Thickness", "Elevation", "Openings"},
		fieldNamesOf(AtlasStructuralWall{}), "a wall names its line and its cuts and nothing mutable")
	require.Equal(t, []string{"ID", "Position", "Width"},
		fieldNamesOf(AtlasStructuralOpening{}), "an opening carries no door id and no state")
	require.Equal(t, []string{"ID", "Ref", "From", "To", "Height", "Thickness", "Elevation"},
		fieldNamesOf(AtlasStructuralDoor{}), "a door stands on its own identity with no parent id")
}

// TestTheStructuralSlicesAreCopiedNotShared is [TestThePlacedCellsAreCopiedNotShared]'s
// claim on the structural collection: projectAtlas hands out its OWN backing
// arrays for the wall list, each wall's opening list, and the door list, so a
// caller editing a returned atlas cannot reach the composition's snapshot.
//
// INTERNAL for the same reason as its neighbours: a seam read reloads every
// call, so an external version cannot fail whether or not the copy happens.
//
// The mutant it kills is the tidy-looking `Openings: wall.Openings`, which
// compiles, passes every value assertion above, and shares the provider's own
// slice with the host.
func TestTheStructuralSlicesAreCopiedNotShared(t *testing.T) {
	innerOpenings := []encounter.AtlasStructuralOpening{{ID: "gap", Position: 5, Width: 2}}
	in := encounter.Atlas{
		Orientation: encounter.HexesArePointyTop(),
		StructuralWalls: []encounter.AtlasStructuralWall{{
			ID: "wall-presence", From: spatial.Point{X: 0, Y: 0}, To: spatial.Point{X: 10, Y: 0},
			Height: 3, Thickness: 0.3, Openings: innerOpenings,
		}},
		StructuralDoors: []encounter.AtlasStructuralDoor{{
			ID: "vault/gate", From: spatial.Point{X: 4, Y: 0}, To: spatial.Point{X: 6, Y: 0},
		}},
	}

	out := projectAtlas(in)
	out.StructuralWalls[0].Openings[0] = AtlasStructuralOpening{ID: "edited"}
	out.StructuralWalls[0].From = FootprintPoint{X: 99, Y: 99}
	out.StructuralDoors[0].ID = "edited/gate"

	require.Equal(t, "gap", innerOpenings[0].ID, "a caller's edit must not reach the composition's snapshot")
	require.Equal(t, spatial.Point{X: 0, Y: 0}, in.StructuralWalls[0].From)
	require.Equal(t, encounter.DoorID("vault/gate"), in.StructuralDoors[0].ID)
}

// TestAnEmptyAtlasProjectsEmptyLists — a field with no walls-as-lines and
// nothing sealed projects empty, never nil-with-a-length, so a host that
// ranges over either gets the same shape whatever the dungeon is.
func TestAnEmptyAtlasProjectsEmptyLists(t *testing.T) {
	out := projectAtlas(encounter.Atlas{Orientation: encounter.HexesArePointyTop()})

	require.Empty(t, out.Segments)
	require.Empty(t, out.Sealed)
	require.Empty(t, out.Placed, "a dungeon whose author drew no rectangles ranges the same as one that did")
	require.Empty(t, out.StructuralWalls, "and one whose author drew no structural walls")
	require.Empty(t, out.StructuralDoors, "nor any independently permitted door")
}

// TestTheStartIsCopiedNotShared pins that projectAtlas hands out its OWN
// pointer for the way in.
//
// INTERNAL, and it has to be. The obvious external version — read an atlas,
// mutate its Start, read again — cannot fail: every seam read reloads the
// encounter from the repository, so each call already builds a fresh pointer
// whether or not this function copies. Verified by writing that test first
// and watching it pass against a projectAtlas that deliberately shared.
//
// The mutant it exists to kill is a plausible one rather than a contrived
// one: [AtlasStart] and [encounter.AtlasStart] have identical layouts, so
// `(*AtlasStart)(in.Start)` compiles and looks like a tidy saving. It would
// hand a host a pointer into the composition's own snapshot, which is exactly
// what S2 forbids.
func TestTheStartIsCopiedNotShared(t *testing.T) {
	inner := &encounter.AtlasStart{At: spatial.Position{X: 1, Y: 3}, Facing: "e"}
	out := projectAtlas(encounter.Atlas{
		Orientation: encounter.HexesArePointyTop(),
		Start:       inner,
	})

	require.NotNil(t, out.Start)
	require.Equal(t, inner.At, out.Start.At, "same fact")
	require.Equal(t, inner.Facing, out.Start.Facing)

	out.Start.Facing = "s"
	out.Start.At = spatial.Position{X: 99, Y: 99}
	require.Equal(t, "e", inner.Facing, "a caller's edit must not reach the composition")
	require.Equal(t, spatial.Position{X: 1, Y: 3}, inner.At)
}

// TestAnAtlasWithNoStartProjectsNil is the zero-value half, in the one place
// the conversion decides it: nil in, nil out, never a zero-valued start that
// would claim the party arrives at the origin looking nowhere.
func TestAnAtlasWithNoStartProjectsNil(t *testing.T) {
	out := projectAtlas(encounter.Atlas{Orientation: encounter.HexesArePointyTop()})
	require.Nil(t, out.Start)
}

// TestAPlacedFootprintCrossesTheSeamWhole is the value half of
// rpg-api-protos#351: every field of a placement arrives, including the one
// the seam has to reshape.
//
// The numbers are deliberately all different — a box wider than it is deep,
// an origin nowhere near the local offset, a facing that is neither — so a
// conversion that swapped any pair fails here instead of drawing a table
// turned ninety degrees in somebody's browser. The box in particular: the
// composition says W across the facing and D along it, and the authored
// dialect's own `width` already meant the other one, so a re-swap at this
// seam is a live mistake rather than a hypothetical.
func TestAPlacedFootprintCrossesTheSeamWhole(t *testing.T) {
	stands := []spatial.Position{{X: 2, Y: 0}, {X: 3, Y: 0}, {X: 3, Y: 1}}
	in := encounter.Atlas{
		Orientation: encounter.HexesArePointyTop(),
		Placed: []encounter.AtlasPlacedProp{{
			ID: "vault-door",
			Placement: spatial.FootprintPlacement{
				Footprint:   spatial.Footprint{Box: &spatial.Box{W: 13, D: 0.75}},
				Origin:      spatial.Point{X: 7.5, Y: 4.25},
				Facing:      -21.5,
				LocalOffset: spatial.Point{X: 0.25, Y: -0.5},
			},
			Cells:             stands,
			BlocksMovement:    true,
			BlocksLineOfSight: false,
			Holdable:          true,
		}},
	}

	out := projectAtlas(in)

	require.Len(t, out.Placed, 1)
	got := out.Placed[0]
	require.Equal(t, "vault-door", got.ID, "the author's name, verbatim")
	require.Equal(t, 13.0, got.Placement.Width, "spatial's W: the extent ACROSS the facing")
	require.Equal(t, 0.75, got.Placement.Depth, "spatial's D: the extent ALONG it")
	require.Equal(t, FootprintPoint{X: 7.5, Y: 4.25}, got.Placement.Origin)
	require.Equal(t, -21.5, got.Placement.Facing, "the exact authored bearing, unsnapped")
	require.Equal(t, FootprintPoint{X: 0.25, Y: -0.5}, got.Placement.LocalOffset,
		"the offset the engine traced its cells from, so a client draws it where the engine has it")

	// THE CELLS, WHICH IS WHAT THE FIELD IS FOR. A conversion that dropped
	// this line would still compile, still return a placement a client could
	// draw, and still leave that client with no adjacency but the one it
	// computed itself — the second geometry the field exists to prevent.
	require.Equal(t, stands, got.Cells,
		"every cell the composition says the rectangle stands on, in its own order")

	// The two blocking answers are independent, so they are asserted as the
	// two different values the fixture gave them rather than together.
	require.True(t, got.BlocksMovement)
	require.False(t, got.BlocksLineOfSight, "a door nobody can see through is not the only kind")
	require.True(t, got.Holdable, "the offer a client puts on the thing it is already drawing")
}

// TestThePlacedCellsAreCopiedNotShared is [TestTheStartIsCopiedNotShared]'s
// claim on the other new field, and it kills the same tidy-looking mutant:
// `Cells: p.Cells` compiles, passes every value assertion above, and hands a
// host the composition's own backing array.
//
// INTERNAL for that test's reason — a seam read reloads the encounter every
// call, so an external version cannot fail whether or not the copy happens.
func TestThePlacedCellsAreCopiedNotShared(t *testing.T) {
	inner := []spatial.Position{{X: 2, Y: 0}, {X: 3, Y: 0}}
	out := projectAtlas(encounter.Atlas{
		Orientation: encounter.HexesArePointyTop(),
		Placed: []encounter.AtlasPlacedProp{{
			ID:        "slab",
			Placement: spatial.FootprintPlacement{Footprint: spatial.Footprint{Box: &spatial.Box{W: 10, D: 5}}},
			Cells:     inner,
		}},
	})

	require.Len(t, out.Placed, 1)
	out.Placed[0].Cells[0] = spatial.Position{X: 99, Y: 99}
	require.Equal(t, spatial.Position{X: 2, Y: 0}, inner[0],
		"a caller's edit must not reach the composition's snapshot")
}

// TestAPlacementWithNoBoxProjectsNoSides pins the guard rather than the
// behaviour it guards.
//
// The composition refuses a boxless placement at construction, so this atlas
// cannot come out of a real field. What the guard buys is that the impossible
// case is a zero-sided rectangle a reader can SEE is wrong, rather than a nil
// dereference thrown from inside a host's read verb — and the guard is worth
// a test because deleting it also compiles.
func TestAPlacementWithNoBoxProjectsNoSides(t *testing.T) {
	out := projectAtlas(encounter.Atlas{
		Orientation: encounter.HexesArePointyTop(),
		Placed: []encounter.AtlasPlacedProp{{
			ID:        "boxless",
			Placement: spatial.FootprintPlacement{Origin: spatial.Point{X: 3, Y: 4}},
		}},
	})

	require.Len(t, out.Placed, 1)
	require.Zero(t, out.Placed[0].Placement.Width)
	require.Zero(t, out.Placed[0].Placement.Depth)
	require.Equal(t, FootprintPoint{X: 3, Y: 4}, out.Placed[0].Placement.Origin,
		"and the rest of the pose still crosses: the guard skips a box, not a placement")
}
