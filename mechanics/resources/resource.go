// Copyright (C) 2024 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

// Package resources provides infrastructure for managing consumable game resources
// such as spell slots, ability uses, hit dice, and action economy.
package resources

import (
	"github.com/KirkDiggler/rpg-toolkit/core"
	"github.com/KirkDiggler/rpg-toolkit/events"
)

//go:generate mockgen -destination=mock/mock_resource.go -package=mock github.com/KirkDiggler/rpg-toolkit/mechanics/resources Resource

// Resource represents a consumable game resource with current and maximum values.
type Resource interface {
	core.Entity // Resources are entities with ID and Type

	// Owner returns the entity that owns this resource.
	Owner() core.Entity

	// Key returns the resource identifier (e.g., "spell_slots_1", "rage_uses").
	Key() string

	// Current returns the current amount of this resource.
	Current() int

	// Maximum returns the maximum amount of this resource.
	Maximum() int

	// Consume attempts to use the specified amount of resource.
	// Returns an error if insufficient resources are available.
	Consume(amount int) error

	// Restore adds the specified amount to the resource, up to maximum.
	Restore(amount int)

	// SetCurrent sets the current value directly.
	SetCurrent(value int)

	// SetMaximum sets the maximum value.
	SetMaximum(value int)

	// RestoreOnShortRest returns the amount restored on a short rest.
	// Deprecated: Use RestoreOnTrigger with game-specific triggers instead
	RestoreOnShortRest() int

	// RestoreOnLongRest returns the amount restored on a long rest.
	// Deprecated: Use RestoreOnTrigger with game-specific triggers instead
	RestoreOnLongRest() int

	// RestoreOnTrigger returns the amount to restore for a given trigger.
	// Triggers are game-specific strings like "my.game.short_rest" or "dawn".
	// Returns 0 if the resource doesn't respond to the trigger.
	// Special return values:
	//   -1: Restore to full (maximum - current)
	RestoreOnTrigger(trigger string) int

	// IsAvailable returns true if any resource is available.
	IsAvailable() bool
}

// ResourceType represents the category of resource.
type ResourceType string

// Resource type constants
const (
	ResourceTypeSpellSlot   ResourceType = "spell_slot"
	ResourceTypeAbilityUse  ResourceType = "ability_use"
	ResourceTypeHitDice     ResourceType = "hit_dice"
	ResourceTypeAction      ResourceType = "action"
	ResourceTypeBonusAction ResourceType = "bonus_action"
	ResourceTypeReaction    ResourceType = "reaction"
	ResourceTypeCustom      ResourceType = "custom"
)

// RestorationType indicates when a resource is restored.
type RestorationType string

// Restoration type constants
const (
	RestoreNever     RestorationType = "never"
	RestoreShortRest RestorationType = "short_rest"
	RestoreLongRest  RestorationType = "long_rest"
	RestoreTurn      RestorationType = "turn"
	RestoreCustom    RestorationType = "custom"
)

// ResourceConsumedEvent is published when a resource is consumed.
type ResourceConsumedEvent struct {
	*events.GameEvent
	Resource Resource
	Amount   int
}

// ResourceRestoredEvent is published when a resource is restored.
type ResourceRestoredEvent struct {
	*events.GameEvent
	Resource Resource
	Amount   int
	Reason   string // "short_rest", "long_rest", "ability", etc.
}

// Event types for resource management
const (
	EventResourceConsumed = "resource.consumed"
	EventResourceRestored = "resource.restored"
	EventShortRest        = "rest.short"
	EventLongRest         = "rest.long"
)
