// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package encounter_test

import (
	"encoding/json"
	"errors"
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/KirkDiggler/rpg-toolkit/core"
	"github.com/KirkDiggler/rpg-toolkit/play/intel"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter"
	"github.com/KirkDiggler/rpg-toolkit/tools/spatial"
)

type DataTestSuite struct {
	suite.Suite
}

// TestGoldenJSONRich pins the tags the two small goldens cannot see:
// occluders, boundaries, connections, a member-filtered ending, an
// External ending, intel-free monsters in a second room, and non-zero
// clock/At fields. Eleven tag renames survived the small goldens
// because omitempty hid these fields (T6 review).
func (s *DataTestSuite) TestGoldenJSONRich() {
	enc, err := encounter.NewEncounter(&encounter.SetupInput{
		Field: encounter.FieldInput{
			Rooms: []encounter.RoomInput{
				{ID: "crypt", Width: 8, Height: 8,
					Occluders:  []spatial.Position{{X: 4, Y: 4}},
					Boundaries: []spatial.Boundary{{From: spatial.Position{X: 2, Y: 2}, To: spatial.Position{X: 2, Y: 3}, BlocksMovement: true, BlocksLineOfSight: true}},
				},
				// hall is axial hex (Width=6,Height=6 => Q,R valid in
				// [-3,3)): door1's arrival endpoint sits at a NEGATIVE
				// axial coordinate on purpose — the ordinary case for an
				// origin-centered hex room, and a fixture proving
				// endpoints validate there at both seams (Setup here,
				// Load in TestConnectionEndpointBoundsBoundariesLoad's
				// hex sibling).
				{ID: "hall", Width: 6, Height: 6, Grid: spatial.GridShapeHex},
			},
			Connections: []encounter.ConnectionInput{{
				ID: "door1", From: "crypt", To: "hall",
				FromPosition: spatial.Position{X: 0, Y: 6},
				ToPosition:   spatial.Position{X: -1, Y: -1},
			}},
		},
		Members: []encounter.MemberInput{
			{ID: "p1", Kind: encounter.KindPlayer, Room: "crypt", Position: spatial.Position{X: 1, Y: 1}},
			{ID: "g1", Kind: encounter.KindMonster, Room: "hall", Position: spatial.Position{X: 0, Y: 0}, Decider: &testDecider{intent: encounter.IntentHold{}}},
		},
		Endings: []encounter.EndingInput{
			{Key: "guarded", Trigger: encounter.TriggerReachedPosition{Room: "crypt", Position: spatial.Position{X: 7, Y: 7}, Member: core.EntityID("p1")}},
			{Key: "leave", Trigger: encounter.TriggerExternal{}},
		},
	})
	s.Require().NoError(err)
	_, err = enc.Pump(&encounter.PumpInput{})
	s.Require().NoError(err)

	bs, err := json.Marshal(enc.ToData())
	s.Require().NoError(err)
	expected := `{"clock":{"driver_progress":{"world":1},"high_water":1},"intel":{},"log":{"next_seq":3,"entries":[{"seq":1,"audience":["p1","g1"],"tags":{"tag":"scene"},"payload":"eyJiZWF0Ijoic2NlbmUtb3BlbmVkIn0="},{"seq":2,"at":1,"audience":["g1","p1"],"tags":{"tag":"clock"},"payload":"eyJiZWF0IjoidGljayIsInRpY2siOjF9"}]},"field":{"rooms":[{"id":"crypt","width":8,"height":8,"occluders":[{"x":4,"y":4}],"boundaries":[{"from":{"x":2,"y":2},"to":{"x":2,"y":3},"blocks_movement":true,"blocks_line_of_sight":true}]},{"id":"hall","width":6,"height":6,"grid":"hex"}],"connections":[{"id":"door1","from":"crypt","to":"hall","from_position":{"x":0,"y":6},"to_position":{"x":-1,"y":-1}}]},"members":[{"id":"g1","kind":"monster","room":"hall","position":{"x":0,"y":0}},{"id":"p1","kind":"player","room":"crypt","position":{"x":1,"y":1}}],"endings":[{"key":"guarded","kind":"reached_position","room":"crypt","position":{"x":7,"y":7},"member":"p1"},{"key":"leave","kind":"external"}],"ever_members":["g1","p1"]}`
	s.Equal(expected, string(bs))
}

// TestEndingsOrderSurvivesReload pins C8 first-declared-wins across the
// persistence boundary: two endings match the same position; the FIRST
// declared fires, before and after a reload. A load that scrambles
// ending order changes which outcome the campaign receives.
func (s *DataTestSuite) TestEndingsOrderSurvivesReload() {
	setup := &encounter.SetupInput{
		Field: encounter.FieldInput{Rooms: []encounter.RoomInput{{ID: "r1", Width: 5, Height: 5}}},
		Members: []encounter.MemberInput{
			{ID: "p1", Kind: encounter.KindPlayer, Room: "r1", Position: spatial.Position{X: 1, Y: 1}},
		},
		Endings: []encounter.EndingInput{
			{Key: "first", Trigger: encounter.TriggerReachedPosition{Room: "r1", Position: spatial.Position{X: 3, Y: 3}}},
			{Key: "second", Trigger: encounter.TriggerReachedPosition{Room: "r1", Position: spatial.Position{X: 3, Y: 3}}},
		},
	}
	enc1, err := encounter.NewEncounter(setup)
	s.Require().NoError(err)
	enc2, err := encounter.LoadEncounter(enc1.ToData(), nil)
	s.Require().NoError(err)

	out, err := enc2.Move(&encounter.MoveInput{Member: "p1", To: spatial.Position{X: 3, Y: 3}})
	s.Require().NoError(err)
	s.Require().NotNil(out.Outcome)
	s.Equal("first", out.Outcome.Ending,
		"first-declared wins after reload — a load that scrambles ending order is a C8 violation")
}

// TestConnectionsSurviveReload pins connection persistence (#922 T1):
// endpoints round-trip through ToData -> LoadEncounter intact, and
// connection order is sorted by ID regardless of declaration order (C8
// determinism — persisted order must not leak the caller's declaration
// sequence). Every connection's FromPosition != ToPosition (X != Y within
// each, and neither is the other transposed) so a From/To swap anywhere in
// the round-trip — most notably in convertConnectionDataToConnectionInput,
// which only ever sees pre-sorted, already-round-tripped ToData output in
// other tests — would change the observed values, not just their order.
func (s *DataTestSuite) TestConnectionsSurviveReload() {
	setup := &encounter.SetupInput{
		Field: encounter.FieldInput{
			Rooms: []encounter.RoomInput{
				{ID: "r1", Width: 5, Height: 5},
				{ID: "r2", Width: 5, Height: 5},
			},
			// Declared out of ID order — persistence must not echo this order.
			Connections: []encounter.ConnectionInput{
				{ID: "z-door", From: "r1", To: "r2", FromPosition: spatial.Position{X: 0, Y: 3}, ToPosition: spatial.Position{X: 2, Y: 1}},
				{ID: "a-door", From: "r1", To: "r2", FromPosition: spatial.Position{X: 1, Y: 4}, ToPosition: spatial.Position{X: 3, Y: 2}},
				{ID: "m-door", From: "r1", To: "r2", FromPosition: spatial.Position{X: 2, Y: 0}, ToPosition: spatial.Position{X: 4, Y: 3}},
			},
		},
		Members: []encounter.MemberInput{
			{ID: "p1", Kind: encounter.KindPlayer, Room: "r1", Position: spatial.Position{X: 4, Y: 4}},
		},
		Endings: []encounter.EndingInput{{Key: "done", Trigger: encounter.TriggerExternal{}}},
	}

	enc1, err := encounter.NewEncounter(setup)
	s.Require().NoError(err)

	data1 := enc1.ToData()
	expected := []encounter.ConnectionData{
		{ID: "a-door", From: "r1", To: "r2", FromPosition: &encounter.PositionData{X: 1, Y: 4}, ToPosition: &encounter.PositionData{X: 3, Y: 2}},
		{ID: "m-door", From: "r1", To: "r2", FromPosition: &encounter.PositionData{X: 2, Y: 0}, ToPosition: &encounter.PositionData{X: 4, Y: 3}},
		{ID: "z-door", From: "r1", To: "r2", FromPosition: &encounter.PositionData{X: 0, Y: 3}, ToPosition: &encounter.PositionData{X: 2, Y: 1}},
	}
	s.Equal(expected, data1.Field.Connections, "connections persist sorted by ID with endpoints intact")

	enc2, err := encounter.LoadEncounter(data1, nil)
	s.Require().NoError(err)

	data2 := enc2.ToData()
	s.Equal(expected, data2.Field.Connections, "connections stay sorted and intact across a second round-trip")
}

// TestLoadSortsUnsortedConnections pins LoadEncounter's own sort in
// isolation (#922 T1 Opus review): every other test's EncounterData either
// comes from ToData (already sorted by construction, since NewEncounter and
// LoadEncounter both sort connectionsInput on the way in) or carries a
// single connection (trivially sorted) — so LoadEncounter's sort.Slice call
// is never exercised on genuinely unsorted input elsewhere in this suite.
// This hand-authors an EncounterData directly, bypassing NewEncounter/ToData
// entirely, with connections declared out of ID order.
func (s *DataTestSuite) TestLoadSortsUnsortedConnections() {
	data := encounter.EncounterData{
		Field: encounter.FieldData{
			Rooms: []encounter.RoomData{
				{ID: "r1", Width: 5, Height: 5},
				{ID: "r2", Width: 5, Height: 5},
			},
			// Hand-authored out of ID order — NOT produced by ToData.
			Connections: []encounter.ConnectionData{
				{ID: "z-door", From: "r1", To: "r2", FromPosition: &encounter.PositionData{X: 0, Y: 0}, ToPosition: &encounter.PositionData{X: 1, Y: 1}},
				{ID: "a-door", From: "r1", To: "r2", FromPosition: &encounter.PositionData{X: 2, Y: 0}, ToPosition: &encounter.PositionData{X: 3, Y: 1}},
				{ID: "m-door", From: "r1", To: "r2", FromPosition: &encounter.PositionData{X: 4, Y: 0}, ToPosition: &encounter.PositionData{X: 0, Y: 4}},
			},
		},
		Members: []encounter.MemberData{
			{ID: "p1", Kind: encounter.KindPlayer, Room: "r1", Position: encounter.PositionData{X: 1, Y: 1}},
		},
		Endings:     []encounter.EndingData{{Key: "done", Kind: "external"}},
		EverMembers: []encounter.MemberID{"p1"},
	}

	enc, err := encounter.LoadEncounter(data, nil)
	s.Require().NoError(err)

	got := enc.ToData().Field.Connections
	s.Require().Len(got, 3)
	gotIDs := []string{got[0].ID, got[1].ID, got[2].ID}
	s.Equal([]string{"a-door", "m-door", "z-door"}, gotIDs,
		"LoadEncounter's own sort must run even when the input was never produced by ToData")
}

// TestRoomGridShapeSurvivesReload pins connection persistence's newest
// field (#922 T1.5): a room's declared Grid shape round-trips through
// ToData -> LoadEncounter -> ToData intact, for both the square zero value
// and a non-zero shape (hex), in the same encounter.
func (s *DataTestSuite) TestRoomGridShapeSurvivesReload() {
	setup := &encounter.SetupInput{
		Field: encounter.FieldInput{
			Rooms: []encounter.RoomInput{
				{ID: "square-room", Width: 5, Height: 5},
				{ID: "hex-room", Width: 5, Height: 5, Grid: spatial.GridShapeHex},
			},
		},
		Members: []encounter.MemberInput{
			{ID: "p1", Kind: encounter.KindPlayer, Room: "square-room", Position: spatial.Position{X: 1, Y: 1}},
		},
		Endings: []encounter.EndingInput{{Key: "done", Trigger: encounter.TriggerExternal{}}},
	}

	enc1, err := encounter.NewEncounter(setup)
	s.Require().NoError(err)

	data1 := enc1.ToData()
	s.Require().Len(data1.Field.Rooms, 2)
	s.Equal("", data1.Field.Rooms[0].Grid, "square is the zero value — omitted, not the literal \"square\"")
	s.Equal(spatial.GridTypeHex, data1.Field.Rooms[1].Grid)

	enc2, err := encounter.LoadEncounter(data1, nil)
	s.Require().NoError(err)

	data2 := enc2.ToData()
	s.Equal("", data2.Field.Rooms[0].Grid, "grid shape must survive a second round-trip")
	s.Equal(spatial.GridTypeHex, data2.Field.Rooms[1].Grid, "grid shape must survive a second round-trip")
}

// TestSetupInputNotAliased pins T6 review M4: a caller that edits its
// own SetupInput after construction must not corrupt the persistence
// source (the encounter deep-copies the field description). Also covers
// connections: mutating the caller's ConnectionInput slice (and its
// endpoint positions) after NewEncounter must not affect the encounter.
func (s *DataTestSuite) TestSetupInputNotAliased() {
	setup := &encounter.SetupInput{
		Field: encounter.FieldInput{
			Rooms: []encounter.RoomInput{
				{ID: "r1", Width: 5, Height: 5, Occluders: []spatial.Position{{X: 3, Y: 3}}},
				{ID: "r2", Width: 5, Height: 5},
			},
			Connections: []encounter.ConnectionInput{
				{ID: "door1", From: "r1", To: "r2", FromPosition: spatial.Position{X: 1, Y: 0}, ToPosition: spatial.Position{X: 0, Y: 0}},
			},
		},
		Members: []encounter.MemberInput{
			{ID: "p1", Kind: encounter.KindPlayer, Room: "r1", Position: spatial.Position{X: 1, Y: 1}},
		},
		Endings: []encounter.EndingInput{{Key: "done", Trigger: encounter.TriggerExternal{}}},
	}
	enc, err := encounter.NewEncounter(setup)
	s.Require().NoError(err)

	setup.Field.Rooms[0].ID = "VANDALIZED"
	setup.Field.Rooms[0].Width = 999
	setup.Field.Rooms[0].Occluders[0] = spatial.Position{X: 4, Y: 4}
	setup.Field.Connections[0].ID = "VANDALIZED"
	setup.Field.Connections[0].From = "VANDALIZED"
	setup.Field.Connections[0].FromPosition = spatial.Position{X: 4, Y: 4}
	setup.Field.Connections[0].ToPosition = spatial.Position{X: 4, Y: 4}

	data := enc.ToData()
	s.Require().Len(data.Field.Rooms, 2)
	s.Equal("r1", data.Field.Rooms[0].ID, "the snapshot must not see the caller's vandalism")
	s.Equal(5, data.Field.Rooms[0].Width)
	s.Equal(encounter.PositionData{X: 3, Y: 3}, data.Field.Rooms[0].Occluders[0])

	s.Require().Len(data.Field.Connections, 1)
	s.Equal("door1", data.Field.Connections[0].ID, "the snapshot must not see the caller's vandalism")
	s.Equal("r1", data.Field.Connections[0].From)
	s.Equal(&encounter.PositionData{X: 1, Y: 0}, data.Field.Connections[0].FromPosition)
	s.Equal(&encounter.PositionData{X: 0, Y: 0}, data.Field.Connections[0].ToPosition)

	// And the corrupted-input snapshot must still LOAD (the M4 symptom
	// was an encounter that became permanently unsavable).
	_, err = encounter.LoadEncounter(data, nil)
	s.Require().NoError(err)
}

func TestDataSuite(t *testing.T) {
	suite.Run(t, new(DataTestSuite))
}

// TestRoundTripPostSetup verifies that an encounter round-trips identically
// after Setup (open state, fresh members, with surveil holdings).
func (s *DataTestSuite) TestRoundTripPostSetup() {
	s.Run("post-setup open encounter survives round-trip", func() {
		// Create a simple encounter
		setup := &encounter.SetupInput{
			Field: encounter.FieldInput{
				Rooms: []encounter.RoomInput{
					{
						ID:        "crypt",
						Width:     10,
						Height:    10,
						Occluders: []spatial.Position{{X: 5, Y: 5}},
						Boundaries: []spatial.Boundary{
							{
								From:              spatial.Position{X: 3, Y: 3},
								To:                spatial.Position{X: 3, Y: 4},
								BlocksMovement:    true,
								BlocksLineOfSight: true,
							},
						},
					},
				},
				Connections: []encounter.ConnectionInput{},
			},
			Members: []encounter.MemberInput{
				{
					ID:       "player1",
					Kind:     encounter.KindPlayer,
					Room:     "crypt",
					Position: spatial.Position{X: 1, Y: 1},
				},
				{
					ID:       "goblin",
					Kind:     encounter.KindMonster,
					Room:     "crypt",
					Position: spatial.Position{X: 8, Y: 8},
					Decider:  &testDecider{intent: encounter.IntentHold{}},
				},
			},
			Endings: []encounter.EndingInput{
				{
					Key:     "stairs",
					Trigger: encounter.TriggerReachedPosition{Room: "crypt", Position: spatial.Position{X: 0, Y: 0}},
				},
			},
		}

		enc1, err := encounter.NewEncounter(setup)
		s.Require().NoError(err)

		// Convert to data
		data1 := enc1.ToData()

		// Load from data (without decider for goblin)
		enc2, err := encounter.LoadEncounter(data1, map[encounter.MemberID]encounter.Decider{})
		s.Require().NoError(err)

		// Convert to data again
		data2 := enc2.ToData()

		// Both data should be equal (deep equal, not just pointer equal)
		s.Equal(data1.Outcome, data2.Outcome)
		s.Equal(data1.Members, data2.Members)
		s.Equal(data1.EverMembers, data2.EverMembers)
		s.Equal(data1.Endings, data2.Endings)
		s.Equal(data1.Field, data2.Field)
	})
}

// TestRoundTripMidFade verifies a mid-fade (ghost present) state survives
// round-trip with the ghost still Held.
func (s *DataTestSuite) TestRoundTripMidFade() {
	s.Run("mid-fade ghost survives reload still Held", func() {
		// Create encounter with a pillar that will cause a ghost to form
		setup := &encounter.SetupInput{
			Field: encounter.FieldInput{
				Rooms: []encounter.RoomInput{
					{
						ID:     "crypt",
						Width:  10,
						Height: 10,
						// Pillar at (5, 5) blocks sight
						Occluders:  []spatial.Position{{X: 5, Y: 5}},
						Boundaries: []spatial.Boundary{},
					},
				},
				Connections: []encounter.ConnectionInput{},
			},
			Members: []encounter.MemberInput{
				{
					ID:       "playerA",
					Kind:     encounter.KindPlayer,
					Room:     "crypt",
					Position: spatial.Position{X: 1, Y: 1},
				},
				{
					ID:       "goblin",
					Kind:     encounter.KindMonster,
					Room:     "crypt",
					Position: spatial.Position{X: 9, Y: 9},
					Decider:  &testDecider{intent: encounter.IntentHold{}},
				},
			},
			Endings: []encounter.EndingInput{
				{
					Key:     "stairs",
					Trigger: encounter.TriggerReachedPosition{Room: "crypt", Position: spatial.Position{X: 0, Y: 0}},
				},
			},
		}

		enc1, err := encounter.NewEncounter(setup)
		s.Require().NoError(err)

		// Move goblin behind pillar to create a ghost
		_, err = enc1.Move(&encounter.MoveInput{
			Member: "playerA",
			To:     spatial.Position{X: 4, Y: 1},
		})
		s.Require().NoError(err)

		// Move goblin to create ghost at last-seen position
		_, err = enc1.Move(&encounter.MoveInput{
			Member: "goblin",
			To:     spatial.Position{X: 5, Y: 6}, // Behind pillar from A's view
		})
		s.Require().NoError(err)

		data1 := enc1.ToData()

		// Load and verify ghost is still there
		enc2, err := encounter.LoadEncounter(data1, map[encounter.MemberID]encounter.Decider{})
		s.Require().NoError(err)

		// Get holdings - ghost should still be Held (not Current)
		holdings, err := enc2.View(&encounter.ViewInput{Member: "playerA"})
		s.Require().NoError(err)

		// Find goblin holding - should be ghost, not current
		var foundGoblin bool
		for _, h := range holdings {
			if h.Subject == intel.Subject("goblin") {
				foundGoblin = true
				s.Equal(intel.Held, h.Status, "reloaded ghost should still be held as ghost")
				break
			}
		}
		s.True(foundGoblin, "goblin holding should exist after reload")
	})
}

// TestRoundTripPostExit verifies everMembers are preserved and exited member
// can still Story.
func (s *DataTestSuite) TestRoundTripPostExit() {
	s.Run("exited member persists in everMembers and can Story", func() {
		setup := &encounter.SetupInput{
			Field: encounter.FieldInput{
				Rooms: []encounter.RoomInput{
					{
						ID:         "crypt",
						Width:      10,
						Height:     10,
						Occluders:  []spatial.Position{},
						Boundaries: []spatial.Boundary{},
					},
				},
				Connections: []encounter.ConnectionInput{},
			},
			Members: []encounter.MemberInput{
				{
					ID:       "playerA",
					Kind:     encounter.KindPlayer,
					Room:     "crypt",
					Position: spatial.Position{X: 1, Y: 1},
				},
			},
			Endings: []encounter.EndingInput{
				{
					Key:     "stairs",
					Trigger: encounter.TriggerReachedPosition{Room: "crypt", Position: spatial.Position{X: 0, Y: 0}},
				},
			},
		}

		enc1, err := encounter.NewEncounter(setup)
		s.Require().NoError(err)

		// Exit the player
		_, err = enc1.Exit(&encounter.ExitInput{Member: "playerA"})
		s.Require().NoError(err)

		data1 := enc1.ToData()

		// Load and verify everMembers includes the exited player
		enc2, err := encounter.LoadEncounter(data1, map[encounter.MemberID]encounter.Decider{})
		s.Require().NoError(err)

		// Story should work for the exited member
		story, err := enc2.Story(&encounter.StoryInput{Audience: "playerA", AfterSeq: 0})
		s.Require().NoError(err)
		s.NotEmpty(story, "exited member should still be able to Story")
	})
}

// TestRoundTripClosed verifies closed encounters (abandoned and stairs) survive
// round-trip with Status matching.
func (s *DataTestSuite) TestRoundTripClosed() {
	s.Run("closed encounter with ending outcome round-trips", func() {
		setup := &encounter.SetupInput{
			Field: encounter.FieldInput{
				Rooms: []encounter.RoomInput{
					{
						ID:         "crypt",
						Width:      10,
						Height:     10,
						Occluders:  []spatial.Position{},
						Boundaries: []spatial.Boundary{},
					},
				},
				Connections: []encounter.ConnectionInput{},
			},
			Members: []encounter.MemberInput{
				{
					ID:       "playerA",
					Kind:     encounter.KindPlayer,
					Room:     "crypt",
					Position: spatial.Position{X: 1, Y: 1},
				},
			},
			Endings: []encounter.EndingInput{
				{
					Key:     "stairs",
					Trigger: encounter.TriggerReachedPosition{Room: "crypt", Position: spatial.Position{X: 0, Y: 0}},
				},
				{
					Key:     "withdraw",
					Trigger: encounter.TriggerExternal{},
				},
			},
		}

		enc1, err := encounter.NewEncounter(setup)
		s.Require().NoError(err)

		// Move to stairs to close
		_, err = enc1.Move(&encounter.MoveInput{
			Member: "playerA",
			To:     spatial.Position{X: 0, Y: 0},
		})
		s.Require().NoError(err)

		status1, _ := enc1.Status()
		data1 := enc1.ToData()

		// Load and verify outcome matches
		enc2, err := encounter.LoadEncounter(data1, map[encounter.MemberID]encounter.Decider{})
		s.Require().NoError(err)

		status2, _ := enc2.Status()

		s.False(status2.Open, "loaded encounter should be closed")
		s.Equal(status1.Outcome.Ending, status2.Outcome.Ending)
		s.Equal(status1.Outcome.At, status2.Outcome.At)
		s.Equal(len(status1.Outcome.Members), len(status2.Outcome.Members))
	})
}

// TestPumpContinuesTick verifies Pump continues the tick sequence on reload.
func (s *DataTestSuite) TestPumpContinuesTick() {
	s.Run("Pump continues tick sequence post-reload", func() {
		setup := &encounter.SetupInput{
			Field: encounter.FieldInput{
				Rooms: []encounter.RoomInput{
					{
						ID:         "crypt",
						Width:      10,
						Height:     10,
						Occluders:  []spatial.Position{},
						Boundaries: []spatial.Boundary{},
					},
				},
				Connections: []encounter.ConnectionInput{},
			},
			Members: []encounter.MemberInput{
				{
					ID:       "playerA",
					Kind:     encounter.KindPlayer,
					Room:     "crypt",
					Position: spatial.Position{X: 1, Y: 1},
				},
			},
			Endings: []encounter.EndingInput{
				{
					Key:     "stairs",
					Trigger: encounter.TriggerReachedPosition{Room: "crypt", Position: spatial.Position{X: 0, Y: 0}},
				},
			},
		}

		enc1, err := encounter.NewEncounter(setup)
		s.Require().NoError(err)

		// Advance the world to reading 2 BEFORE snapshotting — a reload
		// that resets the clock is indistinguishable at reading 0.
		_, err = enc1.Pump(&encounter.PumpInput{})
		s.Require().NoError(err)
		_, err = enc1.Pump(&encounter.PumpInput{})
		s.Require().NoError(err)

		data1 := enc1.ToData()
		s.Require().Equal(2, data1.Clock.HighWater, "precondition: two ticks persisted")

		enc2, err := encounter.LoadEncounter(data1, nil)
		s.Require().NoError(err)

		out, err := enc2.Pump(&encounter.PumpInput{})
		s.Require().NoError(err)
		s.Equal(uint64(3), out.Tick, "the reloaded clock continues the sequence — never resets")
	})
}

// TestMoveWorks verifies Move works on a reloaded encounter.
func (s *DataTestSuite) TestMoveWorksPostReload() {
	s.Run("Move works on reloaded encounter", func() {
		setup := &encounter.SetupInput{
			Field: encounter.FieldInput{
				Rooms: []encounter.RoomInput{
					{
						ID:         "crypt",
						Width:      10,
						Height:     10,
						Occluders:  []spatial.Position{},
						Boundaries: []spatial.Boundary{},
					},
				},
				Connections: []encounter.ConnectionInput{},
			},
			Members: []encounter.MemberInput{
				{
					ID:       "playerA",
					Kind:     encounter.KindPlayer,
					Room:     "crypt",
					Position: spatial.Position{X: 1, Y: 1},
				},
			},
			Endings: []encounter.EndingInput{
				{
					Key:     "stairs",
					Trigger: encounter.TriggerReachedPosition{Room: "crypt", Position: spatial.Position{X: 0, Y: 0}},
				},
			},
		}

		enc1, err := encounter.NewEncounter(setup)
		s.Require().NoError(err)

		data1 := enc1.ToData()

		// Load
		enc2, err := encounter.LoadEncounter(data1, map[encounter.MemberID]encounter.Decider{})
		s.Require().NoError(err)

		// Move should work
		out, err := enc2.Move(&encounter.MoveInput{
			Member: "playerA",
			To:     spatial.Position{X: 2, Y: 2},
		})

		s.Require().NoError(err)
		s.NotNil(out)
		s.Equal(spatial.Position{X: 2, Y: 2}, out.Moved.To)
	})
}

// TestGoldenJSONOpen pins the JSON representation of a small open encounter.
func (s *DataTestSuite) TestGoldenJSONOpen() {
	s.Run("small open encounter golden JSON", func() {
		setup := &encounter.SetupInput{
			Field: encounter.FieldInput{
				Rooms: []encounter.RoomInput{
					{
						ID:         "room1",
						Width:      5,
						Height:     5,
						Occluders:  []spatial.Position{},
						Boundaries: []spatial.Boundary{},
					},
				},
				Connections: []encounter.ConnectionInput{},
			},
			Members: []encounter.MemberInput{
				{
					ID:       "p1",
					Kind:     encounter.KindPlayer,
					Room:     "room1",
					Position: spatial.Position{X: 2, Y: 2},
				},
			},
			Endings: []encounter.EndingInput{
				{
					Key:     "done",
					Trigger: encounter.TriggerReachedPosition{Room: "room1", Position: spatial.Position{X: 0, Y: 0}},
				},
			},
		}

		enc, err := encounter.NewEncounter(setup)
		s.Require().NoError(err)

		data := enc.ToData()
		jsonBytes, err := json.Marshal(data)
		s.Require().NoError(err)

		// Exact-string pin of the full compact marshal: every wire tag,
		// omitempty behavior, and field order — a stowaway field or a
		// renamed tag fails this where a decoded comparison would not.
		// (log carries the opening beat: a fresh encounter is born with
		// its first story entry; clock/intel marshal {} per leaf laws.)
		expectedJSON := `{"clock":{},"intel":{},"log":{"next_seq":2,"entries":[{"seq":1,"audience":["p1"],"tags":{"tag":"scene"},"payload":"eyJiZWF0Ijoic2NlbmUtb3BlbmVkIn0="}]},"field":{"rooms":[{"id":"room1","width":5,"height":5}]},"members":[{"id":"p1","kind":"player","room":"room1","position":{"x":2,"y":2}}],"endings":[{"key":"done","kind":"reached_position","room":"room1","position":{"x":0,"y":0}}],"ever_members":["p1"]}`
		s.Equal(expectedJSON, string(jsonBytes))
	})
}

// TestGoldenJSONClosed pins the JSON representation of a closed encounter.
func (s *DataTestSuite) TestGoldenJSONClosed() {
	s.Run("small closed encounter golden JSON", func() {
		setup := &encounter.SetupInput{
			Field: encounter.FieldInput{
				Rooms: []encounter.RoomInput{
					{
						ID:         "room1",
						Width:      5,
						Height:     5,
						Occluders:  []spatial.Position{},
						Boundaries: []spatial.Boundary{},
					},
				},
				Connections: []encounter.ConnectionInput{},
			},
			Members: []encounter.MemberInput{
				{
					ID:       "p1",
					Kind:     encounter.KindPlayer,
					Room:     "room1",
					Position: spatial.Position{X: 2, Y: 2},
				},
			},
			Endings: []encounter.EndingInput{
				{
					Key:     "done",
					Trigger: encounter.TriggerReachedPosition{Room: "room1", Position: spatial.Position{X: 0, Y: 0}},
				},
			},
		}

		enc, err := encounter.NewEncounter(setup)
		s.Require().NoError(err)

		// Close the encounter
		_, err = enc.Move(&encounter.MoveInput{
			Member: "p1",
			To:     spatial.Position{X: 0, Y: 0},
		})
		s.Require().NoError(err)

		data := enc.ToData()
		jsonBytes, err := json.Marshal(data)
		s.Require().NoError(err)

		// Exact-string pin of the closed shape: outcome present with the
		// fired ending and final member placements; the story carries
		// both beats (opening + the closing move).
		expectedJSON := `{"outcome":{"ending":"done","members":[{"id":"p1","room":"room1","position":{"x":0,"y":0}}]},"clock":{},"intel":{},"log":{"next_seq":3,"entries":[{"seq":1,"audience":["p1"],"tags":{"tag":"scene"},"payload":"eyJiZWF0Ijoic2NlbmUtb3BlbmVkIn0="},{"seq":2,"audience":["p1"],"tags":{"tag":"movement"},"payload":"eyJiZWF0IjoibW92ZWQiLCJtZW1iZXIiOiJwMSIsInBvc2l0aW9uIjp7IngiOjAsInkiOjB9fQ=="}]},"field":{"rooms":[{"id":"room1","width":5,"height":5}]},"members":[{"id":"p1","kind":"player","room":"room1","position":{"x":0,"y":0}}],"endings":[{"key":"done","kind":"reached_position","room":"room1","position":{"x":0,"y":0}}],"ever_members":["p1"]}`
		s.Equal(expectedJSON, string(jsonBytes))
	})
}

// TestAliasImmunity verifies mutating ToData result doesn't affect aggregate.
func (s *DataTestSuite) TestAliasImmunityToData() {
	s.Run("mutating ToData result doesn't affect aggregate", func() {
		setup := &encounter.SetupInput{
			Field: encounter.FieldInput{
				Rooms: []encounter.RoomInput{
					{
						ID:         "room1",
						Width:      5,
						Height:     5,
						Occluders:  []spatial.Position{},
						Boundaries: []spatial.Boundary{},
					},
				},
				Connections: []encounter.ConnectionInput{},
			},
			Members: []encounter.MemberInput{
				{
					ID:       "p1",
					Kind:     encounter.KindPlayer,
					Room:     "room1",
					Position: spatial.Position{X: 2, Y: 2},
				},
			},
			Endings: []encounter.EndingInput{
				{
					Key:     "done",
					Trigger: encounter.TriggerReachedPosition{Room: "room1", Position: spatial.Position{X: 0, Y: 0}},
				},
			},
		}

		enc, err := encounter.NewEncounter(setup)
		s.Require().NoError(err)

		data1 := enc.ToData()

		// Mutate the returned data
		if len(data1.Members) > 0 {
			data1.Members[0].ID = "mutated"
		}
		if len(data1.EverMembers) > 0 {
			data1.EverMembers[0] = "mutated"
		}
		if len(data1.Endings) > 0 {
			data1.Endings[0].Key = "mutated"
		}

		// Get data again
		data2 := enc.ToData()

		// Should not be affected by mutation
		s.NotEqual("mutated", data2.Members[0].ID)
		s.NotEqual("mutated", data2.EverMembers[0])
		s.NotEqual("mutated", data2.Endings[0].Key)
	})
}

// TestAliasImmunityLoadEncounter verifies mutating caller's Data doesn't affect loaded aggregate.
func (s *DataTestSuite) TestAliasImmunityLoadEncounter() {
	s.Run("mutating caller's Data after LoadEncounter doesn't affect loaded aggregate", func() {
		setup := &encounter.SetupInput{
			Field: encounter.FieldInput{
				Rooms: []encounter.RoomInput{
					{
						ID:         "room1",
						Width:      5,
						Height:     5,
						Occluders:  []spatial.Position{},
						Boundaries: []spatial.Boundary{},
					},
				},
				Connections: []encounter.ConnectionInput{},
			},
			Members: []encounter.MemberInput{
				{
					ID:       "p1",
					Kind:     encounter.KindPlayer,
					Room:     "room1",
					Position: spatial.Position{X: 2, Y: 2},
				},
			},
			Endings: []encounter.EndingInput{
				{
					Key:     "done",
					Trigger: encounter.TriggerReachedPosition{Room: "room1", Position: spatial.Position{X: 0, Y: 0}},
				},
			},
		}

		enc1, err := encounter.NewEncounter(setup)
		s.Require().NoError(err)

		data := enc1.ToData()

		// Load FIRST, then vandalize the caller's Data: the loaded
		// aggregate must be untouched (load-side deep copy).
		enc2, err := encounter.LoadEncounter(data, nil)
		s.Require().NoError(err)

		data.Members[0].ID = "mutated"
		data.EverMembers[0] = "mutated"
		data.Endings[0].Key = "mutated"
		data.Field.Rooms[0].ID = "mutated"

		members, err := enc2.Members()
		s.Require().NoError(err)
		s.Equal(encounter.MemberID("p1"), members[0].ID,
			"mutating the caller's Data after load must not reach the aggregate")
		story, err := enc2.Story(&encounter.StoryInput{Audience: "p1"})
		s.Require().NoError(err, "p1 still story-visible (everMembers not aliased)")
		s.Require().NotEmpty(story)
	})
}

// TestNoSurveilOnLoad verifies loading mid-fade doesn't refresh to Current.
func (s *DataTestSuite) TestNoSurveilOnLoad() {
	s.Run("load consumes intel verbatim — never re-derives sight", func() {
		// Beliefs and geometry may legally diverge (C2: intel is what
		// observers BELIEVE, not derivable world state). Build a state
		// where re-derivation would DISAGREE with the loaded belief:
		// alice and the goblin in clear line of sight, but alice's
		// persisted holding says Held (a ghost). A load that re-runs
		// first-light surveil would resurrect it to Current.
		enc1, err := encounter.NewEncounter(&encounter.SetupInput{
			Field: encounter.FieldInput{Rooms: []encounter.RoomInput{{ID: "crypt", Width: 10, Height: 10}}},
			Members: []encounter.MemberInput{
				{ID: "playerA", Kind: encounter.KindPlayer, Room: "crypt", Position: spatial.Position{X: 1, Y: 1}},
				{ID: "goblin", Kind: encounter.KindMonster, Room: "crypt", Position: spatial.Position{X: 5, Y: 5}},
			},
			Endings: []encounter.EndingInput{{Key: "stairs",
				Trigger: encounter.TriggerReachedPosition{Room: "crypt", Position: spatial.Position{X: 0, Y: 0}}}},
		})
		s.Require().NoError(err)

		data := enc1.ToData()

		// Surgical belief edit: alice's holding of the goblin becomes a
		// ghost (CurrentVia cleared) — legal intel data, divergent from
		// the clear-LoS geometry.
		holding := data.Intel.Holdings["playerA"]["goblin"]
		holding.CurrentVia = nil
		data.Intel.Holdings["playerA"]["goblin"] = holding

		enc2, err := encounter.LoadEncounter(data, nil)
		s.Require().NoError(err)

		view, err := enc2.View(&encounter.ViewInput{Member: "playerA"})
		s.Require().NoError(err)
		s.Require().Len(view, 1)
		s.Equal(intel.Held, view[0].Status,
			"the loaded belief (a ghost) must survive verbatim — a load that re-surveils would resurrect it to Current")
	})
}
func (s *DataTestSuite) TestDeciderReattachment() {
	s.Run("reload with decider resumes monster decision", func() {
		setup := &encounter.SetupInput{
			Field: encounter.FieldInput{
				Rooms: []encounter.RoomInput{
					{
						ID:         "crypt",
						Width:      10,
						Height:     10,
						Occluders:  []spatial.Position{},
						Boundaries: []spatial.Boundary{},
					},
				},
				Connections: []encounter.ConnectionInput{},
			},
			Members: []encounter.MemberInput{
				{
					ID:       "playerA",
					Kind:     encounter.KindPlayer,
					Room:     "crypt",
					Position: spatial.Position{X: 1, Y: 1},
				},
				{
					ID:       "goblin",
					Kind:     encounter.KindMonster,
					Room:     "crypt",
					Position: spatial.Position{X: 8, Y: 8},
					Decider:  &testDecider{intent: encounter.IntentHold{}},
				},
			},
			Endings: []encounter.EndingInput{
				{
					Key:     "stairs",
					Trigger: encounter.TriggerReachedPosition{Room: "crypt", Position: spatial.Position{X: 0, Y: 0}},
				},
			},
		}

		enc1, err := encounter.NewEncounter(setup)
		s.Require().NoError(err)

		data1 := enc1.ToData()

		// Load with goblin's decider re-attached
		decider := &testDecider{
			intent: encounter.IntentMoveTo{
				To: spatial.Position{X: 7, Y: 7},
			},
		}
		enc2, err := encounter.LoadEncounter(data1, map[encounter.MemberID]encounter.Decider{
			encounter.MemberID("goblin"): decider,
		})
		s.Require().NoError(err)

		// Pump should execute the decider's move intent
		out, err := enc2.Pump(&encounter.PumpInput{})
		s.Require().NoError(err)
		s.Require().NotNil(out)

		// Goblin should have moved
		s.Len(out.MonsterMoves, 1)
		s.Equal(encounter.MemberID("goblin"), out.MonsterMoves[0].Member)
	})
}

// TestDeciderReattachmentWithoutDecider verifies monster holds without decider.
func (s *DataTestSuite) TestDeciderReattachmentWithoutDecider() {
	s.Run("reload without decider makes monster hold", func() {
		setup := &encounter.SetupInput{
			Field: encounter.FieldInput{
				Rooms: []encounter.RoomInput{
					{
						ID:         "crypt",
						Width:      10,
						Height:     10,
						Occluders:  []spatial.Position{},
						Boundaries: []spatial.Boundary{},
					},
				},
				Connections: []encounter.ConnectionInput{},
			},
			Members: []encounter.MemberInput{
				{
					ID:       "playerA",
					Kind:     encounter.KindPlayer,
					Room:     "crypt",
					Position: spatial.Position{X: 1, Y: 1},
				},
				{
					ID:       "goblin",
					Kind:     encounter.KindMonster,
					Room:     "crypt",
					Position: spatial.Position{X: 8, Y: 8},
					Decider:  &testDecider{intent: encounter.IntentHold{}},
				},
			},
			Endings: []encounter.EndingInput{
				{
					Key:     "stairs",
					Trigger: encounter.TriggerReachedPosition{Room: "crypt", Position: spatial.Position{X: 0, Y: 0}},
				},
			},
		}

		enc1, err := encounter.NewEncounter(setup)
		s.Require().NoError(err)

		data1 := enc1.ToData()

		// Load WITHOUT goblin's decider
		enc2, err := encounter.LoadEncounter(data1, map[encounter.MemberID]encounter.Decider{})
		s.Require().NoError(err)

		// Pump should succeed (goblin holds)
		out, err := enc2.Pump(&encounter.PumpInput{})
		s.Require().NoError(err)
		s.Require().NotNil(out)

		// Goblin should not have moved
		s.Len(out.MonsterMoves, 0)
	})
}

// TestDeciderReattachmentNilEntryHolds pins reject-never-crash at the
// reattachment map itself: a caller-supplied entry that is PRESENT but
// nil (map[MemberID]Decider{"goblin": nil}, distinct from an ABSENT key —
// TestDeciderReattachmentWithoutDecider's case) must not panic Pump. A
// nil entry is equivalent to an absent one: the monster simply holds.
func (s *DataTestSuite) TestDeciderReattachmentNilEntryHolds() {
	setup := &encounter.SetupInput{
		Field: encounter.FieldInput{
			Rooms: []encounter.RoomInput{{ID: "crypt", Width: 10, Height: 10}},
		},
		Members: []encounter.MemberInput{
			{ID: "playerA", Kind: encounter.KindPlayer, Room: "crypt", Position: spatial.Position{X: 1, Y: 1}},
			{ID: "goblin", Kind: encounter.KindMonster, Room: "crypt", Position: spatial.Position{X: 8, Y: 8},
				Decider: &testDecider{intent: encounter.IntentHold{}}},
		},
		Endings: []encounter.EndingInput{{Key: "stairs", Trigger: encounter.TriggerReachedPosition{
			Room: "crypt", Position: spatial.Position{X: 0, Y: 0}}}},
	}
	enc1, err := encounter.NewEncounter(setup)
	s.Require().NoError(err)
	data1 := enc1.ToData()

	enc2, err := encounter.LoadEncounter(data1, map[encounter.MemberID]encounter.Decider{
		"goblin": nil,
	})
	s.Require().NoError(err, "a present-but-nil reattachment entry must load, not reject")

	s.Require().NotPanics(func() {
		out, pumpErr := enc2.Pump(&encounter.PumpInput{})
		s.Require().NoError(pumpErr, "the first pump must not panic on a nil-decider monster")
		s.Empty(out.MonsterMoves, "a nil-decider monster is absent from decisions and beats — it simply holds")
	})
}

// TestDeciderReattachmentMixedNilAndReal pins that a nil entry for one
// monster does not disturb a real decider re-attached for another in the
// same reattachment map: the real one decides normally, the nil one holds.
func (s *DataTestSuite) TestDeciderReattachmentMixedNilAndReal() {
	setup := &encounter.SetupInput{
		Field: encounter.FieldInput{
			Rooms: []encounter.RoomInput{{ID: "crypt", Width: 10, Height: 10}},
		},
		Members: []encounter.MemberInput{
			{ID: "playerA", Kind: encounter.KindPlayer, Room: "crypt", Position: spatial.Position{X: 1, Y: 1}},
			{ID: "goblin", Kind: encounter.KindMonster, Room: "crypt", Position: spatial.Position{X: 8, Y: 8},
				Decider: &testDecider{intent: encounter.IntentHold{}}},
			{ID: "rat", Kind: encounter.KindMonster, Room: "crypt", Position: spatial.Position{X: 2, Y: 8},
				Decider: &testDecider{intent: encounter.IntentHold{}}},
		},
		Endings: []encounter.EndingInput{{Key: "stairs", Trigger: encounter.TriggerReachedPosition{
			Room: "crypt", Position: spatial.Position{X: 0, Y: 0}}}},
	}
	enc1, err := encounter.NewEncounter(setup)
	s.Require().NoError(err)
	data1 := enc1.ToData()

	ratDecider := &testDecider{intent: encounter.IntentMoveTo{To: spatial.Position{X: 3, Y: 8}}}
	enc2, err := encounter.LoadEncounter(data1, map[encounter.MemberID]encounter.Decider{
		"goblin": nil,
		"rat":    ratDecider,
	})
	s.Require().NoError(err)

	out, err := enc2.Pump(&encounter.PumpInput{})
	s.Require().NoError(err)
	s.Require().Len(out.MonsterMoves, 1, "only rat's real decider produces a move")
	s.Equal(encounter.MemberID("rat"), out.MonsterMoves[0].Member)
	s.Equal(spatial.Position{X: 3, Y: 8}, out.MonsterMoves[0].To)
}

// ============================================================================
// Rejection Tests
// ============================================================================

// TestRejectNilData rejects nil-equivalent zero Data.
// validEncounterData is the minimal fully-loadable Data: every
// rejection case below starts here and breaks EXACTLY ONE thing, then
// asserts the DISCRIMINATING message fragment — the T6 review proved
// that fixtures invalid in several ways let the last-run check absorb
// every deletion (7 of 8 checks were individually deletable, suite
// green). One defect per fixture makes each check's pin falsifiable.

// testDecider returns a fixed intent every time (persistence-test fixture).
type testDecider struct {
	intent encounter.Intent
}

func (d *testDecider) Decide(_ encounter.Snapshot) (encounter.Intent, error) {
	return d.intent, nil
}

func validEncounterData() encounter.EncounterData {
	return encounter.EncounterData{
		Field: encounter.FieldData{Rooms: []encounter.RoomData{{ID: "r1", Width: 5, Height: 5}}},
		Members: []encounter.MemberData{
			{ID: "p1", Kind: encounter.KindPlayer, Room: "r1", Position: encounter.PositionData{X: 1, Y: 1}},
		},
		Endings:     []encounter.EndingData{{Key: "done", Kind: "external"}},
		EverMembers: []encounter.MemberID{"p1"},
	}
}

// validEncounterDataWithConnection extends validEncounterData with a second,
// DELIBERATELY mismatched room (r1 resized to 10x4, r2 added at 3x9) and one
// fully valid connection between them — the base for the connection defect
// rows below (one-defect discipline: each row breaks exactly one thing about
// this otherwise-valid connection). FromPosition{7,1} is valid ONLY in r1
// and ToPosition{1,7} valid ONLY in r2: same-sized rooms and equal From/To
// positions would make a check that validates an endpoint against the WRONG
// room (or a From/To transposition in convertConnectionDataToConnectionInput)
// invisible. r1 carries an occluder at (2,2) and r2 an occluder at (1,3), so
// both the from- and to-side "endpoint on occluder" rows have something to hit.
func validEncounterDataWithConnection() encounter.EncounterData {
	d := validEncounterData()
	d.Field.Rooms[0].Width = 10
	d.Field.Rooms[0].Height = 4
	d.Field.Rooms[0].Occluders = []encounter.PositionData{{X: 2, Y: 2}}
	d.Field.Rooms = append(d.Field.Rooms, encounter.RoomData{
		ID: "r2", Width: 3, Height: 9,
		Occluders: []encounter.PositionData{{X: 1, Y: 3}},
	})
	d.Field.Connections = []encounter.ConnectionData{
		{ID: "c1", From: "r1", To: "r2",
			FromPosition: &encounter.PositionData{X: 7, Y: 1},
			ToPosition:   &encounter.PositionData{X: 1, Y: 7}},
	}
	return d
}

// TestLoadRejections: every unreachable state rejects with ErrInvalidData
// AND the check that fired is the one the case targets. Connection rows
// also assert ErrBadConnection (via alsoErr) and, where a room name is
// involved, quote the missing room in the fragment — a check neutered in
// favor of the coincidental zero-value-room bounds fallback must not pass.
func (s *DataTestSuite) TestLoadRejections() {
	cases := []struct {
		name     string
		mutate   func(d *encounter.EncounterData)
		fragment string
		alsoErr  error
	}{
		{"zero data", func(d *encounter.EncounterData) { *d = encounter.EncounterData{} }, "no rooms", nil},
		{"no rooms", func(d *encounter.EncounterData) { d.Field.Rooms = nil; d.Members = nil; d.EverMembers = nil }, "no rooms", nil},
		{"no endings", func(d *encounter.EncounterData) { d.Endings = nil }, "bad endings", nil},
		{"empty ending key", func(d *encounter.EncounterData) { d.Endings[0].Key = "" }, "bad endings", nil},
		{"reserved ending key", func(d *encounter.EncounterData) { d.Endings[0].Key = "abandoned" }, "bad endings", nil},
		{"unknown ending kind", func(d *encounter.EncounterData) { d.Endings[0].Kind = "psychic" }, "unknown ending kind", nil},
		{"reached_position without position", func(d *encounter.EncounterData) {
			d.Endings[0] = encounter.EndingData{Key: "done", Kind: "reached_position", Room: "r1"}
		}, "without room/position", nil},
		{"empty member id", func(d *encounter.EncounterData) { d.Members[0].ID = "" }, "empty member id", nil},
		{"duplicate member ids", func(d *encounter.EncounterData) {
			d.Members = append(d.Members, d.Members[0])
		}, "duplicate member", nil},
		{"member room not in field", func(d *encounter.EncounterData) { d.Members[0].Room = "nowhere" }, "not in field", nil},
		{"member out of bounds", func(d *encounter.EncounterData) {
			d.Members[0].Position = encounter.PositionData{X: 99, Y: 99}
		}, "out of bounds", nil},
		{"connection missing room", func(d *encounter.EncounterData) {
			d.Field.Connections = []encounter.ConnectionData{{ID: "c1", From: "r1", To: "nowhere"}}
		}, `missing room "nowhere"`, encounter.ErrBadConnection},
		{"connection empty id", func(d *encounter.EncounterData) {
			*d = validEncounterDataWithConnection()
			d.Field.Connections[0].ID = ""
		}, "empty id", encounter.ErrBadConnection},
		{"duplicate connection ids", func(d *encounter.EncounterData) {
			*d = validEncounterDataWithConnection()
			d.Field.Connections = append(d.Field.Connections, d.Field.Connections[0])
		}, "duplicate connection", encounter.ErrBadConnection},
		{"connection unknown from room", func(d *encounter.EncounterData) {
			*d = validEncounterDataWithConnection()
			d.Field.Connections[0].From = "nowhere"
		}, `missing room "nowhere"`, encounter.ErrBadConnection},
		{"connection unknown to room", func(d *encounter.EncounterData) {
			*d = validEncounterDataWithConnection()
			d.Field.Connections[0].To = "nowhere"
		}, `missing room "nowhere"`, encounter.ErrBadConnection},
		{"connection self-connection", func(d *encounter.EncounterData) {
			*d = validEncounterDataWithConnection()
			d.Field.Connections[0].To = "r1"
		}, "itself", encounter.ErrBadConnection},
		{"connection from-position out of bounds", func(d *encounter.EncounterData) {
			*d = validEncounterDataWithConnection()
			d.Field.Connections[0].FromPosition = &encounter.PositionData{X: 99, Y: 99}
		}, "from-position out of bounds", encounter.ErrBadConnection},
		{"connection to-position out of bounds", func(d *encounter.EncounterData) {
			*d = validEncounterDataWithConnection()
			d.Field.Connections[0].ToPosition = &encounter.PositionData{X: 99, Y: 99}
		}, "to-position out of bounds", encounter.ErrBadConnection},
		{"connection from-position on occluder", func(d *encounter.EncounterData) {
			*d = validEncounterDataWithConnection()
			d.Field.Connections[0].FromPosition = &encounter.PositionData{X: 2, Y: 2}
		}, "from-position on occluder", encounter.ErrBadConnection},
		{"connection to-position on occluder", func(d *encounter.EncounterData) {
			*d = validEncounterDataWithConnection()
			d.Field.Connections[0].ToPosition = &encounter.PositionData{X: 1, Y: 3}
		}, "to-position on occluder", encounter.ErrBadConnection},
		{"connection missing from_position", func(d *encounter.EncounterData) {
			// Deletes the field from the wire path (nil), not a zero-valued
			// position — a missing endpoint must never silently default to
			// (0,0), a legal cell that would invent topology.
			*d = validEncounterDataWithConnection()
			d.Field.Connections[0].FromPosition = nil
		}, "missing from_position", encounter.ErrBadConnection},
		{"connection missing to_position", func(d *encounter.EncounterData) {
			*d = validEncounterDataWithConnection()
			d.Field.Connections[0].ToPosition = nil
		}, "missing to_position", encounter.ErrBadConnection},
		{"outcome undeclared ending", func(d *encounter.EncounterData) {
			d.Outcome = &encounter.OutcomeData{Ending: "never-declared"}
		}, "outcome", nil},
		{"abandoned outcome with members present", func(d *encounter.EncounterData) {
			d.Outcome = &encounter.OutcomeData{Ending: "abandoned"}
		}, "abandoned outcome with members", nil},
		{"outcome member room missing", func(d *encounter.EncounterData) {
			d.Outcome = &encounter.OutcomeData{Ending: "done", Members: []encounter.MemberOutcomeData{
				{ID: "ghost", Room: "nowhere", Position: encounter.PositionData{X: 1, Y: 1}}}}
		}, "outcome member", nil},
		{"outcome member out of bounds", func(d *encounter.EncounterData) {
			d.Outcome = &encounter.OutcomeData{Ending: "done", Members: []encounter.MemberOutcomeData{
				{ID: "p1", Room: "r1", Position: encounter.PositionData{X: 999, Y: 999}}}}
		}, "out of bounds", nil},
		{"ever_members missing current member", func(d *encounter.EncounterData) {
			d.EverMembers = nil
		}, "ever_members", nil},
	}
	for _, tc := range cases {
		s.Run(tc.name, func() {
			data := validEncounterData()
			tc.mutate(&data)
			_, err := encounter.LoadEncounter(data, nil)
			s.Require().Error(err, tc.name)
			s.Require().ErrorIs(err, encounter.ErrInvalidData, tc.name)
			s.Require().Contains(err.Error(), tc.fragment,
				"the check that fired must be the one this case targets")
			if tc.alsoErr != nil {
				s.Require().ErrorIs(err, tc.alsoErr, tc.name)
			}
		})
	}

	// The valid base itself must load — the one-defect discipline only
	// means something if zero defects pass.
	_, err := encounter.LoadEncounter(validEncounterData(), nil)
	s.Require().NoError(err, "the valid base fixture must load")

	// The valid CONNECTION base must also load. Since FromPosition{7,1} is
	// valid ONLY in r1 and ToPosition{1,7} valid ONLY in r2, this positive
	// control pins that each endpoint is checked against ITS OWN room (a
	// check wired to the wrong room would reject this valid connection),
	// and that endpoints survive Load unswapped (a From/To transposition
	// in convertConnectionDataToConnectionInput would not error here —
	// it would silently swap the values — so the values are re-inspected,
	// not just the absence of an error).
	connEnc, err := encounter.LoadEncounter(validEncounterDataWithConnection(), nil)
	s.Require().NoError(err, "the valid connection base fixture must load")
	connData := connEnc.ToData()
	s.Require().Len(connData.Field.Connections, 1)
	s.Equal(&encounter.PositionData{X: 7, Y: 1}, connData.Field.Connections[0].FromPosition,
		"from-position must survive Load unswapped")
	s.Equal(&encounter.PositionData{X: 1, Y: 7}, connData.Field.Connections[0].ToPosition,
		"to-position must survive Load unswapped")
}

// connBoundsData returns a fresh EncounterData with a 4x3 room r1 (valid
// coordinates 0..3 x 0..2) and one connection — the Load-seam counterpart to
// encounter_test.go's connBoundsSetup, pinning the square grid's strictly-
// less-than bounds semantics at Load independent of any cross-room concern.
func connBoundsData() encounter.EncounterData {
	return encounter.EncounterData{
		Field: encounter.FieldData{
			Rooms: []encounter.RoomData{
				{ID: "r1", Width: 4, Height: 3},
				{ID: "r2", Width: 4, Height: 3},
			},
			Connections: []encounter.ConnectionData{
				{ID: "c1", From: "r1", To: "r2",
					FromPosition: &encounter.PositionData{X: 0, Y: 0},
					ToPosition:   &encounter.PositionData{X: 0, Y: 0}},
			},
		},
		Members: []encounter.MemberData{
			{ID: "p1", Kind: encounter.KindPlayer, Room: "r1", Position: encounter.PositionData{X: 0, Y: 0}},
		},
		Endings:     []encounter.EndingData{{Key: "done", Kind: "external"}},
		EverMembers: []encounter.MemberID{"p1"},
	}
}

// TestConnectionEndpointBoundsBoundariesLoad is the Load-seam counterpart to
// encounter_test.go's TestConnectionEndpointBoundsBoundaries (#922 T1 Opus
// review, minor M3/M4): a coordinate exactly at the room's Width/Height is
// out of bounds, a negative coordinate is out of bounds, and Width-1/Height-1
// — the last valid cell — is accepted.
func (s *DataTestSuite) TestConnectionEndpointBoundsBoundariesLoad() {
	s.Run("X exactly at width is rejected", func() {
		data := connBoundsData()
		data.Field.Connections[0].FromPosition = &encounter.PositionData{X: 4, Y: 0}
		_, err := encounter.LoadEncounter(data, nil)
		s.Require().ErrorIs(err, encounter.ErrBadConnection)
		s.Require().Contains(err.Error(), "from-position out of bounds")
	})

	s.Run("Y exactly at height is rejected", func() {
		data := connBoundsData()
		data.Field.Connections[0].FromPosition = &encounter.PositionData{X: 0, Y: 3}
		_, err := encounter.LoadEncounter(data, nil)
		s.Require().ErrorIs(err, encounter.ErrBadConnection)
		s.Require().Contains(err.Error(), "from-position out of bounds")
	})

	s.Run("negative X is rejected", func() {
		data := connBoundsData()
		data.Field.Connections[0].FromPosition = &encounter.PositionData{X: -1, Y: 0}
		_, err := encounter.LoadEncounter(data, nil)
		s.Require().ErrorIs(err, encounter.ErrBadConnection)
		s.Require().Contains(err.Error(), "from-position out of bounds")
	})

	s.Run("negative Y is rejected", func() {
		data := connBoundsData()
		data.Field.Connections[0].FromPosition = &encounter.PositionData{X: 0, Y: -1}
		_, err := encounter.LoadEncounter(data, nil)
		s.Require().ErrorIs(err, encounter.ErrBadConnection)
		s.Require().Contains(err.Error(), "from-position out of bounds")
	})

	s.Run("Width-1,Height-1 is accepted (positive control)", func() {
		data := connBoundsData()
		data.Field.Connections[0].FromPosition = &encounter.PositionData{X: 3, Y: 2}
		_, err := encounter.LoadEncounter(data, nil)
		s.Require().NoError(err, "the last valid cell must be accepted")
	})
}

// TestLoadRoomValidation is the Load-seam counterpart to
// encounter_test.go's TestSetupRoomValidation (#922 T1.5, deferred from
// the Opus T1 review): empty room ID, duplicate room ID, and an
// unrecognized grid shape all reject with ErrInvalidData + ErrNoField.
func (s *DataTestSuite) TestLoadRoomValidation() {
	cases := []struct {
		name     string
		mutate   func(d *encounter.EncounterData)
		fragment string
	}{
		{"room has empty id", func(d *encounter.EncounterData) {
			d.Field.Rooms[0].ID = ""
		}, "room has empty id"},
		{"duplicate room id", func(d *encounter.EncounterData) {
			d.Field.Rooms = append(d.Field.Rooms, d.Field.Rooms[0])
		}, "duplicate room"},
		{"room has unknown grid shape", func(d *encounter.EncounterData) {
			d.Field.Rooms[0].Grid = "triangle"
		}, "unknown grid shape"},
	}
	for _, tc := range cases {
		s.Run(tc.name, func() {
			data := validEncounterData()
			tc.mutate(&data)
			_, err := encounter.LoadEncounter(data, nil)
			s.Require().Error(err, tc.name)
			s.Require().ErrorIs(err, encounter.ErrInvalidData, tc.name)
			s.Require().ErrorIs(err, encounter.ErrNoField, tc.name)
			s.Require().Contains(err.Error(), tc.fragment,
				"the check that fired must be the one this case targets")
		})
	}
}

// connHexRoomData returns a fresh EncounterData with one 4x3 hex room and a
// member at pos — the Load-seam counterpart to encounter_test.go's
// TestHexRoomBounds. Width=4, Height=3 => Q valid in [-2,2), R valid in
// [-1.5,1.5) (axial, origin-centered — see that test's comment).
func connHexRoomData(pos encounter.PositionData) encounter.EncounterData {
	return encounter.EncounterData{
		Field: encounter.FieldData{
			Rooms: []encounter.RoomData{
				{ID: "r1", Width: 4, Height: 3, Grid: spatial.GridTypeHex},
			},
		},
		Members: []encounter.MemberData{
			{ID: "p1", Kind: encounter.KindPlayer, Room: "r1", Position: pos},
		},
		Endings:     []encounter.EndingData{{Key: "done", Kind: "external"}},
		EverMembers: []encounter.MemberID{"p1"},
	}
}

// TestHexRoomBoundsLoad is the Load-seam counterpart to
// encounter_test.go's TestHexRoomBounds.
func (s *DataTestSuite) TestHexRoomBoundsLoad() {
	s.Run("positive Q, positive R within span accepted", func() {
		_, err := encounter.LoadEncounter(connHexRoomData(encounter.PositionData{X: 1, Y: 1}), nil)
		s.Require().NoError(err)
	})

	s.Run("negative Q within span accepted — rejected under the old offset HexGrid", func() {
		_, err := encounter.LoadEncounter(connHexRoomData(encounter.PositionData{X: -1, Y: 0}), nil)
		s.Require().NoError(err, "axial hex rooms are origin-centered; negative Q is ordinary, not a defect")
	})

	s.Run("Q at exactly +Width/2 rejected (upper bound exclusive)", func() {
		_, err := encounter.LoadEncounter(connHexRoomData(encounter.PositionData{X: 2, Y: 0}), nil)
		s.Require().Error(err)
		s.Require().ErrorIs(err, encounter.ErrInvalidData)
		s.Require().Contains(err.Error(), "out of bounds")
	})

	s.Run("Q at exactly -Width/2 accepted (lower bound inclusive)", func() {
		_, err := encounter.LoadEncounter(connHexRoomData(encounter.PositionData{X: -2, Y: 0}), nil)
		s.Require().NoError(err)
	})

	s.Run("Q beyond -Width/2 rejected", func() {
		_, err := encounter.LoadEncounter(connHexRoomData(encounter.PositionData{X: -3, Y: 0}), nil)
		s.Require().Error(err)
		s.Require().ErrorIs(err, encounter.ErrInvalidData)
		s.Require().Contains(err.Error(), "out of bounds")
	})
}

// TestHexConnectionEndpointNegativeAxialLoad is the Load-seam counterpart
// to encounter_test.go's TestHexConnectionEndpointNegativeAxial.
func (s *DataTestSuite) TestHexConnectionEndpointNegativeAxialLoad() {
	data := encounter.EncounterData{
		Field: encounter.FieldData{
			Rooms: []encounter.RoomData{
				{ID: "square-room", Width: 10, Height: 10},
				{ID: "hex-room", Width: 6, Height: 6, Grid: spatial.GridTypeHex},
			},
			Connections: []encounter.ConnectionData{{
				ID: "gate", From: "square-room", To: "hex-room",
				FromPosition: &encounter.PositionData{X: 9, Y: 9},
				ToPosition:   &encounter.PositionData{X: -2, Y: -2},
			}},
		},
		Members: []encounter.MemberData{
			{ID: "p1", Kind: encounter.KindPlayer, Room: "square-room", Position: encounter.PositionData{X: 1, Y: 1}},
		},
		Endings:     []encounter.EndingData{{Key: "done", Kind: "external"}},
		EverMembers: []encounter.MemberID{"p1"},
	}
	_, err := encounter.LoadEncounter(data, nil)
	s.Require().NoError(err, "a connection endpoint at a negative axial coordinate must validate")
}

// validHexAxialData returns a fresh EncounterData with two hex rooms
// joined by one connection, a member, and an occluder — the Load-seam
// counterpart to encounter_test.go's validHexAxialSetup. Every position
// is integral axial, including a negative one (gate.ToPosition).
func validHexAxialData() encounter.EncounterData {
	return encounter.EncounterData{
		Field: encounter.FieldData{
			Rooms: []encounter.RoomData{
				{ID: "hex-a", Width: 8, Height: 8, Grid: spatial.GridTypeHex,
					Occluders: []encounter.PositionData{{X: 2, Y: 2}}},
				{ID: "hex-b", Width: 8, Height: 8, Grid: spatial.GridTypeHex},
			},
			Connections: []encounter.ConnectionData{{
				ID: "gate", From: "hex-a", To: "hex-b",
				FromPosition: &encounter.PositionData{X: 1, Y: 1},
				ToPosition:   &encounter.PositionData{X: -1, Y: -1},
			}},
		},
		Members: []encounter.MemberData{
			{ID: "p1", Kind: encounter.KindPlayer, Room: "hex-a", Position: encounter.PositionData{X: 0, Y: 0}},
		},
		Endings:     []encounter.EndingData{{Key: "done", Kind: "external"}},
		EverMembers: []encounter.MemberID{"p1"},
	}
}

// TestLoadHexIntegralAxial is the Load-seam counterpart to
// encounter_test.go's TestSetupHexIntegralAxial.
func (s *DataTestSuite) TestLoadHexIntegralAxial() {
	cases := []struct {
		name    string
		mutate  func(d *encounter.EncounterData)
		alsoErr error
	}{
		{"member position fractional", func(d *encounter.EncounterData) {
			d.Members[0].Position = encounter.PositionData{X: 0.5, Y: 0}
		}, nil},
		{"connection from-position fractional", func(d *encounter.EncounterData) {
			d.Field.Connections[0].FromPosition = &encounter.PositionData{X: 1.5, Y: 1}
		}, encounter.ErrBadConnection},
		{"connection to-position fractional", func(d *encounter.EncounterData) {
			d.Field.Connections[0].ToPosition = &encounter.PositionData{X: -1.5, Y: -1}
		}, encounter.ErrBadConnection},
		{"occluder position fractional", func(d *encounter.EncounterData) {
			d.Field.Rooms[0].Occluders[0] = encounter.PositionData{X: 2.5, Y: 2}
		}, encounter.ErrNoField},
	}
	for _, tc := range cases {
		s.Run(tc.name, func() {
			data := validHexAxialData()
			tc.mutate(&data)
			_, err := encounter.LoadEncounter(data, nil)
			s.Require().Error(err, tc.name)
			s.Require().ErrorIs(err, encounter.ErrInvalidData, tc.name)
			if tc.alsoErr != nil {
				s.Require().ErrorIs(err, tc.alsoErr, tc.name)
			}
			s.Require().Contains(err.Error(), "not an integral axial cell",
				"the check that fired must be the one this case targets")
		})
	}

	_, err := encounter.LoadEncounter(validHexAxialData(), nil)
	s.Require().NoError(err, "integral axial positions, including negative ones, must be accepted")
}

// connGridlessRoomData returns a fresh EncounterData with one 4x3 gridless
// room and a member at pos — the Load-seam counterpart to
// encounter_test.go's TestGridlessRoomInclusiveBounds.
func connGridlessRoomData(pos encounter.PositionData) encounter.EncounterData {
	return encounter.EncounterData{
		Field: encounter.FieldData{
			Rooms: []encounter.RoomData{
				{ID: "r1", Width: 4, Height: 3, Grid: spatial.GridTypeGridless},
			},
		},
		Members: []encounter.MemberData{
			{ID: "p1", Kind: encounter.KindPlayer, Room: "r1", Position: pos},
		},
		Endings:     []encounter.EndingData{{Key: "done", Kind: "external"}},
		EverMembers: []encounter.MemberID{"p1"},
	}
}

// TestGridlessRoomInclusiveBoundsLoad is the Load-seam counterpart to
// encounter_test.go's TestGridlessRoomInclusiveBounds — see that test's
// comment for why this is the sharpest available proof that bounds checks
// ask the room's own constructed grid rather than a hardcoded rectangle.
func (s *DataTestSuite) TestGridlessRoomInclusiveBoundsLoad() {
	s.Run("position exactly at Width is accepted (inclusive upper bound)", func() {
		_, err := encounter.LoadEncounter(connGridlessRoomData(encounter.PositionData{X: 4, Y: 0}), nil)
		s.Require().NoError(err, "gridless rooms accept x == Width; a rectangle-math fallback would reject this")
	})

	s.Run("position negative is still rejected", func() {
		_, err := encounter.LoadEncounter(connGridlessRoomData(encounter.PositionData{X: -1, Y: 0}), nil)
		s.Require().Error(err)
		s.Require().ErrorIs(err, encounter.ErrInvalidData)
		s.Require().Contains(err.Error(), "out of bounds")
	})
}

// TestLoadRejectsPlayerWithDecider pins C2 at the third seam: a player
// cannot carry a decider at load any more than at Setup or Join.
func (s *DataTestSuite) TestLoadRejectsPlayerWithDecider() {
	data := validEncounterData()
	_, err := encounter.LoadEncounter(data, map[encounter.MemberID]encounter.Decider{
		"p1": &spyDecider{},
	})
	s.Require().ErrorIs(err, encounter.ErrInvalidData)
	s.Require().Contains(err.Error(), "cannot carry a decider")
}

func (s *DataTestSuite) TestMutation1ToDataAliases() {
	s.Run("mutation 1: ToData aliases slices", func() {
		setup := &encounter.SetupInput{
			Field: encounter.FieldInput{
				Rooms: []encounter.RoomInput{
					{ID: "room1", Width: 5, Height: 5, Occluders: []spatial.Position{}, Boundaries: []spatial.Boundary{}},
				},
				Connections: []encounter.ConnectionInput{},
			},
			Members: []encounter.MemberInput{
				{ID: "p1", Kind: encounter.KindPlayer, Room: "room1", Position: spatial.Position{X: 2, Y: 2}},
			},
			Endings: []encounter.EndingInput{
				{Key: "done", Trigger: encounter.TriggerReachedPosition{Room: "room1", Position: spatial.Position{X: 0, Y: 0}}},
			},
		}

		enc, err := encounter.NewEncounter(setup)
		s.Require().NoError(err)

		data1 := enc.ToData()
		data2 := enc.ToData()

		// Mutate data1's slices
		if len(data1.Members) > 0 {
			data1.Members[0].ID = "mutated"
		}
		if len(data1.Endings) > 0 {
			data1.Endings[0].Key = "mutated"
		}

		// If this mutation test passes, data2 should be unaffected
		// (If mutation test FAILS, data2 will be affected, which violates the requirement)
		s.NotEqual("mutated", data2.Members[0].ID, "ToData must deep-copy Members slice")
		s.NotEqual("mutated", data2.Endings[0].Key, "ToData must deep-copy Endings slice")
	})
}

// Mutation 2: Wire tag renamed (json tag typo)
func (s *DataTestSuite) TestMutation2WireTagRenamed() {
	s.Run("mutation 2: wire tag renamed", func() {
		setup := &encounter.SetupInput{
			Field: encounter.FieldInput{
				Rooms: []encounter.RoomInput{
					{ID: "room1", Width: 5, Height: 5, Occluders: []spatial.Position{}, Boundaries: []spatial.Boundary{}},
				},
				Connections: []encounter.ConnectionInput{},
			},
			Members: []encounter.MemberInput{
				{ID: "p1", Kind: encounter.KindPlayer, Room: "room1", Position: spatial.Position{X: 2, Y: 2}},
			},
			Endings: []encounter.EndingInput{
				{Key: "done", Trigger: encounter.TriggerReachedPosition{Room: "room1", Position: spatial.Position{X: 0, Y: 0}}},
			},
		}

		enc, err := encounter.NewEncounter(setup)
		s.Require().NoError(err)

		data := enc.ToData()
		jsonBytes, _ := json.Marshal(data)
		jsonStr := string(jsonBytes)

		// Must contain exact tag "ever_members" (snake_case)
		s.Contains(jsonStr, `"ever_members"`, "ever_members tag must be present in JSON wire format")
	})
}

// Mutation 3: Stowaway always-marshaled field
func (s *DataTestSuite) TestMutation3StowawayField() {
	s.Run("mutation 3: stowaway field in EncounterData", func() {
		setup := &encounter.SetupInput{
			Field: encounter.FieldInput{
				Rooms: []encounter.RoomInput{
					{ID: "room1", Width: 5, Height: 5, Occluders: []spatial.Position{}, Boundaries: []spatial.Boundary{}},
				},
				Connections: []encounter.ConnectionInput{},
			},
			Members: []encounter.MemberInput{
				{ID: "p1", Kind: encounter.KindPlayer, Room: "room1", Position: spatial.Position{X: 2, Y: 2}},
			},
			Endings: []encounter.EndingInput{
				{Key: "done", Trigger: encounter.TriggerReachedPosition{Room: "room1", Position: spatial.Position{X: 0, Y: 0}}},
			},
		}

		enc, err := encounter.NewEncounter(setup)
		s.Require().NoError(err)

		data := enc.ToData()
		jsonBytes, _ := json.Marshal(data)
		jsonStr := string(jsonBytes)

		// Should NOT contain spurious fields (only: outcome, clock, intel, log, field, members, endings, ever_members)
		s.NotContains(jsonStr, `"stowaway"`, "no stowaway fields should be marshaled")
		s.NotContains(jsonStr, `"extra"`, "no extra fields should be marshaled")
	})
}

// Mutation 4: Leaf substitution (swap Intel and Log)
func (s *DataTestSuite) TestMutation4LeafSubstitution() {
	s.Run("mutation 4: leaf data substitution (Intel/Log swapped)", func() {
		setup := &encounter.SetupInput{
			Field: encounter.FieldInput{
				Rooms: []encounter.RoomInput{
					{ID: "room1", Width: 5, Height: 5, Occluders: []spatial.Position{}, Boundaries: []spatial.Boundary{}},
				},
				Connections: []encounter.ConnectionInput{},
			},
			Members: []encounter.MemberInput{
				{ID: "p1", Kind: encounter.KindPlayer, Room: "room1", Position: spatial.Position{X: 2, Y: 2}},
			},
			Endings: []encounter.EndingInput{
				{Key: "done", Trigger: encounter.TriggerReachedPosition{Room: "room1", Position: spatial.Position{X: 0, Y: 0}}},
			},
		}

		// Two members so p1 actually holds something.
		setup.Members = append(setup.Members, encounter.MemberInput{
			ID: "p2", Kind: encounter.KindPlayer, Room: "room1", Position: spatial.Position{X: 3, Y: 3},
		})
		encA, err := encounter.NewEncounter(setup)
		s.Require().NoError(err)
		dataA := encA.ToData()

		// A second, intel-empty world: same field, p1 alone, no sightings
		// (a lone member surveils an empty percept).
		setupB := *setup
		setupB.Members = setup.Members[:1]
		encB, err := encounter.NewEncounter(&setupB)
		s.Require().NoError(err)
		dataB := encB.ToData()

		// Control: loading dataA verbatim, p1 holds p2.
		ctrl, err := encounter.LoadEncounter(dataA, nil)
		s.Require().NoError(err)
		ctrlView, err := ctrl.View(&encounter.ViewInput{Member: "p1"})
		s.Require().NoError(err)
		s.Require().Len(ctrlView, 1, "control: p1 holds p2")

		// Substitute B's (empty) Intel into A's data: the loaded aggregate
		// must reflect the LOADED intel — empty — proving the Intel field
		// is genuinely consumed, never re-derived from the field.
		dataA.Intel = dataB.Intel
		swapped, err := encounter.LoadEncounter(dataA, nil)
		s.Require().NoError(err)
		swappedView, err := swapped.View(&encounter.ViewInput{Member: "p1"})
		s.Require().NoError(err)
		s.Empty(swappedView, "the substituted (empty) intel is what the aggregate holds — Intel is consumed from data")
	})
}

// Mutation 5: LoadEncounter skips member-room check
func (s *DataTestSuite) TestMutation5MissingRoomCheck() {
	s.Run("mutation 5: missing member-room validation", func() {
		data := encounter.EncounterData{
			Field: encounter.FieldData{
				Rooms: []encounter.RoomData{
					{ID: "room1", Width: 10, Height: 10},
				},
			},
			Members: []encounter.MemberData{
				{
					ID:       "p1",
					Kind:     encounter.KindPlayer,
					Room:     "nonexistent",
					Position: encounter.PositionData{X: 5, Y: 5},
				},
			},
			Endings: []encounter.EndingData{
				{Key: "done", Kind: "reached_position"},
			},
		}

		_, err := encounter.LoadEncounter(data, map[encounter.MemberID]encounter.Decider{})
		s.Require().Error(err)
		s.True(errors.Is(err, encounter.ErrInvalidData), "must validate member rooms exist")
	})
}

// Mutation 6: Re-running first-light surveil on load
func (s *DataTestSuite) TestMutation6ReSurveilOnLoad() {
	s.Run("mutation 6: no re-surveil on load (ghost stays ghost)", func() {
		setup := &encounter.SetupInput{
			Field: encounter.FieldInput{
				Rooms: []encounter.RoomInput{
					{
						ID:         "crypt",
						Width:      10,
						Height:     10,
						Occluders:  []spatial.Position{{X: 5, Y: 5}},
						Boundaries: []spatial.Boundary{},
					},
				},
				Connections: []encounter.ConnectionInput{},
			},
			Members: []encounter.MemberInput{
				{ID: "playerA", Kind: encounter.KindPlayer, Room: "crypt", Position: spatial.Position{X: 1, Y: 1}},
				{ID: "goblin", Kind: encounter.KindMonster, Room: "crypt", Position: spatial.Position{X: 9, Y: 9}, Decider: &testDecider{intent: encounter.IntentHold{}}},
			},
			Endings: []encounter.EndingInput{
				{Key: "stairs", Trigger: encounter.TriggerReachedPosition{Room: "crypt", Position: spatial.Position{X: 0, Y: 0}}},
			},
		}

		enc1, err := encounter.NewEncounter(setup)
		s.Require().NoError(err)

		// Create a ghost
		_, err = enc1.Move(&encounter.MoveInput{Member: "playerA", To: spatial.Position{X: 4, Y: 1}})
		s.Require().NoError(err)
		_, err = enc1.Move(&encounter.MoveInput{Member: "goblin", To: spatial.Position{X: 5, Y: 6}})
		s.Require().NoError(err)

		holdings1, _ := enc1.View(&encounter.ViewInput{Member: "playerA"})
		var goblinStatusBefore intel.Status
		for _, h := range holdings1 {
			if h.Subject == intel.Subject("goblin") {
				goblinStatusBefore = h.Status
				break
			}
		}

		data := enc1.ToData()
		enc2, _ := encounter.LoadEncounter(data, map[encounter.MemberID]encounter.Decider{})

		holdings2, _ := enc2.View(&encounter.ViewInput{Member: "playerA"})
		var goblinStatusAfter intel.Status
		for _, h := range holdings2 {
			if h.Subject == intel.Subject("goblin") {
				goblinStatusAfter = h.Status
				break
			}
		}

		// Status must be preserved (ghost should NOT be re-surveiled to Current)
		s.Equal(goblinStatusBefore, goblinStatusAfter, "re-surveil on load would change ghost to current")
		s.Equal(intel.Held, goblinStatusAfter, "ghost should remain ghost")
	})
}

// Mutation 7: Tick continuation broken (reload resets clock)
func (s *DataTestSuite) TestMutation7TickResetOnLoad() {
	s.Run("mutation 7: tick continuation (clock not reset)", func() {
		setup := &encounter.SetupInput{
			Field: encounter.FieldInput{
				Rooms: []encounter.RoomInput{
					{ID: "room1", Width: 10, Height: 10, Occluders: []spatial.Position{}, Boundaries: []spatial.Boundary{}},
				},
				Connections: []encounter.ConnectionInput{},
			},
			Members: []encounter.MemberInput{
				{ID: "p1", Kind: encounter.KindPlayer, Room: "room1", Position: spatial.Position{X: 5, Y: 5}},
			},
			Endings: []encounter.EndingInput{
				{Key: "done", Trigger: encounter.TriggerReachedPosition{Room: "room1", Position: spatial.Position{X: 0, Y: 0}}},
			},
		}

		enc1, _ := encounter.NewEncounter(setup)
		data1 := enc1.ToData()
		tick1 := data1.Clock.HighWater

		enc2, _ := encounter.LoadEncounter(data1, map[encounter.MemberID]encounter.Decider{})
		data2 := enc2.ToData()
		tick2 := data2.Clock.HighWater

		// Clock must NOT be reset
		s.Equal(tick1, tick2, "clock must not reset on load")
		s.Equal(int(0), tick2, "initial tick reading should be 0")
	})
}
