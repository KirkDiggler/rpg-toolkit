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
}

// GetID returns the entity's unique identifier
func (r *Rage) GetID() string { return r.id }

// GetType returns the entity type (feature)
func (r *Rage) GetType() core.EntityType { return dnd5e.EntityTypeFeature }

// GetResourceType returns what resource this feature consumes
func (r *Rage) GetResourceType() ResourceType { return ResourceTypeRageUses }

// ResetsOn returns when this feature's uses reset
func (r *Rage) ResetsOn() ResetType { return ResetTypeLongRest }

// CanActivate just returns nil - all validation in Activate
func (r *Rage) CanActivate(_ context.Context, _ core.Entity, _ FeatureInput) error {
	return nil // All validation happens in Activate
}

// Activate consumes a use and publishes a condition event
func (r *Rage) Activate(ctx context.Context, owner core.Entity, input FeatureInput) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	// Check if we can activate
	if r.currentUses <= 0 {
		return errors.New("no rage uses remaining")
	}

	// TODO: Check if owner already has raging condition
	// This would require access to the character's condition list
	// For now, we'll let the character handle duplicate condition logic

	// Consume a use
	r.currentUses--

	// Publish condition applied event
	// The character will receive this and apply the raging condition
	return r.bus.Publish(dnd5e.NewConditionAppliedEvent(
		owner.GetID(),
		"dnd5e:conditions:raging",
		r.GetID(),
		map[string]any{
			"level":               r.level,  // For damage bonus calculation
			"duration":            10,       // Rage lasts 10 rounds
			"attacked_this_round": false,
			"was_hit_this_round":  false,
		},
	))
}

