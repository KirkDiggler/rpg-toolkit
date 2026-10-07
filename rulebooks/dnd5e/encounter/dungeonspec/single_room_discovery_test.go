// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package dungeonspec_test

import (
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter/dungeonspec"
	"slices"
)

func (s *SingleRoomWallDoorSuite) TestOrdinaryDiscoveryPartitionsStayFixedAndSeatsStayOnTheStartingSide() {
	spec := s.gapSpec(dungeonspec.RoomDoorBinding{Closed: true})
	spec.Room.Gameplay.MonsterDeclarations = []dungeonspec.RoomMonsterSource{{
		ID: "far-skeleton", Ref: "dnd5e:monsters:skeleton",
		StartingCell: dungeonspec.RoomStartingCell{Location: dungeonspec.RoomCell{Q: 4, R: 0}},
	}}
	closed := s.load(spec)
	s.Require().Len(closed.Field.Regions, 2)
	s.Require().Len(closed.PartyStart, 1)
	s.Equal(closed.Field.Regions[0].ID, closed.PartyStart[0].Region)
	s.Equal(closed.Field.Regions[1].ID, closed.Monsters[0].Region)
	spec.Room.Gameplay.DoorBindings["gap-door"] = dungeonspec.RoomDoorBinding{}
	slices.Reverse(spec.Room.Gameplay.WalkableHexes)
	opened := s.load(spec)
	s.Equal(closed.Field.Regions, opened.Field.Regions, "door state and authoring order do not rename or merge spaces")
	s.Equal(encounter.DoorOpen, opened.Field.Doors[0].State.Kind(), "topology query must not close the actual door")
	enc := s.play(opened)
	atlas, err := enc.AtlasFor("walker")
	s.Require().NoError(err)
	s.Contains(atlas.Cells, axial(4, 0), "an initially open door permits ordinary discovery")
}

func (s *SingleRoomWallDoorSuite) TestOrdinaryDiscoveryStopsAtTheNextClosedDoor() {
	spec := s.gapSpec(dungeonspec.RoomDoorBinding{Closed: true})
	x := boundaryWallX(12.5)
	spec.Room.Gameplay.Walls = append(spec.Room.Gameplay.Walls,
		roomWall("far-boundary", x, -2, x, 2, wallBlocks(true, true, 4, 0.2, 0, 0),
			dungeonspec.RoomWallOpening{ID: "far-gap", Position: 2, Width: 2,
				Door: &dungeonspec.RoomWallDoor{ID: "far-door", AssetRef: "dnd5e:env:test:door"}}))
	spec.Room.Gameplay.DoorBindings["far-door"] = dungeonspec.RoomDoorBinding{Closed: true}
	compiled := s.load(spec)
	s.Require().Len(compiled.Field.Regions, 3)
	enc := s.play(compiled)
	_, err := enc.OpenDoor(&encounter.OpenDoorInput{Actor: "walker", Door: "wall-doors-room/gap-door"})
	s.Require().NoError(err)
	atlas, err := enc.AtlasFor("walker")
	s.Require().NoError(err)
	s.Contains(atlas.Cells, axial(2, 0))
	s.NotContains(atlas.Cells, axial(3, 0), "opening the first door must not reveal room three")
}

type findDoorCheck struct{}

func (findDoorCheck) ResolveCheck(in *encounter.ResolveCheckInput) (*encounter.ResolveCheckOutput, error) {
	return &encounter.ResolveCheckOutput{Beaten: true, Applied: in.Approaches[0], Total: 30}, nil
}
func (f findDoorCheck) ResolveDiscoveryCheck(in *encounter.ResolveCheckInput) (*encounter.ResolveCheckOutput, error) {
	return f.ResolveCheck(in)
}

func (s *SingleRoomWallDoorSuite) TestOrdinaryDiscoveryDoesNotSeeThroughAFoundSecretDoor() {
	spec := s.gapSpec(dungeonspec.RoomDoorBinding{Closed: true})
	spec.Concealments = map[string]dungeonspec.ConcealmentSpec{"secret": {
		Checks: dungeonspec.CheckSpec{{Ability: "perception", DC: 1}}, Props: []string{"gap-door"},
	}}
	compiled := s.load(spec)
	enc, err := encounter.NewEncounter(&encounter.SetupInput{
		Sight: everyoneSeesTheWholeMap{}, Equipment: encounter.UnobservedEquipment{}, Standing: everyoneStanding{},
		Initiative: orderAsGiven{}, TurnDriver: passDriver{}, Striker: noAttacksExpected{}, Mover: quietMover{}, Announcer: quietAnnouncer{},
		CheckResolver: findDoorCheck{}, Witness: nobodyPerceivesAnything{},
		Field: compiled.Field, Members: []encounter.MemberInput{{ID: "walker", Kind: encounter.KindPlayer, Position: axial(0, 0)}},
		Endings: []encounter.EndingInput{{Key: "withdrawn", Trigger: encounter.TriggerExternal{}}},
	})
	s.Require().NoError(err)
	atlas, err := enc.AtlasFor("walker")
	s.Require().NoError(err)
	s.Require().Len(atlas.StructuralDoors, 1, "the automatic check actually found the door")
	s.NotContains(atlas.Cells, axial(4, 0), "finding a door does not see through its closed leaf")
}

func (s *SingleRoomWallDoorSuite) TestOrdinaryDiscoveryWithATwoDimensionalDivider() {
	spec := s.gapSpec(dungeonspec.RoomDoorBinding{Closed: true})
	spec.Room.Gameplay.WalkableHexes = nil
	for q := 0; q <= 3; q++ {
		for r := -1; r <= 1; r++ {
			spec.Room.Gameplay.WalkableHexes = append(spec.Room.Gameplay.WalkableHexes, dungeonspec.RoomCell{Q: q, R: r})
		}
	}
	spec.Room.Gameplay.PartyStart = &dungeonspec.RoomCell{Q: 1, R: 0}
	x := boundaryWallX(7.5)
	spec.Room.Gameplay.Walls = []dungeonspec.RoomWall{
		roomWall("boundary", x, -6, x, 6, wallBlocks(true, true, 12, 0.2, 0, 0),
			dungeonspec.RoomWallOpening{ID: "gap", Position: 6, Width: 2,
				Door: &dungeonspec.RoomWallDoor{ID: "gap-door", AssetRef: "dnd5e:env:test:door"}}),
	}
	enc := s.play(s.load(spec), encounter.MemberInput{ID: "walker", Kind: encounter.KindPlayer, Position: axial(1, 0)})
	canvas, err := enc.Canvas()
	s.Require().NoError(err)
	s.Require().True(canvas.IsLineOfSightBlocked(axial(1, 0), axial(3, 0)))
	before, err := enc.AtlasFor("walker")
	s.Require().NoError(err)
	s.NotContains(before.Cells, axial(3, 0))
	s.NotEmpty(before.StructuralWalls, "the common wall remains visible even though its opaque footing is not another discovered room")
	_, err = enc.OpenDoor(&encounter.OpenDoorInput{Actor: "walker", Door: "wall-doors-room/gap-door"})
	s.Require().NoError(err)
	after, err := enc.AtlasFor("walker")
	s.Require().NoError(err)
	s.Contains(after.Cells, axial(3, 0))
}

// This is ordinary discovery, not a concealment fixture: the geometry already
// blocks sight, so the compiler must not teach both sides as one room.
func (s *SingleRoomWallDoorSuite) TestOrdinaryDiscoveryAcrossAnAuthoredWall() {
	spec := s.gapSpec(dungeonspec.RoomDoorBinding{Closed: true})
	spec.Room.Scene = sceneNode(s.T(), `
version: 1
id: scene-1
name: Wall Doors Room
items:
  - id: books
    kind: prop
    assetRef: dnd5e:props:books
    label: Books
    transform: {x: `+fmtFloat(boundaryWallX(20))+`, y: 0, z: 0, rotationY: 0}
groups: []
`)
	spec.Room.Gameplay.PropDeclarations = map[string]dungeonspec.RoomPropDeclaration{
		"books": wallBlocks(false, false, 0.2, 0.2, 0, 0),
	}
	compiled := s.load(spec)
	s.Require().Empty(compiled.Field.Concealments)
	enc := s.play(compiled,
		encounter.MemberInput{ID: "walker", Kind: encounter.KindPlayer, Position: axial(0, 0)},
		encounter.MemberInput{ID: "other", Kind: encounter.KindPlayer, Position: axial(4, 0)},
	)
	s.Require().True(s.sightAcross(enc), "positive control: existing geometry already blocks the closed lane")
	before, err := enc.AtlasFor("walker")
	s.Require().NoError(err)
	s.Contains(before.Cells, axial(0, 0))
	s.NotContains(before.Cells, axial(4, 0), "unseen far-side floor must not reveal the room's existence")
	for _, placed := range before.Placed {
		s.NotEqual("books", placed.ID, "ordinary far-side props need no explicit concealment")
	}
	s.NotEmpty(before.StructuralWalls, "the near-side wall itself must still be drawable")
	s.NotEmpty(before.StructuralDoors, "an ordinary closed door must remain discoverable from the near side")
	peer, err := enc.AtlasFor("other")
	s.Require().NoError(err)
	s.Contains(peer.Cells, axial(4, 0))
	s.NotContains(peer.Cells, axial(0, 0), "the second observer does not borrow the first's layout knowledge")
	s.NotEmpty(peer.StructuralWalls, "a boundary is visible from either adjoining room")
	s.NotEmpty(peer.StructuralDoors, "door identity does not depend on which side owns its nearest support cell")
	for _, observer := range []encounter.MemberID{"walker", "other"} {
		doors, doorErr := enc.DoorsFor(observer)
		s.Require().NoError(doorErr)
		s.Require().Len(doors, 1, "both sides can observe the actual closed boundary door")
		s.Equal(encounter.DoorClosed, doors[0].State.Kind())
	}

	_, err = enc.OpenDoor(&encounter.OpenDoorInput{Actor: "walker", Door: "wall-doors-room/gap-door"})
	s.Require().NoError(err)
	s.False(s.sightAcross(enc))
	learned, err := enc.AtlasFor("walker")
	s.Require().NoError(err)
	s.Contains(learned.Cells, axial(4, 0))
	found := false
	for _, placed := range learned.Placed {
		if placed.ID == "books" {
			found = true
		}
	}
	s.True(found, "opening permits ordinary discovery of the far-side prop")
	_, err = enc.CloseDoor(&encounter.CloseDoorInput{Actor: "walker", Door: "wall-doors-room/gap-door"})
	s.Require().NoError(err)
	remembered, err := s.reload(enc).AtlasFor("walker")
	s.Require().NoError(err)
	s.Equal(learned, remembered, "closing and reloading retain the discovered fixed layout")
}
