// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package session_test

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/session"
	"github.com/KirkDiggler/rpg-toolkit/tools/spatial"
)

// selectorFixture gives every refusal case fresh repositories and independent
// mutation counters. The counters are reset after setup (and after obtaining a
// current selector), so zero means this attempted verb wrote and recorded
// nothing rather than merely restoring an equal value.
type selectorFixture struct {
	t          *testing.T
	mgr        *session.Manager
	sessions   *fakeSessions
	encounters *fakeEncounters
	characters *fakeCharacters
	stream     *fakeStream
	roller     *sequenceDice
}

func newSelectorFixture(t *testing.T, world *encounter.EncounterData) *selectorFixture {
	t.Helper()
	f := &selectorFixture{
		t:          t,
		sessions:   newFakeSessions(),
		encounters: newFakeEncounters(),
		characters: newFakeCharacters(armedFighter("alice"), armedFighter("bob")),
		stream:     &fakeStream{},
		roller:     &sequenceDice{rolls: []int{15, 5}},
	}
	mgr, err := session.NewManager(&session.Config{PresentationIDs: testPresentationIDs{},
		Dice: f.roller, TurnDriver: session.Pass{}, Sessions: f.sessions, Encounters: f.encounters,
		Characters: f.characters, Events: f.stream,
	})
	require.NoError(t, err)
	f.mgr = mgr
	_, err = mgr.StartSession(context.Background(), &session.StartSessionInput{
		Session: "sess", Encounter: "world", World: world,
	})
	require.NoError(t, err)
	f.resetMutationCounters()
	return f
}

func (f *selectorFixture) resetMutationCounters() {
	f.sessions.saves = 0
	f.encounters.saves = 0
	f.encounters.records = 0
	f.characters.saves = 0
	f.stream.published = nil
	f.roller.next = 0
}

type selectorState struct {
	position spatial.Position
	clock    string
}

func (f *selectorFixture) state(member string) selectorState {
	f.t.Helper()
	world := f.encounters.byID["world"]
	var position spatial.Position
	found := false
	for _, candidate := range world.Members {
		if string(candidate.ID) == member && candidate.Cell != nil {
			position = spatial.Position{X: candidate.Cell.X, Y: candidate.Cell.Y}
			found = true
			break
		}
	}
	require.True(f.t, found, "member %q must be in the stored world", member)
	clock, err := json.Marshal(struct {
		World   any `json:"world"`
		Bubbles any `json:"bubbles"`
	}{World: world.Clock, Bubbles: world.Bubbles})
	require.NoError(f.t, err)
	return selectorState{position: position, clock: string(clock)}
}

func (f *selectorFixture) requireNoMutation(before selectorState) {
	f.t.Helper()
	require.Zero(f.t, f.roller.next, "selector refusal must roll no dice")
	require.Zero(f.t, f.characters.saves, "selector refusal must write no character")
	require.Zero(f.t, f.sessions.saves, "selector refusal must write no session")
	require.Zero(f.t, f.encounters.saves, "selector refusal must write no encounter")
	require.Zero(f.t, f.encounters.records, "selector refusal must record no story beat")
	require.Empty(f.t, f.stream.published, "selector refusal must publish no event")
	require.Equal(f.t, before, f.state("alice"), "selector refusal must leave position and clock unchanged")
}

func TestAttackSelectorRefusalsMutateNothing(t *testing.T) {
	tests := []struct {
		name     string
		world    func() *encounter.EncounterData
		selector func(*selectorFixture) string
	}{
		{
			name:     "stale selector",
			world:    func() *encounter.EncounterData { return turnWorld(freeRoamDuelWorld(t), []string{"alice", "bob"}, 0) },
			selector: func(*selectorFixture) string { return "v1.stale" },
		},
		{
			name:  "wrong verb selector",
			world: func() *encounter.EncounterData { return turnWorld(freeRoamDuelWorld(t), []string{"alice", "bob"}, 0) },
			selector: func(f *selectorFixture) string {
				return currentMoveID(t, f.mgr, "sess", "alice")
			},
		},
		{
			name:  "currently unavailable selector",
			world: func() *encounter.EncounterData { return reachWorld(t, spatial.Position{X: 5, Y: 1}) },
			selector: func(f *selectorFixture) string {
				declaration := currentDeclaration(t, f.mgr, "sess", "alice", session.VerbAttack)
				require.False(t, declaration.Available)
				return declaration.ID
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			f := newSelectorFixture(t, tc.world())
			id := tc.selector(f)
			f.resetMutationCounters()
			before := f.state("alice")

			out, err := f.mgr.Attack(context.Background(), &session.AttackInput{
				Session: "sess", Attacker: "alice", Target: "bob", DeclarationID: id,
			})
			require.ErrorIs(t, err, session.ErrStaleDeclaration)
			require.Nil(t, out)
			f.requireNoMutation(before)
		})
	}
}

func TestStaleTurnMoveAfterWorldTransitionMutatesNothing(t *testing.T) {
	f := newSelectorFixture(t, turnWorld(freeRoamDuelWorld(t), []string{"alice", "bob"}, 0))
	id := currentMoveID(t, f.mgr, "sess", "alice")
	_, err := f.mgr.Dissolve(context.Background(), &session.DissolveInput{
		Session: "sess", Member: "alice", Cause: session.ByDecision(),
	})
	require.NoError(t, err)
	f.resetMutationCounters()
	before := f.state("alice")

	out, err := f.mgr.Move(context.Background(), &session.MoveInput{
		Session: "sess", Member: "alice", Path: []spatial.Position{{X: 1, Y: 2}}, DeclarationID: id,
	})
	require.ErrorIs(t, err, session.ErrStaleDeclaration)
	require.Nil(t, out)
	f.requireNoMutation(before)
}

func TestStaleEndTurnMutatesNothingEvenWhenARealEndWouldWrapBack(t *testing.T) {
	world := turnWorld(ambushWorld(t), []string{"alice", "ogre"}, 0)
	f := newSelectorFixture(t, world)
	f.resetMutationCounters()
	before := f.state("alice")

	out, err := f.mgr.EndTurn(context.Background(), &session.EndTurnInput{
		Session: "sess", Member: "alice", DeclarationID: "v1.stale",
	})
	require.ErrorIs(t, err, session.ErrStaleDeclaration)
	require.Nil(t, out)
	f.requireNoMutation(before)
}

func TestEndTurnNotYourTurnPrecedesSelectorAndMutatesNothing(t *testing.T) {
	f := newSelectorFixture(t, turnWorld(freeRoamDuelWorld(t), []string{"alice", "bob"}, 0))
	f.resetMutationCounters()
	before := f.state("alice")

	out, err := f.mgr.EndTurn(context.Background(), &session.EndTurnInput{
		Session: "sess", Member: "bob", DeclarationID: "v1.stale",
	})
	require.ErrorIs(t, err, session.ErrNotYourTurn)
	require.NotErrorIs(t, err, session.ErrStaleDeclaration)
	require.Nil(t, out)
	f.requireNoMutation(before)
}

func TestAffordThenEndTurnRejectsRepositorySessionIDMismatch(t *testing.T) {
	f := newSelectorFixture(t, turnWorld(freeRoamDuelWorld(t), []string{"alice", "bob"}, 0))
	id := currentEndTurnID(t, f.mgr, "sess", "alice")
	f.sessions.byID["sess"].ID = "different-session"
	f.resetMutationCounters()
	before := f.state("alice")

	afford, err := f.mgr.Afford(context.Background(), &session.AffordInput{
		Session: "sess", Member: "alice",
	})
	require.ErrorIs(t, err, session.ErrBadRepository)
	require.Nil(t, afford)

	out, err := f.mgr.EndTurn(context.Background(), &session.EndTurnInput{
		Session: "sess", Member: "alice", DeclarationID: id,
	})
	require.ErrorIs(t, err, session.ErrBadRepository)
	require.Nil(t, out)
	f.requireNoMutation(before)
}

// actorLoadCheckingDice observes the execution boundary: by the first attack
// roll, the turn path must have loaded the actor exactly once. The resolution
// may legitimately consult standing again after the roll and after damage.
type actorLoadCheckingDice struct {
	t          *testing.T
	characters *fakeCharacters
	rolls      []int
	next       int
}

func (d *actorLoadCheckingDice) Roll(_ context.Context, _ int) (int, error) {
	if d.next == 0 {
		require.Equal(d.t, 1, d.characters.asked["alice"],
			"downed verdict and compiled offer must share one strict actor load before execution")
		require.Equal(d.t, 1, d.characters.asked["bob"],
			"compiled cast must snapshot each non-actor participant once and execution must not refetch it")
	}
	require.Less(d.t, d.next, len(d.rolls), "execution requested an unexpected die roll")
	roll := d.rolls[d.next]
	d.next++
	return roll, nil
}

func TestSuccessfulTurnAttackLoadsActorOnceBeforeExecution(t *testing.T) {
	sessions, encounters := newFakeSessions(), newFakeEncounters()
	characters := newFakeCharacters(armedFighter("alice"), armedFighter("bob"))
	dice := &actorLoadCheckingDice{t: t, characters: characters, rolls: []int{15, 5}}
	mgr, err := session.NewManager(&session.Config{PresentationIDs: testPresentationIDs{},
		Dice: dice, TurnDriver: session.Pass{}, Sessions: sessions, Encounters: encounters,
		Characters: characters, Events: session.DiscardEvents{},
	})
	require.NoError(t, err)
	_, err = mgr.StartSession(context.Background(), &session.StartSessionInput{
		Session: "sess", Encounter: "world",
		World: turnWorld(freeRoamDuelWorld(t), []string{"alice", "bob"}, 0),
	})
	require.NoError(t, err)
	id := currentAttackID(t, mgr, "sess", "alice")
	characters.asked["alice"] = 0
	characters.asked["bob"] = 0
	characters.loads = 0

	out, err := mgr.Attack(context.Background(), &session.AttackInput{
		Session: "sess", Attacker: "alice", Target: "bob", DeclarationID: id,
	})
	require.NoError(t, err)
	require.NotNil(t, out)
}
