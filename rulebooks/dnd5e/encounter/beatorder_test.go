// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package encounter_test

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/KirkDiggler/rpg-toolkit/tools/spatial"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter"
)

// BeatOrderTestSuite guards ONE law, at every verb that can trigger a fight:
//
//	A VERB'S OWN BEAT PRECEDES ANY BEAT ITS CONSEQUENCES APPEND.
//
// Cause before effect, in every story. A reader of Story must be able to see
// the walk that started the fight before the fight — an engine whose product
// IS the narration cannot narrate backwards.
//
// The law is stated at [refreshSight]; these are its seven guards. Setup ruled
// it first (a scene records that it opened before it records a fight starting
// inside it), and trigger detection then arrived at Move, Traverse, Pump and
// Join. Two of the four (Traverse via TestTraverseBeatPinned, Join via
// TestTombWatch) inverted the moment trigger detection moved inside
// refreshSight; the other two were latent only because nothing asserted them.
// Step joined them with rpg-toolkit#1059 and inherits the law by writing the
// obvious call. OpenDoor joined them with rpg-toolkit#1123 — a door swinging
// open is a new way for two sides to come into contact, and a new verb is
// exactly the moment a law like this goes unnoticed (found by Copilot on
// PR #1125). All seven are asserted here so a future verb inherits a guarded
// law rather than a remembered one.
//
// Every scene below uses the same set: a 12x12 room split by a wall across
// y=6, open at either end. Nobody is in contact until the verb under test puts
// them in contact.
type BeatOrderTestSuite struct {
	suite.Suite
}

func TestBeatOrderSuite(t *testing.T) {
	suite.Run(t, new(BeatOrderTestSuite))
}

const beatOrderRoom = "hall"

// wallRoom is the shared set: one room, split by a wall across y=6 with open
// ground at either end of it.
//
// It was a single pillar until spatial v0.9.1, which leans around a lone
// obstacle — see testwalls_test.go. The scenes are unchanged; what changed is
// that the set has to be something a sightline genuinely cannot get past.
func wallRoom() encounter.FieldInput {
	return encounter.FieldInput{
		Canvas:  openAir(),
		Regions: []encounter.RegionInput{rectRegion(beatOrderRoom, 0, 0, 12, 12)},
		Props:   wallRow(6, 4, 8),
	}
}

// beatKinds reads one audience's whole story as its list of beat kinds.
func (s *BeatOrderTestSuite) beatKinds(enc *encounter.Encounter, audience encounter.MemberID) []string {
	story, err := enc.Story(&encounter.StoryInput{Audience: audience})
	s.Require().NoError(err)
	kinds := make([]string, 0, len(story))
	for _, entry := range story {
		var beat map[string]any
		s.Require().NoError(json.Unmarshal(entry.Payload, &beat))
		kinds = append(kinds, beat["beat"].(string))
	}
	return kinds
}

// TestSetupOpensBeforeItFights is the pin Setup already earned: first light
// can start a fight, and the fight belongs INSIDE the scene it happens in.
// Classifying before the opening beat is appended reads as a fight in a room
// that has not opened yet.
func (s *BeatOrderTestSuite) TestSetupOpensBeforeItFights() {
	enc, err := encounter.NewEncounter(&encounter.SetupInput{
		Sight: everyoneSeesTheWholeMap{}, Standing: everyoneStanding{}, Initiative: orderAsGiven{}, TurnDriver: passDriver{}, Striker: passStriker{},
		Field: wallRoom(),
		Members: []encounter.MemberInput{
			// Both clear of the wall's span: they see each other at first light.
			{ID: alice, Kind: encounter.KindPlayer, Position: spatial.Position{X: 0, Y: 2}},
			{ID: goblin, Kind: encounter.KindMonster, Position: spatial.Position{X: 0, Y: 10}},
		},
		Endings: []encounter.EndingInput{{Key: "withdrawn", Trigger: encounter.TriggerExternal{}}},
	})
	s.Require().NoError(err)

	s.Equal([]string{"scene-opened", "bubble-formed"}, s.beatKinds(enc, alice),
		"the scene opens, THEN the fight inside it starts")
}

// TestAStepThroughADoorwayBeforeItFights pins the crossing case — the verb
// whose inversion this suite was written from, back when going through a door
// was its own verb writing its own beat. Alice steps through a doorway into
// the goblin's chamber: the moved beat is the cause, the fight is the effect,
// and the story cannot tell this step apart from any other (rpg-toolkit#1106).
func (s *BeatOrderTestSuite) TestAStepThroughADoorwayBeforeItFights() {
	enc, err := encounter.NewEncounter(&encounter.SetupInput{
		Sight: everyoneSeesTheWholeMap{}, Standing: everyoneStanding{}, Initiative: orderAsGiven{}, TurnDriver: passDriver{}, Striker: passStriker{},
		Field: encounter.FieldInput{
			Canvas:  encounter.CanvasInput{Void: encounter.VoidIsOpaque(), Orientation: encounter.HexesArePointyTop()},
			Regions: []encounter.RegionInput{rectRegion(room1, 0, 0, 10, 10), rectRegion(room2, 10, 0, 10, 10)}, Walls: twoRoomWall(),
			Doors: []encounter.DoorInput{openDoorway("door1", 9, 5, 10, 5)},
		},
		Members: []encounter.MemberInput{
			{ID: alice, Kind: encounter.KindPlayer, Position: spatial.Position{X: 9, Y: 5}},
			// OFF the doorway's row: with one canvas, a monster standing in
			// the opening's line would be seen from room1 at first light and
			// this scene would open as a fight (rpg-toolkit#1106).
			{ID: goblin, Kind: encounter.KindMonster, Position: spatial.Position{X: 11, Y: 1}},
		},
		Endings: []encounter.EndingInput{{Key: "withdrawn", Trigger: encounter.TriggerExternal{}}},
	})
	s.Require().NoError(err)

	// The far side of the doorway, in dungeon-absolute cells: room2 local
	// (0,5) anchored at (10,0).
	out, err := enc.Step(&encounter.StepInput{Member: alice, To: cellAt(10, 5)})
	s.Require().NoError(err)
	s.Require().Len(out.Doors, 1, "the door is named, and decides nothing")
	s.Equal(encounter.DoorID("door1"), out.Doors[0].ID)
	s.Require().NotNil(out.Formed, "she walks into the chamber the goblin is standing in")
	s.Greater(out.Formed.Seq, out.Seq, "she goes through the door, THEN the fight starts")

	s.Equal([]string{"scene-opened", "moved", "bubble-formed"}, s.beatKinds(enc, alice))
}

// TestStepBeforeItFights pins the step verb's half, and it is the one that
// matters most going forward: the seam walks a party one Step at a time, so
// every player movement in the game reaches trigger detection through here.
//
// Alice starts behind the wall and steps out past its end, and the beat it
// writes is "moved". A step is not a new kind of event in the story.
func (s *BeatOrderTestSuite) TestStepBeforeItFights() {
	enc := s.blockedScene()

	out, err := enc.Step(&encounter.StepInput{Member: alice, To: cellAt(1, 2)})
	s.Require().NoError(err)
	s.Require().NotNil(out.Formed, "stepping into the open puts her in contact")
	s.Greater(out.Formed.Seq, out.Seq, "she steps, THEN the fight starts")

	s.Equal([]string{"scene-opened", "moved", "bubble-formed"}, s.beatKinds(enc, alice))
}

// TestPumpBeforeItFights pins Pump's half, and Pump is the verb the whole
// placement argument rests on: here NOBODY walks — the monster does. Pump's
// own beats are the tick frame AND the action inside it, so both precede the
// fight the action caused.
func (s *BeatOrderTestSuite) TestPumpBeforeItFights() {
	enc := s.blockedScene(&patrolDecider{positions: []spatial.Position{cellAt(0, 10)}})

	out, err := enc.Pump(&encounter.PumpInput{})
	s.Require().NoError(err)
	s.Require().NotNil(out.Formed, "the goblin steps out from behind the wall and is seen")
	s.Require().Len(out.Seqs, 2, "pump's own beats: the tick frame and the goblin's step")
	for _, seq := range out.Seqs {
		s.Greater(out.Formed.Seq, seq, "the world moves, THEN the fight starts")
	}

	s.Equal([]string{"scene-opened", "tick", "moved", "bubble-formed"}, s.beatKinds(enc, alice))
}

// TestJoinBeforeItFights pins Join's half. Cormac connects late and lands in
// the goblin's sight: he arrives, and the fight is what his arrival caused.
func (s *BeatOrderTestSuite) TestJoinBeforeItFights() {
	enc := s.blockedScene()

	out, err := enc.Join(&encounter.JoinInput{
		Member: cormac, Kind: encounter.KindPlayer,
		Cell: cellAt(0, 2),
	})
	s.Require().NoError(err)
	s.Require().NotNil(out.Formed, "he lands in the open, in the goblin's sight")
	s.Greater(out.Formed.Seq, out.Seq, "he arrives, THEN the fight starts")

	s.Equal([]string{"scene-opened", "joined", "bubble-formed"}, s.beatKinds(enc, alice))
}

// TestOpenDoorOpensBeforeItFights pins the door's half, and doors are the first
// verb in this suite where NOBODY MOVES: the world changes shape underneath two
// members who are standing still, and the fight is what the changed shape
// caused. The door beat is that cause and belongs first.
func (s *BeatOrderTestSuite) TestOpenDoorOpensBeforeItFights() {
	const shutDoor = "shut-door"

	enc, err := encounter.NewEncounter(&encounter.SetupInput{
		Sight: everyoneSeesTheWholeMap{}, Standing: everyoneStanding{}, Initiative: orderAsGiven{}, TurnDriver: passDriver{}, Striker: passStriker{},
		Field: encounter.FieldInput{
			Canvas:  encounter.CanvasInput{Void: encounter.VoidIsOpaque(), Orientation: encounter.HexesArePointyTop()},
			Regions: []encounter.RegionInput{rectRegion("near", 0, 0, 3, 3), rectRegion("far", 3, 0, 3, 3)}, Walls: seamWallExcept(2, 3, 1),
			Doors: []encounter.DoorInput{{
				ID: shutDoor, Edges: doorEdgesAcross(2, 1), State: encounter.DoorIsClosed(),
			}},
		},
		Members: []encounter.MemberInput{
			{ID: alice, Kind: encounter.KindPlayer, Position: spatial.Position{X: 2, Y: 1}},
			{ID: goblin, Kind: encounter.KindMonster, Position: spatial.Position{X: 3, Y: 1}},
		},
		Endings: []encounter.EndingInput{{Key: "withdrawn", Trigger: encounter.TriggerExternal{}}},
	})
	s.Require().NoError(err)
	s.Require().Equal([]string{"scene-opened"}, s.beatKinds(enc, alice),
		"the shut door keeps them apart: no fight before the verb under test")

	out, err := enc.OpenDoor(&encounter.OpenDoorInput{Door: shutDoor})
	s.Require().NoError(err)
	s.Require().NotNil(out.Formed, "the door opens onto the goblin")
	s.Greater(out.Formed.Seq, out.Seq, "the door opens, THEN the fight starts")

	s.Equal([]string{"scene-opened", "door", "bubble-formed"}, s.beatKinds(enc, alice))
}

// blockedScene opens the shared set with alice at (6,2) and the goblin at
// (6,10) — the wall spans x=4..8 at y=6, so the file they share is blocked and
// first light starts no fight. Each verb under test is then the sole cause of
// the one that follows. An optional decider drives the goblin for the Pump pin.
func (s *BeatOrderTestSuite) blockedScene(decider ...encounter.Decider) *encounter.Encounter {
	monster := encounter.MemberInput{
		ID: goblin, Kind: encounter.KindMonster, Position: spatial.Position{X: 6, Y: 10},
	}
	if len(decider) > 0 {
		monster.Decider = decider[0]
	}

	enc, err := encounter.NewEncounter(&encounter.SetupInput{
		Sight: everyoneSeesTheWholeMap{}, Standing: everyoneStanding{}, Initiative: orderAsGiven{}, TurnDriver: passDriver{}, Striker: passStriker{},
		Field: wallRoom(),
		Members: []encounter.MemberInput{
			{ID: alice, Kind: encounter.KindPlayer, Position: spatial.Position{X: 6, Y: 2}},
			monster,
		},
		Endings: []encounter.EndingInput{{Key: "withdrawn", Trigger: encounter.TriggerExternal{}}},
	})
	s.Require().NoError(err)
	s.Require().Equal([]string{"scene-opened"}, s.beatKinds(enc, alice),
		"the wall keeps them apart: no fight before the verb under test")
	return enc
}
