// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package encounter_test

import (
	"testing"

	"github.com/KirkDiggler/rpg-toolkit/tools/spatial"
	"github.com/stretchr/testify/suite"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter"
)

// footprintdoors_test.go is A DOOR THAT STANDS AS A RECTANGLE
// (rpg-project#485, R1; rpg-toolkit#1850) — the second geometry, proved
// through the same exported queries the first one is: [Encounter.Step],
// [Encounter.CellAt], the canvas's sight read, [Encounter.ToData] and the
// atlas.
//
// THE CLAIM UNDER TEST IS THE GATE, not the rectangle. That a 1x40 foot
// blocker stops a step is placed_props_test.go's claim and it is already
// proved there. What is new here is that the SAME rectangle stops nothing
// once [Encounter.OpenDoor] has been called and stops everything again after
// [Encounter.CloseDoor] — with no recompilation, no re-registration and no
// second copy of the state. Every assertion below is written so that a gate
// wired to the authored state instead of the live one, or to movement instead
// of both facts, fails it.
type FootprintDoorSuite struct {
	suite.Suite
}

func TestFootprintDoorSuite(t *testing.T) {
	suite.Run(t, new(FootprintDoorSuite))
}

// theLeaf is the door under test in this file.
const theLeaf encounter.DoorID = "vault/cellar-door"

// leafAcrossTheHall is the door's rectangle: a one-foot-thick leaf standing
// on the line x = 12.5 and long enough to span the 5x5 hall from end to end.
//
// TWO FACTS IN ONE SHAPE, deliberately. The hall's odd rows are offset half a
// cell, so their centres sit ON that line — cells (2,1) and (2,3) are cells
// the leaf COVERS, which is standing. The even rows' centres sit at x = 10
// and x = 15, so the leaf lies BETWEEN them, which is a crossing. One door
// answers both questions, exactly as a real door leaf does.
func leafAcrossTheHall() spatial.FootprintPlacement {
	return thinWall(1, 40, 0, spatial.Point{X: 12.5, Y: 8.66})
}

// footprintDoorField is the placed-props hall with one footprint door in it,
// in the state the caller names, and NO placed props: what blocks here is the
// door or nothing.
func footprintDoorField(state encounter.DoorState) encounter.FieldInput {
	field := placedField()
	field.Doors = []encounter.DoorInput{{
		ID:        theLeaf,
		Placement: placementPtr(leafAcrossTheHall()),
		State:     state,
	}}

	return field
}

func placementPtr(p spatial.FootprintPlacement) *spatial.FootprintPlacement { return &p }

// placementPtrValue is [placementPtr]'s twin for the list that takes a
// placement by value.
func placementPtrValue(p spatial.FootprintPlacement) spatial.FootprintPlacement { return p }

func (s *FootprintDoorSuite) setup(field encounter.FieldInput) *encounter.Encounter {
	enc, err := encounter.NewEncounter(&encounter.SetupInput{
		Sight:     everyoneSeesTheWholeMap{},
		Equipment: noHandsAreObserved{}, Standing: everyoneStanding{}, Initiative: orderAsGiven{},
		TurnDriver: passDriver{}, Striker: passStriker{}, Mover: quietMover{}, Announcer: quietAnnouncer{},
		Field:   field,
		Members: []encounter.MemberInput{{ID: alice, Kind: encounter.KindPlayer, Position: cellAt(2, 0)}},
		Endings: []encounter.EndingInput{{Key: "withdrawn", Trigger: encounter.TriggerExternal{}}},
	})
	s.Require().NoError(err)

	return enc
}

// sightAcross is whether the hall's two ends can see each other, through the
// canvas every host reads.
func (s *FootprintDoorSuite) sightAcross(enc *encounter.Encounter) bool {
	canvas, err := enc.Canvas()
	s.Require().NoError(err)

	return canvas.IsLineOfSightBlocked(cellAt(0, 0), cellAt(4, 0))
}

// TestAShutLeafRefusesTheWayThroughAndOpeningItOffersIt is the design's own
// "done when", both halves: a creature moving into the door's cells is
// refused while it is closed and allowed once it is open, and sight across it
// likewise.
//
// THE SAME ENCOUNTER THROUGHOUT. Nothing is rebuilt between the two answers,
// so what changed is the state and only the state — which is the whole claim.
// A gate that read the AUTHORED state would pass every "closed" line here and
// fail every line after OpenDoor.
func (s *FootprintDoorSuite) TestAShutLeafRefusesTheWayThroughAndOpeningItOffersIt() {
	enc := s.setup(footprintDoorField(encounter.DoorIsClosed()))

	// CROSSING: the leaf lies between (2,0) and (3,0), whose centres it does
	// not cover. A step through it is refused as the DOOR, by name.
	_, err := enc.Step(&encounter.StepInput{Member: alice, To: cellAt(3, 0)})
	s.Require().Error(err)
	s.ErrorIs(err, encounter.ErrDoorShut, "a shut door refuses as a door, not as a nameless placement")
	s.Contains(err.Error(), string(theLeaf))

	// STANDING: the leaf covers cell (2,1)'s centre, so the doorway itself is
	// nowhere to stand — the same refusal, through the cell fold.
	blocked := enc.CellAt(encounter.CellAtInput{Cell: cellAt(2, 1), Mover: alice})
	s.Equal(encounter.PassageBlocked, blocked.Passage)
	s.Require().Len(blocked.Contribs, 1)
	s.Equal(encounter.ContribDoor, blocked.Contribs[0].Kind,
		"the map says a DOOR is in the way — a route reads this, and a door is the part a caller can act on")
	s.Equal(string(theLeaf), blocked.Contribs[0].ID)

	_, err = enc.Step(&encounter.StepInput{Member: alice, To: cellAt(2, 1)})
	s.Require().Error(err)
	s.ErrorIs(err, encounter.ErrDoorShut)

	// SIGHT: closed blocks both facts, so the far end of the hall is unseen.
	s.True(s.sightAcross(enc), "a shut door is a wall to sight as well as to feet")

	// AND THEN THE VERB.
	_, err = enc.OpenDoor(&encounter.OpenDoorInput{Door: theLeaf})
	s.Require().NoError(err)

	s.False(s.sightAcross(enc), "an open door blocks nothing, including sight")

	offered := enc.CellAt(encounter.CellAtInput{Cell: cellAt(2, 1), Mover: alice})
	s.Equal(encounter.PassageStandable, offered.Passage)
	s.Empty(offered.Contribs, "an open door contributes nothing at all, not a non-blocking row")

	_, err = enc.Step(&encounter.StepInput{Member: alice, To: cellAt(3, 0)})
	s.Require().NoError(err, "the crossing the closed leaf refused is the way they walk through")

	// AND SHUTTING IT AGAIN IS THE SAME DOOR. Symmetric by construction: the
	// state is read per query, so there is nothing to have forgotten.
	_, err = enc.CloseDoor(&encounter.CloseDoorInput{Door: theLeaf})
	s.Require().NoError(err)
	s.True(s.sightAcross(enc))

	_, err = enc.Step(&encounter.StepInput{Member: alice, To: cellAt(2, 0)})
	s.Require().Error(err)
	s.ErrorIs(err, encounter.ErrDoorShut, "back through the shut leaf is refused again")
}

// TestALockedLeafNamesItsStakesAndUnlockOpensTheWay is the lock half: a
// locked footprint door refuses with the lock's own sentinel and its DC, and
// the way through is the verb pair that already existed.
func (s *FootprintDoorSuite) TestALockedLeafNamesItsStakesAndUnlockOpensTheWay() {
	lock := encounter.Lock{Approaches: []encounter.CheckApproach{{Ability: "str", DC: 15}}}
	enc := s.setup(footprintDoorField(encounter.DoorIsLocked(lock)))

	_, err := enc.Step(&encounter.StepInput{Member: alice, To: cellAt(3, 0)})
	s.Require().Error(err)
	s.ErrorIs(err, encounter.ErrLocked, "a locked door is its own sentinel, not a generic refusal")
	s.Contains(err.Error(), "DC 15 (str)", "and it names what beating it takes")

	// A locked leaf blocks exactly what a closed one does: a lock is a fact
	// about who may open a door, never a stronger wall.
	s.True(s.sightAcross(enc))

	// A FAILED ATTEMPT LEAVES IT LOCKED, and the leaf still stands.
	failed, err := enc.Unlock(&encounter.UnlockInput{
		Door: theLeaf, Beaten: false, Applied: lock.Approaches[0],
	})
	s.Require().NoError(err, "a lock that was not beaten is an outcome, not an error")
	s.False(failed.Beaten)
	s.True(s.sightAcross(enc))

	// Beating it is what opens the way — [Encounter.Unlock]'s own contract,
	// and the footprint follows the state it sets like any other.
	_, err = enc.Unlock(&encounter.UnlockInput{
		Door: theLeaf, Beaten: true, Applied: lock.Approaches[0],
	})
	s.Require().NoError(err)
	s.False(s.sightAcross(enc))

	_, err = enc.Step(&encounter.StepInput{Member: alice, To: cellAt(3, 0)})
	s.Require().NoError(err, "and the way through the beaten lock is walkable")
}

// TestAnOpenLeafIsTheGapItWasBefore: the door exists, is authored open, and
// the hall behaves exactly as the hall with no door in it.
func (s *FootprintDoorSuite) TestAnOpenLeafIsTheGapItWasBefore() {
	enc := s.setup(footprintDoorField(encounter.DoorIsOpen()))

	s.False(s.sightAcross(enc))
	fact := enc.CellAt(encounter.CellAtInput{Cell: cellAt(2, 1), Mover: alice})
	s.Equal(encounter.PassageStandable, fact.Passage)

	_, err := enc.Step(&encounter.StepInput{Member: alice, To: cellAt(3, 0)})
	s.Require().NoError(err)
}

// TestTheLeafIsNotADoorwayAndStillHasAState pins R4 and the pin on
// rpg-toolkit#1850: a footprint door never appears in [Atlas.Doorways] —
// which is edge-shaped and is what the web's legacy renderer reads — while
// [Encounter.Doors] lists it under the id the client joins live state by.
func (s *FootprintDoorSuite) TestTheLeafIsNotADoorwayAndStillHasAState() {
	enc := s.setup(footprintDoorField(encounter.DoorIsClosed()))

	atlas, err := enc.Atlas()
	s.Require().NoError(err)
	s.Empty(atlas.Doorways, "a footprint door stands in no crossing, so it is no doorway")

	doors := enc.Doors()
	s.Require().Len(doors, 1)
	s.Equal(theLeaf, doors[0].ID)
	s.Equal(encounter.DoorClosed, doors[0].State.Kind())
	s.Empty(doors[0].Edges)
	s.Require().NotNil(doors[0].Placement, "and the shape it stands as is readable")
	s.InDelta(40.0, doors[0].Placement.Footprint.Box.W, 1e-12)

	// COPY-OUT: editing what a caller was handed must not move the door.
	doors[0].Placement.Origin.X = 999
	again := enc.Doors()
	s.InDelta(12.5, again[0].Placement.Origin.X, 1e-12)
}

// TestASavedLeafComesBackShutAndStillBlocking is the persistence claim, and
// it is not incidental: the rectangle is measured on every read, so a blob
// that dropped it would load a door that names a state and stops nothing.
func (s *FootprintDoorSuite) TestASavedLeafComesBackShutAndStillBlocking() {
	enc := s.setup(footprintDoorField(encounter.DoorIsClosed()))

	data := enc.ToData()
	s.Require().Len(data.Doors, 1)
	s.Require().NotNil(data.Doors[0].Placement, "the blob carries the door's footprint")
	s.Empty(data.Doors[0].Edges)
	s.Equal("closed", data.Doors[0].State)

	loaded, err := encounter.LoadEncounter(&encounter.LoadEncounterInput{
		Data:      data,
		Sight:     everyoneSeesTheWholeMap{},
		Equipment: noHandsAreObserved{}, Standing: everyoneStanding{}, Initiative: orderAsGiven{},
		TurnDriver: passDriver{}, Striker: passStriker{}, Mover: quietMover{}, Announcer: quietAnnouncer{},
	})
	s.Require().NoError(err)

	s.True(s.sightAcross(loaded), "the reloaded leaf is still shut")
	_, err = loaded.Step(&encounter.StepInput{Member: alice, To: cellAt(3, 0)})
	s.Require().Error(err)
	s.ErrorIs(err, encounter.ErrDoorShut)

	// And the state that was SAVED is the one that comes back: open it,
	// save, load, and the reloaded hall is open.
	_, err = loaded.OpenDoor(&encounter.OpenDoorInput{Door: theLeaf})
	s.Require().NoError(err)
	reopened, err := encounter.LoadEncounter(&encounter.LoadEncounterInput{
		Data:      loaded.ToData(),
		Sight:     everyoneSeesTheWholeMap{},
		Equipment: noHandsAreObserved{}, Standing: everyoneStanding{}, Initiative: orderAsGiven{},
		TurnDriver: passDriver{}, Striker: passStriker{}, Mover: quietMover{}, Announcer: quietAnnouncer{},
	})
	s.Require().NoError(err)
	s.False(s.sightAcross(reopened))
}

// TestADoorHasOneGeometry is the construction law: exactly one shape, never
// two and never none.
func (s *FootprintDoorSuite) TestADoorHasOneGeometry() {
	both := footprintDoorField(encounter.DoorIsClosed())
	both.Doors[0].Edges = []encounter.DoorEdge{{From: cellAt(0, 0), To: cellAt(1, 0)}}
	_, err := encounter.NewEncounter(&encounter.SetupInput{
		Sight:     everyoneSeesTheWholeMap{},
		Equipment: noHandsAreObserved{}, Standing: everyoneStanding{}, Initiative: orderAsGiven{},
		TurnDriver: passDriver{}, Striker: passStriker{}, Mover: quietMover{}, Announcer: quietAnnouncer{},
		Field:   both,
		Endings: []encounter.EndingInput{{Key: "withdrawn", Trigger: encounter.TriggerExternal{}}},
	})
	s.Require().Error(err)
	s.ErrorIs(err, encounter.ErrBadDoor)
	s.Contains(err.Error(), "one geometry")

	neither := footprintDoorField(encounter.DoorIsClosed())
	neither.Doors[0].Placement = nil
	_, err = encounter.NewEncounter(&encounter.SetupInput{
		Sight:     everyoneSeesTheWholeMap{},
		Equipment: noHandsAreObserved{}, Standing: everyoneStanding{}, Initiative: orderAsGiven{},
		TurnDriver: passDriver{}, Striker: passStriker{}, Mover: quietMover{}, Announcer: quietAnnouncer{},
		Field:   neither,
		Endings: []encounter.EndingInput{{Key: "withdrawn", Trigger: encounter.TriggerExternal{}}},
	})
	s.Require().Error(err)
	s.ErrorIs(err, encounter.ErrBadDoor)

}

// TestAFootprintDoorMayBeHidden is THE REFUSAL THAT WAS BUILT
// (rpg-project#490, E1). A footprint door earned "a footprint and concealed,
// which nothing has built yet" until the concealment primitive landed,
// because a hidden rectangle a route reads as a cell contributor would have
// leaked its own existence through the map.
//
// What closed that hole is the concealment counting the cells the rectangle
// STANDS ON as its own: the floor goes with the door, so an unaware observer
// sees neither a door nor a hole where one is. Four claims, one per way the
// secret could leak:
//
//  1. the door is absent from their door list and their doorways;
//  2. its rectangle is absent from Atlas.Placed;
//  3. the cells it covers are absent from their Cells — no hole;
//  4. everything else placed in the hall is STILL THERE, which is what makes
//     claim 2 a filter rather than the old wholesale withholding.
func (s *FootprintDoorSuite) TestAFootprintDoorMayBeHidden() {
	field := footprintDoorField(encounter.DoorIsClosed())
	// THE V4 SHAPE, mirrored: that dialect compiles a door item into BOTH
	// lists — the rectangle the map draws, under the item's own id, and the
	// door whose state decides what it blocks, under `<key>/<id>`
	// (single_room_compile.go). So the leaf has a placed twin, and an
	// ordinary table beside it so the projection has something to keep as
	// well as something to withhold.
	field.Placed = []encounter.PlacedPropInput{
		{ID: "cellar-door", Placement: placementPtrValue(leafAcrossTheHall())},
		{ID: "table", Placement: thinWall(4, 4, 0, spatial.Point{X: 2.5, Y: 0})},
	}
	field.Concealments = []encounter.ConcealmentInput{{
		ID:     "cellar",
		Checks: []encounter.CheckApproach{{Ability: "perception", DC: 15}},
		Doors:  []encounter.DoorID{theLeaf},
	}}

	enc, err := encounter.NewEncounter(&encounter.SetupInput{
		Sight:     everyoneSeesTheWholeMap{},
		Equipment: noHandsAreObserved{}, Standing: everyoneStanding{}, Initiative: orderAsGiven{},
		TurnDriver: passDriver{}, Striker: passStriker{}, Mover: quietMover{}, Announcer: quietAnnouncer{},
		CheckResolver: findsEverything{}, Witness: nobodyPerceives{},
		Field:   field,
		Members: []encounter.MemberInput{{ID: alice, Kind: encounter.KindPlayer, Position: cellAt(0, 0)}},
		Endings: []encounter.EndingInput{{Key: "withdrawn", Trigger: encounter.TriggerExternal{}}},
	})
	s.Require().NoError(err, "a footprint door in a concealment is legal now")

	blind, err := enc.AtlasFor(alice)
	s.Require().NoError(err)

	for _, dw := range blind.Doorways {
		s.Require().NotEqual(theLeaf, dw.Door, "no doorway names the secret")
	}
	doors, err := enc.DoorsFor(alice)
	s.Require().NoError(err)
	for _, d := range doors {
		s.Require().NotEqual(theLeaf, d.ID, "nor the door list")
	}

	placedIDs := map[encounter.PropID]bool{}
	for _, p := range blind.Placed {
		placedIDs[p.ID] = true
	}
	s.False(placedIDs["cellar-door"],
		"the leaf's drawn rectangle is withheld — it stands on floor this observer cannot see, "+
			"which is how the v4 twin is covered without the concealment naming it twice")
	s.True(placedIDs["table"],
		"and the table beside it is not — Atlas.Placed is FILTERED now, not withheld wholesale")

	// The cells the leaf covers go with it: a hole in the floor exactly
	// where the secret is would be the tell the masquerade exists to remove.
	full, err := enc.Atlas()
	s.Require().NoError(err)
	s.Greater(len(full.Cells), len(blind.Cells), "the covered floor is withheld with the door")
	blindCells := map[spatial.Position]bool{}
	for _, c := range blind.Cells {
		blindCells[c] = true
	}
	s.False(blindCells[cellAt(2, 1)], "the leaf stands on this cell, so it hides with it")
	s.False(blindCells[cellAt(2, 3)], "and on this one")

	// And finding it hands the whole secret over.
	_, err = enc.Search(&encounter.SearchInput{Member: alice, Region: "hall"})
	s.Require().NoError(err)
	found, err := enc.AtlasFor(alice)
	s.Require().NoError(err)
	s.Equal(full.Cells, found.Cells, "the finder's floor is the whole floor again")
	foundIDs := map[encounter.PropID]bool{}
	for _, p := range found.Placed {
		foundIDs[p.ID] = true
	}
	s.True(foundIDs["cellar-door"], "and the leaf's drawn rectangle arrives with it")
	foundDoors, err := enc.DoorsFor(alice)
	s.Require().NoError(err)
	listed := false
	for _, d := range foundDoors {
		listed = listed || d.ID == theLeaf
	}
	s.True(listed, "as does the door itself")
}

// TestASeatIsNeverInsideAShutLeaf: the authored-placement check that the
// content compiler runs for a party seat and a monster cell sees the door
// too, in the state it was authored in — a creature standing inside a closed
// door is an author's defect, not something the run works out later.
func (s *FootprintDoorSuite) TestASeatIsNeverInsideAShutLeaf() {
	shut := footprintDoorField(encounter.DoorIsClosed())
	err := encounter.ValidateStaticPlacements(shut, []spatial.Position{cellAt(2, 1)})
	s.Require().Error(err)
	s.ErrorIs(err, encounter.ErrBadPlacement)
	s.Contains(err.Error(), string(theLeaf))

	// The same cell under the same door, authored open, is a seat.
	open := footprintDoorField(encounter.DoorIsOpen())
	s.Require().NoError(encounter.ValidateStaticPlacements(open, []spatial.Position{cellAt(2, 1)}))
}
