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
)

func (s *PursuitSuite) SetupTest() {
	field := encounter.FieldInput{
		Rooms: []encounter.RoomInput{
			{ID: westID, Width: 6, Height: 6, Origin: westOrigin},
			{ID: eastID, Width: 6, Height: 6, Origin: eastOrigin},
		},
		Connections: []encounter.ConnectionInput{{
			ID: gateway, From: westID, To: eastID,
			FromPosition: spatial.Position{X: 5, Y: 3},
			ToPosition:   spatial.Position{X: 0, Y: 3},
		}},
	}

	enc, err := encounter.NewEncounter(&encounter.SetupInput{
		Initiative: orderAsGiven{},
		Field:      field,
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
	_ = subject
	return string(status), spatial.Position{X: payload.X, Y: payload.Y}
}

// TestTheHuntCrossesTheDoorwayWithoutKnowingItIsOne is the whole slice in one
// scene.
//
// The decider is told a target and a list of doorway cell pairs, and nothing
// else — no rooms, no local coordinates, no traverse intent. It compares two
// absolute cells, subtracts, and steps. Crossing the threshold happens because
// the far side is the next cell along, which is W3 stated as geometry rather
// than as a special case.
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

	// She slips through. The goblin's sight of her fades to a ghost holding
	// her last-seen cell — the near side of the doorway, never the far side
	// it has not seen.
	_, err = s.enc.Traverse(&encounter.TraverseInput{Member: hunted, Connection: gateway})
	s.Require().NoError(err)

	status, ghost := s.sightOf(hunter, hunted)
	s.Require().Equal("held", status, "she left the room; the sighting becomes a memory")
	s.Equal(westThreshold, ghost, "the ghost stands where she was last seen")

	// First pump: the goblin walks to the ghost's cell — inside its own room,
	// so this is an ordinary move.
	first, err := s.enc.Pump(&encounter.PumpInput{})
	s.Require().NoError(err)
	s.Require().Len(first.MonsterMoves, 1, "it walks toward the last place it saw her")
	s.Empty(first.MonsterTraverses, "no doorway crossed yet — it was not standing at one")

	// Second pump: standing where she vanished, the only place left to look
	// is through the door. The SAME intent type carries it — there is no
	// traverse intent any more — and the composition works out that the cell
	// it named is on the other side of a doorway.
	second, err := s.enc.Pump(&encounter.PumpInput{})
	s.Require().NoError(err)
	s.Require().Len(second.MonsterTraverses, 1, "the hunt follows her through")
	s.Empty(second.MonsterMoves, "the crossing is the whole of this tick's movement")
	s.Equal(hunter, second.MonsterTraverses[0].Member)
	s.Equal(westID, second.MonsterTraverses[0].FromRoom)
	s.Equal(eastID, second.MonsterTraverses[0].ToRoom)

	// And it can see her again, in the room it has just entered.
	status, found := s.sightOf(hunter, hunted)
	s.Equal("current", status, "the hunt closes: she is in sight again")
	s.Equal(eastThreshold, found, "at the far side of the doorway, on the same map")
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
	s.Empty(out.MonsterTraverses)
	s.Equal(spatial.Position{X: 21, Y: 13}, s.whereIs(hunter), "so nobody moved")
}

// TestAStepIntoTheNextRoomWithoutADoorwayIsRefused is the case W2 makes
// possible and coordinates alone cannot rule out: two rooms may TOUCH without
// a door between them, so an absolutely-adjacent cell is not automatically a
// step you may take.
func (s *PursuitSuite) TestAStepIntoTheNextRoomWithoutADoorwayIsRefused() {
	// The east room's cell one row ABOVE the doorway: adjacent to the west
	// room's own (25,12) in absolute space, and joined to nothing.
	decider := &onceStepDecider{to: spatial.Position{X: 26, Y: 12}}
	s.freeRoamWith(decider)

	out, err := s.enc.Pump(&encounter.PumpInput{})
	s.Require().NoError(err, "an unjoined crossing is not an error either")
	s.True(decider.called, "and it really was asked")
	s.Empty(out.MonsterTraverses, "there is no doorway there to cross")
	s.Empty(out.MonsterMoves, "and it is not a move — the cell is in another room")
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
		Initiative: orderAsGiven{},
		Data:       data,
		Deciders:   map[encounter.MemberID]encounter.Decider{hunter: decider},
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
