// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package session_test

// castle_gate_post_test.go is THE GUARD HOLDS THE GATE at the session
// (rpg-project#836 S1, B6): on the castle the api serves, gate-1 is posted on
// the gate's one crossing and his body holds it while he is posted. A player's
// walk into him stops one cell short, the Move answers `stopped`, and its
// stop reason names him. Once he has watched the captain stand aside he is
// cowed, and the crossing is walked.
//
// The captain is cowed by alice's own line on a die the test sets, the one
// door the castle has to `gate-cowed`: the composition teaches a fact only
// through an answer, a record or a door.

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/session"
	"github.com/KirkDiggler/rpg-toolkit/tools/spatial"
)

// world is the session's encounter as a read composes it.
func (c *freeRoamCastle) world(t *testing.T) *encounter.Encounter {
	t.Helper()
	enc, err := c.mgr.LoadWorldForTest(context.Background(), "sess")
	require.NoError(t, err)
	return enc
}

// memberOf is the roster row for id.
func memberOf(t *testing.T, enc *encounter.Encounter, id encounter.MemberID) encounter.Member {
	t.Helper()
	members, err := enc.Members()
	require.NoError(t, err)
	for _, m := range members {
		if m.ID == id {
			return m
		}
	}
	require.FailNowf(t, "not in the run", "%s", id)
	return encounter.Member{}
}

// stateOf is a member's machine state as the record holds it, the recipe's
// initial state written as empty.
func stateOf(t *testing.T, enc *encounter.Encounter, id encounter.MemberID) string {
	t.Helper()
	for _, m := range enc.ToData().Members {
		if m.ID == id {
			if m.Machine == nil {
				return ""
			}
			return m.Machine.State
		}
	}
	require.FailNowf(t, "not in the run", "%s", id)
	return ""
}

// walkInto is alice's route toward cell, ending on it: the gesture of
// clicking the cell itself.
func (c *freeRoamCastle) walkInto(t *testing.T, cell spatial.Position) []spatial.Position {
	t.Helper()
	route, err := c.world(t).Route(encounter.RouteInput{Mover: "alice", Policy: encounter.MoveToward, Anchor: cell, Budget: 200})
	require.NoError(t, err)
	path := route.Path
	if len(path) == 0 || path[len(path)-1] != cell {
		path = append(path, cell)
	}
	return path
}

func (c *freeRoamCastle) move(t *testing.T, path []spatial.Position) *session.MoveOutput {
	t.Helper()
	out, err := c.mgr.Move(context.Background(), &session.MoveInput{Session: "sess", Member: "alice", Path: path})
	require.NoError(t, err)
	return out
}

func (c *freeRoamCastle) where(t *testing.T, member string) spatial.Position {
	t.Helper()
	out, err := c.mgr.Where(context.Background(), &session.WhereInput{Session: "sess", Member: member})
	require.NoError(t, err)
	return out.Position
}

func TestCastleTheGuardHoldsTheGate(t *testing.T) {
	c := openTheCastleForTwo(t, 18)
	gate1 := memberOf(t, c.world(t), "gate-1")
	crossing := gate1.Position
	require.Contains(t, []string{"", "posted"}, stateOf(t, c.world(t), "gate-1"), "precondition: gate-1 is posted")

	// THE BUMP: alice walks into him. Every cell up to his is walked; his is
	// not, and the Move says who stopped her.
	path := c.walkInto(t, crossing)
	require.GreaterOrEqual(t, len(path), 2, "precondition: the walk has a cell before his")
	bump := c.move(t, path)
	require.Equal(t, session.MovementStopped, bump.Status, "the walk stops at him")
	require.Equal(t, gate1.Name+" stands in the way", bump.StopReason, "and names the guard she sees")
	short := path[len(path)-2]
	require.NotEmpty(t, bump.Steps, "the steps before him were walked")
	require.Equal(t, short, bump.Steps[len(bump.Steps)-1].Position, "the last step taken is the cell before his")
	require.Equal(t, short, c.where(t, "alice"), "she stands one cell short of him")
	require.Equal(t, crossing, memberOf(t, c.world(t), "gate-1").Position, "he holds the crossing")
	require.Equal(t, session.ClockWorld, c.bar(t, "alice").Clock, "a bump is not a fight")

	// THE CAPTAIN STANDS ASIDE with gate-1 watching: the post ends in the
	// beat he learns it.
	if toCaptain := c.walkInto(t, memberOf(t, c.world(t), "captain").Position); len(toCaptain) > 1 {
		c.move(t, toCaptain[:len(toCaptain)-1])
	}
	_, err := c.mgr.Speak(context.Background(), &session.SpeakInput{Session: "sess", Member: "alice", Target: "captain", OptionID: "stand-aside"})
	require.NoError(t, err)
	require.Equal(t, "cowed", stateOf(t, c.world(t), "gate-1"), "he stood aside in the round he learned it")

	// THE NEXT WALK CROSSES.
	cross := c.move(t, c.walkInto(t, crossing))
	require.Equal(t, session.MovementCompleted, cross.Status, "nobody stands in the way: %s", cross.StopReason)
	require.Equal(t, crossing, c.where(t, "alice"), "she stands beneath the gate")
	require.Equal(t, session.ClockWorld, c.bar(t, "alice").Clock, "nobody fights")
}
