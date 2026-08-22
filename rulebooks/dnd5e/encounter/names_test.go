// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package encounter_test

import (
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/KirkDiggler/rpg-toolkit/tools/spatial"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter"
)

// NamesTestSuite covers rpg-toolkit#1137's roster half: a display name
// carried alongside a member's id from the moment it enters the encounter,
// through persistence, and back out through the read shape — so a cold read
// can project it without a caller reconstructing "who is this" from an id
// alone.
type NamesTestSuite struct {
	suite.Suite
}

func TestNamesSuite(t *testing.T) {
	suite.Run(t, new(NamesTestSuite))
}

const namesRoom = "hall"

func namesScene(members []encounter.MemberInput) (*encounter.Encounter, error) {
	return encounter.NewEncounter(&encounter.SetupInput{
		Sight: everyoneSeesTheWholeMap{}, Standing: everyoneStanding{},
		Initiative: orderAsGiven{}, TurnDriver: passDriver{}, Striker: passStriker{},
		Field: encounter.FieldInput{
			Canvas: encounter.CanvasInput{Void: encounter.VoidIsOpaque()},
			Rooms: []encounter.RoomInput{{
				ID: namesRoom, Width: 10, Height: 10,
			}},
		},
		Members: members,
		Endings: []encounter.EndingInput{{Key: "withdrawn", Trigger: encounter.TriggerExternal{}}},
	})
}

// TestSetupMemberCarriesName pins the authored half: a monster placed at
// construction (the reference tomb's skeleton) carries the name it was
// authored with into the read shape.
func (s *NamesTestSuite) TestSetupMemberCarriesName() {
	enc, err := namesScene([]encounter.MemberInput{
		{ID: alice, Kind: encounter.KindPlayer, Name: "Aldric", Room: namesRoom, Position: spatial.Position{X: 1, Y: 1}},
		{ID: goblin, Kind: encounter.KindMonster, Name: "skeleton-1", Room: namesRoom, Position: spatial.Position{X: 5, Y: 5}},
	})
	s.Require().NoError(err)

	members, err := enc.Members()
	s.Require().NoError(err)
	s.Require().Len(members, 2)

	byID := make(map[encounter.MemberID]encounter.Member, len(members))
	for _, m := range members {
		byID[m.ID] = m
	}
	s.Equal("Aldric", byID[alice].Name)
	s.Equal("skeleton-1", byID[goblin].Name)
}

// TestSetupMemberNameIsOptional pins that an author who supplies no name
// gets a member this composition can still place and reference — the empty
// string carries forward rather than being refused. Naming is the author's
// business, not this composition's to invent or demand.
func (s *NamesTestSuite) TestSetupMemberNameIsOptional() {
	enc, err := namesScene([]encounter.MemberInput{
		{ID: alice, Kind: encounter.KindPlayer, Room: namesRoom, Position: spatial.Position{X: 1, Y: 1}},
	})
	s.Require().NoError(err)

	members, err := enc.Members()
	s.Require().NoError(err)
	s.Require().Len(members, 1)
	s.Empty(members[0].Name)
}

// TestJoinCarriesNameForward pins the live half: a player joining mid-scene
// (the caller already holds the loaded character's display name) carries it
// into the roster exactly as an authored member's name does.
func (s *NamesTestSuite) TestJoinCarriesNameForward() {
	enc, err := namesScene([]encounter.MemberInput{
		{ID: goblin, Kind: encounter.KindMonster, Name: "skeleton-1", Room: namesRoom, Position: spatial.Position{X: 5, Y: 5}},
	})
	s.Require().NoError(err)

	_, err = enc.Join(&encounter.JoinInput{
		Member: alice, Kind: encounter.KindPlayer, Name: "Aldric", Cell: spatial.Position{X: 1, Y: 1},
	})
	s.Require().NoError(err)

	members, err := enc.Members()
	s.Require().NoError(err)
	for _, m := range members {
		if m.ID == alice {
			s.Equal("Aldric", m.Name)
			return
		}
	}
	s.Fail("alice not found in roster")
}

// TestNameRoundTripsThroughPersistence pins that ToData/LoadEncounter is not
// a second, lossier projection of the roster: a name present before a save
// is the same name present after a reload, the same law data_test.go's
// golden JSON already holds every other member field to.
func (s *NamesTestSuite) TestNameRoundTripsThroughPersistence() {
	enc, err := namesScene([]encounter.MemberInput{
		{ID: alice, Kind: encounter.KindPlayer, Name: "Aldric", Room: namesRoom, Position: spatial.Position{X: 1, Y: 1}},
		{ID: goblin, Kind: encounter.KindMonster, Name: "skeleton-1", Room: namesRoom, Position: spatial.Position{X: 5, Y: 5}},
	})
	s.Require().NoError(err)

	data := enc.ToData()

	reloaded, err := encounter.LoadEncounter(&encounter.LoadEncounterInput{
		Data: data, Initiative: orderAsGiven{}, Standing: everyoneStanding{},
		Sight: everyoneSeesTheWholeMap{}, TurnDriver: passDriver{}, Striker: passStriker{},
	})
	s.Require().NoError(err)

	members, err := reloaded.Members()
	s.Require().NoError(err)
	byID := make(map[encounter.MemberID]encounter.Member, len(members))
	for _, m := range members {
		byID[m.ID] = m
	}
	s.Equal("Aldric", byID[alice].Name)
	s.Equal("skeleton-1", byID[goblin].Name)
}

// DistanceTestSuite covers rpg-toolkit#1010's shared primitive: the same
// grid distance this composition's own reach and sight checks already use
// (refreshSight's e.canvas.GetGrid().Distance call), exposed minimally so a
// host can gate a weapon's reach without re-deriving hex math from Atlas
// data.
type DistanceTestSuite struct {
	suite.Suite
}

func TestDistanceSuite(t *testing.T) {
	suite.Run(t, new(DistanceTestSuite))
}

// TestDistanceOnSquareIsChebyshev pins a known square-grid distance: two
// cells one diagonal step apart are distance 1, matching SquareGrid's own
// Chebyshev metric — the reason a melee weapon's reach is meaningful as "1"
// on this grid family at all.
func (s *DistanceTestSuite) TestDistanceOnSquareIsChebyshev() {
	enc, err := namesScene([]encounter.MemberInput{
		{ID: alice, Kind: encounter.KindPlayer, Room: namesRoom, Position: spatial.Position{X: 4, Y: 4}},
	})
	s.Require().NoError(err)

	s.Equal(1.0, enc.Distance(spatial.Position{X: 4, Y: 4}, spatial.Position{X: 5, Y: 5}))
	s.Equal(4.0, enc.Distance(spatial.Position{X: 1, Y: 1}, spatial.Position{X: 5, Y: 1}))
	s.Equal(0.0, enc.Distance(spatial.Position{X: 4, Y: 4}, spatial.Position{X: 4, Y: 4}))
}
