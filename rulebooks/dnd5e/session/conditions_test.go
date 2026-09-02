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
// WHAT THESE TESTS PIN, EXACTLY: that verbs with no character mutation do not
// damage the host's stored character. First-ever Join is now deliberately
// outside that set: it performs a normal long rest and persists the complete
// outcome. The Join row below is therefore a real NON-FIRST Join, established
// through Join then Exit so persisted EverMembers — not a test-only flag — is
// what suppresses the second rest and save.
//
// That negative remains load-bearing. character.LoadFromData drops conditions
// it cannot parse and Character.ToData drops conditions it cannot serialise,
// both silently and with no error (toolkit#948). A non-first Join may project
// leniently, but it must not save that projection and make the loss permanent.
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
// Every row is a path that has no business writing a sheet. The Join row is
// explicitly an exit/rejoin: first admission must write its rest, while a
// persisted EverMembers match must not. Attack remains a separate writing
// exception — see TestDamagePersists.
//
// Byte comparison rather
// than a field-by-field check on purpose: the failure this guards against is a
// well-meaning SaveCharacter added to the write path, and ToData stamps
// UpdatedAt with time.Now() on every call — so an unconditional save changes
// the bytes even when it changes no game state. A field-wise assertion chosen
// by whoever added the save would likely skip that field and pass.
func (s *ConditionsTestSuite) TestAVerbLeavesTheCharacterStoreUntouched() {
	ctx := context.Background()

	s.Run("non-first join", func() {
		_, err := s.mgr.Join(ctx, &session.JoinInput{
			Session: "sess", Member: "bob",
			Position: spatial.Position{X: 0, Y: 0},
		})
		s.Require().NoError(err, "establish the first admission through the real verb")
		_, err = s.mgr.Exit(ctx, &session.ExitInput{Session: "sess", Member: "bob"})
		s.Require().NoError(err, "EverMembers must survive a real exit")

		before := s.storedBytes("bob")
		_, err = s.mgr.Join(ctx, &session.JoinInput{
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

// TestACharacterTheSDKCannotFullyLoadIsStillNotDamaged is where the non-first
// no-clobber property earns its keep.
//
// A FIRST admission strictly rests and therefore rejects a condition this build
// cannot parse rather than writing a lossy sheet. This test establishes a prior
// admission while the record is clean, exits, then replaces the repository
// record with an unreadable condition. The rejoin still uses the existing
// lenient projection, and because EverMembers suppresses its save, the unknown
// blob remains whole.
func (s *ConditionsTestSuite) TestACharacterTheSDKCannotFullyLoadIsStillNotDamaged() {
	s.characters.byID["dave"] = dwarfCharacter("dave")
	_, err := s.mgr.Join(context.Background(), &session.JoinInput{
		Session: "sess", Member: "dave", Position: spatial.Position{X: 0, Y: 0},
	})
	s.Require().NoError(err)
	_, err = s.mgr.Exit(context.Background(), &session.ExitInput{Session: "sess", Member: "dave"})
	s.Require().NoError(err)

	corrupt := ragingDwarf("dave")
	corrupt.Conditions = append(corrupt.Conditions,
		json.RawMessage(`{"ref":"homebrew:conditions:hexed","character_id":"dave","stacks":3}`))
	s.characters.byID["dave"] = corrupt
	before := s.storedBytes("dave")

	_, err = s.mgr.Join(context.Background(), &session.JoinInput{
		Session: "sess", Member: "dave", Position: spatial.Position{X: 0, Y: 0},
	})
	s.Require().NoError(err, "the non-first projection remains lenient")

	s.Equal(string(before), string(s.storedBytes("dave")),
		"the condition this build could not read is still in the store")

	var held []json.RawMessage
	s.Require().NoError(json.Unmarshal(s.storedBytes("dave"), &struct {
		Conditions *[]json.RawMessage `json:"conditions"`
	}{Conditions: &held}))
	s.Require().Len(held, 2, "both conditions, including the one we cannot parse")
}
