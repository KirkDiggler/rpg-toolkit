// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package encounter_test

import (
	"errors"

	"github.com/stretchr/testify/require"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter"
	"github.com/KirkDiggler/rpg-toolkit/tools/spatial"
)

// testworld_test.go is the shared world-clock fixture kit these tests used to
// get from pump_test.go, which went with [encounter.Pump] (rpg-project#465).
//
// TIME PASSES BECAUSE SOMEBODY ACTS, so a fixture that wants a round of the
// world makes a member act. There is no tick verb to call any more, and there
// deliberately is not one: "advances only because players act" is the rule,
// and a test helper that reached past it would be testing a world this engine
// does not have.

// openDoorway is an OPEN door standing in one crossing — what a connection
// used to be (rpg-project#256): a doorway is the door standing in it. Both
// cells are authored [col,row] pairs, converted through the one conversion.
func openDoorway(id encounter.DoorID, fromCol, fromRow, toCol, toRow int) encounter.DoorInput {
	return encounter.DoorInput{
		ID:    id,
		Edges: []encounter.DoorEdge{{From: cellAt(fromCol, fromRow), To: cellAt(toCol, toRow)}},
		State: encounter.DoorIsOpen(),
	}
}

// twoRoomDoor is the standard fixture doorway: an OPEN door standing in the
// crossing from room-a's [9,5] to room-b's [10,5] — adjacent cells (W3) in two
// regions whose footprints (x:[0,9] vs x:[10,19]) stay disjoint (W2).
var twoRoomDoor = openDoorway("door1", 9, 5, 10, 5)

// twoRoomWall is room-a's east wall, open at the doorway's row. Every edge has
// one endpoint in room-a and one in room-b, which is a sentence no room could
// say before the field became one canvas (rpg-toolkit#1106) — and one these
// fixtures now have to say, because with one canvas and no wall the two
// chambers share an open seam ten cells wide.
func twoRoomWall() []encounter.WallInput { return squareSeamWall(9, 10, 5) }

// twoRoomSealedWall is the same wall with NO opening — for the fixtures that
// declare no doorway at all. Two chambers side by side with nothing joining
// them is a solid partition, and saying so is now the fixture's job: it used to
// be implied by their being different rooms.
func twoRoomSealedWall() []encounter.WallInput { return squareSeamWall(9, 10) }

// tableDriver is the [encounter.Driver] a fixture installs when it wants a
// creature to actually DO what its table says.
//
// rollsLowest, so the FIRST eligible entry always fires: these fixtures assert
// on what the table DECIDED, and a shuffled pick would make every assertion
// depend on a roll nobody is testing. A fixture whose point IS the roll
// installs its own die.
func tableDriver() encounter.TableDriver {
	return encounter.TableDriver{Roller: rollsLowest{}}
}

// walksTo is the table a fixture gives a creature it wants walking to one
// authored cell whenever it has time — the standing order the retired
// patrol decider used to be, said the way an author says it.
func walksTo(cell spatial.Position) encounter.Table {
	at := cell

	return encounter.Table{
		encounter.AnswerTime: {
			{Weight: 1, Toward: &encounter.Selector{At: &at}},
		},
	}
}

// hunts is the table a fixture gives a creature that closes on whoever it is
// opposed to: the one in sight, else the last place it saw one. The retired
// pursuit decider, with the coordinate arithmetic left to the engine that
// owns the walls.
func hunts() encounter.Table {
	return encounter.Table{
		encounter.AnswerTime: {
			{Weight: 1, When: &encounter.When{Enemy: encounter.EnemySeen},
				Toward: &encounter.Selector{Word: encounter.SelectorEnemy}},
			{Weight: 1, When: &encounter.When{Enemy: encounter.EnemyRemembered},
				Toward: &encounter.Selector{Word: encounter.SelectorEnemy}},
			{Weight: 1, Hold: true},
		},
	}
}

// aRound makes one unit of time pass, on whichever clock the scene is
// actually running — the fixture shape that replaced [encounter.Pump]
// (rpg-project#465).
//
// THERE IS NO TICK VERB ANY MORE, and deliberately not: the world advances
// only because somebody acts. So this picks somebody and has them act.
//
//   - Somebody on the WORLD clock searches the region they stand in. Search is
//     universally attemptable, needs no target and no die, and is priced as an
//     action — so it pays one round, the world thinks, and every standing
//     creature out there spends its budget.
//   - Nobody on the world clock means everybody is in the fight, and the unit
//     of time there is the turn: the active member ends theirs, which wraps
//     the round on its way round the order and pays the world for it.
//
// Returns whatever the verb returned, so a fixture that only wants "and then
// time passed" keeps the two-value call shape it already had.
func aRound(enc *encounter.Encounter) (*encounter.SearchOutput, error) {
	members, err := enc.Members()
	if err != nil {
		return nil, err
	}

	var actor encounter.Member
	var bubbled encounter.Member
	for _, m := range members {
		on, cerr := enc.ClockOf(&encounter.ClockOfInput{Member: m.ID})
		if cerr != nil {
			return nil, cerr
		}
		if on.Kind == encounter.ClockTurn {
			if bubbled.ID == "" && on.Active == m.ID {
				bubbled = m
			}
			continue
		}
		if actor.ID == "" || (actor.Kind != encounter.KindPlayer && m.Kind == encounter.KindPlayer) {
			actor = m
		}
	}

	if actor.ID != "" {
		return enc.Search(&encounter.SearchInput{Member: actor.ID, Region: actor.Region})
	}
	if bubbled.ID == "" {
		return nil, errNobodyToActFor
	}
	_, err = enc.EndTurn(&encounter.EndTurnInput{Member: bubbled.ID})

	return &encounter.SearchOutput{}, err
}

// errNobodyToActFor is what aRound answers for a scene with nobody in it.
var errNobodyToActFor = errors.New("fixture: nobody is on any clock to make time pass")

// whereIs is where a member is standing right now, off the roster — what a
// fixture asserts against when what it cares about is that somebody moved.
func whereIs(t require.TestingT, enc *encounter.Encounter, who encounter.MemberID) spatial.Position {
	members, err := enc.Members()
	require.NoError(t, err)
	for _, m := range members {
		if m.ID == who {
			return m.Position
		}
	}
	require.Fail(t, "no such member", "%q is not on the roster", who)

	return spatial.Position{}
}
