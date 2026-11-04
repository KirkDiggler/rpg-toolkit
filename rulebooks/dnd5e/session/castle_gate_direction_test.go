// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package session_test

// castle_gate_direction_test.go is A POST'S DIRECTION AND PASS at the session
// (rpg-project#857, S1c of the guard holds the gate, R6): gate-1's post
// checks arrivals (`against: entering`) and lets a mover who knows
// `troupe-admitted` through. The sighting of him carries the direction and
// the post's side cells beside the held door, so a client previews which
// crossings are barred; the troupe is told no hold; the party walks out past
// him and is barred on the way back in; a both-ways test post, set in the
// stored world, bars the same party on the way out.

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/session"
	"github.com/KirkDiggler/rpg-toolkit/tools/spatial"
)

// theCourtyard is a cell well inside the courtyard, past the gate, toward
// the hall: where a step inward from the gate heads.
var theCourtyard = spatial.Position{X: -3, Y: -6}

// requireOnHisPost requires gate-1 on his cell beneath the gate in a state
// whose post holds it: posted (the recipe's initial state, which the record
// writes as empty) or alert.
func requireOnHisPost(t *testing.T, c *freeRoamCastle, msg string) {
	t.Helper()
	require.Equal(t, beneathTheCastleGate, memberOf(t, c.world(t), "gate-1").Position, msg)
	require.Contains(t, []string{"", "posted", "alert"}, stateOf(t, c.world(t), "gate-1"), msg)
}

// placeInTheRecord stands member on cell in the stored world, as the
// encounter's castle test seats its hero by its host.
func (c *freeRoamCastle) placeInTheRecord(t *testing.T, member encounter.MemberID, cell spatial.Position) {
	t.Helper()
	data, ok := c.encounters.byID["sess"]
	require.True(t, ok, "the launch saved the world")
	for i := range data.Members {
		if data.Members[i].ID == member {
			data.Members[i].Cell = &encounter.PositionData{X: cell.X, Y: cell.Y}
			return
		}
	}
	require.FailNowf(t, "not in the record", "%s", member)
}

// setTheGatePostInTheRecord is a TEST POST: gate-1's posts set to hold the
// gate against `against` in the stored world.
func (c *freeRoamCastle) setTheGatePostInTheRecord(t *testing.T, against encounter.PostDirection) {
	t.Helper()
	data := c.encounters.byID["sess"]
	for i := range data.Members {
		if data.Members[i].ID != "gate-1" {
			continue
		}
		require.NotEmpty(t, data.Members[i].Posts, "precondition: gate-1 is posted on the gate")
		for state, post := range data.Members[i].Posts {
			post.Against = against
			data.Members[i].Posts[state] = post
		}
		return
	}
	require.FailNow(t, "the record holds no gate-1")
}

// insideTheGate is the castle with alice in the courtyard cell past the
// gate, the gate door in state, and gate-1 on his post beneath it. She is
// seated there in the record and then steps a cell into the courtyard and
// back, so what she knows of the gate is what she sees from inside.
func insideTheGate(t *testing.T, gateState string, lock *encounter.LockData) *freeRoamCastle {
	t.Helper()
	c := openTheCastleForTwo(t, 18)
	c.setTheGateInTheRecord(t, gateState, lock)
	c.placeInTheRecord(t, "alice", pastTheCastleGate)
	require.Equal(t, pastTheCastleGate, c.where(t, "alice"), "precondition: alice stands in the courtyard past the gate")
	inward := c.walkInto(t, theCourtyard)
	require.NotEmpty(t, inward, "precondition: the courtyard opens past the gate")
	look := c.move(t, []spatial.Position{inward[0], pastTheCastleGate})
	require.Equal(t, session.MovementCompleted, look.Status, "precondition: a step into the courtyard and back: %s", look.StopReason)
	requireOnHisPost(t, c, "precondition: gate-1 is on his post")
	return c
}

// TestCastleTheGateGuardIsSeenToCheckArrivals: a viewer in sight of the gate
// sees which door gate-1 holds, the way he holds it, and the post's side,
// copied as the composition answers it. His ally sees no hold and so none of
// its settings.
func TestCastleTheGateGuardIsSeenToCheckArrivals(t *testing.T) {
	c := openTheCastleForTwo(t, 18)
	walked := c.move(t, c.walkInto(t, besideTheCastleGate))
	require.Equal(t, session.MovementCompleted, walked.Status, "precondition: alice beside the gate: %s", walked.StopReason)
	requireOnHisPost(t, c, "precondition: gate-1 is on his post")

	seen := c.sightingOf(t, "alice", "gate-1")
	require.Equal(t, []string{"sight"}, seen.CurrentVia, "precondition: she sees him")
	require.Equal(t, castleGateDoor, seen.HoldsDoor, "he holds the gate door")
	require.Equal(t, "entering", seen.HoldsAgainst, "against arrivals")
	require.ElementsMatch(t, []spatial.Position{besideTheCastleGate, beneathTheCastleGate}, seen.HoldsSide,
		"the post's side is the opening's two gatehouse cells")

	// ON THE WIRE the session's own names carry them.
	raw, err := json.Marshal(seen)
	require.NoError(t, err)
	var wire map[string]json.RawMessage
	require.NoError(t, json.Unmarshal(raw, &wire))
	require.JSONEq(t, `"entering"`, string(wire["holds_against"]), "holds_against carries the direction")
	require.Contains(t, wire, "holds_side", "holds_side carries the side")

	ally := c.sightingOf(t, "gate-2", "gate-1")
	require.Equal(t, "allied", ally.Stance, "precondition: gate-2 is gate-1's ally")
	require.Empty(t, ally.HoldsDoor, "an ally is held by nothing")
	require.Empty(t, ally.HoldsAgainst, "so no direction")
	require.Empty(t, ally.HoldsSide, "and no side")
}

// TestCastleTheTroupeIsToldNoHold: the captain let the troupe through, so
// alice knows the post's pass. She sees gate-1 still posted beneath the gate
// and is told no hold, and the step through the opening's other cell takes
// her into the courtyard past him.
func TestCastleTheTroupeIsToldNoHold(t *testing.T) {
	// 12 + 3 beats the stranger's 5 and the captain's 13.
	c := openTheCastleForTwo(t, 12, 12, 18)
	ctx := context.Background()
	walkBeside := func(subject encounter.MemberID) {
		if path := c.walkInto(t, memberOf(t, c.world(t), subject).Position); len(path) > 1 {
			c.move(t, path[:len(path)-1])
		}
	}
	walkBeside("pedlar")
	_, err := c.mgr.Speak(ctx, &session.SpeakInput{Session: "sess", Member: "alice", Target: "pedlar", OptionID: "tonight"})
	require.NoError(t, err)
	walkBeside("captain")
	_, err = c.mgr.Speak(ctx, &session.SpeakInput{Session: "sess", Member: "alice", Target: "captain", OptionID: "troupe"})
	require.NoError(t, err, "the captain lets the troupe through")

	walked := c.move(t, c.walkInto(t, besideTheCastleGate))
	require.Equal(t, session.MovementCompleted, walked.Status, "precondition: alice beside the gate: %s", walked.StopReason)
	requireOnHisPost(t, c, "precondition: the troupe's word leaves gate-1 on his post")
	seen := c.sightingOf(t, "alice", "gate-1")
	require.Equal(t, []string{"sight"}, seen.CurrentVia, "precondition: she sees him")
	require.Empty(t, seen.HoldsDoor, "she knows the pass, so he holds nothing against her")
	require.Empty(t, seen.HoldsAgainst, "and no direction is told")
	require.Empty(t, seen.HoldsSide, "and no side")

	c.openTheGate(t)
	cross := c.move(t, []spatial.Position{pastTheCastleGate})
	require.Equal(t, session.MovementCompleted, cross.Status, "the troupe crosses: %s", cross.StopReason)
	require.Equal(t, pastTheCastleGate, c.where(t, "alice"), "into the courtyard")
	requireOnHisPost(t, c, "past the guard, who stays posted")
}

// TestCastleTheGateLetsThePartyOut: from the courtyard, alice walks out
// through the open gate past the posted guard unchallenged, and the way
// back in is barred with his name.
func TestCastleTheGateLetsThePartyOut(t *testing.T) {
	c := insideTheGate(t, "open", nil)
	gate1 := memberOf(t, c.world(t), "gate-1")

	out := c.move(t, []spatial.Position{besideTheCastleGate})
	require.Equal(t, session.MovementCompleted, out.Status, "out of the courtyard, unchallenged: %s", out.StopReason)
	require.Equal(t, besideTheCastleGate, c.where(t, "alice"), "she stands in the gatehouse")
	requireOnHisPost(t, c, "past the guard, who stays posted")

	back := c.move(t, []spatial.Position{pastTheCastleGate})
	require.Equal(t, session.MovementStopped, back.Status, "the way back in is an arrival")
	require.Equal(t, gate1.Name+" bars the way", back.StopReason, "and names the guard she sees")
	require.Equal(t, besideTheCastleGate, c.where(t, "alice"), "she has not moved")
}

// TestCastleABothWaysPostBarsTheWayOut: a test post set both ways bars the
// same party on its way out of the courtyard, and her sighting says so.
func TestCastleABothWaysPostBarsTheWayOut(t *testing.T) {
	c := insideTheGate(t, "open", nil)
	c.setTheGatePostInTheRecord(t, encounter.PostBothWays)

	out := c.move(t, []spatial.Position{besideTheCastleGate})
	require.Equal(t, session.MovementStopped, out.Status, "a both-ways post bars the way out")
	require.Contains(t, out.StopReason, "bars the way", "as a hold, not a body")
	require.Equal(t, pastTheCastleGate, c.where(t, "alice"), "she has not moved")
}

// TestCastleTheGateDoorWorksFromInside: an entering post bars arrivals, so
// alice's hand on the gate from the courtyard is not refused: she opens it
// shut, and her Unlock of it locked faces the lock rather than the guard.
func TestCastleTheGateDoorWorksFromInside(t *testing.T) {
	t.Run("OpenDoor", func(t *testing.T) {
		c := insideTheGate(t, "closed", nil)
		c.openTheGate(t)
		requireOnHisPost(t, c, "and gate-1 stays posted")
	})

	t.Run("Unlock", func(t *testing.T) {
		lock := &encounter.LockData{Approaches: []encounter.CheckApproachData{{Ability: "dex", DC: 15}}}
		c := insideTheGate(t, "locked", lock)
		before := c.dice.rolls
		_, err := c.mgr.Unlock(context.Background(), &session.UnlockInput{Session: "sess", Member: "alice", Door: castleGateDoor})
		require.NotErrorIs(t, err, session.ErrDoorHeld, "the guard does not answer her hand from inside")
		require.NoError(t, err, "she faces the lock")
		require.Greater(t, c.dice.rolls, before, "and a die is thrown at it")
	})
}
