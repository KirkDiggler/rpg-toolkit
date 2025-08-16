// Copyright (C) 2024 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package features

// ID identifies a specific feature.
// Rulebooks define constants of this type for their features.
// Example: const Rage ID = "rage"
type ID string

// Source identifies where a feature comes from.
// Example: const SourceBarbarian Source = "barbarian"
type Source string

// ActivationType identifies how a feature is activated.
// Example: const ActivationAction ActivationType = "action"
type ActivationType string

// Common activation types that many systems use
const (
	ActivationAction      ActivationType = "action"
	ActivationBonusAction ActivationType = "bonus_action"
	ActivationReaction    ActivationType = "reaction"
	ActivationPassive     ActivationType = "passive"
	ActivationTriggered   ActivationType = "triggered"
)
