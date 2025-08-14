// Copyright (C) 2024 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package events

import "github.com/KirkDiggler/rpg-toolkit/core"

// Common typed keys for event context data.
// Only keys we have actual use cases for.
// Rulebooks define their own domain-specific keys.

var (
	// Core combat data

	// KeyDamage is the amount of damage
	KeyDamage = NewTypedKey[int]("damage")
	// KeyDamageType is the type of damage
	KeyDamageType = NewTypedKey[string]("damageType")

	// Entity references

	// KeySource is the source of the event
	KeySource = NewTypedKey[*core.Entity]("source")
	// KeyTarget is the target of the event
	KeyTarget = NewTypedKey[*core.Entity]("target")
)

// Type aliases for convenience (optional)

// IntKey is an integer typed key
type IntKey = TypedKey[int]

// StringKey is a string typed key
type StringKey = TypedKey[string]

// BoolKey is a boolean typed key
type BoolKey = TypedKey[bool]

// FloatKey is a float64 typed key
type FloatKey = TypedKey[float64]
