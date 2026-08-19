// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package encounter_test

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"

	"github.com/KirkDiggler/rpg-toolkit/core"
	"github.com/KirkDiggler/rpg-toolkit/play/intel"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter"
	"github.com/KirkDiggler/rpg-toolkit/tools/spatial"
)

// canvas_test.go is THE FIELD AS ONE MAP (rpg-toolkit#1106).
//
// The composition authors rooms and always did. What changed is what they
// compile into: one spatial room spanning the whole dungeon, with every wall
// registered as an absolute boundary edge. That single fact is what every test
// in this file is holding, from three directions:
//
//   - A WALL BETWEEN TWO ROOMS IS EXPRESSIBLE. It was not. A room's grid
//     refuses a boundary whose far endpoint is not a cell of that room
//     (tools/spatial/room.go's validateAndNormalizeBoundaryUnsafe), so the
//     seam between two authored chambers was the one place a wall could never
//     be drawn — and the room-membership filter in rebuildPercepts was
//     standing in for every one of them.
//   - SO SIGHT IS DECIDED BY GEOMETRY. Two members a single cell apart through
//     an open doorway see each other; two members a single cell apart through
//     the wall beside it do not. The same fixture answers both, which is the
//     point: no room label can tell those two pairs apart, and geometry tells
//     them apart without being asked.
//   - AND A DOORWAY IS NOT A MECHANISM. Crossing one is a step to the next
//     cell, narrated as a move, refused by a wall the way any other step into
//     a wall is refused.
//
// The fixture is the reference tomb's own shape — three chambers in a chain,
// 6, 10 and 12 wide by 8 tall, joined by two doorways — because that is the
// dungeon this stack has to carry (rpg-project#227), and because its seams are
// where the old model was blind.
type CanvasSuite struct {
	suite.Suite

	enc *encounter.Encounter
}

func TestCanvasSuite(t *testing.T) {
	suite.Run(t, new(CanvasSuite))
}

// carol and dave are this file's own, joining alice and bob from
// encounter_test.go: the fixture needs FOUR members to hold two pairs.
const (
	carol = core.EntityID("carol")
	dave  = core.EntityID("dave")

	tombEntrance = "entrance"
	tombHall     = "hall"
	tombChamber  = "tomb"

	entranceDoor = "entrance-door"
	tombDoor     = "tomb-door"
)

// The chain, anchored so that NOTHING here sits at the origin: a room at (0,0)
// has local coordinates identical to its absolute ones, and a verb answering
// in the wrong frame would pass every assertion anyway (step_test.go's rule).
var (
	tombEntranceOrigin = spatial.Position{X: 3, Y: 4}  // cells x 3..8
	tombHallOrigin     = spatial.Position{X: 9, Y: 4}  // cells x 9..18
	tombChamberOrigin  = spatial.Position{X: 19, Y: 4} // cells x 19..30

	// Both doorways sit on the same row, which is the WORST case for sight and
	// therefore the one worth pinning: it is the only arrangement that gives an
	// unobstructed run from one end of the dungeon to the other.
	tombDoorRow = 3
)

// seamWall returns the boundary edges a room declares along one of its own
// vertical edges: a wall from every cell of column atLocalX to the cell beyond
// it, with a gap at the doorway row.
//
// The far endpoint is OUTSIDE the declaring room, and that is the whole
// expressiveness this slice buys. A room's boundaries are room-local at
// authoring and absolute once compiled onto the canvas, so a room can finally
// draw the wall along its own edge rather than leaving the seam open and
// hoping a room label stands in for it.
func tombSeamWall(atLocalX, height, gapRow int) []spatial.Boundary {
	return squareSeamWall(atLocalX, height, gapRow)
}

func tombField() encounter.FieldInput {
	return encounter.FieldInput{
		Rooms: []encounter.RoomInput{
			// The entrance walls itself off from the hall, all the way up its
			// east edge except the doorway.
			{ID: tombEntrance, Width: 6, Height: 8, Origin: tombEntranceOrigin,
				Boundaries: tombSeamWall(5, 8, tombDoorRow)},
			// The hall walls itself off from the tomb chamber the same way, and
			// carries the pillars the reference tomb puts there.
			{ID: tombHall, Width: 10, Height: 8, Origin: tombHallOrigin,
				Boundaries: tombSeamWall(9, 8, tombDoorRow),
				Occluders: []spatial.Position{
					{X: 2, Y: 1}, {X: 2, Y: 6}, {X: 6, Y: 1}, {X: 6, Y: 6},
				}},
			{ID: tombChamber, Width: 12, Height: 8, Origin: tombChamberOrigin},
		},
		Connections: []encounter.ConnectionInput{
			{ID: entranceDoor, From: tombEntrance, To: tombHall,
				FromPosition: spatial.Position{X: 5, Y: float64(tombDoorRow)},
				ToPosition:   spatial.Position{X: 0, Y: float64(tombDoorRow)}},
			{ID: tombDoor, From: tombHall, To: tombChamber,
				FromPosition: spatial.Position{X: 9, Y: float64(tombDoorRow)},
				ToPosition:   spatial.Position{X: 0, Y: float64(tombDoorRow)}},
		},
	}
}

// at projects a room-local cell through the fixture's own anchor, so a changed
// anchor cannot leave a stale absolute literal passing.
func tombAt(origin spatial.Position, x, y int) spatial.Position {
	return spatial.Position{X: float64(x), Y: float64(y)}.Add(origin)
}

// SetupTest opens the tomb with four members, all players so that nothing
// forms a fight and each test's verb is the only thing that happens.
//
//	alice  entrance, ON the doorway cell
//	bob    hall,     ON the far side of that same doorway — one cell from alice
//	carol  entrance, one row up from alice, against the wall
//	dave   hall,     one row up from bob, against the same wall
//
// alice|bob and carol|dave are the SAME geometric relationship — adjacent
// cells in different rooms — separated only by whether a wall stands between
// them. Nothing about rooms can tell them apart.
func (s *CanvasSuite) SetupTest() {
	enc, err := encounter.NewEncounter(&encounter.SetupInput{
		Sight: everyoneSeesTheWholeMap{}, Standing: everyoneStanding{}, Initiative: orderAsGiven{},
		Field: tombField(),
		Members: []encounter.MemberInput{
			{ID: alice, Kind: encounter.KindPlayer, Room: tombEntrance,
				Position: spatial.Position{X: 5, Y: float64(tombDoorRow)}},
			{ID: bob, Kind: encounter.KindPlayer, Room: tombHall,
				Position: spatial.Position{X: 0, Y: float64(tombDoorRow)}},
			{ID: carol, Kind: encounter.KindPlayer, Room: tombEntrance,
				Position: spatial.Position{X: 5, Y: float64(tombDoorRow - 1)}},
			{ID: dave, Kind: encounter.KindPlayer, Room: tombHall,
				Position: spatial.Position{X: 0, Y: float64(tombDoorRow - 1)}},
		},
		Endings: []encounter.EndingInput{{Key: "withdrawn", Trigger: encounter.TriggerExternal{}}},
	})
	s.Require().NoError(err)
	s.enc = enc
}

// holds reports whether an observer holds anything at all on a subject.
func (s *CanvasSuite) holds(observer, subject core.EntityID) bool {
	view, err := s.enc.View(&encounter.ViewInput{Member: observer})
	s.Require().NoError(err)
	for _, h := range view {
		if h.Subject == intel.Subject(subject) {
			return true
		}
	}
	return false
}

// TestThroughTheDoorwayTheySeeEachOtherThroughTheWallTheyDoNot is the slice in
// one scene.
//
// Two pairs, one cell apart each, in different rooms each. The only difference
// between them is a wall — and until this slice that wall was inexpressible, so
// the composition answered both pairs the same way: blind. It answered blind
// even for the pair standing in an open doorway close enough to touch.
func (s *CanvasSuite) TestThroughTheDoorwayTheySeeEachOtherThroughTheWallTheyDoNot() {
	s.True(s.holds(alice, bob), "alice is in the doorway; bob is the next cell through it")
	s.True(s.holds(bob, alice), "and sight is symmetric — spatial makes that structural")

	s.False(s.holds(carol, dave), "carol and dave are just as close, with a wall between them")
	s.False(s.holds(dave, carol), "from either side")
}

// TestTheWallIsAnAbsoluteEdgeNotARoomsBusiness pins WHY the pair above differ:
// the seam wall the entrance declared is registered in dungeon-absolute space,
// so it stands between two cells belonging to two different rooms. A room-local
// boundary could never have been registered at all — the far endpoint is not a
// cell of the declaring room.
func (s *CanvasSuite) TestTheWallIsAnAbsoluteEdgeNotARoomsBusiness() {
	atlas, err := s.enc.Atlas()
	s.Require().NoError(err)

	want := struct{ from, to spatial.Position }{
		from: tombAt(tombEntranceOrigin, 5, tombDoorRow-1),
		to:   tombAt(tombEntranceOrigin, 6, tombDoorRow-1), // = hall local (0, tombDoorRow-1)
	}
	s.Equal(tombAt(tombHallOrigin, 0, tombDoorRow-1), want.to, "the far endpoint is the HALL's cell")

	var found bool
	for _, region := range atlas.Regions {
		for _, b := range region.Boundaries {
			if b.From == want.from && b.To == want.to {
				found = true
				s.True(b.BlocksLineOfSight)
				s.True(b.BlocksMovement)
			}
		}
	}
	s.True(found, "the seam wall is on the map, in absolute cells, spanning two rooms")
}

// TestCrossingADoorwayIsAnOrdinaryStep. The walk does not know it crossed
// anything: one cell, one "moved" beat. The doorway's name comes back because a
// host narrating the scene should not have to re-derive it, but it is a NAME
// now and not a mechanism — nothing about the step depended on it.
func (s *CanvasSuite) TestCrossingADoorwayIsAnOrdinaryStep() {
	out, err := s.enc.Step(&encounter.StepInput{
		Member: alice, To: tombAt(tombHallOrigin, 0, tombDoorRow),
	})
	s.Require().NoError(err)

	s.Equal(tombAt(tombEntranceOrigin, 5, tombDoorRow), out.Stepped.From)
	s.Equal(tombAt(tombHallOrigin, 0, tombDoorRow), out.Stepped.To)
	s.Equal(entranceDoor, out.Crossing, "the doorway is named")

	s.Equal("moved", s.beatKind(out.Seq), "and the story says she moved, because she did")
}

// beatKind reads the "beat" key off one story entry.
func (s *CanvasSuite) beatKind(seq uint64) string {
	entries, err := s.enc.Story(&encounter.StoryInput{Audience: alice, AfterSeq: seq})
	s.Require().NoError(err)
	s.Require().NotEmpty(entries)
	var payload struct {
		Beat string `json:"beat"`
	}
	s.Require().NoError(json.Unmarshal(entries[0].Payload, &payload))
	return payload.Beat
}

// TestAStepIntoAWallIsRefusedAsAPlacement. The refusal survives the slice —
// what changes is what does the refusing. It used to be the absence of a
// doorway joining two rooms; it is now the wall itself, and the wall is a thing
// on the map rather than a gap in a connection list.
func (s *CanvasSuite) TestAStepIntoAWallIsRefusedAsAPlacement() {
	_, err := s.enc.Step(&encounter.StepInput{
		Member: carol, To: tombAt(tombHallOrigin, 0, tombDoorRow-1),
	})
	s.Require().Error(err)
	s.ErrorIs(err, encounter.ErrBadPlacement, "a wall refuses a step the way any wall does")
}

// TestVoidIsStillNotFloor. The canvas spans the union's bounding box, so a
// field whose rooms do not tile it has cells inside the canvas that belong to
// no chamber. Stepping onto one is refused — the composition asks which
// authored room owns a cell before it moves anybody there, and that question is
// the floor.
func (s *CanvasSuite) TestVoidIsStillNotFloor() {
	// One cell north of the entrance's own top row: inside the canvas's span
	// (the chambers are 8 tall from y=4), owned by nothing.
	_, err := s.enc.Step(&encounter.StepInput{
		Member: alice, To: spatial.Position{X: 5, Y: 0},
	})
	s.Require().Error(err)
	s.ErrorIs(err, encounter.ErrBadPlacement)
}

// TestAnOldDialectBlobIsRefusedByName is the persistence half of the slice.
//
// A member's placement went from a room and a room-local cell to one absolute
// cell, and the two are indistinguishable by inspection: a blob written before
// the flip carries numbers that are legal in the new frame and mean somewhere
// else. So the key MOVED — "room"+"position" became "cell" — and the old keys
// land nowhere: the field arrives absent, and its absence is the signal.
//
// Refused BY NAME, citing the issue, never defaulted to (0,0), which is a legal
// cell that would invent a placement. That is the same call
// MemberOutcomeData.Cell made for #1068, and Kirk's fail-loudly ruling.
func TestAnOldDialectBlobIsRefusedByName(t *testing.T) {
	// A pre-#1106 blob, written by hand in the dialect this module used to
	// produce: the member carries a room and a room-local position.
	const oldDialect = `{
		"clock": {"budgets": {"p1": 0}},
		"intel": {},
		"log": {"next_seq": 2, "entries": [
			{"seq": 1, "audience": ["p1"], "tags": {"tag": "scene"},
			 "payload": "eyJiZWF0Ijoic2NlbmUtb3BlbmVkIn0="}
		]},
		"field": {"rooms": [{"id": "room1", "width": 5, "height": 5, "origin": {"x": 0, "y": 0}}]},
		"members": [{"id": "p1", "kind": "player", "room": "room1", "position": {"x": 2, "y": 2}}],
		"endings": [{"key": "done", "kind": "external"}],
		"ever_members": ["p1"],
		"retention": 32
	}`

	var data encounter.EncounterData
	require.NoError(t, json.Unmarshal([]byte(oldDialect), &data),
		"the old blob still PARSES — that is exactly why the refusal has to be by name")

	_, err := encounter.LoadEncounter(&encounter.LoadEncounterInput{
		Sight: everyoneSeesTheWholeMap{}, Standing: everyoneStanding{}, Initiative: orderAsGiven{}, Data: data,
	})
	require.Error(t, err, "a room-local placement must not load as an absolute one")
	require.ErrorIs(t, err, encounter.ErrInvalidData)
	require.ErrorIs(t, err, encounter.ErrBadPlacement)
	require.Contains(t, err.Error(), "rpg-toolkit#1106",
		"the refusal names the change, so whoever reads it knows what to recreate")
	require.Contains(t, err.Error(), "p1", "and names the member it could not place")
}

// TestASquareFieldMustFitOneGrid is W6, the one new construction law this
// slice adds.
//
// The authored rooms compile into ONE grid, and a square grid is the half-open
// rectangle [0,Width) x [0,Height) — it starts at the origin and cannot be
// moved. So a field whose absolute footprint reaches a negative cell has no
// grid to be drawn on, and construction says so rather than silently losing
// the part that does not fit.
//
// The remedy is a relabeling, not a redesign, and the message says so: shifting
// every Origin by the same vector moves the whole dungeon into the non-negative
// quadrant and changes nothing about it.
func TestASquareFieldMustFitOneGrid(t *testing.T) {
	setup := func(origin spatial.Position) *encounter.SetupInput {
		return &encounter.SetupInput{
			Sight: everyoneSeesTheWholeMap{}, Standing: everyoneStanding{}, Initiative: orderAsGiven{},
			Field: encounter.FieldInput{
				Rooms: []encounter.RoomInput{{ID: "hall", Width: 5, Height: 5, Origin: origin}},
			},
			Members: []encounter.MemberInput{
				{ID: alice, Kind: encounter.KindPlayer, Room: "hall", Position: spatial.Position{X: 0, Y: 0}},
			},
			Endings: []encounter.EndingInput{{Key: "done", Trigger: encounter.TriggerExternal{}}},
		}
	}

	_, err := encounter.NewEncounter(setup(spatial.Position{X: -3, Y: 0}))
	require.Error(t, err)
	require.ErrorIs(t, err, encounter.ErrNoField)
	require.Contains(t, err.Error(), "(-3,0)", "the message names the cell that does not fit")
	require.Contains(t, err.Error(), "shift every room Origin by the same vector",
		"and names the remedy, which is a relabeling")

	// The same dungeon, relabelled: legal, and identical in every way that
	// matters, because a field's absolute frame only ever means "relative to
	// the other rooms".
	_, err = encounter.NewEncounter(setup(spatial.Position{X: 0, Y: 0}))
	require.NoError(t, err)
}

// TestAHexFieldNeedsNoSuchLaw is W6's other half: an axial hex grid is
// origin-CENTRED, negative coordinates are ordinary there, and widening its
// span always reaches further both ways — so a hex field always fits one grid
// and this check never rejects one.
func TestAHexFieldNeedsNoSuchLaw(t *testing.T) {
	_, err := encounter.NewEncounter(&encounter.SetupInput{
		Sight: everyoneSeesTheWholeMap{}, Standing: everyoneStanding{}, Initiative: orderAsGiven{},
		Field: encounter.FieldInput{
			Rooms: []encounter.RoomInput{
				{ID: "crypt", Width: 6, Height: 6, Grid: spatial.GridShapeHex,
					Origin: spatial.Position{X: -20, Y: -20}},
			},
		},
		Members: []encounter.MemberInput{
			{ID: alice, Kind: encounter.KindPlayer, Room: "crypt", Position: spatial.Position{X: 0, Y: 0}},
		},
		Endings: []encounter.EndingInput{{Key: "done", Trigger: encounter.TriggerExternal{}}},
	})
	require.NoError(t, err, "a hex field anchored deep in the negative quadrant is ordinary")
}

// TestOccludersAreCompiledThroughTheirRoomsAnchor is the occluder half of the
// compile, pinned where it is observable: a sightline.
//
// The Atlas reports occluders absolute too, but it computes that projection
// itself, from the same construction data — so an Atlas assertion cannot tell
// whether the CANVAS got them right. This one can: the blocking column sits in
// a chamber anchored well away from the origin, and the two members are placed
// so that an unprojected occluder would land in a different chamber entirely
// and block nothing.
func TestOccludersAreCompiledThroughTheirRoomsAnchor(t *testing.T) {
	// A pillar wall three cells tall at hall-local x=5, y=2..4 — absolute
	// x=14, y=6..8 through the hall's (9,4) anchor. Unprojected it would sit
	// at (5,2..4), which is entrance floor and nowhere near the pair below.
	field := tombField()
	for i := range field.Rooms {
		if field.Rooms[i].ID == tombHall {
			field.Rooms[i].Occluders = wallColumn(5, 2, 4)
		}
	}

	enc, err := encounter.NewEncounter(&encounter.SetupInput{
		Sight: everyoneSeesTheWholeMap{}, Standing: everyoneStanding{}, Initiative: orderAsGiven{},
		Field: field,
		Members: []encounter.MemberInput{
			{ID: alice, Kind: encounter.KindPlayer, Room: tombHall,
				Position: spatial.Position{X: 3, Y: 3}},
			{ID: bob, Kind: encounter.KindPlayer, Room: tombHall,
				Position: spatial.Position{X: 7, Y: 3}},
		},
		Endings: []encounter.EndingInput{{Key: "done", Trigger: encounter.TriggerExternal{}}},
	})
	require.NoError(t, err)

	view, err := enc.View(&encounter.ViewInput{Member: alice})
	require.NoError(t, err)
	require.Empty(t, view, "the pillars stand between them, at the cells their room's anchor puts them")
}

// TestABoundaryThatCannotBeDrawnIsRefusedAtConstruction.
//
// A wall's endpoints have to be two adjacent cells the canvas holds. When they
// are not, spatial says so and construction stops there — R5, and the reason
// this is worth a test of its own: compileCanvas is the one place a boundary is
// registered for BOTH construction seams, so an error swallowed here would be
// a wall the author declared and the encounter silently does not have.
func TestABoundaryThatCannotBeDrawnIsRefusedAtConstruction(t *testing.T) {
	setup := func(b spatial.Boundary) *encounter.SetupInput {
		return &encounter.SetupInput{
			Sight: everyoneSeesTheWholeMap{}, Standing: everyoneStanding{}, Initiative: orderAsGiven{},
			Field: encounter.FieldInput{
				Rooms: []encounter.RoomInput{
					{ID: "hall", Width: 6, Height: 6, Origin: spatial.Position{X: 4, Y: 4},
						Boundaries: []spatial.Boundary{b}},
				},
			},
			Members: []encounter.MemberInput{
				{ID: alice, Kind: encounter.KindPlayer, Room: "hall",
					Position: spatial.Position{X: 0, Y: 0}},
			},
			Endings: []encounter.EndingInput{{Key: "done", Trigger: encounter.TriggerExternal{}}},
		}
	}

	t.Run("endpoints that are not adjacent", func(t *testing.T) {
		_, err := encounter.NewEncounter(setup(spatial.Boundary{
			From: spatial.Position{X: 0, Y: 0}, To: spatial.Position{X: 3, Y: 0},
			BlocksMovement: true, BlocksLineOfSight: true,
		}))
		require.Error(t, err)
		require.ErrorIs(t, err, encounter.ErrBadPlacement)
		require.Contains(t, err.Error(), "adjacent")
	})

	t.Run("an endpoint off the canvas entirely", func(t *testing.T) {
		// The hall's own (-5,0) projects to (-1,4), off the canvas: the field
		// is anchored at (4,4) and a square canvas starts at (0,0), so there
		// is no such cell to draw a wall to.
		//
		// A wall to a cell that is on the canvas but is NOT floor — the space
		// between chambers — registers fine and is simply inert, since nothing
		// can stand there anyway. That is the canvas spanning a bounding box
		// rather than a footprint, said out loud.
		_, err := encounter.NewEncounter(setup(spatial.Boundary{
			From: spatial.Position{X: 0, Y: 0}, To: spatial.Position{X: -5, Y: 0},
			BlocksMovement: true, BlocksLineOfSight: true,
		}))
		require.Error(t, err)
		require.ErrorIs(t, err, encounter.ErrBadPlacement)
		require.Contains(t, err.Error(), "valid positions")
	})
}
