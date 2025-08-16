// Copyright (C) 2024 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package core

import "context"

// Action represents something that can be activated in the game.
// The generic type T defines what input the action requires.
//
// Actions are the fundamental unit of "doing things" in RPGs - casting spells,
// using abilities, activating items, or any other triggered behavior.
// The toolkit provides this interface, but all implementations live in rulebooks.
//
// Example usage in a rulebook:
//
//	type RageInput struct{}  // No input needed for rage
//
//	type Rage struct {
//	    id   string
//	    uses int
//	}
//
//	func (r *Rage) GetID() string { return r.id }
//	func (r *Rage) GetType() string { return "feature" }
//
//	func (r *Rage) CanActivate(ctx context.Context, owner Entity, input RageInput) error {
//	    if r.uses <= 0 {
//	        return errors.New("no rage uses remaining")
//	    }
//	    return nil
//	}
//
//	func (r *Rage) Activate(ctx context.Context, owner Entity, input RageInput) error {
//	    r.uses--
//	    // Apply rage effects via event bus
//	    return nil
//	}
type Action[T any] interface {
	Entity // Has GetID() and GetType()

	// CanActivate checks if this action can currently be activated.
	// Should return an error if the action cannot be used (no resources,
	// conditions not met, on cooldown, etc.)
	CanActivate(ctx context.Context, owner Entity, input T) error

	// Activate performs the action.
	// Should consume resources, apply effects, and trigger events as needed.
	// The owner is the entity performing the action.
	Activate(ctx context.Context, owner Entity, input T) error
}

