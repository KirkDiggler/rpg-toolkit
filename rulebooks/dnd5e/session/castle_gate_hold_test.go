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
	"regexp"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/abilities"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/character"
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

// openTheGateInTheRecord stands the gate door open in the stored world, as
// the encounter's castle test does through its host doors: a held door
// refuses every stranger's hand (R7), so nobody at the table can open it
// while gate-1 holds it.
func (c *freeRoamCastle) openTheGateInTheRecord(t *testing.T) {
	t.Helper()
	c.setTheGateInTheRecord(t, "open", nil)
}

// setTheGateInTheRecord stands the gate door in state in the stored world,
// with lock when it is locked.
func (c *freeRoamCastle) setTheGateInTheRecord(t *testing.T, state string, lock *encounter.LockData) {
	t.Helper()
	data, ok := c.encounters.byID["sess"]
	require.True(t, ok, "the launch saved the world")
	found := false
	for i := range data.Doors {
		if data.Doors[i].ID == castleGateDoor {
			data.Doors[i].State, data.Doors[i].Lock, found = state, lock, true
		}
	}
	require.True(t, found, "precondition: the record holds the gate door")
}

// gateStateInTheRecord is the gate door's stored state.
func (c *freeRoamCastle) gateStateInTheRecord(t *testing.T) string {
	t.Helper()
	for _, d := range c.encounters.byID["sess"].Doors {
		if d.ID == castleGateDoor {
			return d.State
		}
	}
	require.FailNow(t, "the record holds no gate door")
	return ""
}

// heldRefusal requires a door verb's answer to be refused by a hold
// (rpg-project#842): no error, the refusal on the output naming gate-1, and
// returns its reason.
func heldRefusal(t *testing.T, err error, refused *session.DoorRefusal) string {
	t.Helper()
	require.NoError(t, err, "a held door answers her hand, never errs")
	require.NotNil(t, refused, "a held door refuses her hand")
	require.Equal(t, "gate-1", refused.Holder, "and names the guard who holds it")
	return refused.Reason
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
	// With its settings (rpg-project#857, R6; #928): it checks arrivals, and
	// its pass is recognition — he lets through whom he recognises as
	// admitted. The dungeonspec compiles them into the post itself, so the
	// launch carries them by carrying the post.
	recognized := []encounter.TriggerData{{Kind: "recognized", Topic: "admitted"}}
	post := encounter.PostData{
		Door: castleGateDoor, Cells: []spatial.Position{beneathTheCastleGate, besideTheCastleGate},
		Against: encounter.PostEntering, Pass: recognized,
	}
	posted := map[encounter.MemberID]bool{}
	for _, m := range c.world(t).ToData().Members {
		switch m.ID {
		case "gate-1":
			require.Equal(t, map[string]encounter.PostData{"posted": post, "alert": post, "challenged": post}, m.Posts,
				"gate-1 is seated with the post the author wrote")
		case "gate-2", "captain":
			// Every gate guard holds the gate (rpg-project#928), in `ready`.
			require.Contains(t, m.Posts, "ready", "%s is seated with a post", m.ID)
			require.Equal(t, castleGateDoor, string(m.Posts["ready"].Door))
			require.Equal(t, recognized, m.Posts["ready"].Pass)
		default:
			continue
		}
		posted[m.ID] = true
	}
	for _, id := range []encounter.MemberID{"gate-1", "gate-2", "captain"} {
		require.True(t, posted[id], "%s is seated", id)
	}
}

func TestCastleThePostHoldsTheGatesOpening(t *testing.T) {
	c := openTheGateWithGateOneAlone(t, 18)
	gate1 := memberOf(t, c.world(t), "gate-1")
	require.Equal(t, beneathTheCastleGate, gate1.Position, "precondition: gate-1 stands on his post beneath the gate")

	walked := c.move(t, c.walkInto(t, besideTheCastleGate))
	require.Equal(t, session.MovementCompleted, walked.Status, "precondition: alice reaches the post's free cell: %s", walked.StopReason)

	// SHUT, the opening is still his: the hold is read before the door, so
	// the stop names him and never the door.
	shut := c.move(t, []spatial.Position{pastTheCastleGate})
	require.Equal(t, session.MovementStopped, shut.Status, "the held opening stops the walk with the door shut")
	require.Equal(t, gate1.Name+" bars the way", shut.StopReason, "and names him, not the door")
	require.Equal(t, besideTheCastleGate, c.where(t, "alice"), "she stands at the opening")

	c.openTheGateInTheRecord(t)
	require.Equal(t, "open", worldGateState(t, c), "precondition: the gate stands open in the world")

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
	_, err := speakAsOffered(context.Background(), c.mgr, &session.SpeakInput{Session: "sess", Member: "alice", Target: "captain", OptionID: "stand-aside"})
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
	c := openTheGateWithGateOneAlone(t, 18)
	start := c.where(t, "alice")
	c.move(t, c.walkInto(t, besideTheCastleGate))
	require.Equal(t, castleGateDoor, c.sightingOf(t, "alice", "gate-1").HoldsDoor, "precondition: seen on his post, he holds the gate")
	// Whatever state gate-1 is in by now: the captain's spoken hail no
	// longer alerts him, guards answer shouts (rpg-project#933).
	before := stateOrPosted(stateOf(t, c.world(t), "gate-1"))

	away := c.move(t, c.walkInto(t, start))
	require.Equal(t, session.MovementCompleted, away.Status, "precondition: alice walks back out: %s", away.StopReason)
	memory := c.sightingOf(t, "alice", "gate-1")
	require.Empty(t, memory.CurrentVia, "precondition: she remembers him rather than sees him")
	require.Empty(t, memory.HoldsDoor, "a memory names no door")
	require.Equal(t, before, stateOrPosted(stateOf(t, c.world(t), "gate-1")), "and walking away leaves him as he was")
}

// stateOrPosted reads the castle guard's empty recorded state as its
// initial one.
func stateOrPosted(state string) string {
	if state == "" {
		return "posted"
	}
	return state
}

// TestCastleAHeldDoorRefusesItsDoorVerbs (rpg-project#852, R7): a held door
// refuses OpenDoor and CloseDoor to anyone not allied with a holder, and the
// refusal says who bars it as she sees him, in the encounter's words. Once
// he is cowed, she opens it.
func TestCastleAHeldDoorRefusesItsDoorVerbs(t *testing.T) {
	c := openTheGateWithGateOneAlone(t, 18)
	gate1 := memberOf(t, c.world(t), "gate-1")
	walked := c.move(t, c.walkInto(t, besideTheCastleGate))
	require.Equal(t, session.MovementCompleted, walked.Status, "precondition: alice beside the gate: %s", walked.StopReason)
	barred := gate1.Name + " bars the door"

	opened, err := c.mgr.OpenDoor(context.Background(), &session.OpenDoorInput{Session: "sess", Member: "alice", Door: castleGateDoor})
	require.NoError(t, err)
	require.Equal(t, barred, heldRefusal(t, err, opened.Refused), "OpenDoor names the guard she sees")
	require.Equal(t, "closed", opened.Door.State, "the answer says the door is still shut")
	require.Equal(t, "closed", c.gateStateInTheRecord(t), "and the gate stays shut")

	c.openTheGateInTheRecord(t)
	require.Equal(t, "open", worldGateState(t, c), "precondition: the gate stands open in the world")
	closed, err := c.mgr.CloseDoor(context.Background(), &session.CloseDoorInput{Session: "sess", Member: "alice", Door: castleGateDoor})
	require.NoError(t, err)
	require.Equal(t, barred, heldRefusal(t, err, closed.Refused), "CloseDoor is refused the same way")
	require.Equal(t, "open", closed.Door.State, "the answer says the door still stands open")
	require.Equal(t, "open", c.gateStateInTheRecord(t), "and the gate stays open")
	c.setTheGateInTheRecord(t, "closed", nil)

	if toCaptain := c.walkInto(t, memberOf(t, c.world(t), "captain").Position); len(toCaptain) > 1 {
		c.move(t, toCaptain[:len(toCaptain)-1])
	}
	_, err = speakAsOffered(context.Background(), c.mgr, &session.SpeakInput{Session: "sess", Member: "alice", Target: "captain", OptionID: "stand-aside"})
	require.NoError(t, err)
	require.Equal(t, "cowed", stateOf(t, c.world(t), "gate-1"), "precondition: he stood aside")
	if c.where(t, "alice") != besideTheCastleGate {
		c.move(t, c.walkInto(t, besideTheCastleGate))
	}
	c.openTheGate(t)
}

// TestCastleAHeldLockedDoorIsAnsweredByTheGuard (rpg-project#852, R7; #855
// review): the gate locked and held, alice's Unlock is refused by the guard
// before the lock is faced. No die is thrown, no Guidance pause is posed,
// and the lock stays as it was. The attempt is the bump (rpg-project#842):
// the verb is called, the encounter writes `barred` naming the door and the
// verb, and gate-1 answers with his line.
func TestCastleAHeldLockedDoorIsAnsweredByTheGuard(t *testing.T) {
	c := openTheGateWithGateOneAlone(t, 18)
	gate1 := memberOf(t, c.world(t), "gate-1")
	// LOCKED BEFORE SHE ARRIVES: Unlock reads the lock through what she
	// knows of the door, so she sees it locked on her way in.
	lock := &encounter.LockData{Approaches: []encounter.CheckApproachData{{Ability: "dex", DC: 15}}}
	c.setTheGateInTheRecord(t, "locked", lock)
	walked := c.move(t, c.walkInto(t, besideTheCastleGate))
	require.Equal(t, session.MovementCompleted, walked.Status, "precondition: alice beside the gate: %s", walked.StopReason)
	known, err := c.mgr.Doors(context.Background(), &session.DoorsInput{Session: "sess", Member: "alice"})
	require.NoError(t, err)
	i := slicesIndex(known.Doors, castleGateDoor)
	require.GreaterOrEqual(t, i, 0, "precondition: she knows the gate")
	require.Equal(t, "locked", known.Doors[i].State, "precondition: she knows it locked, so an unheld attempt would roll")
	require.NotNil(t, known.Doors[i].Lock, "precondition: with its lock")

	before := c.dice.checks
	var out *session.UnlockOutput
	bump := c.play(t, "unlock the held gate", func() error {
		out, err = c.mgr.Unlock(context.Background(), &session.UnlockInput{Session: "sess", Member: "alice", Door: castleGateDoor})
		return err
	})
	require.Equal(t, gate1.Name+" bars the door", heldRefusal(t, err, out.Refused), "the guard answers, not the lock")
	require.False(t, out.Paused, "no attempt, so no pause")
	require.False(t, out.Beaten, "and no verdict")
	require.Zero(t, out.DC, "and no DC faced")
	require.Equal(t, "locked", out.Door.State, "the answer says the lock stands")
	require.Equal(t, before, c.dice.checks, "no check is rolled at a held lock (his answer throws only its table die)")
	require.Equal(t, "locked", c.gateStateInTheRecord(t), "and the lock stands")

	// THE ATTEMPT IS THE BUMP: written, folded, and answered.
	bumped := only[session.BarredAct](t, bump.to("alice"))
	barred := bumped.body
	require.Equal(t, session.BarredAct{
		Mover: session.Party{Kind: session.PartyMember, ID: "alice"}, Barrier: session.Party{Kind: session.PartyMember, ID: "gate-1"},
		Door: castleGateDoor, Verb: "unlock",
	}, barred, "the bump names the door and the verb, and no cell")
	require.Equal(t, theGuardsQuestion, guardSays(t, bump.to("alice")), "and he asks what she wants")
	require.Equal(t, "unlock", bumped.act.Verb, "under her Unlock")
	answeredUnder(t, bump.to("alice"), bumped)
}

// slicesIndex is the index of door id in doors, failing the caller's
// precondition with -1 when absent.
func slicesIndex(doors []session.Door, id string) int {
	for i, d := range doors {
		if d.ID == id {
			return i
		}
	}
	return -1
}

// worldGateState is the gate door's state in the world as a read composes
// it.
func worldGateState(t *testing.T, c *freeRoamCastle) string {
	t.Helper()
	for _, d := range c.world(t).ToData().Doors {
		if d.ID == castleGateDoor {
			return d.State
		}
	}
	require.FailNow(t, "the world holds no gate door")
	return ""
}

// TestCastleAHeldDoorOutOfReachIsOutOfReach (rpg-project#852, R7): the
// verbs' order holds at the session too. Alice sees the held, locked gate
// from cells away: her Unlock is out of reach, not barred, and no die is
// thrown.
func TestCastleAHeldDoorOutOfReachIsOutOfReach(t *testing.T) {
	c := openTheGateWithGateOneAlone(t, 18)
	lock := &encounter.LockData{Approaches: []encounter.CheckApproachData{{Ability: "dex", DC: 15}}}
	c.setTheGateInTheRecord(t, "locked", lock)
	path := c.walkInto(t, besideTheCastleGate)
	require.GreaterOrEqual(t, len(path), 4, "precondition: the walk to the gate is long enough to stop short")
	short := c.move(t, path[:len(path)-3])
	require.Equal(t, session.MovementCompleted, short.Status, "precondition: alice stops short: %s", short.StopReason)
	known, err := c.mgr.Doors(context.Background(), &session.DoorsInput{Session: "sess", Member: "alice"})
	require.NoError(t, err)
	require.GreaterOrEqual(t, slicesIndex(known.Doors, castleGateDoor), 0, "precondition: she knows the gate")

	before := c.dice.rolls
	_, err = c.mgr.Unlock(context.Background(), &session.UnlockInput{Session: "sess", Member: "alice", Door: castleGateDoor})
	require.ErrorIs(t, err, session.ErrOutOfRange, "a lock across the gatehouse is out of reach first")
	require.NotErrorIs(t, err, session.ErrDoorHeld, "and not barred, which she would only learn at the door")
	require.Equal(t, before, c.dice.rolls, "no die is thrown")
}

// gateOneAlonePosts is the castle's `ready` posts on gate-2 and the captain
// (rpg-project#928), which [openTheGateWithGateOneAlone] strikes.
var gateOneAlonePosts = regexp.MustCompile(`\n        posts:\n          ready:\n(?:            .*\n)+`)

// openTheGateWithGateOneAlone is [openTheCastleForTwo] with gate-1 the gate's
// only holder: a TEST FIXTURE for the routes about one post's mechanics. In
// the file every gate guard holds the gate (rpg-project#928), and the
// captain, standing nearest, would name every stop these routes read as
// gate-1's; the three holders together are castle_gate_recognition_test.go's.
func openTheGateWithGateOneAlone(t *testing.T, d20s ...int) *freeRoamCastle {
	t.Helper()
	source := castleSource(t)
	stripped := gateOneAlonePosts.ReplaceAll(source, []byte("\n"))
	require.Equal(t, 2, len(gateOneAlonePosts.FindAll(source, -1)), "precondition: gate-2 and the captain each carry a ready post")
	alice := armedFighter("alice")
	alice.AbilityScores[abilities.CHA] = 16
	return openTheCastleFrom(t, stripped, []*character.Data{alice, healingCleric()}, d20s...)
}
