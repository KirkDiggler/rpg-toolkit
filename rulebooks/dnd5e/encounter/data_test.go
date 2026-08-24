// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package encounter_test

import (
	"encoding/json"
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
// props, boundaries, connections, a member-filtered ending, an
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
// prop, boundary (still an axial-neighbor pair, ΔR=1), member p1, and
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
//
// rpg-toolkit#1162: g1 (a monster) is first in the rolled order, so
// formation now drives its turn through automatically — the golden gained
// an "active_idx":1 on the bubble (p1 active, not g1), a "turn-ended" beat
// for g1's pass, and next_seq incremented to match.
func (s *DataTestSuite) TestGoldenJSONRich() {
	enc, err := encounter.NewEncounter(&encounter.SetupInput{
		Sight: everyoneSeesTheWholeMap{}, Standing: everyoneStanding{}, Initiative: orderAsGiven{}, TurnDriver: passDriver{}, Striker: passStriker{},
		Field: encounter.FieldInput{
			Canvas: encounter.CanvasInput{Void: encounter.VoidIsOpaque(), Orientation: encounter.HexesArePointyTop()},
			Regions: []encounter.RegionInput{
				rectRegion("crypt", -10, 7, 8, 8),
				rectRegion("hall", -2, 7, 6, 6),
			},
			Props: []encounter.PropInput{rubble(-9, 9)},
			Walls: []spatial.Boundary{wall(-8, 9, -8, 10)},
			Doors: []encounter.DoorInput{openDoorway("door1", -3, 10, -2, 10)},
		},
		Members: []encounter.MemberInput{
			{ID: "p1", Kind: encounter.KindPlayer, Position: spatial.Position{X: -10, Y: 7}},
			{ID: "g1", Kind: encounter.KindMonster, Position: spatial.Position{X: -2, Y: 7}, Decider: &testDecider{intent: encounter.IntentHold{}}},
		},
		Endings: []encounter.EndingInput{
			{Key: "guarded", Trigger: encounter.TriggerReachedPosition{Position: spatial.Position{X: -7, Y: 10}, Member: core.EntityID("p1")}},
			{Key: "leave", Trigger: encounter.TriggerExternal{}},
		},
	})
	s.Require().NoError(err)
	_, err = enc.Pump(&encounter.PumpInput{})
	s.Require().NoError(err)

	bs, err := json.Marshal(enc.ToData())
	s.Require().NoError(err)
	// Exact-string pin of the wire shape since regions replaced rooms
	// (rpg-project#256): a field is "canvas" + "regions" + "props" + "walls",
	// every region listing its AUTHORED cells under "cells" beside its
	// "archetype" and "lighting" block, and an ending's target under "at".
	// Doors are top-level beside members, as before. The room-chain keys —
	// "rooms", "connections", "origin", "room" — are gone, and a blob
	// carrying them is refused by name (FieldData.Rooms).
	//
	// A MEMBER IS A CELL, not a room and a cell (rpg-toolkit#1106): "cell"
	// under its own key, absolute axial, with the region derived from where
	// they stand. The two intel payloads mirror the two members' cells by
	// construction.
	//
	// And this blob carries a FIGHT. The two members see each other across
	// the open seam at first light and trigger detection forms a bubble —
	// so this is the one place the bubbles array and intel's holdings are
	// pinned as exact bytes.
	expected := `{"clock":{"driver_progress":{"world":1},"high_water":1},"bubbles":[{"order":["g1","p1"],"active_idx":1,"round":1}],"intel":{"holdings":{"g1":{"p1":{"payload":"eyJ4IjotMTMsInkiOjd9","channel":"sight","at":1,"current_via":["sight"]}},"p1":{"g1":{"payload":"eyJ4IjotNSwieSI6N30=","channel":"sight","at":1,"current_via":["sight"]}}}},"log":{"next_seq":5,"entries":[{"seq":1,"audience":["p1","g1"],"tags":{"tag":"scene"},"payload":"eyJiZWF0Ijoic2NlbmUtb3BlbmVkIn0="},{"seq":2,"audience":["g1","p1"],"tags":{"tag":"clock"},"payload":"eyJiZWF0IjoiYnViYmxlLWZvcm1lZCIsIm9yZGVyIjpbImcxIiwicDEiXX0="},{"seq":3,"audience":["g1","p1"],"tags":{"tag":"clock"},"payload":"eyJiZWF0IjoidHVybi1lbmRlZCIsIm1lbWJlciI6ImcxIiwibmV4dCI6InAxIn0="},{"seq":4,"at":1,"audience":["g1","p1"],"tags":{"tag":"clock"},"payload":"eyJiZWF0IjoidGljayIsInRpY2siOjF9"}]},"field":{"canvas":{"void":"opaque","orientation":"pointy"},"regions":[{"id":"crypt","name":"crypt","cells":[{"x":-10,"y":7},{"x":-9,"y":7},{"x":-8,"y":7},{"x":-7,"y":7},{"x":-6,"y":7},{"x":-5,"y":7},{"x":-4,"y":7},{"x":-3,"y":7},{"x":-10,"y":8},{"x":-9,"y":8},{"x":-8,"y":8},{"x":-7,"y":8},{"x":-6,"y":8},{"x":-5,"y":8},{"x":-4,"y":8},{"x":-3,"y":8},{"x":-10,"y":9},{"x":-9,"y":9},{"x":-8,"y":9},{"x":-7,"y":9},{"x":-6,"y":9},{"x":-5,"y":9},{"x":-4,"y":9},{"x":-3,"y":9},{"x":-10,"y":10},{"x":-9,"y":10},{"x":-8,"y":10},{"x":-7,"y":10},{"x":-6,"y":10},{"x":-5,"y":10},{"x":-4,"y":10},{"x":-3,"y":10},{"x":-10,"y":11},{"x":-9,"y":11},{"x":-8,"y":11},{"x":-7,"y":11},{"x":-6,"y":11},{"x":-5,"y":11},{"x":-4,"y":11},{"x":-3,"y":11},{"x":-10,"y":12},{"x":-9,"y":12},{"x":-8,"y":12},{"x":-7,"y":12},{"x":-6,"y":12},{"x":-5,"y":12},{"x":-4,"y":12},{"x":-3,"y":12},{"x":-10,"y":13},{"x":-9,"y":13},{"x":-8,"y":13},{"x":-7,"y":13},{"x":-6,"y":13},{"x":-5,"y":13},{"x":-4,"y":13},{"x":-3,"y":13},{"x":-10,"y":14},{"x":-9,"y":14},{"x":-8,"y":14},{"x":-7,"y":14},{"x":-6,"y":14},{"x":-5,"y":14},{"x":-4,"y":14},{"x":-3,"y":14}],"archetype":"crypt","lighting":{"intensity":1}},{"id":"hall","name":"hall","cells":[{"x":-2,"y":7},{"x":-1,"y":7},{"x":0,"y":7},{"x":1,"y":7},{"x":2,"y":7},{"x":3,"y":7},{"x":-2,"y":8},{"x":-1,"y":8},{"x":0,"y":8},{"x":1,"y":8},{"x":2,"y":8},{"x":3,"y":8},{"x":-2,"y":9},{"x":-1,"y":9},{"x":0,"y":9},{"x":1,"y":9},{"x":2,"y":9},{"x":3,"y":9},{"x":-2,"y":10},{"x":-1,"y":10},{"x":0,"y":10},{"x":1,"y":10},{"x":2,"y":10},{"x":3,"y":10},{"x":-2,"y":11},{"x":-1,"y":11},{"x":0,"y":11},{"x":1,"y":11},{"x":2,"y":11},{"x":3,"y":11},{"x":-2,"y":12},{"x":-1,"y":12},{"x":0,"y":12},{"x":1,"y":12},{"x":2,"y":12},{"x":3,"y":12}],"archetype":"crypt","lighting":{"intensity":1}}],"props":[{"ref":"test:props:rubble","at":{"x":-9,"y":9},"blocks_movement":true,"blocks_line_of_sight":true,"offset":[0,0]}],"walls":[{"from":{"x":-8,"y":9},"to":{"x":-8,"y":10},"blocks_movement":true,"blocks_line_of_sight":true}]},"members":[{"id":"g1","kind":"monster","cell":{"x":-5,"y":7}},{"id":"p1","kind":"player","cell":{"x":-13,"y":7}}],"doors":[{"id":"door1","edges":[{"from":{"x":-8,"y":10},"to":{"x":-7,"y":10}}],"state":"open"}],"endings":[{"key":"guarded","kind":"reached_position","at":{"x":-7,"y":10},"member":"p1"},{"key":"leave","kind":"external"}],"ever_members":["g1","p1"],"retention":32}`
	s.Equal(expected, string(bs))
}

// TestEndingsOrderSurvivesReload pins C8 first-declared-wins across the
// persistence boundary: two endings match the same position; the FIRST
// declared fires, before and after a reload. A load that scrambles
// ending order changes which outcome the campaign receives.
func (s *DataTestSuite) TestEndingsOrderSurvivesReload() {
	setup := &encounter.SetupInput{
		Sight: everyoneSeesTheWholeMap{}, Standing: everyoneStanding{}, Initiative: orderAsGiven{}, TurnDriver: passDriver{}, Striker: passStriker{},
		Field: encounter.FieldInput{Canvas: encounter.CanvasInput{Void: encounter.VoidIsOpaque(), Orientation: encounter.HexesArePointyTop()}, Regions: []encounter.RegionInput{rectRegion("r1", 0, 0, 5, 5)}},
		Members: []encounter.MemberInput{
			{ID: "p1", Kind: encounter.KindPlayer, Position: spatial.Position{X: 1, Y: 1}},
		},
		Endings: []encounter.EndingInput{
			{Key: "first", Trigger: encounter.TriggerReachedPosition{Position: spatial.Position{X: 3, Y: 3}}},
			{Key: "second", Trigger: encounter.TriggerReachedPosition{Position: spatial.Position{X: 3, Y: 3}}},
		},
	}
	enc1, err := encounter.NewEncounter(setup)
	s.Require().NoError(err)
	enc2, err := encounter.LoadEncounter(&encounter.LoadEncounterInput{
		Sight: everyoneSeesTheWholeMap{}, Standing: everyoneStanding{}, Initiative: orderAsGiven{}, TurnDriver: passDriver{}, Striker: passStriker{}, Data: enc1.ToData()})
	s.Require().NoError(err)

	out, err := enc2.Step(&encounter.StepInput{Member: "p1", To: cellAt(3, 3)})
	s.Require().NoError(err)
	s.Require().NotNil(out.Outcome)
	s.Equal("first", out.Outcome.Ending,
		"first-declared wins after reload — a load that scrambles ending order is a C8 violation")
}

// TestSetupInputNotAliased pins T6 review M4: a caller that edits its
// own SetupInput after construction must not corrupt the persistence
// source (the encounter deep-copies the field description). Also covers
// connections: mutating the caller's ConnectionInput slice (and its
// endpoint positions) after NewEncounter must not affect the encounter.
//
// #929 T1: door1's FromPosition moved to r1's top-right corner (4,0) — an
// interior cell like the original (1,0)'s neighbor set never fully escapes
// r1's own footprint diagonally, but a corner's does. r1 is anchored at (0,1)
// and r2 at (5,0), which puts r2 diagonally past that corner: absolute
// FromPosition local(4,0)+(0,1)=(4,1) and absolute ToPosition
// local(0,0)+(5,0)=(5,0) are Chebyshev-adjacent (distance 1, a diagonal kiss),
// while r1's absolute footprint (x:[0,4],y:[1,5]) and r2's (x:[5,9],y:[0,4])
// share no x value at all, so they stay disjoint (W2) regardless of y.
//
// Both anchors are non-negative because the field compiles onto ONE grid and a
// square grid starts at (0,0) — W5, rpg-toolkit#1106.
func (s *DataTestSuite) TestSetupInputNotAliased() {
	setup := &encounter.SetupInput{
		Sight: everyoneSeesTheWholeMap{}, Standing: everyoneStanding{}, Initiative: orderAsGiven{}, TurnDriver: passDriver{}, Striker: passStriker{},
		Field: encounter.FieldInput{
			Canvas:  encounter.CanvasInput{Void: encounter.VoidIsOpaque(), Orientation: encounter.HexesArePointyTop()},
			Regions: []encounter.RegionInput{rectRegion("r1", 0, 1, 5, 5), rectRegion("r2", 5, 0, 5, 5)},
			Props:   []encounter.PropInput{rubble(3, 4)},
			Walls:   []spatial.Boundary{wall(4, 1, 5, 1)},
		},
		Members: []encounter.MemberInput{
			{ID: "p1", Kind: encounter.KindPlayer, Position: spatial.Position{X: 1, Y: 2}},
		},
		Endings: []encounter.EndingInput{{Key: "done", Trigger: encounter.TriggerExternal{}}},
	}
	enc, err := encounter.NewEncounter(setup)
	s.Require().NoError(err)

	setup.Field.Regions[0].ID = "VANDALIZED"
	setup.Field.Regions[0].Cells[0] = cellAt(999, 999)
	setup.Field.Regions[0].Lighting.Intensity = 0.1
	setup.Field.Props[0].At = cellAt(4, 4)
	setup.Field.Walls[0].From = spatial.Position{X: 4, Y: 4}

	data := enc.ToData()
	s.Require().Len(data.Field.Regions, 2)
	s.Equal("r1", data.Field.Regions[0].ID, "the snapshot must not see the caller's vandalism")
	s.Equal(encounter.PositionData{X: 0, Y: 1}, data.Field.Regions[0].Cells[0])
	s.Equal(1.0, *data.Field.Regions[0].Lighting.Intensity)
	s.Equal(encounter.PositionData{X: 3, Y: 4}, data.Field.Props[0].At)
	s.Equal(encounter.PositionData{X: 4, Y: 1}, data.Field.Walls[0].From)

	// And the corrupted-input snapshot must still LOAD (the M4 symptom
	// was an encounter that became permanently unsavable).
	_, err = encounter.LoadEncounter(&encounter.LoadEncounterInput{
		Sight: everyoneSeesTheWholeMap{}, Standing: everyoneStanding{}, Initiative: orderAsGiven{}, TurnDriver: passDriver{}, Striker: passStriker{}, Data: data})
	s.Require().NoError(err)
}

func (s *DataTestSuite) TestActionViewUsesRangeFeetJSONWithoutLegacyAlias() {
	raw, err := json.Marshal(encounter.ActionViewData{
		Ref:       core.Ref{Module: "dnd5e", Type: "monster_actions", ID: "skeleton-shortbow"},
		RangeFeet: 320,
	})
	s.Require().NoError(err)
	s.JSONEq(`{"ref":{"module":"dnd5e","type":"monster_actions","id":"skeleton-shortbow"},"range_feet":320}`, string(raw))
	s.NotContains(string(raw), "reach_feet")
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
			Sight: everyoneSeesTheWholeMap{}, Standing: everyoneStanding{}, Initiative: orderAsGiven{}, TurnDriver: passDriver{}, Striker: passStriker{},
			Field: encounter.FieldInput{
				Canvas:  encounter.CanvasInput{Void: encounter.VoidIsOpaque(), Orientation: encounter.HexesArePointyTop()},
				Regions: []encounter.RegionInput{rectRegion("crypt", 0, 0, 10, 10)}, Props: wallColumn(5, 3, 7), Walls: []spatial.Boundary{{
					From:              spatial.Position{X: 3, Y: 3},
					To:                spatial.Position{X: 3, Y: 4},
					BlocksMovement:    true,
					BlocksLineOfSight: true,
				}},
			},
			Members: []encounter.MemberInput{
				{
					ID:       "player1",
					Kind:     encounter.KindPlayer,
					Position: spatial.Position{X: 1, Y: 1},
				},
				{
					ID:       "goblin",
					Kind:     encounter.KindMonster,
					Position: spatial.Position{X: 8, Y: 8},
					Decider:  &testDecider{intent: encounter.IntentHold{}},
				},
			},
			Endings: []encounter.EndingInput{
				{
					Key:     "stairs",
					Trigger: encounter.TriggerReachedPosition{Position: spatial.Position{X: 0, Y: 0}},
				},
			},
		}

		enc1, err := encounter.NewEncounter(setup)
		s.Require().NoError(err)

		// Convert to data
		data1 := enc1.ToData()

		// Load from data (without decider for goblin)
		enc2, err := encounter.LoadEncounter(&encounter.LoadEncounterInput{
			Sight: everyoneSeesTheWholeMap{}, Standing: everyoneStanding{}, Initiative: orderAsGiven{}, TurnDriver: passDriver{}, Striker: passStriker{}, Data: data1, Deciders: map[encounter.MemberID]encounter.Decider{}})
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
		// Create encounter with a wall that will cause a ghost to form
		setup := &encounter.SetupInput{
			Sight: everyoneSeesTheWholeMap{}, Standing: everyoneStanding{}, Initiative: orderAsGiven{}, TurnDriver: passDriver{}, Striker: passStriker{},
			Field: encounter.FieldInput{
				Canvas:  encounter.CanvasInput{Void: encounter.VoidIsOpaque(), Orientation: encounter.HexesArePointyTop()},
				Regions: []encounter.RegionInput{rectRegion("crypt", 0, 0, 10, 10)},
				// A wall across y=3 blocks sight. It was a lone pillar
				// until spatial v0.9.1, which leans around one — see
				// testwalls_test.go. Its span is chosen so the scene
				// still reads the same three ways: blocked at first
				// light, open once playerA steps to (4,1), blocked
				// again once the goblin ducks behind it.
				Props: wallRow(3, 3, 5),
			},
			Members: []encounter.MemberInput{
				{
					ID:       "playerA",
					Kind:     encounter.KindPlayer,
					Position: spatial.Position{X: 1, Y: 1},
				},
				{
					ID:       "goblin",
					Kind:     encounter.KindMonster,
					Position: spatial.Position{X: 9, Y: 9},
					Decider:  &testDecider{intent: encounter.IntentHold{}},
				},
			},
			Endings: []encounter.EndingInput{
				{
					Key:     "stairs",
					Trigger: encounter.TriggerReachedPosition{Position: spatial.Position{X: 0, Y: 0}},
				},
			},
		}

		enc1, err := encounter.NewEncounter(setup)
		s.Require().NoError(err)

		// Move goblin behind the wall to create a ghost
		_, err = enc1.Step(&encounter.StepInput{
			Member: "playerA",
			To:     cellAt(4, 1),
		})
		s.Require().NoError(err)

		// The two saw each other, which starts a fight (rpg-toolkit#964), and
		// a fight member cannot free-roam — so they break off before the
		// goblin steps behind the wall and becomes the ghost this test is
		// about.
		_, err = enc1.Dissolve(&encounter.DissolveInput{Member: "goblin"})
		s.Require().NoError(err)

		// Move goblin to create ghost at last-seen position
		_, err = enc1.Step(&encounter.StepInput{
			Member: "goblin",
			To:     cellAt(5, 6), // Behind the wall from A's view
		})
		s.Require().NoError(err)

		data1 := enc1.ToData()

		// Load and verify ghost is still there
		enc2, err := encounter.LoadEncounter(&encounter.LoadEncounterInput{
			Sight: everyoneSeesTheWholeMap{}, Standing: everyoneStanding{}, Initiative: orderAsGiven{}, TurnDriver: passDriver{}, Striker: passStriker{}, Data: data1, Deciders: map[encounter.MemberID]encounter.Decider{}})
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
			Sight: everyoneSeesTheWholeMap{}, Standing: everyoneStanding{}, Initiative: orderAsGiven{}, TurnDriver: passDriver{}, Striker: passStriker{},
			Field: encounter.FieldInput{
				Canvas:  encounter.CanvasInput{Void: encounter.VoidIsOpaque(), Orientation: encounter.HexesArePointyTop()},
				Regions: []encounter.RegionInput{rectRegion("crypt", 0, 0, 10, 10)},
				// A wall across the room keeps playerA and the
				// goblin out of each other's sight, so the scene
				// opens in free roam and the goblin stays the
				// world's to pump. Co-located and visible would be
				// a fight at first light (rpg-toolkit#964), and a
				// fight monster's decider is never consulted.
				Props: wallRow(5, 1, 8),
			},
			Members: []encounter.MemberInput{
				{
					ID:       "playerA",
					Kind:     encounter.KindPlayer,
					Position: spatial.Position{X: 1, Y: 1},
				},
			},
			Endings: []encounter.EndingInput{
				{
					Key:     "stairs",
					Trigger: encounter.TriggerReachedPosition{Position: spatial.Position{X: 0, Y: 0}},
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
			Sight: everyoneSeesTheWholeMap{}, Standing: everyoneStanding{}, Initiative: orderAsGiven{}, TurnDriver: passDriver{}, Striker: passStriker{}, Data: data1, Deciders: map[encounter.MemberID]encounter.Decider{}})
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
			Sight: everyoneSeesTheWholeMap{}, Standing: everyoneStanding{}, Initiative: orderAsGiven{}, TurnDriver: passDriver{}, Striker: passStriker{},
			Field: encounter.FieldInput{
				Canvas:  encounter.CanvasInput{Void: encounter.VoidIsOpaque(), Orientation: encounter.HexesArePointyTop()},
				Regions: []encounter.RegionInput{rectRegion("crypt", 0, 0, 10, 10)},
				// A wall across the room keeps playerA and the
				// goblin out of each other's sight, so the scene
				// opens in free roam and the goblin stays the
				// world's to pump. Co-located and visible would be
				// a fight at first light (rpg-toolkit#964), and a
				// fight monster's decider is never consulted.
				Props: wallRow(5, 1, 8),
			},
			Members: []encounter.MemberInput{
				{
					ID:       "playerA",
					Kind:     encounter.KindPlayer,
					Position: spatial.Position{X: 1, Y: 1},
				},
			},
			Endings: []encounter.EndingInput{
				{
					Key:     "stairs",
					Trigger: encounter.TriggerReachedPosition{Position: spatial.Position{X: 0, Y: 0}},
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
		_, err = enc1.Step(&encounter.StepInput{
			Member: "playerA",
			To:     cellAt(0, 0),
		})
		s.Require().NoError(err)

		status1, _ := enc1.Status()
		data1 := enc1.ToData()

		// Load and verify outcome matches
		enc2, err := encounter.LoadEncounter(&encounter.LoadEncounterInput{
			Sight: everyoneSeesTheWholeMap{}, Standing: everyoneStanding{}, Initiative: orderAsGiven{}, TurnDriver: passDriver{}, Striker: passStriker{}, Data: data1, Deciders: map[encounter.MemberID]encounter.Decider{}})
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
			Sight: everyoneSeesTheWholeMap{}, Standing: everyoneStanding{}, Initiative: orderAsGiven{}, TurnDriver: passDriver{}, Striker: passStriker{},
			Field: encounter.FieldInput{
				Canvas:  encounter.CanvasInput{Void: encounter.VoidIsOpaque(), Orientation: encounter.HexesArePointyTop()},
				Regions: []encounter.RegionInput{rectRegion("crypt", 0, 0, 10, 10)},
				// A wall across the room keeps playerA and the
				// goblin out of each other's sight, so the scene
				// opens in free roam and the goblin stays the
				// world's to pump. Co-located and visible would be
				// a fight at first light (rpg-toolkit#964), and a
				// fight monster's decider is never consulted.
				Props: wallRow(5, 1, 8),
			},
			Members: []encounter.MemberInput{
				{
					ID:       "playerA",
					Kind:     encounter.KindPlayer,
					Position: spatial.Position{X: 1, Y: 1},
				},
			},
			Endings: []encounter.EndingInput{
				{
					Key:     "stairs",
					Trigger: encounter.TriggerReachedPosition{Position: spatial.Position{X: 0, Y: 0}},
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
			Sight: everyoneSeesTheWholeMap{}, Standing: everyoneStanding{}, Initiative: orderAsGiven{}, TurnDriver: passDriver{}, Striker: passStriker{}, Data: data1})
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
			Sight: everyoneSeesTheWholeMap{}, Standing: everyoneStanding{}, Initiative: orderAsGiven{}, TurnDriver: passDriver{}, Striker: passStriker{},
			Field: encounter.FieldInput{
				Canvas:  encounter.CanvasInput{Void: encounter.VoidIsOpaque(), Orientation: encounter.HexesArePointyTop()},
				Regions: []encounter.RegionInput{rectRegion("crypt", 0, 0, 10, 10)},
				// A wall across the room keeps playerA and the
				// goblin out of each other's sight, so the scene
				// opens in free roam and the goblin stays the
				// world's to pump. Co-located and visible would be
				// a fight at first light (rpg-toolkit#964), and a
				// fight monster's decider is never consulted.
				Props: wallRow(5, 1, 8),
			},
			Members: []encounter.MemberInput{
				{
					ID:       "playerA",
					Kind:     encounter.KindPlayer,
					Position: spatial.Position{X: 1, Y: 1},
				},
			},
			Endings: []encounter.EndingInput{
				{
					Key:     "stairs",
					Trigger: encounter.TriggerReachedPosition{Position: spatial.Position{X: 0, Y: 0}},
				},
			},
		}

		enc1, err := encounter.NewEncounter(setup)
		s.Require().NoError(err)

		data1 := enc1.ToData()

		// Load
		enc2, err := encounter.LoadEncounter(&encounter.LoadEncounterInput{
			Sight: everyoneSeesTheWholeMap{}, Standing: everyoneStanding{}, Initiative: orderAsGiven{}, TurnDriver: passDriver{}, Striker: passStriker{}, Data: data1, Deciders: map[encounter.MemberID]encounter.Decider{}})
		s.Require().NoError(err)

		// Move should work
		out, err := enc2.Step(&encounter.StepInput{
			Member: "playerA",
			To:     cellAt(2, 2),
		})

		s.Require().NoError(err)
		s.NotNil(out)
		s.Equal(cellAt(2, 2), out.Stepped.To)
	})
}

// TestGoldenJSONOpen pins the JSON representation of a small open encounter.
func (s *DataTestSuite) TestGoldenJSONOpen() {
	s.Run("small open encounter golden JSON", func() {
		setup := &encounter.SetupInput{
			Sight: everyoneSeesTheWholeMap{}, Standing: everyoneStanding{}, Initiative: orderAsGiven{}, TurnDriver: passDriver{}, Striker: passStriker{},
			Field: encounter.FieldInput{
				Canvas:  encounter.CanvasInput{Void: encounter.VoidIsOpaque(), Orientation: encounter.HexesArePointyTop()},
				Regions: []encounter.RegionInput{rectRegion("room1", 0, 0, 5, 5)}, Props: []encounter.PropInput{}, Walls: []spatial.Boundary{},
			},
			Members: []encounter.MemberInput{
				{
					ID:       "p1",
					Kind:     encounter.KindPlayer,
					Position: spatial.Position{X: 2, Y: 2},
				},
			},
			Endings: []encounter.EndingInput{
				{
					Key:     "done",
					Trigger: encounter.TriggerReachedPosition{Position: spatial.Position{X: 0, Y: 0}},
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
		expectedJSON := `{"clock":{"budgets":{"p1":0}},"intel":{},"log":{"next_seq":2,"entries":[{"seq":1,"audience":["p1"],"tags":{"tag":"scene"},"payload":"eyJiZWF0Ijoic2NlbmUtb3BlbmVkIn0="}]},"field":{"canvas":{"void":"opaque","orientation":"pointy"},"regions":[{"id":"room1","name":"room1","cells":[{"x":0,"y":0},{"x":1,"y":0},{"x":2,"y":0},{"x":3,"y":0},{"x":4,"y":0},{"x":0,"y":1},{"x":1,"y":1},{"x":2,"y":1},{"x":3,"y":1},{"x":4,"y":1},{"x":0,"y":2},{"x":1,"y":2},{"x":2,"y":2},{"x":3,"y":2},{"x":4,"y":2},{"x":0,"y":3},{"x":1,"y":3},{"x":2,"y":3},{"x":3,"y":3},{"x":4,"y":3},{"x":0,"y":4},{"x":1,"y":4},{"x":2,"y":4},{"x":3,"y":4},{"x":4,"y":4}],"archetype":"crypt","lighting":{"intensity":1}}]},"members":[{"id":"p1","kind":"player","cell":{"x":1,"y":2}}],"endings":[{"key":"done","kind":"reached_position","at":{"x":0,"y":0}}],"ever_members":["p1"],"retention":32}`
		s.Equal(expectedJSON, string(jsonBytes))
	})
}

// TestGoldenJSONClosed pins the JSON representation of a closed encounter.
func (s *DataTestSuite) TestGoldenJSONClosed() {
	s.Run("small closed encounter golden JSON", func() {
		setup := &encounter.SetupInput{
			Sight: everyoneSeesTheWholeMap{}, Standing: everyoneStanding{}, Initiative: orderAsGiven{}, TurnDriver: passDriver{}, Striker: passStriker{},
			Field: encounter.FieldInput{
				Canvas:  encounter.CanvasInput{Void: encounter.VoidIsOpaque(), Orientation: encounter.HexesArePointyTop()},
				Regions: []encounter.RegionInput{rectRegion("room1", 0, 0, 5, 5)}, Props: []encounter.PropInput{}, Walls: []spatial.Boundary{},
			},
			Members: []encounter.MemberInput{
				{
					ID:       "p1",
					Kind:     encounter.KindPlayer,
					Position: spatial.Position{X: 2, Y: 2},
				},
			},
			Endings: []encounter.EndingInput{
				{
					Key:     "done",
					Trigger: encounter.TriggerReachedPosition{Position: spatial.Position{X: 0, Y: 0}},
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
		_, err = enc.Step(&encounter.StepInput{
			Member: "p1",
			To:     cellAt(0, 0),
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
		//
		// The outcome member's key is "cell", not "position" (#1068): room1
		// is anchored at the origin here, so the NUMBERS are unchanged and
		// only the key moved — which is the entire point of the rename. A
		// blob written before the flip lands nowhere on today's shape and is
		// refused by name rather than read in the wrong frame (see
		// dialect_test.go).
		expectedJSON := `{"outcome":{"ending":"done","at":1,"members":[{"id":"p1","cell":{"x":0,"y":0}}]},"clock":{"budgets":{"p1":1},"driver_progress":{"world":1},"high_water":1},"intel":{},"log":{"next_seq":4,"entries":[{"seq":1,"audience":["p1"],"tags":{"tag":"scene"},"payload":"eyJiZWF0Ijoic2NlbmUtb3BlbmVkIn0="},{"seq":2,"at":1,"audience":["p1"],"tags":{"tag":"clock"},"payload":"eyJiZWF0IjoidGljayIsInRpY2siOjF9"},{"seq":3,"at":1,"audience":["p1"],"tags":{"tag":"movement"},"payload":"eyJiZWF0IjoibW92ZWQiLCJtZW1iZXIiOiJwMSIsInBvc2l0aW9uIjp7IngiOjAsInkiOjB9fQ=="}]},"field":{"canvas":{"void":"opaque","orientation":"pointy"},"regions":[{"id":"room1","name":"room1","cells":[{"x":0,"y":0},{"x":1,"y":0},{"x":2,"y":0},{"x":3,"y":0},{"x":4,"y":0},{"x":0,"y":1},{"x":1,"y":1},{"x":2,"y":1},{"x":3,"y":1},{"x":4,"y":1},{"x":0,"y":2},{"x":1,"y":2},{"x":2,"y":2},{"x":3,"y":2},{"x":4,"y":2},{"x":0,"y":3},{"x":1,"y":3},{"x":2,"y":3},{"x":3,"y":3},{"x":4,"y":3},{"x":0,"y":4},{"x":1,"y":4},{"x":2,"y":4},{"x":3,"y":4},{"x":4,"y":4}],"archetype":"crypt","lighting":{"intensity":1}}]},"members":[{"id":"p1","kind":"player","cell":{"x":0,"y":0}}],"endings":[{"key":"done","kind":"reached_position","at":{"x":0,"y":0}}],"ever_members":["p1"],"retention":32}`
		s.Equal(expectedJSON, string(jsonBytes))
	})
}

// TestAliasImmunity verifies mutating ToData result doesn't affect aggregate.
func (s *DataTestSuite) TestAliasImmunityToData() {
	s.Run("mutating ToData result doesn't affect aggregate", func() {
		setup := &encounter.SetupInput{
			Sight: everyoneSeesTheWholeMap{}, Standing: everyoneStanding{}, Initiative: orderAsGiven{}, TurnDriver: passDriver{}, Striker: passStriker{},
			Field: encounter.FieldInput{
				Canvas:  encounter.CanvasInput{Void: encounter.VoidIsOpaque(), Orientation: encounter.HexesArePointyTop()},
				Regions: []encounter.RegionInput{rectRegion("room1", 0, 0, 5, 5)}, Props: []encounter.PropInput{}, Walls: []spatial.Boundary{},
			},
			Members: []encounter.MemberInput{
				{
					ID:       "p1",
					Kind:     encounter.KindPlayer,
					Position: spatial.Position{X: 2, Y: 2},
				},
			},
			Endings: []encounter.EndingInput{
				{
					Key:     "done",
					Trigger: encounter.TriggerReachedPosition{Position: spatial.Position{X: 0, Y: 0}},
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
		// Lighting's intensity is a pointer (RegionData's doc comment —
		// presence itself is meaningful, so it can't be a value type) —
		// mutating THROUGH it must not reach a later call's own fresh pointer.
		s.Require().NotEmpty(data1.Field.Regions)
		s.Require().NotNil(data1.Field.Regions[0].Lighting)
		*data1.Field.Regions[0].Lighting.Intensity = 0.5
		data1.Field.Regions[0].Cells[0].X = 999

		// Get data again
		data2 := enc.ToData()

		// Should not be affected by mutation
		s.NotEqual("mutated", data2.Members[0].ID)
		s.NotEqual("mutated", data2.EverMembers[0])
		s.NotEqual("mutated", data2.Endings[0].Key)
		s.Require().NotNil(data2.Field.Regions[0].Lighting)
		s.Equal(1.0, *data2.Field.Regions[0].Lighting.Intensity,
			"ToData must return a FRESH intensity pointer each call, not alias one across calls")
		s.NotEqual(999.0, data2.Field.Regions[0].Cells[0].X, "nor share a cell slice")
	})
}

// TestAliasImmunityLoadEncounter verifies mutating caller's Data doesn't affect loaded aggregate.
func (s *DataTestSuite) TestAliasImmunityLoadEncounter() {
	s.Run("mutating caller's Data after LoadEncounter doesn't affect loaded aggregate", func() {
		setup := &encounter.SetupInput{
			Sight: everyoneSeesTheWholeMap{}, Standing: everyoneStanding{}, Initiative: orderAsGiven{}, TurnDriver: passDriver{}, Striker: passStriker{},
			Field: encounter.FieldInput{
				Canvas:  encounter.CanvasInput{Void: encounter.VoidIsOpaque(), Orientation: encounter.HexesArePointyTop()},
				Regions: []encounter.RegionInput{rectRegion("room1", 0, 0, 5, 5)}, Props: []encounter.PropInput{}, Walls: []spatial.Boundary{},
			},
			Members: []encounter.MemberInput{
				{
					ID:       "p1",
					Kind:     encounter.KindPlayer,
					Position: spatial.Position{X: 2, Y: 2},
				},
			},
			Endings: []encounter.EndingInput{
				{
					Key:     "done",
					Trigger: encounter.TriggerReachedPosition{Position: spatial.Position{X: 0, Y: 0}},
				},
			},
		}

		enc1, err := encounter.NewEncounter(setup)
		s.Require().NoError(err)

		data := enc1.ToData()

		// Load FIRST, then vandalize the caller's Data: the loaded
		// aggregate must be untouched (load-side deep copy).
		enc2, err := encounter.LoadEncounter(&encounter.LoadEncounterInput{
			Sight: everyoneSeesTheWholeMap{}, Standing: everyoneStanding{}, Initiative: orderAsGiven{}, TurnDriver: passDriver{}, Striker: passStriker{}, Data: data})
		s.Require().NoError(err)

		data.Members[0].ID = "mutated"
		data.EverMembers[0] = "mutated"
		data.Endings[0].Key = "mutated"
		data.Field.Regions[0].ID = "mutated"

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
			Sight: everyoneSeesTheWholeMap{}, Standing: everyoneStanding{}, Initiative: orderAsGiven{}, TurnDriver: passDriver{}, Striker: passStriker{},
			Field: encounter.FieldInput{Canvas: encounter.CanvasInput{Void: encounter.VoidIsOpaque(), Orientation: encounter.HexesArePointyTop()}, Regions: []encounter.RegionInput{rectRegion("crypt", 0, 0, 10, 10)}},
			Members: []encounter.MemberInput{
				{ID: "playerA", Kind: encounter.KindPlayer, Position: spatial.Position{X: 1, Y: 1}},
				{ID: "goblin", Kind: encounter.KindMonster, Position: spatial.Position{X: 5, Y: 5}},
			},
			Endings: []encounter.EndingInput{{Key: "stairs",
				Trigger: encounter.TriggerReachedPosition{Position: spatial.Position{X: 0, Y: 0}}}},
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
			Sight: everyoneSeesTheWholeMap{}, Standing: everyoneStanding{}, Initiative: orderAsGiven{}, TurnDriver: passDriver{}, Striker: passStriker{}, Data: data})
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
			Sight: everyoneSeesTheWholeMap{}, Standing: everyoneStanding{}, Initiative: orderAsGiven{}, TurnDriver: passDriver{}, Striker: passStriker{},
			Field: encounter.FieldInput{
				Canvas:  encounter.CanvasInput{Void: encounter.VoidIsOpaque(), Orientation: encounter.HexesArePointyTop()},
				Regions: []encounter.RegionInput{rectRegion("crypt", 0, 0, 10, 10)},
				// A wall across the room keeps playerA and the
				// goblin out of each other's sight, so the scene
				// opens in free roam and the goblin stays the
				// world's to pump. Co-located and visible would be
				// a fight at first light (rpg-toolkit#964), and a
				// fight monster's decider is never consulted.
				Props: wallRow(5, 1, 8),
			},
			Members: []encounter.MemberInput{
				{
					ID:       "playerA",
					Kind:     encounter.KindPlayer,
					Position: spatial.Position{X: 1, Y: 1},
				},
				{
					ID:       "goblin",
					Kind:     encounter.KindMonster,
					Position: spatial.Position{X: 8, Y: 8},
					Decider:  &testDecider{intent: encounter.IntentHold{}},
				},
			},
			Endings: []encounter.EndingInput{
				{
					Key:     "stairs",
					Trigger: encounter.TriggerReachedPosition{Position: spatial.Position{X: 0, Y: 0}},
				},
			},
		}

		enc1, err := encounter.NewEncounter(setup)
		s.Require().NoError(err)

		data1 := enc1.ToData()

		// Load with goblin's decider re-attached
		decider := &testDecider{
			intent: encounter.IntentMoveTo{
				To: cellAt(7, 7),
			},
		}
		enc2, err := encounter.LoadEncounter(&encounter.LoadEncounterInput{
			Sight: everyoneSeesTheWholeMap{}, Standing: everyoneStanding{}, Initiative: orderAsGiven{}, TurnDriver: passDriver{}, Striker: passStriker{}, Data: data1, Deciders: map[encounter.MemberID]encounter.Decider{
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
			Sight: everyoneSeesTheWholeMap{}, Standing: everyoneStanding{}, Initiative: orderAsGiven{}, TurnDriver: passDriver{}, Striker: passStriker{},
			Field: encounter.FieldInput{
				Canvas:  encounter.CanvasInput{Void: encounter.VoidIsOpaque(), Orientation: encounter.HexesArePointyTop()},
				Regions: []encounter.RegionInput{rectRegion("crypt", 0, 0, 10, 10)},
				// A wall across the room keeps playerA and the
				// goblin out of each other's sight, so the scene
				// opens in free roam and the goblin stays the
				// world's to pump. Co-located and visible would be
				// a fight at first light (rpg-toolkit#964), and a
				// fight monster's decider is never consulted.
				Props: wallRow(5, 1, 8),
			},
			Members: []encounter.MemberInput{
				{
					ID:       "playerA",
					Kind:     encounter.KindPlayer,
					Position: spatial.Position{X: 1, Y: 1},
				},
				{
					ID:       "goblin",
					Kind:     encounter.KindMonster,
					Position: spatial.Position{X: 8, Y: 8},
					Decider:  &testDecider{intent: encounter.IntentHold{}},
				},
			},
			Endings: []encounter.EndingInput{
				{
					Key:     "stairs",
					Trigger: encounter.TriggerReachedPosition{Position: spatial.Position{X: 0, Y: 0}},
				},
			},
		}

		enc1, err := encounter.NewEncounter(setup)
		s.Require().NoError(err)

		data1 := enc1.ToData()

		// Load WITHOUT goblin's decider
		enc2, err := encounter.LoadEncounter(&encounter.LoadEncounterInput{
			Sight: everyoneSeesTheWholeMap{}, Standing: everyoneStanding{}, Initiative: orderAsGiven{}, TurnDriver: passDriver{}, Striker: passStriker{}, Data: data1, Deciders: map[encounter.MemberID]encounter.Decider{}})
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
		Sight: everyoneSeesTheWholeMap{}, Standing: everyoneStanding{}, Initiative: orderAsGiven{}, TurnDriver: passDriver{}, Striker: passStriker{},
		Field: encounter.FieldInput{
			Canvas:  encounter.CanvasInput{Void: encounter.VoidIsOpaque(), Orientation: encounter.HexesArePointyTop()},
			Regions: []encounter.RegionInput{rectRegion("crypt", 0, 0, 10, 10)},
		},
		Members: []encounter.MemberInput{
			{ID: "playerA", Kind: encounter.KindPlayer, Position: spatial.Position{X: 1, Y: 1}},
			{ID: "goblin", Kind: encounter.KindMonster, Position: spatial.Position{X: 8, Y: 8},
				Decider: &testDecider{intent: encounter.IntentHold{}}},
		},
		Endings: []encounter.EndingInput{{Key: "stairs", Trigger: encounter.TriggerReachedPosition{
			Position: spatial.Position{X: 0, Y: 0}}}},
	}
	enc1, err := encounter.NewEncounter(setup)
	s.Require().NoError(err)
	data1 := enc1.ToData()

	enc2, err := encounter.LoadEncounter(&encounter.LoadEncounterInput{
		Sight: everyoneSeesTheWholeMap{}, Standing: everyoneStanding{}, Initiative: orderAsGiven{}, TurnDriver: passDriver{}, Striker: passStriker{}, Data: data1, Deciders: map[encounter.MemberID]encounter.Decider{
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
		Sight: everyoneSeesTheWholeMap{}, Standing: everyoneStanding{}, Initiative: orderAsGiven{}, TurnDriver: passDriver{}, Striker: passStriker{},
		Field: encounter.FieldInput{
			Canvas:  encounter.CanvasInput{Void: encounter.VoidIsOpaque(), Orientation: encounter.HexesArePointyTop()},
			Regions: []encounter.RegionInput{rectRegion("crypt", 0, 0, 10, 10), rectRegion("antechamber", 10, 0, 10, 10)}, Walls: twoRoomSealedWall(),
		},
		Members: []encounter.MemberInput{
			{ID: "playerA", Kind: encounter.KindPlayer, Position: spatial.Position{X: 11, Y: 1}},
			{ID: "goblin", Kind: encounter.KindMonster, Position: spatial.Position{X: 8, Y: 8},
				Decider: &testDecider{intent: encounter.IntentHold{}}},
			{ID: "rat", Kind: encounter.KindMonster, Position: spatial.Position{X: 2, Y: 8},
				Decider: &testDecider{intent: encounter.IntentHold{}}},
		},
		Endings: []encounter.EndingInput{{Key: "stairs", Trigger: encounter.TriggerReachedPosition{
			Position: spatial.Position{X: 0, Y: 0}}}},
	}
	enc1, err := encounter.NewEncounter(setup)
	s.Require().NoError(err)
	data1 := enc1.ToData()

	ratDecider := &testDecider{intent: encounter.IntentMoveTo{To: cellAt(3, 8)}}
	enc2, err := encounter.LoadEncounter(&encounter.LoadEncounterInput{
		Sight: everyoneSeesTheWholeMap{}, Standing: everyoneStanding{}, Initiative: orderAsGiven{}, TurnDriver: passDriver{}, Striker: passStriker{}, Data: data1, Deciders: map[encounter.MemberID]encounter.Decider{
			"goblin": nil,
			"rat":    ratDecider,
		}})
	s.Require().NoError(err)

	out, err := enc2.Pump(&encounter.PumpInput{})
	s.Require().NoError(err)
	s.Require().Len(out.MonsterMoves, 1, "only rat's real decider produces a move")
	s.Equal(encounter.MemberID("rat"), out.MonsterMoves[0].Member)
	s.Equal(cellAt(3, 8), out.MonsterMoves[0].To)
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

// rectRegionData is rectRegion, on the wire.
func rectRegionData(id string, col, row, w, h int) encounter.RegionData {
	cells := make([]encounter.PositionData, 0, w*h)
	for r := 0; r < h; r++ {
		for c := 0; c < w; c++ {
			cells = append(cells, encounter.PositionData{X: float64(col + c), Y: float64(row + r)})
		}
	}
	intensity := 1.0
	return encounter.RegionData{
		ID: id, Name: id, Cells: cells, Archetype: testArchetype,
		Lighting: &encounter.LightingData{Intensity: &intensity},
	}
}

// pointyCanvasData is pointyCanvas, on the wire.
func pointyCanvasData() encounter.CanvasData {
	return encounter.CanvasData{Void: "opaque", Orientation: "pointy"}
}

func validEncounterData() encounter.EncounterData {
	return encounter.EncounterData{
		Field: encounter.FieldData{Canvas: pointyCanvasData(), Regions: []encounter.RegionData{
			rectRegionData("r1", 0, 0, 5, 5),
		}},
		Members: []encounter.MemberData{
			{ID: "p1", Kind: encounter.KindPlayer, Cell: &encounter.PositionData{X: 1, Y: 1}}, /*ROOM:"r1"*/
		},
		Endings:     []encounter.EndingData{{Key: "done", Kind: "external"}},
		EverMembers: []encounter.MemberID{"p1"},
	}
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
		Sight: everyoneSeesTheWholeMap{}, Standing: everyoneStanding{}, Initiative: orderAsGiven{}, TurnDriver: passDriver{}, Striker: passStriker{},
		Data: validEncounterData(),
	})
	s.Require().NoError(err)
	s.Require().NotNil(enc)
}

// TestLoadPropOnBoundaryCellAccepted is the Load-seam counterpart to
// encounter_test.go's TestSetupPropOnBoundaryCellAccepted (#929 T3
// trailing round N2).
func (s *DataTestSuite) TestLoadPropOnBoundaryCellAccepted() {
	data := encounter.EncounterData{
		Field: encounter.FieldData{
			Canvas:  pointyCanvasData(),
			Regions: []encounter.RegionData{rectRegionData("hall", 0, 0, 5, 5)},
			Props:   []encounter.PropData{rubbleData(0, 2), rubbleData(4, 2), rubbleData(2, 0), rubbleData(2, 4), rubbleData(0, 0), rubbleData(4, 4)},
		},
		Endings: []encounter.EndingData{{Key: "done", Kind: "external"}},
	}
	_, err := encounter.LoadEncounter(&encounter.LoadEncounterInput{
		Sight: everyoneSeesTheWholeMap{}, Standing: everyoneStanding{}, Initiative: orderAsGiven{}, TurnDriver: passDriver{}, Striker: passStriker{}, Data: data})
	s.Require().NoError(err, "a prop on a room's boundary cell, including a corner, must be legal at Load too")
}

// TestLoadDuplicatePropRejected is the Load-seam counterpart to
// encounter_test.go's TestSetupDuplicatePropRejected (#929 hardening
// round D).
func (s *DataTestSuite) TestLoadTwoPropsOnOneCellRejected() {
	data := encounter.EncounterData{
		Field: encounter.FieldData{
			Canvas:  pointyCanvasData(),
			Regions: []encounter.RegionData{rectRegionData("hall", 0, 0, 5, 5)},
			Props:   []encounter.PropData{rubbleData(3, 3), rubbleData(3, 3)},
		},
		Endings: []encounter.EndingData{{Key: "done", Kind: "external"}},
	}
	_, err := encounter.LoadEncounter(&encounter.LoadEncounterInput{
		Sight: everyoneSeesTheWholeMap{}, Standing: everyoneStanding{}, Initiative: orderAsGiven{}, TurnDriver: passDriver{}, Striker: passStriker{}, Data: data})
	s.Require().Error(err)
	s.Require().ErrorIs(err, encounter.ErrInvalidData)
	s.Require().ErrorIs(err, encounter.ErrNoField)
	s.Require().Contains(err.Error(), "two props at")
}

// TestLoadDuplicateEndingKeyRejected is the Load-seam counterpart to
// encounter_test.go's TestSetupDuplicateEndingKeyRejected (#929
// hardening round E).
func (s *DataTestSuite) TestLoadDuplicateEndingKeyRejected() {
	data := encounter.EncounterData{
		Field: encounter.FieldData{
			Canvas:  pointyCanvasData(),
			Regions: []encounter.RegionData{rectRegionData("hall", 0, 0, 5, 5)},
		},
		Endings: []encounter.EndingData{
			{Key: "dup", Kind: "reached_position", At: &encounter.PositionData{X: 4, Y: 4}},
			{Key: "dup", Kind: "external"},
		},
	}
	_, err := encounter.LoadEncounter(&encounter.LoadEncounterInput{
		Sight: everyoneSeesTheWholeMap{}, Standing: everyoneStanding{}, Initiative: orderAsGiven{}, TurnDriver: passDriver{}, Striker: passStriker{}, Data: data})
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
		{"zero data", func(d *encounter.EncounterData) { *d = encounter.EncounterData{} }, "bad endings", nil},
		{"no regions", func(d *encounter.EncounterData) { d.Field.Regions = nil; d.Members = nil; d.EverMembers = nil }, "no regions", encounter.ErrNoField},
		{"the room-chain dialect", func(d *encounter.EncounterData) {
			d.Field.Regions = nil
			d.Field.Rooms = json.RawMessage(`[{"id":"r1","width":5,"height":5}]`)
		}, "rpg-project#256", encounter.ErrNoField},
		{"no void", func(d *encounter.EncounterData) { d.Field.Canvas.Void = "" }, "void", encounter.ErrNoField},
		{"no orientation", func(d *encounter.EncounterData) { d.Field.Canvas.Orientation = "" }, "orientation", encounter.ErrNoField},
		{"a region with no cells", func(d *encounter.EncounterData) { d.Field.Regions[0].Cells = nil; d.Members = nil; d.EverMembers = nil }, "r1", encounter.ErrRegionEmpty},
		{"a region with no lighting", func(d *encounter.EncounterData) { d.Field.Regions[0].Lighting = nil }, "r1", encounter.ErrRegionLightingMissing},
		{"a region with no archetype", func(d *encounter.EncounterData) { d.Field.Regions[0].Archetype = "" }, "r1", encounter.ErrRegionArchetypeMissing},
		{"a wall off the floor", func(d *encounter.EncounterData) {
			d.Field.Walls = []encounter.BoundaryData{{From: encounter.PositionData{X: 4, Y: 0}, To: encounter.PositionData{X: 5, Y: 0}, BlocksMovement: true}}
		}, "walls[0]", encounter.ErrEdgeOffFloor},
		{"a wall between cells that do not touch", func(d *encounter.EncounterData) {
			d.Field.Walls = []encounter.BoundaryData{{From: encounter.PositionData{X: 0, Y: 0}, To: encounter.PositionData{X: 2, Y: 0}, BlocksMovement: true}}
		}, "walls[0]", encounter.ErrEdgeNotAdjacent},
		{"no endings", func(d *encounter.EncounterData) { d.Endings = nil }, "bad endings", nil},
		{"empty ending key", func(d *encounter.EncounterData) { d.Endings[0].Key = "" }, "bad endings", nil},
		{"reserved ending key", func(d *encounter.EncounterData) { d.Endings[0].Key = "abandoned" }, "bad endings", nil},
		{"unknown ending kind", func(d *encounter.EncounterData) { d.Endings[0].Kind = "psychic" }, "unknown ending kind", encounter.ErrNoEnding},
		{"reached_position without position", func(d *encounter.EncounterData) {
			d.Endings[0] = encounter.EndingData{Key: "done", Kind: "reached_position"}
		}, "has no at", encounter.ErrNoEnding},
		{"empty member id", func(d *encounter.EncounterData) { d.Members[0].ID = "" }, "empty member id", encounter.ErrNoMember},
		{"duplicate member ids", func(d *encounter.EncounterData) {
			d.Members = append(d.Members, d.Members[0])
		}, "duplicate member", encounter.ErrNoMember},
		{"member cell absent — the pre-#1106 room-local dialect", func(d *encounter.EncounterData) {
			d.Members[0].Cell = nil
		}, "before rpg-toolkit#1106", encounter.ErrBadPlacement},
		{"member cell owned by no region", func(d *encounter.EncounterData) {
			d.Members[0].Cell = &encounter.PositionData{X: 99, Y: 99}
		}, "owned by no region", encounter.ErrBadPlacement},
		{"outcome undeclared ending", func(d *encounter.EncounterData) {
			d.Outcome = &encounter.OutcomeData{Ending: "never-declared"}
		}, "outcome", encounter.ErrNoEnding},
		{"abandoned outcome with members present", func(d *encounter.EncounterData) {
			d.Outcome = &encounter.OutcomeData{Ending: "abandoned"}
		}, "abandoned outcome with members", encounter.ErrNoMember},
		// The "outcome member names a room the field does not have" case went
		// with the field that carried it (rpg-toolkit#1108): the region is
		// derived from the cell now, so the only way an outcome member can be
		// somewhere impossible is by CELL, which is this case.
		{"outcome member cell owned by no region", func(d *encounter.EncounterData) {
			d.Outcome = &encounter.OutcomeData{Ending: "done", Members: []encounter.MemberOutcomeData{
				{ID: "p1", Cell: &encounter.PositionData{X: 999, Y: 999}}}}
		}, "owned by no region", encounter.ErrBadPlacement},
		{"outcome member missing cell", func(d *encounter.EncounterData) {
			// Deletes the field from the wire path (nil), which is exactly what
			// a pre-#1068 blob's room-local "position" key does on arrival —
			// see dialect_test.go for that whole blob read end to end.
			d.Outcome = &encounter.OutcomeData{Ending: "done", Members: []encounter.MemberOutcomeData{
				{ID: "p1"}}}
		}, "has no cell", encounter.ErrBadPlacement},
		{"ever_members missing current member", func(d *encounter.EncounterData) {
			d.EverMembers = nil
		}, "ever_members", encounter.ErrNoMember},
	}
	for _, tc := range cases {
		s.Run(tc.name, func() {
			data := validEncounterData()
			tc.mutate(&data)
			_, err := encounter.LoadEncounter(&encounter.LoadEncounterInput{
				Sight: everyoneSeesTheWholeMap{}, Standing: everyoneStanding{}, Initiative: orderAsGiven{}, TurnDriver: passDriver{}, Striker: passStriker{}, Data: data})
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
		Sight: everyoneSeesTheWholeMap{}, Standing: everyoneStanding{}, Initiative: orderAsGiven{}, TurnDriver: passDriver{}, Striker: passStriker{}, Data: validEncounterData()})
	s.Require().NoError(err, "the valid base fixture must load")
}

// ============================================================
// #929 T2 — Load-side W-laws: the SAME laws encounter_test.go's
// TestSetupAnchoring pins at Setup, now pinned at Load too (unified via
// buildValidRoomGrids/validateConnectionInputs — LoadEncounter's doc
// comment in data.go).
// ============================================================

// TestLoadRejectsPlayerWithDecider pins C2 at the third seam: a player
// cannot carry a decider at load any more than at Setup or Join.
func (s *DataTestSuite) TestLoadRejectsPlayerWithDecider() {
	data := validEncounterData()
	_, err := encounter.LoadEncounter(&encounter.LoadEncounterInput{
		Sight: everyoneSeesTheWholeMap{}, Standing: everyoneStanding{}, Initiative: orderAsGiven{}, TurnDriver: passDriver{}, Striker: passStriker{}, Data: data, Deciders: map[encounter.MemberID]encounter.Decider{
			"p1": &spyDecider{},
		}})
	s.Require().ErrorIs(err, encounter.ErrInvalidData)
	s.Require().Contains(err.Error(), "cannot carry a decider")
}

func (s *DataTestSuite) TestMutation1ToDataAliases() {
	s.Run("mutation 1: ToData aliases slices", func() {
		setup := &encounter.SetupInput{
			Sight: everyoneSeesTheWholeMap{}, Standing: everyoneStanding{}, Initiative: orderAsGiven{}, TurnDriver: passDriver{}, Striker: passStriker{},
			Field: encounter.FieldInput{
				Canvas:  encounter.CanvasInput{Void: encounter.VoidIsOpaque(), Orientation: encounter.HexesArePointyTop()},
				Regions: []encounter.RegionInput{rectRegion("room1", 0, 0, 5, 5)}, Props: []encounter.PropInput{}, Walls: []spatial.Boundary{},
			},
			Members: []encounter.MemberInput{
				{ID: "p1", Kind: encounter.KindPlayer, Position: spatial.Position{X: 2, Y: 2}},
			},
			Endings: []encounter.EndingInput{
				{Key: "done", Trigger: encounter.TriggerReachedPosition{Position: spatial.Position{X: 0, Y: 0}}},
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
			Sight: everyoneSeesTheWholeMap{}, Standing: everyoneStanding{}, Initiative: orderAsGiven{}, TurnDriver: passDriver{}, Striker: passStriker{},
			Field: encounter.FieldInput{
				Canvas:  encounter.CanvasInput{Void: encounter.VoidIsOpaque(), Orientation: encounter.HexesArePointyTop()},
				Regions: []encounter.RegionInput{rectRegion("room1", 0, 0, 5, 5)}, Props: []encounter.PropInput{}, Walls: []spatial.Boundary{},
			},
			Members: []encounter.MemberInput{
				{ID: "p1", Kind: encounter.KindPlayer, Position: spatial.Position{X: 2, Y: 2}},
			},
			Endings: []encounter.EndingInput{
				{Key: "done", Trigger: encounter.TriggerReachedPosition{Position: spatial.Position{X: 0, Y: 0}}},
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
			Sight: everyoneSeesTheWholeMap{}, Standing: everyoneStanding{}, Initiative: orderAsGiven{}, TurnDriver: passDriver{}, Striker: passStriker{},
			Field: encounter.FieldInput{
				Canvas:  encounter.CanvasInput{Void: encounter.VoidIsOpaque(), Orientation: encounter.HexesArePointyTop()},
				Regions: []encounter.RegionInput{rectRegion("room1", 0, 0, 5, 5)}, Props: []encounter.PropInput{}, Walls: []spatial.Boundary{},
			},
			Members: []encounter.MemberInput{
				{ID: "p1", Kind: encounter.KindPlayer, Position: spatial.Position{X: 2, Y: 2}},
			},
			Endings: []encounter.EndingInput{
				{Key: "done", Trigger: encounter.TriggerReachedPosition{Position: spatial.Position{X: 0, Y: 0}}},
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
			Sight: everyoneSeesTheWholeMap{}, Standing: everyoneStanding{}, Initiative: orderAsGiven{}, TurnDriver: passDriver{}, Striker: passStriker{},
			Field: encounter.FieldInput{
				Canvas:  encounter.CanvasInput{Void: encounter.VoidIsOpaque(), Orientation: encounter.HexesArePointyTop()},
				Regions: []encounter.RegionInput{rectRegion("room1", 0, 0, 5, 5)}, Props: []encounter.PropInput{}, Walls: []spatial.Boundary{},
			},
			Members: []encounter.MemberInput{
				{ID: "p1", Kind: encounter.KindPlayer, Position: spatial.Position{X: 2, Y: 2}},
			},
			Endings: []encounter.EndingInput{
				{Key: "done", Trigger: encounter.TriggerReachedPosition{Position: spatial.Position{X: 0, Y: 0}}},
			},
		}

		// Two members so p1 actually holds something.
		setup.Members = append(setup.Members, encounter.MemberInput{
			ID: "p2", Kind: encounter.KindPlayer, Position: spatial.Position{X: 3, Y: 3},
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
			Sight: everyoneSeesTheWholeMap{}, Standing: everyoneStanding{}, Initiative: orderAsGiven{}, TurnDriver: passDriver{}, Striker: passStriker{}, Data: dataA})
		s.Require().NoError(err)
		ctrlView, err := ctrl.View(&encounter.ViewInput{Member: "p1"})
		s.Require().NoError(err)
		s.Require().Len(ctrlView, 1, "control: p1 holds p2")

		// Substitute B's (empty) Intel into A's data: the loaded aggregate
		// must reflect the LOADED intel — empty — proving the Intel field
		// is genuinely consumed, never re-derived from the field.
		dataA.Intel = dataB.Intel
		swapped, err := encounter.LoadEncounter(&encounter.LoadEncounterInput{
			Sight: everyoneSeesTheWholeMap{}, Standing: everyoneStanding{}, Initiative: orderAsGiven{}, TurnDriver: passDriver{}, Striker: passStriker{}, Data: dataA})
		s.Require().NoError(err)
		swappedView, err := swapped.View(&encounter.ViewInput{Member: "p1"})
		s.Require().NoError(err)
		s.Empty(swappedView, "the substituted (empty) intel is what the aggregate holds — Intel is consumed from data")
	})
}

// Mutation 6: Re-running first-light surveil on load
func (s *DataTestSuite) TestMutation6ReSurveilOnLoad() {
	s.Run("mutation 6: no re-surveil on load (ghost stays ghost)", func() {
		setup := &encounter.SetupInput{
			Sight: everyoneSeesTheWholeMap{}, Standing: everyoneStanding{}, Initiative: orderAsGiven{}, TurnDriver: passDriver{}, Striker: passStriker{},
			Field: encounter.FieldInput{
				Canvas:  encounter.CanvasInput{Void: encounter.VoidIsOpaque(), Orientation: encounter.HexesArePointyTop()},
				Regions: []encounter.RegionInput{rectRegion("crypt", 0, 0, 10, 10)},
				// A wall across y=3, not the lone pillar this fixture
				// used to have: spatial v0.9.1 leans around one (see
				// testwalls_test.go). Its span is chosen so the scene
				// still reads the same three ways — blocked at first
				// light, open once playerA steps to (4,1), blocked
				// again once the goblin ducks to (5,6).
				Props: wallRow(3, 3, 5),
			},
			Members: []encounter.MemberInput{
				{ID: "playerA", Kind: encounter.KindPlayer, Position: spatial.Position{X: 1, Y: 1}},
				{ID: "goblin", Kind: encounter.KindMonster, Position: spatial.Position{X: 9, Y: 9}, Decider: &testDecider{intent: encounter.IntentHold{}}},
			},
			Endings: []encounter.EndingInput{
				{Key: "stairs", Trigger: encounter.TriggerReachedPosition{Position: spatial.Position{X: 0, Y: 0}}},
			},
		}

		enc1, err := encounter.NewEncounter(setup)
		s.Require().NoError(err)

		// Create a ghost. Stepping into view starts a fight
		// (rpg-toolkit#964), and a fight member cannot free-roam — so the two
		// break off before the goblin walks back out of sight and fades. The
		// ghost this makes is the same ghost; it just has a story now.
		_, err = enc1.Step(&encounter.StepInput{Member: "playerA", To: cellAt(4, 1)})
		s.Require().NoError(err)
		_, err = enc1.Dissolve(&encounter.DissolveInput{Member: "goblin"})
		s.Require().NoError(err, "the sighting formed a fight to break off")
		_, err = enc1.Step(&encounter.StepInput{Member: "goblin", To: cellAt(5, 6)})
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
			Sight: everyoneSeesTheWholeMap{}, Standing: everyoneStanding{}, Initiative: orderAsGiven{}, TurnDriver: passDriver{}, Striker: passStriker{}, Data: data, Deciders: map[encounter.MemberID]encounter.Decider{}})

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
			Sight: everyoneSeesTheWholeMap{}, Standing: everyoneStanding{}, Initiative: orderAsGiven{}, TurnDriver: passDriver{}, Striker: passStriker{},
			Field: encounter.FieldInput{
				Canvas:  encounter.CanvasInput{Void: encounter.VoidIsOpaque(), Orientation: encounter.HexesArePointyTop()},
				Regions: []encounter.RegionInput{rectRegion("room1", 0, 0, 10, 10)}, Props: []encounter.PropInput{}, Walls: []spatial.Boundary{},
			},
			Members: []encounter.MemberInput{
				{ID: "p1", Kind: encounter.KindPlayer, Position: spatial.Position{X: 5, Y: 5}},
			},
			Endings: []encounter.EndingInput{
				{Key: "done", Trigger: encounter.TriggerReachedPosition{Position: spatial.Position{X: 0, Y: 0}}},
			},
		}

		enc1, _ := encounter.NewEncounter(setup)
		data1 := enc1.ToData()
		tick1 := data1.Clock.HighWater

		enc2, _ := encounter.LoadEncounter(&encounter.LoadEncounterInput{
			Sight: everyoneSeesTheWholeMap{}, Standing: everyoneStanding{}, Initiative: orderAsGiven{}, TurnDriver: passDriver{}, Striker: passStriker{}, Data: data1, Deciders: map[encounter.MemberID]encounter.Decider{}})
		data2 := enc2.ToData()
		tick2 := data2.Clock.HighWater

		// Clock must NOT be reset
		s.Equal(tick1, tick2, "clock must not reset on load")
		s.Equal(int(0), tick2, "initial tick reading should be 0")
	})
}
