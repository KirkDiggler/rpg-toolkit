// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package encounter

// facing_test.go pins the two conversions facing.go owns (rpg-project#582,
// sneaking design R13): a step's direction to a bearing under each hex
// orientation, and an authored compass word to a bearing. Both are tables
// because both are tables in the world: six hex directions, eight words.

import (
	"errors"
	"math"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/KirkDiggler/rpg-toolkit/tools/spatial"
)

// TestTheSixStepsHaveSixBearings walks one step in each axial direction under
// each orientation and reads the bearing back: 0 is map north, clockwise.
//
// The pointy-top row is a hex whose flats face east and west, so its steps
// land on the 30-degree marks; the flat-top row is the same hex turned thirty
// degrees, so its steps land on the 60-degree marks with due north among them.
// The step is taken from a cell that is not the origin, because a bearing is
// a difference and must not care where on the map it is measured.
func TestTheSixStepsHaveSixBearings(t *testing.T) {
	from := spatial.Position{X: 2, Y: -1}
	cases := []struct {
		name        string
		orientation Orientation
		step        spatial.Position
		want        float64
	}{
		{"pointy +q", HexesArePointyTop(), spatial.Position{X: 1, Y: 0}, 90},
		{"pointy +q-r", HexesArePointyTop(), spatial.Position{X: 1, Y: -1}, 30},
		{"pointy -r", HexesArePointyTop(), spatial.Position{X: 0, Y: -1}, 330},
		{"pointy -q", HexesArePointyTop(), spatial.Position{X: -1, Y: 0}, 270},
		{"pointy -q+r", HexesArePointyTop(), spatial.Position{X: -1, Y: 1}, 210},
		{"pointy +r", HexesArePointyTop(), spatial.Position{X: 0, Y: 1}, 150},
		{"flat +q", HexesAreFlatTop(), spatial.Position{X: 1, Y: 0}, 120},
		{"flat +q-r", HexesAreFlatTop(), spatial.Position{X: 1, Y: -1}, 60},
		{"flat -r", HexesAreFlatTop(), spatial.Position{X: 0, Y: -1}, 0},
		{"flat -q", HexesAreFlatTop(), spatial.Position{X: -1, Y: 0}, 300},
		{"flat -q+r", HexesAreFlatTop(), spatial.Position{X: -1, Y: 1}, 240},
		{"flat +r", HexesAreFlatTop(), spatial.Position{X: 0, Y: 1}, 180},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			to := spatial.Position{X: from.X + tc.step.X, Y: from.Y + tc.step.Y}
			got, ok := bearing(tc.orientation, from, to)
			require.True(t, ok, "two different cells have a bearing")
			require.InDelta(t, tc.want, got, 1e-9)
			require.GreaterOrEqual(t, got, 0.0)
			require.Less(t, got, 360.0, "a bearing is in [0, 360): due north is 0, never 360")
		})
	}

	t.Run("a cell has no bearing to itself", func(t *testing.T) {
		_, ok := bearing(HexesArePointyTop(), from, from)
		require.False(t, ok)
	})
}

// TestTheBearingFrameIsTheWiresFrame pins the frame against the plane, so a
// swap of X and Y or a flipped sign cannot pass (rpg-api-protos#397): on the
// field's plane +X is east and +Y is south, a facing is 0 = north = -Y,
// 90 = east = +X, 180 = south = +Y, 270 = west = -X, clockwise, and a placed
// prop's angle (0 at +X, +90 at +Y) is (facing - 90) mod 360.
func TestTheBearingFrameIsTheWiresFrame(t *testing.T) {
	steps := []spatial.Position{{X: 1, Y: 0}, {X: 1, Y: -1}, {X: 0, Y: -1}, {X: -1, Y: 0}, {X: -1, Y: 1}, {X: 0, Y: 1}}
	for _, o := range []Orientation{HexesArePointyTop(), HexesAreFlatTop()} {
		plane := spatial.NewHexEmbedding(spatial.HexEmbeddingConfig{Orientation: o.spatial(), CellWidth: FeetPerCell})
		for _, step := range steps {
			from := spatial.Position{X: -3, Y: 2}
			to := spatial.Position{X: from.X + step.X, Y: from.Y + step.Y}
			a, b := plane.CellCentre(from), plane.CellCentre(to)
			propAngle := math.Mod(math.Atan2(b.Y-a.Y, b.X-a.X)*180/math.Pi+360, 360)
			got, ok := bearing(o, from, to)
			require.True(t, ok)
			require.InDelta(t, propAngle, math.Mod(got-90+360, 360), 1e-6,
				"%s step %v: a prop's angle is the facing less a quarter turn", o.Kind(), step)
		}
	}

	// The compass words name the same frame: the pointy-top +q step runs
	// along +X, due east; the flat-top -r step runs along -Y, due north.
	east, _ := FacingFromCompass("e")
	got, _ := bearing(HexesArePointyTop(), spatial.Position{}, spatial.Position{X: 1})
	require.Equal(t, east, got, "+X is e")
	north, _ := FacingFromCompass("n")
	got, _ = bearing(HexesAreFlatTop(), spatial.Position{}, spatial.Position{Y: -1})
	require.Equal(t, north, got, "-Y is n")
	south, _ := FacingFromCompass("s")
	got, _ = bearing(HexesAreFlatTop(), spatial.Position{}, spatial.Position{Y: 1})
	require.Equal(t, south, got, "+Y is s")
}

// TestTheEightCompassWords is the authored vocabulary's one conversion:
// eight true-compass words, 45 degrees apart from north, and nothing else.
func TestTheEightCompassWords(t *testing.T) {
	for word, want := range map[string]float64{
		"n": 0, "ne": 45, "e": 90, "se": 135, "s": 180, "sw": 225, "w": 270, "nw": 315,
	} {
		got, err := FacingFromCompass(word)
		require.NoError(t, err, word)
		require.Equal(t, want, got, word)
	}

	for _, word := range []string{"north", "N", "", "nne", "up"} {
		_, err := FacingFromCompass(word)
		require.Error(t, err, "%q is not one of the eight words", word)
		require.True(t, errors.Is(err, ErrBadFacing), "%q is refused as a facing", word)
	}
}

// TestAFacingIsABearing is the door's refusal: a stated facing is a bearing
// in [0, 360), and anything else is refused by name rather than wrapped or
// clamped into one somebody did not write.
func TestAFacingIsABearing(t *testing.T) {
	for _, ok := range []float64{0, 90, 359.5} {
		require.NoError(t, validateFacing(&ok), "%v is a bearing", ok)
	}
	require.NoError(t, validateFacing(nil), "no facing stated is the rule's 180, not a refusal")
	for _, bad := range []float64{-1, 360, 720} {
		require.ErrorIs(t, validateFacing(&bad), ErrBadFacing, "%v is not a bearing", bad)
	}
}

// TestAnUnstatedFacingIsSouth is R13's rule for the seed: a member nobody
// gave a facing faces 180, as a rules bearing and not a picture's default.
func TestAnUnstatedFacingIsSouth(t *testing.T) {
	require.Equal(t, 180.0, seedFacing(nil))
	east := 90.0
	require.Equal(t, 90.0, seedFacing(&east))
}
