// Copyright (C) 2024 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package events

import (
	"context"
	"errors"
	"fmt"
	"sync"

	"github.com/KirkDiggler/rpg-toolkit/core/chain"
)

// Common errors for chain operations
var (
	ErrDuplicateID = errors.New("modifier ID already exists")
	ErrIDNotFound  = errors.New("modifier ID not found")
)

// StagedChain implements chain.Chain[T] with ordered stage execution.
// It processes data through stages in the order they were defined.
type StagedChain[T any] struct {
	mu        sync.RWMutex
	stages    []chain.Stage
	modifiers map[chain.Stage][]modifier[T]
	idToStage map[string]chain.Stage // Track which stage an ID belongs to
}

// modifier wraps a handler with its ID
type modifier[T any] struct {
	id      string
	handler func(context.Context, T) (T, error)
}

// NewStagedChain creates a new chain with the specified stage order.
// Modifiers will be executed in the order stages are provided.
func NewStagedChain[T any](stages []chain.Stage) *StagedChain[T] {
	modifiers := make(map[chain.Stage][]modifier[T])
	for _, stage := range stages {
		modifiers[stage] = make([]modifier[T], 0)
	}
	
	return &StagedChain[T]{
		stages:    stages,
		modifiers: modifiers,
		idToStage: make(map[string]chain.Stage),
	}
}

// Add implements chain.Chain[T]
func (c *StagedChain[T]) Add(stage chain.Stage, id string, handler func(context.Context, T) (T, error)) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	
	// Check for duplicate ID
	if _, exists := c.idToStage[id]; exists {
		return ErrDuplicateID
	}
	
	// Add modifier to stage
	c.modifiers[stage] = append(c.modifiers[stage], modifier[T]{
		id:      id,
		handler: handler,
	})
	
	// Track ID to stage mapping
	c.idToStage[id] = stage
	
	return nil
}

// Remove implements chain.Chain[T]
func (c *StagedChain[T]) Remove(id string) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	
	// Find which stage contains this ID
	stage, exists := c.idToStage[id]
	if !exists {
		return ErrIDNotFound
	}
	
	// Remove from stage's modifiers
	mods := c.modifiers[stage]
	for i, mod := range mods {
		if mod.id == id {
			c.modifiers[stage] = append(mods[:i], mods[i+1:]...)
			delete(c.idToStage, id)
			return nil
		}
	}
	
	// Should not reach here if idToStage is consistent
	return ErrIDNotFound
}

// Execute implements chain.Chain[T]
func (c *StagedChain[T]) Execute(ctx context.Context, data T) (T, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	
	result := data
	
	// Process each stage in order
	for _, stage := range c.stages {
		mods := c.modifiers[stage]
		
		// Execute all modifiers in this stage
		for _, mod := range mods {
			var err error
			result, err = mod.handler(ctx, result)
			if err != nil {
				return result, fmt.Errorf("stage %s, modifier %s: %w", stage, mod.id, err)
			}
		}
	}
	
	return result, nil
}