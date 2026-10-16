// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package encounter

// footprint_bounds_internal_test.go is THE PROOF THAT THE PRE-CHECK DECIDES
// NOTHING (rpg-toolkit#2049).
//
// [footprintBounds.mayTouch] skips a [spatial.TraceFootprint] call it can
// prove misses. These tests hold it to that: the predicate on its own (far,
// touching, crossing, degenerate, non-finite), the predicate against spatial
// on many random rectangles and segments, and a whole field asked every
// question its footprint readers answer with the pre-check and with it
// switched off — the zero [footprintBounds], which traces everything.
//
// Internal because the pre-check is a private optimisation of private reads;
// no exported answer is supposed to be able to tell it exists.

import (
	"fmt"
	"math"
	"math/rand"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"

	"github.com/KirkDiggler/rpg-toolkit/tools/spatial"
)

// FootprintBoundsSuite holds the pre-check to the answers spatial gives.
type FootprintBoundsSuite struct {
	suite.Suite
}

func TestFootprintBoundsSuite(t *testing.T) {
	suite.Run(t, new(FootprintBoundsSuite))
}

func boxPlacement(w, d, facing float64, origin, offset spatial.Point) spatial.FootprintPlacement {
	return spatial.FootprintPlacement{
		Footprint:   spatial.Footprint{Box: &spatial.Box{W: w, D: d}},
		Origin:      origin,
		Facing:      facing,
		LocalOffset: offset,
	}
}

func (fs *FootprintBoundsSuite) TestMayTouchRejectsOnlyWhatCannotTouch() {
	t := fs.T()
	// A 10ft-deep, 2ft-wide box facing 0: it spans x in [-5, 5], y in [-1, 1]
	// around an origin of (100, 50).
	origin := spatial.Point{X: 100, Y: 50}
	b := boundsOf(boxPlacement(2, 10, 0, origin, spatial.Point{}))
	require.True(t, b.ok)

	p := func(x, y float64) spatial.Point { return spatial.Point{X: x, Y: y} }

	require.False(t, b.mayTouch(origin, p(200, 200), p(210, 220)),
		"a segment far from the box is rejected")
	require.False(t, b.mayTouch(origin, p(90, 52), p(110, 52)),
		"a segment one foot beyond the box's long side is rejected")
	require.True(t, b.mayTouch(origin, p(90, 51), p(110, 51)),
		"a segment lying ON the long side touches the expanded box and is kept")
	require.True(t, b.mayTouch(origin, p(105, 40), p(105, 60)),
		"a segment touching only the short end's edge is kept")
	require.True(t, b.mayTouch(origin, p(100, 40), p(100, 60)),
		"a segment crossing the box is kept")
	require.True(t, b.mayTouch(origin, p(100, 50), p(100, 50)),
		"a zero-length segment inside the box is kept")
	require.True(t, b.mayTouch(origin, p(105, 51), p(105, 51)),
		"a zero-length segment exactly on a corner is kept")
	require.False(t, b.mayTouch(origin, p(106, 50), p(106, 50)),
		"a zero-length segment beyond the end is rejected")

	moved := spatial.Point{X: 200, Y: 200}
	require.True(t, b.mayTouch(moved, p(200, 200), p(200, 200)),
		"the reach is relative to the origin, so a dropped placement's box moves with it")
	require.False(t, b.mayTouch(moved, p(100, 50), p(100, 50)),
		"and no longer covers where it stood before")

	rotated := boundsOf(boxPlacement(2, 10, 90, origin, spatial.Point{X: 3}))
	require.True(t, rotated.mayTouch(origin, p(100, 53), p(100, 53)),
		"facing turns the box and its local offset: 90 degrees moves the centre to (100, 53)")
	require.False(t, rotated.mayTouch(origin, p(103, 53), p(103, 53)),
		"and its width now lies along x")
}

func (fs *FootprintBoundsSuite) TestMayTouchNeverHidesAnError() {
	t := fs.T()
	origin := spatial.Point{X: 1, Y: 1}
	b := boundsOf(boxPlacement(1, 1, 0, origin, spatial.Point{}))
	far := spatial.Point{X: 1000, Y: 1000}
	for _, bad := range []float64{math.NaN(), math.Inf(1), math.Inf(-1)} {
		require.True(t, b.mayTouch(origin, spatial.Point{X: bad, Y: 1000}, far),
			"a non-finite endpoint (%v) is spatial's to refuse, never the filter's to skip", bad)
		require.True(t, b.mayTouch(spatial.Point{X: bad, Y: 0}, far, far),
			"so is a non-finite origin (%v)", bad)
	}

	require.True(t, footprintBounds{}.mayTouch(origin, far, far),
		"the zero value traces everything")
	require.False(t, boundsOf(spatial.FootprintPlacement{}).ok,
		"a placement with no box has no reach, so it is always traced")
	require.False(t, boundsOf(boxPlacement(1, 1, math.NaN(), origin, spatial.Point{})).ok,
		"nor does one whose facing cannot be measured")
}

// TestMayTouchAgreesWithSpatial is the predicate against spatial itself:
// every random segment the trace says makes contact must be one the filter
// keeps. Segments are drawn near the box so contact is common, and a share of
// them end exactly on a corner or run along an edge, the cases rounding would
// break first.
func (fs *FootprintBoundsSuite) TestMayTouchAgreesWithSpatial() {
	t := fs.T()
	rng := rand.New(rand.NewSource(2049))
	kept, rejected, contacts := 0, 0, 0
	for i := 0; i < 20000; i++ {
		origin := spatial.Point{X: (rng.Float64() - 0.5) * 2000, Y: (rng.Float64() - 0.5) * 2000}
		pl := boxPlacement(0.05+rng.Float64()*20, 0.05+rng.Float64()*40, rng.Float64()*720-360, origin,
			spatial.Point{X: (rng.Float64() - 0.5) * 10, Y: (rng.Float64() - 0.5) * 10})
		b := boundsOf(pl)
		require.True(t, b.ok)

		corners := placementCorners(pl)
		near := func() spatial.Point {
			c := corners[rng.Intn(4)]
			switch rng.Intn(3) {
			case 0:
				return c
			case 1:
				return spatial.Point{X: c.X + (rng.Float64()-0.5)*1e-6, Y: c.Y + (rng.Float64()-0.5)*1e-6}
			default:
				return spatial.Point{X: c.X + (rng.Float64()-0.5)*60, Y: c.Y + (rng.Float64()-0.5)*60}
			}
		}
		from, to := near(), near()
		if rng.Intn(5) == 0 {
			to = from
		}
		if rng.Intn(5) == 0 {
			// Along an edge: two corners sharing a side.
			k := rng.Intn(4)
			from, to = corners[k], corners[(k+1)%4]
		}

		trace, err := spatial.TraceFootprint(spatial.FootprintTraceInput{Placement: pl, From: from, To: to})
		require.NoError(t, err)
		touch := b.mayTouch(pl.Origin, from, to)
		if trace.Contact {
			contacts++
			require.True(t, touch, "trace %d: spatial reports contact the filter would have skipped "+
				"(placement %+v box %+v, %v -> %v)", i, pl, *pl.Footprint.Box, from, to)
		}
		if touch {
			kept++
		} else {
			rejected++
		}
	}
	require.Positive(t, contacts, "the draw must exercise contact")
	require.Positive(t, rejected, "and must exercise rejection, or the agreement proves nothing")
	require.Positive(t, kept)
}

// placementCorners is the rectangle's four corners, measured the way
// [placedCentre] mirrors spatial's transform.
func placementCorners(p spatial.FootprintPlacement) [4]spatial.Point {
	c := placedCentre(p)
	a := math.Mod(p.Facing, 360) * math.Pi / 180
	along := spatial.Point{X: math.Cos(a), Y: math.Sin(a)}
	across := spatial.Point{X: -math.Sin(a), Y: math.Cos(a)}
	hd, hw := p.Footprint.Box.D/2, p.Footprint.Box.W/2
	var out [4]spatial.Point
	for i, s := range [4][2]float64{{-1, -1}, {1, -1}, {1, 1}, {-1, 1}} {
		out[i] = spatial.Point{
			X: c.X + along.X*s[0]*hd + across.X*s[1]*hw,
			Y: c.Y + along.Y*s[0]*hd + across.Y*s[1]*hw,
		}
	}
	return out
}

// prefilterScene is a hall with walls that exercise every way a rectangle
// meets a lane — wall ends, a door threshold, a rotated wall, a box whose
// corner sits exactly on a cell centre and one whose edge runs exactly
// through a row of centres — plus many footprints far off the floor that the
// pre-check exists to skip.
type prefilterScene struct {
	enc    *Encounter
	viewer MemberID
	far    spatial.Position
}

func newPrefilterScene(t testing.TB, farAway int) prefilterScene {
	t.Helper()
	regions := []RegionInput{rectRegion("hall", 0, 0, 20, 14)}
	probe := prefilterEncounter(t, FieldInput{Canvas: openAir(), Regions: regions}, spatial.Position{})
	cells := probe.field.cells
	centre := func(i int) spatial.Point { return probe.field.plane.CellCentre(cells[i]) }

	first, last := centre(0), centre(len(cells)-1)
	midY := (first.Y + last.Y) / 2
	minX, maxX := first.X, last.X
	for i := range cells {
		minX, maxX = math.Min(minX, centre(i).X), math.Max(maxX, centre(i).X)
	}
	span := maxX - minX

	// A horizontal wall across the hall between two rows of centres, with a
	// gap a shut door fills, and an open end on the right that a route must
	// go round: wall ends at the gap's edges and at the opening.
	gapAt := minX + span/2
	left := (gapAt - 3) - (minX - 10)
	right := 30.0
	placed := []PlacedPropInput{
		{ID: "wall/left", BlocksMovement: true, BlocksLineOfSight: true,
			Placement: boxPlacement(1, left, 0, spatial.Point{X: minX - 10 + left/2, Y: midY}, spatial.Point{})},
		{ID: "wall/right", BlocksMovement: true, BlocksLineOfSight: true,
			Placement: boxPlacement(1, right, 0, spatial.Point{X: gapAt + 3 + right/2, Y: midY}, spatial.Point{})},
		{ID: "wall/slant", BlocksMovement: true, BlocksLineOfSight: true,
			Placement: boxPlacement(1, 25, 33, spatial.Point{X: minX + span/4, Y: midY - 15}, spatial.Point{X: 2, Y: 1})},
		{ID: "screen/glass", BlocksMovement: true,
			Placement: boxPlacement(0.5, 12, 120, spatial.Point{X: minX + 3*span/4, Y: midY + 18}, spatial.Point{})},
		{ID: "curtain", BlocksLineOfSight: true,
			Placement: boxPlacement(0.5, 12, 60, spatial.Point{X: minX + 3*span/4, Y: midY - 18}, spatial.Point{})},
	}

	// An 8x5 box whose lower-left corner is exactly one cell centre and
	// whose own centre is nearest a different one, so the corner cell is
	// covered only by boundary contact; and a 3-wide bench whose lower edge
	// runs exactly through two neighbouring centres of one row, so the lane
	// between them grazes it (contact, no interior).
	atCell := func(col, row int) spatial.Point {
		return probe.field.plane.CellCentre(probe.field.cellAt(spatial.Position{X: float64(col), Y: float64(row)}))
	}
	corner := atCell(1, 0)
	placed = append(placed, PlacedPropInput{ID: "crate/corner", BlocksMovement: true, BlocksLineOfSight: true,
		Placement: boxPlacement(5, 8, 0, spatial.Point{X: corner.X + 4, Y: corner.Y + 2.5}, spatial.Point{})})
	a, b := atCell(14, 11), atCell(15, 11)
	placed = append(placed, PlacedPropInput{ID: "bench/graze", BlocksMovement: true, BlocksLineOfSight: true,
		Placement: boxPlacement(3, b.X-a.X, 0, spatial.Point{X: (a.X + b.X) / 2, Y: a.Y + 1.5}, spatial.Point{})})

	for i := 0; i < farAway; i++ {
		placed = append(placed, PlacedPropInput{
			ID: PropID(fmt.Sprintf("far/%03d", i)), BlocksMovement: true, BlocksLineOfSight: true,
			Placement: boxPlacement(1, 30, float64(i*7%180),
				spatial.Point{X: maxX + 500 + float64(i%20)*40, Y: midY + float64(i/20)*40}, spatial.Point{}),
		})
	}

	doors := []DoorInput{
		{ID: "door/gate", State: DoorIsClosed(),
			Placement: &spatial.FootprintPlacement{
				Footprint: spatial.Footprint{Box: &spatial.Box{W: 1, D: 6}},
				Origin:    spatial.Point{X: gapAt, Y: midY},
			}},
		{ID: "door/far", State: DoorIsClosed(),
			Placement: &spatial.FootprintPlacement{
				Footprint: spatial.Footprint{Box: &spatial.Box{W: 1, D: 6}},
				Origin:    spatial.Point{X: maxX + 400, Y: midY},
			}},
	}

	// The viewer stands below the wall near the gap; the route's goal is
	// above it on the left, so the shut gate forces the route round the open
	// right end.
	viewer := probe.field.cellAt(spatial.Position{X: 9, Y: 4})
	far := probe.field.cellAt(spatial.Position{X: 2, Y: 11})
	enc := prefilterEncounter(t, FieldInput{Canvas: openAir(), Regions: regions, Placed: placed, Doors: doors}, viewer)

	return prefilterScene{enc: enc, viewer: "viewer", far: far}
}

func prefilterEncounter(t testing.TB, field FieldInput, at spatial.Position) *Encounter {
	t.Helper()
	enc, err := NewEncounter(&SetupInput{
		Field:   field,
		Members: []MemberInput{{ID: "viewer", Kind: KindPlayer, Position: at}},
		Endings: []EndingInput{{Key: "withdrawn", Trigger: TriggerExternal{}}},
		Capabilities: Capabilities{
			Sight:         benchSight{cells: 60},
			Equipment:     UnobservedEquipment{},
			Conditions:    UnobservedEquipment{},
			Sheets:        zeroSheets{},
			Participation: everyoneStanding{},
			Initiative:    orderAsGiven{},
			Driver:        passDriver{},
			Actors:        Actors{Striker: passStriker{}, Mover: quietMover{}, Announcer: quietAnnouncer{}},
		},
	})
	require.NoError(t, err)

	return enc
}

// withoutPrefilter switches the pre-check off for every contributor and
// door: the zero reach traces everything, which is the behaviour before
// rpg-toolkit#2049.
func withoutPrefilter(f *field, run func()) {
	saved := make([]footprintBounds, len(f.placed))
	for i := range f.placed {
		saved[i], f.placed[i].reach = f.placed[i].reach, footprintBounds{}
	}
	savedDoors := make([]footprintBounds, len(f.doorFootprints))
	for i, d := range f.doorFootprints {
		savedDoors[i], d.reach = d.reach, footprintBounds{}
	}
	defer func() {
		for i := range f.placed {
			f.placed[i].reach = saved[i]
		}
		for i, d := range f.doorFootprints {
			d.reach = savedDoors[i]
		}
	}()
	run()
}

// sceneAnswers is every question the footprint readers answer, over the
// whole floor: standing, screening and cell facts per cell; crossing and lane
// facts for every neighbour step and for every lane from the viewer; the
// viewer's visible set; and a route through the door gap.
type sceneAnswers struct {
	standing  map[spatial.Position]PropID
	origins   map[spatial.Position]bool
	screens   map[spatial.Position]string
	facts     map[spatial.Position]CellFact
	crossings map[[2]spatial.Position]PropID
	lanes     map[[2]spatial.Position]bool
	visible   []spatial.Position
	route     []spatial.Position
	routed    bool
}

func askScene(t *testing.T, s prefilterScene) sceneAnswers {
	t.Helper()
	e, f := s.enc, s.enc.field
	q, err := e.cellQuery()
	require.NoError(t, err)
	out := sceneAnswers{
		standing:  map[spatial.Position]PropID{},
		origins:   map[spatial.Position]bool{},
		screens:   map[spatial.Position]string{},
		facts:     map[spatial.Position]CellFact{},
		crossings: map[[2]spatial.Position]PropID{},
		lanes:     map[[2]spatial.Position]bool{},
	}
	from, placed := e.canvas.GetEntityPosition(string(s.viewer))
	require.True(t, placed)

	for _, cell := range f.cells {
		id, _ := f.standingBlocks(cell)
		out.standing[cell] = id
		covered, err := f.sightBlocksOriginAt(cell)
		require.NoError(t, err)
		out.origins[cell] = covered
		props, doors, err := f.screensOf(cell)
		require.NoError(t, err)
		out.screens[cell] = fmt.Sprint(props, doors)
		fact, err := q.cellAt(CellAtInput{Cell: cell, Mover: s.viewer})
		require.NoError(t, err)
		out.facts[cell] = fact

		for _, n := range adjacencyGrid.GetNeighbors(cell) {
			id, _, err := f.crossingBlocks(cell, n)
			require.NoError(t, err)
			out.crossings[[2]spatial.Position{cell, n}] = id
		}
		blocked, err := f.sightBlocksAlong(from, cell)
		require.NoError(t, err)
		out.lanes[[2]spatial.Position{from, cell}] = blocked
	}

	visible, err := e.VisibleCellsFor(&VisibleCellsInput{Member: s.viewer})
	require.NoError(t, err)
	out.visible = visible.Cells
	out.route, out.routed, err = q.routeTo(s.viewer, from, func(c spatial.Position) bool { return c == s.far })
	require.NoError(t, err)

	return out
}

// TestThePrefilterChangesNoAnswer asks the scene every footprint question
// with the pre-check on and with it off, and requires the same answers.
func (fs *FootprintBoundsSuite) TestThePrefilterChangesNoAnswer() {
	t := fs.T()
	s := newPrefilterScene(t, 120)
	_, _, err := s.enc.refreshSight([]MemberID{s.viewer})
	require.NoError(t, err)

	with := askScene(t, s)
	var without sceneAnswers
	withoutPrefilter(s.enc.field, func() { without = askScene(t, s) })

	require.Equal(t, without, with)

	// The scene must actually exercise what it claims, or equality is cheap.
	own, _ := s.enc.canvas.GetEntityPosition(string(s.viewer))
	require.NotEqual(t, []spatial.Position{own}, with.visible, "the viewer sees past its own cell")
	require.True(t, with.routed, "the route through the door gap exists")
	require.NotEmpty(t, with.route)
	blockedCrossing, blockedLane, coveredCell := false, false, false
	for _, id := range with.crossings {
		blockedCrossing = blockedCrossing || id != ""
	}
	for _, b := range with.lanes {
		blockedLane = blockedLane || b
	}
	for _, id := range with.standing {
		coveredCell = coveredCell || id != ""
	}
	require.True(t, blockedCrossing, "some neighbour step is closed by a footprint")
	require.True(t, blockedLane, "some lane from the viewer is closed by a footprint")
	require.True(t, coveredCell, "some cell centre is covered by a footprint")
}

// TestThePrefilteredCellSupportsMatchTracingEveryCell holds the two
// whole-floor sweeps — where a footprint stands and which cells it overlaps —
// to the same answers a sweep of every cell gives.
func (fs *FootprintBoundsSuite) TestThePrefilteredCellSupportsMatchTracingEveryCell() {
	t := fs.T()
	s := newPrefilterScene(t, 40)
	f := s.enc.field
	placements := make([]spatial.FootprintPlacement, 0, len(f.placed)+len(f.doorFootprints))
	for i := range f.placed {
		placements = append(placements, f.placed[i].placement)
	}
	for _, d := range f.doorFootprints {
		placements = append(placements, *d.placement)
	}

	for _, p := range placements {
		require.Equal(t, placedCellsTracingEveryCell(f, p), f.placedCells(p))

		got, err := f.footprintObservationCells(p)
		require.NoError(t, err)
		coverage, err := spatial.PlacedCoverage(spatial.PlacedCoverageInput{
			Embedding: f.plane, Placement: p, Cells: f.cells,
		})
		require.NoError(t, err)
		var want []spatial.Position
		for _, cell := range f.cells {
			if coverage.Cells[cell] > 0 {
				want = append(want, cell)
			}
		}
		if len(want) == 0 {
			want = f.placedCells(p)
		}
		require.Equal(t, want, got)
	}
}

// placedCellsTracingEveryCell is [field.placedCells] as it was before the
// pre-check: every cell traced.
func placedCellsTracingEveryCell(f *field, p spatial.FootprintPlacement) []spatial.Position {
	centre := placedCentre(p)
	stands := map[spatial.Position]bool{}
	var own spatial.Position
	var ownD2 float64
	found := false
	for _, cell := range f.cells {
		at := f.plane.CellCentre(cell)
		contact, err := spatial.TraceFootprint(spatial.FootprintTraceInput{Placement: p, From: at, To: at})
		if err != nil || contact.Contact {
			stands[cell] = true
		}
		dx, dy := at.X-centre.X, at.Y-centre.Y
		if d2 := dx*dx + dy*dy; !found || d2 < ownD2 || (d2 == ownD2 && cellBefore(cell, own)) {
			found, own, ownD2 = true, cell, d2
		}
	}
	stands[own] = true
	out := []spatial.Position{}
	for _, cell := range f.cells {
		if stands[cell] {
			out = append(out, cell)
		}
	}
	return out
}

// BenchmarkFootprintCrossings is the cost rpg-toolkit#2049 names: every
// neighbour crossing and every lane from one cell on a field of many walls,
// with every footprint traced and with the pre-check.
//
//	go test -run XXX -bench BenchmarkFootprintCrossings .
func BenchmarkFootprintCrossings(b *testing.B) {
	s := newPrefilterScene(b, 200)
	f := s.enc.field
	from, _ := s.enc.canvas.GetEntityPosition(string(s.viewer))
	sweep := func() {
		for _, cell := range f.cells {
			for _, n := range adjacencyGrid.GetNeighbors(cell) {
				if _, _, err := f.crossingBlocks(cell, n); err != nil {
					b.Fatal(err)
				}
			}
			if _, err := f.sightBlocksAlong(from, cell); err != nil {
				b.Fatal(err)
			}
			if _, err := f.sightBlocksOriginAt(cell); err != nil {
				b.Fatal(err)
			}
		}
	}
	b.Run("trace-every-footprint", func(b *testing.B) {
		withoutPrefilter(f, func() {
			for i := 0; i < b.N; i++ {
				sweep()
			}
		})
	})
	b.Run("bounds-pre-check", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			sweep()
		}
	})
}
