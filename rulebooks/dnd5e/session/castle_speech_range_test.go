// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package session_test

// castle_speech_range_test.go is rpg-project#953 at the session door: a
// line spoken at `speak` carries 30 feet into the next room through an OPEN
// doorway. The operator's run, at each their own: alice, admitted at the
// captain, WALKS back to one step inside the gate and vouches to gate-1;
// gate-2 and the cleric — the straggler in the gatehouse — catch the words,
// gate-2 recognises the cleric, and the cleric walks through the gate. With
// the gate closed the words do not carry: the cleric's spoken line from the
// gatehouse does not reach alice behind it.

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/KirkDiggler/rpg-toolkit/core"
	"github.com/KirkDiggler/rpg-toolkit/play/intel"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/session"
	"github.com/KirkDiggler/rpg-toolkit/tools/spatial"
)

var (
	// gate2InTheRun is gatehouse floor two cells from [pastTheCastleGate],
	// where gate-2 stood in the operator's run.
	gate2InTheRun = spatial.Position{X: -2, Y: 8}
	// behindTheGate is courtyard floor just behind the gate.
	behindTheGate = spatial.Position{X: -3, Y: 5}
)

// theFriendInside restores the admitted gate at each their own, gate-2
// and the cleric stood where the operator's run had them, and alice WALKED
// from where the captain admitted her to one step inside the gate — the
// gate open for her walk, then set as given.
func theFriendInside(t *testing.T, gate encounter.DoorStateKind) *freeRoamCastle {
	t.Helper()
	c := atTheAdmittedGate(t)
	c.encounters.byID["sess"].Sharing = encounter.SharingOwn
	c.setTheGateInTheRecord(t, string(encounter.DoorOpen), nil)
	c.standInTheRecord(t, "gate-2", gate2InTheRun)
	c.standInTheRecord(t, "cleric", besideGateOne)

	route, err := c.world(t).Route(encounter.RouteInput{Mover: "alice", Policy: encounter.MoveToward, Anchor: pastTheCastleGate, Budget: 200})
	require.NoError(t, err)
	path := route.Path
	if len(path) == 0 || path[len(path)-1] != pastTheCastleGate {
		path = append(path, pastTheCastleGate)
	}
	walked := c.walk(t, "alice", path)
	require.Equal(t, session.MovementCompleted, walked.Status, "precondition: alice walks in: %s", walked.StopReason)
	require.Equal(t, pastTheCastleGate, c.where(t, "alice"), "precondition: one step inside the gate")
	c.setTheGateInTheRecord(t, string(gate), nil)
	return c
}

// theVouchFromInside is [theFriendInside] with the gate open, and alice's
// vouch to gate-1. It returns the run and what the vouch sent.
func theVouchFromInside(t *testing.T) (*freeRoamCastle, beat) {
	t.Helper()
	c := theFriendInside(t, encounter.DoorOpen)
	vouched := c.play(t, "alice vouches for the cleric", func() error {
		_, err := speakAsOffered(context.Background(), c.mgr, &session.SpeakInput{
			Session: "sess", Member: "alice", Target: "gate-1", OptionID: vouchLine,
		})
		return err
	})
	return c, vouched
}

// caughtTheVouch is whether the cleric was sent the vouch's words, and
// whether gate-2's story holds them.
func caughtTheVouch(t *testing.T, c *freeRoamCastle, vouched beat) (cleric, gate2 bool) {
	t.Helper()
	cleric = len(where[session.SpeechAct](vouched.to("cleric"), func(s session.SpeechAct) bool {
		return s.Option == vouchLine && s.Words != nil && *s.Words != ""
	})) > 0
	for _, b := range beatsNamed(storyBeats(t, c.mgr, "sess", "gate-2"), encounter.BeatSpeech) {
		if b.Fields["option"] == vouchLine && b.Fields["words"] != nil && b.Fields["words"] != "" {
			gate2 = true
		}
	}
	return cleric, gate2
}

// recognises is whether holder holds the gate's `admitted` topic about
// subject in the stored world.
func (c *freeRoamCastle) recognises(holder, subject string) bool {
	_, ok := c.encounters.byID["sess"].Perception.Intel.Holdings[core.EntityID("member|"+holder)][intel.Subject("member|"+subject)]["admitted"]
	return ok
}

// TestCastleAVouchFromInsideTheGateCarries is the operator's run
// (rpg-project#953).
func TestCastleAVouchFromInsideTheGateCarries(t *testing.T) {
	c, vouched := theVouchFromInside(t)
	cleric, gate2 := caughtTheVouch(t, c, vouched)
	require.True(t, cleric, "the cleric, in the gatehouse, is sent the vouch's words")
	require.True(t, gate2, "gate-2 catches them")
	require.True(t, c.recognises("gate-2", "cleric"), "gate-2 recognises the cleric")

	route, err := c.world(t).Route(encounter.RouteInput{Mover: "cleric", Policy: encounter.MoveToward, Anchor: behindTheGate, Budget: 200})
	require.NoError(t, err)
	cross := c.walk(t, "cleric", append(route.Path, behindTheGate))
	require.Equal(t, session.MovementCompleted, cross.Status, "the cleric's walk through the gate is not refused: %s", cross.StopReason)
	require.Equal(t, behindTheGate, c.where(t, "cleric"), "she is in the yard")
}

// TestCastleSpeechThroughTheGateStopsAtAClosedGate: alice walked one step
// inside the gate, as above; the cleric, beside gate-1 in the gatehouse,
// asks after her friends, spoken. Through the open gate alice is sent the
// words; with the gate closed she is sent none. (Behind the closed gate
// gate-1 does not see alice, so the vouch itself cannot be said there:
// the closed case is the cleric's line the other way.)
func TestCastleSpeechThroughTheGateStopsAtAClosedGate(t *testing.T) {
	for _, gate := range []encounter.DoorStateKind{encounter.DoorOpen, encounter.DoorClosed} {
		t.Run(string(gate), func(t *testing.T) {
			c := theFriendInside(t, gate)
			asked := c.play(t, "the cleric asks after her friends", func() error {
				_, err := speakAsOffered(context.Background(), c.mgr, &session.SpeakInput{
					Session: "sess", Member: "cleric", Target: "gate-1", OptionID: friendsLine,
				})
				return err
			})
			heard := where[session.SpeechAct](asked.to("alice"), func(s session.SpeechAct) bool {
				return s.Option == friendsLine && s.Words != nil && *s.Words != ""
			})
			if gate == encounter.DoorOpen {
				require.NotEmpty(t, heard, "through the open gate the words carry")
				return
			}
			require.Empty(t, heard, "a closed gate stops the words")
		})
	}
}
