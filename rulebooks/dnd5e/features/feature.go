// Copyright (C) 2024 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package features

import (
	"github.com/KirkDiggler/rpg-toolkit/core"
)

// FeatureKey identifies specific feature types
type FeatureKey string

const (
	FeatureKeyRage FeatureKey = "rage"
)

// FeatureInput is the standard input for all D&D 5e features
type FeatureInput struct {
	Target core.Entity `json:"target,omitempty"`
	// When we have actual use cases for other fields, we'll add them
}

// ResourceType identifies what resource a feature consumes
type ResourceType string

const (
	ResourceTypeRageUses    ResourceType = "rage_uses"
	ResourceTypeKiPoints    ResourceType = "ki_points"
	ResourceTypeSuperiority ResourceType = "superiority_dice"
)

// ResetType identifies when a feature's uses reset
type ResetType string

const (
	ResetTypeShortRest ResetType = "short_rest"
	ResetTypeLongRest  ResetType = "long_rest"
	ResetTypeDawn      ResetType = "dawn"
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
