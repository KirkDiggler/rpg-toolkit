// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package session_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/session"
)

// sharedSeats hands out the same pointer on every read, the shape that makes
// a Seat answer which aliases the repository visible.
type sharedSeats struct {
	*fakeSeats
	held map[string]*session.SeatData
}

func (s *sharedSeats) GetSeat(ctx context.Context, character string) (*session.SeatData, error) {
	if seat, ok := s.held[character]; ok {
		return seat, nil
	}
	return s.fakeSeats.GetSeat(ctx, character)
}

// SeatReadSuite covers Manager.Seat: the seat is a fact a caller outside the
// run can read, and reading it changes nothing.
type SeatReadSuite struct {
	suite.Suite

	seats *fakeSeats
	mgr   *session.Manager
}

func TestSeatReadSuite(t *testing.T) { suite.Run(t, new(SeatReadSuite)) }

func (s *SeatReadSuite) build(seats session.SeatRepository) *session.Manager {
	sessions, encounters := newFakeSessions(), newFakeEncounters()
	mgr, err := session.NewManager(&session.Config{Seats: seats, Locker: &observingLocker{},
		PresentationIDs: testPresentationIDs{}, Dice: testDice{}, TurnDriver: session.Pass{},
		Sessions: sessions, Encounters: encounters, Characters: testCharacters(),
		Events: session.DiscardEvents{},
	})
	s.Require().NoError(err)
	return mgr
}

func (s *SeatReadSuite) SetupTest() {
	s.seats = newFakeSeats()
	s.mgr = s.build(s.seats)
	run := hexWorld()
	run.Session = "sess"
	launchScene(s.T(), s.mgr, run)
}

func (s *SeatReadSuite) seatOf(character string) (*session.SeatOutput, error) {
	return s.mgr.Seat(context.Background(), &session.SeatInput{Character: character})
}

func (s *SeatReadSuite) TestALaunchedCharacterNamesItsSession() {
	out, err := s.seatOf("alice")
	s.Require().NoError(err)
	s.Equal("alice", out.Seat.Character)
	s.Equal("sess", out.Seat.Session)
}

func (s *SeatReadSuite) TestAnExitedCharacterHoldsNoSeat() {
	_, err := s.mgr.Exit(context.Background(), &session.ExitInput{Session: "sess", Member: "alice"})
	s.Require().NoError(err)
	// The repository still holds a record, with no session, for the cleared seat.
	s.Require().Contains(s.seats.byID, "alice")

	_, err = s.seatOf("alice")
	s.ErrorIs(err, session.ErrNoSeat)
}

func (s *SeatReadSuite) TestAnEndedRunLeavesNoSeat() {
	_, err := s.mgr.End(context.Background(), &session.EndInput{Session: "sess", Ending: "out"})
	s.Require().NoError(err)

	_, err = s.seatOf("alice")
	s.ErrorIs(err, session.ErrNoSeat)
}

func (s *SeatReadSuite) TestANeverSeatedCharacterHoldsNoSeat() {
	_, err := s.seatOf("nobody")
	s.ErrorIs(err, session.ErrNoSeat)
}

func (s *SeatReadSuite) TestABadAskIsRefused() {
	_, err := s.mgr.Seat(context.Background(), nil)
	s.ErrorIs(err, session.ErrNilInput)

	_, err = s.seatOf("")
	s.ErrorIs(err, session.ErrNoCharacter)
}

func (s *SeatReadSuite) TestSeatWritesNothing() {
	before := s.seats.saves
	_, err := s.seatOf("alice")
	s.Require().NoError(err)
	_, err = s.seatOf("nobody")
	s.Require().ErrorIs(err, session.ErrNoSeat)

	s.Equal(before, s.seats.saves)
	_, stored := s.seats.byID["nobody"]
	s.False(stored, "asking never creates a seat")
}

func (s *SeatReadSuite) TestTheAnswerIsACopy() {
	shared := &sharedSeats{fakeSeats: newFakeSeats(), held: map[string]*session.SeatData{
		"alice": {Character: "alice", Session: "sess"},
	}}
	mgr := s.build(shared)

	first, err := mgr.Seat(context.Background(), &session.SeatInput{Character: "alice"})
	s.Require().NoError(err)
	first.Seat.Session = "tampered"

	second, err := mgr.Seat(context.Background(), &session.SeatInput{Character: "alice"})
	s.Require().NoError(err)
	s.Equal("sess", second.Seat.Session)
	s.Equal("sess", shared.held["alice"].Session)
}
