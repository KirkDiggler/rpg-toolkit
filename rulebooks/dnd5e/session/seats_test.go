// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package session_test

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/character"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/session"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/weapons"
	"github.com/stretchr/testify/suite"
)

// SeatSuite covers the seat (rpg-project#542, "The seat"): Join seats, Exit
// and End clear, a character is seated in at most one run, and the guard a
// character verb acts under is the one its seat names.
type SeatSuite struct {
	suite.Suite

	sessions   *fakeSessions
	encounters *fakeEncounters
	characters *fakeCharacters
	seats      *fakeSeats
	locker     *observingLocker
	mgr        *session.Manager
}

func TestSeatSuite(t *testing.T) { suite.Run(t, new(SeatSuite)) }

func (s *SeatSuite) SetupTest() {
	s.sessions, s.encounters = newFakeSessions(), newFakeEncounters()
	s.characters = testCharacters()
	s.seats = newFakeSeats()
	s.locker = &observingLocker{}
	mgr, err := session.NewManager(&session.Config{Seats: s.seats, Locker: s.locker,
		PresentationIDs: testPresentationIDs{}, Dice: testDice{}, TurnDriver: session.Pass{},
		Sessions: s.sessions, Encounters: s.encounters, Characters: s.characters,
		Events: session.DiscardEvents{},
	})
	s.Require().NoError(err)
	s.mgr = mgr
	for _, id := range []string{"sess", "other"} {
		_, err = mgr.StartSession(context.Background(), &session.StartSessionInput{
			Session: id, Encounter: "world-" + id, World: hexWorld(s.T()),
		})
		s.Require().NoError(err)
	}
}

func (s *SeatSuite) join(sess, member string) (*session.JoinOutput, error) {
	return s.mgr.Join(context.Background(), &session.JoinInput{Session: sess, Member: member, Position: hexCell(2, 2)})
}

func (s *SeatSuite) TestJoinSeatsUnderTheSessionThenTheCharacterGuard() {
	s.locker.calls, s.locker.characters = nil, nil

	out, err := s.join("sess", "bob")
	s.Require().NoError(err)

	s.Equal("sess", s.seats.seatOf("bob"))
	s.Contains(out.Saved.Written, "seat:bob")
	s.Equal([]string{"sess"}, s.locker.calls)
	s.Equal([]string{"bob"}, s.locker.characters)
}

func (s *SeatSuite) TestAJoinTheRunRefusesWritesNoSeat() {
	_, err := s.join("sess", "bob")
	s.Require().NoError(err)
	saves := s.seats.saves

	out, err := s.join("sess", "bob")
	s.Require().Error(err, "bob is already in the run")
	s.Nil(out)
	s.Equal(saves, s.seats.saves, "a refused join writes no seat")
	s.Equal("sess", s.seats.seatOf("bob"))
}

func (s *SeatSuite) TestJoiningASecondRunIsRefusedBeforeAnythingIsWritten() {
	_, err := s.join("sess", "bob")
	s.Require().NoError(err)
	charSaves, worldSaves, seatSaves := s.characters.saves, s.encounters.saves, s.seats.saves

	_, err = s.join("other", "bob")
	s.Require().ErrorIs(err, session.ErrSeatedElsewhere)

	s.Equal(charSaves, s.characters.saves, "no first-admission rest is saved")
	s.Equal(worldSaves, s.encounters.saves)
	s.Equal(seatSaves, s.seats.saves)
	s.Equal("sess", s.seats.seatOf("bob"))
}

func (s *SeatSuite) TestExitClearsTheSeatAfterTheRunIsSaved() {
	_, err := s.join("sess", "bob")
	s.Require().NoError(err)
	s.locker.calls, s.locker.characters = nil, nil

	out, err := s.mgr.Exit(context.Background(), &session.ExitInput{Session: "sess", Member: "bob"})
	s.Require().NoError(err)

	s.Empty(s.seats.seatOf("bob"))
	s.Equal([]string{"sess"}, s.locker.calls)
	s.Equal([]string{"bob"}, s.locker.characters, "the seat changes under the character's guard too")
	s.Contains(out.Saved.Written, "seat:bob")
	s.Less(positionOf(out.Saved.Written, "encounter:world-sess"), positionOf(out.Saved.Written, "seat:bob"),
		"the run that no longer holds bob lands before his seat is cleared")

	_, err = s.join("other", "bob")
	s.Require().NoError(err, "a released character can enter another run")
}

func (s *SeatSuite) TestEndClearsEverySeatTheRunHolds() {
	_, err := s.join("sess", "bob")
	s.Require().NoError(err)
	_, err = s.join("sess", "carol")
	s.Require().NoError(err)
	_, err = s.join("other", "dave")
	s.Require().NoError(err)

	out, err := s.mgr.End(context.Background(), &session.EndInput{Session: "sess", Ending: "out"})
	s.Require().NoError(err)

	s.Empty(s.seats.seatOf("bob"))
	s.Empty(s.seats.seatOf("carol"))
	s.Equal("other", s.seats.seatOf("dave"), "another run's seat is that run's business")
	s.Contains(out.Saved.Written, "seat:bob")
	s.Contains(out.Saved.Written, "seat:carol")
}

func (s *SeatSuite) TestLaunchTakesTheSessionThenEachPartyGuardInIDOrder() {
	s.locker.calls, s.locker.characters = nil, nil
	compiled := compileCamp(s.T(), campSource(s.T()))

	_, err := s.mgr.Launch(context.Background(), &session.LaunchInput{
		Session: "run", Dungeon: &compiled, Party: []string{"carol", "bob"},
	})
	s.Require().NoError(err)

	s.Equal([]string{"run"}, s.locker.calls)
	s.Equal([]string{"bob", "carol"}, s.locker.characters)
}

func (s *SeatSuite) TestLevelUpTakesTheGuardItsSeatNames() {
	s.locker.calls, s.locker.characters = nil, nil
	_, _ = s.mgr.LevelUp(context.Background(), &session.LevelUpInput{Character: "erin", HitPointMethod: session.HitPointsAverage})
	s.Equal([]string{"erin"}, s.locker.characters, "unseated: the character's own guard")
	s.Empty(s.locker.calls)

	_, err := s.join("sess", "erin")
	s.Require().NoError(err)
	s.locker.calls, s.locker.characters = nil, nil
	_, _ = s.mgr.LevelUp(context.Background(), &session.LevelUpInput{Character: "erin", HitPointMethod: session.HitPointsAverage})
	s.Equal([]string{"sess"}, s.locker.calls, "seated: its session's guard")
	s.Empty(s.locker.characters)
}

func positionOf(list []string, want string) int {
	for i, item := range list {
		if item == want {
			return i
		}
	}
	return -1
}

// keyedLocker is real per-key exclusion for sessions and characters, the
// shape a host's coordinator has. requested signals each guard request before
// it waits, so a test can tell a verb is blocked on a guard.
type keyedLocker struct {
	mu        sync.Mutex
	held      map[string]*sync.Mutex
	requested chan string
}

func newKeyedLocker() *keyedLocker {
	return &keyedLocker{held: map[string]*sync.Mutex{}, requested: make(chan string, 64)}
}

func (l *keyedLocker) lock(key string) func() {
	l.mu.Lock()
	m, ok := l.held[key]
	if !ok {
		m = &sync.Mutex{}
		l.held[key] = m
	}
	l.mu.Unlock()
	l.requested <- key
	m.Lock()
	return m.Unlock
}

func (l *keyedLocker) LockSession(_ context.Context, in *session.LockSessionInput) (*session.LockSessionOutput, error) {
	return &session.LockSessionOutput{Release: l.lock("session:" + in.Session)}, nil
}

func (l *keyedLocker) LockCharacter(_ context.Context, in *session.LockCharacterInput) (*session.LockCharacterOutput, error) {
	return &session.LockCharacterOutput{Release: l.lock("character:" + in.Character)}, nil
}

// pausingCharacters is a concurrency-safe character store that can hold the
// first read of one character until released, so a test can put a second
// verb in flight while the first holds a stale copy.
type pausingCharacters struct {
	mu      sync.Mutex
	byID    map[string]*character.Data
	pauseOn string
	paused  chan struct{}
	resume  chan struct{}
	once    sync.Once
}

func (p *pausingCharacters) GetCharacter(_ context.Context, id string) (*character.Data, error) {
	p.mu.Lock()
	data, ok := p.byID[id]
	var copied *character.Data
	if ok {
		copied = cloneCharacter(data)
	}
	p.mu.Unlock()
	if id == p.pauseOn {
		p.once.Do(func() {
			close(p.paused)
			<-p.resume
		})
	}
	if !ok {
		return nil, session.ErrNotFound
	}
	return copied, nil
}

func (p *pausingCharacters) SaveCharacter(_ context.Context, data *character.Data) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.byID[data.ID] = cloneCharacter(data)
	return nil
}

// TestALoadAndAnEquipIssuedTogetherBothLand is slice 4's race done-when: a
// verb that loads a seated sheet (a short rest) and an equip for the same
// character, issued together, both land — the equip is not overwritten by
// the rest's stale copy, because the seat sends the equip to the session's
// guard the rest already holds.
func (s *SeatSuite) TestALoadAndAnEquipIssuedTogetherBothLand() {
	wounded := withHitDice(quickFighter("alice"), 3)
	wounded.HitPoints = 10
	chars := &pausingCharacters{
		byID:    map[string]*character.Data{"alice": wounded, "bob": quickFighter("bob")},
		pauseOn: "alice", paused: make(chan struct{}), resume: make(chan struct{}),
	}
	locker := newKeyedLocker()
	seats := newFakeSeats()
	sessions, encounters := newFakeSessions(), newFakeEncounters()
	mgr, err := session.NewManager(&session.Config{Seats: seats, Locker: locker,
		PresentationIDs: testPresentationIDs{}, Dice: testDice{}, TurnDriver: session.Pass{},
		Sessions: sessions, Encounters: encounters, Characters: chars, Events: session.DiscardEvents{},
	})
	s.Require().NoError(err)
	_, err = mgr.StartSession(context.Background(), &session.StartSessionInput{
		Session: "sess", Encounter: "world", World: freeRoamDuelWorld(s.T()),
	})
	s.Require().NoError(err)
	s.Require().NoError(seats.SaveSeat(context.Background(), &session.SeatData{Character: "alice", Session: "sess"}))
	s.Require().NoError(seats.SaveSeat(context.Background(), &session.SeatData{Character: "bob", Session: "sess"}))
	drain(locker.requested)

	var wg sync.WaitGroup
	var restErr, equipErr error
	wg.Add(1)
	go func() {
		defer wg.Done()
		_, restErr = mgr.Rest(context.Background(), &session.RestInput{
			Session: "sess", Kind: session.RestShort, Resters: []session.Rester{{Member: "alice", HitDice: 1}},
		})
	}()
	<-chars.paused // the rest holds the session guard and a copy of alice's sheet
	drain(locker.requested)

	wg.Add(1)
	go func() {
		defer wg.Done()
		_, equipErr = mgr.Equip(context.Background(), &session.EquipInput{
			Character: "alice", Slot: string(character.SlotOffHand), Item: string(weapons.Dagger),
		})
	}()
	s.awaitRequest(locker.requested, "session:sess") // the equip waits on the same guard
	close(chars.resume)
	wg.Wait()

	s.Require().NoError(restErr)
	s.Require().NoError(equipErr)
	final, err := chars.GetCharacter(context.Background(), "alice")
	s.Require().NoError(err)
	s.Greater(final.HitPoints, 10, "the rest landed")
	s.Equal(string(weapons.Dagger), final.EquipmentSlots[character.SlotOffHand], "the equip landed and was not overwritten")
}

func (s *SeatSuite) awaitRequest(requested chan string, key string) {
	deadline := time.After(5 * time.Second)
	for {
		select {
		case got := <-requested:
			if got == key {
				return
			}
		case <-deadline:
			s.FailNow("no request for " + key)
		}
	}
}

func drain(requested chan string) {
	for {
		select {
		case <-requested:
		default:
			return
		}
	}
}
