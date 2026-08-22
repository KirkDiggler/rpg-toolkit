// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package session_test

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/session"
	"github.com/KirkDiggler/rpg-toolkit/tools/spatial"
)

// TurnTestSuite covers the per-member turn read and EndTurn.
type TurnTestSuite struct {
	suite.Suite

	sessions   *fakeSessions
	encounters *fakeEncounters
	mgr        *session.Manager
}

func TestTurnSuite(t *testing.T) {
	suite.Run(t, new(TurnTestSuite))
}

func (s *TurnTestSuite) SetupTest() {
	s.sessions = newFakeSessions()
	s.encounters = newFakeEncounters()
	s.mgr = managerOverRepos(s.T(), s.sessions, s.encounters)
	_, err := s.mgr.StartSession(context.Background(), &session.StartSessionInput{
		Session: "sess", Encounter: "world", World: ambushWorld(s.T()),
	})
	s.Require().NoError(err)
}

// fight walks alice into the ogre so there is a bubble to ask about, and
// returns nothing because every test below asks the world, not the walk.
func (s *TurnTestSuite) fight() {
	out, err := s.mgr.Move(context.Background(), &session.MoveInput{
		Session: "sess", Member: "alice",
		Path: []spatial.Position{{X: 2, Y: 1}, {X: 2, Y: 2}, {X: 2, Y: 3}, {X: 2, Y: 4}},
	})
	s.Require().NoError(err)
	s.Require().NotNil(out.Formed, "the scene must actually put them in a fight")
}

// TestAskingWhatOneMemberIsWaitingOn is charter clause 6 at the seam.
//
// The question a caller may ask is "what is Alice waiting on?" — never "whose
// turn is it?". Several clocks can run at once, so the second question has no
// answer, and a verb that pretended otherwise would create a privileged clock
// by implication.
func (s *TurnTestSuite) TestAskingWhatOneMemberIsWaitingOn() {
	s.fight()
	ctx := context.Background()

	alice, err := s.mgr.Turn(ctx, &session.TurnInput{Session: "sess", Member: "alice"})
	s.Require().NoError(err)
	s.Equal(session.ClockTurn, alice.Clock, "she is in a fight")
	s.Equal([]string{"alice", "ogre"}, alice.Order)
	s.Equal("alice", alice.Active, "and it is her turn")
	s.Equal(1, alice.Round)

	ogre, err := s.mgr.Turn(ctx, &session.TurnInput{Session: "sess", Member: "ogre"})
	s.Require().NoError(err)
	s.Equal("alice", ogre.Active,
		"the same fight reads the same to both of them, and it is not the ogre's turn")
}

// TestTheWorldClockHasNoTurn pins that free roam answers honestly rather than
// inventing an active actor.
//
// On the world clock everyone acts and their own movement is what advances it,
// so there is nobody to be next. A caller that got a name here would build a
// turn indicator for a scene that has no turns.
func (s *TurnTestSuite) TestTheWorldClockHasNoTurn() {
	out, err := s.mgr.Turn(context.Background(), &session.TurnInput{
		Session: "sess", Member: "alice",
	})
	s.Require().NoError(err)
	s.Equal(session.ClockWorld, out.Clock)
	s.Empty(out.Active, "nobody is 'next' in free roam")
	s.Empty(out.Order)
	s.Zero(out.Round)
}

// TestTheMemberIsRequired is the clause-6 guard in its most literal form: the
// verb cannot be called without saying who is asking.
func (s *TurnTestSuite) TestTheMemberIsRequired() {
	_, err := s.mgr.Turn(context.Background(), &session.TurnInput{Session: "sess"})
	s.ErrorIs(err, session.ErrNoMemberID)

	_, err = s.mgr.EndTurn(context.Background(), &session.EndTurnInput{Session: "sess"})
	s.ErrorIs(err, session.ErrNoMemberID)
}

// TestStatusNeverLearnsWhoseTurnItIs is the structural half of clause 6.
//
// The clause names the deflection precisely: a top-level query answering "the"
// active actor. The way that arrives is not as a new verb somebody argues for
// — it is as a convenience field on a verb that already exists, added by
// someone who has one fight and cannot see why it would ever be ambiguous.
// Status is that verb, so its shape is pinned rather than merely documented.
func (s *TurnTestSuite) TestStatusNeverLearnsWhoseTurnItIs() {
	s.Equal([]string{"Open", "Outcome"}, structFields(session.Status{}),
		"a turn-shaped field on Status would create the privileged clock that "+
			"charter clause 6 exists to prevent — put it on Turn, which is asked "+
			"of a member")
}

// TestEndingATurnHandsItOn is rpg-toolkit#1162's headline case at the seam:
// alice ends her turn, the ogre has no player, and this single call must not
// return with the clock parked on it. covers the write verb, including that
// the story records BOTH ends — hers, and the ogre's driven-through pass.
func (s *TurnTestSuite) TestEndingATurnHandsItOn() {
	s.fight()
	ctx := context.Background()

	out, err := s.mgr.EndTurn(ctx, &session.EndTurnInput{Session: "sess", Member: "alice"})
	s.Require().NoError(err)
	s.Equal("alice", out.Next,
		"the ogre has no player; TurnDriver passes its turn and the round wraps straight back to her")
	s.True(out.RoundWrapped, "two in the fight, so the ogre's driven-through pass closes the round")
	s.NotZero(out.Seq)
	s.Equal([]string{"encounter:world"}, out.Saved.Written)

	after, err := s.mgr.Turn(ctx, &session.TurnInput{Session: "sess", Member: "alice"})
	s.Require().NoError(err)
	s.Equal("alice", after.Active, "and the world remembers it")
	s.Equal(2, after.Round, "the driven-through pass advanced the round")

	// The story carries both ends, independently addressed to their member —
	// a client rendering "the ogre does nothing" reads it from here.
	story, err := s.mgr.Story(ctx, &session.StoryInput{Session: "sess", Member: "alice"})
	s.Require().NoError(err)
	var turnEnded, ogrePassed int
	for _, entry := range story {
		if entry.Tags["tag"] != "clock" {
			continue
		}
		var p struct {
			Beat   string `json:"beat"`
			Member string `json:"member"`
		}
		s.Require().NoError(json.Unmarshal(entry.Payload, &p))
		if p.Beat != "turn-ended" {
			continue
		}
		turnEnded++
		if p.Member == "ogre" {
			ogrePassed++
		}
	}
	s.Equal(2, turnEnded, "alice's own end, plus the ogre's driven-through pass")
	s.Equal(1, ogrePassed, "the story names the ogre's own pass by member")
}

// TestTheRoundComesRound pins RoundWrapped, which is the only thing in
// EndTurnOutput a caller cannot derive by asking Turn again.
//
// rpg-toolkit#1162 collapsed this test's original two-call shape (end
// alice's turn, then the ogre's, and see the SECOND one wrap) into
// TestEndingATurnHandsItOn's single call: with only alice and the ogre in
// the fight, the ogre has no player, so its turn is driven through in the
// SAME call that ends alice's — there is no longer a separate "the ogre's
// own turn ends" moment to ask this question about. See that test for the
// wrap assertion; this one is kept to pin Round advancing past the wrap,
// which is the fact TestEndingATurnHandsItOn's own assertions do not
// restate on their own.
func (s *TurnTestSuite) TestTheRoundComesRound() {
	s.fight()
	ctx := context.Background()

	before, err := s.mgr.Turn(ctx, &session.TurnInput{Session: "sess", Member: "alice"})
	s.Require().NoError(err)
	s.Equal(1, before.Round, "control: the fight opens in round 1")

	last, err := s.mgr.EndTurn(ctx, &session.EndTurnInput{Session: "sess", Member: "alice"})
	s.Require().NoError(err)
	s.True(last.RoundWrapped)

	after, err := s.mgr.Turn(ctx, &session.TurnInput{Session: "sess", Member: "alice"})
	s.Require().NoError(err)
	s.Equal(2, after.Round)
}

// TestEndingATurnYouAreNotInIsRefused pins that this verb propagates the
// composition's rules rather than re-deciding them.
func (s *TurnTestSuite) TestEndingATurnYouAreNotInIsRefused() {
	ctx := context.Background()

	_, err := s.mgr.EndTurn(ctx, &session.EndTurnInput{Session: "sess", Member: "alice"})
	s.ErrorIs(err, session.ErrNotInFight, "she is free-roaming, so there is no turn to end")

	s.fight()
	_, err = s.mgr.EndTurn(ctx, &session.EndTurnInput{Session: "sess", Member: "nobody"})
	s.ErrorIs(err, session.ErrNoMember)
}

// TestEndingSomebodyElsesTurnIsRefused pins the rule that makes a turn order
// mean anything.
func (s *TurnTestSuite) TestEndingSomebodyElsesTurnIsRefused() {
	s.fight()

	_, err := s.mgr.EndTurn(context.Background(), &session.EndTurnInput{
		Session: "sess", Member: "ogre",
	})
	s.Require().Error(err, "it is alice's turn, so the ogre cannot end one")
}

// TestTheTurnEndingReachesClients pins the beat's event kind.
//
// It used to arrive as EventUnknown — delivered but uninterpretable, which is
// correct by the delivery rule and useless in practice. A client rendering a
// turn tracker needs to know the turn ended, and "something happened" is not
// that.
func (s *TurnTestSuite) TestTheTurnEndingReachesClients() {
	s.fight()

	stream := &fakeStream{}
	mgr, err := session.NewManager(&session.Config{
		Dice: testDice{}, TurnDriver: session.Pass{}, Sessions: s.sessions, Encounters: s.encounters,
		Characters: testCharacters(), Events: stream,
	})
	s.Require().NoError(err)

	_, err = mgr.EndTurn(context.Background(), &session.EndTurnInput{
		Session: "sess", Member: "alice",
	})
	s.Require().NoError(err)

	kinds := map[session.EventKind]int{}
	for _, e := range stream.published {
		kinds[e.Kind]++
	}
	s.Positive(kinds[session.EventTurnEnded], "the turn ending is a kind, not a mystery")
	s.Zero(kinds[session.EventUnknown], "and nothing else in this verb is one either")
}
