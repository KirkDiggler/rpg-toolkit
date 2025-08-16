// Copyright (C) 2024 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

// Package damage provides type definitions for damage-related constants.
// These types are used by rulebooks to define their damage types.
package damage

// Type identifies a specific damage type.
// Rulebooks define constants of this type for their damage types.
// Example: const Fire Type = "fire"
type Type string

// ResistanceType identifies what provides resistance to damage.
// Example: const ResistanceRage ResistanceType = "rage"
type ResistanceType string

// ImmunityType identifies what provides immunity to damage.
// Example: const ImmunityPotion ImmunityType = "potion_of_fire_immunity"
type ImmunityType string

// VulnerabilityType identifies what causes vulnerability to damage.
// Example: const VulnerabilityHexed VulnerabilityType = "hexed"
type VulnerabilityType string

// Category groups damage types for rules purposes.
// Example: physical damage (bludgeoning, piercing, slashing) vs magical
type Category string

// Common categories that many systems use
const (
	CategoryPhysical Category = "physical"
	CategoryMagical  Category = "magical"
	CategoryPsychic  Category = "psychic"
)
