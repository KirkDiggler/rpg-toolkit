// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package session_test

// act_listeners_test.go is the live fold's bound (rpg-project#891): a commit
// folds each recipient's acts from the verb's delta, widened only to the acts
// that delta continues, and only for the members the stream says listen —
// and a listening recipient receives exactly what the fold over its whole
// story used to hand it.

import (
	"context"
	"errors"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/session"
	"github.com/KirkDiggler/rpg-toolkit/tools/spatial"
)

// listeningCapture is an actCapture whose stream says who listens, or
// fails to.
type listeningCapture struct {
	actCapture
	listening []string
	err       error
}

func (c *listeningCapture) Listening(_ context.Context, _ string) ([]string, error) {
	return c.listening, c.err
}

// skeletonRound plays the fixture where an act outlives a commit: the
// skeleton steps once inside the fighter's reach, asks to leave it, the
// fighter declines, and the next commit continues the same walk.
func (s *ReactWindowSuite) skeletonRound(live session.ActStream) *session.Manager {
	mgr := s.skeletonWalking(live, hexCell(3, 1), hexCell(3, 2))
	s.start(mgr, []sceneSeat{seatAt("fighter", 3, 0)}, reactSkeleton("skel-1", 4, 0))
	s.endTurn(mgr, "fighter")
	s.react(mgr, "fighter", session.Decline())
	return mgr
}

// skeletonTaken is the round where the fighter takes the opportunity attack
// the skeleton's walk out of her reach offers: the window opens in one
// commit and the reaction and the rest of the walk land in the next.
func (s *ReactWindowSuite) skeletonTaken(live session.ActStream) *session.Manager {
	mgr := s.skeletonWalking(live, hexCell(5, 0), hexCell(6, 0))
	s.start(mgr, []sceneSeat{seatAt("fighter", 3, 0)}, reactSkeleton("skel-1", 4, 0))
	s.endTurn(mgr, "fighter")
	s.react(mgr, "fighter", session.Take(""))
	return mgr
}

// skeletonWalking is a manager whose skeleton walks path, acts to live.
func (s *ReactWindowSuite) skeletonWalking(live session.ActStream, path ...spatial.Position) *session.Manager {
	mgr, err := session.NewManager(&session.Config{Record: session.DiscardRecord{}, Acts: live, Seats: newFakeSeats(), CharacterConfig: newFakeCharacterConfig(),
		PresentationIDs: testPresentationIDs{}, Dice: testDice{},
		TurnDriver: &pathWalker{paths: map[string][]spatial.Position{"skel-1": path}, walked: map[string]bool{}},
		Sessions:   s.sessions, Encounters: s.encounters, Characters: s.characters,
	})
	s.Require().NoError(err)
	return mgr
}

// TestLiveFoldIsTheUnboundedFold: across every commit of the round, the acts
// the commit published are, id for id, in the same order and with the same
// bodies, the acts the fold over each recipient's WHOLE story made before
// the bound — including the commit whose walk continues an act an earlier
// commit opened, which only the widened read can fold right.
func (s *ReactWindowSuite) TestLiveFoldIsTheUnboundedFold() {
	s.liveIsUnbounded(s.skeletonRound)
}

// TestLiveFoldIsTheUnboundedFoldWhenTheReactionIsTaken is the same
// comparison over the round where the fighter takes the opportunity attack:
// the reaction lands under the walk's id in the commit after the window.
func (s *ReactWindowSuite) TestLiveFoldIsTheUnboundedFoldWhenTheReactionIsTaken() {
	s.liveIsUnbounded(s.skeletonTaken)
}

// liveIsUnbounded plays round and asserts every published act is the
// unbounded fold's, and that some commit continued an earlier act.
func (s *ReactWindowSuite) liveIsUnbounded(round func(session.ActStream) *session.Manager) {
	watch := session.WatchActFoldForTest(s.T(), "sess")
	live := &actCapture{}
	round(live)

	s.Require().NotEmpty(live.acts)
	s.Equal(watch.Unbounded(), live.acts, "the bounded fold publishes what the unbounded one did")

	reachedBack := false
	for _, fold := range watch.Folds() {
		if fold.Needed > fold.Delta {
			reachedBack = true
		}
	}
	s.True(reachedBack, "a commit continued an act begun before it, or this proves nothing about the widening")
}

// TestFoldReadsTheDeltaNotTheRecord: every recipient fold reads exactly the
// entries it needs — the delta and the acts it continues — and never the
// story before them, so a commit's reads follow the verb, not the record.
// A fold that read each member's whole story again would read more than it
// needs here.
func (s *ReactWindowSuite) TestFoldReadsTheDeltaNotTheRecord() {
	watch := session.WatchActFoldForTest(s.T(), "sess")
	s.skeletonRound(&actCapture{})

	folds := watch.Folds()
	s.Require().NotEmpty(folds)
	grown := false
	for _, fold := range folds {
		s.Equal(fold.Needed, fold.Read, "%s read what its fold needs, no more", fold.Member)
		if fold.Needed < fold.Whole {
			grown = true
		}
	}
	s.True(grown, "the story outgrew a commit's delta, or this proves nothing about the bound")
}

// TestOnlyListenersFoldLiveAndCatchUpServesTheRest: a stream that names its
// listeners gets acts for them alone; a member who was not listening is
// handed nothing live and loses nothing, since Acts folds theirs on demand.
func (s *ReactWindowSuite) TestOnlyListenersFoldLiveAndCatchUpServesTheRest() {
	watch := session.WatchActFoldForTest(s.T(), "sess")
	live := &listeningCapture{listening: []string{"fighter", "not-a-member"}}
	mgr := s.skeletonRound(live)

	for _, fold := range watch.Folds() {
		s.Equal("fighter", fold.Member, "the commit folds only for who listens")
	}
	s.Empty(live.of("skel-1"), "the skeleton was not listening")
	s.NotEmpty(actsOf(s.T(), mgr, "skel-1", "").Acts, "and catch-up still folds its acts")
	s.liveIsCatchUp(mgr, &live.actCapture, "fighter")
}

// TestAStreamThatCannotSayFoldsForEveryone: when Listening fails, the
// commit folds for every member, as a stream without ActListeners gets —
// the safe side for delivery — and the verb does not fail.
func (s *ReactWindowSuite) TestAStreamThatCannotSayFoldsForEveryone() {
	live := &listeningCapture{listening: []string{"fighter"}, err: errors.New("broker unreachable")}
	s.skeletonRound(live)

	s.NotEmpty(live.of("fighter"))
	s.NotEmpty(live.of("skel-1"), "a failed Listening folds for the members it would have left out")
}
