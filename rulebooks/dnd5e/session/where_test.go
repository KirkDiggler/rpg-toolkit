// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package session_test

// where_test.go pins the self-position read (rpg-toolkit#1050), the reshape's
// last piece.
//
// It reuses offsetWorld, whose rooms are anchored at (40,20) and (46,20). That
// is the whole reason these assertions mean anything: a Where that echoed the
// stored room-local cell instead of projecting the live one would answer (1,1)
// where the truth is (41,21), and in a world anchored at the origin those are
// the same number.

import (
	"context"
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/session"
	"github.com/KirkDiggler/rpg-toolkit/tools/spatial"
)

type WhereSuite struct {
	suite.Suite

	sessions   *fakeSessions
	encounters *fakeEncounters
	mgr        *session.Manager
}

func TestWhereSuite(t *testing.T) {
	suite.Run(t, new(WhereSuite))
}

func (s *WhereSuite) SetupTest() {
	s.sessions, s.encounters = newFakeSessions(), newFakeEncounters()
	mgr, err := session.NewManager(&session.Config{
		Dice: testDice{}, Sessions: s.sessions, Encounters: s.encounters,
		Characters: testCharacters(), Events: session.DiscardEvents{},
	})
	s.Require().NoError(err)

	_, err = mgr.StartSession(context.Background(), &session.StartSessionInput{
		Session: "sess", Encounter: "world", World: offsetWorld(s.T()),
	})
	s.Require().NoError(err)
	s.mgr = mgr
}

func (s *WhereSuite) where(member string) spatial.Position {
	out, err := s.mgr.Where(context.Background(), &session.WhereInput{
		Session: "sess", Member: member,
	})
	s.Require().NoError(err)
	return out.Position
}

// TestAClientCanAskWhereItStands is the headline: the question answered
// directly, rather than remembered from the last Move.
func (s *WhereSuite) TestAClientCanAskWhereItStands() {
	s.Equal(spatial.Position{X: 41, Y: 21}, s.where("alice"),
		"hall-local (1,1), anchored at (40,20)")
}

// TestTheAnswerIsWhereTheyAreNow is the derive-don't-echo pin, and the case a
// reconnecting client actually hits.
//
// The member walks, the client is told nothing (a fresh one would not have been
// listening), and the read still answers correctly. An implementation that
// reported the placement the world was CONSTRUCTED with — the obvious way to
// get this wrong, since that is what the stored blob holds — passes the first
// test in this file and fails this one.
func (s *WhereSuite) TestTheAnswerIsWhereTheyAreNow() {
	_, err := s.mgr.Move(context.Background(), &session.MoveInput{
		Session: "sess", Member: "alice",
		Path: []spatial.Position{{X: 42, Y: 22}, {X: 43, Y: 22}},
	})
	s.Require().NoError(err)

	s.Equal(spatial.Position{X: 43, Y: 22}, s.where("alice"), "where the walk left her")
}

// TestTheAnswerAgreesWithTheMap crosses the read against the other one a client
// renders with. A cell that is not on the map is a member drawn outside the
// dungeon.
func (s *WhereSuite) TestTheAnswerAgreesWithTheMap() {
	atlas, err := s.mgr.Atlas(context.Background(), &session.AtlasInput{Session: "sess"})
	s.Require().NoError(err)

	cells := map[spatial.Position]bool{}
	for _, cell := range atlas.Cells {
		cells[cell] = true
	}

	s.True(cells[s.where("alice")], "she stands on a cell the map contains")
}

// TestItSurvivesAReconnect: the read goes through the stored session like every
// other verb, so a client that has lost everything can ask cold.
//
// A second Manager over the same repositories IS the reconnect — nothing of the
// first one's memory carries over, which is the point of the seam holding no
// session process.
func (s *WhereSuite) TestItSurvivesAReconnect() {
	_, err := s.mgr.Move(context.Background(), &session.MoveInput{
		Session: "sess", Member: "alice",
		Path: []spatial.Position{{X: 42, Y: 22}},
	})
	s.Require().NoError(err)

	reconnected, err := session.NewManager(&session.Config{
		Dice: testDice{}, Sessions: s.sessions, Encounters: s.encounters,
		Characters: testCharacters(), Events: session.DiscardEvents{},
	})
	s.Require().NoError(err)

	out, err := reconnected.Where(context.Background(), &session.WhereInput{
		Session: "sess", Member: "alice",
	})
	s.Require().NoError(err)
	s.Equal(spatial.Position{X: 42, Y: 22}, out.Position, "read cold from what was persisted")
}

// TestItRefusesWhatItCannotAnswer covers the three ways to ask badly.
func (s *WhereSuite) TestItRefusesWhatItCannotAnswer() {
	ctx := context.Background()

	_, err := s.mgr.Where(ctx, nil)
	s.ErrorIs(err, session.ErrNilInput)

	_, err = s.mgr.Where(ctx, &session.WhereInput{Session: "sess"})
	s.ErrorIs(err, session.ErrNoMemberID, "refused before the session is loaded")

	_, err = s.mgr.Where(ctx, &session.WhereInput{Session: "sess", Member: "nobody"})
	s.ErrorIs(err, session.ErrNoMember, "a member this encounter does not have")

	_, err = s.mgr.Where(ctx, &session.WhereInput{Session: "nope", Member: "alice"})
	s.ErrorIs(err, session.ErrNoSession)
}
