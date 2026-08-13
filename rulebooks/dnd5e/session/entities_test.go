// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package session_test

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/character"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/races"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/session"
	"github.com/KirkDiggler/rpg-toolkit/tools/spatial"
)

// EntitiesTestSuite covers loading a character through the host's repository:
// the first wave in which this package holds a domain entity at all, even for
// the length of one call.
type EntitiesTestSuite struct {
	suite.Suite

	sessions   *fakeSessions
	encounters *fakeEncounters
	characters *fakeCharacters
	mgr        *session.Manager
}

func (s *EntitiesTestSuite) SetupTest() {
	s.sessions = newFakeSessions()
	s.encounters = newFakeEncounters()
	s.characters = testCharacters()
	mgr, err := session.NewManager(&session.Config{
		Sessions: s.sessions, Encounters: s.encounters, Characters: s.characters,
		Events: session.DiscardEvents{},
	})
	s.Require().NoError(err)
	s.mgr = mgr

	_, err = mgr.StartSession(context.Background(), &session.StartSessionInput{
		Session: "sess", Encounter: "world", World: hexWorld(s.T()),
	})
	s.Require().NoError(err)
}

func (s *EntitiesTestSuite) SetupSubTest() { s.SetupTest() }

func (s *EntitiesTestSuite) joinBob() (*session.JoinOutput, error) {
	return s.mgr.Join(context.Background(), &session.JoinInput{
		Session: "sess", Member: "bob", Kind: session.KindPlayer,
		Room: "vault", Position: spatial.Position{X: 0, Y: 0},
	})
}

// TestJoiningAPlayerLoadsTheirCharacter is the wave's headline, and the
// assertion is chosen so it cannot pass without a real load.
//
// SPEED IS NOT A FIELD OF character.Data. There is nothing to echo. A dwarf's
// 25 exists only because the stored bytes were reconstituted into a character
// and asked, so this single number distinguishes "the repository returned
// something" from "a character actually loaded".
func (s *EntitiesTestSuite) TestJoiningAPlayerLoadsTheirCharacter() {
	out, err := s.joinBob()
	s.Require().NoError(err)

	s.Require().NotNil(out.Character, "a player join must report the loaded character")
	s.Equal("bob", out.Character.ID)
	s.Equal(25, out.Character.Speed, "a dwarf's speed is derived from race, not stored")
	s.Equal(24, out.Character.HitPoints)
	s.Equal(28, out.Character.MaxHitPoints)
	s.Equal(16, out.Character.ArmorClass)
	s.Equal(3, out.Character.Level)
}

// TestSpeedIsDerivedRatherThanDefaulted is the discriminating control for the
// assertion above.
//
// GetSpeed falls back to 30 when it cannot find race data, and 30 is also a
// human's speed. So "25" proving derivation depends on the fallback not being
// 25 — this pins that two characters differing ONLY in race report different
// speeds, which a default could not produce.
func (s *EntitiesTestSuite) TestSpeedIsDerivedRatherThanDefaulted() {
	human := dwarfCharacter("human-one")
	human.RaceID = races.Human
	s.characters.byID[human.ID] = human

	dwarf, err := s.joinBob()
	s.Require().NoError(err)

	humanOut, err := s.mgr.Join(context.Background(), &session.JoinInput{
		Session: "sess", Member: "human-one", Kind: session.KindPlayer,
		Room: "vault", Position: spatial.Position{X: 1, Y: 0},
	})
	s.Require().NoError(err)

	s.Equal(25, dwarf.Character.Speed)
	s.Equal(30, humanOut.Character.Speed)
	s.NotEqual(dwarf.Character.Speed, humanOut.Character.Speed,
		"characters differing only in race must differ in derived speed")
}

// TestJoiningAPlayerWithNoCharacterIsRejected is why the load happens at join
// rather than at first use.
func (s *EntitiesTestSuite) TestJoiningAPlayerWithNoCharacterIsRejected() {
	_, err := s.mgr.Join(context.Background(), &session.JoinInput{
		Session: "sess", Member: "nobody", Kind: session.KindPlayer,
		Room: "vault", Position: spatial.Position{X: 0, Y: 0},
	})
	s.Require().Error(err)
	s.ErrorIs(err, session.ErrNoCharacter)
	s.NotErrorIs(err, session.ErrNoMember, "absent sheet is not an absent roster entry")
}

// TestARejectedJoinPlacesNobody pins that a failed load ABORTS the join.
//
// Worth being precise about what makes this true, because the obvious answer is
// wrong. It is NOT the ordering: mutating the load to run after the placement
// leaves this test passing, because load-act-save (S4) already discards the
// in-memory encounter when a verb returns before commit. Nothing was going to
// persist either way.
//
// What it does guard is the error actually stopping the verb. Swallow the load
// failure and carry on, and the join commits a member whose sheet does not
// exist — which is precisely the state loading at join time is meant to
// prevent.
//
// The ordering is still chosen deliberately, just not for correctness: there is
// no reason to touch the world when the call is already doomed.
func (s *EntitiesTestSuite) TestARejectedJoinPlacesNobody() {
	_, err := s.mgr.Join(context.Background(), &session.JoinInput{
		Session: "sess", Member: "nobody", Kind: session.KindPlayer,
		Room: "vault", Position: spatial.Position{X: 0, Y: 0},
	})
	s.Require().Error(err)

	// No read verb lists members yet (toolkit#933), so the placement is
	// observed through the verb that requires one: exiting someone who was
	// never placed must fail as an absent member.
	_, err = s.mgr.Exit(context.Background(), &session.ExitInput{
		Session: "sess", Member: "nobody",
	})
	s.Require().Error(err)
	s.ErrorIs(err, session.ErrNoMember, "a rejected join must not leave a member placed")
}

// TestAMonsterJoinsWithoutACharacter is the over-tightening defense.
//
// The rejection above proves the check refuses. Only this proves it does not
// over-reach: monsters have no sheet, and a load that fired for every kind
// would break every monster join while passing every rejection row.
func (s *EntitiesTestSuite) TestAMonsterJoinsWithoutACharacter() {
	before := s.characters.loads

	out, err := s.mgr.Join(context.Background(), &session.JoinInput{
		Session: "sess", Member: "ogre-7", Kind: session.KindMonster,
		Room: "vault", Position: spatial.Position{X: 1, Y: 0},
	})
	s.Require().NoError(err, "a monster has no character sheet and must not need one")
	s.Nil(out.Character, "and reports none")
	s.Equal(before, s.characters.loads, "the character repository must not be consulted at all")
}

// TestEveryPlayerJoinConsultsTheRepository pins that the load is per member
// rather than done once for the session.
//
// Deliberately NOT claiming to pin "no caching between calls", which is what an
// earlier version of this test said. It cannot: only one verb loads a character
// today, and it joins a DIFFERENT member each time, so a per-ID cache would
// change nothing observable here. Mutation confirmed that — the caching mutant
// survived.
//
// The real per-call pin becomes writable when a second verb loads a character
// and the same member can be loaded twice. Until then this guards the weaker
// but still useful property, and says so rather than overclaiming.
func (s *EntitiesTestSuite) TestEveryPlayerJoinConsultsTheRepository() {
	_, err := s.joinBob()
	s.Require().NoError(err)
	first := s.characters.loads
	s.Positive(first, "joining a player must consult the repository")

	_, err = s.mgr.Join(context.Background(), &session.JoinInput{
		Session: "sess", Member: "carol", Kind: session.KindPlayer,
		Room: "vault", Position: spatial.Position{X: 1, Y: 0},
	})
	s.Require().NoError(err)
	s.Greater(s.characters.loads, first, "a second join must load again, not reuse")
}

// TestARepositoryReportingSuccessWithNoDataIsRejected covers the contract
// violation rather than the absence: a store that returns (nil, nil) is broken,
// and guessing in either direction is worse than saying so.
func (s *EntitiesTestSuite) TestARepositoryReportingSuccessWithNoDataIsRejected() {
	mgr, err := session.NewManager(&session.Config{
		Sessions: s.sessions, Encounters: s.encounters,
		Characters: &nilDataCharacters{},
		Events:     session.DiscardEvents{},
	})
	s.Require().NoError(err)

	_, err = mgr.Join(context.Background(), &session.JoinInput{
		Session: "sess", Member: "bob", Kind: session.KindPlayer,
		Room: "vault", Position: spatial.Position{X: 0, Y: 0},
	})
	s.Require().Error(err)
	s.ErrorIs(err, session.ErrBadRepository)
	s.NotErrorIs(err, session.ErrNoCharacter, "broken is not the same as absent")
}

// TestACorruptConditionIsDroppedRatherThanRejected documents UPSTREAM
// behaviour that this wave inherits and does not yet fix.
//
// character.LoadFromData logs and continues past a condition it cannot parse
// or apply, so a corrupt blob vanishes silently instead of failing the load.
// That matters here more than it did before: the durable-condition-state
// invariant says a condition must survive save and reload, and this is a path
// where one does not — with no error anywhere.
//
// Pinned rather than fixed because the swallow is in the character package, not
// this one. The test exists so the behaviour is known and so a future upstream
// change that starts returning an error is noticed here rather than in a game.
func (s *EntitiesTestSuite) TestACorruptConditionIsDroppedRatherThanRejected() {
	corrupt := dwarfCharacter("corrupt-one")
	corrupt.Conditions = []json.RawMessage{json.RawMessage(`{"ref":"nonsense","x":`)}
	s.characters.byID[corrupt.ID] = corrupt

	out, err := s.mgr.Join(context.Background(), &session.JoinInput{
		Session: "sess", Member: "corrupt-one", Kind: session.KindPlayer,
		Room: "vault", Position: spatial.Position{X: 1, Y: 0},
	})

	s.Require().NoError(err, "upstream currently swallows this; if that changes, so must this test")
	s.Require().NotNil(out.Character)
	s.Equal(25, out.Character.Speed, "the rest of the character still loaded")
}

// nilDataCharacters violates the repository contract by reporting success with
// no data.
type nilDataCharacters struct{}

func (n *nilDataCharacters) GetCharacter(_ context.Context, _ string) (*character.Data, error) {
	return nil, nil
}

func (n *nilDataCharacters) SaveCharacter(_ context.Context, _ *character.Data) error { return nil }

func TestEntitiesSuite(t *testing.T) { suite.Run(t, new(EntitiesTestSuite)) }
