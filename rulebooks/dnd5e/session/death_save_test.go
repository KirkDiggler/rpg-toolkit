// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package session_test

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/character"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/refs"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/saves"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/session"
	"github.com/KirkDiggler/rpg-toolkit/tools/spatial"
)

type literalDeathSaveDice struct {
	face  int
	calls *int
}

func (d literalDeathSaveDice) Roll(context.Context, int) (int, error) {
	if d.calls != nil {
		*d.calls++
	}
	return d.face, nil
}

type literalPresentationIDs struct {
	value string
	calls *int
}

func (g literalPresentationIDs) Generate() string {
	if g.calls != nil {
		*g.calls++
	}
	return g.value
}

// deathSaveForbiddenIO satisfies every host I/O seam while recording any call.
// Empty declaration validation must return before any of these methods becomes
// reachable.
type deathSaveForbiddenIO struct {
	calls []string
}

func (f *deathSaveForbiddenIO) called(name string) error {
	f.calls = append(f.calls, name)
	return errors.New("unexpected death save I/O: " + name)
}

func (f *deathSaveForbiddenIO) GetSession(context.Context, string) (*session.SessionData, error) {
	return nil, f.called("GetSession")
}

func (f *deathSaveForbiddenIO) SaveSession(context.Context, *session.SessionData) error {
	return f.called("SaveSession")
}

func (f *deathSaveForbiddenIO) GetEncounter(context.Context, string) (*encounter.EncounterData, error) {
	return nil, f.called("GetEncounter")
}

func (f *deathSaveForbiddenIO) SaveEncounter(context.Context, string, *encounter.EncounterData) error {
	return f.called("SaveEncounter")
}

func (f *deathSaveForbiddenIO) GetCharacter(context.Context, string) (*character.Data, error) {
	return nil, f.called("GetCharacter")
}

func (f *deathSaveForbiddenIO) SaveCharacter(context.Context, *character.Data) error {
	return f.called("SaveCharacter")
}

func (f *deathSaveForbiddenIO) Publish(context.Context, []session.Event) error {
	return f.called("Publish")
}

type deathSaveFixture struct {
	t *testing.T

	sessions   *fakeSessions
	encounters *fakeEncounters
	characters *fakeCharacters
	stream     *fakeStream
	mgr        *session.Manager
	rolls      int
	ids        int
	token      string
}

func newDeathSaveFixture(t *testing.T, face int) *deathSaveFixture {
	t.Helper()
	f := &deathSaveFixture{
		t: t, sessions: newFakeSessions(), encounters: newFakeEncounters(),
		characters: newFakeCharacters(armedFighter("alice"), armedFighter("bob")),
		stream:     &fakeStream{}, token: "opaque-save-token",
	}
	mgr, err := session.NewManager(&session.Config{
		Dice: literalDeathSaveDice{face: face, calls: &f.rolls}, TurnDriver: session.Pass{},
		PresentationIDs: testPresentationIDs{value: f.token, calls: &f.ids},
		Sessions:        f.sessions, Encounters: f.encounters, Characters: f.characters, Events: f.stream,
	})
	require.NoError(t, err)
	f.mgr = mgr

	_, err = mgr.StartSession(context.Background(), &session.StartSessionInput{
		Session: "sess", Encounter: "world", World: cryptWorld(t),
	})
	require.NoError(t, err)
	_, err = mgr.Spawn(context.Background(), &session.SpawnInput{
		Session: "sess", ID: "skeleton", Ref: refs.Monsters.Skeleton().String(),
		Position: spatial.Position{X: 2, Y: 1},
	})
	require.NoError(t, err)
	f.rolls = 0
	f.ids = 0
	f.stream.published = nil
	return f
}

func (f *deathSaveFixture) makeDying(successes, failures int) {
	f.t.Helper()
	stored := f.characters.byID["alice"]
	stored.HitPoints = 0
	stored.DeathSaveState = &saves.DeathSaveState{Successes: successes, Failures: failures}

	// Alice became Dying during her current turn. Cross the real turn boundary
	// so her next turn receives the provider's one Death Save capacity.
	_, err := f.mgr.EndTurn(context.Background(), &session.EndTurnInput{
		Session: "sess", Member: "alice",
		DeclarationID: currentEndTurnID(f.t, f.mgr, "sess", "alice"),
	})
	require.NoError(f.t, err)
	f.rolls = 0
	f.ids = 0
	f.stream.published = nil
}

func (f *deathSaveFixture) declaration() session.Declaration {
	f.t.Helper()
	return currentDeclaration(f.t, f.mgr, "sess", "alice", session.VerbDeathSave)
}

func (f *deathSaveFixture) execute(id string) (*session.DeathSaveOutput, error) {
	f.t.Helper()
	return f.mgr.DeathSave(context.Background(), &session.DeathSaveInput{
		Session: "sess", Member: "alice", DeclarationID: id,
	})
}

func TestDeathSaveRejectsEmptyDeclarationBeforeAnyIO(t *testing.T) {
	stores := &deathSaveForbiddenIO{}
	rolls, ids := 0, 0
	mgr, err := session.NewManager(&session.Config{
		Sessions: stores, Encounters: stores, Characters: stores, Events: stores,
		Dice:            literalDeathSaveDice{face: 10, calls: &rolls},
		PresentationIDs: literalPresentationIDs{value: "must-not-generate", calls: &ids},
		TurnDriver:      session.Pass{},
	})
	require.NoError(t, err)

	out, err := mgr.DeathSave(context.Background(), &session.DeathSaveInput{
		Session: "sess", Member: "alice",
	})
	require.Nil(t, out)
	require.EqualError(t, err, "death save: empty declaration id")
	require.ErrorIs(t, err, session.ErrNoDeclarationID)
	require.Empty(t, stores.calls, "empty declaration must precede openForWrite and repository I/O")
	require.Zero(t, ids, "empty declaration must precede offer selection and presentation ID generation")
	require.Zero(t, rolls, "empty declaration must precede dice")
}

func TestDeathSaveAffordIsExplicitAndExclusive(t *testing.T) {
	f := newDeathSaveFixture(t, 10)

	before, err := f.mgr.Afford(context.Background(), &session.AffordInput{Session: "sess", Member: "alice"})
	require.NoError(t, err)
	require.NotContains(t, declarationVerbs(before.Declarations), session.VerbDeathSave,
		"a conscious actor has no death save")

	f.makeDying(0, 0)
	afforded, err := f.mgr.Afford(context.Background(), &session.AffordInput{Session: "sess", Member: "alice"})
	require.NoError(t, err)
	byVerb := declarationsByVerb(afforded.Declarations)
	save := byVerb[session.VerbDeathSave]
	require.Equal(t, session.SlotNone, save.Slot)
	require.Equal(t, session.TargetNone, save.TargetKind)
	require.True(t, save.Available)
	require.NotEmpty(t, save.ID)
	require.Equal(t, &session.DeathSaveRef{Name: "Death Saving Throw"}, save.DeathSave)
	require.Empty(t, save.Candidates)
	require.False(t, byVerb[session.VerbAttack].Available)
	require.False(t, byVerb[session.VerbMove].Available)
	require.False(t, byVerb[session.VerbActivate].Available)
	require.True(t, byVerb[session.VerbEndTurn].Available,
		"End Turn remains independently clock-only")

	for _, state := range []struct {
		name string
		set  func(*character.Data)
	}{
		{name: "stabilized", set: func(data *character.Data) {
			data.DeathSaveState = &saves.DeathSaveState{Successes: 3, Stabilized: true}
		}},
		{name: "dead", set: func(data *character.Data) {
			data.DeathSaveState = &saves.DeathSaveState{Failures: 3, Dead: true}
		}},
	} {
		t.Run(state.name, func(t *testing.T) {
			other := newDeathSaveFixture(t, 10)
			other.makeDying(0, 0)
			state.set(other.characters.byID["alice"])
			out, err := other.mgr.Afford(context.Background(), &session.AffordInput{Session: "sess", Member: "alice"})
			require.NoError(t, err)
			require.NotContains(t, declarationVerbs(out.Declarations), session.VerbDeathSave)
		})
	}

	f.characters.byID["bob"].HitPoints = 0
	world, err := f.mgr.Afford(context.Background(), &session.AffordInput{Session: "sess", Member: "bob"})
	require.NoError(t, err)
	require.Equal(t, session.ClockWorld, world.Clock)
	require.Empty(t, world.Declarations, "world-clock characters receive no declaration")
}

func TestDeathSaveIsNotOfferedToNonActiveDyingCharacter(t *testing.T) {
	sessions, encounters := newFakeSessions(), newFakeEncounters()
	characters := newFakeCharacters(armedFighter("alice"), armedFighter("bob"))
	mgr, err := session.NewManager(&session.Config{
		Dice: testDice{}, TurnDriver: session.Pass{}, PresentationIDs: testPresentationIDs{},
		Sessions: sessions, Encounters: encounters, Characters: characters,
		Events: session.DiscardEvents{},
	})
	require.NoError(t, err)
	_, err = mgr.StartSession(context.Background(), &session.StartSessionInput{
		Session: "sess", Encounter: "world",
		World: turnWorld(freeRoamDuelWorld(t), []string{"alice", "bob"}, 0),
	})
	require.NoError(t, err)
	characters.byID["bob"].HitPoints = 0

	out, err := mgr.Afford(context.Background(), &session.AffordInput{Session: "sess", Member: "bob"})
	require.NoError(t, err)
	require.NotContains(t, declarationVerbs(out.Declarations), session.VerbDeathSave)
}

func TestTurnProjectsExplicitLifeStateAndProviderProgress(t *testing.T) {
	f := newDeathSaveFixture(t, 10)
	f.makeDying(1, 1)

	turn, err := f.mgr.Turn(context.Background(), &session.TurnInput{Session: "sess", Member: "alice"})
	require.NoError(t, err)
	alice := participantByID(t, turn.Participants, "alice")
	require.Equal(t, session.StandingDowned, alice.Standing)
	require.Equal(t, session.LifeStateDying, alice.LifeState)
	require.Equal(t, &session.DeathSaveProgress{
		Successes: 1, Failures: 1, SuccessesNeeded: 2, FailuresRemaining: 2,
	}, alice.DeathSaves)

	// Third success stabilizes but encounter's settlement deferral keeps the
	// current slot available for the explicit END_TURN continuation.
	f.characters.byID["alice"].DeathSaveState.Successes = 2
	out, err := f.execute(f.declaration().ID)
	require.NoError(t, err)
	require.Equal(t, session.DeathSaveOutcomeStabilized, out.Outcome)
	turn, err = f.mgr.Turn(context.Background(), &session.TurnInput{Session: "sess", Member: "alice"})
	require.NoError(t, err)
	alice = participantByID(t, turn.Participants, "alice")
	require.Equal(t, session.LifeStateStabilized, alice.LifeState)
	require.NotNil(t, alice.DeathSaves)
	require.True(t, alice.DeathSaves.Stabilized)
	require.Equal(t, 3, alice.DeathSaves.Successes)
}

func TestDeathSaveOutcomesAndContinuations(t *testing.T) {
	cases := []struct {
		name       string
		face       int
		successes  int
		failures   int
		outcome    session.DeathSaveOutcome
		addedS     int
		addedF     int
		wantS      int
		wantF      int
		stabilized bool
		dead       bool
		recovered  bool
		hp         int
		continueAs session.DeathSaveContinuation
	}{
		{name: "ordinary success", face: 10, outcome: session.DeathSaveOutcomeSuccess,
			addedS: 1, wantS: 1, continueAs: session.DeathSaveContinuationEndTurn},
		{name: "ordinary failure", face: 9, outcome: session.DeathSaveOutcomeFailure,
			addedF: 1, wantF: 1, continueAs: session.DeathSaveContinuationEndTurn},
		{name: "third success", face: 10, successes: 2, outcome: session.DeathSaveOutcomeStabilized,
			addedS: 1, wantS: 3, stabilized: true, continueAs: session.DeathSaveContinuationEndTurn},
		{name: "third failure", face: 9, failures: 2, outcome: session.DeathSaveOutcomeDead,
			addedF: 1, wantF: 3, dead: true, continueAs: session.DeathSaveContinuationAlreadyAdvanced},
		{name: "natural twenty", face: 20, outcome: session.DeathSaveOutcomeRecovered,
			recovered: true, hp: 1, continueAs: session.DeathSaveContinuationKeepTurn},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			f := newDeathSaveFixture(t, tc.face)
			f.makeDying(tc.successes, tc.failures)
			declaration := f.declaration()
			out, err := f.execute(declaration.ID)
			require.NoError(t, err)
			require.Equal(t, tc.face, out.Roll)
			require.Equal(t, tc.outcome, out.Outcome)
			require.Equal(t, tc.addedS, out.SuccessesAdded)
			require.Equal(t, tc.addedF, out.FailuresAdded)
			require.Equal(t, tc.wantS, out.Successes)
			require.Equal(t, tc.wantF, out.Failures)
			require.Equal(t, max(0, 3-tc.wantS), out.SuccessesNeeded)
			require.Equal(t, max(0, 3-tc.wantF), out.FailuresRemaining)
			require.Equal(t, tc.stabilized, out.Stabilized)
			require.Equal(t, tc.dead, out.Dead)
			require.Equal(t, tc.recovered, out.Recovered)
			require.Equal(t, tc.hp, out.HPRestored)
			require.Equal(t, tc.continueAs, out.Continuation)
			require.Equal(t, f.token, out.PresentationID)
			require.Positive(t, out.Seq)
			require.Equal(t, 1, f.rolls, "an accepted declaration rolls exactly once")
			require.Equal(t, 1, f.ids, "an accepted declaration generates exactly one opaque token")
			require.Contains(t, out.Saved.Written, "character:alice")
			require.Contains(t, out.Saved.Written, "encounter:world")
			deathRecipients := map[string]bool{}
			for _, event := range f.stream.published {
				if event.Kind == session.EventDeathSave {
					deathRecipients[event.Recipient] = true
				}
			}
			require.Equal(t, map[string]bool{"alice": true, "bob": true, "skeleton": true},
				deathRecipients, "the Death Save reaches the whole three-member table")

			stored := f.characters.byID["alice"]
			require.Equal(t, tc.hp, stored.HitPoints)
			require.NotNil(t, stored.DeathSaveState)
			require.Equal(t, tc.wantS, stored.DeathSaveState.Successes)
			require.Equal(t, tc.wantF, stored.DeathSaveState.Failures)

			turn, err := f.mgr.Turn(context.Background(), &session.TurnInput{Session: "sess", Member: "alice"})
			require.NoError(t, err)
			switch tc.continueAs {
			case session.DeathSaveContinuationAlreadyAdvanced:
				require.NotEqual(t, "alice", turn.Active)
			default:
				require.Equal(t, "alice", turn.Active)
			}
			if tc.recovered {
				next, err := f.mgr.Afford(context.Background(), &session.AffordInput{Session: "sess", Member: "alice"})
				require.NoError(t, err)
				require.NotContains(t, declarationVerbs(next.Declarations), session.VerbDeathSave)
				require.True(t, declarationsByVerb(next.Declarations)[session.VerbAttack].Available)
			}
		})
	}
}

func TestDeathSaveStoryAndResponseShareOpaqueFactsAcrossLocalSequences(t *testing.T) {
	f := newDeathSaveFixture(t, 10)
	f.makeDying(0, 0)

	// Turn one already-persisted shared beat into an Alice-only beat and update
	// Bob's cursor to the matching honest count. Their next shared beat then has
	// different recipient-local numbers without an invalid cursor.
	world := f.encounters.byID["world"]
	removed := false
	for i := range world.Log.Entries {
		audience := world.Log.Entries[i].Audience[:0]
		for _, recipient := range world.Log.Entries[i].Audience {
			if string(recipient) == "bob" && !removed {
				removed = true
				continue
			}
			audience = append(audience, recipient)
		}
		world.Log.Entries[i].Audience = audience
		if removed {
			break
		}
	}
	require.True(t, removed)
	bobCursor := f.sessions.byID["sess"].Streams["bob"]
	require.Positive(t, bobCursor.Count)
	bobCursor.Count--
	f.sessions.byID["sess"].Streams["bob"] = bobCursor

	out, err := f.execute(f.declaration().ID)
	require.NoError(t, err)
	deathEvents := map[string]session.Event{}
	for _, event := range f.stream.published {
		if event.Kind == session.EventDeathSave {
			deathEvents[event.Recipient] = event
		}
	}
	require.Len(t, deathEvents, 3)
	actorBody, ok := deathEvents["alice"].Body.(session.DeathSaveBody)
	require.True(t, ok)
	witnessBody, ok := deathEvents["bob"].Body.(session.DeathSaveBody)
	require.True(t, ok)
	require.Equal(t, f.token, actorBody.PresentationID)
	require.Equal(t, actorBody, witnessBody)
	require.Equal(t, out.Roll, actorBody.Roll)
	require.Equal(t, out.Outcome, actorBody.Outcome)
	require.Equal(t, out.Seq, deathEvents["alice"].Seq)
	require.NotEqual(t, deathEvents["alice"].Seq, deathEvents["bob"].Seq,
		"recipient-local cursors may differ without changing correlation")
}

func TestDeathSaveRejectsStaleAndReplayBeforeEntropyOrMutation(t *testing.T) {
	f := newDeathSaveFixture(t, 10)
	f.makeDying(0, 0)
	before, err := json.Marshal(f.characters.byID["alice"])
	require.NoError(t, err)

	out, err := f.execute("v2.stale")
	require.Nil(t, out)
	require.ErrorIs(t, err, session.ErrStaleDeclaration)
	require.Zero(t, f.rolls)
	require.Zero(t, f.ids)
	after, err := json.Marshal(f.characters.byID["alice"])
	require.NoError(t, err)
	require.JSONEq(t, string(before), string(after))

	id := f.declaration().ID
	accepted, err := f.execute(id)
	require.NoError(t, err)
	require.NotNil(t, accepted)
	afforded, err := f.mgr.Afford(context.Background(), &session.AffordInput{Session: "sess", Member: "alice"})
	require.NoError(t, err)
	require.NotContains(t, declarationVerbs(afforded.Declarations), session.VerbDeathSave,
		"this turn's accepted save spends the sole declaration")
	rolls, ids := f.rolls, f.ids
	stored, err := json.Marshal(f.characters.byID["alice"])
	require.NoError(t, err)

	replayed, err := f.execute(id)
	require.Nil(t, replayed)
	require.ErrorIs(t, err, session.ErrStaleDeclaration)
	require.Equal(t, rolls, f.rolls)
	require.Equal(t, ids, f.ids)
	afterReplay, err := json.Marshal(f.characters.byID["alice"])
	require.NoError(t, err)
	require.JSONEq(t, string(stored), string(afterReplay))
}

func TestDeathSaveRejectsInvalidGeneratedPresentationIDBeforeRoll(t *testing.T) {
	for _, token := range []string{"", "not/wire-safe", strings.Repeat("a", 129)} {
		t.Run(token, func(t *testing.T) {
			f := newDeathSaveFixture(t, 10)
			f.makeDying(0, 0)
			declaration := f.declaration()
			mgr, err := session.NewManager(&session.Config{
				Dice: literalDeathSaveDice{face: 10, calls: &f.rolls}, TurnDriver: session.Pass{},
				PresentationIDs: literalPresentationIDs{value: token, calls: &f.ids},
				Sessions:        f.sessions, Encounters: f.encounters,
				Characters: f.characters, Events: f.stream,
			})
			require.NoError(t, err)

			savesBefore := f.characters.saves
			out, err := mgr.DeathSave(context.Background(), &session.DeathSaveInput{
				Session: "sess", Member: "alice", DeclarationID: declaration.ID,
			})
			require.Nil(t, out)
			require.Error(t, err)
			require.Equal(t, 1, f.ids)
			require.Zero(t, f.rolls)
			require.Equal(t, savesBefore, f.characters.saves,
				"invalid presentation IDs persist nothing")
		})
	}
}

func TestDeathSavePartialWritePreventsRetry(t *testing.T) {
	f := newDeathSaveFixture(t, 10)
	f.makeDying(0, 0)
	declaration := f.declaration()
	failing := &failingEncounters{fakeEncounters: f.encounters, saveErr: errors.New("world unavailable")}
	mgr, err := session.NewManager(&session.Config{
		Dice: literalDeathSaveDice{face: 10, calls: &f.rolls}, TurnDriver: session.Pass{},
		PresentationIDs: testPresentationIDs{value: f.token, calls: &f.ids},
		Sessions:        f.sessions, Encounters: failing, Characters: f.characters, Events: f.stream,
	})
	require.NoError(t, err)

	out, err := mgr.DeathSave(context.Background(), &session.DeathSaveInput{
		Session: "sess", Member: "alice", DeclarationID: declaration.ID,
	})
	require.Nil(t, out)
	require.ErrorIs(t, err, session.ErrSaveFailed)
	var saveErr *session.SaveError
	require.ErrorAs(t, err, &saveErr)
	require.Contains(t, saveErr.Report.Written, "character:alice")
	require.Contains(t, saveErr.Report.Failed, "encounter:world")

	rolls, ids := f.rolls, f.ids
	retry, err := mgr.DeathSave(context.Background(), &session.DeathSaveInput{
		Session: "sess", Member: "alice", DeclarationID: declaration.ID,
	})
	require.Nil(t, retry)
	require.ErrorIs(t, err, session.ErrStaleDeclaration)
	require.Equal(t, rolls, f.rolls)
	require.Equal(t, ids, f.ids)
}

func participantByID(t *testing.T, participants []session.Participant, member string) session.Participant {
	t.Helper()
	for _, participant := range participants {
		if participant.Member == member {
			return participant
		}
	}
	t.Fatalf("participant %q not found", member)
	return session.Participant{}
}

func declarationVerbs(declarations []session.Declaration) []session.Verb {
	out := make([]session.Verb, 0, len(declarations))
	for _, declaration := range declarations {
		out = append(out, declaration.Verb)
	}
	return out
}

func declarationsByVerb(declarations []session.Declaration) map[session.Verb]session.Declaration {
	out := make(map[session.Verb]session.Declaration, len(declarations))
	for _, declaration := range declarations {
		out[declaration.Verb] = declaration
	}
	return out
}
