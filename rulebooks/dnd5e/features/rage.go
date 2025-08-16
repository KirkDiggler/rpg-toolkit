// Copyright (C) 2024 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package features

import (
	"context"
	"errors"

	"github.com/KirkDiggler/rpg-toolkit/core"
	"github.com/KirkDiggler/rpg-toolkit/events"
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
	bus   *events.Bus

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

	// Now we have bus access! But we need:
	// 1. Combat event types defined (EventTypeAttack, EventTypeDamageReceived)
	// 2. Event handler methods (r.onAttack, r.onDamageReceived)
	// 3. RageStartedEvent type defined
	// For now, just showing we can access the bus:
	if r.bus != nil {
		// TODO: r.bus.Subscribe(combat.EventTypeAttack, r.onAttack)
		// TODO: r.bus.Subscribe(combat.EventTypeDamageReceived, r.onDamageReceived)
		// TODO: r.bus.Publish(&RageStartedEvent{Owner: owner})
	}

	return nil
}