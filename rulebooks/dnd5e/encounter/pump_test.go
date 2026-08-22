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
	sawPeer bool
}

func (v *vandalDecider) Decide(snap encounter.Snapshot) (encounter.Intent, error) {
	for i := range snap.Holdings {
		if snap.Holdings[i].Subject == "rat" {
			v.sawPeer = true
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

// doorwaysFrom projects a field's connections into absolute cell pairs — the
// same thing Atlas.Doorways carries.
//
// For fixtures that must hand a decider its map BEFORE the encounter that
// would produce one exists. chase_test.go does it the other way round, taking
// the real Atlas after construction, which is what a host would do; this is
// the same arithmetic for the cases where ordering makes that awkward.
func doorwaysFrom(field encounter.FieldInput) []encounter.AtlasDoorway {
	rooms := map[string]encounter.RoomInput{}
	for _, room := range field.Rooms {
		rooms[room.ID] = room
	}

	out := make([]encounter.AtlasDoorway, 0, len(field.Connections))
	for _, c := range field.Connections {
		out = append(out, encounter.AtlasDoorway{
			Connection: c.ID,
			From:       c.From,
			FromCell:   authoredCellAt(field, rooms[c.From], c.FromPosition),
			To:         c.To,
			ToCell:     authoredCellAt(field, rooms[c.To], c.ToPosition),
		})
	}
	return out
}

// authoredCellAt is a fixture's own copy of the projection the composition
// spends a room's anchor with — element-wise addition for a square room, and
// for a hex one the authored offset column and row added FIRST and converted
// once (rpg-toolkit#1127).
//
// A copy rather than a call, because the point of a fixture that builds a
// decider's map by hand is to compare the composition's answer against
// arithmetic done independently of it. What it borrows is tools/spatial's
// conversion, through the exported [encounter.HexCellAt], which belongs to
// neither side of that comparison.
func authoredCellAt(field encounter.FieldInput, room encounter.RoomInput, local spatial.Position) spatial.Position {
	if room.Grid != spatial.GridShapeHex {
		return local.Add(room.Origin)
	}
	return encounter.HexCellAt(field.Canvas.Orientation,
		int(local.X)+int(room.Origin.X), int(local.Y)+int(room.Origin.Y))
}

// onceStepDecider intends one step to a fixed absolute cell, then holds
// forever after — a minimal fixture for pinning a single attempt's success or
// failure, whether the cell is next door or through a door.
type onceStepDecider struct {
	to     spatial.Position
	called bool
}

func (t *onceStepDecider) Decide(_ encounter.Snapshot) (encounter.Intent, error) {
	if t.called {
		return encounter.IntentHold{}, nil
	}
	t.called = true
	return encounter.IntentMoveTo{To: t.to}, nil
}

// pursuitDecider walks toward the last place it saw its target.
//
// It is worth comparing against what this fixture used to be. It read a room
// id off the percept, compared it with its own room and held if they differed;
// then, if it happened to be standing exactly where the percept placed its
// target, it scanned a room-local connection list for an endpoint matching
// BOTH its room and its cell, and chose between two different intent types.
// Every one of those steps existed to reconcile two coordinate systems.
//
// In one frame (rpg-toolkit#1044) it subtracts one cell from another and steps.
// It crosses a doorway without knowing what a doorway is: the far side is
// simply the next cell along. The only thing it still needs told is where the
// doorways ARE — because a doorway is the one adjacency that is not implied by
// the coordinates — and it takes that as the Atlas, the same absolute map a
// host renders, rather than as the composition's room-shaped topology.
type pursuitDecider struct {
	doorways []encounter.AtlasDoorway
	target   core.EntityID
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

		here := snap.Position
		there := spatial.Position{X: seen.X, Y: seen.Y}
		if here != there {
			return encounter.IntentMoveTo{To: there}, nil
		}

		// Standing where it last saw them, and they are not here: the only
		// place left to look is through the door it is standing in.
		for _, d := range p.doorways {
			if d.FromCell == here {
				return encounter.IntentMoveTo{To: d.ToCell}, nil
			}
			if d.ToCell == here {
				return encounter.IntentMoveTo{To: d.FromCell}, nil
			}
		}
		return encounter.IntentHold{}, nil
	}
	return encounter.IntentHold{}, nil
}

// TestPumpDoesNotConsultExitedMonsterDecider pins Exit's decider
// cleanup: an exited monster's decider must never be consulted again.
func (s *PumpTestSuite) TestPumpDoesNotConsultExitedMonsterDecider() {
	enc, err := encounter.NewEncounter(&encounter.SetupInput{
		Sight: everyoneSeesTheWholeMap{}, Standing: everyoneStanding{}, Initiative: orderAsGiven{}, TurnDriver: passDriver{},
		Field: encounter.FieldInput{Canvas: encounter.CanvasInput{Void: encounter.VoidIsOpaque()}, Rooms: []encounter.RoomInput{{ID: room1, Width: 10, Height: 10, Boundaries: twoRoomSealedWall()}, {ID: room2, Width: 10, Height: 10, Origin: spatial.Position{X: 10, Y: 0}}}},
		Members: []encounter.MemberInput{
			{ID: core.EntityID("alice"), Kind: encounter.KindPlayer, Room: room2, Position: spatial.Position{X: 1, Y: 1}},
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
			Sight: everyoneSeesTheWholeMap{}, Standing: everyoneStanding{}, Initiative: orderAsGiven{}, TurnDriver: passDriver{},
			Field: encounter.FieldInput{
				Canvas: encounter.CanvasInput{Void: encounter.VoidIsOpaque()},
				Rooms: []encounter.RoomInput{
					{
						ID:     room1,
						Width:  10,
						Height: 10, Boundaries: twoRoomSealedWall()},
					{
						ID:     room2,
						Width:  10,
						Height: 10,
						Origin: spatial.Position{X: 10, Y: 0},
					},
				},
				Connections: []encounter.ConnectionInput{},
			},
			Members: []encounter.MemberInput{
				{
					ID:       aliceID,
					Kind:     encounter.KindPlayer,
					Room:     room2,
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

		// The mutual-sight half of this test is gone with rpg-toolkit#964's v1:
		// alice waits in the next room because a monster she can SEE is a
		// monster she is FIGHTING, and Pump does not move a fight member. What
		// a patrol looks like once somebody notices it is pinned separately,
		// by TestPumpStopsMovingAMonsterOnceSeen.
		//
		// The goblin's own view is still the honest check that it moved and
		// perceives from where it now stands.
		goblinView, err := enc.View(&encounter.ViewInput{Member: goblinID})
		s.Require().NoError(err)
		s.Empty(goblinView, "nobody to see from (5,5) — alice is a room away")

		// Second Pump: goblin should move to (3,3)
		pumpOut2, err := enc.Pump(&encounter.PumpInput{})
		s.Require().NoError(err)
		s.Equal(uint64(2), pumpOut2.Tick, "tick should be 2 after second pump")
		s.Len(pumpOut2.MonsterMoves, 1, "should have one monster move")
		s.Equal(spatial.Position{X: 5, Y: 5}, pumpOut2.MonsterMoves[0].From, "goblin should start at (5,5)")
		s.Equal(spatial.Position{X: 3, Y: 3}, pumpOut2.MonsterMoves[0].To, "goblin should move to (3,3)")

		// Assert: alice sees goblin at new position
		goblinView, err = enc.View(&encounter.ViewInput{Member: goblinID})
		s.Require().NoError(err)
		s.Empty(goblinView, "still a room away")
	})
}

func (s *PumpTestSuite) TestPumpDeciderIsolation() {
	s.Run("a decider's snapshot holds what it can see and nothing else", func() {
		aliceID := core.EntityID("alice")
		goblinID := core.EntityID("goblin")

		spy := &spyDecider{}

		// Alice stands ONE CELL from the goblin with a wall between them. She
		// is real, adjacent, and invisible — which is the only way to ask the
		// anti-wall-hack question in v1: a monster that could SEE her would be
		// fighting her, and Pump would not consult its decider at all
		// (rpg-toolkit#964).
		//
		// WHAT THIS TEST LOST, named rather than quietly dropped: C2 has two
		// halves, and only the no-leak half survives here. The other half —
		// "holdings contain what it DID see" — needs a monster that watches
		// players peacefully, which v1 cannot express. It comes back with
		// asymmetric perception (#1020) or a faction model, at which point
		// this test should grow the positive case again.
		enc, err := encounter.NewEncounter(&encounter.SetupInput{
			Sight: everyoneSeesTheWholeMap{}, Standing: everyoneStanding{}, Initiative: orderAsGiven{}, TurnDriver: passDriver{},
			Field: encounter.FieldInput{
				Canvas: encounter.CanvasInput{Void: encounter.VoidIsOpaque()},
				Rooms: []encounter.RoomInput{{
					ID: room1, Width: 10, Height: 10,
					// A wall down x=5 rather than one cell of it: spatial
					// v0.9.1 leans around a lone obstacle, so a fixture that
					// wants sight blocked builds a wall (testwalls_test.go).
					Props: wallColumn(5, 4, 6),
				}},
			},
			Members: []encounter.MemberInput{
				{ID: aliceID, Kind: encounter.KindPlayer, Room: room1, Position: spatial.Position{X: 4, Y: 5}},
				{
					ID: goblinID, Kind: encounter.KindMonster, Room: room1,
					Position: spatial.Position{X: 6, Y: 5}, Decider: spy,
				},
			},
			Endings: []encounter.EndingInput{{Key: "called", Trigger: encounter.TriggerExternal{}}},
		})
		s.Require().NoError(err)

		// Nothing formed: the wall means neither noticed the other, so the
		// goblin is still the world's to move and its decider still runs.
		clockOf, err := enc.ClockOf(&encounter.ClockOfInput{Member: goblinID})
		s.Require().NoError(err)
		s.Require().Equal(encounter.ClockWorld, clockOf.Kind)

		_, err = enc.Pump(&encounter.PumpInput{})
		s.Require().NoError(err)

		s.Equal(spatial.Position{X: 6, Y: 5}, spy.capturedSnap.Position, "its own cell, on the map")

		// THE PIN: alice exists one cell away and does not appear. The
		// snapshot is built from what this member holds, not from the
		// encounter's live truth — so an adjacent player it cannot see is a
		// player it cannot read.
		s.Empty(spy.capturedView,
			"an adjacent but unseen player must not leak into a decider's snapshot")
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
			Sight: everyoneSeesTheWholeMap{}, Standing: everyoneStanding{}, Initiative: orderAsGiven{}, TurnDriver: passDriver{},
			Field: encounter.FieldInput{
				Canvas: encounter.CanvasInput{Void: encounter.VoidIsOpaque()},
				Rooms: []encounter.RoomInput{
					{
						ID:     room1,
						Width:  10,
						Height: 10, Boundaries: twoRoomSealedWall()},
					{
						ID:     room2,
						Width:  10,
						Height: 10,
						Origin: spatial.Position{X: 10, Y: 0},
					},
				},
				Connections: []encounter.ConnectionInput{},
			},
			Members: []encounter.MemberInput{
				{
					ID:       aliceID,
					Kind:     encounter.KindPlayer,
					Room:     room2,
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

		// Assert: the goblin is where it started. Read from placement rather
		// than from alice's percept — she waits a room away, because a
		// monster she could see would be a monster she was fighting, and a
		// fight monster is never pumped at all (rpg-toolkit#964).
		var goblinPos encounter.PositionData
		for _, m := range enc.ToData().Members {
			if m.ID == goblinID {
				goblinPos = *m.Cell
			}
		}
		s.Equal(5.0, goblinPos.X, "goblin must not have moved on a failed pump (R5)")
		s.Equal(5.0, goblinPos.Y)
	})
}

// TestPumpFailedPumpAdvancesNothing pins the tick side of R5 with a
// fail-once decider: the failed pump must not consume a tick, so the
// NEXT (successful) pump reports reading 1, not 2.
func (s *PumpTestSuite) TestPumpFailedPumpAdvancesNothing() {
	enc, err := encounter.NewEncounter(&encounter.SetupInput{
		Sight: everyoneSeesTheWholeMap{}, Standing: everyoneStanding{}, Initiative: orderAsGiven{}, TurnDriver: passDriver{},
		Field: encounter.FieldInput{Canvas: encounter.CanvasInput{Void: encounter.VoidIsOpaque()}, Rooms: []encounter.RoomInput{{ID: room1, Width: 10, Height: 10, Boundaries: twoRoomSealedWall()}, {ID: room2, Width: 10, Height: 10, Origin: spatial.Position{X: 10, Y: 0}}}},
		Members: []encounter.MemberInput{
			{ID: core.EntityID("alice"), Kind: encounter.KindPlayer, Room: room2, Position: spatial.Position{X: 1, Y: 1}},
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
		Sight: everyoneSeesTheWholeMap{}, Standing: everyoneStanding{}, Initiative: orderAsGiven{}, TurnDriver: passDriver{},
		Field: encounter.FieldInput{Canvas: encounter.CanvasInput{Void: encounter.VoidIsOpaque()}, Rooms: []encounter.RoomInput{{ID: room1, Width: 10, Height: 10, Boundaries: twoRoomSealedWall()}, {ID: room2, Width: 10, Height: 10, Origin: spatial.Position{X: 10, Y: 0}}}},
		Members: []encounter.MemberInput{
			{ID: core.EntityID("alice"), Kind: encounter.KindPlayer, Room: room2, Position: spatial.Position{X: 1, Y: 1}},
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
		Sight: everyoneSeesTheWholeMap{}, Standing: everyoneStanding{}, Initiative: orderAsGiven{}, TurnDriver: passDriver{},
		Field: encounter.FieldInput{Canvas: encounter.CanvasInput{Void: encounter.VoidIsOpaque()}, Rooms: []encounter.RoomInput{{ID: room1, Width: 10, Height: 10, Boundaries: twoRoomSealedWall()}, {ID: room2, Width: 10, Height: 10, Origin: spatial.Position{X: 10, Y: 0}}}},
		Members: []encounter.MemberInput{
			{ID: core.EntityID("alice"), Kind: encounter.KindPlayer, Room: room2, Position: spatial.Position{X: 1, Y: 1}},
			// The vandal's victim is a MONSTER peer, not a player. Two
			// monsters seeing each other starts no fight — classification
			// pairs players against monsters — so the goblin keeps a real,
			// current holding to scribble on while staying the world's to
			// pump (rpg-toolkit#964).
			{ID: core.EntityID("rat"), Kind: encounter.KindMonster, Room: room1,
				Position: spatial.Position{X: 6, Y: 6}},
			{ID: core.EntityID("goblin"), Kind: encounter.KindMonster, Room: room1,
				Position: spatial.Position{X: 4, Y: 4}, Decider: vandal},
		},
		Endings: []encounter.EndingInput{{Key: endingStairs,
			Trigger: encounter.TriggerReachedPosition{Room: room1, Position: spatial.Position{X: 9, Y: 9}}}},
	})
	s.Require().NoError(err)

	_, err = enc.Pump(&encounter.PumpInput{})
	s.Require().NoError(err)
	s.Require().True(vandal.sawPeer, "precondition: the vandal saw and scribbled on a holding")

	goblinView, err := enc.View(&encounter.ViewInput{Member: core.EntityID("goblin")})
	s.Require().NoError(err)
	s.Require().Len(goblinView, 1)
	var p encounter.SightPayload
	s.Require().NoError(json.Unmarshal(goblinView[0].Payload, &p))
	// The rat, where the fixture placed it — room1 is anchored at the origin,
	// so its local cell is its cell on the map.
	s.Equal(6.0, p.X, "the vandal's scribbles must not reach encounter state")
	s.Equal(6.0, p.Y)
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
			Sight: everyoneSeesTheWholeMap{}, Standing: everyoneStanding{}, Initiative: orderAsGiven{}, TurnDriver: passDriver{},
			Field: encounter.FieldInput{
				Canvas: encounter.CanvasInput{Void: encounter.VoidIsOpaque()},
				Rooms: []encounter.RoomInput{
					{
						ID:     room1,
						Width:  10,
						Height: 10, Boundaries: twoRoomSealedWall()},
					{
						ID:     room2,
						Width:  10,
						Height: 10,
						Origin: spatial.Position{X: 10, Y: 0},
					},
				},
				Connections: []encounter.ConnectionInput{},
			},
			Members: []encounter.MemberInput{
				{
					ID:       aliceID,
					Kind:     encounter.KindPlayer,
					Room:     room2,
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
			Sight: everyoneSeesTheWholeMap{}, Standing: everyoneStanding{}, Initiative: orderAsGiven{}, TurnDriver: passDriver{},
			Field: encounter.FieldInput{
				Canvas: encounter.CanvasInput{Void: encounter.VoidIsOpaque()},
				Rooms: []encounter.RoomInput{
					{
						ID:     room1,
						Width:  10,
						Height: 10, Boundaries: twoRoomSealedWall()},
					{
						ID:     room2,
						Width:  10,
						Height: 10,
						Origin: spatial.Position{X: 10, Y: 0},
					},
				},
				Connections: []encounter.ConnectionInput{},
			},
			Members: []encounter.MemberInput{
				{
					ID:       aliceID,
					Kind:     encounter.KindPlayer,
					Room:     room2,
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

		// Assert: the goblin is still at (5,5), read from placement. Alice
		// waits a room away — a monster she could see would be one she was
		// fighting, and a fight monster is never pumped (rpg-toolkit#964).
		var goblinPos encounter.PositionData
		for _, m := range enc.ToData().Members {
			if m.ID == goblinID {
				goblinPos = *m.Cell
			}
		}
		s.Equal(5.0, goblinPos.X)
		s.Equal(5.0, goblinPos.Y)
	})
}

// TestPumpJoinedMonsterWithNilDeciderHolds is Join's counterpart to
// TestPumpMonsterWithNilDecider (Setup) and
// TestDeciderReattachmentWithoutDecider (Load): a monster that joins with
// a nil Decider is legal and simply holds — Join already guards this
// (encounter.go's `if in.Member.Decider != nil`), previously unpinned at
// this seam.
func (s *PumpTestSuite) TestPumpJoinedMonsterWithNilDeciderHolds() {
	enc, err := encounter.NewEncounter(&encounter.SetupInput{
		Sight: everyoneSeesTheWholeMap{}, Standing: everyoneStanding{}, Initiative: orderAsGiven{}, TurnDriver: passDriver{},
		Field: encounter.FieldInput{Canvas: encounter.CanvasInput{Void: encounter.VoidIsOpaque()}, Rooms: []encounter.RoomInput{{ID: room1, Width: 10, Height: 10, Boundaries: twoRoomSealedWall()}, {ID: room2, Width: 10, Height: 10, Origin: spatial.Position{X: 10, Y: 0}}}},
		Members: []encounter.MemberInput{
			{ID: alice, Kind: encounter.KindPlayer, Room: room2, Position: spatial.Position{X: 0, Y: 0}},
		},
		Endings: []encounter.EndingInput{{Key: endingStairs, Trigger: encounter.TriggerReachedPosition{
			Room: room1, Position: spatial.Position{X: 9, Y: 9}}}},
	})
	s.Require().NoError(err)

	_, err = enc.Join(&encounter.JoinInput{
		Member: goblin,
		Kind:   encounter.KindMonster,
		Cell:   spatial.Position{X: 5, Y: 5},
	})
	s.Require().NoError(err)

	out, err := enc.Pump(&encounter.PumpInput{})
	s.Require().NoError(err)
	s.Empty(out.MonsterMoves, "a joined monster with no decider should not move")
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
			Sight: everyoneSeesTheWholeMap{}, Standing: everyoneStanding{}, Initiative: orderAsGiven{}, TurnDriver: passDriver{},
			Field: encounter.FieldInput{
				Canvas: encounter.CanvasInput{Void: encounter.VoidIsOpaque()},
				Rooms: []encounter.RoomInput{
					{
						ID:     room1,
						Width:  10,
						Height: 10, Boundaries: twoRoomSealedWall()},
					{
						ID:     room2,
						Width:  10,
						Height: 10,
						Origin: spatial.Position{X: 10, Y: 0},
					},
				},
				Connections: []encounter.ConnectionInput{},
			},
			Members: []encounter.MemberInput{
				{
					ID:       aliceID,
					Kind:     encounter.KindPlayer,
					Room:     room2,
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
			Sight: everyoneSeesTheWholeMap{}, Standing: everyoneStanding{}, Initiative: orderAsGiven{}, TurnDriver: passDriver{},
			Field: encounter.FieldInput{
				Canvas: encounter.CanvasInput{Void: encounter.VoidIsOpaque()},
				Rooms: []encounter.RoomInput{
					{
						ID:     room1,
						Width:  10,
						Height: 10, Boundaries: twoRoomSealedWall()},
					{
						ID:     room2,
						Width:  10,
						Height: 10,
						Origin: spatial.Position{X: 10, Y: 0},
					},
				},
				Connections: []encounter.ConnectionInput{},
			},
			Members: []encounter.MemberInput{
				{
					ID:       aliceID,
					Kind:     encounter.KindPlayer,
					Room:     room2,
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
			Sight: everyoneSeesTheWholeMap{}, Standing: everyoneStanding{}, Initiative: orderAsGiven{}, TurnDriver: passDriver{},
			Field: encounter.FieldInput{
				Canvas: encounter.CanvasInput{Void: encounter.VoidIsOpaque()},
				Rooms: []encounter.RoomInput{
					{
						ID:     room1,
						Width:  10,
						Height: 10, Boundaries: twoRoomSealedWall()},
					{
						ID:     room2,
						Width:  10,
						Height: 10,
						Origin: spatial.Position{X: 10, Y: 0},
					},
				},
				Connections: []encounter.ConnectionInput{},
			},
			Members: []encounter.MemberInput{
				{
					ID:       aliceID,
					Kind:     encounter.KindPlayer,
					Room:     room2,
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
			Sight: everyoneSeesTheWholeMap{}, Standing: everyoneStanding{}, Initiative: orderAsGiven{}, TurnDriver: passDriver{},
			Field: encounter.FieldInput{
				Canvas: encounter.CanvasInput{Void: encounter.VoidIsOpaque()},
				Rooms: []encounter.RoomInput{
					{
						ID:     room1,
						Width:  10,
						Height: 10, Boundaries: twoRoomSealedWall()},
					{
						ID:     room2,
						Width:  10,
						Height: 10,
						Origin: spatial.Position{X: 10, Y: 0},
					},
				},
				Connections: []encounter.ConnectionInput{},
			},
			Members: []encounter.MemberInput{
				{
					ID:       aliceID,
					Kind:     encounter.KindPlayer,
					Room:     room2,
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
// review lesson) so a from/to mix-up would be observable. room-b's Origin
// (see the room-a/room-b RoomInput literals below) sits it immediately to
// room-a's east — (9,5) and (0,5)+(10,0)=(10,5) are Chebyshev-adjacent
// (distance 1), satisfying W3, while the rooms' absolute footprints
// (x:[0,9] vs x:[10,19]) stay disjoint, satisfying W2 (#929 T1).
var twoRoomDoor = encounter.ConnectionInput{
	ID: "door1", From: "room-a", To: "room-b",
	FromPosition: spatial.Position{X: 9, Y: 5},
	ToPosition:   spatial.Position{X: 0, Y: 5},
}

// twoRoomWall is room-a's east wall, open at the doorway's row. Every edge has
// one endpoint in room-a and one in room-b, which is a sentence no room could
// say before the field became one canvas (rpg-toolkit#1106) — and one these
// fixtures now have to say, because with one canvas and no wall the two
// chambers share an open seam ten cells wide.
func twoRoomWall() []spatial.Boundary { return squareSeamWall(9, 10, 5) }

// twoRoomSealedWall is the same wall with NO opening — for the fixtures that
// declare no doorway at all. Two chambers side by side with nothing joining
// them is a solid partition, and saying so is now the fixture's job: it used to
// be implied by their being different rooms.
func twoRoomSealedWall() []spatial.Boundary { return squareSeamWall(9, 10) }

// twoRoomField is the field most of this file's two-room fixtures build:
// room-a at the origin, room-b anchored immediately east of it, joined at
// the doorway above. Absolute footprints x:[0,9] and x:[10,19] (W2), and
// the doorway's cells (9,5) and (10,5) are adjacent (W3).
func twoRoomField() encounter.FieldInput {
	return encounter.FieldInput{
		Canvas: encounter.CanvasInput{Void: encounter.VoidIsOpaque()},
		Rooms: []encounter.RoomInput{
			{ID: "room-a", Width: 10, Height: 10, Boundaries: twoRoomWall()},
			{ID: "room-b", Width: 10, Height: 10, Origin: spatial.Position{X: 10, Y: 0}},
		},
		Connections: []encounter.ConnectionInput{twoRoomDoor},
	}
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
		Sight: everyoneSeesTheWholeMap{}, Standing: everyoneStanding{}, Initiative: orderAsGiven{}, TurnDriver: passDriver{},
		Field: encounter.FieldInput{
			Canvas: encounter.CanvasInput{Void: encounter.VoidIsOpaque()},
			Rooms: []encounter.RoomInput{
				{ID: "room-a", Width: 10, Height: 10, Boundaries: twoRoomSealedWall()},
				{ID: "room-b", Width: 10, Height: 10, Origin: spatial.Position{X: 10, Y: 0}},
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

	// Absolute cells, so the two snapshots are distinguishable by position
	// alone — which is the whole assertion now that a snapshot no longer
	// names a room. room-b is anchored at (10,0), so its goblin's local
	// (7,8) is (17,8) on the map: a snapshot built in the wrong frame, or
	// from the wrong member, lands somewhere else.
	s.Require().Len(spyA.snapshots, 1)
	s.Equal(spatial.Position{X: 2, Y: 3}, spyA.snapshots[0].Position, "goblin-a's snapshot must be ITS OWN cell")

	s.Require().Len(spyB.snapshots, 1)
	s.Equal(spatial.Position{X: 17, Y: 8}, spyB.snapshots[0].Position, "goblin-b's snapshot must be ITS OWN cell")
}

// TestAnIntendedStepCrossesADoorway pins the crossing as what it now is: a
// monster standing at the threshold names the cell on the far side, and the
// composition carries it through the door.
//
// There is no traverse intent to decide any more (rpg-toolkit#1044) — the same
// IntentMoveTo that walks a monster across a room walks it through a doorway,
// because W3 makes the far side an adjacent cell on the map. What the pump
// reports is unchanged: the room changes, the "traversed" beat is recorded,
// PumpOutput.MonsterTraverses reflects it (and MonsterMoves does not), and the
// clock still advances by exactly 1 — Pump's own tick is unconditional
// regardless of what a monster does within it (a crossing itself is not time,
// law T4, but the PUMP is).
func (s *PumpTestSuite) TestAnIntendedStepCrossesADoorway() {
	goblinID := core.EntityID("goblin")
	// The cell on the far side of the doorway: room-b's local (0,5), anchored
	// at (10,0). One step from the threshold, on the map.
	decider := &onceStepDecider{to: spatial.Position{X: 10, Y: 5}}

	enc, err := encounter.NewEncounter(&encounter.SetupInput{
		Sight: everyoneSeesTheWholeMap{}, Standing: everyoneStanding{}, Initiative: orderAsGiven{}, TurnDriver: passDriver{},
		Field: encounter.FieldInput{
			Canvas: encounter.CanvasInput{Void: encounter.VoidIsOpaque()},
			Rooms: []encounter.RoomInput{
				{ID: "room-a", Width: 10, Height: 10, Boundaries: twoRoomWall()},
				{ID: "room-b", Width: 10, Height: 10, Origin: spatial.Position{X: 10, Y: 0}},
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
	s.Require().Len(out.MonsterMoves, 1, "one step, in the one list there is")
	s.Equal(goblinID, out.MonsterMoves[0].Member)
	s.Equal(spatial.Position{X: 9, Y: 5}, out.MonsterMoves[0].From)
	// The arrival cell as the decider named it: room-b's local (0,5) is
	// (10,5) on the map, and what the pump reports is the cell the decider
	// asked for, not a room-local one it would have to re-anchor.
	s.Equal(spatial.Position{X: 10, Y: 5}, out.MonsterMoves[0].To)

	members, err := enc.Members()
	s.Require().NoError(err)
	s.Require().Len(members, 1)
	s.Equal(encounter.RegionID("room-b"), members[0].Region)
}

// TestPumpReportsMovementOnTheMap is the discriminating probe for
// rpg-toolkit#1062: NEITHER room is anchored at the origin, so a room-local
// coordinate and its absolute one differ in every single assertion below.
//
// Every other fixture in this file anchors room-a at (0,0), which made a
// room-local report indistinguishable from an absolute one for a same-room
// move and for a crossing's departure cell — the frame was pinned by
// coincidence, not by a test. Here the coincidence is gone: the prowler
// walks within a room anchored at (40,20) and then crosses into one anchored
// at (50,20), and the pump's report is checked against BOTH the map and the
// beat that describes the same movement.
//
// The beat cross-check is the point. Pump reports one movement twice — once
// as a typed output a host reads, once as a beat a host renders — and the two
// must be the same cell. They were not: the beat projected and the output did
// not, so a host that trusted both drew the monster in two places the moment a
// room stopped sitting at the origin.
func (s *PumpTestSuite) TestPumpReportsMovementOnTheMap() {
	prowler := core.EntityID("prowler")
	const (
		cellar = "cellar"
		shrine = "shrine"
	)

	// cellar occupies absolute x:[40,49], shrine x:[50,59] — disjoint (W2) —
	// and the door's endpoints, cellar-local (9,5) and shrine-local (0,5),
	// land on absolute (49,25) and (50,25): adjacent cells (W3).
	door := encounter.ConnectionInput{
		ID: "cellar-door", From: cellar, To: shrine,
		FromPosition: spatial.Position{X: 9, Y: 5},
		ToPosition:   spatial.Position{X: 0, Y: 5},
	}
	// Both steps are named in absolute space, which is the only space a
	// decider has spoken since rpg-toolkit#1044: walk to the threshold, then
	// walk to the cell beyond it.
	patrol := &patrolDecider{positions: []spatial.Position{
		{X: 49, Y: 25}, {X: 50, Y: 25},
	}}

	enc, err := encounter.NewEncounter(&encounter.SetupInput{
		Sight: everyoneSeesTheWholeMap{}, Standing: everyoneStanding{}, Initiative: orderAsGiven{}, TurnDriver: passDriver{},
		Field: encounter.FieldInput{
			Canvas: encounter.CanvasInput{Void: encounter.VoidIsOpaque()},
			Rooms: []encounter.RoomInput{
				{ID: cellar, Width: 10, Height: 10, Origin: spatial.Position{X: 40, Y: 20}},
				{ID: shrine, Width: 10, Height: 10, Origin: spatial.Position{X: 50, Y: 20}},
			},
			Connections: []encounter.ConnectionInput{door},
		},
		Members: []encounter.MemberInput{
			// cellar-local (3,5) — absolute (43,25).
			{ID: prowler, Kind: encounter.KindMonster, Room: cellar,
				Position: spatial.Position{X: 3, Y: 5}, Decider: patrol},
		},
		Endings: []encounter.EndingInput{{Key: "done", Trigger: encounter.TriggerExternal{}}},
	})
	s.Require().NoError(err)

	// ---- The same-room move ------------------------------------------------
	out1, err := enc.Pump(&encounter.PumpInput{})
	s.Require().NoError(err)
	s.Require().Len(out1.MonsterMoves, 1)
	s.Equal(prowler, out1.MonsterMoves[0].Member)
	s.Equal(spatial.Position{X: 43, Y: 25}, out1.MonsterMoves[0].From,
		"it departs from (43,25) on the map — cellar-local (3,5) is not an answer without the room")
	s.Equal(spatial.Position{X: 49, Y: 25}, out1.MonsterMoves[0].To,
		"and arrives at the cell the decider named, in the frame the decider named it")
	s.Equal(out1.MonsterMoves[0].To, s.movedBeatPosition(enc, prowler, out1.Seqs),
		"the moved beat and the typed output describe ONE movement")

	// ---- The crossing ------------------------------------------------------
	out2, err := enc.Pump(&encounter.PumpInput{})
	s.Require().NoError(err)
	s.Require().Len(out2.MonsterMoves, 1)
	s.Equal(prowler, out2.MonsterMoves[0].Member)
	s.Equal(spatial.Position{X: 49, Y: 25}, out2.MonsterMoves[0].From,
		"the departure cell is the cellar-side threshold, on the map")
	s.Equal(spatial.Position{X: 50, Y: 25}, out2.MonsterMoves[0].To,
		"and the arrival cell is the shrine-side one — one cell along, on the same map")
	s.Equal(out2.MonsterMoves[0].To, s.movedBeatPosition(enc, prowler, out2.Seqs),
		"the movement beat and the typed output describe ONE movement, crossing or not")

	// And the map agrees with both: Members() has spoken absolute since the
	// seam reshape, so a host comparing a move's destination against a member's
	// position must find them equal without redoing any arithmetic.
	members, err := enc.Members()
	s.Require().NoError(err)
	s.Require().Len(members, 1)
	s.Equal(encounter.RegionID(shrine), members[0].Region)
	s.Equal(out2.MonsterMoves[0].To, members[0].Position,
		"where the pump says it went is where the field says it stands")
}

// movedBeatPosition reads the position off the "moved" beat this member
// recorded among the given seqs — the beat half of the one-movement-one-frame
// claim TestPumpReportsMovementOnTheMap makes.
func (s *PumpTestSuite) movedBeatPosition(
	enc *encounter.Encounter, member core.EntityID, seqs []uint64,
) spatial.Position {
	return s.beatPosition(enc, member, seqs, "moved")
}

// beatPosition finds the one beat of the given kind naming this member within
// seqs and returns the position it carries, failing the test if there is not
// exactly one.
func (s *PumpTestSuite) beatPosition(
	enc *encounter.Encounter, member core.EntityID, seqs []uint64, kind string,
) spatial.Position {
	s.T().Helper()
	wanted := make(map[uint64]bool, len(seqs))
	for _, seq := range seqs {
		wanted[seq] = true
	}
	story, err := enc.Story(&encounter.StoryInput{Audience: member, AfterSeq: 0})
	s.Require().NoError(err)

	var found []spatial.Position
	for _, entry := range story {
		if !wanted[entry.Seq] {
			continue
		}
		var beat struct {
			Beat     string           `json:"beat"`
			Member   string           `json:"member"`
			Position spatial.Position `json:"position"`
		}
		s.Require().NoError(json.Unmarshal(entry.Payload, &beat))
		if beat.Beat == kind && beat.Member == string(member) {
			found = append(found, beat.Position)
		}
	}
	s.Require().Len(found, 1, "expected exactly one %q beat for %s in this pump", kind, member)
	return found[0]
}

// TestPumpDoesNotAbortWhenAWallIsInTheWay pins the silent skip for the step
// that cannot be taken: the monster names a cell in the next chamber with the
// seam wall between them.
//
// Same contract Pump already gave a spatially-rejected step, and the reason it
// must hold is the pump, not the monster: an error here would abort the tick
// for every OTHER monster in the encounter. So the pump does NOT abort — the
// clock still advances, the tick beat is still recorded, refreshSight still
// runs — but no movement beat is recorded and the monster has not moved.
func (s *PumpTestSuite) TestPumpDoesNotAbortWhenAWallIsInTheWay() {
	goblinID := core.EntityID("goblin")
	// Straight across the seam on row 2, where the wall is solid — the doorway
	// is on row 5.
	decider := &onceStepDecider{to: spatial.Position{X: 10, Y: 2}}

	enc, err := encounter.NewEncounter(&encounter.SetupInput{
		Sight: everyoneSeesTheWholeMap{}, Standing: everyoneStanding{}, Initiative: orderAsGiven{}, TurnDriver: passDriver{},
		Field: encounter.FieldInput{
			Canvas: encounter.CanvasInput{Void: encounter.VoidIsOpaque()},
			Rooms: []encounter.RoomInput{
				{ID: "room-a", Width: 10, Height: 10, Boundaries: twoRoomWall()},
				{ID: "room-b", Width: 10, Height: 10, Origin: spatial.Position{X: 10, Y: 0}},
			},
			Connections: []encounter.ConnectionInput{twoRoomDoor},
		},
		Members: []encounter.MemberInput{
			// goblin is NOT on the doorway's row (the doorway is at (9,5)).
			{ID: goblinID, Kind: encounter.KindMonster, Room: "room-a",
				Position: spatial.Position{X: 3, Y: 2}, Decider: decider},
		},
		Endings: []encounter.EndingInput{{Key: "done", Trigger: encounter.TriggerExternal{}}},
	})
	s.Require().NoError(err)

	beforeClock := enc.ToData().Clock.HighWater

	out, err := enc.Pump(&encounter.PumpInput{})
	s.Require().NoError(err, "a step with no doorway to carry it must not abort the pump")

	afterClock := enc.ToData().Clock.HighWater
	s.Equal(beforeClock+1, afterClock, "clock still advances — matches IntentMoveTo's spatial-rejection contract")
	s.Equal(uint64(1), out.Tick)
	s.Empty(out.MonsterMoves, "the refused step must not appear as successful")
	s.Len(out.Seqs, 1, "exactly the tick beat — no movement beat for the failure")

	story, err := enc.Story(&encounter.StoryInput{Audience: goblinID, AfterSeq: 0})
	s.Require().NoError(err)
	for _, entry := range story {
		var beat map[string]any
		s.Require().NoError(json.Unmarshal(entry.Payload, &beat))
		s.NotEqual("moved", beat["beat"], "no movement beat for a refused step")
	}

	members, err := enc.Members()
	s.Require().NoError(err)
	s.Require().Len(members, 1)
	s.Equal(encounter.RegionID("room-a"), members[0].Region, "the monster never left its chamber")
	s.Equal(spatial.Position{X: 3, Y: 2}, members[0].Position, "the monster's cell is unchanged")
}

// TestPumpDoesNotAbortWhenTheCellIsNowhere pins the third silent skip, and the
// one that changed shape: a decider used to be able to name a connection that
// did not exist, and now it can name a CELL that does not — void is not floor.
//
// Treated identically to the other two by stepTo — no abort, no beat, no
// movement — which is what keeps one monster's bad arithmetic from costing
// every other monster its tick.
func (s *PumpTestSuite) TestPumpDoesNotAbortWhenTheCellIsNowhere() {
	goblinID := core.EntityID("goblin")
	// A cell no room owns: void is not floor, and stepping into it is the
	// third way stepTo refuses in silence.
	decider := &onceStepDecider{to: spatial.Position{X: 500, Y: 500}}

	enc, err := encounter.NewEncounter(&encounter.SetupInput{
		Sight: everyoneSeesTheWholeMap{}, Standing: everyoneStanding{}, Initiative: orderAsGiven{}, TurnDriver: passDriver{},
		Field: encounter.FieldInput{
			Canvas: encounter.CanvasInput{Void: encounter.VoidIsOpaque()},
			Rooms: []encounter.RoomInput{
				{ID: "room-a", Width: 10, Height: 10, Boundaries: twoRoomWall()},
				{ID: "room-b", Width: 10, Height: 10, Origin: spatial.Position{X: 10, Y: 0}},
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
	s.Require().NoError(err, "a step into a cell no room owns must not abort the pump")

	afterClock := enc.ToData().Clock.HighWater
	s.Equal(beforeClock+1, afterClock, "clock still advances")
	s.Empty(out.MonsterMoves)
	s.Len(out.Seqs, 1, "exactly the tick beat — no movement beat for the failure")

	story, err := enc.Story(&encounter.StoryInput{Audience: goblinID, AfterSeq: 0})
	s.Require().NoError(err)
	for _, entry := range story {
		var beat map[string]any
		s.Require().NoError(json.Unmarshal(entry.Payload, &beat))
		s.NotEqual("moved", beat["beat"], "no movement beat for a refused step")
	}

	members, err := enc.Members()
	s.Require().NoError(err)
	s.Require().Len(members, 1)
	s.Equal(encounter.RegionID("room-a"), members[0].Region, "the monster never left its chamber")
	s.Equal(spatial.Position{X: 9, Y: 5}, members[0].Position, "the monster's cell is unchanged")
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
		Sight: everyoneSeesTheWholeMap{}, Standing: everyoneStanding{}, Initiative: orderAsGiven{}, TurnDriver: passDriver{},
		Field: encounter.FieldInput{
			Canvas: encounter.CanvasInput{Void: encounter.VoidIsOpaque()},
			Rooms: []encounter.RoomInput{
				{ID: "room-a", Width: 10, Height: 10, Boundaries: twoRoomWall()},
				{ID: "room-b", Width: 10, Height: 10, Origin: spatial.Position{X: 10, Y: 0}},
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

	members, err := enc.Members()
	s.Require().NoError(err)
	s.Require().Len(members, 1)
	s.Equal(encounter.RegionID("room-a"), members[0].Region, "the monster never moved — R5 atomicity")
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
		Sight: everyoneSeesTheWholeMap{}, Standing: everyoneStanding{}, Initiative: orderAsGiven{}, TurnDriver: passDriver{},
		Field: encounter.FieldInput{
			Canvas: encounter.CanvasInput{Void: encounter.VoidIsOpaque()},
			Rooms: []encounter.RoomInput{
				{ID: "room-a", Width: 10, Height: 10, Boundaries: twoRoomWall()},
				{ID: "room-b", Width: 10, Height: 10, Origin: spatial.Position{X: 10, Y: 0}},
			},
			Connections: []encounter.ConnectionInput{twoRoomDoor},
		},
		Members: []encounter.MemberInput{
			{ID: aliceID, Kind: encounter.KindPlayer, Room: "room-a", Position: spatial.Position{X: 9, Y: 5}},
			{ID: goblinID, Kind: encounter.KindMonster, Room: "room-a", Position: spatial.Position{X: 8, Y: 5},
				Decider: &pursuitDecider{doorways: doorwaysFrom(twoRoomField()), target: aliceID}},
		},
		Endings: []encounter.EndingInput{{Key: "done", Trigger: encounter.TriggerExternal{}}},
	})
	s.Require().NoError(err)

	// Precondition: goblin sees alice Current (adjacent, no props).
	goblinView, err := enc.View(&encounter.ViewInput{Member: goblinID})
	s.Require().NoError(err)
	s.Require().Len(goblinView, 1)
	s.Equal(intel.Current, goblinView[0].Status, "precondition: goblin must see alice before she leaves")

	// Seeing each other started a fight (rpg-toolkit#964), and a fight member
	// cannot free-roam. Alice breaks off before she runs — which is the story
	// this test always told, now with the fight it implied made explicit.
	_, err = enc.Dissolve(&encounter.DissolveInput{Member: aliceID})
	s.Require().NoError(err)

	// Stage 1: alice steps through the doorway, then out of its line — behind
	// the wall room-a drew along its own edge, which is what hides her now
	// that a room boundary hides nothing (rpg-toolkit#1106).
	_, err = enc.Step(&encounter.StepInput{Member: aliceID, To: spatial.Position{X: 10, Y: 5}})
	s.Require().NoError(err)
	_, err = enc.Step(&encounter.StepInput{Member: aliceID, To: spatial.Position{X: 13, Y: 8}})
	s.Require().NoError(err)

	goblinView, err = enc.View(&encounter.ViewInput{Member: goblinID})
	s.Require().NoError(err)
	s.Require().Len(goblinView, 1, "the ghost is HELD, not gone")
	s.Equal(intel.Held, goblinView[0].Status, "the wall took her — goblin's sight of her fades")
	var ghostSeen encounter.SightPayload
	s.Require().NoError(json.Unmarshal(goblinView[0].Payload, &ghostSeen))
	s.Equal(10.0, ghostSeen.X, "the ghost holds alice at the doorway's far cell, her last-seen one")
	s.Equal(5.0, ghostSeen.Y)

	// Stage 2: pump — the goblin walks to the ghost's cell, which is through
	// the doorway. ONE step, in the one list there is.
	out1, err := enc.Pump(&encounter.PumpInput{})
	s.Require().NoError(err)
	s.Require().Len(out1.MonsterMoves, 1)
	s.Equal(goblinID, out1.MonsterMoves[0].Member)
	// room-b is anchored at (10,0), so the arrival cell on the map is (10,5)
	// — the same cell the movement beat carries.
	s.Equal(spatial.Position{X: 10, Y: 5}, out1.MonsterMoves[0].To)

	// The monster arrives in alice's chamber and, since THIS pump's own
	// refreshSight ran AFTER the step, already holds her Current.
	goblinView, err = enc.View(&encounter.ViewInput{Member: goblinID})
	s.Require().NoError(err)
	s.Require().Len(goblinView, 1)
	s.Equal(intel.Current, goblinView[0].Status, "the monster holds alice Current again, having come through the doorway")
	var aliceSeen encounter.SightPayload
	s.Require().NoError(json.Unmarshal(goblinView[0].Payload, &aliceSeen))
	s.Equal(13.0, aliceSeen.X)
	s.Equal(8.0, aliceSeen.Y)

	// Also verify via Story that the chase left the expected trail: a MOVEMENT
	// beat, because a step that goes through a doorway is a step
	// (rpg-toolkit#1106). It is NOT named here, and that is the contract
	// rather than an omission — the goblin walked from (8,5) to (10,5) in one
	// move, so its departure cell was not the doorway's near cell, and
	// Crossing names a doorway only for a step from one of its cells to the
	// other.
	story, err := enc.Story(&encounter.StoryInput{Audience: goblinID, AfterSeq: 0})
	s.Require().NoError(err)
	var sawArrival bool
	for _, entry := range story {
		var beat map[string]any
		s.Require().NoError(json.Unmarshal(entry.Payload, &beat))
		if beat["beat"] != "moved" || beat["member"] != string(goblinID) {
			continue
		}
		if pos, ok := beat["position"].(map[string]any); ok && pos["x"] == 10.0 && pos["y"] == 5.0 {
			sawArrival = true
		}
	}
	s.True(sawArrival, "the goblin's arrival on the far side must appear in the Story")
}

// TestPumpFullTickThenEvaluateAcrossTraverse pins full-tick-then-evaluate
// as law, and a doorway crossing is what raises its stakes: a fired ending
// during phase-2 execution would otherwise need to REVERT a room mutation,
// not just a same-room position. Two monsters decide in the
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
		Sight: everyoneSeesTheWholeMap{}, Standing: everyoneStanding{}, Initiative: orderAsGiven{}, TurnDriver: passDriver{},
		Field: encounter.FieldInput{
			Canvas: encounter.CanvasInput{Void: encounter.VoidIsOpaque()},
			Rooms: []encounter.RoomInput{
				{ID: "room-a", Width: 10, Height: 10, Boundaries: twoRoomWall()},
				{ID: "room-b", Width: 10, Height: 10, Origin: spatial.Position{X: 10, Y: 0}},
			},
			Connections: []encounter.ConnectionInput{twoRoomDoor},
		},
		Members: []encounter.MemberInput{
			// aaa-goblin is AT the threshold, ready to traverse onto the
			// filtered ending below.
			{ID: aID, Kind: encounter.KindMonster, Room: "room-a",
				Position: spatial.Position{X: 9, Y: 5},
				Decider:  &onceStepDecider{to: spatial.Position{X: 10, Y: 5}}},
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
	s.Require().Len(out.MonsterMoves, 2, "both steps executed, in decision order")
	s.Equal(aID, out.MonsterMoves[0].Member)
	s.Equal(spatial.Position{X: 10, Y: 5}, out.MonsterMoves[0].To, "through the doorway")
	s.Equal(bID, out.MonsterMoves[1].Member)
	s.Equal(spatial.Position{X: 4, Y: 4}, out.MonsterMoves[1].To)

	s.Require().NotNil(out.Outcome, "aaa-goblin's step through the doorway must fire the filtered ending")
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
	s.Equal(encounter.RegionID("room-b"), aOutcome.Region)
	s.Equal(spatial.Position{X: 10, Y: 5}, aOutcome.Position,
		"room-b-local (0,5) anchored at (10,0) — the outcome speaks the dungeon map (#1068)")
	// The full-tick-then-evaluate law, made observable: zzz-goblin's
	// ALREADY-APPLIED move is reflected in the outcome — its NEW
	// position, not its pre-pump one. A revert-on-close implementation
	// would show (3,3) here instead.
	s.Equal(encounter.RegionID("room-a"), bOutcome.Region)
	s.Equal(spatial.Position{X: 4, Y: 4}, bOutcome.Position,
		"the outcome must reflect zzz-goblin's post-tick position, not its pre-tick one")

	// Both beats actually landed in the Story (not just the output structs).
	story, err := enc.Story(&encounter.StoryInput{Audience: aID, AfterSeq: 0})
	s.Require().NoError(err)
	var sawCrossing, sawPlainMove bool
	for _, entry := range story {
		var beat map[string]any
		s.Require().NoError(json.Unmarshal(entry.Payload, &beat))
		if beat["beat"] != "moved" {
			continue
		}
		if beat["connection"] == "door1" {
			sawCrossing = true
		} else {
			sawPlainMove = true
		}
	}
	s.True(sawCrossing, "aaa-goblin's step through the doorway must be recorded, named")
	s.True(sawPlainMove, "zzz-goblin's move beat must be recorded despite the mid-tick closure")

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
			Sight: everyoneSeesTheWholeMap{}, Standing: everyoneStanding{}, Initiative: orderAsGiven{}, TurnDriver: passDriver{},
			Field: encounter.FieldInput{Canvas: encounter.CanvasInput{Void: encounter.VoidIsOpaque()}, Rooms: []encounter.RoomInput{{ID: room1, Width: 10, Height: 10, Boundaries: twoRoomSealedWall()}, {ID: room2, Width: 10, Height: 10, Origin: spatial.Position{X: 10, Y: 0}}}},
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
			Sight: everyoneSeesTheWholeMap{}, Standing: everyoneStanding{}, Initiative: orderAsGiven{}, TurnDriver: passDriver{},
			Field: encounter.FieldInput{Canvas: encounter.CanvasInput{Void: encounter.VoidIsOpaque()}, Rooms: []encounter.RoomInput{{ID: room1, Width: 10, Height: 10, Boundaries: twoRoomSealedWall()}, {ID: room2, Width: 10, Height: 10, Origin: spatial.Position{X: 10, Y: 0}}}},
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
		Sight: everyoneSeesTheWholeMap{}, Standing: everyoneStanding{}, Initiative: orderAsGiven{}, TurnDriver: passDriver{},
		Field: encounter.FieldInput{Canvas: encounter.CanvasInput{Void: encounter.VoidIsOpaque()}, Rooms: []encounter.RoomInput{{ID: room1, Width: 10, Height: 10, Boundaries: twoRoomSealedWall()}, {ID: room2, Width: 10, Height: 10, Origin: spatial.Position{X: 10, Y: 0}}}},
		Members: []encounter.MemberInput{
			{ID: aliceID, Kind: encounter.KindPlayer, Room: room2, Position: spatial.Position{X: 1, Y: 1}},
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

// TestPumpStopsMovingAMonsterOnceSeen pins the invariant v1's trigger
// detection creates, deliberately rather than incidentally: a monster the
// party has seen is in a bubble, and Pump does not move a bubble member.
//
// This is a regression guard as much as a rule. Pump drives everyone on the
// WORLD clock; if it ever also drove fight members, a monster would act twice
// per round — once on its turn and once on the world tick — and the symptom
// would be a monster that moves further than its turn allowed rather than an
// error anybody could see.
//
// The patrol walks the goblin out of its own room and into alice's view. The
// step that reaches her forms the bubble; the step after that does not happen.
func (s *PumpTestSuite) TestPumpStopsMovingAMonsterOnceSeen() {
	aliceID := core.EntityID("alice")
	goblinID := core.EntityID("goblin")

	// Two cells, one each side of the doorway: the goblin traverses into
	// alice's room and is seen the moment it arrives.
	gate := encounter.ConnectionInput{
		ID: "door", From: room1, To: room2,
		FromPosition: spatial.Position{X: 9, Y: 5},
		ToPosition:   spatial.Position{X: 0, Y: 5},
	}

	enc, err := encounter.NewEncounter(&encounter.SetupInput{
		Sight: everyoneSeesTheWholeMap{}, Standing: everyoneStanding{}, Initiative: orderAsGiven{}, TurnDriver: passDriver{},
		Field: encounter.FieldInput{
			Canvas: encounter.CanvasInput{Void: encounter.VoidIsOpaque()},
			Rooms: []encounter.RoomInput{
				{ID: room1, Width: 10, Height: 10, Boundaries: twoRoomWall()},
				{ID: room2, Width: 10, Height: 10, Origin: spatial.Position{X: 10, Y: 0}},
			},
			Connections: []encounter.ConnectionInput{gate},
		},
		Members: []encounter.MemberInput{
			// Alice waits OFF the doorway's row, so the wall is between her and
			// the goblin standing in the opening: with one canvas, next door is
			// not out of sight and the opening is a window (rpg-toolkit#1106).
			{ID: aliceID, Kind: encounter.KindPlayer, Room: room2, Position: spatial.Position{X: 3, Y: 8}},
			{
				ID: goblinID, Kind: encounter.KindMonster, Room: room1,
				Position: spatial.Position{X: 9, Y: 5},
				Decider:  &crossThenWander{through: spatial.Position{X: 10, Y: 5}},
			},
		},
		Endings: []encounter.EndingInput{{Key: "called", Trigger: encounter.TriggerExternal{}}},
	})
	s.Require().NoError(err)

	// Free roam to start: nobody has seen anybody.
	clockOf, err := enc.ClockOf(&encounter.ClockOfInput{Member: goblinID})
	s.Require().NoError(err)
	s.Require().Equal(encounter.ClockWorld, clockOf.Kind, "unseen monsters are the world's to move")

	// The pump that walks it through the door is the pump that starts the fight.
	first, err := enc.Pump(&encounter.PumpInput{})
	s.Require().NoError(err)
	s.Require().Len(first.MonsterMoves, 1, "the goblin went through the door")
	s.Require().NotNil(first.Formed, "and was seen on arrival, so the bubble formed")
	s.Require().ElementsMatch([]core.EntityID{aliceID, goblinID}, first.Formed.Order)

	clockOf, err = enc.ClockOf(&encounter.ClockOfInput{Member: goblinID})
	s.Require().NoError(err)
	s.Require().Equal(encounter.ClockTurn, clockOf.Kind, "a seen monster is in the fight")

	// THE PIN: the next pump leaves it alone. Its decider would happily wander
	// again — the pump simply is not the thing that moves it any more.
	second, err := enc.Pump(&encounter.PumpInput{})
	s.Require().NoError(err)
	s.Empty(second.MonsterMoves, "a seen monster is not the world's to move")
}

// crossThenWander steps through a doorway once, then shuffles east.
type crossThenWander struct {
	through spatial.Position
	gone    bool
}

func (t *crossThenWander) Decide(snap encounter.Snapshot) (encounter.Intent, error) {
	if !t.gone {
		t.gone = true
		return encounter.IntentMoveTo{To: t.through}, nil
	}

	return encounter.IntentMoveTo{To: spatial.Position{X: snap.Position.X + 1, Y: snap.Position.Y}}, nil
}

// TestPumpDeciderHuntsLastKnownPosition is C2's anti-wall-hack contract in
// full — both halves — in a scene v1 can actually reach.
//
// The trick is fight → Dissolve → hunt. Contact forms the bubble; Dissolve
// returns the goblin to the world clock still CARRYING what it saw, because
// dissolving moves clocks and never touches intel; alice then steps behind a
// wall, which fades her from the goblin's percept without deleting her from
// its memory. Now the goblin is a world-clock monster with real, stale intel,
// so Pump consults its decider and the snapshot has something to be wrong
// about.
//
// Both halves pin at once:
//   - POSITIVE: the snapshot holds what the goblin ITSELF saw — alice at the
//     cell she was standing in when it last had eyes on her.
//   - NEGATIVE: it does NOT hold where she actually is now.
//
// That is the anti-wall-hack in its truest form: the monster hunts your last
// known position rather than your current one. It is falsifiable in the way
// that matters — a snapshot builder that read the room's ground truth instead
// of the member's holdings would report alice's NEW cell, and this test would
// fail with the two coordinates side by side.
func (s *PumpTestSuite) TestPumpDeciderHuntsLastKnownPosition() {
	aliceID := core.EntityID("alice")
	goblinID := core.EntityID("goblin")

	spy := &spyDecider{}

	seen := spatial.Position{X: 7, Y: 5}
	hidden := spatial.Position{X: 4, Y: 5}

	enc, err := encounter.NewEncounter(&encounter.SetupInput{
		Sight: everyoneSeesTheWholeMap{}, Standing: everyoneStanding{}, Initiative: orderAsGiven{}, TurnDriver: passDriver{},
		Field: encounter.FieldInput{
			Canvas: encounter.CanvasInput{Void: encounter.VoidIsOpaque()},
			Rooms: []encounter.RoomInput{{
				ID: room1, Width: 10, Height: 10,
				// A short wall, positioned so that alice is visible from the
				// goblin where she starts and hidden where she ends up. It
				// was a single pillar until spatial v0.9.1, which leans
				// around one — see testwalls_test.go.
				Props: wallColumn(5, 4, 6),
			}},
		},
		Members: []encounter.MemberInput{
			{ID: aliceID, Kind: encounter.KindPlayer, Room: room1, Position: seen},
			{
				ID: goblinID, Kind: encounter.KindMonster, Room: room1,
				Position: spatial.Position{X: 6, Y: 5}, Decider: spy,
			},
		},
		Endings: []encounter.EndingInput{{Key: "called", Trigger: encounter.TriggerExternal{}}},
	})
	s.Require().NoError(err)

	// They saw each other at first light, so they are fighting.
	clockOf, err := enc.ClockOf(&encounter.ClockOfInput{Member: goblinID})
	s.Require().NoError(err)
	s.Require().Equal(encounter.ClockTurn, clockOf.Kind)

	// The fight breaks off. Dissolve moves clocks and nothing else — the
	// goblin walks away still remembering where alice was.
	_, err = enc.Dissolve(&encounter.DissolveInput{Member: goblinID})
	s.Require().NoError(err)

	clockOf, err = enc.ClockOf(&encounter.ClockOfInput{Member: goblinID})
	s.Require().NoError(err)
	s.Require().Equal(encounter.ClockWorld, clockOf.Kind, "back to the world's to move")

	// Alice slips behind the wall. She fades from the goblin's percept — no
	// current sight, so nothing re-triggers (classification reads first
	// contact, and a fade is not a contact).
	moved, err := enc.Step(&encounter.StepInput{Member: aliceID, To: hidden})
	s.Require().NoError(err)
	s.Require().Nil(moved.Formed, "she left its sight rather than entering it")

	clockOf, err = enc.ClockOf(&encounter.ClockOfInput{Member: goblinID})
	s.Require().NoError(err)
	s.Require().Equal(encounter.ClockWorld, clockOf.Kind, "and no fight restarted")

	// The pump consults the decider, because the goblin is the world's again.
	_, err = enc.Pump(&encounter.PumpInput{})
	s.Require().NoError(err)

	s.Require().Len(spy.capturedView, 1, "the goblin still remembers alice")

	var payload encounter.SightPayload
	s.Require().NoError(json.Unmarshal(spy.capturedView[0].Payload, &payload))

	// POSITIVE: its own memory, at the cell it last saw her in.
	s.Equal(float64(seen.X), payload.X, "the goblin hunts where it last saw her")
	s.Equal(float64(seen.Y), payload.Y)

	// NEGATIVE: not where she actually is. A snapshot built from ground truth
	// would report this instead.
	s.NotEqual(float64(hidden.X), payload.X, "her live position must not leak into the snapshot")
}
