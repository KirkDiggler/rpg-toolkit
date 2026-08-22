// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package encounter_test

// pursuit_test.go is rpg-toolkit#1044's payoff pin: a monster hunts a player
// through a doorway while NEITHER ROOM SITS AT THE ORIGIN.
//
// That is the entire design of the fixture. Everywhere else in this package a
// corridor is anchored at (0,0), which makes its local coordinates and its
// absolute ones the same numbers — so a decider reasoning in the wrong frame,
// a snapshot built in the wrong frame, and a sight payload projected in the
// wrong frame all agree with the right ones, and every assertion passes.
// Anchor both rooms away from the origin and there is no frame a mistake can
// hide in.

import (
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/KirkDiggler/rpg-toolkit/core"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter"
	"github.com/KirkDiggler/rpg-toolkit/tools/spatial"
)

type PursuitSuite struct {
	suite.Suite

	enc *encounter.Encounter
}

func TestPursuitSuite(t *testing.T) {
	suite.Run(t, new(PursuitSuite))
}

const (
	hunted  = core.EntityID("alice")
	hunter  = core.EntityID("goblin")
	westID  = "west"
	eastID  = "east"
	gateway = "gateway"
)

// Both rooms are 6x6 and neither is anchored at the origin. The doorway joins
// west-local (5,3) — absolute (25,13) — to east-local (0,3) — absolute
// (26,13). Adjacent (W3), footprints disjoint (W2), and no coordinate in this
// field is its own local twin.
var (
	westOrigin = spatial.Position{X: 20, Y: 10}
	eastOrigin = spatial.Position{X: 26, Y: 10}

	westThreshold = spatial.Position{X: 25, Y: 13}
	eastThreshold = spatial.Position{X: 26, Y: 13}

	// Just inside the east chamber and off the doorway's row: the wall is
	// between it and the goblin's cell, which is the only thing in this fixture
	// that can hide anybody.
	eastCorner = spatial.Position{X: 27, Y: 14}
)

// pursuitSeamWall is the west chamber's east wall, open at the doorway row.
// Without it the two chambers share an open edge six cells wide and there is
// nothing for anybody to hide behind — which is the honest consequence of the
// field being one canvas (rpg-toolkit#1106), and the reason this fixture has to
// say where its walls are instead of relying on a room boundary to imply them.
func pursuitSeamWall() []spatial.Boundary { return squareSeamWall(5, 6, 3) }

func (s *PursuitSuite) SetupTest() {
	field := encounter.FieldInput{
		Canvas: encounter.CanvasInput{Void: encounter.VoidIsOpaque()},
		Rooms: []encounter.RoomInput{
			{ID: westID, Width: 6, Height: 6, Origin: westOrigin,
				Boundaries: pursuitSeamWall()},
			{ID: eastID, Width: 6, Height: 6, Origin: eastOrigin},
		},
		Connections: []encounter.ConnectionInput{{
			ID: gateway, From: westID, To: eastID,
			FromPosition: spatial.Position{X: 5, Y: 3},
			ToPosition:   spatial.Position{X: 0, Y: 3},
		}},
	}

	enc, err := encounter.NewEncounter(&encounter.SetupInput{
		Sight: everyoneSeesTheWholeMap{}, Standing: everyoneStanding{}, Initiative: orderAsGiven{}, TurnDriver: passDriver{},
		Field: field,
		Members: []encounter.MemberInput{
			// Alice stands at the threshold; the goblin is across the room
			// with a clear line to her, so first light makes them mutual.
			{ID: hunted, Kind: encounter.KindPlayer, Room: westID,
				Position: spatial.Position{X: 5, Y: 3}},
			{ID: hunter, Kind: encounter.KindMonster, Room: westID,
				Position: spatial.Position{X: 1, Y: 3},
				Decider:  &pursuitDecider{doorways: doorwaysFrom(field), target: hunted}},
		},
		Endings: []encounter.EndingInput{{Key: "done", Trigger: encounter.TriggerExternal{}}},
	})
	s.Require().NoError(err)
	s.enc = enc
}

// sightOf decodes what one member currently believes about another.
func (s *PursuitSuite) sightOf(observer, subject core.EntityID) (string, spatial.Position) {
	status, payload := seen(s.T(), s.enc, observer, subject)
	return string(status), spatial.Position{X: payload.X, Y: payload.Y}
}

// TestTheHuntCrossesTheDoorwayWithoutKnowingItIsOne is the whole slice in one
// scene.
//
// The decider is told a target and a list of doorway cell pairs, and nothing
// else — no rooms, no local coordinates, no crossing intent. It compares two
// absolute cells, subtracts, and steps. Going through the threshold happens
// because the far side is the next cell along, which is W3 stated as geometry
// rather than as a special case.
//
// A DOORWAY IS A WINDOW NOW, and that is what changed here (rpg-toolkit#1106).
// Alice used to vanish the instant she crossed, because sight stopped at a room
// boundary; she stays visible standing IN the opening, exactly as she would at
// a table, and it takes the wall beside it to hide her.
func (s *PursuitSuite) TestTheHuntCrossesTheDoorwayWithoutKnowingItIsOne() {
	status, at := s.sightOf(hunter, hunted)
	s.Require().Equal("current", status, "first light: the goblin sees her across the room")
	s.Equal(westThreshold, at, "and sees her ON THE MAP, at the west room's threshold cell")

	// Contact started a fight, as it must — sight is what forms a bubble, and
	// nobody asked for one. The party disengages, which returns both to the
	// world clock and touches nothing either of them knows: the goblin keeps
	// its sighting, which is what makes the hunt possible at all.
	_, err := s.enc.Dissolve(&encounter.DissolveInput{Member: hunted})
	s.Require().NoError(err, "the fight ends by decision, and the memory of it does not")

	// She steps through the opening — one step, to the cell on the other side.
	out, err := s.enc.Step(&encounter.StepInput{Member: hunted, To: eastThreshold})
	s.Require().NoError(err)
	s.Equal(gateway, out.Crossing, "the doorway is named, and decides nothing")

	status, at = s.sightOf(hunter, hunted)
	s.Require().Equal("current", status, "standing in the opening, she is still in plain sight")
	s.Equal(eastThreshold, at, "on the far side of it, on the same map")

	// And then out of its line, behind the wall the west chamber drew along
	// its own edge. NOW she is a memory.
	_, err = s.enc.Step(&encounter.StepInput{Member: hunted, To: eastCorner})
	s.Require().NoError(err)

	status, ghost := s.sightOf(hunter, hunted)
	s.Require().Equal("held", status, "the wall takes her; the sighting becomes a memory")
	s.Equal(eastThreshold, ghost, "the ghost stands where she was last seen — in the doorway")

	// The pump: the goblin walks to the ghost's cell, which is on the far side
	// of the doorway. The SAME intent type carries it — there is no crossing
	// intent, and no crossing mechanism for one to name.
	first, err := s.enc.Pump(&encounter.PumpInput{})
	s.Require().NoError(err)
	s.Require().Len(first.MonsterMoves, 1, "it walks toward the last place it saw her")
	s.Equal(hunter, first.MonsterMoves[0].Member)
	s.Equal(eastThreshold, first.MonsterMoves[0].To, "through the doorway, in one ordinary step")

	// And it can see her again, from the chamber it has just entered.
	status, found := s.sightOf(hunter, hunted)
	s.Equal("current", status, "the hunt closes: she is in sight again")
	s.Equal(eastCorner, found, "in the corner she ran to, on the same map")
}

// TestAStepIntoTheVoidIsRefusedInSilence and its sibling below pin the two
// ways stepTo declines without complaining. Both matter because the pump must
// survive a decider that asks for something impossible: an error would abort
// the tick for every other monster in the encounter.
func (s *PursuitSuite) TestAStepIntoTheVoidIsRefusedInSilence() {
	decider := &onceStepDecider{to: spatial.Position{X: 500, Y: 500}}
	s.freeRoamWith(decider)

	out, err := s.enc.Pump(&encounter.PumpInput{})
	s.Require().NoError(err, "an impossible step is not an error")
	s.True(decider.called, "and the refusal happened in stepTo, not by never being asked")
	s.Empty(out.MonsterMoves)
	s.Equal(spatial.Position{X: 21, Y: 13}, s.whereIs(hunter), "so nobody moved")
}

// TestAStepThroughTheWallIsRefused is the case coordinates alone cannot rule
// out: an absolutely-adjacent cell is not automatically a step you may take,
// because something may be standing between the two.
//
// That something used to be the absence of a doorway in the connection list;
// it is a wall on the map now (rpg-toolkit#1106), which is a thing the Atlas
// draws and a host can show a player.
func (s *PursuitSuite) TestAStepThroughTheWallIsRefused() {
	// The east chamber's cell one row ABOVE the doorway: adjacent to the west
	// chamber's own (25,12) in absolute space, with the seam wall between them.
	decider := &onceStepDecider{to: spatial.Position{X: 26, Y: 12}}
	s.freeRoamWith(decider)

	out, err := s.enc.Pump(&encounter.PumpInput{})
	s.Require().NoError(err, "walking into a wall is not an error either")
	s.True(decider.called, "and it really was asked")
	s.Empty(out.MonsterMoves, "the wall refused the step")
	s.Equal(spatial.Position{X: 21, Y: 13}, s.whereIs(hunter), "so the goblin stayed put")
}

// freeRoamWith puts the hunter back on the world clock with the given decider,
// which is what a refusal pin needs before it means anything.
//
// Both halves are load-bearing. The DISSOLVE matters because first light
// starts a fight and Pump only consults world-clock monsters — without it a
// "nobody moved" assertion passes because nobody was ever asked, which is a
// test that cannot fail. Both refusal pins therefore also assert the decider
// was called. The RELOAD is how a decider is swapped at all: they are
// behavior, re-registered at load, never persisted.
func (s *PursuitSuite) freeRoamWith(decider encounter.Decider) {
	_, err := s.enc.Dissolve(&encounter.DissolveInput{Member: hunter})
	s.Require().NoError(err)

	data := s.enc.ToData()
	enc, err := encounter.LoadEncounter(&encounter.LoadEncounterInput{
		Sight: everyoneSeesTheWholeMap{}, Standing: everyoneStanding{}, Initiative: orderAsGiven{}, TurnDriver: passDriver{},
		Data:     data,
		Deciders: map[encounter.MemberID]encounter.Decider{hunter: decider},
	})
	s.Require().NoError(err)
	s.enc = enc
}

// whereIs is the member's cell on the map, read from the roster.
func (s *PursuitSuite) whereIs(member core.EntityID) spatial.Position {
	members, err := s.enc.Members()
	s.Require().NoError(err)
	for _, m := range members {
		if m.ID == member {
			return m.Position
		}
	}
	s.Require().Fail("not a member", "%s", member)
	return spatial.Position{}
}
