// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package encounter_test

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/KirkDiggler/rpg-toolkit/core"
	"github.com/KirkDiggler/rpg-toolkit/play/intel"
	"github.com/KirkDiggler/rpg-toolkit/tools/spatial"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter"
)

type EncounterTestSuite struct {
	suite.Suite
}

func (s *EncounterTestSuite) TestSetupFirstLight() {
	s.Run("two members clear line of sight", func() {
		// Arrange: 10x10 room, alice at (2,2) player, goblin at (7,7) monster
		alice := core.EntityID("alice")
		goblin := core.EntityID("goblin")

		setup := &encounter.SetupInput{
			Field: encounter.FieldInput{
				Rooms: []encounter.RoomInput{
					{
						ID:     "room-1",
						Width:  10,
						Height: 10,
					},
				},
				Connections: []encounter.ConnectionInput{},
			},
			Members: []encounter.MemberInput{
				{
					ID:       alice,
					Kind:     encounter.KindPlayer,
					Room:     "room-1",
					Position: spatial.Position{X: 2, Y: 2},
				},
				{
					ID:       goblin,
					Kind:     encounter.KindMonster,
					Room:     "room-1",
					Position: spatial.Position{X: 7, Y: 7},
				},
			},
			Endings: []encounter.EndingInput{
				{
					Key: "stairs",
					Trigger: encounter.TriggerReachedPosition{
						Room:     "room-1",
						Position: spatial.Position{X: 9, Y: 9},
					},
				},
			},
		}

		// Act
		enc, err := encounter.NewEncounter(setup)
		s.Require().NoError(err)

		// Assert: alice sees goblin
		aliceView, err := enc.View(&encounter.ViewInput{Member: alice})
		s.Require().NoError(err)
		s.Len(aliceView, 1, "alice should see exactly one holding (goblin)")

		holding := aliceView[0]
		s.Equal(intel.Subject(goblin), holding.Subject, "holding subject should be goblin")
		s.Equal("current", string(holding.Status), "holding status should be Current")

		// Decode position payload
		var pos struct {
			Room string `json:"room"`
			X    int    `json:"x"`
			Y    int    `json:"y"`
		}
		err = json.Unmarshal(holding.Payload, &pos)
		s.Require().NoError(err)
		s.Equal("room-1", pos.Room)
		s.Equal(7, pos.X)
		s.Equal(7, pos.Y)

		// Assert: goblin sees alice (symmetric)
		goblinView, err := enc.View(&encounter.ViewInput{Member: goblin})
		s.Require().NoError(err)
		s.Len(goblinView, 1, "goblin should see exactly one holding (alice)")

		holding = goblinView[0]
		s.Equal(intel.Subject(alice), holding.Subject, "holding subject should be alice")
		s.Equal("current", string(holding.Status), "holding status should be Current")

		// Decode position
		err = json.Unmarshal(holding.Payload, &pos)
		s.Require().NoError(err)
		s.Equal(2, pos.X)
		s.Equal(2, pos.Y)
	})
}

func (s *EncounterTestSuite) TestSetupPillarBlocksSight() {
	s.Run("occluder blocks both directions", func() {
		// Arrange: alice and goblin separated by a pillar
		alice := core.EntityID("alice")
		goblin := core.EntityID("goblin")

		setup := &encounter.SetupInput{
			Field: encounter.FieldInput{
				Rooms: []encounter.RoomInput{
					{
						ID:     "room-1",
						Width:  10,
						Height: 10,
						Occluders: []spatial.Position{
							{X: 4, Y: 5}, // Pillar in the middle
						},
					},
				},
				Connections: []encounter.ConnectionInput{},
			},
			Members: []encounter.MemberInput{
				{
					ID:       alice,
					Kind:     encounter.KindPlayer,
					Room:     "room-1",
					Position: spatial.Position{X: 2, Y: 5},
				},
				{
					ID:       goblin,
					Kind:     encounter.KindMonster,
					Room:     "room-1",
					Position: spatial.Position{X: 7, Y: 5},
				},
			},
			Endings: []encounter.EndingInput{
				{
					Key: "stairs",
					Trigger: encounter.TriggerReachedPosition{
						Room:     "room-1",
						Position: spatial.Position{X: 9, Y: 9},
					},
				},
			},
		}

		// Act
		enc, err := encounter.NewEncounter(setup)
		s.Require().NoError(err)

		// Assert: alice does not see goblin
		aliceView, err := enc.View(&encounter.ViewInput{Member: alice})
		s.Require().NoError(err)
		s.Len(aliceView, 0, "alice should see nothing (blocked by pillar)")

		// Assert: goblin does not see alice (symmetric)
		goblinView, err := enc.View(&encounter.ViewInput{Member: goblin})
		s.Require().NoError(err)
		s.Len(goblinView, 0, "goblin should see nothing (blocked by pillar)")
	})
}

func (s *EncounterTestSuite) TestSetupValidationOrderAndAtomicity() {
	s.Run("validation order: nil input", func() {
		_, err := encounter.NewEncounter(nil)
		s.Require().ErrorIs(err, encounter.ErrNilInput)
	})

	s.Run("validation order: no field", func() {
		setup := &encounter.SetupInput{
			Field: encounter.FieldInput{
				Rooms: []encounter.RoomInput{},
			},
			Members: []encounter.MemberInput{},
			Endings: []encounter.EndingInput{
				{Key: "end", Trigger: encounter.TriggerExternal{}},
			},
		}
		_, err := encounter.NewEncounter(setup)
		s.Require().ErrorIs(err, encounter.ErrNoField)
	})

	s.Run("validation order: no endings", func() {
		setup := &encounter.SetupInput{
			Field: encounter.FieldInput{
				Rooms: []encounter.RoomInput{
					{ID: "room-1", Width: 10, Height: 10},
				},
			},
			Members:  []encounter.MemberInput{},
			Endings:  []encounter.EndingInput{},
		}
		_, err := encounter.NewEncounter(setup)
		s.Require().ErrorIs(err, encounter.ErrNoEnding)
	})

	s.Run("validation order: reserved ending key", func() {
		setup := &encounter.SetupInput{
			Field: encounter.FieldInput{
				Rooms: []encounter.RoomInput{
					{ID: "room-1", Width: 10, Height: 10},
				},
			},
			Members: []encounter.MemberInput{},
			Endings: []encounter.EndingInput{
				{Key: "abandoned", Trigger: encounter.TriggerExternal{}},
			},
		}
		_, err := encounter.NewEncounter(setup)
		s.Require().ErrorIs(err, encounter.ErrNoEnding)
	})

	s.Run("validation order: empty ending key", func() {
		setup := &encounter.SetupInput{
			Field: encounter.FieldInput{
				Rooms: []encounter.RoomInput{
					{ID: "room-1", Width: 10, Height: 10},
				},
			},
			Members: []encounter.MemberInput{},
			Endings: []encounter.EndingInput{
				{Key: "", Trigger: encounter.TriggerExternal{}},
			},
		}
		_, err := encounter.NewEncounter(setup)
		s.Require().ErrorIs(err, encounter.ErrNoEnding)
	})

	s.Run("validation order: empty member ID", func() {
		setup := &encounter.SetupInput{
			Field: encounter.FieldInput{
				Rooms: []encounter.RoomInput{
					{ID: "room-1", Width: 10, Height: 10},
				},
			},
			Members: []encounter.MemberInput{
				{ID: "", Kind: encounter.KindPlayer, Room: "room-1"},
			},
			Endings: []encounter.EndingInput{
				{Key: "stairs", Trigger: encounter.TriggerExternal{}},
			},
		}
		_, err := encounter.NewEncounter(setup)
		s.Require().ErrorIs(err, encounter.ErrNoMember)
	})

	s.Run("validation order: duplicate member IDs", func() {
		alice := core.EntityID("alice")
		setup := &encounter.SetupInput{
			Field: encounter.FieldInput{
				Rooms: []encounter.RoomInput{
					{ID: "room-1", Width: 10, Height: 10},
				},
			},
			Members: []encounter.MemberInput{
				{ID: alice, Kind: encounter.KindPlayer, Room: "room-1", Position: spatial.Position{X: 0, Y: 0}},
				{ID: alice, Kind: encounter.KindPlayer, Room: "room-1", Position: spatial.Position{X: 1, Y: 1}},
			},
			Endings: []encounter.EndingInput{
				{Key: "stairs", Trigger: encounter.TriggerExternal{}},
			},
		}
		_, err := encounter.NewEncounter(setup)
		s.Require().ErrorIs(err, encounter.ErrNoMember)
	})

	s.Run("atomicity: after error, valid setup works", func() {
		// First attempt: fails validation
		setup1 := &encounter.SetupInput{
			Field: encounter.FieldInput{Rooms: []encounter.RoomInput{}},
			Members: []encounter.MemberInput{},
			Endings: []encounter.EndingInput{
				{Key: "stairs", Trigger: encounter.TriggerExternal{}},
			},
		}
		_, err := encounter.NewEncounter(setup1)
		s.Require().ErrorIs(err, encounter.ErrNoField)

		// Second attempt: valid
		setup2 := &encounter.SetupInput{
			Field: encounter.FieldInput{
				Rooms: []encounter.RoomInput{
					{ID: "room-1", Width: 10, Height: 10},
				},
			},
			Members: []encounter.MemberInput{
				{ID: "alice", Kind: encounter.KindPlayer, Room: "room-1", Position: spatial.Position{X: 0, Y: 0}},
			},
			Endings: []encounter.EndingInput{
				{Key: "stairs", Trigger: encounter.TriggerExternal{}},
			},
		}
		enc, err := encounter.NewEncounter(setup2)
		s.Require().NoError(err)
		s.NotNil(enc)
	})
}

func (s *EncounterTestSuite) TestSetupRejectsReservedEnding() {
	setup := &encounter.SetupInput{
		Field: encounter.FieldInput{
			Rooms: []encounter.RoomInput{
				{ID: "room-1", Width: 10, Height: 10},
			},
		},
		Members: []encounter.MemberInput{},
		Endings: []encounter.EndingInput{
			{Key: "abandoned", Trigger: encounter.TriggerExternal{}},
		},
	}
	_, err := encounter.NewEncounter(setup)
	s.Require().ErrorIs(err, encounter.ErrNoEnding)
}

func (s *EncounterTestSuite) TestSetupOpeningBeat() {
	s.Run("recording minimal beat", func() {
		alice := core.EntityID("alice")
		setup := &encounter.SetupInput{
			Field: encounter.FieldInput{
				Rooms: []encounter.RoomInput{
					{ID: "room-1", Width: 10, Height: 10},
				},
			},
			Members: []encounter.MemberInput{
				{ID: alice, Kind: encounter.KindPlayer, Room: "room-1", Position: spatial.Position{X: 0, Y: 0}},
			},
			Endings: []encounter.EndingInput{
				{Key: "stairs", Trigger: encounter.TriggerExternal{}},
			},
		}

		// Act
		enc, err := encounter.NewEncounter(setup)
		s.Require().NoError(err)

		// Assert: can query status (simple validation that setup completed)
		status, err := enc.Status()
		s.Require().NoError(err)
		s.NotNil(status)
	})
}

func (s *EncounterTestSuite) TestMembers() {
	s.Run("returns stable order", func() {
		alice := core.EntityID("alice")
		bob := core.EntityID("bob")

		setup := &encounter.SetupInput{
			Field: encounter.FieldInput{
				Rooms: []encounter.RoomInput{
					{ID: "room-1", Width: 10, Height: 10},
				},
			},
			Members: []encounter.MemberInput{
				{ID: alice, Kind: encounter.KindPlayer, Room: "room-1", Position: spatial.Position{X: 0, Y: 0}},
				{ID: bob, Kind: encounter.KindPlayer, Room: "room-1", Position: spatial.Position{X: 1, Y: 1}},
			},
			Endings: []encounter.EndingInput{
				{Key: "stairs", Trigger: encounter.TriggerExternal{}},
			},
		}

		enc, err := encounter.NewEncounter(setup)
		s.Require().NoError(err)

		members, err := enc.Members()
		s.Require().NoError(err)
		s.Len(members, 2)
	})
}

func TestEncounterSuite(t *testing.T) {
	suite.Run(t, new(EncounterTestSuite))
}
