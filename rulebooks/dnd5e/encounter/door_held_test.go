// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package encounter

// door_held_test.go pins A HELD DOOR REFUSES ITS VERBS (rpg-project#835 R7):
// OpenDoor, CloseDoor and Unlock on a door a post holds are refused to any
// actor not allied with a holder, by the same hold read the crossing makes;
// the holder and his allies work the door as before.

import (
	"testing"

	"github.com/stretchr/testify/require"
)

// heldLock is the leaf's lock when a test locks it.
var heldLock = Lock{Approaches: []CheckApproach{{Ability: "dex", DC: 13}}}

// heldGateHall is the hold hall with the leaf in a state, alice beside it on
// the south side, and the guard posted on the south-west cell, on alice's
// side of the door, so a shut door does not hide him.
func heldGateHall(t *testing.T, state DoorState, members ...MemberInput) *Encounter {
	t.Helper()
	guard := holder(t, postedGuard, holdSouthWest, holdLeaf, holdSouthWest)

	return holdHall(t, holdLeafDoor(state), holdSouthEast, append([]MemberInput{guard}, members...)...)
}

// requireDoorHeld requires a door verb's refusal is the held door's, naming
// who the actor is told bars it.
func requireDoorHeld(t *testing.T, err error, public string) {
	t.Helper()
	require.ErrorIs(t, err, ErrDoorHeld)
	require.ErrorIs(t, err, ErrBodyInTheWay, "a held door is a body in the way to a host that maps that")
	var held *DoorHeldError
	require.ErrorAs(t, err, &held)
	require.Equal(t, holdLeaf, held.Door)
	require.Equal(t, public, held.PublicReason())
}

func TestAStrangerCannotOpenAHeldDoor(t *testing.T) {
	enc := heldGateHall(t, DoorIsClosed())
	_, err := enc.OpenDoor(&OpenDoorInput{Door: holdLeaf, Actor: "alice"})
	requireDoorHeld(t, err, "Gate guard bars the door")
	require.Equal(t, DoorClosed, enc.doorsByID[holdLeaf].state.Kind(), "and the door stays shut")
	requireDoorHeld(t, enc.DoorHeldAgainst(holdLeaf, "alice"), "Gate guard bars the door")
}

func TestAnUnseenHolderBarsTheDoorAsSomeone(t *testing.T) {
	guard := holder(t, postedGuard, holdSouthWest, holdLeaf, holdSouthWest)
	enc, err := NewEncounter(holdSetup(wakeBlind{}, holdLeafDoor(DoorIsClosed()), holdSouthEast, guard))
	require.NoError(t, err)
	_, err = enc.OpenDoor(&OpenDoorInput{Door: holdLeaf, Actor: "alice"})
	requireDoorHeld(t, err, "someone bars the door")
}

func TestTheHolderAndAnAllyWorkAHeldDoor(t *testing.T) {
	porter := MemberInput{ID: "porter", Kind: KindMonster, Faction: postedGarrison, Position: holdNorthEast, Table: holds()}
	enc := heldGateHall(t, DoorIsClosed(), porter)
	require.NoError(t, enc.DoorHeldAgainst(holdLeaf, postedGuard), "nothing is held against the holder")
	_, err := enc.OpenDoor(&OpenDoorInput{Door: holdLeaf, Actor: postedGuard})
	require.NoError(t, err, "the holder opens his own door")
	_, err = enc.CloseDoor(&CloseDoorInput{Door: holdLeaf, Actor: "porter"})
	require.NoError(t, err, "and his ally shuts it")
	_, err = enc.OpenDoor(&OpenDoorInput{Door: holdLeaf, Actor: "porter"})
	require.NoError(t, err, "and opens it")
}

func TestAStrangerCannotCloseAHeldDoor(t *testing.T) {
	enc := heldGateHall(t, DoorIsOpen())
	_, err := enc.CloseDoor(&CloseDoorInput{Door: holdLeaf, Actor: "alice"})
	requireDoorHeld(t, err, "Gate guard bars the door")
	require.Equal(t, DoorOpen, enc.doorsByID[holdLeaf].state.Kind(), "and the door stays open")
}

func TestAHeldLockTakesNoAttempt(t *testing.T) {
	enc := heldGateHall(t, DoorIsLocked(heldLock))
	requireDoorHeld(t, enc.DoorHeldAgainst(holdLeaf, "alice"), "Gate guard bars the door")
	before := enc.ToData()

	_, err := enc.Unlock(&UnlockInput{Door: holdLeaf, Actor: "alice", Beaten: true, Total: 20, Applied: heldLock.Approaches[0]})
	requireDoorHeld(t, err, "Gate guard bars the door")
	require.Equal(t, DoorLocked, enc.doorsByID[holdLeaf].state.Kind(), "a beaten lock on a held door is not opened")
	after := enc.ToData()
	require.Equal(t, before.Clock.HighWater, after.Clock.HighWater, "no world round is spent on an attempt that never happened")
	require.Equal(t, len(before.Journal.Facts), len(after.Journal.Facts), "and nothing is written")
}

func TestAHeldDoorOpensWhenTheHoldIsEmptied(t *testing.T) {
	for name, empty := range map[string]func(*testing.T, *Encounter, postedDown){
		"cowed":     func(t *testing.T, enc *Encounter, _ postedDown) { cowGuard(t, enc) },
		"drawn off": func(t *testing.T, enc *Encounter, _ postedDown) { placeAt(t, enc, postedGuard, holdOffPost) },
		"downed":    func(_ *testing.T, _ *Encounter, down postedDown) { down[postedGuard] = true },
	} {
		t.Run(name, func(t *testing.T) {
			down := postedDown{}
			in := holdSetup(everyoneSeesTheWholeMap{}, holdLeafDoor(DoorIsClosed()), holdSouthEast,
				holder(t, postedGuard, holdSouthWest, holdLeaf, holdSouthWest))
			in.Participation = down
			enc, err := NewEncounter(in)
			require.NoError(t, err)
			requireDoorHeld(t, enc.DoorHeldAgainst(holdLeaf, "alice"), "Gate guard bars the door")

			empty(t, enc, down)
			_, err = enc.OpenDoor(&OpenDoorInput{Door: holdLeaf, Actor: "alice"})
			require.NoError(t, err, "%s, he holds nothing and the door opens", name)
		})
	}
}
