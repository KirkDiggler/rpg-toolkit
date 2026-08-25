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

// tombSeamWall is the wall along the seam east of the chamber anchored at
// origin and `width` wide: every crossing from its last column to the column
// beyond, over its rows, with a gap at the doorway row.
//
// The wall is a FIELD fact now (rpg-project#256): both endpoints are authored
// absolute cells, and nothing about which region "declared" it survives.
func tombSeamWall(origin spatial.Position, width, height, gapRow int) []encounter.WallInput {
	return seamWallRows(int(origin.X)+width-1, int(origin.Y), height, int(origin.Y)+gapRow)
}

func tombField() encounter.FieldInput {
	return encounter.FieldInput{
		Canvas: encounter.CanvasInput{Void: encounter.VoidIsOpaque(), Orientation: encounter.HexesArePointyTop()},
		Regions: []encounter.RegionInput{
			rectRegion(tombEntrance, int(tombEntranceOrigin.X), int(tombEntranceOrigin.Y), 6, 8),
			rectRegion(tombHall, int(tombHallOrigin.X), int(tombHallOrigin.Y), 10, 8),
			rectRegion(tombChamber, int(tombChamberOrigin.X), int(tombChamberOrigin.Y), 12, 8),
		},
		// The entrance is walled off from the hall all the way up the seam
		// except the doorway, and the hall from the tomb chamber the same way.
		Walls: append(
			tombSeamWall(tombEntranceOrigin, 6, 8, tombDoorRow),
			tombSeamWall(tombHallOrigin, 10, 8, tombDoorRow)...),
		// The pillars the reference tomb puts in the hall.
		Props: []encounter.PropInput{
			tombProp(tombHallOrigin, 2, 1), tombProp(tombHallOrigin, 2, 6),
			tombProp(tombHallOrigin, 6, 1), tombProp(tombHallOrigin, 6, 6),
		},
		// And an open door standing in each doorway, so a step through one
		// can name it.
		Doors: []encounter.DoorInput{
			{ID: entranceDoor, State: encounter.DoorIsOpen(), Edges: []encounter.DoorEdge{{
				From: tombAt(tombEntranceOrigin, 5, tombDoorRow), To: tombAt(tombHallOrigin, 0, tombDoorRow)}}},
			{ID: tombDoor, State: encounter.DoorIsOpen(), Edges: []encounter.DoorEdge{{
				From: tombAt(tombHallOrigin, 9, tombDoorRow), To: tombAt(tombChamberOrigin, 0, tombDoorRow)}}},
		},
	}
}

// tombSeat is the AUTHORED absolute cell a chamber-local pair names — the
// anchor plus the pair, in offset columns and rows — which is what a seat or
// an ending target is written in.
func tombSeat(origin spatial.Position, x, y int) spatial.Position {
	return spatial.Position{X: float64(x), Y: float64(y)}.Add(origin)
}

// tombAt is the dungeon-absolute AXIAL cell the same pair names — what every
// verb takes and reports — derived through the one conversion so a changed
// anchor cannot leave a stale absolute literal passing.
func tombAt(origin spatial.Position, x, y int) spatial.Position {
	seat := tombSeat(origin, x, y)
	return cellAt(int(seat.X), int(seat.Y))
}

// tombProp is a chamber-local rubble cell, authored absolute.
func tombProp(origin spatial.Position, x, y int) encounter.PropInput {
	seat := tombSeat(origin, x, y)
	return rubble(int(seat.X), int(seat.Y))
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
		Sight: everyoneSeesTheWholeMap{}, Standing: everyoneStanding{}, Initiative: orderAsGiven{}, TurnDriver: passDriver{}, Striker: passStriker{},
		Field: tombField(),
		Members: []encounter.MemberInput{
			{ID: alice, Kind: encounter.KindPlayer, Position: tombSeat(tombEntranceOrigin, 5, tombDoorRow)},
			{ID: bob, Kind: encounter.KindPlayer, Position: tombSeat(tombHallOrigin, 0, tombDoorRow)},
			{ID: carol, Kind: encounter.KindPlayer, Position: tombSeat(tombEntranceOrigin, 5, tombDoorRow-1)},
			{ID: dave, Kind: encounter.KindPlayer, Position: tombSeat(tombHallOrigin, 0, tombDoorRow-1)},
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
	for _, b := range atlas.Boundaries {
		if (b.From == want.from && b.To == want.to) || (b.From == want.to && b.To == want.from) {
			found = true
			s.True(b.BlocksLineOfSight)
			s.True(b.BlocksMovement)
		}
	}
	s.True(found, "the seam wall is on the map, in absolute cells, spanning two regions")
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
	s.Require().Len(out.Doors, 1, "the door is named")
	s.Equal(encounter.DoorID(entranceDoor), out.Doors[0].ID)

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
		Member: alice, To: cellAt(5, 0),
	})
	s.Require().Error(err)
	s.ErrorIs(err, encounter.ErrBadPlacement)
}

// TestAnOldDialectBlobIsRefusedByName is the persistence half of the slice.
//
// A field went from rooms-plus-origins to regions (rpg-project#256), and a
// room-chain blob still PARSES: its "rooms" key lands on a tombstone field
// and nothing else, so the blob would otherwise arrive as a field with no
// regions. Refused BY NAME, citing the change, so whoever reads it knows what
// to recreate — Kirk's fail-loudly ruling (2026-08-17), the same call
// MemberOutcomeData.Cell made for #1068.
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
		"field": {"canvas": {"void": "opaque"}, "rooms": [{"id": "room1", "width": 5, "height": 5, "origin": {"x": 0, "y": 0}}]},
		"members": [{"id": "p1", "kind": "player", "room": "room1", "position": {"x": 2, "y": 2}}],
		"endings": [{"key": "done", "kind": "external"}],
		"ever_members": ["p1"],
		"retention": 32
	}`

	var data encounter.EncounterData
	require.NoError(t, json.Unmarshal([]byte(oldDialect), &data),
		"the old blob still PARSES — that is exactly why the refusal has to be by name")

	_, err := encounter.LoadEncounter(&encounter.LoadEncounterInput{
		Sight: everyoneSeesTheWholeMap{}, Standing: everyoneStanding{}, Initiative: orderAsGiven{}, TurnDriver: passDriver{}, Striker: passStriker{}, Data: data,
	})
	require.Error(t, err, "a room-chain blob must not load as a region one")
	require.ErrorIs(t, err, encounter.ErrInvalidData)
	require.ErrorIs(t, err, encounter.ErrNoField)
	require.Contains(t, err.Error(), "rpg-project#256",
		"the refusal names the change, so whoever reads it knows what to recreate")
	require.Contains(t, err.Error(), "rooms", "and names the key that gave the dialect away")
}

// TestAFieldAlwaysFitsOneGrid is W6: an axial hex grid is origin-CENTRED,
// negative coordinates are ordinary there, and widening its span always
// reaches further both ways — so a field always fits one grid, wherever it is
// painted.
func TestAFieldAlwaysFitsOneGrid(t *testing.T) {
	_, err := encounter.NewEncounter(&encounter.SetupInput{
		Sight: everyoneSeesTheWholeMap{}, Standing: everyoneStanding{}, Initiative: orderAsGiven{}, TurnDriver: passDriver{}, Striker: passStriker{},
		Field: encounter.FieldInput{
			Canvas:  encounter.CanvasInput{Void: encounter.VoidIsOpaque(), Orientation: encounter.HexesArePointyTop()},
			Regions: []encounter.RegionInput{rectRegion("crypt", -20, -20, 6, 6)},
		},
		Members: []encounter.MemberInput{
			{ID: alice, Kind: encounter.KindPlayer, Position: spatial.Position{X: -20, Y: -20}},
		},
		Endings: []encounter.EndingInput{{Key: "done", Trigger: encounter.TriggerExternal{}}},
	})
	require.NoError(t, err, "a hex field anchored deep in the negative quadrant is ordinary")
}

// TestPropsAreCompiledThroughTheOneConversion is the prop half of the
// compile, pinned where it is observable: a sightline.
//
// The Atlas reports props absolute too, but it computes that projection
// itself, from the same construction data — so an Atlas assertion cannot tell
// whether the CANVAS got them right. This one can: the blocking column sits in
// a chamber anchored well away from the origin, and the two members are placed
// so that a prop placed at its authored pair read as axial would land
// somewhere else entirely and block nothing.
func TestPropsAreCompiledThroughTheOneConversion(t *testing.T) {
	// A pillar wall three cells tall at the hall's own x=5, y=2..4 —
	// authored absolute [14, 6..8] through the hall's [9,4] anchor.
	field := tombField()
	field.Props = wallColumn(int(tombHallOrigin.X)+5, int(tombHallOrigin.Y)+2, int(tombHallOrigin.Y)+4)

	enc, err := encounter.NewEncounter(&encounter.SetupInput{
		Sight: everyoneSeesTheWholeMap{}, Standing: everyoneStanding{}, Initiative: orderAsGiven{}, TurnDriver: passDriver{}, Striker: passStriker{},
		Field: field,
		Members: []encounter.MemberInput{
			{ID: alice, Kind: encounter.KindPlayer, Position: tombSeat(tombHallOrigin, 3, 3)},
			{ID: bob, Kind: encounter.KindPlayer, Position: tombSeat(tombHallOrigin, 7, 3)},
		},
		Endings: []encounter.EndingInput{{Key: "done", Trigger: encounter.TriggerExternal{}}},
	})
	require.NoError(t, err)

	view, err := enc.View(&encounter.ViewInput{Member: alice})
	require.NoError(t, err)
	require.Empty(t, view, "the pillars stand between them, at the cells their room's anchor puts them")
}

// TestAWallThatCannotBeDrawnIsRefusedAtConstruction.
//
// A wall's endpoints have to be two adjacent floor cells. When they are not,
// construction says so and stops there — R5, and the reason this is worth a
// test of its own: compileField is the one place a wall is checked for BOTH
// construction seams, so an error swallowed here would be a wall the author
// declared and the encounter silently does not have.
func TestAWallThatCannotBeDrawnIsRefusedAtConstruction(t *testing.T) {
	setup := func(b spatial.Boundary) *encounter.SetupInput {
		return &encounter.SetupInput{
			Sight: everyoneSeesTheWholeMap{}, Standing: everyoneStanding{}, Initiative: orderAsGiven{}, TurnDriver: passDriver{}, Striker: passStriker{},
			Field: encounter.FieldInput{
				Canvas:  encounter.CanvasInput{Void: encounter.VoidIsOpaque(), Orientation: encounter.HexesArePointyTop()},
				Regions: []encounter.RegionInput{rectRegion("hall", 4, 4, 6, 6)}, Walls: []encounter.WallInput{{Boundary: b}},
			},
			Members: []encounter.MemberInput{
				{ID: alice, Kind: encounter.KindPlayer, Position: spatial.Position{X: 4, Y: 4}},
			},
			Endings: []encounter.EndingInput{{Key: "done", Trigger: encounter.TriggerExternal{}}},
		}
	}

	t.Run("endpoints that are not adjacent", func(t *testing.T) {
		_, err := encounter.NewEncounter(setup(spatial.Boundary{
			From: spatial.Position{X: 4, Y: 4}, To: spatial.Position{X: 7, Y: 4},
			BlocksMovement: true, BlocksLineOfSight: true,
		}))
		require.Error(t, err)
		require.ErrorIs(t, err, encounter.ErrEdgeNotAdjacent)
		require.Contains(t, err.Error(), "walls[0]", "the message names the wall by its index")
	})

	t.Run("an endpoint that is not floor", func(t *testing.T) {
		// The envelope is implied, never written: a crossing from floor into
		// void is a crossing nobody can make, so a wall drawn along the rim
		// has nothing to stand on.
		_, err := encounter.NewEncounter(setup(spatial.Boundary{
			From: spatial.Position{X: 4, Y: 4}, To: spatial.Position{X: 3, Y: 4},
			BlocksMovement: true, BlocksLineOfSight: true,
		}))
		require.Error(t, err)
		require.ErrorIs(t, err, encounter.ErrEdgeOffFloor)
		require.Contains(t, err.Error(), "[3,4]", "and names the endpoint that is void")
	})
}
