// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package encounter_test

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter"
	"github.com/KirkDiggler/rpg-toolkit/tools/spatial"
)

// RoomKnowledgeSuite uses ordinary rooms, not concealment flags. Discovery
// must not depend on an author marking every unexplored room as a secret.
type RoomKnowledgeSuite struct {
	suite.Suite
	enc   *encounter.Encounter
	sight *sightList
}

func TestRoomKnowledgeSuite(t *testing.T) { suite.Run(t, new(RoomKnowledgeSuite)) }

func (s *RoomKnowledgeSuite) SetupTest() {
	s.sight = &sightList{fallback: 20}
	var err error
	s.enc, err = encounter.NewEncounter(&encounter.SetupInput{
		Sight: s.sight, Equipment: encounter.UnobservedEquipment{}, Standing: everyoneStanding{},
		Initiative: orderAsGiven{}, TurnDriver: passDriver{}, Striker: passStriker{},
		Mover: quietMover{}, Announcer: quietAnnouncer{},
		Field: encounter.FieldInput{
			Canvas: pointyCanvas(),
			Regions: []encounter.RegionInput{
				rectRegion("entry", 0, 0, 3, 5),
				rectRegion("room", 3, 0, 4, 5),
				rectRegion("beyond", 7, 0, 3, 5),
			},
			Scenery: []spatial.Position{{X: 5, Y: 5}},
			Walls: append(append(seamWallExcept(2, 5, 1), seamWallExcept(6, 5, 1)...),
				wall(0, 4, 0, 3), wall(0, 4, 1, 4)),
			Doors: []encounter.DoorInput{
				{ID: "entry-door", Edges: doorEdgesAcross(2, 1), State: encounter.DoorIsClosed()},
				{ID: "far-door", Edges: doorEdgesAcross(6, 1), State: encounter.DoorIsClosed()},
			},
			Exits: []encounter.FieldExit{{ID: "hidden-exit", At: cellAt(8, 1)}},
		},
		Members: []encounter.MemberInput{
			{ID: "a", Kind: encounter.KindPlayer, Position: cellAt(2, 1)},
			{ID: "b", Kind: encounter.KindPlayer, Position: spatial.Position{X: 0, Y: 4}},
		},
		Endings: []encounter.EndingInput{{Key: "withdrawn", Trigger: encounter.TriggerExternal{}}},
	})
	s.Require().NoError(err)
}

func (s *RoomKnowledgeSuite) atlas(member encounter.MemberID) encounter.Atlas {
	out, err := s.enc.AtlasFor(member)
	s.Require().NoError(err)
	return out
}

func (s *RoomKnowledgeSuite) regions(member encounter.MemberID) []string {
	out := []string{}
	for _, r := range s.atlas(member).Regions {
		out = append(out, r.ID)
	}
	return out
}

func (s *RoomKnowledgeSuite) TestClosedOrdinaryRoomsAreNotAlreadyDelivered() {
	for _, observer := range []encounter.MemberID{"a", "b"} {
		s.Equal([]string{"entry"}, s.regions(observer))
		atlas := s.atlas(observer)
		s.Empty(atlas.Exits)
		s.Len(atlas.Cells, 15)
		for _, doorway := range atlas.Doorways {
			s.Equal("entry-door", doorway.Door)
		}
	}
	// Author/world truth remains available to engine callers, not the player read.
	full, err := s.enc.Atlas()
	s.Require().NoError(err)
	s.Len(full.Regions, 3)
}

func (s *RoomKnowledgeSuite) TestOpeningTeachesFixedRoomWithoutTeachingAnObstructedPeer() {
	// Before opening, the closed edge blocks A. B's alcove walls block its
	// sight of the doorway; neither room is an authored secret.
	_, err := s.enc.OpenDoor(&encounter.OpenDoorInput{Door: "entry-door", Actor: "a"})
	s.Require().NoError(err)
	s.Equal([]string{"entry", "room"}, s.regions("a"))
	s.Equal([]string{"entry"}, s.regions("b"))
	s.Len(s.atlas("a").Regions[1].Cells, 20, "learn the whole fixed room, not only visible cells")
	s.Empty(s.atlas("a").Exits, "the next room remains undiscovered")
}

func (s *RoomKnowledgeSuite) TestOpeningTeachesLayoutEvenWithNoSightBeyondTheDoor() {
	s.sight.reach = map[encounter.MemberID]int{"a": 0}
	_, err := s.enc.OpenDoor(&encounter.OpenDoorInput{Door: "entry-door", Actor: "a"})
	s.Require().NoError(err)
	s.Equal([]string{"entry", "room"}, s.regions("a"))
	s.Equal([]string{"entry"}, s.regions("b"))
}

func (s *RoomKnowledgeSuite) TestAnObserverCanDiscoverWithoutBeingTheOpener() {
	// No actor means a world-side opening: only actual observers learn it.
	_, err := s.enc.OpenDoor(&encounter.OpenDoorInput{Door: "entry-door"})
	s.Require().NoError(err)
	s.Equal([]string{"entry", "room"}, s.regions("a"))
	s.Equal([]string{"entry"}, s.regions("b"))
}

func (s *RoomKnowledgeSuite) TestLayoutDiscoveryDoesNotRepeatOrAlias() {
	before := s.enc.ToData().World
	_, err := s.enc.Recheck(&encounter.RecheckInput{Members: []encounter.MemberID{"a"}})
	s.Require().NoError(err)
	s.Equal(before, s.enc.ToData().World, "unchanged sight does not append discovery history")
	atlas := s.atlas("a")
	atlas.Cells[0].X = 999
	atlas.Regions[0].Cells[0].X = 999
	s.NotEqual(atlas.Cells, s.atlas("a").Cells)
	s.NotEqual(atlas.Regions, s.atlas("a").Regions)
}

func (s *RoomKnowledgeSuite) TestInvalidRoomKnowledgeIsRejectedOnLoad() {
	for _, change := range []string{"room", "audience", "subject", "actor"} {
		s.Run(change, func() {
			data := s.enc.ToData()
			s.Require().NotEmpty(data.World.Facts)
			fact := &data.World.Facts[0]
			switch change {
			case "room":
				fact.Kind = "known:room:does-not-exist"
			case "audience":
				fact.Audience = []string{"a", "b"}
			case "subject":
				fact.Subject = "other"
			case "actor":
				fact.Actor = "other"
			}
			_, err := encounter.LoadEncounter(&encounter.LoadEncounterInput{
				Data: data, Sight: s.sight, Equipment: encounter.UnobservedEquipment{}, Standing: everyoneStanding{},
				Initiative: orderAsGiven{}, TurnDriver: passDriver{}, Striker: passStriker{},
				Mover: quietMover{}, Announcer: quietAnnouncer{},
			})
			s.ErrorIs(err, encounter.ErrInvalidData)
		})
	}
}

func (s *RoomKnowledgeSuite) TestRoomRevealIsRecipientScopedFixedData() {
	_, err := s.enc.OpenDoor(&encounter.OpenDoorInput{Door: "entry-door", Actor: "a"})
	s.Require().NoError(err)
	for _, member := range []encounter.MemberID{"a", "b"} {
		story, err := s.enc.Story(&encounter.StoryInput{Audience: member})
		s.Require().NoError(err)
		count := 0
		for _, entry := range story {
			var beat map[string]json.RawMessage
			s.Require().NoError(json.Unmarshal(entry.Payload, &beat))
			if string(beat["beat"]) != `"room_revealed"` {
				continue
			}
			count++
			var region struct {
				ID    string
				Cells []spatial.Position
			}
			s.Require().NoError(json.Unmarshal(beat["region"], &region))
			s.Equal("room", region.ID)
			s.Len(region.Cells, 20)
			var scenery []spatial.Position
			s.Require().NoError(json.Unmarshal(beat["scenery"], &scenery))
			s.Contains(scenery, cellAt(5, 5), "unowned floor reaches the client with the room")
			s.NotContains(region.Cells, cellAt(5, 5), "scenery does not become walkable owned floor")
			s.NotContains(string(entry.Payload), `"state"`, "door state is not fixed room data")
			s.NotContains(string(entry.Payload), "hidden-exit")
		}
		if member == "a" {
			s.Equal(1, count)
		} else {
			s.Zero(count)
		}
	}
}

func (s *RoomKnowledgeSuite) TestKnowledgeSurvivesLossOfSightAndJSONReload() {
	_, err := s.enc.OpenDoor(&encounter.OpenDoorInput{Door: "entry-door", Actor: "a"})
	s.Require().NoError(err)
	learned := s.atlas("a")
	_, err = s.enc.CloseDoor(&encounter.CloseDoorInput{Door: "entry-door", Actor: "a"})
	s.Require().NoError(err)
	s.Equal(learned, s.atlas("a"))
	bytes, err := json.Marshal(s.enc.ToData())
	s.Require().NoError(err)
	var data encounter.EncounterData
	s.Require().NoError(json.Unmarshal(bytes, &data))
	s.enc, err = encounter.LoadEncounter(&encounter.LoadEncounterInput{
		Data: data, Sight: s.sight, Equipment: encounter.UnobservedEquipment{}, Standing: everyoneStanding{},
		Initiative: orderAsGiven{}, TurnDriver: passDriver{}, Striker: passStriker{},
		Mover: quietMover{}, Announcer: quietAnnouncer{},
	})
	s.Require().NoError(err)
	s.Equal(learned, s.atlas("a"))
	s.Equal([]string{"entry"}, s.regions("b"))
}
