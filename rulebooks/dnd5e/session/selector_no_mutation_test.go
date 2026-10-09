// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package session_test

import (
	"context"
	"encoding/json"
	"runtime"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/character"
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

// newSelectorFixture launches world; a non-empty order also authors a turn
// clock over those members with order[0] active.
func newSelectorFixture(t *testing.T, world scene, order ...string) *selectorFixture {
	t.Helper()
	f := &selectorFixture{
		t:          t,
		sessions:   newFakeSessions(),
		encounters: newFakeEncounters(),
		characters: newFakeCharacters(armedFighter("alice"), armedFighter("bob")),
		stream:     &fakeStream{},
		roller:     &sequenceDice{rolls: []int{15, 5}},
	}
	mgr, err := session.NewManager(&session.Config{Seats: newFakeSeats(), PresentationIDs: testPresentationIDs{},
		Dice: f.roller, TurnDriver: session.Pass{}, Sessions: f.sessions, Encounters: f.encounters,
		Characters: f.characters, Events: f.stream,
	})
	require.NoError(t, err)
	f.mgr = mgr
	if len(order) > 0 {
		launchOnClock(t, mgr, f.encounters, world, order, 0)
	} else {
		launchScene(t, mgr, world)
	}
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
	world := f.encounters.byID[testSession]
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
		world    func() scene
		order    []string
		selector func(*selectorFixture) string
	}{
		{
			name:     "stale selector",
			world:    freeRoamDuelWorld,
			order:    duelClock,
			selector: func(*selectorFixture) string { return "v1.stale" },
		},
		{
			name:  "wrong verb selector",
			world: freeRoamDuelWorld,
			order: duelClock,
			selector: func(f *selectorFixture) string {
				return currentMoveID(t, f.mgr, "sess", "alice")
			},
		},
		{
			name:  "currently unavailable selector",
			world: func() scene { return reachWorld(spatial.Position{X: 5, Y: 1}) },
			order: duelClock,
			selector: func(f *selectorFixture) string {
				declaration := currentDeclaration(t, f.mgr, "sess", "alice", session.VerbAttack)
				require.False(t, declaration.Available)
				return declaration.ID
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			f := newSelectorFixture(t, tc.world(), tc.order...)
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
	f := newSelectorFixture(t, freeRoamDuelWorld(), duelClock...)
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
	f := newSelectorFixture(t, ambushWorld(), "alice", "ogre")
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
	f := newSelectorFixture(t, freeRoamDuelWorld(), duelClock...)
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
	f := newSelectorFixture(t, freeRoamDuelWorld(), duelClock...)
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
	characters *compileLoadCounting
	rolls      []int
	next       int
}

func (d *actorLoadCheckingDice) Roll(_ context.Context, _ int) (int, error) {
	if d.next == 0 {
		require.Equal(d.t, 1, d.characters.compiled["alice"],
			"downed verdict and compiled offer must share one strict actor load before execution")
		require.Equal(d.t, 1, d.characters.compiled["bob"],
			"compiled cast must snapshot each non-actor participant once and execution must not refetch it")
	}
	require.Less(d.t, d.next, len(d.rolls), "execution requested an unexpected die roll")
	roll := d.rolls[d.next]
	d.next++
	return roll, nil
}

// compileLoadCounting counts the store reads the verb's own compile and cast
// make, apart from the sheet seam's (sheets.go). The seam re-reads a sheet at
// every consult on purpose — how far a member sees, how fast it walks, asked
// when the world uses either and never cached (rpg-project#538) — so its
// reads are not the strict load this test pins. Told apart by the caller, the
// one honest way a repository can tell who asked.
type compileLoadCounting struct {
	*fakeCharacters
	compiled map[string]int
}

func (c *compileLoadCounting) GetCharacter(ctx context.Context, id string) (*character.Data, error) {
	if !calledFromSheetSeam() {
		c.compiled[id]++
	}
	return c.fakeCharacters.GetCharacter(ctx, id)
}

// calledFromSheetSeam reports whether the sheet seam (sheets.go) is on the
// caller's stack: the one honest way a test repository can tell the seam's
// per-consult reads apart from every other reader.
func calledFromSheetSeam() bool {
	pcs := make([]uintptr, 64)
	frames := runtime.CallersFrames(pcs[:runtime.Callers(2, pcs)])
	for {
		frame, more := frames.Next()
		if strings.Contains(frame.Function, "session.sheetSeam.") {
			return true
		}
		if !more {
			return false
		}
	}
}

func TestSuccessfulTurnAttackLoadsActorOnceBeforeExecution(t *testing.T) {
	sessions, encounters := newFakeSessions(), newFakeEncounters()
	characters := &compileLoadCounting{
		fakeCharacters: newFakeCharacters(armedFighter("alice"), armedFighter("bob")),
		compiled:       map[string]int{},
	}
	dice := &actorLoadCheckingDice{t: t, characters: characters, rolls: []int{15, 5}}
	mgr, err := session.NewManager(&session.Config{Seats: newFakeSeats(), PresentationIDs: testPresentationIDs{},
		Dice: dice, TurnDriver: session.Pass{}, Sessions: sessions, Encounters: encounters,
		Characters: characters, Events: session.DiscardEvents{},
	})
	require.NoError(t, err)
	launchOnClock(t, mgr, encounters, freeRoamDuelWorld(), []string{"alice", "bob"}, 0)
	id := currentAttackID(t, mgr, "sess", "alice")
	characters.compiled["alice"] = 0
	characters.compiled["bob"] = 0

	out, err := mgr.Attack(context.Background(), &session.AttackInput{
		Session: "sess", Attacker: "alice", Target: "bob", DeclarationID: id,
	})
	require.NoError(t, err)
	require.NotNil(t, out)
}
