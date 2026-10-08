// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package encounter_test

import (
	"encoding/json"
	"testing"

	"github.com/KirkDiggler/rpg-toolkit/mind/perception"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter"
	"github.com/KirkDiggler/rpg-toolkit/tools/spatial"
	"github.com/stretchr/testify/suite"
)

const propObserver encounter.MemberID = "prop|b"

type PropObservationSuite struct {
	suite.Suite
	enc   *encounter.Encounter
	sight *sightList
}

func TestPropObservationSuite(t *testing.T) { suite.Run(t, new(PropObservationSuite)) }

func (s *PropObservationSuite) SetupTest() {
	field := doorField(3, encounter.DoorIsOpen(), "gate", 1)
	field.Props = []encounter.PropInput{{ID: "b", Ref: "test:props:chest", Holdable: true, At: spatial.Position{X: 4, Y: 1}, BlocksMovement: boolPtr(false), BlocksLineOfSight: boolPtr(false)}}
	field.PropPresentations = []encounter.PropPresentation{{ID: "b", Ref: "test:props:chest", Origin: centreOf(cellAt(4, 1)), Elevation: 2, FacingDegrees: 17, HeightScale: 1.5}}
	s.sight = &sightList{fallback: 20}
	var err error
	s.enc, err = encounter.NewEncounter(&encounter.SetupInput{
		Field: field, Sight: s.sight, Equipment: encounter.UnobservedEquipment{}, Sheets: zeroSheets{}, Standing: everyoneStanding{},
		Initiative: orderAsGiven{}, TurnDriver: passDriver{}, Striker: passStriker{}, Mover: quietMover{}, Announcer: quietAnnouncer{},
		Members: []encounter.MemberInput{{ID: propObserver, Kind: encounter.KindPlayer, Position: spatial.Position{X: 2, Y: 1}}, {ID: "b", Kind: encounter.KindPlayer, Position: spatial.Position{X: 3, Y: 1}}},
		Endings: []encounter.EndingInput{{Key: "end", Trigger: encounter.TriggerExternal{}}},
	})
	s.Require().NoError(err)
}

func (s *PropObservationSuite) prop(observer encounter.MemberID) encounter.PropSighting {
	sightings, err := s.enc.PropSightings(&encounter.ViewInput{Member: observer})
	s.Require().NoError(err)
	s.Require().Len(sightings, 1)
	return sightings[0]
}

func (s *PropObservationSuite) door(observer encounter.MemberID) encounter.DoorSighting {
	sightings, err := s.enc.DoorSightings(&encounter.ViewInput{Member: observer})
	s.Require().NoError(err)
	s.Require().Len(sightings, 1)
	return sightings[0]
}

func (s *PropObservationSuite) withdraw() {
	for _, x := range []int{1, 0} {
		_, err := s.enc.Step(&encounter.StepInput{Member: propObserver, To: cellAt(x, 1)})
		s.Require().NoError(err)
	}
	s.sight.reach = map[encounter.MemberID]int{propObserver: 1}
	_, err := s.enc.Recheck(&encounter.RecheckInput{Members: []encounter.MemberID{propObserver}})
	s.Require().NoError(err)
}

func (s *PropObservationSuite) reload() {
	raw, err := json.Marshal(s.enc.ToData())
	s.Require().NoError(err)
	var data encounter.EncounterData
	s.Require().NoError(json.Unmarshal(raw, &data))
	s.enc, err = encounter.LoadEncounter(&encounter.LoadEncounterInput{
		Data: data, Sight: s.sight, Equipment: encounter.UnobservedEquipment{}, Sheets: zeroSheets{}, Standing: everyoneStanding{}, Initiative: orderAsGiven{}, TurnDriver: passDriver{}, Striker: passStriker{}, Mover: quietMover{}, Announcer: quietAnnouncer{},
	})
	s.Require().NoError(err)
}

func (s *PropObservationSuite) TestFootprintMemoryKeepsItsShapeAndNeedsCompleteEmptyEvidence() {
	field := doorField(3, encounter.DoorIsOpen(), "gate", 1)
	box := placed("b", coveredBox(12, centreOf(cellAt(4, 1))), false, false)
	box.Holdable = true
	field.Placed = []encounter.PlacedPropInput{box}
	field.PropPresentations = []encounter.PropPresentation{{ID: "b", Ref: "test:props:chest", Origin: box.Placement.Origin, HeightScale: 1.25, Elevation: 2}}
	var err error
	s.enc, err = encounter.NewEncounter(&encounter.SetupInput{
		Field: field, Sight: s.sight, Equipment: encounter.UnobservedEquipment{}, Sheets: zeroSheets{}, Standing: everyoneStanding{}, Initiative: orderAsGiven{},
		TurnDriver: passDriver{}, Striker: passStriker{}, Mover: quietMover{}, Announcer: quietAnnouncer{},
		Members: []encounter.MemberInput{{ID: propObserver, Kind: encounter.KindPlayer, Position: spatial.Position{X: 2, Y: 1}}, {ID: "b", Kind: encounter.KindPlayer, Position: spatial.Position{X: 3, Y: 1}}},
		Endings: []encounter.EndingInput{{Key: "end", Trigger: encounter.TriggerExternal{}}},
	})
	s.Require().NoError(err)
	before := s.prop(propObserver)
	s.Require().NotNil(before.Placed)
	s.Require().NotNil(before.Presentation)
	s.Equal(box.Placement.Origin, before.Presentation.Origin)
	s.Equal(1.25, before.Presentation.HeightScale)
	s.Greater(len(before.Placed.Cells), 1)
	s.Equal(box.Placement, before.Placed.Placement)
	s.withdraw()
	_, err = s.enc.Hold(&encounter.HoldInput{Member: "b", Target: "b"})
	s.Require().NoError(err)
	s.sight.reach[propObserver] = 3
	canvas, err := s.enc.Canvas()
	s.Require().NoError(err)
	visible := 0
	for _, cell := range before.Placed.Cells {
		if s.enc.Distance(cellAt(0, 1), cell) <= 3 && !canvas.IsLineOfSightBlocked(cellAt(0, 1), cell) {
			visible++
		}
	}
	s.Greater(visible, 0, "the negative control really observes part of the old footprint")
	s.Less(visible, len(before.Placed.Cells))
	_, err = s.enc.Recheck(&encounter.RecheckInput{Members: []encounter.MemberID{propObserver}})
	s.Require().NoError(err)
	s.False(s.prop(propObserver).ObservedEmpty, "partial footprint visibility is not a complete absence witness")
	s.Equal(before.Placed, s.prop(propObserver).Placed)
	s.Equal(before.Presentation, s.prop(propObserver).Presentation)
	s.reload()
	s.Equal(before.Placed, s.prop(propObserver).Placed)
	s.Equal(before.Presentation, s.prop(propObserver).Presentation)
	s.sight.reach[propObserver] = 20
	_, err = s.enc.Step(&encounter.StepInput{Member: "b", To: cellAt(3, 0)})
	s.Require().NoError(err)
	for _, x := range []int{1, 2, 3} {
		_, err = s.enc.Step(&encounter.StepInput{Member: propObserver, To: cellAt(x, 1)})
		s.Require().NoError(err)
	}
	s.True(s.prop(propObserver).ObservedEmpty)
	s.Empty(s.prop(propObserver).Placed.Cells)
	s.Nil(s.prop(propObserver).Presentation)
}

func (s *PropObservationSuite) TestRoomLayoutDoesNotDiscloseAnOccludedProp() {
	field := doorField(3, encounter.DoorIsClosed(), "gate", 1)
	field.Walls = append(field.Walls, seamWallExcept(4, 3)...)
	field.Props = []encounter.PropInput{
		{ID: "chest", Ref: "test:props:chest", Holdable: true, At: spatial.Position{X: 5, Y: 1}, BlocksMovement: boolPtr(false), BlocksLineOfSight: boolPtr(false)},
		{ID: "scenery", Ref: "test:props:statue", At: spatial.Position{X: 5, Y: 2}, BlocksMovement: boolPtr(true), BlocksLineOfSight: boolPtr(false)},
	}
	var err error
	s.enc, err = encounter.NewEncounter(&encounter.SetupInput{
		Field: field, Sight: s.sight, Equipment: encounter.UnobservedEquipment{}, Sheets: zeroSheets{}, Standing: everyoneStanding{}, Initiative: orderAsGiven{},
		TurnDriver: passDriver{}, Striker: passStriker{}, Mover: quietMover{}, Announcer: quietAnnouncer{},
		Members: []encounter.MemberInput{{ID: propObserver, Kind: encounter.KindPlayer, Position: spatial.Position{X: 2, Y: 1}}},
		Endings: []encounter.EndingInput{{Key: "end", Trigger: encounter.TriggerExternal{}}},
	})
	s.Require().NoError(err)
	_, err = s.enc.OpenDoor(&encounter.OpenDoorInput{Door: "gate", Actor: propObserver})
	s.Require().NoError(err)
	atlas, err := s.enc.AtlasFor(propObserver)
	s.Require().NoError(err)
	s.Contains(atlas.Cells, cellAt(5, 1), "floor behind the internal obstruction is learned")
	s.Require().Len(atlas.Props, 1)
	s.Equal("scenery", atlas.Props[0].ID, "fixed scenery follows the room, not internal LOS")
	sightings, err := s.enc.PropSightings(&encounter.ViewInput{Member: propObserver})
	s.Require().NoError(err)
	s.Empty(sightings, "the chest is not room geometry")
	s.reload()
	sightings, err = s.enc.PropSightings(&encounter.ViewInput{Member: propObserver})
	s.Require().NoError(err)
	s.Empty(sightings)
}

func (s *PropObservationSuite) TestCreatureAndPropWithTheSameIDRemainDistinct() {
	prop := s.prop(propObserver)
	s.Equal("b", prop.Prop.ID)
	s.Contains(prop.CurrentVia, perception.Sight)
	view, err := s.enc.View(&encounter.ViewInput{Member: propObserver})
	s.Require().NoError(err)
	s.Require().Len(view, 1, "the creature view does not pretend props or doors are members")
	s.Equal(encounter.MemberID("b"), view[0].Subject)
	s.True(view[0].CurrentOn(perception.Sight))
	// Even an observer whose raw ID equals the prop's qualified ID sees it.
	s.reload()
	s.Contains(s.prop(propObserver).CurrentVia, perception.Sight)
}

func (s *PropObservationSuite) TestPresentationMemoryNeverJoinsAnUnseenDropPose() {
	before := s.prop(propObserver)
	s.Require().NotNil(before.Presentation)
	s.Equal(2.0, before.Presentation.Elevation)
	atlas, err := s.enc.AtlasFor(propObserver)
	s.Require().NoError(err)
	s.Empty(atlas.PropPresentations, "mutable appearance is not fixed room geometry")
	s.withdraw()
	_, err = s.enc.Hold(&encounter.HoldInput{Member: "b", Target: "b"})
	s.Require().NoError(err)
	s.Nil(s.prop("b").Presentation, "witnessed empty has no render pose")
	_, err = s.enc.Exit(&encounter.ExitInput{Member: "b"})
	s.Require().NoError(err)
	s.Equal(before.Presentation, s.prop(propObserver).Presentation, "unseen drop must not move the remembered picture")
	s.reload()
	s.Equal(before.Presentation, s.prop(propObserver).Presentation)
	s.sight.reach[propObserver] = 20
	_, err = s.enc.Recheck(&encounter.RecheckInput{Members: []encounter.MemberID{propObserver}})
	s.Require().NoError(err)
	after := s.prop(propObserver)
	s.Require().NotNil(after.Presentation)
	s.Zero(after.Presentation.Elevation, "the existing drop fact places it on the floor")
	s.Equal(centreOf(cellAt(3, 1)), after.Presentation.Origin)
	s.Equal(before.Presentation.Ref, after.Presentation.Ref)
	s.Equal(before.Presentation.HeightScale, after.Presentation.HeightScale)
}

func (s *PropObservationSuite) TestUnseenPickupAndClosePreserveMemoryUntilEmptyIsObserved() {
	before := s.prop(propObserver)
	s.withdraw()
	s.Empty(s.prop(propObserver).CurrentVia)
	next, err := s.enc.NextStorySeq()
	s.Require().NoError(err)
	_, err = s.enc.Hold(&encounter.HoldInput{Member: "b", Target: "b"})
	s.Require().NoError(err)
	_, err = s.enc.CloseDoor(&encounter.CloseDoorInput{Actor: "b", Door: "gate"})
	s.Require().NoError(err)
	s.Equal(before.Prop, s.prop(propObserver).Prop)
	s.False(s.prop(propObserver).ObservedEmpty)
	s.Equal(encounter.DoorOpen, s.door(propObserver).Door.State.Kind())
	s.Equal(encounter.DoorClosed, s.door("b").Door.State.Kind())
	s.True(s.prop("b").ObservedEmpty, "the witness knows the placement was vacated")
	held, err := s.enc.HeldProps(&encounter.ViewInput{Member: "b"})
	s.Require().NoError(err)
	s.Equal([]encounter.PropID{"b"}, held)
	held, err = s.enc.HeldProps(&encounter.ViewInput{Member: propObserver})
	s.Require().NoError(err)
	s.Empty(held, "another observer does not inherit the carrier's inventory")
	unseen, err := s.enc.Story(&encounter.StoryInput{Audience: propObserver, AfterSeq: next})
	s.Require().NoError(err)
	s.Empty(unseen, "unseen pickup/close does not send facts or a refresh notification")
	legacy, err := s.enc.DoorsFor(propObserver)
	s.Require().NoError(err)
	s.Equal(encounter.DoorOpen, legacy[0].State.Kind(), "the older scoped read is not a live-state bypass")
	s.reload()
	s.Equal(before.Prop, s.prop(propObserver).Prop)
	s.Equal(encounter.DoorOpen, s.door(propObserver).Door.State.Kind())
	// A can now inspect the door, but cannot see the former prop position.
	s.sight.reach[propObserver] = 20
	_, err = s.enc.Recheck(&encounter.RecheckInput{Members: []encounter.MemberID{propObserver}})
	s.Require().NoError(err)
	s.Equal(encounter.DoorClosed, s.door(propObserver).Door.State.Kind())
	s.False(s.prop(propObserver).ObservedEmpty)
	_, err = s.enc.OpenDoor(&encounter.OpenDoorInput{Actor: "b", Door: "gate"})
	s.Require().NoError(err)
	s.True(s.prop(propObserver).ObservedEmpty)
	s.Empty(s.prop(propObserver).CurrentVia)
	s.reload()
	s.True(s.prop(propObserver).ObservedEmpty)
}
