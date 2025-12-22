// Copyright (C) 2024 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

// Package resources provides type definitions for resource-related constants.
// These types are used by rulebooks to define resource keys and reset types
// for managing limited-use abilities, spell slots, and other consumable game resources.
package resources

// ResourceKey identifies a specific resource type.
// Rulebooks should define constants of this type rather than using strings.
// Example: const RageUses ResourceKey = "rage_uses"
type ResourceKey string

// ResetType defines when a resource resets to its maximum value.
// Example: const ResetShortRest ResetType = "short_rest"
type ResetType string

// Common reset types that rulebooks might use.
// These are provided as examples but rulebooks can define their own.
const (
	ResetShortRest ResetType = "short_rest"
	ResetLongRest  ResetType = "long_rest"
	ResetDawn      ResetType = "dawn"
	ResetDusk      ResetType = "dusk"
	ResetNever     ResetType = "never"
	ResetManual    ResetType = "manual" // Only resets when explicitly told to
)

// ResourceAccessor provides access to an entity's resources.
// Features can use this to consume resources without directly depending on Character.
// This enables a clean separation where the owner (Character) manages resources
// and features just consume them via this interface.
type ResourceAccessor interface {
	// IsResourceAvailable returns true if the resource has at least 1 use remaining.
	// Returns false if the resource doesn't exist or is exhausted.
	IsResourceAvailable(key ResourceKey) bool

	// UseResource attempts to consume the specified amount from a resource.
	// Returns an error if the resource doesn't exist or has insufficient uses.
	UseResource(key ResourceKey, amount int) error
}
