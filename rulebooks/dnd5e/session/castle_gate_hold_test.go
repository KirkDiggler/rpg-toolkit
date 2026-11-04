// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package session_test

// castle_gate_hold_test.go is A POST HOLDS A PASSAGE at the session
// (rpg-project#852, S1b of the guard holds the gate, R5): the castle gate is a
// two-cell opening, and gate-1's `posted` and `alert` states hold the gate
// door's opening from either gatehouse-side cell of it. On the castle the api
// serves, the launch seats gate-1 with his post; a walk through the opening's
// other cell stops at it with his name, the door standing open; the viewer's
// sighting of him says which door he holds; once he is cowed the walk
// crosses.
//
// The captain is cowed by alice's own line on a die the test sets, as
// castle_gate_post_test.go does it.

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/session"
	"github.com/KirkDiggler/rpg-toolkit/tools/spatial"
)

// The castle gate as the session's map names it: its door, the cell gate-1
// is posted on, the opening's other gatehouse-side cell, and the door's own
// cell past them. The gate's two crossings are (beside, past) and (past,
// beneath), as the atlas test below pins.
const castleGateDoor = "the-mad-king/gate-wall-door"

var (
	beneathTheCastleGate = spatial.Position{X: -3, Y: 7}
	besideTheCastleGate  = spatial.Position{X: -4, Y: 7}
	pastTheCastleGate    = spatial.Position{X: -3, Y: 6}
)

// sightingOf is viewer's sighting of subject, failing when it holds none.
func (c *freeRoamCastle) sightingOf(t *testing.T, viewer, subject string) session.Sighting {
	t.Helper()
	view, err := c.mgr.View(context.Background(), &session.ViewInput{Session: "sess", Member: viewer})
	require.NoError(t, err)
	for _, s := range view.Sightings {
		if s.Subject == subject {
			return s
		}
	}
	require.FailNowf(t, "no sighting", "%s holds no sighting of %s", viewer, subject)
	return session.Sighting{}
}

// openTheGate pushes the gate door open, alice beside it.
func (c *freeRoamCastle) openTheGate(t *testing.T) {
	t.Helper()
	out, err := c.mgr.OpenDoor(context.Background(), &session.OpenDoorInput{Session: "sess", Member: "alice", Door: castleGateDoor})
	require.NoError(t, err)
	require.Equal(t, "open", out.Door.State, "precondition: the gate stands open")
}

// TestCastleTheLaunchSeatsTheGatePost: the post the author wrote on gate-1's
// `posted` and `alert` states crosses the launch beside his blocking states.
// A launch that dropped it would seat a guard who holds one cell, and the
// party steps through beside him (the S1 walk, rpg-project#841).
func TestCastleTheLaunchSeatsTheGatePost(t *testing.T) {
	c := openTheCastleForTwo(t, 18)
	post := encounter.PostData{Door: castleGateDoor, Cells: []spatial.Position{beneathTheCastleGate, besideTheCastleGate}}
	for _, m := range c.world(t).ToData().Members {
		if m.ID == "gate-1" {
			require.Equal(t, map[string]encounter.PostData{"posted": post, "alert": post}, m.Posts,
				"gate-1 is seated with the post the author wrote")
		}
		if m.ID == "gate-2" {
			require.Nil(t, m.Posts, "gate-2 holds no post")
		}
	}
}

func TestCastleThePostHoldsTheGatesOpening(t *testing.T) {
	c := openTheCastleForTwo(t, 18)
	gate1 := memberOf(t, c.world(t), "gate-1")
	require.Equal(t, beneathTheCastleGate, gate1.Position, "precondition: gate-1 stands on his post beneath the gate")

	walked := c.move(t, c.walkInto(t, besideTheCastleGate))
	require.Equal(t, session.MovementCompleted, walked.Status, "precondition: alice reaches the post's free cell: %s", walked.StopReason)
	c.openTheGate(t)

	// WHAT SHE SEES: the guard she sees on his post holds the gate door.
	require.Equal(t, castleGateDoor, c.sightingOf(t, "alice", "gate-1").HoldsDoor, "her sighting names the door he holds")
	// His own garrison sees him on the same cell and holds nothing against it.
	ally := c.sightingOf(t, "gate-2", "gate-1")
	require.Equal(t, "allied", ally.Stance, "precondition: gate-2 is gate-1's ally")
	require.Empty(t, ally.HoldsDoor, "an ally is barred by nothing, so its sighting names no door")

	// THE OTHER CELL OF THE OPENING: the door stands open and nobody stands
	// in her way, but the step through is his to bar.
	barred := c.move(t, []spatial.Position{pastTheCastleGate})
	require.Equal(t, session.MovementStopped, barred.Status, "the held opening stops the walk")
	require.Equal(t, gate1.Name+" bars the way", barred.StopReason, "and names the guard she sees")
	require.Empty(t, barred.Steps, "no step was taken")
	require.Equal(t, besideTheCastleGate, c.where(t, "alice"), "she stands at the opening")
	require.Equal(t, session.ClockWorld, c.bar(t, "alice").Clock, "a bar is not a fight")

	// HIS OWN CELL OF THE OPENING is his body's, refused as it was before.
	body := c.move(t, []spatial.Position{beneathTheCastleGate, pastTheCastleGate})
	require.Equal(t, session.MovementStopped, body.Status, "his cell stops the walk too")
	require.Equal(t, gate1.Name+" stands in the way", body.StopReason, "as his body")
	require.Equal(t, besideTheCastleGate, c.where(t, "alice"), "she has not moved")

	// THE CAPTAIN STANDS ASIDE with gate-1 watching: he is cowed, and a cowed
	// guard holds nothing.
	if toCaptain := c.walkInto(t, memberOf(t, c.world(t), "captain").Position); len(toCaptain) > 1 {
		c.move(t, toCaptain[:len(toCaptain)-1])
	}
	_, err := c.mgr.Speak(context.Background(), &session.SpeakInput{Session: "sess", Member: "alice", Target: "captain", OptionID: "stand-aside"})
	require.NoError(t, err)
	require.Equal(t, "cowed", stateOf(t, c.world(t), "gate-1"), "precondition: he stood aside")
	cowed := c.sightingOf(t, "alice", "gate-1")
	require.Equal(t, []string{"sight"}, cowed.CurrentVia, "precondition: she still sees him")
	require.Empty(t, cowed.HoldsDoor, "a cowed guard holds no door")

	// THE NEXT WALK CROSSES the opening's free cell.
	if c.where(t, "alice") != besideTheCastleGate {
		back := c.move(t, c.walkInto(t, besideTheCastleGate))
		require.Equal(t, session.MovementCompleted, back.Status, "precondition: back beside the gate: %s", back.StopReason)
	}
	cross := c.move(t, []spatial.Position{pastTheCastleGate})
	require.Equal(t, session.MovementCompleted, cross.Status, "nobody bars the way: %s", cross.StopReason)
	require.Equal(t, pastTheCastleGate, c.where(t, "alice"), "she stands in the gate")
}

// TestCastleAMemoryOfTheGateGuardHoldsNoDoor: what a viewer remembers of a
// holder says nothing about what he holds now. The hold is a state at a
// place, read while she sees him, never carried into a memory.
func TestCastleAMemoryOfTheGateGuardHoldsNoDoor(t *testing.T) {
	c := openTheCastleForTwo(t, 18)
	start := c.where(t, "alice")
	c.move(t, c.walkInto(t, besideTheCastleGate))
	require.Equal(t, castleGateDoor, c.sightingOf(t, "alice", "gate-1").HoldsDoor, "precondition: seen on his post, he holds the gate")

	away := c.move(t, c.walkInto(t, start))
	require.Equal(t, session.MovementCompleted, away.Status, "precondition: alice walks back out: %s", away.StopReason)
	memory := c.sightingOf(t, "alice", "gate-1")
	require.Empty(t, memory.CurrentVia, "precondition: she remembers him rather than sees him")
	require.Empty(t, memory.HoldsDoor, "a memory names no door")
	require.Equal(t, "posted", stateOrPosted(stateOf(t, c.world(t), "gate-1")), "and he is still posted")
}

// stateOrPosted reads the castle guard's empty recorded state as its
// initial one.
func stateOrPosted(state string) string {
	if state == "" {
		return "posted"
	}
	return state
}
