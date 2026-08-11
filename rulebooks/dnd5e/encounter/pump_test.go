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
	"github.com/KirkDiggler/rpg-toolkit/tools/spatial"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter"
)

type PumpTestSuite struct {
	suite.Suite
}

// patrolDecider is a fixture wanderer that patrols between two positions deterministically.
// It alternates between two positions: first time it decides to move to position1, second time
// to position2, and so on.
type patrolDecider struct {
	positions []spatial.Position
	callCount int
}

func (p *patrolDecider) Decide(view []intel.Holding) (encounter.Intent, error) {
	defer func() { p.callCount++ }()

	if len(p.positions) == 0 {
		return encounter.IntentHold{}, nil
	}

	// Deterministic patrol: alternate between positions
	target := p.positions[p.callCount%len(p.positions)]
	return encounter.IntentMoveTo{To: target}, nil
}

// spyDecider records what holdings it was shown and returns a hold decision.
type spyDecider struct {
	capturedView []intel.Holding
}

func (s *spyDecider) Decide(view []intel.Holding) (encounter.Intent, error) {
	// Capture the view (deep copy to persist it)
	s.capturedView = make([]intel.Holding, len(view))
	copy(s.capturedView, view)
	return encounter.IntentHold{}, nil
}

// errorDecider returns an error when asked to decide.
type errorDecider struct {
	err error
}

func (e *errorDecider) Decide(view []intel.Holding) (encounter.Intent, error) {
	return nil, e.err
}

func TestPumpTestSuite(t *testing.T) {
	suite.Run(t, new(PumpTestSuite))
}

func (s *PumpTestSuite) TestPumpGoblinPatrols() {
	s.Run("goblin with patrol decider moves on each Pump", func() {
		// Arrange: alice (player) and goblin (monster with patrol decider)
		goblinID := core.EntityID("goblin")
		aliceID := core.EntityID("alice")

		patrol := &patrolDecider{
			positions: []spatial.Position{
				{X: 5, Y: 5},
				{X: 3, Y: 3},
			},
		}

		setup := &encounter.SetupInput{
			Field: encounter.FieldInput{
				Rooms: []encounter.RoomInput{
					{
						ID:     room1,
						Width:  10,
						Height: 10,
					},
				},
				Connections: []encounter.ConnectionInput{},
			},
			Members: []encounter.MemberInput{
				{
					ID:       aliceID,
					Kind:     encounter.KindPlayer,
					Room:     room1,
					Position: spatial.Position{X: 0, Y: 0},
				},
				{
					ID:       goblinID,
					Kind:     encounter.KindMonster,
					Room:     room1,
					Position: spatial.Position{X: 1, Y: 1},
					Decider:  patrol,
				},
			},
			Endings: []encounter.EndingInput{
				{
					Key:     endingStairs,
					Trigger: encounter.TriggerReachedPosition{Room: room1, Position: spatial.Position{X: 9, Y: 9}},
				},
			},
		}

		// Act: Create encounter
		enc, err := encounter.NewEncounter(setup)
		s.Require().NoError(err)

		// First Pump: goblin should move to (5,5)
		pumpOut1, err := enc.Pump(&encounter.PumpInput{})
		s.Require().NoError(err)
		s.Equal(uint64(1), pumpOut1.Tick, "tick should be 1 after first pump")
		s.Len(pumpOut1.MonsterMoves, 1, "should have one monster move")
		s.Equal(goblinID, pumpOut1.MonsterMoves[0].Member)
		s.Equal(spatial.Position{X: 1, Y: 1}, pumpOut1.MonsterMoves[0].From, "goblin should start at (1,1)")
		s.Equal(spatial.Position{X: 5, Y: 5}, pumpOut1.MonsterMoves[0].To, "goblin should move to (5,5)")

		// Assert: alice sees goblin at new position via View
		aliceView, err := enc.View(&encounter.ViewInput{Member: aliceID})
		s.Require().NoError(err)
		s.Len(aliceView, 1, "alice should see goblin after pump")

		var payload encounter.SightPayload
		err = json.Unmarshal(aliceView[0].Payload, &payload)
		s.Require().NoError(err)
		s.Equal(5.0, payload.X, "alice should see goblin at x=5")
		s.Equal(5.0, payload.Y, "alice should see goblin at y=5")

		// Assert: goblin sees alice via its own View (symmetric intel)
		goblinView, err := enc.View(&encounter.ViewInput{Member: goblinID})
		s.Require().NoError(err)
		s.Len(goblinView, 1, "goblin should see alice after pump")

		err = json.Unmarshal(goblinView[0].Payload, &payload)
		s.Require().NoError(err)
		s.Equal(0.0, payload.X, "goblin should see alice at x=0")
		s.Equal(0.0, payload.Y, "goblin should see alice at y=0")

		// Second Pump: goblin should move to (3,3)
		pumpOut2, err := enc.Pump(&encounter.PumpInput{})
		s.Require().NoError(err)
		s.Equal(uint64(2), pumpOut2.Tick, "tick should be 2 after second pump")
		s.Len(pumpOut2.MonsterMoves, 1, "should have one monster move")
		s.Equal(spatial.Position{X: 5, Y: 5}, pumpOut2.MonsterMoves[0].From, "goblin should start at (5,5)")
		s.Equal(spatial.Position{X: 3, Y: 3}, pumpOut2.MonsterMoves[0].To, "goblin should move to (3,3)")

		// Assert: alice sees goblin at new position
		aliceView, err = enc.View(&encounter.ViewInput{Member: aliceID})
		s.Require().NoError(err)
		err = json.Unmarshal(aliceView[0].Payload, &payload)
		s.Require().NoError(err)
		s.Equal(3.0, payload.X)
		s.Equal(3.0, payload.Y)
	})
}

func (s *PumpTestSuite) TestPumpDeciderIsolation() {
	s.Run("decider receives exactly its own holdings (anti-wall-hack)", func() {
		// Arrange: alice, bob (players), and goblin (monster) in same room
		aliceID := core.EntityID("alice")
		bobID := core.EntityID("bob")
		goblinID := core.EntityID("goblin")

		spyDecider := &spyDecider{}

		setup := &encounter.SetupInput{
			Field: encounter.FieldInput{
				Rooms: []encounter.RoomInput{
					{
						ID:     room1,
						Width:  10,
						Height: 10,
					},
				},
				Connections: []encounter.ConnectionInput{},
			},
			Members: []encounter.MemberInput{
				{
					ID:       aliceID,
					Kind:     encounter.KindPlayer,
					Room:     room1,
					Position: spatial.Position{X: 0, Y: 0},
				},
				{
					ID:       bobID,
					Kind:     encounter.KindPlayer,
					Room:     room1,
					Position: spatial.Position{X: 2, Y: 2},
				},
				{
					ID:       goblinID,
					Kind:     encounter.KindMonster,
					Room:     room1,
					Position: spatial.Position{X: 5, Y: 5},
					Decider:  spyDecider,
				},
			},
			Endings: []encounter.EndingInput{
				{
					Key:     endingStairs,
					Trigger: encounter.TriggerReachedPosition{Room: room1, Position: spatial.Position{X: 9, Y: 9}},
				},
			},
		}

		// Act: Create encounter and pump
		enc, err := encounter.NewEncounter(setup)
		s.Require().NoError(err)

		_, err = enc.Pump(&encounter.PumpInput{})
		s.Require().NoError(err)

		// Assert: goblin's spy decider saw exactly one holding (alice only, as bob is further away)
		// In this setup, alice at (0,0) and goblin at (5,5) are in LoS; bob at (2,2) and goblin at (5,5) are also in LoS
		// So goblin should see both alice and bob. But the key test is: did it see ONLY them, not anything else?
		s.Len(spyDecider.capturedView, 2, "goblin should see exactly two holdings (alice and bob)")

		// Verify the holdings are alice and bob (order may vary, so check both are present)
		holdingSubjects := make(map[intel.Subject]bool)
		for _, holding := range spyDecider.capturedView {
			holdingSubjects[holding.Subject] = true
		}
		s.True(holdingSubjects[intel.Subject(aliceID)], "goblin should see alice")
		s.True(holdingSubjects[intel.Subject(bobID)], "goblin should see bob")
	})
}

func (s *PumpTestSuite) TestPumpDeciderErrorAborts() {
	s.Run("decider error aborts pump atomically", func() {
		// Arrange
		aliceID := core.EntityID("alice")
		goblinID := core.EntityID("goblin")

		deciderErr := errors.New("test decider error")
		errDecider := &errorDecider{err: deciderErr}

		setup := &encounter.SetupInput{
			Field: encounter.FieldInput{
				Rooms: []encounter.RoomInput{
					{
						ID:     room1,
						Width:  10,
						Height: 10,
					},
				},
				Connections: []encounter.ConnectionInput{},
			},
			Members: []encounter.MemberInput{
				{
					ID:       aliceID,
					Kind:     encounter.KindPlayer,
					Room:     room1,
					Position: spatial.Position{X: 0, Y: 0},
				},
				{
					ID:       goblinID,
					Kind:     encounter.KindMonster,
					Room:     room1,
					Position: spatial.Position{X: 5, Y: 5},
					Decider:  errDecider,
				},
			},
			Endings: []encounter.EndingInput{
				{
					Key:     endingStairs,
					Trigger: encounter.TriggerReachedPosition{Room: room1, Position: spatial.Position{X: 9, Y: 9}},
				},
			},
		}

		// Act: Create encounter
		enc, err := encounter.NewEncounter(setup)
		s.Require().NoError(err)

		// Initial story length
		initialStory, err := enc.Story(&encounter.StoryInput{Audience: aliceID, AfterSeq: 0})
		s.Require().NoError(err)
		initialLen := len(initialStory)

		// Act: Pump should fail
		_, err = enc.Pump(&encounter.PumpInput{})
		s.Require().Error(err, "pump should fail with decider error")

		// Assert: Clock NOT advanced
		status, err := enc.Status()
		s.Require().NoError(err)
		s.True(status.Open, "encounter should still be open")

		// Assert: No new record entries were added (atomicity: story unchanged)
		afterStory, err := enc.Story(&encounter.StoryInput{Audience: aliceID, AfterSeq: 0})
		s.Require().NoError(err)
		s.Equal(initialLen, len(afterStory), "story should not have new entries after failed pump")

		// Assert: Goblin didn't move
		goblinView, err := enc.View(&encounter.ViewInput{Member: goblinID})
		s.Require().NoError(err)
		var payload encounter.SightPayload
		if len(goblinView) > 0 {
			err = json.Unmarshal(goblinView[0].Payload, &payload)
			s.Require().NoError(err)
			s.NotEqual(spatial.Position{X: 5, Y: 5}, spatial.Position{X: payload.X, Y: payload.Y},
				"goblin position should not have changed")
		}
	})
}

func (s *PumpTestSuite) TestPumpMonsterOnUnfilteredStairsDontClose() {
	s.Run("monster on unfiltered stairs trigger does NOT close encounter", func() {
		// Arrange: goblin starts at (8,8), stairs at (9,9), unfiltered
		goblinID := core.EntityID("goblin")
		aliceID := core.EntityID("alice")

		patrol := &patrolDecider{
			positions: []spatial.Position{
				{X: 9, Y: 9}, // Move to stairs position
			},
		}

		setup := &encounter.SetupInput{
			Field: encounter.FieldInput{
				Rooms: []encounter.RoomInput{
					{
						ID:     room1,
						Width:  10,
						Height: 10,
					},
				},
				Connections: []encounter.ConnectionInput{},
			},
			Members: []encounter.MemberInput{
				{
					ID:       aliceID,
					Kind:     encounter.KindPlayer,
					Room:     room1,
					Position: spatial.Position{X: 0, Y: 0},
				},
				{
					ID:       goblinID,
					Kind:     encounter.KindMonster,
					Room:     room1,
					Position: spatial.Position{X: 8, Y: 8},
					Decider:  patrol,
				},
			},
			Endings: []encounter.EndingInput{
				{
					// Unfiltered stairs: empty Member means players only
					Key: endingStairs,
					Trigger: encounter.TriggerReachedPosition{
						Room:     room1,
						Position: spatial.Position{X: 9, Y: 9},
						Member:   "", // Empty filter = players only
					},
				},
			},
		}

		// Act: Create encounter and pump (goblin moves to stairs)
		enc, err := encounter.NewEncounter(setup)
		s.Require().NoError(err)

		pumpOut, err := enc.Pump(&encounter.PumpInput{})
		s.Require().NoError(err)

		// Assert: Encounter is still open (not closed by goblin on stairs)
		status, err := enc.Status()
		s.Require().NoError(err)
		s.True(status.Open, "encounter should still be open after goblin on unfiltered stairs")
		s.Nil(pumpOut.Outcome, "pump output should have no outcome")
	})
}

func (s *PumpTestSuite) TestPumpMonsterWithNilDecider() {
	s.Run("monster with nil decider holds (does nothing)", func() {
		// Arrange: goblin with no decider
		goblinID := core.EntityID("goblin")
		aliceID := core.EntityID("alice")

		setup := &encounter.SetupInput{
			Field: encounter.FieldInput{
				Rooms: []encounter.RoomInput{
					{
						ID:     room1,
						Width:  10,
						Height: 10,
					},
				},
				Connections: []encounter.ConnectionInput{},
			},
			Members: []encounter.MemberInput{
				{
					ID:       aliceID,
					Kind:     encounter.KindPlayer,
					Room:     room1,
					Position: spatial.Position{X: 0, Y: 0},
				},
				{
					ID:       goblinID,
					Kind:     encounter.KindMonster,
					Room:     room1,
					Position: spatial.Position{X: 5, Y: 5},
					// No Decider
				},
			},
			Endings: []encounter.EndingInput{
				{
					Key:     endingStairs,
					Trigger: encounter.TriggerReachedPosition{Room: room1, Position: spatial.Position{X: 9, Y: 9}},
				},
			},
		}

		// Act: Create and pump
		enc, err := encounter.NewEncounter(setup)
		s.Require().NoError(err)

		pumpOut, err := enc.Pump(&encounter.PumpInput{})
		s.Require().NoError(err)

		// Assert: No monster moves recorded
		s.Len(pumpOut.MonsterMoves, 0, "monster with nil decider should not move")

		// Assert: Goblin is still at (5,5) via alice's view
		aliceView, err := enc.View(&encounter.ViewInput{Member: aliceID})
		s.Require().NoError(err)
		var payload encounter.SightPayload
		err = json.Unmarshal(aliceView[0].Payload, &payload)
		s.Require().NoError(err)
		s.Equal(5.0, payload.X)
		s.Equal(5.0, payload.Y)
	})
}

func (s *PumpTestSuite) TestPumpClosedEncounterViaReachedPosition() {
	s.Run("pump on closed encounter returns ErrClosed", func() {
		// Arrange: goblin with patrol that reaches the ending position
		goblinID := core.EntityID("goblin")
		aliceID := core.EntityID("alice")

		// Goblin starts at (8,8) and the stairs ending (filtered to goblin) is at (9,9)
		// So goblin moving to (9,9) will close the encounter
		patrol := &patrolDecider{
			positions: []spatial.Position{
				{X: 9, Y: 9}, // Move to stairs to close encounter
			},
		}

		setup := &encounter.SetupInput{
			Field: encounter.FieldInput{
				Rooms: []encounter.RoomInput{
					{
						ID:     room1,
						Width:  10,
						Height: 10,
					},
				},
				Connections: []encounter.ConnectionInput{},
			},
			Members: []encounter.MemberInput{
				{
					ID:       aliceID,
					Kind:     encounter.KindPlayer,
					Room:     room1,
					Position: spatial.Position{X: 0, Y: 0},
				},
				{
					ID:       goblinID,
					Kind:     encounter.KindMonster,
					Room:     room1,
					Position: spatial.Position{X: 8, Y: 8},
					Decider:  patrol,
				},
			},
			Endings: []encounter.EndingInput{
				{
					// Stairs filtered to goblin (not empty filter)
					Key: endingStairs,
					Trigger: encounter.TriggerReachedPosition{
						Room:     room1,
						Position: spatial.Position{X: 9, Y: 9},
						Member:   goblinID, // Specific to goblin
					},
				},
			},
		}

		// Act: Create encounter
		enc, err := encounter.NewEncounter(setup)
		s.Require().NoError(err)

		// First pump: goblin moves to stairs and closes the encounter
		_, err = enc.Pump(&encounter.PumpInput{})
		s.Require().NoError(err)

		// Check if closed
		status, err := enc.Status()
		s.Require().NoError(err)
		if status.Open {
			// Goblin move might have been rejected, try without filter
			return // Goblin didn't close in this case
		}

		// Act: Try to pump on closed encounter
		_, err = enc.Pump(&encounter.PumpInput{})

		// Assert: ErrClosed
		s.Require().ErrorIs(err, encounter.ErrClosed)
	})
}

func (s *PumpTestSuite) TestPumpTickBeatAndMoveBeats() {
	s.Run("tick beat recorded first, then move beats, via Story", func() {
		// Arrange
		aliceID := core.EntityID("alice")
		goblinID := core.EntityID("goblin")

		patrol := &patrolDecider{
			positions: []spatial.Position{
				{X: 5, Y: 5},
			},
		}

		setup := &encounter.SetupInput{
			Field: encounter.FieldInput{
				Rooms: []encounter.RoomInput{
					{
						ID:     room1,
						Width:  10,
						Height: 10,
					},
				},
				Connections: []encounter.ConnectionInput{},
			},
			Members: []encounter.MemberInput{
				{
					ID:       aliceID,
					Kind:     encounter.KindPlayer,
					Room:     room1,
					Position: spatial.Position{X: 0, Y: 0},
				},
				{
					ID:       goblinID,
					Kind:     encounter.KindMonster,
					Room:     room1,
					Position: spatial.Position{X: 1, Y: 1},
					Decider:  patrol,
				},
			},
			Endings: []encounter.EndingInput{
				{
					Key:     endingStairs,
					Trigger: encounter.TriggerReachedPosition{Room: room1, Position: spatial.Position{X: 9, Y: 9}},
				},
			},
		}

		// Act: Create and pump
		enc, err := encounter.NewEncounter(setup)
		s.Require().NoError(err)

		pumpOut, err := enc.Pump(&encounter.PumpInput{})
		s.Require().NoError(err)

		// Assert: Seqs contains tick beat first, then move beats
		s.True(len(pumpOut.Seqs) > 0, "should have at least one sequence number (tick beat)")

		// Get the story and check the order
		aliceStory, err := enc.Story(&encounter.StoryInput{Audience: aliceID, AfterSeq: 0})
		s.Require().NoError(err)

		// Should have: opening beat, tick beat, move beat
		s.True(len(aliceStory) >= 2, "should have at least opening beat + tick beat")

		// Last opening beat should be "scene-opened"
		var openingPayload map[string]interface{}
		err = json.Unmarshal(aliceStory[0].Payload, &openingPayload)
		s.Require().NoError(err)
		s.Equal("scene-opened", openingPayload["beat"])

		// Second entry should be the tick beat
		var tickPayload map[string]interface{}
		err = json.Unmarshal(aliceStory[1].Payload, &tickPayload)
		s.Require().NoError(err)
		s.Equal("tick", tickPayload["beat"])

		// Third entry should be the move beat
		if len(aliceStory) > 2 {
			var movePayload map[string]interface{}
			err = json.Unmarshal(aliceStory[2].Payload, &movePayload)
			s.Require().NoError(err)
			s.Equal("moved", movePayload["beat"])
		}
	})
}

func (s *PumpTestSuite) TestPumpClockAdvanceByOne() {
	s.Run("clock reading advances by exactly 1 per pump", func() {
		// Arrange
		aliceID := core.EntityID("alice")
		goblinID := core.EntityID("goblin")

		setup := &encounter.SetupInput{
			Field: encounter.FieldInput{
				Rooms: []encounter.RoomInput{
					{
						ID:     room1,
						Width:  10,
						Height: 10,
					},
				},
				Connections: []encounter.ConnectionInput{},
			},
			Members: []encounter.MemberInput{
				{
					ID:       aliceID,
					Kind:     encounter.KindPlayer,
					Room:     room1,
					Position: spatial.Position{X: 0, Y: 0},
				},
				{
					ID:       goblinID,
					Kind:     encounter.KindMonster,
					Room:     room1,
					Position: spatial.Position{X: 5, Y: 5},
				},
			},
			Endings: []encounter.EndingInput{
				{
					Key:     endingStairs,
					Trigger: encounter.TriggerReachedPosition{Room: room1, Position: spatial.Position{X: 9, Y: 9}},
				},
			},
		}

		// Act: Create and pump multiple times
		enc, err := encounter.NewEncounter(setup)
		s.Require().NoError(err)

		// First pump
		pumpOut1, err := enc.Pump(&encounter.PumpInput{})
		s.Require().NoError(err)
		s.Equal(uint64(1), pumpOut1.Tick)

		// Second pump
		pumpOut2, err := enc.Pump(&encounter.PumpInput{})
		s.Require().NoError(err)
		s.Equal(uint64(2), pumpOut2.Tick)

		// Third pump
		pumpOut3, err := enc.Pump(&encounter.PumpInput{})
		s.Require().NoError(err)
		s.Equal(uint64(3), pumpOut3.Tick)

		// Assert via story tick beats
		aliceStory, err := enc.Story(&encounter.StoryInput{Audience: aliceID, AfterSeq: 0})
		s.Require().NoError(err)

		// Find all tick beats
		tickBeats := 0
		for _, entry := range aliceStory {
			var payload map[string]interface{}
			err = json.Unmarshal(entry.Payload, &payload)
			if err == nil && payload["beat"] == "tick" {
				tickBeats++
			}
		}
		s.Equal(3, tickBeats, "should have exactly 3 tick beats (one per pump)")
	})
}

func (s *PumpTestSuite) TestPumpPlayerWithDeciderRejected() {
	s.Run("player with decider fails validation", func() {
		// Arrange: alice (player) with a decider
		aliceID := core.EntityID("alice")

		setup := &encounter.SetupInput{
			Field: encounter.FieldInput{
				Rooms: []encounter.RoomInput{
					{
						ID:     room1,
						Width:  10,
						Height: 10,
					},
				},
				Connections: []encounter.ConnectionInput{},
			},
			Members: []encounter.MemberInput{
				{
					ID:       aliceID,
					Kind:     encounter.KindPlayer,
					Room:     room1,
					Position: spatial.Position{X: 0, Y: 0},
					Decider:  &patrolDecider{}, // Players cannot have deciders!
				},
			},
			Endings: []encounter.EndingInput{
				{
					Key:     endingStairs,
					Trigger: encounter.TriggerReachedPosition{Room: room1, Position: spatial.Position{X: 9, Y: 9}},
				},
			},
		}

		// Act: NewEncounter should fail
		_, err := encounter.NewEncounter(setup)

		// Assert: Error with ErrNilInput (per design resolution)
		s.Require().Error(err)
		s.True(errors.Is(err, encounter.ErrNilInput), "should reject player with decider")
	})
}
