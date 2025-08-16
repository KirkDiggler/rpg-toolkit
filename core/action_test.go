// Copyright (C) 2024 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package core_test

import (
	"context"
	"errors"
	"testing"

	"github.com/KirkDiggler/rpg-toolkit/core"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Example: An action with no input (like Rage)
type EmptyInput struct{}

type SimpleAction struct {
	id        string
	uses      int
	activated bool
}

func (s *SimpleAction) GetID() string   { return s.id }
func (s *SimpleAction) GetType() string { return "simple" }

func (s *SimpleAction) CanActivate(ctx context.Context, owner core.Entity, input EmptyInput) error {
	if s.uses <= 0 {
		return errors.New("no uses remaining")
	}
	return nil
}

func (s *SimpleAction) Activate(ctx context.Context, owner core.Entity, input EmptyInput) error {
	s.uses--
	s.activated = true
	return nil
}

// Example: An action with targeting input (like a spell)
type TargetInput struct {
	Target   core.Entity
	Distance float64
}

type TargetedAction struct {
	id       string
	maxRange float64
	lastTarget core.Entity
}

func (t *TargetedAction) GetID() string   { return t.id }
func (t *TargetedAction) GetType() string { return "targeted" }

func (t *TargetedAction) CanActivate(ctx context.Context, owner core.Entity, input TargetInput) error {
	if input.Distance > t.maxRange {
		return errors.New("target out of range")
	}
	if input.Target == nil {
		return errors.New("no target specified")
	}
	return nil
}

func (t *TargetedAction) Activate(ctx context.Context, owner core.Entity, input TargetInput) error {
	t.lastTarget = input.Target
	// Would apply effects to target here
	return nil
}

// MockEntity for testing
type MockEntity struct {
	id    string
	eType string
}

func (m *MockEntity) GetID() string   { return m.id }
func (m *MockEntity) GetType() string { return m.eType }

func TestActionInterface(t *testing.T) {
	t.Run("SimpleAction", func(t *testing.T) {
		action := &SimpleAction{
			id:   "rage",
			uses: 3,
		}
		owner := &MockEntity{id: "barbarian", eType: "character"}
		
		// Verify it implements the interface
		var _ core.Action[EmptyInput] = action
		
		// Can activate when has uses
		err := action.CanActivate(context.Background(), owner, EmptyInput{})
		require.NoError(t, err)
		
		// Activate consumes a use
		err = action.Activate(context.Background(), owner, EmptyInput{})
		require.NoError(t, err)
		assert.Equal(t, 2, action.uses)
		assert.True(t, action.activated)
		
		// Use remaining uses
		action.Activate(context.Background(), owner, EmptyInput{})
		action.Activate(context.Background(), owner, EmptyInput{})
		
		// Cannot activate with no uses
		err = action.CanActivate(context.Background(), owner, EmptyInput{})
		assert.EqualError(t, err, "no uses remaining")
	})
	
	t.Run("TargetedAction", func(t *testing.T) {
		action := &TargetedAction{
			id:       "fireball",
			maxRange: 150.0,
		}
		owner := &MockEntity{id: "wizard", eType: "character"}
		target := &MockEntity{id: "goblin", eType: "monster"}
		
		// Verify it implements the interface
		var _ core.Action[TargetInput] = action
		
		// Can activate with valid target in range
		input := TargetInput{
			Target:   target,
			Distance: 100.0,
		}
		err := action.CanActivate(context.Background(), owner, input)
		require.NoError(t, err)
		
		// Activate tracks the target
		err = action.Activate(context.Background(), owner, input)
		require.NoError(t, err)
		assert.Equal(t, target, action.lastTarget)
		
		// Cannot activate if target out of range
		farInput := TargetInput{
			Target:   target,
			Distance: 200.0,
		}
		err = action.CanActivate(context.Background(), owner, farInput)
		assert.EqualError(t, err, "target out of range")
		
		// Cannot activate without target
		noTargetInput := TargetInput{
			Distance: 50.0,
		}
		err = action.CanActivate(context.Background(), owner, noTargetInput)
		assert.EqualError(t, err, "no target specified")
	})
	
	t.Run("DifferentInputTypes", func(t *testing.T) {
		// This demonstrates that different actions have different input types
		// and they're type-safe at compile time
		
		simple := &SimpleAction{id: "rage", uses: 1}
		targeted := &TargetedAction{id: "fireball", maxRange: 150}
		
		owner := &MockEntity{id: "player", eType: "character"}
		
		// Each action only accepts its specific input type
		simple.Activate(context.Background(), owner, EmptyInput{})
		
		targeted.Activate(context.Background(), owner, TargetInput{
			Target:   &MockEntity{id: "enemy", eType: "monster"},
			Distance: 50.0,
		})
		
		// These would not compile (commented out to keep test passing):
		// simple.Activate(context.Background(), owner, TargetInput{})  // Wrong input type
		// targeted.Activate(context.Background(), owner, EmptyInput{}) // Wrong input type
	})
}