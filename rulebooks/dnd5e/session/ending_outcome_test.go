// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package session_test

// ending_outcome_test.go pins rpg-project#733: the ended clock entry carries
// the result the ending that fired declares, read off the ended beat's
// `outcome`, and an ending that declares none carries none. A reload folds
// the same entry from the stored record, so a closed run keeps its result.

import (
	"context"
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/session"
)

type EndingOutcomeSuite struct {
	suite.Suite

	sessions   *fakeSessions
	encounters *fakeEncounters
	characters *fakeCharacters
}

func TestEndingOutcomeSuite(t *testing.T) { suite.Run(t, new(EndingOutcomeSuite)) }

func (s *EndingOutcomeSuite) SetupTest() {
	s.sessions = newFakeSessions()
	s.encounters = newFakeEncounters()
	s.characters = testCharacters()
}

// manager is a manager over this suite's stores, its acts captured live. A
// second manager over the same stores is a reload.
func (s *EndingOutcomeSuite) manager(live *actCapture) *session.Manager {
	mgr, err := session.NewManager(&session.Config{
		Record: session.DiscardRecord{}, Seats: newFakeSeats(), Explorations: newFakeExplorations(),
		PresentationIDs: testPresentationIDs{}, Dice: testDice{}, TurnDriver: session.Pass{},
		Sessions: s.sessions, Encounters: s.encounters, Characters: s.characters, Acts: live,
	})
	s.Require().NoError(err)
	return mgr
}

// endsBy launches a party whose endings declare success, failure and nothing,
// ends the run by the one named, and returns the manager and its live acts.
func (s *EndingOutcomeSuite) endsBy(ending string) (*session.Manager, *actCapture) {
	live := &actCapture{}
	mgr := s.manager(live)
	sc := twoRoomParty()
	sc.Endings = []encounter.EndingInput{
		{Key: "captain-slain", Trigger: encounter.TriggerExternal{}, Result: encounter.ResultSuccess},
		{Key: "captain-lives", Trigger: encounter.TriggerExternal{}, Result: encounter.ResultFailure},
		{Key: "out", Trigger: encounter.TriggerExternal{}},
	}
	launchScene(s.T(), mgr, sc)

	_, err := mgr.End(context.Background(), &session.EndInput{Session: "sess", Ending: ending})
	s.Require().NoError(err)
	return mgr, live
}

// endedOf is the one ended clock entry among acts.
func (s *EndingOutcomeSuite) endedOf(acts []session.Act) session.ClockEntry {
	var found []session.ClockEntry
	for _, a := range acts {
		if c, ok := a.Body.(session.ClockEntry); ok && c.Mark == session.ClockEnded {
			found = append(found, c)
		}
	}
	s.Require().Len(found, 1, "exactly one ended entry")
	return found[0]
}

func (s *EndingOutcomeSuite) TestASuccessEndingCarriesSuccess() {
	_, live := s.endsBy("captain-slain")

	ended := s.endedOf(live.of("alice"))
	s.Equal("captain-slain", ended.Ending)
	s.Equal(session.ResultSuccess, ended.Outcome)
}

func (s *EndingOutcomeSuite) TestAFailureEndingCarriesFailure() {
	_, live := s.endsBy("captain-lives")

	ended := s.endedOf(live.of("alice"))
	s.Equal("captain-lives", ended.Ending)
	s.Equal(session.ResultFailure, ended.Outcome)
}

func (s *EndingOutcomeSuite) TestAnEndingThatDeclaresNoResultCarriesNone() {
	_, live := s.endsBy("out")

	ended := s.endedOf(live.of("alice"))
	s.Equal("out", ended.Ending)
	s.Empty(ended.Outcome, "no result is declared, so none is read")
}

// TestAReloadKeepsTheResult reads the closed run back through a fresh
// manager over the same stores: the result is folded again from the stored
// ended beat, the same as it arrived live.
func (s *EndingOutcomeSuite) TestAReloadKeepsTheResult() {
	_, live := s.endsBy("captain-slain")

	reloaded := s.manager(&actCapture{})
	caught := actsOf(s.T(), reloaded, "alice", "")

	s.Equal(s.endedOf(live.of("alice")), s.endedOf(caught.Acts))
	s.Equal(session.ResultSuccess, s.endedOf(caught.Acts).Outcome)
}
