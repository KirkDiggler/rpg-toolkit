// Copyright (C) 2024 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package features

import (
	"context"
	"errors"
	"sync"

	"github.com/KirkDiggler/rpg-toolkit/core"
	"github.com/KirkDiggler/rpg-toolkit/events"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e"
)

// Rage implements a barbarian's rage feature
type Rage struct {
	id    string
	uses  int
	level int
	bus   *events.Bus

	// Protect concurrent access to state
	mu sync.RWMutex

	// Track current state (protected by mu)
	currentUses int
	active      bool
	owner       core.Entity // Who is raging

	// Track subscriptions for cleanup (protected by mu)
	subscriptions []string
}

// GetID returns the entity's unique identifier
func (r *Rage) GetID() string { return r.id }

// GetType returns the entity type (feature)
func (r *Rage) GetType() core.EntityType { return dnd5e.EntityTypeFeature }

// GetResourceType returns what resource this feature consumes
func (r *Rage) GetResourceType() ResourceType { return ResourceTypeRageUses }

// ResetsOn returns when this feature's uses reset
func (r *Rage) ResetsOn() ResetType { return ResetTypeLongRest }

// CanActivate checks if rage can be activated
func (r *Rage) CanActivate(_ context.Context, _ core.Entity, _ FeatureInput) error {
	r.mu.RLock()
	defer r.mu.RUnlock()

	if r.currentUses <= 0 {
		return errors.New("no rage uses remaining")
	}
	if r.active {
		return errors.New("already raging")
	}
	return nil
}

// Activate enters rage mode and creates a rage condition
func (r *Rage) Activate(ctx context.Context, owner core.Entity, input FeatureInput) error {
	if err := r.CanActivate(ctx, owner, input); err != nil {
		return err
	}

	r.mu.Lock()
	r.currentUses--
	r.active = true
	r.owner = owner // Store who is raging
	r.mu.Unlock()

	// Create and apply the rage condition
	rageCondition := NewRagingCondition(owner.GetID(), r.level, r.bus)

	// Initialize the condition (sets up event subscriptions)
	if err := rageCondition.OnApply(); err != nil {
		return err
	}

	// Store the rage condition reference
	r.mu.Lock()
	r.subscriptions = append(r.subscriptions, rageCondition.GetID())
	r.mu.Unlock()

	// Publish rage started event for backwards compatibility
	_ = r.bus.Publish(&dnd5e.RageStartedEvent{
		Owner:       owner,
		DamageBonus: r.getDamageBonus(),
	})

	return nil
}

// getDamageBonus returns the rage damage bonus based on barbarian level
func (r *Rage) getDamageBonus() int {
	if r.level >= 16 {
		return 4
	} else if r.level >= 9 {
		return 3
	}
	return 2
}

// Deactivate ends rage mode
func (r *Rage) Deactivate() error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if !r.active {
		return errors.New("not currently raging")
	}

	r.active = false

	// The RagingCondition will handle its own cleanup via deferred operations
	// when it receives the appropriate events or times out

	return nil
}
