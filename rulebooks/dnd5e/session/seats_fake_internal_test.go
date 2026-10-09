// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package session

import (
	"context"
	"sync"
)

// memorySeats is an in-memory seat repository for this package's internal
// tests.
type memorySeats struct {
	mu   sync.Mutex
	byID map[string]SeatData
}

func newFakeSeats() *memorySeats { return &memorySeats{byID: map[string]SeatData{}} }

func (f *memorySeats) GetSeat(_ context.Context, character string) (*SeatData, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	seat, ok := f.byID[character]
	if !ok {
		return nil, ErrNotFound
	}
	return &seat, nil
}

func (f *memorySeats) SaveSeat(_ context.Context, data *SeatData) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.byID[data.Character] = *data
	return nil
}
