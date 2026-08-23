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
	mgr, err := session.NewManager(&session.Config{Dice: testDice{}, TurnDriver: session.Pass{},
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

// walkIntoTheAmbush walks alice into the ogre's line of sight, which starts a
// fight and stops the walk short.
func (s *ConditionsTestSuite) walkIntoTheAmbush(mgr *session.Manager) *session.MoveOutput {
	out, err := mgr.Move(context.Background(), &session.MoveInput{
		Session: "sess", Member: "alice",
		Path: ambushPath(),
	})
	s.Require().NoError(err)
	return out
}

// TestAVerbLeavesTheCharacterStoreUntouched is the no-clobber pin.
//
// Every write verb EXCEPT Attack, which must write — see
// TestDamagePersists. That exception is the pin getting stronger rather than
// weaker: the rule is now "only the verb that should, does", and the guard
// still covers every verb that has no business touching a sheet.
//
// Byte comparison rather
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
			Session: "sess", Member: "bob",
			Position: spatial.Position{X: 0, Y: 0},
		})
		s.Require().NoError(err)
		s.Equal(string(before), string(s.storedBytes("bob")))
	})

	s.Run("move that starts a fight", func() {
		before := s.storedBytes("alice")
		out := s.walkIntoTheAmbush(s.mgr)
		s.Require().NotNil(out.Formed, "this walk is supposed to start a fight")
		s.Equal(string(before), string(s.storedBytes("alice")))
	})
}

// TestTheRageSurvivesTheFightAndARestart is the wave-2 invariant applied to an
// entity, and the closest a test gets to the real failure: play continues from
// a process that never saw the fight start.
//
// It used to resume a suspended walk across the restart. Nothing suspends now
// (rpg-toolkit#964 slice 2), so the restart is what it always really was — the
// stores round-tripping through JSON — and the claim is unchanged: a condition
// is DURABLE, not an artifact of the manager that wrote it. All three stores go
// through, so the second manager shares no pointer with the first. The
// assertion is on the durable FIELDS rather than on "a condition is present",
// because presence is what a reconstructed-from-scratch rage would also
// satisfy.
func (s *ConditionsTestSuite) TestTheRageSurvivesTheFightAndARestart() {
	ctx := context.Background()

	out := s.walkIntoTheAmbush(s.mgr)
	s.Require().NotNil(out.Formed, "she walks into a fight")

	sessions, encounters, characters := s.roundTripAllStores()
	restarted := s.managerOverStores(sessions, encounters, characters)

	// The far side reads the same world: she is where the fight stopped her.
	seen, err := restarted.View(ctx, &session.ViewInput{Session: "sess", Member: "ogre"})
	s.Require().NoError(err)
	s.Require().Len(seen, 1, "the ogre still holds her across the restart")
	s.Equal("alice", seen[0].Subject)

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

// TestACharacterTheSDKCannotFullyLoadIsStillNotDamaged is where the no-clobber
// property earns its keep.
//
// Alice's blob carries a condition this build cannot parse — a ref from a
// module that is not installed, a body written by a newer version, a partial
// write. character.LoadFromData does not reject her: it logs, drops the
// condition, and returns a perfectly usable character with no error. The join
// therefore SUCCEEDS, and nothing in the response hints that anything was lost.
//
// That is toolkit#948, and it is only latent because the SDK does not write.
// The store still holds both conditions afterwards, so whatever could not be
// read here is still there to be read by a build that understands it. Add a
// SaveCharacter to this path and the unreadable condition is gone forever,
// destroyed by a verb that merely moved somebody.
func (s *ConditionsTestSuite) TestACharacterTheSDKCannotFullyLoadIsStillNotDamaged() {
	corrupt := ragingDwarf("dave")
	corrupt.Conditions = append(corrupt.Conditions,
		json.RawMessage(`{"ref":"homebrew:conditions:hexed","character_id":"dave","stacks":3}`))
	s.characters.byID["dave"] = corrupt

	before := s.storedBytes("dave")

	_, err := s.mgr.Join(context.Background(), &session.JoinInput{
		Session: "sess", Member: "dave",
		Position: spatial.Position{X: 0, Y: 0},
	})
	s.Require().NoError(err, "the unreadable condition does not fail the join — that is the trap")

	s.Equal(string(before), string(s.storedBytes("dave")),
		"and the condition this build could not read is still in the store")

	var held []json.RawMessage
	s.Require().NoError(json.Unmarshal(s.storedBytes("dave"), &struct {
		Conditions *[]json.RawMessage `json:"conditions"`
	}{Conditions: &held}))
	s.Require().Len(held, 2, "both conditions, including the one we cannot parse")
}
