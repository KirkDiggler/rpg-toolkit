// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package session_test

import (
	"context"
	"sync"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/session"
)

// fakeSeats is an in-memory seat repository. Safe for concurrent use, because
// the seat race tests drive two verbs at once.
type fakeSeats struct {
	mu     sync.Mutex
	byID   map[string]session.SeatData
	saves  int
	failOn string
}

func newFakeSeats() *fakeSeats { return &fakeSeats{byID: map[string]session.SeatData{}} }

func (f *fakeSeats) GetSeat(_ context.Context, character string) (*session.SeatData, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	seat, ok := f.byID[character]
	if !ok {
		return nil, session.ErrNotFound
	}
	return &seat, nil
}

func (f *fakeSeats) SaveSeat(_ context.Context, data *session.SeatData) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.failOn != "" && f.failOn == data.Character {
		return errSeatOutage
	}
	f.saves++
	f.byID[data.Character] = *data
	return nil
}

// seatOf is the session holding a character, empty when unseated.
func (f *fakeSeats) seatOf(character string) string {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.byID[character].Session
}

// errSeatOutage is a seat repository failing to write.
var errSeatOutage = seatOutage{}

type seatOutage struct{}

func (seatOutage) Error() string { return "seat store unavailable" }
