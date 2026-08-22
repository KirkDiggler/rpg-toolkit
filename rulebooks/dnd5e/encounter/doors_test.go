// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package encounter_test

import (
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/KirkDiggler/rpg-toolkit/core"
	"github.com/KirkDiggler/rpg-toolkit/play/intel"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter"
	"github.com/KirkDiggler/rpg-toolkit/tools/spatial"
)

// doors_test.go is A DOOR IS STATE ON THE WALL (rpg-toolkit#1123, world-model S4).
//
// Since #1106 a wall is a boundary edge and a doorway is the ABSENCE of one, so
// Kirk's wide open gate — "a large gate that is open and could be 4 hexes or so
// where 2 regions touch" — already worked at any width. What did not exist is a
// doorway that can be SHUT.
//
// The whole slice is one sentence of his: **a door is a set of edges sharing
// one state.** Doors were modelled per connection over on the old stack, one
// door per adjacent cell pair — and its `AuthoredDoorID` DERIVES a door's
// identity from that pair, so a four-hex gate over there is four independent
// doors that can disagree with each other. Here the state lives on the door and
// the edges only point at it, which is why TestAGateIsOneThingNotFour can be
// asserted structurally rather than merely observed.
type DoorSuite struct {
	suite.Suite
}

func TestDoorSuite(t *testing.T) {
	suite.Run(t, new(DoorSuite))
}

const (
	doorWest = "west"
	doorEast = "east"

	theDoor = "the-door"
	theGate = "the-gate"

	nessa = core.EntityID("nessa")
	orin  = core.EntityID("orin")
)

// A pair of chambers that TOUCH, walled along the whole seam except where a
// door stands. The footprints tile the canvas exactly, so nothing here depends
// on what void is (rpg-toolkit#1116) — the only thing between the two members
// is the door.
//
//	x: 0 1 2 | 3 4 5
//	    west | east
//
// nessa stands at (2,1) and orin at (3,1), one cell apart, with the door's edge
// between them.
var (
	nessaCell = spatial.Position{X: 2, Y: 1}
	orinCell  = spatial.Position{X: 3, Y: 1}
)

// seamWallExcept walls every crossing of the seam at x=atX|atX+1 for a chamber
// `height` tall — straight AND diagonal — except exactly the straight crossings
// the door stands in.
//
// EXACTLY those, not "everything touching a door row", and the difference
// matters: on a square grid the diagonal crossings into a door's row are a
// separate way through, so leaving them open would let sight round the corner
// of a shut door and the test would be measuring the gap beside the door rather
// than the door. The wall is everything the door is not.
func seamWallExcept(atX, height int, doorRows ...int) []spatial.Boundary {
	isDoor := make(map[int]bool, len(doorRows))
	for _, r := range doorRows {
		isDoor[r] = true
	}

	out := make([]spatial.Boundary, 0, height*3)
	for y := 0; y < height; y++ {
		for _, dy := range []int{-1, 0, 1} {
			to := y + dy
			if to < 0 || to >= height {
				continue
			}
			if dy == 0 && isDoor[y] {
				continue // the door's own crossing
			}
			out = append(out, spatial.Boundary{
				From:              spatial.Position{X: float64(atX), Y: float64(y)},
				To:                spatial.Position{X: float64(atX + 1), Y: float64(to)},
				BlocksMovement:    true,
				BlocksLineOfSight: true,
			})
		}
	}
	return out
}

// doorEdgesAcross returns the straight seam crossings at x=atX|atX+1 for each
// named row — one edge for a doorway, four for a gate, all sharing one state.
func doorEdgesAcross(atX int, rows ...int) []encounter.DoorEdge {
	out := make([]encounter.DoorEdge, 0, len(rows))
	for _, y := range rows {
		out = append(out, encounter.DoorEdge{
			From: spatial.Position{X: float64(atX), Y: float64(y)},
			To:   spatial.Position{X: float64(atX + 1), Y: float64(y)},
		})
	}
	return out
}

// doorField is two chambers `height` tall with one door standing in the rows
// named, in the state given.
func doorField(height int, state encounter.DoorState, id encounter.DoorID, rows ...int) encounter.FieldInput {
	return encounter.FieldInput{
		Canvas: encounter.CanvasInput{Void: encounter.VoidIsOpaque()},
		Rooms: []encounter.RoomInput{
			{ID: doorWest, Width: 3, Height: height, Origin: spatial.Position{X: 0, Y: 0},
				Boundaries: seamWallExcept(2, height, rows...)},
			{ID: doorEast, Width: 3, Height: height, Origin: spatial.Position{X: 3, Y: 0}},
		},
		Doors: []encounter.DoorInput{{ID: id, Edges: doorEdgesAcross(2, rows...), State: state}},
	}
}

// doorway opens the two-chamber fixture with one member on each side of a
// single-edge door, both players so nothing forms a fight.
func (s *DoorSuite) doorway(state encounter.DoorState) *encounter.Encounter {
	enc, err := encounter.NewEncounter(&encounter.SetupInput{
		Sight: everyoneSeesTheWholeMap{}, Standing: everyoneStanding{}, Initiative: orderAsGiven{}, TurnDriver: passDriver{}, Striker: passStriker{},
		Field: doorField(3, state, theDoor, 1),
		Members: []encounter.MemberInput{
			{ID: nessa, Kind: encounter.KindPlayer, Room: doorWest, Position: spatial.Position{X: 2, Y: 1}},
			{ID: orin, Kind: encounter.KindPlayer, Room: doorEast, Position: spatial.Position{X: 0, Y: 1}},
		},
		Endings: []encounter.EndingInput{{Key: "withdrawn", Trigger: encounter.TriggerExternal{}}},
	})
	s.Require().NoError(err)

	return enc
}

// sees reports whether an observer can see a subject RIGHT NOW, which is not
// the same question as whether they hold anything about them.
//
// A door shutting does not un-teach anybody. What a member already saw stays in
// their intel as a HOLDING with no current channel — a ghost, in this stack's
// own word (data_test.go's "CurrentVia cleared") — because knowing where
// somebody was a moment ago is a real thing to know and deleting it would be
// this module editing a memory. What the door changes is whether sight is a
// CURRENT channel on that holding, which is what these tests are about.
// Pinned as its own fact by TestAShutDoorLeavesAGhostRatherThanForgetting.
func (s *DoorSuite) sees(enc *encounter.Encounter, observer, subject core.EntityID) bool {
	view, err := enc.View(&encounter.ViewInput{Member: observer})
	s.Require().NoError(err)
	for _, h := range view {
		if h.Subject != intel.Subject(subject) {
			continue
		}
		for _, via := range h.CurrentVia {
			if via == intel.Sight {
				return true
			}
		}
	}

	return false
}

// holdsAnythingAbout reports whether an observer holds a subject at all,
// current or stale.
func (s *DoorSuite) holdsAnythingAbout(enc *encounter.Encounter, observer, subject core.EntityID) bool {
	view, err := enc.View(&encounter.ViewInput{Member: observer})
	s.Require().NoError(err)
	for _, h := range view {
		if h.Subject == intel.Subject(subject) {
			return true
		}
	}

	return false
}

// TestAClosedDoorStopsBothAndOpeningRestoresBoth is the slice in one scene.
//
// nessa and orin are one cell apart with the door's edge between them. Closed,
// it is a wall: neither sees the other and neither can step through. Opened, it
// is the gap it was before doors existed — and nothing about the geometry
// changed, only the state the edge is in.
func (s *DoorSuite) TestAClosedDoorStopsBothAndOpeningRestoresBoth() {
	enc := s.doorway(encounter.DoorIsClosed())

	s.False(s.sees(enc, nessa, orin), "a closed door is a wall")
	s.False(s.sees(enc, orin, nessa), "from either side")

	_, err := enc.Step(&encounter.StepInput{Member: nessa, To: orinCell})
	s.Require().Error(err, "and it stops a step the way any wall does")
	s.Require().ErrorIs(err, encounter.ErrBadPlacement)
	s.Contains(err.Error(), theDoor, "the refusal names the door standing in the way")

	out, err := enc.OpenDoor(&encounter.OpenDoorInput{Door: theDoor})
	s.Require().NoError(err)
	s.Equal(encounter.DoorOpen, out.State, "it is open now")

	s.True(s.sees(enc, nessa, orin), "opening restores sight")
	s.True(s.sees(enc, orin, nessa))

	stepped, err := enc.Step(&encounter.StepInput{Member: nessa, To: orinCell})
	s.Require().NoError(err, "and restores the way through")
	s.Equal([]encounter.CrossedDoor{{ID: theDoor, State: encounter.DoorOpen}}, stepped.Doors,
		"a step through a door names it, and the state it was in")
}

// TestClosingItAgainStopsBothAgain is the other direction, which is not
// symmetric for free: opening removes blocking from the edges, and closing has
// to put it back on exactly the edges it came off.
func (s *DoorSuite) TestClosingItAgainStopsBothAgain() {
	enc := s.doorway(encounter.DoorIsOpen())
	s.True(s.sees(enc, nessa, orin), "open to start with")

	out, err := enc.CloseDoor(&encounter.CloseDoorInput{Door: theDoor})
	s.Require().NoError(err)
	s.Equal(encounter.DoorClosed, out.State)

	s.False(s.sees(enc, nessa, orin), "shut again")
	_, err = enc.Step(&encounter.StepInput{Member: nessa, To: orinCell})
	s.Require().ErrorIs(err, encounter.ErrBadPlacement)
}

// TestAGateIsOneThingNotFour is Kirk's requirement, and the reason a door is
// not a connection.
//
// A four-edge gate opens and closes as ONE thing. The old stack could not say
// that: its authored door identity is DERIVED from an edge's own endpoint pair
// (encounter/authored_edges.go's AuthoredDoorID), so a four-hex gate there is
// four doors that can disagree. Here there is no per-edge state to disagree
// WITH — asserted structurally below, because a test that only checks the four
// edges agree would pass on a model that merely happened to set them together.
func (s *DoorSuite) TestAGateIsOneThingNotFour() {
	gateRows := []int{1, 2, 3, 4}

	enc, err := encounter.NewEncounter(&encounter.SetupInput{
		Sight: everyoneSeesTheWholeMap{}, Standing: everyoneStanding{}, Initiative: orderAsGiven{}, TurnDriver: passDriver{}, Striker: passStriker{},
		Field:   doorField(6, encounter.DoorIsClosed(), theGate, gateRows...),
		Endings: []encounter.EndingInput{{Key: "withdrawn", Trigger: encounter.TriggerExternal{}}},
	})
	s.Require().NoError(err)

	canvas, err := enc.Canvas()
	s.Require().NoError(err)

	blockedRows := func() []bool {
		out := make([]bool, 0, len(gateRows))
		for _, y := range gateRows {
			out = append(out, canvas.IsLineOfSightBlocked(
				spatial.Position{X: 2, Y: float64(y)}, spatial.Position{X: 3, Y: float64(y)}))
		}
		return out
	}

	s.Equal([]bool{true, true, true, true}, blockedRows(), "four edges, all shut")

	_, err = enc.OpenDoor(&encounter.OpenDoorInput{Door: theGate})
	s.Require().NoError(err)
	s.Equal([]bool{false, false, false, false}, blockedRows(), "one verb opened all four")

	_, err = enc.CloseDoor(&encounter.CloseDoorInput{Door: theGate})
	s.Require().NoError(err)
	s.Equal([]bool{true, true, true, true}, blockedRows(), "and shut all four")
}

// TestNoEdgeCarriesAStateOfItsOwn is the structural half of the gate: there is
// no state for two edges of one door to disagree about, because an edge has no
// state field at all. A door has exactly one.
func (s *DoorSuite) TestNoEdgeCarriesAStateOfItsOwn() {
	s.Equal([]string{"From", "To"}, structFieldNames(encounter.DoorEdge{}),
		"an edge is two cells and nothing else — a blocking flag here would be a second truth")
	s.Equal([]string{"ID", "Edges", "State"}, structFieldNames(encounter.Door{}),
		"and the state is the DOOR's, held once for however many edges it has")
}

// TestADoorMustSayWhatStateItIsIn is #1033's law again: a door with no declared
// state would be this module deciding whether a dungeon's gates start open.
func (s *DoorSuite) TestADoorMustSayWhatStateItIsIn() {
	field := doorField(3, encounter.DoorIsClosed(), theDoor, 1)
	field.Doors[0].State = nil

	_, err := encounter.NewEncounter(&encounter.SetupInput{
		Sight: everyoneSeesTheWholeMap{}, Standing: everyoneStanding{}, Initiative: orderAsGiven{}, TurnDriver: passDriver{}, Striker: passStriker{},
		Field:   field,
		Endings: []encounter.EndingInput{{Key: "withdrawn", Trigger: encounter.TriggerExternal{}}},
	})
	s.Require().ErrorIs(err, encounter.ErrBadDoor)
	s.Contains(err.Error(), theDoor, "and it names the door that did not say")
}

// TestAShutDoorLeavesAGhostRatherThanForgetting pins what closing a door does
// to what people already know, which is a different thing from what they can
// see.
//
// nessa watched orin through the open doorway. Shutting it takes orin out of
// her sight and leaves the last thing she saw of him in her intel, with no
// current channel on it. That is the honest model — she knows where he WAS —
// and it is what makes the distinction in this file's sees() helper load-bearing
// rather than pedantic.
func (s *DoorSuite) TestAShutDoorLeavesAGhostRatherThanForgetting() {
	enc := s.doorway(encounter.DoorIsOpen())
	s.Require().True(s.sees(enc, nessa, orin), "she is watching him through the open door")

	_, err := enc.CloseDoor(&encounter.CloseDoorInput{Door: theDoor})
	s.Require().NoError(err)

	s.False(s.sees(enc, nessa, orin), "the door is shut; she cannot see him")
	s.True(s.holdsAnythingAbout(enc, nessa, orin),
		"but she has not forgotten him — a shut door is not amnesia")
}

// setup opens a field, or returns why it could not be built.
func (s *DoorSuite) setup(field encounter.FieldInput) error {
	_, err := encounter.NewEncounter(&encounter.SetupInput{
		Sight: everyoneSeesTheWholeMap{}, Standing: everyoneStanding{}, Initiative: orderAsGiven{}, TurnDriver: passDriver{}, Striker: passStriker{},
		Field:   field,
		Endings: []encounter.EndingInput{{Key: "withdrawn", Trigger: encounter.TriggerExternal{}}},
	})

	return err
}

// withDoors is the two-chamber fixture with its doors replaced wholesale, for
// the defects below. Its seam is walled everywhere except row 1.
func withDoors(doors ...encounter.DoorInput) encounter.FieldInput {
	field := doorField(3, encounter.DoorIsClosed(), theDoor, 1)
	field.Doors = doors

	return field
}

// TestADoorThatCannotBePartOfAFieldIsRefusedByName walks the defect classes.
//
// Every one names the door and says what is wrong with it, because a field
// refused with "bad door" and nothing else sends its author back to read all of
// them. The floor case is the one worth pointing at: a door hanging in the void
// is a wall drawn across nothing, which is #880's rule — and it is only askable
// at all because rpg-toolkit#1116 made the canvas say what void is.
func (s *DoorSuite) TestADoorThatCannotBePartOfAFieldIsRefusedByName() {
	edge := func(x1, y1, x2, y2 float64) encounter.DoorEdge {
		return encounter.DoorEdge{
			From: spatial.Position{X: x1, Y: y1},
			To:   spatial.Position{X: x2, Y: y2},
		}
	}
	closedAt := func(id encounter.DoorID, edges ...encounter.DoorEdge) encounter.DoorInput {
		return encounter.DoorInput{ID: id, Edges: edges, State: encounter.DoorIsClosed()}
	}

	for _, tc := range []struct {
		name  string
		doors []encounter.DoorInput
		says  string
	}{
		{"no id", []encounter.DoorInput{closedAt("", edge(2, 1, 3, 1))}, "no id"},
		{"duplicate id", []encounter.DoorInput{
			closedAt(theDoor, edge(2, 1, 3, 1)), closedAt(theDoor, edge(2, 0, 3, 0))}, "duplicate door"},
		{"no edges", []encounter.DoorInput{closedAt(theDoor)}, "stands in no edges"},
		{"a cell joined to itself", []encounter.DoorInput{closedAt(theDoor, edge(2, 1, 2, 1))},
			"same cell at both ends"},
		{"cells that do not touch", []encounter.DoorInput{closedAt(theDoor, edge(0, 1, 3, 1))},
			"not adjacent"},
		{"an endpoint off the floor", []encounter.DoorInput{closedAt(theDoor, edge(5, 1, 6, 1))},
			"is not floor"},
		{"the same crossing named twice", []encounter.DoorInput{
			closedAt(theDoor, edge(2, 1, 3, 1), edge(3, 1, 2, 1))}, "twice"},
		{"two doors in one crossing", []encounter.DoorInput{
			closedAt(theDoor, edge(2, 1, 3, 1)), closedAt(theGate, edge(3, 1, 2, 1))},
			"could not then have one state"},
		{"a crossing a room already walled", []encounter.DoorInput{closedAt(theDoor, edge(2, 0, 3, 0))},
			"already drew a wall"},
		{"a lock nothing has to beat", []encounter.DoorInput{{
			ID: theDoor, Edges: []encounter.DoorEdge{edge(2, 1, 3, 1)},
			State: encounter.DoorIsLocked(encounter.Lock{})}}, "nothing has to beat"},
	} {
		s.Run(tc.name, func() {
			err := s.setup(withDoors(tc.doors...))
			s.Require().ErrorIs(err, encounter.ErrBadDoor)
			s.Contains(err.Error(), tc.says)
		})
	}
}

// TestAnUndirectedCrossingIsTheSameCrossingEitherWayRound is why every check
// above normalizes first.
//
// A door is undirected exactly as a [spatial.Boundary] is, so naming a crossing
// backwards has to collide with naming it forwards — otherwise "two doors in
// one crossing" is a defect a caller can walk straight past by writing the
// second one in the other order, and the one-state promise is gone.
func (s *DoorSuite) TestAnUndirectedCrossingIsTheSameCrossingEitherWayRound() {
	backwards := encounter.DoorInput{
		ID:    theDoor,
		Edges: []encounter.DoorEdge{{From: spatial.Position{X: 3, Y: 1}, To: spatial.Position{X: 2, Y: 1}}},
		State: encounter.DoorIsClosed(),
	}
	field := doorField(3, encounter.DoorIsClosed(), theDoor, 1)
	field.Doors = []encounter.DoorInput{backwards}

	enc, err := encounter.NewEncounter(&encounter.SetupInput{
		Sight: everyoneSeesTheWholeMap{}, Standing: everyoneStanding{}, Initiative: orderAsGiven{}, TurnDriver: passDriver{}, Striker: passStriker{},
		Field: field,
		Members: []encounter.MemberInput{
			{ID: nessa, Kind: encounter.KindPlayer, Room: doorWest, Position: spatial.Position{X: 2, Y: 1}},
			{ID: orin, Kind: encounter.KindPlayer, Room: doorEast, Position: spatial.Position{X: 0, Y: 1}},
		},
		Endings: []encounter.EndingInput{{Key: "withdrawn", Trigger: encounter.TriggerExternal{}}},
	})
	s.Require().NoError(err, "written backwards is still a legal crossing")
	s.False(s.sees(enc, nessa, orin), "and it shuts the same crossing")

	s.Equal([]encounter.DoorEdge{{
		From: spatial.Position{X: 2, Y: 1},
		To:   spatial.Position{X: 3, Y: 1},
	}}, enc.Doors()[0].Edges, "and it is reported the one way round, normalized")
}

// TestDoorsAreReadInStableOrder is C8 for the read surface: what a pass
// concludes must be a function of persisted data rather than of the order
// somebody happened to author two doors in.
func (s *DoorSuite) TestDoorsAreReadInStableOrder() {
	// The seam is excepted at BOTH rows, so each door stands in a crossing no
	// room walled — which is the validator working, found by writing this
	// fixture wrong the first time.
	field := doorField(6, encounter.DoorIsClosed(), theDoor, 1, 4)
	field.Doors = []encounter.DoorInput{
		{ID: "zeta", Edges: doorEdgesAcross(2, 4), State: encounter.DoorIsOpen()},
		{ID: "alpha", Edges: doorEdgesAcross(2, 1), State: encounter.DoorIsClosed()},
	}

	enc, err := encounter.NewEncounter(&encounter.SetupInput{
		Sight: everyoneSeesTheWholeMap{}, Standing: everyoneStanding{}, Initiative: orderAsGiven{}, TurnDriver: passDriver{}, Striker: passStriker{},
		Field:   field,
		Endings: []encounter.EndingInput{{Key: "withdrawn", Trigger: encounter.TriggerExternal{}}},
	})
	s.Require().NoError(err)

	doors := enc.Doors()
	s.Require().Len(doors, 2)
	s.Equal(encounter.DoorID("alpha"), doors[0].ID, "sorted by ID, not by declaration order")
	s.Equal(encounter.DoorID("zeta"), doors[1].ID)
	s.Equal(encounter.DoorClosed, doors[0].State.Kind(), "each with its own state")
	s.Equal(encounter.DoorOpen, doors[1].State.Kind())
}

// TestDoorsIsACopyOut: a caller cannot move a door by editing what they were
// handed, which is the same promise every other read on this type makes and the
// opposite of the deliberate exception [Encounter.Canvas] is.
func (s *DoorSuite) TestDoorsIsACopyOut() {
	enc := s.doorway(encounter.DoorIsClosed())

	handed := enc.Doors()
	s.Require().Len(handed, 1)
	handed[0].Edges[0] = encounter.DoorEdge{
		From: spatial.Position{X: 0, Y: 0},
		To:   spatial.Position{X: 1, Y: 0},
	}

	s.Equal(doorEdgesAcross(2, 1), enc.Doors()[0].Edges, "the door did not move")
	s.False(s.sees(enc, nessa, orin), "and it is still shut across the crossing it was authored in")
}

// TestAskingADoorForWhatItHasAlreadyDoneIsRefused is the no-silent-no-op rule,
// which this module applies everywhere else and would be conspicuous for
// dropping here. Compare [Encounter.Dissolve]'s ErrNoBubble.
func (s *DoorSuite) TestAskingADoorForWhatItHasAlreadyDoneIsRefused() {
	open := s.doorway(encounter.DoorIsOpen())
	_, err := open.OpenDoor(&encounter.OpenDoorInput{Door: theDoor})
	s.Require().ErrorIs(err, encounter.ErrBadDoor)
	s.Contains(err.Error(), "already open")

	shut := s.doorway(encounter.DoorIsClosed())
	_, err = shut.CloseDoor(&encounter.CloseDoorInput{Door: theDoor})
	s.Require().ErrorIs(err, encounter.ErrBadDoor)
	s.Contains(err.Error(), "already closed")

	_, err = shut.Unlock(&encounter.UnlockInput{Door: theDoor, Beaten: true})
	s.Require().ErrorIs(err, encounter.ErrBadDoor)
	s.Contains(err.Error(), "not locked", "there is nothing to beat on an unlocked door")
}

// TestALockedDoorCannotBeClosedTwice: locked is closed already, so asking to
// shut it is asking for something that has happened — and answering "done"
// would quietly tell a caller they had shut a door somebody else's key opens.
func (s *DoorSuite) TestALockedDoorCannotBeClosedTwice() {
	enc := s.doorway(encounter.DoorIsLocked(encounter.Lock{DC: 10}))

	_, err := enc.CloseDoor(&encounter.CloseDoorInput{Door: theDoor})
	s.Require().ErrorIs(err, encounter.ErrBadDoor)
	s.Contains(err.Error(), "already locked")
}

// TestABlobWhoseDoorMakesNoSenseIsRefusedByName is the wire half of the
// standing precedent (rpg-toolkit#1053/#1068: fail loudly, no migration).
//
// Two of these are the ordinary absent/unknown pair. The other two are the
// interesting ones: a door's state and its lock are two halves of one fact, and
// a blob where they disagree has no right answer. Guessing — dropping the
// stray lock, or inventing a DC for the locked door that carries none — is this
// module deciding what a dungeon's gate is, which is the thing it is not
// allowed to do.
func (s *DoorSuite) TestABlobWhoseDoorMakesNoSenseIsRefusedByName() {
	saved := func() encounter.EncounterData {
		return s.doorway(encounter.DoorIsClosed()).ToData()
	}

	for _, tc := range []struct {
		name string
		bend func(*encounter.DoorData)
		says string
	}{
		{"no state at all", func(d *encounter.DoorData) { d.State = "" },
			"does not say what state it is in"},
		{"a state from a dialect this build does not speak", func(d *encounter.DoorData) { d.State = "barred" },
			"which this build does not know"},
		{"locked, with nothing to beat", func(d *encounter.DoorData) { d.State = "locked" },
			"says nothing about the lock"},
		{"closed, yet carrying a lock", func(d *encounter.DoorData) { d.Lock = &encounter.LockData{DC: 12} },
			"carries a lock"},
	} {
		s.Run(tc.name, func() {
			data := saved()
			s.Require().Len(data.Doors, 1)
			tc.bend(&data.Doors[0])

			_, err := encounter.LoadEncounter(&encounter.LoadEncounterInput{
				Data:  data,
				Sight: everyoneSeesTheWholeMap{}, Standing: everyoneStanding{}, Initiative: orderAsGiven{}, TurnDriver: passDriver{}, Striker: passStriker{},
			})
			s.Require().ErrorIs(err, encounter.ErrBadDoor)
			s.Require().ErrorIs(err, encounter.ErrInvalidData)
			s.Contains(err.Error(), theDoor, "and it names the door")
			s.Contains(err.Error(), tc.says)
		})
	}
}

// TestABlobFromBeforeDoorsExistedLoadsFine is the other side of that coin, and
// the reason no old blob is refused by this slice.
//
// Doors are ADDITIVE. A field with none is an ordinary field where every
// opening is a gap nobody can shut, which is exactly what every encounter
// written before this slice meant — so there is no dialect change to fail
// loudly about, and inventing one would refuse blobs that are perfectly clear.
// The precedent is about SHAPES THAT MOVED (#1068's room-local cell, #1116's
// undeclared void), not about every new field.
func (s *DoorSuite) TestABlobFromBeforeDoorsExistedLoadsFine() {
	data := s.doorway(encounter.DoorIsClosed()).ToData()
	data.Doors = nil

	back, err := encounter.LoadEncounter(&encounter.LoadEncounterInput{
		Data:  data,
		Sight: everyoneSeesTheWholeMap{}, Standing: everyoneStanding{}, Initiative: orderAsGiven{}, TurnDriver: passDriver{}, Striker: passStriker{},
	})
	s.Require().NoError(err)
	s.Empty(back.Doors(), "no doors, and no complaint about it")

	// Asked of the MAP, not of percepts: LoadEncounter restores the intel the
	// blob carries rather than re-surveying the world, so what these two hold
	// about each other is what they held when it was saved — with the door
	// shut. The geometry is what the dropped door changed, and the geometry is
	// what this asks.
	canvas, err := back.Canvas()
	s.Require().NoError(err)
	s.False(canvas.IsLineOfSightBlocked(nessaCell, orinCell),
		"the crossing the door stood in is an open gap again — which is what a field with no doors IS")
}

// TestTheHandedOutCanvasCannotBeUsedToOpenADoor guards the seam
// rpg-toolkit#1118 opened.
//
// The canvas goes out to be READ, and a door's edges are boundaries on it. If
// that view carried spatial's boundary writers, anybody holding it could open a
// locked door by re-registering its edge — no beat, no sight refresh, and a
// door whose record says locked while the map says open. It does not: the view
// is a [spatial.Room], deliberately not a [spatial.BoundaryAwareRoom].
func (s *DoorSuite) TestTheHandedOutCanvasCannotBeUsedToOpenADoor() {
	enc := s.doorway(encounter.DoorIsLocked(encounter.Lock{DC: 12}))

	canvas, err := enc.Canvas()
	s.Require().NoError(err)

	_, writable := canvas.(spatial.BoundaryAwareRoom)
	s.False(writable, "the read-only canvas must not hand out RegisterBoundary/RemoveBoundary")

	s.True(canvas.IsLineOfSightBlocked(nessaCell, orinCell), "and the door is still shut")
}

// TestAStepSeveralCellsLongStillMeetsTheDoorInItsPath is Copilot's finding on
// PR #1125, and it was a real gap rather than a polish item.
//
// [Encounter.Step] does NOT check adjacency, deliberately: what "one step"
// means for a walk is a rule about walking and it lives with the walk, and a
// decider's IntentMoveTo never carried an adjacency contract. So a caller can
// legitimately name a cell several away — and tools/spatial refuses such a move
// on ANY movement-blocking crossing along the way, not just at the ends.
//
// The first version of the door lookup inspected only the two endpoints. A long
// step through an open door therefore reported no door at all, and a long step
// into a shut one got spatial's "cannot cross movement-blocking boundary"
// instead of the door's name and state — the exact answer this slice exists to
// give. Both halves are pinned here.
func (s *DoorSuite) TestAStepSeveralCellsLongStillMeetsTheDoorInItsPath() {
	far := spatial.Position{X: 5, Y: 1} // three cells past the door, in the east chamber

	s.Run("shut, and refused by the door's name from four cells away", func() {
		enc := s.doorway(encounter.DoorIsClosed())
		_, err := enc.Step(&encounter.StepInput{Member: nessa, To: far})
		s.Require().ErrorIs(err, encounter.ErrBadPlacement)
		s.Contains(err.Error(), theDoor, "the door in the middle of the path is what stopped her")
		s.Contains(err.Error(), string(encounter.DoorClosed), "and the refusal says what state it is in")
	})

	s.Run("open, and reported even though it is at neither end", func() {
		enc := s.doorway(encounter.DoorIsOpen())
		out, err := enc.Step(&encounter.StepInput{Member: nessa, To: far})
		s.Require().NoError(err)
		s.Equal([]encounter.CrossedDoor{{ID: theDoor, State: encounter.DoorOpen}}, out.Doors,
			"she went through it on the way, so the step says so")
	})
}

// threeChambers is a row of three chambers with a door at each seam, so a
// single step can cross two of them.
func threeChambers(first, second encounter.DoorState, gate1, gate2 encounter.DoorID) encounter.FieldInput {
	return encounter.FieldInput{
		Canvas: encounter.CanvasInput{Void: encounter.VoidIsOpaque()},
		Rooms: []encounter.RoomInput{
			{ID: "west", Width: 3, Height: 3, Origin: spatial.Position{X: 0, Y: 0},
				Boundaries: seamWallExcept(2, 3, 1)},
			{ID: "middle", Width: 3, Height: 3, Origin: spatial.Position{X: 3, Y: 0},
				Boundaries: seamWallExcept(2, 3, 1)},
			{ID: "east", Width: 3, Height: 3, Origin: spatial.Position{X: 6, Y: 0}},
		},
		Doors: []encounter.DoorInput{
			{ID: gate1, Edges: doorEdgesAcross(2, 1), State: first},
			{ID: gate2, Edges: doorEdgesAcross(5, 1), State: second},
		},
	}
}

func (s *DoorSuite) walkerIn(field encounter.FieldInput) *encounter.Encounter {
	enc, err := encounter.NewEncounter(&encounter.SetupInput{
		Sight: everyoneSeesTheWholeMap{}, Standing: everyoneStanding{}, Initiative: orderAsGiven{}, TurnDriver: passDriver{}, Striker: passStriker{},
		Field: field,
		Members: []encounter.MemberInput{
			{ID: nessa, Kind: encounter.KindPlayer, Room: "west", Position: spatial.Position{X: 0, Y: 1}},
		},
		Endings: []encounter.EndingInput{{Key: "withdrawn", Trigger: encounter.TriggerExternal{}}},
	})
	s.Require().NoError(err)

	return enc
}

// TestAStepThroughTwoDoorsNamesBothInTravelOrder defines the multi-door case
// explicitly, rather than leaving a singular field to pick one by accident.
//
// A long step crosses both gates, and the answer is both, in the order she met
// them — the MOVER's order, not the rasterization's:
// [spatial.CanonicalBoundaryRay] is ordered from the start cell whichever way
// round the endpoints happen to sort, and walking back proves it.
func (s *DoorSuite) TestAStepThroughTwoDoorsNamesBothInTravelOrder() {
	const gate1, gate2 = "gate-one", "gate-two"
	enc := s.walkerIn(threeChambers(encounter.DoorIsOpen(), encounter.DoorIsOpen(), gate1, gate2))

	out, err := enc.Step(&encounter.StepInput{Member: nessa, To: spatial.Position{X: 8, Y: 1}})
	s.Require().NoError(err)
	s.Equal([]encounter.CrossedDoor{
		{ID: gate1, State: encounter.DoorOpen},
		{ID: gate2, State: encounter.DoorOpen},
	}, out.Doors, "both gates, west to east, in the order she went through them")

	back, err := enc.Step(&encounter.StepInput{Member: nessa, To: spatial.Position{X: 0, Y: 1}})
	s.Require().NoError(err)
	s.Equal([]encounter.CrossedDoor{
		{ID: gate2, State: encounter.DoorOpen},
		{ID: gate1, State: encounter.DoorOpen},
	}, back.Doors, "and the other way round coming back — travel order is hers, not the ray's")
}

// TestTheDoorThatRefusesIsTheOneThatStoppedHer pins that a refusal names the
// door that actually blocked, not merely the first door on the path — the same
// first-blocking-crossing rule spatial applies.
func (s *DoorSuite) TestTheDoorThatRefusesIsTheOneThatStoppedHer() {
	const gate1, gate2 = "gate-one", "gate-two"
	enc := s.walkerIn(threeChambers(encounter.DoorIsOpen(), encounter.DoorIsClosed(), gate1, gate2))

	_, err := enc.Step(&encounter.StepInput{Member: nessa, To: spatial.Position{X: 8, Y: 1}})
	s.Require().ErrorIs(err, encounter.ErrBadPlacement)
	s.Contains(err.Error(), gate2, "the SHUT one stopped her")
	s.NotContains(err.Error(), gate1, "not the open one she walked through first")
}
