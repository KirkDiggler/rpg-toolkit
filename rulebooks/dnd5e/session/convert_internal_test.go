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
//
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

// TestAnEmptyAtlasProjectsEmptyLists — a field with no walls-as-lines and
// nothing sealed projects empty, never nil-with-a-length, so a host that
// ranges over either gets the same shape whatever the dungeon is.
func TestAnEmptyAtlasProjectsEmptyLists(t *testing.T) {
	out := projectAtlas(encounter.Atlas{Orientation: encounter.HexesArePointyTop()})

	require.Empty(t, out.Segments)
	require.Empty(t, out.Sealed)
}
