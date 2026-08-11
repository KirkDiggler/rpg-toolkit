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

func (p *patrolDecider) Decide(_ encounter.Snapshot) (encounter.Intent, error) {
	defer func() { p.callCount++ }()

	if len(p.positions) == 0 {
		return encounter.IntentHold{}, nil
	}

	// Deterministic patrol: alternate between positions
	target := p.positions[p.callCount%len(p.positions)]
	return encounter.IntentMoveTo{To: target}, nil
}

// spyDecider records the Snapshot it was shown and returns a hold decision.
type spyDecider struct {
	capturedView []intel.Holding
	capturedSnap encounter.Snapshot
}

func (s *spyDecider) Decide(snap encounter.Snapshot) (encounter.Intent, error) {
	// Capture the view (deep copy to persist it)
	s.capturedView = make([]intel.Holding, len(snap.Holdings))
	copy(s.capturedView, snap.Holdings)
	s.capturedSnap = snap
	return encounter.IntentHold{}, nil
}

// errorDecider returns an error when asked to decide.
// failOnceDecider errors on its first call, holds forever after —
// lets a test prove a failed pump advanced nothing by succeeding next.
type failOnceDecider struct {
	calls int
}

func (f *failOnceDecider) Decide(_ encounter.Snapshot) (encounter.Intent, error) {
	f.calls++
	if f.calls == 1 {
		return nil, errors.New("first call fails")
	}
	return encounter.IntentHold{}, nil
}

// vandalDecider mutates every byte of its view then holds.
type vandalDecider struct {
	sawAlice bool
}

func (v *vandalDecider) Decide(snap encounter.Snapshot) (encounter.Intent, error) {
	for i := range snap.Holdings {
		if snap.Holdings[i].Subject == "alice" {
			v.sawAlice = true
		}
		for j := range snap.Holdings[i].Payload {
			snap.Holdings[i].Payload[j] = 'X'
		}
	}
	return encounter.IntentHold{}, nil
}

type errorDecider struct {
	err error
}

func (e *errorDecider) Decide(_ encounter.Snapshot) (encounter.Intent, error) {
	return nil, e.err
}

// snapshotSpyDecider records every Snapshot it's given, across calls —
// used to pin that Pump hands each monster its OWN room/position, never
// another member's (Task 3, item 2).
type snapshotSpyDecider struct {
	snapshots []encounter.Snapshot
}

func (s *snapshotSpyDecider) Decide(snap encounter.Snapshot) (encounter.Intent, error) {
	s.snapshots = append(s.snapshots, snap)
	return encounter.IntentHold{}, nil
}

// onceTraverseDecider intends IntentTraverse{Connection} on its first
// call, then holds forever after — a minimal fixture for pinning a
// single traverse attempt's success or failure.
type onceTraverseDecider struct {
	connection string
	called     bool
}

func (t *onceTraverseDecider) Decide(_ encounter.Snapshot) (encounter.Intent, error) {
	if t.called {
		return encounter.IntentHold{}, nil
	}
	t.called = true
	return encounter.IntentTraverse{Connection: t.connection}, nil
}

// pursuitDecider is constructed with the field's static connection
// topology and a target subject to chase — it never reads encounter
// state directly; Snapshot + Holdings + its own construction-time config
// is all it gets (C2 extended to placement, not just sight). Logic: if it
// currently holds a percept of the target, and it is standing EXACTLY
// where that percept places them (its own room and position match), that
// cell is either a connection endpoint in this room — traverse through
// it — or nowhere useful — hold. Otherwise, if the percept's room matches
// its own current room, walk toward the held position. A percept in a
// DIFFERENT room is out of this simple pursuer's reach (it does not
// cross-room-pathfind); it holds until circumstances change.
type pursuitDecider struct {
	connections []encounter.ConnectionInput
	target      core.EntityID
}

func (p *pursuitDecider) Decide(snap encounter.Snapshot) (encounter.Intent, error) {
	for _, h := range snap.Holdings {
		if h.Subject != intel.Subject(p.target) {
			continue
		}
		var seen encounter.SightPayload
		if err := json.Unmarshal(h.Payload, &seen); err != nil {
			return nil, err
		}

		if snap.Room != seen.Room {
			return encounter.IntentHold{}, nil
		}

		if snap.Position.X == seen.X && snap.Position.Y == seen.Y {
			for _, c := range p.connections {
				if c.From == snap.Room && c.FromPosition.X == snap.Position.X && c.FromPosition.Y == snap.Position.Y {
					return encounter.IntentTraverse{Connection: c.ID}, nil
				}
				if c.To == snap.Room && c.ToPosition.X == snap.Position.X && c.ToPosition.Y == snap.Position.Y {
					return encounter.IntentTraverse{Connection: c.ID}, nil
				}
			}
			return encounter.IntentHold{}, nil
		}

		return encounter.IntentMoveTo{To: spatial.Position{X: seen.X, Y: seen.Y}}, nil
	}
	return encounter.IntentHold{}, nil
}

// TestPumpDoesNotConsultExitedMonsterDecider pins Exit's decider
// cleanup: an exited monster's decider must never be consulted again.
func (s *PumpTestSuite) TestPumpDoesNotConsultExitedMonsterDecider() {
	enc, err := encounter.NewEncounter(&encounter.SetupInput{
		Field: encounter.FieldInput{Rooms: []encounter.RoomInput{{ID: room1, Width: 10, Height: 10}}},
		Members: []encounter.MemberInput{
			{ID: core.EntityID("alice"), Kind: encounter.KindPlayer, Room: room1, Position: spatial.Position{X: 1, Y: 1}},
			{ID: core.EntityID("goblin"), Kind: encounter.KindMonster, Room: room1,
				Position: spatial.Position{X: 4, Y: 4}, Decider: &errorDecider{err: errors.New("must never be called")}},
		},
		Endings: []encounter.EndingInput{{Key: endingStairs,
			Trigger: encounter.TriggerReachedPosition{Room: room1, Position: spatial.Position{X: 9, Y: 9}}}},
	})
	s.Require().NoError(err)
	_, err = enc.Exit(&encounter.ExitInput{Member: core.EntityID("goblin")})
	s.Require().NoError(err)
	_, err = enc.Pump(&encounter.PumpInput{})
	s.Require().NoError(err, "an exited monster's decider must not be consulted (no leak)")
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

		// Assert: still open, and no record entries added (R5)
		status, err := enc.Status()
		s.Require().NoError(err)
		s.True(status.Open, "encounter should still be open")
		afterStory, err := enc.Story(&encounter.StoryInput{Audience: aliceID, AfterSeq: 0})
		s.Require().NoError(err)
		s.Equal(initialLen, len(afterStory), "story should not have new entries after failed pump")

		// Assert: alice's holding of the goblin is unchanged (it never moved)
		aliceView, err := enc.View(&encounter.ViewInput{Member: aliceID})
		s.Require().NoError(err)
		s.Require().Len(aliceView, 1, "alice must still hold the goblin")
		var payload encounter.SightPayload
		s.Require().NoError(json.Unmarshal(aliceView[0].Payload, &payload))
		s.Equal(5.0, payload.X, "goblin must not have moved on a failed pump (R5)")
	})
}

// TestPumpFailedPumpAdvancesNothing pins the tick side of R5 with a
// fail-once decider: the failed pump must not consume a tick, so the
// NEXT (successful) pump reports reading 1, not 2.
func (s *PumpTestSuite) TestPumpFailedPumpAdvancesNothing() {
	enc, err := encounter.NewEncounter(&encounter.SetupInput{
		Field: encounter.FieldInput{Rooms: []encounter.RoomInput{{ID: room1, Width: 10, Height: 10}}},
		Members: []encounter.MemberInput{
			{ID: core.EntityID("alice"), Kind: encounter.KindPlayer, Room: room1, Position: spatial.Position{X: 1, Y: 1}},
			{ID: core.EntityID("goblin"), Kind: encounter.KindMonster, Room: room1,
				Position: spatial.Position{X: 4, Y: 4}, Decider: &failOnceDecider{}},
		},
		Endings: []encounter.EndingInput{{Key: endingStairs,
			Trigger: encounter.TriggerReachedPosition{Room: room1, Position: spatial.Position{X: 9, Y: 9}}}},
	})
	s.Require().NoError(err)

	_, err = enc.Pump(&encounter.PumpInput{})
	s.Require().Error(err, "first pump fails via the decider")

	out, err := enc.Pump(&encounter.PumpInput{})
	s.Require().NoError(err, "second pump succeeds")
	s.Equal(uint64(1), out.Tick, "the failed pump must not have consumed a tick (R5)")
}

// TestPumpPartialAbort pins decide-then-execute: when a LATER monster's
// decider errors, an EARLIER monster that would have moved must not
// have moved — no partial mutation (R5).
func (s *PumpTestSuite) TestPumpPartialAbort() {
	enc, err := encounter.NewEncounter(&encounter.SetupInput{
		Field: encounter.FieldInput{Rooms: []encounter.RoomInput{{ID: room1, Width: 10, Height: 10}}},
		Members: []encounter.MemberInput{
			{ID: core.EntityID("alice"), Kind: encounter.KindPlayer, Room: room1, Position: spatial.Position{X: 1, Y: 1}},
			// "aaa-goblin" sorts before "zzz-goblin": it decides (and
			// would move) before the erroring decider is consulted.
			{ID: core.EntityID("aaa-goblin"), Kind: encounter.KindMonster, Room: room1,
				Position: spatial.Position{X: 4, Y: 4}, Decider: &patrolDecider{positions: []spatial.Position{{X: 5, Y: 5}, {X: 6, Y: 6}}}},
			{ID: core.EntityID("zzz-goblin"), Kind: encounter.KindMonster, Room: room1,
				Position: spatial.Position{X: 7, Y: 7}, Decider: &failOnceDecider{}},
		},
		Endings: []encounter.EndingInput{{Key: endingStairs,
			Trigger: encounter.TriggerReachedPosition{Room: room1, Position: spatial.Position{X: 9, Y: 9}}}},
	})
	s.Require().NoError(err)

	_, err = enc.Pump(&encounter.PumpInput{})
	s.Require().Error(err)

	// The phantom half-move is invisible to Views (no refreshSight ran on
	// the aborted pump) and members don't block movement — but it IS
	// visible in where the goblin truly stands next: the following
	// successful pump's move delta must depart from the ORIGINAL
	// position, not from a phantom one.
	out, err := enc.Pump(&encounter.PumpInput{})
	s.Require().NoError(err, "second pump succeeds (fail-once decider)")
	s.Require().NotEmpty(out.MonsterMoves)
	var found bool
	for _, mm := range out.MonsterMoves {
		if mm.Member == core.EntityID("aaa-goblin") {
			found = true
			s.Equal(4.0, mm.From.X,
				"aaa-goblin departs from (4,4): a later decider error aborted the whole first pump (R5)")
		}
	}
	s.Require().True(found, "aaa-goblin moved on the second pump")
}

// TestPumpMutatingDeciderCannotCorrupt is the composed-system aliasing
// pin: a decider that scribbles on its view must not corrupt encounter
// state (the protection is intel.HeldBy's documented copy-out — pinned
// in play/intel; this test pins the composed guarantee end to end).
func (s *PumpTestSuite) TestPumpMutatingDeciderCannotCorrupt() {
	vandal := &vandalDecider{}
	enc, err := encounter.NewEncounter(&encounter.SetupInput{
		Field: encounter.FieldInput{Rooms: []encounter.RoomInput{{ID: room1, Width: 10, Height: 10}}},
		Members: []encounter.MemberInput{
			{ID: core.EntityID("alice"), Kind: encounter.KindPlayer, Room: room1, Position: spatial.Position{X: 1, Y: 1}},
			{ID: core.EntityID("goblin"), Kind: encounter.KindMonster, Room: room1,
				Position: spatial.Position{X: 4, Y: 4}, Decider: vandal},
		},
		Endings: []encounter.EndingInput{{Key: endingStairs,
			Trigger: encounter.TriggerReachedPosition{Room: room1, Position: spatial.Position{X: 9, Y: 9}}}},
	})
	s.Require().NoError(err)

	_, err = enc.Pump(&encounter.PumpInput{})
	s.Require().NoError(err)
	s.Require().True(vandal.sawAlice, "precondition: the vandal saw and scribbled on a holding")

	goblinView, err := enc.View(&encounter.ViewInput{Member: core.EntityID("goblin")})
	s.Require().NoError(err)
	s.Require().Len(goblinView, 1)
	var p encounter.SightPayload
	s.Require().NoError(json.Unmarshal(goblinView[0].Payload, &p))
	s.Equal(room1, p.Room, "the vandal's scribbles must not reach encounter state")
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
		s.Require().False(status.Open,
			"the goblin's filtered move is deterministic and must have closed the encounter")

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

		// Assert: a player carrying a decider is a member-shaped defect
		s.Require().Error(err)
		s.True(errors.Is(err, encounter.ErrNoMember), "player-with-decider wraps ErrNoMember")
	})
}

// twoRoomDoor is the standard fixture connection for the Pump/traverse
// tests below: room-a to room-b, DELIBERATELY asymmetric endpoints (T1
// review lesson) so a from/to mix-up would be observable.
var twoRoomDoor = encounter.ConnectionInput{
	ID: "door1", From: "room-a", To: "room-b",
	FromPosition: spatial.Position{X: 9, Y: 5},
	ToPosition:   spatial.Position{X: 0, Y: 5},
}

// TestPumpSnapshotIsOwnPlacement pins that each monster's Snapshot
// carries ITS OWN room/position — never another member's (Task 3, item
// 2). Two monsters in DIFFERENT rooms each get a spy decider; if Pump
// ever handed either monster the wrong placement, at least one spy's
// captured snapshot would name a room/position neither monster stands in.
func (s *PumpTestSuite) TestPumpSnapshotIsOwnPlacement() {
	spyA := &snapshotSpyDecider{}
	spyB := &snapshotSpyDecider{}

	enc, err := encounter.NewEncounter(&encounter.SetupInput{
		Field: encounter.FieldInput{
			Rooms: []encounter.RoomInput{
				{ID: "room-a", Width: 10, Height: 10},
				{ID: "room-b", Width: 10, Height: 10},
			},
		},
		Members: []encounter.MemberInput{
			{ID: core.EntityID("goblin-a"), Kind: encounter.KindMonster, Room: "room-a",
				Position: spatial.Position{X: 2, Y: 3}, Decider: spyA},
			{ID: core.EntityID("goblin-b"), Kind: encounter.KindMonster, Room: "room-b",
				Position: spatial.Position{X: 7, Y: 8}, Decider: spyB},
		},
		Endings: []encounter.EndingInput{{Key: "done", Trigger: encounter.TriggerExternal{}}},
	})
	s.Require().NoError(err)

	_, err = enc.Pump(&encounter.PumpInput{})
	s.Require().NoError(err)

	s.Require().Len(spyA.snapshots, 1)
	s.Equal("room-a", spyA.snapshots[0].Room, "goblin-a's snapshot must name ITS OWN room")
	s.Equal(spatial.Position{X: 2, Y: 3}, spyA.snapshots[0].Position, "goblin-a's snapshot must be ITS OWN position")

	s.Require().Len(spyB.snapshots, 1)
	s.Equal("room-b", spyB.snapshots[0].Room, "goblin-b's snapshot must name ITS OWN room")
	s.Equal(spatial.Position{X: 7, Y: 8}, spyB.snapshots[0].Position, "goblin-b's snapshot must be ITS OWN position")
}

// TestPumpIntentTraverseSuccess pins that a monster standing at a
// connection's threshold, deciding IntentTraverse, actually crosses:
// room changes, the "traversed" beat is recorded, PumpOutput.MonsterTraverses
// reflects it (and MonsterMoves does not), and the clock still advances
// by exactly 1 — Pump's own tick is unconditional regardless of what a
// monster does within it (traversal itself is not time, law T4, but the
// PUMP is).
func (s *PumpTestSuite) TestPumpIntentTraverseSuccess() {
	goblinID := core.EntityID("goblin")
	decider := &onceTraverseDecider{connection: "door1"}

	enc, err := encounter.NewEncounter(&encounter.SetupInput{
		Field: encounter.FieldInput{
			Rooms: []encounter.RoomInput{
				{ID: "room-a", Width: 10, Height: 10},
				{ID: "room-b", Width: 10, Height: 10},
			},
			Connections: []encounter.ConnectionInput{twoRoomDoor},
		},
		Members: []encounter.MemberInput{
			{ID: goblinID, Kind: encounter.KindMonster, Room: "room-a",
				Position: spatial.Position{X: 9, Y: 5}, Decider: decider},
		},
		Endings: []encounter.EndingInput{{Key: "done", Trigger: encounter.TriggerExternal{}}},
	})
	s.Require().NoError(err)

	out, err := enc.Pump(&encounter.PumpInput{})
	s.Require().NoError(err)
	s.Equal(uint64(1), out.Tick)
	s.Require().Len(out.MonsterTraverses, 1)
	s.Equal(goblinID, out.MonsterTraverses[0].Member)
	s.Equal("room-a", out.MonsterTraverses[0].FromRoom)
	s.Equal(spatial.Position{X: 9, Y: 5}, out.MonsterTraverses[0].From)
	s.Equal("room-b", out.MonsterTraverses[0].ToRoom)
	s.Equal(spatial.Position{X: 0, Y: 5}, out.MonsterTraverses[0].To)
	s.Empty(out.MonsterMoves, "this is a traverse, not a move")

	members, err := enc.Members()
	s.Require().NoError(err)
	s.Require().Len(members, 1)
	s.Equal("room-b", members[0].Room)
}

// TestPumpIntentTraverseIllegalDoesNotAbort pins that an illegal traverse
// intent (decider names a REAL connection but the monster isn't AT its
// threshold) follows the SAME silent-skip contract Pump already
// established for a spatially-rejected IntentMoveTo (traverseMember's own
// doc comment): the pump does NOT abort — the clock still advances, the
// tick beat is still recorded, refreshSight still runs — but no
// "traversed" beat is recorded and the monster's room/position are
// unchanged.
func (s *PumpTestSuite) TestPumpIntentTraverseIllegalDoesNotAbort() {
	goblinID := core.EntityID("goblin")
	decider := &onceTraverseDecider{connection: "door1"}

	enc, err := encounter.NewEncounter(&encounter.SetupInput{
		Field: encounter.FieldInput{
			Rooms: []encounter.RoomInput{
				{ID: "room-a", Width: 10, Height: 10},
				{ID: "room-b", Width: 10, Height: 10},
			},
			Connections: []encounter.ConnectionInput{twoRoomDoor},
		},
		Members: []encounter.MemberInput{
			// goblin is NOT at the threshold (the threshold is (9,5)).
			{ID: goblinID, Kind: encounter.KindMonster, Room: "room-a",
				Position: spatial.Position{X: 3, Y: 3}, Decider: decider},
		},
		Endings: []encounter.EndingInput{{Key: "done", Trigger: encounter.TriggerExternal{}}},
	})
	s.Require().NoError(err)

	beforeClock := enc.ToData().Clock.HighWater

	out, err := enc.Pump(&encounter.PumpInput{})
	s.Require().NoError(err, "an illegal traverse intent must not abort the pump")

	afterClock := enc.ToData().Clock.HighWater
	s.Equal(beforeClock+1, afterClock, "clock still advances — matches IntentMoveTo's spatial-rejection contract")
	s.Equal(uint64(1), out.Tick)
	s.Empty(out.MonsterTraverses, "the illegal traverse must not appear as successful")
	s.Empty(out.MonsterMoves)
	s.Len(out.Seqs, 1, "exactly the tick beat — no traversed beat for the failure")

	story, err := enc.Story(&encounter.StoryInput{Audience: goblinID, AfterSeq: 0})
	s.Require().NoError(err)
	for _, entry := range story {
		var beat map[string]any
		s.Require().NoError(json.Unmarshal(entry.Payload, &beat))
		s.NotEqual("traversed", beat["beat"], "no traversed beat for a failed traverse")
	}

	data := enc.ToData()
	s.Require().Len(data.Members, 1)
	s.Equal("room-a", data.Members[0].Room, "the monster never left its room")
	s.Equal(3.0, data.Members[0].Position.X, "the monster's position is unchanged")
	s.Equal(3.0, data.Members[0].Position.Y)
}

// TestPumpIntentTraverseUnknownConnectionDoesNotAbort pins that an
// IntentTraverse naming an unknown connection (a decider bug) follows the
// SAME silent-skip contract as an illegal-position traverse:
// traverseMember's ErrNoConnection is treated identically to its
// ErrBadPlacement by Pump's phase-2 executor — no abort, no beat, no
// position change.
func (s *PumpTestSuite) TestPumpIntentTraverseUnknownConnectionDoesNotAbort() {
	goblinID := core.EntityID("goblin")
	decider := &onceTraverseDecider{connection: "no-such-door"}

	enc, err := encounter.NewEncounter(&encounter.SetupInput{
		Field: encounter.FieldInput{
			Rooms: []encounter.RoomInput{
				{ID: "room-a", Width: 10, Height: 10},
				{ID: "room-b", Width: 10, Height: 10},
			},
			Connections: []encounter.ConnectionInput{twoRoomDoor},
		},
		Members: []encounter.MemberInput{
			{ID: goblinID, Kind: encounter.KindMonster, Room: "room-a",
				Position: spatial.Position{X: 9, Y: 5}, Decider: decider}, // AT the real door's threshold, but names a fake one
		},
		Endings: []encounter.EndingInput{{Key: "done", Trigger: encounter.TriggerExternal{}}},
	})
	s.Require().NoError(err)

	beforeClock := enc.ToData().Clock.HighWater

	out, err := enc.Pump(&encounter.PumpInput{})
	s.Require().NoError(err, "an unknown-connection traverse intent must not abort the pump")

	afterClock := enc.ToData().Clock.HighWater
	s.Equal(beforeClock+1, afterClock, "clock still advances")
	s.Empty(out.MonsterTraverses)
	s.Len(out.Seqs, 1, "exactly the tick beat — no traversed beat for the failure")

	story, err := enc.Story(&encounter.StoryInput{Audience: goblinID, AfterSeq: 0})
	s.Require().NoError(err)
	for _, entry := range story {
		var beat map[string]any
		s.Require().NoError(json.Unmarshal(entry.Payload, &beat))
		s.NotEqual("traversed", beat["beat"], "no traversed beat for a failed traverse")
	}

	data := enc.ToData()
	s.Require().Len(data.Members, 1)
	s.Equal("room-a", data.Members[0].Room, "the monster never left its room")
	s.Equal(9.0, data.Members[0].Position.X, "the monster's position is unchanged")
	s.Equal(5.0, data.Members[0].Position.Y)
}

// TestPumpDeciderErrorAbortsEvenWithTraversableTopology extends the
// existing decider-error-aborts law (TestPumpDeciderErrorAborts) to a
// pump where a traverse WOULD have been legal — proving the abort
// contract doesn't depend on which Intent kind the decider would have
// returned. PHASE 1's decider-error check runs before ANY intent is even
// inspected, so this must abort exactly like the single-room case.
func (s *PumpTestSuite) TestPumpDeciderErrorAbortsEvenWithTraversableTopology() {
	goblinID := core.EntityID("goblin")
	deciderErr := errors.New("test decider error")

	enc, err := encounter.NewEncounter(&encounter.SetupInput{
		Field: encounter.FieldInput{
			Rooms: []encounter.RoomInput{
				{ID: "room-a", Width: 10, Height: 10},
				{ID: "room-b", Width: 10, Height: 10},
			},
			Connections: []encounter.ConnectionInput{twoRoomDoor},
		},
		Members: []encounter.MemberInput{
			// goblin IS at the threshold — a traverse WOULD be legal, but
			// the decider errors before ever returning an Intent.
			{ID: goblinID, Kind: encounter.KindMonster, Room: "room-a",
				Position: spatial.Position{X: 9, Y: 5}, Decider: &errorDecider{err: deciderErr}},
		},
		Endings: []encounter.EndingInput{{Key: "done", Trigger: encounter.TriggerExternal{}}},
	})
	s.Require().NoError(err)

	beforeClock := enc.ToData().Clock.HighWater
	_, err = enc.Pump(&encounter.PumpInput{})
	s.Require().Error(err, "pump should fail with decider error")
	afterClock := enc.ToData().Clock.HighWater
	s.Equal(beforeClock, afterClock, "R5: no clock advance on a decider error, even with legal traverse topology available")

	data := enc.ToData()
	s.Require().Len(data.Members, 1)
	s.Equal("room-a", data.Members[0].Room, "the monster never moved — R5 atomicity")
}

// TestPumpPursuitAcrossConnection is the wave's integration pin: a
// pursuit decider, constructed with ONLY the field's static topology and
// a target ID — never reading encounter state directly; Snapshot +
// Holdings + its own construction-time config is all it gets (C2) —
// chases a player through a doorway. The player starts AT the threshold,
// the monster sees them; the player traverses away; the monster's ghost
// of the player holds the LAST-SEEN position — the threshold, in the OLD
// room; the monster walks to it, then — standing exactly on the
// connection endpoint — traverses through the SAME connection, arriving
// in the player's new room, and holds them Current again.
func (s *PumpTestSuite) TestPumpPursuitAcrossConnection() {
	aliceID := core.EntityID("alice")
	goblinID := core.EntityID("goblin")

	enc, err := encounter.NewEncounter(&encounter.SetupInput{
		Field: encounter.FieldInput{
			Rooms: []encounter.RoomInput{
				{ID: "room-a", Width: 10, Height: 10},
				{ID: "room-b", Width: 10, Height: 10},
			},
			Connections: []encounter.ConnectionInput{twoRoomDoor},
		},
		Members: []encounter.MemberInput{
			{ID: aliceID, Kind: encounter.KindPlayer, Room: "room-a", Position: spatial.Position{X: 9, Y: 5}},
			{ID: goblinID, Kind: encounter.KindMonster, Room: "room-a", Position: spatial.Position{X: 8, Y: 5},
				Decider: &pursuitDecider{connections: []encounter.ConnectionInput{twoRoomDoor}, target: aliceID}},
		},
		Endings: []encounter.EndingInput{{Key: "done", Trigger: encounter.TriggerExternal{}}},
	})
	s.Require().NoError(err)

	// Precondition: goblin sees alice Current (adjacent, no occluders).
	goblinView, err := enc.View(&encounter.ViewInput{Member: goblinID})
	s.Require().NoError(err)
	s.Require().Len(goblinView, 1)
	s.Equal(intel.Current, goblinView[0].Status, "precondition: goblin must see alice before she leaves")

	// Stage 1: alice traverses away, then repositions within room-b so
	// the monster's eventual arrival isn't a trivial same-cell coincidence.
	_, err = enc.Traverse(&encounter.TraverseInput{Member: aliceID, Connection: "door1"})
	s.Require().NoError(err)
	_, err = enc.Move(&encounter.MoveInput{Member: aliceID, To: spatial.Position{X: 3, Y: 5}})
	s.Require().NoError(err)

	goblinView, err = enc.View(&encounter.ViewInput{Member: goblinID})
	s.Require().NoError(err)
	s.Require().Len(goblinView, 1, "the ghost is HELD, not gone")
	s.Equal(intel.Held, goblinView[0].Status, "alice left the room — goblin's sight of her fades")
	var ghostSeen encounter.SightPayload
	s.Require().NoError(json.Unmarshal(goblinView[0].Payload, &ghostSeen))
	s.Equal("room-a", ghostSeen.Room, "the ghost holds alice's LAST-SEEN room")
	s.Equal(9.0, ghostSeen.X, "the ghost holds alice at the THRESHOLD, her last-seen position")
	s.Equal(5.0, ghostSeen.Y)

	// Stage 2: pump — goblin walks toward the ghost's last-seen position.
	out1, err := enc.Pump(&encounter.PumpInput{})
	s.Require().NoError(err)
	s.Require().Len(out1.MonsterMoves, 1)
	s.Equal(spatial.Position{X: 9, Y: 5}, out1.MonsterMoves[0].To, "goblin walks to the threshold")
	s.Empty(out1.MonsterTraverses, "not standing at the threshold at the time this tick decided")

	// Stage 3: pump — goblin, now AT the threshold, traverses.
	out2, err := enc.Pump(&encounter.PumpInput{})
	s.Require().NoError(err)
	s.Require().Len(out2.MonsterTraverses, 1)
	s.Equal(goblinID, out2.MonsterTraverses[0].Member)
	s.Equal("room-a", out2.MonsterTraverses[0].FromRoom)
	s.Equal("room-b", out2.MonsterTraverses[0].ToRoom)
	s.Equal(spatial.Position{X: 0, Y: 5}, out2.MonsterTraverses[0].To)

	// The monster arrives in alice's room and, since THIS pump's own
	// refreshSight ran AFTER the traverse, already holds her Current.
	goblinView, err = enc.View(&encounter.ViewInput{Member: goblinID})
	s.Require().NoError(err)
	s.Require().Len(goblinView, 1)
	s.Equal(intel.Current, goblinView[0].Status, "the monster holds alice Current again, having crossed the threshold")
	var aliceSeen encounter.SightPayload
	s.Require().NoError(json.Unmarshal(goblinView[0].Payload, &aliceSeen))
	s.Equal("room-b", aliceSeen.Room)
	s.Equal(3.0, aliceSeen.X)
	s.Equal(5.0, aliceSeen.Y)

	// Also verify via Story that the chase left the expected trail.
	story, err := enc.Story(&encounter.StoryInput{Audience: goblinID, AfterSeq: 0})
	s.Require().NoError(err)
	var sawTraversed bool
	for _, entry := range story {
		var beat map[string]any
		s.Require().NoError(json.Unmarshal(entry.Payload, &beat))
		if beat["beat"] == "traversed" && beat["member"] == string(goblinID) {
			sawTraversed = true
		}
	}
	s.True(sawTraversed, "the goblin's traversal must appear in the Story")
}

// TestPumpFullTickThenEvaluateAcrossTraverse pins full-tick-then-evaluate
// as law, now that IntentTraverse raises its stakes (a fired ending
// during phase-2 execution would otherwise need to REVERT a room
// mutation, not just a same-room position). Two monsters decide in the
// SAME pump: "aaa-goblin" (decides first — Members() stable order sorts
// its ID before "zzz-goblin") traverses onto a FILTERED ending naming
// it; "zzz-goblin" has an unrelated same-room move queued. The shipped
// contract (wave-1 loop skeleton, deliberate, reviewer-verified): phase
// 2 executes BOTH actions and records BOTH beats BEFORE any ending
// evaluation; evaluation walks executed in decision order, the first
// match sets the outcome and stops evaluating further endings, but never
// reverts an already-applied action; Outcome.Members reflects the FULL
// post-tick state, including zzz-goblin at its NEW position; the next
// Pump returns ErrClosed.
func (s *PumpTestSuite) TestPumpFullTickThenEvaluateAcrossTraverse() {
	aID := core.EntityID("aaa-goblin")
	bID := core.EntityID("zzz-goblin")

	enc, err := encounter.NewEncounter(&encounter.SetupInput{
		Field: encounter.FieldInput{
			Rooms: []encounter.RoomInput{
				{ID: "room-a", Width: 10, Height: 10},
				{ID: "room-b", Width: 10, Height: 10},
			},
			Connections: []encounter.ConnectionInput{twoRoomDoor},
		},
		Members: []encounter.MemberInput{
			// aaa-goblin is AT the threshold, ready to traverse onto the
			// filtered ending below.
			{ID: aID, Kind: encounter.KindMonster, Room: "room-a",
				Position: spatial.Position{X: 9, Y: 5}, Decider: &onceTraverseDecider{connection: "door1"}},
			// zzz-goblin has an unrelated same-room move queued.
			{ID: bID, Kind: encounter.KindMonster, Room: "room-a",
				Position: spatial.Position{X: 3, Y: 3}, Decider: &patrolDecider{positions: []spatial.Position{{X: 4, Y: 4}}}},
		},
		Endings: []encounter.EndingInput{
			{Key: "escaped", Trigger: encounter.TriggerReachedPosition{
				Room: "room-b", Position: spatial.Position{X: 0, Y: 5}, Member: aID}},
		},
	})
	s.Require().NoError(err)

	out, err := enc.Pump(&encounter.PumpInput{})
	s.Require().NoError(err)

	// Both actions executed and both beats recorded — the closing ending
	// (fired by aaa-goblin's traverse) did not suppress zzz-goblin's move.
	s.Require().Len(out.MonsterTraverses, 1)
	s.Equal(aID, out.MonsterTraverses[0].Member)
	s.Require().Len(out.MonsterMoves, 1)
	s.Equal(bID, out.MonsterMoves[0].Member)
	s.Equal(spatial.Position{X: 4, Y: 4}, out.MonsterMoves[0].To)

	s.Require().NotNil(out.Outcome, "aaa-goblin's traverse must fire the filtered ending")
	s.Equal("escaped", out.Outcome.Ending)
	s.Require().Len(out.Outcome.Members, 2)

	var aOutcome, bOutcome encounter.MemberOutcome
	for _, m := range out.Outcome.Members {
		switch m.ID {
		case aID:
			aOutcome = m
		case bID:
			bOutcome = m
		}
	}
	s.Equal("room-b", aOutcome.Room)
	s.Equal(spatial.Position{X: 0, Y: 5}, aOutcome.Position)
	// The full-tick-then-evaluate law, made observable: zzz-goblin's
	// ALREADY-APPLIED move is reflected in the outcome — its NEW
	// position, not its pre-pump one. A revert-on-close implementation
	// would show (3,3) here instead.
	s.Equal("room-a", bOutcome.Room)
	s.Equal(spatial.Position{X: 4, Y: 4}, bOutcome.Position,
		"the outcome must reflect zzz-goblin's post-tick position, not its pre-tick one")

	// Both beats actually landed in the Story (not just the output structs).
	story, err := enc.Story(&encounter.StoryInput{Audience: aID, AfterSeq: 0})
	s.Require().NoError(err)
	var sawTraversed, sawMoved bool
	for _, entry := range story {
		var beat map[string]any
		s.Require().NoError(json.Unmarshal(entry.Payload, &beat))
		switch beat["beat"] {
		case "traversed":
			sawTraversed = true
		case "moved":
			sawMoved = true
		}
	}
	s.True(sawTraversed, "aaa-goblin's traverse beat must be recorded")
	s.True(sawMoved, "zzz-goblin's move beat must be recorded despite the mid-tick closure")

	// The encounter is closed: the next pump returns ErrClosed.
	_, err = enc.Pump(&encounter.PumpInput{})
	s.Require().ErrorIs(err, encounter.ErrClosed)
}

// TestPumpEndingEvaluationIsDecisionOrderNotDeclarationOrder pins the
// honest law: ending evaluation walks EXECUTED ACTIONS in decision
// order (C8's per-monster ordering), and declaration order is only a
// tiebreak WITHIN one action's own scan of e.endings — not a global
// "first-declared-wins" rule. The control lands each monster on its
// "natural" declared-order slot, where decision order and declaration
// order happen to agree — easy to misread as declaration order
// driving the result. The cross scenario keeps decision order fixed
// but swaps which ending each monster's landing position fires: the
// SECOND-declared ending wins because it belongs to the FIRST-deciding
// monster, disproving the declaration-order reading.
func (s *PumpTestSuite) TestPumpEndingEvaluationIsDecisionOrderNotDeclarationOrder() {
	aID := core.EntityID("aaa-goblin") // decides first (Members() sorts by ID)
	bID := core.EntityID("zzz-goblin") // decides second
	posA := spatial.Position{X: 2, Y: 2}
	posB := spatial.Position{X: 8, Y: 8}

	s.Run("control: decision order and declaration order agree", func() {
		enc, err := encounter.NewEncounter(&encounter.SetupInput{
			Field: encounter.FieldInput{Rooms: []encounter.RoomInput{{ID: room1, Width: 10, Height: 10}}},
			Members: []encounter.MemberInput{
				{ID: aID, Kind: encounter.KindMonster, Room: room1,
					Position: spatial.Position{X: 5, Y: 5}, Decider: &patrolDecider{positions: []spatial.Position{posA}}},
				{ID: bID, Kind: encounter.KindMonster, Room: room1,
					Position: spatial.Position{X: 6, Y: 6}, Decider: &patrolDecider{positions: []spatial.Position{posB}}},
			},
			Endings: []encounter.EndingInput{
				// Declared first: fires for aaa-goblin, who decides first.
				{Key: "first", Trigger: encounter.TriggerReachedPosition{Room: room1, Position: posA, Member: aID}},
				// Declared second: fires for zzz-goblin, who decides second.
				{Key: "second", Trigger: encounter.TriggerReachedPosition{Room: room1, Position: posB, Member: bID}},
			},
		})
		s.Require().NoError(err)

		out, err := enc.Pump(&encounter.PumpInput{})
		s.Require().NoError(err)
		s.Require().NotNil(out.Outcome)
		s.Equal("first", out.Outcome.Ending, "aaa-goblin decides first and lands on the first-declared ending")
	})

	s.Run("cross: decision order dominates — the SECOND-declared ending wins", func() {
		enc, err := encounter.NewEncounter(&encounter.SetupInput{
			Field: encounter.FieldInput{Rooms: []encounter.RoomInput{{ID: room1, Width: 10, Height: 10}}},
			Members: []encounter.MemberInput{
				// Same decision order (aaa first, zzz second), but now each
				// monster's landing position fires the OTHER declared ending.
				{ID: aID, Kind: encounter.KindMonster, Room: room1,
					Position: spatial.Position{X: 5, Y: 5}, Decider: &patrolDecider{positions: []spatial.Position{posB}}},
				{ID: bID, Kind: encounter.KindMonster, Room: room1,
					Position: spatial.Position{X: 6, Y: 6}, Decider: &patrolDecider{positions: []spatial.Position{posA}}},
			},
			Endings: []encounter.EndingInput{
				// Declared first: fires for zzz-goblin (posA), who decides SECOND.
				{Key: "first", Trigger: encounter.TriggerReachedPosition{Room: room1, Position: posA, Member: bID}},
				// Declared second: fires for aaa-goblin (posB), who decides FIRST.
				{Key: "second", Trigger: encounter.TriggerReachedPosition{Room: room1, Position: posB, Member: aID}},
			},
		})
		s.Require().NoError(err)

		out, err := enc.Pump(&encounter.PumpInput{})
		s.Require().NoError(err)
		s.Require().NotNil(out.Outcome)
		s.Equal("second", out.Outcome.Ending,
			"aaa-goblin decides first and lands on the SECOND-declared ending — "+
				"decision order wins, not declaration order")
	})
}

// reentrantSelfExitDecider violates the decider contract: it calls
// Exit on its own member from inside Decide, then still returns a
// move intent. This is a contract-violating decider, not a supported
// pattern — but the pump must survive it. Set enc after NewEncounter
// returns (the encounter doesn't exist yet when Members are declared).
type reentrantSelfExitDecider struct {
	enc  *encounter.Encounter
	self core.EntityID
}

func (r *reentrantSelfExitDecider) Decide(_ encounter.Snapshot) (encounter.Intent, error) {
	if _, err := r.enc.Exit(&encounter.ExitInput{Member: r.self}); err != nil {
		return nil, err
	}
	return encounter.IntentMoveTo{To: spatial.Position{X: 9, Y: 9}}, nil
}

// TestPumpSurvivesReentrantSelfExitingDecider pins the phase-1/phase-2
// seam against a decider that mutates membership mid-decide: phase 1's
// planned list is keyed by MemberID, and phase 2 looks the live member
// pointer up fresh from e.members — which the reentrant Exit has
// already deleted. Without a nil guard at that lookup, phase 2
// dereferences a nil *Member and panics. The shipped contract treats
// this exactly like any other phase-2 execution failure: silent skip,
// nothing else disturbed.
func (s *PumpTestSuite) TestPumpSurvivesReentrantSelfExitingDecider() {
	aliceID := core.EntityID("alice")
	goblinID := core.EntityID("goblin")
	decider := &reentrantSelfExitDecider{self: goblinID}

	enc, err := encounter.NewEncounter(&encounter.SetupInput{
		Field: encounter.FieldInput{Rooms: []encounter.RoomInput{{ID: room1, Width: 10, Height: 10}}},
		Members: []encounter.MemberInput{
			{ID: aliceID, Kind: encounter.KindPlayer, Room: room1, Position: spatial.Position{X: 1, Y: 1}},
			{ID: goblinID, Kind: encounter.KindMonster, Room: room1,
				Position: spatial.Position{X: 4, Y: 4}, Decider: decider},
		},
		Endings: []encounter.EndingInput{{Key: "done", Trigger: encounter.TriggerExternal{}}},
	})
	s.Require().NoError(err)
	decider.enc = enc

	s.Require().NotPanics(func() {
		out, pumpErr := enc.Pump(&encounter.PumpInput{})
		s.Require().NoError(pumpErr, "a contract-violating self-exiting decider must not abort the pump")
		s.Empty(out.MonsterMoves, "the exited monster's planned move has no live member left to execute")
	})

	members, err := enc.Members()
	s.Require().NoError(err)
	s.Require().Len(members, 1, "only alice remains — goblin's self-exit went through")
	s.Equal(aliceID, members[0].ID)
}
