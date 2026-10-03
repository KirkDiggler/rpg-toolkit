// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package session_test

import (
	"context"
	"reflect"
	"testing"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/session"
	"github.com/KirkDiggler/rpg-toolkit/tools/spatial"
	"github.com/stretchr/testify/suite"
)

type observingLocker struct {
	held      bool
	calls     []string
	releases  int
	err       error
	invalid   bool
	nilResult bool
}

func (l *observingLocker) LockSession(_ context.Context, in *session.LockSessionInput) (*session.LockSessionOutput, error) {
	l.calls = append(l.calls, in.Session)
	if l.err != nil {
		return nil, l.err
	}
	if l.invalid {
		// Deliberately violate the provider contract to test fail-closed handling.
		if l.nilResult {
			return nil, nil
		}
		return &session.LockSessionOutput{}, nil
	}
	l.held = true
	return &session.LockSessionOutput{Release: func() {
		l.held = false
		l.releases++
	}}, nil
}

type guardedSessions struct {
	session.SessionRepository
	probe func()
}

func (r guardedSessions) GetSession(ctx context.Context, id string) (*session.SessionData, error) {
	r.probe()
	return r.SessionRepository.GetSession(ctx, id)
}

func (r guardedSessions) SaveSession(ctx context.Context, in *session.SessionData) error {
	r.probe()
	return r.SessionRepository.SaveSession(ctx, in)
}

type guardedEncounters struct {
	session.EncounterRepository
	probe func()
}

func (r guardedEncounters) GetEncounter(ctx context.Context, id string) (*encounter.EncounterData, error) {
	r.probe()
	return r.EncounterRepository.GetEncounter(ctx, id)
}

func (r guardedEncounters) SaveEncounter(ctx context.Context, id string, in *encounter.EncounterData) error {
	r.probe()
	return r.EncounterRepository.SaveEncounter(ctx, id, in)
}

type guardedStream struct {
	probe     func()
	published int
}

func (p *guardedStream) Publish(_ context.Context, events []session.Event) error {
	p.probe()
	p.published += len(events)
	return nil
}

type SessionLockSuite struct {
	suite.Suite
	mgr      *session.Manager
	locker   *observingLocker
	store    *failingSessions
	stream   *guardedStream
	accesses int
	probe    func()
}

func TestSessionLockSuite(t *testing.T) { suite.Run(t, new(SessionLockSuite)) }

func (s *SessionLockSuite) SetupTest() {
	s.accesses = 0
	s.locker = &observingLocker{}
	s.store = &failingSessions{fakeSessions: newFakeSessions()}
	s.probe = func() {
		s.accesses++
		s.True(s.locker.held, "repository access and delivery must remain inside the host guard")
	}
	probe := func() { s.probe() }
	s.stream = &guardedStream{probe: probe}
	mgr, err := session.NewManager(&session.Config{
		Sessions:   guardedSessions{SessionRepository: s.store, probe: probe},
		Encounters: guardedEncounters{EncounterRepository: newFakeEncounters(), probe: probe},
		Characters: testCharacters(), Events: s.stream, Dice: testDice{},
		PresentationIDs: testPresentationIDs{}, TurnDriver: session.Pass{}, Locker: s.locker,
	})
	s.Require().NoError(err)
	s.mgr = mgr
}

func (s *SessionLockSuite) TestCreationReadAndMoveHoldThroughDelivery() {
	ctx := context.Background()
	_, err := s.mgr.StartSession(ctx, &session.StartSessionInput{
		Session: "sess", Encounter: "enc", World: authoredWorld(s.T()),
	})
	s.Require().NoError(err)
	s.Equal(1, s.locker.releases)
	_, err = s.mgr.Status(ctx, &session.StatusInput{Session: "sess"})
	s.Require().NoError(err)
	s.Equal(2, s.locker.releases)
	_, err = s.mgr.Move(ctx, &session.MoveInput{
		Session: "sess", Member: "alice", Path: []spatial.Position{{X: 2, Y: 1}},
	})
	s.Require().NoError(err)
	s.Equal([]string{"sess", "sess", "sess"}, s.locker.calls)
	s.Equal(3, s.locker.releases)
	s.Positive(s.stream.published)
	s.False(s.locker.held)
}

func (s *SessionLockSuite) TestEverySessionOperationStopsBeforeIOOnLockFailure() {
	// Exercise the public surface, not just the helper/AST: a new operation
	// with a Session field must actually consult the configured locker.
	s.locker.err = context.Canceled
	value := reflect.ValueOf(s.mgr)
	typ := value.Type()
	for i := 0; i < typ.NumMethod(); i++ {
		method := typ.Method(i)
		if method.Type.NumIn() != 3 || method.Type.In(2).Kind() != reflect.Pointer {
			continue
		}
		inputType := method.Type.In(2).Elem()
		if inputType.Kind() != reflect.Struct {
			continue
		}
		field, ok := inputType.FieldByName("Session")
		if !ok || field.Type.Kind() != reflect.String {
			continue
		}
		s.Run(method.Name, func() {
			in := reflect.New(inputType)
			in.Elem().FieldByName("Session").SetString("sess")
			result := value.Method(i).Call([]reflect.Value{reflect.ValueOf(context.Background()), in})
			s.Require().Len(result, 2)
			s.True(result[0].IsNil())
			err, ok := result[1].Interface().(error)
			s.Require().True(ok)
			s.ErrorIs(err, context.Canceled)
		})
	}
	s.Zero(s.accesses)
	s.Zero(s.locker.releases)
}

func (s *SessionLockSuite) TestInvalidSuccessfulLockFailsClosed() {
	s.locker.invalid = true
	for _, nilResult := range []bool{false, true} {
		s.locker.nilResult = nilResult
		_, err := s.mgr.Status(context.Background(), &session.StatusInput{Session: "sess"})
		s.ErrorIs(err, session.ErrBadSessionLock)
		s.Zero(s.accesses)
	}
}

func (s *SessionLockSuite) TestRepositoryErrorAndPanicBothRelease() {
	s.store.getErr = errBroken
	_, err := s.mgr.Status(context.Background(), &session.StatusInput{Session: "sess"})
	s.Require().Error(err)
	s.Equal(1, s.locker.releases)
	s.False(s.locker.held)

	s.probe = func() { panic("repository panic") }
	s.Panics(func() {
		_, _ = s.mgr.Status(context.Background(), &session.StatusInput{Session: "sess"})
	})
	s.Equal(2, s.locker.releases)
	s.False(s.locker.held)
}

func (s *SessionLockSuite) TestNilAndMissingSessionKeepValidation() {
	_, err := s.mgr.Status(context.Background(), nil)
	s.ErrorIs(err, session.ErrNilInput)
	_, err = s.mgr.Status(context.Background(), &session.StatusInput{})
	s.ErrorIs(err, session.ErrNoSessionID)
	s.Empty(s.locker.calls)
	s.Zero(s.accesses)
}
