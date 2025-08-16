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
	owner       core.Entity // Who is raging
	
	// Track subscriptions for cleanup
	subscriptions []string
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
	r.owner = owner // Store who is raging

	// Event types will be defined in the rulebook (dnd5e package)
	// For now, documenting what we need:
	
	// TODO: Define in dnd5e package:
	// - EventTypeAttack (check if attacker == r.owner, add damage bonus)
	// - EventTypeDamageReceived (check if target == r.owner, apply resistance)
	// - EventTypeTurnEnd (check if entity == r.owner, track rage ending)
	// - RageStartedEvent, RageEndedEvent types
	
	// The pattern will be:
	// subID, _ := r.bus.SubscribeWithFilter(dnd5e.EventTypeAttack, r.onAttack, filterFunc)
	// r.subscriptions = append(r.subscriptions, subID)

	return nil
}

// endRage handles cleanup when rage ends
func (r *Rage) endRage() {
	r.active = false
	
	// Unsubscribe from all events
	for _, subID := range r.subscriptions {
		r.bus.Unsubscribe(subID)
	}
	r.subscriptions = nil
	
	// TODO: Publish RageEndedEvent
	// r.bus.Publish(&RageEndedEvent{Owner: r.owner})
	
	r.owner = nil
}