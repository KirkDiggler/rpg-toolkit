// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package session_test

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/character"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/conditions"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/refs"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/session"
	"github.com/KirkDiggler/rpg-toolkit/tools/spatial"
)

// Durable condition state across a suspension.
//
// Wave 2 established that no Go stack survives a wait: a suspension drops every
// entity, and Answer reloads from data. That turns a question the design could
// have deferred into an invariant it must honour — if a condition's durable
// state does not round-trip through the character's own blob, a suspension
// quietly ends the rage.
//
// WHAT THESE TESTS PIN, EXACTLY: that the SDK does not damage the host's stored
// character. They do NOT claim a condition intercepted anything during a verb,
// because as of this wave nothing in the composition publishes to the bus and
// no read verb reports a character's active conditions — so "the condition
// behaved" has no observable consequence to assert. Claiming it here would be
// the kind of comment that survives its own mutant. Behaviour is pinned in the
// wave that gives it something to observe.
//
// The guarantee this file does carry is the one that is load-bearing right now,
// and it is a negative: a verb leaves the character store byte-for-byte as it
// found it. That matters because character.LoadFromData drops conditions it
// cannot parse and Character.ToData drops conditions it cannot serialise, both
// silently and with no error (toolkit#948). While the SDK only reads, a corrupt
// blob stays corrupt but stays WHOLE. The moment the SDK writes a character
// back, that loss becomes permanent on an ordinary walk. So "we do not write
// yet" is a data-safety property, not an omission, and it is pinned as one.
type ConditionsTestSuite struct {
	suite.Suite

	sessions   *fakeSessions
	encounters *fakeEncounters
	characters *fakeCharacters
	mgr        *session.Manager
}

func TestConditionsSuite(t *testing.T) {
	suite.Run(t, new(ConditionsTestSuite))
}

func (s *ConditionsTestSuite) SetupTest() {
	s.sessions = newFakeSessions()
	s.encounters = newFakeEncounters()
	s.characters = newFakeCharacters(ragingDwarf("alice"), dwarfCharacter("bob"))
	s.mgr = s.managerOverStores(s.sessions, s.encounters, s.characters)

	_, err := s.mgr.StartSession(context.Background(), &session.StartSessionInput{
		Session: "sess", Encounter: "world", World: ambushWorld(s.T()),
	})
	s.Require().NoError(err)
}

func (s *ConditionsTestSuite) SetupSubTest() { s.SetupTest() }

func (s *ConditionsTestSuite) managerOverStores(
	sessions *fakeSessions, encounters *fakeEncounters, characters *fakeCharacters,
) *session.Manager {
	mgr, err := session.NewManager(&session.Config{
		Sessions: sessions, Encounters: encounters,
		Characters: characters, Events: session.DiscardEvents{},
	})
	s.Require().NoError(err)
	return mgr
}

// ragingDwarf is Alice mid-rage, with the durable state a rage actually carries.
//
// The fields are deliberately non-zero and non-default: TurnsActive 2 rather
// than 0, both this-turn flags set. A round-trip that reconstructed a rage from
// scratch would produce zeroes here and pass any test that only asked "is she
// still raging".
func ragingDwarf(id string) *character.Data {
	data := dwarfCharacter(id)
	raging := &conditions.RagingCondition{
		CharacterID:       id,
		DamageBonus:       2,
		Level:             3,
		Source:            refs.Features.Rage().String(),
		TurnsActive:       2,
		WasHitThisTurn:    true,
		DidAttackThisTurn: true,
	}
	blob, err := raging.ToJSON()
	if err != nil {
		panic("building the raging fixture: " + err.Error())
	}
	data.Conditions = []json.RawMessage{blob}
	return data
}

// storedBytes is the character exactly as the host holds it.
func (s *ConditionsTestSuite) storedBytes(id string) []byte {
	data, ok := s.characters.byID[id]
	s.Require().True(ok, "the store holds %q", id)
	raw, err := json.Marshal(data)
	s.Require().NoError(err)
	return raw
}

func (s *ConditionsTestSuite) walkIntoTheAmbush(mgr *session.Manager) *session.MoveOutput {
	out, err := mgr.Move(context.Background(), &session.MoveInput{
		Session: "sess", Member: "alice",
		Path: []spatial.Position{{X: 2, Y: 2}, {X: 2, Y: 3}, {X: 2, Y: 4}},
	})
	s.Require().NoError(err)
	return out
}

// TestAVerbLeavesTheCharacterStoreUntouched is the no-clobber pin.
//
// Every write verb, including the one that suspends. Byte comparison rather
// than a field-by-field check on purpose: the failure this guards against is a
// well-meaning SaveCharacter added to the write path, and ToData stamps
// UpdatedAt with time.Now() on every call — so an unconditional save changes
// the bytes even when it changes no game state. A field-wise assertion chosen
// by whoever added the save would likely skip that field and pass.
func (s *ConditionsTestSuite) TestAVerbLeavesTheCharacterStoreUntouched() {
	ctx := context.Background()

	s.Run("join", func() {
		before := s.storedBytes("bob")
		_, err := s.mgr.Join(ctx, &session.JoinInput{
			Session: "sess", Member: "bob", Kind: session.KindPlayer,
			Room: "hall", Position: spatial.Position{X: 0, Y: 0},
		})
		s.Require().NoError(err)
		s.Equal(string(before), string(s.storedBytes("bob")))
	})

	s.Run("move that suspends", func() {
		before := s.storedBytes("alice")
		out := s.walkIntoTheAmbush(s.mgr)
		s.Require().NotNil(out.Pending, "this walk is supposed to suspend")
		s.Equal(string(before), string(s.storedBytes("alice")))
	})

	s.Run("answer that resumes", func() {
		out := s.walkIntoTheAmbush(s.mgr)
		s.Require().NotNil(out.Pending)
		before := s.storedBytes("alice")
		_, err := s.mgr.Answer(ctx, &session.AnswerInput{
			Session: "sess", Window: out.Pending.Window,
			Member: "alice", Option: string(session.OptionContinue),
		})
		s.Require().NoError(err)
		s.Equal(string(before), string(s.storedBytes("alice")))
	})
}

// TestTheRageSurvivesSuspensionRestartAndAnswer is the wave-2 invariant applied
// to an entity, and the closest a test gets to the real failure: the answer
// arrives from a process that never saw the walk begin.
//
// All three stores round-trip through JSON, so the resumed manager shares no
// pointer with the one that suspended. The assertion is on the DURABLE fields
// rather than on "a condition is present", because presence is what a
// reconstructed-from-scratch rage would also satisfy.
func (s *ConditionsTestSuite) TestTheRageSurvivesSuspensionRestartAndAnswer() {
	ctx := context.Background()

	out := s.walkIntoTheAmbush(s.mgr)
	s.Require().NotNil(out.Pending)

	sessions, encounters, characters := s.roundTripAllStores()
	restarted := s.managerOverStores(sessions, encounters, characters)

	pending, err := restarted.Pending(ctx, &session.PendingInput{Session: "sess"})
	s.Require().NoError(err)
	s.Require().Len(pending.Windows, 1, "the window outlived the process")

	resumed, err := restarted.Answer(ctx, &session.AnswerInput{
		Session: "sess", Window: pending.Windows[0].Window,
		Member: "alice", Option: string(session.OptionContinue),
	})
	s.Require().NoError(err)
	s.Require().Len(resumed.Steps, 1, "the walk picked up where it stopped")

	stored, ok := characters.byID["alice"]
	s.Require().True(ok)
	s.Require().Len(stored.Conditions, 1, "she is still raging on the far side")

	var got conditions.RagingData
	s.Require().NoError(json.Unmarshal(stored.Conditions[0], &got))
	s.Equal(refs.Conditions.Raging().String(), got.Ref.String())
	s.Equal(2, got.TurnsActive, "the rage remembers how long it has run")
	s.True(got.WasHitThisTurn, "and what happened to her during it")
	s.True(got.DidAttackThisTurn)
	s.Equal(2, got.DamageBonus)
	s.Equal(3, got.Level)
	s.Equal(refs.Features.Rage().String(), got.Source)
}

// roundTripAllStores marshals all three repositories through JSON into fresh
// ones — as close as a test gets to "a different process picked this up".
//
// The character store is included deliberately. Without it the resumed manager
// would read from the same map the suspended one wrote to, and every claim
// about a condition surviving would be a claim about a pointer that never went
// anywhere.
func (s *ConditionsTestSuite) roundTripAllStores() (*fakeSessions, *fakeEncounters, *fakeCharacters) {
	sessions := newFakeSessions()
	for id, data := range s.sessions.byID {
		var reloaded session.SessionData
		s.requeueJSON(data, &reloaded)
		sessions.byID[id] = &reloaded
	}

	encounters := newFakeEncounters()
	for id, data := range s.encounters.byID {
		var reloaded encounter.EncounterData
		s.requeueJSON(data, &reloaded)
		encounters.byID[id] = &reloaded
	}

	characters := newFakeCharacters()
	for id, data := range s.characters.byID {
		var reloaded character.Data
		s.requeueJSON(data, &reloaded)
		characters.byID[id] = &reloaded
	}
	return sessions, encounters, characters
}

func (s *ConditionsTestSuite) requeueJSON(from any, into any) {
	raw, err := json.Marshal(from)
	s.Require().NoError(err)
	s.Require().NoError(json.Unmarshal(raw, into))
}
