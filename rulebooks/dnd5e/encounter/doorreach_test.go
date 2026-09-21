// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package encounter_test

// doorreach_test.go is A DOOR OPENS ONLY FROM BESIDE IT (rpg-toolkit#1856,
// rpg-project#488 R2).
//
// Kirk, 2026-09-21: "Currently I can open a door not next to it." All three
// door verbs took an actor, named them on the beat, and never asked where
// they were standing — so a member across the room could swing a gate, pick
// a lock and shut a door behind somebody they could not touch.
//
// THE RULE IS BORROWED, NOT INVENTED. It is [Encounter.Hold]'s: grid
// distance, adjacent by default, measured against every cell the thing
// stands on, and in reach of any of them is in reach. What is new here is
// only the thing being reached FOR, and a door has two geometries to be:
//
//   - an EDGE door stands in its crossings, which is two cells per edge —
//     so a gate is reachable from either side of the seam it closes, and
//     from anywhere along its width;
//   - a FOOTPRINT door stands where [field.placedCells] says a rectangle
//     stands, INCLUDING its centre-cell clause — the same derivation a
//     placed prop's reach asks (rpg-toolkit#1854), which is why a door leaf
//     too small to cover a cell centre is still reachable from the hex it
//     lies in rather than from nowhere.
//
// AND WHERE IT SITS IN THE ORDER IS ITSELF A CLAIM. Reach is judged after
// the probe law and before the lock: a stranger guessing at a secret still
// hears "no such door" rather than "out of range", and a member across the
// room is told they cannot reach rather than what the lock would cost them.
// Both are pinned below, because both are ways a refusal can leak.

import (
	"fmt"
	"strings"
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter"
	"github.com/KirkDiggler/rpg-toolkit/tools/spatial"
)

type DoorReachSuite struct {
	suite.Suite
}

func TestDoorReachSuite(t *testing.T) {
	suite.Run(t, new(DoorReachSuite))
}

const (
	// theHatch is a door leaf SMALLER THAN A HEX: it covers no cell centre,
	// so only [field.placedCells]' centre clause puts it anywhere at all.
	theHatch encounter.DoorID = "hatch"

	// theCrate is a holdable rectangle in the same hall, so the refusal a
	// far door verb gives can be compared against the refusal Hold gives
	// for the same distance.
	theCrate = "crate"

	// reachDC is the lock the order scene hangs on the edge door. Nothing
	// rolls against it; what matters is that a hand across the room is
	// never told it exists.
	reachDC = 17
)

// The two-chamber fixture's row: doorField stands one door in the seam at
// x = 2|3, so its cells are the two the crossing joins.
const reachRow = 1

var (
	// doorWestCell and doorEastCell are the two cells the edge door stands
	// in — the crossing itself, one cell each side of the seam.
	doorWestCell = cellAt(2, reachRow)
	doorEastCell = cellAt(3, reachRow)

	// theHatch lies a foot off cell (4,4)'s centre: inside that hex, on no
	// centre at all — the World Builder's own sub-hex shape.
	hatchHex = cellAt(4, 4)
)

// edgeDoorAt opens the two-chamber fixture with alice alone, standing in the
// authored seat the scene puts her in.
//
// ALONE, deliberately: a second member would form a bubble, and whose turn
// it is has nothing to do with whether a door is within arm's length.
func (s *DoorReachSuite) edgeDoorAt(state encounter.DoorState, seat spatial.Position) *encounter.Encounter {
	enc, err := encounter.NewEncounter(&encounter.SetupInput{
		Sight:     everyoneSeesTheWholeMap{},
		Equipment: noHandsAreObserved{}, Standing: everyoneStanding{}, Initiative: orderAsGiven{},
		TurnDriver: passDriver{}, Striker: passStriker{}, Mover: quietMover{}, Announcer: quietAnnouncer{},
		Field:   doorField(3, state, theDoor, reachRow),
		Members: []encounter.MemberInput{{ID: alice, Kind: encounter.KindPlayer, Position: seat}},
		Endings: []encounter.EndingInput{{Key: "withdrawn", Trigger: encounter.TriggerExternal{}}},
	})
	s.Require().NoError(err)

	return enc
}

// hatchPlacement is the sub-hex leaf: a ten-inch square sitting a foot off
// cell (4,4)'s centre, which is [scrollPlacement]'s shape wearing a door's
// state.
func hatchPlacement() spatial.FootprintPlacement {
	centre := centreOf(hatchHex)

	return coveredBox(0.87, spatial.Point{X: centre.X + 1.2, Y: centre.Y})
}

// placedDoorHall is the 5x5 hall with the doors the scene names and one
// holdable crate off in the corner, on transparent void.
func placedDoorHall(doors ...encounter.DoorInput) encounter.FieldInput {
	field := placedField(encounter.PlacedPropInput{
		ID: theCrate, Placement: coveredBox(3, centreOf(cellAt(0, 0))), Holdable: true,
	})
	field.Doors = doors

	return field
}

// placedDoorAt opens the hall with alice alone in the authored seat given.
func (s *DoorReachSuite) placedDoorAt(field encounter.FieldInput, seat spatial.Position) *encounter.Encounter {
	enc, err := encounter.NewEncounter(&encounter.SetupInput{
		Sight:     everyoneSeesTheWholeMap{},
		Equipment: noHandsAreObserved{}, Standing: everyoneStanding{}, Initiative: orderAsGiven{},
		TurnDriver: passDriver{}, Striker: passStriker{}, Mover: quietMover{}, Announcer: quietAnnouncer{},
		Field:   field,
		Members: []encounter.MemberInput{{ID: alice, Kind: encounter.KindPlayer, Position: seat}},
		Endings: []encounter.EndingInput{{Key: "withdrawn", Trigger: encounter.TriggerExternal{}}},
	})
	s.Require().NoError(err)

	return enc
}

// leafDoor is the hall-spanning leaf as a door in the state given — the
// footprint geometry, covering three of the hall's cells.
func leafDoor(state encounter.DoorState) encounter.DoorInput {
	return encounter.DoorInput{ID: theLeaf, Placement: placementPtr(leafAcrossTheHall()), State: state}
}

// hatchDoor is the sub-hex leaf as a door in the state given.
func hatchDoor(state encounter.DoorState) encounter.DoorInput {
	return encounter.DoorInput{ID: theHatch, Placement: placementPtr(hatchPlacement()), State: state}
}

// TestAnEdgeDoorIsOpenedFromEitherSideOfItsCrossingAndNotFromBeyond is the
// bug in one scene: the same door, the same actor, five different seats.
//
// The crossing's OWN cells are distance zero and the cells beside them
// distance one — asserted with [Encounter.Distance] rather than read off the
// picture — and both open the door. Two cells out, on either side, is
// refused by name. A verb that measured nothing passes the first three lines
// and fails the last two.
func (s *DoorReachSuite) TestAnEdgeDoorIsOpenedFromEitherSideOfItsCrossingAndNotFromBeyond() {
	s.Run("standing in the crossing's own cell", func() {
		enc := s.edgeDoorAt(encounter.DoorIsClosed(), authoredAt(2, reachRow))
		s.Require().Zero(enc.Distance(doorWestCell, doorWestCell))

		out, err := enc.OpenDoor(&encounter.OpenDoorInput{Door: theDoor, Actor: alice})
		s.Require().NoError(err)
		s.Equal(encounter.DoorOpen, out.State)
	})

	s.Run("beside it, on the west", func() {
		enc := s.edgeDoorAt(encounter.DoorIsClosed(), authoredAt(1, reachRow))
		s.Require().Equal(float64(1), enc.Distance(cellAt(1, reachRow), doorWestCell))

		_, err := enc.OpenDoor(&encounter.OpenDoorInput{Door: theDoor, Actor: alice})
		s.Require().NoError(err)
	})

	s.Run("beside it, on the east — the far half of the same crossing", func() {
		enc := s.edgeDoorAt(encounter.DoorIsClosed(), authoredAt(4, reachRow))
		s.Require().Equal(float64(1), enc.Distance(cellAt(4, reachRow), doorEastCell),
			"a door's two cells are both its own: the east one is what this hand reaches")

		_, err := enc.OpenDoor(&encounter.OpenDoorInput{Door: theDoor, Actor: alice})
		s.Require().NoError(err, "a gate is worked from either side of the seam it closes")
	})

	s.Run("two cells west is out of reach", func() {
		enc := s.edgeDoorAt(encounter.DoorIsClosed(), authoredAt(0, reachRow))
		for _, cell := range []spatial.Position{doorWestCell, doorEastCell} {
			s.Require().Greater(enc.Distance(cellAt(0, reachRow), cell), float64(1))
		}

		_, err := enc.OpenDoor(&encounter.OpenDoorInput{Door: theDoor, Actor: alice})
		s.Require().ErrorIs(err, encounter.ErrOutOfRange)
		s.Equal("open door: "+string(theDoor)+": "+encounter.ErrOutOfRange.Error(), err.Error())

		s.Equal(encounter.DoorClosed, s.stateOf(enc, theDoor), "and the door did not move")
	})

	s.Run("two cells east is out of reach too", func() {
		enc := s.edgeDoorAt(encounter.DoorIsClosed(), authoredAt(5, reachRow))
		for _, cell := range []spatial.Position{doorWestCell, doorEastCell} {
			s.Require().Greater(enc.Distance(cellAt(5, reachRow), cell), float64(1))
		}

		_, err := enc.OpenDoor(&encounter.OpenDoorInput{Door: theDoor, Actor: alice})
		s.Require().ErrorIs(err, encounter.ErrOutOfRange)
	})
}

// TestEveryDoorVerbMeasuresTheSameReach is the claim that this is ONE rule
// rather than three: close and unlock refuse the same hand at the same
// distance, and let the same hand through one cell closer.
//
// Table-driven over the verbs because the near/far pair is the whole point:
// a near case alone would pass a verb that never measured, and a far case
// alone would pass one that refused everybody.
func (s *DoorReachSuite) TestEveryDoorVerbMeasuresTheSameReach() {
	lock := oneApproachLock(reachDC)
	tests := []struct {
		name  string
		state encounter.DoorState
		verb  string
		act   func(*encounter.Encounter) error
	}{
		{
			name: "open", state: encounter.DoorIsClosed(), verb: "open door",
			act: func(enc *encounter.Encounter) error {
				_, err := enc.OpenDoor(&encounter.OpenDoorInput{Door: theDoor, Actor: alice})
				return err
			},
		},
		{
			name: "close", state: encounter.DoorIsOpen(), verb: "close door",
			act: func(enc *encounter.Encounter) error {
				_, err := enc.CloseDoor(&encounter.CloseDoorInput{Door: theDoor, Actor: alice})
				return err
			},
		},
		{
			name: "unlock", state: encounter.DoorIsLocked(lock), verb: "unlock",
			act: func(enc *encounter.Encounter) error {
				_, err := enc.Unlock(&encounter.UnlockInput{
					Door: theDoor, Actor: alice, Beaten: true, Applied: lock.Approaches[0],
				})
				return err
			},
		},
	}

	for _, test := range tests {
		s.Run(test.name+" reaches from beside the door", func() {
			enc := s.edgeDoorAt(test.state, authoredAt(1, reachRow))
			s.Require().NoError(test.act(enc))
		})

		s.Run(test.name+" refuses from two cells away", func() {
			enc := s.edgeDoorAt(test.state, authoredAt(0, reachRow))
			err := test.act(enc)
			s.Require().ErrorIs(err, encounter.ErrOutOfRange)
			s.Equal(test.verb+": "+string(theDoor)+": "+encounter.ErrOutOfRange.Error(), err.Error(),
				"one sentence, with the verb's own name in front of it")
		})
	}
}

// TestTheRefusalIsHoldsOwnSentence pins that this is not a second reach rule
// wearing the first one's sentinel: the door's refusal and Hold's differ
// only by the verb and the thing named, for one hand standing in one place.
//
// Computed by SWAPPING those two words rather than by writing the expected
// string out, so a change to either message that the other did not make
// fails here — which is the whole point of borrowing the rule.
func (s *DoorReachSuite) TestTheRefusalIsHoldsOwnSentence() {
	enc := s.placedDoorAt(placedDoorHall(leafDoor(encounter.DoorIsClosed())), authoredAt(4, 0))

	_, held := enc.Hold(&encounter.HoldInput{Member: alice, Target: theCrate})
	s.Require().ErrorIs(held, encounter.ErrOutOfRange, "the crate is out of reach from here")

	_, opened := enc.OpenDoor(&encounter.OpenDoorInput{Door: theLeaf, Actor: alice})
	s.Require().ErrorIs(opened, encounter.ErrOutOfRange, "and so is the door")

	s.Equal(
		strings.Replace(held.Error(), "hold: "+theCrate, "open door: "+string(theLeaf), 1),
		opened.Error(),
		"the door verb refuses in Hold's own words")
}

// TestAPlacedDoorIsReachedFromEveryCellItStandsOn is the footprint geometry:
// the leaf spans the hall, and a hand on ANY of the cells its rectangle
// stands on can shut it — not only the one its origin sits in.
//
// Shut from each of the three in turn, and refused from a seat beyond all
// three. A rule that measured from the placement's origin alone passes the
// first case and fails the other two.
func (s *DoorReachSuite) TestAPlacedDoorIsReachedFromEveryCellItStandsOn() {
	// The three cells the leaf stands on, in the authored frame the fixture
	// seats members in — and they are not all the same KIND of cell, which
	// is why each is classified here rather than listed.
	//
	// Two of them the rectangle covers: the odd rows' centres sit on the
	// line it stands on, so shut, it is what closes them. The third is the
	// hex its own middle lies in, between two centres it is too thin to
	// cover — [field.placedCells]' second clause. All three are cells this
	// door is worked from, and the difference is asserted rather than
	// assumed.
	for _, seat := range []struct {
		col, row int
		covered  bool
	}{{2, 1, true}, {2, 3, true}, {2, 2, false}} {
		s.Run(fmt.Sprintf("shut from (%d,%d), a cell it stands on", seat.col, seat.row), func() {
			shut := s.placedDoorAt(placedDoorHall(leafDoor(encounter.DoorIsClosed())), authoredAt(0, 0))
			stood := shut.CellAt(encounter.CellAtInput{Cell: cellAt(seat.col, seat.row), Mover: alice})
			if seat.covered {
				s.Require().Equal(encounter.PassageBlocked, stood.Passage)
				s.Require().Len(stood.Contribs, 1)
				s.Require().Equal(string(theLeaf), stood.Contribs[0].ID,
					"shut, the leaf is what closes this cell — it covers its centre")
			} else {
				s.Require().Equal(encounter.PassageStandable, stood.Passage,
					"the hex the leaf's middle lies in, which its rectangle is too thin to close")
			}

			// And every one of them is a cell a hand shuts it from.
			enc := s.placedDoorAt(placedDoorHall(leafDoor(encounter.DoorIsOpen())), authoredAt(seat.col, seat.row))

			out, err := enc.CloseDoor(&encounter.CloseDoorInput{Door: theLeaf, Actor: alice})
			s.Require().NoError(err)
			s.Equal(encounter.DoorClosed, out.State)
		})
	}

	s.Run("and opened from beside one of them", func() {
		enc := s.placedDoorAt(placedDoorHall(leafDoor(encounter.DoorIsClosed())), authoredAt(2, 0))
		s.Require().Equal(float64(1), enc.Distance(cellAt(2, 0), cellAt(2, 1)))

		_, err := enc.OpenDoor(&encounter.OpenDoorInput{Door: theLeaf, Actor: alice})
		s.Require().NoError(err)
	})

	s.Run("and refused from beyond every one of them", func() {
		enc := s.placedDoorAt(placedDoorHall(leafDoor(encounter.DoorIsClosed())), authoredAt(4, 0))
		for _, stood := range []spatial.Position{cellAt(2, 1), cellAt(2, 2), cellAt(2, 3)} {
			s.Require().Greater(enc.Distance(cellAt(4, 0), stood), float64(1),
				"the hand is beyond reach of every cell the leaf stands on")
		}

		_, err := enc.OpenDoor(&encounter.OpenDoorInput{Door: theLeaf, Actor: alice})
		s.Require().ErrorIs(err, encounter.ErrOutOfRange)
	})
}

// TestADoorTooSmallToCoverACentreIsReachedFromItsOwnHex is the centre-cell
// clause, which is the reason the placed rule is [field.placedCells] rather
// than "cells whose centre it covers".
//
// The hatch covers NO cell centre — it blocks nothing, which is asserted
// here by a member standing in its hex at all — and it is still somewhere:
// the hex it lies in. Opened from that hex and from beside it, refused two
// cells out. Without the clause its cell set would be some other hex
// entirely and all three answers would change.
func (s *DoorReachSuite) TestADoorTooSmallToCoverACentreIsReachedFromItsOwnHex() {
	s.Run("from the hex it lies in", func() {
		enc := s.placedDoorAt(placedDoorHall(hatchDoor(encounter.DoorIsClosed())), authoredAt(4, 4))
		s.Require().Zero(enc.Distance(cellAt(4, 4), hatchHex),
			"the seat IS the hatch's hex — a leaf this small closes no cell, so there is room to stand")

		_, err := enc.OpenDoor(&encounter.OpenDoorInput{Door: theHatch, Actor: alice})
		s.Require().NoError(err)
	})

	s.Run("from beside it", func() {
		enc := s.placedDoorAt(placedDoorHall(hatchDoor(encounter.DoorIsClosed())), authoredAt(3, 4))
		s.Require().Equal(float64(1), enc.Distance(cellAt(3, 4), hatchHex))

		_, err := enc.OpenDoor(&encounter.OpenDoorInput{Door: theHatch, Actor: alice})
		s.Require().NoError(err)
	})

	s.Run("and not from two cells away", func() {
		enc := s.placedDoorAt(placedDoorHall(hatchDoor(encounter.DoorIsClosed())), authoredAt(2, 4))
		s.Require().Equal(float64(2), enc.Distance(cellAt(2, 4), hatchHex))

		_, err := enc.OpenDoor(&encounter.OpenDoorInput{Door: theHatch, Actor: alice})
		s.Require().ErrorIs(err, encounter.ErrOutOfRange)
	})
}

// TestAFarHandIsToldItCannotReachRatherThanWhatTheLockCosts is the order
// claim: reach is judged BEFORE the lock.
//
// The same locked door, two seats. Across the room it answers out of range
// and its DC appears nowhere in the sentence; one cell closer the lock
// itself refuses and names its price. A verb that asked the lock first would
// price a door for somebody who cannot touch it — which is a map of the
// dungeon's locks, free, from anywhere.
func (s *DoorReachSuite) TestAFarHandIsToldItCannotReachRatherThanWhatTheLockCosts() {
	far := s.edgeDoorAt(encounter.DoorIsLocked(oneApproachLock(reachDC)), authoredAt(0, reachRow))

	_, err := far.OpenDoor(&encounter.OpenDoorInput{Door: theDoor, Actor: alice})
	s.Require().ErrorIs(err, encounter.ErrOutOfRange)
	s.NotErrorIs(err, encounter.ErrLocked, "the lock is not what a hand across the room hears about")
	s.NotContains(err.Error(), "17", "and its price is not in the sentence either")

	near := s.edgeDoorAt(encounter.DoorIsLocked(oneApproachLock(reachDC)), authoredAt(1, reachRow))

	_, locked := near.OpenDoor(&encounter.OpenDoorInput{Door: theDoor, Actor: alice})
	s.Require().ErrorIs(locked, encounter.ErrLocked, "in reach, the lock is the refusal — and it prices itself")
	s.Contains(locked.Error(), "17")
}

// TestTheProbeLawStillOutranksReach is the other half of the order, and the
// one a leak would be worse for: a hidden door nobody has found answers
// "no such door" BYTE-IDENTICALLY whether the guesser is beside it or across
// the room.
//
// If reach ran first, "out of range" from across the room would confirm that
// something is there to be out of range OF — the exact inference the probe
// law exists to deny (rpg-project#350).
func (s *DoorReachSuite) TestTheProbeLawStillOutranksReach() {
	field := doorField(3, encounter.DoorIsClosed(), theDoor, reachRow)
	field.Concealments = []encounter.ConcealmentInput{{
		ID:     "the-secret",
		Checks: []encounter.CheckApproach{{Ability: "perception", DC: 15}},
		Doors:  []encounter.DoorID{theDoor},
	}}

	answers := make([]string, 0, 2)
	for _, seat := range []spatial.Position{authoredAt(1, reachRow), authoredAt(0, reachRow)} {
		enc, err := encounter.NewEncounter(&encounter.SetupInput{
			Sight:     everyoneSeesTheWholeMap{},
			Equipment: noHandsAreObserved{}, Standing: everyoneStanding{}, Initiative: orderAsGiven{},
			TurnDriver: passDriver{}, Striker: passStriker{}, Mover: quietMover{}, Announcer: quietAnnouncer{},
			CheckResolver: findsNothing{}, Witness: nobodyPerceives{},
			Field:   field,
			Members: []encounter.MemberInput{{ID: alice, Kind: encounter.KindPlayer, Position: seat}},
			Endings: []encounter.EndingInput{{Key: "withdrawn", Trigger: encounter.TriggerExternal{}}},
		})
		s.Require().NoError(err)

		_, probed := enc.OpenDoor(&encounter.OpenDoorInput{Door: theDoor, Actor: alice})
		s.Require().ErrorIs(probed, encounter.ErrNoDoor)
		s.Require().NotErrorIs(probed, encounter.ErrOutOfRange)
		answers = append(answers, probed.Error())
	}

	s.Equal(answers[0], answers[1], "where the guesser stood cannot be read off the answer")
}

// TestAnAuthoredChangeWithNoActorReachesFromNowhere is the empty-actor
// contract, unchanged: the host's own hand — the side of the seam that
// composed the dungeon — moves a door without standing anywhere.
//
// Pinned rather than merely true, because it is the one door a reach rule
// could quietly close: every scene in footprintdoors_test.go drives these
// verbs this way, and a rule that refused an absent actor would have no
// position to refuse them from.
func (s *DoorReachSuite) TestAnAuthoredChangeWithNoActorReachesFromNowhere() {
	enc := s.edgeDoorAt(encounter.DoorIsClosed(), authoredAt(0, reachRow))

	out, err := enc.OpenDoor(&encounter.OpenDoorInput{Door: theDoor})
	s.Require().NoError(err, "nobody is named, so there is nobody to be too far away")
	s.Equal(encounter.DoorOpen, out.State)
}

// stateOf reads one door's live state off the encounter.
func (s *DoorReachSuite) stateOf(enc *encounter.Encounter, id encounter.DoorID) encounter.DoorStateKind {
	for _, door := range enc.Doors() {
		if door.ID == id {
			return door.State.Kind()
		}
	}
	s.Require().FailNowf("no such door", "%q is not in this encounter", id)

	return ""
}
