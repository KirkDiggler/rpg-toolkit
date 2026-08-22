// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package session_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/session"
	"github.com/KirkDiggler/rpg-toolkit/tools/spatial"
)

// DissolveTestSuite covers ending a fight — the half of rpg-toolkit#1024 that
// a verb can fix.
type DissolveTestSuite struct {
	suite.Suite

	sessions   *fakeSessions
	encounters *fakeEncounters
	mgr        *session.Manager
}

func TestDissolveSuite(t *testing.T) {
	suite.Run(t, new(DissolveTestSuite))
}

func (s *DissolveTestSuite) SetupTest() {
	s.sessions = newFakeSessions()
	s.encounters = newFakeEncounters()
	s.mgr = managerOverRepos(s.T(), s.sessions, s.encounters)
	_, err := s.mgr.StartSession(context.Background(), &session.StartSessionInput{
		Session: "sess", Encounter: "world", World: ambushWorld(s.T()),
	})
	s.Require().NoError(err)
}

// fight walks alice into the ogre, which starts one.
func (s *DissolveTestSuite) fight() {
	out, err := s.mgr.Move(context.Background(), &session.MoveInput{
		Session: "sess", Member: "alice",
		Path: []spatial.Position{{X: 2, Y: 1}, {X: 2, Y: 2}, {X: 2, Y: 3}, {X: 2, Y: 4}},
	})
	s.Require().NoError(err)
	s.Require().NotNil(out.Formed, "the scene must actually put them in a fight")
}

// TestTheDeadEndIsClosed is why this verb exists.
//
// Fights start on their own and, until now, nothing could end one: a member
// was refused every free-roam verb with ErrInBubble, correctly and forever
// (rpg-toolkit#1024). The scene is the one the encounter module's own
// narratives use — the party sees the goblin, decides not to fight it, and
// walks away watching.
func (s *DissolveTestSuite) TestTheDeadEndIsClosed() {
	s.fight()
	ctx := context.Background()

	// The dead end, before it is closed.
	_, err := s.mgr.Move(ctx, &session.MoveInput{
		Session: "sess", Member: "alice", Path: []spatial.Position{{X: 2, Y: 3}},
	})
	s.Require().ErrorIs(err, session.ErrInBubble, "a fight member does not free-roam")

	out, err := s.mgr.Dissolve(ctx, &session.DissolveInput{
		Session: "sess", Member: "alice", Cause: session.ByDecision(),
	})
	s.Require().NoError(err)
	s.ElementsMatch([]string{"alice", "ogre"}, out.Members, "everyone in it comes back")
	s.Equal(session.DissolveByDecision, out.Cause)
	s.NotZero(out.Seq)

	// And she can walk again.
	moved, err := s.mgr.Move(ctx, &session.MoveInput{
		Session: "sess", Member: "alice", Path: []spatial.Position{{X: 2, Y: 3}},
	})
	s.Require().NoError(err, "free roam is hers again")
	s.Len(moved.Steps, 1)
}

// TestEveryoneComesBackToFreeRoam pins that the fight is gone rather than
// merely left, asked of both sides.
func (s *DissolveTestSuite) TestEveryoneComesBackToFreeRoam() {
	s.fight()
	ctx := context.Background()

	_, err := s.mgr.Dissolve(ctx, &session.DissolveInput{
		Session: "sess", Member: "ogre", Cause: session.ByDecision(),
	})
	s.Require().NoError(err, "reached through either of them — a fight has no name")

	for _, who := range []string{"alice", "ogre"} {
		turn, terr := s.mgr.Turn(ctx, &session.TurnInput{Session: "sess", Member: who})
		s.Require().NoError(terr)
		s.Equal(session.ClockWorld, turn.Clock, "%s is back in free roam", who)
		s.Empty(turn.Order, "and there is no order left to be in")
	}
}

// TestTheCauseIsRequired is the shape Kirk ruled, asserted.
//
// A fight ends either because somebody chose to leave or because the world
// noticed something. A caller that will not say which is asking for an effect
// with no account of it — and the field is what lets defeat arrive later as
// another caller rather than as a second way to end a fight.
func (s *DissolveTestSuite) TestTheCauseIsRequired() {
	s.fight()

	_, err := s.mgr.Dissolve(context.Background(), &session.DissolveInput{
		Session: "sess", Member: "alice",
	})
	s.ErrorIs(err, session.ErrNoCause)
}

// TestTheCauseSetIsSealed pins the discipline rather than the type.
//
// A bare string enum would let any caller write DissolveCause("defeat") today
// and pretend the world noticed something it cannot yet see — the composition
// has no hit points, no damage and no death. The unexported method makes a
// second case impossible to declare outside this package, so adding one means
// editing the file, and editing the file means having the caller in hand.
//
// This reads the interface's own method set, because the guarantee is
// structural: the day the seal is removed, this fails and says why.
func (s *DissolveTestSuite) TestTheCauseSetIsSealed() {
	s.NotEmpty(unexportedInterfaceMethods((*session.DissolveCause)(nil)),
		"DissolveCause must keep an unexported method: without it any caller can "+
			"declare a cause the world cannot yet notice, and defeat stops being "+
			"something earned by the caller that forces it")
}

// TestDissolvingWhatIsNotAFightIsRefused pins that this verb propagates the
// composition's rules rather than re-deciding them.
func (s *DissolveTestSuite) TestDissolvingWhatIsNotAFightIsRefused() {
	ctx := context.Background()

	_, err := s.mgr.Dissolve(ctx, &session.DissolveInput{
		Session: "sess", Member: "alice", Cause: session.ByDecision(),
	})
	s.ErrorIs(err, session.ErrNotInFight, "she is free-roaming; there is no fight to end")

	s.fight()
	_, err = s.mgr.Dissolve(ctx, &session.DissolveInput{
		Session: "sess", Member: "nobody", Cause: session.ByDecision(),
	})
	s.ErrorIs(err, session.ErrNoMember)

	_, err = s.mgr.Dissolve(ctx, &session.DissolveInput{Session: "sess", Cause: session.ByDecision()})
	s.ErrorIs(err, session.ErrNoMemberID)

	_, err = s.mgr.Dissolve(ctx, nil)
	s.ErrorIs(err, session.ErrNilInput)
}

// TestTheEndingReachesClients pins that the story carries it.
//
// EventFightEnded and its beat mapping have existed since the trigger wave —
// wired, tested, and unreachable, because nothing in this package could
// produce the beat. This is the verb that makes them real.
func (s *DissolveTestSuite) TestTheEndingReachesClients() {
	s.fight()

	stream := &fakeStream{}
	mgr, err := session.NewManager(&session.Config{
		Dice: testDice{}, TurnDriver: session.Pass{}, Sessions: s.sessions, Encounters: s.encounters,
		Characters: testCharacters(), Events: stream,
	})
	s.Require().NoError(err)

	_, err = mgr.Dissolve(context.Background(), &session.DissolveInput{
		Session: "sess", Member: "alice", Cause: session.ByDecision(),
	})
	s.Require().NoError(err)

	kinds := map[session.EventKind]int{}
	for _, e := range stream.published {
		kinds[e.Kind]++
	}
	s.Positive(kinds[session.EventFightEnded], "the fight ending is a kind, not a mystery")
	s.Zero(kinds[session.EventUnknown])
}
