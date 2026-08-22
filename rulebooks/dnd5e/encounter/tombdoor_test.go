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

// tombdoor_test.go is THE NAMED CASE (rpg-toolkit#1123's done-when, and the
// census's): the reference tomb's DC-12 connector blocks sight until it is
// beaten, and then reveals the boss chamber.
//
// It is the one scene this slice exists to make possible, and it is here as its
// own file because it is an END-TO-END pin rather than a unit: authored locked,
// blind through it, a failed attempt that leaves it recoverable, a beaten one
// that opens it — and the boss coming into view, with a fight forming on the
// sight of him the way one forms on any other.
//
// The old stack drives the same loop in encounter/locked_connector_test.go
// against a DC-15 generated connector; this is that test's shape ported onto a
// door that is a set of edges rather than a cell.
type TombDoorSuite struct {
	suite.Suite

	enc *encounter.Encounter
}

func TestTombDoorSuite(t *testing.T) {
	suite.Run(t, new(TombDoorSuite))
}

const (
	tombdoorEntrance = "entrance"
	tombdoorHall     = "hall"
	tombdoorCrypt    = "crypt"

	// The connector the reference tomb authors as `locked: { dc: 12, ability: dex }`.
	cryptDoor = "crypt-door"
	cryptDC   = 12

	delve = core.EntityID("delve")
	wight = core.EntityID("wight")
)

// The chain, walled at both seams, with an open gap at the first and the locked
// connector at the second. Nothing sits at the canvas origin by accident: the
// footprints tile it exactly, so no assertion here is really about void.
//
//	x: 0..5 entrance | 6..15 hall | 16..27 crypt
//	seam 1 at 5|6 (open doorway, row 3)   seam 2 at 15|16 (the DC-12 door, row 3)
const tombdoorRow = 3

var (
	// delve stands in the hall with her hand on the door; the wight is one
	// cell beyond it, in the crypt.
	delveCell = spatial.Position{X: 15, Y: tombdoorRow}
	wightCell = spatial.Position{X: 16, Y: tombdoorRow}
)

func (s *TombDoorSuite) SetupTest() {
	enc, err := encounter.NewEncounter(&encounter.SetupInput{
		Sight: everyoneSeesTheWholeMap{}, Standing: everyoneStanding{}, Initiative: orderAsGiven{}, TurnDriver: passDriver{},
		Field: encounter.FieldInput{
			Canvas: encounter.CanvasInput{Void: encounter.VoidIsOpaque()},
			Rooms: []encounter.RoomInput{
				{ID: tombdoorEntrance, Width: 6, Height: 8, Origin: spatial.Position{X: 0, Y: 0},
					Boundaries: seamWallExcept(5, 8, tombdoorRow)},
				{ID: tombdoorHall, Width: 10, Height: 8, Origin: spatial.Position{X: 6, Y: 0},
					Boundaries: seamWallExcept(9, 8, tombdoorRow)},
				{ID: tombdoorCrypt, Width: 12, Height: 8, Origin: spatial.Position{X: 16, Y: 0}},
			},
			Doors: []encounter.DoorInput{{
				ID:    cryptDoor,
				Edges: doorEdgesAcross(15, tombdoorRow),
				State: encounter.DoorIsLocked(encounter.Lock{DC: cryptDC, Ability: "dex", Tool: "dnd5e:item:thieves-tools"}),
			}},
		},
		Members: []encounter.MemberInput{
			{ID: delve, Kind: encounter.KindPlayer, Room: tombdoorHall,
				Position: spatial.Position{X: 9, Y: tombdoorRow}},
			{ID: wight, Kind: encounter.KindMonster, Room: tombdoorCrypt,
				Position: spatial.Position{X: 0, Y: tombdoorRow}},
		},
		Endings: []encounter.EndingInput{{Key: "withdrawn", Trigger: encounter.TriggerExternal{}}},
	})
	s.Require().NoError(err)
	s.enc = enc
}

// picksTheLock is where the RULE lives, and it lives here on purpose.
//
// "A check that meets the DC succeeds" is D&D 5e — a tie goes to the roller —
// and [Encounter.Unlock] is not allowed to know it. It is told whether the lock
// was beaten and does as it is told (Kirk: "I agree on rules leaking in we need
// to be diligent"). So the comparison sits in this fixture, standing in for the
// rulebook seam that will make it for real, and the scene below still reads as
// dice against a difficulty because that is what is happening — just not inside
// the composition.
//
// A system where ties fail, or where a natural 1 fails regardless, changes this
// function and nothing in the module. That is the whole test of whether the
// seam is in the right place.
func picksTheLock(total, dc int) bool { return total >= dc }

func (s *TombDoorSuite) sees(observer, subject core.EntityID) bool {
	view, err := s.enc.View(&encounter.ViewInput{Member: observer})
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

// TestTheLockedConnectorBlocksSightUntilItIsBeaten is the whole loop.
func (s *TombDoorSuite) TestTheLockedConnectorBlocksSightUntilItIsBeaten() {
	s.Require().Equal(delveCell, s.memberCell(delve), "she is at the door")
	s.Require().Equal(wightCell, s.memberCell(wight), "and he is one cell beyond it")

	s.False(s.sees(delve, wight), "the connector is locked; the crypt is dark to her")
	s.False(s.sees(wight, delve), "and she is hidden from him — geometry is mutual")

	_, err := s.enc.Step(&encounter.StepInput{Member: delve, To: wightCell})
	s.Require().ErrorIs(err, encounter.ErrBadPlacement, "and she cannot walk through it")
	s.Contains(err.Error(), cryptDoor)

	_, err = s.enc.OpenDoor(&encounter.OpenDoorInput{Door: cryptDoor})
	s.Require().ErrorIs(err, encounter.ErrLocked, "nor simply open it")
	s.Contains(err.Error(), "DC 12", "and the refusal says what it would take")

	// A FAILED ATTEMPT LEAVES IT RECOVERABLE — the half the old stack's
	// locked_connector_test pins, and the half a state machine gets wrong.
	const missed, met = cryptDC - 1, cryptDC

	failed, err := s.enc.Unlock(&encounter.UnlockInput{
		Door: cryptDoor, Beaten: picksTheLock(missed, cryptDC)})
	s.Require().NoError(err, "a failed check is an outcome, not an error")
	s.False(failed.Beaten)
	s.Equal(cryptDC, failed.DC, "and it reports the DC it carries, for whoever narrates the near miss")
	s.Equal(encounter.DoorLocked, failed.State, "still locked")
	s.False(s.sees(delve, wight), "and still blind")

	beaten, err := s.enc.Unlock(&encounter.UnlockInput{
		Door: cryptDoor, Beaten: picksTheLock(met, cryptDC)})
	s.Require().NoError(err)
	s.True(beaten.Beaten, "meeting the DC exactly beats it — a tie goes to the roller, per picksTheLock")
	s.Equal(encounter.DoorOpen, beaten.State, "beaten means open, not merely unlocked")

	s.True(s.sees(delve, wight), "THE BOSS CHAMBER IS REVEALED")
	s.True(s.sees(wight, delve), "and he sees what woke him")
	s.NotNil(beaten.Formed, "a player and a monster in plain sight of each other is a fight")

	// The door is unlocked now, not merely open: shutting it gives an ordinary
	// closed door, and nothing re-locks behind the party.
	_, err = s.enc.CloseDoor(&encounter.CloseDoorInput{Door: cryptDoor})
	s.Require().NoError(err)
	s.Equal(encounter.DoorClosed, s.doorState(cryptDoor), "closed, and not locked again")
}

// TestTheOpenDoorwayAtTheOtherSeamIsUnaffected is the control: the first seam
// has a gap and no door, which is what every opening in this composition was
// before this slice. It still works, and no verb can touch it.
func (s *TombDoorSuite) TestTheOpenDoorwayAtTheOtherSeamIsUnaffected() {
	out, err := s.enc.Step(&encounter.StepInput{Member: delve, To: spatial.Position{X: 5, Y: tombdoorRow}})
	s.Require().NoError(err, "the gap at the first seam is a step like any other")
	s.Empty(out.Doors, "and no door stands in it")

	_, err = s.enc.OpenDoor(&encounter.OpenDoorInput{Door: "entrance-door"})
	s.Require().ErrorIs(err, encounter.ErrNoDoor, "there is nothing there to open")
}

// TestTheDoorSurvivesASave round-trips the connector mid-scene: beaten and open
// on one side of a save, beaten and open on the other.
func (s *TombDoorSuite) TestTheDoorSurvivesASave() {
	s.Run("locked, with its DC and its opaque refs", func() {
		data := s.enc.ToData()
		s.Require().Len(data.Doors, 1)
		s.Equal(string(encounter.DoorLocked), data.Doors[0].State)
		s.Require().NotNil(data.Doors[0].Lock)
		s.Equal(cryptDC, data.Doors[0].Lock.DC)
		s.Equal("dex", data.Doors[0].Lock.Ability, "carried verbatim; this module never looks inside it")
		s.Equal("dnd5e:item:thieves-tools", data.Doors[0].Lock.Tool)

		back := s.reload(data)
		_, err := back.OpenDoor(&encounter.OpenDoorInput{Door: cryptDoor})
		s.Require().ErrorIs(err, encounter.ErrLocked, "a reloaded lock is still a lock")
		s.Contains(err.Error(), "DC 12", "at the DC it was saved with")
	})

	s.Run("and open, once it has been beaten", func() {
		_, err := s.enc.Unlock(&encounter.UnlockInput{Door: cryptDoor, Beaten: picksTheLock(30, cryptDC)})
		s.Require().NoError(err)

		data := s.enc.ToData()
		s.Equal(string(encounter.DoorOpen), data.Doors[0].State)
		s.Nil(data.Doors[0].Lock, "a beaten lock is gone, not carried along at zero")

		back := s.reload(data)

		var reloaded encounter.Door
		for _, d := range back.Doors() {
			if d.ID == cryptDoor {
				reloaded = d
			}
		}
		s.Require().Equal(encounter.DoorID(cryptDoor), reloaded.ID)
		s.Equal(encounter.DoorOpen, reloaded.State.Kind(), "a reloaded open door is open")
		s.Equal(doorEdgesAcross(15, tombdoorRow), reloaded.Edges, "standing where it stood")

		// Asked of the MAP rather than by stepping through it, because by now
		// the door has done its job: opening it put the wight in plain sight
		// and a fight formed, and free roam is a world-clock verb that a
		// member in a bubble does not get. That gate is #964's and has nothing
		// to do with doors — but it is exactly the trap a "walk through it to
		// prove it is open" assertion falls into, so the probe is the canvas.
		canvas, err := back.Canvas()
		s.Require().NoError(err)
		s.False(canvas.IsLineOfSightBlocked(delveCell, wightCell),
			"and blocking nothing, on the reloaded map itself")
	})
}

func (s *TombDoorSuite) reload(data encounter.EncounterData) *encounter.Encounter {
	back, err := encounter.LoadEncounter(&encounter.LoadEncounterInput{
		Data:  data,
		Sight: everyoneSeesTheWholeMap{}, Standing: everyoneStanding{}, Initiative: orderAsGiven{}, TurnDriver: passDriver{},
	})
	s.Require().NoError(err)

	return back
}

func (s *TombDoorSuite) memberCell(id core.EntityID) spatial.Position {
	members, err := s.enc.Members()
	s.Require().NoError(err)
	for _, m := range members {
		if m.ID == id {
			return m.Position
		}
	}
	s.Require().Fail("no such member", string(id))

	return spatial.Position{}
}

func (s *TombDoorSuite) doorState(id encounter.DoorID) encounter.DoorStateKind {
	for _, d := range s.enc.Doors() {
		if d.ID == id {
			return d.State.Kind()
		}
	}
	s.Require().Fail("no such door", id)

	return ""
}
