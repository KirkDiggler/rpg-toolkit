// Copyright (C) 2024 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package features

import (
	"github.com/KirkDiggler/rpg-toolkit/core"
)

// FeatureKey identifies specific feature types
type FeatureKey string

// Feature key constants
const (
	FeatureKeyRage FeatureKey = "rage" // Barbarian rage feature
)

// FeatureInput is the standard input for all D&D 5e features
type FeatureInput struct {
	Target core.Entity `json:"target,omitempty"`
	// When we have actual use cases for other fields, we'll add them
}

// ResourceType identifies what resource a feature consumes
type ResourceType string

// Resource type constants for features
const (
	ResourceTypeRageUses    ResourceType = "rage_uses"        // Barbarian rage uses
	ResourceTypeKiPoints    ResourceType = "ki_points"        // Monk ki points
	ResourceTypeSuperiority ResourceType = "superiority_dice" // Fighter superiority dice
)

// ResetType identifies when a feature's uses reset
type ResetType string

// Reset type constants for when features refresh
const (
	ResetTypeShortRest ResetType = "short_rest" // Resets on short rest
	ResetTypeLongRest  ResetType = "long_rest"  // Resets on long rest
	ResetTypeDawn      ResetType = "dawn"       // Resets at dawn
)

// Feature is the D&D 5e specific interface for character features.
// It extends core.Action but adds D&D 5e specific methods.
type Feature interface {
	core.Action[FeatureInput]

	// GetResourceType returns what resource this feature consumes
	GetResourceType() ResourceType

	// ResetsOn returns when this feature's uses reset
	ResetsOn() ResetType
}
