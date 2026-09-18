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
	"encoding/json"
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

	// The verbs speak absolute AXIAL cells, so the authored pairs are
	// converted through the one conversion rather than written out.
	westThreshold = cellAt(25, 13)
	eastThreshold = cellAt(26, 13)

	// Just inside the east chamber and off the doorway's row: the wall is
	// between it and the goblin's cell, which is the only thing in this fixture
	// that can hide anybody.
	eastCorner = cellAt(27, 14)
)

// pursuitSeamWall is the west chamber's east wall, open at the doorway row.
// Without it the two chambers share an open edge six cells wide and there is
// nothing for anybody to hide behind — which is the honest consequence of the
// field being one canvas (rpg-toolkit#1106), and the reason this fixture has to
// say where its walls are instead of relying on a room boundary to imply them.
func pursuitSeamWall() []encounter.WallInput { return seamWallRows(25, 10, 6, 13) }

func (s *PursuitSuite) SetupTest() {
	field := encounter.FieldInput{
		Canvas: encounter.CanvasInput{Void: encounter.VoidIsOpaque(), Orientation: encounter.HexesArePointyTop()},
		Regions: []encounter.RegionInput{
			rectRegion(westID, int(westOrigin.X), int(westOrigin.Y), 6, 6),
			rectRegion(eastID, int(eastOrigin.X), int(eastOrigin.Y), 6, 6),
		},
		Walls: pursuitSeamWall(),
		Doors: []encounter.DoorInput{openDoorway(gateway, 25, 13, 26, 13)},
	}

	enc, err := encounter.NewEncounter(&encounter.SetupInput{
		Sight:     everyoneSeesTheWholeMap{},
		Equipment: noHandsAreObserved{}, Standing: everyoneStanding{}, Initiative: orderAsGiven{}, TurnDriver: tableDriver(), Striker: passStriker{}, Mover: quietMover{}, Announcer: quietAnnouncer{},
		Field: field,
		Members: []encounter.MemberInput{
			// Alice stands at the threshold; the goblin is across the room
			// with a clear line to her, so first light makes them mutual.
			{ID: hunted, Kind: encounter.KindPlayer, Position: spatial.Position{X: 25, Y: 13}},
			{ID: hunter, Kind: encounter.KindMonster, Position: spatial.Position{X: 21, Y: 13},
				SpeedFeet: 30, Table: hunts()},
		},
		Endings: []encounter.EndingInput{{Key: "done", Trigger: encounter.TriggerExternal{}}},
	})
	s.Require().NoError(err)
	s.enc = enc
}

// sightOf decodes what one member currently believes about another, and
// whether that belief is a live sighting rather than a memory.
func (s *PursuitSuite) sightOf(observer, subject core.EntityID) (bool, spatial.Position) {
	current, payload := seen(s.T(), s.enc, observer, subject)
	return current, spatial.Position{X: payload.X, Y: payload.Y}
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
	current, at := s.sightOf(hunter, hunted)
	s.Require().True(current, "first light: the goblin sees her across the room")
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
	s.Require().Len(out.Doors, 1, "the door is named, and decides nothing")
	s.Equal(encounter.DoorID(gateway), out.Doors[0].ID)

	current, at = s.sightOf(hunter, hunted)
	s.Require().True(current, "standing in the opening, she is still in plain sight")
	s.Equal(eastThreshold, at, "on the far side of it, on the same map")

	// And then out of its line, behind the wall the west chamber drew along
	// its own edge. NOW she is a memory.
	_, err = s.enc.Step(&encounter.StepInput{Member: hunted, To: eastCorner})
	s.Require().NoError(err)

	current, ghost := s.sightOf(hunter, hunted)
	s.Require().False(current, "the wall takes her; the sighting becomes a memory")
	s.Equal(eastThreshold, ghost, "the ghost stands where she was last seen — in the doorway")

	// A round of the world: the goblin walks to the ghost's cell, which is on
	// the far side of the doorway. The SAME intent type carries it — there is
	// no crossing intent, and no crossing mechanism for one to name.
	_, err = aRound(s.enc)
	s.Require().NoError(err)
	s.Equal(eastThreshold, s.whereIs(hunter),
		"it walks toward the last place it saw her, through the doorway, in one ordinary step")

	// And it can see her again, from the chamber it has just entered.
	current, found := s.sightOf(hunter, hunted)
	s.True(current, "the hunt closes: she is in sight again")
	s.Equal(eastCorner, found, "in the corner she ran to, on the same map")
}

// TestATableNamingTheVoidNeverWalksAnybodyOffTheFloor and its sibling below
// pin the two ways a walk declines without complaining. Both matter because a
// round of the world must survive a table that asks for something impossible:
// an error would abort the round for every other creature in the encounter.
//
// THE ROUTER IS WHAT REFUSES, not the step. An author naming a cell off the
// map gets the nearest the floor allows — the [encounter.MoveToward] policy's
// own fallback — and never a creature standing in the void. That is a
// different sentence from the one this test made before the table existed,
// when a decider handed a raw cell to the step and the step said no; the
// silent-refusal path is still there under it, and nothing can now reach it
// with an off-map cell.
func (s *PursuitSuite) TestATableNamingTheVoidNeverWalksAnybodyOffTheFloor() {
	s.freeRoamWalkingTo(spatial.Position{X: 500, Y: 500})

	_, err := aRound(s.enc)
	s.Require().NoError(err, "an impossible destination is not an error")
	s.True(s.rolled(), "and it really was asked")

	where := s.whereIs(straggler)
	region, onFloor := s.enc.RegionAt(where)
	s.True(onFloor, "it is standing on floor, not in the void it was sent at")
	s.Equal(encounter.RegionID(westID), region, "in the chamber it started in — the void is not a direction out of it")
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
	// A straggler pressed against the seam wall's north side, ordered at the
	// cell directly across it: adjacent in absolute space, with the wall
	// between.
	s.freeRoamWalkingTo(cellAt(26, 12))

	_, err := aRound(s.enc)
	s.Require().NoError(err, "walking into a wall is not an error either")
	s.Equal(cellAt(25, 12), s.whereIs(straggler), "the wall refused the step, so it stayed put")
	s.True(s.rolled(), "and it really was asked")
}

// rolled reports whether the hunter's own table was rolled at all — the
// `answered` beat is the only account of a pick, so its presence is what
// separates "asked and refused" from "never asked".
//
// BOTH REFUSAL PINS NEED IT. First light starts a fight and the world only
// thinks for creatures outside one, so a "nobody moved" assertion would
// otherwise pass because nobody was ever consulted, which is a test that
// cannot fail.
func (s *PursuitSuite) rolled() bool {
	story, err := s.enc.Story(&encounter.StoryInput{Audience: hunted})
	s.Require().NoError(err)
	for _, e := range story {
		var beat map[string]any
		s.Require().NoError(json.Unmarshal(e.Payload, &beat))
		if beat["beat"] == encounter.BeatAnswered && beat["creature"] == string(straggler) {
			return true
		}
	}

	return false
}

// freeRoamWalkingTo puts the hunter back on the world clock with a table that
// sends it at one authored cell, which is what a refusal pin needs before it
// means anything.
//
// THE DISSOLVE IS LOAD-BEARING: first light starts a fight, and the world only
// thinks for creatures outside one.
func (s *PursuitSuite) freeRoamWalkingTo(cell spatial.Position) {
	_, err := s.enc.Dissolve(&encounter.DissolveInput{Member: hunter})
	s.Require().NoError(err)

	seat := cellAt(22, 10)
	if cell == cellAt(26, 12) {
		// Against the wall, so the refused step is the wall's doing and not
		// the distance's.
		seat = cellAt(25, 12)
	}
	_, err = s.enc.Join(&encounter.JoinInput{
		Member: straggler, Kind: encounter.KindMonster, Cell: seat,
		SpeedFeet: 5, Table: walksTo(cell),
	})
	s.Require().NoError(err)

	// Arriving in sight of her starts a fight, and a fight monster is not the
	// world's to think for — so break it off, which is what leaves the
	// straggler where a refusal pin can reach it.
	on, err := s.enc.ClockOf(&encounter.ClockOfInput{Member: straggler})
	s.Require().NoError(err)
	if on.Kind == encounter.ClockTurn {
		_, err = s.enc.Dissolve(&encounter.DissolveInput{Member: straggler})
		s.Require().NoError(err)
	}
}

// straggler is the monster the refusal pins order about — joined after the
// fight is dissolved, so it is the world's to think for.
const straggler = core.EntityID("straggler")

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
