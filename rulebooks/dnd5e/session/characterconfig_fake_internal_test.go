// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package session

import (
	"context"
	"sync"
)

// memoryCharacterConfig is an in-memory character configuration reader for
// this package's internal tests. Config.CharacterConfig is required, so every
// test Config wires one. A missing entry is a character who has not chosen.
type memoryCharacterConfig struct {
	mu      sync.Mutex
	sharing map[string]bool
}

func newFakeCharacterConfig() *memoryCharacterConfig {
	return &memoryCharacterConfig{sharing: map[string]bool{}}
}

func (f *memoryCharacterConfig) DiscoverySharing(_ context.Context, id string) (*bool, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	choice, ok := f.sharing[id]
	if !ok {
		return nil, nil
	}
	return &choice, nil
}
