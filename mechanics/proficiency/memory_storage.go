// Copyright (C) 2024 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package proficiency

import (
	"fmt"
	"sync"
)

// MemoryStorage is an in-memory implementation of Storage for testing and simple use cases.
type MemoryStorage struct {
	mu            sync.RWMutex
	proficiencies map[string][]Proficiency // entityID -> proficiencies
}

// NewMemoryStorage creates a new in-memory storage.
func NewMemoryStorage() *MemoryStorage {
	return &MemoryStorage{
		proficiencies: make(map[string][]Proficiency),
	}
}

// GetProficiencies returns all proficiencies for an entity.
func (s *MemoryStorage) GetProficiencies(entityID string) ([]Proficiency, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	profs, ok := s.proficiencies[entityID]
	if !ok {
		return nil, nil
	}

	// Return a copy to prevent external modification
	result := make([]Proficiency, len(profs))
	copy(result, profs)
	return result, nil
}

// SaveProficiency adds or updates a proficiency for an entity.
func (s *MemoryStorage) SaveProficiency(entityID string, prof Proficiency) error {
	if prof == nil {
		return fmt.Errorf("proficiency cannot be nil")
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	// Get or create proficiency list
	profs, ok := s.proficiencies[entityID]
	if !ok {
		profs = make([]Proficiency, 0)
	}

	// Check if proficiency already exists and update
	for i, existing := range profs {
		if existing.Key() == prof.Key() {
			profs[i] = prof
			s.proficiencies[entityID] = profs
			return nil
		}
	}

	// Add new proficiency
	profs = append(profs, prof)
	s.proficiencies[entityID] = profs
	return nil
}

// RemoveProficiency removes a proficiency from an entity.
func (s *MemoryStorage) RemoveProficiency(entityID string, profKey string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	profs, ok := s.proficiencies[entityID]
	if !ok {
		return nil
	}

	// Find and remove the proficiency
	for i, prof := range profs {
		if prof.Key() == profKey {
			// Remove by swapping with last element and truncating
			profs[i] = profs[len(profs)-1]
			profs = profs[:len(profs)-1]

			if len(profs) == 0 {
				delete(s.proficiencies, entityID)
			} else {
				s.proficiencies[entityID] = profs
			}
			return nil
		}
	}

	return nil
}

// HasProficiency checks if an entity has a specific proficiency.
func (s *MemoryStorage) HasProficiency(entityID string, profType Type, key string) (bool, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	profs, ok := s.proficiencies[entityID]
	if !ok {
		return false, nil
	}

	for _, prof := range profs {
		if prof.Type() == profType && prof.Key() == key {
			return true, nil
		}
	}

	return false, nil
}
