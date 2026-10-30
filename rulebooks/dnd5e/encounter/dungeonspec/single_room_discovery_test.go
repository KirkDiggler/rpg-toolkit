// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package dungeonspec_test

import (
	"encoding/json"
	"fmt"
	"slices"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter/dungeonspec"
	"github.com/KirkDiggler/rpg-toolkit/tools/spatial"
)

// partitionGroups is the compiled field's wall partitions as the encounter
// derives them — each partition's absolute cells, sorted, the partitions in
// order of their first cell — so two compiles can be compared by what the
// walls make of them, whatever the regions are called (rpg-project#725).
func (s *SingleRoomWallDoorSuite) partitionGroups(compiled dungeonspec.Compiled) [][]spatial.Position {
	enc := s.play(compiled)
	byID := map[encounter.PartitionID][]spatial.Position{}
	for _, r := range compiled.Field.Regions {
		for _, c := range r.Cells {
			abs := encounter.HexCellAt(compiled.Field.Canvas.Orientation, int(c.X), int(c.Y))
			id, ok := enc.PartitionAt(abs)
			s.Require().True(ok, "every owned cell is in a partition")
			byID[id] = append(byID[id], abs)
		}
	}
	out := make([][]spatial.Position, 0, len(byID))
	for _, cells := range byID {
		slices.SortFunc(cells, func(a, b spatial.Position) int {
			if a.Y != b.Y {
				return int(a.Y - b.Y)
			}
			return int(a.X - b.X)
		})
		out = append(out, cells)
	}
	slices.SortFunc(out, func(a, b []spatial.Position) int {
		if a[0].Y != b[0].Y {
			return int(a[0].Y - b[0].Y)
		}
		return int(a[0].X - b[0].X)
	})
	return out
}

// splitInReverse re-authors a room's one region as two — the cells west of q
// and the rest — listed east first: the same floor in another authoring.
func splitInReverse(spec *dungeonspec.SingleRoomSpec, q int) {
	var west, east []dungeonspec.RoomCell
	for _, c := range dungeonspec.Floor(&spec.Room.Gameplay) {
		if c.Q < q {
			west = append(west, c)
		} else {
			east = append(east, c)
		}
	}
	spec.Room.Gameplay.Regions = []dungeonspec.RoomRegion{
		dungeonspec.OneRegion("region-east", "East", east),
		dungeonspec.OneRegion(spec.Room.Gameplay.ImplicitRegionID, "West", west),
	}
}

// TestOrdinaryDiscoveryPartitionsStayFixedAndSeatsStayOnTheStartingSide: one
// authored region cut by a wall with a door is two partitions; the party's
// seats stay in the start's, every member reports the authored region, and
// neither the door's state nor how the floor is authored into regions renames
// or merges a partition (rpg-project#725: the walls' answer, not the
// regions').
func (s *SingleRoomWallDoorSuite) TestOrdinaryDiscoveryPartitionsStayFixedAndSeatsStayOnTheStartingSide() {
	spec := s.gapSpec(dungeonspec.RoomDoorBinding{Closed: true})
	spec.Room.Gameplay.MonsterDeclarations = []dungeonspec.RoomMonsterSource{{
		ID: "far-skeleton", Ref: "dnd5e:monsters:skeleton",
		StartingCell: dungeonspec.RoomStartingCell{Location: dungeonspec.RoomCell{Q: 4, R: 0}},
	}}
	closed := s.load(spec)
	s.Require().Len(closed.Field.Regions, 1, "one authored region")
	groups := s.partitionGroups(closed)
	s.Require().Len(groups, 2, "the wall makes two rooms of it")
	s.Require().Len(closed.PartyStart, 1, "the seats stay on the start's side of the wall")
	s.Equal("room-1-region", closed.PartyStart[0].Region, "a seat reports its authored region")
	s.Equal("room-1-region", closed.Monsters[0].Region, "so does a monster")
	spec.Room.Gameplay.DoorBindings["gap-door"] = dungeonspec.RoomDoorBinding{}
	splitInReverse(spec, 2)
	opened := s.load(spec)
	s.Require().Len(opened.Field.Regions, 2)
	s.Equal(groups, s.partitionGroups(opened), "door state and authoring do not rename or merge partitions")
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
	s.Require().Len(s.partitionGroups(compiled), 3, "two walls, three rooms")
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
		if payload.Beat == encounter.BeatCellsRevealed {
			for _, placed := range payload.Placed {
				delivered[placed.ID] = true
			}
		}
	}
	for _, placed := range atlas.Placed {
		if !known[placed.ID] {
			s.True(delivered[placed.ID], "newly permitted boundary %q must arrive in a cells reveal, as its snapshot does", placed.ID)
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
			Conditions:    encounter.UnobservedEquipment{},
			Participation: everyoneStanding{},
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
			groups := s.partitionGroups(compiled)
			s.Require().Len(groups, 2, "never join the rooms or mint a third phantom room")
			s.Require().Len(compiled.Field.Scenery, 1)
			s.Equal(axial(1, 0), compiled.Field.Scenery[0])
			splitInReverse(spec, 1)
			reordered := s.load(spec)
			s.Equal(groups, s.partitionGroups(reordered))
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
	var floor []dungeonspec.RoomCell
	for q := 0; q <= 3; q++ {
		for r := -1; r <= 1; r++ {
			floor = append(floor, dungeonspec.RoomCell{Q: q, R: r})
		}
	}
	dungeonspec.SetFloor(&spec.Room.Gameplay, floor)
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

// footingSpec is TestOrdinaryDiscoveryOpaqueFootingIsVisibleFromBothSides'
// structural room: the corridor (0,0)..(4,0) with a wall whose footprint
// covers (1,0)'s centre, so (1,0) is opaque floor between two rooms.
func (s *SingleRoomWallDoorSuite) footingSpec() *dungeonspec.SingleRoomSpec {
	spec := s.gapSpec(dungeonspec.RoomDoorBinding{Closed: true})
	spec.Room.Gameplay.DoorBindings = nil
	x := boundaryWallX(5)
	spec.Room.Gameplay.Walls = []dungeonspec.RoomWall{roomWall("boundary", x, -2, x, 2,
		wallBlocks(true, true, 4, boundaryWallX(1.5), 0, 0))}
	return spec
}

// TestCompile_FootingLeavesItsRegion: wall footing no room can claim leaves
// the AUTHORED region that painted it and is carried as scenery, whichever
// region that is (rpg-project#725). A region whose every painted cell is
// footing is refused at its cells — paint the author must see is not floor —
// where a region listed with no cells at all is declared and accepted.
func (s *SingleRoomWallDoorSuite) TestCompile_FootingLeavesItsRegion() {
	spec := s.footingSpec()
	spec.Room.Gameplay.Regions = []dungeonspec.RoomRegion{
		dungeonspec.OneRegion("room-1-region", "West", []dungeonspec.RoomCell{{Q: 0, R: 0}}),
		dungeonspec.OneRegion("region-east", "East", []dungeonspec.RoomCell{{Q: 1, R: 0}, {Q: 2, R: 0}, {Q: 3, R: 0}, {Q: 4, R: 0}}),
	}
	compiled := s.load(spec)
	s.Equal([]spatial.Position{axial(1, 0)}, compiled.Field.Scenery)
	s.Require().Len(compiled.Field.Regions, 2)
	s.Equal("region-east", compiled.Field.Regions[1].ID)
	s.NotContains(compiled.Field.Regions[1].Cells, axial(1, 0), "the footing left the region that painted it")
	s.Len(compiled.Field.Regions[1].Cells, 3)

	spec.Room.Gameplay.Regions = []dungeonspec.RoomRegion{
		dungeonspec.OneRegion("room-1-region", "Corridor", []dungeonspec.RoomCell{{Q: 0, R: 0}, {Q: 2, R: 0}, {Q: 3, R: 0}, {Q: 4, R: 0}}),
		dungeonspec.OneRegion("region-wall", "Wall", []dungeonspec.RoomCell{{Q: 1, R: 0}}),
	}
	_, err := dungeonspec.Load(s.encoded(spec))
	var validation *dungeonspec.ValidationError
	s.Require().ErrorAs(err, &validation)
	s.Equal([]dungeonspec.FieldError{{Path: "room.room.regions[1].cells", Message: "every cell is wall footing"}}, validation.Errors)
}

// TestFootingKeptAsFloorIsARoomOfItsOwn: a field authored in code may keep
// permanent opaque footing as owned floor, which the compiler never does. The
// encounter's own derivation makes each such cell a room of its own, so
// nothing hears or is seated across it, and the rooms either side stay apart.
func (s *SingleRoomWallDoorSuite) TestFootingKeptAsFloorIsARoomOfItsOwn() {
	compiled := s.load(s.footingSpec())
	s.Require().Equal([]spatial.Position{axial(1, 0)}, compiled.Field.Scenery)
	compiled.Field.Regions[0].Cells = append(compiled.Field.Regions[0].Cells, compiled.Field.Scenery...)
	compiled.Field.Scenery = nil
	enc := s.play(compiled)
	at := func(q int) encounter.PartitionID {
		cell := encounter.HexCellAt(compiled.Field.Canvas.Orientation, int(axial(q, 0).X), int(axial(q, 0).Y))
		p, ok := enc.PartitionAt(cell)
		s.Require().True(ok, "(%d,0) is owned floor", q)
		return p
	}
	west, footing, east := at(0), at(1), at(2)
	s.NotEqual(west, east)
	s.NotEqual(footing, west)
	s.NotEqual(footing, east)
}
