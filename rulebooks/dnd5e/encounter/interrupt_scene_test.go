// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package encounter

// interrupt_scene_test.go is the play proof of an authored interrupt
// (rpg-project#1070, slice S1 of rpg-project#1061): a creature whose machine
// an author wrote to stop on a heard failed claim, stay stopped for the
// rounds the author set, and walk on. Everything is read as positions and
// machine states, never as a count of record entries. Internal, because the
// walker's machine state and the canvas are the composition's own.

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/KirkDiggler/rpg-toolkit/mind/machine"
	"github.com/KirkDiggler/rpg-toolkit/tools/spatial"
)

const (
	interruptWalker = MemberID("walker")
	// The claim the walker answers: a checked line, and a plain one.
	interruptClaim = "claim"
	interruptHello = "hello"
	interruptDC    = 12
	// How many world rounds the author makes him watch.
	interruptRounds = 3
)

// interruptTable walks on `time` toward a place far enough off that he is
// still walking after six rounds, and stands on `watch`.
func interruptTable(watchWalks bool) Table {
	far := spatial.Position{X: 59, Y: 1}
	walk := Answer{Weight: 1, Toward: &Selector{At: &far}}
	table := Table{AnswerTime: {walk}, "watch": {{Weight: 1, Hold: true}}}
	if watchWalks {
		table["watch"] = []Answer{walk}
	}

	return table
}

// interruptRecipe is the author's machine. `ready` walks; a heard failed
// claim moves him to `watching` and anchors the round; `watching` stands
// until three rounds are up, then he is ready again. dropHeard removes the
// author's `heard` step, for the mutant.
func interruptRecipe(t *testing.T, dropHeard bool) *machine.Recipe {
	t.Helper()
	failed := &machine.Guard{Fact: &machine.FactLeaf{Name: "heard.claim", Op: machine.OpEq, Value: ClaimFailed}}
	ready := []machine.Transition{
		{On: beatHeard, When: failed, To: "watching", Do: []machine.Act{{Anchor: true}}, Reason: "a claim on him failed"},
		pickTime,
		{On: beatTurn, Do: []machine.Act{{Pick: string(AnswerTime)}}},
	}
	if dropHeard {
		ready = ready[1:]
	}

	return recipe(t, "ready", map[string]machine.State{
		"ready": {Transitions: ready},
		"watching": {Transitions: []machine.Transition{
			{On: beatTime, When: &machine.Guard{Rounds: &machine.RoundsLeaf{Op: machine.OpGte, N: interruptRounds}}, To: "ready", Reason: "he looked long enough"},
			{On: beatTime, Do: []machine.Act{{Pick: "watch"}}},
			{On: beatTurn, Do: []machine.Act{{Pick: string(AnswerTime)}}},
		}},
	})
}

// interruptScene is a hall with alice and the walker within reach. The
// walker carries a checked athletics line and a plain one.
func interruptScene(t *testing.T, watchWalks, dropHeard bool) *Encounter {
	t.Helper()
	walker := monster(interruptWalker, 2, 1, interruptTable(watchWalks))
	walker.Behaviors.Instances[0].Instance.Recipe = *interruptRecipe(t, dropHeard)
	walker.Options = []Option{
		{ID: interruptClaim, Say: "Let me by.", Check: []CheckApproach{{Ability: "athletics", DC: interruptDC}},
			Success: []Answer{{Weight: 1, Say: "Go on."}}, Failure: []Answer{{Weight: 1, Say: "Not a chance."}}},
		{ID: interruptHello, Say: "Evening.", Success: []Answer{{Weight: 1, Say: "Evening."}}},
	}
	walker.Faction = postedGarrison
	// A long, low hall, the garrison neutral to alice's party: the walker is
	// not in a fight, so he lives on the world's clock.
	in := postedSetup(everyoneSeesTheWholeMap{}, walker)
	in.Field.Regions = []RegionInput{rectRegion("hall", 0, 0, 60, 3)}
	enc, err := NewEncounter(in)
	require.NoError(t, err)

	return enc
}

// interruptSay has alice say a line to the walker. A checked line is said
// beaten or missed.
func interruptSay(t *testing.T, enc *Encounter, line string, beaten bool) {
	t.Helper()
	in := &SpeakInput{Range: 3, Actor: "alice", Target: interruptWalker, OptionID: line, Roller: wakeRollsOne{}}
	if line == interruptClaim {
		in.Skill, in.Beaten, in.DC = "athletics", beaten, interruptDC
		in.Total = map[bool]int{true: interruptDC + 3, false: interruptDC - 8}[beaten]
	}
	_, err := enc.Speak(context.Background(), in)
	require.NoError(t, err)
}

// walkerAt is where the walker stands.
func walkerAt(t *testing.T, enc *Encounter) spatial.Position {
	t.Helper()
	at, ok := enc.canvas.GetEntityPosition(string(interruptWalker))
	require.True(t, ok, "the walker is on the canvas")

	return at
}

// round spends one world round and reads where and in what state he is.
func interruptRound(t *testing.T, enc *Encounter) (spatial.Position, string) {
	t.Helper()
	require.NoError(t, enc.spendWorldAction("alice"))

	return walkerAt(t, enc), stateOf(enc, interruptWalker)
}

func TestAFailedClaimStopsAWalkerUntilTheRoundsAreUp(t *testing.T) {
	enc := interruptScene(t, false, false)
	start := walkerAt(t, enc)
	require.Equal(t, "ready", stateOf(enc, interruptWalker))

	// (a) before anything is said he walks.
	at, state := interruptRound(t, enc)
	t.Logf("a: round 1 at %v state %s", at, state)
	require.NotEqual(t, start, at, "(a) he walks before anything is said")
	require.Equal(t, "ready", state)

	// He walked six cells out of reach, so alice catches up: she is placed on
	// the cell behind him, as a player who followed him would stand.
	require.NoError(t, enc.canvas.MoveEntity("alice", spatial.Position{X: at.X - 1, Y: at.Y}))

	// (b) alice's claim fails and he is watching, where he stood.
	said := walkerAt(t, enc)
	interruptSay(t, enc, interruptClaim, false)
	stood := walkerAt(t, enc)
	require.Equal(t, "watching", stateOf(enc, interruptWalker), "(b) a failed claim moves him to watching")
	require.Equal(t, said, stood, "(b) the line does not move him")

	// (c) THE ENGINE'S COUNT, which differs by one from "three rounds after
	// the line": Speak itself pays a world round, and the anchor is stamped
	// with the round the line STARTED in, so that round already counts toward
	// the author's three. One more round after the line he is still watching,
	// still where he stood.
	for i := 1; i <= interruptRounds-2; i++ {
		at, state = interruptRound(t, enc)
		t.Logf("c: round %d after the line at %v state %s", i, at, state)
		require.Equal(t, "watching", state, "(c) round %d after the line: still watching", i)
		require.Equal(t, stood, at, "(c) round %d after the line: he has not moved", i)
	}

	// (d) the next round and he is ready again, standing where he stood.
	at, state = interruptRound(t, enc)
	t.Logf("d: round %d after the line at %v state %s", interruptRounds-1, at, state)
	require.Equal(t, "ready", state, "(d) the round the author's count is met he is ready")
	require.Equal(t, stood, at, "(d) he is ready, but that round did not move him")

	// (e) one round on, he walks on.
	at, state = interruptRound(t, enc)
	t.Logf("e: round %d after the line at %v state %s", interruptRounds, at, state)
	require.Equal(t, "ready", state)
	require.NotEqual(t, stood, at, "(e) he walks on")
}

func TestAPassedClaimLeavesHimWalking(t *testing.T) {
	enc := interruptScene(t, false, false)
	interruptSay(t, enc, interruptClaim, true)
	before := walkerAt(t, enc)
	require.Equal(t, "ready", stateOf(enc, interruptWalker), "a held claim does not stop him")

	at, state := interruptRound(t, enc)
	require.Equal(t, "ready", state)
	require.NotEqual(t, before, at, "the next round moves him")
}

func TestAPlainLineLeavesHimWalking(t *testing.T) {
	enc := interruptScene(t, false, false)
	interruptSay(t, enc, interruptHello, true)
	before := walkerAt(t, enc)
	require.Equal(t, "ready", stateOf(enc, interruptWalker), "a plain line does not stop him")

	at, state := interruptRound(t, enc)
	require.Equal(t, "ready", state)
	require.NotEqual(t, before, at, "the next round moves him")
}
