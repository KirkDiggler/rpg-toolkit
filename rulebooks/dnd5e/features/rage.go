// Copyright (C) 2024 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package features

import (
	"context"
	"errors"

	"github.com/KirkDiggler/rpg-toolkit/core"
)

// EntityType constants for features
const (
	EntityTypeFeature core.EntityType = "feature"
)

// Rage implements a barbarian's rage feature
type Rage struct {
	id    string
	uses  int
	level int

	// Track current state
	currentUses int
	active      bool
}

// Entity interface
func (r *Rage) GetID() string            { return r.id }
func (r *Rage) GetType() core.EntityType { return EntityTypeFeature }

// Feature interface methods
func (r *Rage) GetResourceType() ResourceType { return ResourceTypeRageUses }
func (r *Rage) ResetsOn() ResetType           { return ResetTypeLongRest }

// Action interface
func (r *Rage) CanActivate(ctx context.Context, owner core.Entity, input FeatureInput) error {
	if r.currentUses <= 0 {
		return errors.New("no rage uses remaining")
	}
	if r.active {
		return errors.New("already raging")
	}
	return nil
}

func (r *Rage) Activate(ctx context.Context, owner core.Entity, input FeatureInput) error {
	if err := r.CanActivate(ctx, owner, input); err != nil {
		return err
	}

	r.currentUses--
	r.active = true

	// TODO: When we have event bus access:
	// 1. Subscribe to attack events to add damage bonus
	// 2. Subscribe to damage received events for resistance
	// 3. Publish RageStartedEvent
	// The subscriptions ARE the rage effect - they persist until rage ends

	return nil
}