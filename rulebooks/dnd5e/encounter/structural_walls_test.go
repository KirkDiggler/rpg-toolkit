// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package encounter_test

import (
	"encoding/json"
	"math"
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter"
	"github.com/KirkDiggler/rpg-toolkit/tools/spatial"
)

// structural_walls_test.go is P1's encounter-side witness (rpg-project#169):
// the fixed structural layout compiles, copies, round-trips and projects, and
// the per-member read answers on the SAME permitted-identity rule the placed
// contributors use — a hidden parent never conceals an unlisted door, a hidden
// door leaves no tell, and no mutable door state decides a fixed identity.
type StructuralWallSuite struct {
	suite.Suite
}

func TestStructuralWallSuite(t *testing.T) {
	suite.Run(t, new(StructuralWallSuite))
}

// structuralWallRecord is the one authored wall the fixtures share: a
// canonical ten-foot line at the origin, an assembled three-by-0.3 appearance,
// and one two-foot gap bound to the footprint door "vault/gate".
func structuralWallRecord() encounter.StructuralWallInput {
	return encounter.StructuralWallInput{
		ID:        "wall-presence",
		Ref:       "dnd5e:env:test:wall",
		From:      spatial.Point{X: 0, Y: 0},
		To:        spatial.Point{X: 10, Y: 0},
		Height:    3,
		Thickness: 0.3,
		Elevation: 0,
		Openings: []encounter.StructuralOpeningInput{{
			ID: "gap", Position: 5, Width: 2,
			Door: &encounter.StructuralDoorBindingInput{
				PlacedID: "door-presence",
				DoorID:   "vault/gate",
				Ref:      "dnd5e:env:test:door",
				From:     spatial.Point{X: 4, Y: 0},
				To:       spatial.Point{X: 6, Y: 0},
			},
		}},
	}
}

// structuralFootprintDoor is the placement the bound door and its placed
// presence share.
func structuralFootprintDoor() spatial.FootprintPlacement {
	return coveredBox(1, centreOf(cellAt(2, 1)))
}

// structuralField is a one-hall field carrying the wall presence, the door
// presence and the compiled layout beside them.
func structuralField() encounter.FieldInput {
	doorPlacement := structuralFootprintDoor()
	field := placedField(
		placed("wall-presence", coveredBox(6, centreOf(cellAt(1, 1))), false, false),
		placed("door-presence", doorPlacement, false, false),
	)
	field.Doors = []encounter.DoorInput{{ID: "vault/gate", Placement: &doorPlacement, State: encounter.DoorIsClosed()}}
	field.StructuralWalls = []encounter.StructuralWallInput{structuralWallRecord()}

	return field
}

// concealedStructuralField is structuralField with one secret naming the
// supplied placed ids, door ids and cells.
func concealedStructuralField(props []encounter.PropID, doors []encounter.DoorID, cells []spatial.Position) encounter.FieldInput {
	field := structuralField()
	field.Concealments = []encounter.ConcealmentInput{{
		ID: "secret", Checks: vaultCheck(), Props: props, Doors: doors, Cells: cells,
	}}

	return field
}

func (s *StructuralWallSuite) build(field encounter.FieldInput, members ...encounter.MemberInput) (*encounter.Encounter, error) {
	if len(members) == 0 {
		members = []encounter.MemberInput{{ID: "walker", Kind: encounter.KindPlayer, Position: cellAt(0, 0)}}
	}

	return encounter.NewEncounter(&encounter.SetupInput{
		Sight: everyoneSeesTheWholeMap{}, Equipment: noHandsAreObserved{}, Standing: everyoneStanding{},
		Initiative: orderAsGiven{}, TurnDriver: passDriver{}, Striker: passStriker{}, Mover: quietMover{},
		Announcer: quietAnnouncer{}, CheckResolver: findsNothing{}, Witness: nobodyPerceives{},
		Field:   field,
		Members: members,
		Endings: []encounter.EndingInput{{Key: "withdrawn", Trigger: encounter.TriggerExternal{}}},
	})
}

func (s *StructuralWallSuite) play(field encounter.FieldInput, members ...encounter.MemberInput) *encounter.Encounter {
	enc, err := s.build(field, members...)
	s.Require().NoError(err)

	return enc
}

func (s *StructuralWallSuite) reload(enc *encounter.Encounter) *encounter.Encounter {
	raw, err := json.Marshal(enc.ToData())
	s.Require().NoError(err)
	var data encounter.EncounterData
	s.Require().NoError(json.Unmarshal(raw, &data))
	loaded, err := encounter.LoadEncounter(&encounter.LoadEncounterInput{
		Data: data, Sight: everyoneSeesTheWholeMap{}, Equipment: noHandsAreObserved{}, Standing: everyoneStanding{},
		Initiative: orderAsGiven{}, TurnDriver: passDriver{}, Striker: passStriker{}, Mover: quietMover{},
		Announcer: quietAnnouncer{}, CheckResolver: findsNothing{}, Witness: nobodyPerceives{},
	})
	s.Require().NoError(err)

	return loaded
}

// --- The definition compiles, copies and round-trips ---

func (s *StructuralWallSuite) TestTheFullAtlasCarriesEveryDefinitionAndOpening() {
	enc := s.play(structuralField())
	atlas, err := enc.Atlas()
	s.Require().NoError(err)

	s.Require().Len(atlas.StructuralWalls, 1)
	wall := atlas.StructuralWalls[0]
	s.Equal(encounter.PropID("wall-presence"), wall.ID)
	s.Equal("dnd5e:env:test:wall", wall.Ref)
	s.Equal(spatial.Point{X: 0, Y: 0}, wall.From)
	s.Equal(spatial.Point{X: 10, Y: 0}, wall.To)
	s.Equal(3.0, wall.Height)
	s.Equal(0.3, wall.Thickness)
	s.Equal(0.0, wall.Elevation)
	s.Require().Len(wall.Openings, 1)
	s.Equal("gap", wall.Openings[0].ID)
	s.Equal(5.0, wall.Openings[0].Position)
	s.Equal(2.0, wall.Openings[0].Width)

	s.Require().Len(atlas.StructuralDoors, 1)
	door := atlas.StructuralDoors[0]
	s.Equal(encounter.DoorID("vault/gate"), door.ID)
	s.Equal("dnd5e:env:test:door", door.Ref)
	s.Equal(spatial.Point{X: 4, Y: 0}, door.From)
	s.Equal(spatial.Point{X: 6, Y: 0}, door.To)
	s.Equal(3.0, door.Height)
	s.Equal(0.3, door.Thickness)
	s.Equal(0.0, door.Elevation)
}

func (s *StructuralWallSuite) TestMemberProjectionUsesTheFullAtlasIdentityOrder() {
	field := structuralField()
	placement := coveredBox(1, centreOf(cellAt(3, 1)))
	field.Placed = append(field.Placed,
		placed("a-wall", coveredBox(1, centreOf(cellAt(2, 2))), false, false),
		placed("other-door", placement, false, false),
	)
	field.Doors = append(field.Doors, encounter.DoorInput{ID: "vault/z", Placement: &placement, State: encounter.DoorIsClosed()})
	second := structuralWallRecord()
	second.ID = "a-wall"
	second.Openings[0].ID = "other-gap"
	second.Openings[0].Door.PlacedID = "other-door"
	second.Openings[0].Door.DoorID = "vault/z"
	// Wall order and door order intentionally differ; sorting one cannot
	// accidentally make both collections satisfy their identity ordering.
	field.StructuralWalls = append(field.StructuralWalls, second)
	enc := s.play(field)
	full, err := enc.Atlas()
	s.Require().NoError(err)
	view, err := enc.AtlasFor("walker")
	s.Require().NoError(err)
	s.Require().Len(full.StructuralWalls, 2)
	s.Equal("a-wall", full.StructuralWalls[0].ID)
	s.Require().Len(full.StructuralDoors, 2)
	s.Equal("vault/gate", full.StructuralDoors[0].ID)
	s.Equal(full.StructuralWalls, view.StructuralWalls)
	s.Equal(full.StructuralDoors, view.StructuralDoors)
}

func (s *StructuralWallSuite) TestSaveReloadPreservesTheLayoutAndProjection() {
	enc := s.play(structuralField())
	before, err := enc.Atlas()
	s.Require().NoError(err)

	reloaded := s.reload(enc)
	after, err := reloaded.Atlas()
	s.Require().NoError(err)
	s.Equal(before.StructuralWalls, after.StructuralWalls)
	s.Equal(before.StructuralDoors, after.StructuralDoors)

	view, err := reloaded.AtlasFor("walker")
	s.Require().NoError(err)
	s.Equal(before.StructuralWalls, view.StructuralWalls)
	s.Equal(before.StructuralDoors, view.StructuralDoors)
}

func (s *StructuralWallSuite) TestNeitherTheInputNorTheOutputAliasesTheEncounter() {
	field := structuralField()
	enc := s.play(field)
	original, err := enc.Atlas()
	s.Require().NoError(err)

	// Mutating the input the caller handed in must not reach the field.
	field.StructuralWalls[0].From.X = 999
	field.StructuralWalls[0].Height = 999
	field.StructuralWalls[0].Openings[0].Position = 999
	field.StructuralWalls[0].Openings[0].Door.From.X = 999
	unchanged, err := enc.Atlas()
	s.Require().NoError(err)
	s.Equal(original.StructuralWalls, unchanged.StructuralWalls)
	s.Equal(original.StructuralDoors, unchanged.StructuralDoors)

	// Mutating a returned atlas must not reach the next read.
	unchanged.StructuralWalls[0].Openings[0].Position = 999
	unchanged.StructuralWalls[0].From.X = 999
	unchanged.StructuralDoors[0].From.X = 999
	again, err := enc.Atlas()
	s.Require().NoError(err)
	s.Equal(original.StructuralWalls, again.StructuralWalls)
	s.Equal(original.StructuralDoors, again.StructuralDoors)

	// Mutating a ToData result must not reach the field either.
	data := enc.ToData()
	data.Field.StructuralWalls[0].Openings[0].Position = 999
	data.Field.StructuralWalls[0].Openings[0].Door.DoorID = "tampered"
	roundTripped, err := enc.Atlas()
	s.Require().NoError(err)
	s.Equal(original.StructuralWalls, roundTripped.StructuralWalls)
	s.Equal(original.StructuralDoors, roundTripped.StructuralDoors)
}

// --- Definitions refuse broken references and geometry ---

func (s *StructuralWallSuite) TestABrokenStructuralDefinitionRefusesWithoutAnEncounter() {
	edgeDoor := func(field *encounter.FieldInput) {
		field.Doors = append(field.Doors, encounter.DoorInput{
			ID: "edge/gate", Edges: []encounter.DoorEdge{{From: cellAt(0, 0), To: cellAt(1, 0)}},
			State: encounter.DoorIsClosed(),
		})
	}
	secondWall := func(id string, openings ...encounter.StructuralOpeningInput) func(*encounter.FieldInput) {
		return func(field *encounter.FieldInput) {
			record := structuralWallRecord()
			record.ID = id
			record.Openings = openings
			field.StructuralWalls = append(field.StructuralWalls, record)
		}
	}

	cases := map[string]func(*encounter.FieldInput){
		"missing wall placed id": func(f *encounter.FieldInput) {
			f.StructuralWalls[0].ID = "nowhere"
		},
		"holdable wall placement": func(f *encounter.FieldInput) {
			f.Placed[0].Holdable = true
		},
		"missing door placed id": func(f *encounter.FieldInput) {
			f.StructuralWalls[0].Openings[0].Door.PlacedID = "nowhere"
		},
		"holdable door placement": func(f *encounter.FieldInput) {
			f.Placed[1].Holdable = true
		},
		"door id names nothing": func(f *encounter.FieldInput) {
			f.StructuralWalls[0].Openings[0].Door.DoorID = "absent/gate"
		},
		"door id names an edge door": func(f *encounter.FieldInput) {
			edgeDoor(f)
			f.StructuralWalls[0].Openings[0].Door.DoorID = "edge/gate"
		},
		"duplicate wall ids": secondWall("wall-presence"),
		"zero height": func(f *encounter.FieldInput) {
			f.StructuralWalls[0].Height = 0
		},
		"negative thickness": func(f *encounter.FieldInput) {
			f.StructuralWalls[0].Thickness = -1
		},
		"nonfinite coordinate": func(f *encounter.FieldInput) {
			f.StructuralWalls[0].From.X = math.Inf(1)
		},
		"nonfinite elevation": func(f *encounter.FieldInput) {
			f.StructuralWalls[0].Elevation = math.NaN()
		},
		"zero-length line": func(f *encounter.FieldInput) {
			f.StructuralWalls[0].To = f.StructuralWalls[0].From
		},
		"zero-width opening": func(f *encounter.FieldInput) {
			f.StructuralWalls[0].Openings[0].Width = 0
		},
		"opening outside the line": func(f *encounter.FieldInput) {
			f.StructuralWalls[0].Openings[0].Position = 11
		},
		"overlapping openings": func(f *encounter.FieldInput) {
			f.StructuralWalls[0].Openings = append(f.StructuralWalls[0].Openings, encounter.StructuralOpeningInput{ID: "second", Position: 5.5, Width: 2})
		},
		"duplicate opening id": func(f *encounter.FieldInput) {
			f.StructuralWalls[0].Openings = append(f.StructuralWalls[0].Openings, encounter.StructuralOpeningInput{ID: "gap", Position: 1, Width: 1})
		},
		"duplicate door binding": func(f *encounter.FieldInput) {
			f.StructuralWalls[0].Openings = append(f.StructuralWalls[0].Openings, encounter.StructuralOpeningInput{
				ID: "second", Position: 8, Width: 1,
				Door: &encounter.StructuralDoorBindingInput{
					PlacedID: "door-presence", DoorID: "vault/gate", Ref: "dnd5e:env:test:door",
					From: spatial.Point{X: 7.5, Y: 0}, To: spatial.Point{X: 8.5, Y: 0},
				},
			})
		},
		"door endpoints not distinct": func(f *encounter.FieldInput) {
			f.StructuralWalls[0].Openings[0].Door.To = f.StructuralWalls[0].Openings[0].Door.From
		},
		"opening id collides across walls": func(f *encounter.FieldInput) {
			f.Placed = append(f.Placed, placed("other-presence", coveredBox(1, centreOf(cellAt(3, 3))), false, false))
			secondWall("other-presence", encounter.StructuralOpeningInput{ID: "gap", Position: 1, Width: 1})(f)
		},
	}

	for name, mutate := range cases {
		s.Run(name, func() {
			field := structuralField()
			mutate(&field)
			enc, err := s.build(field)
			s.Require().Error(err, "the field must be refused")
			s.Nil(enc, "a refusal produces no encounter")
		})
	}
}

func (s *StructuralWallSuite) TestANegativeElevationIsLegal() {
	field := structuralField()
	field.StructuralWalls[0].Elevation = -1.5
	enc := s.play(field)
	atlas, err := enc.Atlas()
	s.Require().NoError(err)
	s.Require().Len(atlas.StructuralWalls, 1)
	s.Equal(-1.5, atlas.StructuralWalls[0].Elevation)
}

// --- The per-member read: explicit membership, no parent-dependent subtype ---

func (s *StructuralWallSuite) TestAKnownDoorIsPresentWithoutAnyDoorSighting() {
	// No state is consulted, so the layout is exactly as authored whether or
	// not a mutable sighting exists.
	enc := s.play(structuralField())
	view, err := enc.AtlasFor("walker")
	s.Require().NoError(err)
	s.Require().Len(view.StructuralWalls, 1)
	s.Require().Len(view.StructuralWalls[0].Openings, 1)
	s.Require().Len(view.StructuralDoors, 1)
	s.Equal(encounter.DoorID("vault/gate"), view.StructuralDoors[0].ID)

	// Opening the door is a state change and leaves the fixed layout alone.
	_, err = enc.OpenDoor(&encounter.OpenDoorInput{Door: "vault/gate"})
	s.Require().NoError(err)
	after, err := enc.AtlasFor("walker")
	s.Require().NoError(err)
	s.Equal(view.StructuralWalls, after.StructuralWalls)
	s.Equal(view.StructuralDoors, after.StructuralDoors)
}

func (s *StructuralWallSuite) TestAnExplicitlyHiddenDoorLeavesNoTell() {
	field := concealedStructuralField([]encounter.PropID{"door-presence"}, []encounter.DoorID{"vault/gate"}, nil)
	enc := s.play(field)

	// The author atlas keeps the whole definition.
	full, err := enc.Atlas()
	s.Require().NoError(err)
	s.Require().Len(full.StructuralWalls, 1)
	s.Len(full.StructuralWalls[0].Openings, 1)
	s.Len(full.StructuralDoors, 1)

	view, err := enc.AtlasFor("walker")
	s.Require().NoError(err)
	s.Require().Len(view.StructuralWalls, 1, "the parent wall is not concealed")
	s.Empty(view.StructuralWalls[0].Openings, "the whole opening is omitted, id and width included")
	s.Empty(view.StructuralDoors, "and no independent door row is emitted")
}

func (s *StructuralWallSuite) TestADoorIdentityAloneHidesTheOpening() {
	// The placed presence survives, but the canonical DoorID is concealed: the
	// identity answer is what withholds the cut, not the placed list.
	field := concealedStructuralField(nil, []encounter.DoorID{"vault/gate"}, nil)
	enc := s.play(field)
	view, err := enc.AtlasFor("walker")
	s.Require().NoError(err)
	s.Require().Len(view.StructuralWalls, 1)
	s.Empty(view.StructuralWalls[0].Openings)
	s.Empty(view.StructuralDoors)
}

func (s *StructuralWallSuite) TestConcealedDoorDoesNotLeakItsExplicitPlacedRepresentation() {
	field := concealedStructuralField(nil, []encounter.DoorID{"vault/gate"}, nil)
	field.Placed = append(field.Placed, placed("unlisted-overlap", structuralFootprintDoor(), false, false))
	enc := s.play(field)
	for _, current := range []*encounter.Encounter{enc, s.reload(enc)} {
		view, err := current.AtlasFor("walker")
		s.Require().NoError(err)
		ids := make([]string, 0, len(view.Placed))
		for _, p := range view.Placed {
			ids = append(ids, p.ID)
		}
		s.NotContains(ids, "door-presence", "the declared representation of the concealed door must not leak its identity or rectangle")
		s.Contains(ids, "unlisted-overlap", "an unrelated overlapping placement is not the door's representation")
		s.Empty(view.StructuralDoors)
	}
}

func (s *StructuralWallSuite) TestAHiddenParentLeavesAnUnlistedDoorStanding() {
	field := concealedStructuralField([]encounter.PropID{"wall-presence"}, nil, nil)
	enc := s.play(field)
	view, err := enc.AtlasFor("walker")
	s.Require().NoError(err)
	s.Empty(view.StructuralWalls, "the concealed wall is withheld whole")
	s.Require().Len(view.StructuralDoors, 1, "the unlisted door is independently permitted")
	s.Equal(encounter.DoorID("vault/gate"), view.StructuralDoors[0].ID)
	s.Equal(spatial.Point{X: 4, Y: 0}, view.StructuralDoors[0].From)
	// The door carries no parent id — the type has none — so a client places
	// it without the withheld wall's identity.
}

func (s *StructuralWallSuite) TestAHiddenParentAndDoorLeaveNothing() {
	field := concealedStructuralField(
		[]encounter.PropID{"wall-presence", "door-presence"}, []encounter.DoorID{"vault/gate"}, nil)
	enc := s.play(field)
	view, err := enc.AtlasFor("walker")
	s.Require().NoError(err)
	s.Empty(view.StructuralWalls)
	s.Empty(view.StructuralDoors)
}

func (s *StructuralWallSuite) TestAConcealedCellDoesNotSelectAnUnlistedWallOrDoor() {
	// Explicit concealment of a floor cell is not membership for a placed
	// thing: an unlisted wall and door over concealed floor stay permitted.
	field := concealedStructuralField(nil, nil, []spatial.Position{cellAt(4, 4)})
	enc := s.play(field)
	view, err := enc.AtlasFor("walker")
	s.Require().NoError(err)
	s.Require().Len(view.StructuralWalls, 1)
	s.Len(view.StructuralWalls[0].Openings, 1)
	s.Len(view.StructuralDoors, 1)
}

func (s *StructuralWallSuite) TestAnUnknownOrdinaryRoomWithholdsItsLayout() {
	doorPlacement := coveredBox(1, centreOf(cellAt(5, 1)))
	field := encounter.FieldInput{
		Canvas: pointyCanvas(),
		Regions: []encounter.RegionInput{
			rectRegion("entry", 0, 0, 3, 5),
			rectRegion("room", 3, 0, 4, 5),
		},
		Walls: seamWallExcept(2, 5, 1),
		Doors: []encounter.DoorInput{
			{ID: "entry-door", Edges: doorEdgesAcross(2, 1), State: encounter.DoorIsClosed()},
			{ID: "vault/gate", Placement: &doorPlacement, State: encounter.DoorIsClosed()},
		},
		Placed: []encounter.PlacedPropInput{
			placed("wall-presence", coveredBox(6, centreOf(cellAt(4, 1))), false, false),
			placed("door-presence", doorPlacement, false, false),
		},
		StructuralWalls: []encounter.StructuralWallInput{structuralWallRecord()},
	}
	enc := s.play(field, encounter.MemberInput{ID: "walker", Kind: encounter.KindPlayer, Position: cellAt(1, 1)})

	full, err := enc.Atlas()
	s.Require().NoError(err)
	s.Len(full.StructuralWalls, 1, "the author atlas has every valid definition")
	s.Len(full.StructuralDoors, 1)

	view, err := enc.AtlasFor("walker")
	s.Require().NoError(err)
	s.Empty(view.StructuralWalls, "the undiscovered room's layout is withheld")
	s.Empty(view.StructuralDoors)

	// Opening the way through teaches the room, and the fixed layout arrives
	// through the existing discovery path with no new rule.
	_, err = enc.OpenDoor(&encounter.OpenDoorInput{Door: "entry-door", Actor: "walker"})
	s.Require().NoError(err)
	learned, err := enc.AtlasFor("walker")
	s.Require().NoError(err)
	s.Require().Len(learned.StructuralWalls, 1)
	s.Require().Len(learned.StructuralDoors, 1)
}
