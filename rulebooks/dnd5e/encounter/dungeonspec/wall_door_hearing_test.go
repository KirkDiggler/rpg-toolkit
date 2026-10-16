// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package dungeonspec_test

import (
	"context"
	"encoding/json"

	"github.com/KirkDiggler/rpg-toolkit/core"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter/dungeonspec"
)

// wall_door_hearing_test.go is HEARING THROUGH A STUDIO WALL DOOR
// (rpg-toolkit#2066): a door attached to a wall's opening joins the two
// regions either side of it for sound, the way an edge door always has. The
// hearing law is untouched — a voice is DC 5 where it is said, every region
// crossed adds 5, and a shut door on the way adds 5 more — so through an open
// wall door a passive-10 listener hears a line (DC 10) and through a shut one
// does not (DC 15).
//
// THE ROOM IS ONE CORRIDOR, axial (0,0)..(6,0) on row 0, cut by walls across
// it. A wall at canonical x = 12.5 stands on the seam between (2,0) and (3,0);
// one at x = 22.5 between (4,0) and (5,0). Each wall's opening sits where the
// wall crosses the corridor, so a door in it is the corridor's only way on.

// The corridor's cast. The talker answers the hero's one line; the listeners
// each stand somewhere a region away and say only whether they heard it.
const (
	hearHero   encounter.MemberID = "hero"
	hearTalk   encounter.MemberID = "talker"
	hearNear   encounter.MemberID = "near"
	hearFar    encounter.MemberID = "far"
	hearBeyond encounter.MemberID = "beyond"
	hearKeen   encounter.MemberID = "keen"
)

// hearDoorID is the compiled id of the corridor's first wall door.
const hearDoorID = encounter.DoorID("wall-doors-room/seam-door")

// corridorWall is a wall across the corridor at canonical x, with its
// opening, and a door in it when door is not empty.
func corridorWall(id string, x float64, door string) dungeonspec.RoomWall {
	opening := dungeonspec.RoomWallOpening{ID: id + "-gap", Position: 2, Width: 2}
	if door != "" {
		opening.Door = &dungeonspec.RoomWallDoor{ID: door, AssetRef: "dnd5e:env:test:door"}
	}
	wall := roomWall(id, boundaryWallX(x), -2, boundaryWallX(x), 2, wallBlocks(true, true, 4, 0.2, 0, 0))
	if door != "" {
		wall.Openings = []dungeonspec.RoomWallOpening{opening}
	}

	return wall
}

// corridorSpec is the corridor with the walls given and the door bindings
// for the doors they carry.
func (s *SingleRoomWallDoorSuite) corridorSpec(bindings map[string]dungeonspec.RoomDoorBinding, walls ...dungeonspec.RoomWall) *dungeonspec.SingleRoomSpec {
	spec := s.baseSpec()
	spec.Room.Gameplay.WalkableHexes = nil
	for q := 0; q <= 6; q++ {
		spec.Room.Gameplay.WalkableHexes = append(spec.Room.Gameplay.WalkableHexes, dungeonspec.RoomCell{Q: q, R: 0})
	}
	spec.Room.Gameplay.Walls = walls
	spec.Room.Gameplay.DoorBindings = bindings

	return spec
}

// hearingSheets is every member's sheet in the corridor: the passive
// Perception given, zero (deaf) for anyone not named.
type hearingSheets map[encounter.MemberID]int

func (h hearingSheets) Sheets(members []encounter.MemberID) (map[encounter.MemberID]encounter.SheetFacts, error) {
	out := make(map[encounter.MemberID]encounter.SheetFacts, len(members))
	for _, id := range members {
		out[id] = encounter.SheetFacts{SpeedFeet: 30, PassivePerception: h[id]}
	}

	return out, nil
}

// hearingFaction is the corridor's one faction, civil to the party, so
// nobody's first sight of the hero forms a fight before a word is said.
const hearingFaction encounter.FactionID = "townsfolk"

// listenIn opens the corridor with the hero at (0,0), the talker at (1,0),
// and each listener where the scene puts it, at the passive given.
func (s *SingleRoomWallDoorSuite) listenIn(compiled dungeonspec.Compiled, passive hearingSheets, listeners map[encounter.MemberID]int) *encounter.Encounter {
	compiled.Field.Factions = append(compiled.Field.Factions, encounter.FactionInput{ID: hearingFaction})
	compiled.Field.Dispositions = append(compiled.Field.Dispositions, encounter.DispositionInput{
		Between: [2]encounter.FactionID{hearingFaction, encounter.FactionParty}, Stance: encounter.StanceNeutral,
	})
	members := []encounter.MemberInput{
		{ID: hearHero, Kind: encounter.KindPlayer, Position: axial(0, 0)},
		{ID: hearTalk, Kind: encounter.KindMonster, Position: axial(1, 0), Faction: hearingFaction,
			Options: []encounter.Option{{
				ID: "greet", Say: "Good evening.",
				Check:   []encounter.CheckApproach{{Ability: "persuasion", DC: 9}},
				Success: []encounter.Answer{{Weight: 1, Say: "And to you!"}},
			}}},
	}
	for _, id := range sortedListeners(listeners) {
		members = append(members, encounter.MemberInput{
			ID: id, Kind: encounter.KindMonster, Position: axial(listeners[id], 0), Faction: hearingFaction,
		})
	}
	enc, err := encounter.NewEncounter(&encounter.SetupInput{
		Field:   compiled.Field,
		Members: members,
		Endings: []encounter.EndingInput{{Key: "withdrawn", Trigger: encounter.TriggerExternal{}}},
		Capabilities: encounter.Capabilities{
			Sheets:        passive,
			Sight:         everyoneSeesTheWholeMap{},
			Equipment:     encounter.UnobservedEquipment{},
			Conditions:    encounter.UnobservedEquipment{},
			Participation: everyoneStanding{},
			Initiative:    orderAsGiven{},
			Driver:        passDriver{},
			Roller:        rollsOne{},
			CheckResolver: nothingIsEverFound{},
			Witness:       nobodyPerceivesAnything{},
			Actors: encounter.Actors{
				Striker:   noAttacksExpected{},
				Mover:     quietMover{},
				Announcer: quietAnnouncer{},
			},
		},
	})
	s.Require().NoError(err)

	return enc
}

// sortedListeners is the listeners' ids in order (C8).
func sortedListeners(listeners map[encounter.MemberID]int) []encounter.MemberID {
	out := make([]encounter.MemberID, 0, len(listeners))
	for _, id := range []encounter.MemberID{hearNear, hearFar, hearBeyond, hearKeen} {
		if _, ok := listeners[id]; ok {
			out = append(out, id)
		}
	}

	return out
}

// greet is the hero's line to the talker, and the talker's answer: a voice
// said at (1,0).
func (s *SingleRoomWallDoorSuite) greet(enc *encounter.Encounter) {
	_, err := enc.Speak(context.Background(), &encounter.SpeakInput{
		Actor: hearHero, Target: hearTalk, OptionID: "greet", Skill: "persuasion",
		Beaten: true, DC: 9, Total: 14, Roller: rollsOne{},
	})
	s.Require().NoError(err)
}

// voicesHeardBy is how many voices a member holds on its sounds channel —
// what a reload would hand its table.
func (s *SingleRoomWallDoorSuite) voicesHeardBy(enc *encounter.Encounter, member encounter.MemberID) int {
	n := 0
	for _, h := range enc.ToData().Perception.Intel.Holdings[core.EntityID("member|"+string(member))] {
		if string(h.Channel) != "sounds" {
			continue
		}
		var payload map[string]any
		s.Require().NoError(json.Unmarshal(h.Payload, &payload))
		if payload["kind"] == encounter.SoundVoice {
			n++
		}
	}

	return n
}

// regionOf is the region holding a corridor cell.
func (s *SingleRoomWallDoorSuite) regionOf(enc *encounter.Encounter, q int) encounter.RegionID {
	region, ok := enc.RegionAt(axial(q, 0))
	s.Require().True(ok, "(%d,0) is floor", q)

	return region
}

// TestAWallDoorCarriesAVoiceOpenAndStopsItShut is the proving test of
// rpg-toolkit#2066: two regions split by a wall with one attached door, a
// passive-10 listener on each side, and a line said on the near side. Open,
// the far listener hears it (1 hop, DC 10); shut, it does not (1 hop and a
// shut door, DC 15). The near listener hears it both times (0 hops, DC 5).
func (s *SingleRoomWallDoorSuite) TestAWallDoorCarriesAVoiceOpenAndStopsItShut() {
	for _, tc := range []struct {
		name    string
		binding dungeonspec.RoomDoorBinding
		heard   bool
	}{
		{name: "open", binding: dungeonspec.RoomDoorBinding{}, heard: true},
		{name: "shut", binding: dungeonspec.RoomDoorBinding{Closed: true}, heard: false},
	} {
		s.Run(tc.name, func() {
			compiled := s.load(s.corridorSpec(map[string]dungeonspec.RoomDoorBinding{"seam-door": tc.binding},
				corridorWall("seam", 12.5, "seam-door")))
			enc := s.listenIn(compiled, hearingSheets{hearNear: 10, hearFar: 10},
				map[encounter.MemberID]int{hearNear: 2, hearFar: 4})
			s.Require().Equal(s.regionOf(enc, 1), s.regionOf(enc, 2), "precondition: the near listener shares the talker's region")
			s.Require().NotEqual(s.regionOf(enc, 1), s.regionOf(enc, 4), "precondition: the wall splits the corridor in two")

			s.greet(enc)

			s.Positive(s.voicesHeardBy(enc, hearNear), "the near listener hears the line where it is said: DC 5")
			if tc.heard {
				s.Positive(s.voicesHeardBy(enc, hearFar), "through the open door: DC 5 + 5 for the hop = 10")
			} else {
				s.Zero(s.voicesHeardBy(enc, hearFar), "through the shut door: DC 5 + 5 + 5 = 15")
			}
		})
	}
}

// TestOpeningTheWallDoorChangesTheNextSound: the door's state is read when
// the sound is made, not when the graph was drawn. Shut, the far listener
// misses the line; opened, it hears the next one.
func (s *SingleRoomWallDoorSuite) TestOpeningTheWallDoorChangesTheNextSound() {
	compiled := s.load(s.corridorSpec(map[string]dungeonspec.RoomDoorBinding{"seam-door": {Closed: true}},
		corridorWall("seam", 12.5, "seam-door")))
	enc := s.listenIn(compiled, hearingSheets{hearFar: 10}, map[encounter.MemberID]int{hearFar: 4})

	s.greet(enc)
	s.Require().Zero(s.voicesHeardBy(enc, hearFar), "shut: DC 15 beats passive 10")

	_, err := enc.OpenDoor(&encounter.OpenDoorInput{Door: hearDoorID})
	s.Require().NoError(err)
	s.greet(enc)
	s.Positive(s.voicesHeardBy(enc, hearFar), "opened: the next line is DC 10, and passive 10 hears it")
}

// TestAWallDoorLinksOnlyTheTwoRegionsItStandsBetween: three regions in a row,
// an open door in the first wall and a shut one in the second. A line said in
// the first region reaches the third only the long way, through both doors:
// 2 regions crossed and 1 shut door, 3 hops, DC 20. Were the first door to
// join the first region to the third as well, the third would hear it at
// DC 10.
func (s *SingleRoomWallDoorSuite) TestAWallDoorLinksOnlyTheTwoRegionsItStandsBetween() {
	compiled := s.load(s.corridorSpec(
		map[string]dungeonspec.RoomDoorBinding{"seam-door": {}, "far-door": {Closed: true}},
		corridorWall("seam", 12.5, "seam-door"), corridorWall("far", 22.5, "far-door")))
	enc := s.listenIn(compiled, hearingSheets{hearFar: 10, hearBeyond: 15, hearKeen: 20},
		map[encounter.MemberID]int{hearFar: 3, hearBeyond: 5, hearKeen: 6})
	s.Require().Equal(s.regionOf(enc, 5), s.regionOf(enc, 6), "precondition: the third region holds (5,0) and (6,0)")
	s.Require().NotEqual(s.regionOf(enc, 3), s.regionOf(enc, 5), "precondition: the second wall splits it off")
	s.Require().NotEqual(s.regionOf(enc, 1), s.regionOf(enc, 5), "precondition: and it is not the talker's")

	s.greet(enc)

	s.Positive(s.voicesHeardBy(enc, hearFar), "the second region hears through the open door: DC 10")
	s.Zero(s.voicesHeardBy(enc, hearBeyond), "passive 15 in the third does not: the first door does not reach it")
	s.Positive(s.voicesHeardBy(enc, hearKeen), "passive 20 does: through both doors, DC 5 + 5 + 5 + 5 = 20")
}

// TestARoomWithNoDoorsLinksNothing is the v4 room before this fix and after:
// a solid wall across the corridor and no door, and nothing crosses it.
func (s *SingleRoomWallDoorSuite) TestARoomWithNoDoorsLinksNothing() {
	compiled := s.load(s.corridorSpec(nil, corridorWall("solid", 12.5, "")))
	s.Require().Empty(compiled.Field.Doors, "precondition: the room has no doors")
	enc := s.listenIn(compiled, hearingSheets{hearNear: 10, hearFar: 30},
		map[encounter.MemberID]int{hearNear: 2, hearFar: 4})
	s.Require().NotEqual(s.regionOf(enc, 1), s.regionOf(enc, 4), "precondition: the wall splits the corridor in two")

	s.greet(enc)

	s.Positive(s.voicesHeardBy(enc, hearNear), "the talker's own region hears it")
	s.Zero(s.voicesHeardBy(enc, hearFar), "and nothing crosses a wall with no door")
}
