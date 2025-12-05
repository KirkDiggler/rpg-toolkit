// Copyright (C) 2024 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

// Package combat provides D&D 5e combat mechanics implementation
package combat

import (
	"context"

	coreResources "github.com/KirkDiggler/rpg-toolkit/core/resources"
	"github.com/KirkDiggler/rpg-toolkit/events"
	"github.com/KirkDiggler/rpg-toolkit/mechanics/resources"
	"github.com/KirkDiggler/rpg-toolkit/rpgerr"
	dnd5eEvents "github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/events"
)

// RecoverableResource wraps a mechanics/resources.Resource and implements
// events.BusEffect to automatically restore resources based on rest events.
//
// Purpose: Provides self-managing resource recovery tied to the event system.
// When applied, it subscribes to rest events and automatically restores the
// resource when a matching rest type occurs.
//
// Example: A Fighter's Second Wind feature that recovers on a short rest,
// or a Wizard's spell slots that recover on a long rest.
type RecoverableResource struct {
	*resources.Resource                         // Embedded resource for tracking current/maximum
	CharacterID         string                  // Filter rest events by character
	ResetType           coreResources.ResetType // When to restore (short_rest, long_rest, etc)
	subscriptionID      string                  // Track subscription for removal
	applied             bool                    // Track if subscribed
}

// Ensure RecoverableResource implements events.BusEffect
var _ events.BusEffect = (*RecoverableResource)(nil)

// RecoverableResourceConfig provides configuration for creating a recoverable resource
type RecoverableResourceConfig struct {
	ID          string                  // Unique identifier for the resource
	Maximum     int                     // Maximum value for the resource
	CharacterID string                  // Character this resource belongs to
	ResetType   coreResources.ResetType // When the resource recovers
}

// NewRecoverableResource creates a new recoverable resource with the given configuration.
// The resource starts at full capacity (Current = Maximum).
func NewRecoverableResource(config RecoverableResourceConfig) *RecoverableResource {
	return &RecoverableResource{
		Resource:    resources.NewResource(config.ID, config.Maximum),
		CharacterID: config.CharacterID,
		ResetType:   config.ResetType,
		applied:     false,
	}
}

// Apply subscribes this resource to the rest event system.
// When a matching rest event occurs (same CharacterID and ResetType),
// the resource will automatically restore to full.
func (r *RecoverableResource) Apply(ctx context.Context, bus events.EventBus) error {
	if r.applied {
		return rpgerr.New(rpgerr.CodeAlreadyExists, "recoverable resource already applied")
	}

	// Subscribe to rest events
	rests := dnd5eEvents.RestTopic.On(bus)
	subID, err := rests.Subscribe(ctx, r.onRest)
	if err != nil {
		return err
	}

	r.subscriptionID = subID
	r.applied = true
	return nil
}

// Remove unsubscribes this resource from the rest event system.
// After removal, rest events will no longer restore this resource.
func (r *RecoverableResource) Remove(ctx context.Context, bus events.EventBus) error {
	if !r.applied {
		return nil // Not applied, nothing to remove
	}

	err := bus.Unsubscribe(ctx, r.subscriptionID)
	if err != nil {
		return err
	}

	r.subscriptionID = ""
	r.applied = false
	return nil
}

// IsApplied returns true if this resource is currently subscribed to rest events
func (r *RecoverableResource) IsApplied() bool {
	return r.applied
}

// onRest handles rest events and restores the resource if the event matches
// our CharacterID and ResetType
func (r *RecoverableResource) onRest(_ context.Context, event dnd5eEvents.RestEvent) error {
	// Only restore if the rest is for our character
	if event.CharacterID != r.CharacterID {
		return nil
	}

	// Only restore if the rest type matches our reset type
	if event.RestType != r.ResetType {
		return nil
	}

	// Restore resource to full
	r.RestoreToFull()
	return nil
}
