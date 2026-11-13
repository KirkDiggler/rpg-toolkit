// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package session_test

// castle_gate_bump_test.go is THE BUMP SPEAKS at the session (rpg-project#842,
// S2 of the guard holds the gate, design R2 and R7): a player's attempt a
// body stops is a `barred` beat the body hears at once. The session folds
// the beat into a BarredAct, answers the Move with Barred and a door verb
// with Refused (never an error), and the guard's line reaches the act stream
// as any table pick does. Each test plays one verb and reads the acts it
// published.

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/session"
	"github.com/KirkDiggler/rpg-toolkit/tools/spatial"
)

// The castle's two lines for gate-1 (the-mad-king.yaml, rpg-project#842):
// the first bump's question, and the shorter one while he is challenged.
const (
	theGuardsQuestion = "Whoa there, buddy. Can I help you?"
	theGuardsRepeat   = "I said, can I help you?"
)

// alice and gate1 are the two parties every bump names.
var (
	aliceParty = session.Party{Kind: session.PartyMember, ID: "alice"}
	gate1Party = session.Party{Kind: session.PartyMember, ID: "gate-1"}
)

// throughTheGateWall is the gate's wall cell north of alice beside the gate:
// a step there is refused by the wall, never a body.
var throughTheGateWall = spatial.Position{X: -4, Y: 6}

// guardSays is gate-1's one line in acts: the words of his one Speech act
// (rpg-project#934).
func guardSays(t *testing.T, acts []session.Act) string {
	t.Helper()
	lines := saidBy(acts, gate1Party)
	require.Len(t, lines, 1, "gate-1 says one line")
	return *lines[0].body.Words
}

// saidBy is every Speech act in acts that names who as its speaker and
// carries their words.
func saidBy(acts []session.Act, who session.Party) []found[session.SpeechAct] {
	return where[session.SpeechAct](acts, func(s session.SpeechAct) bool { return s.Speaker == who && s.Words != nil })
}

// answeredUnder requires gate-1's answer to bumped to share its act
// correlation (rpg-project#842): the barred act, his state change and his
// line are one act on the stream, never a fresh `time` act, because the web
// keys the bubble on it.
func answeredUnder(t *testing.T, acts []session.Act, bumped found[session.BarredAct]) {
	t.Helper()
	require.NotEmpty(t, bumped.act.Act, "the bump carries an act correlation")
	answers := saidBy(acts, gate1Party)
	require.Len(t, answers, 1, "gate-1 says one line")
	require.Equal(t, bumped.act.Act, answers[0].act.Act, "his line shares the bump's act")
	require.Equal(t, bumped.act.Verb, answers[0].act.Verb, "under the same verb")
	for _, st := range where[session.StateChange](acts, func(s session.StateChange) bool { return s.Party == gate1Party }) {
		require.Equal(t, bumped.act.Act, st.act.Act, "his state change shares the bump's act")
	}
}

// bump is alice's Move through gate-1's cell, played: the walk aims past
// him, as a click on the courtyard does.
func (c *freeRoamCastle) bump(t *testing.T, path ...spatial.Position) (*session.MoveOutput, beat) {
	t.Helper()
	var out *session.MoveOutput
	played := c.play(t, "bump", func() error {
		var err error
		out, err = walkOn(context.Background(), c.mgr, &session.MoveInput{Session: "sess", Member: "alice", Path: path})
		return err
	})
	return out, played
}

// besideTheGate walks alice to the opening's free cell beside gate-1.
func (c *freeRoamCastle) besideTheGate(t *testing.T) {
	t.Helper()
	walked := c.move(t, c.walkInto(t, besideTheCastleGate))
	require.Equal(t, session.MovementCompleted, walked.Status, "precondition: alice beside the gate: %s", walked.StopReason)
}

// TestCastleTheBumpSpeaks: alice walks into the posted gate-1. The Move
// answers stopped, with Barred naming him and the cell she was refused
// into, and she has not moved. The act stream carries the bump as a
// BarredAct, his turn to challenged, and his question. A second bump while
// he is challenged gets the shorter line.
func TestCastleTheBumpSpeaks(t *testing.T) {
	c := openTheGateWithGateOneAlone(t, 18)
	gate1 := memberOf(t, c.world(t), "gate-1")
	c.besideTheGate(t)

	out, first := c.bump(t, beneathTheCastleGate, pastTheCastleGate)
	require.Equal(t, session.MovementStopped, out.Status, "the walk stops at him")
	require.Equal(t, &session.Barred{Barrier: "gate-1", Cell: beneathTheCastleGate, Reason: gate1.Name + " stands in the way"}, out.Barred,
		"Barred names him, his cell, and the reason")
	require.Equal(t, out.Barred.Reason, out.StopReason, "the stop reason is the bump's")
	require.Empty(t, out.Steps, "she took no step")
	require.Nil(t, out.Formed, "a bump is not a fight")
	require.Equal(t, besideTheCastleGate, c.where(t, "alice"), "she stands where she stood")
	require.Equal(t, session.ClockWorld, c.bar(t, "alice").Clock, "and roams free")

	alice := first.to("alice")
	bumped := only[session.BarredAct](t, alice)
	require.Equal(t, session.BarredAct{Mover: aliceParty, Barrier: gate1Party, Cell: &beneathTheCastleGate}, bumped.body,
		"the bump names her, him, and the cell, and no door")
	require.Equal(t, "move", bumped.act.Verb, "under her Move")
	turned := where[session.StateChange](alice, func(s session.StateChange) bool { return s.Change == session.StateMachine })
	require.Len(t, turned, 1, "his machine turns once")
	require.Equal(t, "challenged", turned[0].body.Machine.To, "he is challenged")
	require.Equal(t, theGuardsQuestion, guardSays(t, alice), "and asks what she wants")
	spokenTo := saidBy(alice, gate1Party)
	require.Equal(t, aliceParty, spokenTo[0].body.SpokenTo, "his line is spoken to the bumper (rpg-project#923)")
	answeredUnder(t, alice, bumped)
	require.Equal(t, "challenged", stateOf(t, c.world(t), "gate-1"), "the record holds him challenged")

	again, second := c.bump(t, beneathTheCastleGate, pastTheCastleGate)
	require.Equal(t, session.MovementStopped, again.Status, "challenged, he still holds")
	require.NotNil(t, again.Barred, "and the second bump is barred")
	again2 := only[session.BarredAct](t, second.to("alice"))
	require.Equal(t, theGuardsRepeat, guardSays(t, second.to("alice")), "the second bump gets the shorter line")
	answeredUnder(t, second.to("alice"), again2)
	require.NotEqual(t, bumped.act.Act, again2.act.Act, "each bump is its own act")
}

// TestCastleTheHeldCrossingIsABump: the opening's free cell is his to bar.
// A step across it is the bump too, and the cell is the one she was
// refused into, past the gate, never his own.
func TestCastleTheHeldCrossingIsABump(t *testing.T) {
	c := openTheGateWithGateOneAlone(t, 18)
	gate1 := memberOf(t, c.world(t), "gate-1")
	c.besideTheGate(t)
	c.openTheGateInTheRecord(t)

	out, played := c.bump(t, pastTheCastleGate)
	require.Equal(t, session.MovementStopped, out.Status)
	require.Equal(t, &session.Barred{Barrier: "gate-1", Cell: pastTheCastleGate, Reason: gate1.Name + " bars the way"}, out.Barred,
		"the refused cell is past the gate")
	require.Equal(t, session.BarredAct{Mover: aliceParty, Barrier: gate1Party, Cell: &pastTheCastleGate},
		only[session.BarredAct](t, played.to("alice")).body)
	require.Equal(t, theGuardsQuestion, guardSays(t, played.to("alice")))
}

// TestCastleAWallBumpWritesNothing: a wall refuses the step in its own
// words, and no one hears it. No Barred, no BarredAct, and gate-1, a cell
// away, says nothing and stays posted.
func TestCastleAWallBumpWritesNothing(t *testing.T) {
	c := openTheGateWithGateOneAlone(t, 18)
	c.besideTheGate(t)
	// Whatever state gate-1 is in by now — the captain's spoken hail no
	// longer alerts him, guards answer shouts (rpg-project#933) — the wall
	// changes nothing.
	before := stateOrPosted(stateOf(t, c.world(t), "gate-1"))

	out, played := c.bump(t, throughTheGateWall, spatial.Position{X: -4, Y: 5})
	require.Equal(t, session.MovementStopped, out.Status, "the wall stops the walk")
	require.Nil(t, out.Barred, "a wall is no body")
	require.NotContains(t, out.StopReason, "gate-1", "and its reason names no one")
	require.Empty(t, where[session.BarredAct](played.acts, func(session.BarredAct) bool { return true }), "nothing is written")
	require.Empty(t, where[session.TableAnswer](played.acts, func(session.TableAnswer) bool { return true }), "nobody answers")
	require.Equal(t, before, stateOrPosted(stateOf(t, c.world(t), "gate-1")), "the wall leaves him as he was")
}

// TestCastleOpeningTheHeldDoorIsABump: OpenDoor on the gate he holds is
// answered with Refused, the door shut, and the act stream carries the
// bump naming the door and the verb, and his question.
func TestCastleOpeningTheHeldDoorIsABump(t *testing.T) {
	c := openTheGateWithGateOneAlone(t, 18)
	gate1 := memberOf(t, c.world(t), "gate-1")
	c.besideTheGate(t)

	var out *session.OpenDoorOutput
	played := c.play(t, "open the held gate", func() error {
		var err error
		out, err = c.mgr.OpenDoor(context.Background(), &session.OpenDoorInput{Session: "sess", Member: "alice", Door: castleGateDoor})
		return err
	})
	require.Equal(t, &session.DoorRefusal{Holder: "gate-1", Reason: gate1.Name + " bars the door"}, out.Refused)
	require.Equal(t, "closed", out.Door.State, "the gate stays shut")
	require.Equal(t, "closed", c.gateStateInTheRecord(t), "in the record too")
	bumped := only[session.BarredAct](t, played.to("alice"))
	require.Equal(t, session.BarredAct{Mover: aliceParty, Barrier: gate1Party, Door: castleGateDoor, Verb: "open"}, bumped.body)
	require.Equal(t, "open_door", bumped.act.Verb, "under her OpenDoor")
	require.Equal(t, theGuardsQuestion, guardSays(t, played.to("alice")))
	answeredUnder(t, played.to("alice"), bumped)
}

// TestCastleClosingTheHeldDoorIsABump: CloseDoor on the open gate he holds
// is answered with Refused, the door still open, and the act stream carries
// the bump naming the door and the close, and his question.
func TestCastleClosingTheHeldDoorIsABump(t *testing.T) {
	c := openTheGateWithGateOneAlone(t, 18)
	gate1 := memberOf(t, c.world(t), "gate-1")
	c.besideTheGate(t)
	c.openTheGateInTheRecord(t)

	var out *session.CloseDoorOutput
	played := c.play(t, "close the held gate", func() error {
		var err error
		out, err = c.mgr.CloseDoor(context.Background(), &session.CloseDoorInput{Session: "sess", Member: "alice", Door: castleGateDoor})
		return err
	})
	require.Equal(t, &session.DoorRefusal{Holder: "gate-1", Reason: gate1.Name + " bars the door"}, out.Refused)
	require.Equal(t, "open", out.Door.State, "the gate stays open")
	require.Equal(t, "open", c.gateStateInTheRecord(t), "in the record too")
	bumped := only[session.BarredAct](t, played.to("alice"))
	require.Equal(t, session.BarredAct{Mover: aliceParty, Barrier: gate1Party, Door: castleGateDoor, Verb: "close"}, bumped.body)
	require.Equal(t, "close_door", bumped.act.Verb, "under her CloseDoor")
	require.Equal(t, theGuardsQuestion, guardSays(t, played.to("alice")))
	answeredUnder(t, played.to("alice"), bumped)
}

// TestCastleAClickOnTheGuardIsABump (rpg-project#835 R8): a click on a
// blocking body sends a Move with his cell as the goal. The session walks
// it, the step into him is the bump, and she stops on the cell before him.
// From beside him that is no step at all. From the party start the walk
// goes as far as the cell before him. Either way the bump, his turn and his
// line are one act under her Move.
func TestCastleAClickOnTheGuardIsABump(t *testing.T) {
	t.Run("from beside him", func(t *testing.T) {
		c := openTheGateWithGateOneAlone(t, 18)
		gate1 := memberOf(t, c.world(t), "gate-1")
		c.besideTheGate(t)

		out, played := c.bump(t, beneathTheCastleGate)
		require.Equal(t, session.MovementStopped, out.Status)
		require.Equal(t, &session.Barred{Barrier: "gate-1", Cell: beneathTheCastleGate, Reason: gate1.Name + " stands in the way"}, out.Barred)
		require.Empty(t, out.Steps, "no step past where she stood")
		require.Equal(t, besideTheCastleGate, c.where(t, "alice"))
		bumped := only[session.BarredAct](t, played.to("alice"))
		require.Equal(t, "move", bumped.act.Verb, "under her Move")
		require.Equal(t, theGuardsQuestion, guardSays(t, played.to("alice")))
		answeredUnder(t, played.to("alice"), bumped)
	})
	t.Run("from the party start", func(t *testing.T) {
		c := openTheGateWithGateOneAlone(t, 18)
		path := c.walkInto(t, beneathTheCastleGate)
		require.GreaterOrEqual(t, len(path), 2, "precondition: the walk has a cell before his")
		before := path[len(path)-2]

		out, played := c.bump(t, path...)
		require.Equal(t, session.MovementStopped, out.Status)
		require.NotNil(t, out.Barred, "the last step is the bump")
		require.Equal(t, beneathTheCastleGate, out.Barred.Cell)
		require.Len(t, out.Steps, len(path)-1, "every cell before his is walked")
		require.Equal(t, before, c.where(t, "alice"), "she stops on the cell before him")
		bumped := only[session.BarredAct](t, played.to("alice"))
		answeredUnder(t, played.to("alice"), bumped)
	})
}

// TestCastleAClickOnAPassableBodyIsRefused: the goal R8 opens is a cell a
// body blocks. A body she may pass but not stop on, her ally the cleric or
// the neutral pedlar, is still refused before any step, as the composition's
// ValidateWalkEnd answers: ErrBadPosition, she has not moved, and nothing is
// written. Neither hears a bump.
func TestCastleAClickOnAPassableBodyIsRefused(t *testing.T) {
	for _, who := range []string{"cleric", "pedlar"} {
		t.Run(who, func(t *testing.T) {
			c := openTheGateWithGateOneAlone(t, 18)
			start := c.where(t, "alice")
			path := c.walkInto(t, memberOf(t, c.world(t), encounter.MemberID(who)).Position)
			c.live.mu.Lock()
			c.live.acts = nil
			c.live.mu.Unlock()

			_, err := walkOn(context.Background(), c.mgr, &session.MoveInput{Session: "sess", Member: "alice", Path: path})
			require.ErrorIs(t, err, session.ErrBadPosition, "a passable body's cell is no place to stop")
			require.Equal(t, start, c.where(t, "alice"), "and she has not moved")
			c.live.mu.Lock()
			defer c.live.mu.Unlock()
			require.Empty(t, where[session.BarredAct](c.live.acts, func(session.BarredAct) bool { return true }), "no bump is written")
		})
	}
}
