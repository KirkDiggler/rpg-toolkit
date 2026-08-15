// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package encounter_test

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"math"
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
//
// #929 T1 follow-up: rewritten all-hex (both crypt and hall) — hex is the
// game's shipped family, so the rich golden should exercise it, not
// square+hex mixed (W1 forbids that field shape outright now — see
// TestSetupAnchoring's W1 row). The square case's own exact-byte pin lives
// in TestGoldenJSONOpen/TestGoldenJSONClosed ("grid" key absent for the
// zero value) — unaffected by this rewrite. Every position below is
// re-derived for crypt's (Width=8,Height=8 => Q,R valid [-4,4)) and hall's
// (Width=6,Height=6 => Q,R valid [-3,3)) origin-centered spans: crypt's
// occluder, boundary (still an axial-neighbor pair, ΔR=1), member p1, and
// the "guarded" ending's position are distinct in-bounds cells; door1's
// endpoints are each room's own boundary cell (an interior cell can never
// kiss anything) — FromPosition (3,0) is crypt's max-Q edge, ToPosition
// (-3,0) is hall's min-Q edge, still NEGATIVE-axial on purpose (the
// ordinary case for an origin-centered hex room).
//
// #929 T2: both Origins shifted by (-10,0) from the T1 follow-up's
// (crypt (0,0), hall (7,0)) to (crypt (-10,0), hall (-3,0)) — a uniform
// translation of the whole field preserves every W2/W3 relationship
// (absolute (4,0)/(-6,0) are still cube-distance 1; the rooms' absolute
// Q spans [-14,-7] and [-6,-1] still share no Q value) while making BOTH
// origins negative-axial, so the rich golden's bytes are the wire-shape
// proof for negative anchors this wave's persistence work needs (a
// declared origin round-trips as an explicit, signed value, not just a
// non-negative one).
//
// #929 T2 second review round: re-translated AGAIN, by (0,+7) on top of
// the above, to (crypt (-10,7), hall (-3,7)) — transposition hardening.
// Both prior shifts kept Y=0 for both rooms, so an X/Y transposition bug
// anywhere in the ToData/marshal path (an M7/M8-class mutant) had a real
// chance of surviving undetected here: with Y=0, a transposed (-10,0)
// would marshal as (0,-10) — a DIFFERENT byte string, so that specific
// case was already caught — but any bug that transposed X/Y AFTER first
// somehow losing or ignoring one axis could still slip through a
// fixture where one axis is always zero. With BOTH X and Y now nonzero
// and DISTINCT for both rooms (crypt: -10≠7; hall: -3≠7), no such gap
// remains: any X/Y transposition changes the bytes unambiguously. This
// shift is STILL a uniform translation of the whole field (adding (0,7)
// to both rooms alike), so every W2/W3 relationship from the comment
// above is preserved exactly — translation invariance holds regardless
// of the shift vector's own shape.
func (s *DataTestSuite) TestGoldenJSONRich() {
	enc, err := encounter.NewEncounter(&encounter.SetupInput{
		Initiative: orderAsGiven{},
		Field: encounter.FieldInput{
			Rooms: []encounter.RoomInput{
				{ID: "crypt", Width: 8, Height: 8, Grid: spatial.GridShapeHex, Origin: spatial.Position{X: -10, Y: 7},
					Occluders:  []spatial.Position{{X: 1, Y: 2}},
					Boundaries: []spatial.Boundary{{From: spatial.Position{X: -2, Y: -2}, To: spatial.Position{X: -2, Y: -1}, BlocksMovement: true, BlocksLineOfSight: true}},
				},
				{ID: "hall", Width: 6, Height: 6, Grid: spatial.GridShapeHex, Origin: spatial.Position{X: -3, Y: 7}},
			},
			Connections: []encounter.ConnectionInput{{
				ID: "door1", From: "crypt", To: "hall",
				FromPosition: spatial.Position{X: 3, Y: 0},
				ToPosition:   spatial.Position{X: -3, Y: 0},
			}},
		},
		Members: []encounter.MemberInput{
			{ID: "p1", Kind: encounter.KindPlayer, Room: "crypt", Position: spatial.Position{X: 0, Y: 0}},
			{ID: "g1", Kind: encounter.KindMonster, Room: "hall", Position: spatial.Position{X: 0, Y: 0}, Decider: &testDecider{intent: encounter.IntentHold{}}},
		},
		Endings: []encounter.EndingInput{
			{Key: "guarded", Trigger: encounter.TriggerReachedPosition{Room: "crypt", Position: spatial.Position{X: 3, Y: 3}, Member: core.EntityID("p1")}},
			{Key: "leave", Trigger: encounter.TriggerExternal{}},
		},
	})
	s.Require().NoError(err)
	_, err = enc.Pump(&encounter.PumpInput{})
	s.Require().NoError(err)

	bs, err := json.Marshal(enc.ToData())
	s.Require().NoError(err)
	// Exact-string pin: every room now carries "origin" (#929 T2), always
	// present (no omitempty — RoomData's doc comment) — crypt's and hall's
	// are both negative-axial, the wire-shape proof this golden exists for.
	expected := `{"clock":{"budgets":{"g1":1,"p1":1},"driver_progress":{"world":1},"high_water":1},"intel":{},"log":{"next_seq":3,"entries":[{"seq":1,"audience":["p1","g1"],"tags":{"tag":"scene"},"payload":"eyJiZWF0Ijoic2NlbmUtb3BlbmVkIn0="},{"seq":2,"at":1,"audience":["g1","p1"],"tags":{"tag":"clock"},"payload":"eyJiZWF0IjoidGljayIsInRpY2siOjF9"}]},"field":{"rooms":[{"id":"crypt","width":8,"height":8,"grid":"hex","occluders":[{"x":1,"y":2}],"boundaries":[{"from":{"x":-2,"y":-2},"to":{"x":-2,"y":-1},"blocks_movement":true,"blocks_line_of_sight":true}],"origin":{"x":-10,"y":7}},{"id":"hall","width":6,"height":6,"grid":"hex","origin":{"x":-3,"y":7}}],"connections":[{"id":"door1","from":"crypt","to":"hall","from_position":{"x":3,"y":0},"to_position":{"x":-3,"y":0}}]},"members":[{"id":"g1","kind":"monster","room":"hall","position":{"x":0,"y":0}},{"id":"p1","kind":"player","room":"crypt","position":{"x":0,"y":0}}],"endings":[{"key":"guarded","kind":"reached_position","room":"crypt","position":{"x":3,"y":3},"member":"p1"},{"key":"leave","kind":"external"}],"ever_members":["g1","p1"],"retention":32}`
	s.Equal(expected, string(bs))
}

// TestEndingsOrderSurvivesReload pins C8 first-declared-wins across the
// persistence boundary: two endings match the same position; the FIRST
// declared fires, before and after a reload. A load that scrambles
// ending order changes which outcome the campaign receives.
func (s *DataTestSuite) TestEndingsOrderSurvivesReload() {
	setup := &encounter.SetupInput{
		Initiative: orderAsGiven{},
		Field:      encounter.FieldInput{Rooms: []encounter.RoomInput{{ID: "r1", Width: 5, Height: 5}}},
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
	enc2, err := encounter.LoadEncounter(&encounter.LoadEncounterInput{
		Initiative: orderAsGiven{}, Data: enc1.ToData()})
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
//
// #929 T1: r2's Origin (5,0) anchors it immediately east of r1 (5x5 each,
// so r1's absolute footprint is x:[0,4],y:[0,4] and r2's is x:[5,9],y:[0,4]
// — disjoint, W2). All three doors sit on r1's east edge (x=4) and r2's
// west edge (local x=0, absolute x=5), one per row, so every one
// independently satisfies W3 (Chebyshev distance 1, dx=1/dy=0) under this
// SAME shared origin — the original fixture's three endpoint pairs had
// three different relative offsets and could not all kiss under any single
// origin simultaneously.
func (s *DataTestSuite) TestConnectionsSurviveReload() {
	setup := &encounter.SetupInput{
		Initiative: orderAsGiven{},
		Field: encounter.FieldInput{
			Rooms: []encounter.RoomInput{
				{ID: "r1", Width: 5, Height: 5},
				{ID: "r2", Width: 5, Height: 5, Origin: spatial.Position{X: 5, Y: 0}},
			},
			// Declared out of ID order — persistence must not echo this order.
			Connections: []encounter.ConnectionInput{
				{ID: "z-door", From: "r1", To: "r2", FromPosition: spatial.Position{X: 4, Y: 1}, ToPosition: spatial.Position{X: 0, Y: 1}},
				{ID: "a-door", From: "r1", To: "r2", FromPosition: spatial.Position{X: 4, Y: 2}, ToPosition: spatial.Position{X: 0, Y: 2}},
				{ID: "m-door", From: "r1", To: "r2", FromPosition: spatial.Position{X: 4, Y: 3}, ToPosition: spatial.Position{X: 0, Y: 3}},
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
		{ID: "a-door", From: "r1", To: "r2", FromPosition: &encounter.PositionData{X: 4, Y: 2}, ToPosition: &encounter.PositionData{X: 0, Y: 2}},
		{ID: "m-door", From: "r1", To: "r2", FromPosition: &encounter.PositionData{X: 4, Y: 3}, ToPosition: &encounter.PositionData{X: 0, Y: 3}},
		{ID: "z-door", From: "r1", To: "r2", FromPosition: &encounter.PositionData{X: 4, Y: 1}, ToPosition: &encounter.PositionData{X: 0, Y: 1}},
	}
	s.Equal(expected, data1.Field.Connections, "connections persist sorted by ID with endpoints intact")

	enc2, err := encounter.LoadEncounter(&encounter.LoadEncounterInput{
		Initiative: orderAsGiven{}, Data: data1})
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
// #929 T2: r2's Origin (5,0) anchors it immediately east of r1 (5x5 each),
// matching TestConnectionsSurviveReload's geometry exactly — all three
// doors sit on r1's east edge (x=4) and r2's west edge (local x=0), one
// per row, so every one independently satisfies W3 under this SAME shared
// origin (this test only cares about sort order, not which door owns
// which row).
func (s *DataTestSuite) TestLoadSortsUnsortedConnections() {
	data := encounter.EncounterData{
		Field: encounter.FieldData{
			Rooms: []encounter.RoomData{
				{ID: "r1", Width: 5, Height: 5, Origin: &encounter.PositionData{X: 0, Y: 0}},
				{ID: "r2", Width: 5, Height: 5, Origin: &encounter.PositionData{X: 5, Y: 0}},
			},
			// Hand-authored out of ID order — NOT produced by ToData.
			Connections: []encounter.ConnectionData{
				{ID: "z-door", From: "r1", To: "r2", FromPosition: &encounter.PositionData{X: 4, Y: 1}, ToPosition: &encounter.PositionData{X: 0, Y: 1}},
				{ID: "a-door", From: "r1", To: "r2", FromPosition: &encounter.PositionData{X: 4, Y: 2}, ToPosition: &encounter.PositionData{X: 0, Y: 2}},
				{ID: "m-door", From: "r1", To: "r2", FromPosition: &encounter.PositionData{X: 4, Y: 3}, ToPosition: &encounter.PositionData{X: 0, Y: 3}},
			},
		},
		Members: []encounter.MemberData{
			{ID: "p1", Kind: encounter.KindPlayer, Room: "r1", Position: encounter.PositionData{X: 1, Y: 1}},
		},
		Endings:     []encounter.EndingData{{Key: "done", Kind: "external"}},
		EverMembers: []encounter.MemberID{"p1"},
	}

	enc, err := encounter.LoadEncounter(&encounter.LoadEncounterInput{
		Initiative: orderAsGiven{}, Data: data})
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
// and a non-zero shape (hex).
//
// #929 T1 follow-up: originally both shapes lived in ONE encounter (two
// rooms, one square, one hex). W1 ("one geometry per field" — see
// TestSetupAnchoring's W1 row) now makes that field itself a defect, not
// just an inconvenience — a mixed-family field has no coherent absolute
// space to validate at all, so this is correctly impossible, not a
// regression to work around. Split into two single-room encounters, one
// per family; each still proves its own shape survives a full
// ToData -> LoadEncounter -> ToData round trip.
func (s *DataTestSuite) TestRoomGridShapeSurvivesReload() {
	s.Run("square", func() {
		setup := &encounter.SetupInput{
			Initiative: orderAsGiven{},
			Field: encounter.FieldInput{
				Rooms: []encounter.RoomInput{
					{ID: "square-room", Width: 5, Height: 5},
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
		s.Require().Len(data1.Field.Rooms, 1)
		s.Equal("", data1.Field.Rooms[0].Grid, "square is the zero value — omitted, not the literal \"square\"")

		enc2, err := encounter.LoadEncounter(&encounter.LoadEncounterInput{
			Initiative: orderAsGiven{}, Data: data1})
		s.Require().NoError(err)

		data2 := enc2.ToData()
		s.Equal("", data2.Field.Rooms[0].Grid, "grid shape must survive a second round-trip")
	})

	s.Run("hex", func() {
		setup := &encounter.SetupInput{
			Initiative: orderAsGiven{},
			Field: encounter.FieldInput{
				Rooms: []encounter.RoomInput{
					{ID: "hex-room", Width: 5, Height: 5, Grid: spatial.GridShapeHex},
				},
			},
			Members: []encounter.MemberInput{
				{ID: "p1", Kind: encounter.KindPlayer, Room: "hex-room", Position: spatial.Position{X: 1, Y: 1}},
			},
			Endings: []encounter.EndingInput{{Key: "done", Trigger: encounter.TriggerExternal{}}},
		}

		enc1, err := encounter.NewEncounter(setup)
		s.Require().NoError(err)

		data1 := enc1.ToData()
		s.Require().Len(data1.Field.Rooms, 1)
		s.Equal(spatial.GridTypeHex, data1.Field.Rooms[0].Grid)

		enc2, err := encounter.LoadEncounter(&encounter.LoadEncounterInput{
			Initiative: orderAsGiven{}, Data: data1})
		s.Require().NoError(err)

		data2 := enc2.ToData()
		s.Equal(spatial.GridTypeHex, data2.Field.Rooms[0].Grid, "grid shape must survive a second round-trip")
	})
}

// TestSetupInputNotAliased pins T6 review M4: a caller that edits its
// own SetupInput after construction must not corrupt the persistence
// source (the encounter deep-copies the field description). Also covers
// connections: mutating the caller's ConnectionInput slice (and its
// endpoint positions) after NewEncounter must not affect the encounter.
//
// #929 T1: door1's FromPosition moved to r1's top-right corner (4,0) — an
// interior cell like the original (1,0)'s neighbor set never fully escapes
// r1's own footprint diagonally, but a corner's does. r2's Origin (5,-1)
// anchors it diagonally past that corner: absolute FromPosition (4,0) and
// absolute ToPosition local(0,0)+(5,-1)=(5,-1) are Chebyshev-adjacent
// (distance 1, a diagonal kiss), while r1's absolute footprint
// (x:[0,4],y:[0,4]) and r2's (x:[5,9],y:[-1,3]) share no x value at all,
// so they stay disjoint (W2) regardless of y.
func (s *DataTestSuite) TestSetupInputNotAliased() {
	setup := &encounter.SetupInput{
		Initiative: orderAsGiven{},
		Field: encounter.FieldInput{
			Rooms: []encounter.RoomInput{
				{ID: "r1", Width: 5, Height: 5, Occluders: []spatial.Position{{X: 3, Y: 3}}},
				{ID: "r2", Width: 5, Height: 5, Origin: spatial.Position{X: 5, Y: -1}},
			},
			Connections: []encounter.ConnectionInput{
				{ID: "door1", From: "r1", To: "r2", FromPosition: spatial.Position{X: 4, Y: 0}, ToPosition: spatial.Position{X: 0, Y: 0}},
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
	s.Equal(&encounter.PositionData{X: 4, Y: 0}, data.Field.Connections[0].FromPosition)
	s.Equal(&encounter.PositionData{X: 0, Y: 0}, data.Field.Connections[0].ToPosition)

	// And the corrupted-input snapshot must still LOAD (the M4 symptom
	// was an encounter that became permanently unsavable).
	_, err = encounter.LoadEncounter(&encounter.LoadEncounterInput{
		Initiative: orderAsGiven{}, Data: data})
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
			Initiative: orderAsGiven{},
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
		enc2, err := encounter.LoadEncounter(&encounter.LoadEncounterInput{
			Initiative: orderAsGiven{}, Data: data1, Deciders: map[encounter.MemberID]encounter.Decider{}})
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
			Initiative: orderAsGiven{},
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

		// The two saw each other, which starts a fight (rpg-toolkit#964), and
		// a fight member cannot free-roam — so they break off before the
		// goblin steps behind the pillar and becomes the ghost this test is
		// about.
		_, err = enc1.Dissolve(&encounter.DissolveInput{Member: "goblin"})
		s.Require().NoError(err)

		// Move goblin to create ghost at last-seen position
		_, err = enc1.Move(&encounter.MoveInput{
			Member: "goblin",
			To:     spatial.Position{X: 5, Y: 6}, // Behind pillar from A's view
		})
		s.Require().NoError(err)

		data1 := enc1.ToData()

		// Load and verify ghost is still there
		enc2, err := encounter.LoadEncounter(&encounter.LoadEncounterInput{
			Initiative: orderAsGiven{}, Data: data1, Deciders: map[encounter.MemberID]encounter.Decider{}})
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
			Initiative: orderAsGiven{},
			Field: encounter.FieldInput{
				Rooms: []encounter.RoomInput{
					{
						ID:     "crypt",
						Width:  10,
						Height: 10,
						// A pillar on the diagonal keeps playerA and the
						// goblin out of each other's sight, so the scene
						// opens in free roam and the goblin stays the
						// world's to pump. Co-located and visible would be
						// a fight at first light (rpg-toolkit#964), and a
						// fight monster's decider is never consulted.
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
		enc2, err := encounter.LoadEncounter(&encounter.LoadEncounterInput{
			Initiative: orderAsGiven{}, Data: data1, Deciders: map[encounter.MemberID]encounter.Decider{}})
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
			Initiative: orderAsGiven{},
			Field: encounter.FieldInput{
				Rooms: []encounter.RoomInput{
					{
						ID:     "crypt",
						Width:  10,
						Height: 10,
						// A pillar on the diagonal keeps playerA and the
						// goblin out of each other's sight, so the scene
						// opens in free roam and the goblin stays the
						// world's to pump. Co-located and visible would be
						// a fight at first light (rpg-toolkit#964), and a
						// fight monster's decider is never consulted.
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
		enc2, err := encounter.LoadEncounter(&encounter.LoadEncounterInput{
			Initiative: orderAsGiven{}, Data: data1, Deciders: map[encounter.MemberID]encounter.Decider{}})
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
			Initiative: orderAsGiven{},
			Field: encounter.FieldInput{
				Rooms: []encounter.RoomInput{
					{
						ID:     "crypt",
						Width:  10,
						Height: 10,
						// A pillar on the diagonal keeps playerA and the
						// goblin out of each other's sight, so the scene
						// opens in free roam and the goblin stays the
						// world's to pump. Co-located and visible would be
						// a fight at first light (rpg-toolkit#964), and a
						// fight monster's decider is never consulted.
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

		enc2, err := encounter.LoadEncounter(&encounter.LoadEncounterInput{
			Initiative: orderAsGiven{}, Data: data1})
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
			Initiative: orderAsGiven{},
			Field: encounter.FieldInput{
				Rooms: []encounter.RoomInput{
					{
						ID:     "crypt",
						Width:  10,
						Height: 10,
						// A pillar on the diagonal keeps playerA and the
						// goblin out of each other's sight, so the scene
						// opens in free roam and the goblin stays the
						// world's to pump. Co-located and visible would be
						// a fight at first light (rpg-toolkit#964), and a
						// fight monster's decider is never consulted.
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
		enc2, err := encounter.LoadEncounter(&encounter.LoadEncounterInput{
			Initiative: orderAsGiven{}, Data: data1, Deciders: map[encounter.MemberID]encounter.Decider{}})
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
			Initiative: orderAsGiven{},
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
		expectedJSON := `{"clock":{"budgets":{"p1":0}},"intel":{},"log":{"next_seq":2,"entries":[{"seq":1,"audience":["p1"],"tags":{"tag":"scene"},"payload":"eyJiZWF0Ijoic2NlbmUtb3BlbmVkIn0="}]},"field":{"rooms":[{"id":"room1","width":5,"height":5,"origin":{"x":0,"y":0}}]},"members":[{"id":"p1","kind":"player","room":"room1","position":{"x":2,"y":2}}],"endings":[{"key":"done","kind":"reached_position","room":"room1","position":{"x":0,"y":0}}],"ever_members":["p1"],"retention":32}`
		s.Equal(expectedJSON, string(jsonBytes))
	})
}

// TestGoldenJSONClosed pins the JSON representation of a closed encounter.
func (s *DataTestSuite) TestGoldenJSONClosed() {
	s.Run("small closed encounter golden JSON", func() {
		setup := &encounter.SetupInput{
			Initiative: orderAsGiven{},
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

		// Advance the clock BEFORE closing (#929 T2 second review round —
		// golden law: every omitempty field must be exercised at least
		// once; OutcomeData.At omits at tick 0, so a golden that closes
		// immediately can never prove `at` actually persists a non-zero
		// value). One Pump advances the world tick to 1, adding its own
		// "clock" beat to the log ahead of the closing move.
		_, err = enc.Pump(&encounter.PumpInput{})
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
		// fired ending (AT A NON-ZERO TICK — "at":1, the field this golden
		// exists to exercise) and final member placements; the story
		// carries all three beats (opening + the pump's tick + the
		// closing move).
		expectedJSON := `{"outcome":{"ending":"done","at":1,"members":[{"id":"p1","room":"room1","position":{"x":0,"y":0}}]},"clock":{"budgets":{"p1":1},"driver_progress":{"world":1},"high_water":1},"intel":{},"log":{"next_seq":4,"entries":[{"seq":1,"audience":["p1"],"tags":{"tag":"scene"},"payload":"eyJiZWF0Ijoic2NlbmUtb3BlbmVkIn0="},{"seq":2,"at":1,"audience":["p1"],"tags":{"tag":"clock"},"payload":"eyJiZWF0IjoidGljayIsInRpY2siOjF9"},{"seq":3,"at":1,"audience":["p1"],"tags":{"tag":"movement"},"payload":"eyJiZWF0IjoibW92ZWQiLCJtZW1iZXIiOiJwMSIsInBvc2l0aW9uIjp7IngiOjAsInkiOjB9fQ=="}]},"field":{"rooms":[{"id":"room1","width":5,"height":5,"origin":{"x":0,"y":0}}]},"members":[{"id":"p1","kind":"player","room":"room1","position":{"x":0,"y":0}}],"endings":[{"key":"done","kind":"reached_position","room":"room1","position":{"x":0,"y":0}}],"ever_members":["p1"],"retention":32}`
		s.Equal(expectedJSON, string(jsonBytes))
	})
}

// TestAliasImmunity verifies mutating ToData result doesn't affect aggregate.
func (s *DataTestSuite) TestAliasImmunityToData() {
	s.Run("mutating ToData result doesn't affect aggregate", func() {
		setup := &encounter.SetupInput{
			Initiative: orderAsGiven{},
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
		// #929 T2: Origin is a pointer (RoomData's doc comment — presence
		// itself is meaningful, so it can't be a value type) — mutating
		// THROUGH it must not reach a later call's own fresh pointer.
		s.Require().NotEmpty(data1.Field.Rooms)
		s.Require().NotNil(data1.Field.Rooms[0].Origin)
		data1.Field.Rooms[0].Origin.X = 999

		// Get data again
		data2 := enc.ToData()

		// Should not be affected by mutation
		s.NotEqual("mutated", data2.Members[0].ID)
		s.NotEqual("mutated", data2.EverMembers[0])
		s.NotEqual("mutated", data2.Endings[0].Key)
		s.Require().NotNil(data2.Field.Rooms[0].Origin)
		s.NotEqual(999.0, data2.Field.Rooms[0].Origin.X,
			"ToData must return a FRESH Origin pointer each call, not alias the same PositionData across calls")
	})
}

// TestAliasImmunityLoadEncounter verifies mutating caller's Data doesn't affect loaded aggregate.
func (s *DataTestSuite) TestAliasImmunityLoadEncounter() {
	s.Run("mutating caller's Data after LoadEncounter doesn't affect loaded aggregate", func() {
		setup := &encounter.SetupInput{
			Initiative: orderAsGiven{},
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
		enc2, err := encounter.LoadEncounter(&encounter.LoadEncounterInput{
			Initiative: orderAsGiven{}, Data: data})
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
			Initiative: orderAsGiven{},
			Field:      encounter.FieldInput{Rooms: []encounter.RoomInput{{ID: "crypt", Width: 10, Height: 10}}},
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

		enc2, err := encounter.LoadEncounter(&encounter.LoadEncounterInput{
			Initiative: orderAsGiven{}, Data: data})
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
			Initiative: orderAsGiven{},
			Field: encounter.FieldInput{
				Rooms: []encounter.RoomInput{
					{
						ID:     "crypt",
						Width:  10,
						Height: 10,
						// A pillar on the diagonal keeps playerA and the
						// goblin out of each other's sight, so the scene
						// opens in free roam and the goblin stays the
						// world's to pump. Co-located and visible would be
						// a fight at first light (rpg-toolkit#964), and a
						// fight monster's decider is never consulted.
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
		enc2, err := encounter.LoadEncounter(&encounter.LoadEncounterInput{
			Initiative: orderAsGiven{}, Data: data1, Deciders: map[encounter.MemberID]encounter.Decider{
				encounter.MemberID("goblin"): decider,
			}})
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
			Initiative: orderAsGiven{},
			Field: encounter.FieldInput{
				Rooms: []encounter.RoomInput{
					{
						ID:     "crypt",
						Width:  10,
						Height: 10,
						// A pillar on the diagonal keeps playerA and the
						// goblin out of each other's sight, so the scene
						// opens in free roam and the goblin stays the
						// world's to pump. Co-located and visible would be
						// a fight at first light (rpg-toolkit#964), and a
						// fight monster's decider is never consulted.
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
		enc2, err := encounter.LoadEncounter(&encounter.LoadEncounterInput{
			Initiative: orderAsGiven{}, Data: data1, Deciders: map[encounter.MemberID]encounter.Decider{}})
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
		Initiative: orderAsGiven{},
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

	enc2, err := encounter.LoadEncounter(&encounter.LoadEncounterInput{
		Initiative: orderAsGiven{}, Data: data1, Deciders: map[encounter.MemberID]encounter.Decider{
			"goblin": nil,
		}})
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
		Initiative: orderAsGiven{},
		Field: encounter.FieldInput{
			Rooms: []encounter.RoomInput{
				{ID: "crypt", Width: 10, Height: 10},
				// playerA watches from the antechamber. Two monsters sharing
				// the crypt see only each other, which starts nothing —
				// classification pairs players against monsters — so both
				// stay the world's to pump (rpg-toolkit#964).
				{ID: "antechamber", Width: 10, Height: 10, Origin: spatial.Position{X: 10, Y: 0}},
			},
		},
		Members: []encounter.MemberInput{
			{ID: "playerA", Kind: encounter.KindPlayer, Room: "antechamber", Position: spatial.Position{X: 1, Y: 1}},
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
	enc2, err := encounter.LoadEncounter(&encounter.LoadEncounterInput{
		Initiative: orderAsGiven{}, Data: data1, Deciders: map[encounter.MemberID]encounter.Decider{
			"goblin": nil,
			"rat":    ratDecider,
		}})
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
		Field: encounter.FieldData{Rooms: []encounter.RoomData{
			{ID: "r1", Width: 5, Height: 5, Origin: &encounter.PositionData{X: 0, Y: 0}},
		}},
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
// this otherwise-valid connection). FromPosition{9,1} is valid ONLY in r1
// and ToPosition{0,7} valid ONLY in r2: same-sized rooms and equal From/To
// positions would make a check that validates an endpoint against the WRONG
// room (or a From/To transposition in convertConnectionDataToConnectionInput)
// invisible. r1 carries an occluder at (2,2) and r2 an occluder at (1,3), so
// both the from- and to-side "endpoint on occluder" rows have something to hit.
//
// #929 T2: mirrors encounter_test.go's validConnSetup exactly (its own
// comment explains the geometry in full) — FromPosition sits on r1's own
// right-edge boundary (an interior cell can never kiss anything), r2's
// Origin (10,-6) anchors it so absolute FromPosition (9,1) and absolute
// ToPosition (10,7... local(0,7)+Origin(10,-6)=(10,1)) are Chebyshev-
// adjacent (W3), while r1's absolute footprint (x:[0,9],y:[0,3]) and r2's
// (x:[10,12],y:[-6,2]) share no x value at all, so they stay disjoint (W2).
func validEncounterDataWithConnection() encounter.EncounterData {
	d := validEncounterData()
	d.Field.Rooms[0].Width = 10
	d.Field.Rooms[0].Height = 4
	d.Field.Rooms[0].Occluders = []encounter.PositionData{{X: 2, Y: 2}}
	d.Field.Rooms = append(d.Field.Rooms, encounter.RoomData{
		ID: "r2", Width: 3, Height: 9, Origin: &encounter.PositionData{X: 10, Y: -6},
		Occluders: []encounter.PositionData{{X: 1, Y: 3}},
	})
	d.Field.Connections = []encounter.ConnectionData{
		{ID: "c1", From: "r1", To: "r2",
			FromPosition: &encounter.PositionData{X: 9, Y: 1},
			ToPosition:   &encounter.PositionData{X: 0, Y: 7}},
	}
	return d
}

// TestLoadSquareOccluderFractionalRejected is the Load-seam counterpart to
// encounter_test.go's TestSetupSquareOccluderFractionalRejected (#929 T3
// Opus round F2).
func (s *DataTestSuite) TestLoadSquareOccluderFractionalRejected() {
	data := validEncounterDataWithConnection()
	data.Field.Rooms[0].Occluders[0] = encounter.PositionData{X: 2.5, Y: 2}
	_, err := encounter.LoadEncounter(&encounter.LoadEncounterInput{
		Initiative: orderAsGiven{}, Data: data})
	s.Require().Error(err)
	s.Require().ErrorIs(err, encounter.ErrInvalidData)
	s.Require().ErrorIs(err, encounter.ErrNoField)
	s.Require().Contains(err.Error(), "not a representable integral cell")
}

// TestLoadNilInputRejected pins the guard the Input signature introduced (#976).
// A nil input is the one failure the two-parameter form could not have: it must
// reject rather than panicking on a nil dereference.
//
// The pin is the *shape* of the answer, and specifically that it is ErrNilInput
// rather than ErrInvalidData. Those are different categories — ErrNilInput
// "indicates a caller defect" per its own doc, while ErrInvalidData means the
// persisted blob does not describe a valid encounter — and a nil input supplied
// no blob to be invalid. Every other *XxxInput seam here answers ErrNilInput,
// NewEncounter included; Load answering differently would make
// errors.Is(err, ErrNilInput) unreliable for the one caller defect it exists to
// name. ErrInvalidData is asserted absent so the two stay distinguishable.
//
// (#929 hardening round F fixed this same species of conflation, where Load's
// member checks carried only ErrInvalidData instead of ErrNoMember. See
// ErrNoMember's doc comment.)
func (s *DataTestSuite) TestLoadNilInputRejected() {
	s.Require().NotPanics(func() {
		enc, err := encounter.LoadEncounter(nil)
		s.Require().Error(err)
		s.Require().ErrorIs(err, encounter.ErrNilInput)
		s.Require().NotErrorIs(err, encounter.ErrInvalidData)
		s.Require().Nil(enc)
	})
}

// TestLoadNilDecidersIsLegal pins that omitting Deciders entirely is a supported
// call rather than an oversight — an encounter whose members all act by explicit
// verb has nothing to re-attach. This is the overwhelmingly common form at the
// call sites, so it is stated once rather than left implied by them.
func (s *DataTestSuite) TestLoadNilDecidersIsLegal() {
	enc, err := encounter.LoadEncounter(&encounter.LoadEncounterInput{
		Initiative: orderAsGiven{},
		Data:       validEncounterData(),
	})
	s.Require().NoError(err)
	s.Require().NotNil(enc)
}

// TestLoadOccluderOnBoundaryCellAccepted is the Load-seam counterpart to
// encounter_test.go's TestSetupOccluderOnBoundaryCellAccepted (#929 T3
// trailing round N2).
func (s *DataTestSuite) TestLoadOccluderOnBoundaryCellAccepted() {
	data := encounter.EncounterData{
		Field: encounter.FieldData{
			Rooms: []encounter.RoomData{
				{ID: "hall", Width: 5, Height: 5, Origin: &encounter.PositionData{X: 0, Y: 0}, Occluders: []encounter.PositionData{
					{X: 0, Y: 2}, {X: 4, Y: 2}, {X: 2, Y: 0}, {X: 2, Y: 4}, {X: 0, Y: 0}, {X: 4, Y: 4},
				}},
			},
		},
		Endings: []encounter.EndingData{{Key: "done", Kind: "external"}},
	}
	_, err := encounter.LoadEncounter(&encounter.LoadEncounterInput{
		Initiative: orderAsGiven{}, Data: data})
	s.Require().NoError(err, "an occluder on a room's boundary cell, including a corner, must be legal at Load too")
}

// TestLoadOccluderIDCrossRoomCollisionAccepted is the Load-seam counterpart
// to encounter_test.go's TestSetupOccluderIDCrossRoomCollisionAccepted
// (#929 hardening round C).
func (s *DataTestSuite) TestLoadOccluderIDCrossRoomCollisionAccepted() {
	data := encounter.EncounterData{
		Field: encounter.FieldData{
			Rooms: []encounter.RoomData{
				{ID: "r", Width: 12, Height: 10, Grid: spatial.GridTypeHex,
					Origin: &encounter.PositionData{X: 0, Y: 0}, Occluders: []encounter.PositionData{{X: -5, Y: 4}}},
				{ID: "r-", Width: 12, Height: 10, Grid: spatial.GridTypeHex,
					Origin: &encounter.PositionData{X: 1000, Y: 0}, Occluders: []encounter.PositionData{{X: 5, Y: 4}}},
			},
		},
		Endings: []encounter.EndingData{{Key: "done", Kind: "external"}},
	}
	_, err := encounter.LoadEncounter(&encounter.LoadEncounterInput{
		Initiative: orderAsGiven{}, Data: data})
	s.Require().NoError(err, `room "r" occluder (-5,4) and room "r-" occluder (5,4) must not collide at Load either`)
}

// TestLoadDuplicateOccluderRejected is the Load-seam counterpart to
// encounter_test.go's TestSetupDuplicateOccluderRejected (#929 hardening
// round D).
func (s *DataTestSuite) TestLoadDuplicateOccluderRejected() {
	data := encounter.EncounterData{
		Field: encounter.FieldData{
			Rooms: []encounter.RoomData{
				{ID: "hall", Width: 5, Height: 5, Origin: &encounter.PositionData{X: 0, Y: 0},
					Occluders: []encounter.PositionData{{X: 3, Y: 3}, {X: 3, Y: 3}}},
			},
		},
		Endings: []encounter.EndingData{{Key: "done", Kind: "external"}},
	}
	_, err := encounter.LoadEncounter(&encounter.LoadEncounterInput{
		Initiative: orderAsGiven{}, Data: data})
	s.Require().Error(err)
	s.Require().ErrorIs(err, encounter.ErrInvalidData)
	s.Require().ErrorIs(err, encounter.ErrNoField)
	s.Require().Contains(err.Error(), "duplicate occluder")
}

// TestLoadDuplicateEndingKeyRejected is the Load-seam counterpart to
// encounter_test.go's TestSetupDuplicateEndingKeyRejected (#929
// hardening round E).
func (s *DataTestSuite) TestLoadDuplicateEndingKeyRejected() {
	data := encounter.EncounterData{
		Field: encounter.FieldData{
			Rooms: []encounter.RoomData{{ID: "hall", Width: 5, Height: 5, Origin: &encounter.PositionData{X: 0, Y: 0}}},
		},
		Endings: []encounter.EndingData{
			{Key: "dup", Kind: "reached_position", Room: "hall", Position: &encounter.PositionData{X: 4, Y: 4}},
			{Key: "dup", Kind: "external"},
		},
	}
	_, err := encounter.LoadEncounter(&encounter.LoadEncounterInput{
		Initiative: orderAsGiven{}, Data: data})
	s.Require().Error(err)
	s.Require().ErrorIs(err, encounter.ErrInvalidData)
	s.Require().ErrorIs(err, encounter.ErrNoEnding)
	s.Require().Contains(err.Error(), "duplicate ending")
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
		{"unknown ending kind", func(d *encounter.EncounterData) { d.Endings[0].Kind = "psychic" }, "unknown ending kind", encounter.ErrNoEnding},
		{"reached_position without position", func(d *encounter.EncounterData) {
			d.Endings[0] = encounter.EndingData{Key: "done", Kind: "reached_position", Room: "r1"}
		}, "without room/position", encounter.ErrNoEnding},
		{"empty member id", func(d *encounter.EncounterData) { d.Members[0].ID = "" }, "empty member id", encounter.ErrNoMember},
		{"duplicate member ids", func(d *encounter.EncounterData) {
			d.Members = append(d.Members, d.Members[0])
		}, "duplicate member", encounter.ErrNoMember},
		{"member room not in field", func(d *encounter.EncounterData) { d.Members[0].Room = "nowhere" }, "not in field", encounter.ErrBadPlacement},
		{"member out of bounds", func(d *encounter.EncounterData) {
			d.Members[0].Position = encounter.PositionData{X: 99, Y: 99}
		}, "out of bounds", encounter.ErrBadPlacement},
		{"connection missing room", func(d *encounter.EncounterData) {
			// FromPosition/ToPosition present (any value) so the ONLY
			// defect this fixture carries is the unknown "to" room — #929
			// T2: endpoint presence is now checked during conversion,
			// before room-existence, so a nil endpoint here would mask
			// this row's own target defect behind "missing from_position".
			d.Field.Connections = []encounter.ConnectionData{{ID: "c1", From: "r1", To: "nowhere",
				FromPosition: &encounter.PositionData{X: 1, Y: 1}, ToPosition: &encounter.PositionData{X: 1, Y: 1}}}
		}, `unknown room "nowhere"`, encounter.ErrBadConnection},
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
		}, `unknown room "nowhere"`, encounter.ErrBadConnection},
		{"connection unknown to room", func(d *encounter.EncounterData) {
			*d = validEncounterDataWithConnection()
			d.Field.Connections[0].To = "nowhere"
		}, `unknown room "nowhere"`, encounter.ErrBadConnection},
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
		}, "outcome", encounter.ErrNoEnding},
		{"abandoned outcome with members present", func(d *encounter.EncounterData) {
			d.Outcome = &encounter.OutcomeData{Ending: "abandoned"}
		}, "abandoned outcome with members", encounter.ErrNoMember},
		{"outcome member room missing", func(d *encounter.EncounterData) {
			d.Outcome = &encounter.OutcomeData{Ending: "done", Members: []encounter.MemberOutcomeData{
				{ID: "ghost", Room: "nowhere", Position: encounter.PositionData{X: 1, Y: 1}}}}
		}, "outcome member", encounter.ErrBadPlacement},
		{"outcome member out of bounds", func(d *encounter.EncounterData) {
			d.Outcome = &encounter.OutcomeData{Ending: "done", Members: []encounter.MemberOutcomeData{
				{ID: "p1", Room: "r1", Position: encounter.PositionData{X: 999, Y: 999}}}}
		}, "out of bounds", encounter.ErrBadPlacement},
		{"ever_members missing current member", func(d *encounter.EncounterData) {
			d.EverMembers = nil
		}, "ever_members", encounter.ErrNoMember},
	}
	for _, tc := range cases {
		s.Run(tc.name, func() {
			data := validEncounterData()
			tc.mutate(&data)
			_, err := encounter.LoadEncounter(&encounter.LoadEncounterInput{
				Initiative: orderAsGiven{}, Data: data})
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
	_, err := encounter.LoadEncounter(&encounter.LoadEncounterInput{
		Initiative: orderAsGiven{}, Data: validEncounterData()})
	s.Require().NoError(err, "the valid base fixture must load")

	// The valid CONNECTION base must also load. Since FromPosition{9,1} is
	// valid ONLY in r1 and ToPosition{0,7} valid ONLY in r2, this positive
	// control pins that each endpoint is checked against ITS OWN room (a
	// check wired to the wrong room would reject this valid connection),
	// and that endpoints survive Load unswapped (a From/To transposition
	// in convertConnectionDataToConnectionInput would not error here —
	// it would silently swap the values — so the values are re-inspected,
	// not just the absence of an error).
	connEnc, err := encounter.LoadEncounter(&encounter.LoadEncounterInput{
		Initiative: orderAsGiven{}, Data: validEncounterDataWithConnection()})
	s.Require().NoError(err, "the valid connection base fixture must load")
	connData := connEnc.ToData()
	s.Require().Len(connData.Field.Connections, 1)
	s.Equal(&encounter.PositionData{X: 9, Y: 1}, connData.Field.Connections[0].FromPosition,
		"from-position must survive Load unswapped")
	s.Equal(&encounter.PositionData{X: 0, Y: 7}, connData.Field.Connections[0].ToPosition,
		"to-position must survive Load unswapped")
}

// connBoundsData returns a fresh EncounterData with a 4x3 room r1 (valid
// coordinates 0..3 x 0..2) and one connection — the Load-seam counterpart to
// encounter_test.go's connBoundsSetup, pinning the square grid's strictly-
// less-than bounds semantics at Load independent of any cross-room concern.
//
// #929 T2: r2's Origin (4,3) mirrors connBoundsSetup exactly — its own
// comment explains the diagonal-kiss/disjointness geometry (positive
// control FromPosition (3,2), r1's own bottom-right corner, kisses
// ToPosition (0,0)+Origin(4,3)=(4,3) at Chebyshev distance 1; the rooms'
// absolute footprints x:[0,3] vs x:[4,7] share no x value).
func connBoundsData() encounter.EncounterData {
	return encounter.EncounterData{
		Field: encounter.FieldData{
			Rooms: []encounter.RoomData{
				{ID: "r1", Width: 4, Height: 3, Origin: &encounter.PositionData{X: 0, Y: 0}},
				{ID: "r2", Width: 4, Height: 3, Origin: &encounter.PositionData{X: 4, Y: 3}},
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
		_, err := encounter.LoadEncounter(&encounter.LoadEncounterInput{
			Initiative: orderAsGiven{}, Data: data})
		s.Require().ErrorIs(err, encounter.ErrBadConnection)
		s.Require().Contains(err.Error(), "from-position out of bounds")
	})

	s.Run("Y exactly at height is rejected", func() {
		data := connBoundsData()
		data.Field.Connections[0].FromPosition = &encounter.PositionData{X: 0, Y: 3}
		_, err := encounter.LoadEncounter(&encounter.LoadEncounterInput{
			Initiative: orderAsGiven{}, Data: data})
		s.Require().ErrorIs(err, encounter.ErrBadConnection)
		s.Require().Contains(err.Error(), "from-position out of bounds")
	})

	s.Run("negative X is rejected", func() {
		data := connBoundsData()
		data.Field.Connections[0].FromPosition = &encounter.PositionData{X: -1, Y: 0}
		_, err := encounter.LoadEncounter(&encounter.LoadEncounterInput{
			Initiative: orderAsGiven{}, Data: data})
		s.Require().ErrorIs(err, encounter.ErrBadConnection)
		s.Require().Contains(err.Error(), "from-position out of bounds")
	})

	s.Run("negative Y is rejected", func() {
		data := connBoundsData()
		data.Field.Connections[0].FromPosition = &encounter.PositionData{X: 0, Y: -1}
		_, err := encounter.LoadEncounter(&encounter.LoadEncounterInput{
			Initiative: orderAsGiven{}, Data: data})
		s.Require().ErrorIs(err, encounter.ErrBadConnection)
		s.Require().Contains(err.Error(), "from-position out of bounds")
	})

	s.Run("Width-1,Height-1 is accepted (positive control)", func() {
		data := connBoundsData()
		data.Field.Connections[0].FromPosition = &encounter.PositionData{X: 3, Y: 2}
		_, err := encounter.LoadEncounter(&encounter.LoadEncounterInput{
			Initiative: orderAsGiven{}, Data: data})
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
			_, err := encounter.LoadEncounter(&encounter.LoadEncounterInput{
				Initiative: orderAsGiven{}, Data: data})
			s.Require().Error(err, tc.name)
			s.Require().ErrorIs(err, encounter.ErrInvalidData, tc.name)
			s.Require().ErrorIs(err, encounter.ErrNoField, tc.name)
			s.Require().Contains(err.Error(), tc.fragment,
				"the check that fired must be the one this case targets")
		})
	}
}

// TestLoadRoomEmptyIDReportsIDDefectNotOrigin pins F3 (#929 T2 second
// review round): a room with BOTH an empty ID AND a nil origin must report
// the ID defect ("room has empty id"), never a presence error that names
// the empty ID. Before this fix, convertRoomDataToRoomInput's per-room
// conversion loop checked Origin presence directly (ID validity was only
// ever checked LATER, by buildValidRoomGrids, after conversion had already
// succeeded) — so this exact fixture produced `room "" missing origin`,
// naming a room by an ID that was itself the real defect. The wire-only ID
// pre-pass (empty/duplicate) now runs as its own first pass over ALL rooms,
// before any other conversion, so an ID-defective room can never reach a
// presence check at all.
func (s *DataTestSuite) TestLoadRoomEmptyIDReportsIDDefectNotOrigin() {
	data := validEncounterData()
	data.Field.Rooms[0].ID = ""
	data.Field.Rooms[0].Origin = nil

	_, err := encounter.LoadEncounter(&encounter.LoadEncounterInput{
		Initiative: orderAsGiven{}, Data: data})
	s.Require().Error(err)
	s.Require().ErrorIs(err, encounter.ErrInvalidData)
	s.Require().ErrorIs(err, encounter.ErrNoField)
	s.Require().Contains(err.Error(), "room has empty id")
	s.Require().NotContains(err.Error(), "missing origin",
		"the ID defect must be reported — a presence error here would misname the room")
}

// connHexRoomData returns a fresh EncounterData with one 4x3 hex room and a
// member at pos — the Load-seam counterpart to encounter_test.go's
// TestHexRoomBounds. Width=4, Height=3 => Q valid in [-2,2), R valid in
// [-1.5,1.5) (axial, origin-centered — see that test's comment).
func connHexRoomData(pos encounter.PositionData) encounter.EncounterData {
	return encounter.EncounterData{
		Field: encounter.FieldData{
			Rooms: []encounter.RoomData{
				{ID: "r1", Width: 4, Height: 3, Grid: spatial.GridTypeHex, Origin: &encounter.PositionData{X: 0, Y: 0}},
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
		_, err := encounter.LoadEncounter(&encounter.LoadEncounterInput{
			Initiative: orderAsGiven{}, Data: connHexRoomData(encounter.PositionData{X: 1, Y: 1})})
		s.Require().NoError(err)
	})

	s.Run("negative Q within span accepted — rejected under the old offset HexGrid", func() {
		_, err := encounter.LoadEncounter(&encounter.LoadEncounterInput{
			Initiative: orderAsGiven{}, Data: connHexRoomData(encounter.PositionData{X: -1, Y: 0})})
		s.Require().NoError(err, "axial hex rooms are origin-centered; negative Q is ordinary, not a defect")
	})

	s.Run("Q at exactly +Width/2 rejected (upper bound exclusive)", func() {
		_, err := encounter.LoadEncounter(&encounter.LoadEncounterInput{
			Initiative: orderAsGiven{}, Data: connHexRoomData(encounter.PositionData{X: 2, Y: 0})})
		s.Require().Error(err)
		s.Require().ErrorIs(err, encounter.ErrInvalidData)
		s.Require().Contains(err.Error(), "out of bounds")
	})

	s.Run("Q at exactly -Width/2 accepted (lower bound inclusive)", func() {
		_, err := encounter.LoadEncounter(&encounter.LoadEncounterInput{
			Initiative: orderAsGiven{}, Data: connHexRoomData(encounter.PositionData{X: -2, Y: 0})})
		s.Require().NoError(err)
	})

	s.Run("Q beyond -Width/2 rejected", func() {
		_, err := encounter.LoadEncounter(&encounter.LoadEncounterInput{
			Initiative: orderAsGiven{}, Data: connHexRoomData(encounter.PositionData{X: -3, Y: 0})})
		s.Require().Error(err)
		s.Require().ErrorIs(err, encounter.ErrInvalidData)
		s.Require().Contains(err.Error(), "out of bounds")
	})
}

// TestHexConnectionEndpointNegativeAxialLoad is the Load-seam counterpart
// to encounter_test.go's TestHexConnectionEndpointNegativeAxial.
// #929 T2: mirrors encounter_test.go's TestHexConnectionEndpointNegativeAxial
// exactly (its own comment explains the geometry) — both rooms are hex now,
// not square+hex (W1 forbids mixing families in one field — see
// TestSetupAnchoring's W1 row on the Setup side). hex-b's negative-axial
// corner (-3,-3) is still what gives this test its name.
func (s *DataTestSuite) TestHexConnectionEndpointNegativeAxialLoad() {
	data := encounter.EncounterData{
		Field: encounter.FieldData{
			Rooms: []encounter.RoomData{
				{ID: "hex-a", Width: 10, Height: 10, Grid: spatial.GridTypeHex, Origin: &encounter.PositionData{X: 0, Y: 0}},
				{ID: "hex-b", Width: 6, Height: 6, Grid: spatial.GridTypeHex, Origin: &encounter.PositionData{X: 8, Y: 7}},
			},
			Connections: []encounter.ConnectionData{{
				ID: "gate", From: "hex-a", To: "hex-b",
				FromPosition: &encounter.PositionData{X: 4, Y: 4},
				ToPosition:   &encounter.PositionData{X: -3, Y: -3},
			}},
		},
		Members: []encounter.MemberData{
			{ID: "p1", Kind: encounter.KindPlayer, Room: "hex-a", Position: encounter.PositionData{X: 1, Y: 1}},
		},
		Endings:     []encounter.EndingData{{Key: "done", Kind: "external"}},
		EverMembers: []encounter.MemberID{"p1"},
	}
	_, err := encounter.LoadEncounter(&encounter.LoadEncounterInput{
		Initiative: orderAsGiven{}, Data: data})
	s.Require().NoError(err, "a connection endpoint at a negative axial coordinate must validate")
}

// validHexAxialData returns a fresh EncounterData with two hex rooms
// joined by one connection, a member, and an occluder — the Load-seam
// counterpart to encounter_test.go's validHexAxialSetup, mirrored exactly
// (its own comment explains the geometry). Every position is integral
// axial, including a negative one (gate.ToPosition).
func validHexAxialData() encounter.EncounterData {
	return encounter.EncounterData{
		Field: encounter.FieldData{
			Rooms: []encounter.RoomData{
				{ID: "hex-a", Width: 8, Height: 8, Grid: spatial.GridTypeHex, Origin: &encounter.PositionData{X: 0, Y: 0},
					Occluders: []encounter.PositionData{{X: 2, Y: 2}}},
				{ID: "hex-b", Width: 8, Height: 8, Grid: spatial.GridTypeHex, Origin: &encounter.PositionData{X: 8, Y: 1}},
			},
			Connections: []encounter.ConnectionData{{
				ID: "gate", From: "hex-a", To: "hex-b",
				FromPosition: &encounter.PositionData{X: 3, Y: 0},
				ToPosition:   &encounter.PositionData{X: -4, Y: -1},
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
		name     string
		mutate   func(d *encounter.EncounterData)
		alsoErr  error
		fragment string
	}{
		{"member position fractional", func(d *encounter.EncounterData) {
			d.Members[0].Position = encounter.PositionData{X: 0.5, Y: 0}
		}, nil, "not an integral axial cell"},
		{"connection from-position fractional", func(d *encounter.EncounterData) {
			d.Field.Connections[0].FromPosition = &encounter.PositionData{X: 1.5, Y: 1}
		}, encounter.ErrBadConnection, "not an integral axial cell"},
		{"connection to-position fractional", func(d *encounter.EncounterData) {
			d.Field.Connections[0].ToPosition = &encounter.PositionData{X: -1.5, Y: -1}
		}, encounter.ErrBadConnection, "not an integral axial cell"},
		{"occluder position fractional", func(d *encounter.EncounterData) {
			// #929 T3 Opus round F2: occluder integrality is now universal
			// (isIntegralPosition), not hex-only — see the Setup-seam
			// counterpart in encounter_test.go's TestSetupHexIntegralAxial.
			d.Field.Rooms[0].Occluders[0] = encounter.PositionData{X: 2.5, Y: 2}
		}, encounter.ErrNoField, "not a representable integral cell"},
	}
	for _, tc := range cases {
		s.Run(tc.name, func() {
			data := validHexAxialData()
			tc.mutate(&data)
			_, err := encounter.LoadEncounter(&encounter.LoadEncounterInput{
				Initiative: orderAsGiven{}, Data: data})
			s.Require().Error(err, tc.name)
			s.Require().ErrorIs(err, encounter.ErrInvalidData, tc.name)
			if tc.alsoErr != nil {
				s.Require().ErrorIs(err, tc.alsoErr, tc.name)
			}
			s.Require().Contains(err.Error(), tc.fragment,
				"the check that fired must be the one this case targets")
		})
	}

	_, err := encounter.LoadEncounter(&encounter.LoadEncounterInput{
		Initiative: orderAsGiven{}, Data: validHexAxialData()})
	s.Require().NoError(err, "integral axial positions, including negative ones, must be accepted")
}

// ============================================================
// #929 T2 — Load-side W-laws: the SAME laws encounter_test.go's
// TestSetupAnchoring pins at Setup, now pinned at Load too (unified via
// buildValidRoomGrids/validateConnectionInputs — LoadEncounter's doc
// comment in data.go).
// ============================================================

// validAnchoredHexData mirrors encounter_test.go's validAnchoredHexSetup
// exactly (its own comment explains the geometry in full: hex-big 10x4 at
// the zero-value Origin, hex-small 3x9 at the NEGATIVE-axial Origin
// (6,-5), an off-axis kissing doorway, disjoint absolute Q spans).
func validAnchoredHexData() encounter.EncounterData {
	return encounter.EncounterData{
		Field: encounter.FieldData{
			Rooms: []encounter.RoomData{
				{ID: "hex-big", Width: 10, Height: 4, Grid: spatial.GridTypeHex, Origin: &encounter.PositionData{X: 0, Y: 0}},
				{ID: "hex-small", Width: 3, Height: 9, Grid: spatial.GridTypeHex, Origin: &encounter.PositionData{X: 6, Y: -5}},
			},
			Connections: []encounter.ConnectionData{{
				ID: "gate", From: "hex-big", To: "hex-small",
				FromPosition: &encounter.PositionData{X: 4, Y: 0},
				ToPosition:   &encounter.PositionData{X: -1, Y: 4},
			}},
		},
		Members: []encounter.MemberData{
			{ID: "p1", Kind: encounter.KindPlayer, Room: "hex-big", Position: encounter.PositionData{X: 0, Y: 0}},
		},
		Endings:     []encounter.EndingData{{Key: "done", Kind: "external"}},
		EverMembers: []encounter.MemberID{"p1"},
	}
}

// TestLoadAnchoring pins the Load-seam one-defect rows for W1 (mixed grid
// families), origin legality (non-integral, infinite, and non-representable
// — isRepresentableInteger, encounter.go), W5 (nil origin presence, naming
// the room), room legality (non-positive dimensions, both axes), and W3
// (endpoints that don't kiss) — each breaking exactly ONE thing about
// validAnchoredHexData's otherwise-valid base.
func (s *DataTestSuite) TestLoadAnchoring() {
	cases := []struct {
		name     string
		mutate   func(d *encounter.EncounterData)
		alsoErr  error
		fragment string
	}{
		{"W1: mixed grid families", func(d *encounter.EncounterData) {
			d.Field.Rooms[1].Grid = ""
		}, encounter.ErrNoField, "declare different grid families"},
		{"origin legality: fractional hex origin", func(d *encounter.EncounterData) {
			d.Field.Rooms[1].Origin = &encounter.PositionData{X: 6.5, Y: -5}
		}, encounter.ErrNoField, "not a representable integral cell"},
		{"origin legality: NaN origin", func(d *encounter.EncounterData) {
			// #929 hardening round, test-gap closure item 4: NaN had ZERO
			// coverage anywhere in the suite before this. Unlike Inf below,
			// NaN does NOT get caught by room legality's magnitude check —
			// math.Abs(NaN) is NaN, and every comparison against NaN
			// (including >) is false, so a NaN origin silently slips PAST
			// `math.Abs(r.Origin.X) > maxAnchorCoord` uncaught, and is only
			// rejected later, by origin legality's isRepresentableInteger
			// (which explicitly tests IsNaN). The fragment asserts THAT
			// message specifically, proving the ordering, not just that
			// SOME check eventually rejects it.
			d.Field.Rooms[1].Origin = &encounter.PositionData{X: math.NaN(), Y: -5}
		}, encounter.ErrNoField, "not a representable integral cell"},
		{"room legality: infinite origin", func(d *encounter.EncounterData) {
			// #929 T2 second review round: caught by maxAnchorCoord's bound
			// in room legality — Inf is never <= a finite bound — before
			// origin legality's representability check ever runs.
			d.Field.Rooms[1].Origin = &encounter.PositionData{X: math.Inf(1), Y: -5}
		}, encounter.ErrNoField, "exceeds max anchor coordinate"},
		{"room legality: negative infinite origin", func(d *encounter.EncounterData) {
			// #929 hardening round, test-gap closure item 4: the row above
			// only ever tried +Inf; -Inf takes the SAME path (math.Abs(-Inf)
			// is +Inf, and +Inf > any finite bound is true) but had never
			// actually been exercised.
			d.Field.Rooms[1].Origin = &encounter.PositionData{X: math.Inf(-1), Y: -5}
		}, encounter.ErrNoField, "exceeds max anchor coordinate"},
		{"room legality: origin exceeds max anchor coordinate (1e19)", func(d *encounter.EncounterData) {
			// #929 T2 second review round: 1e19 is both non-representable
			// AND out of bounds — the bound fires first, in room legality.
			d.Field.Rooms[1].Origin = &encounter.PositionData{X: 1e19, Y: -5}
		}, encounter.ErrNoField, "exceeds max anchor coordinate"},
		{"W5: nil origin — absent, not a declared zero", func(d *encounter.EncounterData) {
			d.Field.Rooms[1].Origin = nil
		}, encounter.ErrNoField, `room "hex-small" missing origin`},
		{"room legality: zero width", func(d *encounter.EncounterData) {
			d.Field.Rooms[0].Width = 0
		}, encounter.ErrNoField, "non-positive dimensions"},
		{"room legality: zero height", func(d *encounter.EncounterData) {
			d.Field.Rooms[0].Height = 0
		}, encounter.ErrNoField, "non-positive dimensions"},
		{"room legality: negative width", func(d *encounter.EncounterData) {
			d.Field.Rooms[0].Width = -1
		}, encounter.ErrNoField, "non-positive dimensions"},
		{"room legality: negative height", func(d *encounter.EncounterData) {
			d.Field.Rooms[0].Height = -1
		}, encounter.ErrNoField, "non-positive dimensions"},
		{"W3: endpoints do not kiss", func(d *encounter.EncounterData) {
			// (1,4) is still a legal LOCAL cell in hex-small (max Q, max R)
			// — this must fail on adjacency, not on the earlier bounds check.
			d.Field.Connections[0].ToPosition = &encounter.PositionData{X: 1, Y: 4}
		}, encounter.ErrBadConnection, "not adjacent"},
		{"W3: hex axial (1,1) delta is NOT adjacent (cube distance 2)", func(d *encounter.EncounterData) {
			// Load-seam mirror of encounter_test.go's TestSetupAnchoring
			// row of the same name (#929 hardening round, test-gap closure
			// item 6): both existing Load W3 rows above sit at cube/
			// Chebyshev distance 3 either way ("endpoints do not kiss") —
			// neither discriminates a Chebyshev-on-axial mutant (which
			// would wrongly accept max(|ΔQ|,|ΔR|)=1 as adjacent) from the
			// correct cube-distance formula. hex-small's Origin shifts
			// from (6,-5) to (6,-3): the gate's endpoints, once anchored,
			// differ by axial (ΔQ=1,ΔR=1) — cube distance (1+1+2)/2=2, NOT
			// 1 — while still disjoint from hex-big (W2 passes).
			d.Field.Rooms[1].Origin = &encounter.PositionData{X: 6, Y: -3}
		}, encounter.ErrBadConnection, "distance 2"},
	}
	for _, tc := range cases {
		s.Run(tc.name, func() {
			data := validAnchoredHexData()
			tc.mutate(&data)
			_, err := encounter.LoadEncounter(&encounter.LoadEncounterInput{
				Initiative: orderAsGiven{}, Data: data})
			s.Require().Error(err, tc.name)
			s.Require().ErrorIs(err, encounter.ErrInvalidData, tc.name)
			s.Require().ErrorIs(err, tc.alsoErr, tc.name)
			s.Require().Contains(err.Error(), tc.fragment,
				"the check that fired must be the one this case targets")
		})
	}

	// The valid base itself must load — the one-defect discipline only
	// means something if zero defects pass.
	_, err := encounter.LoadEncounter(&encounter.LoadEncounterInput{
		Initiative: orderAsGiven{}, Data: validAnchoredHexData()})
	s.Require().NoError(err, "the valid anchored base fixture must load")
}

// TestLoadAnchoringOverlapNonAdjacentPair is the Load-seam counterpart to
// encounter_test.go's TestSetupAnchoringOverlapNonAdjacentPair — the TRUE
// one-defect W2 row (no connection involved at all): three square rooms,
// only the non-adjacent-in-slice (r-a, r-c) pair overlaps, witness (2,2).
// See that test's comment for why this specifically kills a W2
// adjacent-pairs-only mutant where W1's equivalent does not.
func (s *DataTestSuite) TestLoadAnchoringOverlapNonAdjacentPair() {
	data := encounter.EncounterData{
		Field: encounter.FieldData{
			Rooms: []encounter.RoomData{
				{ID: "r-a", Width: 5, Height: 5, Origin: &encounter.PositionData{X: 0, Y: 0}},
				{ID: "r-b", Width: 5, Height: 5, Origin: &encounter.PositionData{X: 20, Y: 20}},
				{ID: "r-c", Width: 5, Height: 5, Origin: &encounter.PositionData{X: 2, Y: 2}},
			},
		},
		Members: []encounter.MemberData{
			{ID: "p1", Kind: encounter.KindPlayer, Room: "r-a", Position: encounter.PositionData{X: 1, Y: 1}},
		},
		Endings:     []encounter.EndingData{{Key: "done", Kind: "external"}},
		EverMembers: []encounter.MemberID{"p1"},
	}
	_, err := encounter.LoadEncounter(&encounter.LoadEncounterInput{
		Initiative: orderAsGiven{}, Data: data})
	s.Require().Error(err)
	s.Require().ErrorIs(err, encounter.ErrInvalidData)
	s.Require().ErrorIs(err, encounter.ErrNoField)
	s.Require().Contains(err.Error(), `room "r-a" and room "r-c" overlap at absolute cell (2, 2)`)
}

// TestLoadAnchoringSquareOriginRejected is the Load-seam counterpart to
// encounter_test.go's TestSetupAnchoringSquareOriginRejected (#929
// hardening round, test-gap closure item 6): TestLoadAnchoring's
// "origin legality: fractional hex origin" row exercises this same
// check, but only ever against a HEX room — Load-seam coverage of
// origin legality was hex-shaped, leaving the square-family case to
// shared-code reasoning alone. Same geometry as the Setup sibling: a
// single 5x5 SQUARE room at Origin (0.5,1.5).
func (s *DataTestSuite) TestLoadAnchoringSquareOriginRejected() {
	data := encounter.EncounterData{
		Field: encounter.FieldData{
			Rooms: []encounter.RoomData{
				{ID: "r1", Width: 5, Height: 5, Origin: &encounter.PositionData{X: 0.5, Y: 1.5}},
			},
		},
		Members: []encounter.MemberData{
			{ID: "p1", Kind: encounter.KindPlayer, Room: "r1", Position: encounter.PositionData{X: 1, Y: 1}},
		},
		Endings:     []encounter.EndingData{{Key: "done", Kind: "external"}},
		EverMembers: []encounter.MemberID{"p1"},
	}
	_, err := encounter.LoadEncounter(&encounter.LoadEncounterInput{
		Initiative: orderAsGiven{}, Data: data})
	s.Require().Error(err, "a fractional Origin on a square room is now a defect — origin legality is universal")
	s.Require().ErrorIs(err, encounter.ErrInvalidData)
	s.Require().ErrorIs(err, encounter.ErrNoField)
	s.Require().Contains(err.Error(), "not a representable integral cell")
}

// TestLoadAnchoringHugeSquareOriginRejectedNotFalseOverlap is the
// Load-seam counterpart to encounter_test.go's
// TestSetupAnchoringHugeOriginRejectedNotFalseOverlap (#929 hardening
// round, test-gap closure item 6): TestLoadAnchoring's "room legality:
// origin exceeds max anchor coordinate (1e19)" row mutates ONLY
// hex-small's Origin within the shared hex base fixture — Load-seam
// coverage of the maxAnchorCoord bound, like origin legality above, was
// hex-shaped. Same geometry as the Setup sibling: two 5x5 SQUARE rooms
// at Origins 1e19 and 2e19 — nowhere near each other in real space, but
// which would truncate to the SAME implementation-defined int64 value
// without this bound, producing a false W2 overlap verdict.
func (s *DataTestSuite) TestLoadAnchoringHugeSquareOriginRejectedNotFalseOverlap() {
	data := encounter.EncounterData{
		Field: encounter.FieldData{
			Rooms: []encounter.RoomData{
				{ID: "r1", Width: 5, Height: 5, Origin: &encounter.PositionData{X: 1e19, Y: 0}},
				{ID: "r2", Width: 5, Height: 5, Origin: &encounter.PositionData{X: 2e19, Y: 0}},
			},
		},
		Members: []encounter.MemberData{
			{ID: "p1", Kind: encounter.KindPlayer, Room: "r1", Position: encounter.PositionData{X: 1, Y: 1}},
		},
		Endings:     []encounter.EndingData{{Key: "done", Kind: "external"}},
		EverMembers: []encounter.MemberID{"p1"},
	}
	_, err := encounter.LoadEncounter(&encounter.LoadEncounterInput{
		Initiative: orderAsGiven{}, Data: data})
	s.Require().Error(err)
	s.Require().ErrorIs(err, encounter.ErrInvalidData)
	s.Require().ErrorIs(err, encounter.ErrNoField)
	s.Require().Contains(err.Error(), "exceeds max anchor coordinate",
		"must reject at room legality, not fall through to a W2 overlap verdict on garbage-truncated positions")
	s.Require().NotContains(err.Error(), "overlap at absolute cell",
		"the old bug produced a W2 overlap message here — this must NOT be that")
}

// TestLoadAnchoringFractionalSquareEndpointSubUnitDistance is the
// Load-seam counterpart to encounter_test.go's
// TestSetupAnchoringFractionalSquareEndpointSubUnitDistance (#929
// hardening round, test-gap closure item 6): W3's strict `dist != 1`
// comparison — as opposed to a `> 1` mutant that would wrongly accept a
// sub-1 gap as "close enough" — had no Load-seam coverage at all. Same
// geometry as the Setup sibling: r1 is 3x3 at the zero-value Origin; r2
// is 3x3 at Origin (3,0), immediately east — fully disjoint (r1 absolute
// x:[0,2], r2 absolute x:[3,5]). FromPosition (2.5,1), a legal
// fractional cell in r1, projects to absolute (2.5,1) — only 0.5
// Chebyshev distance from ToPosition (0,1)'s absolute (3,1).
func (s *DataTestSuite) TestLoadAnchoringFractionalSquareEndpointSubUnitDistance() {
	data := encounter.EncounterData{
		Field: encounter.FieldData{
			Rooms: []encounter.RoomData{
				{ID: "r1", Width: 3, Height: 3, Origin: &encounter.PositionData{X: 0, Y: 0}},
				{ID: "r2", Width: 3, Height: 3, Origin: &encounter.PositionData{X: 3, Y: 0}},
			},
			Connections: []encounter.ConnectionData{{
				ID: "gate", From: "r1", To: "r2",
				FromPosition: &encounter.PositionData{X: 2.5, Y: 1},
				ToPosition:   &encounter.PositionData{X: 0, Y: 1},
			}},
		},
		Members: []encounter.MemberData{
			{ID: "p1", Kind: encounter.KindPlayer, Room: "r1", Position: encounter.PositionData{X: 0, Y: 0}},
		},
		Endings:     []encounter.EndingData{{Key: "done", Kind: "external"}},
		EverMembers: []encounter.MemberID{"p1"},
	}
	_, err := encounter.LoadEncounter(&encounter.LoadEncounterInput{
		Initiative: orderAsGiven{}, Data: data})
	s.Require().Error(err)
	s.Require().ErrorIs(err, encounter.ErrInvalidData)
	s.Require().ErrorIs(err, encounter.ErrBadConnection)
	s.Require().Contains(err.Error(), "distance 0.5")
}

// TestLoadAnchoringOversizedRoomRejectedNotFalseDisjoint is the Load-seam
// counterpart to encounter_test.go's
// TestSetupAnchoringOversizedRoomRejectedNotFalseDisjoint — same geometry,
// same maxRoomSpan bound, unified path (see LoadEncounter's doc comment).
func (s *DataTestSuite) TestLoadAnchoringOversizedRoomRejectedNotFalseDisjoint() {
	data := encounter.EncounterData{
		Field: encounter.FieldData{
			Rooms: []encounter.RoomData{
				{ID: "r1", Width: 1000, Height: 5, Origin: &encounter.PositionData{X: 0, Y: 0}},
				{ID: "r2", Width: math.MaxInt, Height: 5, Origin: &encounter.PositionData{X: 999, Y: 0}},
			},
		},
		Members: []encounter.MemberData{
			{ID: "p1", Kind: encounter.KindPlayer, Room: "r1", Position: encounter.PositionData{X: 1, Y: 1}},
		},
		Endings:     []encounter.EndingData{{Key: "done", Kind: "external"}},
		EverMembers: []encounter.MemberID{"p1"},
	}
	_, err := encounter.LoadEncounter(&encounter.LoadEncounterInput{
		Initiative: orderAsGiven{}, Data: data})
	s.Require().Error(err)
	s.Require().ErrorIs(err, encounter.ErrInvalidData)
	s.Require().ErrorIs(err, encounter.ErrNoField)
	s.Require().Contains(err.Error(), "exceed max room span")
	s.Require().NotContains(err.Error(), "overlap at absolute cell")
}

// TestLoadRoomCellBudgetRejectsPanicReproduction is the Load-seam
// counterpart to encounter_test.go's TestSetupRoomCellBudgetRejectsPanicReproduction
// (#929 T3 Opus round F1): the exact 2^30 x 2^30 reproduction, but as a
// hand-built persisted blob — LoadEncounter is the trust boundary for
// bytes nobody validated at declaration time, so this is the more
// important of the two reproductions to pin.
func (s *DataTestSuite) TestLoadRoomCellBudgetRejectsPanicReproduction() {
	data := encounter.EncounterData{
		Field: encounter.FieldData{
			Rooms: []encounter.RoomData{
				{ID: "huge", Width: 1 << 30, Height: 1 << 30, Origin: &encounter.PositionData{X: 0, Y: 0}},
			},
		},
		Endings: []encounter.EndingData{{Key: "done", Kind: "external"}},
	}
	_, err := encounter.LoadEncounter(&encounter.LoadEncounterInput{
		Initiative: orderAsGiven{}, Data: data})
	s.Require().Error(err, "a 2^30 x 2^30 room from a persisted blob must REJECT, not panic")
	s.Require().ErrorIs(err, encounter.ErrInvalidData)
	s.Require().ErrorIs(err, encounter.ErrNoField)
	s.Require().Contains(err.Error(), "exceeding max room cells")
}

// TestLoadOversizedRoomHeightRejected is the Load-seam counterpart to
// encounter_test.go's TestSetupOversizedRoomHeightRejected — same
// reasoning, same fixture (#929 hardening round, test-gap closure item
// 3): maxRoomSpan's Height clause was unpinned at both seams.
func (s *DataTestSuite) TestLoadOversizedRoomHeightRejected() {
	data := encounter.EncounterData{
		Field: encounter.FieldData{
			Rooms: []encounter.RoomData{
				{ID: "tall", Width: 5, Height: (1 << 30) + 1, Origin: &encounter.PositionData{X: 0, Y: 0}},
			},
		},
		Endings: []encounter.EndingData{{Key: "done", Kind: "external"}},
	}
	_, err := encounter.LoadEncounter(&encounter.LoadEncounterInput{
		Initiative: orderAsGiven{}, Data: data})
	s.Require().Error(err)
	s.Require().ErrorIs(err, encounter.ErrInvalidData)
	s.Require().ErrorIs(err, encounter.ErrNoField)
	s.Require().Contains(err.Error(), "exceed max room span",
		"a Height-only oversize must reject at room-span legality, not fall through to a different check with a different message")
}

// TestLoadFieldCellBudgetRejectsIndividuallyLegalRooms is the Load-seam
// counterpart to encounter_test.go's
// TestSetupFieldCellBudgetRejectsIndividuallyLegalRooms — SIX rooms, not
// five, and the exact true-total number asserted, for the same reason
// (that test's doc comment): #929 hardening round, test-gap closure
// item 1.
func (s *DataTestSuite) TestLoadFieldCellBudgetRejectsIndividuallyLegalRooms() {
	rooms := make([]encounter.RoomData, 6)
	for i := range rooms {
		rooms[i] = encounter.RoomData{
			ID: fmt.Sprintf("room-%d", i), Width: 1024, Height: 1024,
			Origin: &encounter.PositionData{X: 0, Y: 0},
		}
	}
	data := encounter.EncounterData{
		Field:   encounter.FieldData{Rooms: rooms},
		Endings: []encounter.EndingData{{Key: "done", Kind: "external"}},
	}
	_, err := encounter.LoadEncounter(&encounter.LoadEncounterInput{
		Initiative: orderAsGiven{}, Data: data})
	s.Require().Error(err, "individually-legal rooms whose SUM exceeds the field budget must reject")
	s.Require().ErrorIs(err, encounter.ErrInvalidData)
	s.Require().ErrorIs(err, encounter.ErrNoField)
	s.Require().Contains(err.Error(), "field has 6291456 total cells across all rooms",
		"must name the TRUE total over all six rooms, not a running total that stopped short at a mid-loop room")
	s.Require().Contains(err.Error(), "exceeding max field cells")
}

// validEndingTriggerData is the Load-seam counterpart to
// encounter_test.go's validEndingTriggerSetup.
func validEndingTriggerData() encounter.EncounterData {
	return encounter.EncounterData{
		Field: encounter.FieldData{
			Rooms: []encounter.RoomData{{ID: "hall", Width: 5, Height: 5, Origin: &encounter.PositionData{X: 0, Y: 0}}},
		},
		Endings: []encounter.EndingData{
			{Key: "reach", Kind: "reached_position", Room: "hall", Position: &encounter.PositionData{X: 3, Y: 3}},
		},
	}
}

// TestLoadEndingTriggerValidation is the Load-seam counterpart to
// encounter_test.go's TestSetupEndingTriggerValidation (#929 T3 Opus
// round F5).
func (s *DataTestSuite) TestLoadEndingTriggerValidation() {
	cases := []struct {
		name     string
		mutate   func(d *encounter.EncounterData)
		fragment string
	}{
		{"unknown room", func(d *encounter.EncounterData) {
			d.Endings[0].Room = "nowhere"
		}, "unknown room"},
		{"out of bounds position", func(d *encounter.EncounterData) {
			d.Endings[0].Position = &encounter.PositionData{X: 100, Y: 100}
		}, "out of bounds"},
	}
	for _, tc := range cases {
		s.Run(tc.name, func() {
			data := validEndingTriggerData()
			tc.mutate(&data)
			_, err := encounter.LoadEncounter(&encounter.LoadEncounterInput{
				Initiative: orderAsGiven{}, Data: data})
			s.Require().Error(err, tc.name)
			s.Require().ErrorIs(err, encounter.ErrInvalidData, tc.name)
			s.Require().ErrorIs(err, encounter.ErrNoEnding, tc.name)
			s.Require().Contains(err.Error(), tc.fragment,
				"the check that fired must be the one this case targets")
		})
	}

	_, err := encounter.LoadEncounter(&encounter.LoadEncounterInput{
		Initiative: orderAsGiven{}, Data: validEndingTriggerData()})
	s.Require().NoError(err, "a trigger naming a real room and in-bounds position must validate")
}

// TestLoadEndingTriggerHexNonIntegralRejected is the Load-seam counterpart
// to encounter_test.go's TestSetupEndingTriggerHexNonIntegralRejected.
func (s *DataTestSuite) TestLoadEndingTriggerHexNonIntegralRejected() {
	data := encounter.EncounterData{
		Field: encounter.FieldData{
			Rooms: []encounter.RoomData{{ID: "hall", Width: 8, Height: 8, Grid: spatial.GridTypeHex, Origin: &encounter.PositionData{X: 0, Y: 0}}},
		},
		Endings: []encounter.EndingData{
			{Key: "reach", Kind: "reached_position", Room: "hall", Position: &encounter.PositionData{X: 1.5, Y: 0}},
		},
	}
	_, err := encounter.LoadEncounter(&encounter.LoadEncounterInput{
		Initiative: orderAsGiven{}, Data: data})
	s.Require().Error(err)
	s.Require().ErrorIs(err, encounter.ErrInvalidData)
	s.Require().ErrorIs(err, encounter.ErrNoEnding)
	s.Require().Contains(err.Error(), "not an integral axial cell")
}

// TestOriginRoundTripByteIdentical pins W5's round-trip law: origins
// survive ToData -> LoadEncounter -> ToData byte-identically, including a
// NEGATIVE-axial one (hex-small's Origin, validAnchoredHexData) — not just
// structurally equal but literally the same bytes, since a byte-level pin
// catches a tag or field-order regression a decoded-struct comparison
// would not.
func (s *DataTestSuite) TestOriginRoundTripByteIdentical() {
	enc1, err := encounter.LoadEncounter(&encounter.LoadEncounterInput{
		Initiative: orderAsGiven{}, Data: validAnchoredHexData()})
	s.Require().NoError(err)
	data1 := enc1.ToData()
	bs1, err := json.Marshal(data1.Field)
	s.Require().NoError(err)

	enc2, err := encounter.LoadEncounter(&encounter.LoadEncounterInput{
		Initiative: orderAsGiven{}, Data: data1})
	s.Require().NoError(err)
	data2 := enc2.ToData()
	bs2, err := json.Marshal(data2.Field)
	s.Require().NoError(err)

	s.Equal(string(bs1), string(bs2), "origins (including hex-small's negative-axial one) must survive a second round trip byte-identically")
	s.Contains(string(bs1), `"origin":{"x":6,"y":-5}`, "the negative-axial origin must be present, not truncated or dropped")
}

// TestReloadedAnchoredEncounterAcceptsSameTraverse pins behavior-identity
// after reload for an anchored multi-room encounter (#929 T2 round-trip
// requirement): a member traverses a kissing doorway on the ORIGINAL
// encounter; a FRESH member placed at the same threshold on the RELOADED
// encounter traverses the same connection and lands in the same room at
// the same local position — the reload changed nothing observable about
// how the field behaves, even though the field now carries persisted
// Origins it didn't in v0.2.
func (s *DataTestSuite) TestReloadedAnchoredEncounterAcceptsSameTraverse() {
	setup := &encounter.SetupInput{
		Initiative: orderAsGiven{},
		Field: encounter.FieldInput{
			Rooms: []encounter.RoomInput{
				{ID: "hex-big", Width: 10, Height: 4, Grid: spatial.GridShapeHex},
				{ID: "hex-small", Width: 3, Height: 9, Grid: spatial.GridShapeHex, Origin: spatial.Position{X: 6, Y: -5}},
			},
			Connections: []encounter.ConnectionInput{{
				ID: "gate", From: "hex-big", To: "hex-small",
				FromPosition: spatial.Position{X: 4, Y: 0},
				ToPosition:   spatial.Position{X: -1, Y: 4},
			}},
		},
		Members: []encounter.MemberInput{
			{ID: "p1", Kind: encounter.KindPlayer, Room: "hex-big", Position: spatial.Position{X: 4, Y: 0}},
		},
		Endings: []encounter.EndingInput{{Key: "done", Trigger: encounter.TriggerExternal{}}},
	}
	enc1, err := encounter.NewEncounter(setup)
	s.Require().NoError(err)

	out1, err := enc1.Traverse(&encounter.TraverseInput{Member: "p1", Connection: "gate"})
	s.Require().NoError(err, "the original encounter accepts the traverse")
	s.Equal("hex-small", out1.Traversed.ToRoom)
	s.Equal(spatial.Position{X: -1, Y: 4}, out1.Traversed.To)

	// Reload from BEFORE the traverse (so a fresh member can repeat it) —
	// the round-trip law under test is "the FIELD behaves identically",
	// not "the same member can traverse twice".
	dataPreTraverse := enc1.ToData()
	// p1 already moved on enc1; roll dataPreTraverse's member back to the
	// threshold so the reloaded encounter's own member reenacts the same
	// traverse enc1 just proved.
	for i, m := range dataPreTraverse.Members {
		if m.ID == "p1" {
			dataPreTraverse.Members[i].Room = "hex-big"
			dataPreTraverse.Members[i].Position = encounter.PositionData{X: 4, Y: 0}
		}
	}
	enc2, err := encounter.LoadEncounter(&encounter.LoadEncounterInput{
		Initiative: orderAsGiven{}, Data: dataPreTraverse})
	s.Require().NoError(err)

	out2, err := enc2.Traverse(&encounter.TraverseInput{Member: "p1", Connection: "gate"})
	s.Require().NoError(err, "the reloaded encounter accepts the SAME traverse")
	s.Equal(out1.Traversed.ToRoom, out2.Traversed.ToRoom)
	s.Equal(out1.Traversed.To, out2.Traversed.To)
}

// TestLoadAnchoringSquareEndpointNotAdjacentDistance2 is the Load-seam
// counterpart to encounter_test.go's
// TestSetupAnchoringSquareEndpointNotAdjacentDistance2 — same geometry
// (#929 hardening round, test-gap closure item 5): square-family W3
// non-adjacency at a genuine distance (2), not the sub-unit case, had no
// coverage at either seam.
func (s *DataTestSuite) TestLoadAnchoringSquareEndpointNotAdjacentDistance2() {
	data := encounter.EncounterData{
		Field: encounter.FieldData{
			Rooms: []encounter.RoomData{
				{ID: "r1", Width: 3, Height: 3, Origin: &encounter.PositionData{X: 0, Y: 0}},
				{ID: "r2", Width: 3, Height: 3, Origin: &encounter.PositionData{X: 4, Y: 0}},
			},
			Connections: []encounter.ConnectionData{{
				ID: "gate", From: "r1", To: "r2",
				FromPosition: &encounter.PositionData{X: 2, Y: 1},
				ToPosition:   &encounter.PositionData{X: 0, Y: 1},
			}},
		},
		Members: []encounter.MemberData{
			{ID: "p1", Kind: encounter.KindPlayer, Room: "r1", Position: encounter.PositionData{X: 0, Y: 0}},
		},
		Endings:     []encounter.EndingData{{Key: "done", Kind: "external"}},
		EverMembers: []encounter.MemberID{"p1"},
	}
	_, err := encounter.LoadEncounter(&encounter.LoadEncounterInput{
		Initiative: orderAsGiven{}, Data: data})
	s.Require().Error(err)
	s.Require().ErrorIs(err, encounter.ErrInvalidData)
	s.Require().ErrorIs(err, encounter.ErrBadConnection)
	s.Require().Contains(err.Error(), "not adjacent")
}

// TestLoadRejectsHostileNonKissingBlob pins W3 against a HAND-EDITED wire
// blob, not a Go-struct mutation (the hostile-blob standard, #929 T2
// mutation evidence): marshal a genuinely valid encounter, edit the JSON
// BYTES directly to move one connection endpoint off its kissing cell, and
// confirm LoadEncounter still rejects it — proving the check runs against
// whatever bytes actually arrive over the wire, not just against values a
// well-behaved Go caller would ever construct.
func (s *DataTestSuite) TestLoadRejectsHostileNonKissingBlob() {
	valid := validAnchoredHexData()
	bs, err := json.Marshal(valid)
	s.Require().NoError(err)

	// gate.ToPosition is {"x":-1,"y":4} in the valid blob (hex-small's own
	// max-Q/max-R corner, kissing hex-big's (4,0)). Move it to hex-small's
	// OTHER corner (1,4) — still a legal LOCAL cell, but cube distance 2
	// from the absolute anchor, same defect TestLoadAnchoring's "W3" row
	// pins via a struct mutation — this pins it via raw bytes instead.
	hostile := bytes.Replace(bs, []byte(`"to_position":{"x":-1,"y":4}`), []byte(`"to_position":{"x":1,"y":4}`), 1)
	s.Require().NotEqual(bs, hostile, "the byte replacement must actually have matched something")

	var data encounter.EncounterData
	s.Require().NoError(json.Unmarshal(hostile, &data))

	_, err = encounter.LoadEncounter(&encounter.LoadEncounterInput{
		Initiative: orderAsGiven{}, Data: data})
	s.Require().Error(err, "a hand-edited blob with a non-kissing doorway must reject")
	s.Require().ErrorIs(err, encounter.ErrInvalidData)
	s.Require().ErrorIs(err, encounter.ErrBadConnection)
	s.Require().Contains(err.Error(), "not adjacent")
}

// connGridlessRoomData returns a fresh EncounterData with one 4x3 gridless
// room and a member at pos — the Load-seam counterpart to
// encounter_test.go's TestGridlessRoomInclusiveBounds.
func connGridlessRoomData(pos encounter.PositionData) encounter.EncounterData {
	return encounter.EncounterData{
		Field: encounter.FieldData{
			Rooms: []encounter.RoomData{
				{ID: "r1", Width: 4, Height: 3, Grid: spatial.GridTypeGridless, Origin: &encounter.PositionData{X: 0, Y: 0}},
			},
		},
		Members: []encounter.MemberData{
			{ID: "p1", Kind: encounter.KindPlayer, Room: "r1", Position: pos},
		},
		Endings:     []encounter.EndingData{{Key: "done", Kind: "external"}},
		EverMembers: []encounter.MemberID{"p1"},
	}
}

// TestGridlessRoomInclusiveBoundsLoad was the Load-seam counterpart to
// encounter_test.go's TestGridlessRoomInclusiveBounds, pinning gridless's
// old inclusive-bounds semantics. #929 T2 (mirroring the Setup-side
// rewrite): a stored "gridless" grid string no longer loads at all —
// gridDataToShape's doc comment — so this now pins THAT rejection instead,
// regardless of the member position within it.
func (s *DataTestSuite) TestGridlessRoomInclusiveBoundsLoad() {
	s.Run("gridless grid string rejected regardless of member position", func() {
		_, err := encounter.LoadEncounter(&encounter.LoadEncounterInput{
			Initiative: orderAsGiven{}, Data: connGridlessRoomData(encounter.PositionData{X: 4, Y: 0})})
		s.Require().Error(err, `a stored "gridless" grid string no longer loads`)
		s.Require().ErrorIs(err, encounter.ErrInvalidData)
		s.Require().ErrorIs(err, encounter.ErrNoField)
		s.Require().Contains(err.Error(), "unknown grid shape",
			"the check that fired must be shape resolution, not a downstream placement check")
	})

	s.Run("still rejected for a position that would also be out of bounds", func() {
		_, err := encounter.LoadEncounter(&encounter.LoadEncounterInput{
			Initiative: orderAsGiven{}, Data: connGridlessRoomData(encounter.PositionData{X: -1, Y: 0})})
		s.Require().Error(err)
		s.Require().ErrorIs(err, encounter.ErrInvalidData)
		s.Require().ErrorIs(err, encounter.ErrNoField,
			"shape resolution fires before any placement check ever sees this position")
	})
}

// TestLoadRejectsPlayerWithDecider pins C2 at the third seam: a player
// cannot carry a decider at load any more than at Setup or Join.
func (s *DataTestSuite) TestLoadRejectsPlayerWithDecider() {
	data := validEncounterData()
	_, err := encounter.LoadEncounter(&encounter.LoadEncounterInput{
		Initiative: orderAsGiven{}, Data: data, Deciders: map[encounter.MemberID]encounter.Decider{
			"p1": &spyDecider{},
		}})
	s.Require().ErrorIs(err, encounter.ErrInvalidData)
	s.Require().Contains(err.Error(), "cannot carry a decider")
}

func (s *DataTestSuite) TestMutation1ToDataAliases() {
	s.Run("mutation 1: ToData aliases slices", func() {
		setup := &encounter.SetupInput{
			Initiative: orderAsGiven{},
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
			Initiative: orderAsGiven{},
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
			Initiative: orderAsGiven{},
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

		// Should NOT contain spurious fields (only: outcome, clock, intel, log,
		// field, members, endings, ever_members, retention)
		s.NotContains(jsonStr, `"stowaway"`, "no stowaway fields should be marshaled")
		s.NotContains(jsonStr, `"extra"`, "no extra fields should be marshaled")
	})
}

// Mutation 4: Leaf substitution (swap Intel and Log)
func (s *DataTestSuite) TestMutation4LeafSubstitution() {
	s.Run("mutation 4: leaf data substitution (Intel/Log swapped)", func() {
		setup := &encounter.SetupInput{
			Initiative: orderAsGiven{},
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
		ctrl, err := encounter.LoadEncounter(&encounter.LoadEncounterInput{
			Initiative: orderAsGiven{}, Data: dataA})
		s.Require().NoError(err)
		ctrlView, err := ctrl.View(&encounter.ViewInput{Member: "p1"})
		s.Require().NoError(err)
		s.Require().Len(ctrlView, 1, "control: p1 holds p2")

		// Substitute B's (empty) Intel into A's data: the loaded aggregate
		// must reflect the LOADED intel — empty — proving the Intel field
		// is genuinely consumed, never re-derived from the field.
		dataA.Intel = dataB.Intel
		swapped, err := encounter.LoadEncounter(&encounter.LoadEncounterInput{
			Initiative: orderAsGiven{}, Data: dataA})
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

		_, err := encounter.LoadEncounter(&encounter.LoadEncounterInput{
			Initiative: orderAsGiven{}, Data: data, Deciders: map[encounter.MemberID]encounter.Decider{}})
		s.Require().Error(err)
		s.True(errors.Is(err, encounter.ErrInvalidData), "must validate member rooms exist")
	})
}

// Mutation 6: Re-running first-light surveil on load
func (s *DataTestSuite) TestMutation6ReSurveilOnLoad() {
	s.Run("mutation 6: no re-surveil on load (ghost stays ghost)", func() {
		setup := &encounter.SetupInput{
			Initiative: orderAsGiven{},
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

		// Create a ghost. Stepping into view starts a fight
		// (rpg-toolkit#964), and a fight member cannot free-roam — so the two
		// break off before the goblin walks back out of sight and fades. The
		// ghost this makes is the same ghost; it just has a story now.
		_, err = enc1.Move(&encounter.MoveInput{Member: "playerA", To: spatial.Position{X: 4, Y: 1}})
		s.Require().NoError(err)
		_, err = enc1.Dissolve(&encounter.DissolveInput{Member: "goblin"})
		s.Require().NoError(err, "the sighting formed a fight to break off")
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
		enc2, _ := encounter.LoadEncounter(&encounter.LoadEncounterInput{
			Initiative: orderAsGiven{}, Data: data, Deciders: map[encounter.MemberID]encounter.Decider{}})

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
			Initiative: orderAsGiven{},
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

		enc2, _ := encounter.LoadEncounter(&encounter.LoadEncounterInput{
			Initiative: orderAsGiven{}, Data: data1, Deciders: map[encounter.MemberID]encounter.Decider{}})
		data2 := enc2.ToData()
		tick2 := data2.Clock.HighWater

		// Clock must NOT be reset
		s.Equal(tick1, tick2, "clock must not reset on load")
		s.Equal(int(0), tick2, "initial tick reading should be 0")
	})
}
