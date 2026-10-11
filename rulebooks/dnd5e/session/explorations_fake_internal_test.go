// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package session

import (
	"context"
	"sync"
)

// memoryExplorations is an in-memory exploration repository for this
// package's internal tests. Config.Explorations is required, so every test
// Config wires one.
type memoryExplorations struct {
	mu   sync.Mutex
	byID map[string]ExplorationData
}

func newFakeExplorations() *memoryExplorations {
	return &memoryExplorations{byID: map[string]ExplorationData{}}
}

func (f *memoryExplorations) GetExploration(_ context.Context, id string) (*ExplorationData, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	profile, ok := f.byID[id]
	if !ok {
		return nil, ErrNotFound
	}
	return &profile, nil
}

func (f *memoryExplorations) SaveExploration(_ context.Context, data *ExplorationData) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.byID[data.Character] = *data
	return nil
}
