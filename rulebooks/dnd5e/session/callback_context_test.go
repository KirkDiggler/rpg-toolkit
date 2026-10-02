// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package session_test

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/suite"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/character"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/conditions"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/refs"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/session"
	"github.com/KirkDiggler/rpg-toolkit/tools/spatial"
)

type callbackContextKey struct{}

// contextCharacters models a host whose storage requires request-scoped values.
// It checks deadline and cancellation identity too, without teaching the SDK
// what the host's value means.
type contextCharacters struct {
	inner    *fakeCharacters
	want     context.Context
	reads    int
	writes   int
	canceled int
}

func (r *contextCharacters) check(ctx context.Context) error {
	if ctx == nil || ctx.Value(callbackContextKey{}) != r.want.Value(callbackContextKey{}) {
		return fmt.Errorf("character callback lost request value")
	}
	deadline, hasDeadline := ctx.Deadline()
	wantDeadline, wantHasDeadline := r.want.Deadline()
	if hasDeadline != wantHasDeadline || !deadline.Equal(wantDeadline) || ctx.Done() != r.want.Done() {
		return fmt.Errorf("character callback lost deadline or cancellation")
	}
	if ctx.Err() == context.Canceled {
		r.canceled++
	}
	return nil
}

func (r *contextCharacters) GetCharacter(ctx context.Context, id string) (*character.Data, error) {
	if err := r.check(ctx); err != nil {
		return nil, err
	}
	r.reads++
	return r.inner.GetCharacter(ctx, id)
}

func (r *contextCharacters) SaveCharacter(ctx context.Context, data *character.Data) error {
	if err := r.check(ctx); err != nil {
		return err
	}
	r.writes++
	return r.inner.SaveCharacter(ctx, data)
}

type CallbackContextSuite struct {
	suite.Suite
	characters *contextCharacters
	manager    *session.Manager
}

func TestCallbackContextSuite(t *testing.T) { suite.Run(t, new(CallbackContextSuite)) }

func (s *CallbackContextSuite) SetupTest() {
	s.characters = &contextCharacters{inner: newFakeCharacters(armedFighter("fighter"))}
}

func (s *CallbackContextSuite) callContext(name string) context.Context {
	ctx, cancel := context.WithTimeout(context.WithValue(context.Background(), callbackContextKey{}, name), time.Minute)
	s.T().Cleanup(cancel)
	s.characters.want = ctx
	s.characters.reads, s.characters.writes = 0, 0
	return ctx
}

func (s *CallbackContextSuite) start(driver session.TurnDriver, skeletonAt spatial.Position) {
	var err error
	s.manager, err = session.NewManager(&session.Config{
		PresentationIDs: testPresentationIDs{}, Dice: testDice{}, TurnDriver: driver,
		Sessions: newFakeSessions(), Encounters: newFakeEncounters(),
		Characters: s.characters, Events: session.DiscardEvents{},
	})
	s.Require().NoError(err)
	_, err = s.manager.StartSession(s.callContext("start"), &session.StartSessionInput{
		Session: "sess", Encounter: "world", World: tombRoom(12, 6),
	})
	s.Require().NoError(err)
	_, err = s.manager.Join(s.callContext("join"), &session.JoinInput{
		Session: "sess", Member: "fighter", Position: hexCell(1, 0),
	})
	s.Require().NoError(err)
	spawned, err := s.manager.Spawn(s.callContext("spawn"), &session.SpawnInput{
		Session: "sess", ID: "skel-1", Ref: refs.Monsters.Skeleton().String(), Position: skeletonAt,
	})
	s.Require().NoError(err, "fight formation's boundary callback keeps this Spawn's context")
	s.Require().NotNil(spawned.Formed)
	s.Require().Equal("fighter", spawned.Formed.Order[0])
}

func (s *CallbackContextSuite) endTurn(ctx context.Context) {
	afford, err := s.manager.Afford(ctx, &session.AffordInput{Session: "sess", Member: "fighter"})
	s.Require().NoError(err)
	var declaration string
	for _, offered := range afford.Declarations {
		if offered.Verb == session.VerbEndTurn {
			declaration = offered.ID
		}
	}
	s.Require().NotEmpty(declaration)
	_, err = s.manager.EndTurn(ctx, &session.EndTurnInput{
		Session: "sess", Member: "fighter", DeclarationID: declaration,
	})
	s.Require().NoError(err)
}

func (s *CallbackContextSuite) TestBoundaryReadsAndWritesUseEachVerbsContext() {
	s.start(session.Pass{}, hexCell(2, 0))
	stored, err := s.characters.inner.GetCharacter(context.Background(), "fighter")
	s.Require().NoError(err)
	dodge, err := (&conditions.DodgingCondition{MemberID: "fighter"}).ToJSON()
	s.Require().NoError(err)
	stored.Conditions = append(stored.Conditions, dodge)
	s.Require().NoError(s.characters.inner.SaveCharacter(context.Background(), stored))

	// A new context on the same Manager must replace the one used at Spawn.
	ctx := s.callContext("end-turn")
	s.endTurn(ctx)
	s.Positive(s.characters.reads)
	s.Positive(s.characters.writes, "boundary resolution persists the changed sheet")
	stored, err = s.characters.inner.GetCharacter(context.Background(), "fighter")
	s.Require().NoError(err)
	s.Empty(stored.Conditions, "the returning turn expires dodge through the real announcer")
}

func (s *CallbackContextSuite) TestDrivenStrikeKeepsCallerContext() {
	s.start(session.Driver(), hexCell(5, 0))
	before, err := s.characters.inner.GetCharacter(context.Background(), "fighter")
	s.Require().NoError(err)

	s.endTurn(s.callContext("monster-strike"))
	after, err := s.characters.inner.GetCharacter(context.Background(), "fighter")
	s.Require().NoError(err)
	s.Less(after.HitPoints, before.HitPoints, "the real driven strike reaches and saves the target")
	s.Positive(s.characters.writes)
}

func (s *CallbackContextSuite) TestDrivenMovementKeepsCallerContext() {
	s.start(&retreatWalker{path: []spatial.Position{hexCell(3, 0), hexCell(4, 0)}}, hexCell(2, 0))
	stored, err := s.characters.inner.GetCharacter(context.Background(), "fighter")
	s.Require().NoError(err)
	stored.ActionEconomy = &character.ActionEconomyData{
		TurnNumber: 1, ActionsRemaining: 1, BonusActionsRemaining: 1,
		ReactionsRemaining: 1, MovementRemaining: 30,
	}
	s.Require().NoError(s.characters.inner.SaveCharacter(context.Background(), stored))

	ctx := s.callContext("monster-move")
	s.endTurn(ctx)
	afford, err := s.manager.Afford(ctx, &session.AffordInput{Session: "sess", Member: "fighter"})
	s.Require().NoError(err)
	var reaction *session.Declaration
	for i := range afford.Declarations {
		if afford.Declarations[i].Verb == session.VerbReact {
			reaction = &afford.Declarations[i]
		}
	}
	s.Require().NotNil(reaction, "a lost repository context must not silently omit the player's reaction")
	s.Require().NotNil(reaction.Reaction)
	s.Equal(oaRef(), reaction.Reaction.Ref)
}

type cancelingPass struct{ cancel context.CancelFunc }

func (p *cancelingPass) Act(session.MonsterView) (session.TurnIntent, error) {
	if p.cancel != nil {
		p.cancel()
	}
	return session.Pass{}, nil
}

func (s *CallbackContextSuite) TestCancellationDuringDrivenTurnReachesBoundaryStorage() {
	driver := &cancelingPass{}
	s.start(driver, hexCell(2, 0))
	ctx, cancel := context.WithCancel(s.callContext("cancel-during-drive"))
	defer cancel()
	s.characters.want = ctx
	driver.cancel = cancel

	// The test store deliberately completes I/O even when canceled so this
	// checks signal propagation, not a database's cancellation policy.
	s.endTurn(ctx)
	s.ErrorIs(ctx.Err(), context.Canceled)
	s.Positive(s.characters.canceled, "the post-drive boundary sees the original call's cancellation")
}
