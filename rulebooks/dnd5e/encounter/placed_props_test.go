// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package encounter_test

import (
	"math"
	"testing"

	"github.com/KirkDiggler/rpg-toolkit/tools/spatial"
	"github.com/stretchr/testify/suite"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter"
)

// placed_props_test.go proves the placed-footprint facts end to end through
// the real exported queries (issue #1753): standing through [Encounter.CellAt]
// and [Encounter.Join], crossings through [Encounter.Step] and
// [Encounter.Route], sight through the canvas the encounter hands out, and
// persistence through [Encounter.ToData]/[encounter.LoadEncounter]. The
// geometry is never re-implemented here: every expected answer comes from
// spatial's published placement/trace/embedding types — the same primitives
// the field compiles — and the source-frame mapping is pinned separately in
// dungeonspec's own suite.
type PlacedPropsSuite struct {
	suite.Suite
}

func TestPlacedPropsSuite(t *testing.T) {
	suite.Run(t, new(PlacedPropsSuite))
}

// placedEmbedding is the canonical plane the fixtures compute expected cell
// centres in: pointy-top hexes five feet across the flats — the same frame
// compileField builds from the field's own orientation.
var placedEmbedding = spatial.NewHexEmbedding(spatial.HexEmbeddingConfig{
	CellWidth:   5,
	Orientation: spatial.HexOrientationPointyTop,
})

// centreOf is the canonical plane point at the middle of an absolute cell.
func centreOf(cell spatial.Position) spatial.Point {
	return placedEmbedding.CellCentre(cell)
}

// coveredBox is a square footprint placement centred on a plane point.
func coveredBox(side float64, at spatial.Point) spatial.FootprintPlacement {
	return spatial.FootprintPlacement{
		Footprint: spatial.Footprint{Box: &spatial.Box{W: side, D: side}},
		Origin:    at,
	}
}

// thinWall is an along-by-across rectangle standing on a plane point at the
// given facing — the shape of a blocker whose interior lies BETWEEN cells.
func thinWall(alongFeet, acrossFeet, facing float64, at spatial.Point) spatial.FootprintPlacement {
	return spatial.FootprintPlacement{
		Footprint: spatial.Footprint{Box: &spatial.Box{W: acrossFeet, D: alongFeet}},
		Origin:    at,
		Facing:    facing,
	}
}

// placed is a named placed contributor with its two flags.
func placed(id string, placement spatial.FootprintPlacement, movement, sight bool) encounter.PlacedPropInput {
	return encounter.PlacedPropInput{
		ID: id, Placement: placement, BlocksMovement: movement, BlocksLineOfSight: sight,
	}
}

// placedField paints the 5x5 fixture hall with the given placed contributors
// on transparent void — the approved single-room declaration.
func placedField(props ...encounter.PlacedPropInput) encounter.FieldInput {
	return encounter.FieldInput{
		Canvas:  openAir(),
		Regions: []encounter.RegionInput{rectRegion("hall", 0, 0, 5, 5)},
		Placed:  props,
	}
}

func (s *PlacedPropsSuite) setup(field encounter.FieldInput, members ...encounter.MemberInput) (*encounter.Encounter, error) {
	return encounter.NewEncounter(&encounter.SetupInput{
		Sight:     everyoneSeesTheWholeMap{},
		Equipment: noHandsAreObserved{}, Standing: everyoneStanding{}, Initiative: orderAsGiven{},
		TurnDriver: passDriver{}, Striker: passStriker{}, Mover: quietMover{}, Announcer: quietAnnouncer{},
		Field:   field,
		Members: members,
		Endings: []encounter.EndingInput{{Key: "withdrawn", Trigger: encounter.TriggerExternal{}}},
	})
}

// reload compiles the field, saves it, and loads the save back — the second
// construction seam, which must rebuild the same facts from the same inputs.
func (s *PlacedPropsSuite) reload(field encounter.FieldInput) *encounter.Encounter {
	enc, err := s.setup(field, encounter.MemberInput{ID: alice, Kind: encounter.KindPlayer, Position: cellAt(0, 0)})
	s.Require().NoError(err)

	return s.loadFrom(enc.ToData())
}

func (s *PlacedPropsSuite) loadFrom(data encounter.EncounterData) *encounter.Encounter {
	reloaded, err := encounter.LoadEncounter(&encounter.LoadEncounterInput{
		Data:      data,
		Sight:     everyoneSeesTheWholeMap{},
		Equipment: noHandsAreObserved{}, Standing: everyoneStanding{}, Initiative: orderAsGiven{},
		TurnDriver: passDriver{}, Striker: passStriker{}, Mover: quietMover{}, Announcer: quietAnnouncer{},
	})
	s.Require().NoError(err)

	return reloaded
}

// --- The approved frame: centre-covered standing, by the stationary contact query ---

func (s *PlacedPropsSuite) TestACentreCoveredCellRefusesStandingAndIsNamed() {
	// A 5x5-foot table exactly over cell (1,1)'s centre: it covers that
	// centre and touches no other (the nearest is five feet away).
	enc, err := s.setup(placedField(placed("table-a", coveredBox(5, centreOf(cellAt(1, 1))), true, false)))
	s.Require().NoError(err)

	_, err = enc.Join(&encounter.JoinInput{Member: "late", Kind: encounter.KindPlayer, Cell: cellAt(1, 1)})
	s.Require().Error(err)
	s.ErrorIs(err, encounter.ErrBadPlacement)
	s.Contains(err.Error(), "table-a", "the refusal names the placement that covers the cell")

	// A step onto the covered cell is the same refusal, through the fold.
	_, err = enc.Join(&encounter.JoinInput{Member: alice, Kind: encounter.KindPlayer, Cell: cellAt(0, 0)})
	s.Require().NoError(err)
	_, err = enc.Step(&encounter.StepInput{Member: alice, To: cellAt(1, 1)})
	s.Require().Error(err)
	s.ErrorIs(err, encounter.ErrBadPlacement)
	s.Contains(err.Error(), "table-a")

	// AND THE FOLD SAYS WHY: CellAt reports the covering contributor.
	fact := enc.CellAt(encounter.CellAtInput{Cell: cellAt(1, 1), Mover: alice})
	s.Equal(encounter.PassageBlocked, fact.Passage)
	s.Require().Len(fact.Contribs, 1)
	s.Equal("table-a", fact.Contribs[0].ID)
	s.Equal(encounter.ContribProp, fact.Contribs[0].Kind)

	// A CLEAR CENTRE STANDS: the neighbouring centre is untouched — asked by
	// its own occupant, since an ally standing there is pass-through to others.
	_, err = enc.Join(&encounter.JoinInput{Member: "late", Kind: encounter.KindPlayer, Cell: cellAt(2, 1)})
	s.Require().NoError(err)
	fact = enc.CellAt(encounter.CellAtInput{Cell: cellAt(2, 1), Mover: "late"})
	s.Equal(encounter.PassageStandable, fact.Passage)
	s.Empty(fact.Contribs, "no footprint fact reached the clear cell")

	// NO ANCHOR, NO FAKE PROP: the placement appears in the atlas's placed
	// list and never as a cell prop, and the floor mask grew by nothing.
	atlas, err := enc.Atlas()
	s.Require().NoError(err)
	s.Require().Len(atlas.Placed, 1)
	s.Equal("table-a", atlas.Placed[0].ID)
	s.True(atlas.Placed[0].BlocksMovement)
	s.False(atlas.Placed[0].BlocksLineOfSight)
	s.Empty(atlas.Props, "a placed footprint is not a legacy cell prop")
	s.Len(atlas.Cells, 25, "a footprint invents no floor")
}

func (s *PlacedPropsSuite) TestSetupRefusesAMemberAuthoredOntoACoveredCentre() {
	field := placedField(placed("table-a", coveredBox(5, centreOf(cellAt(1, 1))), true, false))
	_, err := s.setup(field, encounter.MemberInput{ID: alice, Kind: encounter.KindPlayer, Position: cellAt(1, 1)})
	s.Require().Error(err)
	s.ErrorIs(err, encounter.ErrBadPlacement)
	s.Contains(err.Error(), "table-a")
}

// --- The two flags are independent; all four combinations are real content ---

func (s *PlacedPropsSuite) TestMovementWithoutSightClosesFeetAndOpensSight() {
	// A movement-blocking, sight-transparent table over the middle cell of a
	// three-cell row: nobody stands there, everybody sees across it.
	enc, err := s.setup(placedField(
		placed("table-a", coveredBox(5, centreOf(cellAt(1, 0))), true, false),
	))
	s.Require().NoError(err)

	fact := enc.CellAt(encounter.CellAtInput{Cell: cellAt(1, 0), Mover: ""})
	s.Equal(encounter.PassageBlocked, fact.Passage)

	canvas, err := enc.Canvas()
	s.Require().NoError(err)
	s.False(canvas.IsLineOfSightBlocked(cellAt(0, 0), cellAt(2, 0)),
		"a table the author did not declare a sight blocker obstructs nothing")
	s.False(canvas.IsLineOfSightBlocked(cellAt(2, 0), cellAt(0, 0)),
		"and the answer does not depend on the direction asked")
}

func (s *PlacedPropsSuite) TestSightWithoutMovementBlocksLookingAndAllowsWalking() {
	// A sight-blocking, walk-through footprint over cell (1,0): a join onto
	// it succeeds — standing only asks the movement fact — and sight across
	// it is a SOFT obstruction a lane can lean around.
	enc, err := s.setup(placedField(
		placed("veil-a", coveredBox(2, centreOf(cellAt(2, 0))), false, true),
	))
	s.Require().NoError(err)

	_, err = enc.Join(&encounter.JoinInput{Member: "seer", Kind: encounter.KindPlayer, Cell: cellAt(2, 0)})
	s.Require().NoError(err, "a sight blocker that does not block movement does not refuse standing")

	// The direct lane from (-2,0) crosses the veil's interior and is
	// soft-blocked — but the lane into (1,0), whose centre stops short of
	// the two-foot shape, is clear: the sightline leans, either way round.
	canvas, err := enc.Canvas()
	s.Require().NoError(err)
	s.False(canvas.IsLineOfSightBlocked(cellAt(-2, 0), cellAt(2, 0)),
		"a soft obstruction is leaned around while an uncovered lane remains")
	s.False(canvas.IsLineOfSightBlocked(cellAt(2, 0), cellAt(-2, 0)),
		"symmetrically, from either end")

	// A WALL OF IT blocks every lane: the same thickness, thirty feet across
	// the corridor, standing BETWEEN the two cells.
	walled, err := s.setup(placedField(
		placed("veil-a", thinWall(0.2, 30, 0, centreOf(cellAt(1, 0))), false, true),
	))
	s.Require().NoError(err)
	wallCanvas, err := walled.Canvas()
	s.Require().NoError(err)
	s.True(wallCanvas.IsLineOfSightBlocked(cellAt(-2, 0), cellAt(2, 0)),
		"a footprint blocking every lane blocks sight")
	s.True(wallCanvas.IsLineOfSightBlocked(cellAt(2, 0), cellAt(-2, 0)),
		"and the block is symmetric")
}

// --- A thin segment between two clear centres closes the crossing, not the cells ---

func (s *PlacedPropsSuite) TestAThinCrossingBlocksTheStepAndNotTheCells() {
	// A hand-width blocker on the crossing between (0,0) and (1,0): neither
	// centre covered, the crossing's interior inside the shape.
	enc, err := s.setup(placedField(
		placed("beam-a", thinWall(1, 0.2, 90, spatial.Point{X: 2.5, Y: 0}), true, false),
	))
	s.Require().NoError(err)

	_, err = enc.Join(&encounter.JoinInput{Member: alice, Kind: encounter.KindPlayer, Cell: cellAt(0, 0)})
	s.Require().NoError(err, "both centres are clear; standing is unaffected")
	_, err = enc.Join(&encounter.JoinInput{Member: "east", Kind: encounter.KindPlayer, Cell: cellAt(1, 0)})
	s.Require().NoError(err)

	_, err = enc.Step(&encounter.StepInput{Member: alice, To: cellAt(1, 0)})
	s.Require().Error(err)
	s.ErrorIs(err, encounter.ErrBadPlacement)
	s.Contains(err.Error(), "beam-a", "the crossing refusal names the thin blocker")

	// THE WAY ROUND EXISTS, and the route reads the same fold the step does:
	// the flood closes the blocked crossing and routes through (0,1),(1,1)
	// instead — two hexes around, never through.
	out, err := enc.Route(encounter.RouteInput{
		Mover: alice, Policy: encounter.MoveToward, Anchor: cellAt(2, 0), Budget: 3,
	})
	s.Require().NoError(err)
	s.Require().Len(out.Path, 2)
	s.Equal(cellAt(0, 1), out.Path[0])
	s.Equal(cellAt(1, 1), out.Path[1])
	s.Empty(out.StoppedBy)

	// And the route's answer is walkable: every step it names succeeds, and
	// so does the last one in, whose crossing the beam does not cover.
	for _, cell := range out.Path {
		_, err = enc.Step(&encounter.StepInput{Member: alice, To: cell})
		s.Require().NoError(err)
	}
	_, err = enc.Step(&encounter.StepInput{Member: alice, To: cellAt(2, 0)})
	s.Require().NoError(err)
}

func (s *PlacedPropsSuite) TestARemovedCrossingOpensTheWay() {
	enc, err := s.setup(placedField())
	s.Require().NoError(err)
	_, err = enc.Join(&encounter.JoinInput{Member: alice, Kind: encounter.KindPlayer, Cell: cellAt(0, 0)})
	s.Require().NoError(err)
	_, err = enc.Step(&encounter.StepInput{Member: alice, To: cellAt(1, 0)})
	s.Require().NoError(err, "with no placed contributor the step behaves as every step always has")
}

// --- Overlapping contributors keep their identities; removal keeps the survivor's fact ---

func (s *PlacedPropsSuite) TestOverlappingContributorsDoNotEraseEachOther() {
	centre := centreOf(cellAt(1, 1))
	enc, err := s.setup(placedField(
		placed("table-a", coveredBox(5, centre), true, false),
		placed("table-b", coveredBox(3, centre), true, true),
	))
	s.Require().NoError(err)

	fact := enc.CellAt(encounter.CellAtInput{Cell: cellAt(1, 1), Mover: ""})
	s.Equal(encounter.PassageBlocked, fact.Passage)
	s.Require().Len(fact.Contribs, 2, "both placements cover the centre and both are reported")
	s.ElementsMatch([]string{"table-a", "table-b"}, []string{fact.Contribs[0].ID, fact.Contribs[1].ID})

	// Recompile with one placement gone: the survivor still closes the cell,
	// by both of its remaining facts.
	trimmed := s.reload(placedField(placed("table-b", coveredBox(3, centre), true, true)))
	fact = trimmed.CellAt(encounter.CellAtInput{Cell: cellAt(1, 1), Mover: ""})
	s.Equal(encounter.PassageBlocked, fact.Passage)
	s.Require().Len(fact.Contribs, 1)
	s.Equal("table-b", fact.Contribs[0].ID)

	// And with BOTH gone the cell is an ordinary one again.
	opened := s.reload(placedField())
	fact = opened.CellAt(encounter.CellAtInput{Cell: cellAt(1, 1), Mover: ""})
	s.Equal(encounter.PassageStandable, fact.Passage)
	s.Empty(fact.Contribs)
}

func (s *PlacedPropsSuite) TestAChangedTransformChangesTheNextCompiledField() {
	enc, err := s.setup(placedField(
		placed("table-a", coveredBox(5, centreOf(cellAt(1, 1))), true, false),
	))
	s.Require().NoError(err)
	_, err = enc.Join(&encounter.JoinInput{Member: "late", Kind: encounter.KindPlayer, Cell: cellAt(1, 1)})
	s.Require().Error(err, "covered, before the change")

	// The same placement, recompiled one hex over: the next compiled field
	// answers at the new centre and not at the old one. A changed transform
	// is a recompilation, never a runtime verb.
	moved := s.reload(placedField(
		placed("table-a", coveredBox(5, centreOf(cellAt(2, 2))), true, false),
	))
	_, err = moved.Join(&encounter.JoinInput{Member: "late", Kind: encounter.KindPlayer, Cell: cellAt(2, 2)})
	s.Require().Error(err)
	s.Contains(err.Error(), "table-a")
	_, err = moved.Join(&encounter.JoinInput{Member: "late", Kind: encounter.KindPlayer, Cell: cellAt(1, 1)})
	s.Require().NoError(err, "the old centre is clear again")
}

// --- Caller-owned inputs cannot reach a running field ---

func (s *PlacedPropsSuite) TestCallerMutationAfterConstructionChangesNothing() {
	box := &spatial.Box{W: 5, D: 5}
	in := placedField(encounter.PlacedPropInput{
		ID:             "table-a",
		Placement:      spatial.FootprintPlacement{Footprint: spatial.Footprint{Box: box}, Origin: centreOf(cellAt(1, 1))},
		BlocksMovement: true,
	})
	enc, err := s.setup(in)
	s.Require().NoError(err)

	// The caller flips every mutable fact they still hold.
	box.W, box.D = 500, 500
	in.Placed[0].Placement.Origin = spatial.Point{X: 0, Y: 0}
	in.Placed[0].BlocksMovement = false

	fact := enc.CellAt(encounter.CellAtInput{Cell: cellAt(1, 1), Mover: ""})
	s.Equal(encounter.PassageBlocked, fact.Passage, "the compiled field kept the placement it copied")
	fact = enc.CellAt(encounter.CellAtInput{Cell: cellAt(0, 0), Mover: ""})
	s.Equal(encounter.PassageStandable, fact.Passage, "and the mutated geometry never reached it")

	atlas, err := enc.Atlas()
	s.Require().NoError(err)
	s.Require().Len(atlas.Placed, 1)
	s.Equal(5.0, atlas.Placed[0].Placement.Footprint.Box.W)
	s.True(atlas.Placed[0].BlocksMovement)
}

// --- Invalid placements are refused by name, at construction and at load ---

func (s *PlacedPropsSuite) TestInvalidPlacementsAreRefused() {
	valid := coveredBox(5, centreOf(cellAt(1, 1)))
	cases := []struct {
		name string
		prop encounter.PlacedPropInput
		seed func(*encounter.FieldInput)
	}{
		{"no id", encounter.PlacedPropInput{Placement: valid, BlocksMovement: true}, nil},
		{"no box", encounter.PlacedPropInput{ID: "ghost", BlocksMovement: true}, nil},
		{"zero side", placed("ghost", coveredBox(0, centreOf(cellAt(1, 1))), true, false), nil},
		{"nan origin", placed("ghost", spatial.FootprintPlacement{
			Footprint: spatial.Footprint{Box: &spatial.Box{W: 5, D: 5}},
			Origin:    spatial.Point{X: math.NaN(), Y: 0},
		}, true, false), nil},
		{"vast origin", placed("ghost", spatial.FootprintPlacement{
			Footprint: spatial.Footprint{Box: &spatial.Box{W: 5, D: 5}},
			Origin:    spatial.Point{X: 1 << 40, Y: 0},
		}, true, false), nil},
		{"legacy id collision", placed("table-a", coveredBox(5, centreOf(cellAt(1, 1))), true, false),
			func(f *encounter.FieldInput) {
				f.Props = append(f.Props, encounter.PropInput{
					ID: "table-a", Ref: "dnd5e:props:pillar", At: cellAt(2, 2),
					BlocksMovement: boolPtr(false), BlocksLineOfSight: boolPtr(false),
				})
			}},
	}
	for _, tc := range cases {
		s.Run(tc.name, func() {
			field := placedField(tc.prop)
			if tc.seed != nil {
				tc.seed(&field)
			}
			_, err := s.setup(field)
			s.Require().Error(err)
			s.ErrorIs(err, encounter.ErrNoField, "a placement the field cannot be built from is declaration-time")
		})
	}

	// A DUPLICATE among placed contributors is the same refusal, named.
	_, err := s.setup(placedField(
		placed("twin", coveredBox(5, centreOf(cellAt(1, 1))), true, false),
		placed("twin", coveredBox(5, centreOf(cellAt(2, 2))), true, false),
	))
	s.Require().Error(err)
	s.ErrorIs(err, encounter.ErrNoField)
	s.Contains(err.Error(), "twin")
}

func (s *PlacedPropsSuite) TestAHandEditedBlobPlacementIsRefusedAtLoad() {
	enc, err := s.setup(placedField(placed("table-a", coveredBox(5, centreOf(cellAt(1, 1))), true, false)))
	s.Require().NoError(err)
	data := enc.ToData()
	s.Require().Len(data.Field.Placed, 1)
	data.Field.Placed[0].Placement.Footprint.W = 0

	_, err = encounter.LoadEncounter(&encounter.LoadEncounterInput{
		Data:      data,
		Sight:     everyoneSeesTheWholeMap{},
		Equipment: noHandsAreObserved{}, Standing: everyoneStanding{}, Initiative: orderAsGiven{},
		TurnDriver: passDriver{}, Striker: passStriker{}, Mover: quietMover{}, Announcer: quietAnnouncer{},
	})
	s.Require().Error(err, "a blob whose placement is unmeasurable loads nowhere")
}

// --- Persistence: compiled placements survive save/load with their facts ---

func (s *PlacedPropsSuite) TestSaveLoadRetainsPlacementsAndTheirFacts() {
	enc, err := s.setup(placedField(
		placed("table-a", coveredBox(5, centreOf(cellAt(1, 1))), true, false),
		placed("beam-a", thinWall(1, 0.2, 90, spatial.Point{X: 12.5, Y: 0}), true, false),
	))
	s.Require().NoError(err)

	data := enc.ToData()
	s.Require().Len(data.Field.Placed, 2)
	s.Equal("table-a", data.Field.Placed[0].ID)
	s.True(data.Field.Placed[0].BlocksMovement)
	s.False(data.Field.Placed[0].BlocksLineOfSight)
	s.InDelta(5.0, data.Field.Placed[0].Placement.Footprint.W, 1e-12)

	loaded := s.loadFrom(data)

	// The same facts, from the same queries, on the reloaded field.
	fact := loaded.CellAt(encounter.CellAtInput{Cell: cellAt(1, 1), Mover: ""})
	s.Equal(encounter.PassageBlocked, fact.Passage)
	s.Equal("table-a", fact.Contribs[0].ID)

	_, err = loaded.Join(&encounter.JoinInput{Member: alice, Kind: encounter.KindPlayer, Cell: cellAt(2, 0)})
	s.Require().NoError(err)
	_, err = loaded.Step(&encounter.StepInput{Member: alice, To: cellAt(3, 0)})
	s.Require().Error(err)
	s.ErrorIs(err, encounter.ErrBadPlacement)
	s.Contains(err.Error(), "beam-a", "the reloaded crossing still refuses, by name")

	canvas, err := loaded.Canvas()
	s.Require().NoError(err)
	s.False(canvas.IsLineOfSightBlocked(cellAt(1, 0), cellAt(3, 0)),
		"the reloaded table still obeys its sight-transparent flag")

	atlas, err := loaded.Atlas()
	s.Require().NoError(err)
	s.Len(atlas.Placed, 2)
}

// --- Void and floor-mask truth: footprints outside the painted cells still govern sight ---

func (s *PlacedPropsSuite) TestAFootprintOffTheFloorMaskStillGovernsSight() {
	// Two painted cells with the whole corridor between them UNOWNED: the
	// canvas spans the bounding box, and cell (2,0) in the middle is void.
	gap := func(props ...encounter.PlacedPropInput) encounter.FieldInput {
		return encounter.FieldInput{
			Canvas: openAir(),
			Regions: []encounter.RegionInput{{
				ID: "islands", Name: "islands", Archetype: testArchetype, Lighting: fullLight(),
				Cells: []spatial.Position{{X: 0, Y: 0}, {X: 4, Y: 0}},
			}},
			Placed: props,
		}
	}

	// A wall of veil across the corridor — standing entirely on void —
	// closes sight between the islands.
	walled, err := s.setup(gap(placed("veil-a", thinWall(0.2, 30, 0, centreOf(cellAt(2, 0))), false, true)))
	s.Require().NoError(err)
	canvas, err := walled.Canvas()
	s.Require().NoError(err)
	s.True(canvas.IsLineOfSightBlocked(cellAt(0, 0), cellAt(4, 0)),
		"a sight blocker on void is retained; the footprint invents no floor and needs none")

	// Without it the transparent gap is nothing to see through.
	open, err := s.setup(gap())
	s.Require().NoError(err)
	canvas, err = open.Canvas()
	s.Require().NoError(err)
	s.False(canvas.IsLineOfSightBlocked(cellAt(0, 0), cellAt(4, 0)))

	// And the SAME corridor under an opaque declaration is hard-blocked by
	// the void alone, footprint or none — the declaration still decides.
	tombed := gap()
	tombed.Canvas = pointyCanvas()
	rocked, err := s.setup(tombed)
	s.Require().NoError(err)
	canvas, err = rocked.Canvas()
	s.Require().NoError(err)
	s.True(canvas.IsLineOfSightBlocked(cellAt(0, 0), cellAt(4, 0)), "opaque void keeps its hard answer")
}

// --- Legacy walls, doors and entity facts keep their semantics beside the new ones ---

func (s *PlacedPropsSuite) TestLegacyWallAndDoorStillDecideBesidePlacedFootprints() {
	field := placedField(placed("table-a", coveredBox(5, centreOf(cellAt(3, 3))), true, false))
	field.Canvas = pointyCanvas()
	field.Walls = []encounter.WallInput{wall(0, 0, 1, 0)}
	field.Doors = []encounter.DoorInput{{
		ID:    "gate",
		State: encounter.DoorIsClosed(),
		Edges: []encounter.DoorEdge{{From: cellAt(1, 1), To: cellAt(2, 1)}},
	}}
	enc, err := s.setup(field, encounter.MemberInput{ID: alice, Kind: encounter.KindPlayer, Position: cellAt(0, 0)})
	s.Require().NoError(err)

	// THE WALL: refused by the canvas's own boundary rule, exactly as before.
	_, err = enc.Step(&encounter.StepInput{Member: alice, To: cellAt(1, 0)})
	s.Require().Error(err)
	s.ErrorIs(err, encounter.ErrBadPlacement)
	s.Contains(err.Error(), "boundary")

	// THE DOOR: the near cell of the doorway is an ordinary step; the step
	// THROUGH it refuses as the door, named.
	_, err = enc.Step(&encounter.StepInput{Member: alice, To: cellAt(0, 1)})
	s.Require().NoError(err)
	_, err = enc.Step(&encounter.StepInput{Member: alice, To: cellAt(1, 1)})
	s.Require().NoError(err)
	_, err = enc.Step(&encounter.StepInput{Member: alice, To: cellAt(2, 1)})
	s.Require().Error(err)
	s.ErrorIs(err, encounter.ErrDoorShut)
	s.Contains(err.Error(), "gate")

	// And sight across the wall is hard-blocked on the handed-out canvas.
	canvas, err := enc.Canvas()
	s.Require().NoError(err)
	s.True(canvas.IsLineOfSightBlocked(cellAt(0, 0), cellAt(1, 0)), "a wall is still absolute")

	// THE PLACED FACT COMPOSES: the table elsewhere is exactly what it was.
	fact := enc.CellAt(encounter.CellAtInput{Cell: cellAt(3, 3), Mover: alice})
	s.Equal(encounter.PassageBlocked, fact.Passage)
}

func (s *PlacedPropsSuite) TestLegacyEntityObstructionsStillFlowThroughTheLanes() {
	// A legacy occluding prop stands in the middle of a three-cell row: the
	// direct lane is soft-blocked by the entity and every alternate lane's
	// only middle cell is excluded by the At rule — blocked, spatial's way.
	field := placedField()
	field.Props = append(field.Props, encounter.PropInput{
		ID: "pillar", Ref: "dnd5e:props:pillar", At: cellAt(1, 0),
		BlocksMovement: boolPtr(false), BlocksLineOfSight: boolPtr(true),
	})
	enc, err := s.setup(field)
	s.Require().NoError(err)

	canvas, err := enc.Canvas()
	s.Require().NoError(err)
	s.True(canvas.IsLineOfSightBlocked(cellAt(0, 0), cellAt(2, 0)),
		"the legacy entity facts reach SightLanes through the same adapter")
}

func boolPtr(v bool) *bool { return &v }
