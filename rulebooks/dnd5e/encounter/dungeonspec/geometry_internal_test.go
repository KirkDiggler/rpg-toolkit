// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package dungeonspec

import (
	"math"
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter"
	"github.com/KirkDiggler/rpg-toolkit/tools/spatial"
)

// geometry_internal_test.go is THE CALIBRATION (rpg-project#360, wall-geometry
// design §4.3 C10).
//
// Four numbers decide what a wall costs, and the design states all four in
// closed form: a midpoint line shaves 1/24, a quarter line shaves 5/24, a
// square room's inside corner keeps 3/4, a hexagonal room's corner keeps 7/12.
// They are why MinStandable is 0.7 and not 0.75 — the square corner must not be
// decided by float noise — so the whole standability rule rests on them being
// what the design says they are.
//
// EVERY EXPECTATION HERE IS THE DESIGN'S OWN ARITHMETIC, written as the
// fraction it is, never as a number read back out of this code. A test that
// echoed the implementation would pass on a wrong embedding.

type GeometrySuite struct {
	suite.Suite

	pointy hexGeom
	flat   hexGeom
}

func TestGeometrySuite(t *testing.T) {
	suite.Run(t, new(GeometrySuite))
}

func (s *GeometrySuite) SetupTest() {
	s.pointy = geometryOf(encounter.HexesArePointyTop())
	s.flat = geometryOf(encounter.HexesAreFlatTop())
}

// cell is a whole axial cell.
func cell(q, r int) spatial.Position {
	return spatial.Position{X: float64(q), Y: float64(r)}
}

// wallThrough is the world segment between two fractional axial points — the
// shape [hexGeom.standingFraction] and [hexGeom.meets] take a wall in.
func (s *GeometrySuite) wallThrough(g hexGeom, from, to axialPoint) [2]worldPoint {
	return [2]worldPoint{g.world(from), g.world(to)}
}

// TestTheSevenPositionsAreTheMidpointsAndTheCentre pins the offset table
// against the embedding it describes: each of the six is the midpoint of the
// side it names, in bounding-box fractions, and the seventh is the middle.
//
// The table is written as literals because the file's offsets are literals and
// must compare exactly (design F8, "compare as exact floats"). This is what
// stops the literals and the geometry drifting apart.
func (s *GeometrySuite) TestTheSevenPositionsAreTheMidpointsAndTheCentre() {
	for _, tc := range []struct {
		name string
		g    hexGeom
	}{{"pointy", s.pointy}, {"flat", s.flat}} {
		s.Run(tc.name, func() {
			g := tc.g
			origin := cell(0, 0)
			centre := g.centreOf(origin)

			s.Require().Len(g.sides, 6, "a hex has six sides")
			for _, side := range g.sides {
				neighbour := cell(side.Step[0], side.Step[1])
				midpoint := worldPoint{
					X: (centre.X + g.centreOf(neighbour).X) / 2,
					Y: (centre.Y + g.centreOf(neighbour).Y) / 2,
				}
				s.InDelta(side.Offset[0], (midpoint.X-centre.X)/g.width, 1e-12,
					"offset %v x is the side midpoint in widths", side.Offset)
				s.InDelta(side.Offset[1], (midpoint.Y-centre.Y)/g.height, 1e-12,
					"offset %v y is the side midpoint in heights", side.Offset)

				at, ok := g.axialAt(origin, side.Offset)
				s.Require().True(ok)
				s.Equal(float64(side.Step[0])/2, at.Q, "half the step, exactly")
				s.Equal(float64(side.Step[1])/2, at.R, "half the step, exactly")
			}

			at, ok := g.axialAt(cell(3, 4), centreOffset)
			s.Require().True(ok)
			s.Equal(axialPoint{Q: 3, R: 4}, at, "the centre is the cell itself")

			_, ok = g.axialAt(origin, [2]float64{0.3, 0.3})
			s.False(ok, "an offset outside the seven is not a position")
		})
	}
}

// TestTheBoundingBoxIsTheOffsetUnit pins the two spans the offsets are
// fractions of: a pointy-top hex of circumradius 1 is sqrt(3) wide and 2 tall,
// and a flat-top one is the same hex turned 30°.
func (s *GeometrySuite) TestTheBoundingBoxIsTheOffsetUnit() {
	s.InDelta(math.Sqrt(3), s.pointy.width, 1e-12)
	s.InDelta(2.0, s.pointy.height, 1e-12)
	s.InDelta(2.0, s.flat.width, 1e-12)
	s.InDelta(math.Sqrt(3), s.flat.height, 1e-12)

	for _, tc := range []struct {
		name string
		g    hexGeom
	}{{"pointy", s.pointy}, {"flat", s.flat}} {
		s.Run(tc.name, func() {
			hex := tc.g.hexOf(cell(0, 0))
			var wide, tall float64
			for _, c := range hex {
				wide = math.Max(wide, 2*math.Abs(c.X))
				tall = math.Max(tall, 2*math.Abs(c.Y))
			}
			s.InDelta(tc.g.width, wide, 1e-12, "the corners span the width")
			s.InDelta(tc.g.height, tall, 1e-12, "the corners span the height")
			s.InDelta(hexArea, polygonArea(hex), 1e-12, "3*sqrt(3)/2 at circumradius 1")
		})
	}
}

// TestAMidpointLineShavesOneTwentyFourth is calibration number one (design
// §4.3, "along a row, midpoint line (thin) — shaved 1/24 both sides").
//
// The line runs along a row through the slanted-side midpoints. It cuts a
// triangle off every cell it passes, above and below alike, and the triangle is
// 1/24 of a hex: base 3/4 of a width, height 1/8 of a height.
func (s *GeometrySuite) TestAMidpointLineShavesOneTwentyFourth() {
	g := s.pointy
	// Along row 0, on the south midpoints: [0.25,0.375] of each cell.
	wall := s.wallThrough(g, axialPoint{Q: -0.5, R: 0.5}, axialPoint{Q: 3.5, R: 0.5})
	deg, ok := g.directionOf(axialPoint{Q: -0.5, R: 0.5}, axialPoint{Q: 3.5, R: 0.5})
	s.Require().True(ok)
	s.InDelta(0.0, deg, 1e-9, "along the row")

	for _, c := range []spatial.Position{cell(0, 0), cell(1, 0), cell(2, 0)} {
		s.Require().True(g.meets(c, wall[0], wall[1]), "row 0 cell %v is footprint", c)
		s.InDelta(23.0/24.0, g.standingFraction(c, [][2]worldPoint{wall}), 1e-12,
			"the row above keeps 23/24")
	}
	for _, c := range []spatial.Position{cell(0, 1), cell(1, 1), cell(2, 1)} {
		s.Require().True(g.meets(c, wall[0], wall[1]), "row 1 cell %v is footprint", c)
		s.InDelta(23.0/24.0, g.standingFraction(c, [][2]worldPoint{wall}), 1e-12,
			"the row below keeps 23/24")
	}

	s.False(g.meets(cell(0, -1), wall[0], wall[1]), "a row further off is untouched")
	s.False(g.meets(cell(0, 2), wall[0], wall[1]), "and so is the row past it")
}

// TestAQuarterLineShavesFiveTwentyFourths is calibration number two (design
// §4.3, "across rows, quarter line (thin) — shaved 5/24, alternating").
//
// The line runs across rows a quarter of a width from the centres it passes,
// and takes a trapezoid off the cells on both sides — alternating, because the
// rows stagger: the cell it shaves in one row is west of it and in the next is
// east.
func (s *GeometrySuite) TestAQuarterLineShavesFiveTwentyFourths() {
	g := s.pointy
	from, to := axialPoint{Q: 0.5, R: -0.5}, axialPoint{Q: -1.5, R: 3.5}
	wall := s.wallThrough(g, from, to)
	deg, ok := g.directionOf(from, to)
	s.Require().True(ok)
	s.InDelta(90.0, deg, 1e-9, "across the rows")

	for _, c := range []spatial.Position{cell(0, 0), cell(-1, 2)} {
		s.Require().True(g.meets(c, wall[0], wall[1]), "cell %v is footprint", c)
		s.InDelta(19.0/24.0, g.standingFraction(c, [][2]worldPoint{wall}), 1e-12,
			"a cell west of the line keeps 19/24")
		s.Less(g.centreOf(c).X, wall[0].X, "and its centre is west of it")
	}
	for _, c := range []spatial.Position{cell(0, 1), cell(-1, 3)} {
		s.Require().True(g.meets(c, wall[0], wall[1]), "cell %v is footprint", c)
		s.InDelta(19.0/24.0, g.standingFraction(c, [][2]worldPoint{wall}), 1e-12,
			"the alternating cell east of the line keeps 19/24")
		s.Greater(g.centreOf(c).X, wall[0].X, "and its centre is east of it")
	}

	s.False(g.meets(cell(1, 0), wall[0], wall[1]), "the cell beyond is untouched")
	s.False(g.meets(cell(-1, 0), wall[0], wall[1]), "and so is the one behind")
}

// TestASquareInsideCornerKeepsThreeQuarters is calibration number three
// (design §4.3: "a square room's inside corner (quarter line + midpoint line)
// keeps exactly 3/4").
//
// THIS IS WHY THE THRESHOLD IS 0.7 AND NOT 0.75. The two shavings — 5/24 and
// 1/24 — meet at the corner position and overlap in nothing but that point, so
// the corner cell keeps exactly six twenty-fourths less than the whole. A
// threshold of 0.75 would decide this cell on the last bit of a float; 0.7
// leaves it standable with room to spare.
func (s *GeometrySuite) TestASquareInsideCornerKeepsThreeQuarters() {
	g := s.pointy
	corner := axialPoint{Q: 0, R: 0.5} // [0.25,0.375] of cell (0,0)
	across := s.wallThrough(g, axialPoint{Q: 0.5, R: -0.5}, corner)
	along := s.wallThrough(g, corner, axialPoint{Q: 3, R: 0.5})

	s.Require().True(g.meets(cell(0, 0), across[0], across[1]))
	s.Require().True(g.meets(cell(0, 0), along[0], along[1]),
		"the wall leaving the corner still stands on the corner cell")

	s.InDelta(0.75, g.standingFraction(cell(0, 0), [][2]worldPoint{across, along}), 1e-12,
		"18/24 — the two cuts share only the corner point")
	s.Greater(0.75, MinStandable, "and 3/4 clears the threshold")
}

// TestAHexagonalCornerKeepsSevenTwelfths is calibration number four (design
// §4.3: "a hexagonal room's corner (two quarter lines) keeps 7/12").
//
// Two quarter lines meeting at 60° take 5/24 each and overlap in nothing but
// the corner position, so the corner cell keeps 14/24 — BELOW the threshold.
// The design's own answer to that is not a special case: the cell goes scenery
// and the designer shows it.
func (s *GeometrySuite) TestAHexagonalCornerKeepsSevenTwelfths() {
	g := s.pointy
	corner := axialPoint{Q: 0, R: 0.5}
	first := s.wallThrough(g, axialPoint{Q: 0.5, R: -0.5}, corner)
	second := s.wallThrough(g, corner, axialPoint{Q: 1, R: 1.5})

	degA, okA := g.directionOf(axialPoint{Q: 0.5, R: -0.5}, corner)
	degB, okB := g.directionOf(corner, axialPoint{Q: 1, R: 1.5})
	s.Require().True(okA)
	s.Require().True(okB)
	s.InDelta(90.0, degA, 1e-9)
	s.InDelta(30.0, degB, 1e-9)

	s.InDelta(7.0/12.0, g.standingFraction(cell(0, 0), [][2]worldPoint{first, second}), 1e-12,
		"14/24 — two quarter cuts, sharing only the corner point")
	s.Less(7.0/12.0, MinStandable, "and 7/12 does not clear the threshold")
}

// TestAThickWallHalvesTheCellsItCentres pins the mechanism behind "a thick
// wall seals the cells it runs through the centre of" (design §1.7, F15):
// there is no special case, only a half-plane through a centre leaving half.
func (s *GeometrySuite) TestAThickWallHalvesTheCellsItCentres() {
	g := s.pointy
	// The centre line along row 0.
	wall := s.wallThrough(g, axialPoint{Q: -1, R: 0}, axialPoint{Q: 4, R: 0})
	for _, c := range []spatial.Position{cell(0, 0), cell(1, 0), cell(2, 0)} {
		s.InDelta(0.5, g.standingFraction(c, [][2]worldPoint{wall}), 1e-12,
			"a wall through a centre leaves half")
		s.Less(0.5, MinStandable, "which is below the threshold")
	}
	s.False(g.meets(cell(0, 1), wall[0], wall[1]), "and the next row is whole")
	s.False(g.meets(cell(0, -1), wall[0], wall[1]))
}

// TestAFlatSideWallLeavesItsNeighboursWhole pins the other thick line (design
// §4.3, "across rows, flat-side line — whole; one odd-row cell sealed per two
// rows"): it runs ALONG a side, so it stands on the cells either side of it and
// takes nothing from them, and halves only the staggered cells it centres.
func (s *GeometrySuite) TestAFlatSideWallLeavesItsNeighboursWhole() {
	g := s.pointy
	// x = sqrt(3)/2: the east side of column 0's even rows, and the middle
	// of the cells that stagger into it.
	from, to := axialPoint{Q: 0.5, R: 0}, axialPoint{Q: -1.5, R: 4}
	wall := s.wallThrough(g, from, to)
	deg, ok := g.directionOf(from, to)
	s.Require().True(ok)
	s.InDelta(90.0, deg, 1e-9)

	for _, c := range []spatial.Position{cell(0, 0), cell(0, 2), cell(-1, 4)} {
		s.True(g.meets(c, wall[0], wall[1]), "the wall stands on %v", c)
		s.InDelta(1.0, g.standingFraction(c, [][2]worldPoint{wall}), 1e-12,
			"and leaves it whole")
	}
	for _, c := range []spatial.Position{cell(0, 1), cell(-1, 3)} {
		s.True(g.meets(c, wall[0], wall[1]))
		s.InDelta(0.5, g.standingFraction(c, [][2]worldPoint{wall}), 1e-12,
			"the staggered cell is centred and halved")
	}
}

// TestTheTwelveDirections pins design F13: twelve bearings 30° apart are legal
// and everything between them is refused.
func (s *GeometrySuite) TestTheTwelveDirections() {
	g := s.pointy
	// Every 30° step is realizable between two positions of the closed set:
	// six neighbour steps from a centre, and six midpoints of one cell.
	seen := map[int]bool{}
	origin := cell(0, 0)
	for _, side := range g.sides {
		at, ok := g.axialAt(origin, side.Offset)
		s.Require().True(ok)
		deg, legal := g.directionOf(axialPoint{Q: 0, R: 0}, at)
		s.Require().True(legal, "a centre-to-midpoint ray is one of the twelve")
		seen[int(math.Round(deg))] = true

		// The ray on to the neighbour's far midpoint is the same bearing.
		far, ok := g.axialAt(cell(side.Step[0], side.Step[1]), side.Offset)
		s.Require().True(ok)
		deg2, legal2 := g.directionOf(at, far)
		s.Require().True(legal2)
		s.InDelta(deg, deg2, 1e-9, "the ray keeps its bearing")
	}
	// The six hex-side rays sit between the six neighbour rays. Each is taken
	// both ways round, which is what makes twelve rather than six.
	for _, pair := range [][2]axialPoint{
		{{Q: 0, R: 0.5}, {Q: 1, R: 1.5}},    // 30°
		{{Q: 0, R: 0.5}, {Q: -2, R: 1.5}},   // 150°
		{{Q: 0.5, R: -0.5}, {Q: 0, R: 0.5}}, // 90°
	} {
		deg, ok := g.directionOf(pair[0], pair[1])
		s.Require().True(ok, "a hex-side bearing is legal")
		s.InDelta(0.0, math.Mod(deg, 30), 1e-9)
		seen[int(math.Round(deg))] = true

		back, ok := g.directionOf(pair[1], pair[0])
		s.Require().True(ok, "and so is the same line read the other way")
		seen[int(math.Round(back))] = true
	}
	s.Len(seen, 12, "all twelve are reachable from the closed set, and no thirteenth")

	// And the refusals: a bearing off the grid.
	for _, off := range []axialPoint{
		{Q: 1, R: 0.5},   // a shallow ray between 0° and 30°
		{Q: 0.5, R: 3.5}, // the tomb's straight seam read end to end
	} {
		_, ok := g.directionOf(axialPoint{Q: 0, R: 0}, off)
		s.False(ok, "%v is not one of the twelve", off)
	}
}

// TestClippingIsRatioOnly pins the two helpers the standability rule is built
// from, against answers computed by hand rather than by them.
func (s *GeometrySuite) TestClippingIsRatioOnly() {
	square := []worldPoint{{X: 0, Y: 0}, {X: 2, Y: 0}, {X: 2, Y: 2}, {X: 0, Y: 2}}
	s.InDelta(4.0, polygonArea(square), 1e-12)

	// Keep x <= 1: half the square.
	half := clipHalfPlane(square, 1, 0, 1)
	s.InDelta(2.0, polygonArea(half), 1e-12)

	// Keep y <= 0: the bottom edge, which has no area.
	edge := clipHalfPlane(square, 0, 1, 0)
	s.InDelta(0.0, polygonArea(edge), 1e-12)

	// Wholly outside.
	s.Empty(clipHalfPlane(square, 1, 0, -1))
}
