// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package encounter_test

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/KirkDiggler/rpg-toolkit/core"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter"
	"github.com/KirkDiggler/rpg-toolkit/tools/spatial"
)

// regions_test.go is A ROOM IS A REGION (rpg-toolkit#1108).
//
// S0 made the field one canvas and left the authored chambers as construction
// data with one runtime job: saying which of them holds a given cell. This
// slice gives that job a name. A region is a NAMED SET OF CELLS — you ask it
// which region holds a cell, or which members are standing in one — and it is
// not a coordinate space, not a container, and not a visibility rule.
//
// The fixture is canvas_test.go's reference tomb, deliberately: three chambers
// in a chain with two doorways, which is the shape that makes the doorway
// question answerable rather than hypothetical.
type RegionSuite struct {
	suite.Suite

	enc *encounter.Encounter
}

func TestRegionSuite(t *testing.T) {
	suite.Run(t, new(RegionSuite))
}

// SetupTest opens the same tomb canvas_test.go opens, with the same four
// members: alice ON the entrance's doorway cell, bob on the hall's side of that
// same opening, carol and dave one row up with the seam wall between them.
func (s *RegionSuite) SetupTest() {
	enc, err := encounter.NewEncounter(&encounter.SetupInput{
		Sight: everyoneSeesTheWholeMap{}, Standing: everyoneStanding{}, Initiative: orderAsGiven{}, TurnDriver: passDriver{},
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

// TestTheTombsThreeChambersAreRegions is the headline: the dungeon the stack
// has to carry answers RegionAt for every cell it owns, and answers it with the
// name the designer authored.
func (s *RegionSuite) TestTheTombsThreeChambersAreRegions() {
	chambers := []struct {
		id            encounter.RegionID
		origin        spatial.Position
		width, height int
	}{
		{tombEntrance, tombEntranceOrigin, 6, 8},
		{tombHall, tombHallOrigin, 10, 8},
		{tombChamber, tombChamberOrigin, 12, 8},
	}

	for _, c := range chambers {
		for x := 0; x < c.width; x++ {
			for y := 0; y < c.height; y++ {
				cell := tombAt(c.origin, x, y)
				got, ok := s.enc.RegionAt(cell)
				s.Require().True(ok, "cell %v is floor in %q", cell, c.id)
				s.Require().Equal(c.id, got, "cell %v", cell)
			}
		}
	}
}

// TestRegionAtIsTotalOverFloorAndSilentOverVoid pins the invariant the whole
// slice rests on, swept over the canvas the field actually compiles onto: a
// cell is floor if and only if exactly one region owns it.
//
// The tomb's canvas is its bounding box, so there ARE cells on it that no
// chamber owns — the strip below and left of the chain, which the authored
// rooms never reach. Those are the void S0 refuses to place anybody on, and
// they are what "region membership is total over FLOOR" means as distinct from
// "total over the canvas".
func (s *RegionSuite) TestRegionAtIsTotalOverFloorAndSilentOverVoid() {
	owners := 0
	void := 0
	for x := 0; x <= 30; x++ {
		for y := 0; y <= 11; y++ {
			cell := spatial.Position{X: float64(x), Y: float64(y)}
			id, ok := s.enc.RegionAt(cell)

			onFloor := y >= 4 && y <= 11 && x >= 3 && x <= 30
			if !onFloor {
				s.Require().False(ok, "cell %v is not floor", cell)
				s.Require().Empty(id, "a cell no region owns is not named")
				void++
				continue
			}
			s.Require().True(ok, "cell %v is floor", cell)
			s.Require().NotEmpty(id, "cell %v", cell)
			owners++
		}
	}
	s.Equal(6*8+10*8+12*8, owners, "every authored cell, once")
	s.Positive(void, "and the fixture must actually contain void, or this pins nothing")
}

// TestAMemberInTheDoorwayStandsInTheRegionTheyStandOn is THE DOORWAY DECISION.
//
// The old stack made a door's own cell belong to NO region on purpose
// (encounter/knowledge.go: "doors sit between two regions' tagged hex sets,
// never inside either"). That is a fact about a canvas whose compiler puts a
// one-cell WALL COLUMN between chambers and carves a floor door cell in it.
//
// This composition cannot express that cell. A connection's two endpoints are
// room-LOCAL cells of their own rooms (ConnectionInput's doc comment: "the
// position a member must stand on in each room to be at the doorway"), and W3
// makes them ADJACENT absolute cells — there is nothing between them. And under
// S0 a cell no room owns is not floor at all: Step and Join refuse it. So in
// this model "belongs to no region" and "you cannot stand there" are the same
// sentence, and a no-region doorway would be a doorway nobody could stand in.
//
// The answer here, therefore: a member in a doorway stands in the region whose
// cell is under their feet. alice and bob are one cell apart in the same
// opening and are in DIFFERENT regions — and they see each other, because sight
// is geometry and region membership is not sight.
func (s *RegionSuite) TestAMemberInTheDoorwayStandsInTheRegionTheyStandOn() {
	entranceSide := tombAt(tombEntranceOrigin, 5, tombDoorRow)
	hallSide := tombAt(tombHallOrigin, 0, tombDoorRow)
	s.Require().Equal(float64(1), hallSide.X-entranceSide.X, "the two endpoints are adjacent (W3)")

	got, ok := s.enc.RegionAt(entranceSide)
	s.Require().True(ok, "the doorway cell is floor — somebody is standing on it")
	s.Equal(encounter.RegionID(tombEntrance), got)

	got, ok = s.enc.RegionAt(hallSide)
	s.Require().True(ok)
	s.Equal(encounter.RegionID(tombHall), got, "the far endpoint belongs to the hall, not to nobody")

	s.Equal(encounter.RegionID(tombEntrance), s.regionOf(alice), "alice is IN the doorway")
	s.Equal(encounter.RegionID(tombHall), s.regionOf(bob), "bob is one cell through it")

	s.True(s.sees(alice, bob), "and they can see each other: the opening is open")
	s.True(s.sees(bob, alice))
}

// TestMembersInReportsEachChambersRoster is the other half of the pair: given a
// region, who is standing in it. The doorway pair lands on opposite sides of
// this answer, which is the same decision the test above makes, read from the
// other end.
func (s *RegionSuite) TestMembersInReportsEachChambersRoster() {
	s.Equal([]encounter.MemberID{alice, carol}, s.idsIn(tombEntrance))
	s.Equal([]encounter.MemberID{bob, dave}, s.idsIn(tombHall))
	s.Empty(s.idsIn(tombChamber), "nobody has reached the tomb yet, and that is an answer")
}

// TestMembersInCarriesTheSamePlacementEveryReadSpeaks — MembersIn is a roster
// read, so it reports what Members reports, filtered. A parallel projection
// here would be a second answer to "where is alice" (placementOf's rule).
func (s *RegionSuite) TestMembersInCarriesTheSamePlacementEveryReadSpeaks() {
	in, err := s.enc.MembersIn(tombEntrance)
	s.Require().NoError(err)
	s.Require().Len(in, 2)

	all, err := s.enc.Members()
	s.Require().NoError(err)
	byID := map[encounter.MemberID]encounter.Member{}
	for _, m := range all {
		byID[m.ID] = m
	}
	for _, m := range in {
		s.Equal(byID[m.ID], m, "MembersIn and Members must agree about %q", m.ID)
	}
}

// TestMembersInRefusesARegionTheFieldDoesNotHave. A typo'd region name must not
// read as "nobody is in the hall" — an empty roster is a real answer for a real
// region (see the tomb chamber above), so an unknown one has to be an error.
func (s *RegionSuite) TestMembersInRefusesARegionTheFieldDoesNotHave() {
	_, err := s.enc.MembersIn("scullery")
	s.Require().Error(err)
	s.ErrorIs(err, encounter.ErrNoRegion)
}

// TestAStepChangesTheRegionAMemberIsIn pins that membership is DERIVED from the
// cell rather than stored beside it: nothing tells the encounter that alice
// changed chambers, and the encounter says she did.
//
// Bob steps out of the opening first, because he is standing in it. That is the
// doorway decision showing up as a fact about the map rather than as a rule: an
// opening is one cell wide on each side, both of them ordinary floor of their
// own region, and two people cannot stand on one.
func (s *RegionSuite) TestAStepChangesTheRegionAMemberIsIn() {
	s.Require().Equal(encounter.RegionID(tombEntrance), s.regionOf(alice))

	_, err := s.enc.Step(&encounter.StepInput{
		Member: bob, To: tombAt(tombHallOrigin, 1, tombDoorRow),
	})
	s.Require().NoError(err)

	_, err = s.enc.Step(&encounter.StepInput{
		Member: alice, To: tombAt(tombHallOrigin, 0, tombDoorRow),
	})
	s.Require().NoError(err)

	s.Equal(encounter.RegionID(tombHall), s.regionOf(alice))
	s.Equal([]encounter.MemberID{carol}, s.idsIn(tombEntrance))
	s.Equal([]encounter.MemberID{alice, bob, dave}, s.idsIn(tombHall))
}

// TestReachingTheTombChamberClosesTheDelve is the issue's own done-when, in the
// dungeon it names: an ending declared on a tile in the tomb chamber fires when
// a player walks into that chamber.
//
// Worth having in the tomb's terms rather than only on a purpose-built fixture,
// because the two seams are what make it a real walk: alice starts on the
// hall's side of the tomb doorway and crosses into the chamber, which is one
// ordinary step through a gap in a wall, and the ending is one cell equality
// (arrival_test.go holds the rules that equality carries). The outcome then
// reports every member's region, derived from where they each stand.
func (s *RegionSuite) TestReachingTheTombChamberClosesTheDelve() {
	enc, err := encounter.NewEncounter(&encounter.SetupInput{
		Sight: everyoneSeesTheWholeMap{}, Standing: everyoneStanding{}, Initiative: orderAsGiven{}, TurnDriver: passDriver{},
		Field: tombField(),
		Members: []encounter.MemberInput{
			{ID: alice, Kind: encounter.KindPlayer, Room: tombHall,
				Position: spatial.Position{X: 9, Y: float64(tombDoorRow)}},
			{ID: carol, Kind: encounter.KindPlayer, Room: tombEntrance,
				Position: spatial.Position{X: 1, Y: 1}},
		},
		Endings: []encounter.EndingInput{
			{Key: "the-tomb", Trigger: encounter.TriggerReachedPosition{
				Room: tombChamber, Position: spatial.Position{X: 0, Y: float64(tombDoorRow)}}},
		},
	})
	s.Require().NoError(err)

	theTomb := tombAt(tombChamberOrigin, 0, tombDoorRow)
	got, ok := enc.RegionAt(theTomb)
	s.Require().True(ok)
	s.Require().Equal(encounter.RegionID(tombChamber), got, "the ending's tile is in the tomb chamber")

	out, err := enc.Step(&encounter.StepInput{Member: alice, To: theTomb})
	s.Require().NoError(err)
	s.Require().NotNil(out.Outcome, "walking into the tomb chamber closes the delve")
	s.Equal("the-tomb", out.Outcome.Ending)
	s.Equal(tombDoor, out.Crossing, "and it was an ordinary step through the doorway")

	finished := map[encounter.MemberID]encounter.RegionID{}
	for _, mo := range out.Outcome.Members {
		finished[mo.ID] = mo.Region
	}
	s.Equal(map[encounter.MemberID]encounter.RegionID{
		alice: tombChamber, carol: tombEntrance,
	}, finished, "the outcome names where each of them finished")
}

// TestTheRegionIsDerivedAcrossPersistenceToo is the persistence half of the
// same claim: nothing about a region is stored, at any point.
//
// An outcome used to carry a "room" key beside every member's cell — the last
// derived spatial fact this module persisted, and load validation had a branch
// whose whole job was policing the two against each other. The blob no longer
// says it and the reloaded outcome still names it, because the cell and the
// authored field say it between them (rpg-toolkit#1108).
func (s *RegionSuite) TestTheRegionIsDerivedAcrossPersistenceToo() {
	ended, err := s.enc.End(&encounter.EndInput{Ending: "withdrawn"})
	s.Require().NoError(err)
	s.Require().Len(ended.Outcome.Members, 4)

	data := s.enc.ToData()

	blob, err := json.Marshal(data.Outcome)
	s.Require().NoError(err)
	s.NotContains(string(blob), `"room"`, "an outcome member's region is derived, so it is not in the blob")
	s.NotContains(string(blob), `"region"`, "and it did not come back under a new name either")

	reloaded, err := encounter.LoadEncounter(&encounter.LoadEncounterInput{
		Sight: everyoneSeesTheWholeMap{}, Standing: everyoneStanding{}, Initiative: orderAsGiven{}, TurnDriver: passDriver{}, Data: data})
	s.Require().NoError(err)

	status, err := reloaded.Status()
	s.Require().NoError(err)
	s.Require().NotNil(status.Outcome)

	want := map[encounter.MemberID]encounter.RegionID{
		alice: tombEntrance, carol: tombEntrance, bob: tombHall, dave: tombHall,
	}
	for _, mo := range status.Outcome.Members {
		s.Equal(want[mo.ID], mo.Region, "%s finished in %q", mo.ID, want[mo.ID])
	}
	s.Require().Equal(ended.Outcome.Members, status.Outcome.Members,
		"a reloaded outcome must be the outcome the host already saw")
}

// TestAStaleRegionLabelInABlobCannotChangeTheAnswer is why dropping the
// persisted "room" key is a quiet change rather than a dialect break.
//
// #1068 renamed a persisted field so old blobs would FAIL LOUDLY, and it was
// right to: the coordinate FRAME had changed, so a blob written before the flip
// meant something different from what it said, and silence would have been a
// lie. Nothing like that is true here. The region was never information — it is
// a function of the cell and the authored field, both still in the blob — so a
// reader that ignores the old key cannot lose anything.
//
// This proves it rather than arguing it. The blob below is handed to
// LoadEncounter with every outcome member labelled "tomb", which is wrong for
// all four of them, and the encounter that comes back says where they actually
// stood. A field whose value cannot change the answer was never load-bearing —
// and a future reader tempted to "restore" it has to get past this test.
func (s *RegionSuite) TestAStaleRegionLabelInABlobCannotChangeTheAnswer() {
	ended, err := s.enc.End(&encounter.EndInput{Ending: "withdrawn"})
	s.Require().NoError(err)

	raw, err := json.Marshal(s.enc.ToData())
	s.Require().NoError(err)

	// Write the old key back in, with a value that is wrong for everybody.
	var generic map[string]any
	s.Require().NoError(json.Unmarshal(raw, &generic))
	outcome, ok := generic["outcome"].(map[string]any)
	s.Require().True(ok, "the fixture must have closed")
	members, ok := outcome["members"].([]any)
	s.Require().True(ok)
	s.Require().Len(members, 4)
	for _, m := range members {
		m.(map[string]any)["room"] = tombChamber
	}
	tampered, err := json.Marshal(generic)
	s.Require().NoError(err)
	s.Require().Contains(string(tampered), `"room":"`+tombChamber+`"`,
		"the stale label must actually be in the blob, or this test pins nothing")

	var data encounter.EncounterData
	s.Require().NoError(json.Unmarshal(tampered, &data))

	loaded, err := encounter.LoadEncounter(&encounter.LoadEncounterInput{
		Sight: everyoneSeesTheWholeMap{}, Standing: everyoneStanding{}, Initiative: orderAsGiven{}, TurnDriver: passDriver{}, Data: data})
	s.Require().NoError(err, "an unknown key is ignored, not a defect to refuse")

	status, err := loaded.Status()
	s.Require().NoError(err)
	s.Require().NotNil(status.Outcome)
	s.Equal(ended.Outcome.Members, status.Outcome.Members,
		"the cells decide, so the reloaded outcome is the one the host already saw — "+
			"nobody is in the tomb chamber, whatever the blob claimed")
}

// TestRegionAtAnswersExactlyWhatTheAuthoredGridWould, in both families, ANCHORED
// AWAY FROM THE ORIGIN — which is the whole of what it adds.
//
// atlas_test.go's TestRegionMembershipMatchesIsValidPosition sweeps every
// dimension parity against spatial's own IsValidPosition, and is the stronger
// test of the bounds rule; its rooms sit at Origin zero, where a lookup that
// forgot to project would pass anyway because local and absolute coincide.
// This one sweeps a window wider than the field around a region anchored
// somewhere else, in each family, so the projection is load-bearing.
func (s *RegionSuite) TestRegionAtAnswersExactlyWhatTheAuthoredGridWould() {
	s.Run("square", func() {
		origin := spatial.Position{X: 4, Y: 6}
		enc := s.oneRoom(spatial.GridShapeSquare, 5, 3, origin)
		for x := -4; x <= 14; x++ {
			for y := -4; y <= 14; y++ {
				cell := spatial.Position{X: float64(x), Y: float64(y)}
				local := cell.Subtract(origin)
				want := local.X >= 0 && local.X < 5 && local.Y >= 0 && local.Y < 3
				_, ok := enc.RegionAt(cell)
				s.Require().Equal(want, ok, "square cell %v (local %v)", cell, local)
			}
		}
	})

	s.Run("hex", func() {
		// A hex chamber is the OFFSET rectangle its author drew
		// (rpg-toolkit#1127), anchored at its corner — so the oracle is
		// "convert this cell back to offset and see whether it lands in the
		// rectangle", which is the same sentence as the square case above in
		// a frame that has to be converted into.
		//
		// This used to read `local.X >= -3 && local.X < 3` and so on, because
		// a hex room's span was origin-CENTRED. That reading is what put 156
		// cells of the reference tomb's chambers on the floor that nobody had
		// drawn.
		origin := spatial.Position{X: 3, Y: -2}
		enc := s.oneRoom(spatial.GridShapeHex, 6, 4, origin)
		for q := -10; q <= 10; q++ {
			for r := -10; r <= 10; r++ {
				cell := spatial.Position{X: float64(q), Y: float64(r)}
				offset := spatial.AxialToCube(cell).
					ToOffsetCoordinateWithOrientation(spatial.HexOrientationPointyTop)
				col, row := offset.X-origin.X, offset.Y-origin.Y
				want := col >= 0 && col < 6 && row >= 0 && row < 4
				_, ok := enc.RegionAt(cell)
				s.Require().Equal(want, ok, "hex cell %v (authored col,row %v,%v)", cell, col, row)
			}
		}
	})
}

// TestAFractionalHexPositionIsNotACell pins the answer to the question a public
// predicate can be asked that an internal one could not.
//
// A hex grid's IsValidPosition bounds-checks and nothing else, so while this
// lookup was internal the integrality rule lived at the verbs — every caller
// asked first, and a check inside would have been unreachable. Making the
// question public made it reachable: a host can hand RegionAt any position at
// all. Before this was fixed, RegionAt answered ("only", true) for the axial
// position below while Join refused the very same one as not a cell — two
// answers to "is this floor", which is what region.go exists to prevent.
//
// Square is deliberately NOT covered by that rule: it is fractional-tolerant by
// design, and a member standing between its cells really is in the region whose
// span holds them.
func (s *RegionSuite) TestAFractionalHexPositionIsNotACell() {
	hexOrigin := spatial.Position{X: 3, Y: -2}
	hex := s.oneRoom(spatial.GridShapeHex, 6, 6, hexOrigin)

	// A REAL cell of this room, asked for in the frame the room is authored in
	// rather than written down as an absolute literal. It used to be the
	// literal (4,-1); that stopped being floor when rpg-toolkit#1141 corrected
	// the offset schemes and the projection moved underneath it. What the test
	// is about — that a fractional position is not a cell and an integral one
	// beside it is — has nothing to do with which absolute cell that happens to
	// be, so it no longer names one.
	cell := encounter.HexCellAt(orientationFor(spatial.GridShapeHex),
		int(hexOrigin.X)+3, int(hexOrigin.Y)+3)

	// Half a step off a genuine cell, so it is unambiguously between cells
	// rather than merely outside the room.
	frac := spatial.Position{X: cell.X + 0.5, Y: cell.Y + 0.5}
	_, ok := hex.RegionAt(frac)
	s.False(ok, "a fractional axial position is not a cell, so no region holds it")

	_, err := hex.Join(&encounter.JoinInput{Member: bob, Kind: encounter.KindPlayer, Cell: frac})
	s.Require().Error(err, "and the verbs agree — which is the whole point")
	s.ErrorIs(err, encounter.ErrBadPlacement)

	_, ok = hex.RegionAt(cell)
	s.True(ok, "the integral cell beside it is ordinary floor")

	squareOrigin := spatial.Position{X: 4, Y: 6}
	square := s.oneRoom(spatial.GridShapeSquare, 5, 3, squareOrigin)
	got, ok := square.RegionAt(spatial.Position{X: 5.5, Y: 7.5})
	s.True(ok, "square is fractional-tolerant by design")
	s.Equal(encounter.RegionID("only"), got)
}

// oneRoom opens a single-region encounter of the given family, anchored away
// from the origin so a local coordinate cannot pass as an absolute one.
func (s *RegionSuite) oneRoom(shape spatial.GridShape, w, h int, origin spatial.Position) *encounter.Encounter {
	enc, err := encounter.NewEncounter(&encounter.SetupInput{
		Sight: everyoneSeesTheWholeMap{}, Standing: everyoneStanding{}, Initiative: orderAsGiven{}, TurnDriver: passDriver{},
		Field: encounter.FieldInput{
			Canvas: encounter.CanvasInput{Void: encounter.VoidIsOpaque(), Orientation: orientationFor(shape)},
			Rooms: []encounter.RoomInput{
				{ID: "only", Width: w, Height: h, Grid: shape, Origin: origin},
			}},
		Members: []encounter.MemberInput{
			// Local (0,0) is a cell in both families, and since
			// rpg-toolkit#1127 it is the same cell in both: the corner a
			// chamber is counted from. Hex rooms used to be centred on it.
			{ID: alice, Kind: encounter.KindPlayer, Room: "only", Position: spatial.Position{X: 0, Y: 0}},
		},
		Endings: []encounter.EndingInput{{Key: "done", Trigger: encounter.TriggerExternal{}}},
	})
	s.Require().NoError(err)
	return enc
}

// regionOf reads a member's region off the roster.
func (s *RegionSuite) regionOf(id core.EntityID) encounter.RegionID {
	members, err := s.enc.Members()
	s.Require().NoError(err)
	for _, m := range members {
		if m.ID == id {
			return m.Region
		}
	}
	s.Require().Fail("no such member", string(id))
	return ""
}

// idsIn names who MembersIn reports, in the order it reports them.
func (s *RegionSuite) idsIn(region encounter.RegionID) []encounter.MemberID {
	members, err := s.enc.MembersIn(region)
	s.Require().NoError(err)
	ids := make([]encounter.MemberID, 0, len(members))
	for _, m := range members {
		ids = append(ids, m.ID)
	}
	return ids
}

// sees reports whether an observer holds anything on a subject.
func (s *RegionSuite) sees(observer, subject core.EntityID) bool {
	view, err := s.enc.View(&encounter.ViewInput{Member: observer})
	s.Require().NoError(err)
	for _, h := range view {
		if string(h.Subject) == string(subject) {
			return true
		}
	}
	return false
}
