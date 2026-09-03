// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package session_test

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/refs"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/session"
	"github.com/KirkDiggler/rpg-toolkit/tools/spatial"
)

// StandingSplitSuite covers the standing seam asking resolution in two calls,
// split by which store the record came from.
//
// The split exists for ONE reason: the two failures are different and a host
// branches on which. A stored NPC that will not reconstitute is corrupt SESSION
// state, because this record is the only thing that could have written it; a
// character that will not load is the host's own store. One call would come
// back with one error and no way to say which.
//
// That justification was held by inference until this suite existed — the
// seam's code said it, and nothing checked it. These are the tests that make
// the two sentinels a fact rather than a comment.
type StandingSplitSuite struct {
	suite.Suite
	mgr        *session.Manager
	sessions   *fakeSessions
	encounters *fakeEncounters
	characters *fakeCharacters
}

func TestStandingSplitSuite(t *testing.T) { suite.Run(t, new(StandingSplitSuite)) }

func (s *StandingSplitSuite) SetupTest() {
	s.sessions, s.encounters = newFakeSessions(), newFakeEncounters()
	s.characters = newFakeCharacters(armedFighter("alice"), armedFighter("bob"))

	mgr, err := session.NewManager(&session.Config{PresentationIDs: testPresentationIDs{},
		Dice: testDice{}, TurnDriver: session.Pass{}, Sessions: s.sessions,
		Encounters: s.encounters, Characters: s.characters, Events: session.DiscardEvents{},
	})
	s.Require().NoError(err)
	s.mgr = mgr

	_, err = s.mgr.StartSession(context.Background(), &session.StartSessionInput{
		Session: "sess", Encounter: "world", World: cryptWorld(s.T()),
	})
	s.Require().NoError(err)
}

func (s *StandingSplitSuite) spawnSkeleton() {
	s.T().Helper()
	_, err := s.mgr.Spawn(context.Background(), &session.SpawnInput{
		Session: "sess", ID: "skeleton", Ref: refs.Monsters.Skeleton().String(),
		Position: spatial.Position{X: 2, Y: 1},
	})
	s.Require().NoError(err)
}

// A CHARACTER and a MONSTER, both at zero, in one consult.
//
// The point is that the two halves rejoin: they are asked in separate calls and
// the answer is one list. An implementation that returned only the characters,
// or only the monsters, passes every test that has one kind in it — so this
// fixture has both, each downed, and asserts both come back.
func (s *StandingSplitSuite) TestBothHalvesOfTheSplitAreReported() {
	s.spawnSkeleton()

	s.floorCharacter("alice")
	s.floorNPC("skeleton")

	// Asked from both sides, because a sighting is what somebody ELSE can see:
	// an observer is not in their own view, so no single call can show both.
	// Each call consults the whole roster, so each exercises both halves of the
	// split; what differs is which answer is visible.
	s.Assert().Contains(s.downSeenBy("skeleton"), "alice",
		"the character half of the split answered")
	s.Assert().Contains(s.downSeenBy("alice"), "skeleton",
		"and so did the monster half")
}

// Only members that were ASKED about are named, and in the order asked.
//
// The composition refuses an answer naming a non-member (ErrNotMember), so a
// seam that replied out of its whole store would abort every verb. The store
// really does hold strangers: bob has a sheet and is not on this roster.
func (s *StandingSplitSuite) TestOnlyTheMembersAskedAboutAreNamed() {
	s.spawnSkeleton()
	s.floorCharacter("alice")
	s.floorCharacter("bob") // has a sheet, is not a member

	down := s.downSeenBy("skeleton")

	s.Assert().Contains(down, "alice")
	s.Assert().NotContains(down, "bob",
		"bob is downed and is not on this roster; naming him would abort the verb (ErrNotMember)")
}

// A CORRUPT NPC RECORD is corrupt SESSION state.
//
// This record is the only thing that could have written it, so ErrInvalidSession
// sends whoever debugs it to the session store rather than to a player's sheet.
// It is also the F2 asymmetry arriving at the seam: monstertraits has no lenient
// loader, so a trait blob this build cannot parse refuses however leniently the
// entry was asked to read.
func (s *StandingSplitSuite) TestACorruptNPCIsCorruptSessionState() {
	s.spawnSkeleton()
	s.corruptNPC("skeleton")

	_, err := s.mgr.View(context.Background(), &session.ViewInput{Session: "sess", Member: "alice"})

	s.Require().Error(err)
	s.Assert().ErrorIs(err, session.ErrInvalidSession,
		"the session record wrote this sheet, so the session record is what is wrong")
	s.Assert().NotErrorIs(err, session.ErrBadCharacter,
		"and NOT a character problem — that is the distinction the split exists to keep")
}

// A CHARACTER RECORD THE HOST'S STORE CANNOT ANSWER FOR is ErrBadCharacter.
//
// The mirror of the case above, and the reason both calls exist. A record with
// no identity cannot be read back out of the cast it was put into, so the entry
// refuses it — and it refuses as a CHARACTER problem, which is where the repair
// is.
func (s *StandingSplitSuite) TestAnUnusableCharacterRecordIsACharacterProblem() {
	s.spawnSkeleton()

	stored := s.characters.byID["alice"]
	stored.ID = "" // a record that cannot say who it is

	_, err := s.mgr.View(context.Background(), &session.ViewInput{Session: "sess", Member: "skeleton"})

	s.Require().Error(err)
	s.Assert().ErrorIs(err, session.ErrBadCharacter,
		"the host's store holds this one, so the host's store is what is wrong")
	s.Assert().NotErrorIs(err, session.ErrInvalidSession,
		"and NOT session state — collapsing the two is what one call would have forced")
}

// downSeenBy asks a read verb that consults standing over the WHOLE roster,
// and reports who the given observer sees as downed.
//
// View is the verb because it batches the standing question over every member
// rather than the observer alone, so one call runs both halves of the split
// whatever it ends up showing.
func (s *StandingSplitSuite) downSeenBy(observer string) []string {
	s.T().Helper()

	sightings, err := s.mgr.View(context.Background(), &session.ViewInput{Session: "sess", Member: observer})
	s.Require().NoError(err)

	var down []string
	for _, sighting := range sightings {
		if sighting.Seen.Standing == session.StandingDowned {
			down = append(down, sighting.Subject)
		}
	}

	return down
}

func (s *StandingSplitSuite) floorCharacter(id string) {
	s.T().Helper()
	s.characters.byID[id].HitPoints = 0
}

func (s *StandingSplitSuite) floorNPC(id string) {
	s.T().Helper()
	data := s.sessions.byID["sess"]
	for i := range data.NPCs {
		if data.NPCs[i].ID == id {
			data.NPCs[i].HitPoints = 0
			return
		}
	}
	s.Require().Fail("no NPC " + id)
}

func (s *StandingSplitSuite) corruptNPC(id string) {
	s.T().Helper()
	data := s.sessions.byID["sess"]
	for i := range data.NPCs {
		if data.NPCs[i].ID == id {
			data.NPCs[i].Conditions = []json.RawMessage{json.RawMessage(`{"ref":{"module":"dnd5e","type":"conditions","id":"not-a-real-condition"}}`)}
			return
		}
	}
	s.Require().Fail("no NPC " + id)
}
