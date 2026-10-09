// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package dungeonspec_test

import (
	"encoding/json"
	"fmt"
	"slices"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter/dungeonspec"
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
	// Put the next boundary's nearest centre on its undiscovered side.
	x := boundaryWallX(12.5 + 0.000001)
	spec.Room.Gameplay.Walls = append(spec.Room.Gameplay.Walls,
		roomWall("far-boundary", x, -2, x, 2, wallBlocks(true, true, 4, 0.2, 0, 0),
			dungeonspec.RoomWallOpening{ID: "far-gap", Position: 2, Width: 2,
				Door: &dungeonspec.RoomWallDoor{ID: "far-door", AssetRef: "dnd5e:env:test:door"}}))
	spec.Room.Gameplay.DoorBindings["far-door"] = dungeonspec.RoomDoorBinding{Closed: true}
	compiled := s.load(spec)
	s.Require().Len(compiled.Field.Regions, 3)
	enc := s.play(compiled)
	before, err := enc.AtlasFor("walker")
	s.Require().NoError(err)
	_, err = enc.OpenDoor(&encounter.OpenDoorInput{Actor: "walker", Door: "wall-doors-room/gap-door"})
	s.Require().NoError(err)
	atlas, err := enc.AtlasFor("walker")
	s.Require().NoError(err)
	s.Contains(atlas.Cells, axial(2, 0))
	s.NotContains(atlas.Cells, axial(3, 0), "opening the first door must not reveal room three")
	known := make(map[string]bool)
	for _, placed := range before.Placed {
		known[placed.ID] = true
	}
	story, err := enc.Story(&encounter.StoryInput{Audience: "walker"})
	s.Require().NoError(err)
	delivered := make(map[string]bool)
	for _, entry := range story {
		var payload struct {
			Beat   string `json:"beat"`
			Placed []struct {
				ID string `json:"id"`
			} `json:"placed"`
		}
		s.Require().NoError(json.Unmarshal(entry.Payload, &payload))
		if payload.Beat == encounter.BeatRoomRevealed {
			for _, placed := range payload.Placed {
				delivered[placed.ID] = true
			}
		}
	}
	for _, placed := range atlas.Placed {
		if !known[placed.ID] {
			s.True(delivered[placed.ID], "newly permitted boundary %q must arrive in the same room reveal as its snapshot", placed.ID)
		}
	}
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
		Field:   compiled.Field,
		Members: []encounter.MemberInput{{ID: "walker", Kind: encounter.KindPlayer, Position: axial(0, 0)}},
		Endings: []encounter.EndingInput{{Key: "withdrawn", Trigger: encounter.TriggerExternal{}}},
		Capabilities: encounter.Capabilities{
			Sheets:        zeroSheets{},
			Sight:         everyoneSeesTheWholeMap{},
			Equipment:     encounter.UnobservedEquipment{},
			Standing:      everyoneStanding{},
			Initiative:    orderAsGiven{},
			Driver:        passDriver{},
			CheckResolver: findDoorCheck{},
			Witness:       nobodyPerceivesAnything{},
			Actors: encounter.Actors{
				Striker:   noAttacksExpected{},
				Mover:     quietMover{},
				Announcer: quietAnnouncer{},
			},
		},
	})
	s.Require().NoError(err)
	atlas, err := enc.AtlasFor("walker")
	s.Require().NoError(err)
	s.Require().Len(atlas.StructuralDoors, 1, "the automatic check actually found the door")
	s.NotContains(atlas.Cells, axial(4, 0), "finding a door does not see through its closed leaf")
}

// Review F1/F2: an opaque centre-covered cell separating two spaces is
// permanent boundary footing, not a phantom room and not a bridge between rooms.
func (s *SingleRoomWallDoorSuite) TestOrdinaryDiscoveryOpaqueFootingIsVisibleFromBothSides() {
	for _, structural := range []bool{false, true} {
		s.Run(fmt.Sprintf("structural=%t", structural), func() {
			spec := s.gapSpec(dungeonspec.RoomDoorBinding{Closed: true})
			spec.Room.Gameplay.DoorBindings = nil
			id := "boundary"
			if structural {
				x := boundaryWallX(5)
				spec.Room.Gameplay.Walls = []dungeonspec.RoomWall{roomWall(id, x, -2, x, 2,
					wallBlocks(true, true, 4, boundaryWallX(1.5), 0, 0))}
			} else {
				id = "pillar"
				spec.Room.Gameplay.Walls = nil
				spec.Room.Scene = sceneNode(s.T(), `
version: 1
id: scene-1
name: Divider
items: [{id: pillar, kind: prop, assetRef: 'dnd5e:props:pillar', transform: {x: `+fmtFloat(boundaryWallX(5))+`, y: 0, z: 0, rotationY: 0}}]
groups: []
`)
				spec.Room.Gameplay.PropDeclarations = map[string]dungeonspec.RoomPropDeclaration{
					id: wallBlocks(true, true, boundaryWallX(1.5), boundaryWallX(1.5), 0, 0),
				}
			}
			compiled := s.load(spec)
			s.Require().Len(compiled.Field.Regions, 2, "never join the rooms or mint a third phantom region")
			s.Require().Len(compiled.Field.Scenery, 1)
			s.Equal(axial(1, 0), compiled.Field.Scenery[0])
			slices.Reverse(spec.Room.Gameplay.WalkableHexes)
			reordered := s.load(spec)
			s.Equal(compiled.Field.Regions, reordered.Field.Regions)
			s.Equal(compiled.Field.Scenery, reordered.Field.Scenery)
			enc := s.play(compiled,
				encounter.MemberInput{ID: "walker", Kind: encounter.KindPlayer, Position: axial(0, 0)},
				encounter.MemberInput{ID: "other", Kind: encounter.KindPlayer, Position: axial(2, 0)})
			for _, member := range []encounter.MemberID{"walker", "other"} {
				atlas, err := enc.AtlasFor(member)
				s.Require().NoError(err)
				s.Contains(atlas.Cells, axial(1, 0), "the known boundary has floor beneath it")
				s.Contains(atlas.Sealed, axial(1, 0), "footing is not standable")
				found := false
				for _, p := range atlas.Placed {
					if p.ID == id {
						found = true
					}
				}
				s.True(found, "the boundary itself is visible from %s", member)
				if structural {
					s.Len(atlas.StructuralWalls, 1)
				}
				if member == "walker" {
					s.NotContains(atlas.Cells, axial(2, 0))
				} else {
					s.NotContains(atlas.Cells, axial(0, 0))
				}
				restored, err := s.reload(enc).AtlasFor(member)
				s.Require().NoError(err)
				s.Equal(atlas, restored)
			}
			_, err := enc.Step(&encounter.StepInput{Member: "walker", To: axial(1, 0)})
			s.ErrorIs(err, encounter.ErrBadPlacement)

			// Explicit floor secrecy is separate from ordinary boundary
			// knowledge, and does not select the unlisted wall/pillar.
			spec.Concealments = map[string]dungeonspec.ConcealmentSpec{"footing": {
				Checks: dungeonspec.CheckSpec{{Ability: "perception", DC: 20}},
				Cells:  []dungeonspec.RoomCell{{Q: 1, R: 0}},
			}}
			secret := s.play(s.load(spec))
			concealed, err := secret.AtlasFor("walker")
			s.Require().NoError(err)
			s.NotContains(concealed.Cells, axial(1, 0), "boundary knowledge does not discover a selected secret cell")
			present := false
			for _, p := range concealed.Placed {
				if p.ID == id {
					present = true
				}
			}
			s.True(present, "floor secrecy does not conceal an unlisted boundary")
		})
	}
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
