// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package session_test

import (
	"context"
	"sync"
	"time"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/session"
	"github.com/KirkDiggler/rpg-toolkit/tools/spatial"
)

// This test coordinator deliberately exercises real mutex exclusion. Its
// requests signal precedes acquisition so the test can prove calls overlap.
type overlapLocker struct {
	mu       sync.Mutex
	requests chan struct{}
}

func (l *overlapLocker) LockSession(_ context.Context, _ *session.LockSessionInput) (*session.LockSessionOutput, error) {
	if l.requests != nil {
		l.requests <- struct{}{}
	}
	l.mu.Lock()
	return &session.LockSessionOutput{Release: l.mu.Unlock}, nil
}

// LockCharacter grants every character guard at once: this coordinator
// proves session exclusion, and a character guard taken while the session's
// is held cannot contend with anything in these scenes.
func (l *overlapLocker) LockCharacter(context.Context, *session.LockCharacterInput) (*session.LockCharacterOutput, error) {
	return &session.LockCharacterOutput{Release: func() {}}, nil
}

type overlapEncounters struct {
	*fakeEncounters
	pause   bool
	once    sync.Once
	entered chan struct{}
	resume  chan struct{}
}

func (r *overlapEncounters) SaveEncounter(ctx context.Context, id string, data *encounter.EncounterData) error {
	if r.pause {
		r.once.Do(func() {
			close(r.entered)
			select {
			case <-r.resume:
			case <-ctx.Done():
			}
		})
	}
	return r.fakeEncounters.SaveEncounter(ctx, id, data)
}

func (s *SessionLockSuite) TestOverlappingMovesBothSurviveWithoutLostUpdates() {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	locker := &overlapLocker{}
	worlds := &overlapEncounters{fakeEncounters: newFakeEncounters(), entered: make(chan struct{}), resume: make(chan struct{})}
	var resumeOnce sync.Once
	resume := func() { resumeOnce.Do(func() { close(worlds.resume) }) }
	defer resume()
	mgr, err := session.NewManager(&session.Config{Seats: newFakeSeats(),
		Sessions: newFakeSessions(), Encounters: worlds, Characters: testCharacters(),
		Events: session.DiscardEvents{}, Dice: testDice{}, PresentationIDs: testPresentationIDs{},
		TurnDriver: session.Pass{}, Locker: locker,
	})
	s.Require().NoError(err)
	world := authoredWorld()
	world.Session = "run"
	world.Party = append(world.Party, sceneSeat{ID: "bob", At: authoredOf(spatial.Position{X: 2, Y: 2})})
	launchScene(s.T(), mgr, world)

	worlds.pause = true
	locker.requests = make(chan struct{}, 2)
	completed := make(chan error, 2)
	go func() {
		_, moveErr := mgr.Move(ctx, &session.MoveInput{Session: "run", Member: "alice", Path: []spatial.Position{{X: 2, Y: 1}}})
		completed <- moveErr
	}()
	select {
	case <-worlds.entered:
	case <-ctx.Done():
		s.FailNow("first move never reached its save")
	}
	select {
	case <-locker.requests: // first operation's acquisition
	case <-ctx.Done():
		s.FailNow("first move did not acquire the guard")
	}
	go func() {
		_, moveErr := mgr.Move(ctx, &session.MoveInput{Session: "run", Member: "bob", Path: []spatial.Position{{X: 3, Y: 2}}})
		completed <- moveErr
	}()
	select {
	case <-locker.requests: // second call begins while first is paused before save
	case <-ctx.Done():
		s.FailNow("second move never requested the guard")
	}
	resume()
	for range 2 {
		select {
		case moveErr := <-completed:
			s.Require().NoError(moveErr)
		case <-ctx.Done():
			s.FailNow("overlapping calls did not finish")
		}
	}
	locker.requests = nil

	for member, expected := range map[string]spatial.Position{
		"alice": {X: 2, Y: 1}, "bob": {X: 3, Y: 2},
	} {
		where, readErr := mgr.Where(ctx, &session.WhereInput{Session: "run", Member: member})
		s.Require().NoError(readErr)
		s.Equal(expected, where.Position, "both independently valid moves must survive persistence")
	}
	story, err := mgr.Story(ctx, &session.StoryInput{Session: "run", Member: "alice"})
	s.Require().NoError(err)
	moves := map[string]int{}
	for _, event := range story {
		if body, ok := event.Body.(session.MovedBody); ok {
			moves[body.Member]++
		}
	}
	s.Equal(map[string]int{"alice": 1, "bob": 1}, moves, "the recorded outcomes survive too")
}
