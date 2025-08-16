// Copyright (C) 2024 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

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
