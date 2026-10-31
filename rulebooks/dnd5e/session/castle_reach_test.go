// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package session_test

// castle_reach_test.go is the QA castle's "walk up, then speak" step
// (rpg-project#639, rpg-project#753 R4): at the party start the captain is a
// gatehouse away, so his lines are not on alice's menu and a line said to him
// from there is refused for the reason it always was (he cannot see her: reach
// is asked after the witness test); she walks in, and his lines appear on
// arrival.

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/session"
)

func TestCastleWalkUpThenSpeak(t *testing.T) {
	ctx := context.Background()
	g := openTheGatehouse(14)
	speakTargets := func() map[string]bool {
		out, err := g.mgr.Afford(ctx, &session.AffordInput{Session: "sess", Member: "alice"})
		require.NoError(t, err)
		targets := map[string]bool{}
		for _, d := range out.Declarations {
			if d.Verb != session.VerbSpeak {
				continue
			}
			for _, line := range d.SpeakOptions {
				targets[line.Target] = true
			}
		}
		return targets
	}
	distanceToCaptain := func() float64 {
		enc, err := g.mgr.LoadWorldForTest(ctx, "sess")
		require.NoError(t, err)
		return enc.Distance(standingAt(enc, "alice"), standingAt(enc, "captain"))
	}

	require.Greater(t, distanceToCaptain(), float64(session.InteractionReach), "the party starts beyond the captain's reach")
	assert.False(t, speakTargets()["captain"], "so his lines are not on the menu")
	_, err := g.mgr.Speak(ctx, &session.SpeakInput{Session: "sess", Member: "alice", Target: "captain", OptionID: "stand-aside"})
	// THE ORIGINAL REASON HOLDS: from outside the gatehouse he cannot see
	// her, and reach is asked only after the witness test, so the refusal
	// names the missing audience rather than the distance.
	require.ErrorIs(t, err, session.ErrUnwitnessed, "and a line said to him from the start is refused")

	require.NoError(t, g.walkIn(), "alice walks up into the gatehouse")
	require.LessOrEqual(t, distanceToCaptain(), float64(session.InteractionReach), "now she stands within his reach")
	assert.True(t, speakTargets()["captain"], "and his lines appear on arrival")
	_, err = g.mgr.Speak(ctx, &session.SpeakInput{Session: "sess", Member: "alice", Target: "captain", OptionID: "stand-aside"})
	require.NoError(t, err, "the line is heard")
}
