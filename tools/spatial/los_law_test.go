package spatial_test

import (
	"fmt"
	"math"
	"math/rand"
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/KirkDiggler/rpg-toolkit/core"
	"github.com/KirkDiggler/rpg-toolkit/tools/spatial"
)

// LineOfSightLawSuite holds the two properties sight must satisfy in every grid
// family, over rooms nobody wrote by hand.
//
// They are laws rather than cases because rpg-toolkit#1022 was a defect no case
// would have caught: sight disagreed with itself by direction on squares, and
// hid roughly one in seven things a player should have seen on hexes. Both had
// been shipped for a long time under a suite of green hand-written examples.
//
// FIXTURE RULE FOR ANYONE ADDING TO THIS FILE — walls in hex tests are built
// and classified IN THE PLANE, never by offset column. A hex offset column is a
// DIAGONAL once embedded, so "cells with X == 4" is not a wall and "cells with
// X < 4" is not a side of one. The first version of the leak law below did
// exactly that and produced a confident, meaningless number: it reported the
// rule leaking through a wall that had holes in it by construction. If a test
// here needs a barrier, it gets one from plane geometry.
type LineOfSightLawSuite struct {
	suite.Suite
}

func TestLineOfSightLawSuite(t *testing.T) {
	suite.Run(t, new(LineOfSightLawSuite))
}

const (
	lawRoomType   = "law"
	lawGridSquare = "square"

	// blockersPerRoom is how cluttered a fuzzed room gets. Eight in an 8x8 is
	// enough to produce the corner-clipping cases the old rule disagreed with
	// itself about — it measured 4.97% asymmetric at this density.
	blockersPerRoom = 8
)

// opaque is a wall for the purposes of these laws.
type opaque struct{ id string }

func (o *opaque) GetID() string            { return o.id }
func (o *opaque) GetType() core.EntityType { return "wall" }
func (o *opaque) GetSize() int             { return 1 }
func (o *opaque) BlocksMovement() bool     { return true }
func (o *opaque) BlocksLineOfSight() bool  { return true }

// lawGrid is one grid family plus the plane embedding its cells live in, which
// is what lets a test say "this wall is solid" in a way that means the same
// thing on squares and hexes.
type lawGrid struct {
	name   string
	grid   spatial.Grid
	cells  []spatial.Position
	centre func(spatial.Position) (float64, float64)
	// half is the cell's inradius in that embedding: a wall of cells whose
	// centres lie within half of a line leaves no gap for sight to pass.
	half float64
	// tiles reports whether this family's cells tile the plane. Gridless is
	// the one that does not — see TestASolidWallLeaksNothing.
	tiles bool
}

// hexPixel embeds a cube coordinate in the plane, pointy-top, circumradius 1.
func hexPixel(c spatial.CubeCoordinate) (float64, float64) {
	q, r := float64(c.X), float64(c.Z)
	return math.Sqrt(3) * (q + r/2), 1.5 * r
}

func lawGrids() []lawGrid {
	const w, h = 8, 8
	rect := func() []spatial.Position {
		out := make([]spatial.Position, 0, w*h)
		for x := 0; x < w; x++ {
			for y := 0; y < h; y++ {
				out = append(out, spatial.Position{X: float64(x), Y: float64(y)})
			}
		}
		return out
	}

	grids := []lawGrid{{
		name:   lawGridSquare,
		grid:   spatial.NewSquareGrid(spatial.SquareGridConfig{Width: w, Height: h}),
		cells:  rect(),
		centre: func(p spatial.Position) (float64, float64) { return p.X, p.Y },
		half:   0.5,
		tiles:  true,
	}}

	for _, o := range []struct {
		label string
		or    spatial.HexOrientation
	}{
		{"hex-pointy", spatial.HexOrientationPointyTop},
		{"hex-flat", spatial.HexOrientationFlatTop},
	} {
		orientation := o.or
		grids = append(grids, lawGrid{
			name:  o.label,
			grid:  spatial.NewHexGrid(spatial.HexGridConfig{Width: w, Height: h, Orientation: orientation}),
			cells: rect(),
			centre: func(p spatial.Position) (float64, float64) {
				return hexPixel(spatial.OffsetCoordinateToCubeWithOrientation(p, orientation))
			},
			half:  math.Sqrt(3) / 2,
			tiles: true,
		})
	}

	const span = 4
	axial := spatial.NewAxialHexGrid(spatial.AxialHexGridConfig{SpanWidth: span, SpanHeight: span})
	axialCells := []spatial.Position{}
	for q := -span; q <= span; q++ {
		for r := -span; r <= span; r++ {
			p := spatial.Position{X: float64(q), Y: float64(r)}
			if axial.IsValidPosition(p) {
				axialCells = append(axialCells, p)
			}
		}
	}
	grids = append(grids, lawGrid{
		name:  "axial-hex",
		grid:  axial,
		cells: axialCells,
		centre: func(p spatial.Position) (float64, float64) {
			return hexPixel(spatial.CubeCoordinate{X: int(p.X), Y: int(-p.X - p.Y), Z: int(p.Y)})
		},
		half:  math.Sqrt(3) / 2,
		tiles: true,
	})

	grids = append(grids, lawGrid{
		name: "gridless",
		grid: spatial.NewGridlessRoom(spatial.GridlessConfig{Width: w, Height: h}),
		cells: func() []spatial.Position {
			out := []spatial.Position{}
			for x := 0; x < w; x++ {
				for y := 0; y < h; y++ {
					out = append(out, spatial.Position{X: float64(x), Y: float64(y)})
				}
			}
			return out
		}(),
		centre: func(p spatial.Position) (float64, float64) { return p.X, p.Y },
		half:   0.5,
		tiles:  false,
	})

	return grids
}

// TestSightIsSymmetric is the law rpg-toolkit#1022 is named for: what two cells
// can see of each other cannot depend on which one asked.
//
// Fuzzed rather than enumerated. The defect this replaces was a rasterizer tie
// broken one way going out and the other coming back, which no hand-written
// case had happened to straddle — before this law, squares disagreed with
// themselves on 4.97% of pairs in a cluttered room, and the suite was green.
func (s *LineOfSightLawSuite) TestSightIsSymmetric() {
	//nolint:gosec // G404: seeded for reproducible fuzz rooms, not cryptographic
	rng := rand.New(rand.NewSource(1022))

	for _, g := range lawGrids() {
		s.Run(g.name, func() {
			for run := 0; run < 12; run++ {
				room := spatial.NewBasicRoom(spatial.BasicRoomConfig{
					ID: lawRoomType, Type: lawRoomType, Grid: g.grid,
				})
				// Distinct cells, and every placement is REQUIRED to land.
				// Drawing at random and discarding the error looked like eight
				// blockers and was sometimes fewer: opaque blocks movement, so
				// a repeat draw collides and is refused. A fuzz whose strength
				// is unknown is a fuzz that quietly weakens.
				chosen := make(map[spatial.Position]bool, blockersPerRoom)
				for len(chosen) < blockersPerRoom {
					cell := g.cells[rng.Intn(len(g.cells))]
					if chosen[cell] {
						continue
					}
					chosen[cell] = true
					s.Require().NoError(room.PlaceEntity(
						&opaque{id: fmt.Sprintf("w%d-%d", run, len(chosen))}, cell))
				}
				s.Require().Len(chosen, blockersPerRoom, "the room must be as cluttered as claimed")
				for i := 0; i < len(g.cells); i++ {
					for j := i + 1; j < len(g.cells); j++ {
						a, b := g.cells[i], g.cells[j]
						s.Require().Equal(
							room.IsLineOfSightBlocked(a, b),
							room.IsLineOfSightBlocked(b, a),
							"run %d: sight between %v and %v depends on who asked", run, a, b)
					}
				}
			}
		})
	}
}

// TestASolidWallLeaksNothing is the law that keeps the other one honest.
//
// Sight became more permissive with rpg-toolkit#1022 — that was the point, the
// old rule hid about one in seven things a player should have seen. Permissive
// has a floor: a barrier with no gap in it must stop everything, or "you can
// lean around a corner" quietly becomes "you can see through walls". Every
// future loosening has to clear this.
//
// The wall is built in the plane (see the fixture rule on the suite), so it is
// genuinely solid in every family rather than solid-looking in one coordinate
// system and full of holes in another.
//
// GRIDLESS IS EXEMPT, and not as a convenience. Its positions are points in
// continuous space with no cell around them, so entities placed on a lattice
// leave the space between them empty and no arrangement of them is a barrier
// at all. That is a fact about the model, not about the sight rule — and it is
// the same fact that keeps gridless on the single-lane rule in
// IsLineOfSightBlocked. Gridless still owes the symmetry law above.
func (s *LineOfSightLawSuite) TestASolidWallLeaksNothing() {
	for _, g := range lawGrids() {
		if !g.tiles {
			continue
		}
		s.Run(g.name, func() {
			room := spatial.NewBasicRoom(spatial.BasicRoomConfig{
				ID: "wall", Type: lawRoomType, Grid: g.grid,
			})

			minX, maxX := math.Inf(1), math.Inf(-1)
			for _, c := range g.cells {
				x, _ := g.centre(c)
				minX, maxX = math.Min(minX, x), math.Max(maxX, x)
			}
			wallX := (minX + maxX) / 2

			side := map[spatial.Position]int{}
			walls := 0
			for _, c := range g.cells {
				x, _ := g.centre(c)
				if math.Abs(x-wallX) <= g.half {
					s.Require().NoError(room.PlaceEntity(&opaque{id: fmt.Sprintf("wall%d", walls)}, c))
					walls++
					continue
				}
				if x < wallX {
					side[c] = -1
				} else {
					side[c] = 1
				}
			}
			s.Require().Positive(walls, "the fixture must actually build a wall")

			crossings := 0
			for i := 0; i < len(g.cells); i++ {
				for j := i + 1; j < len(g.cells); j++ {
					a, b := g.cells[i], g.cells[j]
					if side[a] == 0 || side[b] == 0 || side[a] == side[b] {
						continue
					}
					crossings++
					s.Require().True(room.IsLineOfSightBlocked(a, b),
						"%v sees %v through a solid wall", a, b)
				}
			}
			s.Require().Positive(crossings, "the fixture must actually pose the question")
		})
	}
}

// TestLeaningIsNotWandering pins the guard that keeps an alternative lane
// honest: a neighbour must be strictly CLOSER to the other end, not merely no
// further from it.
//
// One blocker, diagonally between two cells two steps apart. The only cells
// that would restore sight are the ones beside the viewer — a full cell
// sideways, no closer to the target than where they already stand. Letting
// those count turns leaning around an obstacle into wandering to a different
// vantage, and it is measurably worse: with the looser guard, hexes let
// through 88 pairs the game's corner rule denies instead of 3, and squares 226
// instead of 158.
//
// Being honest about the cost, since this test is where someone will come
// looking: the corner rule would allow THIS pair. Strict is deliberately the
// more conservative of the two grid-native options — it gives up a little on
// cases like this one to give up much less on seeing through things elsewhere.
// If that trade is ever revisited, revisit it here, with numbers.
func (s *LineOfSightLawSuite) TestLeaningIsNotWandering() {
	for _, tc := range []struct {
		name    string
		grid    spatial.Grid
		blocker spatial.Position
		from    spatial.Position
		// sideways is reached only by giving ground, so it stays blocked.
		sideways spatial.Position
		// forward is reached by a neighbour that closes the distance.
		forward spatial.Position
	}{
		{
			name:     lawGridSquare,
			grid:     spatial.NewSquareGrid(spatial.SquareGridConfig{Width: 6, Height: 6}),
			blocker:  spatial.Position{X: 1, Y: 1},
			from:     spatial.Position{X: 0, Y: 0},
			sideways: spatial.Position{X: 2, Y: 2},
			forward:  spatial.Position{X: 0, Y: 2},
		},
		{
			name: "hex-pointy",
			grid: spatial.NewHexGrid(spatial.HexGridConfig{
				Width: 6, Height: 6, Orientation: spatial.HexOrientationPointyTop,
			}),
			blocker:  spatial.Position{X: 0, Y: 1},
			from:     spatial.Position{X: 0, Y: 0},
			sideways: spatial.Position{X: 0, Y: 2},
			forward:  spatial.Position{X: 1, Y: 2},
		},
	} {
		s.Run(tc.name, func() {
			room := spatial.NewBasicRoom(spatial.BasicRoomConfig{
				ID: "lean", Type: lawRoomType, Grid: tc.grid,
			})
			s.Require().NoError(room.PlaceEntity(&opaque{id: "pillar"}, tc.blocker))

			s.True(room.IsLineOfSightBlocked(tc.from, tc.sideways),
				"a sideways step is not a lane: it gives no ground toward the target")
			s.True(room.IsLineOfSightBlocked(tc.sideways, tc.from), "and the answer is symmetric")

			// The guard is about DIRECTION, not about refusing alternatives. The
			// same blocker, a target reached by a neighbour that genuinely closes
			// the distance: sight comes back. Without this half the test above
			// would also pass on an implementation that blocked everything.
			s.False(room.IsLineOfSightBlocked(tc.from, tc.forward),
				"a neighbour that closes the distance is a real lane")
			s.False(room.IsLineOfSightBlocked(tc.forward, tc.from))
		})
	}
}

// TestMovementDoesNotConsultSight pins the separation rpg-toolkit#1022 was
// careful not to cross.
//
// Sight and movement share a rasterizer — GetLineOfSight — and the sight rule
// changed while the rasterizer did not. That is deliberate and it is the whole
// reason movement was safe to leave alone: MoveEntity and the movement query
// walk the ray themselves and consult movement boundaries, never
// IsLineOfSightBlocked. Whether a charge lane or a reach check WANTS the
// permissive rule is a separate question per call site, and answering it
// silently for all of them would have been a gameplay change nobody asked for.
//
// This is a structural pin, not a behavioural one: it fails the day something
// routes movement through sight.
func (s *LineOfSightLawSuite) TestMovementDoesNotConsultSight() {
	grid := spatial.NewSquareGrid(spatial.SquareGridConfig{Width: 8, Height: 8})
	room := spatial.NewBasicRoom(spatial.BasicRoomConfig{
		ID: "movement", Type: lawRoomType, Grid: grid,
	})

	// A curtain wall: opaque along its whole length, and no obstacle at all to
	// anything walking. Solid enough that even the permissive rule stops.
	for y := 0; y < 8; y++ {
		s.Require().NoError(room.PlaceEntity(
			&sightOnly{id: fmt.Sprintf("curtain%d", y)}, spatial.Position{X: 4, Y: float64(y)}))
	}

	mover := &sightOnly{id: "mover", transparent: true}
	s.Require().NoError(room.PlaceEntity(mover, spatial.Position{X: 3, Y: 4}))

	s.True(room.IsLineOfSightBlocked(spatial.Position{X: 3, Y: 4}, spatial.Position{X: 5, Y: 4}),
		"the curtain is solid, so sight stops at it")

	// And movement walks straight through it, because movement never asked.
	s.Require().NoError(room.MoveEntity(mover.GetID(), spatial.Position{X: 4, Y: 4}),
		"an opaque entity that does not block movement must not stop a mover")
	s.Require().NoError(room.MoveEntity(mover.GetID(), spatial.Position{X: 5, Y: 4}),
		"including straight out the other side")
}

// sightOnly blocks line of sight and never movement, which is the combination
// that tells the two systems apart.
type sightOnly struct {
	id          string
	transparent bool
}

func (o *sightOnly) GetID() string            { return o.id }
func (o *sightOnly) GetType() core.EntityType { return "curtain" }
func (o *sightOnly) GetSize() int             { return 1 }
func (o *sightOnly) BlocksMovement() bool     { return false }
func (o *sightOnly) BlocksLineOfSight() bool  { return !o.transparent }
