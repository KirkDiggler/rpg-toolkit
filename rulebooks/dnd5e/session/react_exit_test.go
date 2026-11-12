// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package session_test

// react_exit_test.go pins THE HOST'S HALF OF "NO EXIT MAY SAVE A RECORD
// THAT CANNOT LOAD" (rpg-project#942). The encounter refuses to load a
// stored pause whose walker is not in the run, and it deliberately does not
// police its own verbs while a walk is paused: the session's freeze does
// ([writeScope.frozen]). So while a walk is paused for a reaction, the
// session must refuse the paused walker's Exit and every reactor's Exit
// with ErrWindowOpen, and the stored run must load before and after. If the
// freeze is ever loosened for Exit, this fails.

import (
	"context"
	"testing"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/session"
)

// TestNobodyLeavesWhileAWalkIsPausedForAReaction runs on its own clock, with
// [ReactWindowSuite]'s scene and helpers, rather than as one more method of
// that suite.
func TestNobodyLeavesWhileAWalkIsPausedForAReaction(t *testing.T) {
	s := new(ReactWindowSuite)
	s.SetT(t)
	s.SetupTest()
	s.nobodyLeavesWhileAWalkIsPaused()
}

// nobodyLeavesWhileAWalkIsPaused: skel-1 walks out of the
// fighter's reach on its turn and the step pauses to ask her. Neither the
// paused walker nor the fighter being asked may leave; the run loads
// throughout, and once she has answered every question, she may.
func (s *ReactWindowSuite) nobodyLeavesWhileAWalkIsPaused() {
	mgr := s.twoSkeletons()
	ctx := context.Background()
	s.endTurn(mgr, "fighter")
	s.Require().Len(s.windowBeats(mgr, "fighter"), 1, "precondition: skel-1's step asks the fighter")

	_, err := mgr.LoadWorldForTest(ctx, "sess")
	s.Require().NoError(err, "the paused run loads")
	for _, leaver := range []string{"skel-1", "fighter"} {
		_, err := mgr.Exit(ctx, &session.ExitInput{Session: "sess", Member: leaver})
		s.Require().ErrorIs(err, session.ErrWindowOpen, "%s may not leave while the walk waits on the fighter", leaver)
		_, err = mgr.LoadWorldForTest(ctx, "sess")
		s.Require().NoError(err, "the run loads after %s's refused exit", leaver)
	}

	// Declining lets skel-1 walk on, and skel-2's walk then asks her too.
	s.react(mgr, "fighter", session.Decline())
	s.react(mgr, "fighter", session.Decline())
	s.Require().Empty(s.reactRow(mgr, "fighter").ID, "precondition: nothing is being asked")
	_, err = mgr.Exit(ctx, &session.ExitInput{Session: "sess", Member: "fighter"})
	s.Require().NoError(err, "answered, the fighter may leave")
	_, err = mgr.LoadWorldForTest(ctx, "sess")
	s.Require().NoError(err, "and the run she left loads")
}
